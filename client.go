package notebooklm

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Client is the Go client for Google NotebookLM's internal RPC API.
type Client struct {
	httpClient   *http.Client
	cookieHeader string
	csrfToken    string
	sessionID    string
	// dlClient is a separate HTTP client with a domain-aware cookie jar,
	// used ONLY for media downloads that cross-domain redirect to
	// googleusercontent.com etc. The RPC client (httpClient) keeps using
	// the flat Cookie header for consistency with existing code.
	dlClient *http.Client
}

// NewClient creates a new NotebookLM client from a storage state file.
func NewClient(storagePath string) (*Client, error) {
	if storagePath == "" {
		storagePath = DefaultStoragePath()
	}

	cookies, err := LoadCookiesFromStorage(storagePath)
	if err != nil {
		return nil, err
	}

	jar, err := LoadCookieJarFromStorage(storagePath)
	if err != nil {
		return nil, err
	}

	tokens, err := FetchTokens(cookies)
	if err != nil {
		return nil, err
	}

	return &Client{
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		dlClient: &http.Client{
			Timeout: 600 * time.Second, // bigger downloads can take minutes
			Jar:     jar,
		},
		cookieHeader: cookies,
		csrfToken:    tokens.CSRF,
		sessionID:    tokens.SessionID,
	}, nil
}

// --- Notebook operations ---

// Notebook represents a NotebookLM notebook.
type Notebook struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// ListNotebooks returns all notebooks in the account.
func (c *Client) ListNotebooks() ([]Notebook, error) {
	result, err := c.doRPC(RPCListNotebooks, []interface{}{}, "/")
	if err != nil {
		return nil, err
	}

	// Result is a nested array — extract notebook entries
	var raw []json.RawMessage
	if err := json.Unmarshal(result, &raw); err != nil {
		return nil, fmt.Errorf("parsing notebooks: %w", err)
	}

	var notebooks []Notebook
	// First element is typically the notebook list
	if len(raw) > 0 {
		var entries []json.RawMessage
		if json.Unmarshal(raw[0], &entries) == nil {
			for _, entry := range entries {
				var arr []json.RawMessage
				if json.Unmarshal(entry, &arr) == nil && len(arr) >= 2 {
					var id, title string
					json.Unmarshal(arr[0], &id)
					// Title is usually at index 1 or nested
					if len(arr) > 1 {
						// Try direct string
						if json.Unmarshal(arr[1], &title) != nil {
							// Try nested array [[title]]
							var nested []json.RawMessage
							if json.Unmarshal(arr[1], &nested) == nil && len(nested) > 0 {
								json.Unmarshal(nested[0], &title)
							}
						}
					}
					if id != "" {
						notebooks = append(notebooks, Notebook{ID: id, Title: title})
					}
				}
			}
		}
	}

	return notebooks, nil
}

// CreateNotebook creates a new notebook with the given title.
func (c *Client) CreateNotebook(title string) (string, error) {
	params := []interface{}{title, nil, nil, []int{2}, []int{1}}
	result, err := c.doRPC(RPCCreateNotebook, params, "/")
	if err != nil {
		return "", err
	}

	// Response: [title, null, notebook_id, ...]
	var arr []json.RawMessage
	if err := json.Unmarshal(result, &arr); err != nil {
		// Try as plain string
		var id string
		if json.Unmarshal(result, &id) == nil && id != "" {
			return id, nil
		}
		return "", fmt.Errorf("parsing create response: %w (raw: %s)", err, string(result))
	}

	// Notebook ID is at index 2
	if len(arr) > 2 {
		var id string
		if json.Unmarshal(arr[2], &id) == nil && id != "" {
			return id, nil
		}
	}
	// Fallback: try index 0
	if len(arr) > 0 {
		var id string
		if json.Unmarshal(arr[0], &id) == nil && id != "" {
			return id, nil
		}
	}

	return "", fmt.Errorf("could not extract notebook ID from response: %s", string(result))
}

// DeleteNotebook deletes a notebook by ID.
func (c *Client) DeleteNotebook(notebookID string) error {
	_, err := c.doRPC(RPCDeleteNotebook, []interface{}{notebookID}, "/")
	return err
}

// --- Source operations ---

// AddYouTubeSource adds a YouTube video as a source to a notebook.
func (c *Client) AddYouTubeSource(notebookID, youtubeURL string) error {
	params := []interface{}{
		// Source definition: URL at index 7
		[]interface{}{[]interface{}{nil, nil, nil, nil, nil, nil, nil, []string{youtubeURL}, nil, nil, 1}},
		notebookID,
		[]int{2},
		[]interface{}{1, nil, nil, nil, nil, nil, nil, nil, nil, nil, []int{1}},
	}
	_, err := c.doRPC(RPCAddSource, params, "/notebook/"+notebookID)
	return err
}

// AddURLSource adds a web URL as a source to a notebook.
func (c *Client) AddURLSource(notebookID, sourceURL string) error {
	params := []interface{}{
		[]interface{}{[]interface{}{nil, nil, nil, nil, nil, nil, nil, nil, []string{sourceURL}, nil, 1}},
		notebookID,
		[]int{2},
		[]interface{}{1, nil, nil, nil, nil, nil, nil, nil, nil, nil, []int{1}},
	}
	_, err := c.doRPC(RPCAddSource, params, "/notebook/"+notebookID)
	return err
}

// AddTextSource adds plain text as a source to a notebook.
func (c *Client) AddTextSource(notebookID, title, content string) error {
	params := []interface{}{
		[]interface{}{[]interface{}{nil, nil, content, nil, nil, nil, nil, nil, nil, nil, 1}},
		notebookID,
		[]int{2},
		[]interface{}{1, nil, nil, nil, nil, nil, nil, nil, nil, nil, []int{1}},
	}
	_, err := c.doRPC(RPCAddSource, params, "/notebook/"+notebookID)
	return err
}

// --- Chat operations ---

// Chat sends a question to a notebook and returns the response.
// Uses NotebookLM's GenerateFreeFormStreamed endpoint (not batchexecute).
func (c *Client) Chat(notebookID, question string) (string, error) {
	queryURL := "https://notebooklm.google.com/_/LabsTailwindUi/data/google.internal.labs.tailwind.orchestration.v1.LabsTailwindOrchestrationService/GenerateFreeFormStreamed"

	// Build params: [sources, question, history, metadata, convID, null, null, notebookID, 1]
	convID := fmt.Sprintf("%x", time.Now().UnixNano())
	params := []interface{}{
		nil,      // sources (nil = all)
		question, // question
		nil,      // conversation history
		[]interface{}{2, nil, []int{1}, []int{1}}, // metadata
		convID,     // conversation ID
		nil,        // [5]
		nil,        // [6]
		notebookID, // [7] notebook ID
		1,          // [8]
	}

	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return "", err
	}

	// Wrap: [null, paramsJSON]
	wrapper := []interface{}{nil, string(paramsJSON)}
	wrapperJSON, err := json.Marshal(wrapper)
	if err != nil {
		return "", err
	}

	body := "f.req=" + url.QueryEscape(string(wrapperJSON)) + "&at=" + url.QueryEscape(c.csrfToken) + "&"

	qParams := url.Values{
		"bl":     {""},
		"hl":     {"en"},
		"_reqid": {fmt.Sprintf("%d", time.Now().UnixMilli()%1000000)},
		"rt":     {"c"},
	}
	if c.sessionID != "" {
		qParams.Set("f.sid", c.sessionID)
	}
	fullURL := queryURL + "?" + qParams.Encode()

	req, err := http.NewRequest("POST", fullURL, strings.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8")
	req.Header.Set("Cookie", c.cookieHeader)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("chat request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("chat failed with HTTP %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Parse streamed response — extract text chunks
	return extractChatText(string(respBody)), nil
}

// extractChatText pulls readable text from the streaming response.
// Response format: chunked, with ["wrb.fr", null, "[[\"text\", ...]]"] entries.
func extractChatText(body string) string {
	cleaned := stripAntiXSSI(body)
	chunks := parseChunkedResponse(cleaned)

	// Collect ALL text chunks — NotebookLM (Gemini 2.x) now sends "thinking"
	// tokens first, then the actual answer. We want the last (longest) chunk
	// which is the real response, not the "Initiating analysis..." preamble.
	var allTexts []string

	for _, chunk := range chunks {
		var items []json.RawMessage
		if json.Unmarshal(chunk, &items) != nil {
			continue
		}
		for _, item := range items {
			var arr []json.RawMessage
			if json.Unmarshal(item, &arr) != nil || len(arr) < 3 {
				continue
			}
			var tag string
			json.Unmarshal(arr[0], &tag)
			if tag != "wrb.fr" {
				continue
			}
			// arr[2] is a JSON-encoded string containing the response
			var dataStr string
			if json.Unmarshal(arr[2], &dataStr) != nil {
				continue
			}
			// Parse the inner JSON: [["text", null, [...]]]
			var inner []json.RawMessage
			if json.Unmarshal([]byte(dataStr), &inner) != nil || len(inner) == 0 {
				continue
			}
			// First element is an array with text at index 0
			var textArr []json.RawMessage
			if json.Unmarshal(inner[0], &textArr) == nil && len(textArr) > 0 {
				var text string
				if json.Unmarshal(textArr[0], &text) == nil && text != "" {
					allTexts = append(allTexts, text)
				}
			}
			// Or first element is the text directly
			var text string
			if json.Unmarshal(inner[0], &text) == nil && text != "" {
				allTexts = append(allTexts, text)
			}
		}
	}

	if len(allTexts) > 0 {
		// Return the longest text — that's the actual answer, not the thinking preamble
		best := allTexts[0]
		for _, t := range allTexts[1:] {
			if len(t) > len(best) {
				best = t
			}
		}
		return best
	}

	// Fallback
	if len(body) > 500 {
		return body[:500] + "..."
	}
	return body
}

// --- Artifact operations ---

// GenerateAudio creates an audio overview (podcast) for a notebook.
func (c *Client) GenerateAudio(notebookID string, format int) (string, error) {
	params := []interface{}{
		notebookID,
		nil,           // sources
		ArtifactAudio, // type
		nil,           // custom instructions
		nil,
		format, // AudioDeepDive, AudioBrief, AudioCritique, AudioDebate
	}
	result, err := c.doRPC(RPCCreateArtifact, params, "/notebook/"+notebookID)
	if err != nil {
		return "", err
	}

	// Extract artifact ID
	var arr []json.RawMessage
	if json.Unmarshal(result, &arr) == nil && len(arr) > 0 {
		var id string
		json.Unmarshal(arr[0], &id)
		return id, nil
	}
	return "", fmt.Errorf("could not extract artifact ID")
}

// GenerateArtifact creates any artifact type (video, quiz, slides, etc.).
func (c *Client) GenerateArtifact(notebookID string, typeCode int) (string, error) {
	params := []interface{}{
		notebookID,
		nil,      // sources
		typeCode, // artifact type
	}
	result, err := c.doRPC(RPCCreateArtifact, params, "/notebook/"+notebookID)
	if err != nil {
		return "", err
	}

	var arr []json.RawMessage
	if json.Unmarshal(result, &arr) == nil && len(arr) > 0 {
		var id string
		json.Unmarshal(arr[0], &id)
		return id, nil
	}
	return "", fmt.Errorf("could not extract artifact ID")
}

// Artifact represents a NotebookLM artifact (audio, video, quiz, etc.).
//
// Field mapping to the raw RPC response array (verified against notebooklm-py
// v0.3 Artifact.from_api_response, 2026-04-20):
//
//	data[0]  → ID       (string UUID)
//	data[1]  → Title    (string; CJK chars → zh, else en)
//	data[2]  → TypeCode (1=video, 3=audio, 7=infographic, etc.)
//	data[4]  → Status
//	data[15][0] → CreatedAt (unix seconds)
type Artifact struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	TypeCode  int    `json:"type_code"`
	Status    int    `json:"status"`
	CreatedAt int64  `json:"created_at,omitempty"`
}

// listArtifactsRaw — internal helper. Returns both the parsed Artifact list
// AND the raw per-artifact JSON arrays, so callers who need to extract the
// download URL (which lives at type-specific positions inside the raw array)
// don't have to re-fetch.
func (c *Client) listArtifactsRaw(notebookID string) ([]Artifact, []json.RawMessage, error) {
	const filterSuggested = `NOT artifact.status = "ARTIFACT_STATUS_SUGGESTED"`
	params := []interface{}{[]int{2}, notebookID, filterSuggested}

	result, err := c.doRPC(RPCListArtifacts, params, "/notebook/"+notebookID)
	if err != nil {
		return nil, nil, err
	}

	if os.Getenv("LOCALKIN_NB_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[DEBUG ListArtifacts raw] %s\n", string(result))
	}

	var outer []json.RawMessage
	if json.Unmarshal(result, &outer) != nil || len(outer) == 0 {
		return nil, nil, nil
	}
	entries := outer
	var probeOuter []json.RawMessage
	if json.Unmarshal(outer[0], &probeOuter) == nil && len(probeOuter) > 0 {
		var probeInner []json.RawMessage
		if json.Unmarshal(probeOuter[0], &probeInner) == nil {
			entries = probeOuter
		}
	}

	var artifacts []Artifact
	var rawEntries []json.RawMessage
	for _, entry := range entries {
		var arr []json.RawMessage
		if json.Unmarshal(entry, &arr) != nil || len(arr) < 3 {
			continue
		}
		var a Artifact
		json.Unmarshal(arr[0], &a.ID)
		if len(arr) > 1 {
			json.Unmarshal(arr[1], &a.Title)
		}
		json.Unmarshal(arr[2], &a.TypeCode)
		if len(arr) > 4 {
			json.Unmarshal(arr[4], &a.Status)
		}
		if len(arr) > 15 {
			var tsArr []json.RawMessage
			if json.Unmarshal(arr[15], &tsArr) == nil && len(tsArr) > 0 {
				json.Unmarshal(tsArr[0], &a.CreatedAt)
			}
		}
		if a.ID != "" {
			artifacts = append(artifacts, a)
			rawEntries = append(rawEntries, entry)
		}
	}
	return artifacts, rawEntries, nil
}

// ListArtifacts returns all artifacts in a notebook.
//
// RPC params (verified against notebooklm-py v0.3 _artifacts.py:263, 2026-04-20):
//
//	params = [[2], notebook_id, "NOT artifact.status = \"ARTIFACT_STATUS_SUGGESTED\""]
//
// Passing only [notebook_id] makes Google's backend return a null payload
// (the outer envelope says "generic" but the inner data slot is null).
//
// Response envelope is typically [[<artifact-entry>, ...]] — a single-element
// outer list wrapping the actual entries. Each entry is:
//
//	[ id, title, typeCode, _, status, _, _, _, _, [..variant..], _, _, _, _, _, [ts_sec, ts_ns], ... ]
//
// We read fields 0 (id), 1 (title), 2 (type), 4 (status), 15[0] (createdAt).
func (c *Client) ListArtifacts(notebookID string) ([]Artifact, error) {
	artifacts, _, err := c.listArtifactsRaw(notebookID)
	return artifacts, err
}

// DownloadArtifact downloads an artifact to a local file.
//
// Architecture note (2026-04-20): the previous implementation called a
// separate RPC `RPCExportArtifact` to get a download URL — that RPC is for
// exporting reports/data-tables to Google Docs/Sheets, NOT for media
// download. Its response contains no URL, so `could not extract download URL`
// was the inevitable outcome.
//
// The correct mechanism (per notebooklm-py's download_audio/video/infographic
// in _artifacts.py:955+) is: the ListArtifacts response already contains
// media URLs embedded in type-specific positions:
//
//   - Audio (type 1):       artifact[6][5]  — list of {url, ?, mime} tuples, find mime="audio/mp4"
//   - Video (type 3):       artifact[8]     — list of lists, first URL with mime="video/mp4" preferred (quality tag == 4)
//   - Infographic (type 7): artifact[N][2][0][1][0] for N scanned from end of entry — raw PNG url
//
// So "downloading" is really: list → find by ID → parse URL from raw → HTTP GET.
func (c *Client) DownloadArtifact(notebookID, artifactID, outputPath string) error {
	artifacts, rawEntries, err := c.listArtifactsRaw(notebookID)
	if err != nil {
		return fmt.Errorf("listing artifacts: %w", err)
	}

	var target Artifact
	var targetRaw json.RawMessage
	for i, a := range artifacts {
		if a.ID == artifactID {
			target = a
			targetRaw = rawEntries[i]
			break
		}
	}
	if target.ID == "" {
		return fmt.Errorf("artifact %s not found in notebook %s", artifactID, notebookID)
	}

	var arr []json.RawMessage
	if err := json.Unmarshal(targetRaw, &arr); err != nil {
		return fmt.Errorf("parsing artifact entry: %w", err)
	}

	var downloadURL string
	switch target.TypeCode {
	case ArtifactAudio:
		downloadURL = extractAudioURL(arr)
	case ArtifactVideo:
		downloadURL = extractVideoURL(arr)
	case ArtifactInfogr:
		downloadURL = extractInfographicURL(arr)
	default:
		return fmt.Errorf("download not yet implemented for type code %d (%s)",
			target.TypeCode, target.Title)
	}

	if downloadURL == "" {
		return fmt.Errorf("could not extract download URL for %s (%s)",
			target.Title, target.ID)
	}

	return c.httpGetToFile(downloadURL, outputPath)
}

// extractAudioURL — audio URL lives at artifact[6][5], which is a list of
// [url, ?, mime_type] tuples. Prefer mime "audio/mp4", fallback to first url.
func extractAudioURL(arr []json.RawMessage) string {
	if len(arr) <= 6 {
		return ""
	}
	var metadata []json.RawMessage
	if json.Unmarshal(arr[6], &metadata) != nil || len(metadata) <= 5 {
		return ""
	}
	var mediaList []json.RawMessage
	if json.Unmarshal(metadata[5], &mediaList) != nil {
		return ""
	}
	var fallback string
	for _, item := range mediaList {
		var tuple []json.RawMessage
		if json.Unmarshal(item, &tuple) != nil || len(tuple) < 1 {
			continue
		}
		var url string
		json.Unmarshal(tuple[0], &url)
		if fallback == "" {
			fallback = url
		}
		if len(tuple) > 2 {
			var mime string
			if json.Unmarshal(tuple[2], &mime) == nil && mime == "audio/mp4" {
				return url
			}
		}
	}
	return fallback
}

// extractVideoURL — video URL lives inside artifact[8], which is a list of
// sub-lists; the first sub-list whose first element starts with "http" is
// the media list. Within that media list, prefer mime "video/mp4" with
// quality tag 4, else take the first URL.
func extractVideoURL(arr []json.RawMessage) string {
	if len(arr) <= 8 {
		return ""
	}
	var metadata []json.RawMessage
	if json.Unmarshal(arr[8], &metadata) != nil {
		return ""
	}
	var mediaList []json.RawMessage
	for _, item := range metadata {
		var sub []json.RawMessage
		if json.Unmarshal(item, &sub) != nil || len(sub) == 0 {
			continue
		}
		var first []json.RawMessage
		if json.Unmarshal(sub[0], &first) != nil || len(first) == 0 {
			continue
		}
		var maybeURL string
		if json.Unmarshal(first[0], &maybeURL) == nil && strings.HasPrefix(maybeURL, "http") {
			mediaList = sub
			break
		}
	}
	if mediaList == nil {
		return ""
	}
	var fallback string
	for _, item := range mediaList {
		var tuple []json.RawMessage
		if json.Unmarshal(item, &tuple) != nil || len(tuple) < 1 {
			continue
		}
		var url string
		json.Unmarshal(tuple[0], &url)
		if fallback == "" {
			fallback = url
		}
		if len(tuple) > 2 {
			var mime string
			if json.Unmarshal(tuple[2], &mime) == nil && mime == "video/mp4" {
				// Prefer quality-tag 4 if present
				if len(tuple) > 1 {
					var q int
					if json.Unmarshal(tuple[1], &q) == nil && q == 4 {
						return url
					}
				}
				return url
			}
		}
	}
	return fallback
}

// extractInfographicURL — infographic URL lives at a deeply-nested position.
// notebooklm-py scans entry items in reverse looking for `item[2][0][1][0]`
// being an http string. Mirror that logic.
func extractInfographicURL(arr []json.RawMessage) string {
	for i := len(arr) - 1; i >= 0; i-- {
		var item []json.RawMessage
		if json.Unmarshal(arr[i], &item) != nil || len(item) < 3 {
			continue
		}
		var contentList []json.RawMessage
		if json.Unmarshal(item[2], &contentList) != nil || len(contentList) == 0 {
			continue
		}
		var contentFirst []json.RawMessage
		if json.Unmarshal(contentList[0], &contentFirst) != nil || len(contentFirst) < 2 {
			continue
		}
		var imgData []json.RawMessage
		if json.Unmarshal(contentFirst[1], &imgData) != nil || len(imgData) < 1 {
			continue
		}
		var url string
		if json.Unmarshal(imgData[0], &url) == nil && strings.HasPrefix(url, "http") {
			return url
		}
	}
	return ""
}

// httpGetToFile — fetch URL via the download-dedicated client (which has a
// domain-aware cookie jar), stream to file. Uses User-Agent mimicking a real
// browser because some Google content CDNs refuse the default Go UA.
func (c *Client) httpGetToFile(downloadURL, outputPath string) error {
	req, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "*/*")

	resp, err := c.dlClient.Do(req)
	if err != nil {
		return fmt.Errorf("downloading: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("download failed with HTTP %d", resp.StatusCode)
	}

	// If Google served an HTML page (redirect wall, auth challenge) instead
	// of media, the download has actually failed — surface it clearly.
	ct := resp.Header.Get("Content-Type")
	if strings.Contains(strings.ToLower(ct), "text/html") {
		return fmt.Errorf("server returned HTML (content-type=%q) instead of media — cookies or URL likely invalid", ct)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return err
	}
	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}

// --- Research operations ---

// StartResearch begins a web research query and auto-imports discovered sources.
// mode: "fast" or "deep"
func (c *Client) StartResearch(notebookID, query, mode string) (string, error) {
	rpcID := RPCStartFastResearch
	if mode == "deep" {
		rpcID = RPCStartDeepResearch
	}

	params := []interface{}{notebookID, query}
	result, err := c.doRPC(rpcID, params, "/notebook/"+notebookID)
	if err != nil {
		return "", err
	}

	// Extract research task ID
	var arr []json.RawMessage
	if json.Unmarshal(result, &arr) == nil && len(arr) > 0 {
		var taskID string
		json.Unmarshal(arr[0], &taskID)
		return taskID, nil
	}
	return string(result), nil
}

// GetNotebookMetadata returns detailed notebook info including sources.
func (c *Client) GetNotebookMetadata(notebookID string) (json.RawMessage, error) {
	return c.doRPC(RPCGetNotebook, []interface{}{notebookID}, "/notebook/"+notebookID)
}

// CheckAuth verifies that cookies and tokens are valid by making a test request.
func (c *Client) CheckAuth() error {
	_, err := c.ListNotebooks()
	return err
}

// GetShareStatus returns the sharing state of a notebook.
func (c *Client) GetShareStatus(notebookID string) (json.RawMessage, error) {
	return c.doRPC(RPCGetShareStatus, []interface{}{notebookID}, "/notebook/"+notebookID)
}

// GetSourceGuide retrieves the auto-generated summary for a notebook's sources.
func (c *Client) GetSourceGuide(notebookID string) (string, error) {
	params := []interface{}{notebookID}
	result, err := c.doRPC(RPCGetSourceGuide, params, "/notebook/"+notebookID)
	if err != nil {
		return "", err
	}

	var arr []json.RawMessage
	if json.Unmarshal(result, &arr) == nil && len(arr) > 0 {
		var guide string
		if json.Unmarshal(arr[0], &guide) == nil {
			return guide, nil
		}
	}
	return string(result), nil
}

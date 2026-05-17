// Package notebooklm provides a Go client for Google NotebookLM's internal RPC API.
// Reverse-engineered from the notebooklm-py project (github.com/teng-lin/notebooklm-py).
//
// This is an unofficial client using undocumented Google APIs.
// Use at your own risk — endpoints may change without notice.
package notebooklm

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

const (
	batchexecuteURL = "https://notebooklm.google.com/_/LabsTailwindUi/data/batchexecute"
	baseURL         = "https://notebooklm.google.com"
)

// RPC method IDs — obfuscated identifiers used by Google's batchexecute API.
const (
	// Notebook operations
	RPCListNotebooks  = "wXbhsf"
	RPCCreateNotebook = "CCqFvf"
	RPCGetNotebook    = "rLM1Ne"
	RPCRenameNotebook = "s0tc2d"
	RPCDeleteNotebook = "WWINqb"

	// Source operations
	RPCAddSource     = "izAoDd"
	RPCDeleteSource  = "tGMBJ"
	RPCGetSource     = "hizoJc"
	RPCRefreshSource = "FLmJqe"
	RPCUpdateSource  = "b7Wfje"

	// Summary and query
	RPCSummarize      = "VfAZjd"
	RPCGetSourceGuide = "tr032e"

	// Artifact operations
	RPCCreateArtifact = "R7cb6c"
	RPCListArtifacts  = "gArtLc"
	RPCDeleteArtifact = "V5N4be"
	RPCExportArtifact = "Krh3pd"

	// Conversation
	RPCGetLastConvID = "hPTbtc"
	RPCGetConvTurns  = "khqZz"

	// Notes & mind maps
	RPCGenerateMindMap = "yyryJe"
	RPCCreateNote      = "CYK0Xb"
	RPCGetNotes        = "cFji9"

	// Research
	RPCStartFastResearch = "Ljjv0c"
	RPCStartDeepResearch = "QA9ei"
	RPCPollResearch      = "e3bVqc"
	RPCImportResearch    = "LBwxtb"

	// Sharing
	RPCShareNotebook  = "QDyure"
	RPCGetShareStatus = "JFMDGd"

	// Settings
	RPCGetUserSettings = "ZwVcOc"
)

// Artifact type codes
const (
	ArtifactAudio     = 1
	ArtifactReport    = 2
	ArtifactVideo     = 3
	ArtifactQuiz      = 4
	ArtifactMindMap   = 5
	ArtifactInfogr    = 7
	ArtifactSlideDeck = 8
	ArtifactDataTable = 9
)

// Audio format codes
const (
	AudioDeepDive = 1
	AudioBrief    = 2
	AudioCritique = 3
	AudioDebate   = 4
)

// encodeRPCRequest builds the triple-nested batchexecute payload.
// Format: [[[rpcID, jsonParams, null, "generic"]]]
func encodeRPCRequest(rpcID string, params interface{}) (string, error) {
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return "", fmt.Errorf("marshaling params: %w", err)
	}

	// Build inner: [rpcID, paramsString, null, "generic"]
	inner := []interface{}{rpcID, string(paramsJSON), nil, "generic"}
	// Triple-wrap: [[[inner]]]
	wrapped := [][][]interface{}{{inner}}

	data, err := json.Marshal(wrapped)
	if err != nil {
		return "", fmt.Errorf("marshaling request: %w", err)
	}
	return string(data), nil
}

// buildRequestBody creates the form-encoded POST body for batchexecute.
func buildRequestBody(encodedReq, csrfToken string) string {
	parts := []string{
		"f.req=" + url.QueryEscape(encodedReq),
	}
	if csrfToken != "" {
		parts = append(parts, "at="+url.QueryEscape(csrfToken))
	}
	return strings.Join(parts, "&") + "&"
}

// buildURL constructs the batchexecute URL with query parameters.
func buildURL(rpcID, sourcePath, sessionID string) string {
	params := url.Values{
		"rpcids":      {rpcID},
		"source-path": {sourcePath},
		"hl":          {"en"},
		"rt":          {"c"},
	}
	if sessionID != "" {
		params.Set("f.sid", sessionID)
	}
	return batchexecuteURL + "?" + params.Encode()
}

// decodeResponse parses the batchexecute response.
// Strips anti-XSSI prefix, parses chunked format, extracts result for rpcID.
func decodeResponse(body string, rpcID string) (json.RawMessage, error) {
	// Strip anti-XSSI prefix: )]}'\n
	cleaned := stripAntiXSSI(body)

	// Parse chunked response (alternating: byte-count line, JSON line)
	chunks := parseChunkedResponse(cleaned)

	// Extract result for our RPC ID
	return extractRPCResult(chunks, rpcID)
}

// stripAntiXSSI removes Google's anti-XSSI prefix.
// The prefix is literal: )]}' followed by a newline.
func stripAntiXSSI(s string) string {
	prefixes := []string{")]}'\\'\n", ")]\\'\n", ")]}'\r\n", ")]}'\n"}
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return s[len(p):]
		}
	}
	return s
}

// parseChunkedResponse parses the alternating byte-count/JSON format.
func parseChunkedResponse(s string) []json.RawMessage {
	var chunks []json.RawMessage
	lines := strings.Split(strings.TrimSpace(s), "\n")

	i := 0
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			i++
			continue
		}

		// Try parsing as byte count (integer line)
		if _, err := strconv.Atoi(line); err == nil {
			i++ // skip byte count
			if i < len(lines) {
				jsonStr := lines[i]
				if json.Valid([]byte(jsonStr)) {
					chunks = append(chunks, json.RawMessage(jsonStr))
				}
				i++
			}
			continue
		}

		// Try direct JSON parse
		if json.Valid([]byte(line)) {
			chunks = append(chunks, json.RawMessage(line))
		}
		i++
	}
	return chunks
}

// extractRPCResult finds the result for a specific RPC ID in the chunks.
// Looks for ["wrb.fr", rpcID, resultData, ...] pattern.
func extractRPCResult(chunks []json.RawMessage, rpcID string) (json.RawMessage, error) {
	for _, chunk := range chunks {
		// Each chunk may be an array of items
		var items []json.RawMessage
		if err := json.Unmarshal(chunk, &items); err != nil {
			continue
		}

		// Items may be nested: check if first element is an array
		for _, item := range items {
			result, err := checkItem(item, rpcID)
			if err != nil {
				return nil, err
			}
			if result != nil {
				return result, nil
			}

			// Try nested: item might be an array of sub-items
			var subItems []json.RawMessage
			if json.Unmarshal(item, &subItems) == nil {
				for _, sub := range subItems {
					result, err := checkItem(sub, rpcID)
					if err != nil {
						return nil, err
					}
					if result != nil {
						return result, nil
					}
				}
			}
		}
	}
	return nil, fmt.Errorf("no result found for RPC ID: %s", rpcID)
}

// checkItem checks if a JSON array matches ["wrb.fr", rpcID, data, ...]
func checkItem(raw json.RawMessage, rpcID string) (json.RawMessage, error) {
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil || len(arr) < 3 {
		return nil, nil
	}

	var tag, id string
	json.Unmarshal(arr[0], &tag)
	json.Unmarshal(arr[1], &id)

	if tag == "er" && id == rpcID {
		return nil, fmt.Errorf("RPC error for %s: %s", rpcID, string(arr[2]))
	}

	if tag == "wrb.fr" && id == rpcID {
		// Result data might be a JSON string that needs one more parse
		var strData string
		if json.Unmarshal(arr[2], &strData) == nil {
			// It's a JSON-encoded string — parse the inner JSON
			if json.Valid([]byte(strData)) {
				return json.RawMessage(strData), nil
			}
			return json.RawMessage(`"` + strData + `"`), nil
		}
		// Already a JSON value
		return arr[2], nil
	}

	return nil, nil
}

// doRPC executes a single RPC call against NotebookLM's batchexecute API.
func (c *Client) doRPC(rpcID string, params interface{}, sourcePath string) (json.RawMessage, error) {
	encoded, err := encodeRPCRequest(rpcID, params)
	if err != nil {
		return nil, err
	}

	body := buildRequestBody(encoded, c.csrfToken)
	reqURL := buildURL(rpcID, sourcePath, c.sessionID)

	req, err := http.NewRequest("POST", reqURL, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8")
	req.Header.Set("Cookie", c.cookieHeader)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return nil, fmt.Errorf("authentication failed (HTTP %d) — run 'localkin notebooklm-login' to refresh", resp.StatusCode)
	}
	if resp.StatusCode == 429 {
		return nil, fmt.Errorf("rate limited by Google — wait and retry")
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d from NotebookLM API", resp.StatusCode)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if os.Getenv("LOCALKIN_NB_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[nb-debug] RPC %s → HTTP %d, body %d bytes\n", rpcID, resp.StatusCode, len(respBody))
		if len(respBody) < 2000 {
			fmt.Fprintf(os.Stderr, "[nb-debug] body: %s\n", string(respBody))
		}
	}

	return decodeResponse(string(respBody), rpcID)
}

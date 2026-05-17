package notebooklm

import (
	"encoding/json"
)

// =============================================================================
// EXPERIMENTAL — v0.1.1 surface expansion
// =============================================================================
//
// The methods in this file invoke RPC IDs declared in `rpc.go` but whose
// param shape we have NOT verified against a live NotebookLM session at
// release time. They use the most-likely shape inferred from neighbouring
// methods in `client.go` and from notebooklm-py's wire conventions.
//
// If you hit a runtime error like:
//
//	RPC error for s0tc2d: "..."
//
// the param shape is likely wrong. Run with LOCALKIN_NB_DEBUG=1 to capture
// the failing payload (the rpc.go doRPC dumps it on stderr), then file an
// issue with the redacted body. The fix is almost always a 1-line edit to
// the params slice below.
//
// Battle-tested, production-validated methods live in `client.go`.

// --- Notebooks (write + read) ---

// RenameNotebook changes a notebook's title.
//
// EXPERIMENTAL.
func (c *Client) RenameNotebook(notebookID, newTitle string) error {
	_, err := c.doRPC(RPCRenameNotebook, []interface{}{notebookID, newTitle}, "/notebook/"+notebookID)
	return err
}

// GetNotebook returns the full notebook record. This is the raw RPC; the
// already-stable `GetNotebookMetadata` is an alias kept for callers that
// don't want to type-assert.
//
// EXPERIMENTAL — response shape varies by Google build; treat as RawMessage.
func (c *Client) GetNotebook(notebookID string) (json.RawMessage, error) {
	return c.doRPC(RPCGetNotebook, []interface{}{notebookID}, "/notebook/"+notebookID)
}

// --- Sources ---

// DeleteSource removes a source from a notebook by source ID.
//
// EXPERIMENTAL — param shape mirrors notebooklm-py's batch-delete: the
// first param is a list because the underlying RPC supports deleting
// multiple sources in one call.
func (c *Client) DeleteSource(notebookID, sourceID string) error {
	_, err := c.doRPC(RPCDeleteSource, []interface{}{[]string{sourceID}, notebookID}, "/notebook/"+notebookID)
	return err
}

// GetSource returns a single source's full record (content excerpts,
// ingestion status, metadata).
//
// EXPERIMENTAL.
func (c *Client) GetSource(notebookID, sourceID string) (json.RawMessage, error) {
	return c.doRPC(RPCGetSource, []interface{}{notebookID, sourceID}, "/notebook/"+notebookID)
}

// RefreshSource asks NotebookLM to re-ingest a source (e.g. re-fetch a URL
// whose content has changed).
//
// EXPERIMENTAL.
func (c *Client) RefreshSource(notebookID, sourceID string) error {
	_, err := c.doRPC(RPCRefreshSource, []interface{}{notebookID, sourceID}, "/notebook/"+notebookID)
	return err
}

// UpdateSource changes a source's metadata (currently: title).
//
// EXPERIMENTAL.
func (c *Client) UpdateSource(notebookID, sourceID, newTitle string) error {
	_, err := c.doRPC(RPCUpdateSource, []interface{}{notebookID, sourceID, newTitle}, "/notebook/"+notebookID)
	return err
}

// --- Summaries ---

// Summarize returns the full auto-summary for a notebook (the long-form
// text NotebookLM generates when sources change).
//
// EXPERIMENTAL — distinct from `GetSourceGuide` which returns the
// shorter per-source overview.
func (c *Client) Summarize(notebookID string) (json.RawMessage, error) {
	return c.doRPC(RPCSummarize, []interface{}{notebookID}, "/notebook/"+notebookID)
}

// --- Artifacts ---

// DeleteArtifact removes an artifact (audio, report, mind map, etc.) from
// a notebook.
//
// EXPERIMENTAL — batch-delete shape (mirrors DeleteSource).
func (c *Client) DeleteArtifact(notebookID, artifactID string) error {
	_, err := c.doRPC(RPCDeleteArtifact, []interface{}{[]string{artifactID}, notebookID}, "/notebook/"+notebookID)
	return err
}

// ExportArtifact exports a Report or Data Table artifact to Google Docs /
// Sheets and returns the export response (containing the destination URL).
//
// NOTE: this is NOT the right way to download Audio/Video/Infographic —
// those URLs live inside ListArtifacts entries. See DownloadArtifact in
// client.go for the correct media-download path.
//
// EXPERIMENTAL.
func (c *Client) ExportArtifact(notebookID, artifactID string) (json.RawMessage, error) {
	return c.doRPC(RPCExportArtifact, []interface{}{notebookID, artifactID}, "/notebook/"+notebookID)
}

// --- Conversations ---

// GetLastConvID returns the most-recent conversation ID for a notebook.
// Useful for resuming a chat without spawning a fresh thread.
//
// EXPERIMENTAL.
func (c *Client) GetLastConvID(notebookID string) (string, error) {
	result, err := c.doRPC(RPCGetLastConvID, []interface{}{notebookID}, "/notebook/"+notebookID)
	if err != nil {
		return "", err
	}
	var arr []json.RawMessage
	if json.Unmarshal(result, &arr) == nil && len(arr) > 0 {
		var id string
		if json.Unmarshal(arr[0], &id) == nil && id != "" {
			return id, nil
		}
	}
	return string(result), nil
}

// GetConvTurns returns the message history of a conversation.
//
// EXPERIMENTAL.
func (c *Client) GetConvTurns(notebookID, convID string) (json.RawMessage, error) {
	return c.doRPC(RPCGetConvTurns, []interface{}{notebookID, convID}, "/notebook/"+notebookID)
}

// --- Notes & Mind Maps (RPC-level) ---

// GenerateMindMap creates a mind map via the legacy direct RPC. Distinct
// from `GenerateArtifact(notebookID, ArtifactMindMap)` which goes through
// the artifact pipeline (preferred for v2.x+ NotebookLM frontends).
//
// EXPERIMENTAL — kept for parity with notebooklm-py's `generate_mind_map`.
func (c *Client) GenerateMindMap(notebookID string) (json.RawMessage, error) {
	return c.doRPC(RPCGenerateMindMap, []interface{}{notebookID}, "/notebook/"+notebookID)
}

// CreateNote saves a note attached to a notebook (the right-hand "Notes"
// panel in the NotebookLM UI).
//
// EXPERIMENTAL.
func (c *Client) CreateNote(notebookID, title, content string) error {
	_, err := c.doRPC(RPCCreateNote, []interface{}{notebookID, title, content}, "/notebook/"+notebookID)
	return err
}

// GetNotes lists notes for a notebook.
//
// EXPERIMENTAL.
func (c *Client) GetNotes(notebookID string) (json.RawMessage, error) {
	return c.doRPC(RPCGetNotes, []interface{}{notebookID}, "/notebook/"+notebookID)
}

// --- Research ---

// PollResearch returns the status (and, when ready, output) of a research
// task started via StartResearch.
//
// EXPERIMENTAL.
func (c *Client) PollResearch(notebookID, researchTaskID string) (json.RawMessage, error) {
	return c.doRPC(RPCPollResearch, []interface{}{notebookID, researchTaskID}, "/notebook/"+notebookID)
}

// ImportResearch imports a finished research task's output as a source on
// the same notebook (so the report becomes searchable / summarizable).
//
// EXPERIMENTAL.
func (c *Client) ImportResearch(notebookID, researchTaskID string) error {
	_, err := c.doRPC(RPCImportResearch, []interface{}{notebookID, researchTaskID}, "/notebook/"+notebookID)
	return err
}

// --- Sharing (write) ---

// ShareNotebook grants access to a notebook.
//
// shareLevel: 1 = view-only, 2 = edit (best-guess from notebooklm-py;
// confirm against Google's UI semantics if it matters).
//
// EXPERIMENTAL — exact param shape (especially the email-list encoding)
// has the highest uncertainty in this file. If the call returns
// `RPC error for QDyure`, capture the failing payload and file an issue.
func (c *Client) ShareNotebook(notebookID string, emails []string, shareLevel int) error {
	_, err := c.doRPC(RPCShareNotebook, []interface{}{notebookID, emails, shareLevel}, "/notebook/"+notebookID)
	return err
}

// --- Settings ---

// GetUserSettings returns the current user's NotebookLM settings
// (audio voice preferences, notification toggles, etc.).
//
// EXPERIMENTAL.
func (c *Client) GetUserSettings() (json.RawMessage, error) {
	return c.doRPC(RPCGetUserSettings, []interface{}{}, "/")
}

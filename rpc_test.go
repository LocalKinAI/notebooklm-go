package notebooklm

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEncodeRPCRequest(t *testing.T) {
	params := []interface{}{"notebook123"}
	encoded, err := encodeRPCRequest(RPCGetNotebook, params)
	if err != nil {
		t.Fatal(err)
	}

	// Should be triple-nested: [[[rpcID, paramsJSON, null, "generic"]]]
	if !strings.Contains(encoded, RPCGetNotebook) {
		t.Error("should contain RPC ID")
	}
	if !strings.Contains(encoded, `"generic"`) {
		t.Error("should contain 'generic' marker")
	}

	// Verify structure
	var outer [][][]json.RawMessage
	if err := json.Unmarshal([]byte(encoded), &outer); err != nil {
		t.Fatalf("should be valid triple-nested JSON: %v", err)
	}
	if len(outer) != 1 || len(outer[0]) != 1 || len(outer[0][0]) != 4 {
		t.Error("should be [[[rpcID, params, null, generic]]]")
	}
}

func TestBuildRequestBody(t *testing.T) {
	body := buildRequestBody(`[[[test]]]`, "csrf_token_123")

	if !strings.Contains(body, "f.req=") {
		t.Error("should contain f.req parameter")
	}
	if !strings.Contains(body, "at=csrf_token_123") {
		t.Error("should contain CSRF token")
	}
	if !strings.HasSuffix(body, "&") {
		t.Error("should end with &")
	}
}

func TestBuildURL(t *testing.T) {
	u := buildURL(RPCListNotebooks, "/", "sid123")

	if !strings.Contains(u, batchexecuteURL) {
		t.Error("should start with batchexecute URL")
	}
	if !strings.Contains(u, "rpcids="+RPCListNotebooks) {
		t.Error("should contain RPC ID in query params")
	}
	if !strings.Contains(u, "f.sid=sid123") {
		t.Error("should contain session ID")
	}
}

func TestStripAntiXSSI(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{")]}'\n{\"data\":1}", `{"data":1}`},
		{")]\\'\n{\"data\":1}", `{"data":1}`},
		{"{\"data\":1}", `{"data":1}`}, // no prefix
	}
	for _, tt := range tests {
		got := stripAntiXSSI(tt.input)
		if got != tt.expected {
			t.Errorf("stripAntiXSSI(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestParseChunkedResponse(t *testing.T) {
	// Simulate chunked response: byte-count + JSON alternating
	response := "42\n[[\"wrb.fr\",\"wXbhsf\",\"[\\\"nb1\\\",\\\"My Notebook\\\"]\"]]\n"
	chunks := parseChunkedResponse(response)
	if len(chunks) == 0 {
		t.Fatal("should parse at least one chunk")
	}
}

func TestExtractRPCResult_Success(t *testing.T) {
	// Simulate a successful response chunk
	chunk := `[["wrb.fr","wXbhsf","[\"notebook_id_123\"]",null,null,null,"generic"]]`
	chunks := []json.RawMessage{json.RawMessage(chunk)}

	result, err := extractRPCResult(chunks, "wXbhsf")
	if err != nil {
		t.Fatal(err)
	}

	// Result should be the parsed inner JSON
	var arr []string
	if err := json.Unmarshal(result, &arr); err != nil {
		t.Fatalf("result should be valid JSON array: %v (raw: %s)", err, string(result))
	}
	if len(arr) == 0 || arr[0] != "notebook_id_123" {
		t.Errorf("unexpected result: %v", arr)
	}
}

func TestExtractRPCResult_Error(t *testing.T) {
	chunk := `[["er","wXbhsf",404]]`
	chunks := []json.RawMessage{json.RawMessage(chunk)}

	_, err := extractRPCResult(chunks, "wXbhsf")
	if err == nil {
		t.Fatal("should return error for RPC error response")
	}
}

func TestIsGoogleDomain(t *testing.T) {
	tests := []struct {
		domain string
		want   bool
	}{
		{".google.com", true},
		{"google.com", true},
		{"notebooklm.google.com", true},
		{".googleusercontent.com", true},
		{"evil.com", false},
		{"google.com.evil.com", false},
	}
	for _, tt := range tests {
		got := isGoogleDomain(tt.domain)
		if got != tt.want {
			t.Errorf("isGoogleDomain(%q) = %v, want %v", tt.domain, got, tt.want)
		}
	}
}

package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ruslano69/funcfinder/internal/embed"
	"github.com/ruslano69/funcfinder/internal/truth"
)

// runSession pipes JSON-RPC request lines through the dispatcher and returns
// the decoded responses (one per line the server emitted).
func runSession(t *testing.T, dir string, lines ...string) []rpcResponse {
	t.Helper()
	store, err := truth.Open(dir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	var buf strings.Builder
	m := &mcpServer{store: store, embc: embed.New("", ""), out: json.NewEncoder(&buf)}
	for _, l := range lines {
		m.dispatch(l)
	}

	var out []rpcResponse
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var r rpcResponse
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("bad response line %q: %v", line, err)
		}
		out = append(out, r)
	}
	return out
}

// contentText extracts the text payload from a tools/call result.
func contentText(t *testing.T, r rpcResponse) (string, bool) {
	t.Helper()
	res, ok := r.Result.(map[string]any)
	if !ok {
		t.Fatalf("result is not an object: %#v", r.Result)
	}
	isErr, _ := res["isError"].(bool)
	content, ok := res["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("no content in %#v", res)
	}
	first := content[0].(map[string]any)
	return first["text"].(string), isErr
}

func TestMCPHandshakeAndToolsList(t *testing.T) {
	resp := runSession(t, t.TempDir(),
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
	)
	// notification produced no response → exactly 2 responses.
	if len(resp) != 2 {
		t.Fatalf("want 2 responses (notification silent), got %d", len(resp))
	}
	init := resp[0].Result.(map[string]any)
	if init["protocolVersion"] != mcpProtocolVersion {
		t.Fatalf("protocolVersion = %v", init["protocolVersion"])
	}
	tools := resp[1].Result.(map[string]any)["tools"].([]any)
	if len(tools) != 8 {
		t.Fatalf("want 8 tools, got %d", len(tools))
	}
}

func TestMCPLifecycleViaTools(t *testing.T) {
	dir := t.TempDir()
	resp := runSession(t, dir,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"ingest","arguments":{"title":"Auth spec","content":"login uses OAuth2 device flow","type":"spec"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"publish","arguments":{"name":"2026.07","channel":"stable"}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"search","arguments":{"query":"OAuth2","channel":"stable"}}}`,
	)
	if len(resp) != 3 {
		t.Fatalf("want 3 responses, got %d", len(resp))
	}
	for i, r := range resp {
		if _, isErr := contentText(t, r); isErr {
			t.Fatalf("response %d unexpectedly isError", i+1)
		}
	}
	searchText, _ := contentText(t, resp[2])
	if !strings.Contains(searchText, "Auth spec") {
		t.Fatalf("search did not ground on the release: %s", searchText)
	}
}

func TestMCPErrorSemantics(t *testing.T) {
	resp := runSession(t, t.TempDir(),
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"search","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"bogus","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"nonexistent/method"}`,
	)
	// Tool-level failures are isError results, not JSON-RPC errors.
	if _, isErr := contentText(t, resp[0]); !isErr {
		t.Fatal("missing-arg should be a tool isError result")
	}
	if _, isErr := contentText(t, resp[1]); !isErr {
		t.Fatal("unknown tool should be a tool isError result")
	}
	// Unknown protocol method is a real JSON-RPC error.
	if resp[2].Error == nil || resp[2].Error.Code != -32601 {
		t.Fatalf("unknown method should be JSON-RPC -32601, got %+v", resp[2].Error)
	}
}

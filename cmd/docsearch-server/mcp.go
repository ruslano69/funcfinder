package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ruslano69/funcfinder/internal/embed"
	"github.com/ruslano69/funcfinder/internal/knowledge"
	"github.com/ruslano69/funcfinder/internal/truth"
)

// MCP server for docsearch-server (TZ §10.1): the first-class interface for
// LLM agents. Speaks JSON-RPC 2.0 over stdio (line-delimited), implemented by
// hand to keep the zero-infra, minimal-dep, single-binary property. Tools are
// split strictly along the CQRS line — readonly grounding vs rewrite feedback.
//
// Only the handshake surface MCP clients actually require is implemented:
// initialize, notifications/initialized, tools/list, tools/call.

const mcpProtocolVersion = "2024-11-05"

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"` // echoed verbatim; absent for notifications
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// mcpServer holds the store; tool calls are processed one request per line, so
// the write-log never sees concurrent access from this transport.
type mcpServer struct {
	store *truth.Store
	embc  *embed.Client
	out   *json.Encoder
}

func runMCP(store *truth.Store, embc *embed.Client, args []string) {
	fs := flag.NewFlagSet("mcp", flag.ExitOnError)
	fs.Parse(args)

	m := &mcpServer{store: store, embc: embc, out: json.NewEncoder(os.Stdout)}
	// Progress/diagnostics go to stderr so they never corrupt the JSON-RPC
	// stream on stdout.
	fmt.Fprintln(os.Stderr, "docsearch-server MCP on stdio (protocol "+mcpProtocolVersion+")")

	r := bufio.NewReader(os.Stdin)
	for {
		line, err := r.ReadString('\n')
		if len(strings.TrimSpace(line)) > 0 {
			m.dispatch(line)
		}
		if err == io.EOF {
			return
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "mcp read error: %v\n", err)
			return
		}
	}
}

func (m *mcpServer) reply(id json.RawMessage, result any) {
	m.out.Encode(rpcResponse{JSONRPC: "2.0", ID: id, Result: result})
}

func (m *mcpServer) replyErr(id json.RawMessage, code int, msg string) {
	m.out.Encode(rpcResponse{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: msg}})
}

func (m *mcpServer) dispatch(line string) {
	var req rpcRequest
	if err := json.Unmarshal([]byte(line), &req); err != nil {
		m.replyErr(nil, -32700, "parse error: "+err.Error())
		return
	}

	switch req.Method {
	case "initialize":
		m.reply(req.ID, map[string]any{
			"protocolVersion": mcpProtocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "docsearch-server", "version": "0.1"},
		})
	case "notifications/initialized", "notifications/cancelled":
		// notifications carry no id and expect no response
	case "tools/list":
		m.reply(req.ID, map[string]any{"tools": toolSchemas()})
	case "tools/call":
		m.handleToolCall(req)
	case "ping":
		m.reply(req.ID, map[string]any{})
	default:
		if len(req.ID) > 0 {
			m.replyErr(req.ID, -32601, "method not found: "+req.Method)
		}
	}
}

// toolResult wraps text output in the MCP content envelope.
func toolResult(text string, isErr bool) map[string]any {
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": text}},
		"isError": isErr,
	}
}

func (m *mcpServer) handleToolCall(req rpcRequest) {
	var p struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &p); err != nil {
		m.replyErr(req.ID, -32602, "invalid params: "+err.Error())
		return
	}
	if p.Arguments == nil {
		p.Arguments = map[string]any{}
	}

	text, err := m.callTool(p.Name, p.Arguments)
	if err != nil {
		// Tool-level failures surface as isError results, not JSON-RPC errors,
		// so the agent sees the message and can adapt.
		m.reply(req.ID, toolResult("error: "+err.Error(), true))
		return
	}
	m.reply(req.ID, toolResult(text, false))
}

func (m *mcpServer) callTool(name string, args map[string]any) (string, error) {
	switch name {
	case "search":
		return m.toolSearch(args)
	case "recall":
		// recall is search framed as topic retrieval (TZ FR-8).
		if t, ok := args["topic"]; ok {
			args["query"] = t
		}
		return m.toolSearch(args)
	case "list_releases":
		return m.toolListReleases()
	case "channels":
		return m.toolChannels()
	case "ingest":
		return m.toolIngest(args)
	case "record":
		return m.toolRecord(args)
	case "publish":
		return m.toolPublish(args)
	case "set_channel":
		return m.toolSetChannel(args)
	default:
		return "", fmt.Errorf("unknown tool %q", name)
	}
}

// ---- argument helpers ------------------------------------------------------

func argStr(args map[string]any, key, def string) string {
	if v, ok := args[key].(string); ok && v != "" {
		return v
	}
	return def
}

func argInt(args map[string]any, key string, def int) int {
	if v, ok := args[key].(float64); ok { // JSON numbers decode as float64
		return int(v)
	}
	return def
}

func argFloats(args map[string]any, key string) []float32 {
	raw, ok := args[key].([]any)
	if !ok {
		return nil
	}
	out := make([]float32, 0, len(raw))
	for _, x := range raw {
		if f, ok := x.(float64); ok {
			out = append(out, float32(f))
		}
	}
	return out
}

// ---- readonly tools --------------------------------------------------------

func (m *mcpServer) toolSearch(args map[string]any) (string, error) {
	query := argStr(args, "query", "")
	if query == "" {
		return "", fmt.Errorf("query is required")
	}
	ref := argStr(args, "channel", truth.ChannelStable)
	mode := argStr(args, "mode", "hybrid")
	limit := argInt(args, "limit", 10)
	emb := argFloats(args, "embedding")

	// Embed the query live when no explicit vector was passed and a model is
	// configured — this is what makes semantic/hybrid grounding just work.
	if len(emb) == 0 && m.embc.Enabled() && (mode == "vec" || mode == "hybrid") {
		if v, err := m.embc.Embed(query); err == nil {
			emb = v
		}
	}

	path, err := m.store.Resolve(ref)
	if err != nil {
		return "", err
	}
	db, err := truth.OpenRelease(path)
	if err != nil {
		return "", err
	}
	defer db.Close()

	var res []knowledge.Result
	switch mode {
	case "fts":
		res, err = knowledge.SearchFTS(db, query, limit, true)
	case "vec":
		if len(emb) == 0 {
			return "", fmt.Errorf("embedding required for vec mode")
		}
		res, err = knowledge.SearchVec(db, emb, limit, knowledge.MetricCosine, "")
	case "regex":
		res, err = knowledge.SearchRegex(db, query, limit, "")
	default:
		res, err = knowledge.SearchHybrid(db, query, emb, limit, knowledge.MetricCosine, "", true)
	}
	if err != nil {
		return "", err
	}
	return jsonString(map[string]any{"release": ref, "results": res}), nil
}

func (m *mcpServer) toolListReleases() (string, error) {
	rels, err := m.store.ListReleases()
	if err != nil {
		return "", err
	}
	return jsonString(rels), nil
}

func (m *mcpServer) toolChannels() (string, error) {
	chans, err := m.store.Channels()
	if err != nil {
		return "", err
	}
	return jsonString(chans), nil
}

// ---- rewrite tools ---------------------------------------------------------

func (m *mcpServer) toolIngest(args map[string]any) (string, error) {
	title := argStr(args, "title", "")
	content := argStr(args, "content", "")
	if title == "" || content == "" {
		return "", fmt.Errorf("title and content are required")
	}
	db, err := knowledge.Open(m.store.WriteLogPath())
	if err != nil {
		return "", err
	}
	defer db.Close()
	meta := metaJSON(argStr(args, "author", ""), argStr(args, "role_tags", ""),
		argStr(args, "source_version", ""), "")
	id, err := knowledge.Add(db, title, content, argStr(args, "type", "general"), meta, argFloats(args, "embedding"))
	if err != nil {
		return "", err
	}
	return jsonString(map[string]any{"id": id, "status": "ingested (rides next publish)"}), nil
}

func (m *mcpServer) toolRecord(args map[string]any) (string, error) {
	title := argStr(args, "title", "")
	result := argStr(args, "result", "")
	if title == "" || result == "" {
		return "", fmt.Errorf("title and result are required")
	}
	sourceRef := argStr(args, "source_ref", "")
	author := argStr(args, "author", "")
	db, err := knowledge.Open(m.store.WriteLogPath())
	if err != nil {
		return "", err
	}
	defer db.Close()
	id, err := knowledge.Add(db, title, result, argStr(args, "type", "changelog"),
		metaJSON(author, "", "", sourceRef), nil)
	if err != nil {
		return "", err
	}
	if err := m.store.RecordProvenance(id, author, sourceRef); err != nil {
		return "", err
	}
	return jsonString(map[string]any{"id": id, "status": "recorded (rides next publish)"}), nil
}

func (m *mcpServer) toolPublish(args map[string]any) (string, error) {
	name := argStr(args, "name", "")
	if name == "" {
		return "", fmt.Errorf("name is required")
	}
	rel, err := m.store.Publish(name, argStr(args, "notes", ""))
	if err != nil {
		return "", err
	}
	if ch := argStr(args, "channel", ""); ch != "" {
		if err := m.store.SetChannel(ch, rel.Version); err != nil {
			return "", err
		}
	}
	return jsonString(map[string]any{"version": rel.Version, "path": m.store.ReleasePath(rel.Version)}), nil
}

func (m *mcpServer) toolSetChannel(args map[string]any) (string, error) {
	name := argStr(args, "name", "")
	release := argStr(args, "release", "")
	if name == "" || release == "" {
		return "", fmt.Errorf("name and release are required")
	}
	if err := m.store.SetChannel(name, release); err != nil {
		return "", err
	}
	return jsonString(map[string]any{"channel": name, "release": release}), nil
}

func jsonString(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// toolSchemas returns the MCP tool catalog. Descriptions double as the agent's
// grounding on when to reach for each tool; input schemas are JSON Schema.
func toolSchemas() []map[string]any {
	strProp := func(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }
	intProp := func(desc string) map[string]any { return map[string]any{"type": "integer", "description": desc} }

	tool := func(name, desc string, props map[string]any, required ...string) map[string]any {
		return map[string]any{
			"name":        name,
			"description": desc,
			"inputSchema": map[string]any{
				"type":       "object",
				"properties": props,
				"required":   required,
			},
		}
	}

	return []map[string]any{
		// readonly
		tool("search", "Ground a query against a release of team truth (specs, decisions, changelog, code map). Hybrid FTS+vector by default.",
			map[string]any{
				"query":   strProp("search query"),
				"channel": strProp("stable|testing|unstable or a release version (default stable)"),
				"mode":    strProp("fts|vec|hybrid|regex (default hybrid)"),
				"limit":   intProp("max results (default 10)"),
			}, "query"),
		tool("recall", "Retrieve truth by topic (thin alias over search) for grounding an agent before it acts.",
			map[string]any{
				"topic":   strProp("topic to recall"),
				"channel": strProp("stable|testing|unstable or a release version"),
				"limit":   intProp("max results"),
			}, "topic"),
		tool("list_releases", "List published, immutable releases of truth (newest first).", map[string]any{}),
		tool("channels", "Show channels (stable/testing/unstable) and which release each points at.", map[string]any{}),

		// rewrite
		tool("ingest", "Add a document to the write-log (spec/ТЗ/lib_doc/decision/…). Rides the next publish.",
			map[string]any{
				"title":          strProp("document title"),
				"content":        strProp("document body"),
				"type":           strProp("spec|ТЗ|lib_doc|sprint|changelog|task|decision|general"),
				"role_tags":      strProp("comma-separated roles for context() view filter"),
				"author":         strProp("author (provenance)"),
				"source_version": strProp("source version (provenance)"),
			}, "title", "content"),
		tool("record", "Feedback loop: return a work result (changelog/closed task/decision) into truth with provenance.",
			map[string]any{
				"title":      strProp("short title of the result"),
				"result":     strProp("the result body"),
				"type":       strProp("changelog|task|decision (default changelog)"),
				"source_ref": strProp("link to the source task/spec this answers"),
				"author":     strProp("who produced this"),
			}, "title", "result"),
		tool("publish", "Snapshot the write-log into an immutable named release; optionally point a channel at it.",
			map[string]any{
				"name":    strProp("release version, e.g. 2026.07 or 2026.07.1"),
				"notes":   strProp("release notes"),
				"channel": strProp("optionally point this channel at the new release"),
			}, "name"),
		tool("set_channel", "Repoint a channel (stable|testing) at a published release — the release-day pointer flip.",
			map[string]any{
				"name":    strProp("channel: stable|testing"),
				"release": strProp("release version to point at"),
			}, "name", "release"),
	}
}

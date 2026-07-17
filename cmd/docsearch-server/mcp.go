package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ruslano69/funcfinder/internal/codemap"
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
	case "suggest_terms":
		return m.toolSuggest(args)
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
	case "freeze":
		return m.toolFreeze(args)
	case "prune":
		return m.toolPrune(args)
	case "set_channel":
		return m.toolSetChannel(args)
	case "provenance":
		return m.toolProvenance(args)
	case "context":
		return m.toolContext(args)
	case "diff_releases":
		return m.toolDiffReleases(args)
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

func argInt64(args map[string]any, key string, def int64) int64 {
	if v, ok := args[key].(float64); ok { // JSON numbers decode as float64
		return int64(v)
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

func (m *mcpServer) toolSuggest(args map[string]any) (string, error) {
	prefix := argStr(args, "prefix", "")
	if prefix == "" {
		return "", fmt.Errorf("prefix is required")
	}
	ref := argStr(args, "channel", truth.ChannelStable)
	relativeTo := argStr(args, "relative_to", "")
	includeNumbers, _ := args["include_numbers"].(bool)
	limit := argInt(args, "limit", 20)

	path, err := m.store.Resolve(ref)
	if err != nil {
		return "", err
	}
	db, err := truth.OpenRelease(path)
	if err != nil {
		return "", err
	}
	defer db.Close()

	var terms []knowledge.Term
	if relativeTo != "" {
		terms, err = knowledge.SuggestRelativeTo(db, prefix, relativeTo, limit, includeNumbers)
	} else {
		terms, err = knowledge.Suggest(db, prefix, limit, includeNumbers)
	}
	if err != nil {
		return "", err
	}
	return jsonString(map[string]any{"channel": ref, "prefix": prefix, "relative_to": relativeTo, "terms": terms}), nil
}

func (m *mcpServer) toolContext(args map[string]any) (string, error) {
	role := argStr(args, "role", "")
	if role == "" {
		return "", fmt.Errorf("role is required")
	}
	ref := argStr(args, "channel", truth.ChannelStable)
	limit := argInt(args, "limit", 20)

	path, err := m.store.Resolve(ref)
	if err != nil {
		return "", err
	}
	db, err := truth.OpenRelease(path)
	if err != nil {
		return "", err
	}
	defer db.Close()

	docs, err := knowledge.ByRole(db, role, limit)
	if err != nil {
		return "", err
	}
	return jsonString(map[string]any{"role": role, "channel": ref, "documents": docs}), nil
}

func (m *mcpServer) toolDiffReleases(args map[string]any) (string, error) {
	from := argStr(args, "from", "")
	to := argStr(args, "to", "")
	if from == "" || to == "" {
		return "", fmt.Errorf("from and to are required")
	}
	fromPath, err := m.store.Resolve(from)
	if err != nil {
		return "", err
	}
	toPath, err := m.store.Resolve(to)
	if err != nil {
		return "", err
	}
	fromDB, err := truth.OpenRelease(fromPath)
	if err != nil {
		return "", err
	}
	defer fromDB.Close()
	toDB, err := truth.OpenRelease(toPath)
	if err != nil {
		return "", err
	}
	defer toDB.Close()

	diff, err := knowledge.DiffDocs(fromDB, toDB)
	if err != nil {
		return "", err
	}
	return jsonString(map[string]any{"from": from, "to": to, "diff": diff}), nil
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

func (m *mcpServer) toolProvenance(args map[string]any) (string, error) {
	recordID := argInt64(args, "record_id", 0)
	if recordID == 0 {
		return "", fmt.Errorf("record_id is required")
	}
	entries, err := m.store.Provenance(recordID)
	if err != nil {
		return "", err
	}
	return jsonString(entries), nil
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

	var codeStats codemap.Stats
	if codeDir := argStr(args, "code_dir", ""); codeDir != "" {
		db, err := knowledge.Open(m.store.WriteLogPath())
		if err != nil {
			return "", err
		}
		codeStats, err = codemap.Ingest(db, codeDir)
		db.Close()
		if err != nil {
			return "", err
		}
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
	result := map[string]any{"version": rel.Version, "path": m.store.ReleasePath(rel.Version)}
	if codeStats.Files > 0 {
		result["code_map_files"] = codeStats.Files
		result["code_map_commit"] = codeStats.CommitSHA
		if w := codeStats.Warning(); w != "" {
			result["code_map_warning"] = w
		}
	}
	return jsonString(result), nil
}

func (m *mcpServer) toolFreeze(args map[string]any) (string, error) {
	release := argStr(args, "release", "")
	if release == "" {
		return "", fmt.Errorf("release is required")
	}
	ts, err := m.store.Freeze(release)
	if err != nil {
		return "", err
	}
	return jsonString(map[string]any{"version": release, "frozen_at": ts}), nil
}

func (m *mcpServer) toolPrune(args map[string]any) (string, error) {
	keep := argInt(args, "keep", 0)
	if keep <= 0 {
		return "", fmt.Errorf("keep is required and must be > 0")
	}
	pruned, err := m.store.PruneReleases(keep)
	if err != nil {
		return "", err
	}
	return jsonString(map[string]any{"pruned": pruned, "count": len(pruned)}), nil
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
		tool("suggest_terms", "Discover which terms actually exist in the corpus's FTS index for a prefix, ranked by frequency — look this up BEFORE searching so you use real corpus terms (and see inflected/foreign-language forms) instead of guessing.",
			map[string]any{
				"prefix":      strProp("term prefix, e.g. 'sort' or 'сорт'"),
				"channel":     strProp("stable|testing|unstable or a release version"),
				"relative_to":     strProp("optional: compute IDF relative to a partition (a doc type, e.g. reference_ru) for a mixed-language/source corpus"),
				"include_numbers": map[string]any{"type": "boolean", "description": "include pure-digit tokens (off by default — they are useless search keys)"},
				"limit":           intProp("max terms (default 20)"),
			}, "prefix"),
		tool("context", "Role-scoped view over the same corpus (TZ FR-9): documents tagged for a role via ingest's role_tags, newest first — one database, different lenses.",
			map[string]any{
				"role":    strProp("role tag to filter by, e.g. backend"),
				"channel": strProp("stable|testing|unstable or a release version"),
				"limit":   intProp("max documents (default 20)"),
			}, "role"),
		tool("diff_releases", "Compare two releases (or channels) and report added/removed/changed documents by id (TZ FR-18) — 'what changed going from A to B'.",
			map[string]any{
				"from": strProp("release version or channel to diff from"),
				"to":   strProp("release version or channel to diff to"),
			}, "from", "to"),
		tool("list_releases", "List published, immutable releases of truth (newest first).", map[string]any{}),
		tool("channels", "Show channels (stable/testing/unstable) and which release each points at.", map[string]any{}),
		tool("provenance", "Look up who produced a recorded document, when, and against which source task/spec.",
			map[string]any{
				"record_id": intProp("the recorded document's id (returned by record/ingest)"),
			}, "record_id"),

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
		tool("publish", "Snapshot the write-log into an immutable named release; optionally point a channel at it and bake in a funcfinder structural code map.",
			map[string]any{
				"name":     strProp("release version, e.g. 2026.07 or 2026.07.1"),
				"notes":    strProp("release notes"),
				"channel":  strProp("optionally point this channel at the new release"),
				"code_dir": strProp("optional: path to a source tree to bake in as a structural code map (functions/types per file, tagged with its git commit) — TZ FR-22. Replaces any code map from a previous publish."),
			}, "name"),
		tool("freeze", "Open the stabilization window on a published release: a signal that further fixes should land in unstable rather than repointing this release.",
			map[string]any{
				"release": strProp("released version to freeze, e.g. 2026.07"),
			}, "release"),
		tool("prune", "Retention policy (TZ FR-15): keep the newest N releases and delete the rest (both the release file and its control-DB row). A release currently pinned by a channel is never pruned, regardless of age.",
			map[string]any{
				"keep": intProp("number of newest releases to retain"),
			}, "keep"),
		tool("set_channel", "Repoint a channel (stable|testing) at a published release — the release-day pointer flip.",
			map[string]any{
				"name":    strProp("channel: stable|testing"),
				"release": strProp("release version to point at"),
			}, "name", "release"),
	}
}

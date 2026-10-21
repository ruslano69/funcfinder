module cgraph_core;

import ffd_calls;
import ffd_langs;
import ffd_config : rootConfig;
import ffd_model;
import ffd_tokens;
import std.algorithm : canFind, sort;
import std.array : appender;
import std.json;
import std.path : baseName;
import std.string : split, strip, startsWith, endsWith, join, count;

struct Edge { string caller, callee; int line; }
struct FileGraph { string path; Edge[] calls; }
struct Graph { FileGraph[] files; size_t totalFunctions; }
struct Pending { string caller; CallSite site; }
struct ParsedGraph {
    string path, error;
    Sym[] functions;
    Pending[] calls;
    string[string] aliases;
}

private __gshared string[][Lang.max+1] excluded;

void prepareGraph() {
    auto cfg = rootConfig()["languages"];
    foreach (i; 0 .. Lang.max+1) {
        excluded[i] = null;
        excluded[i] = cfg[langKey(cast(Lang)i)]["exclude_words"].strings.dup;
    }
}

private struct Region { int first = -1, last = -1; }

// Native bounds own scope recognition. Tokens refine same-line neighbors and
// brace bodies; they do not rediscover declarations or enter anonymous callers.
private Region[] regions(Tokens tokens, Sym[] functions, int[] declarations, string lang) {
    Region[] result; result.length = functions.length;
    foreach (i,f; functions) {
        int first = declarations[i];
        if (first < 0) continue;
        int last = first;
        while (last+1 < tokens.items.length && tokens.items[last+1].line <= f.end) ++last;
        // The first top-level body on the native last line gives a token end.
        // Parameter/default expressions are balanced and skipped as a unit.
        for (int j=first+1; j<=last; ++j) {
            auto text = tokens.items[j].text;
            auto pair = tokens.pairs[j];
            if ((text == "(" || text == "[") && pair > j) { j = pair; continue; }
            if (lang == "py" && text == ":") { first = j+1; break; }
            if (text == "<" && lang != "py") {
                int depth = 1, k = j+1;
                for (; k<=last && depth; ++k) {
                    if (tokens.items[k].text == "<") ++depth;
                    if (tokens.items[k].text == ">") --depth;
                }
                if (!depth) { j = k-1; continue; }
            }
            if (text == "{" && pair > j) {
                int next = pair+1;
                while (next < tokens.items.length && tokens.items[next].text == "\n") ++next;
                bool typeContinues = next < tokens.items.length && ["{","|","&",">"].canFind(tokens.items[next].text);
                if (lang == "cpp" && next < tokens.items.length && tokens.items[next].text == ",") typeContinues = true;
                if (tokens.items[pair].line == f.end && !typeContinues) { last = pair; break; }
                j = pair;
            }
            if (text == ";" && tokens.items[j].line == f.end) { last = j; break; }
        }
        result[i] = Region(first,last);
    }
    // A later sibling declaration may share the native start/end line (notably
    // arrows without braces). A contained body remains a separate inner scope.
    size_t[] ordered;
    foreach (i,r; result) if (r.first >= 0) ordered ~= i;
    ordered.sort!((a,b) => result[a].first < result[b].first);
    foreach (n; 1..ordered.length) {
        auto i = ordered[n-1], j = ordered[n];
        auto f = functions[i], g = functions[j];
        if (result[j].first > result[i].first && result[j].first <= result[i].last &&
            g.start == f.end && f.start == f.end && g.start == g.end) {
            // Body-brace ends distinguish a nested function from a neighbor.
            if (tokens.items[result[i].last].text != "}") result[i].last = result[j].first-1;
        }
    }
    return result;
}

private string callerAt(CallSite site, Tokens tokens, Sym[] functions, Region[] scope_) {
    int best = -1;
    foreach (i,f; functions) {
        if (site.line < f.start || site.line > f.end) continue;
        auto r = scope_[i];
        if (r.first >= 0 && (site.token < r.first || site.token > r.last)) continue;
        if (best < 0 || f.start > functions[best].start || f.start == functions[best].start &&
            (f.end < functions[best].end || f.end == functions[best].end && r.first > scope_[best].first)) best = cast(int)i;
    }
    return best < 0 ? "" : functions[best].name;
}

// Import locations come from the shared Go lexer. Recover only opaque path
// literals on those active lines; comments and unrelated strings are not imports.
private string[string] goAliases(string source, Tokens tokens) {
    string[string] aliases;
    auto lines = source.split("\n");
    foreach (i,t; tokens.items) {
        if (t.text != "import" || t.literal) continue;
        int end = t.line;
        if (i+1 < tokens.items.length && tokens.items[i+1].text == "(" && tokens.pairs[i+1] > i+1)
            end = tokens.items[tokens.pairs[i+1]].line;
        foreach (line; t.line .. end+1) {
            if (line < 1 || line > lines.length || tokens.metadataLines.length && !tokens.metadataLines[line-1]) continue;
            auto raw = lines[line-1].strip;
            if (line == t.line && raw.startsWith("import")) raw = raw[6..$].strip;
            if (raw.startsWith("(")) raw = raw[1..$].strip;
            if (!raw.length || raw.startsWith("//") || raw.startsWith("/*")) continue;
            size_t start;
            while (start < raw.length && raw[start] != '"' && raw[start] != '`') ++start;
            if (start == raw.length) continue;
            auto close = start+1;
            while (close < raw.length && raw[close] != raw[start]) ++close;
            if (close == raw.length) continue;
            auto pkg = raw[start+1..close];
            auto alias_ = raw[0..start].strip;
            auto base = baseName(pkg);
            if (!alias_.length) alias_ = base;
            if (alias_ == "." || alias_ == "_") continue;
            aliases[alias_.idup] = base.idup;
        }
    }
    return aliases;
}

ParsedGraph parseGraph(string path, Lang lang) {
    ParsedGraph result; result.path = path;
    try {
        auto source = readSource(path);
        ScanNeeds needs; needs.signatureMetadata = true; needs.structTypes = false;
        auto found = analyzeSource(path,lang,source,needs);
        if (found.error.length) throw new Exception(found.error);
        result.functions = found.functions;
        auto tokens = tokensFor(source,langKey(lang));
        auto scan = scanCalls(tokens,found.functions,found.signatures,langKey(lang),excluded[lang]);
        auto scope_ = regions(tokens,found.functions,scan.declarationTokens,langKey(lang));
        if (lang == Lang.go) result.aliases = goAliases(source,tokens);
        foreach (site; scan.sites) {
            auto caller = callerAt(site,tokens,found.functions,scope_);
            if (caller.length) result.calls ~= Pending(caller,site);
        }
    } catch (Exception e) { result.error = e.msg.idup; }
    return result;
}

Graph buildGraph(ParsedGraph[] parsed, bool singleFile = false) {
    bool[string] known;
    foreach (p; parsed) foreach (f; p.functions) known[f.name] = true;
    Graph result; result.totalFunctions = known.length;
    foreach (p; parsed) {
        if (p.error.length) throw new Exception(p.path ~ ": " ~ p.error);
        FileGraph file; file.path = p.path;
        bool[string] seen;
        foreach (c; p.calls) {
            string callee;
            if (c.site.receiver.length) {
                auto receiver = c.site.receiver;
                if (auto alias_ = receiver in p.aliases) receiver = *alias_;
                callee = receiver ~ "." ~ c.site.name;
            } else if (c.site.name in known) callee = c.site.name;
            else continue;
            auto key = c.caller ~ "\0" ~ callee;
            if (key in seen) continue;
            seen[key] = true;
            file.calls ~= Edge(c.caller,callee,c.site.line);
        }
        file.calls.sort!((a,b) => a.caller != b.caller ? a.caller < b.caller : a.callee < b.callee);
        if (singleFile || file.calls.length) result.files ~= file;
    }
    result.files.sort!((a,b) => a.path < b.path);
    return result;
}

size_t edgeCount(Graph g) {
    size_t count_; foreach (f; g.files) count_ += f.calls.length; return count_;
}

JSONValue graphJSON(Graph g) {
    JSONValue[] files;
    foreach (f; g.files) {
        JSONValue[] calls;
        foreach (e; f.calls) calls ~= JSONValue(["caller":JSONValue(e.caller),"callee":JSONValue(e.callee),"line":JSONValue(e.line)]);
        files ~= JSONValue(["path":JSONValue(f.path),"calls":calls.length ? JSONValue(calls) : JSONValue(null)]);
    }
    return JSONValue(["files":files.length ? JSONValue(files) : JSONValue(null),
        "total_calls":JSONValue(edgeCount(g)),"total_functions":JSONValue(g.totalFunctions)]);
}

string[][string] reverseGraph(Graph g) {
    string[][string] result; bool[string] seen;
    foreach (f; g.files) foreach (e; f.calls) {
        auto key = e.callee ~ "\0" ~ e.caller;
        if (key in seen) continue;
        seen[key] = true; result[e.callee] ~= e.caller;
    }
    foreach (ref callers; result) callers.sort;
    return result;
}

string[] reverseCallers(string[][string] reversed, string name) {
    bool[string] seen;
    foreach (callee,callers; reversed) {
        if (callee != name && (name.canFind(".") || !callee.endsWith("." ~ name))) continue;
        foreach (caller; callers) seen[caller] = true;
    }
    auto result = seen.keys; result.sort; return result;
}

// Shortest-path reachability makes --depth independent of DFS/file order.
// Retain the original file and call-site line in filtered JSON.
Graph filterGraph(Graph g, string root, int depth) {
    string[][string] adjacency;
    foreach (f; g.files) foreach (e; f.calls) adjacency[e.caller] ~= e.callee;
    int[string] distance = [root:0]; string[] queue = [root];
    for (size_t i; i<queue.length; ++i) {
        auto name = queue[i]; auto d = distance[name];
        if (depth > 0 && d >= depth) continue;
        foreach (callee; adjacency.get(name,null)) if (callee !in distance) {
            distance[callee] = d+1; queue ~= callee;
        }
    }
    Graph result; result.totalFunctions = g.totalFunctions;
    foreach (f; g.files) {
        FileGraph file; file.path = f.path;
        foreach (e; f.calls) if (auto d = e.caller in distance) {
            if (depth == 0 || *d < depth) file.calls ~= e;
        }
        if (file.calls.length) result.files ~= file;
    }
    return result;
}

string graphText(Graph g) {
    auto out_ = appender!string();
    foreach (f; g.files) {
        out_.put(f.path); out_.put("\n");
        foreach (e; f.calls) { out_.put("  "); out_.put(e.caller); out_.put(" → "); out_.put(e.callee); out_.put("\n"); }
    }
    return out_.data;
}

string treeText(Graph g, string root) {
    string[][string] callees;
    foreach (f; g.files) foreach (e; f.calls) callees[e.caller] ~= e.callee;
    struct Frame { string name; size_t depth; bool exit_; }
    Frame[] stack = [Frame(root,0,false)]; bool[string] onPath, shown;
    auto out_ = appender!string();
    while (stack.length) {
        auto frame = stack[$-1]; stack.length--;
        if (frame.exit_) { onPath.remove(frame.name); continue; }
        foreach (_; 0..frame.depth) out_.put("  ");
        out_.put(frame.name);
        if (frame.name in onPath) { out_.put(" (↑ recursive)\n"); continue; }
        if (frame.name in shown) { out_.put(" (see above)\n"); continue; }
        out_.put("\n"); shown[frame.name] = onPath[frame.name] = true;
        stack ~= Frame(frame.name,frame.depth,true);
        auto children = callees.get(frame.name,null);
        for (size_t i=children.length; i>0; --i) stack ~= Frame(children[i-1],frame.depth+1,false);
    }
    return out_.data;
}

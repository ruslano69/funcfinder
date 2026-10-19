module ffd_file;

import ffd_fields;
import ffd_langs;
import ffd_model;
import ffd_run : applyMode;
import ffd_views;
import std.algorithm : sort, SwapStrategy;
import std.array : appender, join;
import std.conv : to, ConvException;
import std.file : read, FileException;
import std.stdio : stderr, stdout;
import std.string : split, strip, indexOf, replace;
import std.traits : EnumMembers;

// The single-file run of `funcfinder --inp F [--source L]`: the same validations, messages and exit codes as the Go tool's
// handleFileMode (cmd/funcfinder/main.go), over the D finders, with `--lines` (a range of lines, alone or as the window the
// functions are searched in) and `--raw`.

struct FileRequest {
    string inp, source, funcStr, typeStr, lines;
    bool structMode, allMode, mapMode, treeMode, treeFull, json, extract, raw;
}

private int fail(int code, string text) {
    stderr.writeln("Error: ", text);
    return code;
}

private string[] supportedLanguages() {
    string[] keys;
    foreach (l; EnumMembers!Lang) keys ~= langKey(l);
    keys.sort();
    return keys;
}

/// Splits a comma-separated list the way internal.ParseFuncNames does: trimmed, empty names dropped.
string[] parseNames(string s) {
    string[] r;
    foreach (p; s.split(",")) {
        auto t = strip(p);
        if (t.length) r ~= t;
    }
    return r;
}

private const(Sym)[] namedOnly(const(Sym)[] syms, string[] names) {
    const(Sym)[] r;
    foreach (ref s; syms)
        foreach (n; names)
            if (s.name == n) { r ~= s; break; }
    return r;
}

private void println(string text) {
    stdout.rawWrite(text);
    stdout.rawWrite("\n");
}

// --- line ranges ----------------------------------------------------------------------------------------------------------

/// A range of lines, 1-based and inclusive; end -1 means to the end of the file.
struct LineRange {
    int start, end;
}

private bool parseInt(string s, out int value) {
    if (s.length == 0) return false;
    try {
        value = s.to!int;
        return true;
    } catch (ConvException) {
        return false;
    }
}

/// internal.ParseLineRange: "100", "100:150", ":50", "100:".
bool parseLineRange(string text, out LineRange range, out string error) {
    if (text.length == 0) { error = "empty line range"; return false; }
    immutable colon = text.indexOf(':');
    if (colon < 0) {
        int line;
        if (!parseInt(text, line)) { error = "invalid line number: " ~ text; return false; }
        if (line < 1) { error = "line number must be >= 1, got " ~ line.to!string; return false; }
        range = LineRange(line, line);
        return true;
    }
    auto head = text[0 .. colon], tail = text[colon + 1 .. $];
    int start = 1, end = -1;
    if (head.length) {
        if (!parseInt(head, start)) { error = "invalid start line: " ~ head; return false; }
        if (start < 1) { error = "start line must be >= 1, got " ~ start.to!string; return false; }
    }
    if (tail.length) {
        if (!parseInt(tail, end)) { error = "invalid end line: " ~ tail; return false; }
        if (end < 1) { error = "end line must be >= 1, got " ~ end.to!string; return false; }
    }
    if (end != -1 && start > end) {
        error = "start line (" ~ start.to!string ~ ") must be <= end line (" ~ end.to!string ~ ")";
        return false;
    }
    range = LineRange(start, end);
    return true;
}

/// The lines of a file as the Go reader gives them (bytes as they are: a CR before the LF dropped, no empty last line).
private string[] rawLines(string path, out string error) {
    string[] lines;
    try {
        auto data = cast(const(char)[]) read(path);
        size_t from;
        foreach (i; 0 .. data.length)
            if (data[i] == '\n') {
                auto end = i > from && data[i - 1] == '\r' ? i - 1 : i;
                lines ~= data[from .. end].idup;
                from = i + 1;
            }
        if (from < data.length) {
            auto tail = data[from .. $];
            if (tail.length && tail[$ - 1] == '\r') tail = tail[0 .. $ - 1];
            lines ~= tail.idup;
        }
    } catch (FileException) {
        version (Windows) enum reason = "The system cannot find the file specified.";
        else enum reason = "no such file or directory";
        error = "open " ~ path ~ ": " ~ reason;
    }
    return lines;
}

/// internal.ReadFileLines: the lines of the range; false, with Go's message, when there are none.
private bool selectLines(const(string)[] all, LineRange range, out string[] selected, out string error) {
    foreach (i, l; all) {
        immutable n = cast(int) i + 1;
        if (n >= range.start && (range.end == -1 || n <= range.end)) selected ~= l.idup;
    }
    if (selected.length == 0) {
        immutable end = range.end == -1 ? cast(int) all.length : range.end;
        error = "no lines found in range " ~ range.start.to!string ~ ":" ~ end.to!string ~ " (file has " ~ all.length.to!string ~ " lines)";
        return false;
    }
    return true;
}

private string jsonLine(string s) {
    // OutputJSONLines escapes the backslash, the quote, LF, CR and tab, and nothing else.
    return s.replace("\\", "\\\\").replace("\"", "\\\"").replace("\n", "\\n").replace("\r", "\\r").replace("\t", "\\t");
}

private int runLines(ref const FileRequest q) {
    LineRange range;
    string error;
    if (!parseLineRange(q.lines, range, error)) return fail(1, "parsing line range: " ~ error);
    auto all = rawLines(q.inp, error);
    if (error.length) return fail(1, "reading lines: " ~ error);
    string[] sel;
    if (!selectLines(all, range, sel, error)) return fail(1, "reading lines: " ~ error);
    auto o = appender!string();
    if (q.json) {
        immutable end = range.start + cast(int) sel.length - 1;
        o ~= "{\n  \"range\": {\"start\": " ~ range.start.to!string ~ ", \"end\": " ~ end.to!string ~ "},\n";
        o ~= "  \"line_count\": " ~ sel.length.to!string ~ ",\n  \"lines\": [\n";
        foreach (i, l; sel)
            o ~= "    {\"line\": " ~ (range.start + cast(int) i).to!string ~ ", \"content\": \"" ~ jsonLine(l) ~ "\"}" ~ (i + 1 < sel.length ? "," : "") ~ "\n";
        o ~= "  ]\n}\n";
    } else
        foreach (i, l; sel) o ~= (range.start + cast(int) i).to!string ~ ": " ~ l ~ "\n";
    stdout.rawWrite(o.data);
    return 0;
}

// --- Python: the range is widened to whole scopes ---------------------------------------------------------------------------

private struct Scope {
    string name, kind;
    int start, end;
}

private struct Adjustment {
    int originalStart, originalEnd, fixedStart, fixedEnd;
    string reason, scopeName, scopeKind;
}

private struct Adjusted {
    int start, end;
    Adjustment[] adjustments;
}

/// The scopes of the Go tool's first pass (a function or a class with the decorators above it), from the finder's results.
private Scope[] pythonScopes(ref const FileResult r) {
    Scope[] scopes;
    foreach (ref f; r.functions) scopes ~= Scope(f.name, "function", f.start, f.end);
    foreach (ref c; r.classes) scopes ~= Scope(c.name, "class", c.start, c.end);
    scopes.sort!((a, b) => a.start < b.start, SwapStrategy.stable);
    return scopes;
}

/// internal.ValidateAndFixLineRange, line for line.
private Adjusted fixLineRange(const(Scope)[] scopes, int requestedStart, int requestedEnd) {
    Adjusted a;
    a.start = requestedStart;
    a.end = requestedEnd;
    int maxLine;
    foreach (ref s; scopes) if (s.end > maxLine) maxLine = s.end;
    // line -> the scope that was written last for it
    auto owner = new int[](maxLine + 2);
    owner[] = -1;
    foreach (i, ref s; scopes)
        foreach (line; (s.start < 0 ? 0 : s.start) .. s.end + 1) owner[line] = cast(int) i;
    int scopeAt(int line) { return line >= 0 && line <= maxLine ? owner[line] : -1; }

    Adjustment adj(int fs, int fe, string reason, ref const Scope s) {
        return Adjustment(requestedStart, requestedEnd, fs, fe, reason, s.name, s.kind);
    }
    immutable startScope = scopeAt(requestedStart);
    if (startScope < 0) {
        // The start is in global scope: expand to the next function or class.
        int next = -1;
        foreach (i, ref s; scopes)
            if (s.start > requestedStart && (next < 0 || s.start < scopes[next].start)) next = cast(int) i;
        if (next >= 0) {
            auto s = scopes[next];
            a.adjustments ~= adj(s.start, s.end, "Requested range is in global scope, expanded to include " ~ s.kind ~ " '" ~ s.name ~ "'", s);
            a.start = s.start;
            a.end = s.end;
            return a;
        }
        immutable endScope = scopeAt(requestedEnd);
        if (endScope >= 0) {
            auto s = scopes[endScope];
            a.adjustments ~= adj(requestedStart, s.end, "End line is inside " ~ s.kind ~ " '" ~ s.name ~ "', expanded to include full scope", s);
            a.end = s.end;
        }
        return a;
    }
    auto ss = scopes[startScope];
    if (ss.kind == "function") {
        if (requestedStart != ss.start) {
            a.adjustments ~= adj(ss.start, ss.end, "Requested range is inside function body, expanded to full function", ss);
            a.start = ss.start;
            a.end = ss.end;
        } else if (requestedEnd > ss.end) {
            a.adjustments ~= adj(a.start, ss.end, "Requested end exceeds function body, clipped to function end", ss);
            a.end = ss.end;
        }
    }
    immutable endScope = scopeAt(requestedEnd);
    if (endScope >= 0 && endScope != startScope) {
        if (scopes[endScope].kind == "function" && ss.kind == "class") {
            a.adjustments ~= adj(a.start, ss.end, "End line is inside a method of this class, clipped to class end", ss);
            a.end = ss.end;
        }
    }
    return a;
}

private string rangeText(int start, int end) {
    return end == -1 ? start.to!string ~ ":EOF" : start.to!string ~ ":" ~ end.to!string;
}

private string spaces(size_t n) {
    auto b = new char[n];
    b[] = ' ';
    return cast(string) b;
}

// padToLen of the Go tool: the spaces that follow a range up to the box's edge (a long range gets the full width).
private string padTo(string s, size_t length) {
    return spaces(s.length >= length ? length : length - s.length);
}

private string adjustmentReport(const(Adjustment)[] adjustments, int originalStart, int originalEnd, int fixedStart, int fixedEnd) {
    if (adjustments.length == 0) return "";
    auto o = appender!string();
    enum rule = "+------------------------------------------------------------------+\n";
    o ~= "\n";
    o ~= rule;
    o ~= "|           PYTHON LINES RANGE ADJUSTMENT REPORT                  |\n";
    o ~= rule;
    auto req = rangeText(originalStart, originalEnd), fix = rangeText(fixedStart, fixedEnd);
    o ~= "| Requested range: " ~ req ~ padTo(req, 55) ~ "|\n";
    o ~= "| Adjusted range:  " ~ fix ~ padTo(fix, 55) ~ "|\n";
    o ~= rule;
    foreach (i, ref a; adjustments) {
        o ~= "| Adjustment #" ~ (i + 1).to!string ~ ":\n";
        o ~= "|   Scope: " ~ a.scopeKind ~ " '" ~ a.scopeName ~ "'\n";
        o ~= "|   Reason: " ~ a.reason ~ "\n";
    }
    o ~= rule;
    return o.data;
}

// --- the run ---------------------------------------------------------------------------------------------------------------------

/// The types as editable copies, with their fields read.
private Sym[] withFields(const(Sym)[] types, const(string)[] lines, Lang lang) {
    Sym[] list;
    foreach (ref t; types) list ~= t;
    auto cleaned = cleanLines(lines, lang);
    if (lang == Lang.py) {
        foreach (ref t; list) t.fields = pythonFields(t.start, t.end, cleaned);
    } else {
        auto spec = fieldSpecFor(lang);
        foreach (ref t; list) t.fields = fieldsOfType(spec, t.start, t.end, cleaned.lines);
    }
    return list;
}

/// Runs one file through the request and prints the result; returns the exit code. Languages must be prepared by the caller.
int runFile(ref const FileRequest q) {
    if (q.source.length == 0 && q.lines.length) return runLines(q);
    if (q.source.length == 0) return fail(1, "--source parameter is required (or use --lines alone for plain text extraction)");
    immutable mapLike = q.mapMode || q.treeMode || q.treeFull;
    if (q.structMode && q.allMode) return fail(1, "--struct and --all are mutually exclusive");
    string workMode = q.structMode ? "structs" : q.allMode ? "all" : "functions";
    if (workMode == "functions") {
        if (q.funcStr.length == 0 && !mapLike) return fail(1, "either --func, --map, or --tree must be specified");
        if (q.funcStr.length && mapLike) return fail(1, "--func is mutually exclusive with --map and --tree");
        if (q.typeStr.length) return fail(1, "--type can only be used with --struct or --all");
    } else if (workMode == "structs") {
        if (q.typeStr.length == 0 && !mapLike && !q.extract) return fail(1, "either --type, --map, --tree, or --extract must be specified with --struct");
        if (q.typeStr.length && mapLike) return fail(1, "--type is mutually exclusive with --map and --tree");
        if (q.funcStr.length) return fail(1, "--func cannot be used with --struct");
    } else {
        if (!mapLike && !q.json) return fail(1, "--all requires --map, --tree, or --json output mode");
        if (q.funcStr.length || q.typeStr.length) return fail(1, "--func and --type cannot be used with --all (use --map instead)");
    }
    if (q.treeMode && q.treeFull) return fail(1, "--tree and --tree-full are mutually exclusive");
    Lang lang;
    if (!languageFromKey(q.source, lang))
        return fail(1, "unsupported language: " ~ q.source ~ "\nSupported languages: " ~ supportedLanguages().join(", "));

    prepareLanguages([lang]);
    wantKinds = workMode != "functions";
    rawStrings = q.raw;
    if (q.raw && !rawApplies(lang)) stderr.writeln("INFO: --raw has no effect on ", langKey(lang), " in ffd");
    auto r = analyzeFile(q.inp, lang);
    if (r.error.length) return fail(1, r.error);
    string[] lines;
    {
        auto raw = splitLines(readSource(q.inp));
        lines.length = raw.length;
        foreach (i, l; raw) lines[i] = l.idup;
    }

    if (q.lines.length) {
        if (workMode == "structs") return fail(1, "--lines is not yet supported with --struct mode");
        if (workMode == "all") return fail(1, "--lines is not yet supported with --all mode");
        LineRange range;
        string error;
        if (!parseLineRange(q.lines, range, error)) return fail(1, "parsing line range: " ~ error);
        if (lang == Lang.py) {
            auto fixed = fixLineRange(pythonScopes(r), range.start, range.end);
            if (fixed.adjustments.length) println(adjustmentReport(fixed.adjustments, range.start, range.end, fixed.start, fixed.end));
            range.start = fixed.start;
            range.end = fixed.end;
        }
        string[] sel;
        if (!selectLines(lines, range, sel, error)) return fail(1, "reading lines: " ~ error);
        if (range.end == -1) stderr.writeln("INFO: Using --lines filter (", range.start, ":EOF). Functions outside this range will be excluded.");
        else stderr.writeln("INFO: Using --lines filter (", range.start, ":", range.end, "). Functions outside this range will be excluded.");
        if (lang == Lang.py) {
            // The Python finder reads the whole file; the functions inside the range are kept.
            Sym[] kept;
            foreach (ref f; r.functions)
                if (f.start >= range.start && (range.end == -1 || f.end <= range.end)) kept ~= f;
            r.functions = kept;
        } else {
            // The other finders read the window as a file of its own, its lines numbered from the range's start.
            FileResult window;
            window.path = q.inp;
            window.lang = langKey(lang);
            // A PHP window is code, as the Go tool reads it: it starts after an opening tag of its own, a line of its own.
            immutable prefix = lang == Lang.php ? "<?php\n" : "";
            try {
                analyzeInto(window, prefix ~ sel.join("\n"), lang);
            } catch (Exception e) {
                return fail(1, e.msg);
            }
            immutable shift = range.start - 1 - (prefix.length ? 1 : 0);
            foreach (ref f; window.functions) { f.start += shift; f.end += shift; }
            foreach (ref c; window.classes) { c.start += shift; c.end += shift; }
            r.functions = window.functions;
            r.classes = window.classes;
        }
    }

    // The types are the struct finder's: for JavaScript and TypeScript its own list, for the others the finders' classes.
    const(Sym)[] types = r.hasStructTypes && !q.lines.length ? r.structTypes : r.classes;
    auto view = FileView(q.inp, r.functions, r.classes, lines);

    switch (workMode) {
        case "functions": {
            auto found = q.funcStr.length ? namedOnly(r.functions, parseNames(q.funcStr)) : r.functions;
            if (found.length == 0) return fail(2, mapLike ? "No functions found in file" : "Specified functions not found");
            Sym[] fns;
            foreach (ref f; found) fns ~= f;
            if (lang == Lang.py && q.json && !q.extract) {
                auto cleaned = cleanLines(lines, Lang.py);
                foreach (ref f; fns) f.decorators = pythonDecorators(lines, f.start, cleaned);
            }
            view.functions = fns;
            string out_;
            if (q.extract) out_ = formatExtract(view, fns);
            else if (q.json) out_ = formatFunctionsJson(fns);
            else if (q.treeMode) out_ = formatFileTree(view, false);
            else if (q.treeFull) out_ = formatFileTree(view, true);
            else out_ = formatGrepStyle(fns);
            println(out_);
            return 0;
        }
        case "structs": {
            auto found = q.typeStr.length ? namedOnly(types, parseNames(q.typeStr)) : types;
            if (found.length == 0) return fail(2, mapLike ? "No types found in file" : "Specified types not found");
            auto typed = withFields(found, lines, lang);
            string out_;
            if (q.extract) out_ = formatTypeExtract(view, typed);
            else if (q.json) out_ = formatTypeJson(q.inp, typed);
            else if (q.treeMode || q.treeFull) out_ = formatTypeTree(typed);
            else out_ = formatTypeMap(typed);
            println(out_);
            return 0;
        }
        case "all": {
            if (r.functions.length == 0 && types.length == 0) return fail(2, "No functions or types found in file");
            auto typed = withFields(types, lines, lang);
            if (q.json) {
                stdout.rawWrite(formatAllJson(q.inp, r.functions, typed));
                return 0;
            }
            if (r.functions.length) {
                println("=== FUNCTIONS ===");
                println(q.extract ? formatExtract(view, r.functions) : q.treeMode ? formatFileTree(view, false) : q.treeFull ? formatFileTree(view, true) : formatGrepStyle(r.functions));
            }
            if (typed.length) {
                if (r.functions.length) println("");
                println("=== TYPES ===");
                println(q.extract ? formatTypeExtract(view, typed) : q.treeMode || q.treeFull ? formatTypeTree(typed) : formatTypeMap(typed));
            }
            return 0;
        }
        default:
            assert(0);
    }
}

unittest {
    LineRange r;
    string e;
    assert(parseLineRange("5", r, e) && r.start == 5 && r.end == 5);
    assert(parseLineRange(":9", r, e) && r.start == 1 && r.end == 9);
    assert(parseLineRange("3:", r, e) && r.start == 3 && r.end == -1);
    assert(parseLineRange("3:7", r, e) && r.end == 7);
    assert(!parseLineRange("7:3", r, e) && e == "start line (7) must be <= end line (3)");
    assert(!parseLineRange("0", r, e) && e == "line number must be >= 1, got 0");
    assert(!parseLineRange("x:2", r, e) && e == "invalid start line: x");
    assert(!parseLineRange("", r, e) && e == "empty line range");
}

unittest {
    // A range inside a function body widens to the whole function; one in global scope to the next scope.
    Scope[] scopes = [Scope("C", "class", 1, 10), Scope("m", "function", 3, 6), Scope("g", "function", 12, 15)];
    auto a = fixLineRange(scopes, 4, 5);
    assert(a.start == 3 && a.end == 6 && a.adjustments.length == 1);
    auto b = fixLineRange(scopes, 11, 11);
    assert(b.start == 12 && b.end == 15);
    auto c = fixLineRange(scopes, 12, 20);
    assert(c.start == 12 && c.end == 15);
    auto d = fixLineRange(scopes, 1, 4);
    assert(d.start == 1 && d.end == 10);
    assert(adjustmentReport(a.adjustments, 4, 5, 3, 6).length > 200);
}

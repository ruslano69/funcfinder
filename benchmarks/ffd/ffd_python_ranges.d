module ffd_python_ranges;

import ffd_model;
import python_port : tokenizePython;
import std.algorithm : sort, canFind;
import std.array : appender;
import std.conv : to;
import std.format : format;
import std.string : split, replace, join, strip;

struct PythonRange { int start, end; string report; }
private struct Scope { string name, kind; int start, end, declaration; }
private struct Adjustment { string name, kind, reason; }

private int goIndent(string line) {
    int indent;
    foreach (c; line) { if (c == ' ') ++indent; else if (c == '\t') indent += 4; else break; }
    return indent;
}

private string rangeText(int start, int end) { return start.to!string ~ ":" ~ (end == -1 ? "EOF" : end.to!string); }

string[] pythonDecoratorTexts(string[] lines, Sym fn) {
    if (fn.start < 1 || fn.end > lines.length || !lines[fn.start-1].strip.length || lines[fn.start-1].strip[0] != '@') return null;
    auto raw = lines[fn.start-1..fn.end];
    auto tokens = tokenizePython(raw.join("\n").replace("\r\n","\n"));
    auto t = tokens.items;
    int[] starts; int first;
    foreach (i, token; t) if (token.text == "\n") { if (first < i) starts ~= first; first = cast(int)i+1; }
    if (first < t.length) starts ~= first;
    string[] result;
    foreach (i, at; starts) {
        if (t[at].text != "@") break;
        auto end = i+1 < starts.length ? t[starts[i+1]].line-1 : raw.length;
        result ~= raw[t[at].line-1..end].join("\n").strip.idup;
    }
    return result;
}

private string report(Adjustment[] changes, int start, int end, int fixedStart, int fixedEnd) {
    if (!changes.length) return "";
    auto out_ = appender!(char[])();
    enum border = "+------------------------------------------------------------------+\n";
    out_ ~= "\n" ~ border ~ "|           PYTHON LINES RANGE ADJUSTMENT REPORT                  |\n" ~ border;
    foreach (i, value; [rangeText(start,end), rangeText(fixedStart,fixedEnd)]) {
        out_ ~= (i ? "| Adjusted range:  " : "| Requested range: ") ~ value;
        auto padding = value.length < 55 ? 55-value.length : 55;
        foreach (_; 0..padding) out_ ~= ' ';
        out_ ~= "|\n";
    }
    out_ ~= border;
    foreach (i, change; changes) out_ ~= format("| Adjustment #%s:\n|   Scope: %s '%s'\n|   Reason: %s\n", i+1, change.kind, change.name, change.reason);
    out_ ~= border; return out_.data.idup;
}

// Go's Python CLI filters whole-file bounds after range adjustment. Keep the
// native token finder; adapt its significant ends to the CLI's trailing-trivia
// convention only here. Ordinary file/directory maps remain unchanged.
PythonRange adjustPythonRange(ref FileResult result, string raw, int start, int end) {
    auto lines = raw.split("\n");
    auto source = raw.replace("\r\n", "\n");
    auto tokens = tokenizePython(source);
    auto t = tokens.items; auto pairs = tokens.pairs;
    struct Statement { int first, last, indent; }
    Statement[] statements; int first;
    foreach (i, token; t) if (token.text == "\n") {
        if (first < i) statements ~= Statement(first, cast(int)i-1, goIndent(lines[t[first].line-1]));
        first = cast(int)i+1;
    }
    if (first < t.length) statements ~= Statement(first, cast(int)t.length-1, goIndent(lines[t[first].line-1]));
    Scope[] scopes;
    void add(ref Sym sym, string kind) {
        foreach (index, st; statements) {
            int keyword = st.first;
            if (t[keyword].text == "async") ++keyword;
            if (keyword+1 > st.last || t[keyword].text != (kind == "function" ? "def" : "class") ||
                t[keyword+1].text != sym.name || t[keyword].line < sym.start || t[keyword].line > sym.end) continue;
            int colon = -1;
            for (int i = keyword+2; i <= st.last; ++i) {
                if (t[i].text == ":") { colon = i; break; }
                if (["(","[","{"].canFind(t[i].text) && pairs[i] > i) i = pairs[i];
            }
            if (colon < 0) continue;
            bool inlineSuite = colon < st.last;
            int limit = cast(int)lines.length;
            foreach (next; statements[index+1..$]) {
                if (inlineSuite || next.indent <= st.indent) { limit = t[next.first].line-1; break; }
            }
            sym.end = limit;
            scopes ~= Scope(sym.name,kind,sym.start,limit,t[keyword].line);
            break;
        }
    }
    foreach (ref fn; result.functions) add(fn,"function");
    foreach (c; result.classes) add(c,"class");
    scopes.sort!((a,b) => a.declaration < b.declaration);
    int atStart = -1, atEnd = -1;
    foreach (i, scope_; scopes) {
        if (scope_.start <= start && start <= scope_.end) atStart = cast(int)i;
        if (scope_.start <= end && end <= scope_.end) atEnd = cast(int)i;
    }
    PythonRange fixed = PythonRange(start,end);
    Adjustment[] changes;
    void change(Scope s, string reason) { changes ~= Adjustment(s.name,s.kind,reason); }
    if (atStart < 0) {
        int next = -1;
        foreach (i, s; scopes) if (s.start > start && (next < 0 || s.start < scopes[next].start)) next = cast(int)i;
        if (next >= 0) {
            auto s = scopes[next]; fixed.start = s.start; fixed.end = s.end;
            change(s,"Requested range is in global scope, expanded to include " ~ s.kind ~ " '" ~ s.name ~ "'");
        } else if (atEnd >= 0) {
            auto s = scopes[atEnd]; fixed.end = s.end;
            change(s,"End line is inside " ~ s.kind ~ " '" ~ s.name ~ "', expanded to include full scope");
        }
    } else {
        auto s = scopes[atStart];
        if (s.kind == "function") {
            if (start != s.start) {
                fixed.start = s.start; fixed.end = s.end;
                change(s,"Requested range is inside function body, expanded to full function");
            } else if (end > s.end) {
                fixed.end = s.end;
                change(s,"Requested end exceeds function body, clipped to function end");
            }
        }
        if (atEnd >= 0 && atEnd != atStart && scopes[atEnd].kind == "function" && s.kind == "class") {
            fixed.end = s.end;
            change(s,"End line is inside a method of this class, clipped to class end");
        }
    }
    fixed.report = report(changes,start,end,fixed.start,fixed.end);
    return fixed;
}

unittest {
    auto r = FileResult("x.py","py"); r.functions = [Sym("f",2,3),Sym("g",6,6)];
    auto a = adjustPythonRange(r,"# global\ndef f():\n pass\n\n# trivia\ndef g(): pass\n",3,3);
    assert(a.start == 2 && a.end == 5 && r.functions[0].end == 5);
    assert(a.report.canFind("Requested range: 3:3") && a.report.canFind("Adjusted range:  2:5"));
}

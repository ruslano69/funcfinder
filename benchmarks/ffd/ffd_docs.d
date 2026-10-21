module ffd_docs;

import ffd_model;
import ffd_langs : configText, splitLines;
import std.algorithm : canFind, sort, reverse, SwapStrategy;
import std.array : appender;
import std.exception : enforce;
import std.file : read;
import std.format : format;
import std.json : parseJSON;
import std.string : split, strip, startsWith, endsWith;

struct DocConfig { string lang, lineComment, blockStart, blockEnd; }

string[] docNames(string text) {
    string[] names;
    foreach (part; text.split(",")) if (part.strip.length) names ~= part.strip;
    enforce(names.length, "--doc needs at least one name"); return names;
}

private int skipAnnotations(string[] lines, int i, string lang) {
    while (i >= 0) {
        auto text = lines[i].strip;
        if (text.startsWith("@") || lang == "rust" && text.startsWith("#[")) --i;
        else if (lang == "rust" && (text.startsWith(")]") || text.startsWith("]"))) {
            auto j = i-1;
            while (j >= 0 && i-j <= 20 && !lines[j].strip.startsWith("#[")) --j;
            if (j < 0 || i-j > 20) return i;
            i = j-1;
        } else return i;
    }
    return i;
}

// Raw source is output data here, not declaration discovery. Match Go's adjacent
// comment rule and annotation skipping, without adding a parser or a regex.
string[] docLines(string[] lines, int start, DocConfig cfg) {
    string[] bottomUp;
    int i = start-2;
    if (i >= lines.length) i = cast(int)lines.length-1;
    i = skipAnnotations(lines,i,cfg.lang);
    while (i >= 0) {
        auto text = lines[i].strip;
        if (cfg.lineComment.length && text.startsWith(cfg.lineComment)) { bottomUp ~= text; --i; }
        else if (cfg.blockStart.length && cfg.blockEnd.length && text.endsWith(cfg.blockEnd)) {
            auto j = i;
            while (!lines[j].strip.startsWith(cfg.blockStart)) {
                --j;
                if (j < 0) { bottomUp.reverse; return bottomUp; }
                auto above = lines[j].strip;
                if (!above.startsWith("*") && !above.startsWith(cfg.blockStart)) { bottomUp.reverse; return bottomUp; }
            }
            for (auto k = i; k >= j; --k) bottomUp ~= lines[k].strip;
            i = j-1;
        } else break;
    }
    bottomUp.reverse; return bottomUp;
}

string formatDocs(FileResult[] results, string[] wanted, string[] singleLines = null) {
    auto config = parseJSON(configText)["languages"];
    auto files = results.dup; files.sort!((a,b) => a.path < b.path);
    auto out_ = appender!(char[])(); bool haveEntry;
    struct Row { string kind, name; int line; }
    foreach (ref r; files) {
        if (r.error.length) continue;
        Row[] rows;
        foreach (f; r.functions) if (wanted.canFind(f.name)) rows ~= Row(f.className.length ? "method" : "function",f.name,f.start);
        foreach (c; r.classes) if (wanted.canFind(c.name)) rows ~= Row(c.kind.length ? c.kind : "class",c.name,c.start);
        foreach (a; r.aliases) if (wanted.canFind(a.name)) rows ~= Row("alias",a.name,a.line);
        if (!rows.length) continue;
        auto cfg = config[r.lang];
        DocConfig comments = DocConfig(r.lang,cfg["line_comment"].str,cfg["block_comment_start"].str,cfg["block_comment_end"].str);
        auto lines = singleLines;
        if (lines is null) lines = splitLines(cast(string)read(r.path)).dup;
        rows.sort!((a,b) => a.line < b.line, SwapStrategy.stable);
        foreach (row; rows) {
            if (haveEntry) out_ ~= "\n";
            haveEntry = true;
            out_ ~= format("%s:%s: %s %s\n",r.path,row.line,row.kind,row.name);
            foreach (comment_; docLines(lines,row.line,comments)) out_ ~= comment_ ~ "\n";
            if (row.line >= 1 && row.line <= lines.length) out_ ~= lines[row.line-1].strip ~ "\n";
        }
    }
    return out_.data.idup;
}

unittest {
    auto cfg = DocConfig("go","//","/*","*/");
    assert(docLines(["// first","// second","func f() {}"],3,cfg) == ["// first","// second"]);
    assert(docLines(["// detached","","func f() {}"],3,cfg).length == 0);
    assert(docLines(["/**"," * docs"," */","func f() {}"],4,cfg) == ["/**","* docs","*/"]);
    assert(docLines(["code"," * not a comment"," */","func f() {}"],4,cfg).length == 0);
    assert(docLines(["// docs","@decorator","def f():"],3,DocConfig("py","#","","" )).length == 0);
    assert(docLines(["# docs","@decorator","def f():"],3,DocConfig("py","#","","" )) == ["# docs"]);
    assert(docLines(["/// docs","#[cfg(","test",")]","fn f() {}"],5,DocConfig("rust","//","/*","*/")) == ["/// docs"]);
    assert(docNames(" A, ,B ") == ["A","B"]);
    auto r = FileResult("x.go","go"); r.functions = [Sym("f",2,2)]; r.aliases = [Alias("A",4,"int")];
    assert(formatDocs([r],["f","A","f"],["// f docs","func f() {}","// alias docs","type A = int"]) ==
        "x.go:2: function f\n// f docs\nfunc f() {}\n\nx.go:4: alias A\n// alias docs\ntype A = int\n");
}

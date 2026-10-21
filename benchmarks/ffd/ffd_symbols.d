module ffd_symbols;

import ffd_model;
import std.algorithm : sort;
import std.array : appender;
import std.format : format;
import std.string : split, join;

// Metadata only: declaration discovery and boundaries still belong to the native finders.
// Scan tokens, ignoring comments/literals, instead of adding declaration regexes.
private string[] headerWords(string source, string lang) {
    string[] words; size_t start; bool word;
    for (size_t i; i < source.length;) {
        char c = source[i];
        if (word && !(c >= 0x80 || c == '_' || c == '$' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9')) {
            words ~= source[start..i]; word = false;
        }
        if (i+1 < source.length && source[i..i+2] == "/*") {
            i += 2; int depth = 1;
            while (i < source.length && depth) {
                if (i+1 < source.length && source[i..i+2] == "*/") { --depth; i += 2; }
                else if ((lang == "d" || lang == "scala" || lang == "rust") && i+1 < source.length && source[i..i+2] == "/*") { ++depth; i += 2; }
                else ++i;
            }
            continue;
        }
        if (i+1 < source.length && source[i..i+2] == "//" || c == '#' && (lang == "py" || lang == "ruby" || lang == "php")) {
            while (i < source.length && source[i] != '\n') ++i;
            continue;
        }
        if (c == '\'' || c == '"' || c == '`') {
            ++i;
            while (i < source.length) {
                if (source[i] == '\\') { i += i+1 < source.length ? 2 : 1; continue; }
                if (source[i++] == c) break;
            }
            continue;
        }
        if (c == '{' || c == ';') break;
        if (!word && (c >= 0x80 || c == '_' || c == '$' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z')) { word = true; start = i; }
        ++i;
    }
    if (word) words ~= source[start..$];
    return words;
}

void annotateTypes(ref FileResult r, string source) {
    auto lines = source.split("\n");
    foreach (ref sym; r.classes) {
        if (sym.kind.length || sym.start < 1 || sym.start > lines.length) continue;
        auto end = sym.end < lines.length ? sym.end : lines.length;
        auto words = headerWords(lines[sym.start-1..end].join("\n"), r.lang);
        string kind = "class";
        foreach (word; words) {
            switch (word) {
                case "class": if (kind != "enum") kind = word; break;
                case "struct", "interface", "enum", "union", "trait", "object", "protocol", "record", "module": kind = word; break;
                case "typealias", "alias", "typedef": kind = "type_alias"; break;
                case "type": kind = r.lang == "go" ? "named" : "type_alias"; break;
                default: break;
            }
        }
        sym.kind = kind;
    }
}

string formatNames(FileResult[] results) {
    auto files = results.dup; files.sort!((a,b) => a.path < b.path);
    auto out_ = appender!(char[])();
    struct Row { string kind, name; int line; }
    foreach (ref r; files) {
        Row[] rows;
        foreach (f; r.functions) rows ~= Row(f.className.length ? "method" : "function", f.name, f.start);
        foreach (c; r.classes) rows ~= Row(c.kind.length ? c.kind : "class", c.name, c.start);
        foreach (a; r.aliases) rows ~= Row("alias", a.name, a.line);
        import std.algorithm : SwapStrategy;
        rows.sort!((a,b) => a.line < b.line, SwapStrategy.stable);
        foreach (row; rows) out_ ~= format("%s\t%s\t%s:%s\n", row.kind, row.name, r.path, row.line);
    }
    return out_.data.idup;
}

unittest {
    auto r = FileResult("x.go", "go"); r.classes = [Sym("C",2,2)];
    annotateTypes(r, "package p\ntype C struct{}\n"); assert(r.classes[0].kind == "struct");
    r.functions = [Sym("f",1,1), Sym("m",3,3,"C")]; r.aliases = [Alias("A",4,"C")];
    assert(formatNames([r]) == "function\tf\tx.go:1\nstruct\tC\tx.go:2\nmethod\tm\tx.go:3\nalias\tA\tx.go:4\n");
    r = FileResult("x.cpp", "cpp"); r.classes = [Sym("C",1,1)];
    annotateTypes(r, "/* class decoy */ enum class C {};"); assert(r.classes[0].kind == "enum");
}

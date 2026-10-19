module ffd_views;

import ffd_json : putString;
import ffd_langs : Lang, commentMarkers, languageFromKey, readSource, splitLines;
import ffd_model;
import ffd_paths;
import std.algorithm : sort, SwapStrategy, startsWith, endsWith, canFind, min;
import std.array : Appender, appender, join;
import std.conv : to;
import std.path : dirSeparator;
import std.string : strip, indexOf, split, lastIndexOf;
import std.utf : decode, UTFException;

// The text views of the Go tool, written over the neutral result: --names, --doc, the directory tree, the file views
// (--func, --map, --tree, --tree-full, --extract, --json, and the same for types). Every function returns the text the
// Go formatter returns (internal/names.go, docs.go, tree.go, formatter.go, struct_formatter.go, dirprocessor.go); the
// caller prints it and adds the newline fmt.Println adds. Differences that come from the finders (the D finders keep no
// decorators and no struct fields) are in benchmarks/ffd/AGENTS.md.

// --- names and docs -------------------------------------------------------------------------------------------------------

private struct Declaration {
    string kind;
    string name;
    int line;
}

private string functionKind(ref const Sym fn) { return fn.className.length ? "method" : "function"; }
private string classKind(ref const Sym c) { return c.kind.length ? c.kind : "class"; }

private Declaration[] declarationsOf(ref const FileResult r, scope bool delegate(string) wanted) {
    Declaration[] d;
    foreach (ref f; r.functions) if (wanted(f.name)) d ~= Declaration(functionKind(f), f.name, f.start);
    foreach (ref c; r.classes) if (wanted(c.name)) d ~= Declaration(classKind(c), c.name, c.start);
    foreach (ref a; r.aliases) if (wanted(a.name)) d ~= Declaration("alias", a.name, a.line);
    d.sort!((a, b) => a.line < b.line, SwapStrategy.stable);
    return d;
}

/// `kind<TAB>name<TAB>path:line`, one per declaration: files in path order, the names of a file in line order.
string formatNames(const(FileResult)[] results) {
    auto o = appender!string();
    foreach (ref r; results)
        foreach (ref d; declarationsOf(r, (string) => true)) {
            o ~= d.kind; o ~= '\t'; o ~= d.name; o ~= '\t'; o ~= r.path; o ~= ':'; o ~= d.line.to!string; o ~= '\n';
        }
    return o.data;
}

// A comment line is trimmed text; the lists are in file order.
private string[] reversed(string[] s) {
    foreach (i; 0 .. s.length / 2) {
        auto t = s[i];
        s[i] = s[s.length - 1 - i];
        s[s.length - 1 - i] = t;
    }
    return s;
}

private long skipAnnotations(const(string)[] lines, long i, bool rust) {
    while (i >= 0) {
        auto trimmed = strip(lines[cast(size_t) i]);
        if (trimmed.startsWith("@")) --i;
        else if (rust && trimmed.startsWith("#[")) --i;
        else if (rust && (trimmed.startsWith(")]") || trimmed.startsWith("]"))) {
            long j = i - 1;
            while (j >= 0 && i - j <= 20 && !strip(lines[cast(size_t) j]).startsWith("#[")) --j;
            if (j < 0 || i - j > 20) return i;
            i = j - 1;
        } else return i;
    }
    return i;
}

/// The comment lines directly above the declaration that starts at line `start` (1-based), in file order, as internal.DocLines
/// reads them: decorators and annotations are skipped first, a blank line or code ends the comment, a block comment counts
/// when it ends on the line above and only its lines starting with "*" are taken.
string[] docLines(const(string)[] lines, int start, string lineComment, string blockStart, string blockEnd, bool rust) {
    string[] bottomUp;
    long i = start - 2;
    if (i >= cast(long) lines.length) i = cast(long) lines.length - 1;
    i = skipAnnotations(lines, i, rust);
    while (i >= 0) {
        auto trimmed = strip(lines[cast(size_t) i]);
        if (lineComment.length && trimmed.startsWith(lineComment)) {
            bottomUp ~= trimmed;
            --i;
        } else if (blockStart.length && blockEnd.length && trimmed.endsWith(blockEnd)) {
            long j = i;
            while (!strip(lines[cast(size_t) j]).startsWith(blockStart)) {
                --j;
                if (j < 0) return reversed(bottomUp);
                auto above = strip(lines[cast(size_t) j]);
                if (!above.startsWith("*") && !above.startsWith(blockStart)) return reversed(bottomUp);
            }
            for (long k = i; k >= j; --k) bottomUp ~= strip(lines[cast(size_t) k]);
            i = j - 1;
        } else return reversed(bottomUp);
    }
    return reversed(bottomUp);
}

/// `path:line: kind name`, the doc comment and the first line of each declaration whose name is in `wanted`; entries are
/// separated by a blank line, files are in path order and entries in line order. "" when nothing matches. Throws when a
/// file cannot be read again.
string formatDocs(const(FileResult)[] results, bool[string] wanted) {
    string[] entries;
    foreach (ref r; results) {
        if (r.error.length) continue;
        auto decls = declarationsOf(r, (string n) => (n in wanted) !is null);
        if (decls.length == 0) continue;
        Lang lang;
        if (!languageFromKey(r.lang, lang)) throw new Exception("unsupported language: " ~ r.lang);
        string lineC, blockS, blockE;
        commentMarkers(lang, lineC, blockS, blockE);
        auto lines = splitLines(readSource(r.path));
        foreach (ref d; decls) {
            auto b = appender!string();
            b ~= r.path; b ~= ':'; b ~= d.line.to!string; b ~= ": "; b ~= d.kind; b ~= ' '; b ~= d.name; b ~= '\n';
            foreach (c; docLines(lines, d.line, lineC, blockS, blockE, lang == Lang.rust)) { b ~= c; b ~= '\n'; }
            if (d.line >= 1 && d.line <= lines.length) { b ~= strip(lines[d.line - 1]); b ~= '\n'; }
            entries ~= b.data;
        }
    }
    return entries.join("\n");
}

// --- the directory tree ---------------------------------------------------------------------------------------------------

private final class DirNode {
    string name;
    DirNode[string] children;
    const(Sym)[] functions, classes;
}

private void writeDirNode(ref Appender!string o, DirNode node, string prefix, bool isLast, bool root) {
    if (!root) {
        o ~= prefix; o ~= isLast ? "└── " : "├── "; o ~= node.name; o ~= '\n';
        immutable newPrefix = prefix ~ (isLast ? "    " : "│   ");
        foreach (i, ref fn; node.functions) {
            o ~= newPrefix;
            o ~= i == node.functions.length + node.classes.length - 1 ? "└── " : "├── ";
            o ~= "def "; o ~= fn.name; o ~= " (line "; o ~= fn.start.to!string; o ~= ")\n";
        }
        foreach (i, ref c; node.classes) {
            o ~= newPrefix;
            o ~= i == node.classes.length - 1 && node.functions.length == 0 ? "└── " : "├── ";
            o ~= "class "; o ~= c.name; o ~= " (line "; o ~= c.start.to!string; o ~= ")\n";
        }
    }
    auto names = node.children.keys;
    names.sort();
    foreach (i, name; names) writeDirNode(o, node.children[name], prefix, i + 1 == names.length, false); // the Go tool passes the prefix on unchanged
}

/// The tree of `funcfinder --dir --tree` (--tree-full gives the same): files under their directories, each with its
/// functions (`def`) and types (`class`), names in byte order. "No functions found" for a run that found no file at all.
string formatDirTree(const(FileResult)[] results) {
    if (results.length == 0) return "No functions found";
    auto root = new DirNode;
    foreach (ref r; results) {
        if (r.functions.length == 0 && r.classes.length == 0) continue;
        auto rel = relativePath(".", r.path);
        if (rel.length == 0) rel = r.path;
        string[] parts;
        {
            size_t s = 0;
            foreach (k; 0 .. rel.length)
                if (rel[k] == dirSeparatorChar) { parts ~= rel[s .. k]; s = k + 1; }
            parts ~= rel[s .. $];
        }
        auto current = root;
        foreach (part; parts[0 .. $ - 1]) {
            auto p = part in current.children;
            if (p is null) {
                auto n = new DirNode;
                // filepath.Base of a bare volume name ("G:") is ".".
                n.name = part.length == 2 && part[1] == ':' ? "." : part;
                current.children[part] = n;
                current = n;
            } else current = *p;
        }
        auto f = new DirNode;
        f.name = parts[$ - 1];
        f.functions = r.functions;
        f.classes = r.classes;
        current.children[parts[$ - 1]] = f;
    }
    auto o = appender!string();
    writeDirNode(o, root, "", true, true);
    return o.data;
}

private enum dirSeparatorChar = dirSeparator[0];

// --- the file views: functions ---------------------------------------------------------------------------------------------

/// What a single-file run shows: the functions and types of a file and its lines (for bodies and signatures).
struct FileView {
    string path;
    const(Sym)[] functions;
    const(Sym)[] classes;
    const(string)[] lines;
}

private string[] bodyOf(ref const FileView v, ref const Sym s) {
    string[] r;
    for (int i = s.start - 1; i < s.end && i < cast(int) v.lines.length; ++i)
        if (i >= 0) r ~= v.lines[i].idup;
    return r;
}

/// `Name: 45-78; Other: 120-145;`
string formatGrepStyle(const(Sym)[] functions) {
    if (functions.length == 0) return "";
    auto o = appender!string();
    foreach (i, ref f; functions) {
        if (i) o ~= "; ";
        o ~= f.name; o ~= ": "; o ~= f.start.to!string; o ~= '-'; o ~= f.end.to!string;
    }
    o ~= ';';
    return o.data;
}

/// The functions as a JSON object keyed by name (a name that occurs twice keeps its last entry), keys in order, two-space
/// indent, as json.MarshalIndent writes the Go tool's map.
string formatFunctionsJson(const(Sym)[] functions) {
    Sym[string] byName;
    foreach (ref f; functions) byName[f.name] = f;
    auto keys = byName.keys;
    keys.sort();
    if (keys.length == 0) return "{}";
    auto o = appender!(char[])();
    o ~= "{\n";
    foreach (i, k; keys) {
        auto f = byName[k];
        o ~= "  "; putString(o, k); o ~= ": {\n";
        if (f.decorators.length) {
            o ~= "    \"decorators\": [\n";
            foreach (d, text; f.decorators) { o ~= "      "; putString(o, text); o ~= d + 1 < f.decorators.length ? ",\n" : "\n"; }
            o ~= "    ],\n";
        }
        o ~= "    \"end\": "; o ~= f.end.to!string; o ~= ",\n    \"start\": "; o ~= f.start.to!string;
        o ~= "\n  }"; o ~= i + 1 < keys.length ? ",\n" : "\n";
    }
    o ~= "}";
    return cast(string) o.data;
}

/// `// Name: 45-78` and the body, entries separated by a blank line.
string formatExtract(ref const FileView v, const(Sym)[] functions) {
    string[] parts;
    foreach (ref f; functions) parts ~= "// " ~ f.name ~ ": " ~ f.start.to!string ~ "-" ~ f.end.to!string ~ "\n" ~ bodyOf(v, f).join("\n");
    return parts.join("\n\n");
}

// --- the file views: the function tree ----------------------------------------------------------------------------------------

private final class Node {
    string name;
    bool isClass;
    int start, end;
    Node[] children;
    int depth;
    bool isLast;
    string[] lines;
}

private Node[] buildFunctionTree(ref const FileView v, const(Sym)[] functions) {
    if (functions.length == 0) return null;
    auto sorted = functions.dup;
    // The Go tool orders by exchange: a swap when the earlier one starts later, so equal starts may change places.
    foreach (i; 0 .. sorted.length - 1)
        foreach (j; i + 1 .. sorted.length)
            if (sorted[i].start > sorted[j].start) {
                auto t = sorted[i];
                sorted[i] = sorted[j];
                sorted[j] = t;
            }
    Node[] all;
    foreach (ref f; sorted) {
        auto n = new Node;
        n.name = f.name;
        n.start = f.start;
        n.end = f.end;
        n.lines = bodyOf(v, f);
        all ~= n;
    }
    Node parentOf(Node node) {
        foreach (c; all) {
            if (c is node) continue;
            if (c.start < node.start && c.end > node.end) return c;
        }
        return null;
    }
    bool sameLevel = true;
    foreach (n; all) {
        auto p = parentOf(n);
        if (p !is null) {
            sameLevel = false;
            p.children ~= n;
        }
    }
    if (sameLevel) return all;
    Node[] roots;
    foreach (n; all) if (parentOf(n) is null) roots ~= n;
    return roots;
}

private const(Sym)[] mergedByName(const(Sym)[] classes) {
    Sym[] outList;
    size_t[string] index;
    foreach (ref c; classes) {
        if (auto p = c.name in index) {
            if (c.start < outList[*p].start) outList[*p].start = c.start;
            if (c.end > outList[*p].end) outList[*p].end = c.end;
        } else {
            index[c.name] = outList.length;
            outList ~= c;
        }
    }
    return outList;
}

private Node[] buildTree(ref const FileView v) {
    Node[] roots;
    if (v.classes.length > 0) {
        foreach (ref c; mergedByName(v.classes)) {
            auto n = new Node;
            n.name = c.name;
            n.isClass = true;
            n.start = c.start;
            n.end = c.end;
            const(Sym)[] methods;
            foreach (ref f; v.functions) if (f.className == c.name) methods ~= f;
            if (methods.length) n.children = buildFunctionTree(v, methods);
            roots ~= n;
        }
        const(Sym)[] top;
        foreach (ref f; v.functions) if (f.className.length == 0) top ~= f;
        if (top.length) roots ~= buildFunctionTree(v, top);
    } else roots = buildFunctionTree(v, v.functions);
    void flags(Node[] nodes) {
        foreach (i, n; nodes) {
            if (i + 1 == nodes.length) n.isLast = true;
            foreach (c; n.children) c.depth = n.depth + 1;
            flags(n.children);
        }
    }
    flags(roots);
    return roots;
}

/// The signature of a function from its lines (internal.extractSignatureFromLines): `func ` headers, `def` headers up to the
/// colon, `function ` headers up to the brace, and the first line with a parenthesis, joined with the lines that follow it
/// until a `{` (at most four more).
string signatureOf(const(string)[] lines) {
    if (lines.length == 0) return "";
    string full;
    foreach (i, line; lines) {
        auto trimmed = strip(line);
        if (trimmed.length == 0 || trimmed.startsWith("@")) continue;
        if (trimmed.startsWith("func ")) {
            auto idx = trimmed.indexOf('{');
            if (idx > 0) return trimmed[0 .. idx]["func ".length .. $];
            return trimmed["func ".length .. $];
        }
        if (trimmed.startsWith("def ") || trimmed.startsWith("async def ")) {
            string sig = trimmed;
            for (size_t j = i + 1; j < lines.length && sig.indexOf(':') < 0; ++j) sig ~= " " ~ strip(lines[j]);
            auto idx = sig.indexOf(':');
            if (idx > 0) return strip(sig[0 .. idx]);
            return sig;
        }
        if (trimmed.canFind("function ")) {
            auto idx = trimmed.indexOf('{');
            if (idx > 0) return strip(trimmed[0 .. idx]);
            return trimmed;
        }
        if (trimmed.canFind('(') && (trimmed.canFind('{') || i + 1 < lines.length)) {
            string sig = trimmed;
            if (sig.indexOf('{') < 0) {
                for (size_t j = i + 1; j < lines.length && j < i + 5; ++j) {
                    auto next = strip(lines[j]);
                    sig ~= " " ~ next;
                    if (next.canFind('{')) break;
                }
            }
            auto open = sig.indexOf('{');
            full = open > 0 ? strip(sig[0 .. open]) : sig;
            return full.split().join(" ");
        }
    }
    return full;
}

private string nodeText(Node n, bool showTypes) {
    if (showTypes && !n.isClass) {
        auto sig = signatureOf(n.lines);
        if (sig.length) return sig ~ " (" ~ n.start.to!string ~ "-" ~ n.end.to!string ~ ")";
    }
    return n.name ~ " (" ~ n.start.to!string ~ "-" ~ n.end.to!string ~ ")";
}

private void writeNode(ref Appender!string o, Node n, bool showTypes, bool[] ancestorLast) {
    foreach (i; 0 .. cast(long) ancestorLast.length - 1) o ~= ancestorLast[cast(size_t) i] ? "    " : "│   ";
    if (ancestorLast.length) o ~= n.isLast ? "└── " : "├── ";
    if (n.isClass) o ~= "class ";
    else if (n.depth > 0) o ~= "method ";
    o ~= nodeText(n, showTypes);
    o ~= '\n';
}

private void writeChildren(ref Appender!string o, Node[] children, bool showTypes, bool[] ancestorLast) {
    foreach (i, c; children) {
        auto next = ancestorLast.dup ~ (i + 1 == children.length);
        writeNode(o, c, showTypes, next);
        writeChildren(o, c.children, showTypes, next);
    }
}

/// `--tree` and `--tree-full` for one file: types with their methods, then the functions outside a type; nested
/// functions under the function that holds them. `showTypes` writes signatures instead of names.
string formatFileTree(ref const FileView v, bool showTypes) {
    auto nodes = buildTree(v);
    if (nodes.length == 0) return "";
    auto o = appender!string();
    foreach (n; nodes) {
        writeNode(o, n, showTypes, null);
        writeChildren(o, n.children, showTypes, null);
    }
    auto text = o.data;
    return text.length && text[$ - 1] == '\n' ? text[0 .. $ - 1] : text;
}

// --- the file views: types ----------------------------------------------------------------------------------------------------

private string kindOrClass(ref const Sym t) { return t.kind.length ? t.kind : "class"; }

/// `Name: 3-9; fields: a, b Other: 12-20;`: a type with fields lists them after `; fields: `, one without ends in `;`.
string formatTypeMap(const(Sym)[] types) {
    string[] parts;
    foreach (ref t; types) {
        auto part = t.name ~ ": " ~ t.start.to!string ~ "-" ~ t.end.to!string;
        if (t.fields.length) {
            string[] names;
            foreach (ref f; t.fields) names ~= f.name;
            part ~= "; fields: " ~ names.join(", ");
        } else part ~= ";";
        parts ~= part;
    }
    return parts.join(" ");
}

string formatTypeTree(const(Sym)[] types) {
    string[] lines;
    foreach (ref t; types) {
        auto line = t.name ~ " (" ~ t.start.to!string ~ "-" ~ t.end.to!string ~ ") [" ~ kindOrClass(t) ~ "]";
        foreach (i, ref f; t.fields)
            line ~= "\n│   " ~ (i + 1 == t.fields.length ? "└── " : "├── ") ~ f.name ~ " " ~ f.type ~ ": " ~ f.line.to!string;
        lines ~= line;
    }
    return lines.join("\n");
}

string formatTypeExtract(ref const FileView v, const(Sym)[] types) {
    string[] parts;
    foreach (ref t; types) parts ~= "=== " ~ t.name ~ " (" ~ kindOrClass(t) ~ ") ===\n" ~ bodyOf(v, t).join("\n");
    return parts.join("\n\n");
}

/// `{"filename": ..., "types": [...]}`, two-space indent, as json.MarshalIndent writes the Go tool's struct.
string formatTypeJson(string filename, const(Sym)[] types) {
    auto o = appender!(char[])();
    o ~= "{\n  \"filename\": "; putString(o, filename); o ~= ",\n  \"types\": [";
    if (types.length == 0) o ~= "]";
    else {
        o ~= '\n';
        foreach (i, ref t; types) {
            o ~= "    {\n      \"name\": "; putString(o, t.name);
            o ~= ",\n      \"kind\": "; putString(o, kindOrClass(t));
            o ~= ",\n      \"start\": "; o ~= t.start.to!string;
            o ~= ",\n      \"end\": "; o ~= t.end.to!string;
            if (t.fields.length) {
                o ~= ",\n      \"fields\": [\n";
                foreach (k, ref f; t.fields) {
                    o ~= "        {\n          \"name\": "; putString(o, f.name);
                    o ~= ",\n          \"type\": "; putString(o, f.type);
                    o ~= ",\n          \"line\": "; o ~= f.line.to!string;
                    o ~= k + 1 < t.fields.length ? "\n        },\n" : "\n        }\n";
                }
                o ~= "      ]";
            }
            o ~= "\n    }"; o ~= i + 1 < types.length ? ",\n" : "\n";
        }
        o ~= "  ]";
    }
    o ~= "\n}";
    return cast(string) o.data;
}

/// A string as Go's %q writes it (strconv.Quote): the combined --all JSON prints names and the file name that way.
private string quoted(string s) {
    auto o = appender!string();
    o ~= '"';
    size_t i = 0;
    while (i < s.length) {
        immutable start = i;
        dchar c;
        try {
            c = decode(s, i);
        } catch (UTFException) {
            i = start + 1;
            o ~= "\\x"; o ~= "0123456789abcdef"[(cast(ubyte) s[start]) >> 4]; o ~= "0123456789abcdef"[s[start] & 15];
            continue;
        }
        switch (c) {
            case '"': o ~= "\\\""; break;
            case '\\': o ~= "\\\\"; break;
            case '\a': o ~= "\\a"; break;
            case '\b': o ~= "\\b"; break;
            case '\f': o ~= "\\f"; break;
            case '\n': o ~= "\\n"; break;
            case '\r': o ~= "\\r"; break;
            case '\t': o ~= "\\t"; break;
            case '\v': o ~= "\\v"; break;
            default:
                if (c < 0x20 || c == 0x7f) {
                    o ~= "\\x"; o ~= "0123456789abcdef"[c >> 4]; o ~= "0123456789abcdef"[c & 15];
                } else o ~= s[start .. i];
        }
    }
    o ~= '"';
    return o.data;
}

/// The combined `--all --json` of one file: functions then types, one entry per line, each type with its fields.
string formatAllJson(string filename, const(Sym)[] functions, const(Sym)[] types) {
    auto o = appender!string();
    o ~= "{\n  \"filename\": "; o ~= quoted(filename); o ~= ",\n  \"functions\": [\n";
    foreach (i, ref f; functions) {
        o ~= "    {\"name\": "; o ~= quoted(f.name); o ~= ", \"start\": "; o ~= f.start.to!string; o ~= ", \"end\": "; o ~= f.end.to!string; o ~= "}";
        o ~= i + 1 < functions.length ? ",\n" : "\n";
    }
    o ~= "  ],\n  \"types\": [\n";
    foreach (i, ref t; types) {
        o ~= "    {\"name\": "; o ~= quoted(t.name); o ~= ", \"kind\": "; o ~= quoted(kindOrClass(t));
        o ~= ", \"start\": "; o ~= t.start.to!string; o ~= ", \"end\": "; o ~= t.end.to!string; o ~= ", \"fields\": [";
        foreach (k, ref f; t.fields) {
            if (k) o ~= ", ";
            o ~= "{\"name\": "; o ~= quoted(f.name); o ~= ", \"type\": "; o ~= quoted(f.type); o ~= ", \"line\": "; o ~= f.line.to!string; o ~= "}";
        }
        o ~= "]}";
        o ~= i + 1 < types.length ? ",\n" : "\n";
    }
    o ~= "  ]\n}\n";
    return o.data;
}

version (unittest) {
    private FileView viewOf(string[] lines, Sym[] fns, Sym[] classes = null) {
        return FileView("x.go", fns, classes, lines);
    }
}

unittest {
    FileResult r;
    r.path = "a.go";
    r.functions = [Sym("Run", 10, 12, ""), Sym("Get", 5, 6, "Box")];
    r.classes = [Sym("Box", 3, 7, "", "struct")];
    r.aliases = [Alias("ID", 1, "int")];
    assert(formatNames([r]) == "alias\tID\ta.go:1\nstruct\tBox\ta.go:3\nmethod\tGet\ta.go:5\nfunction\tRun\ta.go:10\n");
}

unittest {
    assert(docLines(["// A doc", "// more", "func F() {}"], 3, "//", "/*", "*/", false) == ["// A doc", "// more"]);
    assert(docLines(["// A doc", "", "func F() {}"], 3, "//", "/*", "*/", false).length == 0);
    assert(docLines(["/**", " * Text", " */", "void f() {}"], 4, "//", "/*", "*/", false) == ["/**", "* Text", "*/"]);
    assert(docLines(["// doc", "@decorator", "def f():"], 3, "#", "", "", false).length == 0);
    assert(docLines(["# doc", "@decorator", "def f():"], 3, "#", "", "", false) == ["# doc"]);
    assert(docLines(["/// doc", "#[derive(Debug)]", "struct S;"], 3, "//", "/*", "*/", true) == ["/// doc"]);
    assert(docLines(["x = 1"], 1, "//", "/*", "*/", false).length == 0);
}

unittest {
    auto lines = ["func A() {", "  inner()", "}", "func B() {}"];
    auto v = viewOf(lines, [Sym("A", 1, 3, ""), Sym("B", 4, 4, "")]);
    assert(formatFileTree(v, false) == "A (1-3)\nB (4-4)");
    assert(formatFileTree(v, true) == "A()  (1-3)\nB()  (4-4)"); // the signature keeps the space before the brace
    auto nested = viewOf(["func A() {", "  func B() {", "  }", "}"], [Sym("A", 1, 4, ""), Sym("B", 2, 3, "")]);
    assert(formatFileTree(nested, false) == "A (1-4)\n└── method B (2-3)");
    auto cls = viewOf(["type Box struct {", "}", "func (b Box) Get() {}", "func Run() {}"],
                      [Sym("Get", 3, 3, "Box"), Sym("Run", 4, 4, "")], [Sym("Box", 1, 2, "", "struct")]);
    assert(formatFileTree(cls, false) == "class Box (1-2)\n└── method Get (3-3)\nRun (4-4)");
}

unittest {
    assert(formatGrepStyle([Sym("A", 1, 3, ""), Sym("B", 5, 9, "")]) == "A: 1-3; B: 5-9;");
    assert(formatFunctionsJson([Sym("B", 5, 9, ""), Sym("A", 1, 3, "")]) == "{\n  \"A\": {\n    \"end\": 3,\n    \"start\": 1\n  },\n  \"B\": {\n    \"end\": 9,\n    \"start\": 5\n  }\n}");
    auto v = viewOf(["a", "b", "c"], [Sym("F", 2, 3, "")]);
    assert(formatExtract(v, v.functions) == "// F: 2-3\nb\nc");
    auto tv = viewOf(["x", "y"], [], [Sym("T", 1, 2, "", "enum")]);
    assert(formatTypeExtract(tv, tv.classes) == "=== T (enum) ===\nx\ny");
    assert(formatTypeTree([Sym("T", 1, 2, "", "enum"), Sym("U", 4, 5, "")]) == "T (1-2) [enum]\nU (4-5) [class]");
    assert(quoted("a\"b\n") == "\"a\\\"b\\n\"");
}

unittest {
    // The Go tool passes the prefix on unchanged, so the children of a directory are indented like the directory itself.
    version (Windows) enum sep = "\\"; else enum sep = "/";
    FileResult x, y;
    x.path = "a" ~ sep ~ "x.go";
    x.functions = [Sym("F", 3, 4, "")];
    y.path = "b.go";
    y.classes = [Sym("C", 2, 5, "")];
    assert(formatDirTree([x, y]) == "├── a
└── x.go
    └── def F (line 3)
└── b.go
    └── class C (line 2)
");
    assert(formatDirTree(null) == "No functions found");
}

unittest {
    assert(signatureOf(["func (b Box) Get(x int) int {"]) == "(b Box) Get(x int) int ");
    assert(signatureOf(["def f(a,", "      b):", "    pass"]) == "def f(a, b)");
    assert(signatureOf(["public int add(int a, int b) {"]) == "public int add(int a, int b)");
    assert(signatureOf(["export function f(a) {"]) == "export function f(a)");
}

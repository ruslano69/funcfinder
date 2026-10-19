module ffd_json;

import ffd_model;
import std.array : Appender, appender;
import std.format : format;
import std.utf : decode, UTFException;

// Output in the exact shape of the Go tool's `--dir --json` (internal/dirprocessor.go: formatDirResultsJSON):
// json.MarshalIndent with two spaces, keys in struct order, files in path order, files without symbols left out and
// their count left out of total_files. The string rules are encoding/json's: <, > and & are written as \u003c, \u003e
// and \u0026, U+2028 and U+2029 as \u2028 and \u2029, control characters as \u00XX except \b \f \n \r \t, and every
// invalid UTF-8 byte as \ufffd; DEL and non-ASCII text are written as they are.

/// Appends s as a JSON string, as Go's encoding/json writes it.
void putString(ref Appender!(char[]) o, string s) {
    static immutable hex = "0123456789abcdef";
    o ~= '"';
    size_t i = 0;
    while (i < s.length) {
        immutable char c = s[i];
        if (c < 0x80) {
            ++i;
            switch (c) {
                case '"': o ~= `\"`; break;
                case '\\': o ~= `\\`; break;
                case '\n': o ~= `\n`; break;
                case '\r': o ~= `\r`; break;
                case '\t': o ~= `\t`; break;
                case '\b': o ~= `\b`; break;
                case '\f': o ~= `\f`; break;
                case '<': o ~= `\u003c`; break;
                case '>': o ~= `\u003e`; break;
                case '&': o ~= `\u0026`; break;
                default:
                    if (c < 0x20) {
                        o ~= `\u00`;
                        o ~= hex[c >> 4];
                        o ~= hex[c & 15];
                    } else
                        o ~= c;
            }
            continue;
        }
        // A multi-byte sequence: valid ones are copied, an invalid byte becomes U+FFFD (one per byte, as Go ranges).
        size_t j = i;
        dchar d;
        try {
            d = decode(s, j);
        } catch (UTFException) {
            o ~= `\ufffd`;
            ++i;
            continue;
        }
        if (d == 0x2028) o ~= `\u2028`;
        else if (d == 0x2029) o ~= `\u2029`;
        else o ~= s[i .. j];
        i = j;
    }
    o ~= '"';
}

private void putNumber(ref Appender!(char[]) o, long n) {
    import std.conv : to;
    o ~= n.to!string;
}

private void putSymbols(ref Appender!(char[]) o, const(Sym)[] syms, string indent) {
    if (syms.length == 0) {
        o ~= "[]";
        return;
    }
    o ~= "[\n";
    foreach (i, ref s; syms) {
        o ~= indent; o ~= "  {\n";
        o ~= indent; o ~= "    \"name\": "; putString(o, s.name); o ~= ",\n";
        o ~= indent; o ~= "    \"line\": "; putNumber(o, s.start); o ~= "\n";
        o ~= indent; o ~= i + 1 < syms.length ? "  },\n" : "  }\n";
    }
    o ~= indent; o ~= "]";
}

/// The text of `funcfinder --dir --json` for results already in path order, without the final newline Println adds.
string formatJson(const(FileResult)[] results) {
    auto o = appender!(char[])();
    o.reserve(1 << 16);
    long files, functions, classes;
    bool first = true;
    o ~= "{\n  \"files\": ";
    foreach (ref r; results) {
        if (r.functions.length == 0 && r.classes.length == 0 && r.aliases.length == 0) continue;
        o ~= first ? "[\n" : ",\n";
        first = false;
        o ~= "    {\n      \"path\": "; putString(o, r.path); o ~= ",\n";
        o ~= "      \"functions\": "; putSymbols(o, r.functions, "      "); o ~= ",\n";
        o ~= "      \"classes\": "; putSymbols(o, r.classes, "      ");
        if (r.aliases.length) {
            o ~= ",\n      \"aliases\": [\n";
            foreach (i, ref a; r.aliases) {
                o ~= "        {\n          \"name\": "; putString(o, a.name); o ~= ",\n";
                o ~= "          \"line\": "; putNumber(o, a.line); o ~= ",\n";
                o ~= "          \"target\": "; putString(o, a.target); o ~= "\n";
                o ~= i + 1 < r.aliases.length ? "        },\n" : "        }\n";
            }
            o ~= "      ]";
        }
        o ~= "\n    }";
        ++files;
        functions += r.functions.length;
        classes += r.classes.length;
    }
    if (first) o ~= "[]"; else o ~= "\n  ]";
    o ~= ",\n  \"total_files\": "; putNumber(o, files);
    o ~= ",\n  \"total_functions\": "; putNumber(o, functions);
    o ~= ",\n  \"total_classes\": "; putNumber(o, classes);
    o ~= "\n}\n";
    return cast(string) o.data;
}

/// The default (`--map`) text of `funcfinder --dir`: one `path:line: name` per function, then per type.
string formatGrep(const(FileResult)[] results) {
    auto o = appender!(char[])();
    foreach (ref r; results) {
        foreach (ref f; r.functions) { o ~= r.path; o ~= ':'; putNumber(o, f.start); o ~= ": "; o ~= f.name; o ~= '\n'; }
        foreach (ref c; r.classes) { o ~= r.path; o ~= ':'; putNumber(o, c.start); o ~= ": "; o ~= c.name; o ~= '\n'; }
    }
    return cast(string) o.data;
}

version (unittest) private FileResult made(string path, Sym[] functions, Sym[] classes, Alias[] aliases) {
    FileResult r;
    r.path = path;
    r.lang = "go";
    r.functions = functions;
    r.classes = classes;
    r.aliases = aliases;
    return r;
}

unittest {
    // Expected strings are what Go 1.25's json.Marshal prints for the same inputs.
    static immutable string[2][] cases = [
        ["plain", `"plain"`],
        ["a\"b", `"a\"b"`],
        ["a\\b", `"a\\b"`],
        ["a<b>&c", `"a\u003cb\u003e\u0026c"`],
        ["tab\there", `"tab\there"`],
        ["nl\nx", `"nl\nx"`],
        ["cr\rx", `"cr\rx"`],
        ["bs\bx", `"bs\bx"`],
        ["ff\fx", `"ff\fx"`],
        ["nul\0x", `"nul\u0000x"`],
        ["us\x1fx", `"us\u001fx"`],
        ["del\x7fx", "\"del\x7fx\""],
        ["é日本", "\"é日本\""],
        ["\u2028\u2029", `"\u2028\u2029"`],
        ["bad\xffbyte", `"bad\ufffdbyte"`],
        ["bad\xc3", `"bad\ufffd"`],
        ["emoji😀", "\"emoji😀\""],
        ["G:\\DEV\\x.go", `"G:\\DEV\\x.go"`],
        ["\x1b[0m", `"\u001b[0m"`],
    ];
    foreach (c; cases) {
        auto o = appender!(char[])();
        putString(o, c[0]);
        assert(o.data == c[1], format("escape of %(%s%): got %s, want %s", [c[0]], o.data, c[1]));
    }
}

unittest {
    FileResult[] none;
    assert(formatJson(none) == "{\n  \"files\": [],\n  \"total_files\": 0,\n  \"total_functions\": 0,\n  \"total_classes\": 0\n}\n");
    FileResult[] one = [made("a.go", [Sym("f", 3, 5, "")], [], [])];
    assert(formatJson(one) ==
        "{\n  \"files\": [\n    {\n      \"path\": \"a.go\",\n      \"functions\": [\n        {\n          \"name\": \"f\",\n"
        ~ "          \"line\": 3\n        }\n      ],\n      \"classes\": []\n    }\n  ],\n  \"total_files\": 1,\n"
        ~ "  \"total_functions\": 1,\n  \"total_classes\": 0\n}\n");
}

unittest {
    // A file with only an alias is listed, with the alias's three keys after the classes.
    FileResult[] one = [made("a.go", [], [], [Alias("A", 2, "S<x>")])];
    immutable want = `{
  "files": [
    {
      "path": "a.go",
      "functions": [],
      "classes": [],
      "aliases": [
        {
          "name": "A",
          "line": 2,
          "target": "S` ~ "\\" ~ `u003cx` ~ "\\" ~ `u003e"
        }
      ]
    }
  ],
  "total_files": 1,
  "total_functions": 0,
  "total_classes": 0
}
`;
    auto got = formatJson(one);
    size_t k;
    while (k < got.length && k < want.length && got[k] == want[k]) ++k;
    assert(got == want, format("first difference at %s (got %s want %s): ...%s", k, got.length, want.length, got[k > 10 ? k - 10 : 0 .. $]));
}

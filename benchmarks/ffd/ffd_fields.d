module ffd_fields;

import ffd_langs : Expect, Lang, configText, expectation, langKey;
import ffd_model;
import std.algorithm : canFind, startsWith, endsWith, sort;
import std.array : appender, join;
import std.json;
import std.regex : Regex, regex, matchFirst;
import std.string : strip, indexOf;

// What the single-file type views need beyond the finders' keys: the fields of a type (Go's StructFinder.findFieldsForType
// and the Python struct finder's), the decorators of a Python function, and the kind of a Python class. The fields are read
// from the language's field pattern over the file's cleaned lines (comments, strings and character literals blanked),
// so a cleaner for every language is needed: the one below is driven by the lexical settings the finders were written
// against (`Expect`), the same ones the configuration check pins.

/// The lines with comments, string literals and character literals blanked (same length), and for each line whether it
/// starts inside a string or a comment.
struct Cleaned {
    string[] lines;
    bool[] startsInside;
}

private bool at(const(char)[] s, size_t i, string p) {
    return i + p.length <= s.length && s[i .. i + p.length] == p;
}

private string closerOf(string opener) {
    // r#"..."# closes with a quote and the same number of hashes; others close with their last quote run.
    if (opener.length >= 3 && opener[0] == 'r' && opener[1] == '#') {
        size_t hashes = 0;
        while (opener[1 + hashes] == '#') ++hashes;
        string c = "\"";
        foreach (_; 0 .. hashes) c ~= "#";
        return c;
    }
    if (opener == "\"\"\"" || opener == "'''") return opener;
    if (opener == "`") return "`";
    return opener[$ - 1 .. $];
}

/// Blanks the comments and literals of `lines` as the language's lexical settings say (see the module comment). Block
/// comments do not nest; a string that does not close on its line ends with it, a raw string or docstring goes on.
Cleaned cleanLines(const(string)[] lines, Lang lang) {
    const e = expectation(lang);
    Cleaned result;
    result.lines.length = lines.length;
    result.startsInside.length = lines.length;
    enum Mode { code, block, raw }
    Mode mode;
    string closing; // what ends a block comment or a raw string
    // Longest opener first, so r#" wins over r".
    string[] openers;
    foreach (o; e.raw) openers ~= o;
    foreach (o; e.docStrings) openers ~= o;
    openers.sort!((a, b) => a.length > b.length);
    foreach (n, line; lines) {
        auto blank = new char[line.length];
        blank[] = ' ';
        result.startsInside[n] = mode != Mode.code;
        size_t i = 0;
        while (i < line.length) {
            if (mode != Mode.code) {
                auto k = line.indexOf(closing, i);
                if (k < 0) { i = line.length; break; }
                i = k + closing.length;
                mode = Mode.code;
                continue;
            }
            if (e.lineComment.length && at(line, i, e.lineComment)) break;
            if (e.blockStart.length && at(line, i, e.blockStart)) {
                mode = Mode.block;
                closing = e.blockEnd;
                i += e.blockStart.length;
                continue;
            }
            bool opened;
            foreach (o; openers) {
                // The patterns of some languages are regular expressions (Swift's multi-line string); read their opener.
                string op = o;
                if (op.canFind(".*?")) op = op[0 .. op.indexOf(".*?")];
                if (op.length && at(line, i, op)) {
                    mode = Mode.raw;
                    closing = closerOf(op);
                    i += op.length;
                    opened = true;
                    break;
                }
            }
            if (opened) continue;
            bool literal;
            foreach (d; e.strings) {
                if (at(line, i, d)) {
                    // A string runs to its closing quote, with backslash escapes; it ends with the line when it does not close.
                    size_t j = i + d.length;
                    while (j < line.length && !at(line, j, d)) j += line[j] == '\\' ? 2 : 1;
                    i = j < line.length ? j + d.length : line.length;
                    literal = true;
                    break;
                }
            }
            if (literal) continue;
            foreach (d; e.chars) {
                if (at(line, i, d)) {
                    size_t j = i + d.length;
                    if (e.charLookahead) {
                        // Rust: 'a' and '\n' are characters, 'a (a lifetime) is code.
                        immutable isChar = (j + 1 < line.length && line[j] == '\\') || (j + 1 < line.length && line[j + 1] == '\'');
                        if (!isChar) break;
                    }
                    while (j < line.length && !at(line, j, d)) j += line[j] == '\\' ? 2 : 1;
                    i = j < line.length ? j + d.length : line.length;
                    literal = true;
                    break;
                }
            }
            if (literal) continue;
            blank[i] = line[i];
            ++i;
        }
        result.lines[n] = cast(string) blank;
    }
    return result;
}

struct FieldSpec {
    Regex!char pattern;
    bool hasPattern;
    string lineComment;
    string[] excluded;
}

/// Go's regexp (RE2) accepts a bare `[` inside a character class (`[\w<>[\]]`); std.regex needs it escaped. Nothing else
/// the configured field patterns use differs.
string forStdRegex(string pattern) {
    string r;
    bool inClass;
    size_t i = 0;
    while (i < pattern.length) {
        immutable c = pattern[i];
        if (c == '\\' && i + 1 < pattern.length) {
            r ~= pattern[i .. i + 2];
            i += 2;
            continue;
        }
        if (!inClass) {
            r ~= c;
            if (c == '[') {
                inClass = true;
                if (i + 1 < pattern.length && pattern[i + 1] == '^') { r ~= '^'; ++i; }
                // a "]" right after the opening is a literal
                if (i + 1 < pattern.length && pattern[i + 1] == ']') { r ~= "\\]"; ++i; }
            }
        } else if (c == ']') {
            r ~= c;
            inClass = false;
        } else if (c == '[') r ~= "\\[";
        else r ~= c;
        ++i;
    }
    return r;
}

FieldSpec fieldSpecFor(Lang lang) {
    FieldSpec spec;
    auto cfg = parseJSON(configText)["languages"][langKey(lang)];
    if (auto p = "field_pattern" in cfg.object)
        if (p.type == JSONType.string && p.str.length) {
            spec.pattern = regex(forStdRegex(p.str));
            spec.hasPattern = true;
        }
    if (auto p = "exclude_words" in cfg.object)
        if (p.type == JSONType.array)
            foreach (w; p.array) spec.excluded ~= w.str;
    spec.lineComment = expectation(lang).lineComment;
    return spec;
}

/// The fields of the lines [start, end) of a type (1-based start, end exclusive: the last line of the type is its closing
/// line), as the Go struct finder reads them: every non-blank, non-comment cleaned line is matched with the language's field
/// pattern; a name that is empty, an excluded word, or on a line with a parenthesis is not a field.
Field[] fieldsOfType(ref FieldSpec spec, int start, int end, const(string)[] cleaned) {
    Field[] fields;
    if (!spec.hasPattern) return fields;
    for (int n = start - 1; n < cleaned.length && n < end - 1; ++n) {
        auto line = cleaned[n];
        auto trimmed = strip(line);
        if (trimmed.length == 0 || trimmed.startsWith(spec.lineComment)) continue;
        auto m = matchFirst(line, spec.pattern);
        if (m.empty || m.captures.length < 2) continue;
        auto name = m.captures[1];
        string type;
        if (m.captures.length >= 3) type = strip(m.captures[2]).idup;
        if (name.length == 0 || line.canFind('(') || line.canFind(')')) continue;
        bool excluded;
        foreach (w; spec.excluded) if (w == name) { excluded = true; break; }
        if (excluded) continue;
        fields ~= Field(name.idup, type, n + 1);
    }
    return fields;
}

// --- Python ---------------------------------------------------------------------------------------------------------------

private bool isWordChar(char c) {
    return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_';
}

/// A Python line that is code: it starts outside a string and holds text after blanking (a blank line, a comment and a
/// line of a docstring are not).
private bool pythonCode(ref const Cleaned c, size_t n) {
    return !c.startsInside[n] && strip(c.lines[n]).length > 0;
}

/// The fields of a Python class as the Python struct finder reads them: `name: type` and `name: type = value` lines among
/// the class's code lines, other than decorators, `def` lines and special methods. The end is inclusive here: the Go
/// finder's class end takes in the blank lines after the class, so its exclusive end still reaches the last code line,
/// and the D finders' end is that last line.
Field[] pythonFields(int start, int end, ref const Cleaned c) {
    Field[] fields;
    for (int n = start - 1; n < c.lines.length && n < end; ++n) {
        if (n < 0 || !pythonCode(c, n)) continue;
        auto t = strip(c.lines[n]);
        if (t.startsWith("@") || t.startsWith("def ")) continue;
        if (t.startsWith("__") && t.endsWith("__():")) continue;
        // ^\s*(\w+)\s*:\s*([\w\[\],\s\.\*]*)\s*(=|$)
        size_t i = 0;
        while (i < t.length && isWordChar(t[i])) ++i;
        if (i == 0) continue;
        auto name = t[0 .. i];
        while (i < t.length && (t[i] == ' ' || t[i] == '\t')) ++i;
        if (i >= t.length || t[i] != ':') continue;
        ++i;
        size_t typeStart = i;
        while (i < t.length && (isWordChar(t[i]) || t[i] == '[' || t[i] == ']' || t[i] == ',' || t[i] == ' ' || t[i] == '\t' || t[i] == '.' || t[i] == '*')) ++i;
        // The type run is greedy; the pattern needs "=" or the end after it (with whitespace allowed in between).
        auto rest = t[i .. $];
        if (!(rest.length == 0 || rest[0] == '=')) continue;
        auto type = strip(t[typeStart .. i]);
        if (name == "class" || name == "def") continue;
        if (t.canFind("->") && t.canFind("def ")) continue;
        fields ~= Field(name.idup, type.idup, n + 1);
    }
    return fields;
}

private bool decoratorIs(string trimmed, string head) {
    // ^@head(\(.*\))?$
    if (!trimmed.startsWith(head)) return false;
    auto rest = trimmed[head.length .. $];
    return rest.length == 0 || (rest[0] == '(' && rest[$ - 1] == ')');
}

/// The kind of the Python class declared on line `start` (1-based): a dataclass or attrs decorator above it decides;
/// otherwise its base classes do (NamedTuple, TypedDict, Enum, ABC, Protocol); else "class".
string pythonKind(const(string)[] lines, int start, ref const Cleaned c) {
    string decorated;
    for (int n = start - 2; n >= 0; --n) {
        if (!pythonCode(c, n)) continue;
        auto t = strip(c.lines[n]);
        if (!t.startsWith("@")) break;
        if (decorated.length == 0) {
            if (decoratorIs(t, "@dataclass") || decoratorIs(t, "@dataclasses.dataclass")) decorated = "dataclass";
            else if (decoratorIs(t, "@attr.s")) decorated = "attrs";
        }
    }
    if (decorated.length) return decorated;
    if (start < 1 || start > cast(int) lines.length) return "class";
    auto line = lines[start - 1];
    auto open = line.indexOf('(');
    if (open < 0) return "class";
    auto close = line.indexOf(')', open);
    auto bases = close < 0 ? line[open .. $] : line[open .. close];
    if (bases.canFind("NamedTuple")) return "NamedTuple";
    if (bases.canFind("TypedDict")) return "TypedDict";
    if (bases.canFind("Enum")) return "enum";
    if (bases.canFind("ABC")) return "abstract";
    if (bases.canFind("Protocol")) return "Protocol";
    return "class";
}

/// The decorators of the Python function that starts at line `start` (1-based; its first decorator when it has any), as
/// the Go finder lists them: each decorator statement from its `@` line to the line before the next statement, trimmed.
string[] pythonDecorators(const(string)[] lines, int start, ref const Cleaned c) {
    string[] result;
    if (start < 1 || start > cast(int) lines.length || !strip(lines[start - 1]).startsWith("@")) return result;
    // Statement starts from `start` on: a line outside brackets and strings that holds code.
    int depth;
    size_t[] starts;
    size_t n = start - 1;
    for (; n < lines.length; ++n) {
        if (depth == 0 && pythonCode(c, n)) {
            auto t = strip(c.lines[n]);
            if (!t.startsWith("@")) break; // the def (or class): the statement after the decorators
            starts ~= n;
        }
        foreach (ch; c.lines[n]) {
            if (ch == '(' || ch == '[' || ch == '{') ++depth;
            else if (ch == ')' || ch == ']' || ch == '}') --depth;
        }
        if (depth < 0) depth = 0;
    }
    foreach (k, s; starts) {
        immutable stop = k + 1 < starts.length ? starts[k + 1] : n;
        result ~= strip(lines[s .. stop].join("\n")).idup;
    }
    return result;
}

unittest {
    assert(forStdRegex(`[\w<>[\]]*`) == `[\w<>\[\]]*`);
    assert(forStdRegex(`[^[]x`) == `[^\[]x`);
    assert(forStdRegex(`[]a]`) == `[\]a]`);
    assert(forStdRegex(`a\[b`) == `a\[b`);
    // every configured field pattern compiles
    import std.traits : EnumMembers;
    foreach (l; EnumMembers!Lang) fieldSpecFor(l);
}

unittest {
    auto c = cleanLines(["x = \"a // b\" // c", "/* open", "still */ y", "`raw ` z"], Lang.go);
    assert(c.lines[0] == "x =              ");
    assert(strip(c.lines[1]).length == 0 && c.startsInside[2]);
    assert(strip(c.lines[2]) == "y");
    assert(c.lines[3] == "       z");
}

unittest {
    auto lines = ["@dataclass(frozen=True)", "class P:", "    x: int = 0", "    y: List[int]", "    def f(self): pass"];
    auto c = cleanLines(lines, Lang.py);
    assert(pythonKind(lines, 2, c) == "dataclass");
    auto f = pythonFields(2, 6, c);
    assert(f.length == 2 && f[0].name == "x" && f[0].type == "int" && f[0].line == 3 && f[1].name == "y" && f[1].type == "List[int]");
    auto lines2 = ["class E(Enum):", "    A = 1"];
    auto c2 = cleanLines(lines2, Lang.py);
    assert(pythonKind(lines2, 1, c2) == "enum");
    auto dl = ["@a", "@b(", "    1)", "# note", "def f():", "    pass"];
    auto dc = cleanLines(dl, Lang.py);
    assert(pythonDecorators(dl, 1, dc) == ["@a", "@b(\n    1)\n# note"]);
}

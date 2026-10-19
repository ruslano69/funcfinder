module ffd_hybrid;

import ffd_model;
import js_ts_cleaner : LineFact;
import std.json;
import std.uni : unicode, CodepointSet;
import std.utf : decode, UTFException;

// The types of a JavaScript or TypeScript file as the Go tool's struct finder lists them (internal/struct_finder_factory.go:
// HybridStructFinder.findAllTypes), without a regex engine. The D finders list classes and interfaces; Go's directory run
// adds the struct finder's types to them: enums, type aliases (`type X = ...`) and, for JavaScript, `const X = {` objects.
//
// The four patterns of each language (languages.json: struct_type_patterns, resolved by the Go package) are matched by hand:
//
//   class       ^\s*(?:export\s+)?(?:abstract\s+)?class\s+(NAME)
//   enum (ts)   ^\s*(?:export\s+)?(?:const\s+)?enum\s+(NAME)
//   enum (js)   ^\s*(?:export\s+)?const\s+(NAME)\s*=\s*\{
//   interface   ^\s*(?:export\s+)?interface\s+(NAME)
//   type_alias  ^\s*(?:export\s+)?type\s+(NAME)\s*=
//
// where NAME is [\p{L}\p{Nd}_]+ (for a JavaScript class [\p{L}_$#][\p{L}\p{Nd}\p{M}_$\x{200c}\x{200d}]*). On one line the
// first kind in sorted order wins; a type that opens a brace stays open until the braces balance, and no type starts
// while one is open. checkStructPatterns refuses a configuration whose patterns are not these.

private __gshared CodepointSet letters, digits, marks;
shared static this() {
    letters = unicode.L;
    digits = unicode.Nd;
    marks = unicode.M;
}

private bool isWs(char c) { return c == ' ' || c == '\t' || c == '\n' || c == '\f' || c == '\r'; }

private size_t skipWs(string s, size_t i) {
    while (i < s.length && isWs(s[i])) ++i;
    return i;
}

// The keyword, then at least one space: moves i after them.
private bool keyword(string s, ref size_t i, string word) {
    if (s.length - i < word.length || s[i .. i + word.length] != word) return false;
    immutable j = i + word.length;
    if (j >= s.length || !isWs(s[j])) return false;
    i = skipWs(s, j);
    return true;
}

private dchar runeAt(string s, ref size_t i) {
    if (s[i] < 0x80) return s[i++];
    immutable start = i;
    try {
        return decode(s, i);
    } catch (UTFException) {
        i = start + 1;
        return 0xFFFD;
    }
}

private bool identChar(dchar c) {
    if (c < 0x80) return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_';
    return letters[c] || digits[c];
}

private bool jsClassStart(dchar c) {
    if (c < 0x80) return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' || c == '$' || c == '#';
    return letters[c];
}

private bool jsClassPart(dchar c) {
    if (c < 0x80) return identChar(c) || c == '$';
    return letters[c] || digits[c] || marks[c] || c == 0x200c || c == 0x200d;
}

// The end of the name that starts at i, or i when there is none.
private size_t nameEnd(string s, size_t i, bool jsClass) {
    size_t j = i;
    bool first = true;
    while (j < s.length) {
        size_t k = j;
        immutable c = runeAt(s, k);
        immutable ok = jsClass ? (first ? jsClassStart(c) : jsClassPart(c)) : identChar(c);
        if (!ok) break;
        j = k;
        first = false;
    }
    return j;
}

private enum Kind : ubyte { class_, enum_, interface_, typeAlias }

// Kinds in the order Go tries them: sorted by name (class, enum, interface, type_alias).
private immutable Kind[4] kindOrder = [Kind.class_, Kind.enum_, Kind.interface_, Kind.typeAlias];

private bool matchKind(Kind kind, string s, bool js, out string name) {
    size_t i = skipWs(s, 0);
    keyword(s, i, "export");
    bool jsClass;
    final switch (kind) {
        case Kind.class_:
            keyword(s, i, "abstract");
            if (!keyword(s, i, "class")) return false;
            jsClass = js;
            break;
        case Kind.enum_:
            if (js) {
                if (!keyword(s, i, "const")) return false;
            } else {
                keyword(s, i, "const");
                if (!keyword(s, i, "enum")) return false;
            }
            break;
        case Kind.interface_:
            if (!keyword(s, i, "interface")) return false;
            break;
        case Kind.typeAlias:
            if (!keyword(s, i, "type")) return false;
            break;
    }
    immutable end = nameEnd(s, i, jsClass);
    if (end == i) return false;
    name = s[i .. end];
    i = end;
    if (kind == Kind.typeAlias) {
        i = skipWs(s, i);
        return i < s.length && s[i] == '=';
    }
    if (kind == Kind.enum_ && js) {
        i = skipWs(s, i);
        if (i >= s.length || s[i] != '=') return false;
        i = skipWs(s, i + 1);
        return i < s.length && s[i] == '{';
    }
    return true;
}

private int countBraces(string s) {
    int n;
    foreach (c; s) {
        if (c == '{') ++n;
        else if (c == '}') --n;
    }
    return n;
}

private bool containsAny(string s, const(string)[] words) {
    foreach (w; words)
        for (size_t i = 0; i + w.length <= s.length; ++i)
            if (s[i .. i + w.length] == w) return true;
    return false;
}

/// The struct finder's types of a JavaScript (js) or TypeScript file, in the order the Go finder lists them. Names are
/// copied: the cleaned lines are reused for the next file.
Sym[] hybridTypes(const(LineFact)[] facts, bool js) {
    static immutable string[] tsWords = ["class", "interface", "enum", "type"];
    static immutable string[] jsWords = ["class", "interface", "const", "type"];
    immutable words = js ? jsWords : tsWords;
    Sym[] types;
    Sym current;
    bool open;
    int depth;
    foreach (n, ref fact; facts) {
        immutable line = fact.cleaned;
        if (open) {
            immutable previous = depth;
            depth += countBraces(line);
            if (depth == 0 && previous > 0) {
                current.end = cast(int) n + 1;
                types ~= current;
                open = false;
            }
        } else if (containsAny(line, words)) {
            foreach (kind; kindOrder) {
                string name;
                if (!matchKind(kind, line, js, name)) continue;
                current = Sym(name.idup, cast(int) n + 1, 0, "");
                immutable braces = countBraces(line);
                if (braces > 0) {
                    depth = braces;
                    open = true;
                } else {
                    // A construct with no brace (`type Handler = (req: Request) => void;`) ends on its line.
                    current.end = cast(int) n + 1;
                    types ~= current;
                }
                break;
            }
        }
    }
    if (open) {
        current.end = cast(int) facts.length;
        types ~= current;
    }
    return types;
}

// The patterns this file stands for, as the Go package resolves them.
private immutable string[2][] tsPatterns = [
    ["class", `^\s*(?:export\s+)?(?:abstract\s+)?class\s+([\p{L}\p{Nd}_]+)`],
    ["enum", `^\s*(?:export\s+)?(?:const\s+)?enum\s+([\p{L}\p{Nd}_]+)`],
    ["interface", `^\s*(?:export\s+)?interface\s+([\p{L}\p{Nd}_]+)`],
    ["type_alias", `^\s*(?:export\s+)?type\s+([\p{L}\p{Nd}_]+)\s*=`],
];
private immutable string[2][] jsPatterns = [
    ["class", `^\s*(?:export\s+)?(?:abstract\s+)?class\s+([\p{L}_$#][\p{L}\p{Nd}\p{M}_$\x{200c}\x{200d}]*)`],
    ["enum", `^\s*(?:export\s+)?const\s+([\p{L}\p{Nd}_]+)\s*=\s*\{`],
    ["interface", `^\s*(?:export\s+)?interface\s+([\p{L}\p{Nd}_]+)`],
    ["type_alias", `^\s*(?:export\s+)?type\s+([\p{L}\p{Nd}_]+)\s*=`],
];

/// Throws when the language's struct patterns or type keywords are not the ones hybridTypes was written for.
void checkStructPatterns(bool js, JSONValue cfg) {
    immutable lang = js ? "js" : "ts";
    immutable expected = js ? jsPatterns : tsPatterns;
    auto actual = cfg["struct_patterns"].object;
    if (actual.length != expected.length) throw new Exception(lang ~ ": the struct patterns changed: update ffd_hybrid.d");
    foreach (pair; expected) {
        auto p = pair[0] in actual;
        if (p is null || p.str != pair[1])
            throw new Exception(lang ~ ": the struct pattern '" ~ pair[0] ~ "' differs from the one ffd_hybrid.d implements");
    }
    string[] words;
    foreach (w; cfg["type_keywords"].array) words ~= w.str;
    immutable want = js ? ["class", "interface", "const", "type"] : ["class", "interface", "enum", "type"];
    if (words != want) throw new Exception(lang ~ ": the type_keywords changed: update ffd_hybrid.d");
}

unittest {
    LineFact[] make(string[] lines) {
        LineFact[] fs;
        foreach (l; lines) {
            LineFact f;
            f.cleaned = l;
            fs ~= f;
        }
        return fs;
    }
    auto t = hybridTypes(make([
        "export class A extends B {",
        "  type inner = 1;",
        "}",
        "type Handler = (req: Request) => void;",
        "export const enum Color { Red, Green }",
        "enum E {",
        "  X,",
        "}",
        "export interface I<T> {",
        "}",
        "  type Spaced   = string",
        "type NotAlias",
        "declare abstract class C {}",
    ]), false);
    string[] names;
    foreach (x; t) names ~= x.name;
    // `declare abstract class C {}` is not a type for the Go patterns: none allows `declare`.
    assert(names == ["A", "Handler", "Color", "E", "I", "Spaced"], "got " ~ names.to!string);
    assert(t[0].start == 1 && t[0].end == 3);
    assert(t[1].start == 4 && t[1].end == 4);
    assert(t[3].start == 6 && t[3].end == 8);
    auto j = hybridTypes(make([
        "export const Colors = {",
        "  red: 1,",
        "};",
        "const plain = 3;",
        "class #P {}",
        "class $Dollar {}",
    ]), true);
    string[] jnames;
    foreach (x; j) jnames ~= x.name;
    assert(jnames == ["Colors", "#P", "$Dollar"], "got " ~ jnames.to!string);
}

import std.conv : to;

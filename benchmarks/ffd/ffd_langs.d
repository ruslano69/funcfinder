module ffd_langs;

import ffd_goalias;
import ffd_hybrid;
import ffd_model;
import std.conv : to;
import std.exception : assumeUnique;
import std.stdio : File;
import std.json;
import std.traits : EnumMembers;
import std.utf : validate;

import bcl = brace_port_cleaner;
import bm = brace_port_model;
import dp = dlang_port;
import jcl = js_ts_cleaner;
import jm = js_ts_model;
import brace_port_unicode : unicodeVersion;
import c_port : findC;
import cpp_tokens_port : findCppTokens;
import csharp_port : findCSharp;
import go_port : findGo;
import java_port : findJava;
import javascript_port : findJavaScript;
import kotlin_port : findKotlin;
import php_port : findPHP;
import python_port : findPython;
import ruby_port : findRuby;
import rust_port : findRust;
import scala_port : findScala;
import swift_port : findSwift;
import typescript_port : findTypeScript;
import typescript_port;

// The language registry: which finder reads which file, and the configuration each finder was written against.
//
// ffd_config.json (generated from languages.json by benchmarks/ffd/genconfig, so that the D finders see Go's resolved
// patterns) is compiled into the executable. The finders carry their own frozen assumptions about the lexical
// settings; checkConfig refuses a configuration that departs from them rather than parse with the wrong rules.

/// The language keys of languages.json, in sorted order (the order Go visits them in).
enum Lang : ubyte { c, cpp, cs, d, go, java, js, kotlin, php, py, ruby, rust, scala, swift, ts }

enum configText = import("ffd_config.json");

string langKey(Lang l) {
    static immutable string[] keys = ["c", "cpp", "cs", "d", "go", "java", "js", "kotlin", "php", "py", "ruby", "rust", "scala", "swift", "ts"];
    return keys[l];
}

/// The extension -> language table, filled once by initLanguages before any thread reads it.
__gshared Lang[string] extToLang;

private string[] getStrings(JSONValue cfg, string key) {
    string[] r;
    if (auto p = key in cfg.object)
        if (p.type == JSONType.array)
            foreach (v; p.array) r ~= v.str;
    return r;
}

private string getString(JSONValue cfg, string key) {
    if (auto p = key in cfg.object) return p.type == JSONType.string ? p.str : "";
    return "";
}

private bool getFlag(JSONValue cfg, string key) {
    if (auto p = key in cfg.object) return p.type == JSONType.true_;
    return false;
}

private struct Expect {
    string lineComment, blockStart, blockEnd;
    string[] strings, raw, chars, docStrings;
    bool charLookahead, percent, regex, template_, singleLine;
}

private Expect expectation(Lang l) {
    enum slashes = Expect("//", "/*", "*/");
    final switch (l) {
        case Lang.c: { auto e = slashes; e.strings = ["\"", "'"]; return e; }
        case Lang.cpp: { auto e = slashes; e.strings = ["\"", "'"]; return e; }
        case Lang.cs: { auto e = slashes; e.strings = ["\""]; e.raw = ["@\""]; e.chars = ["'"]; return e; }
        case Lang.d: { auto e = slashes; return e; }
        case Lang.go: { auto e = slashes; e.strings = ["\""]; e.raw = ["`"]; e.chars = ["'"]; return e; }
        case Lang.java: { auto e = slashes; e.strings = ["\""]; e.chars = ["'"]; return e; }
        case Lang.js:
        case Lang.ts: { auto e = slashes; e.strings = ["\"", "'"]; e.raw = ["`"]; e.regex = e.template_ = e.singleLine = true; return e; }
        case Lang.kotlin: { auto e = slashes; e.strings = ["\""]; e.raw = ["\"\"\""]; e.chars = ["'"]; return e; }
        case Lang.php: { auto e = slashes; e.strings = ["\"", "'"]; return e; }
        case Lang.py: { auto e = Expect("#", "", ""); e.strings = ["\"", "'"]; e.raw = ["r\"", "r'"]; e.docStrings = ["\"\"\"", "'''"]; return e; }
        case Lang.ruby: { auto e = Expect("#", "=begin", "=end"); e.strings = ["\"", "'"]; e.percent = e.regex = true; return e; }
        case Lang.rust: { auto e = slashes; e.strings = ["\""]; e.chars = ["'"]; e.charLookahead = true;
                          e.raw = ["r\"", "r#\"", "r##\"", "r###\"", "r####\""]; return e; }
        case Lang.scala: { auto e = slashes; e.strings = ["\""]; e.raw = ["\"\"\""]; return e; }
        case Lang.swift: { auto e = slashes; e.strings = ["\""]; e.raw = ["\"\"\".*?\"\"\""]; return e; }
    }
}

/// Throws when cfg is not the language's configuration or differs from what its finder assumes (the lexical rules the
/// finders were measured with: comment markers, string and character delimiters, a few mode flags).
void checkConfig(Lang l, JSONValue cfg) {
    immutable key = langKey(l);
    if (cfg.getString("lang") != key) throw new Exception(key ~ ": the configuration names another language");
    if (cfg.getString("unicode_version") != unicodeVersion)
        throw new Exception(key ~ ": the configuration was made for another Unicode version");
    if (l == Lang.js || l == Lang.ts) checkStructPatterns(l == Lang.js, cfg);
    if (l == Lang.d) return; // dlang_port.configure checks the D patterns itself
    const e = expectation(l);
    void need(bool ok, string what) {
        if (!ok) throw new Exception(key ~ ": unsupported configuration: " ~ what);
    }
    need(cfg.getString("line_comment") == e.lineComment, "line_comment");
    need(cfg.getString("block_comment_start") == e.blockStart, "block_comment_start");
    need(cfg.getString("block_comment_end") == e.blockEnd, "block_comment_end");
    need(cfg.getString("escape_char") == "\\", "escape_char");
    need(cfg.getStrings("string_chars") == e.strings, "string_chars");
    need(cfg.getStrings("raw_string_chars") == e.raw, "raw_string_chars");
    need(cfg.getStrings("char_delimiters") == e.chars, "char_delimiters");
    need(cfg.getFlag("char_literal_lookahead") == e.charLookahead, "char_literal_lookahead");
    need(cfg.getFlag("percent_literals") == e.percent, "percent_literals");
    need(cfg.getFlag("regex_literals") == e.regex, "regex_literals");
    need(cfg.getFlag("template_expressions") == e.template_, "template_expressions");
    need(cfg.getFlag("single_line_strings") == e.singleLine, "single_line_strings");
    need(cfg.getStrings("doc_string_markers") == e.docStrings, "doc_string_markers");
}

/// Reads the embedded configuration, checks every language and fills extToLang. Call once, before any worker runs.
void initLanguages() {
    auto root = parseJSON(configText);
    if (root["unicode_version"].str != unicodeVersion)
        throw new Exception("the configuration was made for another Unicode version");
    foreach (l; EnumMembers!Lang) {
        auto cfg = root["languages"][langKey(l)];
        checkConfig(l, cfg);
        foreach (e; cfg["extensions"].array) {
            immutable ext = e.str;
            // Only ".h" is claimed twice (C and C++); the C++ configuration gets it.
            if (auto p = ext in extToLang) {
                if (l == Lang.cpp) *p = l;
            } else
                extToLang[ext] = l;
        }
    }
}

/// The language whose key is `key` (--source), or false.
bool languageFromKey(string key, out Lang lang) {
    foreach (l; EnumMembers!Lang)
        if (langKey(l) == key) {
            lang = l;
            return true;
        }
    return false;
}

/// The comment markers of a language: line comment, block comment start and end ("" when it has none).
void commentMarkers(Lang l, out string line, out string blockStart, out string blockEnd) {
    const e = expectation(l);
    line = e.lineComment;
    blockStart = e.blockStart;
    blockEnd = e.blockEnd;
}

/// The language of a file extension (with its dot, case-sensitive), or false.
bool languageOf(string ext, out Lang lang) {
    if (auto p = ext in extToLang) {
        lang = *p;
        return true;
    }
    return false;
}

// --- shared state --------------------------------------------------------------------------------------------------------
// The configurations are built once, on the calling thread, before any worker starts, and only read afterwards: a
// compiled std.regex Regex holds no mutable state (each match allocates its own matcher), and building them per worker
// thread cost more than the parsing of a small tree (every thread compiled the same patterns, all through the one GC
// lock). The finders keep their own patterns in __gshared variables, set by the same call.

private __gshared bm.Config[Lang.max + 1] braceConfigs;
private __gshared jm.Config[Lang.max + 1] jsConfigs;

/// Builds what the finders of the given languages read: call it once, before processFiles.
void prepareLanguages(const(Lang)[] used) {
    auto root = parseJSON(configText);
    bool[Lang.max + 1] done;
    foreach (l; used) {
        if (done[l]) continue;
        done[l] = true;
        auto cfg = root["languages"][langKey(l)];
        switch (l) {
            case Lang.cpp:
                // Patterns are compiled only for what a token finder reads; C++ reads none of them in a normal run.
                braceConfigs[l] = bm.Config(cfg, false);
                break;
            case Lang.js:
                jsConfigs[l] = jm.Config(cfg);
                break;
            case Lang.ts:
                jsConfigs[l] = jm.Config(cfg);
                typescript_port.setup();
                break;
            case Lang.d:
                dp.configure(cfg);
                break;
            default:
                break;
        }
    }
}

// --- reading and finding ------------------------------------------------------------------------------------------------

/// True when no byte has its high bit set, i.e. the text is ASCII and so valid UTF-8; eight bytes at a time.
private bool allAscii(const(char)[] s) {
    size_t i = 0;
    for (; i + 8 <= s.length; i += 8)
        if (*cast(const(ulong)*) (s.ptr + i) & 0x8080808080808080UL) return false;
    for (; i < s.length; ++i)
        if (s[i] >= 0x80) return false;
    return true;
}

/// The bytes of the previous file read on this thread, kept for the next one (reuse.d).
private char[] sourcePool;

/// The file as a string: Go decodes every malformed byte as U+FFFD, so only malformed input is rewritten. Source code
/// is mostly ASCII, which needs no validation. The text lives in a block that the next call on the same thread
/// overwrites: nothing the finders return may point into it (toSyms copies the names).
string readSource(string path) {
    auto file = File(path, "rb");
    scope (exit) file.close();
    immutable size = file.size;
    if (sourcePool.length < size) sourcePool = new char[cast(size_t) size + cast(size_t) size / 2 + 4096];
    string source = cast(string) file.rawRead(sourcePool[0 .. cast(size_t) size]);
    if (allAscii(source)) return source;
    try {
        validate(source);
    } catch (Exception) {
        source = to!string(bcl.runes(source));
    }
    return source;
}

/// The array of lines of the previous file on this thread, kept for the next one (reuse.d): the lines of a file are a
/// slice of it, so a thread allocates this array only when a file has more lines than any before it.
private string[] linePool;

/// Lines as the Go reader splits them: on LF, no empty last line, a CR before the LF dropped. The result is valid
/// until the next call on the same thread.
string[] splitLines(string source) {
    import core.stdc.string : memchr;
    size_t count = 0;
    for (const(char)* p = source.ptr, end = source.ptr + source.length; p < end;) {
        p = cast(const(char)*) memchr(p, '\n', end - p);
        if (p is null) break;
        ++count;
        ++p;
    }
    immutable total = count + (source.length && source[$ - 1] != '\n' ? 1 : 0);
    if (linePool.length < total) linePool = new string[total + total / 2 + 16];
    auto lines = linePool[0 .. total];
    size_t start = 0, n = 0;
    foreach (i; 0 .. source.length) {
        if (source[i] == '\n') {
            lines[n++] = source[start .. i - (i > start && source[i - 1] == '\r' ? 1 : 0)];
            start = i + 1;
        }
    }
    if (start < source.length) {
        auto last = source[start .. $];
        lines[n++] = last.length && last[$ - 1] == '\r' ? last[0 .. $ - 1] : last;
    }
    return lines;
}

private Sym[] toSyms(B)(const(B)[] bounds) {
    // The names are slices of the file's text and of its cleaned lines, which are reused for the next file: copy them.
    auto o = new Sym[bounds.length];
    foreach (i, ref b; bounds) o[i] = Sym(b.name.idup, b.start, b.end, b.className.idup);
    return o;
}

private void take(R)(ref FileResult r, R found) {
    r.functions = toSyms(found.functions);
    r.classes = toSyms(found.classes);
}

/// Set by the modes that print the kind of a type: each file then also names the kind of its types.
__gshared bool wantKinds;

private bool isWordChar(char c) {
    return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_';
}

/// The kind of the type declared on `line`: the first of class, struct, interface, enum, union, trait, protocol, object,
/// module and record among its words; else "type_alias" (JavaScript, TypeScript), "named" (Go) or "type" for a `type`
/// declaration; else "class". The D finders do not keep a kind, so it is read off the declaration line; the Go tool takes
/// it from the pattern that matched, so the two agree on the usual declarations and may differ on unusual ones
/// (data/sealed class, dataclass, ...). Kinds are not part of the key contract.
string kindOfDeclaration(string line, Lang lang) {
    static immutable strong = ["class", "struct", "interface", "enum", "union", "trait", "protocol", "object", "module", "record"];
    bool sawType;
    int index;
    size_t i = 0;
    while (i < line.length) {
        if (!isWordChar(line[i])) { ++i; continue; }
        size_t j = i;
        while (j < line.length && isWordChar(line[j])) ++j;
        auto word = line[i .. j];
        i = j;
        // Go: `type Name struct` and `type Name interface` name their kind right after the name; any other `type Name ...`
        // (a map, a function type, a defined type) is "named", whatever words its definition holds.
        if (lang == Lang.go && sawType) {
            if (++index == 2) return word == "struct" ? "struct" : word == "interface" ? "interface" : "named"; // literals: `word` is a slice of the reused source
            continue;
        }
        foreach (k; strong) if (word == k) return k;
        if (word == "type") sawType = true;
    }
    if (sawType) return lang == Lang.go ? "named" : lang == Lang.js || lang == Lang.ts ? "type_alias" : "type";
    return "class";
}

private void fillKinds(ref FileResult r, string source, Lang lang) {
    auto lines = splitLines(source);
    foreach (ref c; r.classes) {
        c.kind = c.start >= 1 && c.start <= lines.length ? kindOfDeclaration(lines[c.start - 1], lang) : "class";
    }
    foreach (ref c; r.structTypes)
        c.kind = c.start >= 1 && c.start <= lines.length ? kindOfDeclaration(lines[c.start - 1], lang) : "class";
}

/// Finds the functions and types of one file with the finder of its language. Errors are returned in the result.
FileResult analyzeFile(string path, Lang lang) {
    FileResult r;
    r.path = path;
    r.lang = langKey(lang);
    try {
        auto source = readSource(path);
        final switch (lang) {
            case Lang.kotlin: take(r, findKotlin(source)); break;
            case Lang.php: take(r, findPHP(source)); break;
            case Lang.py: take(r, findPython(source, true)); break;
            case Lang.ruby: take(r, findRuby(source)); break;
            case Lang.rust: take(r, findRust(source)); break;
            case Lang.scala: take(r, findScala(source)); break;
            case Lang.swift: take(r, findSwift(source)); break;
            case Lang.c:
            case Lang.cpp:
            case Lang.cs:
            case Lang.go:
            case Lang.java: {
                auto lines = splitLines(source);
                bcl.Cleaner cleaner;
                cleaner.csharp = lang == Lang.cs;
                cleaner.golang = lang == Lang.go;
                cleaner.charLiterals = lang == Lang.go || lang == Lang.java;
                auto facts = cleaner.facts(lines);
                final switch (lang) {
                    case Lang.c: take(r, findC(lines, facts)); break;
                    case Lang.cs: take(r, findCSharp(facts)); break;
                    case Lang.go:
                        take(r, findGo(facts));
                        r.aliases = findGoAliases(facts);
                        foreach (ref a; r.aliases) {
                            a.name = a.name.idup;
                            a.target = a.target.idup;
                        }
                        r.classes = withoutAliases(r.classes, r.aliases);
                        break;
                    case Lang.java: take(r, findJava(facts)); break;
                    case Lang.cpp: {
                        auto found = findCppTokens(braceConfigs[Lang.cpp], lines, facts);
                        // Go starts a function on the line of its header proper, after a template prefix; the token
                        // finder starts it at the prefix (the declaration, as libclang counts it).
                        foreach (ref f; found.functions)
                            if (f.signatureLine > f.start) f.start = f.signatureLine;
                        take(r, found);
                        break;
                    }
                    case Lang.d, Lang.js, Lang.kotlin, Lang.php, Lang.py, Lang.ruby, Lang.rust, Lang.scala, Lang.swift, Lang.ts:
                        assert(0);
                }
                break;
            }
            case Lang.js:
            case Lang.ts: {
                auto lines = splitLines(source);
                jcl.Cleaner cleaner;
                auto facts = cleaner.facts(lines);
                if (lang == Lang.js) take(r, findJavaScript(jsConfigs[Lang.js], lines, facts));
                else take(r, findTypeScript(jsConfigs[Lang.ts], lines, facts));
                r.structTypes = hybridTypes(facts, lang == Lang.js);
                r.hasStructTypes = true;
                break;
            }
            case Lang.d: {
                take(r, dp.find(splitLines(source)));
                break;
            }
        }
        if (wantKinds) fillKinds(r, source, lang);
    } catch (Exception e) {
        r.functions = null;
        r.classes = null;
        r.aliases = null;
        r.structTypes = null;
        r.error = e.msg;
    }
    return r;
}

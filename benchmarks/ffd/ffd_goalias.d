module ffd_goalias;

import brace_port_cleaner : LineFact;
import ffd_model;

// Go type aliases (`type A = S`, and the `A = S` lines of a `type ( ... )` group), read from cleaned lines as the Go
// tool reads them (internal/go_aliases.go: findGoAliases) and listed apart from the types: an alias names an existing
// type and is not a type of its own. The Go patterns, which this follows without a regex engine, are
//
//   decl   ^\s*type\s+(\w+)\s*=\s*(.*?)\s*(?://.*)?$        member ^\s*(\w+)\s*=\s*(.*?)\s*(?://.*)?$
//   open   ^\s*type\s*\(\s*$                                close  ^\s*\)\s*$
//
// with RE2's \s ([\t\n\f\r ]) and \w ([0-9A-Za-z_]). The cleaned text has its comments blanked, so the optional `//`
// tail only matters for text the cleaner left alone; the target is cut at a "//" and trimmed either way.

private bool isSpace(char c) { return c == ' ' || c == '\t' || c == '\n' || c == '\f' || c == '\r'; }
private bool isWord(char c) { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'; }

private size_t skipSpace(string s, size_t i) {
    while (i < s.length && isSpace(s[i])) ++i;
    return i;
}

private bool restIsBlank(string s, size_t i) { return skipSpace(s, i) == s.length; }

// `\s*(\w+)\s*=\s*(.*?)\s*(?://.*)?$` from i: the name and the target, or false.
private bool nameEqualsTarget(string s, size_t i, out string name, out string target) {
    i = skipSpace(s, i);
    immutable start = i;
    while (i < s.length && isWord(s[i])) ++i;
    if (i == start) return false;
    name = s[start .. i];
    i = skipSpace(s, i);
    if (i >= s.length || s[i] != '=') return false;
    i = skipSpace(s, i + 1);
    size_t end = s.length;
    foreach (k; i .. (s.length > 0 ? s.length - 1 : 0))
        if (s[k] == '/' && s[k + 1] == '/') {
            end = k;
            break;
        }
    while (end > i && isSpace(s[end - 1])) --end;
    target = s[i .. end];
    return true;
}

// `\s*type\s+` from the start of s: the position after it, or -1.
private ptrdiff_t afterTypeKeyword(string s) {
    auto i = skipSpace(s, 0);
    if (i + 4 > s.length || s[i .. i + 4] != "type") return -1;
    i += 4;
    if (i >= s.length || !isSpace(s[i])) return -1;
    return skipSpace(s, i);
}

private bool isGroupOpen(string s) {
    auto i = skipSpace(s, 0);
    if (i + 4 > s.length || s[i .. i + 4] != "type") return false;
    i = skipSpace(s, i + 4);
    if (i >= s.length || s[i] != '(') return false;
    return restIsBlank(s, i + 1);
}

private bool isGroupClose(string s) {
    auto i = skipSpace(s, 0);
    if (i >= s.length || s[i] != ')') return false;
    return restIsBlank(s, i + 1);
}

/// The aliases of a Go file, in source order.
Alias[] findGoAliases(const(LineFact)[] facts) {
    Alias[] aliases;
    bool inGroup;
    foreach (i, ref fact; facts) {
        string line = fact.cleaned;
        string name, target;
        if (inGroup && isGroupClose(line)) {
            inGroup = false;
        } else if (inGroup) {
            if (nameEqualsTarget(line, 0, name, target)) aliases ~= Alias(name, cast(int) i + 1, target);
        } else if (isGroupOpen(line)) {
            inGroup = true;
        } else {
            immutable after = afterTypeKeyword(line);
            if (after >= 0 && nameEqualsTarget(line, after, name, target)) aliases ~= Alias(name, cast(int) i + 1, target);
        }
    }
    return aliases;
}

/// Removes from classes the entries that are aliases (same name and line): the Go finder never lists an alias as a type.
Sym[] withoutAliases(Sym[] classes, const(Alias)[] aliases) {
    if (aliases.length == 0) return classes;
    Sym[] kept;
    foreach (ref c; classes) {
        bool alias_;
        foreach (ref a; aliases)
            if (a.name == c.name && a.line == c.start) {
                alias_ = true;
                break;
            }
        if (!alias_) kept ~= c;
    }
    return kept;
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
    auto a = findGoAliases(make([
        "package p",
        "type A = S",
        "type B=map[string]int",
        "type C [4]int",
        "type (",
        "\tD = []byte",
        "\tE int",
        "\tF = struct {",
        ")",
        "x = 3",
        "type G  =  pkg.T   ",
        "type H = ",
    ]));
    assert(a.length == 6);
    assert(a[0] == Alias("A", 2, "S"));
    assert(a[1] == Alias("B", 3, "map[string]int"));
    assert(a[2] == Alias("D", 6, "[]byte"));
    assert(a[3] == Alias("F", 8, "struct {"));
    assert(a[4] == Alias("G", 11, "pkg.T"));
    assert(a[5] == Alias("H", 12, ""));
}

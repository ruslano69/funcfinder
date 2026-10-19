module ffd_ignore;

import std.array : split;
import std.file : exists, isFile, read;
import std.path : buildPath;
import std.string : startsWith, endsWith, strip;
import std.utf : decode, UTFException;

// The root .gitignore of a directory run, read as the Go tool reads it (internal/dirprocessor.go: IgnoreMatcher).
// The Go matcher turns each glob into a regular expression on slash-separated paths relative to the root; this one
// turns it into the same tokens and walks them over the path, so the decision is the same without a regex engine:
//   `*` and `?` stay within one path segment; `**/` at the start or after a "/" is any number of leading directories,
//   any other `**` is anything; a glob with a "/" in it (after the trailing one is removed) is anchored at the root,
//   one without matches at any depth; a match also covers what is inside a matched directory;
//   `!` lines are dropped (the Go tool drops them too), and only the root file is read.

private enum Kind : ubyte { lit, any1, star, dstarSlash, dstar }

private struct Tok {
    Kind kind;
    dchar cp;
}

private struct Pattern {
    Tok[] toks;
    bool anchored;
    bool directory;
    string literal; // a piece every matching path contains; checked before the walk over the tokens
}

/// The longest piece of a glob that every path it matches contains (internal/dirprocessor.go: globLiteral).
string globLiteral(string glob) {
    if (glob.startsWith("/")) glob = glob[1 .. $];
    // "**/" counts as a wildcard together with its slash: `**/build` also matches "build" at the root.
    string g;
    for (size_t i = 0; i < glob.length;) {
        if (i + 3 <= glob.length && glob[i .. i + 3] == "**/") {
            g ~= '*';
            i += 3;
        } else {
            g ~= glob[i];
            ++i;
        }
    }
    string best;
    size_t start = 0;
    foreach (i; 0 .. g.length + 1) {
        if (i == g.length || g[i] == '*' || g[i] == '?') {
            if (i - start > best.length) best = g[start .. i];
            start = i + 1;
        }
    }
    return best;
}

private size_t runeWidth(string s, size_t k) {
    immutable ubyte c = s[k];
    if (c < 0x80) return 1;
    immutable n = c >= 0xF0 ? 4 : c >= 0xE0 ? 3 : c >= 0xC0 ? 2 : 1;
    if (n == 1 || k + n > s.length) return 1;
    foreach (x; 1 .. n)
        if ((s[k + x] & 0xC0) != 0x80) return 1;
    return n;
}

private dchar runeAt(string s, size_t k, out size_t width) {
    width = runeWidth(s, k);
    if (width == 1) return s[k] < 0x80 ? s[k] : cast(dchar) 0xFFFD;
    size_t j = k;
    try {
        return decode(s, j);
    } catch (UTFException) {
        width = 1;
        return 0xFFFD;
    }
}

private Pattern compile(string glob, bool directory) {
    Pattern p;
    p.directory = directory;
    p.anchored = false;
    foreach (c; glob)
        if (c == '/') p.anchored = true;
    p.literal = globLiteral(glob);
    string s = glob.startsWith("/") ? glob[1 .. $] : glob;
    for (size_t i = 0; i < s.length;) {
        if (i + 3 <= s.length && s[i .. i + 3] == "**/" && (i == 0 || s[i - 1] == '/')) {
            p.toks ~= Tok(Kind.dstarSlash);
            i += 3;
        } else if (i + 2 <= s.length && s[i .. i + 2] == "**") {
            p.toks ~= Tok(Kind.dstar);
            i += 2;
        } else if (s[i] == '*') {
            p.toks ~= Tok(Kind.star);
            ++i;
        } else if (s[i] == '?') {
            p.toks ~= Tok(Kind.any1);
            ++i;
        } else {
            size_t w;
            immutable d = runeAt(s, i, w);
            p.toks ~= Tok(Kind.lit, d);
            i += w;
        }
    }
    return p;
}

/// The patterns of a root .gitignore, and the scratch the matching uses (one matcher serves one thread).
final class IgnoreMatcher {
    private Pattern[] patterns;
    private bool[] cur, next;

    private this() {}

    /// Reads root/.gitignore; with no such file nothing is ignored.
    this(string root) {
        immutable path = buildPath(root, ".gitignore");
        if (!exists(path) || !isFile(path)) return;
        string text;
        try {
            text = cast(string) read(path);
        } catch (Exception) {
            return;
        }
        parse(text);
    }

    /// For tests: patterns from text.
    static IgnoreMatcher fromText(string text) {
        auto m = new IgnoreMatcher();
        m.parse(text);
        return m;
    }

    private void parse(string content) {
        foreach (raw; content.split("\n")) {
            string line;
            try {
                line = raw.strip();
            } catch (UTFException) {
                continue;
            }
            // A comment is a line that starts with '#', as in git; a '#' elsewhere is part of the pattern.
            if (line.length == 0 || line[0] == '#') continue;
            bool negate = false;
            if (line[0] == '!') {
                negate = true;
                try {
                    line = line[1 .. $].strip();
                } catch (UTFException) {
                    continue;
                }
            }
            if (line.length == 0) continue;
            immutable isDir = line[$ - 1] == '/';
            if (isDir) line = line[0 .. $ - 1];
            if (negate) continue; // dropped, as the Go tool drops it
            patterns ~= compile(line, isDir);
        }
    }

    /// Whether the slash-separated path relative to the root is ignored (isDir: it names a directory).
    bool matches(string path, bool isDir) {
        foreach (ref p; patterns) {
            if (p.directory && !isDir) continue;
            if (p.literal.length && !contains(path, p.literal)) continue;
            if (matchPath(p, path)) return true;
        }
        return false;
    }

    @property size_t length() const { return patterns.length; }

    private static bool contains(string hay, string needle) {
        if (needle.length > hay.length) return false;
        foreach (i; 0 .. hay.length - needle.length + 1)
            if (hay[i .. i + needle.length] == needle) return true;
        return false;
    }

    // (^|/)body($|/) for a glob without a "/", ^body($|/) for one with it: the body must start at the beginning or after
    // a "/" and end at the end or before a "/".
    private bool matchPath(ref const Pattern p, string s) {
        if (cur.length < s.length + 1) {
            cur.length = s.length + 1;
            next.length = s.length + 1;
        }
        if (bodyMatches(p, s, 0)) return true;
        if (p.anchored) return false;
        foreach (k; 0 .. s.length)
            if (s[k] == '/' && bodyMatches(p, s, k + 1)) return true;
        return false;
    }

    // Walks the tokens over s from start, keeping the set of byte offsets reachable after each token.
    private bool bodyMatches(ref const Pattern p, string s, size_t start) {
        immutable n = s.length + 1;
        cur[0 .. n] = false;
        cur[start] = true;
        foreach (ref t; p.toks) {
            next[0 .. n] = false;
            bool any = false;
            foreach (pos; 0 .. n) {
                if (!cur[pos]) continue;
                final switch (t.kind) {
                    case Kind.lit:
                        if (pos < s.length) {
                            size_t w;
                            if (runeAt(s, pos, w) == t.cp) { next[pos + w] = true; any = true; }
                        }
                        break;
                    case Kind.any1:
                        if (pos < s.length && s[pos] != '/') { next[pos + runeWidth(s, pos)] = true; any = true; }
                        break;
                    case Kind.star: {
                        size_t k = pos;
                        next[k] = true;
                        while (k < s.length && s[k] != '/') {
                            k += runeWidth(s, k);
                            next[k] = true;
                        }
                        any = true;
                        break;
                    }
                    case Kind.dstar: {
                        size_t k = pos;
                        next[k] = true;
                        while (k < s.length) {
                            k += runeWidth(s, k);
                            next[k] = true;
                        }
                        any = true;
                        break;
                    }
                    case Kind.dstarSlash:
                        next[pos] = true;
                        foreach (k; pos .. s.length)
                            if (s[k] == '/') next[k + 1] = true;
                        any = true;
                        break;
                }
            }
            if (!any) return false;
            auto swap = cur;
            cur = next;
            next = swap;
        }
        foreach (pos; 0 .. n)
            if (cur[pos] && (pos == s.length || s[pos] == '/')) return true;
        return false;
    }
}

unittest {
    // Each case is (patterns, path, isDir, ignored); the answers are the ones git check-ignore and the Go matcher
    // (TestIgnoreMatcher_GitSemantics) give.
    struct Case { string patterns; string path; bool isDir; bool want; }
    static immutable Case[] cases = [
        Case("build", "build", true, true),
        Case("build", "src/build", true, true),
        Case("build", "src/build/out.o", false, true),
        Case("build", "rebuild", true, false),
        Case("build/", "build", true, true),
        Case("build/", "build", false, false),
        Case("/build", "build", true, true),
        Case("/build", "src/build", true, false),
        Case("src/gen", "src/gen", true, true),
        Case("src/gen", "other/src/gen", true, false),
        Case("*.log", "a.log", false, true),
        Case("*.log", "dir/a.log", false, true),
        Case("*.log", "dir/a.logx", false, false),
        Case("a?c", "abc", false, true),
        Case("a?c", "a/c", false, false),
        Case("**/cache", "cache", true, true),
        Case("**/cache", "a/b/cache", true, true),
        Case("a/**", "a/b", false, true),
        Case("a/**/z", "a/z", false, true),
        Case("a/**/z", "a/b/c/z", false, true),
        Case("a/**/z", "a/b/c/y", false, false),
        Case("docs/*.md", "docs/x.md", false, true),
        Case("docs/*.md", "docs/sub/x.md", false, false),
        Case("# comment\n\n  trimmed  \r\nx", "trimmed", false, true),
        Case("# comment\n\n  trimmed  \r\nx", "x", false, true),
        Case("a#b", "a#b", false, true),
        Case("!keep\nkeep", "keep", false, true),
        Case("é*", "été", false, true),
        Case("é*", "ete", false, false),
        Case("[ab]", "a", false, false),
        Case("[ab]", "[ab]", false, true),
        Case("*", ".", true, true),
    ];
    foreach (c; cases) {
        auto m = IgnoreMatcher.fromText(c.patterns);
        assert(m.matches(c.path, c.isDir) == c.want, "pattern " ~ c.patterns ~ " on " ~ c.path);
    }
    assert(globLiteral("/a*bc?d") == "bc");
    assert(globLiteral("**/build") == "build");
    assert(globLiteral("*") == "");
}

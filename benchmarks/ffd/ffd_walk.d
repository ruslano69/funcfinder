module ffd_walk;

import ffd_ignore;
import ffd_langs;
import std.algorithm : sort;
import std.array : array, appender;
import std.conv : to;
import std.file : DirEntry, dirEntries, SpanMode;
import std.path : baseName, buildNormalizedPath, dirSeparator;
import std.string : endsWith, toLower;

// Directory discovery, in the order and with the rules of the Go tool's DirProcessor.collectFiles:
// the root .gitignore first, then hidden names (a leading "."; ".gitignore" itself is not hidden), then directories
// (not entered when the run is not recursive), then the extension. A file whose extension has no language is counted
// by extension and reported, not read. Symbolic links are not followed: a link is a file, as filepath.Walk's Lstat
// sees it.

/// One file to process, with the size the largest-first dispatch needs.
struct Job {
    string path;
    Lang lang;
    long size;
}

struct Collected {
    Job[] jobs;
    size_t[string] skipped; // lower-case extension (with its dot) -> files that no language claims
}

/// filepath.Ext: the final element from its last dot, "" when it has none (so ".gitignore" is all extension).
string fileExtension(string name) {
    for (size_t i = name.length; i > 0; --i)
        if (name[i - 1] == '.') return name[i - 1 .. $];
    return "";
}

private string joinPath(string dir, string name) {
    if (dir == ".") return name;
    return dir.endsWith(dirSeparator) ? dir ~ name : dir ~ dirSeparator ~ name;
}

private void walk(string dir, string rel, bool recursive, IgnoreMatcher matcher, ref Collected c) {
    DirEntry[] entries;
    try {
        entries = dirEntries(dir, SpanMode.shallow, false).array;
    } catch (Exception) {
        return; // a directory that cannot be read is skipped, as the Go walk skips it
    }
    foreach (ref e; entries) {
        immutable name = baseName(e.name);
        immutable relPath = rel == "." ? name : rel ~ "/" ~ name;
        immutable isDir = e.isDir && !e.isSymlink;
        if (matcher !is null && matcher.matches(relPath, isDir)) continue;
        if (name.length > 0 && name[0] == '.' && name != ".gitignore") continue;
        immutable path = joinPath(dir, name);
        if (isDir) {
            if (recursive) walk(path, relPath, recursive, matcher, c);
            continue;
        }
        immutable ext = fileExtension(name);
        Lang lang;
        if (languageOf(ext, lang)) {
            long size;
            try {
                size = cast(long) e.size;
            } catch (Exception) {
                continue;
            }
            c.jobs ~= Job(path, lang, size);
        } else
            c.skipped[toLower(ext)]++;
    }
}

/// The source files under root. Paths are cleaned as filepath.Join cleans them.
Collected collectFiles(string root, bool recursive, bool useGitignore) {
    Collected c;
    auto matcher = useGitignore ? new IgnoreMatcher(root) : null;
    if (matcher !is null && matcher.matches(".", true)) return c;
    walk(buildNormalizedPath(root), ".", recursive, matcher, c);
    return c;
}

/// The "skipped N files ..." line the Go tool prints for files no language claims, or "" when there were none:
/// the most files first, ties by extension, at most six kinds.
string skippedNotice(const size_t[string] skipped) {
    if (skipped.length == 0) return "";
    struct Kind { string ext; size_t n; }
    Kind[] kinds;
    size_t total;
    foreach (ext, n; skipped) {
        kinds ~= Kind(ext, n);
        total += n;
    }
    kinds.sort!((a, b) => a.n != b.n ? a.n > b.n : a.ext < b.ext);
    auto parts = appender!(string[])();
    foreach (i, k; kinds) {
        if (i == 6) {
            parts ~= "...";
            break;
        }
        parts ~= (k.ext.length ? k.ext : "(no extension)") ~ " " ~ k.n.to!string;
    }
    import std.array : join;
    return "skipped " ~ total.to!string ~ " files with no language for their extension: " ~ parts.data.join(", ");
}

unittest {
    assert(fileExtension("a.go") == ".go");
    assert(fileExtension("a.tar.gz") == ".gz");
    assert(fileExtension(".gitignore") == ".gitignore");
    assert(fileExtension("Makefile") == "");
    size_t[string] s = [".md": 5, ".json": 5, "": 1, ".txt": 2];
    assert(skippedNotice(s) == "skipped 13 files with no language for their extension: .json 5, .md 5, .txt 2, (no extension) 1");
}

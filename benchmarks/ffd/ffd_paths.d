module ffd_paths;

import core.int128 : Cent, mul, add;
import std.ascii : isAlpha;
import std.path : dirSeparator;
import std.stdio : File;
import std.string : toLower, join;
import std.format : format;
import std.algorithm : sort;

// Path rules of Go's path/filepath (Clean, Rel, Dir) for the places where ffd must print what the Go tool prints: the
// tree view shows paths relative to ".", and the shards of --split are named after the directory of a file relative to
// the root. The separator is the platform's; on Windows a "/" is read as one too, as Go does.

private enum sep = dirSeparator[0];

private bool isSep(char c) {
    version (Windows) return c == '\\' || c == '/';
    else return c == '/';
}

/// The length of the volume name at the start of p ("G:" on Windows, nothing elsewhere).
private size_t volumeLength(string p) {
    version (Windows) {
        if (p.length >= 2 && p[1] == ':' && isAlpha(p[0])) return 2;
    }
    return 0;
}

private bool sameWord(string a, string b) {
    version (Windows) return toLower(a) == toLower(b);
    else return a == b;
}

/// filepath.Clean: slashes made the platform's, "." and empty elements dropped, ".." resolved against the element before it.
string cleanPath(string path) {
    immutable vol = volumeLength(path);
    string rest = path[vol .. $];
    if (rest.length == 0) return vol ? path[0 .. vol] ~ "." : ".";
    immutable rooted = isSep(rest[0]);
    string[] out_;
    size_t i = 0;
    while (i < rest.length) {
        while (i < rest.length && isSep(rest[i])) ++i;
        size_t j = i;
        while (j < rest.length && !isSep(rest[j])) ++j;
        auto part = rest[i .. j];
        i = j;
        if (part.length == 0 || part == ".") continue;
        if (part == "..") {
            if (out_.length && out_[$ - 1] != "..") out_ = out_[0 .. $ - 1];
            else if (!rooted) out_ ~= part;
            continue;
        }
        out_ ~= part;
    }
    string body_ = out_.join(dirSeparator);
    if (rooted) body_ = dirSeparator ~ body_;
    else if (body_.length == 0) body_ = ".";
    return path[0 .. vol] ~ body_;
}

private string[] elements(string cleaned, size_t vol) {
    string rest = cleaned[vol .. $];
    string[] r;
    if (rest == ".") return r;
    size_t i = 0;
    while (i < rest.length) {
        while (i < rest.length && isSep(rest[i])) ++i;
        size_t j = i;
        while (j < rest.length && !isSep(rest[j])) ++j;
        if (j > i) r ~= rest[i .. j];
        i = j;
    }
    return r;
}

/// filepath.Rel(base, target): the path of target from base, or "" when there is none (different volumes, one rooted and
/// one not, or a base that climbs out of the common part).
string relativePath(string base, string target) {
    auto b = cleanPath(base), t = cleanPath(target);
    if (sameWord(b, t)) return ".";
    immutable bv = volumeLength(b), tv = volumeLength(t);
    immutable bRooted = b.length > bv && isSep(b[bv]), tRooted = t.length > tv && isSep(t[tv]);
    if (bRooted != tRooted || !sameWord(b[0 .. bv], t[0 .. tv])) return "";
    auto be = elements(b, bv), te = elements(t, tv);
    size_t n;
    while (n < be.length && n < te.length && sameWord(be[n], te[n])) ++n;
    foreach (e; be[n .. $]) if (e == "..") return "";
    string[] parts;
    foreach (_; be[n .. $]) parts ~= "..";
    parts ~= te[n .. $];
    return parts.length ? parts.join(dirSeparator) : ".";
}

/// filepath.Dir: everything before the last element, cleaned; "." when there is none.
string directoryOf(string path) {
    immutable vol = volumeLength(path);
    size_t i = path.length;
    while (i > vol && !isSep(path[i - 1])) --i;
    auto dir = cleanPath(path[0 .. i]);
    return dir;
}

/// filepath.Base: the last element, "." for an empty path, the separator for a path of separators only.
string baseOf(string path) {
    if (path.length == 0) return ".";
    size_t end = path.length;
    while (end > 0 && isSep(path[end - 1])) --end;
    if (end == 0) return dirSeparator;
    size_t start = end;
    while (start > 0 && !isSep(path[start - 1])) --start;
    path = path[start .. end];
    version (Windows) {
        // A bare volume name leaves the element empty after the volume.
        immutable v = volumeLength(path);
        if (v && path.length == v) return ".";
        if (v) path = path[v .. $];
    }
    return path.length ? path : ".";
}

// --- FNV-1a, 128 bits: the checksum of a shard (hash/fnv New128a, as the Go tool's default build computes it) ----------

/// The streaming FNV-1a 128-bit hash. The offset basis is 0x6c62272e07bb014262b821756295c58d and the prime 2^88 + 0x13b.
struct Fnv128a {
    private Cent h = Cent(0x62b821756295c58dUL, 0x6c62272e07bb0142UL);
    private enum Cent prime = Cent(0x000000000000013bUL, 0x0000000001000000UL);

    void put(const(ubyte)[] data) {
        foreach (b; data) {
            h.lo ^= b;
            h = mul(h, prime);
        }
    }

    void put(string s) { put(cast(const(ubyte)[]) s); }

    string hex() const {
        return format("%016x%016x", h.hi, h.lo);
    }
}

/// The hash of a file's bytes as 32 hex digits, or "" when it cannot be read.
string fileChecksum(string path, out bool ok) {
    Fnv128a h;
    try {
        auto f = File(path, "rb");
        scope (exit) f.close();
        ubyte[65536] buf;
        while (true) {
            auto got = f.rawRead(buf[]);
            if (got.length == 0) break;
            h.put(got);
        }
    } catch (Exception) {
        ok = false;
        return "";
    }
    ok = true;
    return h.hex();
}

/// The checksum of a set of files: their paths in order, each with its file's hash ("path:hash\n"; the path alone when the
/// file cannot be read), hashed once more.
string shardChecksum(string[] paths) {
    auto sorted = paths.dup;
    sorted.sort();
    Fnv128a h;
    foreach (p; sorted) {
        bool ok;
        auto sum = fileChecksum(p, ok);
        if (!ok) h.put(p);
        else h.put(p ~ ":" ~ sum ~ "\n");
    }
    return h.hex();
}

unittest {
    Fnv128a empty;
    assert(empty.hex() == "6c62272e07bb014262b821756295c58d");
    Fnv128a a;
    a.put("a");
    assert(a.hex() == "d228cb696f1a8caf78912b704e4a8964");
    Fnv128a foobar;
    foobar.put("foobar");
    assert(foobar.hex() == "343e1662793c64bf6f0d3597ba446f18");
}

unittest {
    version (Windows) {
        assert(cleanPath("a/b\\..\\c") == "a\\c");
        assert(cleanPath("G:\\x\\.\\y\\") == "G:\\x\\y");
        assert(cleanPath("") == ".");
        assert(cleanPath("..\\x") == "..\\x");
        assert(relativePath(".", "a\\b.go") == "a\\b.go");
        assert(relativePath(".", "G:\\x\\a.go") == "");
        assert(relativePath("G:\\x", "G:\\x\\a\\b.go") == "a\\b.go");
        assert(relativePath("G:\\x", "G:\\x") == ".");
        assert(relativePath("G:\\x\\y", "G:\\x\\a.go") == "..\\a.go");
        assert(directoryOf("a\\b.go") == "a");
        assert(directoryOf("b.go") == ".");
        assert(directoryOf("G:\\x\\b.go") == "G:\\x");
        assert(baseOf("G:\\x\\y.go") == "y.go");
    } else {
        assert(cleanPath("a//b/../c/") == "a/c");
        assert(cleanPath("/../x") == "/x");
        assert(relativePath(".", "a/b.go") == "a/b.go");
        assert(relativePath(".", "/x/a.go") == "");
        assert(relativePath("/x", "/x/a/b.go") == "a/b.go");
        assert(relativePath("/x/y", "/x/a.go") == "../a.go");
        assert(directoryOf("a/b.go") == "a");
        assert(directoryOf("b.go") == ".");
        assert(directoryOf("/b.go") == "/");
        assert(baseOf("/x/y.go") == "y.go");
    }
}

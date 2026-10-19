module ffd_split;

import ffd_json : formatJson, putString;
import ffd_model;
import ffd_paths;
import ffd_walk : Job;
import std.algorithm : sort, canFind;
import std.array : appender, join;
import std.conv : to;
import std.file : exists, isFile, mkdirRecurse, readText, remove, write;
import std.format : format;
import std.json;
import std.path : buildPath, dirSeparator;
import std.string : replace, strip;

// --split: the codemap of `funcfinder --dir --json --split` (internal/dirprocessor.go: WriteSplitOutput,
// WriteSplitOutputIncremental, ProcessDirectoryIncremental). Files are grouped into shards, by directory or by file; each
// shard with a declaration is written as a JSON file of the --dir --json shape, and manifest.json lists the shards with
// their counts and a checksum of every file of the shard (FNV-1a 128, as the Go tool's default build computes it, so an
// `--inc` run continues a manifest the Go tool wrote, and the other way round). The manifest lists shards in name order
// (the Go tool's map order is random). A shard without a declaration has no file; its checksum goes to "empty_shards".

struct ShardInfo {
    string path;
    long files, totalFunctions, totalClasses;
    string checksum;
    string[] dependsOn;
}

struct Manifest {
    string version_ = "1.0";
    string rootDir, splitBy;
    ShardInfo[] shards;
    long totalFiles, totalFunctions, totalClasses;
    string[string] emptyShards;
}

/// The key of the shard a file belongs to: its path relative to the root ("file"), or the directory of that ("dir"; "" for
/// the root itself).
string shardKeyOf(string rootDir, string path, string splitBy) {
    auto rel = relativePath(rootDir, path);
    if (rel.length == 0) rel = path;
    if (splitBy == "file") return rel;
    auto key = directoryOf(rel);
    return key == "." ? "" : key;
}

/// The file name of a shard: separators and dots become "_", leading "_" go, "" is "root".
string shardName(string key) {
    string n = key.replace(dirSeparator, "_").replace("/", "_").replace(".", "_");
    size_t i = 0;
    while (i < n.length && n[i] == '_') ++i;
    n = n[i .. $];
    return (n.length ? n : "root") ~ ".json";
}

private string jsonNumber(long n) { return n.to!string; }

/// The manifest as json.MarshalIndent writes the Go tool's Manifest (two spaces; a nil shard list is `null`).
string formatManifest(ref const Manifest m) {
    auto o = appender!(char[])();
    o ~= "{\n  \"version\": "; putString(o, m.version_);
    o ~= ",\n  \"root_dir\": "; putString(o, m.rootDir);
    o ~= ",\n  \"split_by\": "; putString(o, m.splitBy);
    o ~= ",\n  \"shards\": ";
    if (m.shards.length == 0) o ~= "null";
    else {
        o ~= "[\n";
        foreach (i, ref s; m.shards) {
            o ~= "    {\n      \"path\": "; putString(o, s.path);
            o ~= ",\n      \"files\": "; o ~= jsonNumber(s.files);
            o ~= ",\n      \"total_functions\": "; o ~= jsonNumber(s.totalFunctions);
            o ~= ",\n      \"total_classes\": "; o ~= jsonNumber(s.totalClasses);
            if (s.checksum.length) { o ~= ",\n      \"checksum\": "; putString(o, s.checksum); }
            if (s.dependsOn.length) {
                o ~= ",\n      \"depends_on\": [\n";
                foreach (k, d; s.dependsOn) { o ~= "        "; putString(o, d); o ~= k + 1 < s.dependsOn.length ? ",\n" : "\n"; }
                o ~= "      ]";
            }
            o ~= i + 1 < m.shards.length ? "\n    },\n" : "\n    }\n";
        }
        o ~= "  ]";
    }
    o ~= ",\n  \"total_files\": "; o ~= jsonNumber(m.totalFiles);
    o ~= ",\n  \"total_functions\": "; o ~= jsonNumber(m.totalFunctions);
    o ~= ",\n  \"total_classes\": "; o ~= jsonNumber(m.totalClasses);
    if (m.emptyShards.length) {
        auto names = m.emptyShards.keys;
        names.sort();
        o ~= ",\n  \"empty_shards\": {\n";
        foreach (i, n; names) {
            o ~= "    "; putString(o, n); o ~= ": "; putString(o, m.emptyShards[n]); o ~= i + 1 < names.length ? ",\n" : "\n";
        }
        o ~= "  }";
    }
    o ~= "\n}\n";
    return cast(string) o.data;
}

/// The manifest of an earlier run in outDir, or false when there is none (or it cannot be read).
bool loadManifest(string outDir, out Manifest m) {
    immutable path = buildPath(outDir, "manifest.json");
    try {
        if (!exists(path) || !isFile(path)) return false;
        auto root = parseJSON(readText(path));
        if (auto p = "shards" in root.object)
            if (p.type == JSONType.array)
                foreach (s; p.array) {
                    ShardInfo si;
                    si.path = s["path"].str;
                    si.files = s["files"].integer;
                    si.totalFunctions = s["total_functions"].integer;
                    si.totalClasses = s["total_classes"].integer;
                    if (auto c = "checksum" in s.object) si.checksum = c.str;
                    if (auto d = "depends_on" in s.object) foreach (x; d.array) si.dependsOn ~= x.str;
                    m.shards ~= si;
                }
        if (auto e = "empty_shards" in root.object)
            if (e.type == JSONType.object)
                foreach (k, v; e.object) m.emptyShards[k] = v.str;
        return true;
    } catch (Exception) {
        return false;
    }
}

/// What an `--inc` run has to process: the jobs of the shards whose checksum differs from the manifest's (or that it does
/// not know), and how many shards that is.
struct Plan {
    Job[] changed;
    size_t changedShards, unchangedShards;
}

Plan planIncremental(Job[] jobs, string rootDir, string outDir, string splitBy) {
    Manifest old;
    string[string] oldChecksums;
    if (loadManifest(outDir, old)) {
        foreach (ref s; old.shards) oldChecksums[s.path] = s.checksum;
        foreach (k, v; old.emptyShards) oldChecksums[k] = v;
    }
    Plan plan;
    Job[][string] byKey;
    string[] keys;
    foreach (ref j; jobs) {
        auto key = shardKeyOf(rootDir, j.path, splitBy);
        if (key !in byKey) keys ~= key;
        byKey[key] ~= j;
    }
    keys.sort();
    foreach (key; keys) {
        auto group = byKey[key];
        string[] paths;
        foreach (ref j; group) paths ~= j.path;
        auto sum = shardChecksum(paths);
        if (auto p = shardName(key) in oldChecksums)
            if (*p == sum) { ++plan.unchangedShards; continue; }
        ++plan.changedShards;
        plan.changed ~= group;
    }
    return plan;
}

private struct Grouped {
    const(FileResult)[][string] shards;      // shard key -> files with a declaration
    string[][string] allPaths;        // shard name -> every file, for the checksum
    string[] keys;                    // the keys of `shards`, in order
}

private Grouped groupResults(const(FileResult)[] results, string rootDir, string splitBy) {
    Grouped g;
    foreach (ref r; results) {
        auto key = shardKeyOf(rootDir, r.path, splitBy);
        g.allPaths[shardName(key)] ~= r.path;
        if (r.functions.length || r.classes.length) {
            if (key !in g.shards) g.keys ~= key;
            g.shards[key] ~= r;
        }
    }
    g.keys.sort();
    return g;
}

private string[string] emptyChecksums(string[][string] allPaths, bool[string] written) {
    string[string] empty;
    foreach (name, paths; allPaths)
        if (name !in written) empty[name] = shardChecksum(paths);
    return empty;
}

/// Writes the shards of `results` and the manifest into outDir; returns the summary the Go tool prints. With `incremental`
/// the shards of the earlier manifest that this run did not re-process are kept.
string writeSplit(const(FileResult)[] results, string outDir, string rootDir, string splitBy, bool incremental) {
    mkdirRecurse(outDir);
    Manifest old;
    immutable haveOld = incremental && loadManifest(outDir, old);
    auto g = groupResults(results, rootDir, splitBy);

    Manifest m;
    m.rootDir = rootDir;
    m.splitBy = splitBy;
    bool[string] written;
    foreach (key; g.keys) {
        auto name = shardName(key);
        auto shardResults = g.shards[key];
        write(buildPath(outDir, name), formatJson(shardResults));
        long fns, classes;
        foreach (ref r; shardResults) { fns += r.functions.length; classes += r.classes.length; }
        m.shards ~= ShardInfo(name, shardResults.length, fns, classes, shardChecksum(g.allPaths[name]));
        m.totalFiles += shardResults.length;
        m.totalFunctions += fns;
        m.totalClasses += classes;
        written[name] = true;
    }
    if (haveOld) {
        // A re-processed shard that holds no declaration any more loses its file from an earlier run.
        foreach (ref s; old.shards)
            if ((s.path in g.allPaths) !is null && s.path !in written) {
                auto p = buildPath(outDir, s.path);
                if (exists(p)) remove(p);
            }
    }
    m.emptyShards = emptyChecksums(g.allPaths, written);
    if (haveOld) {
        foreach (name, sum; old.emptyShards)
            if (name !in g.allPaths) m.emptyShards[name] = sum;
        foreach (ref s; old.shards)
            if (s.path !in g.allPaths) {
                m.shards ~= s;
                m.totalFiles += s.files;
                m.totalFunctions += s.totalFunctions;
                m.totalClasses += s.totalClasses;
            }
    }
    write(buildPath(outDir, "manifest.json"), formatManifest(m));
    if (incremental)
        return format("Split output written to %s/\n  Manifest: manifest.json\n  Shards: %s updated, %s unchanged\n  Total: %s files, %s functions, %s classes",
                      outDir, g.keys.length, m.shards.length - g.keys.length, m.totalFiles, m.totalFunctions, m.totalClasses);
    return format("Split output written to %s/\n  Manifest: manifest.json\n  Shards: %s files\n  Total: %s files, %s functions, %s classes",
                  outDir, m.shards.length, m.totalFiles, m.totalFunctions, m.totalClasses);
}

unittest {
    assert(shardName("") == "root.json");
    assert(shardName("internal") == "internal.json");
    assert(shardName("a" ~ dirSeparator ~ "b.c") == "a_b_c.json");
    assert(shardName("." ~ dirSeparator ~ "x") == "x.json");
    assert(shardKeyOf("root", buildPath("root", "a.go"), "dir") == "");
    assert(shardKeyOf("root", buildPath("root", "p", "a.go"), "dir") == "p");
    assert(shardKeyOf("root", buildPath("root", "p", "a.go"), "file") == buildPath("p", "a.go"));
    assert(shardKeyOf(".", buildPath("p", "q", "a.go"), "dir") == buildPath("p", "q"));
}

unittest {
    Manifest m;
    m.rootDir = "src";
    m.splitBy = "dir";
    assert(formatManifest(m) == "{\n  \"version\": \"1.0\",\n  \"root_dir\": \"src\",\n  \"split_by\": \"dir\",\n  \"shards\": null,\n"
        ~ "  \"total_files\": 0,\n  \"total_functions\": 0,\n  \"total_classes\": 0\n}\n");
    m.shards = [ShardInfo("root.json", 2, 3, 1, "abc", ["x.json"])];
    m.totalFiles = 2; m.totalFunctions = 3; m.totalClasses = 1;
    m.emptyShards["e.json"] = "def";
    assert(formatManifest(m) == "{\n  \"version\": \"1.0\",\n  \"root_dir\": \"src\",\n  \"split_by\": \"dir\",\n  \"shards\": [\n    {\n"
        ~ "      \"path\": \"root.json\",\n      \"files\": 2,\n      \"total_functions\": 3,\n      \"total_classes\": 1,\n"
        ~ "      \"checksum\": \"abc\",\n      \"depends_on\": [\n        \"x.json\"\n      ]\n    }\n  ],\n  \"total_files\": 2,\n"
        ~ "  \"total_functions\": 3,\n  \"total_classes\": 1,\n  \"empty_shards\": {\n    \"e.json\": \"def\"\n  }\n}\n");
}

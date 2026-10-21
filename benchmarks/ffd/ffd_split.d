module ffd_split;

import ffd_model;
import ffd_json : formatJson, putString;
import std.algorithm : sort;
import std.array : appender;
import std.exception : enforce;
import std.file : mkdirRecurse, write;
import std.format : format;
import std.path : relativePath, dirName, buildPath, dirSeparator, isAbsolute;
import std.string : replace, stripLeft;
import std.json : JSONValue, parseJSON;
import ffd_walk : Job;

class SplitCache {
    bool[string] unchanged;
    JSONValue[string] rows;
    string[string] empty;
    string state;
    string[] owned;
    size_t changedCount, unchangedCount;
    Job[] pending;
    FileResult[] placeholders;
}

SplitCache prepareIncremental(Job[] jobs, string outDir, string root, string by, string profile) {
    import std.file : readText, thisExePath, exists;
    auto cache = new SplitCache;
    cache.state = fileChecksum(thisExePath()) ~ "\n" ~ root ~ "\n" ~ by ~ "\n" ~ profile;
    bool valid;
    try {
        auto manifest = parseJSON(readText(buildPath(outDir,"manifest.json")));
        import std.json : JSONType;
        auto header = manifest["version"].str == "1.0" && manifest["root_dir"].str == root && manifest["split_by"].str == by;
        if (header && manifest["shards"].type == JSONType.array) foreach (row; manifest["shards"].array) {
            auto name = row["path"].str;
            if (safeShard(name)) cache.owned ~= name;
        }
        valid = readText(buildPath(outDir,".ffd-state")) == cache.state && header;
        if (valid) {
            if (manifest["shards"].type == JSONType.array) foreach (row; manifest["shards"].array) {
                auto name = row["path"].str;
                if (safeShard(name)) cache.rows[name] = row;
            }
            if (auto empty = "empty_shards" in manifest.object) foreach (name,sum; empty.object) if (safeShard(name)) cache.empty[name] = sum.str;
        }
    } catch (Exception) { valid = false; }
    if (!valid) { cache.rows = null; cache.empty = null; }
    Job[][string] groups;
    string[string] owners;
    foreach (job; jobs) {
        auto key = shardKey(job.path,root,by), name = shardName(key);
        enforce(name != "manifest.json", "shard name conflicts with manifest.json: " ~ key);
        if (auto old = name in owners) enforce(*old == key, "colliding shard name " ~ name);
        owners[name] = key; groups[name] ~= job;
    }
    foreach (name,group; groups) {
        string[] paths; foreach (j; group) paths ~= j.path;
        auto checksum = shardChecksum(paths);
        bool same;
        if (auto row = name in cache.rows) {
            try {
                auto content = parseJSON(readText(buildPath(outDir,name)));
                // Missing/corrupt shard files are repaired even when source bytes have not changed.
                same = (*row)["checksum"].str == checksum && content["total_files"].integer == (*row)["files"].integer;
            } catch (Exception) { same = false; }
        } else if (auto sum = name in cache.empty) same = *sum == checksum;
        if (same) {
            cache.unchanged[name] = true; ++cache.unchangedCount;
            foreach (j; group) cache.placeholders ~= FileResult(j.path);
        } else { cache.pending ~= group; ++cache.changedCount; }
    }
    return cache;
}

private bool safeShard(string name) {
    import std.path : baseName;
    import std.string : endsWith;
    return baseName(name) == name && name.endsWith(".json") && name != "manifest.json";
}

string shardName(string key) {
    auto name = key.replace(dirSeparator,"_").replace("/","_").replace(".","_").stripLeft("_");
    return (name.length ? name : "root") ~ ".json";
}

private string shardKey(string path, string root, string by) {
    string relative;
    // Match filepath.Rel's rejection of mixed absolute/relative operands.
    try { relative = isAbsolute(path) == isAbsolute(root) ? relativePath(path,root) : path; }
    catch (Exception) { relative = path; }
    if (by == "file") return relative;
    auto dir = dirName(relative);
    return dir == "." ? "" : dir;
}

// FNV-1a 128, modulo 2^128, using two portable 64-bit limbs.
private struct Hash {
    ulong hi = 0x6c62272e07bb0142UL, lo = 0x62b821756295c58dUL;
    void add(const(ubyte)[] bytes) {
        foreach (b; bytes) {
            lo ^= b;
            ulong low = (lo & 0xffffffffUL)*315;
            ulong high = (lo >> 32)*315 + (low >> 32);
            hi = hi*315 + (high >> 32) + (lo << 24);
            lo = (high << 32) | (low & 0xffffffffUL);
        }
    }
    string hex() { return format("%016x%016x",hi,lo); }
}

private string fileChecksum(string path) {
    import std.stdio : File;
    auto file = File(path,"rb");
    Hash hash;
    foreach (chunk; file.byChunk(64*1024)) hash.add(chunk);
    return hash.hex;
}

string shardChecksum(string[] paths) {
    auto ordered = paths.dup; ordered.sort;
    Hash hash;
    foreach (path; ordered) {
        string value;
        try { value = path ~ ":" ~ fileChecksum(path) ~ "\n"; }
        catch (Exception) { value = path; }
        hash.add(cast(const(ubyte)[])value);
    }
    return hash.hex;
}

private string quote(string s) {
    auto out_ = appender!(char[])(); putString(out_,s); return out_.data.idup;
}

string writeSplit(FileResult[] results, string outDir, string root, string by, SplitCache cache = null) {
    enforce(by == "dir" || by == "file", "--split-by must be 'dir' or 'file'");
    FileResult[][string] content;
    string[][string] paths;
    string[string] owners;
    foreach (r; results) {
        auto key = shardKey(r.path,root,by), name = shardName(key);
        enforce(name != "manifest.json", "shard name conflicts with manifest.json: " ~ key);
        if (auto old = name in owners) enforce(*old == key, "colliding shard name " ~ name ~ ": " ~ *old ~ " and " ~ key);
        else owners[name] = key;
        paths[name] ~= r.path;
        // Match Go: an alias-only result is not a populated split shard.
        if (r.functions.length || r.classes.length) content[name] ~= r;
    }
    auto names = paths.keys; names.sort;
    auto out_ = appender!(char[])();
    out_ ~= "{\n  \"version\": \"1.0\",\n  \"root_dir\": " ~ quote(root) ~ ",\n  \"split_by\": " ~ quote(by) ~ ",\n  \"shards\": ";
    size_t shards, files, functions, classes;
    string[string] empty;
    // Construct and validate the complete plan before touching output files.
    string[string] shardText;
    foreach (name; names) {
        if (cache !is null && name in cache.unchanged) {
            if (auto row = name in cache.rows) {
                out_ ~= shards ? ",\n" : "[\n";
                out_ ~= "    " ~ (*row).toString;
                ++shards; files += (*row)["files"].integer; functions += (*row)["total_functions"].integer; classes += (*row)["total_classes"].integer;
            } else empty[name] = cache.empty[name];
            continue;
        }
        auto checksum = shardChecksum(paths[name]);
        auto entries = name in content;
        if (entries is null) { empty[name] = checksum; continue; }
        (*entries).sort!((a,b) => a.path < b.path);
        shardText[name] = formatJson(*entries);
        size_t funcs, types;
        foreach (r; *entries) { funcs += r.functions.length; types += r.classes.length; }
        out_ ~= shards ? ",\n" : "[\n";
        out_ ~= "    {\n      \"path\": " ~ quote(name) ~ format(",\n      \"files\": %s,\n      \"total_functions\": %s,\n      \"total_classes\": %s,\n      \"checksum\": %s\n    }",(*entries).length,funcs,types,quote(checksum));
        ++shards; files += (*entries).length; functions += funcs; classes += types;
    }
    out_ ~= shards ? "\n  ]" : "null";
    out_ ~= format(",\n  \"total_files\": %s,\n  \"total_functions\": %s,\n  \"total_classes\": %s",files,functions,classes);
    if (empty.length) {
        out_ ~= ",\n  \"empty_shards\": {";
        auto keys = empty.keys; keys.sort;
        foreach (i,k; keys) out_ ~= (i ? ",\n" : "\n") ~ "    " ~ quote(k) ~ ": " ~ quote(empty[k]);
        out_ ~= "\n  }";
    }
    out_ ~= "\n}\n";
    mkdirRecurse(outDir);
    foreach (name,text; shardText) write(buildPath(outDir,name),text);
    if (cache !is null) {
        import std.file : exists, remove;
        // Only remove direct shard filenames owned by the validated prior D manifest.
        foreach (name; cache.owned) if (!(name in shardText) && !(name in cache.unchanged)) {
            auto path = buildPath(outDir,name);
            if (exists(path)) remove(path);
        }
    }
    write(buildPath(outDir,"manifest.json"),out_.data);
    if (cache !is null) write(buildPath(outDir,".ffd-state"),cache.state);
    else {
        import std.file : exists, remove;
        auto state = buildPath(outDir,".ffd-state");
        if (exists(state)) remove(state);
    }
    if (cache !is null) return format("Split output written to %s/\n  Manifest: manifest.json\n  Shards: %s updated, %s unchanged\n  Total: %s files, %s functions, %s classes",outDir,shardText.length,shards-shardText.length,files,functions,classes);
    return format("Split output written to %s/\n  Manifest: manifest.json\n  Shards: %s files\n  Total: %s files, %s functions, %s classes",outDir,shards,files,functions,classes);
}

unittest {
    Hash h;
    assert(h.hex == "6c62272e07bb014262b821756295c58d");
    h.add(cast(const(ubyte)[])"hello");
    assert(h.hex == "e3e1efd54283d94f7081314b599d31b3");
    assert(shardName("") == "root.json");
    assert(shardName("./a/b.go") == "a_b_go.json");
}

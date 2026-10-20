module ffd_main;

import ffd_json;
import ffd_langs;
import ffd_model;
import ffd_run;
import ffd_walk;
import ffd_file;
import std.conv : to, ConvException;
import std.datetime.stopwatch : AutoStart, StopWatch;
import std.file : exists, isDir;
import std.parallelism : totalCPUs;
import std.stdio : stderr, stdout;
import std.algorithm : sort;
import std.string : startsWith, strip;

version (unittest) import ffd_vectors; // unit tests of the scanners against Go's patterns

// Directory discovery plus single-file function maps, selection, extraction and line ranges.

enum ffdVersion = "0.1.0";
enum referenceVersion = import("VERSION").strip;

version (Windows) {
    extern (C) int _setmode(int fd, int mode) nothrow @nogc;
    enum _O_BINARY = 0x8000;
}

private int fatal(string text) {
    stderr.writeln("Error: ", text);
    return 1;
}

private struct Options {
    string dir;
    FileOptions file;
    bool all, structs, json, noGitignore, recursive = true, version_, timing, timingAlloc;
    int workers;
    string unsupported;
}

/// Go's flag package accepts -name, --name, -name=value and --name value; bool flags take only the first forms.
private bool parseArgs(string[] args, ref Options o, ref string error) {
    static immutable boolFlags = ["all", "struct", "json", "no-gitignore", "recursive", "version", "map", "extract", "timing", "timing-alloc"];
    static immutable valueFlags = ["dir", "workers", "inp", "source", "func", "lines"];
    static immutable unsupportedBool = ["tree", "tree-full", "names", "raw", "split", "inc"];
    static immutable unsupportedValue = ["type", "doc", "split-by", "out"];
    for (size_t i = 1; i < args.length; ++i) {
        string a = args[i];
        if (!a.startsWith("-") || a == "-" || a == "--") {
            error = "unexpected argument " ~ a;
            return false;
        }
        a = a[a.startsWith("--") ? 2 : 1 .. $];
        string name = a, value;
        bool hasValue;
        foreach (k; 0 .. a.length)
            if (a[k] == '=') {
                name = a[0 .. k];
                value = a[k + 1 .. $];
                hasValue = true;
                break;
            }
        bool isBool, isValue;
        foreach (f; boolFlags) isBool = isBool || f == name;
        foreach (f; valueFlags ~ unsupportedValue) isValue = isValue || f == name;
        bool unsupportedB;
        foreach (f; unsupportedBool) unsupportedB = unsupportedB || f == name;
        if (isValue) {
            if (!hasValue) {
                if (i + 1 >= args.length) {
                    error = "flag needs an argument: -" ~ name;
                    return false;
                }
                value = args[++i];
            }
        } else if (isBool || unsupportedB) {
            bool b = true;
            if (hasValue) {
                if (value == "true" || value == "1") b = true;
                else if (value == "false" || value == "0") b = false;
                else {
                    error = "invalid boolean value \"" ~ value ~ "\" for -" ~ name;
                    return false;
                }
            }
            if (unsupportedB) {
                if (b) o.unsupported = "--" ~ name;
                continue;
            }
            switch (name) {
                case "all": o.all = b; break;
                case "struct": o.structs = b; break;
                case "json": o.json = b; break;
                case "no-gitignore": o.noGitignore = b; break;
                case "recursive": o.recursive = b; break;
                case "version": o.version_ = b; break;
                case "timing": o.timing = b; break;
                case "timing-alloc": o.timingAlloc = b; o.timing = o.timing || b; break;
                case "map": o.file.map = b; break;
                case "extract": o.file.extract = b; break;
                default: break; // --map is the default of --dir
            }
            continue;
        } else {
            error = "flag provided but not defined: -" ~ name;
            return false;
        }
        switch (name) {
            case "dir": o.dir = value; break;
            case "inp": o.file.path = value; break;
            case "source": o.file.source = value; break;
            case "func": o.file.names = value; break;
            case "lines": o.file.lines = value; break;
            case "workers":
                try {
                    o.workers = value.to!int;
                } catch (ConvException) {
                    error = "invalid value \"" ~ value ~ "\" for flag -workers";
                    return false;
                }
                break;
            default: o.unsupported = "--" ~ name; break;
        }
    }
    return true;
}

unittest {
    Options o; string error;
    assert(parseArgs(["ffd", "--inp=a.cpp", "-source", "cpp", "--func", "a,b", "--extract", "--lines=2:"], o, error));
    assert(o.file.path == "a.cpp" && o.file.source == "cpp" && o.file.names == "a,b" && o.file.extract && o.file.lines == "2:");
    o = Options.init;
    assert(parseArgs(["ffd", "--inp", "a.go", "--map=false", "--extract=false"], o, error));
    assert(!o.file.map && !o.file.extract);
    assert(!parseArgs(["ffd", "--source"], o, error));
    assert(!parseArgs(["ffd", "--extract=maybe"], o, error));
}

int main(string[] args) {
    Options o;
    string error;
    if (!parseArgs(args, o, error)) {
        stderr.writeln("ffd: ", error);
        return 2;
    }
    if (o.version_) {
        stdout.writeln("ffd version ", ffdVersion, " (D port; reference funcfinder ", referenceVersion, ")");
        return 0;
    }
    if (o.unsupported.length) return fatal(o.unsupported ~ " is not implemented in ffd yet");
    if (o.dir.length && o.file.path.length) return fatal("--dir and --inp are mutually exclusive");
    o.file.json = o.json;
    if (o.file.path.length) {
        if (o.all || o.structs) return fatal("--all and --struct in file mode are not implemented in ffd yet");
        version (Windows) { _setmode(1, _O_BINARY); _setmode(2, _O_BINARY); }
        return runFile(o.file);
    }
    if (o.dir.length == 0) return fatal("--dir or --inp is required");
    if (o.file.source.length || o.file.names.length || o.file.lines.length || o.file.extract)
        return fatal("--source, --func, --lines and --extract require --inp");
    if (o.structs && o.all) return fatal("--struct and --all are mutually exclusive");
    if (!exists(o.dir)) return fatal("directory does not exist: " ~ o.dir);
    if (!isDir(o.dir)) return fatal("path is not a directory: " ~ o.dir);

    version (Windows) {
        _setmode(1, _O_BINARY); // a line ends in LF on both streams, as Go writes it
        _setmode(2, _O_BINARY);
    }

    try {
        initLanguages();
    } catch (Exception e) {
        return fatal("loading config: " ~ e.msg);
    }
    immutable mode = o.structs ? Mode.structs : o.all ? Mode.all : Mode.functions;
    stderr.writeln("INFO: Scanning directory: ", o.dir, " (mode=", modeName(mode), ", recursive=", o.recursive,
                   ", workers=", o.workers, ", gitignore=", !o.noGitignore, ")");

    ffd_run.trackAllocations = o.timingAlloc;
    auto watch = StopWatch(AutoStart.yes);
    auto collected = collectFiles(o.dir, o.recursive, !o.noGitignore);
    immutable walkMs = watch.peek.total!"usecs" / 1000.0;
    immutable notice = skippedNotice(collected.skipped);
    if (notice.length) stderr.writeln("INFO: ", notice);

    // Threads are not free here: the D allocator is shared, and past eight workers the run got slower, not faster
    // (OmniRoute: 559 ms at 8, 870 at 16, 1463 at 32). The default is therefore eight, and no run uses more than sixteen.
    enum defaultWorkers = 8, maxWorkers = 16;
    int workers = o.workers > 0 ? o.workers : cast(int) (totalCPUs < defaultWorkers ? totalCPUs : defaultWorkers);
    if (workers > maxWorkers) {
        stderr.writeln("INFO: --workers ", workers, " lowered to ", maxWorkers);
        workers = maxWorkers;
    }
    watch.reset();
    bool[Lang.max + 1] seen;
    Lang[] used;
    foreach (ref j; collected.jobs)
        if (!seen[j.lang]) {
            seen[j.lang] = true;
            used ~= j.lang;
        }
    prepareLanguages(used);
    immutable setupMs = watch.peek.total!"usecs" / 1000.0;
    watch.reset();
    auto results = processFiles(collected.jobs, workers, mode);
    immutable parseMs = watch.peek.total!"usecs" / 1000.0;
    foreach (ref r; results)
        if (r.error.length) stderr.writeln("Warning: ", r.path, ": ", r.error);

    watch.reset();
    immutable text = o.json ? formatJson(results) : formatGrep(results);
    immutable formatMs = watch.peek.total!"usecs" / 1000.0;
    stdout.rawWrite(text);
    stdout.rawWrite("\n"); // fmt.Println adds one after the formatter's own text
    stdout.flush();
    if (o.timing) {
        long sum;
        foreach (ref r; results) sum += r.micros;
        stderr.writefln("INFO: timing: per-file times add up to %.1f ms", sum / 1000.0);
        struct Row { long files, size, allocated, micros; }
        Row[string] byLang;
        foreach (ref r; results) {
            auto row = byLang.require(r.lang, Row.init);
            row.files++; row.size += r.size; row.allocated += r.allocated; row.micros += r.micros;
            byLang[r.lang] = row;
        }
        foreach (lang, row; byLang)
            stderr.writefln("INFO: timing: %-7s %5s files %8.2f MB source %8.2f MB allocated (%5.1f per source byte) %8.1f ms",
                            lang, row.files, row.size / 1e6, row.allocated / 1e6, row.size ? cast(double) row.allocated / row.size : 0.0, row.micros / 1000.0);
        auto slow = results.dup;
        slow.sort!((a, b) => a.micros > b.micros);
        foreach (ref r; slow[0 .. (slow.length < 6 ? slow.length : 6)])
            stderr.writefln("INFO: slowest: %7.1f ms  %s (%s)", r.micros / 1000.0, r.path, r.lang);
    }
    if (o.timing) stderr.writefln("INFO: timing: walk %.1f ms (%s files), setup %.1f ms, parse %.1f ms (%s workers), format+write %.1f ms",
                                  walkMs, collected.jobs.length, setupMs, parseMs, workers, formatMs + watch.peek.total!"usecs" / 1000.0 - formatMs);

    size_t functions, classes;
    foreach (ref r; results) {
        functions += r.functions.length;
        classes += r.classes.length;
    }
    if (mode == Mode.functions)
        stderr.writeln("INFO: Processed ", results.length, " files, found ", functions, " functions");
    else
        stderr.writeln("INFO: Processed ", results.length, " files, found ", functions, " functions, ", classes, " classes/types");
    return 0;
}

module ffd_main;

import ffd_file;
import ffd_json;
import ffd_langs;
import ffd_model;
import ffd_run;
import ffd_split;
import ffd_views;
import ffd_walk;
import std.conv : to, ConvException;
import std.datetime.stopwatch : AutoStart, StopWatch;
import std.file : exists, isDir;
import std.parallelism : totalCPUs;
import std.stdio : stderr, stdout;
import std.algorithm : sort, canFind;
import std.string : startsWith, strip;

version (unittest) import ffd_vectors; // unit tests of the scanners against Go's patterns

// ffd: the funcfinder command line, written in D. Directory runs (`--dir` with --all/--struct, --map, --tree,
// --tree-full, --json, --split [--inc]), the lists of --names and --doc, and the single-file run (`--inp --source` with
// --func, --type, --map, --tree, --tree-full, --extract, --json) behave as the Go tool does, with the .gitignore rules and
// a worker pool. `--lines` and `--raw` are not implemented and say so.

enum ffdVersion = "0.2.0";
enum referenceVersion = import("VERSION").strip;

version (Windows) {
    extern (C) int _setmode(int fd, int mode) nothrow @nogc;
    enum _O_BINARY = 0x8000;
}

private int fatal(string text, int code = 1) {
    stderr.writeln("Error: ", text);
    return code;
}

private struct Options {
    string dir, inp, source, funcStr, typeStr, docNames, splitBy = "dir", outDir = ".codemap";
    bool all, structs, json, noGitignore, recursive = true, version_, timing, timingAlloc;
    bool map, tree, treeFull, extract, names, split, inc;
    int workers;
    string unsupported;
    string[] seen; // the flags given, in order (--names and --doc accept only some)
}

private immutable string[] boolFlags = ["all", "struct", "json", "no-gitignore", "recursive", "version", "map", "timing", "timing-alloc",
                                       "tree", "tree-full", "extract", "names", "split", "inc", "raw"];
private immutable string[] valueFlags = ["dir", "workers", "inp", "source", "func", "type", "doc", "split-by", "out", "lines"];

/// Go's flag package accepts -name, --name, -name=value and --name value; bool flags take only the first forms. Like the
/// Go tool, `--struct TypeA,TypeB` is read as `--struct --type TypeA,TypeB` (the names are not a flag's value).
private bool parseArgs(string[] args, ref Options o, ref string error) {
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
        if (name == "struct") {
            o.seen ~= name;
            o.structs = true;
            if (hasValue) {
                if (value.length && value != "true" && value != "false") o.typeStr = value;
                if (value == "false") o.structs = false;
            } else if (i + 1 < args.length && !args[i + 1].startsWith("-")) o.typeStr = args[++i];
            continue;
        }
        bool isBool, isValue;
        foreach (f; boolFlags) isBool = isBool || f == name;
        foreach (f; valueFlags) isValue = isValue || f == name;
        if (!isBool && !isValue) {
            error = "flag provided but not defined: -" ~ name;
            return false;
        }
        o.seen ~= name;
        if (isValue) {
            if (!hasValue) {
                if (i + 1 >= args.length) {
                    error = "flag needs an argument: -" ~ name;
                    return false;
                }
                value = args[++i];
            }
            switch (name) {
                case "dir": o.dir = value; break;
                case "inp": o.inp = value; break;
                case "source": o.source = value; break;
                case "func": o.funcStr = value; break;
                case "type": o.typeStr = value; break;
                case "doc": o.docNames = value; break;
                case "split-by": o.splitBy = value; break;
                case "out": o.outDir = value; break;
                case "workers":
                    try {
                        o.workers = value.to!int;
                    } catch (ConvException) {
                        error = "invalid value \"" ~ value ~ "\" for flag -workers";
                        return false;
                    }
                    break;
                default: o.unsupported = "--" ~ name; break; // --lines
            }
            continue;
        }
        bool b = true;
        if (hasValue) {
            if (value == "true" || value == "1") b = true;
            else if (value == "false" || value == "0") b = false;
            else {
                error = "invalid boolean value \"" ~ value ~ "\" for -" ~ name;
                return false;
            }
        }
        switch (name) {
            case "all": o.all = b; break;
            case "json": o.json = b; break;
            case "no-gitignore": o.noGitignore = b; break;
            case "recursive": o.recursive = b; break;
            case "version": o.version_ = b; break;
            case "timing": o.timing = b; break;
            case "timing-alloc": o.timingAlloc = b; o.timing = o.timing || b; break;
            case "map": o.map = b; break;
            case "tree": o.tree = b; break;
            case "tree-full": o.treeFull = b; break;
            case "extract": o.extract = b; break;
            case "names": o.names = b; break;
            case "split": o.split = b; break;
            case "inc": o.inc = b; break;
            default: if (b) o.unsupported = "--" ~ name; break; // --raw
        }
    }
    return true;
}

/// The flags --names and --doc accept; any other flag given with them is an error (ffd's own --timing flags and --version
/// are let through).
private int checkOnly(ref const Options o, string mode, string[] allowed) {
    foreach (f; o.seen) {
        if (f == mode || f == "timing" || f == "timing-alloc" || f == "version") continue;
        if (!allowed.canFind(f)) return fatal("--" ~ mode ~ " cannot be combined with --" ~ f);
    }
    return 0;
}

private int workerCount(int requested) {
    // Threads are not free here: the D allocator is shared, and past eight workers the run got slower, not faster
    // (OmniRoute: 559 ms at 8, 870 at 16, 1463 at 32). The default is therefore eight, and no run uses more than sixteen.
    enum defaultWorkers = 8, maxWorkers = 16;
    int workers = requested > 0 ? requested : cast(int) (totalCPUs < defaultWorkers ? totalCPUs : defaultWorkers);
    if (workers > maxWorkers) {
        stderr.writeln("INFO: --workers ", workers, " lowered to ", maxWorkers);
        workers = maxWorkers;
    }
    return workers;
}

/// What a directory run produced and how long its phases took.
private struct Run {
    FileResult[] results;
    Collected collected;
    double walkMs, setupMs, parseMs;
    int workers;
}

private void prepareFor(const(Job)[] jobs) {
    bool[Lang.max + 1] seen;
    Lang[] used;
    foreach (ref j; jobs)
        if (!seen[j.lang]) {
            seen[j.lang] = true;
            used ~= j.lang;
        }
    prepareLanguages(used);
}

/// Walks the directory and processes its files. `jobsOf` may narrow the jobs (--split --inc); null keeps all of them.
private Run runDirectory(ref const Options o, Mode mode, Job[] delegate(Job[]) jobsOf = null) {
    Run run;
    auto watch = StopWatch(AutoStart.yes);
    run.collected = collectFiles(o.dir, o.recursive, !o.noGitignore);
    run.walkMs = watch.peek.total!"usecs" / 1000.0;
    immutable notice = skippedNotice(run.collected.skipped);
    if (notice.length) stderr.writeln("INFO: ", notice);
    run.workers = workerCount(o.workers);
    auto jobs = jobsOf is null ? run.collected.jobs : jobsOf(run.collected.jobs);
    watch.reset();
    prepareFor(jobs);
    run.setupMs = watch.peek.total!"usecs" / 1000.0;
    watch.reset();
    run.results = processFiles(jobs, run.workers, mode);
    run.parseMs = watch.peek.total!"usecs" / 1000.0;
    foreach (ref r; run.results)
        if (r.error.length) stderr.writeln("Warning: ", r.path, ": ", r.error);
    return run;
}

private void printTiming(ref Run run, double formatMs) {
    long sum;
    foreach (ref r; run.results) sum += r.micros;
    stderr.writefln("INFO: timing: per-file times add up to %.1f ms", sum / 1000.0);
    struct Row { long files, size, allocated, micros; }
    Row[string] byLang;
    foreach (ref r; run.results) {
        auto row = byLang.require(r.lang, Row.init);
        row.files++; row.size += r.size; row.allocated += r.allocated; row.micros += r.micros;
        byLang[r.lang] = row;
    }
    foreach (lang, row; byLang)
        stderr.writefln("INFO: timing: %-7s %5s files %8.2f MB source %8.2f MB allocated (%5.1f per source byte) %8.1f ms",
                        lang, row.files, row.size / 1e6, row.allocated / 1e6, row.size ? cast(double) row.allocated / row.size : 0.0, row.micros / 1000.0);
    auto slow = run.results.dup;
    slow.sort!((a, b) => a.micros > b.micros);
    foreach (ref r; slow[0 .. (slow.length < 6 ? slow.length : 6)])
        stderr.writefln("INFO: slowest: %7.1f ms  %s (%s)", r.micros / 1000.0, r.path, r.lang);
    stderr.writefln("INFO: timing: walk %.1f ms (%s files), setup %.1f ms, parse %.1f ms (%s workers), format+write %.1f ms",
                    run.walkMs, run.collected.jobs.length, run.setupMs, run.parseMs, run.workers, formatMs);
}

private Mode modeOf(ref const Options o) { return o.structs ? Mode.structs : o.all ? Mode.all : Mode.functions; }

private int checkDirectory(string dir) {
    if (!exists(dir)) return fatal("directory does not exist: " ~ dir);
    if (!isDir(dir)) return fatal("path is not a directory: " ~ dir);
    return 0;
}

/// A file named by --inp with --source, as the job of the directory run: the same finders, so a file and a directory
/// give the same names.
private int singleFile(ref const Options o, string flag, out FileResult result, Mode mode) {
    if (o.source.length == 0) return fatal("--source parameter is required with " ~ flag);
    Lang lang;
    if (!languageFromKey(o.source, lang)) {
        result.error = "unsupported language: " ~ o.source;
        return fatal(result.error);
    }
    prepareLanguages([lang]);
    result = applyMode(analyzeFile(o.inp, lang), mode);
    if (result.error.length) return fatal(result.error);
    return 0;
}

private int runNames(ref const Options o) {
    if (auto c = checkOnly(o, "names", ["inp", "dir", "source", "workers", "recursive", "no-gitignore", "struct", "all"])) return c;
    if (o.structs && o.all) return fatal("--struct and --all are mutually exclusive");
    wantKinds = modeOf(o) != Mode.functions; // in the default mode the Go tool names no kind: a type is a "class"
    FileResult[] results;
    if (o.dir.length) {
        if (auto c = checkDirectory(o.dir)) return c;
        results = runDirectory(o, modeOf(o)).results;
    } else {
        FileResult r;
        if (auto c = singleFile(o, "--names", r, modeOf(o))) return c;
        results = [r];
    }
    stdout.rawWrite(formatNames(results));
    return 0;
}

private int runDocs(ref const Options o) {
    if (auto c = checkOnly(o, "doc", ["inp", "dir", "source", "workers", "recursive", "no-gitignore"])) return c;
    bool[string] wanted;
    foreach (n; parseNames(o.docNames)) wanted[n] = true;
    if (wanted.length == 0) return fatal("--doc needs at least one name");
    wantKinds = true;
    FileResult[] results;
    if (o.dir.length) {
        if (auto c = checkDirectory(o.dir)) return c;
        results = runDirectory(o, Mode.all).results;
    } else {
        FileResult r;
        if (auto c = singleFile(o, "--doc", r, Mode.all)) return c;
        results = [r];
    }
    string text;
    try {
        text = formatDocs(results, wanted);
    } catch (Exception e) {
        return fatal(e.msg);
    }
    if (text.length == 0) return fatal("No declarations named " ~ o.docNames ~ " found", 2);
    stdout.rawWrite(text);
    return 0;
}

private int runDir(ref const Options o) {
    if (auto c = checkDirectory(o.dir)) return c;
    if (o.structs && o.all) return fatal("--struct and --all are mutually exclusive");
    immutable mode = modeOf(o);
    immutable mapLike = o.map || o.tree || o.treeFull;
    if (o.funcStr.length && mapLike) return fatal("--func is mutually exclusive with --map and --tree");
    if (o.tree && o.treeFull) return fatal("--tree and --tree-full are mutually exclusive");
    if (o.split) {
        if (!o.json) return fatal("--split requires --json output mode");
        if (o.splitBy != "dir" && o.splitBy != "file") return fatal("--split-by must be 'dir' or 'file'");
        if (o.inc) stderr.writeln("INFO: Incremental split mode enabled (--inc)");
    }
    stderr.writeln("INFO: Scanning directory: ", o.dir, " (mode=", modeName(mode), ", recursive=", o.recursive,
                   ", workers=", o.workers, ", gitignore=", !o.noGitignore, ")");
    ffd_run.trackAllocations = o.timingAlloc;
    wantKinds = false;

    Job[] delegate(Job[]) narrow;
    immutable dir = o.dir, outDir = o.outDir, splitBy = o.splitBy;
    if (o.split && o.inc)
        narrow = (Job[] jobs) {
            if (jobs.length == 0) return jobs;
            auto plan = planIncremental(jobs, dir, outDir, splitBy);
            stderr.writeln("INFO: Incremental: ", plan.changedShards, " shards changed, ", plan.unchangedShards, " unchanged");
            return plan.changed;
        };
    auto run = runDirectory(o, mode, narrow);

    if (o.split) {
        string summary;
        try {
            summary = writeSplit(run.results, o.outDir, o.dir, o.splitBy, o.inc);
        } catch (Exception e) {
            return fatal("writing split output: " ~ e.msg);
        }
        stdout.rawWrite(summary);
        stdout.rawWrite("\n");
        return 0;
    }

    auto watch = StopWatch(AutoStart.yes);
    string text;
    if (o.json) text = formatJson(run.results);
    else if (o.tree || o.treeFull) text = formatDirTree(run.results);
    else text = formatGrep(run.results);
    immutable formatMs = watch.peek.total!"usecs" / 1000.0;
    stdout.rawWrite(text);
    stdout.rawWrite("\n"); // fmt.Println adds one after the formatter's own text
    stdout.flush();
    if (o.timing) printTiming(run, formatMs);

    size_t functions, classes;
    foreach (ref r; run.results) {
        functions += r.functions.length;
        classes += r.classes.length;
    }
    if (mode == Mode.functions)
        stderr.writeln("INFO: Processed ", run.results.length, " files, found ", functions, " functions");
    else
        stderr.writeln("INFO: Processed ", run.results.length, " files, found ", functions, " functions, ", classes, " classes/types");
    return 0;
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
    if (o.inp.length == 0 && o.dir.length == 0) return fatal("either --inp (single file) or --dir (directory) parameter is required");
    if (o.inp.length && o.dir.length) return fatal("--inp and --dir are mutually exclusive");
    if (o.unsupported.length) return fatal(o.unsupported ~ " is not implemented in ffd yet");

    version (Windows) {
        _setmode(1, _O_BINARY); // a line ends in LF on both streams, as Go writes it
        _setmode(2, _O_BINARY);
    }
    try {
        initLanguages();
    } catch (Exception e) {
        return fatal("loading config: " ~ e.msg);
    }
    if (o.names) return runNames(o);
    if (o.docNames.length) return runDocs(o);
    if (o.dir.length) return runDir(o);
    FileRequest q;
    q.inp = o.inp; q.source = o.source; q.funcStr = o.funcStr; q.typeStr = o.typeStr;
    q.structMode = o.structs; q.allMode = o.all; q.mapMode = o.map; q.treeMode = o.tree; q.treeFull = o.treeFull;
    q.json = o.json; q.extract = o.extract;
    return runFile(q);
}

module ffd_run;

import ffd_langs;
import ffd_model;
import ffd_walk;
import std.algorithm : sort, SwapStrategy;
import std.datetime.stopwatch : AutoStart, StopWatch;
import std.parallelism : TaskPool;
import core.memory : GC;
import std.range : iota;
import std.typecons : Tuple, tuple;

/// Set by --timing: each file then also records the bytes its finder allocated.
__gshared bool trackAllocations;

// The worker pool of a directory run (the Go tool's DirProcessor.processFilesParallel): files are handed out largest
// first, so a big file that sits late in the tree starts while the small ones fill the other workers; every file
// writes only its own result slot; the results are then put in path order, so the output does not depend on which
// worker finished first.

/// Drops what the work mode does not report: --struct lists types only.
FileResult applyMode(FileResult r, Mode mode) {
    if (r.hasStructTypes) {
        // JavaScript and TypeScript: --struct lists the struct finder's types alone; --all adds to the finder's classes the
        // struct types it does not have (same name and line), as the Go tool does.
        if (mode == Mode.structs) {
            r.classes = r.structTypes;
        } else if (mode == Mode.all) {
            bool[Tuple!(string, int)] have;
            foreach (ref c; r.classes) have[tuple(c.name, c.start)] = true;
            foreach (ref t; r.structTypes)
                if (tuple(t.name, t.start) !in have) r.classes ~= t;
        }
        r.structTypes = null;
    }
    if (mode == Mode.structs) r.functions = null;
    if (mode == Mode.functions) r.aliases = null; // the Go tool reads aliases only when it reads types
    return r;
}

/// Processes every job with `workers` threads (one means no pool) and returns the results in path order.
FileResult[] processFiles(Job[] jobs, int workers, Mode mode) {
    auto order = jobs.dup;
    order.sort!((a, b) => a.size > b.size, SwapStrategy.stable);
    auto results = new FileResult[order.length];
    void one(size_t i) {
        immutable before = trackAllocations ? GC.stats().allocatedInCurrentThread : 0;
        auto watch = StopWatch(AutoStart.yes);
        results[i] = applyMode(analyzeFile(order[i].path, order[i].lang), mode);
        results[i].micros = watch.peek.total!"usecs";
        results[i].size = order[i].size;
        if (trackAllocations) results[i].allocated = GC.stats().allocatedInCurrentThread - before;
    }
    if (workers <= 1 || order.length < 2) {
        foreach (i; 0 .. order.length) one(i);
    } else {
        // The calling thread works too, so the pool needs one thread fewer.
        auto pool = new TaskPool(workers - 1);
        scope (exit) pool.finish(true);
        foreach (i; pool.parallel(iota(order.length), 1)) one(i);
    }
    results.sort!((a, b) => a.path < b.path);
    return results;
}

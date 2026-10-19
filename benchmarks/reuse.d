module reuse;

import core.memory : GC;

// Blocks of memory reused from one file to the next.
//
// A finder builds a few large arrays per file (lines, line facts, tokens, bracket pairs) by appending to them, and drops
// them when it returns. Each of those blocks, and every smaller one the appends left behind, comes from the runtime's
// allocator, which one lock guards: with several worker threads the time of a file grew with the number of workers
// (tdtp-framework: the files took 115 ms in all with one worker, 748 ms with eight). Keeping the block in a thread-local
// variable and starting from its capacity again removes almost all of that allocation.
//
// The pool variables of the modules that use this are module-level, hence thread-local in D: every worker has its own.
// What is taken from a pool must not be kept after the next call that takes the same pool; the finders return only
// copies of the names they found (slices of the cleaned lines, not of these arrays).

/// The pooled array, emptied, with its capacity open for appending again.
T[] recycled(T)(ref T[] pool) {
    pool.length = 0;
    pool.assumeSafeAppend();
    return pool;
}

/// Keeps the block of `used` (grown by appending) as the pool for the next call.
void keep(T)(ref T[] pool, T[] used) {
    pool = used;
}

unittest {
    static int[] pool;
    auto a = recycled(pool);
    foreach (i; 0 .. 1000) a ~= i;
    keep(pool, a);
    auto capacity = a.capacity;
    auto b = recycled(pool);
    assert(b.length == 0 && b.capacity >= 1000 && b.ptr is a.ptr);
    foreach (i; 0 .. 1000) b ~= i;
    assert(b.ptr is a.ptr, "appending to the recycled block must not move it");
}

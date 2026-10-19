module reuse;

import core.memory : GC;
import core.stdc.string : memcpy;
import std.traits : hasIndirections;

// Blocks of memory reused from one file to the next.
//
// A finder builds a few large arrays per file (lines, line facts, tokens, bracket pairs) by appending to them, and drops
// them when it returns. Each of those blocks, and every smaller one the appends left behind, comes from the runtime's
// allocator, which one lock guards: with several worker threads the files took longer the more workers there were. And
// `array ~= x` is a runtime call every time (measured: 6 times slower than a plain store). Two tools:
//
//  * recycled/keep: an ordinary array kept in a thread-local variable and appended to again from its capacity;
//  * Buffer: a growable buffer whose append is a bounds check and a store, kept the same way.
//
// The pool variables of the modules that use them are module-level, hence thread-local in D: every worker has its own.
// What is taken from a pool must not be kept after the next call that takes the same pool: the finders return only
// copies of the names they found (slices of the cleaned lines and the source, not of these arrays).

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

/// A growable buffer of plain values. `clear()` empties it and keeps the memory; `buffer ~= x` appends in place;
/// `data` is a slice of the contents, valid until the next append that has to grow it and until `clear()`. The memory is
/// zeroed when it is allocated and scanned by the collector when T holds pointers (token texts point into the source).
struct Buffer(T) {
    private T* ptr;
    private size_t len, cap;

    @property size_t length() const { return len; }
    @property bool empty() const { return len == 0; }
    alias opDollar = length;

    ref T opIndex(size_t i) {
        assert(i < len, "Buffer index out of range");
        return ptr[i];
    }

    @property ref T back() {
        assert(len > 0, "Buffer is empty");
        return ptr[len - 1];
    }

    void put(T value) {
        if (len == cap) grow();
        ptr[len++] = value;
    }

    void opOpAssign(string op : "~")(T value) {
        put(value);
    }

    int opApply(scope int delegate(ref T) dg) {
        foreach (i; 0 .. len)
            if (auto r = dg(ptr[i])) return r;
        return 0;
    }

    int opApply(scope int delegate(size_t, ref T) dg) {
        foreach (i; 0 .. len)
            if (auto r = dg(i, ptr[i])) return r;
        return 0;
    }

    int opApplyReverse(scope int delegate(ref T) dg) {
        foreach_reverse (i; 0 .. len)
            if (auto r = dg(ptr[i])) return r;
        return 0;
    }

    void popBack() {
        assert(len > 0, "Buffer is empty");
        --len;
    }

    void clear() {
        len = 0;
    }

    /// Makes room for at least n values in all, without changing the contents.
    void reserve(size_t n) {
        while (cap < n) grow();
    }

    T[] data() {
        return ptr[0 .. len];
    }

    private void grow() {
        enum attrs = hasIndirections!T ? 0 : GC.BlkAttr.NO_SCAN;
        immutable newCap = cap ? cap * 2 : 1024;
        auto block = cast(T*) GC.calloc(newCap * T.sizeof, attrs);
        if (len) memcpy(block, ptr, len * T.sizeof);
        ptr = block;
        cap = newCap;
    }
}

/// The parts joined with `separator` between them, in the pooled block (grown when a file is bigger than any before it).
/// The result is valid until the next call with the same pool.
string joinInto(string[] parts, ref char[] pool, char separator = '\n') {
    size_t total;
    foreach (p; parts) total += p.length;
    if (parts.length > 1) total += parts.length - 1;
    if (pool.length < total) pool = new char[total + total / 2 + 4096];
    size_t at;
    foreach (i, p; parts) {
        if (i) pool[at++] = separator;
        pool[at .. at + p.length] = p[];
        at += p.length;
    }
    return cast(string) pool[0 .. total];
}

/// What a tokenizer builds for one file and keeps for the next: the tokens, the index of each bracket's partner (-1 for
/// none) and the stacks of open "(", "[" and "{". Tokenizers that need one stack use stack[0].
struct TokenPool(T) {
    Buffer!T items;
    Buffer!int pairs;
    Buffer!int[3] stack;

    void reset() {
        items.clear();
        pairs.clear();
        foreach (ref s; stack) s.clear();
    }
}

unittest {
    static char[] joined;
    assert(joinInto(["a", "bc", ""], joined) == "a\nbc\n");
    assert(joinInto([], joined) == "");
    assert(joinInto(["x"], joined) == "x");
    assert(joinInto(["longer", "text"], joined, ',') == "longer,text");
}

unittest {
    static int[] pool;
    auto a = recycled(pool);
    foreach (i; 0 .. 1000) a ~= i;
    keep(pool, a);
    auto b = recycled(pool);
    assert(b.length == 0 && b.capacity >= 1000 && b.ptr is a.ptr);
    foreach (i; 0 .. 1000) b ~= i;
    assert(b.ptr is a.ptr, "appending to the recycled block must not move it");
}

unittest {
    struct Token { string text; int line; }
    Buffer!Token tokens;
    Buffer!int pairs;
    foreach (round; 0 .. 3) {
        tokens.clear();
        pairs.clear();
        foreach (i; 0 .. 5000) {
            tokens ~= Token("x", i);
            pairs.put(-1);
        }
        assert(tokens.length == 5000 && tokens[$ - 1].line == 4999 && tokens.back.line == 4999);
        pairs[10] = 20;
        assert(pairs.data[10] == 20 && pairs.data.length == 5000);
        tokens.popBack();
        assert(tokens.length == 4999);
    }
    // After the first round the block is reused: no growth, the same memory.
    auto before = tokens.data.ptr;
    tokens.clear();
    foreach (i; 0 .. 5000) tokens ~= Token("y", i);
    assert(tokens.data.ptr is before);
    // Growing keeps the contents.
    Buffer!int ints;
    foreach (i; 0 .. 100_000) ints ~= i;
    foreach (i; 0 .. 100_000) assert(ints[i] == i);
}

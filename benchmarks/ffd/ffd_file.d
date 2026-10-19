module ffd_file;

import ffd_langs;
import ffd_model;
import ffd_run : applyMode;
import ffd_views;
import std.algorithm : sort;
import std.array : join;
import std.conv : to;
import std.stdio : stderr, stdout;
import std.string : split, strip;
import std.traits : EnumMembers;

// The single-file run of `funcfinder --inp F --source L`: the same validations, messages and exit codes as the Go tool's
// handleFileMode (cmd/funcfinder/main.go), over the D finders. `--lines` and `--raw` are not implemented.

struct FileRequest {
    string inp, source, funcStr, typeStr;
    bool structMode, allMode, mapMode, treeMode, treeFull, json, extract;
}

private int fail(int code, string text) {
    stderr.writeln("Error: ", text);
    return code;
}

private string[] supportedLanguages() {
    string[] keys;
    foreach (l; EnumMembers!Lang) keys ~= langKey(l);
    keys.sort();
    return keys;
}

/// Splits a comma-separated list the way internal.ParseFuncNames does: trimmed, empty names dropped.
string[] parseNames(string s) {
    string[] r;
    foreach (p; s.split(",")) {
        auto t = strip(p);
        if (t.length) r ~= t;
    }
    return r;
}

private const(Sym)[] namedOnly(const(Sym)[] syms, string[] names) {
    const(Sym)[] r;
    foreach (ref s; syms)
        foreach (n; names)
            if (s.name == n) { r ~= s; break; }
    return r;
}

private void println(string text) {
    stdout.rawWrite(text);
    stdout.rawWrite("\n");
}

/// Runs one file through the request and prints the result; returns the exit code. Languages must be prepared by the caller.
int runFile(ref const FileRequest q) {
    if (q.source.length == 0) return fail(1, "--source parameter is required (or use --lines alone for plain text extraction)");
    immutable mapLike = q.mapMode || q.treeMode || q.treeFull;
    if (q.structMode && q.allMode) return fail(1, "--struct and --all are mutually exclusive");
    string workMode = q.structMode ? "structs" : q.allMode ? "all" : "functions";
    if (workMode == "functions") {
        if (q.funcStr.length == 0 && !mapLike) return fail(1, "either --func, --map, or --tree must be specified");
        if (q.funcStr.length && mapLike) return fail(1, "--func is mutually exclusive with --map and --tree");
        if (q.typeStr.length) return fail(1, "--type can only be used with --struct or --all");
    } else if (workMode == "structs") {
        if (q.typeStr.length == 0 && !mapLike && !q.extract) return fail(1, "either --type, --map, --tree, or --extract must be specified with --struct");
        if (q.typeStr.length && mapLike) return fail(1, "--type is mutually exclusive with --map and --tree");
        if (q.funcStr.length) return fail(1, "--func cannot be used with --struct");
    } else {
        if (!mapLike && !q.json) return fail(1, "--all requires --map, --tree, or --json output mode");
        if (q.funcStr.length || q.typeStr.length) return fail(1, "--func and --type cannot be used with --all (use --map instead)");
    }
    if (q.treeMode && q.treeFull) return fail(1, "--tree and --tree-full are mutually exclusive");
    Lang lang;
    if (!languageFromKey(q.source, lang))
        return fail(1, "unsupported language: " ~ q.source ~ "\nSupported languages: " ~ supportedLanguages().join(", "));

    prepareLanguages([lang]);
    wantKinds = workMode != "functions";
    auto r = analyzeFile(q.inp, lang);
    if (r.error.length) return fail(1, r.error);
    string[] lines;
    {
        auto raw = splitLines(readSource(q.inp));
        lines.length = raw.length;
        foreach (i, l; raw) lines[i] = l.idup;
    }
    // The types are the struct finder's: for JavaScript and TypeScript its own list, for the others the finders' classes.
    const(Sym)[] types = r.hasStructTypes ? r.structTypes : r.classes;
    auto view = FileView(q.inp, r.functions, r.classes, lines);

    switch (workMode) {
        case "functions": {
            auto fns = q.funcStr.length ? namedOnly(r.functions, parseNames(q.funcStr)) : r.functions;
            if (fns.length == 0) return fail(2, mapLike ? "No functions found in file" : "Specified functions not found");
            view.functions = fns;
            string out_;
            if (q.extract) out_ = formatExtract(view, fns);
            else if (q.json) out_ = formatFunctionsJson(fns);
            else if (q.treeMode) out_ = formatFileTree(view, false);
            else if (q.treeFull) out_ = formatFileTree(view, true);
            else out_ = formatGrepStyle(fns);
            println(out_);
            return 0;
        }
        case "structs": {
            auto found = q.typeStr.length ? namedOnly(types, parseNames(q.typeStr)) : types;
            if (found.length == 0) return fail(2, mapLike ? "No types found in file" : "Specified types not found");
            string out_;
            if (q.extract) out_ = formatTypeExtract(view, found);
            else if (q.json) out_ = formatTypeJson(q.inp, found);
            else if (q.treeMode || q.treeFull) out_ = formatTypeTree(found);
            else out_ = formatTypeMap(found);
            println(out_);
            return 0;
        }
        case "all": {
            if (r.functions.length == 0 && types.length == 0) return fail(2, "No functions or types found in file");
            if (q.json) {
                stdout.rawWrite(formatAllJson(q.inp, r.functions, types));
                return 0;
            }
            if (r.functions.length) {
                println("=== FUNCTIONS ===");
                println(q.extract ? formatExtract(view, r.functions) : q.treeMode ? formatFileTree(view, false) : q.treeFull ? formatFileTree(view, true) : formatGrepStyle(r.functions));
            }
            if (types.length) {
                if (r.functions.length) println("");
                println("=== TYPES ===");
                println(q.extract ? formatTypeExtract(view, types) : q.treeMode || q.treeFull ? formatTypeTree(types) : formatTypeMap(types));
            }
            return 0;
        }
        default:
            assert(0);
    }
}

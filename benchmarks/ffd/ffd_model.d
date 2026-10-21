module ffd_model;

// The language-neutral result every finder of the D port is reduced to. The finders return their own Result types
// (brace_port_model, js_ts_model, dlang_port); ffd_langs converts them here, so the walker, the pool and the
// formatters know no language.

/// A function or a type: the name, the first and last line (1-based) and the enclosing type's name, if any.
struct Sym {
    string name;
    int start;
    int end;
    string className;
    string kind; // optional metadata for --names and file type output; directory JSON is unchanged
    Field[] fields; // file type output only
}

struct Field { string name, type; int line; }

/// A Go type alias: `type name = target`, listed apart from the types.
struct Alias {
    string name;
    int line;
    string target;
}

/// Internal consumer requirements. Bounds needed for ownership are distinct
/// from function rows requested in the output. Defaults preserve full scans.
struct ScanNeeds {
    bool functions = true;
    bool functionContext = true;
    bool nativeTypes = true;
    bool structTypes = true;
    bool details;
    bool useRaw;

    static ScanNeeds directory(Mode mode, bool details = false) {
        ScanNeeds needs;
        needs.details = details;
        if (mode == Mode.structs) {
            needs.functions = false;
            needs.functionContext = false;
            needs.nativeTypes = false;
        }
        return needs;
    }

    bool needsNative() const { return functions || functionContext || nativeTypes; }
}

// Available only in diagnostic builds; native time includes tokenization and
// result conversion. The phases do not overlap and are never timed as release work.
version (FFDScanStats) struct ScanStats {
    long cleanMicros, nativeMicros, hybridMicros;
    size_t nativeCalls, hybridCalls, functionBounds, nativeTypeBounds, hybridTypeBounds;
}

/// What one file gave. `error` is set when the file could not be read or parsed; the symbols are then empty.
struct FileResult {
    string path;
    string lang;
    Sym[] functions;
    Sym[] classes;
    Alias[] aliases;
    Sym[] structTypes;     // JavaScript and TypeScript: the types of Go's struct finder (ffd_hybrid)
    bool hasStructTypes;
    string error;
    long micros; // time spent on the file (for --timing)
    long size;   // bytes of source (for --timing)
    long allocated; // bytes the finder allocated (for --timing)
    version (FFDScanStats) ScanStats scanStats;
}

/// The work modes of `funcfinder --dir`: functions only (the default), types only (--struct), both (--all).
enum Mode { functions, structs, all }

string modeName(Mode m) {
    final switch (m) {
        case Mode.functions: return "functions";
        case Mode.structs: return "structs";
        case Mode.all: return "all";
    }
}

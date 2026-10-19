module ffd_model;

// The language-neutral result every finder of the D port is reduced to. The finders return their own Result types
// (brace_port_model, js_ts_model, dlang_port); ffd_langs converts them here, so the walker, the pool and the
// formatters know no language.

/// A function or a type: the name, the first and last line (1-based), the enclosing type's name, if any, and, for a type
/// and only when a run asks for it (--names, --doc, the file views), its kind (struct, interface, enum, ...).
struct Sym {
    string name;
    int start;
    int end;
    string className;
    string kind;
}

/// A Go type alias: `type name = target`, listed apart from the types.
struct Alias {
    string name;
    int line;
    string target;
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

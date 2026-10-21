module ffd_file;

import ffd_langs;
import ffd_model;
import ffd_json : putString;
import ffd_run : applyMode;
import ffd_symbols : formatNames;
import std.algorithm : canFind, sort;
import std.array : appender;
import std.conv : to;
import std.exception : enforce;
import std.file : read;
import std.format : format;
import std.stdio : stdout, stderr;
import std.string : split, strip, join;

struct FileOptions {
    string path, source, names, lines, typeNames, doc;
    bool map, extract, json, structs, all, namesOnly;
}

struct LineRange { int start = 1; int end = -1; }

LineRange parseRange(string text) {
    enforce(text.length, "empty line range");
    auto parts = text.split(":");
    enforce(parts.length <= 2, "invalid line range: " ~ text);
    LineRange r;
    if (parts[0].length) r.start = parts[0].to!int;
    if (parts.length == 1) r.end = r.start;
    else if (parts[1].length) r.end = parts[1].to!int;
    enforce(r.start >= 1, "start line must be >= 1");
    enforce(r.end == -1 && (parts.length == 2 && parts[1].length == 0) || r.end >= 1, "end line must be >= 1");
    enforce(r.end == -1 || r.start <= r.end, "start line must be <= end line");
    return r;
}

private string quoted(string s) {
    auto out_ = appender!(char[])(); putString(out_, s); return out_.data.idup;
}

// File JSON follows Go's name-keyed map, including its last-overload-wins convention.
string fileJson(Sym[] functions, string[] lines = null, bool python = false) {
    Sym[string] byName;
    foreach (fn; functions) byName[fn.name] = fn;
    auto names = byName.keys; names.sort;
    auto out_ = appender!(char[])(); out_ ~= "{";
    foreach (i, name; names) {
        if (i) out_ ~= ",";
        auto fn = byName[name];
        out_ ~= format("\n  %s: {", quoted(name));
        string[] decorators;
        if (python) {
            import ffd_python_ranges : pythonDecoratorTexts;
            decorators = pythonDecoratorTexts(lines, fn);
        }
        if (decorators.length) {
            out_ ~= "\n    \"decorators\": [";
            foreach (j, decorator; decorators) out_ ~= (j ? ",\n      " : "\n      ") ~ quoted(decorator);
            out_ ~= "\n    ],";
        }
        out_ ~= format("\n    \"end\": %s,\n    \"start\": %s\n  }", fn.end, fn.start);
    }
    if (names.length) out_ ~= "\n";
    out_ ~= "}"; return out_.data.idup;
}

string fileText(Sym[] functions, string[] lines, bool extract) {
    auto out_ = appender!(char[])();
    foreach (i, fn; functions) {
        enforce(fn.start >= 1 && fn.end >= fn.start && fn.end <= lines.length, "invalid function bounds: " ~ fn.name);
        if (i) out_ ~= extract ? "\n\n" : "; ";
        out_ ~= format(extract ? "// %s: %s-%s\n" : "%s: %s-%s", fn.name, fn.start, fn.end);
        if (extract) out_ ~= lines[fn.start-1 .. fn.end].join("\n");
    }
    if (!extract && functions.length) out_ ~= ";";
    return out_.data.idup;
}

string lineText(string[] lines, int start, bool json) {
    auto out_ = appender!(char[])();
    if (json) out_ ~= format("{\n  \"range\": {\"start\": %s, \"end\": %s},\n  \"line_count\": %s,\n  \"lines\": [\n", start, start+lines.length-1, lines.length);
    foreach (i, line; lines) {
        if (json) out_ ~= format("    {\"line\": %s, \"content\": %s}%s\n", start+i, quoted(line), i+1 < lines.length ? "," : "");
        else out_ ~= format("%s: %s\n", start+i, line);
    }
    if (json) out_ ~= "  ]\n}\n";
    return out_.data.idup;
}

private string typeKind(Sym s) { return s.kind.length ? s.kind : "class"; }

string typesText(Sym[] types, string[] lines, bool extract) {
    auto out_ = appender!(char[])();
    foreach (i, t; types) {
        enforce(t.start >= 1 && t.end >= t.start && t.end <= lines.length, "invalid type bounds: " ~ t.name);
        if (i) out_ ~= extract ? "\n\n" : " ";
        if (extract) out_ ~= format("=== %s (%s) ===\n%s", t.name, typeKind(t), lines[t.start-1..t.end].join("\n"));
        else {
            out_ ~= format("%s: %s-%s;", t.name, t.start, t.end);
            if (t.fields.length) {
                string[] names; foreach (f; t.fields) names ~= f.name;
                out_ ~= " fields: " ~ names.join(", ");
            }
        }
    }
    return out_.data.idup;
}

string typesJson(FileResult r, bool combined) {
    auto out_ = appender!(char[])();
    out_ ~= "{\n  \"filename\": " ~ quoted(r.path) ~ ",\n";
    if (combined) {
        out_ ~= "  \"functions\": [";
        foreach (i, f; r.functions) {
            if (i) out_ ~= ",";
            out_ ~= format("\n    {\"name\": %s, \"start\": %s, \"end\": %s}", quoted(f.name), f.start, f.end);
        }
        out_ ~= "\n  ],\n";
    }
    out_ ~= "  \"types\": [";
    foreach (i, t; r.classes) {
        if (i) out_ ~= ",";
        out_ ~= format("\n    {\"name\": %s, \"kind\": %s, \"start\": %s, \"end\": %s", quoted(t.name), quoted(typeKind(t)), t.start, t.end);
        if (combined || t.fields.length) {
            out_ ~= ", \"fields\": [";
            foreach (j,f; t.fields) {
                if (j) out_ ~= ", ";
                out_ ~= format("{\"name\": %s, \"type\": %s, \"line\": %s}", quoted(f.name), quoted(f.type), f.line);
            }
            out_ ~= "]";
        }
        out_ ~= "}";
    }
    out_ ~= "\n  ]\n}"; return out_.data.idup;
}

int runFile(FileOptions o) {
    try {
        bool standalone = o.lines.length && !o.source.length;
        enforce(o.source.length || standalone, "--source parameter is required (or use --lines alone for plain text extraction)");
        if (!standalone) {
            enforce(!o.structs || !o.all, "--struct and --all are mutually exclusive");
            if (o.doc.length) {
                import ffd_docs : docNames;
                docNames(o.doc); // validate before reading the file
            } else if (o.namesOnly) {
                // Main validates the allowed flag set; names mode needs no --map.
            } else if (o.all) {
                enforce(o.map || o.json, "--all requires --map or --json output mode");
                enforce(!o.names.length && !o.typeNames.length, "--func and --type cannot be used with --all");
            } else if (o.structs) {
                enforce(!o.names.length, "--func cannot be used with --struct");
                enforce(o.typeNames.length || o.map || o.extract, "either --type, --map or --extract must be specified with --struct");
                enforce(!o.typeNames.length || !o.map, "--type is mutually exclusive with --map");
            } else {
                enforce(!o.typeNames.length, "--type can only be used with --struct or --all");
                enforce(o.names.length || o.map, "either --func or --map must be specified");
                enforce(!o.names.length || !o.map, "--func is mutually exclusive with --map");
            }
            enforce(!o.lines.length || !o.structs && !o.all, "--lines is not yet supported with --struct or --all");
        }
        Lang lang;
        if (!standalone) {
            bool found;
            foreach (l; 0 .. Lang.max+1) if (langKey(cast(Lang)l) == o.source) { lang = cast(Lang)l; found = true; break; }
            enforce(found, "unsupported language: " ~ o.source);
        }
        auto raw = cast(string)read(o.path);
        // Keep the original lines for extraction: parser normalization must not alter source bytes.
        auto original = splitLines(raw).dup;
        LineRange range;
        if (o.lines.length) range = parseRange(o.lines);
        auto end = range.end == -1 || range.end > original.length ? original.length : cast(size_t)range.end;
        if (o.lines.length && (standalone || lang != Lang.py)) enforce(range.start <= end, "no lines found in requested range");
        if (standalone) { stdout.rawWrite(lineText(original[range.start-1 .. end], range.start, o.json)); return 0; }
        initLanguages(); prepareLanguages([lang]);
        bool pythonRange = lang == Lang.py && o.lines.length;
        auto source = o.lines.length && !pythonRange ? original[range.start-1 .. end].join("\n") : raw;
        auto result = analyzeSource(o.path, lang, source, o.structs || o.all || o.doc.length);
        enforce(!result.error.length, result.error);
        if (o.doc.length) {
            import ffd_docs : docNames, formatDocs;
            auto text = formatDocs([applyMode(result,Mode.all)],docNames(o.doc),original);
            if (!text.length) { stderr.writeln("Error: No declarations named ",o.doc," found"); return 2; }
            stdout.rawWrite(text); return 0;
        }
        if (pythonRange) {
            import ffd_python_ranges : adjustPythonRange;
            auto fixed = adjustPythonRange(result, raw, range.start, range.end);
            range.start = fixed.start; range.end = fixed.end;
            if (fixed.report.length) { stdout.rawWrite(fixed.report); stdout.rawWrite("\n"); }
            enforce(range.start <= original.length, "no lines found in requested range");
            stderr.writeln("INFO: Using --lines filter (", range.start, ":", range.end == -1 ? "EOF" : range.end.to!string,
                           "). Functions outside this range will be excluded.");
        }
        if (o.namesOnly) {
            result = applyMode(result, o.structs ? Mode.structs : o.all ? Mode.all : Mode.functions);
            stdout.rawWrite(formatNames([result])); return 0;
        }
        if (o.structs || o.all) {
            auto allFunctions = result.functions;
            result = applyMode(result, Mode.structs); // select the same native type set as directory --struct
            // Field ownership needs every nested type and method, even when the user selects one type.
            result.functions = allFunctions;
            import ffd_fields : extractFields;
            if (!o.extract || o.all && o.json) extractFields(result, raw);
            if (!o.all) result.functions = null;
            if (o.typeNames.length) {
                auto wanted = o.typeNames.split(","); foreach (ref n; wanted) n = n.strip;
                Sym[] selected; foreach (t; result.classes) if (wanted.canFind(t.name)) selected ~= t;
                result.classes = selected;
            }
            if (!result.classes.length && (!o.all || !result.functions.length)) {
                stderr.writeln("Error: ", o.all ? "No functions or types found in file" : o.typeNames.length ? "Specified types not found" : "No types found in file"); return 2;
            }
            string text;
            if (o.all && o.json || !o.extract && o.json) text = typesJson(result, o.all);
            else if (!o.all) text = typesText(result.classes, original, o.extract);
            else {
                if (result.functions.length) text = "=== FUNCTIONS ===\n" ~ fileText(result.functions, lang == Lang.py ? raw.split("\n") : original, o.extract) ~ "\n";
                if (result.classes.length) text ~= (text.length ? "\n" : "") ~ "=== TYPES ===\n" ~ typesText(result.classes, original, o.extract) ~ "\n";
                // The outer write supplies the final newline, as the Go CLI does.
                if (text.length) text = text[0..$-1];
            }
            stdout.rawWrite(text); stdout.rawWrite("\n"); return 0;
        }
        auto requested = o.names.split(",");
        foreach (ref name; requested) name = name.strip;
        Sym[] functions;
        foreach (fn; result.functions) {
            if (pythonRange && (fn.start < range.start || range.end != -1 && fn.end > range.end)) continue;
            if (o.lines.length && !pythonRange) { fn.start += range.start-1; fn.end += range.start-1; }
            if (o.map || requested.canFind(fn.name)) functions ~= fn;
        }
        if (!functions.length) { stderr.writeln("Error: ", o.map ? "No functions found in file" : "Specified functions not found"); return 2; }
        // Go's dedicated Python reader retains CR bytes; its other readers normalize CRLF.
        auto extractLines = lang == Lang.py ? raw.split("\n") : original;
        auto text = o.extract ? fileText(functions, extractLines, true) : o.json ? fileJson(functions, lang == Lang.py ? extractLines : original, lang == Lang.py) : fileText(functions, lang == Lang.py ? extractLines : original, false);
        stdout.rawWrite(text); stdout.rawWrite("\n"); return 0;
    } catch (Exception e) { stderr.writeln("Error: ", e.msg); return 1; }
}

unittest {
    assert(parseRange("4").start == 4 && parseRange("4").end == 4);
    assert(parseRange(":8").start == 1 && parseRange("3:").end == -1);
    foreach (bad; ["", "0", "-1:2", "3:2", "1:0", "1:-1", "1:2:3", "x"]) {
        bool failed; try { parseRange(bad); } catch (Exception) { failed = true; } assert(failed, bad);
    }
    auto syms = [Sym("f",1,1), Sym("f",2,2)];
    assert(fileText(syms, ["a", "b"], true) == "// f: 1-1\na\n\n// f: 2-2\nb");
    assert(fileJson(syms) == "{\n  \"f\": {\n    \"end\": 2,\n    \"start\": 2\n  }\n}");
    assert(lineText(["a", "b"], 3, false) == "3: a\n4: b\n");
    import std.json : parseJSON;
    auto r = FileResult("x.ts", "ts"); r.classes = [Sym("C",1,1,"","interface")];
    auto doc = parseJSON(typesJson(r, false));
    assert(doc["filename"].str == "x.ts" && doc["types"].array[0]["kind"].str == "interface");
    assert(typesText(r.classes, ["interface C {}"], true) == "=== C (interface) ===\ninterface C {}");
    r.functions = syms;
    doc = parseJSON(typesJson(r, true)); assert(doc["functions"].array.length == 2);
    assert(doc["types"].array[0]["fields"].array.length == 0);
}

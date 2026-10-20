module ffd_file;

import ffd_langs;
import ffd_model;
import ffd_json : putString;
import std.algorithm : canFind, sort;
import std.array : appender;
import std.conv : to;
import std.exception : enforce;
import std.file : read;
import std.format : format;
import std.stdio : stdout, stderr;
import std.string : split, strip, join;

struct FileOptions {
    string path, source, names, lines;
    bool map, extract, json;
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
            foreach (line; lines[fn.start-1 .. fn.end]) {
                auto text = line.strip;
                if (text.length && text[0] == '@') decorators ~= text;
                if (text.length >= 4 && text[0..4] == "def " || text.length >= 10 && text[0..10] == "async def ") break;
            }
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

int runFile(FileOptions o) {
    try {
        bool standalone = o.lines.length && !o.source.length;
        enforce(o.source.length || standalone, "--source parameter is required (or use --lines alone for plain text extraction)");
        if (!standalone) {
            enforce(o.names.length || o.map, "either --func or --map must be specified");
            enforce(!o.names.length || !o.map, "--func is mutually exclusive with --map");
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
        if (o.lines.length) enforce(range.start <= end, "no lines found in requested range");
        if (standalone) { stdout.rawWrite(lineText(original[range.start-1 .. end], range.start, o.json)); return 0; }
        initLanguages(); prepareLanguages([lang]);
        auto source = o.lines.length ? original[range.start-1 .. end].join("\n") : raw;
        auto result = analyzeSource(o.path, lang, source);
        enforce(!result.error.length, result.error);
        auto requested = o.names.split(",");
        foreach (ref name; requested) name = name.strip;
        Sym[] functions;
        foreach (fn; result.functions) {
            if (o.lines.length) { fn.start += range.start-1; fn.end += range.start-1; }
            if (o.map || requested.canFind(fn.name)) functions ~= fn;
        }
        if (!functions.length) { stderr.writeln("Error: ", o.map ? "No functions found in file" : "Specified functions not found"); return 2; }
        // Go's dedicated Python reader retains CR bytes; its other readers normalize CRLF.
        auto extractLines = lang == Lang.py ? raw.split("\n") : original;
        auto text = o.extract ? fileText(functions, extractLines, true) : o.json ? fileJson(functions, original, lang == Lang.py) : fileText(functions, original, false);
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
}

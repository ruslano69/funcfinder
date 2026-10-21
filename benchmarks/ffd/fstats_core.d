module fstats_core;

import ffd_langs;
import ffd_config : rootConfig;
import ffd_model;
import ffd_tokens;
import ffd_calls : scanCalls, callWord;
import brace_port_model : goRegex;
import std.algorithm : canFind, sort;
import std.array : array;
import std.json;
import std.regex : Regex, matchFirst;
import std.string : strip, startsWith, split, join, count;

struct CallEntry { string name; int count; }
struct Stats {
    string path, language;
    ulong bytes;
    int total, code, comments, blank;
    string[] imports, decorators;
    int[string] calls, uses;
    Sym[] functions;
    string error;
}

private struct Profile {
    string name;
    string[] excluded;
    Regex!char imports, decorators;
    bool hasImports, hasDecorators;
}
private __gshared Profile[Lang.max+1] profiles;
private bool[] codePool, metadataPool;

void prepareStats() {
    auto languages = rootConfig()["languages"];
    foreach (i; 0 .. profiles.length) {
        auto cfg = languages[langKey(cast(Lang)i)];
        Profile p; p.name = cfg["name"].str;
        p.excluded = cfg["exclude_words"].strings.dup;
        foreach (key; ["import_pattern", "decorator_pattern"]) {
            auto pattern = cfg[key].str;
            if (!pattern.length) continue;
            if (key == "import_pattern") { p.imports = goRegex(pattern); p.hasImports = true; }
            else { p.decorators = goRegex(pattern); p.hasDecorators = true; }
        }
        profiles[i] = p;
    }
}

string languageName(Lang lang) { return profiles[lang].name; }

private void capture(string line, Regex!char pattern, ref string[] values, ref bool[string] seen, bool firstOnly) {
    auto m = matchFirst(line, pattern);
    if (m.empty) return;
    foreach (i; 1 .. m.captures.length) {
        auto value = m.captures[i];
        if (value.length && value !in seen) { seen[value.idup] = true; values ~= value.idup; }
        if (firstOnly) break;
    }
}

Stats analyzeStats(string path, Lang lang, bool collectUses = true) {
    import std.file : getSize;
    Stats s; s.path = path; s.language = languageName(lang);
    try {
        s.bytes = getSize(path);
        // The source/finder arenas are thread-local; every value retained below owns its strings.
        auto source = readSource(path);
        ScanNeeds needs; needs.signatureMetadata = true;
        auto found = analyzeSource(path,lang,source,needs);
        if (found.error.length) throw new Exception(found.error);
        s.functions = found.functions;
        auto lines = splitLines(source);
        auto tokens = tokensFor(source,langKey(lang));
        auto calls = scanCalls(tokens,s.functions,found.signatures,langKey(lang),profiles[lang].excluded,false);
        foreach (site; calls.sites) {
            if (auto count = site.name in s.calls) ++*count;
            else s.calls[site.name.idup] = 1;
        }
        auto code = zeroScratch(codePool,lines.length);
        auto copied = code.length < tokens.codeLines.length ? code.length : tokens.codeLines.length;
        code[0..copied] = tokens.codeLines[0..copied];
        auto metadata = zeroScratch(metadataPool,lines.length);
        foreach (i, t; tokens.items) {
            if (t.text == "\n") continue;
            auto last = t.line + cast(int)t.text.count('\n');
            foreach (line; t.line .. last+1) if (line > 0 && line <= code.length) code[line-1] = true;
            if (!t.literal && t.line > 0 && t.line <= metadata.length) metadata[t.line-1] = true;
            if (!collectUses || !callWord(t) || t.literal || calls.declaredNames[i]) continue;
            if (auto count = t.text in s.uses) ++*count;
            else s.uses[t.text.idup] = 1;
        }
        bool[string] imports, decorators;
        foreach (i, line; lines) {
            ++s.total;
            auto text = line.strip;
            if (!text.length) { ++s.blank; continue; }
            if (lang == Lang.php && (text.startsWith("<?php") || text.startsWith("<?=") || text == "?>")) code[i] = true;
            if (text.startsWith("#!")) code[i] = false;
            if (code[i]) ++s.code; else ++s.comments;
            // Preserve the configured import/annotation measurement profile. It is not dependency resolution.
            bool active = tokens.metadataLines.length ? tokens.metadataLines[i] : metadata[i];
            if (active && profiles[lang].hasImports) capture(line,profiles[lang].imports,s.imports,imports,false);
            if (active && profiles[lang].hasDecorators) capture(line,profiles[lang].decorators,s.decorators,decorators,true);
        }
    } catch (Exception e) { s.error = e.msg.idup; }
    return s;
}

CallEntry[] sortedCalls(int[string] counts, int top = 0) {
    CallEntry[] result;
    foreach (name, n; counts) result ~= CallEntry(name,n);
    result.sort!((a,b) => a.count != b.count ? a.count > b.count : a.name < b.name);
    if (top > 0 && top < result.length) result = result[0..top];
    return result;
}

JSONValue callJSON(int[string] calls, int top) {
    JSONValue[] rows = [];
    foreach (c; sortedCalls(calls,top)) rows ~= JSONValue(["name":JSONValue(c.name),"count":JSONValue(c.count)]);
    return JSONValue(rows);
}

private JSONValue stringsJSON(string[] values) {
    JSONValue[] rows = []; foreach (v; values) rows ~= JSONValue(v); return JSONValue(rows);
}

JSONValue statsJSON(Stats s, int top) {
    return JSONValue([
        "language":JSONValue(s.language), "file":JSONValue(s.path), "file_size_kb":JSONValue(s.bytes / 1024.0),
        "total_lines":JSONValue(s.total), "code_lines":JSONValue(s.code), "comment_lines":JSONValue(s.comments),
        "blank_lines":JSONValue(s.blank), "imports":stringsJSON(s.imports), "decorators":stringsJSON(s.decorators),
        "unique_calls":JSONValue(s.calls.length), "top_calls":callJSON(s.calls,top)
    ]);
}

void addStats(ref Stats sum, Stats s) {
    sum.total += s.total; sum.code += s.code; sum.comments += s.comments; sum.blank += s.blank;
    foreach (k,n; s.calls) sum.calls[k] += n;
    foreach (k,n; s.uses) sum.uses[k] += n;
}

private bool testFile(string path, Lang lang) {
    import std.path : baseName;
    import std.string : endsWith;
    auto name = baseName(path);
    return lang == Lang.go && name.endsWith("_test.go") || lang == Lang.py && (name.startsWith("test_") || name.endsWith("_test.py")) ||
        (lang == Lang.js || lang == Lang.ts) && (name.canFind(".test.") || name.canFind(".spec."));
}

struct Candidate { string file; int line; string name; bool exported; }
struct Uncalled { int considered; Candidate[] candidates; }

Uncalled findUncalled(Stats[] files, string root, Lang lang) {
    import std.path : relativePath, absolutePath, dirName;
    import std.string : replace;
    import std.uni : isUpper;
    int[string] uses;
    foreach (s; files) foreach (name,n; s.uses) uses[name] += n;
    Uncalled report;
    foreach (s; files) foreach (f; s.functions) {
        if (testFile(s.path,lang) || ["main","init"].canFind(f.name)) continue;
        if (lang == Lang.go && ["String","Error","Len","Less","Swap","Close","Read","Write","MarshalJSON","UnmarshalJSON","ServeHTTP"].canFind(f.name)) continue;
        ++report.considered;
        if (f.name in uses) continue;
        auto rel = relativePath(absolutePath(s.path),absolutePath(root));
        bool internal_ = dirName(rel).replace("\\","/").split("/").canFind("internal");
        bool exported;
        foreach (dchar c; f.name) { exported = lang == Lang.go && !internal_ && isUpper(c); break; }
        report.candidates ~= Candidate(s.path,f.start,f.name,exported);
    }
    report.candidates.sort!((a,b) => a.file != b.file ? a.file < b.file : a.line < b.line);
    return report;
}

JSONValue uncalledJSON(Uncalled report) {
    JSONValue[] rows = [];
    foreach (c; report.candidates) rows ~= JSONValue(["file":JSONValue(c.file),"line":JSONValue(c.line),"name":JSONValue(c.name),"exported":JSONValue(c.exported)]);
    return JSONValue(["considered":JSONValue(report.considered),"candidates":JSONValue(rows)]);
}

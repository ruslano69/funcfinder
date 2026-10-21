module fstats_walk;

import ffd_langs;
import ffd_walk;
import std.algorithm : canFind;
import std.json;
import std.regex : Regex, regex, matchFirst;
import std.string : replace;
import std.path : baseName, relativePath, absolutePath;

// path.Match-style exclusions: * and ? never cross a slash; ** has no special meaning.
private Regex!char glob(string pattern) {
    string text = "^";
    for (size_t i; i < pattern.length; ++i) {
        auto c = pattern[i];
        if (c == '*') text ~= "[^/]*";
        else if (c == '?') text ~= "[^/]";
        else if (c == '\\') {
            if (++i == pattern.length) throw new Exception("bad -exclude pattern: trailing escape");
            if ("\\.*+?()[]|^${}".canFind(pattern[i])) text ~= '\\';
            text ~= pattern[i];
        } else if (c == '[') {
            text ~= "(?!/)["; ++i;
            if (i < pattern.length && pattern[i] == '^') { text ~= '^'; ++i; }
            bool any;
            for (; i < pattern.length && pattern[i] != ']'; ++i) {
                if (pattern[i] == '\\') {
                    if (++i == pattern.length) throw new Exception("bad -exclude pattern: trailing escape");
                    if ("\\]-^".canFind(pattern[i])) text ~= '\\';
                    text ~= pattern[i];
                } else {
                    if (pattern[i] == '-' && (!any || i+1 >= pattern.length || pattern[i+1] == ']'))
                        throw new Exception("bad -exclude pattern: invalid range");
                    text ~= pattern[i];
                }
                any = true;
            }
            if (!any || i == pattern.length) throw new Exception("bad -exclude pattern: unclosed or empty class");
            text ~= ']';
        } else {
            if (".()|+^${}".canFind(c)) text ~= '\\';
            text ~= c;
        }
    }
    try { return regex(text ~ "$(?![\\s\\S])"); }
    catch (Exception e) { throw new Exception("bad -exclude pattern: " ~ e.msg); }
}

Job[] statsFiles(string root, out Lang language, string forced, string[] excludes) {
    import std.file : exists, isDir, getSize;
    auto configs = parseJSON(configText)["languages"];
    if (forced.length) {
        bool valid;
        foreach (i; 0 .. Lang.max+1) if (langKey(cast(Lang)i) == forced) { language = cast(Lang)i; valid = true; }
        if (!valid) throw new Exception("unsupported language: " ~ forced);
    } else if (!exists(root)) throw new Exception("detecting language: directory does not exist: " ~ root);
    Job[] jobs;
    if (exists(root) && !isDir(root)) {
        Lang detected;
        if (languageOf(fileExtension(baseName(root)),detected)) jobs ~= Job(root,detected,cast(long)getSize(root));
    } else jobs = collectFiles(root,true,true,true).jobs;
    if (!forced.length) {
        size_t[Lang.max+1] counts;
        foreach (j; jobs) ++counts[j.lang];
        size_t best;
        foreach (i,n; counts) if (n > best) { best = n; language = cast(Lang)i; }
        if (!best) throw new Exception("no supported files found in directory");
    }
    Regex!char[] patterns; foreach (p; excludes) patterns ~= glob(p);
    string[] extensions; foreach (e; configs[langKey(language)]["extensions"].array) extensions ~= e.str;
    Job[] result;
    foreach (j; jobs) {
        if (!extensions.canFind(fileExtension(baseName(j.path)))) continue;
        auto rel = relativePath(absolutePath(j.path),absolutePath(root)).replace("\\","/");
        bool excluded;
        foreach (p; patterns) if (!matchFirst(baseName(j.path),p).empty || !matchFirst(rel,p).empty) { excluded = true; break; }
        if (!excluded) { j.lang = language; result ~= j; }
    }
    return result;
}

unittest {
    assert(!matchFirst("foo_test.go",glob("*_test.go")).empty);
    assert(matchFirst("x/foo_test.go",glob("*_test.go")).empty);
    assert(!matchFirst("x/foo.go",glob("x/*.go")).empty);
    assert(!matchFirst("a.go",glob("[a-c].go")).empty);
    assert(!matchFirst("z.go",glob("[^a-c].go")).empty);
    foreach (bad; ["[", "[]", "[a-]", "abc\\"]) {
        bool failed; try { glob(bad); } catch (Exception) { failed = true; } assert(failed);
    }
}

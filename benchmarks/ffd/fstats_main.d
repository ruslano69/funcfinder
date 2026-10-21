module fstats_main;

import fstats_core;
import fstats_walk;
import ffd_langs;
import ffd_walk : Job, fileExtension;
import std.algorithm : min, canFind;
import std.conv : to;
import std.format : format;
import std.json;
import std.parallelism : TaskPool;
import std.path : baseName;
import std.range : iota;
import std.stdio : stdout, stderr;
import std.string : join, startsWith, strip;

version (Windows) {
    extern (C) int _setmode(int fd, int mode) nothrow @nogc;
}

private void usage() {
    stdout.writeln("Usage: funcstats [OPTIONS] <source_file>\n       funcstats [OPTIONS] --dir <directory>");
    stdout.writeln("  --version      Show version and exit");
    stdout.writeln("  --dir <path>   Analyze all source files in directory recursively");
    stdout.writeln("  -l <lang>      Force language (c, cpp, cs, d, go, java, js, kotlin, php, py, ruby, rust, scala, swift, ts)");
    stdout.writeln("  -n <num>       Show top N functions");
    stdout.writeln("  -j, --json     Output JSON");
    stdout.writeln("  --uncalled     Dir mode: list functions no code line names (candidates to delete)");
    stdout.writeln("  --exclude GLOB Dir mode: leave out files whose name or path under <dir> matches (repeatable)");
}

private void printFile(Stats s, int top) {
    stdout.writef("Language: %s\nFile: %s (%.1f KB)\n%s\nLines: %d\n",s.language,baseName(s.path),s.bytes/1024.0,"-----------------------------------",s.total);
    if (s.total) {
        stdout.writef("  Code:     %d (%.1f%%)\n",s.code,s.code*100.0/s.total);
        stdout.writef("  Comments: %d (%.1f%%)\n",s.comments,s.comments*100.0/s.total);
        stdout.writef("  Blank:    %d (%.1f%%)\n",s.blank,s.blank*100.0/s.total);
    }
    if (s.imports.length) stdout.writef("Imports: %d (%s%s)\n",s.imports.length,s.imports[0..min(5,s.imports.length)].join(", "),s.imports.length>5?", ...":"");
    if (s.decorators.length) stdout.writef("Decorators: %d (%s)\n",s.decorators.length,s.decorators.join(", "));
    stdout.writef("%s\nFunction calls: %d unique\n%s\n","-----------------------------------",s.calls.length,"-----------------------------------");
    foreach (c; sortedCalls(s.calls,top)) stdout.writef("%-25s %d\n",c.name,c.count);
}

private void printUncalled(Uncalled report, bool json) {
    if (json) { stdout.writeln(uncalledJSON(report).toPrettyString); return; }
    stdout.writef("Uncalled: %d of %d functions have no use in the tree (candidates to check before deleting)\n",report.candidates.length,report.considered);
    foreach (c; report.candidates) stdout.writef("  %s:%d  %s%s\n",c.file,c.line,c.name,c.exported?"  (exported: code outside this tree may call it)":"");
    stdout.writeln("Not counted: functions in test files, main and init, and Go interface methods (String, Error, Len, ...).");
    stdout.writeln("A name used only in strings, through reflection, or from another repository looks uncalled.");
}

int main(string[] args) {
    version (Windows) { _setmode(1,0x8000); _setmode(2,0x8000); }
    string file, dir, forced; string[] excludes; int top; bool json, uncalled, version_;
    try {
        for (size_t i=1; i<args.length; ++i) {
            auto a = args[i];
            if (a == "--help" || a == "-h") { usage(); return 0; }
            if (a == "--version") { version_ = true; continue; }
            if (a == "-j" || a == "--json") { json = true; continue; }
            if (a == "--uncalled") { uncalled = true; continue; }
            if (["--dir","-l","-n","--exclude"].canFind(a)) {
                if (++i == args.length) throw new Exception("missing value for " ~ a);
                auto value = args[i];
                if (a == "--dir") dir = value;
                if (a == "-l") forced = value;
                if (a == "--exclude") excludes ~= value;
                if (a == "-n") top = value.to!int;
                continue;
            }
            if (a.startsWith("-")) throw new Exception("unknown option: " ~ a);
            file = a;
        }
        if (version_) { stdout.writeln("funcstats D 0.1.0 (reference funcstats " ~ import("VERSION").strip ~ ")"); return 0; }
        if (uncalled && !dir.length) throw new Exception("--uncalled needs --dir: one file cannot show what is unused elsewhere");
        if (!file.length && !dir.length) throw new Exception("source file or --dir is required\nUsage: funcstats [OPTIONS] <source_file>\n       funcstats [OPTIONS] --dir <directory>");
        initLanguages(); prepareStats();
        Lang lang;
        Job[] jobs;
        if (dir.length) jobs = statsFiles(dir,lang,forced,excludes);
        else {
            if (forced.length) {
                bool found;
                foreach (i; 0 .. Lang.max+1) if (langKey(cast(Lang)i) == forced) { lang=cast(Lang)i; found=true; }
                if (!found) throw new Exception("unsupported language: " ~ forced);
            } else if (!languageOf(fileExtension(baseName(file)),lang)) throw new Exception("cannot detect language from file extension");
            jobs = [Job(file,lang)];
        }
        prepareLanguages([lang]);
        auto results = new Stats[jobs.length];
        if (jobs.length < 2) foreach (i,j; jobs) results[i] = analyzeStats(j.path,lang);
        else {
            auto pool = new TaskPool(7); scope(exit) pool.finish(true);
            foreach (i; pool.parallel(iota(jobs.length),1)) results[i] = analyzeStats(jobs[i].path,lang);
        }
        foreach (s; results) if (s.error.length) throw new Exception("reading file: " ~ s.error);
        if (uncalled) { printUncalled(findUncalled(results,dir,lang),json); return 0; }
        if (!dir.length) {
            if (json) stdout.writeln(statsJSON(results[0],top).toPrettyString);
            else printFile(results[0],top);
            return 0;
        }
        Stats sum; foreach (s; results) addStats(sum,s);
        if (json) {
            JSONValue[] rows=[]; foreach(s; results) rows ~= statsJSON(s,top);
            auto output=JSONValue([
                "language":JSONValue(languageName(lang)),"dir":JSONValue(dir),"total_files":JSONValue(results.length),
                "total_lines":JSONValue(sum.total),"code_lines":JSONValue(sum.code),"comment_lines":JSONValue(sum.comments),
                "blank_lines":JSONValue(sum.blank),"unique_calls":JSONValue(sum.calls.length),"top_calls":callJSON(sum.calls,top),"files":JSONValue(rows)
            ]);
            stdout.writeln(output.toPrettyString);
        } else {
            stdout.writef("Language: %s  Dir: %s\n%s\n",languageName(lang),dir,"=============================================");
            foreach (s; results) {
                stdout.writef("\n%s (%.1f KB, %d lines, %d unique calls)\n",s.path,s.bytes/1024.0,s.total,s.calls.length);
                foreach (c; sortedCalls(s.calls,top)) stdout.writef("  %-23s %d\n",c.name,c.count);
            }
            stdout.writef("\n%s\nTOTAL  files: %d  lines: %d  unique calls: %d\n%s\n","=============================================",results.length,sum.total,sum.calls.length,"---------------------------------------------");
            foreach(c; sortedCalls(sum.calls,top)) stdout.writef("%-25s %d\n",c.name,c.count);
        }
        return 0;
    } catch (Exception e) { stderr.writeln("Error: ",e.msg); return 1; }
}

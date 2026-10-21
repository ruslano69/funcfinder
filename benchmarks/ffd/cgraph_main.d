module cgraph_main;

import cgraph_core;
import ffd_langs;
import ffd_config : languageConfig;
import ffd_walk;
import std.algorithm : canFind, sort;
import std.conv : to;
import std.file : exists, isDir;
import std.json;
import std.parallelism : TaskPool;
import std.range : iota;
import std.stdio : stdout, stderr;
import std.string : startsWith, strip, join;

version (Windows) { extern (C) int _setmode(int fd, int mode) nothrow @nogc; }

private void usage() {
    stdout.writeln("Usage: callgraph [OPTIONS]\n" ~
        "  --dir <path>       Analyze directory\n" ~
        "  --inp <file>       Analyze single file\n" ~
        "  -l <lang>          Language (required with --inp; directory filter)\n" ~
        "  --json, -j         JSON output\n" ~
        "  --reverse, -r      Reverse graph: who calls each function\n" ~
        "  --func <name>      Focus on one function\n" ~
        "  --depth <n>        Limit forward traversal depth (0: unlimited)\n" ~
        "  --no-gitignore     Ignore .gitignore rules\n" ~
        "  --version          Print version");
}

private void output(Graph graph, bool json, bool reverse_, string focus, int depth) {
    if (reverse_) {
        auto reversed = reverseGraph(graph);
        if (focus.length) {
            auto callers = reverseCallers(reversed,focus);
            if (json) stdout.writeln(JSONValue(["function":JSONValue(focus),"callers":JSONValue(callers)]).toPrettyString);
            else if (!callers.length) stdout.writef("%s: not called by any known function\n",focus);
            else { stdout.writef("%s is called by:\n",focus); foreach (c; callers) stdout.writeln("  ",c); }
        } else {
            auto keys = reversed.keys; keys.sort;
            JSONValue[] rows;
            foreach (key; keys) {
                if (json) rows ~= JSONValue(["callee":JSONValue(key),"callers":JSONValue(reversed[key])]);
                else stdout.writeln(key," ← ",reversed[key].join(", "));
            }
            if (json) stdout.writeln(JSONValue(["reverse":rows.length ? JSONValue(rows) : JSONValue(null)]).toPrettyString);
        }
        return;
    }
    if (focus.length) graph = filterGraph(graph,focus,depth);
    if (json) stdout.writeln(graphJSON(graph).toPrettyString);
    else stdout.write(focus.length ? treeText(graph,focus) : graphText(graph));
}

int main(string[] args) {
    version (Windows) { _setmode(1,0x8000); _setmode(2,0x8000); }
    string dir, file, forced, focus; bool json, reverse_, noIgnore, version_; int depth;
    try {
        for (size_t i=1; i<args.length; ++i) {
            auto a = args[i];
            if (a == "--help" || a == "-h") { usage(); return 0; }
            if (a == "--version") { version_ = true; continue; }
            if (a == "--json" || a == "-j") { json = true; continue; }
            if (a == "--reverse" || a == "-r") { reverse_ = true; continue; }
            if (a == "--no-gitignore") { noIgnore = true; continue; }
            if (["--dir","--inp","-l","--func","--depth"].canFind(a)) {
                if (++i == args.length) throw new Exception("missing value for " ~ a);
                auto value = args[i];
                if (a == "--dir") dir = value;
                if (a == "--inp") file = value;
                if (a == "-l") forced = value;
                if (a == "--func") focus = value;
                if (a == "--depth") { depth = value.to!int; if (depth < 0) throw new Exception("--depth must be nonnegative"); }
                continue;
            }
            if (a.startsWith("-") || dir.length || file.length) throw new Exception("unexpected argument: " ~ a);
            dir = a;
        }
        if (version_) { stdout.writeln("callgraph D 0.1.0 (reference callgraph " ~ import("VERSION").strip ~ ")"); return 0; }
        if (dir.length && file.length) throw new Exception("choose either --dir or --inp");
        if (!dir.length && !file.length) throw new Exception("either --dir or --inp must be specified");
        if (file.length && !forced.length) throw new Exception("--inp mode requires -l <lang>");
        initLanguages();
        Lang language; bool valid;
        if (forced.length) {
            foreach (i; 0..Lang.max+1) if (langKey(cast(Lang)i) == forced) { language = cast(Lang)i; valid = true; }
            if (!valid) throw new Exception("unsupported language: " ~ forced);
        }
        Job[] jobs;
        if (file.length) jobs = [Job(file,language)];
        else {
            if (!exists(dir) || !isDir(dir)) throw new Exception("directory does not exist: " ~ dir);
            auto collected = collectFiles(dir,true,!noIgnore,true);
            string[] extensions;
            if (forced.length) foreach (e; languageConfig(forced)["extensions"].strings) extensions ~= e;
            foreach (j; collected.jobs) {
                if (forced.length && !extensions.canFind(fileExtension(j.path))) continue;
                if (forced.length) j.lang = language;
                jobs ~= j;
            }
        }
        Lang[] languages; foreach (j; jobs) languages ~= j.lang;
        prepareLanguages(languages); prepareGraph();
        auto parsed = new ParsedGraph[jobs.length];
        if (jobs.length < 2) foreach (i,j; jobs) parsed[i] = parseGraph(j.path,j.lang);
        else {
            auto pool = new TaskPool(7); scope(exit) pool.finish(true);
            foreach (i; pool.parallel(iota(jobs.length),1)) parsed[i] = parseGraph(jobs[i].path,jobs[i].lang);
        }
        auto graph = buildGraph(parsed,file.length != 0);
        output(graph,json,reverse_,focus,depth);
        return 0;
    } catch (Exception e) { stderr.writeln("Error: ",e.msg); return 1; }
}

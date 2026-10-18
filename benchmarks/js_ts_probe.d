module js_ts_probe;
import js_ts_cleaner;
import js_ts_model;
import javascript_port;
import typescript_port;
import std.file : readText,read;
import std.exception : assumeUnique;
import std.utf : validate;
import std.array : split;
import std.string : endsWith;
import std.json;
import std.conv : to;
import std.stdio;
import std.datetime.stopwatch : StopWatch,AutoStart;

int main(string[] args) {
    if(args.length<4) { stderr.writeln("usage: js-ts-probe config manifest repeats [--time] [--clean] [--raw]"); return 2; }
    Config config=Config(args[1]); auto paths=parseJSON(readText(args[2])).array;
    bool timed,cleanOnly,raw,headers;
    foreach(a;args[4..$]) { if(a=="--time") timed=true; if(a=="--clean") cleanOnly=true; if(a=="--raw") raw=true; if(a=="--headers") headers=true; }
    if(headers) {
        foreach(s;paths) {
            auto m=config.captures("func_pattern",s.str);
            if(!m.length) writeln("null");
            else { JSONValue[] values; foreach(v;m) values~=JSONValue(v); writeln(JSONValue(values)); }
        }
        return 0;
    }
    string[][] corpus;
    foreach(p;paths) {
        auto bytes=cast(char[])read(p.str); auto source=assumeUnique(bytes);
        // Go ranges decode each malformed byte as U+FFFD. Normalize only bad
        // input; valid source keeps its owned original UTF-8 storage.
        try { validate(source); } catch(Exception) { source=to!string(runes(source)); }
        auto lines=source.split("\n");
        if(lines.length && !lines[$-1].length) lines.length--;
        foreach(ref l;lines) if(l.endsWith("\r")) l=l[0..$-1];
        corpus~=lines;
    }
    auto results=new Result[paths.length]; auto facts=new LineFact[][paths.length]; size_t checksum;
    void pass(bool collect) {
        foreach(i,lines;corpus) {
            Cleaner cleaner; cleaner.useRaw=raw; auto fs=cleaner.facts(lines);
            if(cleanOnly) {
                if(collect) facts[i]=fs;
                foreach(f;fs) checksum+=f.cleaned.length+1;
            } else {
                auto result=config.lang=="js" ? findJavaScript(config,lines,fs) : findTypeScript(config,lines,fs);
                checksum+=result.functions.length+result.classes.length;
                if(collect) results[i]=result;
            }
        }
    }
    if(timed) pass(false);
    checksum=0;
    auto sw=StopWatch(AutoStart.yes);
    foreach(n;0..to!int(args[3])) pass(true);
    sw.stop();
    if(timed) { writeln(JSONValue(["nanoseconds":JSONValue(sw.peek.total!"nsecs"),"checksum":JSONValue(checksum)])); return 0; }
    foreach(i,p;paths) {
        JSONValue row;
        if(cleanOnly) {
            JSONValue[] rows;
            foreach(f;facts[i]) rows~=JSONValue([JSONValue(f.cleaned),JSONValue(f.startState),JSONValue(f.indent),JSONValue(f.codeText)]);
            row=JSONValue(["facts":JSONValue(rows)]);
        } else row=encode(results[i]);
        row["path"]=p; writeln(row);
    }
    return 0;
}

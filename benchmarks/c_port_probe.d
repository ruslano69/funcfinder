module c_port_probe;
import c_port;
import brace_port_cleaner;
import brace_port_model : Result,encode;
import brace_port_unicode : unicodeVersion;
import std.file : read;
import std.exception : assumeUnique;
import std.utf : validate;
import std.array : split;
import std.string : endsWith;
import std.json;
import std.conv : to;
import std.stdio;
import std.datetime.stopwatch : StopWatch,AutoStart;

int main(string[] args) {
    if(args.length<4){stderr.writeln("usage: c-port-probe config manifest repeats [--time] [--clean] [--raw]");return 2;}
    auto config=parseJSON(cast(string)read(args[1]));
    if(config["lang"].str!="c" || config["unicode_version"].str!=unicodeVersion)throw new Exception("unsupported configuration");
    foreach(k,v;["line_comment":"//","block_comment_start":"/*","block_comment_end":"*/","escape_char":"\\"])
        if(config[k].str!=v)throw new Exception("unsupported lexical setting: "~k);
    if(config["string_chars"].array!=[JSONValue("\""),JSONValue("'")])throw new Exception("unsupported string delimiters");
    foreach(k;["raw_string_chars","char_delimiters","doc_string_markers"])
        if(auto p=k in config.object)if(p.array.length)throw new Exception("unsupported delimiters: "~k);
    foreach(k;["single_line_strings","template_expressions","regex_literals","char_literal_lookahead"])
        if(auto p=k in config.object)if(p.type==JSONType.true_)throw new Exception("unsupported lexical mode: "~k);
    auto paths=parseJSON(cast(string)read(args[2])).array;bool timed,cleanOnly,raw;
    foreach(a;args[4..$]){if(a=="--time")timed=true;else if(a=="--clean")cleanOnly=true;else if(a=="--raw")raw=true;else throw new Exception("unknown option");}
    string[][] corpus;
    foreach(p;paths){auto source=assumeUnique(cast(char[])read(p.str));try{validate(source);}catch(Exception){source=to!string(runes(source));}auto lines=source.split("\n");if(lines.length && !lines[$-1].length)lines.length--;foreach(ref line;lines)if(line.endsWith("\r"))line=line[0..$-1];corpus~=lines;}
    auto results=new Result[paths.length];auto facts=new LineFact[][paths.length];size_t checksum;
    void pass(bool collect){foreach(i,lines;corpus){Cleaner cleaner;cleaner.useRaw=raw;auto fs=cleaner.facts(lines);
        if(cleanOnly){if(collect)facts[i]=fs;foreach(f;fs)checksum+=f.cleaned.length+1;}
        else{auto result=findC(lines,fs);checksum+=result.functions.length+result.classes.length;if(collect)results[i]=result;}
    }}
    if(timed)pass(false);checksum=0;auto sw=StopWatch(AutoStart.yes);foreach(n;0..to!int(args[3]))pass(true);sw.stop();
    if(timed){writeln(JSONValue(["nanoseconds":JSONValue(sw.peek.total!"nsecs"),"checksum":JSONValue(checksum)]));return 0;}
    foreach(i,p;paths){JSONValue row;if(cleanOnly){JSONValue[] rows;foreach(f;facts[i])rows~=JSONValue([JSONValue(f.cleaned),JSONValue(f.startState),JSONValue(f.indent),JSONValue(f.codeText)]);row=JSONValue(["facts":JSONValue(rows)]);}else row=encode(results[i]);row["path"]=p;writeln(row);}return 0;
}

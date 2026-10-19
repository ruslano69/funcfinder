module rust_port_probe;
import rust_port;
import brace_port_cleaner : runes;
import brace_port_model : Result,encode;
import brace_port_unicode : unicodeVersion;
import std.file : read;
import std.exception : assumeUnique;
import std.utf : validate;
import std.json;
import std.conv : to;
import std.stdio;
import std.datetime.stopwatch : StopWatch,AutoStart;

int main(string[] args) {
    if(args.length<4){stderr.writeln("usage: rust-port-probe config manifest repeats [--time]");return 2;}
    auto config=parseJSON(cast(string)read(args[1]));
    if(config["lang"].str!="rust" || config["unicode_version"].str!=unicodeVersion)throw new Exception("unsupported configuration");
    foreach(k,v;["line_comment":"//","block_comment_start":"/*","block_comment_end":"*/","escape_char":"\\"])
        if(config[k].str!=v)throw new Exception("unsupported lexical setting: "~k);
    if(config["string_chars"].array!=[JSONValue("\"")])throw new Exception("unsupported strings");
    if(config["char_delimiters"].array!=[JSONValue("'")] || config["char_literal_lookahead"].type!=JSONType.true_)throw new Exception("unsupported character literals");
    string[] raw;foreach(v;config["raw_string_chars"].array)raw~=v.str;
    if(raw!=["r\"","r#\"","r##\"","r###\"","r####\""])throw new Exception("unsupported raw string configuration");
    foreach(k;["single_line_strings","template_expressions","regex_literals"])
        if(auto p=k in config.object)if(p.type==JSONType.true_)throw new Exception("unsupported lexical mode: "~k);
    auto paths=parseJSON(cast(string)read(args[2])).array;bool timed;
    foreach(a;args[4..$]){if(a=="--time")timed=true;else throw new Exception("unknown option");}
    string[] corpus;
    foreach(p;paths){auto source=assumeUnique(cast(char[])read(p.str));try{validate(source);}catch(Exception){source=to!string(runes(source));}corpus~=source;}
    auto results=new Result[paths.length];size_t checksum;
    void pass(bool collect){foreach(i,source;corpus){auto result=findRust(source);checksum+=result.functions.length+result.classes.length;if(collect)results[i]=result;}}
    if(timed)pass(false);checksum=0;auto sw=StopWatch(AutoStart.yes);foreach(n;0..to!int(args[3]))pass(true);sw.stop();
    if(timed){writeln(JSONValue(["nanoseconds":JSONValue(sw.peek.total!"nsecs"),"checksum":JSONValue(checksum)]));return 0;}
    foreach(i,p;paths){auto row=encode(results[i]);row["path"]=p;writeln(row);}return 0;
}

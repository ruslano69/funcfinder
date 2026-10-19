module js_ts_model;
import js_ts_cleaner;
import std.json;
import std.file : readText;
import std.regex;
import std.algorithm : canFind;
import std.string : strip, startsWith, endsWith, replace;
import typescript_header;

struct Bounds { string name; int start,end; string className; int signatureLine; string signatureName; }
struct Result { Bounds[] functions,classes; }
struct Config {
    string lang; JSONValue data; Regex!char[string] patterns;
    bool declarationScanner,declaratorScanner; // typescript_header.declarationMatch / declaratorMatch stand for the patterns
    this(string path) { this(parseJSON(readText(path))); }
    this(JSONValue source) {
        data=source; lang=data["lang"].str;
        if(lang!="js" && lang!="ts") throw new Exception("unsupported language");
        foreach(key,value;["line_comment":"//","block_comment_start":"/*","block_comment_end":"*/","escape_char":"\\"])
            if(data[key].str!=value) throw new Exception("unsupported lexical configuration: " ~ key);
        foreach(key;["regex_literals","template_expressions","single_line_strings"])
            if(data[key].type!=JSONType.true_) throw new Exception("unsupported lexical configuration: " ~ key);
        if(data["string_chars"].array.length!=2 || data["string_chars"][0].str!="\"" || data["string_chars"][1].str!="'" || data["raw_string_chars"].array.length!=1 || data["raw_string_chars"][0].str!="`")
            throw new Exception("unsupported string delimiters");
        if(lang=="ts" && data["func_pattern"].str!=typescript_header.functionPattern)
            throw new Exception("TypeScript header pattern differs from scanner");
        if(lang=="ts") {
            declarationScanner=data["declaration_pattern"].str==typescript_header.declarationPattern;
            declaratorScanner=data["declarator_pattern"].str==typescript_header.declaratorPattern;
        }
        foreach(key;["func_pattern","class_pattern","class_method_open","header_open_pattern","open_type_header_pattern","declaration_pattern","declarator_pattern","quoted_method_pattern"])
            if(auto p=key in data.object) if(p.type==JSONType.string && p.str.length) {
                // A pattern the scanners stand for is not compiled (only the audit build compares them with it).
                version(AuditTSHeader) {} else {
                    if(key=="declaration_pattern" && declarationScanner) continue;
                    if(key=="declarator_pattern" && declaratorScanner) continue;
                }
                patterns[key]=regex(p.str.replace(`\x{200c}`,"\u200c").replace(`\x{200d}`,"\u200d"));
            }
    }
    string[] captures(string key,string text) {
        if(lang=="ts" && key=="func_pattern") {
            auto result=typescript_header.headerMatch(text);
            version(AuditTSHeader) {
                auto p=key in patterns; auto m=matchFirst(text,*p); string[] expected;
                if(!m.empty) foreach(c;m.captures) expected~=c;
                assert(result==expected,"TypeScript header scanner mismatch: " ~ text);
            }
            return result;
        }
        if(declarationScanner && key=="declaration_pattern" || declaratorScanner && key=="declarator_pattern") {
            auto result=key=="declaration_pattern" ? typescript_header.declarationMatch(text) : typescript_header.declaratorMatch(text);
            version(AuditTSHeader) {
                auto p=key in patterns; auto m=matchFirst(text,*p); string[] expected;
                if(!m.empty) foreach(c;m.captures) expected~=c;
                assert(result==expected,"TypeScript " ~ key ~ " scanner mismatch: " ~ text);
            }
            return result;
        }
        auto p=key in patterns; if(!p) return null;
        auto m=matchFirst(text,*p); if(m.empty) return null;
        string[] result; foreach(c;m.captures) result~=c; return result;
    }
    bool containsAny(string key,string text) {
        auto p=key in data.object; if(!p || p.type!=JSONType.array || !p.array.length) return true;
        foreach(w;p.array) if(text.canFind(w.str)) return true; return false;
    }
    bool excluded(string name) {
        if(auto p="exclude_words" in data.object) foreach(w;p.array) if(w.str==name) return true;
        return false;
    }
}
string capturedName(string[] matches) {
    if(matches.length>5) { if(matches[3].length) return matches[3]; if(matches[5].length) return matches[5]; }
    for(auto i=matches.length;i>1;--i) if(matches[i-1].length) return matches[i-1];
    return "";
}
int braces(string s) { int n; foreach(c;s) { if(c=='{') ++n; else if(c=='}') --n; } return n; }
int parens(string s) { int n; foreach(c;s) { if(c=='(' || c=='[') ++n; else if(c==')' || c==']') --n; } return n; }
string classForLine(Bounds[] classes,int line) {
    foreach(c;classes) if(c.start<=line+1 && c.end>=line+1) return c.name;
    return "";
}
bool classContinues(LineFact[] facts,size_t i,string s) {
    int angles; foreach(c;s) { if(c=='<') ++angles; if(c=='>') --angles; }
    if(angles>0 || parens(s)>0) return true;
    auto t=s.strip;
    if(t.endsWith(",") || t.endsWith(":") || t.endsWith("extends") || t.endsWith("implements")) return true;
    foreach(f;facts[i+1..$]) if(f.cleaned.strip.length) {
        t=f.cleaned.strip; return t.startsWith("{") || t.startsWith(":") || t.startsWith("extends") || t.startsWith("implements");
    }
    return false;
}
Bounds[] findClasses(ref Config config,LineFact[] facts) {
    Bounds[] result; Bounds current; bool active,opened; int depth;
    foreach(i,f;facts) {
        auto text=f.cleaned;
        if(active) {
            if(text.canFind('{')) opened=true;
            depth+=braces(text);
            if(opened && depth<=0) { current.end=cast(int)i+1; result~=current; active=false; depth=0; opened=false; }
        } else {
            auto m=config.containsAny("type_keywords",text) ? config.captures("class_pattern",text) : null;
            if(m.length<2) continue;
            if(!text.canFind('{') && text.canFind(';')) continue;
            current=Bounds(m[1],cast(int)i+1,0); active=true; depth=0;
            if(text.canFind('{')) {
                opened=true; depth=braces(text);
                if(depth<=0) { current.end=cast(int)i+1; result~=current; active=false; depth=0; opened=false; }
            } else if(!classContinues(facts,i,text)) { current.end=cast(int)i+1; result~=current; active=false; depth=0; }
        }
    }
    if(active) { current.end=cast(int)facts.length; result~=current; }
    return result;
}
JSONValue encode(Result result) {
    JSONValue[] fs,cs;
    foreach(f;result.functions) fs~=JSONValue([JSONValue(f.name),JSONValue(f.start),JSONValue(f.end),JSONValue(f.className),JSONValue(f.signatureLine),JSONValue(f.signatureName)]);
    foreach(c;result.classes) cs~=JSONValue([JSONValue(c.name),JSONValue(c.start),JSONValue(c.end)]);
    return JSONValue(["functions":JSONValue(fs),"classes":JSONValue(cs)]);
}

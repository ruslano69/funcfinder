module brace_port_model;
import brace_port_cleaner;
import std.json;
import std.file : readText;
import std.regex;
import std.string : strip,startsWith,endsWith;
import std.algorithm : canFind;
import brace_port_unicode : unicodeVersion,letterClass,digitClass;

// RE2's Perl classes and word boundaries use ASCII, unlike std.regex.
Regex!char goRegex(string source) {
    string text;bool inClass;
    for(size_t i=0;i<source.length;++i){auto c=source[i];
        if(c=='\\' && i+1<source.length){auto next=source[++i];string content;
            if(next=='p' && source[i..$].startsWith("p{L}")){content=letterClass;i+=3;}
            else if(next=='p' && source[i..$].startsWith("p{Nd}")){content=digitClass;i+=4;}
            else if(next=='w')content="A-Za-z0-9_";else if(next=='s')content=`\t\n\f\r `;else if(next=='d')content="0-9";
            if(content.length){text~=inClass?content:"["~content~"]";continue;}
            if(!inClass && next=='b'){text~=`(?:(?<![A-Za-z0-9_])(?=[A-Za-z0-9_])|(?<=[A-Za-z0-9_])(?![A-Za-z0-9_]))`;continue;}
            text~='\\';text~=next;continue;
        }
        if(c=='[')inClass=true;else if(c==']')inClass=false;
        if(c=='$' && !inClass)text~=`$(?![\s\S])`;else text~=c;
    }
    return regex(text);
}

struct Bounds {string name;int start,end;string className;int signatureLine;string signatureName;}
struct Result {Bounds[] functions,classes;}
struct Config {
    string lang;JSONValue data;Regex!char[string] patterns;
    this(string path,bool compilePatterns=true) { this(parseJSON(readText(path)),compilePatterns); }
    this(JSONValue source,bool compilePatterns=true) {
        data=source;lang=data["lang"].str;
        if(!["cs","cpp","go","java"].canFind(lang)) throw new Exception("unsupported language");
        if(data["unicode_version"].str!=unicodeVersion) throw new Exception("unsupported Go Unicode version");
        foreach(k,v;["line_comment":"//","block_comment_start":"/*","block_comment_end":"*/","escape_char":"\\"])
            if(data[k].str!=v) throw new Exception("unsupported lexical configuration: "~k);
        foreach(k;["single_line_strings","template_expressions","regex_literals","char_literal_lookahead"])
            if(auto p=k in data.object) if(p.type==JSONType.true_) throw new Exception("unsupported lexical configuration: "~k);
        string[] strings,raw,chars;foreach(v;data["string_chars"].array) strings~=v.str;
        if(auto p="raw_string_chars" in data.object) foreach(v;p.array) raw~=v.str;
        if(auto p="char_delimiters" in data.object) foreach(v;p.array) chars~=v.str;
        string[] expectedStrings=lang=="cpp"?["\"","'"]:["\""];
        string[] expectedRaw=lang=="cs"?["@\""]:lang=="go"?["`"]:null;
        string[] expectedChars=lang=="cpp"?null:["'"];
        if(strings != expectedStrings || raw != expectedRaw || chars != expectedChars) throw new Exception("unsupported string delimiters");
        if(auto p="doc_string_markers" in data.object) if(p.array.length) throw new Exception("unsupported doc string markers");
        if(!compilePatterns) return;
        foreach(k;["func_pattern","class_pattern","header_open_pattern","constructor_pattern","declaration_pattern","member_declaration_pattern","conversion_operator_pattern","probe_pattern"])
            if(auto p=k in data.object) if(p.type==JSONType.string && p.str.length) patterns[k]=goRegex(p.str);
        patterns["decl_start"]=goRegex(`^(?:@|(?:(?:public|private|protected|internal|override|open|abstract|final|inline|suspend|const|lateinit|external|operator|infix|data|sealed|enum|companion|inner|static|mutating|lazy)\s+)*(?:val|var|fun|func|class|object|interface|typealias|init|constructor|struct|protocol|extension)\b)`);
        patterns["anonymous_object"]=goRegex(`^object\s*:`);
        patterns["continuation"]=goRegex(`^(?:\.|\?[.:]|else\b)`);
        patterns["template_prefix"]=goRegex(`^\s*template\s*<.*?>\s*`);
        patterns["leading_word"]=goRegex(`^\s*([A-Za-z_]\w*)`);
    }
    string[] captures(string key,string text) {auto p=key in patterns;if(!p) return null;auto m=matchFirst(text,*p);if(m.empty)return null;string[] result;foreach(c;m.captures) result~=c;return result;}
    bool containsAny(string key,string text) {auto p=key in data.object;if(!p || !p.array.length)return true;foreach(v;p.array) if(text.canFind(v.str))return true;return false;}
    bool listed(string key,string text) {auto p=key in data.object;if(p)foreach(v;p.array)if(v.str==text)return true;return false;}
    bool declaration(string text) {return captures("decl_start",text).length && !captures("anonymous_object",text).length;}
    bool continues(string[] lines,size_t i) {foreach(line;lines[i+1..$])if(line.strip.length)return captures("continuation",line.strip).length!=0;return false;}
}
string capturedName(string[] m) {if(m.length>5){if(m[3].length)return m[3];if(m[5].length)return m[5];}for(auto i=m.length;i>1;--i)if(m[i-1].length)return m[i-1];return "";}
int braces(string s) {int n;foreach(c;s){if(c=='{')++n;else if(c=='}')--n;}return n;}
int parens(string s) {int n;foreach(c;s){if(c=='(' || c=='[')++n;else if(c==')' || c==']')--n;}return n;}
string classForLine(Bounds[] cs,int line) {foreach(c;cs)if(c.start<=line+1 && c.end>=line+1)return c.name;return "";}
bool classContinues(LineFact[] facts,size_t i,string s) {
    int angles;foreach(c;s){if(c=='<')++angles;if(c=='>')--angles;}
    if(angles>0 || parens(s)>0)return true;auto t=s.strip;
    if(t.endsWith(",") || t.endsWith(":") || t.endsWith("extends") || t.endsWith("implements"))return true;
    foreach(f;facts[i+1..$])if(f.cleaned.strip.length){t=f.cleaned.strip;return t.startsWith("{") || t.startsWith(":") || t.startsWith("extends") || t.startsWith("implements");}return false;
}
Bounds[] findClasses(ref Config config,LineFact[] facts) {
    Bounds[] result;Bounds current;bool active,opened;int depth;
    foreach(i,f;facts) {auto text=f.cleaned;
        if(active){if(text.canFind('{'))opened=true;depth+=braces(text);if(opened && depth<=0){current.end=cast(int)i+1;result~=current;active=false;depth=0;opened=false;}}
        else {auto m=config.containsAny("type_keywords",text)?config.captures("class_pattern",text):null;if(m.length<2)continue;if(!text.canFind('{') && text.canFind(';'))continue;
            current=Bounds(m[1],cast(int)i+1,0);active=true;depth=0;
            if(text.canFind('{')){opened=true;depth=braces(text);if(depth<=0){current.end=cast(int)i+1;result~=current;active=false;depth=0;opened=false;}}
            else if(!classContinues(facts,i,text)){current.end=cast(int)i+1;result~=current;active=false;depth=0;}
        }
    }
    if(active){current.end=cast(int)facts.length;result~=current;}return result;
}
JSONValue encode(Result r) {JSONValue[] fs,cs;foreach(f;r.functions)fs~=JSONValue([JSONValue(f.name),JSONValue(f.start),JSONValue(f.end),JSONValue(f.className),JSONValue(f.signatureLine),JSONValue(f.signatureName)]);foreach(c;r.classes)cs~=JSONValue([JSONValue(c.name),JSONValue(c.start),JSONValue(c.end)]);return JSONValue(["functions":JSONValue(fs),"classes":JSONValue(cs)]);}

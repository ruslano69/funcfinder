module brace_port_cleaner;
import std.string : strip,startsWith;
import std.utf : decode,UTFException;
import std.conv : to;
import reuse : recycled, keep;

struct LineFact { string cleaned;int startState,indent;bool codeText; }
dchar nextRune(string text,ref size_t i) {
    if(cast(ubyte)text[i]<128) return text[i++];
    auto start=i;try{return decode(text,i);}catch(UTFException){i=start+1;return '\uFFFD';}
}
dchar[] runes(string s) {dchar[] result;size_t i;while(i<s.length) result~=nextRune(s,i);return result;}
bool ascii(string s) {foreach(c;s) if(cast(ubyte)c>=128) return false;return true;}
// Blocks kept from the previous file on this thread (reuse.d): the facts array, the scratch of the cleaner and the arena
// that holds the cleaned lines. Cleaned text and facts are valid until the next facts() call on the same thread.
private LineFact[] factPool;
private char[] arenaPool, bytesPool;
private dchar[] charsPool;
struct Cleaner {
    bool csharp,useRaw,golang,charLiterals;int state;dchar delimiter;char[] bytes;dchar[] chars;
    char[] arena;size_t arenaUsed;
    // The cleaned text of a line: a copy in the arena while it has room (facts() sizes it), else on the heap.
    string own(const(char)[] s) {
        if(arenaUsed+s.length>arena.length) return s.idup;
        auto dest=arena[arenaUsed..arenaUsed+s.length];dest[]=s[];arenaUsed+=s.length;return cast(string)dest;
    }
    string clean(string line) {
        version(AuditBraceCleaner) {auto slow=this;slow.bytes=null;slow.chars=null;auto expected=slow.units!dchar(runes(line));}
        auto text=ascii(line)?units!char(line):units!dchar(runes(line));
        version(AuditBraceCleaner) {assert(text==expected && state==slow.state && delimiter==slow.delimiter,"brace cleaner fast-path mismatch");}
        return text;
    }
    string units(T)(const(T)[] s) {
        static if(is(T==char)) {if(bytes.length<s.length) bytes.length=s.length;auto result=bytes[0..s.length];}
        else {if(chars.length<s.length) chars.length=s.length;auto result=chars[0..s.length];}
        result[]=' ';size_t i;
        while(i<s.length) {
            if(state==2) {if(i+1<s.length && s[i]=='*' && s[i+1]=='/') {i+=2;state=0;}else ++i;continue;}
            if(state==3 || state==5) {
                if(s[i]=='\\' && i+1<s.length) i+=2;
                else {if(s[i]==delimiter) {state=0;delimiter=0;}++i;}continue;
            }
            if(state==4) {
                if(golang) {if(s[i]=='`'){++i;state=0;delimiter=0;}else ++i;continue;}
                if(s[i]=='"') {if(i+1<s.length && s[i+1]=='"') i+=2;else {++i;state=0;delimiter=0;}}
                else ++i;continue;
            }
            if(s[i]=='/' && i+1<s.length) {
                if(s[i+1]=='/') break;
                if(s[i+1]=='*') {
                    int depth=1;auto j=i+2;
                    while(j<s.length) {
                        if(j+1<s.length && s[j]=='/' && s[j+1]=='*') {++depth;j+=2;}
                        else if(j+1<s.length && s[j]=='*' && s[j+1]=='/') {--depth;j+=2;if(!depth) break;}
                        else ++j;
                    }
                    if(depth) state=2;i=j;continue;
                }
            }
            if(csharp && !useRaw && s[i]=='@' && i+1<s.length && s[i+1]=='"') {state=4;delimiter='"';i+=2;continue;}
            if(golang && !useRaw && s[i]=='`'){state=4;delimiter='`';++i;continue;}
            if(s[i]=='"' || s[i]=='\'') {delimiter=s[i];state=(csharp || charLiterals) && s[i]=='\''?5:3;++i;continue;}
            result[i]=s[i];++i;
        }
        static if(is(T==char)) return own(result);
        else return to!string(result);
    }
    LineFact[] facts(string[] lines) {
        size_t need;foreach(line;lines)need+=line.length;
        if(arenaPool.length<need)arenaPool=new char[need+need/2+4096];
        arena=arenaPool;arenaUsed=0;bytes=bytesPool;chars=charsPool;
        scope(exit){bytesPool=bytes;charsPool=chars;}
        if(factPool.length<lines.length)factPool=new LineFact[lines.length+lines.length/2+64];
        auto output=factPool[0..lines.length];
        foreach(i,line;lines) {auto f=LineFact();f.startState=state;foreach(c;line){if(c==' ') ++f.indent;else if(c=='\t') f.indent+=4;else break;}auto t=line.strip;f.codeText=t.length && !t.startsWith("//") && !t.startsWith("/*");f.cleaned=clean(line);output[i]=f;}
        return output;
    }
}

// Experimental JS/TS lexical port. Frozen Go Sanitizer semantics are the oracle.
module js_ts_cleaner;
import std.algorithm : canFind, max;
import std.conv : to;
import std.string : strip, startsWith;
import std.utf : decode;
import std.typecons : Yes;
import std.exception : assumeUnique;
import std.uni : unicode,CodepointSet,isWhite;

private __gshared CodepointSet templateLetters,templateDigits,templateMarks;
shared static this() { templateLetters=unicode.L;templateDigits=unicode.Nd;templateMarks=unicode.M; }
private bool templateWord(dchar c) {
    if(c<128) return wordChar(c) || (c>='0' && c<='9');
    return templateLetters[c] || templateDigits[c] || templateMarks[c] || c==0x200c || c==0x200d;
}
private bool regexKeyword(string word) {
    switch(word) {case "return":case "typeof":case "case":case "in":case "of":case "yield":case "await":case "if":case "elsif":case "unless":case "when":return true;default:return false;}
}
ptrdiff_t regexEnd(T)(const(T)[] s,size_t i) {
    bool inClass;
    for(auto j=i+1;j<s.length;++j) {
        auto c=s[j];
        if(c=='\n' || c=='\r') return -1;
        if(c=='\\') ++j;
        else if(inClass) {if(c==']') inClass=false;}
        else if(c=='[') inClass=true;
        else if(c=='/') {
            ++j;while(j<s.length && "dgimsuvy"d.canFind(cast(dchar)s[j])) ++j;
            return j<s.length && templateWord(s[j]) ? -1 : cast(ptrdiff_t)j;
        }
    }
    return -1;
}
struct TemplateFrame { dchar kind;int braces;bool operand; }
struct TemplateScanner {
    TemplateFrame[] frames;
    size_t feed(T)(const(T)[] s,size_t i,out bool closed) {
        while(i<s.length && frames.length) {
            ref f=frames[$-1];auto c=s[i];
            switch(f.kind) {
                case '`':
                    if(c=='\\') {i=i+2<s.length ? i+2 : s.length;continue;}
                    if(c=='`') {frames.length--; ++i;continue;}
                    if(c=='$' && i+1<s.length && s[i+1]=='{') {frames~=TemplateFrame('{',0,true);i+=2;continue;}
                    break;
                case '\'':case '"':
                    if(c=='\\') {i=i+2<s.length ? i+2 : s.length;continue;}
                    if(c==f.kind) frames.length--;
                    break;
                case '*':
                    if(c=='*' && i+1<s.length && s[i+1]=='/') {frames.length--;i+=2;continue;}
                    break;
                case '{':
                    if(isWhite(cast(dchar)c)) {++i;continue;}
                    if(templateWord(c)) {auto start=i;while(i<s.length && templateWord(s[i])) ++i;f.operand=regexKeyword(to!string(s[start..i]));continue;}
                    switch(c) {
                        case '\'':case '"':case '`':f.operand=false;frames~=TemplateFrame(c);break;
                        case '{':++f.braces;f.operand=true;break;
                        case '}':if(!f.braces) frames.length--;else {--f.braces;f.operand=false;}break;
                        case '/':
                            if(i+1<s.length && s[i+1]=='/') {while(i<s.length && s[i]!='\n') ++i;continue;}
                            if(i+1<s.length && s[i+1]=='*') {frames~=TemplateFrame('*');i+=2;continue;}
                            if(f.operand) {auto end=regexEnd(s,i);if(end>=0) {f.operand=false;i=end;continue;}}
                            f.operand=true;break;
                        default:f.operand="(,=:[!&|?{};+-*%<>~^"d.canFind(cast(dchar)c);break;
                    }
                    break;
                default:break;
            }
            ++i;
        }
        closed=!frames.length;return i;
    }
}

enum LexState { normal=0, block=2, quoted=3, raw=4 }
struct LineFact { string cleaned; int startState, indent; bool codeText; }
T[] span(T)(ref T[] storage,size_t n) {
    if(storage.length<n) storage.length=max(n,max(cast(size_t)32,storage.length*2));
    return storage[0..n];
}
bool ascii(string s) @nogc nothrow { foreach(c;s) if(c>=128) return false; return true; }
// Go range consumes one byte for each invalid UTF-8 encoding unit.
dchar nextRune(string s,ref size_t i) {
    auto first=i; auto r=decode!(Yes.useReplacementDchar)(s,i);
    if(r==0xfffd && !(first+3<=s.length && s[first..first+3]=="\xef\xbf\xbd")) i=first+1;
    return r;
}
dstring runes(string s) {
    auto result=new dchar[s.length]; size_t i,n;
    while(i<s.length) result[n++]=nextRune(s,i);
    result.length=n;
    // Fresh storage has no aliases; transfer ownership without a second copy.
    return assumeUnique(result);
}
string owned(T)(T[] s) { static if(is(T==char)) return s.idup; else return to!string(s); }
size_t quoteEnd(T)(const(T)[] s,size_t i) {
    auto q=s[i];
    for(auto j=i+1;j<s.length;++j) {
        if(s[j]=='\\') ++j;
        else if(s[j]==q) return j+1;
    }
    return s.length;
}
ptrdiff_t templateExprEnd(T)(const(T)[] s,size_t i) {
    TemplateScanner scanner;scanner.frames~=TemplateFrame('{',0,true);bool closed;
    auto end=scanner.feed(s,i,closed);return closed ? cast(ptrdiff_t)end : -1;
}
ptrdiff_t templateEnd(T)(const(T)[] s,size_t i) {
    TemplateScanner scanner;scanner.frames~=TemplateFrame('`');bool closed;
    auto end=scanner.feed(s,i,closed);return closed ? cast(ptrdiff_t)end : -1;
}
bool wordChar(dchar c) @nogc nothrow {
    return c=='_' || c=='$' || (c>='a' && c<='z') || (c>='A' && c<='Z');
}
bool endsAttribute(T)(const(T)[] result,size_t i) {
    ptrdiff_t k=cast(ptrdiff_t)i-1;
    while(k>=0 && (result[k]==' ' || result[k]=='\t')) --k;
    if(k<0) return false;
    auto c=result[k];
    return c=='}' || c=='"' || c=='\'' || c=='`' || c==']' || wordChar(c) || (c>='0' && c<='9');
}
bool regexAllowed(T)(const(T)[] result,size_t i) {
    if(i && result[i-1]=='<') return false;
    ptrdiff_t k=cast(ptrdiff_t)i-1;
    while(k>=0 && (result[k]==' ' || result[k]=='\t')) --k;
    if(k<0) return true;
    if("(,=:[!&|?{};+-*%<>~^"d.canFind(cast(dchar)result[k])) return true;
    auto end=k+1;
    while(k>=0 && wordChar(result[k])) --k;
    auto word=to!string(result[k+1..end]);
    switch(word) {
        case "return": case "typeof": case "case": case "in": case "of": case "yield":
        case "await": case "if": case "elsif": case "unless": case "when": return true;
        default: return false;
    }
}
struct Cleaner {
    LexState state; dchar delimiter=0; bool useRaw;
    char[] byteScratch; dchar[] inputScratch,runeScratch;
    TemplateScanner templateScan;
    LineFact fact(string line) {
        LineFact f; f.startState=state;
        foreach(c;line) { if(c==' ') ++f.indent; else if(c=='\t') f.indent+=4; else break; }
        auto text=line.strip; f.codeText=text.length && !text.startsWith("//") && !text.startsWith("/*");
        f.cleaned=clean(line); return f;
    }
    string clean(string line) {
        if(!line.length) return line;
        bool marker;
        foreach(c;line) if(c=='"' || c=='\'' || c=='`' || c=='/') { marker=true; break; }
        if(state==LexState.normal && !marker) {
            // Decode only to normalize invalid UTF-8, matching Go's slow path.
            import std.utf : validate;
            try { validate(line); return line; } catch(Exception) {}
        }
        if(ascii(line)) return units!char(line,byteScratch);
        auto input=span(inputScratch,line.length); size_t i,n;
        while(i<line.length) input[n++]=nextRune(line,i);
        return units!dchar(input[0..n],runeScratch);
    }
    string units(T)(const(T)[] s,ref T[] scratch) {
        auto outText=span(scratch,s.length); outText[]=' ';
        size_t i;
        while(i<s.length) {
            if(state==LexState.block) {
                while(i+1<s.length && !(s[i]=='*' && s[i+1]=='/')) ++i;
                if(i+1<s.length) { i+=2; state=LexState.normal; } else i=s.length;
                continue;
            }
            if(state==LexState.quoted) {
                if(s[i]=='\\' && i+1<s.length) i+=2;
                else { if(s[i]==delimiter) { delimiter=0; state=LexState.normal; } ++i; }
                continue;
            }
            if(state==LexState.raw) {
                if(!templateScan.frames.length) templateScan.frames~=TemplateFrame('`');bool closed;
                auto end=templateScan.feed(s,i,closed);if(useRaw) outText[i..end]=s[i..end];i=end;
                if(closed) {delimiter=0;state=LexState.normal;}continue;
            }
            if(s[i]=='/' && i+1<s.length) {
                if(s[i+1]=='*') {
                    auto j=i+2; int depth=1;
                    while(j<s.length) {
                        if(j+1<s.length && s[j]=='*' && s[j+1]=='/') { --depth; j+=2; break; }
                        else ++j;
                    }
                    if(depth) state=LexState.block;
                    i=j; continue;
                }
                if(s[i+1]=='/') break;
                if(!(s[i+1]=='>' && endsAttribute(outText,i)) && regexAllowed(outText,i)) {
                    auto end=regexEnd(s,i);if(end>=0) {i=end;continue;}
                }
            }
            if(s[i]=='`' && !useRaw) { delimiter='`'; state=LexState.raw;templateScan.frames.length=0; ++i; continue; }
            if(s[i]=='\'' || s[i]=='"') { delimiter=s[i]; state=LexState.quoted; ++i; continue; }
            outText[i]=s[i]; ++i;
        }
        if(state==LexState.quoted) { state=LexState.normal; delimiter=0; }
        return owned(outText);
    }
    LineFact[] facts(string[] lines) {
        auto result=new LineFact[lines.length];
        foreach(i,line;lines) result[i]=fact(line);
        return result;
    }
}

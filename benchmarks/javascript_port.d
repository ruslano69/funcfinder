module javascript_port;
import js_ts_cleaner;
import js_ts_model;
import std.algorithm : canFind, sort;
import std.array : join,appender;
import std.string : startsWith;
import std.uni : unicode, CodepointSet, isWhite;
import std.conv : to;
import std.utf : validate;

private __gshared CodepointSet letters,digits,marks;
shared static this() { letters=unicode.L; digits=unicode.Nd; marks=unicode.M; }
struct Token { string text; int line,end; bool literal; }
bool jsWord(dchar c) {
    bool found=c<128 ? c=='_' || c=='$' || (c>='a' && c<='z') || (c>='A' && c<='Z') || (c>='0' && c<='9')
                    : letters[c] || digits[c] || marks[c] || c==0x200c || c==0x200d;
    version(AuditJSTokens) assert(found==(c=='_' || c=='$' || letters[c] || digits[c] || marks[c] || c==0x200c || c==0x200d));
    return found;
}
private bool decimal(dchar c) {
    bool found=c<128 ? c>='0' && c<='9' : digits[c];
    version(AuditJSTokens) assert(found==digits[c]);
    return found;
}
private bool whitespace(dchar c) {
    bool found=c<128 ? c==' ' || (c>='\t' && c<='\r') : isWhite(c);
    version(AuditJSTokens) assert(found==isWhite(c));
    return found;
}
bool identifier(Token t) {
    if(t.literal || !t.text.length) return false;
    foreach(i,dchar c;t.text) if(!jsWord(c) || (!i && decimal(c))) return false;
    return true;
}
Token[] tokenize(string[] lines,LineFact[] facts,out int[] pairs) {
    auto result=tokenizeFast(lines,facts,pairs);
    version(AuditJSTokens) {
        auto raw=lines.join("\n");string[] cleaned;foreach(f;facts) cleaned~=f.cleaned;
        int[] expectedPairs;
        auto expected=tokenizeUnits!(dchar,false)(runes(raw),runes(cleaned.join("\n")),expectedPairs);
        assert(result==expected,"JS token text/line/literal mismatch");
        assert(pairs==expectedPairs,"JS bracket-pair mismatch");
    }
    return result;
}
private Token[] tokenizeFast(string[] lines,LineFact[] facts,out int[] pairs) {
    auto rawBytes=lines.join("\n"); string[] cleaned;
    foreach(f;facts) cleaned~=f.cleaned;
    auto cleanBytes=cleaned.join("\n");
    if(ascii(rawBytes) && ascii(cleanBytes)) return tokenizeUnits!char(rawBytes,cleanBytes,pairs);
    auto rawRunes=runes(rawBytes),cleanRunes=runes(cleanBytes);
    // The probe supplies valid UTF-8. Preserve the API's replacement behavior
    // for direct callers too before retaining slices of the joined strings.
    try { validate(rawBytes); } catch(Exception) { rawBytes=to!string(rawRunes); }
    try { validate(cleanBytes); } catch(Exception) { cleanBytes=to!string(cleanRunes); }
    return tokenizeUnits!dchar(rawRunes,cleanRunes,pairs,rawBytes,cleanBytes);
}
private size_t utf8Width(dchar c) { return c<0x80 ? 1 : c<0x800 ? 2 : c<0x10000 ? 3 : 4; }
private Token[] tokenizeUnits(T,bool retainSlices=true)(immutable(T)[] raw,immutable(T)[] clean,out int[] pairs,string rawUtf8=null,string cleanUtf8=null) {
    assert(raw.length==clean.length,"rune coordinate mismatch");
    auto tokens=appender!(Token[])(); auto paired=appender!(int[])(); int[] stack; int line;
    size_t rawByte,cleanByte;
    for(size_t i;i<clean.length;) {
        auto start=i; auto first=line; bool literal;
        if(whitespace(clean[i])) {
            if(raw[i]=='/' && i+1<raw.length && raw[i+1]=='/') { while(i<raw.length && raw[i]!='\n') ++i; }
            else if(raw[i]=='/' && i+1<raw.length && raw[i+1]=='*') {
                i+=2; while(i+1<raw.length && !(raw[i]=='*' && raw[i+1]=='/')) ++i;
                i=(i+2<raw.length) ? i+2 : raw.length;
            } else if(raw[i]=='\'' || raw[i]=='"') { i=quoteEnd(raw,i); literal=true; }
            else if(raw[i]=='`') { auto e=templateEnd(raw,i+1); i=e<0 ? raw.length : e; literal=true; }
            else if(raw[i]=='/') {
                ++i; bool inClass;
                while(i<raw.length && raw[i]!='\n') {
                    if(raw[i]=='\\') { i+=2; continue; }
                    if(raw[i]=='/' && !inClass) { ++i; while(i<raw.length && "dgimsuvy"d.canFind(raw[i])) ++i; break; }
                    if(raw[i]=='[') inClass=true; else if(raw[i]==']') inClass=false;
                    ++i;
                }
                if(i>raw.length) i=raw.length; literal=true;
            } else ++i;
        } else {
            ++i;
            if(jsWord(clean[start]) || clean[start]=='#') while(i<clean.length && jsWord(clean[i])) ++i;
        }
        auto rawStart=rawByte,cleanStart=cleanByte;
        foreach(pos;start..i) {
            if(raw[pos]=='\n') ++line;
            static if(is(T==dchar) && retainSlices) { rawByte+=utf8Width(raw[pos]); cleanByte+=utf8Width(clean[pos]); }
        }
        if(whitespace(clean[start]) && !literal) continue;
        // Both paths retain immutable joined input, never mutable scratch.
        static if(is(T==char)) string text=literal ? raw[start..i] : clean[start..i];
        else static if(retainSlices) string text=literal ? rawUtf8[rawStart..rawByte] : cleanUtf8[cleanStart..cleanByte];
        else string text=to!string(literal ? raw[start..i] : clean[start..i]);
        int index=cast(int)tokens.length;
        tokens~=Token(text,first,line,literal); paired~=-1;
        if(literal) continue;
        if(text=="(" || text=="[" || text=="{") stack~=index;
        else if((text==")" || text=="]" || text=="}") && stack.length) {
            auto open=stack[$-1]; auto want=text==")" ? "(" : text=="]" ? "[" : "{";
            if(tokens.data[open].text==want) { paired.data[open]=index; paired.data[index]=open; stack.length--; }
        }
    }
    pairs=paired.data; return tokens.data;
}
bool arrowAt(Token[] t,int i) { return i+1<t.length && t[i].text=="=" && t[i+1].text==">"; }
bool expressionContinues(Token previous,Token next) {
    if(!previous.literal) switch(previous.text) {
        case "=": case ">": case "+": case "-": case "*": case "/": case "%": case "?": case ":":
        case ".": case ",": case "!": case "&": case "|": case "^": case "~": case "<": case "await":
        case "yield": case "new": case "typeof": case "void": case "delete": case "in": case "instanceof": return true;
        default: break;
    }
    if(next.literal) return next.text.startsWith("`");
    switch(next.text) {
        case "(": case "[": case ".": case "?": case "+": case "-": case "*": case "/": case "%":
        case "&": case "|": case "^": case "<": case ">": case ":": case "=": return true;
        default: return false;
    }
}
int expressionEnd(Token[] t,int[] pairs,int first) {
    int last=first;
    for(int i=first;i<t.length;++i) {
        auto text=t[i].text;
        if(text==";" || text=="," || text==")" || text=="]" || text=="}") break;
        if(i>first && t[i].line>t[last].end && !expressionContinues(t[last],t[i])) break;
        if(text=="function" && !t[i].literal) {
            auto open=i+1;
            if(open<t.length && t[open].text=="*") ++open;
            if(open<t.length && identifier(t[open])) ++open;
            if(open<t.length && t[open].text=="(" && pairs[open]>open) {
                auto body=pairs[open]+1;
                if(body<t.length && t[body].text=="{" && pairs[body]>body) { i=pairs[body]; last=i; continue; }
            }
        }
        if((text=="(" || text=="[" || text=="{") && pairs[i]>i) i=pairs[i];
        last=i;
    }
    return last;
}
int[int] bindings(Token[] t,int[] pairs) {
    int[int] result;
    foreach(i,token;t) if(token.text=="const" || token.text=="let" || token.text=="var") {
        for(int j=cast(int)i+1;j+1<t.length && identifier(t[j]);) {
            if(t[j+1].text==",") { j+=2; continue; }
            if(j+2>=t.length || t[j+1].text!="=") break;
            result[j+2]=j;
            auto end=expressionEnd(t,pairs,j+2);
            if(end+1>=t.length || t[end+1].text!=",") break;
            j=end+2;
        }
    }
    return result;
}
struct Method { string name; int first,body; }
Method method(Token[] t,int[] pairs,int open) {
    if(!open || pairs[open]<open) return Method.init;
    auto body=pairs[open]+1;
    if(body>=t.length || t[body].text!="{" || pairs[body]<body) return Method.init;
    auto first=open-1; auto name=t[first];
    bool numeric; foreach(dchar c;name.text) { numeric=decimal(c); break; }
    if(!identifier(name) && !name.literal && !name.text.startsWith("#") && !numeric) return Method.init;
    while(first>0) {
        auto previous=t[first-1].text;
        if(previous!="async" && previous!="get" && previous!="set" && previous!="static" && previous!="*") break;
        --first;
    }
    if(!first || !"{},;".canFind(t[first-1].text)) return Method.init;
    return Method(name.text,first,body);
}
Result findJavaScript(ref Config config,string[] lines,LineFact[] facts) {
    int[] pairs; auto t=tokenize(lines,facts,pairs);
    Result result; result.classes=findClasses(config,facts);
    auto bound=bindings(t,pairs); bool[int] bodies;
    struct Located { int at; Bounds fn; }
    Located[] functions;
    void add(string name,int first,int end,int signature) {
        Bounds fn=Bounds(name,t[first].line+1,t[end].end+1);
        if(signature>=0) { fn.signatureLine=t[signature].line+1; fn.signatureName=name; }
        fn.className=classForLine(result.classes,fn.start);
        functions~=Located(first,fn);
    }
    foreach(index,token;t) {
        int i=cast(int)index;
        if(token.literal || token.text!="function" || (i>0 && t[i-1].text==".")) continue;
        auto open=i+1; string name;
        if(open<t.length && t[open].text=="*") ++open;
        if(open<t.length && identifier(t[open])) { name=t[open].text; ++open; }
        if(open>=t.length || t[open].text!="(" || pairs[open]<open) continue;
        auto body=pairs[open]+1;
        if(body>=t.length || t[body].text!="{" || pairs[body]<body) continue;
        bodies[body]=true; auto first=i;
        if(i>0 && t[i-1].text=="async" && t[i-1].line==token.line) --first;
        auto signature=open-1;
        if(!name.length) {
            signature=-1;
            if(auto p=first in bound) if(expressionEnd(t,pairs,first)==pairs[body]) { name=t[*p].text; first=*p; }
        }
        if(name.length) add(name,first,pairs[body],signature);
    }
    foreach(value,binding;bound) {
        auto open=value;
        if(t[open].text=="async" && !arrowAt(t,open+1)) ++open;
        if(open>=t.length) continue;
        auto arrow=open+1;
        if(t[open].text=="(" && pairs[open]>open) arrow=pairs[open]+1;
        else if(!identifier(t[open])) continue;
        if(!arrowAt(t,arrow) || arrow+2>=t.length) continue;
        auto body=arrow+2; int end=-1;
        if(t[body].text=="{") { bodies[body]=true; end=pairs[body]; } else end=expressionEnd(t,pairs,body);
        if(end>=body) add(t[binding].text,binding,end,-1);
    }
    foreach(i;0..t.length) if(arrowAt(t,cast(int)i) && i+2<t.length && t[i+2].text=="{") bodies[cast(int)i+2]=true;
    bool[int] classBodies; string[] scopes;
    foreach(i,token;t) {
        if(token.literal || token.text!="class" || (i>0 && t[i-1].text==".")) continue;
        if(i+1>=t.length || (!identifier(t[i+1]) && t[i+1].text!="{")) continue;
        for(auto j=i+1;j<t.length;++j) {
            if(t[j].text=="{") { classBodies[cast(int)j]=true; break; }
            if(t[j].text==";") break;
            if(t[j].text=="(" && pairs[j]>j) j=pairs[j];
        }
    }
    foreach(i,token;t) switch(token.text) {
        case "{":
            string kind="block";
            if(cast(int)i in classBodies) kind="class";
            else if(!(cast(int)i in bodies) && i>0) switch(t[i-1].text) {
                case "=": case "(": case "[": case ",": case ":": case "?": case "return": kind="object"; break;
                default: break;
            }
            scopes~=kind; break;
        case "}": case ")": case "]": if(scopes.length) scopes.length--; break;
        case "[": scopes~="block"; break;
        case "(":
            if(scopes.length && scopes[$-1]!="block") {
                auto m=method(t,pairs,cast(int)i);
                if(m.name.length) { bodies[m.body]=true; add(m.name,m.first,pairs[m.body],cast(int)i-1); }
            }
            scopes~="block"; break;
        default: break;
    }
    // Go sorts stably by token position; serial order is the tie breaker.
    import std.algorithm : SwapStrategy;
    sort!("a.at < b.at",SwapStrategy.stable)(functions);
    foreach(f;functions) result.functions~=f.fn;
    return result;
}

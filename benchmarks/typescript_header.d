// Direct translation of Go ts_header.go; returns the complete eight-string match.
module typescript_header;
import std.string : startsWith, indexOf;
import std.uni : unicode,CodepointSet;
import std.utf : decode;
import std.typecons : Yes;
import reuse : Buffer;
enum functionPattern=`^\s*(?:(export\s+)?(?:default\s+)?(async\s+)?function\s*\*?\s+([\p{L}\p{Nd}_]+)\s*[<(]|(?:export\s+)?(const|let|var)\s+([\p{L}\p{Nd}_]+)\s*(?::\s*(?:[^=]|=>)+?)?=\s*(async\s*)?[<(]|(?:(?:public|private|protected|static|async|override|readonly|abstract|get|set)\s+)*\*?\s*([\p{L}\p{Nd}_]+)\s*(?:<[^>]*>)?\s*\((?:[^()]|\([^()]*\))*\)\s*(?::\s*[^{;]+?)?\s*\{)`;

private __gshared CodepointSet letters,digits;
private immutable string[] variableWords=["const","let","var"];
private immutable string[] methodWords=["public","private","protected","static","async","override","readonly","abstract","get","set"];
shared static this() { letters=unicode.L; digits=unicode.Nd; }
bool space(char c) { return c==' ' || c=='\t' || c=='\n' || c=='\f' || c=='\r'; }
size_t skip(string s,size_t i) { while(i<s.length && space(s[i])) ++i; return i; }
size_t identEnd(string s,size_t i) {
    while(i<s.length) {
        auto c=s[i];
        if(c<128) { if(c=='_' || (c>='a' && c<='z') || (c>='A' && c<='Z') || (c>='0' && c<='9')) { ++i; continue; } return i; }
        auto j=i; auto r=decode!(Yes.useReplacementDchar)(s,j);
        if(!letters[r] && !digits[r]) return i;
        i=j;
    }
    return i;
}
ptrdiff_t parenEnd(string s,size_t i) {
    if(i>=s.length || s[i]!='(') return -1;
    for(auto j=i+1;j<s.length;) {
        if(s[j]==')') return j+1;
        if(s[j]=='(') {
            auto k=j+1; while(k<s.length && s[k]!='(' && s[k]!=')') ++k;
            if(k>=s.length || s[k]!=')') return -1;
            j=k+1;
        } else ++j;
    }
    return -1;
}
ptrdiff_t wordSpace(string s,size_t i,string word) {
    if(!s[i..$].startsWith(word)) return -1;
    auto e=i+word.length,w=skip(s,e); return w>e ? cast(ptrdiff_t)w : -1;
}
string[] functionDeclaration(string s,size_t i) {
    string exported,async;
    auto e=wordSpace(s,i,"export"); if(e>=0) { exported=s[i..e]; i=e; }
    e=wordSpace(s,i,"default"); if(e>=0) i=e;
    e=wordSpace(s,i,"async"); if(e>=0) { async=s[i..e]; i=e; }
    if(!s[i..$].startsWith("function")) return null;
    i+=8; auto w=skip(s,i),name=w;
    if(w<s.length && s[w]=='*') { name=skip(s,w+1); if(name==w+1) return null; }
    else if(w==i) return null;
    auto end=identEnd(s,name); if(end==name) return null;
    auto x=skip(s,end); if(x>=s.length || (s[x]!='<' && s[x]!='(')) return null;
    return [s[0..x+1],exported,async,s[name..end],"","","",""];
}
string[] variableValue(string s,size_t i,string keyword,string name) {
    if(i>=s.length || s[i]!='=') return null;
    auto j=skip(s,i+1);
    if(s[j..$].startsWith("async")) {
        auto k=skip(s,j+5);
        if(k<s.length && (s[k]=='<' || s[k]=='(')) return [s[0..k+1],"","","",keyword,name,s[j..k],""];
    }
    if(j<s.length && (s[j]=='<' || s[j]=='(')) return [s[0..j+1],"","","",keyword,name,"",""];
    return null;
}
string[] variableHeader(string s,size_t i) {
    auto e=wordSpace(s,i,"export"); if(e>=0) i=e;
    string keyword;
    foreach(kw;variableWords) if(s[i..$].startsWith(kw)) { keyword=kw; break; }
    if(!keyword.length) return null;
    auto end=i+keyword.length,w=skip(s,end); if(w==end) return null;
    auto n=identEnd(s,w); if(n==w) return null;
    auto q=skip(s,n); if(q>=s.length || s[q]!=':') return variableValue(s,q,keyword,s[w..n]);
    auto start=q+1,ws=skip(s,start);
    for(auto cur=ws;cur<s.length;) {
        if(s[cur]=='=') { if(cur+1>=s.length || s[cur+1]!='>') break; cur+=2; }
        else { decode!(Yes.useReplacementDchar)(s,cur); }
        auto m=variableValue(s,cur,keyword,s[w..n]); if(m.length) return m;
    }
    return ws>start ? variableValue(s,ws,keyword,s[w..n]) : null;
}
string[] methodAt(string s,size_t i) {
    auto j=i; if(j<s.length && s[j]=='*') ++j;
    j=skip(s,j); auto n=identEnd(s,j); if(n==j) return null;
    auto x=skip(s,n);
    if(x<s.length && s[x]=='<') { auto k=s[x+1..$].indexOf('>'); if(k<0) return null; x=skip(s,x+k+2); }
    auto pe=parenEnd(s,x); if(pe<0) return null;
    auto q=skip(s,pe);
    if(q<s.length && s[q]==':') {
        auto k=q+1; while(k<s.length && s[k]!='{' && s[k]!=';') ++k;
        if(k<=q+1 || k==s.length || s[k]!='{') return null;
        return [s[0..k+1],"","","","","","",s[j..n]];
    }
    if(q<s.length && s[q]=='{') return [s[0..q+1],"","","","","","",s[j..n]];
    return null;
}
private Buffer!size_t positions; // reuse.d: appending is a store, not a runtime call
string[] headerMatch(string s) {
    auto p=skip(s,0); auto m=functionDeclaration(s,p); if(m.length) return m;
    m=variableHeader(s,p); if(m.length) return m;
    positions.clear(); positions~=p;
    while(true) {
        ptrdiff_t e=-1;
        foreach(kw;methodWords) {
            e=wordSpace(s,positions[$-1],kw); if(e>=0) break;
        }
        if(e<0) break;
        positions~=e;
    }
    foreach_reverse(pos;positions) { m=methodAt(s,pos); if(m.length) return m; }
    return null;
}

// --- declaration_pattern and declarator_pattern ----------------------------------------------------------------------
// Direct translation of Go ts_declaration.go: the TS declaration pattern (a bodyless signature or `abstract` member) and
// declarator pattern (a later declarator of a const list) read without a regex. Each returns what the pattern's captures
// give for the same text: declarationMatch the whole line and two groups (the name of the first alternative or of the
// second), declaratorMatch the match up to the value's "<" or "(" and the name. They are used only while the
// configuration holds these two texts (js_ts_model.Config); AuditTSHeader compares every call with the regex.
enum declarationPattern=`^(?:\s*(?:(?:export|default|declare|public|protected|private|static|readonly|override|async|function)\s+)*([\p{L}\p{Nd}_]+)\??\s*(?:<[^()]*>)?\s*\((?:[^()]|\([^()]*\))*\)\s*:.+;\s*$|\s*(?:(?:export|default|declare|public|protected|private|static|readonly|override|async)\s+)*abstract\s+(?:async\s+)?([\p{L}\p{Nd}_]+)\??\s*(?:<[^()]*>)?\s*\((?:[^()]|\([^()]*\))*\)(?:\s*:.+)?;\s*$)`;
enum declaratorPattern=`^\s*([\p{L}\p{Nd}_]+)\s*(?::\s*(?:[^=]|=>)+?)?=\s*(?:async\s*)?[<(]`;

private immutable string[] declarationWords=["export","default","declare","public","protected","private","static","readonly","override","async","function"];
private immutable string[] abstractWords=["export","default","declare","public","protected","private","static","readonly","override","async"];
private Buffer!size_t chain; // thread-local scratch, like positions above: non-reentrant

// The offsets after each prefix of the greedy `(?:(?:word)\s+)*` from p (the first is p itself).
private size_t[] modifierChain(string s,size_t p,immutable(string)[] words) {
    chain.clear(); chain~=p;
    while(true) {
        ptrdiff_t e=-1;
        foreach(kw;words) { e=wordSpace(s,chain[$-1],kw); if(e>=0) break; }
        if(e<0) break;
        chain~=e;
    }
    return chain.data;
}

// `([\p{L}\p{Nd}_]+)\??\s*(?:<[^()]*>)?\s*\((?:[^()]|\([^()]*\))*\)` at i: the name and the offset of the closing ")".
private bool nameAndParameters(string s,size_t i,out string name,out size_t close) {
    auto e=identEnd(s,i); if(e==i) return false;
    name=s[i..e];
    if(e<s.length && s[e]=='?') ++e;
    auto x=skip(s,e);
    if(x<s.length && s[x]=='<') {
        // `<[^()]*>` then `\s*\(`: the run of non-parenthesis characters ends at the first "(" or ")"; the greedy
        // `[^()]*` leaves the run's last ">" for the closing one, and only it can be followed by whitespace and "(".
        auto r=x+1;
        while(r<s.length && s[r]!='(' && s[r]!=')') ++r;
        if(r>=s.length || s[r]!='(') return false;
        ptrdiff_t gt=-1;
        foreach_reverse(k;x+1..r) if(s[k]=='>') { gt=k; break; }
        if(gt<0 || skip(s,gt+1)!=r) return false;
        x=r;
    }
    if(x>=s.length || s[x]!='(') return false;
    auto pe=parenEnd(s,x); if(pe<0) return false;
    close=pe-1; return true;
}

// `\s*:.+;\s*$` after the ")" at close: the ";" is the last character that is not whitespace, with at least one character
// that is not a newline between it and the ":".
private bool annotatedEnd(string s,size_t close) {
    auto j=skip(s,close+1);
    if(j>=s.length || s[j]!=':') return false;
    auto e=s.length; while(e>0 && space(s[e-1])) --e;
    if(e<j+3 || s[e-1]!=';') return false;
    foreach(c;s[j+1..e-1]) if(c=='\n') return false;
    return true;
}

// `(?:\s*:.+)?;\s*$` after the ")" at close.
private bool abstractEnd(string s,size_t close) {
    if(annotatedEnd(s,close)) return true;
    auto i=close+1;
    return i<s.length && s[i]==';' && skip(s,i+1)==s.length;
}

private bool signatureDeclaration(string s,size_t p,out string name) {
    auto positions=modifierChain(s,p,declarationWords);
    foreach_reverse(pos;positions) {
        size_t close;
        if(nameAndParameters(s,pos,name,close) && annotatedEnd(s,close)) return true;
    }
    return false;
}

private bool abstractDeclaration(string s,size_t p,out string name) {
    auto positions=modifierChain(s,p,abstractWords);
    // Fewer modifiers would leave a modifier word where `abstract` is needed: only the longest chain can match.
    auto i=wordSpace(s,positions[$-1],"abstract"); if(i<0) return false;
    size_t close;
    // `(?:async\s+)?` is greedy: with the word first, then without it (the name may be "async").
    auto e=wordSpace(s,i,"async");
    if(e>=0 && nameAndParameters(s,e,name,close) && abstractEnd(s,close)) return true;
    return nameAndParameters(s,i,name,close) && abstractEnd(s,close);
}

string[] declarationMatch(string s) {
    auto p=skip(s,0); string name;
    if(signatureDeclaration(s,p,name)) return [s,name,""];
    if(abstractDeclaration(s,p,name)) return [s,"",name];
    return null;
}

// `=\s*(async\s*)?[<(]` at i: the offset just past the "<" or "(".
private bool valueEnd(string s,size_t i,out size_t end) {
    if(i>=s.length || s[i]!='=') return false;
    auto j=skip(s,i+1);
    if(s[j..$].startsWith("async")) {
        auto k=skip(s,j+5);
        if(k<s.length && (s[k]=='<' || s[k]=='(')) { end=k+1; return true; }
    }
    if(j<s.length && (s[j]=='<' || s[j]=='(')) { end=j+1; return true; }
    return false;
}

// `(?::\s*(?:[^=]|=>)+?)?=\s*(async\s*)?[<(]` from q (after the name and its whitespace), as variableHeader reads it:
// the lazy annotation chain, then the one end that gives back whitespace.
private bool assignedValue(string s,size_t q,out size_t end) {
    if(q>=s.length || s[q]!=':') return valueEnd(s,q,end);
    auto start=q+1,ws=skip(s,start);
    for(auto cur=ws;cur<s.length;) {
        if(s[cur]=='=') { if(cur+1>=s.length || s[cur+1]!='>') break; cur+=2; }
        else { decode!(Yes.useReplacementDchar)(s,cur); }
        if(valueEnd(s,cur,end)) return true;
    }
    return ws>start && valueEnd(s,ws,end);
}

string[] declaratorMatch(string s) {
    auto p=skip(s,0), n=identEnd(s,p); if(n==p) return null;
    size_t end;
    if(!assignedValue(s,skip(s,n),end)) return null;
    return [s[0..end],s[p..n]];
}

unittest {
    assert(declarationMatch("foo(x: T): U;")==["foo(x: T): U;","foo",""]);
    assert(declarationMatch("  export declare function f<T>(a: (b)): void;  ")==["  export declare function f<T>(a: (b)): void;  ","f",""]);
    assert(declarationMatch("abstract run(): void;")==["abstract run(): void;","","run"]);
    assert(declarationMatch("protected abstract async go();")==["protected abstract async go();","","go"]);
    assert(declarationMatch("abstract async(): void;")==["abstract async(): void;","","async"]);
    assert(declarationMatch("abstract run() ;") is null);
    assert(declarationMatch("foo(x): ;")==["foo(x): ;","foo",""]);
    assert(declarationMatch("foo(x):;") is null);
    assert(declarationMatch("foo((a)(b)): T;")==["foo((a)(b)): T;","foo",""]); // two one-level groups are fine
    assert(declarationMatch("foo(((a))): T;") is null);   // nested ones are not
    assert(declarationMatch("static (x): T;")==["static (x): T;","static",""]);
    assert(declarationMatch("foo<T>(x): T")  is null);
    assert(declaratorMatch("a = (x) => 1")==["a = (","a"]);
    assert(declaratorMatch("  handler: (a: A) => B = async <T>(x) => x")==["  handler: (a: A) => B = async <","handler"]);
    assert(declaratorMatch("b = 2") is null);
    assert(declaratorMatch("é: T = (") ==["é: T = (","é"]);
}

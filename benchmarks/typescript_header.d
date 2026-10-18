// Direct translation of Go ts_header.go; returns the complete eight-string match.
module typescript_header;
import std.string : startsWith, indexOf;
import std.uni : unicode,CodepointSet;
import std.utf : decode;
import std.typecons : Yes;
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
private size_t[] positions;
string[] headerMatch(string s) {
    auto p=skip(s,0); auto m=functionDeclaration(s,p); if(m.length) return m;
    m=variableHeader(s,p); if(m.length) return m;
    positions.length=0; positions.assumeSafeAppend; positions~=p;
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

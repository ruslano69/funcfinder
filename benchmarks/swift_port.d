module swift_port;
import reuse : TokenPool;
import brace_port_cleaner : nextRune;
import brace_port_unicode : goLetter,goDigit;
import brace_port_model : Result,Bounds;
import std.algorithm : canFind;
import std.uni : isWhite;

struct SwiftToken {string text;size_t start,end;int line,last;bool word,literal;}
struct SwiftTokens {SwiftToken[] items;int[] pairs;}
private enum modifiers=["public","private","fileprivate","internal","open","package","static","class","override","final","mutating","nonmutating","convenience","required","dynamic","lazy","nonisolated","isolated","distributed","borrowing","consuming","prefix","postfix","infix","indirect","optional"];
private bool head(dchar c){return c=='_' || goLetter(c) || c>=0x1f000 && c<=0x1faff;}
private bool tail(dchar c){return head(c) || goDigit(c) || c>=0x300 && c<=0x36f;}
private struct Lexer {
    string s;size_t i;int line=1;
    bool at(string text){return i+text.length<=s.length && s[i..i+text.length]==text;}
    void step(){if(s[i]=='\n')++line;++i;}
    void comment(){i+=2;int depth=1;while(i<s.length && depth){if(at("/*")){i+=2;++depth;}else if(at("*/")){i+=2;--depth;}else step();}}
    bool stringStart(){auto q=i;while(q<s.length && s[q]=='#')++q;return q<s.length && s[q]=='"';}
    void stringLiteral(int depth=0) {
        if(depth>256){i=s.length;return;}
        size_t hashes;while(i<s.length && s[i]=='#'){++hashes;++i;}
        bool triple=at("\"\"\"");auto width=triple?3:1;i+=width;
        while(i<s.length) {
            if(at(triple?"\"\"\"":"\"")) {
                auto end=i+width;size_t n;while(n<hashes && end<s.length && s[end]=='#'){++end;++n;}
                if(n==hashes){i=end;return;}
            }
            if(s[i]=='\\') {
                auto q=i+1;size_t n;while(n<hashes && q<s.length && s[q]=='#'){++q;++n;}
                if(n==hashes && q<s.length) {
                    if(s[q]=='('){i=q+1;int parens=1;while(i<s.length && parens){
                        if(at("/*"))comment();
                        else if(at("//")){while(i<s.length && s[i]!='\n')++i;}
                        else if(stringStart())stringLiteral(depth+1);
                        else {if(s[i]=='(')++parens;else if(s[i]==')')--parens;step();}
                    }continue;}
                    i=q;step();continue;
                }
            }
            step();
        }
    }
    bool regexLiteral(bool allowBare) {
        auto start=i;size_t hashes;while(i<s.length && s[i]=='#'){++hashes;++i;}
        if(i>=s.length || s[i]!='/' || !hashes && !allowBare){i=start;return false;}
        auto q=i+1;bool inClass;int newlines;
        while(q<s.length) {
            if(s[q]=='\n'){if(!hashes){i=start;return false;}++newlines;}
            if(s[q]=='\\' && q+1<s.length){if(s[q+1]=='\n')++newlines;q+=2;continue;}
            if(s[q]=='[')inClass=true;else if(s[q]==']')inClass=false;
            if(s[q]=='/' && !inClass){auto end=q+1;size_t n;while(n<hashes && end<s.length && s[end]=='#'){++end;++n;}if(n==hashes){i=end;line+=newlines;return true;}}
            ++q;
        }
        i=start;return false;
    }
}

// the tokens of the previous file on this thread (reuse.d): valid until the next call
private TokenPool!SwiftToken pool;
SwiftTokens tokenizeSwift(string source) {
    Lexer l=Lexer(source);alias output=pool;pool.reset();auto stack=pool.stack[];
    while(l.i<source.length) {
        auto start=l.i;auto line=l.line;auto c=cast(ubyte)source[l.i];
        if(l.at("//")){while(l.i<source.length && source[l.i]!='\n')++l.i;continue;}
        if(l.at("/*")){l.comment();continue;}
        bool directive;
        if(c=='#')foreach(d;["#if","#elseif","#else","#endif","#sourceLocation"])
            if(l.at(d) && (l.i+d.length==source.length || !head(cast(dchar)source[l.i+d.length]))){directive=true;break;}
        if(directive){while(l.i<source.length && source[l.i]!='\n')++l.i;continue;}
        bool word,literal;
        if((c=='#' || c=='"') && l.stringStart()){literal=true;l.stringLiteral();}
        else if((c=='#' || c=='/') && l.regexLiteral(!output.items.length || ["=","(","[",":",",","return","case","~"].canFind(output.items[$-1].text))){literal=true;}
        else if(c=='`'){word=true;++l.i;while(l.i<source.length && source[l.i]!='`')l.step();if(l.i<source.length)++l.i;}
        else {
            auto ch=nextRune(source,l.i);if(ch=='\n')++l.line;if(isWhite(ch))continue;
            word=head(ch);literal=goDigit(ch);
            if(word || literal)while(l.i<source.length){auto end=l.i;auto next=nextRune(source,end);if(!tail(next))break;l.i=end;}
            else if(ch=='-' && l.i<source.length && source[l.i]=='>')++l.i;
        }
        auto n=cast(int)output.items.length;auto token=SwiftToken(source[start..l.i],start,l.i,line,l.line,word,literal);
        output.items~=token;output.pairs~=-1;if(word || literal)continue;
        auto open=token.text=="("?0:token.text=="["?1:token.text=="{"?2:-1;
        if(open>=0)stack[open]~=n;
        else {auto close=token.text==")"?0:token.text=="]"?1:token.text=="}"?2:-1;if(close>=0 && stack[close].length){auto other=stack[close][$-1];stack[close].popBack();output.pairs[n]=other;output.pairs[other]=n;}}
    }
    version(AuditSwiftTokens)foreach(n,t;output.items){assert(t.start<t.end && t.end<=source.length && source[t.start..t.end]==t.text && t.line<=t.last);if(n)assert(output.items[n-1].end<=t.start && output.items[n-1].last<=t.line);auto p=output.pairs[n];if(p>=0)assert(p<output.items.length && output.pairs[p]==n);}
    return SwiftTokens(pool.items.data,pool.pairs.data);
}

private struct Scan {
    SwiftToken[] t;int[] p;
    bool at(int i,string text){return i>=0 && i<t.length && t[i].text==text;}
    bool word(int i){return i>=0 && i<t.length && t[i].word;}
    bool op(int i){return i>=0 && i<t.length && !t[i].word && !t[i].literal && (t[i].text.length==1 && "!%&*+-./<=>?^|~".canFind(t[i].text[0]) || cast(ubyte)t[i].text[0]>=128);}
    int angleEnd(int start){int depth=1;for(int i=start+1;i<t.length;++i){if(at(i,"<"))++depth;else if(at(i,">") && --depth==0)return i+1;else if(at(i,"(") || at(i,"[")){if(p[i]<i)return -1;i=p[i];}else if(at(i,"{") || at(i,";") || at(i,"="))return -1;}return -1;}
    bool declaration(int i){if(i<0 || i>=t.length || at(i-1,"."))return false;auto j=i;while(j<t.length && modifiers.canFind(t[j].text))++j;return j<t.length && ["func","var","let","struct","class","enum","actor","protocol","extension","typealias","associatedtype","init","deinit","subscript","import"].canFind(t[j].text);}
    int prefix(int keyword){auto i=keyword;while(i>0 && modifiers.canFind(t[i-1].text))--i;return i;}
    int annotationEnd(int i){++i;if(word(i))++i;while(at(i,".") && word(i+1))i+=2;if(at(i,"(") && p[i]>i)i=p[i]+1;return i;}
    int headerEnd(int start,bool aliasType=false) {
        auto last=start-1;
        for(int i=start;i<t.length;++i) {
            if(at(i,"{"))return p[i]>i?p[i]:last;
            if(at(i,";") || at(i,"}"))return last;
            if(declaration(i))return last;
            if(at(i,"@")){if(last>=0 && t[i].line>t[last].last && ![":","->",",","="].canFind(t[last].text))return last;i=annotationEnd(i)-1;last=i;continue;}
            if(aliasType && i>start && t[i].line>t[last].last && ![".","->","&",",","where"].canFind(t[i].text) && !["=","->","&",",",":"].canFind(t[last].text))return last;
            if(at(i,"<")){auto next=angleEnd(i);if(next<0)return last;i=next-1;}
            else if(at(i,"(") || at(i,"[")){if(p[i]<i)return last;i=p[i];}
            last=i;
        }
        return last;
    }
}

Result findSwift(string source) {
    auto tokens=tokenizeSwift(source);auto t=tokens.items;auto p=tokens.pairs;auto s=Scan(t,p);Result result;
    struct Scope {string name;int end;}Scope[] scopes;
    for(int i=0;i<t.length;++i) {
        while(scopes.length && scopes[$-1].end<i)scopes.length--;
        if(s.at(i,"func")) {
            auto name=i+1;if(name>=t.length)continue;auto next=name+1;string text=t[name].text;
            if(!s.word(name)) {
                if(!s.op(name))continue;
                while(next<t.length && s.op(next) && t[next-1].end==t[next].start){if(s.at(next,"<")){auto end=s.angleEnd(next);if(end>next && s.at(end,"("))break;}text~=t[next].text;++next;}
            }
            if(s.at(next,"<")){next=s.angleEnd(next);if(next<0)continue;}
            if(!s.at(next,"(") || p[next]<next)continue;
            auto end=s.headerEnd(p[next]+1);if(end<p[next])end=p[next];
            result.functions~=Bounds(text,t[s.prefix(i)].line,t[end].last,scopes.length?scopes[$-1].name:"",t[name].line,text);
        } else if(["class","struct","enum","actor","protocol","extension","typealias","associatedtype"].canFind(t[i].text) && s.word(i+1) && !["func","var","let","subscript"].canFind(t[i+1].text) && !s.at(i-1,".")) {
            auto name=i+1;bool aliasType=s.at(i,"typealias") || s.at(i,"associatedtype");auto end=s.headerEnd(name+1,aliasType);if(end<name)end=name;
            if(!s.at(i,"extension"))result.classes~=Bounds(t[name].text,t[s.prefix(i)].line,t[end].last);
            if(!aliasType)scopes~=Scope(t[name].text,end);
        }
    }
    return result;
}

unittest {
    auto r=findSwift("protocol P {\nfunc f<T>(_ x:T) async throws -> Int where T:Equatable\nfunc g()\n}\nstruct S {\nstatic func ==(a:S,b:S)->Bool {true}\nfunc outer(){func local(){}}\n}\nextension S {func ext(){}}\ntypealias Alias = (Int) -> String\n");
    string[] names;foreach(f;r.functions)names~=f.name;assert(names==["f","g","==","outer","local","ext"]);assert(r.functions[0].end==2 && r.functions[2].end==6 && r.classes.length==3);
    r=findSwift("/* outer /* nested */ */\nfunc `repeat`() {let a = ##\"} func fake(){} \\##(foo(\"}\"))\"##;let b = #/ } func fake2(){} /#}\nfunc 🚀() {}\n");
    assert(r.functions.length==2 && r.functions[0].name=="`repeat`" && r.functions[1].name=="🚀" && r.functions[0].end==2);
    r=findSwift("struct C {class func f(){};init(){};deinit{};var p:Int {0}}\n#if X\nfunc left(){}\n#else\nfunc right(){}\n#endif\n");
    assert(r.functions.length==3 && r.classes.length==1);
    r=findSwift("protocol P {\n @A func first(\n _:Int\n )\n @B func second()\n}\n");
    assert(r.functions.length==2 && r.functions[0].end==4 && r.functions[1].end==5);
}

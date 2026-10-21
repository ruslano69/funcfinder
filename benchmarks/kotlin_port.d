module kotlin_port;
import brace_port_cleaner : nextRune;
import brace_port_unicode : goLetter,goDigit;
import brace_port_model : Result,Bounds;
import std.algorithm : canFind;
import std.uni : isWhite;
import reuse : TokenPool;

struct KotlinToken {string text;size_t start,end;int line,last;bool word,literal;}
struct KotlinTokens {KotlinToken[] items;int[] pairs;}
private enum modifiers=["public","private","protected","internal","override","open","abstract","final","suspend","inline","infix","operator","tailrec","external","actual","expect","const","lateinit","data","sealed","enum","annotation","inner","value","companion","crossinline","noinline","reified"];
private bool head(dchar c){return c=='_' || goLetter(c);}
private bool tail(dchar c){return head(c) || goDigit(c) || c>=0x300 && c<=0x36f;}

private struct Lexer {
    string source;size_t i;int line=1;
    bool at(string text){return i+text.length<=source.length && source[i..i+text.length]==text;}
    void step(){if(source[i]=='\n')++line;++i;}
    void comment(){i+=2;int depth=1;while(i<source.length && depth){if(at("/*")){i+=2;++depth;}else if(at("*/")){i+=2;--depth;}else step();}}
    void stringLiteral(int depth=0) {
        if(depth>256){i=source.length;return;}
        bool raw=at("\"\"\"");i+=raw?3:1;
        while(i<source.length) {
            if(raw && at("\"\"\"")){while(i<source.length && source[i]=='"')++i;return;}
            if(!raw && source[i]=='"'){++i;return;}
            if(!raw && source[i]=='\\'){step();if(i<source.length)step();continue;}
            if(at("${")){i+=2;int braces=1;while(i<source.length && braces){
                if(at("/*"))comment();
                else if(at("//")){while(i<source.length && source[i]!='\n')++i;}
                else if(source[i]=='"')stringLiteral(depth+1);
                else if(source[i]=='\''){++i;while(i<source.length){if(source[i]=='\\'){step();if(i<source.length)step();}else if(source[i]=='\''){++i;break;}else step();}}
                else {if(source[i]=='{')++braces;else if(source[i]=='}')--braces;step();}
            }continue;}
            step();
        }
    }
}

private TokenPool!KotlinToken pool; // the tokens of the previous file on this thread (reuse.d): valid until the next call
KotlinTokens tokenizeKotlin(string source) {
    Lexer l=Lexer(source);alias output=pool;pool.reset();auto stack=pool.stack[];
    while(l.i<source.length) {
        auto start=l.i;auto line=l.line;auto c=cast(ubyte)source[l.i];
        if(l.at("//")){while(l.i<source.length && source[l.i]!='\n')++l.i;continue;}
        if(l.at("/*")){l.comment();continue;}
        bool word,literal;
        if(c=='"'){literal=true;l.stringLiteral();}
        else if(c=='\''){literal=true;++l.i;while(l.i<source.length){if(source[l.i]=='\\'){l.step();if(l.i<source.length)l.step();}else if(source[l.i]=='\''){++l.i;break;}else l.step();}}
        else if(c=='`'){word=true;++l.i;while(l.i<source.length && source[l.i]!='`')l.step();if(l.i<source.length)++l.i;}
        else {
            auto ch=nextRune(source,l.i);if(ch=='\n')++l.line;if(isWhite(ch))continue;
            word=head(ch);literal=goDigit(ch);
            if(word || literal)while(l.i<source.length){auto end=l.i;auto next=nextRune(source,end);if(!tail(next))break;l.i=end;}
            else if(l.i<source.length && ["?.","?:","!!","::","->","&&","||","<=",">=","==","!=","..","+=","-=","*=","/=","++","--"].canFind(source[start..l.i+1]))++l.i;
        }
        auto n=cast(int)output.items.length;auto token=KotlinToken(source[start..l.i],start,l.i,line,l.line,word,literal);
        output.items~=token;output.pairs~=-1;
        if(literal || word)continue;
        auto open=token.text=="("?0:token.text=="["?1:token.text=="{"?2:-1;
        if(open>=0)stack[open]~=n;
        else {auto close=token.text==")"?0:token.text=="]"?1:token.text=="}"?2:-1;
            if(close>=0 && stack[close].length){auto other=stack[close][$-1];stack[close].popBack();output.pairs[n]=other;output.pairs[other]=n;}}
    }
    version(AuditKotlinTokens)foreach(n,t;output.items){
        assert(t.start<t.end && t.end<=source.length && source[t.start..t.end]==t.text && t.line<=t.last);
        if(n)assert(output.items[n-1].end<=t.start && output.items[n-1].last<=t.line);
        auto p=output.pairs[n];if(p>=0)assert(p<output.items.length && output.pairs[p]==n);
    }
    return KotlinTokens(pool.items.data,pool.pairs.data);
}

private struct Scan {
    KotlinToken[] t;int[] p;
    bool at(int i,string text){return i>=0 && i<t.length && t[i].text==text;}
    bool word(int i){return i>=0 && i<t.length && t[i].word;}
    bool declaration(int i){
        if(i<0 || i>=t.length || at(i-1,".") || at(i-1,"?.") || at(i-1,"::"))return false;
        auto j=i;while(j<t.length && modifiers.canFind(t[j].text))++j;
        return j<t.length && ["fun","val","var","class","interface","object","typealias","constructor","init","package","import"].canFind(t[j].text);
    }
    int annotationEnd(int i){++i;if(word(i))++i;if(at(i,":")){++i;if(word(i))++i;}while(at(i,".") && word(i+1))i+=2;if(at(i,"(") && p[i]>i)i=p[i]+1;return i;}
    int angleEnd(int start){int depth=1;for(int i=start+1;i<t.length;++i){if(at(i,"<"))++depth;else if(at(i,">") && --depth==0)return i+1;else if(at(i,"(") || at(i,"[")){if(p[i]<i)return -1;i=p[i];}else if(at(i,"{") || at(i,";") || at(i,"="))return -1;}return -1;}
    int prefix(int keyword){auto i=keyword;while(i>0 && modifiers.canFind(t[i-1].text)) {
        // Soft modifier words can also end the preceding property expression.
        if(i>1 && [".","?.","::"].canFind(t[i-2].text))break;
        --i;
    }return i;}
    int parameters(int start,out int name) {
        name=-1;
        for(int i=start;i<t.length;++i) {
            if(word(i) && at(i+1,"(")){name=i;return i+1;}
            if(at(i,"<")){auto next=angleEnd(i);if(next<0)return -1;i=next-1;}
            else if(at(i,"(")) {
                if(i>start && word(i-1)){name=i-1;return i;}
                if(p[i]>i && at(p[i]+1,".")){i=p[i];continue;}
                return -1;
            } else if(at(i,"{") || at(i,"=") || at(i,";") || at(i,"}"))return -1;
            else if(i>start && declaration(i))return -1;
        }
        return -1;
    }
    bool canEnd(int i){return word(i) || t[i].literal || [")","]","}","!!","++","--",">","?"].canFind(t[i].text);}
    bool continuation(int i){return i<t.length && [".","?.","?:","::","+","-","*","/","%","&&","||","==","!=","<=",">=","<",">","in","is","as","else","catch","finally","!!"].canFind(t[i].text);}
    int expressionEnd(int start) {
        auto last=start-1;bool need=true;
        for(int i=start;i<t.length;++i) {
            if(at(i,";") || at(i,"}") || at(i,"]") || at(i,")"))return last;
            if(i>start && t[i].line>t[last].last && !need && canEnd(last) && !continuation(i) && !at(i,"{") && !at(i,"("))return last;
            if(at(i,"@")){i=annotationEnd(i)-1;last=i;continue;}
            if(i>start && declaration(i) && !at(i,"object"))return last;
            if(["if","when","catch"].canFind(t[i].text)) {
                need=true;if(at(i+1,"(") && p[i+1]>i+1)i=p[i+1];last=i;continue;
            }
            if(["else","try","finally","throw","return"].canFind(t[i].text)){need=true;last=i;continue;}
            if(at(i,"object")) {
                auto body=i+1;while(body<t.length && !at(body,"{") && !at(body,";") && !at(body,"}")){if(at(body,"(") && p[body]>body)body=p[body];++body;}
                if(at(body,"{") && p[body]>body){i=p[body];last=i;need=false;continue;}
            }
            if(at(i,"(") || at(i,"[") || at(i,"{")){if(p[i]<i)return last;i=p[i];need=false;}
            else if(at(i,"<") && i>start && word(i-1)){auto next=angleEnd(i);if(next>i && (at(next,"(") || at(next,"{") || at(next,"."))){i=next-1;need=false;}else need=true;}
            else if(word(i) && !need && last>=start && canEnd(last))need=true; // infix function needs a right operand
            else if(continuation(i) && !at(i,"!!"))need=true;
            else need=!canEnd(i);
            last=i;
        }
        return last;
    }
    int functionEnd(int close) {
        auto last=close;
        for(int i=close+1;i<t.length;++i) {
            if(at(i,"@")){if(t[i].line>t[last].last && ![":","->","<",","].canFind(t[last].text))return last;i=annotationEnd(i)-1;last=i;continue;}
            if(at(i,"="))return expressionEnd(i+1);
            if(at(i,"{"))return p[i]>i?p[i]:last;
            if(at(i,";") || at(i,"}"))return last;
            if(declaration(i))return last;
            if(at(i,"<")){auto next=angleEnd(i);if(next<0)return last;i=next-1;}
            else if(at(i,"(") || at(i,"[")){if(p[i]<i)return last;i=p[i];}
            last=i;
        }
        return last;
    }
    int typeEnd(int name,bool isAlias) {
        if(isAlias)return expressionEnd(name+1);
        auto last=name;
        for(int i=name+1;i<t.length;++i) {
            if(at(i,"{"))return p[i]>i?p[i]:last;
            if(at(i,";") || at(i,"}"))return last;
            if(at(i,"@")){auto next=annotationEnd(i);if(!at(next,"constructor") && !at(next,"@") && !at(last,":"))return last;i=next-1;last=i;continue;}
            if(at(i,"constructor")){last=i;continue;}
            if(["private","protected","internal","public"].canFind(t[i].text)){auto next=i+1;while(next<t.length && modifiers.canFind(t[next].text))++next;if(at(next,"constructor")){last=i;continue;}}
            if(declaration(i))return last;
            if(t[i].line>t[last].last && ![":",",","where","by","(","<"].canFind(t[i].text) && ![":",",","where","by","constructor"].canFind(t[last].text))return last;
            if(at(i,"<")){auto next=angleEnd(i);if(next<0)return last;i=next-1;}
            else if(at(i,"(") || at(i,"[")){if(p[i]<i)return last;i=p[i];}
            last=i;
        }
        return last;
    }
}

Result findKotlin(string source) {
    auto tokens=tokenizeKotlin(source);auto t=tokens.items;auto p=tokens.pairs;auto s=Scan(t,p);Result result;
    struct Scope {string name;int end;}Scope[] scopes;
    for(int i=0;i<t.length;++i) {
        while(scopes.length && scopes[$-1].end<i)scopes.length--;
        if(s.at(i,"fun")) {
            int name;auto open=s.parameters(i+1,name);if(open<0 || p[open]<open)continue;
            auto end=s.functionEnd(p[open]);if(end<open)continue;
            result.functions~=Bounds(t[name].text,t[s.prefix(i)].line,t[end].last,scopes.length?scopes[$-1].name:"",t[name].line,t[name].text);
        } else if(["class","interface","object","typealias"].canFind(t[i].text) && s.word(i+1) && !s.at(i-1,"::")) {
            auto name=i+1;auto end=s.typeEnd(name,s.at(i,"typealias"));if(end<name)continue;
            result.classes~=Bounds(t[name].text,t[s.prefix(i)].line,t[end].last);
            if(!s.at(i,"typealias"))scopes~=Scope(t[name].text,end);
        }
    }
    return result;
}

unittest {
    auto prefixResult=findKotlin("class C {\n val state get() = flow.value\n fun f() {}\n val other get() = flow?.value\n suspend fun g() {}\n}\n");
    assert(prefixResult.functions.length==2 && prefixResult.functions[0].start==3 && prefixResult.functions[1].start==5);
    auto r=findKotlin("interface I {\n fun f(x: Int): Int\n fun g(): String\n}\nfun outer() {\n fun counters() = value.toInt()\n if (ready) { use() }\n}\nfun String.ext() = length\n");
    assert(r.functions.length==5 && r.functions[0].end==2 && r.functions[2].end==8 && r.functions[3].end==6 && r.functions[4].end==9);
    r=findKotlin("fun f() = if (ok)\n  foo()\nelse\n  bar()\nfun g() = run { x() }\n .let { y() }\nfun h() = \"\"\"multi\n} fun fake() {}\n\"\"\"\n");
    assert(r.functions.length==3 && r.functions[0].end==4 && r.functions[1].end==6 && r.functions[2].end==9);
    r=findKotlin("/* outer /* nested */ */\nfun `a test`() = \"${run { \"}\" }}\"\nclass C<T>(val f: () -> Unit)\nobject O {}\ntypealias Alias = List<String>\n");
    assert(r.functions.length==1 && r.functions[0].name=="`a test`" && r.classes.length==3);
    r=findKotlin("fun T.init() {}\nfun f(): suspend R.() -> Unit {return x}\nfun get() = if (ok) @A(1) {foo()} else bar()\nclass C\n@JvmOverloads constructor(x:Int) {fun f(){}}\n");
    assert(r.functions.length==4 && r.functions[0].name=="init" && r.functions[1].end==2 && r.functions[2].end==3 && r.classes[0].end==5);
    r=findKotlin("interface I {\n @A fun a(): Int\n @B(1) fun b(): Int\n}\nclass C(x:Int)\n@A fun next(){}\n");
    assert(r.functions.length==3 && r.functions[0].end==2 && r.functions[1].end==3 && r.classes[1].end==5);
    r=findKotlin("fun f() = call() togetherWith\n other()\nfun g() = a to\n b\ndata class C(val x:Int)\nprivate const val X=1\n");
    assert(r.functions[0].end==2 && r.functions[1].end==4 && r.classes[0].end==5);
}

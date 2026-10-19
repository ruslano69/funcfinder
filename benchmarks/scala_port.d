module scala_port;
import brace_port_cleaner : nextRune;
import brace_port_unicode : goLetter,goDigit;
import brace_port_model : Result,Bounds;
import std.algorithm : canFind;
import std.uni : isWhite;
import std.string : strip,endsWith;

struct ScalaToken {string text;size_t start,end;int line,last;bool word,literal;int column;}
struct ScalaTokens {ScalaToken[] items;int[] pairs;}
private enum modifiers=["private","protected","public","override","final","abstract","implicit","inline","transparent","infix","open","sealed","case","lazy","opaque","erased"];
private bool head(dchar c){return c=='_' || c=='$' || goLetter(c);}
private bool tail(dchar c){return head(c) || goDigit(c) || c>=0x300 && c<=0x36f;}

private bool isOp(dchar c){return c<128 ? "!#%&*+-/:<=>?@\\^|~".canFind(cast(char)c) : !head(c) && !goDigit(c) && !isWhite(c);}
private struct Lexer {
    string source;size_t i;int line=1;size_t lineStart;
    bool at(string text){return i+text.length<=source.length && source[i..i+text.length]==text;}
    void step(){if(source[i]=='\n'){++line;lineStart=i+1;}++i;}
    void comment(){i+=2;int depth=1;while(i<source.length && depth){if(at("/*")){i+=2;++depth;}else if(at("*/")){i+=2;--depth;}else step();}}
    void apostrophe(){
        ++i;auto q=i;
        if(q<source.length && source[q]=='\\'){
            ++q;
            if(q<source.length && source[q]=='u'){while(q<source.length && source[q]=='u')++q;int digits;while(q<source.length && digits++<4)++q;}
            else if(q<source.length && source[q]>='0' && source[q]<='7'){int digits;while(q<source.length && source[q]>='0' && source[q]<='7' && digits++<3)++q;}
            else if(q<source.length)++q;
        }else if(q<source.length)nextRune(source,q);
        if(q<source.length && source[q]=='\'')i=q+1;
        else while(i<source.length){auto end=i;auto ch=nextRune(source,end);if(!tail(ch))break;i=end;}
    }
    void stringLiteral(int depth=0) {
        if(depth>256){while(i<source.length)step();return;}
        bool interpolate=i>0 && tail(cast(ubyte)source[i-1]);bool raw=at("\"\"\"");i+=raw?3:1;
        while(i<source.length) {
            if(raw && at("\"\"\"")){while(i<source.length && source[i]=='"')++i;return;}
            if(!raw && source[i]=='"'){++i;return;}
            if(!raw && source[i]=='\\'){step();if(i<source.length)step();continue;}
            if(interpolate && at("${")){i+=2;int braces=1;while(i<source.length && braces){
                if(at("/*"))comment();
                else if(at("//")){while(i<source.length && source[i]!='\n')++i;}
                else if(source[i]=='"')stringLiteral(depth+1);
                else if(source[i]=='\'')apostrophe();
                else {if(source[i]=='{')++braces;else if(source[i]=='}')--braces;step();}
            }continue;}
            step();
        }
    }
}

ScalaTokens tokenizeScala(string source) {
    Lexer l=Lexer(source);ScalaTokens output;int[][3] stack;
    while(l.i<source.length) {
        auto start=l.i;auto line=l.line;auto column=cast(int)(l.i-l.lineStart);auto c=cast(ubyte)source[l.i];
        if(l.at("//")){while(l.i<source.length && source[l.i]!='\n')++l.i;continue;}
        if(l.at("/*")){l.comment();continue;}
        bool word,literal;
        if(c=='"'){literal=true;l.stringLiteral();}
        else if(c=='\''){literal=true;l.apostrophe();}
        else if(c=='`'){word=true;++l.i;while(l.i<source.length && source[l.i]!='`')l.step();if(l.i<source.length)++l.i;}
        else {
            auto ch=nextRune(source,l.i);if(ch=='\n'){++l.line;l.lineStart=l.i;}if(isWhite(ch))continue;
            word=head(ch);literal=goDigit(ch);
            if(word || literal)while(l.i<source.length){auto end=l.i;auto next=nextRune(source,end);if(!tail(next))break;l.i=end;}
            else if(isOp(ch))while(l.i<source.length){auto end=l.i;auto next=nextRune(source,end);if(!isOp(next) || source[l.i..$].length>=2 && ["//","/*"].canFind(source[l.i..l.i+2]))break;l.i=end;}
        }
        auto n=cast(int)output.items.length;auto token=ScalaToken(source[start..l.i],start,l.i,line,l.line,word,literal,column);
        output.items~=token;output.pairs~=-1;
        if(literal || word)continue;
        auto open=token.text=="("?0:token.text=="["?1:token.text=="{"?2:-1;
        if(open>=0)stack[open]~=n;
        else {auto close=token.text==")"?0:token.text=="]"?1:token.text=="}"?2:-1;
            if(close>=0 && stack[close].length){auto other=stack[close][$-1];stack[close].length--;output.pairs[n]=other;output.pairs[other]=n;}}
    }
    version(AuditScalaTokens){size_t cursor,lineStart;int actualLine=1;foreach(n,t;output.items){
        assert(t.start<t.end && t.end<=source.length && source[t.start..t.end]==t.text && t.line<=t.last);
        while(cursor<t.start){if(source[cursor++]=='\n'){++actualLine;lineStart=cursor;}}assert(actualLine==t.line && t.column==t.start-lineStart);
        while(cursor<t.end){if(source[cursor++]=='\n'){++actualLine;lineStart=cursor;}}assert(actualLine==t.last);
        if(n)assert(output.items[n-1].end<=t.start && output.items[n-1].last<=t.line);
        auto p=output.pairs[n];if(p>=0)assert(p<output.items.length && output.pairs[p]==n);
    }}
    return output;
}

private struct Scan {
    ScalaToken[] t;int[] p;
    bool at(int i,string text){return i>=0 && i<t.length && t[i].text==text;}
    bool word(int i){return i>=0 && i<t.length && t[i].word;}
    bool oper(int i){if(i<0 || i>=t.length || t[i].word || t[i].literal || !t[i].text.length)return false;size_t offset;return isOp(nextRune(t[i].text,offset));}
    int indent(int i){while(i>0 && t[i-1].line==t[i].line)--i;return t[i].column;}
    bool declaration(int i){
        if(i<0 || i>=t.length || at(i-1,"."))return false;
        auto j=i;while(j<t.length && modifiers.canFind(t[j].text)){++j;if(at(j,"[") && p[j]>j)j=p[j]+1;}
        return j<t.length && ["def","val","var","class","trait","object","enum","type","package","import","extension","given"].canFind(t[j].text);
    }
    bool canEnd(int i){return word(i) || t[i].literal || [")","]","}"].canFind(t[i].text);}
    bool continuation(int i){return i<t.length && (oper(i) && !at(i,"@") || [".","else","catch","finally","match","yield","then","do","with","extends"].canFind(t[i].text));}
    int annotationEnd(int i){++i;if(word(i))++i;while(at(i,".") && word(i+1))i+=2;if(at(i,"[") && p[i]>i)i=p[i]+1;if(at(i,"(") && p[i]>i)i=p[i]+1;return i;}
    int expressionEnd(int start,int base,int headerLine){
        if(start>=t.length)return start-1;
        auto last=start-1;bool need=true;
        bool indented=t[start].line>headerLine && indent(start)>base;
        for(int i=start;i<t.length;++i){
            if(at(i,";") || at(i,"}") || at(i,"]") || at(i,")"))return last;
            if(i>start && t[i].line>t[last].last){
                if(indented){if(at(i,"end") && indent(i)==base)return word(i+1)?i+1:i;if(indent(i)<=base && !continuation(i))return last;}
                else if(!need && canEnd(last) && !continuation(i) && !at(i,"{") && !at(i,"("))return last;
            }
            if(!indented && i>start && declaration(i))return last;
            if(["if","while","for","catch"].canFind(t[i].text)){
                need=true;if(at(i+1,"(") && p[i+1]>i+1)i=p[i+1];last=i;continue;
            }
            if(["else","try","finally","throw","return","new","yield","then","do","macro"].canFind(t[i].text)){need=true;last=i;continue;}
            if(at(i,"@")){i=annotationEnd(i)-1;last=i;continue;}
            if(at(i,"(") || at(i,"[") || at(i,"{")){if(p[i]<i)return last;i=p[i];need=false;}
            else if(word(i) && !need && last>=start && canEnd(last))need=true;
            else if(continuation(i))need=true;
            else need=!canEnd(i);
            last=i;
        }return last;
    }
    int functionEnd(int keyword,int last){
        auto base=indent(keyword);bool returnType;
        for(int i=last+1;i<t.length;++i){
            if(at(i,"="))return expressionEnd(i+1,base,t[i].line);
            if(at(i,"{")){
                if(p[i]<i)return last;
                // A structural/refinement return type is followed by '=' or more type syntax.
                if(returnType && (at(p[i]+1,"=") || at(p[i]+1,"with") || at(p[i]+1,"#"))){i=p[i];last=i;continue;}
                return p[i];
            }
            if(at(i,";") || at(i,"}"))return last;
            if(declaration(i) || at(i,"@"))return last;
            if(t[i].line>t[last].last && indent(i)<=base && !continuation(i) && !at(i,"(") && !at(i,"["))return last;
            if(at(i,":"))returnType=true;
            if(at(i,"(") || at(i,"[")){if(p[i]<i)return last;i=p[i];}
            last=i;
        }return last;
    }
    int typeEnd(int keyword,int name,bool aliasType){
        auto base=indent(keyword);auto last=name;
        for(int i=name+1;i<t.length;++i){
            if(at(i,"="))return expressionEnd(i+1,base,t[i].line);
            if(at(i,"{"))return p[i]>i?p[i]:last;
            if(at(i,";") || at(i,"}"))return last;
            if(at(i,":") && !aliasType && t[i].line==t[last].last){
                // Scala 3 template indentation, with no extra named type for its end marker.
                auto j=i+1;while(j<t.length){if(t[j].line>t[i].line && indent(j)<=base){if(at(j,"end") && indent(j)==base)last=word(j+1)?j+1:j;break;}if((at(j,"(") || at(j,"[") || at(j,"{")) && p[j]>j)j=p[j];last=j;++j;}return last;
            }
            if(declaration(i))return last;
            if(t[i].line>t[last].last && !continuation(i) && !at(i,"{") && !at(i,"(") && !at(i,"[") && !["extends","with",",",":","<:",">:","="].canFind(t[last].text))return last;
            if(at(i,"(") || at(i,"[")){if(p[i]<i)return last;i=p[i];}
            last=i;
        }return last;
    }
}

Result findScala(string source){
    auto tokens=tokenizeScala(source);auto t=tokens.items;auto p=tokens.pairs;auto s=Scan(t,p);Result result;
    struct Scope {string name;int end;}Scope[] scopes;
    for(int i=0;i<t.length;++i){
        while(scopes.length && scopes[$-1].end<i)scopes.length--;
        if(s.at(i,"def")){
            auto name=i+1;if(!s.word(name) && !s.oper(name))continue;
            auto next=name+1;string text=t[name].text.strip("`");
            if(text.endsWith("_") && s.oper(next) && t[name].end==t[next].start){text~=t[next].text;++next;}
            auto last=next-1;
            while((s.at(next,"[") || s.at(next,"(")) && p[next]>next){last=p[next];next=last+1;}
            if(next<t.length && !s.at(next,":") && !s.at(next,"=") && !s.at(next,"{") && !s.at(next,";") && !s.at(next,"}") && t[next].line==t[last].last)continue;
            auto end=s.functionEnd(i,last);if(end<name)continue;
            result.functions~=Bounds(text,t[i].line,t[end].last,scopes.length?scopes[$-1].name:"",t[name].line,text);
        }else if(["class","object","trait","enum","type"].canFind(t[i].text) && (s.word(i+1) || s.oper(i+1)) && !s.at(i-1,".")){
            auto name=i+1;auto end=s.typeEnd(i,name,s.at(i,"type"));if(end<name)continue;
            result.classes~=Bounds(t[name].text.strip("`"),t[i].line,t[end].last);
            if(!s.at(i,"type"))scopes~=Scope(t[name].text.strip("`"),end);
        }
    }return result;
}

unittest {
    auto r=findScala("trait T {def f[A](x:A)(y:Int):Int\n def unary_! : Boolean = true\n def +(x:Int)=x\n def `a b`=1}\nclass C {def this(x:Int)={this()} ;def outer()={def inner=1;inner}}\n");
    assert(r.functions.length==7 && r.functions[0].end==1 && r.functions[1].name=="unary_!" && r.functions[3].name=="a b" && r.classes.length==2);
    r=findScala("object O {def a = if(ok)\n foo()\n else bar()\n def b = try foo() catch {case e: E => bar()} finally done()\n def text=s\"${run { \"}\" }} def fake()={}\"\n def sym = 'hello\n def char = '}'\n}");
    assert(r.functions.length==5 && r.functions[0].end==3 && r.functions[2].end==5);
    r=findScala("def f()={1}\n@A(1) def g=2\n@B def h=\n  f()\n@C def i=3\n");
    assert(r.functions.length==4 && r.functions[0].end==1 && r.functions[1].end==2 && r.functions[2].end==4);
    r=findScala("class C:\n  def f =\n    val x = 1\n    x + 1\n  def g = 2\nend C\ndef h =\n  1\nend h\ndef i=2\n");
    assert(r.functions.length==4 && r.functions[0].end==4 && r.functions[2].end==9 && r.classes[0].end==6);
    r=findScala("trait T {def ⊕(x:Int)=x; def field_=(x:Int):Unit={}}\npackage object foo {object / {def unapply(x:Int)=true}}");
    assert(r.functions.length==3 && r.functions[0].name=="⊕" && r.functions[1].name=="field_=" && r.classes.length==3);
}


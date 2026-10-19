module php_port;
import brace_port_model : Result,Bounds;
import std.algorithm : canFind;

// PHP identifiers admit bytes 0x80..0xff, independently of Unicode categories.
private bool head(ubyte c){return c=='_' || c>='a' && c<='z' || c>='A' && c<='Z' || c>=128;}
private bool tail(ubyte c){return head(c) || c>='0' && c<='9';}
private bool equalWord(string a,string b){if(a.length!=b.length)return false;foreach(i,value;a){char c=value;if(c>='A' && c<='Z')c+=32;if(c!=b[i])return false;}return true;}
struct PHPToken {string text;size_t start,end;int line,last;bool word,literal;}
struct PHPTokens {PHPToken[] items;int[] pairs;}
private struct Lexer {
    string s;size_t i;int line=1;
    bool at(string text){return i+text.length<=s.length && s[i..i+text.length]==text;}
    void step(){if(s[i]=='\n')++line;++i;}
    void block(){i+=2;while(i<s.length && !at("*/"))step();if(i<s.length)i+=2;}
    void lineComment(){while(i<s.length && s[i]!='\n' && !at("?>"))++i;}
    void interpolation(int depth){
        if(depth>256){while(i<s.length)step();return;}
        int braces=1;
        while(i<s.length && braces){
            if(at("/*"))block();
            else if(at("//") || at("#") && !at("#["))lineComment();
            else if(s[i]=='\'' || s[i]=='"' || s[i]=='`')quoted(depth+1);
            else {if(s[i]=='{')++braces;else if(s[i]=='}')--braces;step();}
        }
    }
    void quoted(int depth=0){
        if(depth>256){while(i<s.length)step();return;}
        auto quote=s[i++];
        while(i<s.length){
            if(s[i]==quote){++i;return;}
            if(s[i]=='\\'){++i;if(i<s.length)step();continue;}
            if(quote!='\'' && at("{$")){++i;interpolation(depth+1);continue;}
            if(quote!='\'' && at("${")){i+=2;interpolation(depth+1);continue;}
            step();
        }
    }
    bool heredoc(){
        if(!at("<<<"))return false;
        auto q=i+3;while(q<s.length && (s[q]==' ' || s[q]=='\t'))++q;
        char quote=0;if(q<s.length && (s[q]=='\'' || s[q]=='"'))quote=s[q++];
        auto start=q;if(q>=s.length || !head(cast(ubyte)s[q]))return false;
        while(q<s.length && tail(cast(ubyte)s[q]))++q;auto label=s[start..q];
        if(quote){if(q>=s.length || s[q]!=quote)return false;++q;}
        while(q<s.length && (s[q]==' ' || s[q]=='\t' || s[q]=='\r'))++q;
        if(q>=s.length || s[q]!='\n')return false;
        i=q+1;++line;
        while(i<s.length){
            q=i;while(q<s.length && (s[q]==' ' || s[q]=='\t'))++q;
            auto end=q+label.length;
            if(end<=s.length && s[q..end]==label && (end==s.length || !tail(cast(ubyte)s[end]))){i=end;return true;}
            while(i<s.length && s[i]!='\n')++i;if(i<s.length)step();
        }
        return true;
    }
}

PHPTokens tokenizePHP(string source){
    Lexer l=Lexer(source);PHPTokens output;int[] stack;bool code;
    while(l.i<source.length){
        if(!code){
            if(l.at("<?")){l.i+=2;if(l.at("="))++l.i;else if(l.i+3<=source.length && equalWord(source[l.i..l.i+3],"php") && (l.i+3==source.length || source[l.i+3]<=' '))l.i+=3;code=true;}
            else l.step();continue;
        }
        if(l.at("?>")){l.i+=2;code=false;continue;}
        if(l.at("//") || l.at("#") && !l.at("#[")){l.lineComment();continue;}
        if(l.at("/*")){l.block();continue;}
        auto start=l.i;auto line=l.line;auto c=cast(ubyte)source[l.i];
        if(c<=32){l.step();continue;}
        bool word,literal;
        if(c=='\'' || c=='"' || c=='`'){literal=true;l.quoted();}
        else if(c=='<' && l.heredoc())literal=true;
        else if(c=='$' && l.i+1<source.length && head(cast(ubyte)source[l.i+1])){l.i+=2;while(l.i<source.length && tail(cast(ubyte)source[l.i]))++l.i;}
        else if(head(c)){word=true;++l.i;while(l.i<source.length && tail(cast(ubyte)source[l.i]))++l.i;}
        else if(c>='0' && c<='9'){literal=true;++l.i;while(l.i<source.length && (tail(cast(ubyte)source[l.i]) || source[l.i]=='.'))++l.i;}
        else if(l.at("::") || l.at("->") || l.at("=>")){l.i+=2;}
        else ++l.i;
        auto n=cast(int)output.items.length;auto t=PHPToken(source[start..l.i],start,l.i,line,l.line,word,literal);
        output.items~=t;output.pairs~=-1;if(word || literal)continue;
        if(["(","[","{"].canFind(t.text))stack~=n;
        else if([")","]","}"].canFind(t.text) && stack.length){auto other=stack[$-1];auto open=output.items[other].text;
            if(open=="(" && t.text==")" || open=="[" && t.text=="]" || open=="{" && t.text=="}"){stack.length--;output.pairs[n]=other;output.pairs[other]=n;}
        }
    }
    version(AuditPHPTokens){size_t cursor;int actualLine=1;foreach(n,t;output.items){
        assert(t.start<t.end && t.end<=source.length && source[t.start..t.end]==t.text && t.line<=t.last);
        while(cursor<t.start){if(source[cursor++]=='\n')++actualLine;}assert(actualLine==t.line);
        while(cursor<t.end){if(source[cursor++]=='\n')++actualLine;}assert(actualLine==t.last);
        if(n)assert(output.items[n-1].end<=t.start && output.items[n-1].last<=t.line);auto p=output.pairs[n];if(p>=0)assert(p<output.items.length && output.pairs[p]==n);
    }}
    return output;
}

Result findPHP(string source){
    auto tokens=tokenizePHP(source);auto t=tokens.items;auto p=tokens.pairs;Result result;
    bool at(int i,string text){return i>=0 && i<t.length && equalWord(t[i].text,text);}
    bool word(int i){return i>=0 && i<t.length && t[i].word;}
    int endHeader(int start){
        for(int i=start;i<t.length;++i){
            if(at(i,"{"))return p[i]>i?p[i]:-1;
            if(at(i,";"))return i;
            if(at(i,"}"))return -1;
            if(at(i,"(") || at(i,"[")){if(p[i]<i)return -1;i=p[i];}
        }return -1;
    }
    struct Scope {string name;int end;}Scope[] scopes;
    for(int i=0;i<t.length;++i){
        while(scopes.length && scopes[$-1].end<i)scopes.length--;
        // Attributes can contain class constants and arbitrary expressions.
        if(at(i,"#") && at(i+1,"[") && p[i+1]>i){i=p[i+1];continue;}
        if(at(i,"function") && !at(i-1,"::") && !at(i-1,"->") && !at(i-1,"\\")){
            auto name=i+1;if(at(name,"&"))++name;
            if(!word(name) || !at(name+1,"(") || p[name+1]<name+1)continue;
            auto end=endHeader(p[name+1]+1);if(end<0)continue;
            result.functions~=Bounds(t[name].text,t[i].line,t[end].last,scopes.length?scopes[$-1].name:"",t[name].line,t[name].text);
        }else if((at(i,"class") || at(i,"interface") || at(i,"trait") || at(i,"enum")) && !at(i-1,"::") && !at(i-1,"->")){
            bool named=word(i+1) && !at(i+1,"extends") && !at(i+1,"implements");auto end=endHeader(i+1);if(end<0)continue;
            if(named)result.classes~=Bounds(t[i+1].text,t[i].line,t[end].last);
            scopes~=Scope(named?t[i+1].text:"",end);
        }
    }
    return result;
}

unittest {
    auto r=findPHP("<?php interface I {public function &f(\nint $x\n): A&B;} class C implements I {function &f(int $x):A&B {return $x;} function outer(){class Inner {function local(){}}}} enum E {case X;function e(){}};");
    assert(r.functions.length==5 && r.classes.length==4 && r.functions[0].end==3 && r.functions[3].className=="Inner");
    r=findPHP("function fake(){} <?php FUNCTION Café(){$x=<<<'END'\nfunction fake(){}\n END;\n} ?> <b>function fake(){}</b> <?= (function(){function inner(){}})();");
    assert(r.functions.length==2 && r.functions[0].name=="Café" && r.functions[0].end==4 && r.functions[1].name=="inner");
    r=findPHP("<?php #[A(C::class)] function good(){ $v = new class {function method(){}}; $s=\"{$v->call(\"}\")} function fake(){}\";} // ?> function fake2(){}\n<?php function after(){} # comment\n");
    assert(r.functions.length==3 && r.classes.length==0 && r.functions[1].className=="");
    r=findPHP("<?php class C {function outer(){ $x=new class extends C implements I {function inner(){}}; } }");
    assert(r.classes.length==1 && r.functions.length==2 && r.functions[1].className=="");
}

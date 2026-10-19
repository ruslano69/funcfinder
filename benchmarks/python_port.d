module python_port;
import brace_port_cleaner : nextRune;
import brace_port_unicode : goLetter,goDigit;
import brace_port_model : Result,Bounds;
import std.algorithm : canFind;
import std.string : replace;

struct PythonToken {string text;size_t start,end;int line,last,indent;bool word,literal;}
struct PythonTokens {PythonToken[] items;int[] pairs;}
private bool head(dchar c){return c=='_' || goLetter(c);}
private bool tail(dchar c){return head(c) || goDigit(c) || c>=0x300 && c<=0x36f;}
private struct Lexer {
    string s;size_t i;int line=1,indent;bool leading=true;
    bool at(string text){return i+text.length<=s.length && s[i..i+text.length]==text;}
    void step(){if(s[i]=='\n'){++line;indent=0;leading=true;}else if(leading){if(s[i]==' ')++indent;else if(s[i]=='\t')indent=(indent/8+1)*8;else if(s[i]=='\f')indent=0;else leading=false;}++i;}
    bool stringPrefix(out size_t quote,out bool interpolation){
        quote=i;interpolation=false;
        if(s[i]=='\'' || s[i]=='"')return true;
        auto q=i;while(q<s.length && q<i+3 && "rRuUbBfFtT".canFind(s[q])){if("fFtT".canFind(s[q]))interpolation=true;++q;}
        if(q>i && q<s.length && (s[q]=='\'' || s[q]=='"')){quote=q;return true;}return false;
    }
    void expression(int depth){
        if(depth>256){while(i<s.length)step();return;}
        int braces=1;
        while(i<s.length && braces){size_t quote;bool interpolation;
            if(s[i]=='#'){while(i<s.length && s[i]!='\n')++i;}
            else if(stringPrefix(quote,interpolation))quoted(quote,interpolation,depth+1);
            else {if(s[i]=='{')++braces;else if(s[i]=='}')--braces;step();}
        }
    }
    void quoted(size_t quote,bool interpolation,int depth=0){
        if(depth>256){while(i<s.length)step();return;}
        auto c=s[quote];i=quote;bool triple=i+2<s.length && s[i+1]==c && s[i+2]==c;auto width=triple?3:1;i+=width;leading=false;
        while(i<s.length){
            if(s[i]==c && (!triple || i+2<s.length && s[i+1]==c && s[i+2]==c)){i+=width;leading=false;return;}
            if(s[i]=='\\'){step();if(i<s.length && !(interpolation && s[i]=='{'))step();continue;}
            if(interpolation && s[i]=='{'){if(at("{{")){i+=2;continue;}++i;expression(depth+1);continue;}
            step();
        }
    }
}

PythonTokens tokenizePython(string source){
    Lexer l=Lexer(source);PythonTokens output;int[] stack;
    if(source.length>=3 && source[0..3]=="\xef\xbb\xbf")l.i=3;
    while(l.i<source.length){
        auto start=l.i;auto line=l.line;auto indent=l.indent;auto c=cast(ubyte)source[l.i];
        if(c=='#'){while(l.i<source.length && source[l.i]!='\n')++l.i;continue;}
        if(l.at("\\\n")){l.i+=2;++l.line;l.leading=false;continue;}
        if(c=='\n'){
            l.step();if(stack.length)continue;
            if(!output.items.length || output.items[$-1].text=="\n")continue;
            output.items~=PythonToken("\n",start,l.i,line,line,indent);output.pairs~=-1;continue;
        }
        if(c<=32){l.step();continue;}
        indent=l.indent;l.leading=false;bool word,literal;size_t quote;bool interpolation;
        if(l.stringPrefix(quote,interpolation)){literal=true;l.quoted(quote,interpolation);}
        else {auto ch=nextRune(source,l.i);word=head(ch);literal=goDigit(ch);
            if(word || literal)while(l.i<source.length){auto end=l.i;auto next=nextRune(source,end);if(!tail(next))break;l.i=end;}
            else if(l.i<source.length && ["->",":=","**","//","==","!=","<=",">=","<<",">>"].canFind(source[start..l.i+1]))++l.i;
        }
        auto n=cast(int)output.items.length;auto t=PythonToken(source[start..l.i],start,l.i,line,l.line,indent,word,literal);output.items~=t;output.pairs~=-1;
        if(word || literal)continue;
        if(["(","[","{"].canFind(t.text))stack~=n;
        else if([")","]","}"].canFind(t.text) && stack.length){auto other=stack[$-1];auto open=output.items[other].text;
            if(open=="(" && t.text==")" || open=="[" && t.text=="]" || open=="{" && t.text=="}"){stack.length--;output.pairs[n]=other;output.pairs[other]=n;}
        }
    }
    version(AuditPythonTokens){size_t cursor;int actualLine=1;foreach(n,t;output.items){
        assert(t.start<t.end && t.end<=source.length && source[t.start..t.end]==t.text);
        while(cursor<t.start){if(source[cursor++]=='\n')++actualLine;}assert(actualLine==t.line);
        while(cursor<t.end){if(source[cursor++]=='\n')++actualLine;}assert(actualLine==t.last+(t.text=="\n"?1:0));
        if(n)assert(output.items[n-1].end<=t.start);auto p=output.pairs[n];if(p>=0)assert(p<output.items.length && output.pairs[p]==n);
    }}return output;
}

Result findPython(string source){
    source=source.replace("\r\n","\n").replace("\r","\n");auto tokens=tokenizePython(source);auto t=tokens.items;auto p=tokens.pairs;Result result;
    struct Statement {int first,last,indent;}Statement[] statements;
    int first;
    foreach(i,token;t)if(token.text=="\n"){if(first<i)statements~=Statement(first,cast(int)i-1,t[first].indent);first=cast(int)i+1;}
    if(first<t.length)statements~=Statement(first,cast(int)t.length-1,t[first].indent);
    // A monotonic indentation stack assigns the final logical statement of each suite.
    auto ends=new size_t[statements.length];size_t[] stack;
    foreach(i,st;statements){while(stack.length && statements[stack[$-1]].indent>=st.indent){ends[stack[$-1]]=i-1;stack.length--;}stack~=i;}
    while(stack.length){ends[stack[$-1]]=statements.length-1;stack.length--;}
    struct Scope {string name;size_t end;}Scope[] scopes;
    foreach(index,st;statements){
        while(scopes.length && scopes[$-1].end<index)scopes.length--;
        auto keyword=st.first;if(t[keyword].text=="async")++keyword;
        if(keyword>st.last || !["def","class"].canFind(t[keyword].text) || keyword+1>st.last || !t[keyword+1].word)continue;
        auto name=keyword+1;auto next=name+1;if(next<=st.last && t[next].text=="[" && p[next]>next)next=p[next]+1;
        if(t[keyword].text=="def" && (next>st.last || t[next].text!="(" || p[next]<next))continue;
        int colon=-1;
        for(int i=next;i<=st.last;++i){if(t[i].text==":"){colon=i;break;}if(["(","[","{"].canFind(t[i].text)){if(p[i]<i)break;i=p[i];}}
        if(colon<0)continue;
        auto end=colon<st.last?index:ends[index];if(end==index && colon==st.last)continue;
        auto bound=Bounds(t[name].text,t[st.first].line,t[statements[end].last].last,scopes.length?scopes[$-1].name:"",t[name].line,t[name].text);
        if(t[keyword].text=="def")result.functions~=bound;
        else {result.classes~=bound;scopes~=Scope(bound.name,end);}
    }return result;
}

unittest {
    auto r=findPython("@decorator(\n 1\n)\nasync def f[T](\n x:T\n)->T:\n # comment\n return x\n\nclass C:\n def inline(self): return 1; return 2\n def outer(self):\n  def inner():\n   pass\n  return inner\n# trailing\n");
    assert(r.functions.length==4 && r.functions[0].end==8 && r.functions[1].end==11 && r.functions[2].end==15 && r.classes[0].end==15);
    r=findPython("def text():\n s=f\"{call(\"}\")} def fake(): pass\"\n s=r'''\ndef fake():pass\n'''\n return s\ndef next():pass\n");
    assert(r.functions.length==2 && r.functions[0].end==6);
    r=findPython("def f():\r\n\tif True:\r\n\t\tpass\r\n\treturn 1\r\n\r\ndef g(): pass\r");
    assert(r.functions.length==2 && r.functions[0].end==4 && r.functions[1].end==6);
}

module ruby_port;
import reuse : TokenPool;
import brace_port_cleaner : nextRune;
import brace_port_unicode : goLetter,goDigit;
import brace_port_model : Result,Bounds;
import std.algorithm : canFind;
import std.uni : isWhite;

struct RubyToken {string text;size_t start,end;int line,last;bool word,literal;}
struct RubyTokens {RubyToken[] items;int[] pairs;}
private bool head(dchar c){return c=='_' || goLetter(c);}
private bool tail(dchar c){return head(c) || goDigit(c) || c>=0x300 && c<=0x36f;}
private struct Heredoc {string label;bool indent;}
private struct Lexer {
    string s;size_t i;int line=1;size_t lineStart;Heredoc[] pending;
    bool at(string text){return i+text.length<=s.length && s[i..i+text.length]==text;}
    void step(){if(s[i]=='\n'){++line;lineStart=i+1;}++i;}
    bool regexEndOnLine(){auto q=i+1;bool inClass;while(q<s.length && s[q]!='\n'){if(s[q]=='\\'){q+=q+1<s.length?2:1;continue;}if(s[q]=='[')inClass=true;else if(s[q]==']')inClass=false;else if(s[q]=='/' && !inClass)return true;++q;}return false;}
    void interpolation(int depth){
        if(depth>256){while(i<s.length)step();return;}
        int braces=1;while(i<s.length && braces){
            if(s[i]=='#'){while(i<s.length && s[i]!='\n')++i;}
            else if(s[i]=='\'' || s[i]=='"' || s[i]=='`')quoted(s[i],s[i],s[i]!='\'',false,depth+1);
            else if(s[i]=='%' && percent(true,depth+1)){}
            else {if(s[i]=='{')++braces;else if(s[i]=='}')--braces;step();}
        }
    }
    void quoted(char open,char close,bool interpolate,bool regexp,int depth=0){
        if(depth>256){while(i<s.length)step();return;}
        ++i;int nesting=1;bool inClass;
        while(i<s.length){
            if(s[i]=='\\'){step();if(i<s.length)step();continue;}
            if(interpolate && at("#{")){i+=2;interpolation(depth+1);continue;}
            if(regexp && open==close){if(s[i]=='[')inClass=true;else if(s[i]==']')inClass=false;}
            if(!inClass){if(open!=close && s[i]==open)++nesting;else if(s[i]==close && --nesting==0){++i;if(regexp)while(i<s.length && "imxounes".canFind(s[i]))++i;return;}}
            step();
        }
    }
    bool percent(bool allowed,int depth=0){
        if(!at("%") || !allowed)return false;
        auto q=i+1;bool interpolate=true,regexp;
        if(q<s.length && "qQwWiIxrs".canFind(s[q])){interpolate=!"qwis".canFind(s[q]);regexp=s[q]=='r';++q;}
        if(q>=s.length || cast(ubyte)s[q]<=32 || head(cast(ubyte)s[q]) || goDigit(cast(ubyte)s[q]) || s[q]=='=')return false;
        auto open=s[q];auto close=open=='('?')':open=='['?']':open=='{'?'}':open=='<'?'>':open;i=q;quoted(open,close,interpolate,regexp,depth+1);return true;
    }
    bool heredoc(bool allowed){
        if(!allowed || !at("<<"))return false;auto q=i+2;bool indent;
        if(q<s.length && (s[q]=='-' || s[q]=='~')){indent=true;++q;}
        if(q>=s.length)return false;string label;
        if(s[q]=='\'' || s[q]=='"' || s[q]=='`'){auto quote=s[q++];auto start=q;while(q<s.length && s[q]!=quote && s[q]!='\n')++q;if(q>=s.length || s[q]!=quote)return false;label=s[start..q++];}
        else {auto start=q;if(!head(cast(ubyte)s[q]))return false;while(q<s.length && tail(cast(ubyte)s[q]))++q;label=s[start..q];}
        if(!label.length)return false;pending~=Heredoc(label,indent);i=q;return true;
    }
    void bodies(){
        step();
        foreach(n,h;pending){
            while(i<s.length){
                auto q=i;if(h.indent)while(q<s.length && (s[q]==' ' || s[q]=='\t'))++q;auto end=q+h.label.length;
                if(end<=s.length && s[q..end]==h.label && (end==s.length || s[end]=='\n' || s[end]=='\r')){
                    i=end;while(i<s.length && s[i]!='\n')++i;
                    if(n+1<pending.length && i<s.length)step();break;
                }
                while(i<s.length && s[i]!='\n')++i;if(i<s.length)step();
            }
        }pending.length=0;
    }
}

private bool operand(RubyToken t){return t.word || t.literal || t.text.length && (t.text[0]=='@' || t.text[0]=='$') || [")","]","}","end"].canFind(t.text);}
// the tokens of the previous file on this thread (reuse.d): valid until the next call
private TokenPool!RubyToken pool;
RubyTokens tokenizeRuby(string source){
    Lexer l=Lexer(source);alias output=pool;pool.reset();auto stack=pool.stack[0..1];
    bool expect=true;
    while(l.i<source.length){
        auto start=l.i;auto line=l.line;auto c=cast(ubyte)source[l.i];
        if(l.i==l.lineStart && l.at("__END__") && (l.i+7==source.length || source[l.i+7]=='\r' || source[l.i+7]=='\n'))break;
        if(l.i==l.lineStart && l.at("=begin") && (l.i+6==source.length || source[l.i+6]<=' ')){
            while(l.i<source.length){if(l.i==l.lineStart && l.at("=end") && (l.i+4==source.length || source[l.i+4]<=' ')){while(l.i<source.length && source[l.i]!='\n')++l.i;break;}l.step();}continue;
        }
        if(c=='#'){while(l.i<source.length && source[l.i]!='\n')++l.i;continue;}
        if(l.at("\\\r\n")){l.i+=3;++l.line;l.lineStart=l.i;continue;}
        if(l.at("\\\n")){l.i+=2;++l.line;l.lineStart=l.i;continue;}
        if(c=='\n'){
            if(l.pending.length){l.bodies();output.items~=RubyToken(source[start..l.i],start,l.i,line,l.line,false,true);output.pairs~=-1;start=l.i;line=l.line;if(l.i==source.length)break;}
            l.step();if(output.items.length && output.items[$-1].text!="\n"){output.items~=RubyToken("\n",start,l.i,line,line);output.pairs~=-1;}expect=true;continue;
        }
        if(c<=32){l.step();continue;}
        bool word,literal;bool spaced=start>0 && source[start-1]<=' ';bool naming=output.items.length && ["def",".","::","&."].canFind(output.items[$-1].text);
        if(c=='\'' || c=='"' || c=='`'){literal=true;l.quoted(c,c,c!='\'',false);}
        else if(c=='%' && l.percent(!naming && (expect || spaced))){literal=true;}
        else if(c=='<' && l.heredoc(expect || spaced)){literal=true;}
        else if(c=='/' && !naming && (expect || spaced && l.i+1<source.length && source[l.i+1]>' ' && source[l.i+1]!='=' && l.regexEndOnLine()) && !l.at("/=")){literal=true;l.quoted('/','/',true,true);}
        else if(c==':' && !l.at("::") && l.i+1<source.length && (head(cast(ubyte)source[l.i+1]) || source[l.i+1]=='\'' || source[l.i+1]=='"')){
            literal=true;++l.i;if(source[l.i]=='\'' || source[l.i]=='"')l.quoted(source[l.i],source[l.i],source[l.i]!='\'',false);else {while(l.i<source.length && (tail(cast(ubyte)source[l.i]) || "?!=".canFind(source[l.i])))++l.i;}
        }
        else if(c=='@' || c=='$'){
            ++l.i;if(l.i<source.length && source[l.i]=='@')++l.i;
            if(l.i<source.length){auto q=l.i;auto ch=nextRune(source,q);if(head(ch) || goDigit(ch)){l.i=q;while(l.i<source.length){q=l.i;if(!tail(nextRune(source,q)))break;l.i=q;}}else if(c=='$')l.i=q;}
        }
        else if(c=='?' && expect && l.i+1<source.length && source[l.i+1]>' '){literal=true;++l.i;if(source[l.i]=='\\'){++l.i;if(l.i<source.length)nextRune(source,l.i);}else nextRune(source,l.i);}
        else {
            auto ch=nextRune(source,l.i);word=head(ch);literal=goDigit(ch);
            if(word || literal){while(l.i<source.length){auto q=l.i;if(!tail(nextRune(source,q)))break;l.i=q;}if(word && l.i<source.length && "?!".canFind(source[l.i]) && !(source[l.i]=='!' && l.i+1<source.length && source[l.i+1]=='='))++l.i;}
            else foreach(op;["<=>","===","**=","&&=","||=","==","!=","=~","!~","<<",">>","<=",">=","**","&&","||","&.","::","=>","+=","-=","*=","/=","%=","&=","|=","^=",".."])
                if(start+op.length<=source.length && source[start..start+op.length]==op){l.i=start+op.length;break;}
        }
        auto n=cast(int)output.items.length;auto t=RubyToken(source[start..l.i],start,l.i,line,l.line,word,literal);output.items~=t;output.pairs~=-1;
        expect=!operand(t) || word && ["if","unless","while","until","when","case","return","yield","raise","and","or","not","then","else","elsif","in"].canFind(t.text);
        if(word || literal)continue;
        if(["(","[","{"].canFind(t.text))stack[0]~=n;
        else if([")","]","}"].canFind(t.text) && stack[0].length){auto other=stack[0][$-1];auto open=output.items[other].text;
            if(open=="(" && t.text==")" || open=="[" && t.text=="]" || open=="{" && t.text=="}"){stack[0].popBack();output.pairs[n]=other;output.pairs[other]=n;}
        }
    }
    version(AuditRubyTokens){size_t cursor;int actualLine=1;foreach(n,t;output.items){
        assert(t.start<t.end && t.end<=source.length && source[t.start..t.end]==t.text);
        while(cursor<t.start){if(source[cursor++]=='\n')++actualLine;}assert(actualLine==t.line);
        while(cursor<t.end){if(source[cursor++]=='\n')++actualLine;}assert(actualLine==t.last+(t.text=="\n"?1:0));
        if(n)assert(output.items[n-1].end<=t.start);auto p=output.pairs[n];if(p>=0)assert(p<output.items.length && output.pairs[p]==n);
    }}return RubyTokens(pool.items.data,pool.pairs.data);
}

private struct Decl {int name=-1,nameEnd=-1,equal=-1;string text;}
private struct Scan {
    RubyToken[] t;int[] p;
    bool at(int i,string text){return i>=0 && i<t.length && t[i].text==text;}
    Decl method(int start){
        Decl d;auto i=start+1;if(i>=t.length)return d;
        if(at(i,"(") && p[i]>i && (at(p[i]+1,".") || at(p[i]+1,"::")))i=p[i]+2;
        else while(i+2<t.length && (at(i+1,".") || at(i+1,"::")))i+=2;
        if(i>=t.length || at(i,"\n"))return d;d.name=i;d.nameEnd=i;d.text=t[i].text;
        if(at(i,"[") && at(i+1,"]")){d.nameEnd=++i;d.text="[]";if(at(i+1,"=")){d.text~="=";d.nameEnd=++i;}}
        else if(t[i].word && at(i+1,"=") && t[i].end==t[i+1].start){d.text~="=";d.nameEnd=++i;}
        else if(["+","-","~","!"].canFind(t[i].text) && at(i+1,"@")){d.text~="@";d.nameEnd=++i;}
        if(!t[d.name].word && !["[]","[]=","<=>","===","==","=~","!=","!","<<",">>","<",">","<=",">=","**","+","-","*","/","%","&","|","^","~","+@","-@","~@","!@"].canFind(d.text))return Decl();
        auto next=d.nameEnd+1;if(at(next,"(") && p[next]>next)next=p[next]+1;
        if(at(next,"="))d.equal=next;return d;
    }
    bool keyword(int i){return t[i].word && !at(i-1,".") && !at(i-1,"&.") && !at(i-1,"::") && !at(i+1,":");}
    bool prefix(int i){if(i==0)return true;if(!operand(t[i-1]) && ![".","::","&."].canFind(t[i-1].text))return true;return ["and","or","not","then","else","elsif","when","rescue","ensure","do"].canFind(t[i-1].text);}
    int expressionEnd(int start,int[] blocks){
        auto last=start-1;
        for(int i=start;i<t.length;++i){
            if(at(i,";") || at(i,"end") || at(i,"}") || at(i,"]") || at(i,")"))return last;
            if(at(i,"\n")){
                if(last<start)continue;
                if(!operand(t[last]) || at(i+1,".") || at(i+1,"&.") || i+1<t.length && t[i+1].literal && t[i+1].line==t[i].line)continue;
                return last;
            }
            if((at(i,"(") || at(i,"[") || at(i,"{")) && p[i]>i)i=p[i];
            else if(blocks[i]>i)i=blocks[i];
            last=i;
        }return last;
    }
}

Result findRuby(string source){
    auto tokens=tokenizeRuby(source);auto t=tokens.items;auto p=tokens.pairs;auto s=Scan(t,p);Result result;
    auto decls=new Decl[t.length];auto names=new bool[t.length];
    foreach(i,token;t)if(token.text=="def" && s.keyword(cast(int)i)){auto d=s.method(cast(int)i);decls[i]=d;if(d.name>=0)foreach(j;i+1..cast(size_t)d.nameEnd+1)names[j]=true;}
    auto blocks=new int[t.length];blocks[]=-1;
    struct Frame {int token;bool loopHeader;}Frame[] stack;
    foreach(index,token;t){auto i=cast(int)index;
        if(token.text=="\n"){foreach(ref f;stack)f.loopHeader=false;continue;}
        if(names[i] || !s.keyword(i))continue;
        if(token.text=="end"){if(stack.length){auto other=stack[$-1].token;stack.length--;blocks[other]=i;blocks[i]=other;}continue;}
        if(token.text=="def"){if(decls[i].name>=0 && decls[i].equal<0)stack~=Frame(i);continue;}
        if(["class","module","begin","case"].canFind(token.text)){stack~=Frame(i);continue;}
        if(["if","unless","while","until","for"].canFind(token.text)){
            if(s.prefix(i))stack~=Frame(i,["while","until","for"].canFind(token.text));continue;
        }
        if(token.text=="do"){
            if(stack.length && stack[$-1].loopHeader){stack[$-1].loopHeader=false;continue;}
            stack~=Frame(i);
        }
    }
    struct Scope {string name;int end;}Scope[] scopes;
    foreach(index,token;t){auto i=cast(int)index;while(scopes.length && scopes[$-1].end<i)scopes.length--;
        if(token.text=="def" && decls[i].name>=0){auto d=decls[i];auto end=d.equal>=0?s.expressionEnd(d.equal+1,blocks):blocks[i];if(end<d.name)continue;
            result.functions~=Bounds(d.text,t[i].line,t[end].last,scopes.length?scopes[$-1].name:"",t[d.name].line,d.text);
        }else if(["class","module"].canFind(token.text) && s.keyword(i) && !names[i] && blocks[i]>i){
            auto end=blocks[i];string name;
            auto start=i+1;if(s.at(start,"::")){name="::";++start;}
            if(start<t.length && t[start].word){name~=t[start].text;auto j=start+1;while(s.at(j,"::") && j+1<t.length && t[j+1].word){name~="::"~t[j+1].text;j+=2;}}else name="";
            if(name.length)result.classes~=Bounds(name,t[i].line,t[end].last);scopes~=Scope(name,end);
        }
    }return result;
}

unittest {
    auto r=findRuby("class C\n def outer\n  def inner(x)\n   x if x\n  end\n  3.times do |x|\n   x\n  end\n end\n def self.[](x) = x\n def []=(x,y)\n end\nend\n");
    assert(r.functions.length==4 && r.functions[0].end==9 && r.functions[1].end==5 && r.functions[2].name=="[]" && r.functions[3].name=="[]=" && r.classes[0].end==13);
    r=findRuby("def text\n x=%q{end def fake; end}\n x=<<~TXT\n end def fake; end\n TXT\nend\ndef endless = 1\n");
    assert(r.functions.length==2 && r.functions[0].end==6 && r.functions[1].end==7);
}

module rust_port;
import reuse : TokenPool;
import brace_port_cleaner : nextRune;
import brace_port_unicode : goLetter,goDigit;
import brace_port_model : Result,Bounds;
import std.algorithm : canFind;
import std.uni : isWhite;

private enum keywords=["as","async","await","break","const","continue","crate","dyn","else","enum","extern","false","fn","for","if","impl","in","let","loop","match","mod","move","mut","pub","ref","return","self","Self","static","struct","super","trait","true","type","union","unsafe","use","where","while","yield"];

struct RustToken {string text;size_t start,end;int line;bool word;}
struct RustTokens {RustToken[] items;int[] pairs;}
private bool head(dchar c){return c=='_' || goLetter(c);}
private bool tail(dchar c){return head(c) || goDigit(c) || c>=0x300 && c<=0x36f;}

// Returns the byte after an actual character literal, otherwise preserves the
// apostrophe for a lifetime/label. Escapes never consume unrelated source lines.
private size_t characterEnd(string source,size_t quote) {
    auto i=quote+1;if(i>=source.length || source[i]=='\n' || source[i]=='\r')return quote;
    if(source[i]=='\\') {
        ++i;if(i>=source.length)return quote;
        if(source[i]=='u' && i+1<source.length && source[i+1]=='{') {
            i+=2;while(i<source.length && source[i]!='}' && source[i]!='\n')++i;
            if(i<source.length && source[i]=='}')++i;
        } else if(source[i]=='x')i+=3;
        else ++i;
    } else nextRune(source,i);
    return i<source.length && source[i]=='\''?i+1:quote;
}

// the tokens of the previous file on this thread (reuse.d): valid until the next call
private TokenPool!RustToken pool;
RustTokens tokenizeRust(string source) {
    alias output=pool;pool.reset();auto stack=pool.stack[];size_t i;int line=1;
    while(i<source.length) {
        auto start=i;auto atLine=line;auto c=cast(ubyte)source[i];
        if(c=='\n'){++i;++line;continue;}
        if(c=='/' && i+1<source.length && source[i+1]=='/') {
            i+=2;while(i<source.length && source[i]!='\n')++i;continue;
        }
        if(c=='/' && i+1<source.length && source[i+1]=='*') {
            i+=2;int depth=1;
            while(i<source.length && depth) {
                if(i+1<source.length && source[i..i+2]=="/*"){++depth;i+=2;}
                else if(i+1<source.length && source[i..i+2]=="*/"){--depth;i+=2;}
                else {if(source[i]=='\n')++line;++i;}
            }
            continue;
        }
        auto quote=i;
        if((c=='b' || c=='c') && quote+1<source.length && (source[quote+1]=='r' || source[quote+1]=='"' || source[quote+1]=='\''))++quote;
        bool raw;size_t hashes;
        if(source[quote]=='r') {
            auto q=quote+1;while(q<source.length && source[q]=='#'){++hashes;++q;}
            if(q<source.length && source[q]=='"'){quote=q;raw=true;}
        }
        if(source[quote]=='"') {
            i=quote+1;
            while(i<source.length) {
                if(source[i]=='\n')++line;
                if(!raw && source[i]=='\\' && i+1<source.length) {if(source[i+1]=='\n')++line;i+=2;continue;}
                if(source[i]=='"') {
                    auto end=i+1;size_t n;
                    while(n<hashes && end<source.length && source[end]=='#'){++end;++n;}
                    if(n==hashes){i=end;break;}
                }
                ++i;
            }
            continue;
        }
        if(source[quote]=='\'') {auto end=characterEnd(source,quote);if(end>quote){i=end;continue;}}
        auto ch=nextRune(source,i);if(isWhite(ch))continue;
        bool word=head(ch);
        if(word) {
            if(ch=='r' && i<source.length && source[i]=='#')++i;
            while(i<source.length){auto end=i;auto next=nextRune(source,end);if(!tail(next))break;i=end;}
        }
        auto index=cast(int)output.items.length;
        auto token=RustToken(source[start..i],start,i,atLine,word);
        output.items~=token;output.pairs~=-1;
        auto open=token.text=="("?0:token.text=="["?1:token.text=="{"?2:-1;
        if(open>=0)stack[open]~=index;
        else {
            auto close=token.text==")"?0:token.text=="]"?1:token.text=="}"?2:-1;
            if(close>=0 && stack[close].length){auto other=stack[close][$-1];stack[close].popBack();output.pairs[index]=other;output.pairs[other]=index;}
        }
    }
    version(AuditRustTokens) foreach(n,t;output.items) {
        assert(t.start<t.end && t.end<=source.length && source[t.start..t.end]==t.text);
        if(n)assert(output.items[n-1].end<=t.start && output.items[n-1].line<=t.line);
        auto p=output.pairs[n];if(p>=0)assert(p<output.items.length && output.pairs[p]==n);
    }
    return RustTokens(pool.items.data,pool.pairs.data);
}

private struct Scan {
    RustToken[] t;int[] pair;
    bool at(int i,string text){return i>=0 && i<t.length && t[i].text==text;}
    bool word(int i){return i>=0 && i<t.length && t[i].word;}
    int angleEnd(int start) {
        int depth=1;
        for(int i=start+1;i<t.length;++i) {
            if(at(i,"<"))++depth;
            else if(at(i,">") && !at(i-1,"-") && --depth==0)return i+1;
            else if(at(i,"(") || at(i,"[") || at(i,"{")){if(pair[i]<i)return -1;i=pair[i];}
            else if(at(i,";"))return -1;
        }
        return -1;
    }
    int end(int start) {
        for(auto i=start;i<t.length;++i) {
            if(at(i,"<")){auto next=angleEnd(i);if(next<0)return -1;i=next-1;}
            else if(at(i,"(") || at(i,"[")){if(pair[i]<i)return -1;i=pair[i];}
            else if(at(i,"{"))return pair[i]>i?pair[i]:-1;
            else if(at(i,";"))return i;
            else if(at(i,"}"))return -1;
        }
        return -1;
    }
    int prefix(int keyword) {
        auto start=keyword;
        while(start>0) {
            if(["pub","async","unsafe","const","extern","default","auto"].canFind(t[start-1].text)){--start;continue;}
            if(at(start-1,")") && pair[start-1]>=1 && at(pair[start-1]-1,"pub")){start=pair[start-1]-1;continue;}
            break;
        }
        return start;
    }
    string implOwner(int start,int finish) {
        auto i=start+1;if(at(i,"<")){i=angleEnd(i);if(i<0)return "";}
        auto target=i;
        for(;i<finish;++i){if(at(i,"where") || at(i,"{"))break;if(at(i,"for"))target=i+1;if(at(i,"<")){auto next=angleEnd(i);if(next<0)break;i=next-1;}}
        string owner;for(i=target;i<finish && !at(i,"where") && !at(i,"{");++i)owner~=t[i].text;
        return owner;
    }
}

Result findRust(string source) {
    auto tokens=tokenizeRust(source);auto t=tokens.items;auto p=tokens.pairs;auto s=Scan(t,p);Result result;
    struct Scope {string name;int end;}Scope[] scopes;
    for(int i=0;i<t.length;++i) {
        while(scopes.length && scopes[$-1].end<i)scopes.length--;
        // Token trees are macro input, not parsed Rust items. No expansion is
        // attempted; actual nested functions in executable bodies remain visible.
        if(s.at(i,"!") && i>0 && s.word(i-1) && !keywords.canFind(t[i-1].text)) {
            auto open=i+1;if(s.at(i-1,"macro_rules") && s.word(open))++open;
            if((s.at(open,"{") || s.at(open,"(") || s.at(open,"[")) && p[open]>open){i=p[open];continue;}
        }
        if(s.at(i,"#") && (s.at(i+1,"[") || s.at(i+1,"!") && s.at(i+2,"["))) {auto open=s.at(i+1,"[")?i+1:i+2;if(p[open]>open)i=p[open];continue;}
        if(s.at(i,"fn") && s.word(i+1)) {
            auto name=i+1;auto open=name+1;
            if(s.at(open,"<")){open=s.angleEnd(open);if(open<0)continue;}
            if(!s.at(open,"(") || p[open]<open)continue;
            auto end=s.end(p[open]+1);if(end<open)continue;
            auto owner=scopes.length?scopes[$-1].name:"";
            result.functions~=Bounds(t[name].text,t[s.prefix(i)].line,t[end].line,owner,t[name].line,t[name].text);
        } else if(["struct","enum","union","trait","type"].canFind(t[i].text) && s.word(i+1)) {
            auto end=s.end(i+2);if(end<i)continue;
            // Tuple structs end at ';', after their balanced field tuple.
            result.classes~=Bounds(t[i+1].text,t[s.prefix(i)].line,t[end].line);
            if(s.at(i,"trait"))scopes~=Scope(t[i+1].text,end);
        } else if(s.at(i,"impl") && (i==0 || ["{","}",";","]","unsafe","default"].canFind(t[i-1].text))) {
            auto end=s.end(i+1);if(end>i)scopes~=Scope(s.implOwner(i,end),end);
        }
    }
    return result;
}

unittest {
    auto r=findRust("pub trait T {type Item;fn required(&self);fn provided(&self) {fn nested(){} }}\nimpl<'a> T for S<'a> {type Item=fn(i32)->i32;pub unsafe extern \"C\" fn run<const N: usize>(&self)->[u8;N] where Self:Sized {let c='}';let l: &'a str=r#####\"fn fake() {}\"#####;}}\n");
    string[] names;foreach(f;r.functions)names~=f.name;
    assert(names==["required","provided","nested","run"]);
    assert(r.functions[0].className=="T" && r.functions[3].className=="S<'a>" && r.functions[3].end==2);
    r=findRust("/* outer\n/* inner */ fn hidden(){} */\nfn real(){let c='\\u{7d}';let b=b'{';let s=br###\"}\"###;}\nmacro_rules! item {()=>{fn fake(){}}}\ncall!{fn fake2(){}}\nfn r#type(){}\n");
    assert(r.functions.length==2 && r.functions[0].name=="real" && r.functions[0].start==3 && r.functions[1].name=="r#type");
    r=findRust("fn generic<T: Fn() -> Vec<u8>>() -> impl Fn() -> i32 {|| 1}\nstruct Tuple(pub fn() -> i32);\nstruct Unit;\ntype Alias = fn(i32)->i32;\n");
    assert(r.functions.length==1 && r.functions[0].end==1 && r.classes.length==3);
    assert(!findRust("fn unfinished<T(").functions.length);
    r=findRust("impl T for ! {fn method(&self){}}\nfn f()->impl Fn(){fn nested(){} || ()}\n");
    assert(r.functions.length==3 && r.functions[0].name=="method" && r.functions[0].className=="!" && !r.functions[2].className.length);
}

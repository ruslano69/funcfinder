module typescript_candidates;
import js_ts_cleaner;
import js_ts_model;
import javascript_port;
import std.string : strip,startsWith,endsWith;
import std.algorithm : canFind;

private enum modifiers=["public","private","protected","static","async","override","readonly","abstract","get","set"];
private enum recoveryModifiers=modifiers~["export","default","declare"];
struct CandidateKey { string name;int line; }
private int nameEnd(string s,int i=0) {auto pos=cast(size_t)i;while(pos<s.length){auto first=pos;if(!jsWord(nextRune(s,pos))) return cast(int)first;}return cast(int)pos;}
private int count(ref int[CandidateKey] known,CandidateKey key) {auto p=key in known;return p ? *p : 0;}
bool candidateFile(string[] raw,LineFact[] facts,ref int[CandidateKey] known) {
    foreach(i,f;facts) {
        if(f.startState!=0) continue;
        auto text=f.cleaned.strip;string kind;
        while(true) {
            auto end=nameEnd(text);auto word=text[0..end];
            if(word=="function" || word=="const" || word=="let" || word=="var") {kind=word;text=text[end..$].strip;if(text.startsWith("*")) text=text[1..$].strip;break;}
            if(!recoveryModifiers.canFind(word) || end==text.length || (text[end]!=' ' && text[end]!='\t')) break;
            text=text[end..$].strip;
        }
        auto end=nameEnd(text);
        if(!end && text.startsWith("(") && (raw[i].canFind('"') || raw[i].canFind('\''))) return true;
        if(!end || end>text.length) continue;
        auto name=text[0..end],rest=text[end..$].strip;
        if(count(known,CandidateKey(name,cast(int)i+1))) continue;
        if(kind=="function" && (rest.startsWith("(") || rest.startsWith("<"))) return true;
        if(kind=="const" || kind=="let" || kind=="var") {
            if(rest.endsWith("=")) foreach(j;i+1..facts.length) {
                auto next=facts[j].cleaned.strip;if(!next.length) continue;
                if(next.startsWith("(") || next.startsWith("<") || next.startsWith("async") || next.startsWith("function")) return true;
                break;
            }
            if(name.canFind('$') && f.cleaned.canFind("=>")) return true;
            if(rest.startsWith("=")) {auto rhs=rest[1..$].strip;if(rhs.startsWith("async ")) rhs=rhs[6..$].strip;auto n=nameEnd(rhs);if(n>0 && rhs[n..$].strip.startsWith("=>")) return true;}continue;
        }
        if((rest.startsWith("<") && candidateGeneric(rest)) || (name=="catch" && rest.startsWith("("))) return true;
        if(rest.startsWith("(") && f.cleaned.strip.endsWith("(")) foreach(j;i+1..facts.length) {
            auto next=facts[j].cleaned.strip;if(!next.length) continue;auto n=nameEnd(next);
            if(n>0 && next[n..$].strip.startsWith(":")) return true;
            break;
        }
    }
    return false;
}
bool candidateGeneric(string s) {
    int depth;
    foreach(i,c;s) {
        if(c=='<') ++depth;if(c!='>' || (i>0 && s[i-1]=='=')) continue;--depth;if(depth>0) continue;
        auto j=i+1;while(j<s.length && (s[j]==' ' || s[j]=='\t')) ++j;if(j>=s.length || s[j]!='(') return false;
        int parens;for(;j<s.length;++j) {if(s[j]=='(') ++parens;if(s[j]==')') {--parens;if(!parens) {auto tail=s[j+1..$].strip;return tail.startsWith(":") || tail.startsWith("{");}}}return true;
    }
    return true;
}
int angleEnd(Token[] t,int[] pairs,int i) {
    if(i>=t.length || t[i].text!="<") return i;int depth;
    for(;i<t.length;++i) switch(t[i].text) {
        case "<":++depth;break;
        case ">":if(i>0 && t[i-1].text=="=") continue;--depth;if(!depth) return i+1;break;
        case "(":case "[":case "{":if(pairs[i]>i) i=pairs[i];break;
        case ";":return -1;
        default:break;
    }
    return -1;
}
int afterParams(Token[] t,int[] pairs,int i) {
    if(i>=t.length) return -1;
    if(t[i].text!=":") return t[i].text=="{" || t[i].text==";" ? i : -1;
    bool atom=true;
    for(++i;i<t.length;++i) switch(t[i].text) {
        case "{":if(atom && pairs[i]>i) {i=pairs[i];atom=false;}else return i;break;
        case "(":case "[":if(pairs[i]<=i) return -1;i=pairs[i];atom=false;break;
        case "<":auto end=angleEnd(t,pairs,i);if(end<0) return -1;i=end-1;atom=false;break;
        case ";":return i;
        case "}":case "=":return -1;
        case "|":case "&":case "?":case ":":atom=true;break;
        default:atom=false;break;
    }
    return -1;
}
int functionShape(Token[] t,int[] pairs,int i,out int name) {
    name=-1;++i;if(i<t.length && t[i].text=="*") ++i;
    if(i<t.length && identifier(t[i])) {name=i;++i;}
    i=angleEnd(t,pairs,i);if(i<0 || i>=t.length || t[i].text!="(" || pairs[i]<=i) return -1;
    return afterParams(t,pairs,pairs[i]+1);
}
int arrowBody(Token[] t,int[] pairs,int i) {
    if(i>=t.length) return -1;if(t[i].text=="async" && !arrowAt(t,i+1)) ++i;i=angleEnd(t,pairs,i);
    if(i<0 || i>=t.length) return -1;
    if(t[i].text=="(" && pairs[i]>i) i=pairs[i]+1;else if(identifier(t[i])) ++i;else return -1;
    if(i<t.length && t[i].text==":") for(++i;i<t.length;++i) {
        if(arrowAt(t,i)) break;
        if(t[i].text=="(" || t[i].text=="[" || t[i].text=="{") {if(pairs[i]<=i) return -1;i=pairs[i];}
        if(t[i].text=="<") {auto end=angleEnd(t,pairs,i);if(end<0) return -1;i=end-1;}
        if(t[i].text==";" || t[i].text=="}") return -1;
    }
    return arrowAt(t,i) && i+2<t.length ? i+2 : -1;
}
int arrowEnd(Token[] t,int[] pairs,int body) {
    while(true) {if(t[body].text=="{") return pairs[body];auto next=arrowBody(t,pairs,body);if(next<0) return expressionEnd(t,pairs,body);body=next;}
}
bool directValue(Token[] t,int end) {
    if(end<0) return false;if(end+1>=t.length) return true;
    return !["as","satisfies","(",".","[","!"].canFind(t[end+1].text);
}
void recoverTypeScript(string[] raw,LineFact[] facts,ref Result result) {
    int[CandidateKey] known;foreach(fn;result.functions) ++known[CandidateKey(fn.name,fn.start)];
    if(!candidateFile(raw,facts,known)) return;
    int[] pairs;auto t=tokenize(raw,facts,pairs);bool[int] bodies,classBodies;
    void add(string name,int first,int end) {
        if(end<first || end>=t.length) return;
        auto key=CandidateKey(name,t[first].line+1);if(count(known,key)) {--known[key];return;}
        auto fn=Bounds(name,key.line,t[end].end+1,classForLine(result.classes,key.line-1));
        if(name.startsWith("\"") || name.startsWith("'")) for(int j=0;j<result.functions.length;++j) if(result.functions[j].start==fn.start && result.functions[j].name=="async") {result.functions=result.functions[0..j]~result.functions[j+1..$];--j;}
        result.functions~=fn;
    }
    foreach(index,token;t) {
        auto i=cast(int)index;if(token.literal) continue;
        if(token.text=="function") {
            int name;auto body=functionShape(t,pairs,i,name);if(body<0) continue;bodies[body]=true;
            auto prev=i-1;if(prev>=0 && t[prev].text=="async") --prev;
            if(name>=0 && (prev<0 || "{};".canFind(t[prev].text) || t[prev].text=="export" || t[prev].text=="default")) add(t[name].text,i,t[body].text==";" ? body : pairs[body]);
        }
        if(token.text=="class" || token.text=="interface") for(auto j=i+1;j<t.length;++j) {
            if(t[j].text=="<") {auto end=angleEnd(t,pairs,j);if(end<0) break;j=end-1;continue;}
            if(t[j].text=="(" && pairs[j]>j) {j=pairs[j];continue;}
            if(t[j].text=="{") {classBodies[j]=true;break;}if(t[j].text==";") break;
        }
        if(token.text!="const" && token.text!="let" && token.text!="var") continue;
        auto name=i+1;if(name>=t.length || !identifier(t[name])) continue;
        for(auto j=name+1;j<t.length;++j) {
            if(t[j].text=="=" && !arrowAt(t,j)) {
                auto value=j+1;auto body=arrowBody(t,pairs,value);
                if(body>=0) {bodies[body]=true;auto end=arrowEnd(t,pairs,body);if(directValue(t,end)) add(t[name].text,name,end);}
                else if(value<t.length && t[value].text=="function") {int ignored;auto b=functionShape(t,pairs,value,ignored);if(b>=0 && t[b].text=="{") {bodies[b]=true;if(directValue(t,pairs[b])) add(t[name].text,name,pairs[b]);}}
                break;
            }
            if(t[j].text=="(" || t[j].text=="[" || t[j].text=="{") {if(pairs[j]<=j) break;j=pairs[j];continue;}
            if(t[j].text=="<") {auto end=angleEnd(t,pairs,j);if(end<0) break;j=end-1;continue;}
            if([";",",",")","}"].canFind(t[j].text)) break;
        }
    }
    string[] scopes;
    foreach(index,token;t) {
        auto i=cast(int)index;
        if(scopes.length && (scopes[$-1]=="class" || scopes[$-1]=="object") && (identifier(token) || (token.literal && (token.text.startsWith("\"") || token.text.startsWith("'"))))) {
            auto first=i;while(first>0) {auto modifier=t[first-1].text;if(modifier!="*" && !modifiers.canFind(modifier)) break;--first;}
            if(first>0 && "{};,".canFind(t[first-1].text)) {
                auto open=i+1;if(open<t.length && t[open].text=="?") ++open;open=angleEnd(t,pairs,open);
                if(open>=0 && open<t.length && t[open].text=="(" && pairs[open]>open) {
                    auto body=afterParams(t,pairs,pairs[open]+1);
                    if(body>=0) {if(t[body].text=="{" && pairs[body]>body) {bodies[body]=true;add(token.text,first,pairs[body]);}else if(t[body].text==";" && scopes[$-1]=="class" && token.text!="new") add(token.text,first,body);}
                }
            }
        }
        if(token.literal) continue;
        switch(token.text) {
            case "{":
                string kind="block";
                if(i in classBodies) kind="class";else if(!(i in bodies) && i>0 && ["=","(","[",",",":","?","return"].canFind(t[i-1].text)) kind="object";
                if(kind=="object" && i>0 && t[i-1].text==":") for(auto j=i-2;j>=0;--j) {if(t[j].text=="case" || t[j].text=="default") {kind="block";break;}if("{};:".canFind(t[j].text)) break;}
                scopes~=kind;break;
            case "(":case "[":scopes~="block";break;
            case "}":case ")":case "]":if(scopes.length) scopes.length--;break;
            default:break;
        }
    }
}

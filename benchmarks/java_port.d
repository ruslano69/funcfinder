module java_port;
import brace_port_cleaner : LineFact;
import brace_port_model : Result,Bounds;
import csharp_port : Token,tokenize,identifier;
import std.string : startsWith,replace;
import std.algorithm : canFind;

private enum modifiers=["public","private","protected","static","final","abstract","synchronized","native","strictfp","default","sealed"];
private enum statements=["return","throw","new","if","else","for","while","do","switch","case","catch","try","finally","assert","break","continue","class","interface","enum","record","package","import","this","super","instanceof"];
bool javaIdentifier(string text) {return identifier(text.replace("$","_")) && !text.startsWith("@");}
int prefix(Token[] t,int[] p,int start,int end) {
    while(start<end){auto word=t[start].text;
        if(word.startsWith("@") && word!="@interface"){++start;while(start+1<end && t[start].text==".")start+=2;if(start<end && t[start].text=="(" && p[start]>start)start=p[start]+1;}
        else if(modifiers.canFind(word))++start;
        else if(start+2<end && word=="non" && t[start+1].text=="-" && t[start+2].text=="sealed")start+=3;
        else break;
    }
    return start;
}
int typeEnd(Token[] t,int[] p,int start,int end) {
    auto i=prefix(t,p,start,end);if(i>=end || !javaIdentifier(t[i].text) || statements.canFind(t[i].text))return start;++i;
    while(i<end)switch(t[i].text){
        case "<":int depth=1;++i;while(i<end && depth>0){if(t[i].text=="<")++depth;else if(t[i].text==">")--depth;++i;}if(depth)return start;break;
        case ".":i=prefix(t,p,i+1,end);if(i>=end || !javaIdentifier(t[i].text))return start;++i;break;
        case "[":if(p[i]!=i+1 || p[i]>=end)return start;i+=2;break;
        default:auto next=prefix(t,p,i,end);if(next==i)return i;i=next;break;
    }
    return i;
}
string functionName(Token[] t,int[] p,int start,int open,string cls) {
    start=prefix(t,p,start,open);
    if(start<open && t[start].text=="<"){int depth=1;++start;while(start<open && depth>0){if(t[start].text=="<")++depth;else if(t[start].text==">")--depth;++start;}if(depth)return "";}
    start=prefix(t,p,start,open);auto nameAt=open-1;
    if(start>nameAt || !javaIdentifier(t[nameAt].text))return "";auto name=t[nameAt].text;if(statements.canFind(name))return "";
    return start==nameAt && name==cls || start<nameAt && typeEnd(t,p,start,nameAt)==nameAt?name:"";
}
int functionEnd(Token[] t,int[] p,int close,int limit,out int body) {
    body=-1;bool value;
    for(auto i=close+1;i<limit;++i)switch(t[i].text){
        case "default":value=true;break;
        case "(":case "[":if(p[i]<i || p[i]>=limit)return -1;i=p[i];break;
        case "{":if(p[i]<i || p[i]>=limit)return -1;if(!value){body=i;return p[i];}i=p[i];break;
        case ";":return i;
        case "}":return -1;
        default:break;
    }
    return -1;
}
Result findJava(LineFact[] facts) {
    Token[] t;int[] p;tokenize(facts,t,p,true);Result result;
    struct Scope {int end;string cls;bool members,record,isEnum;}
    Scope[] scopes=[Scope(cast(int)t.length)];int boundary,functionBody=-1;
    void append(string name,int start,int nameAt,int end,string cls) {
        auto f=Bounds(name,t[start].line+1,t[end].line+1,cls);
        if(nameAt>=0){f.signatureLine=t[nameAt].line+1;f.signatureName=name;}result.functions~=f;
    }
    for(int i=0;i<t.length;++i){auto parent=scopes[$-1];
        switch(t[i].text){
            case "{":if(p[i]<=i)break;auto child=Scope(p[i],parent.cls);auto start=prefix(t,p,boundary,i);
                if(i!=functionBody && start<i){
                    if(["class","interface","enum","record","@interface"].canFind(t[start].text)){
                        if(start+1<i && javaIdentifier(t[start+1].text)){child.cls=t[start+1].text;child.members=true;child.record=t[start].text=="record";child.isEnum=t[start].text=="enum";result.classes~=Bounds(child.cls,t[start].line+1,t[p[i]].line+1);}
                    }else if(parent.members && parent.record && start+1==i && t[start].text==parent.cls)append(parent.cls,boundary,-1,p[i],parent.cls);
                    else{foreach(j;boundary..i)if(t[j].text=="new" && t[i-1].text==")"){child.members=true;break;}if(parent.members && parent.isEnum && (t[i-1].text==")" || start+1==i && javaIdentifier(t[start].text)))child.members=true;}
                }
                scopes~=child;boundary=i+1;break;
            case "}":if(scopes.length>1 && scopes[$-1].end==i)scopes.length--;boundary=i+1;break;
            case ";":if(parent.members)scopes[$-1].isEnum=false;boundary=i+1;break;
            case ",":if(parent.members && parent.isEnum)boundary=i+1;break;
            case "(":if(p[i]<i)break;
                auto declaration=prefix(t,p,boundary,i);
                if(declaration<i && ["class","interface","enum","record","@interface"].canFind(t[declaration].text)){i=p[i];break;}
                if(boundary<i && t[boundary].text.startsWith("@") && prefix(t,p,boundary,i)>=i){i=p[i];break;}
                if(!parent.members)break;auto name=functionName(t,p,boundary,i,parent.cls);if(!name.length)break;int body;auto end=functionEnd(t,p,p[i],parent.end,body);if(end<i)break;append(name,boundary,i-1,end,parent.cls);
                if(body>=0){functionBody=body;i=body-1;}else{i=end;boundary=end+1;}break;
            default:break;
        }
    }
    return result;
}

unittest {
    import brace_port_cleaner : Cleaner;
    import std.array : split;
    Cleaner c;c.charLiterals=true;
    auto r=findJava(c.facts(("@interface A {int[] value() default {1,2};}\nrecord R(int x) {R {if(x<0)throw new Error();} int f(){return x;}}\nclass C {void outer(){class L {int inner(){return 1;}} Runnable r=new Runnable(){public void run(){}};}}\n").split("\n")));
    string[] names;foreach(f;r.functions)names~=f.name;assert(names==["value","R","f","outer","inner","run"]);
    assert(r.functions[0].end==1 && r.functions[1].signatureLine==0 && r.classes.length==4);
    c=Cleaner();c.charLiterals=true;
    r=findJava(c.facts(("class C {\n @a.b.A({1,2}) public <T> T[] f(T x) throws Exception {return null;}\n void $name();\n}\n").split("\n")));
    assert(r.functions.length==2 && r.functions[0].name=="f" && r.functions[0].end==2 && r.functions[1].name=="$name");
    c=Cleaner();c.charLiterals=true;
    r=findJava(c.facts(("record R(\n @A(values={1,2})\n int x) {\n}\n").split("\n")));
    assert(r.classes.length==1 && r.classes[0].start==1 && r.classes[0].end==4);
}

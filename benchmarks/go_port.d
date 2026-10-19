module go_port;
import brace_port_cleaner : LineFact;
import brace_port_model : Result,Bounds;
import csharp_port : Token,tokenize;
import std.algorithm : canFind;

private enum keywords=["break","default","func","interface","select","case","defer","go","map","struct","chan","else","goto","package","switch","const","fallthrough","if","range","type","continue","for","import","return","var"];
struct GoScan {
    Token[] t;int[] pair;
    bool at(int i,string text){return i>=0 && i<t.length && t[i].text==text;}
    bool word(int i){if(i<0 || i>=t.length || keywords.canFind(t[i].text))return false;import csharp_port:identifier;return identifier(t[i].text) && !at(i,"@");}
    bool semi(int i){return word(i) || at(i,")") || at(i,"]") || at(i,"}") || t[i].text=="any" || t[i].text=="interface";}
    int typeExpressionEnd(int start,int limit) {
        auto last=start-1;
        for(auto i=start;i<limit;++i){
            if(i>start && t[i].line>t[last].line && semi(last))return last;
            if(at(i,";"))return last;
            if(at(i,"(") || at(i,"[") || at(i,"{")){if(pair[i]<i || pair[i]>=limit)return last;i=pair[i];}
            last=i;
        }
        return last;
    }
    int functionEnd(int close) {
        auto last=close;
        for(auto i=close+1;i<t.length;++i){
            if(t[i].line>t[last].line && semi(last))return last;
            if(at(i,";"))return last;
            if(at(i,"{")){
                if(pair[i]<i)return -1;
                if(at(i-1,"struct") || at(i-1,"interface")){i=pair[i];last=i;continue;}
                return pair[i];
            }
            if(at(i,"(") || at(i,"[")){if(pair[i]<i)return -1;i=pair[i];}
            last=i;
        }
        return last;
    }
}
Result findGo(LineFact[] facts) {
    Token[] t;int[] p;tokenize(facts,t,p);auto scan=GoScan(t,p);Result result;
    for(int i=0;i<t.length;++i){
        if(scan.at(i,"func")){
            auto name=i+1;if(scan.at(name,"(")){if(p[name]<name)continue;name=p[name]+1;}
            if(!scan.word(name))continue;auto open=name+1;
            if(scan.at(open,"[")){if(p[open]<open)continue;open=p[open]+1;}
            if(!scan.at(open,"(") || p[open]<open)continue;auto end=scan.functionEnd(p[open]);if(end<open)continue;
            result.functions~=Bounds(t[name].text,t[i].line+1,t[end].line+1,"",t[name].line+1,t[name].text);
        }else if(scan.at(i,"type")){
            auto start=i+1;bool group=scan.at(start,"(");int limit=cast(int)t.length;
            if(group){if(p[start]<start)continue;limit=p[start];++start;}
            while(start<limit){
                if(scan.at(start,";")){++start;continue;}
                if(!scan.word(start))break;auto expr=start+1;
                if(scan.at(expr,"[")){if(p[expr]<expr)break;expr=p[expr]+1;}
                if(scan.at(expr,"="))++expr;
                if(expr>=limit)break;auto end=scan.typeExpressionEnd(expr,limit);if(end<expr)break;
                result.classes~=Bounds(t[start].text,t[start].line+1,t[end].line+1);
                if(!group)break;start=end+1;
            }
        }
    }
    return result;
}

unittest {
    import brace_port_cleaner : Cleaner;
    import std.array : split;
    Result parse(string text){Cleaner c;c.golang=true;c.charLiterals=true;return findGo(c.facts(text.split("\n")));}
    auto r=parse("package p\ntype (\n S[T any] struct{v T}\n I interface{M()}\n Alias = []int\n Scalar int\n Fn func(int) int\n)\nfunc F[T interface{~[]byte|string}](x T) T {type Local struct{};return x}\nfunc(s S[T]) M(x func() struct{v int}) interface{M()} {return nil}\nfunc Asm(x int) (n int)\nvar V=func(x int) int{return x}\n");
    string[] names;foreach(f;r.functions)names~=f.name;assert(names==["F","M","Asm"]);
    foreach(i,f;r.functions)assert(f.start==i+9 && f.end==i+9);
    assert(r.classes.length==6);assert(r.classes[0].name=="S" && r.classes[0].start==3 && r.classes[0].end==3);
    r=parse("package p\nfunc Asm(x int) int // native body\nfunc Full() {\n var f=func() string {return `fake func F() {}`}; _=f\n}\n");
    assert(r.functions.length==2 && r.functions[0].end==2 && r.functions[1].end==5);
    r=parse("package p\ntype A = struct {\n V func() struct{I int}\n}\nfunc F() func()struct{I int} {return nil}\n");
    assert(r.classes.length==1 && r.classes[0].end==4);assert(r.functions.length==1 && r.functions[0].end==5);
}

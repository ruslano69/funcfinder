module c_port;
import cpp_tokenizer;
import brace_port_cleaner : LineFact;
import brace_port_model : Result,Bounds;
import std.algorithm : canFind;
import std.conv : to;

// The first operator outward from the name determines whether a declarator
// declares a function or a pointer/array variable. Parentheses change that order.
private struct Declarator { int name=-1, next=-1, params=-1; char first=0; }
private struct Scan {
    CppToken[] t; int[] pair;
    bool at(int i,string s) { return i>=0 && i<t.length && t[i].text==s; }
    bool word(int i) { return i>=0 && i<t.length && t[i].word; }
    bool attribute(int i) {return word(i) && ["__attribute__","__attribute","__declspec","asm","__asm__","__asm"].canFind(t[i].text);}
    int attributes(int i) {
        while(attribute(i) && at(i+1,"(") && pair[i+1]>i+1)i=pair[i+1]+1;
        return i;
    }
    Declarator declarator(int i,int depth=0) {
        if(depth>256)return Declarator.init;
        bool pointer;
        while(at(i,"*")) {
            pointer=true;++i;
            while(word(i) && ["const","volatile","restrict","__restrict","__restrict__","_Atomic"].canFind(t[i].text))++i;
            i=attributes(i);
        }
        Declarator d;
        if(word(i)) {d.name=i;d.next=i+1;}
        else if(at(i,"(") && pair[i]>i) {
            d=declarator(i+1,depth+1);
            if(d.next!=pair[i])return Declarator.init;
            d.next=pair[i]+1;
        } else return Declarator.init;
        bool functionSuffix;
        while(at(d.next,"(") || at(d.next,"[")) {
            // A function cannot directly return a function or an array. This
            // also disambiguates `Result (name)(args)` from a function Result.
            if(functionSuffix)return Declarator.init;
            auto open=d.next;if(pair[open]<=open)return Declarator.init;
            functionSuffix=at(open,"(");
            if(!d.first){d.first=at(open,"(")?'f':'a';if(d.first=='f')d.params=open;}
            d.next=pair[open]+1;
        }
        if(!d.first && pointer)d.first='p';
        return d;
    }
    // Only declaration specifiers may precede the declarator. Balanced macro
    // attributes are allowed, but expression operators and statement keywords aren't.
    bool prefix(int start,int end) {
        bool type;
        for(int i=start;i<end;++i) {
            if(word(i)) {
                if(["return","if","else","while","for","switch","case","goto","break","continue","do","sizeof","typedef","_Static_assert"].canFind(t[i].text))return false;
                type=true;
                if(at(i+1,"(") && pair[i+1]>i+1 && pair[i+1]<end)i=pair[i+1];
            } else if(at(i,"{") && pair[i]>i && pair[i]<end)i=pair[i];
            else return false;
        }
        return type;
    }
    int knrBody(Declarator d,int start) {
        if(d.params<0)return -1;
        for(int j=d.params+1;j<pair[d.params];++j)
            if(!at(j,",") && (!word(j) || ["void","int","char","long","short","float","double","struct","union","enum"].canFind(t[j].text)))return -1;
        bool declaration;
        for(int j=start;j<t.length;++j) {
            if(at(j,"{"))return declaration?j:-1;
            if(at(j,";") )declaration=true;
            else if(at(j,"(") || at(j,"[")){if(pair[j]<j)return -1;j=pair[j];}
            else if(!word(j) && !at(j,"*") && !at(j,","))return -1;
        }
        return -1;
    }
}

Result findC(string[] lines,LineFact[] facts) {
    auto tokens=tokenizeCpp(facts,lines);auto s=Scan(tokens.items,tokens.pairs);auto t=s.t;
    Result result;int boundary,executableEnd=-1;
    for(int i=0;i<t.length;++i) {
        if(s.at(i,";") || s.at(i,"}") || s.at(i,"{")){boundary=i+1;continue;}
        while(boundary<i && t[boundary].segment<t[i].segment)++boundary;
        // In executable scope a bare word(args); is an invocation, even when
        // args contain another call. Otherwise assert(check(x)); looks like a
        // parenthesized declarator named check with return type assert.
        if(i<executableEnd && i==boundary && s.word(i) && s.at(i+1,"(") && s.pair[i+1]>i+1 && (s.at(s.pair[i+1]+1,";") || s.at(s.pair[i+1]+1,"{")) &&
           !["void","char","short","int","long","float","double","signed","unsigned","_Bool","_Complex","_Atomic","extern","static","const","volatile","struct","union","enum"].canFind(t[i].text)) {
            i=s.pair[i+1];if(s.at(i+1,";"))++i;boundary=i+1;continue;
        }
        // Declaration-wrapper macros can contain semicolons in their arguments.
        // Skip that balanced invocation as a unit instead of leaving a ')' prefix.
        if(i==boundary && s.word(i) && s.at(i+1,"(") && s.pair[i+1]>i+1) {
            auto close=s.pair[i+1];bool macroName=true,semicolon;
            foreach(c;t[i].text)if(c>='a' && c<='z')macroName=false;
            foreach(j;i+2..close)if(s.at(j,";"))semicolon=true;
            if(macroName && semicolon){i=close;boundary=i+1;continue;}
        }
        if(!s.word(i) && !s.at(i,"*") && !s.at(i,"("))continue;
        if(!s.prefix(boundary,i))continue;
        auto d=s.declarator(i);if(d.next<0)continue;
        auto end=s.attributes(d.next);
        if(d.first=='f' && !s.at(end,";") && !s.at(end,",") && !s.at(end,"{")) {
            auto body=s.knrBody(d,end);if(body>=0)end=body;
        }
        if(!s.at(end,";") && !s.at(end,",") && !s.at(end,"{"))continue;
        if(d.first!='f') {
            // Skip complete pointer variables, not their nested function suffixes.
            if(d.first=='p' || d.first=='a')i=end-1;
            continue;
        }
        auto start=boundary;
        while(start<d.name && t[start].segment<t[d.name].segment)++start;
        auto add=(Declarator decl,int last) {
            result.functions~=Bounds(t[decl.name].text,t[start].line+1,t[last].line+1,"",t[decl.name].line+1,t[decl.name].text);
        };
        if(s.at(end,"{")) {
            if(s.pair[end]>end){add(d,s.pair[end]);executableEnd=s.pair[end];i=end;boundary=i+1;}
        } else {
            Declarator[] declarations=[d];auto last=end;
            while(s.at(last,",")) {
                auto next=s.declarator(last+1);if(next.next<0)break;
                declarations~=next;last=s.attributes(next.next);
            }
            if(s.at(last,";")) {
                foreach(decl;declarations)if(decl.first=='f')add(decl,last);
                i=last;boundary=i+1;
            }
        }
    }
    // Named tag definitions are raw type results, including nested tags.
    foreach(i,token;t)if(["struct","union","enum"].canFind(token.text) && s.word(cast(int)i+1) && s.at(cast(int)i+2,"{") && s.pair[i+2]>i+2)
        result.classes~=Bounds(t[i+1].text,token.line+1,t[s.pair[i+2]].line+1);
    return result;
}

unittest {
    import brace_port_cleaner : Cleaner;
    import std.array : split;
    Result parse(string text) {auto lines=text.split("\n");Cleaner cleaner;return findC(lines,cleaner.facts(lines));}
    auto r=parse("int f(void), *g(int), (*variable)(int);\nint (*factory(void))(int) {return 0;}\nstruct S *make(void) {return 0;}\nint (*callback)(int);\nint array[4];\nint value=call(1);\n");
    string[] names;foreach(f;r.functions)names~=f.name;
    assert(names==["f","g","factory","make"],names.to!string);
    assert(r.functions[0].start==1 && r.functions[0].end==1 && r.functions[2].end==2);
    r=parse("#define FAKE() \\\n int hidden(void) {}\nextern int\nreal(int (*cb)(int)) __attribute__((unused));\n");
    assert(r.functions.length==1 && r.functions[0].name=="real" && r.functions[0].start==3 && r.functions[0].end==4);
    r=parse("typedef int Fn(int);\nstruct S {int (*callback)(int);};\nint f(void) {if(1){return call(2);}return 0;} int g(void);\n");
    assert(r.functions.length==2 && r.functions[0].name=="f" && r.functions[1].name=="g");
    assert(r.classes.length==1 && r.classes[0].name=="S");
    r=parse("API Result (name)(int);\nAPI int (other)(void);\n");
    assert(r.functions.length==2 && r.functions[0].name=="name" && r.functions[1].name=="other");
    r=parse("int old(a,b)\nint a; char *b;\n{extern int local(void); return a;}\nint next(void);\n");
    assert(r.functions.length==3 && r.functions[0].name=="old" && r.functions[0].end==3 && r.functions[1].name=="local" && r.functions[2].name=="next");
    r=parse("int f(void){assert(check(x));check(x);extern int local(void);return 0;}\n");
    assert(r.functions.length==2 && r.functions[1].name=="local");
    r=parse("int f(void){switch_macro(OP(x)){case 1: break;}return 0;}\n");
    assert(r.functions.length==1 && r.functions[0].name=="f");
}

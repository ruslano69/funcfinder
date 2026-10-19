module cpp_tokens_port;
import cpp_tokenizer;
import brace_port_cleaner : LineFact;
import brace_port_model : Config,Result,Bounds;
import std.algorithm : canFind;
import std.array : appender;

private enum statements=["return","throw","co_return","co_await","co_yield","if","else","for","while","do","switch","case","catch","try","sizeof","decltype","alignof","alignas","noexcept","static_assert","using","typedef","namespace","delete","new"];
private enum qualifiers=["const","volatile","noexcept","override","final","constexpr","consteval","constinit","inline","static","virtual","explicit","extern","friend","typename","public","private","protected"];
private enum typeWords=["class","struct","union","enum"];

struct CppScan {
    CppToken[] t;int[] pair;string[] raw;
    bool at(int i,string text) {return i>=0 && i<t.length && t[i].text==text;}
    bool identifier(int i) {return i>=0 && i<t.length && t[i].word;}
    bool fence(int i) {
        if(!identifier(i))return false;auto s=t[i].text;
        foreach(c;s)if(c>='a' && c<='z')return false;
        return (s.canFind("BEGIN") || s.canFind("END")) && (s.canFind("NAMESPACE") || s.canFind("EXPORT"));
    }
    int headerStart(int start,int name) {
        while(start<name && fence(start))++start;
        // Conditional attributes outside the name's preprocessor segment do not
        // force the declaration's span to start in an inactive branch.
        while(start<name && t[start].segment<t[name].segment)++start;
        return start;
    }
    int angleEnd(int i) {
        if(!at(i,"<")) return i;
        int depth=1;
        for(++i;i<t.length;++i) {
            if(at(i,"<")) ++depth;
            else if(at(i,">") && --depth==0) return i+1;
            else if(at(i,"(") || at(i,"[") || at(i,"{")) {if(pair[i]<i)return -1;i=pair[i];}
            else if(at(i,";") || at(i,"}")) return -1;
        }
        return -1;
    }
    int angleStart(int i,int start) {
        int depth=1;
        for(--i;i>=start;--i) {
            if(at(i,">")) ++depth;
            else if(at(i,"<") && --depth==0)return i;
            else if(at(i,")") || at(i,"]")){if(pair[i]<start)return -1;i=pair[i];}
        }
        return -1;
    }
    int skipPrefix(int start,int end) {
        while(start<end && fence(start))++start;
        while(start<end) {
            if(at(start,"template") && at(start+1,"<")){auto next=angleEnd(start+1);if(next<0 || next>end)return end;start=next;}
            else if(at(start,"[") && pair[start]>start && pair[start]<end)start=pair[start]+1;
            else break;
        }
        return start;
    }
    string compact(int start,int end) {auto text=appender!string();foreach(token;t[start..end])text.put(token.text);return text.data;}
    string typeName(int start,int end,out int nameAt) {
        start=skipPrefix(start,end);nameAt=-1;
        for(auto i=start;i<end;++i) {
            if(at(i,"(") || at(i,"[") || at(i,"<")) {
                if(at(i,"<")){auto next=angleEnd(i);if(next<0)return "";i=next-1;}else if(pair[i]>=i)i=pair[i];
                continue;
            }
            if(typeWords.canFind(t[i].text)) {++i;if(at(i,"class") || at(i,"struct"))++i;if(!identifier(i))return "";
                bool macroName=t[i].text.canFind('_');foreach(ch;t[i].text)if(ch>='a' && ch<='z')macroName=false;
                if(macroName && i+1<end && identifier(i+1) && !at(i+1,"final"))++i;nameAt=i;return t[i].text;}
        }
        return "";
    }
    string functionName(int start,int open,string cls,out int nameAt) {
        nameAt=-1;start=skipPrefix(start,open);
        if(start>=open || statements.canFind(t[start].text) || at(start,"template") || at(start,"extern") && at(start+1,"template"))return "";
        int operatorAt=-1;
        for(auto i=start;i<open;++i) {
            if(at(i,"operator")){operatorAt=i;break;}
            if(at(i,"=") || at(i,".") || at(i,"?") || at(i,";") || at(i,"{") || at(i,"}"))return "";
            if(at(i,"(") || at(i,"[")){if(pair[i]<i || pair[i]>=open)return "";i=pair[i];}
        }
        if(operatorAt>=0) {
            if(operatorAt+1>=open)return "";nameAt=operatorAt;auto suffix=compact(operatorAt+1,open);
            if(identifier(operatorAt+1)) {
                auto first=operatorAt+1;
                if(raw.length && t[first].line==t[operatorAt].line) {
                    auto between=raw[t[first].line][t[operatorAt].endColumn..t[first].column];
                    if(between.canFind('"'))return "operator\"\""~suffix;
                }
                if(["new","delete","co_await"].canFind(t[first].text))return "operator "~suffix;
                // Aliases/dependent conversions need semantic type resolution;
                // keep the frozen finder's exclusion for those shapes.
                auto text=appender!string();bool previousWord;
                foreach(i;first..open) {
                    if(identifier(i) && !["void","bool","char","wchar_t","char8_t","char16_t","char32_t","int","short","long","float","double","signed","unsigned","const","volatile"].canFind(t[i].text))return "";
                    if(at(i,"<") || at(i,":"))return "";
                    if(previousWord && (identifier(i) || at(i,"*") || at(i,"&")))text.put(' ');
                    text.put(t[i].text);previousWord=identifier(i);
                }
                auto type=text.data;
                if(type=="unsigned")type="unsigned int";
                else if(type=="signed")type="int";
                return "operator "~type;
            }
            return "operator"~suffix;
        }
        auto name=open-1;
        if(at(name,">")){auto angle=angleStart(name,start);if(angle<0)return "";name=angle-1;}
        if(!identifier(name) || statements.canFind(t[name].text) || qualifiers.canFind(t[name].text))return "";
        nameAt=name;auto result=t[name].text;auto prefixEnd=name;
        if(at(prefixEnd-1,"~")){--prefixEnd;result="~"~result;}
        string owner;
        while(at(prefixEnd-1,":") && at(prefixEnd-2,":")) {
            auto id=prefixEnd-3;if(at(id,">")){auto angle=angleStart(id,start);if(angle<0)return "";id=angle-1;}
            if(!identifier(id))return "";if(!owner.length)owner=t[id].text;prefixEnd=id;
        }
        auto bare=t[name].text;bool constructor=bare==cls || owner.length && bare==owner;bool hasType;
        for(auto i=start;i<prefixEnd;++i) {
            if(typeWords.canFind(t[i].text))return "";
            if(identifier(i) && !qualifiers.canFind(t[i].text))hasType=true;
            if(at(i,"(") || at(i,"[")){if(pair[i]<i || pair[i]>=prefixEnd)return "";i=pair[i];}
        }
        return hasType || constructor?result:"";
    }
    int functionEnd(int close,out int body) {
        body=-1;bool inits,assigned;
        for(auto i=close+1;i<t.length;++i) {
            if(at(i,"(") || at(i,"[")){if(pair[i]<i)return -1;i=pair[i];}
            else if(at(i,":")){if(!at(i-1,":") && !at(i+1,":"))inits=true;}
            else if(at(i,"="))assigned=true;
            else if(at(i,"{")){if(pair[i]<i)return -1;if(inits && (identifier(i-1) || at(i-1,">") || at(i-1,"]"))){i=pair[i];continue;}body=i;return pair[i];}
            else if(at(i,";"))return i;
            else if(at(i,"}"))return -1;
            else if(assigned && at(i,","))return -1;
        }
        return -1;
    }
    bool parameters(int open,int close) {
        bool start=true,defaultValue;
        for(auto i=open+1;i<close;++i) {
            if(at(i,"(") || at(i,"[") || at(i,"{")){if(pair[i]<i || pair[i]>close)return false;i=pair[i];start=false;continue;}
            if(at(i,"<")){auto next=angleEnd(i);if(next>i && next<=close){i=next-1;start=false;continue;}}
            if(at(i,",")){start=true;defaultValue=false;continue;}
            if(at(i,"=")){defaultValue=true;continue;}
            if(defaultValue)continue;
            if(start && (!identifier(i) && !at(i,":") && !at(i,".") && !at(i,"[") || ["true","false","nullptr","this","new"].canFind(t[i].text)))return false;
            start=false;
        }
        return true;
    }
}

Result findCppTokens(ref Config config,string[] lines,LineFact[] facts) {
    version(AuditCppTokens) assert(!config.patterns.length,"C++ token scan must not compile regex patterns");
    auto tokens=tokenizeCpp(facts,lines);CppScan scan=CppScan(tokens.items,tokens.pairs,lines);auto t=scan.t;auto p=scan.pair;
    Result result;struct Scope {string name;int end;}Scope[] scopes;int boundary;
    for(int i=0;i<t.length;++i) {
        while(scopes.length && scopes[$-1].end<i)scopes.length--;
        if(scan.at(i,"{")) {
            int nameAt;auto name=scan.typeName(boundary,i,nameAt);
            if(name.length && p[i]>i){scopes~=Scope(name,p[i]);result.classes~=Bounds(name,t[boundary].line+1,t[p[i]].line+1);}
            boundary=i+1;
        } else if(scan.at(i,"}") || scan.at(i,";"))boundary=i+1;
        else if(scan.at(i,":") && i==boundary+1 && ["public","private","protected"].canFind(t[boundary].text))boundary=i+1;
        else if(scan.at(i,"(")) {
            if(p[i]<i)continue;auto cls=scopes.length?scopes[$-1].name:"";int nameAt;auto name=scan.functionName(boundary,i,cls,nameAt);if(!name.length){i=p[i];continue;}
            int body;auto end=scan.functionEnd(p[i],body);if(end<i || !scan.parameters(i,p[i]))continue;
            auto start=scan.headerStart(boundary,nameAt);
            result.functions~=Bounds(name,t[start].line+1,t[end].line+1,cls,t[nameAt].line+1,name);
            i=end;boundary=i+1;
        }
    }
    return result;
}

unittest {
    import brace_port_cleaner : Cleaner;
    import std.array : split;
    Result parse(string source) {Cleaner cleaner;Config config;auto lines=source.split("\n");return findCppTokens(config,lines,cleaner.facts(lines));}
    auto r=parse("struct C {\n C() : x{1}, y(2) {}\n ~C() = default;\n operator unsigned() const {return 1;}\n int operator()(int v) const {return v;}\n int operator[](int v) const {return v;}\n int x,y;\n};");
    string[] names;foreach(f;r.functions)names~=f.name;
    assert(names==["C","~C","operator unsigned int","operator()","operator[]"]);
    foreach(i,f;r.functions)assert(f.start==i+2 && f.end==i+2);
    r=parse("template<class T>\nstruct C {int f();};\ntemplate<class T>\nint C<T>::f() {return 1;}");
    assert(r.functions.length==2 && r.functions[0].name=="f" && r.functions[1].name=="f");
    assert(r.functions[1].start==3 && r.functions[1].signatureLine==4 && r.functions[1].end==4);
    r=parse("auto value=make(1);\nextern template int f<int>(int);\nnamespace N {\n [[deprecated(\"ж\")]] int f() {return 0;}\n}");
    assert(r.functions.length==1 && r.functions[0].name=="f" && r.functions[0].start==4 && r.functions[0].end==4);
    r=parse("[[deprecated(\"ж\")]] unsigned long long operator\"\"_x(const char*,unsigned long n) {return n;}");
    assert(r.functions.length==1 && r.functions[0].name=="operator\"\"_x");
    r=parse("#if FLAG\nATTR\n#endif\nint f(){return 1;}");
    assert(r.functions.length==1 && r.functions[0].start==4 && r.functions[0].end==4);
    r=parse("#define DEFINE() \\\n int fake() {}\nint real(){return 1;}");
    assert(r.functions.length==1 && r.functions[0].name=="real");
    assert(!parse("int unfinished(\n").functions.length);
    r=parse("class C final {\npublic:\n C(){}\n};\nint value(42);\nint other(true);");
    assert(r.classes.length==1 && r.classes[0].name=="C");
    assert(r.functions.length==1 && r.functions[0].name=="C");
    r=parse("int f(std::array<int,3> a, int b=make(1,2), S c={1,2}) {return b;}");
    assert(r.functions.length==1 && r.functions[0].name=="f");
}

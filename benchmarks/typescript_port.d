module typescript_port;
import js_ts_cleaner;
import js_ts_model;
import std.algorithm : canFind,swap;
import std.array : appender,Appender;
import std.string : strip, startsWith, endsWith, lastIndexOf;
import std.regex;
import typescript_candidates : recoverTypeScript;

enum Event { open,body,end,unterminated }
char previous(string s,size_t i) { while(i) { --i; if(s[i]!=' ' && s[i]!='\t') return s[i]; } return 0; }
struct Signature {
    int brackets,typeBraces; size_t bodyAt;
    Event feed(string s) {
        foreach(i,c;s) switch(c) {
            case '(': case '[': ++brackets; break;
            case ')': case ']': if(brackets>0) --brackets; break;
            case '{': if(!brackets) {
                if(typeBraces>0 || previous(s,i)=='<') { ++typeBraces; continue; }
                bodyAt=i; return Event.body;
            } break;
            case '}': if(!brackets && typeBraces>0) --typeBraces; break;
            case ';': if(!brackets && !typeBraces) return Event.end; break;
            default: break;
        }
        return Event.open;
    }
}
struct Context { Bounds fn; int depth; Signature sig; bool pending; int sigLast; bool afterBlock,bodyStarted; int bodyDelta; }
int closeIndex(string s,int depth) { int n; foreach(i,c;s) { if(c=='{') ++n; else if(c=='}') --n; if(n<=-depth) return cast(int)i; } return -1; }
size_t blanks(string s,size_t i) { while(i<s.length && (s[i]==' ' || s[i]=='\t')) ++i; return i; }
int declParen(string s) {
    int eq=-1,angles;
    foreach(i,c;s) {
        if(c=='<') { ++angles; continue; }
        if(c=='>' && angles>0 && !(i && s[i-1]=='=')) { --angles; continue; }
        if(c!='=' || angles>0) continue;
        auto prev=i ? s[i-1] : '\0'; auto next=i+1<s.length ? s[i+1] : '\0';
        if(next=='>' || next=='=' || prev=='=' || prev=='!' || prev=='<' || prev=='>') continue;
        eq=cast(int)i; break;
    }
    if(eq<0) return -1;
    auto j=blanks(s,eq+1);
    if(s[j..$].startsWith("async")) j=blanks(s,j+5);
    if(j<s.length && s[j]=='<') {
        int depth;
        for(;j<s.length;++j) {
            if(s[j]=='<') ++depth;
            else if(s[j]=='>' && !(j && s[j-1]=='=')) { --depth; if(!depth) { ++j; break; } }
        }
        j=blanks(s,j);
    }
    return j<s.length && s[j]=='(' ? cast(int)j : -1;
}
bool arrowOrType(string[] lines,size_t first) {
    foreach(s;lines[first..$]) if(s.strip.length) return s.strip.startsWith("=>") || s.strip.startsWith(":");
    return false;
}
bool completeValue(string[] matches,string s) {
    if(matches.length<=5 || !(matches[4]=="const" || matches[4]=="let" || matches[4]=="var") || s.canFind("function")) return false;
    auto open=declParen(s);
    if(open>=0) {
        auto first=blanks(s,open+1);
        if(first<s.length && s[first]=='(') return true;
        int depth;
        foreach(i;cast(size_t)open..s.length) {
            if(s[i]=='(') ++depth;
            else if(s[i]==')') { --depth; if(!depth) { auto tail=s[i+1..$].strip; return !tail.startsWith("=>") && !tail.startsWith(":"); } }
        }
    }
    if(s.canFind("await")) return true;
    return !s.canFind("=>") && s.strip.endsWith(";");
}
bool valueSpans(string[] lines,size_t first) {
    auto open=declParen(lines[first]); if(open<0) return false;
    int depth;
    foreach(j;first..lines.length) {
        auto s=lines[j]; auto from=j==first ? cast(size_t)open : 0;
        foreach(k;from..s.length) {
            if(s[k]=='(') ++depth;
            else if(s[k]==')') { --depth; if(!depth) {
                auto tail=s[k+1..$].strip;
                return !tail.length ? !arrowOrType(lines,j+1) : !(tail.startsWith("=>") || tail.startsWith(":"));
            } }
        }
    }
    return false;
}
int openAngles(string s) { int n; foreach(i,c;s) { if(c=='<') ++n; else if(c=='>' && !(i && s[i-1]=='=') && n>0) --n; } return n; }
bool typedFunction(string[] lines,size_t first) {
    int depth=openAngles(lines[first]);
    for(auto j=first+1;j<lines.length && j<first+200;++j) foreach(k,c;lines[j]) {
        if(c=='<') ++depth;
        else if(c=='>') {
            if(k && lines[j][k-1]=='=') continue;
            --depth;
            if(!depth) {
                auto t=lines[j][k+1..$].strip;
                for(auto next=j+1;!t.length && next<lines.length;++next) t=lines[next].strip;
                if(t.startsWith("{") || t.startsWith(";")) return true;
                if(t.startsWith("=") && !t.startsWith("=>")) { auto rhs=t[1..$].strip; return rhs.startsWith("(") || rhs.startsWith("async") || rhs.startsWith("<"); }
                return false;
            }
        }
    }
    return false;
}
Regex!char declarationStart,anonymousObject,continuationStart,arrowTail,listDeclaration;
bool configured;
void setup() {
    if(configured) return;
    declarationStart=regex(`^(?:@|(?:(?:public|private|protected|internal|override|open|abstract|final|inline|suspend|const|lateinit|external|operator|infix|data|sealed|enum|companion|inner|static|mutating|lazy)\s+)*(?:val|var|fun|func|class|object|interface|typealias|init|constructor|struct|protocol|extension)\b)`);
    anonymousObject=regex(`^object\s*:`); continuationStart=regex(`^(?:\.|\?[.:]|else\b)`);
    arrowTail=regex(`=>\s*\(?\s*$`); listDeclaration=regex(`^(?:export\s+)?(?:const|let|var)\s`); configured=true;
}
bool hit(string s,Regex!char p) { return !matchFirst(s,p).empty; }
bool startsDeclaration(string s) { return hit(s,declarationStart) && !hit(s,anonymousObject); }
bool continues(string[] lines,size_t i) { foreach(t;lines[i+1..$]) if(t.strip.length) return hit(t.strip,continuationStart); return false; }
int statementStart(string s,size_t comma) {
    int depth;
    for(auto j=cast(ptrdiff_t)comma-1;j>=0;--j) switch(s[j]) {
        case ')': case ']': case '}': ++depth; break;
        case '(': case '[': if(!depth) return -1; --depth; break;
        case '{': if(!depth) return cast(int)j+1; --depth; break;
        case ';': if(!depth) return cast(int)j+1; break;
        default: break;
    }
    return 0;
}
struct Inline { int at; bool declarator; }
Inline[] inlineStarts(ref Config config,string s) {
    Inline[] result;
    foreach(i,c;s) if(";,{}".canFind(c)) {
        auto p=blanks(s,i+1); if(p>=s.length) continue;
        if(config.captures("func_pattern",s[p..$]).length) result~=Inline(cast(int)p,false);
        else if(c==',' && config.captures("declarator_pattern",s[p..$]).length) {
            auto start=statementStart(s,i);
            if(start>=0 && hit(s[start..i].strip,listDeclaration)) result~=Inline(cast(int)p,true);
        }
    }
    return result;
}
bool keyword(ref Config config,string name) {
    if(auto p="statement_words" in config.data.object) foreach(w;p.array) if(w.str==name) return true;
    if(auto p="keywords_not_names" in config.data.object) if(p.type==JSONType.true_) return config.excluded(name);
    return false;
}
import std.json : JSONType;
void inlineFunction(ref Config config,string s,Inline start,int number,Bounds[] classes,ref Bounds[] done,ref Appender!(Context[]) stack) {
    auto text=s[start.at..$];
    auto m=config.captures(start.declarator ? "declarator_pattern" : "func_pattern",text);
    if(!m.length) return;
    if(start.declarator) m=[m[0],"","","","const",m[1]];
    if(completeValue(m,text)) return;
    auto name=capturedName(m); if(keyword(config,name)) return;
    Context ctx; ctx.fn=Bounds(name,number,0,classForLine(classes,number-1));
    int brackets,at=-1; Event ev;
    foreach(i,c;text) {
        if(c=='(' || c=='[') ++brackets;
        else if(c==')' || c==']') --brackets;
        else if(!brackets && (c=='{' || c==';')) { at=cast(int)i; ev=c=='{' ? Event.body : Event.end; break; }
    }
    if(ev==Event.end) { ctx.fn.end=number; done~=ctx.fn; }
    else if(ev==Event.body) {
        int depth;
        foreach(c;text[at..$]) { if(c=='{') ++depth; else if(c=='}') --depth; if(!depth) { ctx.fn.end=number; done~=ctx.fn; return; } }
        ctx.depth=depth; stack~=ctx;
    } else { ctx.sig.feed(text); ctx.pending=true; ctx.sigLast=number; stack~=ctx; }
}
Result findTypeScript(ref Config config,string[] raw,LineFact[] facts) {
    setup(); Result result; result.classes=findClasses(config,facts);
    string[] lines; foreach(f;facts) lines~=f.cleaned;
    auto stack=appender!(Context[])(); auto next=appender!(Context[])();
    Event[] eventScratch; int[] depthScratch; int typeDepth;
    foreach(lineNum,s;lines) {
        int number=cast(int)lineNum+1,delta;
        foreach(i,c;s) {
            if(c=='{') { if(typeDepth>0 || previous(s,i)=='<') ++typeDepth; else ++delta; }
            else if(c=='}') { if(typeDepth>0) --typeDepth; else --delta; }
        }
        auto may=config.containsAny("func_requires",s);
        auto matches=may ? config.captures("func_pattern",s) : null;
        auto trimmed=raw[lineNum]; while(trimmed.length && " \t\n\f\r".canFind(trimmed[0])) trimmed=trimmed[1..$];
        if(!matches.length && facts[lineNum].startState==0 && trimmed.length && (trimmed[0]=='"' || trimmed[0]=='\'')) matches=config.captures("quoted_method_pattern",raw[lineNum]);
        if(matches.length && completeValue(matches,s)) matches=null;
        if(matches.length>5 && (matches[4]=="const" || matches[4]=="let" || matches[4]=="var") && valueSpans(lines,lineNum)) matches=null;
        auto className=classForLine(result.classes,cast(int)lineNum);
        if(!matches.length && !stack.length && className.length) matches=config.captures("class_method_open",s);
        if(!matches.length && !stack.length && className.length && parens(s)>0) matches=config.captures("header_open_pattern",s);
        bool typedOpen;
        if(!matches.length && !stack.length && openAngles(s)>0) {
            auto m=config.captures("open_type_header_pattern",s);
            if(m.length && typedFunction(lines,lineNum)) { matches=m; typedOpen=true; }
        }
        auto tail=s; while(tail.length && " \t\n\f\r".canFind(tail[$-1])) tail=tail[0..$-1];
        if(!matches.length && !stack.length && may && tail.endsWith(";")) matches=config.captures("declaration_pattern",s);
        auto events=span(eventScratch,stack.length); events[]=Event.open;
        foreach(i,ref ctx;stack.data) if(ctx.pending) {
            if((matches.length || startsDeclaration(s.strip)) && !ctx.sig.brackets) { events[i]=Event.unterminated; ctx.pending=false; continue; }
            auto atTop=!ctx.sig.brackets; events[i]=ctx.sig.feed(s);
            if(events[i]==Event.body) { ctx.bodyStarted=true; ctx.bodyDelta=braces(s[ctx.sig.bodyAt..$]); }
            if(events[i]!=Event.open) ctx.pending=false;
            else if((s.strip.length || (facts[lineNum].startState!=2 && facts[lineNum].codeText)) && !(atTop && s.strip.startsWith("}")) && !(atTop && hit(s.strip,declarationStart))) ctx.sigLast=number;
        }
        bool headerOpen; foreach(ctx;stack.data) if(ctx.pending) headerOpen=true;
        Inline[] starts; if(!headerOpen && may) starts=inlineStarts(config,s);
        auto previousDepths=span(depthScratch,stack.length);
        foreach(i,ref ctx;stack.data) {
            previousDepths[i]=ctx.depth;
            if(!ctx.pending) { ctx.depth+=ctx.bodyStarted ? ctx.bodyDelta : delta; ctx.bodyStarted=false; }
        }
        if(matches.length) {
            auto name=capturedName(matches);
            if(!keyword(config,name)) {
                Context ctx; ctx.fn=Bounds(name,number,0,className); Signature probe;
                auto body=probe.feed(s)==Event.body; auto eq=s.strip.endsWith("=") || s.strip.endsWith("=>");
                if(typedOpen) { ctx.sig.feed(s); ctx.pending=true; ctx.sigLast=number; stack~=ctx; }
                else if(delta<0 && body && !parens(s[0..probe.bodyAt]) && !eq && closeIndex(s[probe.bodyAt..$],0)>=0) { ctx.fn.end=number; result.functions~=ctx.fn; }
                else if(!delta && body && !parens(s) && !eq) { ctx.fn.end=number; result.functions~=ctx.fn; }
                else if(!delta) {
                    if(ctx.sig.feed(s)==Event.end) { ctx.fn.end=number; result.functions~=ctx.fn; }
                    else { ctx.pending=true; ctx.sigLast=number; stack~=ctx; }
                } else { ctx.depth=delta; stack~=ctx; }
            }
        }
        foreach(start;starts) inlineFunction(config,s,start,number,result.classes,result.functions,stack);
        next.clear();
        foreach(i,ctx;stack.data) {
            auto ev=i<events.length ? events[i] : Event.open;
            if(ev==Event.unterminated) { ctx.fn.end=ctx.sigLast; result.functions~=ctx.fn; continue; }
            if(ev==Event.end) { ctx.fn.end=number; result.functions~=ctx.fn; continue; }
            auto prev=i<previousDepths.length ? previousDepths[i] : 0;
            auto close=prev>0 ? closeIndex(s,prev) : -1;
            bool afterHeader; foreach(start;starts) if(start.at>close) afterHeader=true;
            if(close>=0 && ctx.depth!=0 && (ctx.depth<0 || afterHeader)) { ctx.fn.end=number; result.functions~=ctx.fn; }
            else if(!ctx.depth && (prev>0 || ev==Event.body || (ctx.afterBlock && s.strip.length))) {
                if(hit(s.strip,arrowTail)) { Signature sig; sig.feed(s.strip[s.strip.lastIndexOf("=>")+2..$]); ctx.sig=sig; ctx.pending=true; ctx.sigLast=number; next~=ctx; }
                // Go's continuation lookup reads original lines in this path.
                else if(continues(raw,lineNum)) { ctx.afterBlock=true; next~=ctx; }
                else { ctx.fn.end=number; result.functions~=ctx.fn; }
            } else next~=ctx;
        }
        swap(stack,next);
    }
    foreach(ctx;stack.data) {
        if(ctx.pending) { ctx.fn.end=ctx.sigLast; result.functions~=ctx.fn; }
        else if(ctx.afterBlock) { for(auto i=raw.length;i>0;--i) if(raw[i-1].strip.length) { ctx.fn.end=cast(int)i; break; } result.functions~=ctx.fn; }
    }
    recoverTypeScript(raw,facts,result);
    return result;
}

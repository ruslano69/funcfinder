module csharp_port;
import brace_port_cleaner;
import brace_port_model;
import std.string : strip,startsWith;
import std.algorithm : canFind;
import std.uni : isWhite;
import brace_port_unicode : goLetter,goDigit;
import reuse : TokenPool;

struct Token {string text;int line;}
private enum modifiers=["public","private","protected","internal","static","abstract","virtual","override","sealed","async","unsafe","extern","partial","readonly","new","ref","scoped"];
private enum statements=["return","throw","await","yield","new","if","else","for","foreach","while","do","switch","case","catch","finally","try","lock","using","fixed","checked","unchecked","nameof","typeof","sizeof","class","struct","interface","record","enum","namespace","delegate","base","this","default"];
bool digit(dchar c) {return goDigit(c);}
bool letter(dchar c) {return goLetter(c);}
bool identifier(string text) {if(text.startsWith("@"))text=text[1..$];if(!text.length)return false;size_t i;bool first=true;while(i<text.length){auto c=nextRune(text,i);if(c!='_' && !letter(c) && !(!first && digit(c)))return false;first=false;}return true;}
// The tokens, pairs and bracket stacks of the previous file on this thread (reuse.d): tokens and pairs are valid until the
// next call.
private TokenPool!Token pool;
void tokenize(LineFact[] facts,out Token[] tokens,out int[] pairs,bool java=false) {
    pool.reset();
    auto stacks=pool.stack[];
    foreach(line,f;facts){if(f.cleaned.strip.startsWith("#"))continue;size_t i;
        while(i<f.cleaned.length){auto start=i;auto c=nextRune(f.cleaned,i);if(isWhite(c))continue;
            if(letter(c) || c=='_' || c=='@' || java && c=='$')while(i<f.cleaned.length){auto j=i;auto next=nextRune(f.cleaned,j);if(!letter(next) && !digit(next) && next!='_' && !(java && next=='$'))break;i=j;}
            auto text=f.cleaned[start..i];auto index=cast(int)pool.items.length;pool.items~=Token(text,cast(int)line);pool.pairs~=-1;
            auto open=text=="("?0:text=="["?1:text=="{"?2:-1;
            if(open>=0)stacks[open]~=index;
            else {auto close=text==")"?0:text=="]"?1:text=="}"?2:-1;if(close>=0 && stacks[close].length){auto at=stacks[close][$-1];stacks[close].popBack();pool.pairs[at]=index;pool.pairs[index]=at;}}
        }
    }
    tokens=pool.items.data;pairs=pool.pairs.data;
    version(AuditBraceTokens)foreach(i,t;tokens){assert(t.line>=0 && t.line<facts.length);if(i)assert(tokens[i-1].line<=t.line);auto other=pairs[i];if(other>=0)assert(other<tokens.length && pairs[other]==i);}
}
int skipAttributes(Token[] t,int[] p,int start,int end) {while(start<end && t[start].text=="["){if(p[start]<start || p[start]>=end)return end;start=p[start]+1;}return start;}
string className(Token[] t,int[] p,int start,int end) {start=skipAttributes(t,p,start,end);while(start<end && modifiers.canFind(t[start].text))++start;if(start>=end)return "";
    if(["class","struct","interface","record","enum"].canFind(t[start].text)){++start;if(start<end && (t[start].text=="class" || t[start].text=="struct"))++start;if(start<end && identifier(t[start].text))return unescape(t[start].text);}return "";
}
string unescape(string s) {return s.startsWith("@")?s[1..$]:s;}
int skipType(Token[] t,int[] p,int start,int end) {
    auto i=start;if(i>=end)return start;
    if(t[i].text=="("){auto close=p[i];if(close<i || close>=end)return start;bool comma;foreach(j;i+1..close)if(t[j].text==",")comma=true;if(!comma)return start;i=close+1;}
    else if(identifier(t[i].text))++i;else return start;
    while(i<end)switch(t[i].text){
        case "<":int depth=1;++i;while(i<end && depth>0){if(t[i].text=="<")++depth;else if(t[i].text==">")--depth;++i;}if(depth)return start;break;
        case "?":case "*":++i;break;
        case "[":if(p[i]<i || p[i]>=end)return start;i=p[i]+1;break;
        case ".":if(i+1>=end || !identifier(t[i+1].text))return i;i+=2;break;
        case ":":if(i+2>=end || t[i+1].text!=":" || !identifier(t[i+2].text))return i;i+=3;break;
        default:return i;
    }
    return i;
}
bool propertyBody(Token[] t,int[] p,int start,int end) {
    start=skipAttributes(t,p,start,end);while(start<end && (modifiers.canFind(t[start].text) || t[start].text=="event"))++start;
    auto typeEnd=skipType(t,p,start,end);if(typeEnd<=start || typeEnd>=end)return false;
    if(typeEnd+1==end && identifier(t[typeEnd].text))return true;
    auto nameAt=end-1;if(t[nameAt].text=="]"){nameAt=p[nameAt]-1;if(nameAt<typeEnd || t[nameAt].text!="this")return false;}
    if(!identifier(t[nameAt].text))return false;
    return nameAt==typeEnd || skipType(t,p,typeEnd,nameAt)==nameAt-1 && t[nameAt-1].text==".";
}
string functionName(Token[] t,int[] p,int boundary,int open,string cls,out int start) {
    start=skipAttributes(t,p,boundary,open);if(start>=open || statements.canFind(t[start].text) && t[start].text!="new")return "";
    int operatorAt=-1;foreach(i;start..open)if(t[i].text=="operator")operatorAt=i;
    foreach(i;start..open)switch(t[i].text){
        case "=":if(operatorAt<0 || i<operatorAt)return "";break;
        case ";":case "{":case "}":case "class":case "struct":case "interface":case "record":case "enum":case "namespace":case "delegate":return "";
        case "(":case "[":if(p[i]<i || p[i]>=open)return "";break;
        default:break;
    }
    foreach(i;start..open)if(t[i].text=="operator"){string suffix;foreach(tok;t[i+1..open])suffix~=tok.text;if(i>start && (t[i-1].text=="implicit" || t[i-1].text=="explicit"))return t[i-1].text~" operator "~suffix;return "operator "~suffix;}
    auto nameAt=open-1;if(t[nameAt].text==">"){int depth=1;--nameAt;while(nameAt>=start && depth>0){if(t[nameAt].text==">")++depth;else if(t[nameAt].text=="<")--depth;--nameAt;}}
    if(nameAt<start || !identifier(t[nameAt].text) || statements.canFind(t[nameAt].text) || modifiers.canFind(t[nameAt].text))return "";
    auto name=unescape(t[nameAt].text);auto prefixEnd=nameAt;if(prefixEnd>start && t[prefixEnd-1].text=="~"){name="~"~name;--prefixEnd;}
    auto returnType=start;while(returnType<prefixEnd && modifiers.canFind(t[returnType].text))++returnType;
    if(returnType==prefixEnd){if(t[start].text!="new" && (name==cls || name=="~"~cls))return name;return "";}
    auto typeEnd=skipType(t,p,returnType,prefixEnd);if(typeEnd==returnType)return "";
    if(typeEnd!=prefixEnd){if(!identifier(t[typeEnd].text))return "";auto end=skipType(t,p,typeEnd,prefixEnd);if(end!=prefixEnd-1 || t[end].text!=".")return "";}
    return name;
}
int functionEnd(Token[] t,int[] p,int close,out int body) {
    body=-1;for(auto i=close+1;i<t.length;++i)switch(t[i].text){
        case "(":case "[":if(p[i]<i)return -1;i=p[i];break;
        case "{":body=i;return p[i];
        case ";":return i;
        case "=":if(i+1<t.length && t[i+1].text==">"){for(auto j=i+2;j<t.length;++j){if(t[j].text==";")return j;if(["(","[","{"].canFind(t[j].text)){if(p[j]<j)return -1;j=p[j];}}return -1;}break;
        case "}":return -1;
        default:break;
    }
    return -1;
}
struct Conditional {int start,end;}
Conditional[] conditionals(LineFact[] fs) {int[] stack;Conditional[] result;foreach(line,f;fs){auto text=f.cleaned.strip;if(text.startsWith("#if ") || text.startsWith("#if\t"))stack~=cast(int)line;else if(text.startsWith("#endif") && stack.length){result~=Conditional(stack[$-1],cast(int)line);stack.length--;}}return result;}
int expressionAlternatives(Token[] t,int[] p,int close,int end,Conditional[] ranges) {foreach(span;ranges){if(span.start<=t[close].line || span.start>t[end].line || span.end<=t[end].line)continue;for(auto i=end+1;i<t.length && t[i].line<span.end;++i){if(t[i].text==";")end=i;else if(["(","[","{"].canFind(t[i].text) && p[i]>i)i=p[i];}}return end;}
Result findCSharp(LineFact[] facts) {
    Token[] t;int[] p;tokenize(facts,t,p);auto ranges=conditionals(facts);Result result;
    struct ClassScope {string name;int end;}
    ClassScope[] classes;auto boundaries=new int[t.length];auto blocks=new bool[t.length];int boundary;
    for(int i=0;i<t.length;++i){while(classes.length && classes[$-1].end<i)classes.length--;
        switch(t[i].text){
            case "{":boundaries[i]=boundary;if(propertyBody(t,p,boundary,i))blocks[i]=true;
                auto name=className(t,p,boundary,i);if(name.length && p[i]>=i){classes~=ClassScope(name,p[i]);result.classes~=Bounds(name,t[boundary].line+1,t[p[i]].line+1);blocks[i]=true;}
                if(boundary==i)blocks[i]=true;else if(boundary<i && ["if","else","for","foreach","while","do","switch","try","catch","finally","lock","using","fixed","get","set","init","add","remove"].canFind(t[boundary].text))blocks[i]=true;
                boundary=i+1;break;
            case "}":auto open=p[i];if(open>=0 && !blocks[open])boundary=boundaries[open];else boundary=i+1;break;
            case ";":boundary=i+1;break;
            case "(":if(p[i]<i)break;auto cls=classes.length?classes[$-1].name:"";int start;auto name=functionName(t,p,boundary,i,cls,start);if(!name.length)break;int body;auto end=functionEnd(t,p,p[i],body);if(end<i)break;if(body<0)end=expressionAlternatives(t,p,p[i],end,ranges);
                result.functions~=Bounds(name,t[start].line+1,t[end].line+1,cls,t[i-1].line+1,unescape(t[i-1].text));
                if(body>=0){blocks[body]=true;i=body-1;}else{i=p[i];boundary=i+1;}break;
            default:break;
        }
    }
    return result;
}

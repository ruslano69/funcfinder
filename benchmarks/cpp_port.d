module cpp_port;
import brace_port_cleaner;
import brace_port_model;
import std.string : strip,startsWith,endsWith;
import std.algorithm : canFind;

struct Signature {
    int brackets,typeBraces,initBraces,bodyAt;bool initList;
    int feed(string s) {
        foreach(i,c;s)switch(c){
            case '(':case '[':++brackets;break;
            case ')':case ']':if(brackets>0)--brackets;break;
            case ':':if(!brackets && !initList && !s[i..$].startsWith("::") && (!i || s[i-1]!=':'))initList=true;break;
            case '{':if(!brackets){auto prev=previous(s,i);if(typeBraces>0 || prev=='<'){++typeBraces;break;}if(initBraces>0 || initList && (prev=='>' || prev=='_' || prev>='a' && prev<='z' || prev>='A' && prev<='Z' || prev>='0' && prev<='9')){++initBraces;break;}bodyAt=cast(int)i;return 1;}break;
            case '}':if(!brackets){if(typeBraces>0)--typeBraces;else if(initBraces>0)--initBraces;}break;
            case ';':if(!brackets && !typeBraces)return 2;break;
            default:break;
        }
        return 0;
    }
}
char previous(string s,size_t i) {while(i){--i;if(s[i]!=' ' && s[i]!='\t')return s[i];}return 0;}
bool reserved(ref Config c,string s) {if(parens(s)==0){auto m=c.captures("template_prefix",s);if(m.length)s=s[m[0].length..$];}auto m=c.captures("leading_word",s);return m.length>1 && c.listed("statement_words",m[1]);}
Result findCpp(ref Config config,string[] lines,LineFact[] facts) {
    Result result;result.classes=findClasses(config,facts);Bounds current;Signature sig;bool active,pending,afterBlock;int sigLast,depth;
    foreach(lineNum,line;lines){auto cleaned=facts[lineNum].cleaned;
        if(active && pending && !sig.brackets && (config.captures("func_pattern",cleaned).length || config.declaration(cleaned.strip))){current.end=sigLast;result.functions~=current;active=false;pending=false;}
        if(active){auto counted=cleaned;
            if(pending){bool atTop=!sig.brackets;auto ev=sig.feed(cleaned);auto text=cleaned.strip;if(ev==0 && text.length && !(atTop && text.startsWith("}")) && !(atTop && config.captures("decl_start",text).length))sigLast=cast(int)lineNum+1;
                if(ev==2){current.end=cast(int)lineNum+1;result.functions~=current;active=false;pending=false;continue;}
                if(ev==1){counted=cleaned[sig.bodyAt..$];pending=false;}
            }
            if(pending)continue;auto prevDepth=depth;bool hasBrace=counted.canFind('{');depth+=braces(counted);
            if(depth==0 && (prevDepth>0 || hasBrace || afterBlock && cleaned.strip.length)){if(config.continues(lines,lineNum))afterBlock=true;else{current.end=cast(int)lineNum+1;result.functions~=current;active=false;afterBlock=false;}}
        }else{
            auto matches=config.captures("func_pattern",cleaned);
            if(!matches.length && !reserved(config,cleaned)){
                auto cls=classForLine(result.classes,cast(int)lineNum);
                if(parens(cleaned)>0)matches=config.captures("header_open_pattern",cleaned);
                if(!matches.length && cls.length){auto m=config.captures("constructor_pattern",cleaned);auto name=capturedName(m);if(m.length && (name==cls || name=="~"~cls))matches=m;}
                if(!matches.length)matches=config.captures(cls.length?"member_declaration_pattern":"declaration_pattern",cleaned);
            }
            if(matches.length){auto m=config.captures("conversion_operator_pattern",cleaned);if(m.length>1 && !["new","delete","co_await","co_yield","co_return"].canFind(m[1].strip))matches=null;}
            if(!matches.length)continue;auto name=capturedName(matches);if(config.listed("exclude_words",name) || config.listed("statement_words",name))continue;
            current=Bounds(name,cast(int)lineNum+1,0,classForLine(result.classes,cast(int)lineNum));active=true;bool hasBrace=cleaned.canFind('{');depth=braces(cleaned);
            if(!depth){sig=Signature();pending=true;sigLast=cast(int)lineNum+1;auto ev=sig.feed(cleaned);bool closes=ev==2 || ev==1 && braces(cleaned[sig.bodyAt..$])==0 && !cleaned.strip.endsWith("=");
                if(closes){current.end=cast(int)lineNum+1;result.functions~=current;active=false;pending=false;}
            }
        }
    }
    if(active && pending){current.end=sigLast;result.functions~=current;}
    else if(active && afterBlock){for(auto j=lines.length;j>0;--j)if(lines[j-1].strip.length){current.end=cast(int)j;break;}result.functions~=current;}
    return result;
}

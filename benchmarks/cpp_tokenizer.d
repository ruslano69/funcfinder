module cpp_tokenizer;
import brace_port_cleaner : LineFact,nextRune;
import brace_port_unicode : goLetter,goDigit;
import std.string : strip,startsWith;
import std.uni : isWhite;
import reuse : TokenPool;

struct CppToken {string text;int line;size_t column,endColumn;bool word;int segment;}
struct CppTokens {CppToken[] items;int[] pairs;}
// The tokens of the previous file on this thread and the column map of a line (reuse.d): valid until the next call.
private TokenPool!CppToken pool;
private size_t[] offsetPool;
CppTokens tokenizeCpp(LineFact[] facts,string[] raw=null) {
    alias output=pool;pool.reset();
    auto stacks=pool.stack[];
    bool directive;int segment;
    foreach(line,fact;facts) {
        auto text=fact.cleaned;
        if(directive || text.strip.startsWith("#")) {
            ++segment;
            auto trimmed=text.strip;
            directive=trimmed.length && trimmed[$-1]=='\\';
            continue;
        }
        size_t[] offsets;
        if(raw.length && raw[line].length!=text.length) {
            if(offsetPool.length<text.length+1)offsetPool.length=text.length+1+text.length/2;
            offsets=offsetPool[0..text.length+1];offsets[]=0;size_t cleanAt,rawAt;
            while(cleanAt<text.length && rawAt<raw[line].length){offsets[cleanAt]=rawAt;nextRune(text,cleanAt);nextRune(raw[line],rawAt);}offsets[text.length]=rawAt;
        }
        size_t i;
        while(i<text.length) {
            auto start=i;
            auto ch=nextRune(text,i);
            if(isWhite(ch)) continue;
            bool word=goLetter(ch) || ch=='_' || ch=='$';
            if(word || goDigit(ch)) {
                while(i<text.length) {
                    auto end=i;
                    auto next=nextRune(text,end);
                    if(!goLetter(next) && !goDigit(next) && next!='_' && next!='$') break;
                    i=end;
                }
            }
            auto token=CppToken(text[start..i],cast(int)line,offsets.length?offsets[start]:start,offsets.length?offsets[i]:i,word,segment);
            auto index=cast(int)output.items.length;
            output.items~=token;
            output.pairs~=-1;
            auto open=token.text=="("?0:token.text=="["?1:token.text=="{"?2:-1;
            if(open>=0) stacks[open]~=index;
            else {
                auto close=token.text==")"?0:token.text=="]"?1:token.text=="}"?2:-1;
                if(close>=0 && stacks[close].length) {
                    auto at=stacks[close][$-1];stacks[close].popBack();
                    output.pairs[at]=index;output.pairs[index]=at;
                }
            }
        }
    }
    version(AuditCppTokens) foreach(i,token;output.items) {
        assert(token.text.length && token.line>=0 && token.line<facts.length);
        if(i)assert(output.items[i-1].line<=token.line && output.items[i-1].segment<=token.segment);
        auto other=output.pairs[i];
        if(other>=0) {
            assert(other<output.items.length && output.pairs[other]==i);
            auto text=output.items[other].text;
            assert(token.text=="(" && text==")" || token.text==")" && text=="(" || token.text=="[" && text=="]" || token.text=="]" && text=="[" || token.text=="{" && text=="}" || token.text=="}" && text=="{");
        }
        if(raw.length)assert(token.column<token.endColumn && token.endColumn<=raw[token.line].length && raw[token.line][token.column..token.endColumn]==token.text);
    }
    return CppTokens(pool.items.data,pool.pairs.data);
}

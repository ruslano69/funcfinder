module ffd_calls;

import ffd_model;
import ffd_tokens;
import std.algorithm : canFind;
import std.string : startsWith;
import reuse : Buffer;

struct CallSite { string name, receiver; int line; size_t token; }
struct CallScan { bool[] declaredNames; int[] declarationTokens; CallSite[] sites; }
private bool[] namesPool, callsPool, nonCallsPool;
private int[] positionsPool, rangesPool;
private Buffer!CallSite sitesBuffer;

bool callWord(Token t) {
    return t.word || t.text.startsWith("#") && identifier(t.text[1..$]);
}

// Default results own storage. A synchronous statistics consumer can borrow the result
// until the next scanCalls/tokensFor on this thread, copying only distinct retained names.
CallScan scanCalls(Tokens tokens, Sym[] functions, FunctionSignature[] signatures, string lang, string[] excluded, bool owned = true) {
    CallScan result; bool[] declaredCalls;
    declarations(tokens,functions,signatures,lang,result.declaredNames,declaredCalls,result.declarationTokens);
    auto nonCalls = lang == "go" ? goNonCalls(tokens) : null;
    sitesBuffer.data[] = CallSite.init;
    sitesBuffer.clear();
    foreach (i,t; tokens.items) {
        if (!callWord(t) || t.literal || declaredCalls[i] || nonCalls.length && nonCalls[i] ||
            !callAfter(tokens,i,lang == "go",lang == "py") || excluded.canFind(t.text)) continue;
        string receiver;
        if (i >= 2 && tokens.items[i-1].text == "." && callWord(tokens.items[i-2]) && !tokens.items[i-2].literal)
            receiver = tokens.items[i-2].text;
        else if (i >= 3 && tokens.items[i-1].text == "." && ["?","!"].canFind(tokens.items[i-2].text) &&
            callWord(tokens.items[i-3]) && !tokens.items[i-3].literal)
            receiver = tokens.items[i-3].text;
        sitesBuffer ~= CallSite(owned ? t.text.idup : t.text,owned ? receiver.idup : receiver,t.line,i);
    }
    result.sites = sitesBuffer.data;
    if (owned) {
        result.declaredNames = result.declaredNames.dup;
        result.declarationTokens = result.declarationTokens.dup;
        result.sites = result.sites.dup;
    }
    return result;
}

// Native coordinates identify declaration occurrences, not whole lines. An arrow's recursive call
// on its binding line remains a use. Identifier suppression is separate from call suppression.
private void declarations(Tokens tokens, Sym[] functions, FunctionSignature[] signatures, string lang, out bool[] names, out bool[] calls, out int[] positionsOut) {
    names = zeroScratch(namesPool,tokens.items.length);
    calls = zeroScratch(callsPool,tokens.items.length);
    positionsOut = zeroScratch(positionsPool,functions.length); positionsOut[] = -1;
    size_t[][string] positions;
    foreach (i,f; functions) {
        auto name = i < signatures.length && signatures[i].name.length ? signatures[i].name : f.name;
        if (name.startsWith("~")) name = name[1..$];
        positions[name] = null;
    }
    foreach (i,t; tokens.items) if (callWord(t) && !t.literal)
        if (auto list = t.text in positions) *list ~= i;
    foreach (fnIndex,f; functions) {
        auto signature = fnIndex < signatures.length ? signatures[fnIndex] : FunctionSignature.init;
        auto name = signature.name.length ? signature.name : f.name;
        if (name.startsWith("~")) name = name[1..$];
        int selected = -1;
        auto candidates = name in positions;
        if (candidates is null) continue;
        foreach (i; *candidates) {
            auto t = tokens.items[i];
            if (names[i] || t.literal || t.line < f.start || t.line > f.end || t.text != name) continue;
            if (signature.line > 0 && t.line != signature.line) continue;
            if (lang == "py" && (i == 0 || tokens.items[i-1].text != "def")) continue;
            bool namedKeyword = i > 0 && ["def","fun","func","fn","function","const","let","var"].canFind(tokens.items[i-1].text);
            if (!namedKeyword && !(lang == "js" && signature.line == 0) && !callAfter(tokens,i)) continue;
            selected = cast(int)i; break;
        }
        if (selected < 0) continue;
        names[selected] = true;
        positionsOut[fnIndex] = selected;
        // JS native metadata deliberately omits a signature for arrow/anonymous bindings.
        if (lang != "js" || signature.line > 0) calls[selected] = true;
    }
}

private bool callAfter(Tokens tokens, size_t i, bool go = false, bool python = false) {
    auto t = tokens.items;
    auto j = i+1;
    if (python && j < t.length && t[j].text == "\n") return false;
    while (j < t.length && t[j].text == "\n") ++j;
    if (go && j < t.length && t[j].text == "[" && tokens.pairs[j] > j)
        j = tokens.pairs[j]+1; // generic instantiation or indexed callable
    if (j < t.length && t[j].text == "<") {
        int depth;
        for (; j < t.length; ++j) {
            if (t[j].text == "<") ++depth;
            if (t[j].text == ">" && --depth == 0) { ++j; break; }
            if ([";","{","}"].canFind(t[j].text)) return false;
        }
    }
    return j < t.length && t[j].text == "(";
}

private bool[] goNonCalls(Tokens tokens) {
    auto t = tokens.items;
    auto blocked = zeroScratch(nonCallsPool,t.length);
    auto ranges = zeroScratch(rangesPool,t.length+1);
    foreach (i, token; t) {
        if (token.text == "interface" && i+1 < t.length && t[i+1].text == "{" && tokens.pairs[i+1] > i+1) {
            ++ranges[i+2]; --ranges[tokens.pairs[i+1]];
        }
        if (["if","else","for","switch","select","case","default","return","defer","go","range","break","continue","fallthrough"].canFind(token.text)) blocked[i] = true;
        // []T(nil), [N]T(...), map[K]T(...) and chan T(...): the callable expression
        // is a composite type, not the final identifier in that type.
        auto before = i;
        while (before >= 2 && t[before-1].text == "." && t[before-2].word) before -= 2;
        while (before > 0 && t[before-1].text == "*") --before;
        if (before > 0 && (t[before-1].text == "]" && t[before-1].line == token.line || t[before-1].text == "chan")) blocked[i] = true;
    }
    int depth;
    foreach (i; 0..t.length) { depth += ranges[i]; if (depth > 0) blocked[i] = true; }
    return blocked;
}

unittest {
    // A default graph result survives reuse of token/mask/site scratch.
    auto saved = scanCalls(tokensFor("void f() { obj.ping(); f(); }","cpp"),
        [Sym("f",1,1)],[FunctionSignature(1,"f")],"cpp",[]);
    assert(saved.sites.length == 2 && saved.sites[0].name == "ping" && saved.sites[0].receiver == "obj");
    auto declaration = saved.declarationTokens[0];
    assert(declaration >= 0 && saved.declaredNames[declaration]);
    foreach (_; 0..3) {
        auto borrowed = scanCalls(tokensFor("ping();","cpp"),[],[],"cpp",[],false);
        assert(borrowed.sites.length == 1 && borrowed.sites[0].name == "ping");
        assert(!borrowed.declaredNames[0]);
        assert(saved.sites[0].name == "ping" && saved.sites[0].receiver == "obj");
        assert(saved.sites[1].name == "f" && saved.declarationTokens[0] == declaration && saved.declaredNames[declaration]);
    }
}

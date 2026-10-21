module ffd_tokens;

import std.string : split, replace, strip, startsWith;
import std.uni : isAlpha, isNumber;
import reuse : Buffer;

struct Token { string text; int line; bool word, literal; }
struct Tokens { Token[] items; int[] pairs; bool[] codeLines, metadataLines; }

// All returned arrays/text are scratch until the next tokensFor call on this thread.
private Buffer!Token tokenBuffer;
private bool[] codePool, metadataPool;

private void startTokens(size_t count) {
    tokenBuffer.data[] = Token.init; // release old text references, including an unused tail
    tokenBuffer.clear();
    tokenBuffer.reserve(count);
}

// Reset a consumer's thread-local primitive array without allocating for each file.
T[] zeroScratch(T)(ref T[] pool, size_t count) {
    if (pool.length < count) pool = new T[count + count/2 + 16];
    auto result = pool[0..count]; result[] = T.init;
    return result;
}

private Tokens convert(T)(T native) {
    startTokens(native.items.length);
    Tokens result; result.pairs = native.pairs;
    foreach (t; native.items) {
        bool literal;
        static if (__traits(hasMember, typeof(t), "literal")) literal = t.literal;
        tokenBuffer ~= Token(t.text, t.line, t.word, literal);
    }
    result.items = tokenBuffer.data;
    return result;
}

bool identifier(string s) {
    if (!s.length) return false;
    foreach (i, dchar c; s) if (!(isAlpha(c) || c == '_' || c == '$' || i > 0 && isNumber(c))) return false;
    return true;
}

// Reuse the finders' lexical rules: no independent comment/string cleaner.
Tokens tokensFor(string source, string lang) {
    switch (lang) {
        case "py": import python_port : tokenizePython; return convert(tokenizePython(source.replace("\r\n","\n")));
        case "rust": import rust_port : tokenizeRust; return convert(tokenizeRust(source));
        case "kotlin": import kotlin_port : tokenizeKotlin; return convert(tokenizeKotlin(source));
        case "swift": import swift_port : tokenizeSwift; return convert(tokenizeSwift(source));
        case "scala": import scala_port : tokenizeScala; return convert(tokenizeScala(source));
        case "php": import php_port : tokenizePHP; return convert(tokenizePHP(source));
        case "ruby": import ruby_port : tokenizeRuby; return convert(tokenizeRuby(source));
        case "js", "ts": {
            import js_ts_cleaner : Cleaner;
            import javascript_port : tokenize;
            auto lines = source.split("\n"); Cleaner cleaner; auto facts = cleaner.facts(lines); int[] pairs;
            auto native = tokenize(lines, facts, pairs); Tokens result; result.pairs = pairs;
            startTokens(native.length);
            foreach (t; native) tokenBuffer ~= Token(t.text,t.line+1,identifier(t.text),t.literal);
            result.items = tokenBuffer.data;
            return result;
        }
        default: {
            import brace_port_cleaner : Cleaner, LineFact;
            import cpp_tokenizer : tokenizeCpp;
            auto lines = source.split("\n"); LineFact[] facts;
            if (lang == "d") {
                import dlang_port : prepare;
                int[int] ends; byte[] hints; auto cleaned = prepare(lines,ends,hints);
                facts.length = cleaned.length; foreach (i,line; cleaned) facts[i].cleaned = line;
            } else {
                Cleaner cleaner; cleaner.csharp = lang == "cs"; cleaner.golang = lang == "go";
                cleaner.charLiterals = lang == "go" || lang == "java";
                facts = cleaner.facts(lines);
            }
            // Retain line classification before finder-only directive masking.
            Tokens result;
            result.codeLines = zeroScratch(codePool,facts.length);
            result.metadataLines = zeroScratch(metadataPool,facts.length);
            foreach (i,f; facts) {
                result.codeLines[i] = f.cleaned.strip.length != 0;
                result.metadataLines[i] = f.startState == 0 && (result.codeLines[i] || lang == "go" && lines[i].strip.startsWith("\""));
            }
            // Exclude continued directives just as the finder does.
            bool directive;
            foreach (ref f; facts) {
                auto text = f.cleaned.strip;
                if (directive || text.startsWith("#")) { directive = text.length && text[$-1] == '\\'; f.cleaned = ""; }
            }
            auto native = tokenizeCpp(facts); result.pairs = native.pairs;
            startTokens(native.items.length);
            foreach (t; native.items) tokenBuffer ~= Token(t.text,t.line+1,t.word,false);
            result.items = tokenBuffer.data;
            return result;
        }
    }
}

unittest {
    // The neutral pool is reused across backend changes; shorter files must not retain flags/tokens.
    foreach (lang; ["go","js","py","rust","cpp"]) {
        auto t = tokensFor("ping()",lang);
        assert(t.items.length && t.items[0].text == "ping" && t.items[0].line == 1);
        assert(t.items.length == t.pairs.length);
    }
    assert(tokensFor("","cpp").items.length == 0);
}

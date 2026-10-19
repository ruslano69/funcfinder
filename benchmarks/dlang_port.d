// Experimental D-only finder API port. It does not replace the shipped CLI.
module dlang_port;

import std.algorithm : canFind, any, max, swap;
import std.array : split, join, array, appender;
import std.conv : to;
import std.file : readText;
import std.json;
import std.regex;
import std.stdio;
import std.string : strip, stripLeft, startsWith, endsWith;
import std.datetime.stopwatch : StopWatch, AutoStart;
import std.uni : isWhite, isAlpha, isNumber;
import std.utf : validate, decode;
import std.uni : unicode, CodepointSet;
import std.typecons : Yes;

struct Bounds { string name; int start, end; string className; }
struct Result { Bounds[] functions, classes; }
// __gshared: configure() runs once, before any worker thread, and the patterns are only read afterwards (a compiled
// std.regex Regex holds no mutable state; each match allocates its own matcher).
__gshared Regex!char funcPattern, classPattern, attrPattern, typePattern, stmtPattern;
__gshared Regex!char declarationPattern, continuationPattern;
__gshared string[] excluded;
Regex!char makePattern(string p)() {
    version (CompiledPatterns) return ctRegex!p;
    else return regex(p);
}

void configure(string path) { configure(parseJSON(readText(path))); }
void configure(JSONValue cfg) {
    version (CompiledPatterns) {
        import dlang_port_patterns;
        if (cfg["func_pattern"].str!=functionPatternText || cfg["class_pattern"].str!=classPatternText)
            throw new Exception("compiled pattern configuration mismatch");
        version (AuditScanner) funcPattern=makePattern!functionPatternText();
        classPattern=makePattern!classPatternText();
    } else {
        version (AuditScanner) funcPattern = regex(cfg["func_pattern"].str);
        classPattern = regex(cfg["class_pattern"].str);
    }
    if (cfg["func_pattern"].str != headerPatternText)
        throw new Exception("func_pattern differs from the header scanner's pattern");
    foreach (w; cfg["exclude_words"].array) excluded ~= w.str;
    attrPattern = makePattern!(`^\s*(?:(?:@[\p{L}\p{Nd}_]+|public|private|protected|export|pure|nothrow|const|immutable|shared|inout|ref|scope)\s*)+$`)();
    typePattern = makePattern!(`^\s*(?:(?:@[\p{L}\p{Nd}_]+|public|private|protected|package|static|final|abstract)\s+)*(?:class|struct|interface|union|enum|template|alias)\b`)();
    stmtPattern = makePattern!(`^\s*(?:(?:static|final)\s+)*(?:return|throw|new|delete|if|else|for|foreach|while|switch|try|catch|with|assert|version|debug|scope|mixin|pragma|cast)\b`)();
    declarationPattern = makePattern!(`^(?:@|(?:(?:public|private|protected|internal|override|open|abstract|final|inline|suspend|const|lateinit|external|operator|infix|data|sealed|enum|companion|inner|static|mutating|lazy)\s+)*(?:val|var|fun|func|class|object|interface|typealias|init|constructor|struct|protocol|extension)\b)`)();
    continuationPattern = makePattern!(`^(?:\.|\?[.:]|else\b)`)();
}
// --- D function headers without the regular expression -------------------------------------------------------------
// A port of internal/d_header.go (Go funcfinder): a scanner that returns exactly the result of the D func_pattern,
// whether a line matches and which name it captures, in the pattern's leftmost-first order. It stands in for the
// pattern only while the configuration holds headerPatternText; configure() rejects any other pattern. Built with
// -d-version=AuditScanner, every call is compared with the regular expression.
enum headerPatternText =
    `^\s*(?:(?:@[\p{L}\p{Nd}_]+(?:\((?:[^()]|\([^()]*\))*\))?|public|private|protected|package|export|sta` ~
    `tic|final|override|abstract|synchronized|pure|nothrow|scope|ref|const|immutable|shared|inout|auto)\s` ~
    `+)*(?:(~?this|new)|(?:(?:const|immutable|shared|inout|typeof)\s*\((?:[^()]|\([^()]*\))*\)|\.?[\p{L}\` ~
    `p{Nd}_]+(?:\.[\p{L}\p{Nd}_]+)*(?:!(?:\((?:[^()]|\([^()]*\))*\)|[\p{L}\p{Nd}_]+))?)(?:\s*\*|\[[^\]]*\` ~
    `])*(?:(?:\s*\*)+\s*|\s+)(?:(?:delegate|function)\((?:[^()]|\([^()]*\))*\)(?:\s+(?:@[\p{L}\p{Nd}_]+|p` ~
    `ure|nothrow))*\s+)?([\p{L}\p{Nd}_]+))\s*\(|^\s*(?:(?:@[\p{L}\p{Nd}_]+(?:\((?:[^()]|\([^()]*\))*\))?|` ~
    `public|private|protected|package|export|static|final|override|abstract|synchronized|pure|nothrow|sco` ~
    `pe|ref|const|immutable|shared|inout|auto)\s+)*@property\s+([\p{L}\p{Nd}_]+)\s*\(`;

__gshared CodepointSet headerLetters, headerDigits;
shared static this() { headerLetters = unicode.L; headerDigits = unicode.Nd; }

immutable string[] headerModifiers = ["public", "private", "protected", "package", "export", "static", "final",
    "override", "abstract", "synchronized", "pure", "nothrow", "scope", "ref", "const", "immutable", "shared",
    "inout", "auto"];
immutable string[] headerTypeWords = ["const", "immutable", "shared", "inout", "typeof"];
immutable string[] headerDelegateWords = ["delegate", "function"];
immutable string[] headerSelfWords = ["~this", "this", "new"];

// Scratch lists, reused per call: positions after modifiers, after type suffixes, after separator stars, after
// delegate attributes. They are never in use twice at once.
size_t[] headerPositions, headerEnds, headerStars, headerAttrs;

bool headerSpace(char c) { return c == ' ' || c == '\t' || c == '\n' || c == '\f' || c == '\r'; }

size_t headerSkip(string s, size_t i) {
    while (i < s.length && headerSpace(s[i])) ++i;
    return i;
}

// End of [\p{L}\p{Nd}_]* from i.
size_t headerIdentEnd(string s, size_t i) {
    while (i < s.length) {
        immutable char b = s[i];
        if (b < 0x80) {
            if (b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')) { ++i; continue; }
            return i;
        }
        size_t j = i;
        immutable dchar r = decode!(Yes.useReplacementDchar)(s, j);
        if (!headerLetters[r] && !headerDigits[r]) return i;
        i = j;
    }
    return i;
}

// `\((?:[^()]|\([^()]*\))*\)` at i: end or -1.
ptrdiff_t headerParenEnd(string s, size_t i) {
    if (i >= s.length || s[i] != '(') return -1;
    size_t j = i + 1;
    while (j < s.length) {
        immutable char c = s[j];
        if (c == ')') return j + 1;
        if (c == '(') {
            size_t k = j + 1;
            while (k < s.length && s[k] != '(' && s[k] != ')') ++k;
            if (k >= s.length || s[k] != ')') return -1;
            j = k + 1;
        } else ++j;
    }
    return -1;
}

// One modifier and its whitespace at i: end or -1.
ptrdiff_t headerModifierEnd(string s, size_t i) {
    if (i < s.length && s[i] == '@') {
        immutable j = headerIdentEnd(s, i + 1);
        if (j == i + 1) return -1;
        immutable p = headerParenEnd(s, j);
        if (p >= 0) { immutable w = headerSkip(s, cast(size_t) p); if (w > cast(size_t) p) return w; }
        immutable w = headerSkip(s, j);
        return w > j ? cast(ptrdiff_t) w : -1;
    }
    foreach (kw; headerModifiers) {
        if (s[i .. $].startsWith(kw)) {
            immutable e = i + kw.length;
            immutable w = headerSkip(s, e);
            if (w > e) return cast(ptrdiff_t) w;
        }
    }
    return -1;
}

bool headerNameAt(string s, size_t h, ref string name) {
    immutable e = headerIdentEnd(s, h);
    if (e == h) return false;
    immutable w = headerSkip(s, e);
    if (w >= s.length || s[w] != '(') return false;
    name = s[h .. e];
    return true;
}

bool headerDelegateName(string s, size_t g, ref string name) {
    foreach (kw; headerDelegateWords) {
        if (!s[g .. $].startsWith(kw)) continue;
        immutable p = headerParenEnd(s, g + kw.length);
        if (p < 0) break;
        headerAttrs.length = 0; headerAttrs.assumeSafeAppend; headerAttrs ~= cast(size_t) p;
        size_t cur = cast(size_t) p;
        for (;;) {
            immutable w = headerSkip(s, cur);
            if (w == cur || w >= s.length) break;
            if (s[w] == '@') {
                immutable n = headerIdentEnd(s, w + 1);
                if (n == w + 1) break;
                cur = n;
            } else if (s[w .. $].startsWith("pure")) cur = w + 4;
            else if (s[w .. $].startsWith("nothrow")) cur = w + 7;
            else break;
            headerAttrs ~= cur;
        }
        foreach_reverse (a; headerAttrs) {
            immutable h = headerSkip(s, a);
            if (h > a && headerNameAt(s, h, name)) return true;
        }
        break;
    }
    return headerNameAt(s, g, name);
}

bool headerTypeTail(string s, size_t t, ref string name) {
    headerEnds.length = 0; headerEnds.assumeSafeAppend; headerEnds ~= t;
    for (size_t cur = t;;) {
        immutable w = headerSkip(s, cur);
        if (w < s.length && s[w] == '*') cur = w + 1;
        else if (cur < s.length && s[cur] == '[') {
            size_t k = cur + 1;
            while (k < s.length && s[k] != ']') ++k;
            if (k >= s.length) break;
            cur = k + 1;
        } else break;
        headerEnds ~= cur;
    }
    foreach_reverse (e; headerEnds) {
        headerStars.length = 0; headerStars.assumeSafeAppend;
        for (size_t cur = e;;) {
            immutable w = headerSkip(s, cur);
            if (w >= s.length || s[w] != '*') break;
            cur = w + 1;
            headerStars ~= cur;
        }
        foreach_reverse (f; headerStars)
            if (headerDelegateName(s, headerSkip(s, f), name)) return true;
        immutable g = headerSkip(s, e);
        if (g > e && headerDelegateName(s, g, name)) return true;
    }
    return false;
}

bool headerAt(string s, size_t pos, ref string name) {
    foreach (word; headerSelfWords) {
        if (!s[pos .. $].startsWith(word)) continue;
        immutable e = pos + word.length;
        immutable w = headerSkip(s, e);
        if (w < s.length && s[w] == '(') { name = s[pos .. e]; return true; }
    }
    foreach (kw; headerTypeWords) {
        if (s[pos .. $].startsWith(kw)) {
            immutable p = headerParenEnd(s, headerSkip(s, pos + kw.length));
            if (p >= 0 && headerTypeTail(s, cast(size_t) p, name)) return true;
            break;
        }
    }
    size_t j = pos;
    if (j < s.length && s[j] == '.') ++j;
    size_t e = headerIdentEnd(s, j);
    if (e == j) return false;
    while (e < s.length && s[e] == '.') {
        immutable n = headerIdentEnd(s, e + 1);
        if (n == e + 1) break;
        e = n;
    }
    if (e < s.length && s[e] == '!') {
        immutable p = headerParenEnd(s, e + 1);
        if (p >= 0) e = cast(size_t) p;
        else {
            immutable n = headerIdentEnd(s, e + 1);
            if (n > e + 1) e = n; else return false;
        }
    }
    return headerTypeTail(s, e, name);
}

// The D func_pattern's result for s: whether it matches, and the captured name (the last nonempty group).
bool headerMatch(string s, ref string name) {
    headerPositions.length = 0; headerPositions.assumeSafeAppend;
    headerPositions ~= headerSkip(s, 0);
    for (;;) {
        immutable e = headerModifierEnd(s, headerPositions[$ - 1]);
        if (e < 0) break;
        headerPositions ~= cast(size_t) e;
    }
    foreach_reverse (pos; headerPositions) if (headerAt(s, pos, name)) return true;
    foreach_reverse (pos; headerPositions) {
        if (!s[pos .. $].startsWith("@property")) continue;
        immutable w = headerSkip(s, pos + 9);
        if (w > pos + 9 && headerNameAt(s, w, name)) return true;
    }
    return false;
}

// header() and the scan's name: the scanner, audited against the regular expression when built for it.
bool headerScan(string s, ref string name) {
    immutable got = headerMatch(s, name);
    version (AuditScanner) {
        immutable want = nameOf(s, funcPattern);
        if (got != !matchFirst(s, funcPattern).empty || (got && name != want))
            throw new Exception("header scanner mismatch: " ~ s);
    }
    return got;
}

bool hit(string s, Regex!char re) { return !matchFirst(s, re).empty; }
string nameOf(string s, Regex!char re) {
    auto m = matchFirst(s, re);
    if (m.empty) return "";
    for (size_t i=m.captures.length; i>1; --i)
        if (m.captures[i-1].length) return m.captures[i-1];
    return "";
}
bool header(string s) { string name; return s.canFind('(') && headerScan(s, name); }
string headerName(string s) { string name; return headerScan(s, name) ? name : ""; }
bool attribute(string s) {
    auto got=attributeScan(s);
    version(AuditPredicates) assert(got==attributeRegex(s), "attribute scanner mismatch: " ~ s);
    return got;
}
bool attributeRegex(string s) {
    auto t=s;
    while(t.length && " \t\n\f\r".canFind(t[0])) t=t[1..$];
    return t.length && "@pencisr".canFind(t[0]) && hit(s,attrPattern);
}
bool typeHeader(string s) {
    auto got=typeScan(s);
    version(AuditPredicates) assert(got==typeRegex(s), "type scanner mismatch: " ~ s);
    return got;
}
bool typeRegex(string s) {
    static immutable string[7] words=["class","struct","interface","union","enum","template","alias"];
    foreach(word;words) if(s.canFind(word)) return hit(s,typePattern);
    return false;
}
bool statementHeader(string s) {
    auto got=statementScan(s);
    version(AuditPredicates) assert(got==statementRegex(s), "statement scanner mismatch: " ~ s);
    return got;
}
bool statementRegex(string s) {
    auto t=s.stripLeft;
    // Every statement alternative, including static/final prefixes, starts here.
    return t.length && "sftrndiewcavmp".canFind(t[0]) && hit(s,stmtPattern);
}
// Rare non-ASCII whitespace/boundaries retain std.regex semantics. Ordinary
// ASCII prefixes are read directly; identifiers after @ use the same L/Nd sets.
bool predicateOdd(string s, size_t i) {
    return i<s.length && (s[i]>=0x80 || s[i]=='\v');
}
bool predicateBoundary(string s, size_t i) {
    return i==s.length || !(s[i]=='_' || (s[i]>='a' && s[i]<='z') ||
        (s[i]>='A' && s[i]<='Z') || (s[i]>='0' && s[i]<='9'));
}
bool attributeScan(string s) {
    static immutable string[] words=["public","private","protected","export","pure","nothrow","const","immutable","shared","inout","ref","scope"];
    auto i=headerSkip(s,0);
    if(i==s.length || !"@pencisr".canFind(s[i])) return false;
    while(i<s.length) {
        if(s[i]=='@') {
            auto e=headerIdentEnd(s,i+1);
            if(e==i+1) return false;
            i=e;
        } else {
            size_t end=i;
            foreach(word;words) if(s[i..$].startsWith(word)) { end=i+word.length; break; }
            if(end==i) return false;
            i=end;
        }
        i=headerSkip(s,i);
        if(predicateOdd(s,i)) return attributeRegex(s);
    }
    return true;
}
bool typeScan(string s) {
    static immutable string[] modifiers=["public","private","protected","package","static","final","abstract"];
    static immutable string[] kinds=["class","struct","interface","union","enum","template","alias"];
    auto i=headerSkip(s,0);
    while(i<s.length) {
        if(predicateOdd(s,i)) return typeRegex(s);
        foreach(word;kinds) if(s[i..$].startsWith(word)) {
            auto e=i+word.length;
            if(predicateOdd(s,e)) return typeRegex(s);
            if(predicateBoundary(s,e)) return true;
        }
        size_t end=i;
        if(s[i]=='@') {
            end=headerIdentEnd(s,i+1);
            if(end==i+1) return false;
        } else foreach(word;modifiers) if(s[i..$].startsWith(word)) { end=i+word.length; break; }
        if(end==i) return false;
        auto w=headerSkip(s,end);
        if(predicateOdd(s,w)) return typeRegex(s);
        if(w==end) return false;
        i=w;
    }
    return false;
}
bool statementScan(string s) {
    static immutable string[] words=["return","throw","new","delete","if","else","for","foreach","while","switch","try","catch","with","assert","version","debug","scope","mixin","pragma","cast"];
    auto i=headerSkip(s,0);
    while(i<s.length) {
        if(predicateOdd(s,i)) return statementRegex(s);
        size_t end=i;
        if(s[i..$].startsWith("static")) end=i+6;
        else if(s[i..$].startsWith("final")) end=i+5;
        if(end==i) break;
        auto w=headerSkip(s,end);
        if(predicateOdd(s,w)) return statementRegex(s);
        if(w==end) return false;
        i=w;
    }
    foreach(word;words) if(s[i..$].startsWith(word)) {
        auto e=i+word.length;
        if(predicateOdd(s,e)) return statementRegex(s);
        if(predicateBoundary(s,e)) return true;
    }
    return false;
}
bool prefixAt(T)(const(T)[] s, size_t i, const(dchar)[] p) {
    if(i+p.length>s.length) return false;
    foreach(j,ch;p) if(s[i+j]!=ch) return false;
    return true;
}
bool asciiLine(string line) @nogc nothrow {
    foreach(ch;line) if(ch>=0x80) return false;
    return true;
}
string ownCleanText(T)(T[] code) {
    static if(is(T==char)) return code.idup;
    else return to!string(code);
}
bool plainCode(string line) @nogc nothrow {
    foreach(i,ch;line) {
        if(ch>=0x80 || ch=='/' || ch=='"' || ch=='\'' || ch=='`') return false;
        if(ch=='q' && i+1<line.length && line[i+1]=='{') return false;
    }
    return true;
}
bool plainPrefix(string line) {
    size_t i;
    while(i<line.length && " \t\n\v\f\r".canFind(line[i])) ++i;
    if(i==line.length) return true;
    if(line[i]>=0x80 || line[i]=='@') return false;
    auto start=i;
    while(i<line.length && (line[i]=='_' || line[i]=='.' || (line[i]>='0' && line[i]<='9') || (line[i]>='a' && line[i]<='z') || (line[i]>='A' && line[i]<='Z'))) ++i;
    if(i<line.length && line[i]>=0x80) return false;
    if(start==i) return true;
    auto word=line[start..i];
    switch(word) {
        case "static": case "version": case "debug": case "pragma": case "extern":
        case "deprecated": case "package": case "final": case "override": case "abstract": case "auto": return false;
        default: return !attribute(word);
    }
}
// Buffers never escape: UTF-8 results own copies; capacity grows for long lines.
T[] bufferSpan(T)(ref T[] buffer, size_t count) {
    if(buffer.length<count) buffer.length=max(count,max(cast(size_t)32,buffer.length*2));
    return buffer[0..count];
}
dchar[] decodeBuffer(string line, ref dchar[] buffer) {
    auto space=bufferSpan(buffer,line.length);
    size_t count;
    foreach(dchar ch;line) space[count++]=ch;
    return space[0..count];
}
struct Lexer {
    int comments, tokens, qDepth;
    dchar quote=0, qOpen=0, qClose=0;
    bool raw;
    dstring commentClose, qEnd;
    dchar[] inputScratch, codeScratch;
    char[] asciiScratch;
    string clean(string line) {
        if(!comments && !qEnd.length && !qDepth && !quote && !tokens && plainCode(line)) {
            version(AuditFastPaths) {
                auto shadow=this;
                if(shadow.cleanSlow(line)!=line || shadow.comments!=comments || shadow.tokens!=tokens || shadow.qDepth!=qDepth || shadow.quote!=quote || shadow.raw!=raw || shadow.qOpen!=qOpen || shadow.qClose!=qClose || shadow.commentClose!=commentClose || shadow.qEnd!=qEnd)
                    throw new Exception("lexer fast-path/state mismatch");
            }
            return line;
        }
        if(asciiLine(line)) {
            version(AuditFastPaths) {
                auto shadow=this;
                auto expected=shadow.cleanSlow(line);
                auto actual=cleanUnits!char(line,asciiScratch);
                assert(actual==expected && comments==shadow.comments && tokens==shadow.tokens && qDepth==shadow.qDepth && quote==shadow.quote && raw==shadow.raw && qOpen==shadow.qOpen && qClose==shadow.qClose && commentClose==shadow.commentClose && qEnd==shadow.qEnd, "ASCII cleaner/state mismatch");
                return actual;
            } else return cleanUnits!char(line,asciiScratch);
        }
        return cleanSlow(line);
    }
    string cleanSlow(string line) {
        auto s = decodeBuffer(line,inputScratch);
        return cleanUnits!dchar(s,codeScratch);
    }
    string cleanUnits(T)(const(T)[] s, ref T[] scratch) {
        auto code = bufferSpan(scratch,s.length); code[] = ' ';
        size_t i;
        while (i<s.length) {
            dchar ch=s[i];
            if (comments>0) {
                if (commentClose=="+/"d && prefixAt(s,i,"/+"d)) { ++comments; i+=2; }
                else if (prefixAt(s,i,commentClose)) { --comments; i+=2; }
                else ++i;
                continue;
            }
            if (qEnd.length) {
                if (prefixAt(s,i,qEnd)) { i+=qEnd.length; qEnd=null; } else ++i;
                continue;
            }
            if (qDepth>0) {
                if (ch==qOpen) ++qDepth;
                else if (ch==qClose) { --qDepth; if (!qDepth && i+1<s.length && s[i+1]=='"') ++i; }
                ++i; continue;
            }
            if (quote) {
                if (ch=='\\' && !raw) i+=2;
                else { if (ch==quote) quote=0; ++i; }
                continue;
            }
            if (i+1<s.length && ch=='/') {
                if (s[i+1]=='/') return ownCleanText(code);
                if (s[i+1]=='*' || s[i+1]=='+') {
                    comments=1; commentClose=s[i+1]=='+' ? "+/"d : "*/"d; i+=2; continue;
                }
            }
            if (i+1<s.length && ch=='q' && (s[i+1]=='{' || s[i+1]=='"')) {
                if (s[i+1]=='{') { ++tokens; i+=2; continue; }
                if (i+2<s.length) {
                    auto op=s[i+2]; dchar cl=0;
                    switch (op) { case '[': cl=']'; break; case '(': cl=')'; break;
                        case '{': cl='}'; break; case '<': cl='>'; break; default: break; }
                    if (cl) { qOpen=op; qClose=cl; qDepth=1; }
                    else {
                        auto e=i+3;
                        while (e<s.length && (isAlpha(s[e]) || isNumber(s[e]) || s[e]=='_')) ++e;
                        qEnd=to!dstring(s[i+2..e]) ~ cast(dchar)'"'; i=e; continue;
                    }
                    i+=3; continue;
                }
            }
            if (ch=='"' || ch=='\'' || ch=='`' || (ch=='r' && i+1<s.length && s[i+1]=='"')) {
                raw=ch=='`' || ch=='r';
                if (ch=='r') { ++i; ch='"'; }
                quote=ch; ++i; continue;
            }
            if (tokens>0) { if (ch=='{') ++tokens; else if (ch=='}') --tokens; }
            else code[i]=cast(T)ch;
            ++i;
        }
        return ownCleanText(code);
    }
}
size_t skipSpace(const(dchar)[] s, size_t i) { while (i<s.length && isWhite(s[i])) ++i; return i; }
string normalize(string line, ref dchar[] scratch) {
    if(plainPrefix(line)) {
        validate(line);
        version(AuditFastPaths) if(normalizeSlow(line,scratch)!=line) throw new Exception("prefix fast-path mismatch");
        return line;
    }
    return normalizeSlow(line,scratch);
}
string normalizeSlow(string line, ref dchar[] scratch) {
    auto code=decodeBuffer(line,scratch);
    for (size_t i; i<code.length;) {
        if (isWhite(code[i])) { ++i; continue; }
        auto start=i;
        if (code[i]=='@') ++i;
        while (i<code.length && (isAlpha(code[i]) || isNumber(code[i]) || code[i]=='_' || code[i]=='.')) ++i;
        auto word=to!string(code[start..i]);
        if (word=="static") {
            i=skipSpace(code,i);
            if (prefixAt(code,i,"if"d) && (i+2==code.length || !isAlpha(code[i+2]))) { word="static if"; i+=2; }
        }
        i=skipSpace(code,i);
        bool param=word=="static if" || word=="version" || word=="debug" || word=="pragma" || word=="extern" || word=="deprecated" || word=="package" || word.startsWith("@");
        if (param && i<code.length && code[i]=='(') {
            int depth=1; auto e=i+1;
            while (e<code.length && depth>0) { if (code[e]=='(') ++depth; else if (code[e]==')') --depth; ++e; }
            if (depth) break;
            auto tail=to!string(code[e..$]).strip;
            if (tail.length && !tail.startsWith("{")) code[start..e]=' ';
            i=e; continue;
        }
        if (word=="extern" || word=="deprecated" || word=="debug") { code[start..i]=' '; continue; }
        if (start==i || (!attribute(word) && !["static","final","override","abstract","auto"].canFind(word))) break;
    }
    return to!string(code);
}
string[] prepare(string[] lines, out int[int] headerEnds, out byte[] hints) {
    Lexer lexer; auto result=new string[lines.length];
    dchar[] scratch;
    foreach (i,line; lines) result[i]=normalize(lexer.clean(line),scratch);
    foreach (i; 0..result.length) {
        if (i+1>=result.length) break;
        auto t=result[i];
        if (!t.strip.length || t.canFind(';') || t.canFind('{') || t.canFind('}') || t.canFind('=') || statementHeader(t) || typeHeader(t) || attribute(t) || header(t)) continue;
        // A header requires '('. Grow one immutable-prefix candidate only when
        // it can match, instead of rebuilding every prefix for each lookahead.
        auto candidate=appender!string();
        bool hasParen=t.canFind('('), started;
        auto buffered=i;
        for (auto j=i+1; j<result.length && j<=i+4; ++j) {
            hasParen=hasParen || result[j].canFind('(');
            if(hasParen) {
                if(!started) { candidate~=result[i]; started=true; }
                foreach(k;buffered+1..j+1) { candidate~=" "; candidate~=result[k]; }
                buffered=j;
                if(header(candidate.data)) { result[i]=candidate.data; headerEnds[cast(int)i]=cast(int)j; result[i+1..j+1]=""; break; }
            }
            if (result[j].canFind(';') || result[j].canFind('{') || result[j].canFind('}') || result[j].canFind('=')) break;
        }
    }
    bool pending, contract, outSelector, outExpression;
    int parens, contractDepth, contractParens;
    hints=new byte[lines.length];
    foreach (i,line; result) {
        dchar[] code;
        auto text=line.strip; bool statement, modified;
        if (typeHeader(text)) {
            if(line.canFind('(') || line.canFind(')')) {
                code=decodeBuffer(line,scratch);
                foreach (ref ch; code) { if (ch=='(') { ch='['; modified=true; } else if (ch==')') { ch=']'; modified=true; } }
            }
        } else if (statementHeader(text)) {
            statement=true; code=decodeBuffer(line,scratch);
            foreach (ref ch; code) { if (!isWhite(ch)) { ch='?'; modified=true; break; } }
        }
        auto checked=modified ? to!string(code) : line;
        if (text.length && attribute(checked)) {
            if(!code.length) code=decodeBuffer(line,scratch);
            code[]=' '; result[i]=to!string(code); continue;
        }
        byte hint;
        if(statement) hint=-1;
        else if(!contractDepth) {
            hint=-1;
            if(header(checked)) { hint=1; pending=true; parens=0; contract=false; }
        }
        if(!pending && !contractDepth && !contractParens) {
            result[i]=checked; hints[i]=hint; continue;
        }
        if(!code.length) code=decodeBuffer(line,scratch);
        bool changed;
        for (size_t j; j<code.length; ++j) {
            auto ch=code[j];
            if (contractParens>0) {
                if (ch=='(') ++contractParens;
                else if (ch==')') { --contractParens; if (!contractParens && outSelector && !outExpression) contract=true; }
                else if (ch==';') outExpression=true;
                code[j]=' '; changed=true; continue;
            }
            if (contractDepth>0) {
                if (ch=='{') ++contractDepth; else if (ch=='}') --contractDepth;
                code[j]=' '; changed=true; continue;
            }
            if (!pending) continue;
            if (isAlpha(ch) && !parens) {
                auto e=j+1;
                while (e<code.length && (isAlpha(code[e]) || code[e]=='_')) ++e;
                auto word=code[j..e];
                if (word=="in"d || word=="out"d) { contract=true; outSelector=word=="out"d; outExpression=false; code[j..e]=' '; changed=true; }
                else if (word=="do"d || word=="body"d) { code[j..e]=' '; changed=true; }
                j=e-1; continue;
            }
            switch (ch) {
                case '(': case '[':
                    if (contract && ch=='(') { contractParens=1; contract=false; code[j]=' '; changed=true; } else ++parens;
                    break;
                case ')': case ']': --parens; break;
                case '{': if (!parens) { if (contract) { contractDepth=1; contract=false; code[j]=' '; changed=true; } else pending=false; } break;
                case ';': if (!parens) pending=false; break;
                default: break;
            }
        }
        result[i]=changed ? to!string(code) : checked;
        // Contract masking can invalidate an earlier match. Reuse only exact-text results.
        if(result[i]==checked) hints[i]=hint;
    }
    version (AuditHints) foreach(i,line;result) {
        auto matches=header(line);
        if(hints[i] && (hints[i]>0)!=matches) throw new Exception("invalid function hint at line " ~ to!string(i+1));
        if(attribute(line)!=hit(line,attrPattern)) throw new Exception("attribute gate mismatch at line " ~ to!string(i+1));
        if(typeHeader(line)!=hit(line,typePattern)) throw new Exception("type gate mismatch at line " ~ to!string(i+1));
        if(statementHeader(line)!=hit(line,stmtPattern)) throw new Exception("statement gate mismatch at line " ~ to!string(i+1));
    }
    return result;
}
Bounds[] classes(string[] lines) {
    Bounds[] result;
    foreach (start,line; lines) {
        if (!["class","struct","interface","enum","union"].any!(w=>line.canFind(w))) continue;
        auto name=nameOf(line,classPattern); if (!name.length) continue;
        auto end=start; int brackets, depth; bool opened;
        scan: foreach (i; start..lines.length) {
            foreach (ch; lines[i]) {
                if (!opened) {
                    switch (ch) {
                        case '(': case '[': ++brackets; break;
                        case ')': case ']': --brackets; break;
                        case ';': case '=': if (!brackets) { end=i; break scan; } break;
                        case '{': if (!brackets) { opened=true; depth=1; } break;
                        default: break;
                    }
                } else {
                    if (ch=='{') ++depth;
                    else if (ch=='}') { --depth; if (!depth) { end=i; break scan; } }
                }
            }
            end=i;
        }
        result ~= Bounds(name,cast(int)start+1,cast(int)end+1,"");
    }
    return result;
}
enum Event { open, body, end, unterminated }
char previous(string s, size_t i) { while (i>0) { --i; if (s[i]!=' ' && s[i]!='\t') return s[i]; } return 0; }
struct Signature {
    int brackets, typeBraces; size_t bodyAt;
    Event feed(string s) {
        foreach (i,ch; s) {
            switch (ch) {
                case '(': case '[': ++brackets; break;
                case ')': case ']': if (brackets>0) --brackets; break;
                case '{':
                    if (!brackets) { if (typeBraces>0 || previous(s,i)=='<') { ++typeBraces; continue; } bodyAt=i; return Event.body; } break;
                case '}': if (!brackets && typeBraces>0) --typeBraces; break;
                case ';': if (!brackets && !typeBraces) return Event.end; break;
                default: break;
            }
        }
        return Event.open;
    }
}
int braces(string s) { int n; foreach(ch;s) { if(ch=='{')++n; else if(ch=='}')--n; } return n; }
int parens(string s) { int n; foreach(ch;s) { if(ch=='(')++n; else if(ch==')')--n; } return n; }
int closeIndex(string s, int depth) {
    int n; foreach(i,ch;s) { if(ch=='{')++n; else if(ch=='}')--n; if(n<=-depth)return cast(int)i; } return -1;
}
bool continues(string[] lines, size_t i) {
    foreach (t; lines[i+1..$]) if (t.strip.length) return hit(t.strip,continuationPattern);
    return false;
}
struct Context { Bounds fn; int depth; Signature sig; bool pending; int sigLast; bool afterBlock, bodyStarted; int bodyDelta; }
Result find(string[] original) {
    int[int] headerEnds; byte[] hints; auto lines=prepare(original,headerEnds,hints);
    Result result; result.classes=classes(lines); int typeDepth;
    auto stack=appender!(Context[])();
    auto next=appender!(Context[])();
    Event[] eventScratch; int[] depthScratch;
    foreach (lineNum,cleaned; lines) {
        auto number=cast(int)lineNum+1; int delta;
        foreach(i,ch;cleaned) {
            if(ch=='{') { if(typeDepth>0 || previous(cleaned,i)=='<') ++typeDepth; else ++delta; }
            else if(ch=='}') { if(typeDepth>0)--typeDepth; else --delta; }
        }
        auto name=cleaned.canFind('(') && hints[lineNum]!=-1 ? headerName(cleaned) : "";
        auto events=bufferSpan(eventScratch,stack.length); events[]=Event.open;
        foreach(i,ref ctx;stack[]) {
            if(!ctx.pending) continue;
            bool decl=hit(cleaned.strip,declarationPattern) && !cleaned.strip.startsWith("object :");
            if((name.length || decl) && !ctx.sig.brackets) { events[i]=Event.unterminated; ctx.pending=false; continue; }
            auto atTop=!ctx.sig.brackets;
            events[i]=ctx.sig.feed(cleaned);
            if(events[i]==Event.body) { ctx.bodyStarted=true; ctx.bodyDelta=braces(cleaned[ctx.sig.bodyAt..$]); }
            if(events[i]!=Event.open) ctx.pending=false;
            else if(cleaned.strip.length && !(atTop && cleaned.strip.startsWith("}")) && !(atTop && hit(cleaned.strip,declarationPattern))) ctx.sigLast=number;
        }
        auto previousDepths=bufferSpan(depthScratch,stack.length);
        foreach(i,ref ctx;stack[]) {
            previousDepths[i]=ctx.depth;
            if(!ctx.pending) { if(ctx.bodyStarted) { ctx.depth+=ctx.bodyDelta; ctx.bodyStarted=false; } else ctx.depth+=delta; }
        }
        if(name.length && !excluded.canFind(name)) {
            Bounds fn=Bounds(name,number,0,""); Signature probe;
            auto body=probe.feed(cleaned)==Event.body;
            bool eq=cleaned.strip.endsWith("=") || cleaned.strip.endsWith("=>");
            if(delta<0 && body && !parens(cleaned[0..probe.bodyAt]) && !eq && closeIndex(cleaned[probe.bodyAt..$],0)>=0) { fn.end=number; result.functions~=fn; }
            else if(!delta && body && !parens(cleaned) && !eq) { fn.end=number; result.functions~=fn; }
            else if(!delta) {
                Signature sig;
                if(sig.feed(cleaned)==Event.end) { fn.end=number; result.functions~=fn; }
                else { Context ctx; ctx.fn=fn; ctx.sig=sig; ctx.pending=true; ctx.sigLast=number; stack~=ctx; }
            } else { Context ctx; ctx.fn=fn; ctx.depth=delta; stack~=ctx; }
        }
        next.clear();
        foreach(i,ctx;stack[]) {
            auto ev=i<events.length ? events[i] : Event.open;
            if(ev==Event.unterminated) { ctx.fn.end=ctx.sigLast; result.functions~=ctx.fn; continue; }
            if(ev==Event.end) { ctx.fn.end=number; result.functions~=ctx.fn; continue; }
            auto prev=i<previousDepths.length ? previousDepths[i] : 0;
            auto close=prev>0 ? closeIndex(cleaned,prev) : -1;
            if(close>=0 && ctx.depth<0) { ctx.fn.end=number; result.functions~=ctx.fn; }
            else if(!ctx.depth && (prev>0 || ev==Event.body || (ctx.afterBlock && cleaned.strip.length))) {
                if(continues(lines,lineNum)) { ctx.afterBlock=true; next~=ctx; }
                else { ctx.fn.end=number; result.functions~=ctx.fn; }
            } else next~=ctx;
        }
        swap(stack,next);
    }
    foreach(ctx;stack[]) {
        if(ctx.pending) { ctx.fn.end=ctx.sigLast; result.functions~=ctx.fn; }
        else if(ctx.afterBlock) {
            for(auto i=lines.length;i>0;--i) if(lines[i-1].strip.length) { ctx.fn.end=cast(int)i; break; }
            result.functions~=ctx.fn;
        }
    }
    foreach(ref fn;result.functions) {
        if(auto p=(fn.start-1) in headerEnds) { if(fn.end<*p+1)fn.end=*p+1; }
        int classStart=-1;
        foreach(cl;result.classes) if(cl.start<=fn.start && cl.end>=fn.end && cl.start>classStart) { fn.className=cl.name; classStart=cl.start; }
    }
    Bounds[] kept;
    foreach(fn;result.functions) {
        bool call;
        if(fn.name=="this") {
            Signature sig;
            if(sig.feed(lines[fn.start-1..fn.end].join("\n"))!=Event.body)
                foreach(parent;result.functions) if(parent.start<fn.start && parent.end>=fn.end && parent.className==fn.className) { call=true; break; }
        }
        if(!call)kept~=fn;
    }
    result.functions=kept;
    return result;
}
JSONValue encode(Result r) {
    JSONValue[] fs,cs;
    foreach(f;r.functions) fs~=JSONValue([JSONValue(f.name),JSONValue(f.start),JSONValue(f.end),JSONValue(f.className)]);
    foreach(c;r.classes) cs~=JSONValue([JSONValue(c.name),JSONValue(c.start),JSONValue(c.end)]);
    return JSONValue(["functions":JSONValue(fs),"classes":JSONValue(cs)]);
}
version(PredicateCheck) {} else
int main(string[] args) {
    if(args.length<4) { stderr.writeln("usage: dlang-port config.json manifest.json repeats [--time]"); return 2; }
    configure(args[1]); auto manifest=parseJSON(readText(args[2])).array;
    string[][] corpus;
    foreach(p;manifest) {
        auto text=readText(p.str); auto lines=text.split("\n");
        if(lines.length && !lines[$-1].length) lines.length--;
        foreach(ref line;lines) if(line.endsWith("\r"))line=line[0..$-1];
        corpus~=lines;
    }
    bool timing=args.length>4 && args[4]=="--time";
    if(timing) foreach(lines;corpus) find(lines);
    auto repeats=to!int(args[3]); size_t checksum; Result[] results;
    auto sw=StopWatch(AutoStart.yes);
    foreach(n;0..repeats) {
        results.length=0;
        foreach(lines;corpus) { auto r=find(lines); checksum+=r.functions.length+r.classes.length; results~=r; }
    }
    sw.stop();
    if(timing) writeln(JSONValue(["nanoseconds":JSONValue(sw.peek.total!"nsecs"),"checksum":JSONValue(checksum)]));
    else { foreach(i,r;results) { auto value=encode(r); value["path"]=manifest[i]; writeln(value); } }
    return 0;
}

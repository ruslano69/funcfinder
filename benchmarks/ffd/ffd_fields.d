module ffd_fields;

import ffd_model;
import std.algorithm : canFind;
import std.array : appender;
import std.string : join, split, strip, startsWith, replace;
import std.uni : isAlpha, isNumber, toLower;

import ffd_tokens : Token, Tokens, tokensFor, identifier;

private string spelling(Token[] t) {
    auto out_ = appender!(char[])();
    foreach (i, token; t) {
        if (token.text == "\n") continue;
        if (i && (identifier(t[i-1].text) && identifier(token.text) || t[i-1].text == ",")) out_ ~= ' ';
        out_ ~= token.text;
    }
    return out_.data.idup;
}

private enum modifiers = ["public","private","protected","internal","fileprivate","package","static","readonly","final","volatile","abstract","override","lateinit","lazy","var","val","let","mut","pub","export","declare","required","nonisolated"];
private enum declarations = ["class","interface","enum","union","trait","object","protocol","record","module","def","fun","func","fn","function","return","throw","if","while","for","switch","case","import","using","typedef","typealias","alias","type","init","constructor","deinit","get","set","friend","template","static_assert"];

private Field[] declaration(Token[] t, string lang) {
    while (t.length && t[$-1].text == "\n") t = t[0..$-1];
    if (!t.length) return null;
    bool annotation = t.length > 1 && t[1].text == ":";
    if (declarations.canFind(t[0].text) && !annotation) return null;
    size_t first;
    while (first < t.length && modifiers.canFind(lang == "php" ? t[first].text.toLower : t[first].text)) {
        ++first;
        // Rust restricted visibility: pub(crate), pub(in path).
        if (lang == "rust" && first < t.length && t[first].text == "(") {
            while (first < t.length && t[first].text != ")") ++first;
            if (first < t.length) ++first;
        }
    }
    if (first >= t.length || declarations.canFind(t[first].text) && !annotation) return null;
    size_t end = t.length;
    foreach (i, token; t) if (token.text == "=") { end = i; break; }
    if (lang == "php") {
        for (size_t i = first; i < end; ++i) {
            if (t[i].text == "$" && i+1 < end && t[i+1].word) return [Field(t[i+1].text.idup,spelling(t[first..i]),t[i+1].line)];
            if (t[i].text.startsWith("$") && t[i].text.length > 1) return [Field(t[i].text[1..$].idup,spelling(t[first..i]),t[i].line)];
        }
        return null;
    }
    // name: Type (Python/Rust/TS/Swift/Kotlin/Scala).
    size_t colon = end;
    foreach (i, token; t[first..end]) if (token.text == ":") { colon = first+i; break; }
    if (["py","rust","ts","js","swift","kotlin","scala"].canFind(lang)) {
        if (colon < end) {
            auto nameEnd = colon;
            if (nameEnd > first && ["?","!"].canFind(t[nameEnd-1].text)) --nameEnd;
            if (nameEnd != first+1 || !t[first].word) return null;
            return [Field(t[first].text.idup,spelling(t[colon+1..end]),t[first].line)];
        }
        if (lang != "py" && lang != "rust" && first+1 == end && t[first].word && end < t.length)
            return [Field(t[first].text.idup,"",t[first].line)];
        return null;
    }
    if (lang == "go") {
        // Named fields (including grouped names). Embedded fields have no declared name.
        if (!t[first].word || first+1 >= end) return null;
        if (t[first+1].text != "func") foreach (token; t[first..end]) if (token.text == "(" || token.text == ")") return null;
        auto afterNames = first+1;
        string[] names = [t[first].text]; int[] positions = [t[first].line];
        while (afterNames+1 < end && t[afterNames].text == "," && t[afterNames+1].word) {
            names ~= t[afterNames+1].text; positions ~= t[afterNames+1].line; afterNames += 2;
        }
        if (afterNames >= end || t[afterNames].text == ".") return null;
        if (t[afterNames].text == "[" && t[end-1].text == "]") return null; // embedded generic type
        // Go struct tags are masked by the existing cleaner.
        Field[] fields; auto type = spelling(t[afterNames..end]);
        foreach (i,name; names) fields ~= Field(name.idup,type,positions[i]);
        return fields;
    }
    // Type name; (C/C++/C#/Java/D). Keep pointers and arrays with the type.
    // A call/parameter list before the initializer is a method, not a field.
    foreach (token; t[first..end]) if (token.text == "(" || token.text == ")") return null;
    if (end <= first+1) return null;
    if (!t[first].word) return null;
    // Multi-declarator C-family statements need declarator-specific type ownership;
    // do not mislabel the last variable as a type/name pair.
    int brackets, angles;
    foreach (token; t[first..$]) {
        if (["(","[","{"].canFind(token.text)) ++brackets;
        else if ([")","]","}"].canFind(token.text) && brackets) --brackets;
        else if (!brackets && token.text == "<") ++angles;
        else if (!brackets && token.text == ">" && angles) --angles;
        else if (!brackets && !angles && token.text == ",") return null;
    }
    if (t[first].text == "struct" && end == first+2) return null;
    size_t nameEnd = end;
    foreach (i, token; t[first..end]) if (token.text == "[") { nameEnd = first+i; break; }
    size_t name = nameEnd;
    while (name > first) { --name; if (t[name].word) break; }
    if (name <= first || declarations.canFind(t[name].text)) return null;
    auto type = spelling(t[first..name]);
    if (name+1 < end) type ~= spelling(t[name+1..end]);
    return [Field(t[name].text.idup,type,t[name].line)];
}

private void pythonFields(ref FileResult result, Token[] t, int[] pairs) {
    int first;
    foreach (i; 0..t.length+1) if (i == t.length || t[i].text == "\n") {
        if (first < i) {
            auto line = t[first].line;
            int owner = -1;
            foreach (j,c; result.classes) if (c.start < line && line <= c.end && (owner < 0 || c.start > result.classes[owner].start)) owner = cast(int)j;
            if (owner >= 0) {
                bool inMethod;
                foreach (f; result.functions) if (f.start <= line && line <= f.end) { inMethod = true; break; }
                auto part = t[first..i];
                if (inMethod && part.length > 3 && part[0].text == "self" && part[1].text == ".") part = part[2..$];
                else if (inMethod) part = null;
                if (part.length) result.classes[owner].fields ~= declaration(part,"py");
            }
        }
        first = cast(int)i+1;
    }
}

private void rubyFields(ref FileResult result, Token[] t) {
    foreach (i, token; t) {
        if (!["attr_reader","attr_writer","attr_accessor"].canFind(token.text)) continue;
        if (i && [".","::","&."].canFind(t[i-1].text)) continue;
        int owner = -1;
        foreach (j,c; result.classes) if (c.start <= token.line && token.line <= c.end && (owner < 0 || c.start > result.classes[owner].start)) owner = cast(int)j;
        if (owner < 0) continue;
        bool inMethod;
        foreach (f; result.functions) if (f.start <= token.line && token.line <= f.end) { inMethod = true; break; }
        if (inMethod) continue;
        foreach (arg; t[i+1..$]) {
            if (arg.text == "\n" || arg.text == ";") break;
            if (!arg.literal) continue;
            auto name = arg.text;
            if (name.startsWith(":")) name = name[1..$];
            if (name.length >= 2 && (name[0] == '\'' && name[$-1] == '\'' || name[0] == '"' && name[$-1] == '"')) name = name[1..$-1];
            if (!identifier(name)) continue;
            bool have; foreach (f; result.classes[owner].fields) if (f.name == name) have = true;
            if (!have) result.classes[owner].fields ~= Field(name.idup,"",arg.line);
        }
    }
}

// Called only for file --struct/--all, after mode/type selection. The normal
// directory path never tokenizes for fields or retains field arrays.
void extractFields(ref FileResult result, string source) {
    auto tokens = tokensFor(source,result.lang); auto t = tokens.items; auto pairs = tokens.pairs;
    if (result.lang == "py") { pythonFields(result,t,pairs); return; }
    if (result.lang == "ruby") { rubyFields(result,t); return; }
    foreach (ref c; result.classes) {
        if (["enum","trait","protocol","type_alias","named"].canFind(c.kind)) continue;
        int name = -1, body = -1;
        foreach (i, token; t) if (token.line >= c.start && token.line <= c.end && token.text == c.name) { name = cast(int)i; break; }
        if (name < 0) continue;
        for (int i = name+1; i < t.length && t[i].line <= c.end; ++i) {
            if (["(","["].canFind(t[i].text) && pairs[i] > i) { i = pairs[i]; continue; }
            if (t[i].text == ";") break;
            if (t[i].text == "{") { if (pairs[i] > i) body = i; break; }
        }
        if (body < 0) continue; // Primary-constructor and braceless field profiles are not implemented yet.
        if (t[pairs[body]].line > c.end) continue;
        auto close = pairs[body]; int first = body+1; int angles;
        bool linesEnd = ["go","rust","ts","js","kotlin","swift","scala"].canFind(result.lang);
        for (int i = first; i < close; ++i) {
            if (i > first && linesEnd && !angles && t[i].line > t[i-1].line &&
                ![",",":","=",".","<","->"].canFind(t[i-1].text) && ![".","?",":","=",";",","].canFind(t[i].text)) {
                c.fields ~= declaration(t[first..i],result.lang); first = i;
            }
            if (t[i].text == ":" && i == first+1 && ["public","private","protected"].canFind(t[first].text)) { first = i+1; continue; }
            if (t[i].text == "<") ++angles;
            else if (t[i].text == ">" && angles) --angles;
            else if (t[i].text == ">>" && angles) angles = angles > 1 ? angles-2 : 0;
            if (["(","["].canFind(t[i].text) && pairs[i] > i) { i = pairs[i]; continue; }
            if (t[i].text == "{" && pairs[i] > i) {
                auto prefix = t[first..i];
                c.fields ~= declaration(prefix,result.lang);
                i = pairs[i]; first = i+1; angles = 0;
            } else if (t[i].text == ";" || result.lang == "rust" && t[i].text == "," && !angles) {
                c.fields ~= declaration(t[first..i],result.lang); first = i+1; angles = 0;
            }
        }
        if (first < close) c.fields ~= declaration(t[first..close],result.lang);
    }
}

unittest {
    auto r = FileResult("x.go","go"); r.classes = [Sym("C",2,7,"","struct")];
    extractFields(r,"package p\ntype C struct {\n A, B int\n Name string\n // Fake string\n Embedded\n}\n");
    assert(r.classes[0].fields == [Field("A","int",3),Field("B","int",3),Field("Name","string",4)]);
    r = FileResult("x.cpp","cpp"); r.classes = [Sym("C",1,8,"","class"),Sym("Inner",4,6,"","struct")];
    extractFields(r,"class C {\n int first, second;\n double values[Size];\n struct Inner {\n int nested;\n };\n void method() { int local; }\n};");
    assert(r.classes[0].fields == [Field("values","double[Size]",3)]);
    assert(r.classes[1].fields == [Field("nested","int",5)]);
    r = FileResult("x.py","py"); r.classes = [Sym("C",1,7,"","class"),Sym("Inner",6,7,"","class")];
    r.functions = [Sym("method",3,5,"C")];
    extractFields(r,"class C:\n x: int\n def method(self):\n  local: int\n  self.y: float\n class Inner:\n  hidden: str");
    assert(r.classes[0].fields == [Field("x","int",2),Field("y","float",5)]);
    assert(r.classes[1].fields == [Field("hidden","str",7)]);
    r = FileResult("x.rs","rust"); r.classes = [Sym("C",1,4,"","struct")];
    extractFields(r,"struct C {\n map: HashMap<String, Vec<u8>>,\n count: usize,\n}");
    assert(r.classes[0].fields == [Field("map","HashMap<String, Vec<u8>>",2),Field("count","usize",3)]);
}

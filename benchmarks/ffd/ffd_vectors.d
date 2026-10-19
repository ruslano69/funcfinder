module ffd_vectors;

// Unit test only: the D declaration and declarator scanners against what Go's patterns return for 6,000 lines
// (ts_declaration_vectors.jsonl, made by genvectors from Go's regexp, so the reference is the pattern itself).

version (unittest):

import std.file : readText;
import std.json;
import std.path : buildPath, dirName;
import std.stdio : writeln;
import typescript_header : declarationMatch, declaratorMatch;

private string[] groups(JSONValue v) {
    if (v.type == JSONType.null_) return null;
    string[] r;
    foreach (g; v.array) r ~= g.str;
    return r;
}

unittest {
    enum path = buildPath(__FILE_FULL_PATH__.dirName, "ts_declaration_vectors.jsonl");
    size_t lines, declarations, declarators;
    foreach (text; readText(path).splitter("\n")) {
        if (text.length == 0) continue;
        auto v = parseJSON(text);
        auto s = v["s"].str;
        auto wantDecl = groups(v["decl"]);
        auto wantDeclr = groups(v["declr"]);
        auto gotDecl = declarationMatch(s);
        auto gotDeclr = declaratorMatch(s);
        assert(gotDecl == wantDecl, "declaration scanner differs from Go's pattern on: " ~ s);
        assert(gotDeclr == wantDeclr, "declarator scanner differs from Go's pattern on: " ~ s);
        ++lines;
        if (wantDecl !is null) ++declarations;
        if (wantDeclr !is null) ++declarators;
    }
    assert(lines >= 6000 && declarations > 500 && declarators > 500, "the vector file is too small to prove anything");
}

import std.algorithm : splitter;

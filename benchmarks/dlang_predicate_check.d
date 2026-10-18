// Differential tests: independent std.regex predicates remain the oracle.
module dlang_predicate_check;
import dlang_port;
import std.random : Random, uniform;
import std.stdio : writeln;
import std.array : replicate;

int main(string[] args) {
    assert(args.length==2, "usage: predicate-check config.json");
    configure(args[1]);
    size_t checked;
    void check(string s) {
        assert(attribute(s)==attributeRegex(s), "attribute: " ~ s);
        assert(typeHeader(s)==typeRegex(s), "type: " ~ s);
        assert(statementHeader(s)==statementRegex(s), "statement: " ~ s);
        ++checked;
    }
    immutable words=["", "public", "private", "protected", "package", "export", "pure", "nothrow", "const", "immutable", "shared", "inout", "ref", "scope", "static", "final", "abstract", "class", "struct", "interface", "union", "enum", "template", "alias", "return", "throw", "new", "delete", "if", "else", "for", "foreach", "while", "switch", "try", "catch", "with", "assert", "version", "debug", "mixin", "pragma", "cast", "@safe", "@ж", "@١", "@", "publicprivate", "className", "staticX"];
    immutable spaces=["", " ", "\t", "\n", "\r", "\f", "\v", "\u0085", "\u00a0", "\u2003", "  \t"];
    immutable tails=["", " ", "()", "{}", ":", ";", "_", "9", "X", "ж", "١", "\u0301", "\v", "\u00a0", "\n!", "@safe", " public"];
    foreach(word;words) foreach(space;spaces) foreach(tail;tails) {
        check(space ~ word ~ tail);
        check("static" ~ space ~ word ~ tail);
        check("@safe " ~ word ~ space ~ tail);
    }
    auto rng=Random(20261018);
    foreach(n;0..300_000) {
        string s;
        foreach(k;0..uniform(1,9,rng)) {
            s~=words[uniform(0,words.length,rng)];
            s~=spaces[uniform(0,spaces.length,rng)];
            if(uniform(0,4,rng)==0) s~=tails[uniform(0,tails.length,rng)];
        }
        check(s);
    }
    check("public " ~ " ".replicate(131072));
    writeln("predicate comparisons passed: ",checked);
    return 0;
}

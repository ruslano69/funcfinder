package internal

import (
	"math/rand"
	"reflect"
	"strings"
	"testing"
)

// TestDHeaderScannerPatternIsCurrent fails when languages.json changes the D function pattern: the scanner stands
// for dFuncPatternSource only, and must be updated with it.
func TestDHeaderScannerPatternIsCurrent(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	lc, err := cfg.GetLanguageConfig("d")
	if err != nil {
		t.Fatal(err)
	}
	if lc.FuncPattern != dFuncPatternSource || !dUsesHeaderScanner(lc) {
		t.Fatal("the D func_pattern in languages.json differs from dFuncPatternSource: update dHeaderSubmatch and the constant together")
	}
}

// dHeaderTokens are the pieces random lines are built from: the pattern's words and punctuation, near misses, and
// characters at the edges of its classes.
var dHeaderTokens = []string{
	" ", "  ", "\t", "\f", "\r", "\v", "\n",
	"public", "private", "protected", "package", "export", "static", "final", "override", "abstract", "synchronized",
	"pure", "nothrow", "scope", "ref", "const", "immutable", "shared", "inout", "auto", "typeof", "delegate", "function",
	"this", "~this", "~", "new", "@property", "@safe", "@nogc", "@", "@uda(1)", "@uda((a))", "property",
	"int", "void", "T", "foo", "bar", "x1", "_", "9", "é", "ж", "١", "\x80", "�",
	"(", ")", "()", "(int a)", "((a))", "(((a)))", "(a, (b))", "[", "]", "[]", "[5]", "[K]", "*", " *", "**", "!", "!(T)",
	"!T", "!(", ".", ".foo", "a.b", "=", ";", "{", "}", ",", "pureX", "nothrowY", "publicZ",
}

func randomDHeaderLine(rng *rand.Rand) string {
	var b strings.Builder
	for n := 1 + rng.Intn(10); n > 0; n-- {
		b.WriteString(dHeaderTokens[rng.Intn(len(dHeaderTokens))])
		if rng.Intn(3) == 0 {
			b.WriteByte(' ')
		}
	}
	return b.String()
}

// TestDHeaderScannerMatchesPattern compares dHeaderSubmatch with FindStringSubmatch of the D function pattern: the
// same nil, the same match and the same three groups, on every D fixture line (raw and prepared), every join of up
// to five prepared lines the header pass may build, and generated lines.
func TestDHeaderScannerMatchesPattern(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	lc, err := cfg.GetLanguageConfig("d")
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, file := range dFixtureLines(t) {
		prepared, _ := prepareDLines(file, lc)
		lines = append(lines, file...)
		lines = append(lines, prepared...)
		for i := range prepared {
			for j := i + 1; j < len(prepared) && j <= i+4; j++ {
				lines = append(lines, strings.Join(prepared[i:j+1], " "))
			}
		}
	}
	rng := rand.New(rand.NewSource(1))
	for k := 0; k < 300000; k++ {
		lines = append(lines, randomDHeaderLine(rng))
	}
	for k := 0; k < 200000; k++ {
		lines = append(lines, structuredDHeader(rng))
	}
	re := lc.FuncRegex()
	matched := map[int]int{}
	for _, line := range lines {
		want := re.FindStringSubmatch(line)
		got := dHeaderSubmatch(line)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("dHeaderSubmatch(%q) = %q, pattern gives %q", line, got, want)
		}
		if want != nil {
			for g := 1; g <= 3; g++ {
				if want[g] != "" {
					matched[g]++
				}
			}
		}
	}
	for g := 1; g <= 3; g++ {
		if matched[g] == 0 {
			t.Errorf("no line matched through group %d: the corpus does not exercise that alternative", g)
		}
	}
}

// dPick returns one of xs at random.
func dPick(r *rand.Rand, xs ...string) string { return xs[r.Intn(len(xs))] }

// structuredDHeader builds a line shaped like a D header, with deliberate near misses at every part.
func structuredDHeader(r *rand.Rand) string {
	var b strings.Builder
	b.WriteString(dPick(r, "", " ", "\t", "  "))
	for n := r.Intn(4); n > 0; n-- {
		b.WriteString(dPick(r, "public", "static", "const", "auto", "ref", "@safe", "@uda(1)", "@uda((x))", "@property", "pure", "nothrow", "shared", "scope", "final", "publicX", "@"))
		b.WriteString(dPick(r, " ", "  ", "\t", "", " "))
	}
	switch r.Intn(10) {
	case 0:
		b.WriteString(dPick(r, "this", "~this", "new", "~ this", "thisX"))
	default:
		b.WriteString(dPick(r, "int", "void", "T", "const(char)", "immutable(T)", "typeof(x)", "inout (T)", "const", "auto", "foo.Bar", ".Bar", "Foo!(T)", "Foo!T", "Foo!", "a.b.c", "const(", "x!(a(b))", "static"))
		for n := r.Intn(3); n > 0; n-- {
			b.WriteString(dPick(r, "*", " *", "[]", "[5]", "[K]", " []", "["))
		}
		b.WriteString(dPick(r, " ", "*", " * ", "**", "", "  ", "\t", " *x"))
		if r.Intn(4) == 0 {
			b.WriteString(dPick(r, "delegate()", "function(int)", "delegate(a(b))", "delegate", "function((x))"))
			for n := r.Intn(3); n > 0; n-- {
				b.WriteString(dPick(r, " pure", " nothrow", " @safe", " @", " pureX", ""))
			}
			b.WriteString(dPick(r, " ", "", "  "))
		}
		b.WriteString(dPick(r, "foo", "bar", "pure", "nothrow", "x9", "é", "", "opCall", "@safe"))
	}
	b.WriteString(dPick(r, "(", " (", "  (", "", "()", "(int a);", "(int a) {", " = 3;", "!(T)("))
	return b.String()
}

// Command genvectors writes ts_declaration_vectors.jsonl: lines with what the Go TypeScript declaration and declarator
// patterns return for them. The D scanners (benchmarks/typescript_header.d) are checked against it by a unit test, so the
// reference is Go's regexp, not another scanner. Run it after changing the patterns or the generators:
//
//	go run ./benchmarks/ffd/genvectors > benchmarks/ffd/ts_declaration_vectors.jsonl
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"strings"

	ff "github.com/ruslano69/funcfinder/internal"
)

var declarationTokens = []string{
	" ", "  ", "\t", "\f", "\r", "\n",
	"export", "default", "declare", "public", "protected", "private", "static", "readonly", "override", "async",
	"function", "abstract", "get", "set", "new",
	"foo", "bar", "x1", "_", "9", "é", "ж", "$", "#",
	"?", "(", ")", "()", "(a: T)", "((a))", "(((a)))", "(a, (b), c)", "<", ">", "<T>", "<T, U>", "<T extends (A)>", "=>", "=",
	":", ": T", ": void", ";", "; ", ";;", " ;", "{", "}", ",", ".", "[", "]", "\"", "'",
}

var declaratorTokens = []string{
	" ", "  ", "\t", "\f", "\r", "\n",
	"async", "foo", "x1", "_", "é", "$",
	"=", "==", "=>", ":", ": T", ": (a) => T", "<", ">", "<T>", "(", ")", "()", ",", ";", "{", "}", "[", "]", "?", ".", "\"",
}

func pick(r *rand.Rand, options ...string) string { return options[r.Intn(len(options))] }

func randomLine(r *rand.Rand, tokens []string, max int) string {
	var b strings.Builder
	for n := 1 + r.Intn(max); n > 0; n-- {
		b.WriteString(tokens[r.Intn(len(tokens))])
		if r.Intn(3) == 0 {
			b.WriteByte(' ')
		}
	}
	return b.String()
}

func structuredDeclaration(r *rand.Rand) string {
	var b strings.Builder
	b.WriteString(pick(r, "", " ", "\t", "  ", "\n"))
	for n := r.Intn(4); n > 0; n-- {
		b.WriteString(pick(r, "export", "default", "declare", "public", "protected", "private", "static", "readonly",
			"override", "async", "function", "publicx", "abstract"))
		b.WriteString(pick(r, " ", "  ", "\t", "", "\n"))
	}
	if r.Intn(3) == 0 {
		b.WriteString("abstract" + pick(r, " ", "", "  ", "\t"))
		if r.Intn(2) == 0 {
			b.WriteString("async" + pick(r, " ", "", "  "))
		}
	}
	b.WriteString(pick(r, "foo", "get", "async", "abstract", "static", "é", "$x", "bar9", "function", ""))
	b.WriteString(pick(r, "", "?", "??", " ", " ?"))
	b.WriteString(pick(r, "", " ", "<T>", " <T>", "<T, U>", "<T extends (A)>", "<T", "<(T)>", "<T> ", "<T>>", "<<T>", "<T>x>"))
	b.WriteString(pick(r, "(", " (", "(", "", "x", "\t("))
	b.WriteString(pick(r, ")", "a)", "a: T)", "(a))", "(a)", "(a, (b))", "((a))", "(((a)))", "a, b)", "a: (b, c))", "(a", "a) b)"))
	b.WriteString(pick(r, "", " ", ": void", " : T", ":", ": ", ": T;", ":;", ": a; b", ": {", ": T ", " \n: T", ": a\nb"))
	b.WriteString(pick(r, ";", " ;", "; ", ";  ", ";\n", "", ";;", "}", "; // c", ";\t\n"))
	return b.String()
}

func structuredDeclarator(r *rand.Rand) string {
	var b strings.Builder
	b.WriteString(pick(r, "", "", " ", "\t", "  "))
	b.WriteString(pick(r, "f", "handler", "é", "async", "x$", "", "9"))
	b.WriteString(pick(r, "", " ", "  "))
	if r.Intn(2) == 0 {
		b.WriteString(pick(r, ":", ": ", ":  "))
		for n := r.Intn(4); n > 0; n-- {
			b.WriteString(pick(r, "T", "(a: A) => B", "=>", " ", "==", "<T>", "{ a: string }", "=", "é", "\n"))
		}
	}
	b.WriteString(pick(r, "=", " = ", "= ", "==", "", "=>"))
	b.WriteString(pick(r, "", "async", "async ", " async", "asyncx", "async\t"))
	b.WriteString(pick(r, "(", "<", " (", "x", "", "()", "[", "{"))
	b.WriteString(pick(r, "", " ,", ") => 1", ")", " b = 2"))
	return b.String()
}

type vector struct {
	S    string   `json:"s"`
	Decl []string `json:"decl"`
	Decr []string `json:"declr"`
}

// generate returns the vectors: about 3,000 lines that match something and 3,000 that do not, in generation order.
func generate() ([]byte, error) {
	cfg, err := ff.LoadConfig()
	if err != nil {
		return nil, err
	}
	lc, err := cfg.GetLanguageConfig("ts")
	if err != nil {
		return nil, err
	}
	decl, declr := lc.DeclarationRegex(), lc.DeclaratorRegex()
	r := rand.New(rand.NewSource(1))
	var out bytes.Buffer
	matching, other := 0, 0
	for matching < 3000 || other < 3000 {
		var s string
		switch r.Intn(4) {
		case 0:
			s = randomLine(r, declarationTokens, 12)
		case 1:
			s = structuredDeclaration(r)
		case 2:
			s = randomLine(r, declaratorTokens, 10)
		default:
			s = structuredDeclarator(r)
		}
		v := vector{S: s, Decl: decl.FindStringSubmatch(s), Decr: declr.FindStringSubmatch(s)}
		hit := v.Decl != nil || v.Decr != nil
		if hit && matching >= 3000 || !hit && other >= 3000 {
			continue
		}
		if hit {
			matching++
		} else {
			other++
		}
		line, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		out.Write(line)
		out.WriteByte('\n')
	}
	return out.Bytes(), nil
}

func main() {
	b, err := generate()
	if err != nil {
		fmt.Fprintln(os.Stderr, "genvectors:", err)
		os.Exit(1)
	}
	os.Stdout.Write(b)
}

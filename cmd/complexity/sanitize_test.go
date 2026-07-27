package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/ruslano69/funcfinder/internal"
)

// Комментарные маркеры внутри строкового литерала не должны считаться
// комментариями. Раньше строка резалась по первому "//" или "#" без учёта
// языка и контекста, вместе с закрывающей скобкой — и каждая такая строка
// навсегда добавляла +1 к глубине вложенности.
func TestLiteralsWithCommentMarkersDoNotInflateDepth(t *testing.T) {
	const rows = 6

	cases := []struct {
		name string
		cell string
	}{
		{"plain", `"col1"`},
		{"hash", `"col#1"`},
		{"url", `"http://example.com"`},
		{"xml char ref", `"&#xD;"`},
		{"excel error", `"#N/A"`},
		{"block comment start", `"/* not a comment"`},
		{"hash alone", `"#"`},
		{"slashes alone", `"//"`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			depth := depthOfTable(t, tc.cell, rows)
			if depth != 1 {
				t.Errorf("depth = %d, want 1 — %s in a string literal must not nest", depth, tc.cell)
			}
		})
	}
}

// Контроль: настоящая вложенность обязана считаться по-прежнему.
func TestRealNestingStillCounted(t *testing.T) {
	src := `package p

func f(xs [][]int) int {
	n := 0
	for _, row := range xs {
		for _, v := range row {
			if v > 0 {
				n++
			}
		}
	}
	return n
}
`
	if got := depthOfSource(t, src, "f"); got != 4 {
		t.Errorf("depth = %d, want 4 (for/for/if + body)", got)
	}
}

// Скобки внутри комментариев — как однострочных, так и блочных — не считаются.
func TestBracesInCommentsIgnored(t *testing.T) {
	src := `package p

func f() int {
	n := 0 // brace here: {
	/* and here: { { {
	   still a comment */
	return n
}
`
	if got := depthOfSource(t, src, "f"); got != 1 {
		t.Errorf("depth = %d, want 1", got)
	}
}

// Многострочный raw-литерал в бэктиках не должен ни нести скобки, ни
// обрываться на "//" внутри себя.
func TestRawStringSpanningLines(t *testing.T) {
	src := "package p\n\nfunc f() string {\n\ts := `\nline { with brace\nhttp://example.com\n{{{\n`\n\treturn s\n}\n"
	if got := depthOfSource(t, src, "f"); got != 1 {
		t.Errorf("depth = %d, want 1", got)
	}
}

// ─── helpers ────────────────────────────────────────────────────────────────

func depthOfTable(t *testing.T, cell string, rows int) int {
	t.Helper()

	src := "package p\n\nfunc f() {\n\tcases := []struct{ a, b string }{\n"
	for i := 0; i < rows; i++ {
		src += fmt.Sprintf("\t\t{%s, \"x\"},\n", cell)
	}
	src += "\t}\n\t_ = cases\n}\n"

	return depthOfSource(t, src, "f")
}

func depthOfSource(t *testing.T, src, funcName string) int {
	t.Helper()

	path := filepath.Join(t.TempDir(), "probe.go")
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := internal.LoadConfig()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	langConfig, err := cfg.GetLanguageConfig("go")
	if err != nil {
		t.Fatalf("go language config: %v", err)
	}

	fc := analyzeFileComplexity(path, langConfig)
	for _, fn := range fc.Functions {
		if fn.Name == funcName {
			return fn.MaxNestingDepth
		}
	}

	t.Fatalf("function %q not found in %d analyzed function(s)", funcName, len(fc.Functions))
	return 0
}

// cleanBody должен гасить содержимое литерала, сохраняя позиции — на этом
// держится и подсчёт скобок, и то, что ключевое слово внутри строки не
// принимается за управляющую конструкцию.
func TestCleanBodyBlanksLiteralsInPlace(t *testing.T) {
	cfg, err := internal.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	langConfig, err := cfg.GetLanguageConfig("go")
	if err != nil {
		t.Fatal(err)
	}

	in := []string{`	s := "if x { y }"`}
	out := cleanBody(in, internal.NewSanitizer(langConfig, false))

	if len(out) != 1 {
		t.Fatalf("expected 1 line, got %d", len(out))
	}
	if len(out[0]) != len(in[0]) {
		t.Errorf("length changed: %d -> %d; literals must be blanked, not removed", len(in[0]), len(out[0]))
	}
	for _, bad := range []string{"{", "}"} {
		if got := regexp.MustCompile(regexp.QuoteMeta(bad)).FindString(out[0]); got != "" {
			t.Errorf("brace %q from inside a literal survived: %q", bad, out[0])
		}
	}
}

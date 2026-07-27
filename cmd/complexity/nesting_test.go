package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ruslano69/funcfinder/internal"
)

// ─── скобочные языки ────────────────────────────────────────────────────────

// Сбалансированная строка не должна уводить глубину вниз. Раньше все
// закрывающие вычитались до того, как учитывались открывающие, поэтому
// `X{Y{{...}}}` и `} else {` давали отрицательный вклад.
func TestBalancedLineDoesNotLowerDepth(t *testing.T) {
	src := `package p

func f() {
	d := &Cfg{Items: []Item{{Name: "a"}}}
	if d != nil {
		if len(d.Items) > 0 {
			_ = d
		}
	}
}
`
	if got := depthOfGo(t, src, "f"); got != 3 {
		t.Errorf("depth = %d, want 3 (func/if/if) — a balanced literal line must not consume depth", got)
	}
}

func TestElseKeepsDepth(t *testing.T) {
	src := `package p

func f(x int) int {
	if x > 0 {
		return 1
	} else {
		return 2
	}
}
`
	if got := depthOfGo(t, src, "f"); got != 2 {
		t.Errorf("depth = %d, want 2 — \"} else {\" is the same level, not one less", got)
	}
}

// Скобка, открытая и закрытая в пределах строки, ничего не охватывает и
// уровня не добавляет.
func TestSingleLineLiteralAddsNoDepth(t *testing.T) {
	withLiteral := `package p

func f() {
	m := map[string]int{"a": 1}
	_ = m
}
`
	plain := `package p

func f() {
	m := 1
	_ = m
}
`
	a, b := depthOfGo(t, withLiteral, "f"), depthOfGo(t, plain, "f")
	if a != b {
		t.Errorf("one-line literal changed depth: %d vs %d", a, b)
	}
}

// А многострочный — добавляет, потому что охватывает строки.
func TestMultiLineLiteralAddsDepth(t *testing.T) {
	src := `package p

func f() {
	m := map[string]int{
		"a": 1,
	}
	_ = m
}
`
	if got := depthOfGo(t, src, "f"); got != 2 {
		t.Errorf("depth = %d, want 2", got)
	}
}

// ─── Python: отступы ────────────────────────────────────────────────────────

// Последовательные конструкции на одном уровне не накапливаются. Раньше
// накапливались: Python шёл по скобочной ветке, где инкремент был, а
// декремента не было вовсе.
func TestPythonSequentialBlocksDoNotAccumulate(t *testing.T) {
	src := `def flat(xs):
    a = 0
    if xs:
        a += 1
    if xs:
        a += 1
    if xs:
        a += 1
    return a
`
	if got := depthOfPy(t, src, "flat"); got != 2 {
		t.Errorf("depth = %d, want 2 — sequential ifs share a level", got)
	}
}

func TestPythonNestedBlocks(t *testing.T) {
	src := `def nested(xs):
    n = 0
    for row in xs:
        for v in row:
            if v > 0:
                if v % 2 == 0:
                    n += 1
    return n
`
	if got := depthOfPy(t, src, "nested"); got != 5 {
		t.Errorf("depth = %d, want 5 (for/for/if/if + body)", got)
	}
}

// Отступ продолжения внутри скобок — выравнивание, а не вложенность.
func TestPythonContinuationLinesIgnored(t *testing.T) {
	src := `def f(xs):
    total = sum(xs,
                start=0,
                )
    return total
`
	if got := depthOfPy(t, src, "f"); got != 1 {
		t.Errorf("depth = %d, want 1 — a wrapped call argument list is not nesting", got)
	}
}

// Ширина отступа не должна быть зашита: 2 пробела дают ту же глубину, что и 4.
func TestPythonIndentWidthAgnostic(t *testing.T) {
	four := `def f(xs):
    for x in xs:
        if x:
            return x
    return None
`
	two := `def f(xs):
  for x in xs:
    if x:
      return x
  return None
`
	a, b := depthOfPy(t, four, "f"), depthOfPy(t, two, "f")
	if a != b {
		t.Errorf("indent width changed the result: 4-space %d, 2-space %d", a, b)
	}
	if a != 3 {
		t.Errorf("depth = %d, want 3 (for/if + body)", a)
	}
}

func TestIndentWidthExpandsTabs(t *testing.T) {
	cases := []struct {
		line string
		want int
	}{
		{"code", 0},
		{"    code", 4},
		{"\tcode", 8},
		{"\t\tcode", 16},
		{"    \tcode", 8}, // 4 пробела, затем таб до следующей границы 8
		{"        code", 8},
	}

	for _, tc := range cases {
		if got := indentWidth(tc.line); got != tc.want {
			t.Errorf("indentWidth(%q) = %d, want %d", tc.line, got, tc.want)
		}
	}
}

// ─── end-delimited языки (Ruby) ─────────────────────────────────────────────

// Вызывается напрямую, минуя finder: тот для Ruby ищет тело в фигурных
// скобках, не находит и не возвращает ни одной функции (см. комментарий у
// nestingByBlockKeyword). Ветка от этого не перестаёт быть нужной — без неё
// Ruby попадёт в скобочную и снова начнёт расти без остановки.
func TestBlockKeywordDepth(t *testing.T) {
	nestingRe := getNestingPattern("ruby")
	flatRe := getFlatPattern("ruby")

	flat := []string{
		"def flat(xs)", "  a = 0",
		"  if xs", "    a += 1", "  end",
		"  if xs", "    a += 1", "  end",
		"  a", "end",
	}
	nested := []string{
		"def nested(xs)", "  n = 0",
		"  if xs", "    if xs.any?", "      n += 1", "    end", "  end",
		"  n", "end",
	}

	flatDepth := nestingByBlockKeyword(flat, "end", nestingRe, flatRe).maxDepth
	nestedDepth := nestingByBlockKeyword(nested, "end", nestingRe, flatRe).maxDepth

	if nestedDepth <= flatDepth {
		t.Errorf("nested (%d) must measure deeper than sequential (%d) — "+
			"without an \"end\" decrement the two are indistinguishable",
			nestedDepth, flatDepth)
	}
}

// "end" внутри строкового литерала — не закрытие блока. Литералы гасит
// cleanBody, так что до счётчика доходят только настоящие ключевые слова.
func TestBlockKeywordIgnoresEndInLiteral(t *testing.T) {
	cfg, err := internal.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	langConfig, err := cfg.GetLanguageConfig("ruby")
	if err != nil {
		t.Skipf("ruby config unavailable: %v", err)
	}

	raw := []string{"def f", `  s = "end end end"`, "  s", "end"}
	cleaned := cleanBody(raw, internal.NewSanitizer(langConfig, false))

	for _, line := range cleaned {
		if strings.Contains(line, "end end") {
			t.Errorf("literal survived sanitizing: %q", line)
		}
	}
}

// ─── helpers ────────────────────────────────────────────────────────────────

func depthOfGo(t *testing.T, src, name string) int {
	t.Helper()
	return depthOf(t, src, name, "go", "probe.go")
}

func depthOfPy(t *testing.T, src, name string) int {
	t.Helper()
	return depthOf(t, src, name, "py", "probe.py")
}

func depthOf(t *testing.T, src, name, lang, filename string) int {
	t.Helper()

	path := filepath.Join(t.TempDir(), filename)
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := internal.LoadConfig()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	langConfig, err := cfg.GetLanguageConfig(lang)
	if err != nil {
		t.Fatalf("%s language config: %v", lang, err)
	}

	fc := analyzeFileComplexity(path, langConfig)
	for _, fn := range fc.Functions {
		if fn.Name == name {
			return fn.MaxNestingDepth
		}
	}

	t.Fatalf("function %q not found among %d analyzed", name, len(fc.Functions))
	return 0
}

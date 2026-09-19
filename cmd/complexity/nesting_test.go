package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ruslano69/funcfinder/internal"
)

// ─── скобочные языки ────────────────────────────────────────────────────────

// С этого файла глубина считает только точки решения (if/for/switch/...),
// а не любую скобку/отступ — своя же скобка функции (или def/class) ничего
// не решает и в счёт не идёт. Раньше это давало +1 к базовой глубине даже
// в пустой функции без единого if; в ожидаемых числах ниже этой единицы
// больше нет.

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
	if got := depthOfGo(t, src, "f"); got != 2 {
		t.Errorf("depth = %d, want 2 (if/if) — a balanced literal line must not consume depth", got)
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
	if got := depthOfGo(t, src, "f"); got != 1 {
		t.Errorf("depth = %d, want 1 — \"} else {\" is the same level as the if, not one less", got)
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

// Ни многострочный литерал тоже не добавляет — map/struct-литерал не точка
// решения независимо от того, на скольких строках он записан. (Раньше
// строка литерала здесь давала +1 просто за то, что охватывает содержимое;
// теперь охват сам по себе ничего не значит без if/for/switch внутри.)
func TestMultiLineLiteralAddsNoDepth(t *testing.T) {
	src := `package p

func f() {
	m := map[string]int{
		"a": 1,
	}
	_ = m
}
`
	if got := depthOfGo(t, src, "f"); got != 0 {
		t.Errorf("depth = %d, want 0", got)
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
	if got := depthOfPy(t, src, "flat"); got != 1 {
		t.Errorf("depth = %d, want 1 — sequential ifs share a level, not 3", got)
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
	if got := depthOfPy(t, src, "nested"); got != 4 {
		t.Errorf("depth = %d, want 4 (for/for/if/if)", got)
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
	if got := depthOfPy(t, src, "f"); got != 0 {
		t.Errorf("depth = %d, want 0 — a wrapped call argument list is not nesting", got)
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
	if a != 2 {
		t.Errorf("depth = %d, want 2 (for/if)", a)
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
	cfg, err := internal.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	langConfig, err := cfg.GetLanguageConfig("ruby")
	if err != nil {
		t.Skipf("ruby config unavailable: %v", err)
	}

	nestingRe := getNestingPattern(langConfig)
	flatRe := getFlatPattern(langConfig)

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

// ─── loop-nesting signal ────────────────────────────────────────────────────

// Loop nested in loop is a distinct risk (O(n^2)+, iteration-interaction
// bugs) that if-in-loop simply doesn't share — MaxLoopNestingDepth tracks it
// separately from the general branching depth above.
func TestLoopInLoopFlagged(t *testing.T) {
	src := `package p

func matrixSum(m [][]int) int {
	total := 0
	for i := range m {
		for j := range m[i] {
			total += m[i][j]
		}
	}
	return total
}
`
	if got := loopDepthOfGo(t, src, "matrixSum"); got != 2 {
		t.Errorf("MaxLoopNestingDepth = %d, want 2 (for/for)", got)
	}
}

// An `if` nested inside a loop is the ordinary, safe filter-in-a-loop
// pattern — it must NOT trip the loop-nesting signal just for containing a
// loop at all.
func TestIfInLoopNotFlaggedAsLoopNesting(t *testing.T) {
	src := `package p

func filterPositive(xs []int) []int {
	var out []int
	for _, x := range xs {
		if x > 0 {
			out = append(out, x)
		}
	}
	return out
}
`
	if got := loopDepthOfGo(t, src, "filterPositive"); got != 1 {
		t.Errorf("MaxLoopNestingDepth = %d, want 1 (one loop, the if inside it doesn't count)", got)
	}
}

// A bare call to a function whose name starts with a reserved word
// ("forEach", not "for each") must not be mistaken for opening a loop, even
// when — as here — it has its own trailing "{" from a callback literal on
// the same line, which is exactly the shape that turned the false match
// into a real, visible +1 in the reported bug.
func TestKeywordPrefixedCallNotCountedAsNesting(t *testing.T) {
	src := `package p

func caller(items []int) {
	forEach(items, func(x int) {
		println(x)
	})
}
`
	if got := depthOfGo(t, src, "caller"); got != 0 {
		t.Errorf("depth = %d, want 0 — forEach(...) is a call, not a for loop", got)
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

// loopDepthOfGo mirrors depthOfGo but returns MaxLoopNestingDepth — the
// signal that flags loop-in-loop specifically, independent of general
// branching depth.
func loopDepthOfGo(t *testing.T, src, name string) int {
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
		if fn.Name == name {
			return fn.MaxLoopNestingDepth
		}
	}

	t.Fatalf("function %q not found among %d analyzed", name, len(fc.Functions))
	return 0
}

package internal

import (
	"regexp"
	"strings"
	"testing"
)

var benchLines = []string{
	"func (s *Server) HandleRequest(w http.ResponseWriter, r *http.Request) error {",
	"func computeShardChecksum(paths []string) string {",
	"   x := foo(bar(baz(1, 2, 3)))",
	"type Manifest struct {",
	"func Привет(имя string) (результат int) {",
	"\tСтарт(); process(); validate()",
}

// Old ASCII (\w) vs new Unicode ([\p{L}\p{Nd}_]) name capture in func_pattern.
var reASCIIFunc = regexp.MustCompile(`^\s*func\s+(\([^)]*\)\s+)?(\w+)\s*\(`)
var reUnicodeFunc = regexp.MustCompile(`^\s*func\s+(\([^)]*\)\s+)?([\p{L}\p{Nd}_]+)\s*\(`)

func BenchmarkFuncPattern_ASCII(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for _, l := range benchLines {
			_ = reASCIIFunc.FindStringSubmatch(l)
		}
	}
}

func BenchmarkFuncPattern_Unicode(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for _, l := range benchLines {
			_ = reUnicodeFunc.FindStringSubmatch(l)
		}
	}
}

// Old ASCII call regex (with \b) vs the new Unicode-aware one.
var reASCIICall = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z_][A-Za-z0-9_]*)\s*\(|\b([A-Za-z_][A-Za-z0-9_]*)\s*\(`)

func BenchmarkCallPattern_ASCII(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for _, l := range benchLines {
			_ = reASCIICall.FindAllStringSubmatch(l, -1)
		}
	}
}

func BenchmarkCallPattern_Unicode(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for _, l := range benchLines {
			_ = callIdentRe.FindAllStringSubmatch(l, -1)
		}
	}
}

// End-to-end: FindFunctions over a synthetic file (uses the live Unicode patterns).
func BenchmarkFindFunctions_EndToEnd(b *testing.B) {
	cfg, _ := LoadConfig()
	goCfg := cfg["go"]
	var sb strings.Builder
	for i := 0; i < 200; i++ {
		sb.WriteString("func Handler(x int) error {\n\treturn process(x)\n}\n\n")
	}
	lines := strings.Split(sb.String(), "\n")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f := NewFinder(goCfg, nil, true, false, false)
		_, _ = f.FindFunctionsInLines(lines, 1, "bench.go")
	}
}

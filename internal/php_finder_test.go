package internal

import (
	"os"
	"strings"
	"testing"
)

func TestPHPFinder_ModifiersReferencesAndNestedMethods(t *testing.T) {
	config, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	content := strings.Join([]string{
		"<?php", "abstract class A {",
		"    abstract protected function write(int $v): void;",
		"    final public static function &get(): mixed { return $this->value; }", "}",
		"function outer() {", "    $obj = new class {",
		"        final protected function nested(): void {}", "    };", "}",
	}, "\n")
	path := createTempFile(t, content, "php_discovery_*.php")
	t.Cleanup(func() { os.Remove(path) })
	result, err := NewFinder(config["php"], nil, true, true, false).FindFunctions(path)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][2]int{"write": {3, 3}, "get": {4, 4}, "outer": {6, 10}, "nested": {8, 8}}
	if len(result.Functions) != len(want) {
		t.Fatalf("functions = %v, want four declarations", result.Functions)
	}
	for _, fn := range result.Functions {
		bounds, ok := want[fn.Name]
		if !ok || fn.Start != bounds[0] || fn.End != bounds[1] {
			t.Errorf("unexpected bounds: %+v", fn)
		}
	}
}

func TestPHPFinder_HeredocPreservesCodeAndExtraction(t *testing.T) {
	config, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	for _, label := range []string{"TEXT", "'TEXT'", `"TEXT"`} {
		t.Run(label, func(t *testing.T) {
			content := strings.Join([]string{
				"<?php", "function first() {", "    $text = call(<<<" + label,
				"        function bogus() {", "        can't parse these quotes: ' \" }", "        TEXTsuffix {",
				"        TEXT); $other = [1, 2];", "    return $text;", "}",
				"function second() {}",
			}, "\n")
			path := createTempFile(t, content, "php_heredoc_*.php")
			t.Cleanup(func() { os.Remove(path) })
			finder := NewFinder(config["php"], nil, true, true, false)
			result, err := finder.FindFunctions(path)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Functions) != 2 {
				t.Fatalf("functions = %v, want first and second", result.Functions)
			}
			first := result.Functions[0]
			if first.Name != "first" || first.Start != 2 || first.End != 9 {
				t.Fatalf("first bounds = %+v, want 2-9", first)
			}
			if strings.Join(first.Lines, "\n") != strings.Join(strings.Split(content, "\n")[1:9], "\n") {
				t.Error("extraction did not preserve the heredoc source")
			}
			// The same preprocessing must retain slice offsets for --lines callers.
			result, err = finder.FindFunctionsInLines(strings.Split(content, "\n")[1:], 2, path)
			if err != nil || len(result.Functions) != 2 || result.Functions[0].Start != 2 || result.Functions[0].End != 9 {
				t.Fatalf("offset scan = %v, %v", result, err)
			}
		})
	}
}

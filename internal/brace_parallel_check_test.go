package internal

import (
	"encoding/json"
	"os"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestBraceParallelOrderingExtractionAndBudget(t *testing.T) {
	config, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	for _, language := range []string{"cs", "java", "d"} {
		text := `class Owner {
    Owner(int value) { value++; }
    class Inner {
        int Get() { return 1; }
    }
    int A(int n) { if (n > 0) { n--; } return n; }
    int B(int n) { if (n > 0) { n--; } return n; }
    int C(int n) { if (n > 0) { n--; } return n; }
}`
		if language == "d" {
			text = strings.Replace(text, "Owner(int", "this(int", 1)
		}
		lines := strings.Split(text, "\n")
		for _, selected := range []bool{false, true} {
			var names []string
			if selected {
				names = []string{"Get", "A"}
			}
			serial := NewFinder(config[language], names, !selected, true, false)
			want, err := serial.findBraceFunctionsRecursive(lines, 71, "ordering."+language)
			if err != nil || len(want.Functions) < 2 {
				t.Fatalf("%s: unusable serial reference: %+v, %v", language, want, err)
			}
			for _, workers := range []int{1, 2, 4, 32} {
				for repeat := 0; repeat < 20; repeat++ {
					executor := newBraceScopeExecutor(workers, 0)
					finder := NewFinder(config[language], names, !selected, true, false)
					got, err := finder.findBraceFunctionsParallelWith(lines, 71, "ordering."+language, executor)
					if err != nil || !reflect.DeepEqual(got, want) {
						t.Fatalf("%s workers=%d selected=%v: parallel differs, error=%v\ngot=%+v\nwant=%+v", language, workers, selected, err, got, want)
					}
					if workers > 1 && executor.started.Load() == 0 {
						t.Fatalf("%s: no tasks were forked", language)
					}
					if len(executor.slots) != 0 {
						t.Fatalf("%s: leaked task permits", language)
					}
				}
			}
		}
	}
}

func TestBraceParallelCorpusParity(t *testing.T) {
	var projects []struct{ Language, Root string }
	if value := os.Getenv("BRACE_PARALLEL_PROJECTS"); value != "" {
		if err := json.Unmarshal([]byte(value), &projects); err != nil {
			t.Fatal(err)
		}
	}
	if len(projects) == 0 {
		t.Skip("set BRACE_PARALLEL_PROJECTS to JSON language/root pairs")
	}
	config, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	executor := newBraceScopeExecutor(runtime.NumCPU(), 2048)
	count := 0
	for _, project := range projects {
		before := executor.started.Load()
		pattern := map[string]string{"cs": "*.cs", "java": "*.java", "d": "*.d"}[project.Language]
		files, _, _ := braceCompareFiles(t, project.Root, pattern)
		for _, file := range files {
			serial := NewFinder(config[project.Language], nil, true, false, false)
			want, err := serial.findBraceFunctionsRecursive(file.lines, 1, file.path)
			if err != nil {
				t.Fatal(err)
			}
			finder := NewFinder(config[project.Language], nil, true, false, false)
			got, err := finder.findBraceFunctionsParallelWith(file.lines, 1, file.path, executor)
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("%s: parallel result differs, error=%v", file.path, err)
			}
			count++
		}
		t.Logf("%s %s: %d files, %d forked scope tasks", project.Language, project.Root, len(files), executor.started.Load()-before)
	}
	t.Logf("exact ordered result parity on %d files; forked %d scope tasks", count, executor.started.Load())
}

func TestBraceParallelSharedAcrossFiles(t *testing.T) {
	config, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split("class C {\n int A() { if (true) { return 1; } return 0; }\n int B() { return 2; }\n}", "\n")
	serial := NewFinder(config["cs"], nil, true, true, false)
	want, err := serial.findBraceFunctionsRecursive(lines, 1, "shared.cs")
	if err != nil {
		t.Fatal(err)
	}
	executor := newBraceScopeExecutor(4, 0)
	ForEachParallel(64, func(i int) {
		finder := NewFinder(config["cs"], nil, true, true, false)
		got, err := finder.findBraceFunctionsParallelWith(lines, 1, "shared.cs", executor)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("file %d: shared executor changed result, error=%v", i, err)
		}
	})
	if executor.started.Load() == 0 || len(executor.slots) != 0 {
		t.Fatalf("bad shared executor state: started=%d active=%d", executor.started.Load(), len(executor.slots))
	}
}

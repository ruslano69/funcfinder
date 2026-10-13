package internal

import (
	"os"
	"strings"
	"testing"
)

// These shapes caused wrong spans on Flask, Requests and hr_portal. The
// scope-based --lines path must agree with the finder, including nested defs.
func TestPythonBounds_LogicalStatements(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		start int
		end   int
	}{
		{"continued signature indent", []string{
			"def first(arg,", "          other):", "    value = arg + other", "    return value",
			"", "def second():", "    pass",
		}, 1, 4},
		{"header comment", []string{
			"def first(): # type: ignore", "    if True:", "        return 1", "    value = 2", "    return value",
			"", "def second():", "    pass",
		}, 1, 5},
		{"inline overload", []string{
			"def first(arg): ...", "", "def second():", "    pass",
		}, 1, 1},
		{"inline string suite", []string{
			"def first(): 'a docstring' # comment", "", "def second():", "    pass",
		}, 1, 1},
		{"inline continued suite", []string{
			"def first(): return (", "1", ")", "outside = 2", "def second():", "    pass",
		}, 1, 3},
		{"nested signature colons", []string{
			"def first(", "    value={'key:': (lambda item: item)},", ") -> str: # comment",
			"    return 'value: # text'", "", "def second():", "    pass",
		}, 1, 4},
		{"docstring example and dedent", []string{
			"def first():", "    \"\"\"", "def bogus():", "    pass", "\"\"\"", "    return 1",
			"", "def second():", "    pass",
		}, 1, 6},
		{"docstring only body", []string{
			"def first():", "    '''", "unindented text", "'''", "", "def second():", "    pass",
		}, 1, 4},
		{"triple strings contain other quotes", []string{
			"def first():", "    '''", `a "quoted" example and """ are text`, "def bogus():", "    pass", "'''", "    return 1",
			"", "def second():", "    pass",
		}, 1, 7},
		{"code after triple string", []string{
			"def first():", `    execute("""`, `SQL ("quoted")`, `""", {"key": 1})`, "    return 1",
			"", "def second():", "    pass",
		}, 1, 5},
		{"implicit body continuation", []string{
			"def first():", "    value = (", "1", ")", "    return value", "", "def second():", "    pass",
		}, 1, 5},
		{"explicit body continuation", []string{
			"def first():", `    return 1 + \`, "2", "outside = 3", "def second():", "    pass",
		}, 1, 3},
		{"multiline decorator", []string{
			"@decorator(", "    'text',", ")", "def first():", "    return 1", "", "def second():", "    pass",
		}, 1, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := strings.Join(tt.lines, "\n") + "\n"
			path := createTempFile(t, content, "python_bounds_*.py")
			t.Cleanup(func() { os.Remove(path) })
			finder := NewPythonFinder(getPyConfig(t), "", "map", true)
			result, err := finder.FindFunctions(path)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Functions) != 2 {
				t.Fatalf("functions = %v, want first and second only", result.Functions)
			}
			fn := result.Functions[0]
			if fn.Name != "first" || fn.Start != tt.start || pythonTestEnd(content, fn.End) != tt.end {
				t.Errorf("first bounds = %s %d-%d, want %d-%d (ignoring trailing trivia)", fn.Name, fn.Start, fn.End, tt.start, tt.end)
			}
			if strings.Join(fn.Lines, "\n") != strings.Join(strings.Split(content, "\n")[fn.Start-1:fn.End], "\n") {
				t.Error("extraction changed the original source")
			}
			scopes, err := AnalyzePythonScopes(path)
			if err != nil {
				t.Fatal(err)
			}
			if len(scopes) != 2 {
				t.Fatalf("scopes = %v, want first and second only", scopes)
			}
			if scopes[0].Name != "first" || scopes[0].StartLine != tt.start || pythonTestEnd(content, scopes[0].EndLine) != tt.end {
				t.Errorf("first scope = %+v, want %d-%d", scopes[0], tt.start, tt.end)
			}
		})
	}
}

func TestPythonScopes_NestedParentsAndDocstrings(t *testing.T) {
	content := strings.Join([]string{
		"class Container:", "    \"\"\"", "class Fake:", "    def bogus(): ...", "\"\"\"",
		"    @marker(", "        'text',", "    )", "    def outer(self):",
		"        def inner(): return (", "            1", "        )", "        return inner()",
		"    def sibling(self): ...", "outside = 1",
	}, "\n") + "\n"
	path := createTempFile(t, content, "python_scopes_*.py")
	t.Cleanup(func() { os.Remove(path) })
	scopes, err := AnalyzePythonScopes(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(scopes) != 4 {
		t.Fatalf("scopes = %v, want Container, outer, inner, sibling", scopes)
	}
	for i, parent := range []string{"", "Container", "outer", "Container"} {
		actual := ""
		if scopes[i].Parent != nil {
			actual = scopes[i].Parent.Name
		}
		if actual != parent {
			t.Errorf("%s parent = %s, want %s", scopes[i].Name, actual, parent)
		}
	}
	for i, end := range []int{14, 13, 12, 14} {
		if pythonTestEnd(content, scopes[i].EndLine) != end {
			t.Errorf("%s end = %d, want %d", scopes[i].Name, scopes[i].EndLine, end)
		}
	}
	finder := NewPythonFinder(getPyConfig(t), "", "map", false)
	result, err := finder.FindFunctions(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Classes) != 1 || result.Classes[0].Name != "Container" || result.Classes[0].Start != 1 || pythonTestEnd(content, result.Classes[0].End) != 14 {
		t.Errorf("classes = %v, want Container at 1-14", result.Classes)
	}
	for _, fn := range result.Functions {
		if fn.ClassName != "Container" {
			t.Errorf("%s ClassName = %s, want Container", fn.Name, fn.ClassName)
		}
	}
}

func pythonTestEnd(content string, end int) int {
	lines := strings.Split(content, "\n")
	for end > 0 && (strings.TrimSpace(lines[end-1]) == "" || strings.HasPrefix(strings.TrimSpace(lines[end-1]), "#")) {
		end--
	}
	return end
}

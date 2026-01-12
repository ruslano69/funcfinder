package internal

import (
	"os"
	"testing"
)

// Helper function to get Python language config for tests
func getPyConfig(t *testing.T) *LanguageConfig {
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	pyConfig := config["py"]
	if pyConfig == nil {
		t.Fatal("Python config not found")
	}
	return pyConfig
}

func TestNewPythonFinder(t *testing.T) {
	config := getPyConfig(t)

	tests := []struct {
		name      string
		funcNames string
		mode      string
		extract   bool
	}{
		{
			name:      "map mode",
			funcNames: "",
			mode:      "map",
			extract:   false,
		},
		{
			name:      "search mode single function",
			funcNames: "test_func",
			mode:      "search",
			extract:   false,
		},
		{
			name:      "search mode multiple functions",
			funcNames: "func1,func2,func3",
			mode:      "search",
			extract:   false,
		},
		{
			name:      "extract mode",
			funcNames: "test_func",
			mode:      "search",
			extract:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			finder := NewPythonFinder(*config, tt.funcNames, tt.mode, tt.extract)

			if finder == nil {
				t.Fatal("NewPythonFinder() returned nil")
			}

			if finder.config.Name != "Python" {
				t.Errorf("config.Name = %v, want Python", finder.config.Name)
			}

			if finder.mode != tt.mode {
				t.Errorf("mode = %v, want %v", finder.mode, tt.mode)
			}

			if finder.extract != tt.extract {
				t.Errorf("extract = %v, want %v", finder.extract, tt.extract)
			}

			if finder.decoratorWindow == nil {
				t.Error("decoratorWindow is nil")
			}
		})
	}
}

func TestPythonFinder_SimpleFunctions(t *testing.T) {
	config := getPyConfig(t)

	content := `def simple_func():
    return 1

def func_with_params(x, y):
    result = x + y
    return result

def multiline_body():
    a = 1
    b = 2
    c = 3
    return a + b + c
`

	tmpfile := createTempFile(t, content, "test_simple_*.py")
	defer os.Remove(tmpfile)

	finder := NewPythonFinder(*config, "", "map", false)
	result, err := finder.FindFunctions(tmpfile)

	if err != nil {
		t.Fatalf("FindFunctions() error = %v", err)
	}

	if len(result.Functions) != 3 {
		t.Fatalf("FindFunctions() found %d functions, want 3", len(result.Functions))
	}

	// Check simple_func
	if result.Functions[0].Name != "simple_func" {
		t.Errorf("Functions[0].Name = %v, want simple_func", result.Functions[0].Name)
	}
	if result.Functions[0].Start != 1 || result.Functions[0].End != 3 {
		t.Errorf("Functions[0] bounds = %d-%d, want 1-3", result.Functions[0].Start, result.Functions[0].End)
	}

	// Check func_with_params
	if result.Functions[1].Name != "func_with_params" {
		t.Errorf("Functions[1].Name = %v, want func_with_params", result.Functions[1].Name)
	}
	if result.Functions[1].Start != 4 || result.Functions[1].End != 7 {
		t.Errorf("Functions[1] bounds = %d-%d, want 4-7", result.Functions[1].Start, result.Functions[1].End)
	}

	// Check multiline_body
	if result.Functions[2].Name != "multiline_body" {
		t.Errorf("Functions[2].Name = %v, want multiline_body", result.Functions[2].Name)
	}
	if result.Functions[2].Start != 8 || result.Functions[2].End != 13 {
		t.Errorf("Functions[2] bounds = %d-%d, want 8-13", result.Functions[2].Start, result.Functions[2].End)
	}
}

func TestPythonFinder_WithDecorators(t *testing.T) {
	config := getPyConfig(t)

	content := `@decorator
def decorated_func():
    return 1

@decorator1
@decorator2
def multi_decorated():
    return 2

@decorator(param=True)
def decorated_with_params():
    return 3
`

	tmpfile := createTempFile(t, content, "test_decorators_*.py")
	defer os.Remove(tmpfile)

	finder := NewPythonFinder(*config, "", "map", false)
	result, err := finder.FindFunctions(tmpfile)

	if err != nil {
		t.Fatalf("FindFunctions() error = %v", err)
	}

	if len(result.Functions) != 3 {
		t.Fatalf("FindFunctions() found %d functions, want 3", len(result.Functions))
	}

	// Check single decorator
	if result.Functions[0].Name != "decorated_func" {
		t.Errorf("Functions[0].Name = %v, want decorated_func", result.Functions[0].Name)
	}
	if result.Functions[0].Start != 1 {
		t.Errorf("Functions[0].Start = %d, want 1 (should include decorator)", result.Functions[0].Start)
	}
	if len(result.Functions[0].Decorators) != 1 {
		t.Errorf("Functions[0] has %d decorators, want 1", len(result.Functions[0].Decorators))
	}
	if result.Functions[0].Decorators[0] != "@decorator" {
		t.Errorf("Functions[0].Decorators[0] = %v, want @decorator", result.Functions[0].Decorators[0])
	}

	// Check multiple decorators
	if result.Functions[1].Name != "multi_decorated" {
		t.Errorf("Functions[1].Name = %v, want multi_decorated", result.Functions[1].Name)
	}
	if len(result.Functions[1].Decorators) != 2 {
		t.Errorf("Functions[1] has %d decorators, want 2", len(result.Functions[1].Decorators))
	}

	// Check decorator with params
	if result.Functions[2].Name != "decorated_with_params" {
		t.Errorf("Functions[2].Name = %v, want decorated_with_params", result.Functions[2].Name)
	}
	if len(result.Functions[2].Decorators) != 1 {
		t.Errorf("Functions[2] has %d decorators, want 1", len(result.Functions[2].Decorators))
	}
}

func TestPythonFinder_ClassMethods(t *testing.T) {
	config := getPyConfig(t)

	content := `class MyClass:
    def __init__(self):
        self.value = 0

    def method_one(self, arg):
        return arg * 2

    def method_two(self):
        return self.value
`

	tmpfile := createTempFile(t, content, "test_class_*.py")
	defer os.Remove(tmpfile)

	finder := NewPythonFinder(*config, "", "map", false)
	result, err := finder.FindFunctions(tmpfile)

	if err != nil {
		t.Fatalf("FindFunctions() error = %v", err)
	}

	if len(result.Functions) != 3 {
		t.Fatalf("FindFunctions() found %d functions, want 3", len(result.Functions))
	}

	// Check __init__
	if result.Functions[0].Name != "__init__" {
		t.Errorf("Functions[0].Name = %v, want __init__", result.Functions[0].Name)
	}

	// Check method_one
	if result.Functions[1].Name != "method_one" {
		t.Errorf("Functions[1].Name = %v, want method_one", result.Functions[1].Name)
	}

	// Check method_two
	if result.Functions[2].Name != "method_two" {
		t.Errorf("Functions[2].Name = %v, want method_two", result.Functions[2].Name)
	}
}

func TestPythonFinder_AsyncFunctions(t *testing.T) {
	config := getPyConfig(t)

	content := `async def async_func():
    return await something()

async def async_with_params(x, y):
    result = await process(x, y)
    return result
`

	tmpfile := createTempFile(t, content, "test_async_*.py")
	defer os.Remove(tmpfile)

	finder := NewPythonFinder(*config, "", "map", false)
	result, err := finder.FindFunctions(tmpfile)

	if err != nil {
		t.Fatalf("FindFunctions() error = %v", err)
	}

	if len(result.Functions) != 2 {
		t.Fatalf("FindFunctions() found %d functions, want 2", len(result.Functions))
	}

	if result.Functions[0].Name != "async_func" {
		t.Errorf("Functions[0].Name = %v, want async_func", result.Functions[0].Name)
	}

	if result.Functions[1].Name != "async_with_params" {
		t.Errorf("Functions[1].Name = %v, want async_with_params", result.Functions[1].Name)
	}
}

func TestPythonFinder_MultilineSignature(t *testing.T) {
	config := getPyConfig(t)

	content := `def multiline_func(
    arg1,
    arg2,
    arg3
):
    return arg1 + arg2 + arg3

def another_multiline(
    param1: int,
    param2: str
) -> int:
    return param1
`

	tmpfile := createTempFile(t, content, "test_multiline_*.py")
	defer os.Remove(tmpfile)

	finder := NewPythonFinder(*config, "", "map", false)
	result, err := finder.FindFunctions(tmpfile)

	if err != nil {
		t.Fatalf("FindFunctions() error = %v", err)
	}

	if len(result.Functions) != 2 {
		t.Fatalf("FindFunctions() found %d functions, want 2", len(result.Functions))
	}

	if result.Functions[0].Name != "multiline_func" {
		t.Errorf("Functions[0].Name = %v, want multiline_func", result.Functions[0].Name)
	}

	if result.Functions[1].Name != "another_multiline" {
		t.Errorf("Functions[1].Name = %v, want another_multiline", result.Functions[1].Name)
	}
}

func TestPythonFinder_ExtractMode(t *testing.T) {
	config := getPyConfig(t)

	content := `@decorator
def decorated_func(x):
    result = x * 2
    return result
`

	tmpfile := createTempFile(t, content, "test_extract_*.py")
	defer os.Remove(tmpfile)

	finder := NewPythonFinder(*config, "", "map", true)
	result, err := finder.FindFunctions(tmpfile)

	if err != nil {
		t.Fatalf("FindFunctions() error = %v", err)
	}

	if len(result.Functions) != 1 {
		t.Fatalf("FindFunctions() found %d functions, want 1", len(result.Functions))
	}

	fn := result.Functions[0]
	if fn.Name != "decorated_func" {
		t.Errorf("Function name = %v, want decorated_func", fn.Name)
	}

	if len(fn.Lines) == 0 {
		t.Fatal("Extract mode: Lines is empty")
	}

	// Check that decorator is included
	if fn.Lines[0] != "@decorator" {
		t.Errorf("Lines[0] = %v, want @decorator", fn.Lines[0])
	}

	// Check function definition
	if fn.Lines[1] != "def decorated_func(x):" {
		t.Errorf("Lines[1] = %v, want def decorated_func(x):", fn.Lines[1])
	}
}

func TestPythonFinder_FilterByName(t *testing.T) {
	config := getPyConfig(t)

	content := `def func_one():
    pass

def func_two():
    pass

def func_three():
    pass
`

	tmpfile := createTempFile(t, content, "test_filter_*.py")
	defer os.Remove(tmpfile)

	// Test filtering for specific functions
	finder := NewPythonFinder(*config, "func_one,func_three", "search", false)
	result, err := finder.FindFunctions(tmpfile)

	if err != nil {
		t.Fatalf("FindFunctions() error = %v", err)
	}

	if len(result.Functions) != 2 {
		t.Fatalf("FindFunctions() found %d functions, want 2", len(result.Functions))
	}

	if result.Functions[0].Name != "func_one" {
		t.Errorf("Functions[0].Name = %v, want func_one", result.Functions[0].Name)
	}

	if result.Functions[1].Name != "func_three" {
		t.Errorf("Functions[1].Name = %v, want func_three", result.Functions[1].Name)
	}
}

func TestPythonFinder_WithComments(t *testing.T) {
	config := getPyConfig(t)

	content := `def func_with_comments():
    # This is a comment
    x = 1
    # Another comment
    return x

# Module level comment
def another_func():
    """Docstring"""
    return 2
`

	tmpfile := createTempFile(t, content, "test_comments_*.py")
	defer os.Remove(tmpfile)

	finder := NewPythonFinder(*config, "", "map", false)
	result, err := finder.FindFunctions(tmpfile)

	if err != nil {
		t.Fatalf("FindFunctions() error = %v", err)
	}

	if len(result.Functions) != 2 {
		t.Fatalf("FindFunctions() found %d functions, want 2", len(result.Functions))
	}
}

func TestPythonFinder_EmptyFile(t *testing.T) {
	config := getPyConfig(t)

	content := ``

	tmpfile := createTempFile(t, content, "test_empty_*.py")
	defer os.Remove(tmpfile)

	finder := NewPythonFinder(*config, "", "map", false)
	result, err := finder.FindFunctions(tmpfile)

	if err != nil {
		t.Fatalf("FindFunctions() error = %v", err)
	}

	if len(result.Functions) != 0 {
		t.Errorf("FindFunctions() found %d functions in empty file, want 0", len(result.Functions))
	}
}

func TestPythonFinder_InvalidFile(t *testing.T) {
	config := getPyConfig(t)

	finder := NewPythonFinder(*config, "", "map", false)
	_, err := finder.FindFunctions("/nonexistent/file.py")

	if err == nil {
		t.Error("FindFunctions() with invalid file should return error")
	}
}

func TestPythonFinder_NestedFunctions(t *testing.T) {
	config := getPyConfig(t)

	content := `def outer_func():
    def inner_func():
        return 1
    return inner_func()
`

	tmpfile := createTempFile(t, content, "test_nested_*.py")
	defer os.Remove(tmpfile)

	finder := NewPythonFinder(*config, "", "map", false)
	result, err := finder.FindFunctions(tmpfile)

	if err != nil {
		t.Fatalf("FindFunctions() error = %v", err)
	}

	// Should find both outer and inner functions
	if len(result.Functions) < 1 {
		t.Fatalf("FindFunctions() found %d functions, want at least 1", len(result.Functions))
	}

	if result.Functions[0].Name != "outer_func" {
		t.Errorf("Functions[0].Name = %v, want outer_func", result.Functions[0].Name)
	}
}

// Helper function to create temporary file with content
func createTempFile(t *testing.T, content string, pattern string) string {
	t.Helper()

	tmpfile, err := os.CreateTemp("", pattern)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	if err := tmpfile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	return tmpfile.Name()
}

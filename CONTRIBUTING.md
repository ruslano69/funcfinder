# Contributing to funcfinder

Thank you for your interest in contributing to funcfinder! This document provides guidelines and instructions for contributing.

## 🎯 Ways to Contribute

- 🐛 **Report bugs** via GitHub Issues
- 💡 **Suggest features** via GitHub Issues
- 📝 **Improve documentation** (README, comments, examples)
- 🔧 **Fix bugs** via Pull Requests
- ✨ **Add features** via Pull Requests
- 🌐 **Add language support** (see below)
- 🧪 **Add tests** to improve coverage

## 🚀 Getting Started

### Prerequisites

- Go 1.22 or higher
- Git
- Basic knowledge of regex patterns (for language support)

### Setup Development Environment

```bash
# Clone the repository
git clone https://github.com/yourusername/funcfinder.git
cd funcfinder

# Build
go build -o funcfinder

# Run tests
go test ./...

# Test locally
./funcfinder --inp main.go --source go --map
```

## 📋 Development Workflow

### 1. Fork and Clone

```bash
# Fork on GitHub, then:
git clone https://github.com/yourusername/funcfinder.git
cd funcfinder
git remote add upstream https://github.com/originalowner/funcfinder.git
```

### 2. Create a Branch

```bash
git checkout -b feature/your-feature-name
# or
git checkout -b fix/issue-123
```

### 3. Make Changes

- Follow existing code style
- Add tests for new functionality
- Update documentation if needed
- Keep commits atomic and descriptive

### 4. Test Your Changes

```bash
# Run tests
go test ./...

# Test manually
go build -o funcfinder
./funcfinder --inp test_file.go --source go --map

# Test on multiple languages
./funcfinder --inp test.c --source c --map
./funcfinder --inp test.java --source java --map
```

### 5. Commit

Use clear, descriptive commit messages:

```bash
git add .
git commit -m "Add Python language support"

# Or more descriptive:
git commit -m "feat: add Python language support

- Add regex pattern for Python functions
- Add Python to languages.json
- Update README with Python example
- Add tests for Python parsing"
```

**Commit message format:**
- `feat:` - New feature
- `fix:` - Bug fix
- `docs:` - Documentation only
- `test:` - Adding tests
- `refactor:` - Code refactoring
- `perf:` - Performance improvement
- `chore:` - Maintenance tasks

### 6. Push and Create PR

```bash
git push origin feature/your-feature-name
```

Then create a Pull Request on GitHub with:
- Clear title and description
- Reference related issues (`Fixes #123`)
- Screenshots/examples if applicable

## 🌐 Adding Language Support

To add support for a new language:

### 1. Add to languages.json

```json
{
  "python": {
    "func_pattern": "^\\s*def\\s+(\\w+)\\s*\\(",
    "line_comment": "#",
    "block_comment_start": "\"\"\"",
    "block_comment_end": "\"\"\"",
    "string_chars": ["\"", "'"],
    "raw_string_chars": ["r\"", "r'"],
    "escape_char": "\\"
  }
}
```

### 2. Test Pattern

```bash
# Create test file
cat > test.py << 'EOF'
def hello():
    pass

def world(arg):
    # comment
    return arg
EOF

# Build and test
go build -o funcfinder
./funcfinder --inp test.py --source python --map
```

### 3. Document

Update README.md:
- Add language to supported list
- Add example

### 4. Add Tests

Create test cases in appropriate test file.

## 🧪 Testing Guidelines

### Unit Tests

```go
func TestSanitizer(t *testing.T) {
    config := &LanguageConfig{
        LineComment: "//",
        // ...
    }
    
    sanitizer := NewSanitizer(config, false)
    cleaned, _ := sanitizer.CleanLine("func main() { // comment", StateNormal)
    
    if !strings.Contains(cleaned, "func main()") {
        t.Errorf("Expected function signature in cleaned output")
    }
}
```

### Integration Tests

Test complete workflows:

```go
func TestFindFunctions(t *testing.T) {
    // Create temporary test file
    content := `package main
    
func main() {
    println("test")
}

func helper() {
    // helper
}`
    
    tmpfile, _ := ioutil.TempFile("", "test*.go")
    defer os.Remove(tmpfile.Name())
    tmpfile.Write([]byte(content))
    tmpfile.Close()
    
    // Test finder
    config, _ := LoadConfig()
    langConfig, _ := config.GetLanguageConfig("go")
    finder := NewFinder(langConfig, []string{}, true, false, false)
    result, err := finder.FindFunctions(tmpfile.Name())
    
    if err != nil {
        t.Fatalf("FindFunctions failed: %v", err)
    }
    
    if len(result.Functions) != 2 {
        t.Errorf("Expected 2 functions, got %d", len(result.Functions))
    }
}
```

## 📝 Code Style

### Go Style Guide

- Follow [Effective Go](https://go.dev/doc/effective_go)
- Use `gofmt` for formatting
- Keep functions focused and testable
- Add comments for exported functions

### Example

```go
// FindFunctions searches for function boundaries in the source file.
// It returns a FindResult containing all matched functions or an error.
func (f *Finder) FindFunctions(filename string) (*FindResult, error) {
    // Implementation...
}
```

## 🐛 Reporting Issues

### Bug Reports

Include:
- funcfinder version (or commit hash)
- Operating system and version
- Go version
- Complete command that reproduces the issue
- Expected vs actual behavior
- Sample input file (if possible)

**Template:**

```markdown
**Version:** v1.0.0
**OS:** Ubuntu 22.04
**Go:** 1.22.2

**Command:**
funcfinder --inp test.go --source go --func MyFunc

**Expected:**
MyFunc: 10-20;

**Actual:**
No functions found

**Sample file:** (attached or inline)
```

### Feature Requests

Include:
- Use case / problem to solve
- Proposed solution
- Alternative solutions considered
- Examples

## 🔍 Code Review Process

PRs will be reviewed for:
- Code quality and style
- Test coverage
- Documentation
- Performance impact
- Breaking changes

**Review timeframe:** Typically 1-3 days for initial feedback.

## 📜 License

By contributing, you agree that your contributions will be licensed under the MIT License.

## 💬 Questions?

- Open a [Discussion](https://github.com/yourusername/funcfinder/discussions)
- Ask in [Issues](https://github.com/yourusername/funcfinder/issues)

## 🙏 Thank You!

Every contribution makes funcfinder better for the entire community!

---

**Happy coding! 🚀**

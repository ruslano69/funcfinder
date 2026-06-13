package internal

import (
	"os"
	"testing"
)

// hasEdge reports whether the call graph contains a caller->callee edge.
func hasEdge(fcg *FileCallGraph, caller, callee string) bool {
	for _, e := range fcg.Calls {
		if e.Caller == caller && e.Callee == callee {
			return true
		}
	}
	return false
}

// TestBuildFileCallGraph_CallAfterDocstring is a regression test for the bug
// where Python triple-quoted docstrings were configured as block comments
// (block_comment_start equal to block_comment_end). The nested-aware block
// comment scanner mistook the closing delimiter for a nested opening, so the
// sanitizer stayed "inside a comment" forever and every call after the first
// docstring was blanked out, silently dropping ~80% of Python call edges.
//
// The fix routes triple-quoted strings through doc_string_markers instead.
// This test ensures calls placed after a docstring are still detected.
func TestBuildFileCallGraph_CallAfterDocstring(t *testing.T) {
	config := getPyConfig(t)

	content := `def target():
    return 1


def before_docstring(p):
    return target()


def with_docstring(p):
    """A one-line docstring."""
    return target()


def after_docstring(p):
    """Another docstring."""
    return target()
`

	tmpfile := createTempFile(t, content, "test_callgraph_docstring_*.py")
	defer os.Remove(tmpfile)

	fcg, err := BuildFileCallGraph(tmpfile, config, nil, nil)
	if err != nil {
		t.Fatalf("BuildFileCallGraph() error = %v", err)
	}

	// before_docstring sits before any docstring; it worked even with the bug.
	if !hasEdge(fcg, "before_docstring", "target") {
		t.Error("missing edge before_docstring -> target")
	}

	// These two are the regression: with the bug they were dropped because the
	// preceding docstring left the sanitizer stuck in a comment state.
	if !hasEdge(fcg, "with_docstring", "target") {
		t.Error("missing edge with_docstring -> target (docstring leaked sanitizer state)")
	}
	if !hasEdge(fcg, "after_docstring", "target") {
		t.Error("missing edge after_docstring -> target (docstring leaked sanitizer state)")
	}
}

// TestSanitizer_DocstringDoesNotLeakState pins the root cause at the sanitizer
// level: a single-line triple-quoted docstring must leave the parser back in
// StateNormal, and the code on the following line must survive cleaning.
func TestSanitizer_DocstringDoesNotLeakState(t *testing.T) {
	config := getPyConfig(t)
	san := NewSanitizer(config, false)

	// Single-line docstring: opens and closes on the same line.
	_, state := san.CleanLine(`    """A one-line docstring."""`, StateNormal)
	if state != StateNormal {
		t.Fatalf("after single-line docstring, state = %v, want StateNormal", state)
	}

	// The following line's call must not be blanked out.
	clean, state := san.CleanLine(`    return target()`, state)
	if state != StateNormal {
		t.Errorf("line after docstring, state = %v, want StateNormal", state)
	}
	if !containsSubstr(clean, "target") {
		t.Errorf("call after docstring was blanked: cleaned line = %q", clean)
	}
}

func containsSubstr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

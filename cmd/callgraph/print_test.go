package main

import (
	"strings"
	"testing"

	"github.com/ruslano69/funcfinder/internal"
)

func treeOf(edges ...internal.CallEdge) *internal.CallGraphResult {
	return &internal.CallGraphResult{Files: []internal.FileCallGraph{{Path: "x.rs", Calls: edges}}}
}

// A function reached through two branches is recursive only if it calls itself
// along the path. Being shown again under another caller is not recursion.
func TestPrintCallTree_SharedCalleeIsNotRecursive(t *testing.T) {
	cg := treeOf(
		internal.CallEdge{Caller: "main", Callee: "helper"},
		internal.CallEdge{Caller: "main", Callee: "new"},
		internal.CallEdge{Caller: "helper", Callee: "new"},
	)
	out := captureStdout(t, func() { printCallTree(cg, "main") })

	if strings.Contains(out, "recursive") {
		t.Fatalf("a shared callee is reported as recursive:\n%s", out)
	}
	if !strings.Contains(out, "(see above)") {
		t.Fatalf("the second appearance of new is not marked as already shown:\n%s", out)
	}
}

// A call that leads back to an ancestor on the current path is recursion.
func TestPrintCallTree_RealRecursionIsMarked(t *testing.T) {
	cg := treeOf(
		internal.CallEdge{Caller: "a", Callee: "b"},
		internal.CallEdge{Caller: "b", Callee: "a"},
	)
	out := captureStdout(t, func() { printCallTree(cg, "a") })

	want := "a\n  b\n    a (↑ recursive)\n"
	if out != want {
		t.Fatalf("output:\n%q\nwant:\n%q", out, want)
	}
}

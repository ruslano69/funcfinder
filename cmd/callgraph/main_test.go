package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/ruslano69/funcfinder/internal"
)

// captureStdout runs fn and returns what it wrote to os.Stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	done := make(chan string)
	go func() {
		var b bytes.Buffer
		_, _ = io.Copy(&b, r)
		done <- b.String()
	}()
	fn()
	_ = w.Close()
	os.Stdout = old
	return <-done
}

// issue53Graph is the reproduction from funcfinder issue #53: a method called
// through a field, a method called through its receiver, and a plain function
// that calls the method through a parameter.
func issue53Graph() *internal.CallGraphResult {
	return &internal.CallGraphResult{Files: []internal.FileCallGraph{{
		Path: "repro.go",
		Calls: []internal.CallEdge{
			{Caller: "Run", Callee: "d.db.Save", Line: 10},
			{Caller: "Flush", Callee: "d.Save", Line: 13},
			{Caller: "helper", Callee: "db.Save", Line: 15},
			{Caller: "entry", Callee: "helper", Line: 17},
		},
	}}}
}

func TestReverseOutput_TextFindsMethodCallers(t *testing.T) {
	out := captureStdout(t, func() { output(issue53Graph(), false, true, "Save", 0) })
	want := "Save is called by:\n  Flush\n  Run\n  helper\n"
	if out != want {
		t.Fatalf("text output:\n%q\nwant:\n%q", out, want)
	}
}

func TestReverseOutput_JSONFindsMethodCallers(t *testing.T) {
	out := captureStdout(t, func() { output(issue53Graph(), true, true, "Save", 0) })
	var got struct {
		Function string   `json:"function"`
		Callers  []string `json:"callers"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out)
	}
	if got.Function != "Save" || !reflect.DeepEqual(got.Callers, []string{"Flush", "Run", "helper"}) {
		t.Fatalf("JSON callers = %+v, want Save called by Flush, Run, helper", got)
	}
}

func TestReverseOutput_NoCallersSaysSo(t *testing.T) {
	out := captureStdout(t, func() { output(issue53Graph(), false, true, "Nobody", 0) })
	if !strings.Contains(out, "Nobody: not called by any known function") {
		t.Fatalf("output = %q, want the not-called message", out)
	}
}

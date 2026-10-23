package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ruslano69/funcfinder/internal"
)

// Exercise the production flags/decoder/write path without rebuilding a CLI.
func TestGooseObjectCLIProcess(t *testing.T) {
	if os.Getenv("FUNCFINDER_GOOSE_CLI_HELPER") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			os.Args = append([]string{"funcfinder"}, os.Args[i+1:]...)
			break
		}
	}
	flag.CommandLine = flag.NewFlagSet("funcfinder", flag.ExitOnError)
	main()
	os.Exit(0)
}

func TestGooseObjectCLI(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.goose")
	src := []byte("\xef\xbb\xbfnamespace sample;\r\nfn a() { let old = 1; } fn b() {}\r\nstruct Box {data:u8[>..]}\r\n")
	if err := os.WriteFile(path, src, 0600); err != nil {
		t.Fatal(err)
	}
	run := func(request any, success bool, args ...string) []byte {
		t.Helper()
		cmd := exec.Command(os.Args[0], append([]string{"-test.run=^TestGooseObjectCLIProcess$", "--"}, args...)...)
		cmd.Env = append(os.Environ(), "FUNCFINDER_GOOSE_CLI_HELPER=1")
		if request != nil {
			data, err := json.Marshal(request)
			if err != nil {
				t.Fatal(err)
			}
			cmd.Stdin = bytes.NewReader(data)
		}
		out, err := cmd.CombinedOutput()
		if (err == nil) != success {
			t.Fatalf("CLI %v: %s %v", args, out, err)
		}
		return out
	}
	read := func(name string) internal.CodeObject {
		t.Helper()
		var o internal.CodeObject
		if err := json.Unmarshal(run(nil, true, "--inp", path, "--source", "goose", "--read-object", name), &o); err != nil {
			t.Fatal(err)
		}
		return o
	}
	a, b := read("a"), read("b")
	a.Source = "fn a() { let renamed = helper(); }"
	request := internal.CodeEditBatch{Protocol: "funcfinder.edits/v1", Edits: []internal.CodeEdit{
		{Operation: "replace", Object: a},
		{Operation: "insert_after", Object: b, Source: "fn helper() -> i64 { return 1; }"},
		{Operation: "remove", Object: b},
	}}
	preview := run(request, true, "--edit-object", "-")
	before, _ := os.ReadFile(path)
	if !bytes.Equal(before, src) || !bytes.Contains(preview, []byte(`"goose_tokens_and_declaration_identity"`)) {
		t.Fatalf("preview modified file or mislabeled validation: %s", preview)
	}
	var result struct {
		Status  string
		Results []internal.CodeEditResult
	}
	if err := json.Unmarshal(run(request, true, "--edit-object", "-", "--apply"), &result); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(path)
	if result.Status != "applied" || len(result.Results) != 3 || bytes.Contains(after, []byte("fn b()")) || !bytes.Contains(after, []byte("fn helper()")) || !bytes.HasSuffix(after, []byte("\r\nstruct Box {data:u8[>..]}\r\n")) {
		t.Fatalf("apply: %s %#v", after, result)
	}
	refusal := run(request, false, "--edit-object", "-", "--apply")
	unchanged, _ := os.ReadFile(path)
	if !bytes.Contains(refusal, []byte(`"stale_revision"`)) || !bytes.Equal(after, unchanged) {
		t.Fatalf("stale request wrote bytes: %s", refusal)
	}
	next := result.Results[0].Objects[0]
	next.Source = "fn a() { let answer = helper(); }"
	run(next, true, "--edit-object", "-", "--apply")
}

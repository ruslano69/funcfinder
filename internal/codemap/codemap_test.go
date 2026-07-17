package codemap

import (
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ruslano69/funcfinder/internal/knowledge"
)

func openDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := knowledge.Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatalf("knowledge.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func writeGoFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// initGitRepo makes dir a git repo with one commit, using repo-local config
// so the test doesn't depend on (or pollute) the machine's global git
// identity. Returns the commit SHA it created.
func initGitRepo(t *testing.T, dir string) string {
	t.Helper()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")
	run("add", "-A")
	run("commit", "-q", "-m", "initial")

	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("git rev-parse HEAD: %v", err)
	}
	return string(out[:len(out)-1]) // trim trailing newline
}

func TestIngest_BasicGoFile(t *testing.T) {
	dir := t.TempDir()
	writeGoFile(t, dir, "main.go", `package main

func helper() int { return 42 }

type Widget struct {
	Name string
}

func main() {
	x := helper()
	_ = x
}
`)
	db := openDB(t)

	stats, err := Ingest(db, dir)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if stats.Files != 1 {
		t.Fatalf("want 1 file ingested, got %d", stats.Files)
	}

	rows, err := db.Query(`SELECT title, content, type FROM docs WHERE type = ?`, DocType)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()

	var found bool
	for rows.Next() {
		var title, content, docType string
		if err := rows.Scan(&title, &content, &docType); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if title != "main.go" {
			t.Errorf("title = %q, want %q", title, "main.go")
		}
		if docType != DocType {
			t.Errorf("type = %q, want %q", docType, DocType)
		}
		for _, want := range []string{"helper:", "main:", "Widget:"} {
			if !strings.Contains(content, want) {
				t.Errorf("content missing %q; got:\n%s", want, content)
			}
		}
		found = true
	}
	if !found {
		t.Fatal("no code_map document found")
	}
}

func TestIngest_ReplacesStaleDocsOnRerun(t *testing.T) {
	dir := t.TempDir()
	writeGoFile(t, dir, "main.go", "package main\n\nfunc first() {}\n")
	db := openDB(t)

	if _, err := Ingest(db, dir); err != nil {
		t.Fatalf("first Ingest: %v", err)
	}

	// Simulate the code changing between publishes: rename the function and
	// add a second file.
	writeGoFile(t, dir, "main.go", "package main\n\nfunc renamed() {}\n")
	writeGoFile(t, dir, "extra.go", "package main\n\nfunc second() {}\n")

	stats, err := Ingest(db, dir)
	if err != nil {
		t.Fatalf("second Ingest: %v", err)
	}
	if stats.Files != 2 {
		t.Fatalf("want 2 files on second ingest, got %d", stats.Files)
	}
	if stats.Replaced == 0 {
		t.Fatalf("want Replaced > 0 (the stale main.go doc), got %d", stats.Replaced)
	}

	n, err := knowledge.Count(db)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if n != 2 {
		t.Fatalf("want exactly 2 code_map docs after replace (no stale accumulation), got %d", n)
	}

	rows, err := db.Query(`SELECT content FROM docs WHERE type = ?`, DocType)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()
	var all string
	for rows.Next() {
		var c string
		rows.Scan(&c)
		all += c
	}
	if strings.Contains(all, "first:") {
		t.Error("stale function 'first' from the first ingest should not still be present")
	}
	if !strings.Contains(all, "renamed:") || !strings.Contains(all, "second:") {
		t.Errorf("want both current functions present; got:\n%s", all)
	}
}

func TestIngest_GitProvenance(t *testing.T) {
	dir := t.TempDir()
	writeGoFile(t, dir, "main.go", "package main\n\nfunc f() {}\n")
	wantSHA := initGitRepo(t, dir)

	db := openDB(t)
	stats, err := Ingest(db, dir)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if stats.CommitSHA != wantSHA {
		t.Errorf("CommitSHA = %q, want %q", stats.CommitSHA, wantSHA)
	}
	if stats.Dirty {
		t.Error("want Dirty=false right after a clean commit")
	}
	if w := stats.Warning(); w != "" {
		t.Errorf("want no warning for a clean git checkout, got: %s", w)
	}

	// Now make an uncommitted change.
	writeGoFile(t, dir, "main.go", "package main\n\nfunc f() {}\nfunc g() {}\n")
	stats2, err := Ingest(db, dir)
	if err != nil {
		t.Fatalf("second Ingest: %v", err)
	}
	if !stats2.Dirty {
		t.Error("want Dirty=true after an uncommitted edit")
	}
	if w := stats2.Warning(); w == "" {
		t.Error("want a warning when the tree is dirty")
	}
}

func TestIngest_NonGitDirectory(t *testing.T) {
	dir := t.TempDir()
	writeGoFile(t, dir, "main.go", "package main\n\nfunc f() {}\n")
	db := openDB(t)

	stats, err := Ingest(db, dir)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if stats.CommitSHA != "" {
		t.Errorf("want empty CommitSHA for a non-git directory, got %q", stats.CommitSHA)
	}
	if w := stats.Warning(); w == "" {
		t.Error("want a warning when no commit SHA could be determined")
	}
}

func TestIngest_EmptyRerunWarnsInsteadOfSilentlyClearingMap(t *testing.T) {
	dir := t.TempDir()
	writeGoFile(t, dir, "main.go", "package main\n\nfunc f() {}\n")
	db := openDB(t)

	stats, err := Ingest(db, dir)
	if err != nil {
		t.Fatalf("first Ingest: %v", err)
	}
	if stats.Files != 1 {
		t.Fatalf("want 1 file on first ingest, got %d", stats.Files)
	}

	// Simulate a misrooted re-ingest (e.g. --code-dir typo'd to an empty or
	// wrong directory): every prior code_map doc gets replaced with nothing.
	if err := os.Remove(filepath.Join(dir, "main.go")); err != nil {
		t.Fatalf("remove main.go: %v", err)
	}
	writeGoFile(t, dir, "consts.go", "package main\n\nconst X = 1\n")

	stats2, err := Ingest(db, dir)
	if err != nil {
		t.Fatalf("second Ingest: %v", err)
	}
	if stats2.Files != 0 {
		t.Fatalf("want 0 files on second ingest, got %d", stats2.Files)
	}
	if stats2.Replaced == 0 {
		t.Fatalf("want Replaced > 0 (the first ingest's doc was cleared), got %d", stats2.Replaced)
	}
	if w := stats2.Warning(); w == "" {
		t.Error("want a warning when a non-empty code map is replaced by an empty one")
	}

	n, _ := knowledge.Count(db)
	if n != 0 {
		t.Errorf("want 0 code_map docs left after the empty re-ingest, got %d", n)
	}
}

func TestIngest_NonexistentDirectoryErrors(t *testing.T) {
	db := openDB(t)
	if _, err := Ingest(db, filepath.Join(t.TempDir(), "does-not-exist")); err == nil {
		t.Fatal("want an error for a nonexistent code-dir")
	}
}

func TestIngest_NoFunctionsOrTypesSkipped(t *testing.T) {
	dir := t.TempDir()
	writeGoFile(t, dir, "consts.go", "package main\n\nconst X = 1\n")
	db := openDB(t)

	stats, err := Ingest(db, dir)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if stats.Files != 0 {
		t.Errorf("want 0 files (no functions/types to report), got %d", stats.Files)
	}
	n, _ := knowledge.Count(db)
	if n != 0 {
		t.Errorf("want 0 docs, got %d", n)
	}
}

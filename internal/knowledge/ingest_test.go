package knowledge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHasRepetitiveRuns(t *testing.T) {
	cases := []struct {
		name string
		text string
		want bool
	}{
		// TOC lines with dot leaders — should be filtered
		{"dot leaders spaced", "StartFingerprint . . . . . . . . . . . . . . . 474", true},
		{"dot leaders dense", "StartFingerprint................474", true},
		{"dash line", "--------------------------------", true},
		{"mixed TOC", "99.4 Alpha . . . . . . . . . . . . . . . 346\n99.5 RGB . . . . . . . . . . . . . 346", true},

		// Good content — should NOT be filtered
		{"normal prose", "StartFingerprint initialises a fingerprint calculation.", false},
		{"code sample", "Result = StartFingerprint(0, #PB_Cipher_MD5)", false},
		{"short sentence", "Returns nonzero if the key was generated.", false},
		// Dots in URLs / ellipsis are fine (< 25% threshold)
		{"url", "See http://www.purebasic.com/ for details on how to use this.", false},
		// A few dashes as separator are fine
		{"short dash", "--- note ---", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := hasRepetitiveRuns(c.text)
			if got != c.want {
				t.Errorf("hasRepetitiveRuns(%q) = %v, want %v", c.text, got, c.want)
			}
		})
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

func writeFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeFile: %v", err)
	}
	return path
}

// ── UTF-8 / encoding ─────────────────────────────────────────────────────────

func TestToUTF8_Valid(t *testing.T) {
	s, err := toUTF8([]byte("hello мир"))
	if err != nil || s != "hello мир" {
		t.Fatalf("got %q, err=%v", s, err)
	}
}

func TestToUTF8_BOMStripped(t *testing.T) {
	raw := append([]byte{0xEF, 0xBB, 0xBF}, []byte("hello")...)
	s, err := toUTF8(raw)
	if err != nil || s != "hello" {
		t.Fatalf("BOM not stripped: %q, err=%v", s, err)
	}
}

func TestToUTF8_Windows1252(t *testing.T) {
	// 0xe9 = é in Windows-1252 / ISO-8859-1
	raw := []byte("caf\xe9")
	s, err := toUTF8(raw)
	if err != nil {
		t.Fatalf("toUTF8: %v", err)
	}
	if !strings.Contains(s, "caf") {
		t.Fatalf("expected 'caf...' got %q", s)
	}
}

// ── TXT ingestion ─────────────────────────────────────────────────────────────

func TestIngestTXT_Basic(t *testing.T) {
	path := writeFile(t, "notes.txt", "First paragraph here.\n\nSecond paragraph here.")
	chunks, err := IngestFile(path, ChunkOpts{})
	if err != nil {
		t.Fatalf("IngestFile: %v", err)
	}
	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk")
	}
	if chunks[0].Title != "notes" {
		t.Errorf("title: want 'notes', got %q", chunks[0].Title)
	}
	if !strings.Contains(chunks[0].Content, "First paragraph") {
		t.Errorf("content missing first paragraph: %q", chunks[0].Content)
	}
}

func TestIngestTXT_ChunkSplit(t *testing.T) {
	// Force split: short max, two big paragraphs
	para := strings.Repeat("word ", 100)
	path := writeFile(t, "long.txt", para+"\n\n"+para)
	chunks, err := IngestFile(path, ChunkOpts{MaxRunes: 200, OverlapRunes: 20})
	if err != nil {
		t.Fatalf("IngestFile: %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("expected split into >=2 chunks, got %d", len(chunks))
	}
}

// ── Markdown ingestion ────────────────────────────────────────────────────────

func TestIngestMD_Sections(t *testing.T) {
	md := `# Installation

Install the tool with go get.

## Quick Start

Run the binary.

## Configuration

Set the flags.
`
	path := writeFile(t, "doc.md", md)
	chunks, err := IngestFile(path, ChunkOpts{})
	if err != nil {
		t.Fatalf("IngestFile: %v", err)
	}
	titles := make([]string, len(chunks))
	for i, c := range chunks {
		titles[i] = c.Title
	}
	// Each section becomes at least one chunk
	wantTitles := []string{"Installation", "Quick Start", "Configuration"}
	for _, want := range wantTitles {
		found := false
		for _, got := range titles {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("section %q not found in chunks: %v", want, titles)
		}
	}
}

func TestIngestMD_CodeFenceNotSplit(t *testing.T) {
	md := "## Example\n\n```go\nfunc hello() {\n\n\tfmt.Println(\"hi\")\n}\n```\n"
	path := writeFile(t, "code.md", md)
	chunks, err := IngestFile(path, ChunkOpts{MaxRunes: 2000})
	if err != nil {
		t.Fatalf("IngestFile: %v", err)
	}
	// The code block should not be split across chunks
	full := strings.Join(func() []string {
		ss := make([]string, len(chunks))
		for i, c := range chunks {
			ss[i] = c.Content
		}
		return ss
	}(), " ")
	if !strings.Contains(full, "fmt.Println") {
		t.Error("code block content lost or split")
	}
}

func TestIngestMD_NoCrossSection(t *testing.T) {
	// Two sections; tiny max so each section gets its own chunk(s)
	md := "## Alpha\n\nContent alpha.\n\n## Beta\n\nContent beta.\n"
	path := writeFile(t, "two.md", md)
	chunks, err := IngestFile(path, ChunkOpts{MaxRunes: 50, OverlapRunes: 5})
	if err != nil {
		t.Fatalf("IngestFile: %v", err)
	}
	for _, c := range chunks {
		if strings.Contains(c.Content, "alpha") && strings.Contains(c.Content, "beta") {
			t.Errorf("chunk crosses section boundary: %q", c.Content)
		}
	}
}

// ── chunk logic ───────────────────────────────────────────────────────────────

func TestChunkOverlap(t *testing.T) {
	para := strings.Repeat("x", 300) // 300 runes each
	sec := docSection{title: "T", paras: []string{para, para, para}}
	opts := ChunkOpts{MaxRunes: 400, OverlapRunes: 50}
	chunks := chunkSection(sec, "src", opts)
	if len(chunks) < 2 {
		t.Fatalf("expected >=2 chunks")
	}
	// Second chunk should start with overlap from first
	if len(chunks[1].Content) == 0 {
		t.Fatal("second chunk is empty")
	}
}

func TestUnsupportedExtension(t *testing.T) {
	path := writeFile(t, "file.docx", "content")
	_, err := IngestFile(path, ChunkOpts{})
	if err == nil {
		t.Fatal("expected error for unsupported extension")
	}
}

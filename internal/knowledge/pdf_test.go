package knowledge

import (
	"errors"
	"fmt"
	"go/build"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ── real PDF from ledongthuc/pdf test fixtures ────────────────────────────────

func TestIngestPDF_Real(t *testing.T) {
	// Use the test fixture bundled with the ledongthuc/pdf module.
	gopath := build.Default.GOPATH
	src := filepath.Join(gopath,
		"pkg/mod/github.com/ledongthuc/pdf@v0.0.0-20250511090121-5959a4027728",
		"examples/read_plain_text/pdf_test.pdf")
	if _, err := os.Stat(src); err != nil {
		t.Skipf("test PDF not found: %v", err)
	}

	chunks, err := IngestFile(src, ChunkOpts{MaxRunes: 800, OverlapRunes: 80})
	if err != nil {
		t.Fatalf("IngestFile: %v", err)
	}
	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk")
	}
	t.Logf("real PDF → %d chunks, first title: %q", len(chunks), chunks[0].Title)
	for _, c := range chunks {
		if c.Content == "" {
			t.Errorf("chunk %q has empty content", c.Title)
		}
	}
}

// ── synthetic large PDF (100 pages) ──────────────────────────────────────────

func TestIngestPDF_Large(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large.pdf")
	const numPages = 100
	if err := generateTestPDF(path, numPages); err != nil {
		t.Fatalf("generate: %v", err)
	}

	fi, _ := os.Stat(path)
	t.Logf("generated PDF: %d bytes", fi.Size())

	start := time.Now()
	chunks, err := IngestFile(path, ChunkOpts{MaxRunes: 800, OverlapRunes: 80})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("IngestFile: %v", err)
	}
	t.Logf("%d pages → %d chunks in %v", numPages, len(chunks), elapsed)

	if len(chunks) == 0 {
		t.Fatal("no chunks returned")
	}
	// Every chunk must have a page reference.
	for _, c := range chunks {
		if c.Page == 0 {
			t.Errorf("chunk %q has page=0 (not set)", c.Title)
		}
	}
	// Chunks must not cross page/section boundaries.
	for _, c := range chunks {
		if c.Section == "" {
			t.Errorf("chunk missing Section field: %+v", c)
		}
	}
	// Overlap: second chunk within a long page should start with tail of prev.
	// (just verify chunks are non-empty and distinct)
	seen := map[string]bool{}
	for _, c := range chunks {
		if seen[c.Content] {
			t.Errorf("duplicate chunk content in %q", c.Title)
		}
		seen[c.Content] = true
	}
}

// ── PDF generator ─────────────────────────────────────────────────────────────

// generateTestPDF creates a minimal valid multi-page PDF with structured text.
func generateTestPDF(path string, numPages int) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	pos := int64(0)
	write := func(s string) {
		f.WriteString(s) //nolint
		pos += int64(len(s))
	}
	writef := func(format string, args ...any) {
		s := fmt.Sprintf(format, args...)
		write(s)
	}

	offsets := make([]int64, 0, 2+numPages*2)
	mark := func() { offsets = append(offsets, pos) }

	// Header
	write("%PDF-1.4\n%\x80\x80\x80\x80\n")

	// Object 1: Catalog
	mark()
	write("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")

	// Object 2: Pages (with shared font resource)
	kids := make([]string, numPages)
	for i := range kids {
		kids[i] = fmt.Sprintf("%d 0 R", 3+i*2)
	}
	mark()
	writef("2 0 obj\n<< /Type /Pages /Kids [%s] /Count %d\n   /Resources << /Font << /F1 << /Type /Font /Subtype /Type1 /BaseFont /Helvetica >> >> >> >>\nendobj\n",
		strings.Join(kids, " "), numPages)

	// Objects 3..2+2N: alternating page + content stream
	for i := 1; i <= numPages; i++ {
		pageObj := 3 + (i-1)*2
		contObj := 4 + (i-1)*2

		// Page object
		mark()
		writef("%d 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents %d 0 R >>\nendobj\n",
			pageObj, contObj)

		// Content stream
		stream := buildPageStream(i)
		mark()
		writef("%d 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj\n",
			contObj, len(stream), stream)
	}

	// Cross-reference table
	xrefPos := pos
	totalObjs := 2 + numPages*2 // objects 1..totalObjs
	writef("xref\n0 %d\n", totalObjs+1)
	write("0000000000 65535 f \n")
	for _, off := range offsets {
		writef("%010d 00000 n \n", off)
	}

	// Trailer
	writef("trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n",
		totalObjs+1, xrefPos)

	return nil
}

func TestIngestPDF_DensePages(t *testing.T) {
	// Pages with lots of text → forces chunking within a single page
	path := filepath.Join(t.TempDir(), "dense.pdf")
	if err := generateDensePDF(path, 20); err != nil {
		t.Fatalf("generate: %v", err)
	}
	chunks, err := IngestFile(path, ChunkOpts{MaxRunes: 300, OverlapRunes: 30})
	if err != nil {
		t.Fatalf("IngestFile: %v", err)
	}
	t.Logf("20 dense pages → %d chunks (>20 means multi-chunk pages)", len(chunks))
	if len(chunks) <= 20 {
		t.Error("expected more than 1 chunk per page for dense content")
	}
	// Overlap: second chunk on the same page should start with tail of first
	for i := 1; i < len(chunks); i++ {
		if chunks[i].Page == chunks[i-1].Page {
			// Both chunks from same page — content should not be identical
			if chunks[i].Content == chunks[i-1].Content {
				t.Errorf("consecutive same-page chunks are identical at index %d", i)
			}
		}
	}
}

func TestIngestPDF_EmptyPages(t *testing.T) {
	// PDF where every other page is blank — should skip gracefully
	path := filepath.Join(t.TempDir(), "sparse.pdf")
	if err := generateSparsePDF(path, 10); err != nil {
		t.Fatalf("generate: %v", err)
	}
	chunks, err := IngestFile(path, ChunkOpts{})
	if err != nil {
		t.Fatalf("IngestFile: %v", err)
	}
	// Only 5 pages have content
	t.Logf("10-page sparse PDF (5 empty) → %d chunks", len(chunks))
	for _, c := range chunks {
		if strings.TrimSpace(c.Content) == "" {
			t.Error("chunk with empty content returned for blank page")
		}
	}
}

func TestPageTextQuality(t *testing.T) {
	cases := []struct {
		name    string
		text    string
		wantMin float64
		wantMax float64
	}{
		{
			name:    "good prose",
			text:    "The quick brown fox jumps over the lazy dog. This is a normal sentence with readable words.",
			wantMin: 0.8, wantMax: 1.0,
		},
		{
			name:    "code sample",
			text:    "Result$ = Left(String$, Length)\nDebug InsertString(\"Hello\", \"World\", 7)",
			wantMin: 0.5, wantMax: 1.0,
		},
		{
			name:    "spaced-out OCR (T e x t)",
			text:    "T h e   q u i c k   b r o w n   f o x   j u m p s   o v e r   t h e   l a z y   d o g",
			wantMin: 0.0, wantMax: 0.44,
		},
		{
			name:    "garbage symbols",
			text:    "ÃÂ©â€™â€œâ€ □■▪▫ \x00\x01\x02 ÃÂ©â€™ □■ ÃÂ© â€™",
			wantMin: 0.0, wantMax: 0.44, // below minOCRQuality threshold (0.45)
		},
		{
			name:    "empty",
			text:    "",
			wantMin: 0.0, wantMax: 0.0,
		},
		{
			name:    "mixed bad OCR",
			text:    "F i l e N a m e $ = G e t F i l e N a m e ( ) R e s u l t = O p e n F i l e ( F i l e N a m e $ )",
			wantMin: 0.0, wantMax: 0.44,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := pageTextQuality(c.text)
			if got < c.wantMin || got > c.wantMax {
				t.Errorf("pageTextQuality = %.3f, want [%.2f, %.2f]", got, c.wantMin, c.wantMax)
			}
		})
	}
}

func TestOCRQualityError_GoodPDF(t *testing.T) {
	// A synthetic PDF with normal text should pass OCR quality check.
	path := filepath.Join(t.TempDir(), "good.pdf")
	if err := generatePDFWithStreamFn(path, 20, buildPageStream); err != nil {
		t.Fatalf("generate: %v", err)
	}
	_, err := IngestFile(path, ChunkOpts{})
	if err != nil {
		t.Errorf("expected no error for good PDF, got: %v", err)
	}
}

func TestOCRQualityError_BadOCR(t *testing.T) {
	// A synthetic PDF where every page is spaced-out OCR garbage.
	path := filepath.Join(t.TempDir(), "bad_ocr.pdf")
	badStream := func(_ int) string {
		// Simulate spaced-out OCR: each letter separated by space
		line := "T h i s   i s   b a d   O C R   t e x t   w i t h   s p a c e d   c h a r s"
		var sb strings.Builder
		sb.WriteString("BT\n/F1 12 Tf\n50 750 Td\n")
		for i := 0; i < 10; i++ {
			sb.WriteString(fmt.Sprintf("0 -16 Td\n(%s) Tj\n", line))
		}
		sb.WriteString("ET\n")
		return sb.String()
	}
	if err := generatePDFWithStreamFn(path, 20, badStream); err != nil {
		t.Fatalf("generate: %v", err)
	}
	_, err := IngestFile(path, ChunkOpts{})
	if err == nil {
		t.Fatal("expected OCRQualityError for bad OCR PDF, got nil")
	}
	var ocrErr *OCRQualityError
	if !errors.As(err, &ocrErr) {
		t.Fatalf("expected *OCRQualityError, got %T: %v", err, err)
	}
	if ocrErr.Score >= minOCRQuality {
		t.Errorf("expected score < %.2f, got %.2f", minOCRQuality, ocrErr.Score)
	}
	t.Logf("bad OCR score: %.3f", ocrErr.Score)
}

func TestLooksGlued(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		// Normal readable text — not glued
		{"Left returns characters from the left side of a string.", false},
		{"The quick brown fox jumps over the lazy dog.", false},
		// Empty / whitespace — looksGlued returns false (nothing to fall back to)
		{"", false},
		{"   ", false},
		// Fewer than 3 words but non-empty — treat as glued (can't judge quality)
		{"LeftStrRightMid", true},
		{"AB", true},
		// Long average word length — classic glued symptom
		{"LeftStr()Right()Mid()InsertString()RemoveString()FindString()", true},
		{"CreateWindowEx(WS_OVERLAPPEDWINDOW,NULL,NULL,0,0,CW_USEDEFAULT,CW_USEDEFAULT)", true},
	}
	for _, c := range cases {
		got := looksGlued(c.text)
		if got != c.want {
			t.Errorf("looksGlued(%q) = %v, want %v", c.text, got, c.want)
		}
	}
}

var chapterTopics = []string{
	"Variables and Data Types",
	"Control Flow Statements",
	"Procedures and Functions",
	"Arrays and Linked Lists",
	"File Input and Output",
	"String Manipulation",
	"Network Programming",
	"Graphics and Drawing",
	"Date and Time",
	"Mathematical Functions",
	"Regular Expressions",
	"Database Operations",
	"Memory Management",
	"Error Handling",
	"Module System",
}

// buildPageStream generates a PDF content stream for page i.
func buildPageStream(pageNum int) string {
	topic := chapterTopics[(pageNum-1)%len(chapterTopics)]
	lines := []string{
		fmt.Sprintf("Chapter %d: %s", pageNum, topic),
		"",
		fmt.Sprintf("This section introduces %s concepts used in PureBasic.", strings.ToLower(topic)),
		fmt.Sprintf("Understanding %s is essential for writing robust programs.", strings.ToLower(topic)),
		"",
		fmt.Sprintf("The %s module provides a rich set of built-in commands.", strings.ToLower(topic)),
		"Each command is documented with parameters, return values, and examples.",
		"Error conditions are described for every function that can fail.",
		"",
		fmt.Sprintf("Advanced %s techniques allow developers to optimize performance.", strings.ToLower(topic)),
		"Profiling tools help identify bottlenecks in production code.",
		"Memory usage can be monitored using built-in diagnostic functions.",
		"",
		"See the companion examples directory for complete working programs.",
		fmt.Sprintf("Page %d of the PureBasic reference manual.", pageNum),
	}

	var sb strings.Builder
	sb.WriteString("BT\n/F1 12 Tf\n50 750 Td\n")
	for i, line := range lines {
		escaped := pdfEscape(line)
		if i == 0 {
			sb.WriteString(fmt.Sprintf("(%s) Tj\n", escaped))
		} else if line == "" {
			sb.WriteString("0 -8 Td\n")
		} else {
			sb.WriteString(fmt.Sprintf("0 -16 Td\n(%s) Tj\n", escaped))
		}
	}
	sb.WriteString("ET\n")
	return sb.String()
}

func pdfEscape(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "(", "\\(")
	s = strings.ReplaceAll(s, ")", "\\)")
	return s
}

// generateDensePDF creates a PDF where each page has many paragraphs of text.
func generateDensePDF(path string, numPages int) error {
	buildStream := func(pageNum int) string {
		var sb strings.Builder
		sb.WriteString("BT\n/F1 10 Tf\n50 750 Td\n")
		sb.WriteString(fmt.Sprintf("(Page %d - Dense Content Test) Tj\n", pageNum))
		// 12 paragraphs of ~80 chars each → ~960 chars per page
		for p := 1; p <= 12; p++ {
			sb.WriteString(fmt.Sprintf(
				"0 -14 Td\n(Paragraph %d: Lorem ipsum text block number %d for page %d of the test document.) Tj\n",
				p, p, pageNum))
		}
		sb.WriteString("ET\n")
		return sb.String()
	}
	return generatePDFWithStreamFn(path, numPages, buildStream)
}

// generateSparsePDF creates a PDF where every other page is blank.
func generateSparsePDF(path string, numPages int) error {
	buildStream := func(pageNum int) string {
		if pageNum%2 == 0 {
			return "BT ET\n" // blank page: valid but empty
		}
		return fmt.Sprintf("BT\n/F1 12 Tf\n50 750 Td\n(Content on page %d.) Tj\nET\n", pageNum)
	}
	return generatePDFWithStreamFn(path, numPages, buildStream)
}

// generatePDFWithStreamFn is the generic PDF builder used by all generators.
func generatePDFWithStreamFn(path string, numPages int, streamFn func(int) string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	pos := int64(0)
	write := func(s string) { f.WriteString(s); pos += int64(len(s)) } //nolint
	writef := func(format string, args ...any) { write(fmt.Sprintf(format, args...)) }

	offsets := make([]int64, 0, 2+numPages*2)
	mark := func() { offsets = append(offsets, pos) }

	write("%PDF-1.4\n%\x80\x80\x80\x80\n")

	mark()
	write("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")

	kids := make([]string, numPages)
	for i := range kids {
		kids[i] = fmt.Sprintf("%d 0 R", 3+i*2)
	}
	mark()
	writef("2 0 obj\n<< /Type /Pages /Kids [%s] /Count %d\n   /Resources << /Font << /F1 << /Type /Font /Subtype /Type1 /BaseFont /Helvetica >> >> >> >>\nendobj\n",
		strings.Join(kids, " "), numPages)

	for i := 1; i <= numPages; i++ {
		mark()
		writef("%d 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents %d 0 R >>\nendobj\n",
			3+(i-1)*2, 4+(i-1)*2)
		stream := streamFn(i)
		mark()
		writef("%d 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj\n",
			4+(i-1)*2, len(stream), stream)
	}

	xrefPos := pos
	totalObjs := 2 + numPages*2
	writef("xref\n0 %d\n", totalObjs+1)
	write("0000000000 65535 f \n")
	for _, off := range offsets {
		writef("%010d 00000 n \n", off)
	}
	writef("trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n",
		totalObjs+1, xrefPos)
	return nil
}

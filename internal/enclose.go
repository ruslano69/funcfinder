// enclose.go - --enclose: for each grep-style hit (path:line or path:line:text), the innermost function that holds the
// line, and whether the line is code, or only a comment or a string.
package internal

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

// Hit is a grep-style location: a file and a 1-based line, with the text grep printed after it when there is any.
type Hit struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Text string `json:"text,omitempty"`
}

// Enclosure is a hit with the innermost function that holds its line. Function is empty when no function does. Kind is
// "code", "blank", "comment or string", "code with comment or string", or "unknown" when the file cannot be read or its
// language is not supported.
type Enclosure struct {
	Path     string `json:"path"`
	Line     int    `json:"line"`
	Function string `json:"function,omitempty"`
	Start    int    `json:"start,omitempty"`
	End      int    `json:"end,omitempty"`
	Kind     string `json:"kind"`
	// Documents names the function that a comment hit documents: the comment sits directly above its declaration.
	Documents string   `json:"documents,omitempty"`
	DocStart  int      `json:"doc_start,omitempty"`
	DocEnd    int      `json:"doc_end,omitempty"`
	Doc       []string `json:"doc,omitempty"`
}

// hitPattern matches path:line and path:line:text. The path is matched lazily, so a Windows drive letter (H:) stays part
// of the path: the line number has to follow the colon.
var hitPattern = regexp.MustCompile(`^(.+?):(\d+)(?::(.*))?$`)

// ParseHits reads grep-style lines and returns the locations they name. A line that is not a location is skipped.
func ParseHits(r io.Reader) []Hit {
	var hits []Hit
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024) // grep prints whole lines, minified ones can be long
	for sc.Scan() {
		m := hitPattern.FindStringSubmatch(sc.Text())
		if m == nil {
			continue
		}
		line, err := strconv.Atoi(m[2])
		if err != nil || line < 1 {
			continue
		}
		hits = append(hits, Hit{Path: m[1], Line: line, Text: m[3]})
	}
	return hits
}

// fileIndex is what EncloseHits knows about one file: its lines, the same lines with comments and strings blanked, and
// its functions. known is false when the file is not read or its language is not supported.
type fileIndex struct {
	lines     []string
	cleaned   []string
	functions []FunctionBounds
	lang      *LanguageConfig
	known     bool
}

// EncloseHits returns, for each hit, the innermost function of its file that holds the hit's line, and what the line is.
// Each file is read, cleaned and searched for functions once, however many hits it has.
func EncloseHits(config Config, hits []Hit) []Enclosure {
	cache := make(map[string]*fileIndex)
	out := make([]Enclosure, 0, len(hits))
	for _, h := range hits {
		idx, ok := cache[h.Path]
		if !ok {
			idx = indexFile(config, h.Path)
			cache[h.Path] = idx
		}
		out = append(out, idx.enclose(h))
	}
	return out
}

// indexFile reads, cleans and searches one file. A file that cannot be read or has no supported language gives an index
// with known false.
func indexFile(config Config, path string) *fileIndex {
	idx := &fileIndex{}
	lang := config.GetLanguageByExtension(path)
	if lang == nil {
		return idx
	}
	open := resolvePath(path)
	lines, err := readFileLines(open)
	if err != nil {
		return idx
	}
	idx.lines = lines
	idx.lang = lang

	sanitizer := NewSanitizer(lang, false)
	state := StateNormal
	idx.cleaned = make([]string, len(lines))
	for i, line := range lines {
		idx.cleaned[i], state = sanitizer.CleanLine(line, state)
	}

	if fr, err := CreateFinder(lang, "", "map", false, false).FindFunctions(open); err == nil {
		idx.functions = fr.Functions
	}
	idx.known = true
	return idx
}

// msysPath converts a Git Bash path (/h/dir/file.go) to the Windows form (H:/dir/file.go). It reports false for any other
// path.
func msysPath(path string) (string, bool) {
	if len(path) < 3 || path[0] != '/' || path[2] != '/' {
		return "", false
	}
	c := path[1]
	if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z') {
		return "", false
	}
	return strings.ToUpper(string(c)) + ":" + path[2:], true
}

// resolvePath returns the path to open. grep run in Git Bash prints /h/... paths, which Go on Windows cannot open; such a
// path is converted when it does not exist as it is. The hit keeps the path grep printed.
func resolvePath(path string) string {
	if runtime.GOOS != "windows" {
		return path
	}
	if _, err := os.Stat(path); err == nil {
		return path
	}
	if converted, ok := msysPath(path); ok {
		return converted
	}
	return path
}

// enclose answers one hit from the index of its file.
func (idx *fileIndex) enclose(h Hit) Enclosure {
	e := Enclosure{Path: h.Path, Line: h.Line}
	if !idx.known || h.Line > len(idx.lines) {
		e.Kind = "unknown"
		return e
	}
	e.Kind = lineKind(idx.lines[h.Line-1], idx.cleaned[h.Line-1])

	// The innermost function is the one that starts last among those that hold the line.
	var best *FunctionBounds
	for i := range idx.functions {
		fn := &idx.functions[i]
		if fn.Start <= h.Line && h.Line <= fn.End && (best == nil || fn.Start > best.Start) {
			best = fn
		}
	}
	if best != nil {
		e.Function = qualifiedName(best)
		e.Start, e.End = best.Start, best.End
		return e
	}
	if e.Kind == "comment or string" {
		if fn := idx.documentedBy(h.Line); fn != nil {
			e.Documents = qualifiedName(fn)
			e.DocStart, e.DocEnd = fn.Start, fn.End
			e.Doc = DocLines(idx.lines, fn.Start, idx.lang)
		}
	}
	return e
}

// qualifiedName is the function's name, with its class or receiver type in front when it has one.
func qualifiedName(fn *FunctionBounds) string {
	if fn.ClassName != "" {
		return fn.ClassName + "." + fn.Name
	}
	return fn.Name
}

// documentedBy returns the function that a comment at line documents. The comment must sit directly above the function's
// declaration: a blank line breaks it, as it breaks DocLines. Nil when no function is documented.
func (idx *fileIndex) documentedBy(line int) *FunctionBounds {
	for j := line + 1; j <= len(idx.lines); j++ {
		if strings.TrimSpace(idx.lines[j-1]) == "" {
			return nil
		}
		if strings.TrimSpace(idx.cleaned[j-1]) == "" {
			continue // another comment line of the same block
		}
		for i := range idx.functions {
			if idx.functions[i].Start == j {
				return &idx.functions[i]
			}
		}
		return nil
	}
	return nil
}

// lineKind says what a line is: blank, code, a comment or string only, or code with a comment or string in it. The
// cleaned line has comments removed and string contents blanked, so a difference from the raw line is a comment or string.
func lineKind(raw, cleaned string) string {
	r := strings.TrimSpace(raw)
	c := strings.TrimSpace(cleaned)
	switch {
	case r == "":
		return "blank"
	case c == "":
		return "comment or string"
	case c != r:
		return "code with comment or string"
	}
	return "code"
}

// RunEnclose reads grep-style hits from r and writes the enclosing function of each one to w, as text or as JSON.
func RunEnclose(r io.Reader, w io.Writer, asJSON bool) error {
	config, err := LoadConfig()
	if err != nil {
		return err
	}
	encl := EncloseHits(config, ParseHits(r))

	if asJSON {
		data, err := json.MarshalIndent(encl, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(w, string(data))
		return err
	}
	shown := make(map[string]bool) // a comment block is printed once, however many of its lines are hits
	for _, e := range encl {
		switch {
		case e.Kind == "unknown":
			fmt.Fprintf(w, "%s:%d  no supported language, or the line is past the end of the file  [%s]\n", e.Path, e.Line, e.Kind)
		case e.Documents != "":
			fmt.Fprintf(w, "%s:%d  comment of %s (%d-%d)  [%s]\n", e.Path, e.Line, e.Documents, e.DocStart, e.DocEnd, e.Kind)
			key := e.Path + "#" + strconv.Itoa(e.DocStart)
			if !shown[key] {
				shown[key] = true
				for _, c := range e.Doc {
					fmt.Fprintf(w, "    %s\n", c)
				}
			}
		case e.Function == "":
			fmt.Fprintf(w, "%s:%d  outside any function  [%s]\n", e.Path, e.Line, e.Kind)
		default:
			fmt.Fprintf(w, "%s:%d  in %s (%d-%d)  [%s]\n", e.Path, e.Line, e.Function, e.Start, e.End, e.Kind)
		}
	}
	return nil
}

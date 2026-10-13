// docs.go - the doc comments of named declarations (--doc): the comment above a declaration, and its first line.
package internal

import (
	"fmt"
	"sort"
	"strings"
)

// DocLines returns the comment lines directly above the declaration that starts at line start (1-based), in file order.
// Decorators and annotations directly above the declaration are skipped: the doc comment comes above them. A blank line
// or code ends the comment. A block comment (/* ... */ or /** ... */) counts when it ends on the line directly above,
// and only its lines starting with "*" are taken: any other text stops the walk.
func DocLines(lines []string, start int, lang *LanguageConfig) []string {
	var bottomUp []string // collected from the declaration upwards, reversed at the end
	i := start - 2        // the line above the declaration
	if i >= len(lines) {
		i = len(lines) - 1
	}
	i = skipAnnotations(lines, i, lang)

	for i >= 0 {
		trimmed := strings.TrimSpace(lines[i])
		switch {
		case lang.LineComment != "" && strings.HasPrefix(trimmed, lang.LineComment):
			bottomUp = append(bottomUp, trimmed)
			i--

		case lang.BlockCommentStart != "" && lang.BlockCommentEnd != "" && strings.HasSuffix(trimmed, lang.BlockCommentEnd):
			j := i
			for !strings.HasPrefix(strings.TrimSpace(lines[j]), lang.BlockCommentStart) {
				j--
				if j < 0 {
					return reverseLines(bottomUp)
				}
				above := strings.TrimSpace(lines[j])
				if !strings.HasPrefix(above, "*") && !strings.HasPrefix(above, lang.BlockCommentStart) {
					return reverseLines(bottomUp)
				}
			}
			for k := i; k >= j; k-- {
				bottomUp = append(bottomUp, strings.TrimSpace(lines[k]))
			}
			i = j - 1

		default:
			return reverseLines(bottomUp)
		}
	}
	return reverseLines(bottomUp)
}

// skipAnnotations moves i up past the decorators and attributes directly above a declaration: an `@name` line (Python,
// Java, TypeScript), and in Rust a `#[...]` attribute, which may run over several lines. A multi-line attribute is
// read from its closing `)]` or `]` line up to its `#[` opener, within 20 lines. Anything else stops the walk.
func skipAnnotations(lines []string, i int, lang *LanguageConfig) int {
	for i >= 0 {
		trimmed := strings.TrimSpace(lines[i])
		switch {
		case strings.HasPrefix(trimmed, "@"):
			i--
		case lang.LangKey == "rust" && strings.HasPrefix(trimmed, "#["):
			i--
		case lang.LangKey == "rust" && (strings.HasPrefix(trimmed, ")]") || strings.HasPrefix(trimmed, "]")):
			j := i - 1
			for j >= 0 && i-j <= 20 && !strings.HasPrefix(strings.TrimSpace(lines[j]), "#[") {
				j--
			}
			if j < 0 || i-j > 20 {
				return i
			}
			i = j - 1
		default:
			return i
		}
	}
	return i
}

// reverseLines reverses s in place and returns it.
func reverseLines(s []string) []string {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
	return s
}

// FormatDocs prints each declaration whose name is in names: functions, types and Go aliases. An entry is a header
// (path:line: kind name), the doc comment as written (none if the declaration has none), and the first line of the
// declaration. Files are in path order and entries in line order; entries are separated by a blank line. It returns ""
// when no declaration matches.
func FormatDocs(config Config, results []DirResult, names map[string]bool) (string, error) {
	results = append([]DirResult(nil), results...)
	sort.SliceStable(results, func(i, j int) bool { return results[i].Path < results[j].Path })

	var entries []string
	for _, r := range results {
		if r.Error != nil {
			continue
		}

		var decls []nameLine
		for _, fn := range r.Functions {
			if names[fn.Name] {
				decls = append(decls, nameLine{Kind: functionKind(fn), Name: fn.Name, Line: fn.Start})
			}
		}
		for _, cl := range r.Classes {
			if names[cl.Name] {
				decls = append(decls, nameLine{Kind: classKind(cl), Name: cl.Name, Line: cl.Start})
			}
		}
		for _, a := range r.Aliases {
			if names[a.Name] {
				decls = append(decls, nameLine{Kind: "alias", Name: a.Name, Line: a.Start})
			}
		}
		if len(decls) == 0 {
			continue
		}

		lang, err := config.GetLanguageConfig(r.LangKey)
		if err != nil {
			return "", err
		}
		lines, err := readFileLines(r.Path)
		if err != nil {
			return "", err
		}

		sort.SliceStable(decls, func(i, j int) bool { return decls[i].Line < decls[j].Line })
		for _, d := range decls {
			var b strings.Builder
			fmt.Fprintf(&b, "%s:%d: %s %s\n", r.Path, d.Line, d.Kind, d.Name)
			for _, c := range DocLines(lines, d.Line, lang) {
				b.WriteString(c + "\n")
			}
			if d.Line >= 1 && d.Line <= len(lines) {
				b.WriteString(strings.TrimSpace(lines[d.Line-1]) + "\n")
			}
			entries = append(entries, b.String())
		}
	}
	return strings.Join(entries, "\n"), nil
}

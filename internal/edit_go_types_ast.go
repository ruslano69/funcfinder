package internal

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

// editGoType selects TypeSpecs rather than line-based struct patterns. A group
// member owns its spec only; the surrounding type keyword/parentheses survive.
func editGoType(cfg *LanguageConfig, src []byte, target FuncTarget, old, replacement []byte, op string) (*ReplaceResult, error) {
	if cfg.LangKey != "go" {
		return nil, fmt.Errorf("type edits currently support Go only")
	}
	if target.Line < 0 {
		return nil, fmt.Errorf("line must be positive")
	}
	if bytes.Count(src, []byte("\r\n")) > 0 && bytes.Count(src, []byte("\r\n")) != bytes.Count(src, []byte("\n")) {
		return nil, fmt.Errorf("mixed line endings are not supported")
	}
	fs := token.NewFileSet()
	f, err := parser.ParseFile(fs, "", src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("source does not parse as Go: %v", err)
	}
	var group *ast.GenDecl
	var spec *ast.TypeSpec
	for _, d := range f.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, s := range gd.Specs {
			ts := s.(*ast.TypeSpec)
			pos := ts.Pos()
			if !gd.Lparen.IsValid() {
				pos = gd.Pos()
			}
			if ts.Name.Name == target.Name && (target.Line == 0 || fs.PositionFor(pos, false).Line == target.Line) {
				if spec != nil {
					return nil, fmt.Errorf("ambiguous type %s; select its declaration with --line", target.Name)
				}
				group, spec = gd, ts
			}
		}
	}
	if spec == nil {
		return nil, fmt.Errorf("no type named %s at requested line", target.Name)
	}
	grouped := group.Lparen.IsValid()
	pos, endPos := spec.Pos(), spec.End()
	if !grouped {
		pos, endPos = group.Pos(), group.End()
	}
	a, b := fs.PositionFor(pos, false), fs.PositionFor(endPos, false)
	start, end := a.Offset, b.Offset
	normal := func(s string) string { return strings.TrimSuffix(strings.ReplaceAll(s, "\r\n", "\n"), "\n") }
	lo := bytes.LastIndexByte(src[:start], '\n') + 1
	hi := end
	if n := bytes.IndexByte(src[end:], '\n'); n >= 0 {
		hi += n
	} else {
		hi = len(src)
	}
	if old != nil {
		want := stripExtractHeader(normal(string(old)), target.Name)
		if want != normal(string(src[start:end])) && want != normal(string(src[lo:hi])) {
			return nil, fmt.Errorf("type %s is not the expected text (it changed since it was read)", target.Name)
		}
	}
	var specs []*ast.TypeSpec
	payload := ""
	if op != "remove" {
		payload, err = cleanReplacement(normal(string(replacement)), target.Name)
		if err != nil {
			return nil, err
		}
		pfs := token.NewFileSet()
		const prefix = "package edit\n"
		pf, e := parser.ParseFile(pfs, "", prefix+payload+"\n", parser.ParseComments|parser.SkipObjectResolution)
		if e != nil {
			return nil, fmt.Errorf("replacement does not parse as Go: %v", e)
		}
		var members []string
		for _, d := range pf.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				return nil, fmt.Errorf("type edits accept only type declarations")
			}
			for _, s := range gd.Specs {
				ts := s.(*ast.TypeSpec)
				specs = append(specs, ts)
				members = append(members, payload[pfs.Position(ts.Pos()).Offset-len(prefix):pfs.Position(ts.End()).Offset-len(prefix)])
			}
		}
		if len(specs) == 0 {
			return nil, fmt.Errorf("replacement defines no types")
		}
		if op == "replace" && (len(specs) != 1 || specs[0].Name.Name != target.Name) {
			return nil, fmt.Errorf("replacement must define exactly type %s", target.Name)
		}
		if op == "replace" && !grouped && len(pf.Decls) != 1 {
			return nil, fmt.Errorf("replacement must contain one declaration")
		}
		if grouped {
			for _, comment := range pf.Comments {
				contained := false
				for _, member := range specs {
					if member.Pos() <= comment.Pos() && comment.End() <= member.End() {
						contained = true
						break
					}
				}
				if !contained {
					return nil, fmt.Errorf("group member replacement cannot relocate declaration comments; keep them outside the edited spec")
				}
			}
			payload = strings.Join(members, "; ")
		}
		if op == "insert" {
			seen := map[string]bool{}
			for _, d := range f.Decls {
				switch d := d.(type) {
				case *ast.FuncDecl:
					if d.Recv == nil {
						seen[d.Name.Name] = true
					}
				case *ast.GenDecl:
					for _, s := range d.Specs {
						switch s := s.(type) {
						case *ast.TypeSpec:
							seen[s.Name.Name] = true
						case *ast.ValueSpec:
							for _, n := range s.Names {
								seen[n.Name] = true
							}
						}
					}
				}
			}
			for _, s := range specs {
				if seen[s.Name.Name] {
					return nil, fmt.Errorf("duplicate declaration %s", s.Name.Name)
				}
				seen[s.Name.Name] = true
			}
		}
		if bytes.Contains(src, []byte("\r\n")) {
			payload = strings.ReplaceAll(payload, "\n", "\r\n")
		}
	}
	left, right := start, end
	semi := goFollowingSemicolon(src, end)
	ownLine := !grouped && strings.TrimSpace(string(src[lo:start])) == "" && strings.TrimSpace(string(src[end:hi])) == ""
	switch op {
	case "insert":
		left, right = end, end
		if ownLine {
			payload = "\n\n" + payload
			if bytes.Contains(src, []byte("\r\n")) {
				payload = strings.ReplaceAll(strings.ReplaceAll(payload, "\r\n", "\n"), "\n", "\r\n")
			}
		} else if semi >= 0 {
			left, right = semi+1, semi+1
			payload = " " + payload + ";"
		} else {
			payload = "; " + payload + ";"
		}
	case "remove":
		if semi >= 0 {
			payload = string(src[end:semi])
			right = semi + 1
		}
		if ownLine {
			left, right = lo, hi
			if right < len(src) {
				right++
			}
			for right < len(src) {
				n := bytes.IndexByte(src[right:], '\n')
				if n < 0 {
					break
				}
				if strings.TrimSpace(string(src[right:right+n])) != "" {
					break
				}
				right += n + 1
			}
		}
	}
	out := append([]byte(nil), src[:left]...)
	out = append(out, payload...)
	out = append(out, src[right:]...)
	af, e := parser.ParseFile(token.NewFileSet(), "", out, parser.ParseComments|parser.SkipObjectResolution)
	if e != nil {
		return nil, fmt.Errorf("edit does not give a valid Go file: %v", e)
	}
	// Compare flattened type identities and complete untouched declarations.
	var expected []string
	for _, d := range f.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			expected = append(expected, goSharedIdentity(d))
			continue
		}
		for _, s := range gd.Specs {
			ts := s.(*ast.TypeSpec)
			if ts == spec {
				if op == "insert" {
					expected = append(expected, "type "+ts.Name.Name)
				}
				if op != "remove" {
					for _, n := range specs {
						expected = append(expected, "type "+n.Name.Name)
					}
				}
			} else {
				expected = append(expected, "type "+ts.Name.Name)
			}
		}
	}
	var actual []string
	for _, d := range af.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			actual = append(actual, goSharedIdentity(d))
			continue
		}
		for _, s := range gd.Specs {
			actual = append(actual, "type "+s.(*ast.TypeSpec).Name.Name)
		}
	}
	if !sameNames(expected, actual) {
		return nil, fmt.Errorf("edit changes neighboring declarations")
	}
	res := &ReplaceResult{Start: a.Line, End: b.Line, Output: out}
	if op != "insert" {
		res.Removed = strings.Split(normal(string(src[left:right])), "\n")
	}
	if op != "remove" {
		res.Added = strings.Split(normal(payload), "\n")
		res.NewStart = bytes.Count(src[:left], []byte("\n")) + 1
		res.NewEnd = res.NewStart + strings.Count(payload, "\n")
	}
	return res, nil
}

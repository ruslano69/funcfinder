package internal

import "regexp"

// AliasBounds is a Go type alias: `type A = S`, or a line `A = S` of a `type ( ... )` group. It names an existing
// type and is not a type of its own, so it is listed apart from the types: one struct with several names is one struct.
type AliasBounds struct {
	Name   string // the alias name
	Target string // the type it names, as written after '='
	Start  int    // line (1-based)
}

var (
	goAliasDeclRe     = regexp.MustCompile(`^\s*type\s+(\w+)\s*=\s*(.*?)\s*(?://.*)?$`)
	goAliasMemberRe   = regexp.MustCompile(`^\s*(\w+)\s*=\s*(.*?)\s*(?://.*)?$`)
	goTypeGroupOpenRe = regexp.MustCompile(`^\s*type\s*\(\s*$`)
	goGroupCloseRe    = regexp.MustCompile(`^\s*\)\s*$`)
)

// findGoAliases returns the type aliases of a Go file: single-line `type A = S` declarations and the `A = S` lines
// of `type ( ... )` groups. An assignment outside a type group is not an alias.
func findGoAliases(lines []string) []AliasBounds {
	var aliases []AliasBounds
	inGroup := false
	for i, line := range lines {
		switch {
		case inGroup && goGroupCloseRe.MatchString(line):
			inGroup = false
		case inGroup:
			if m := goAliasMemberRe.FindStringSubmatch(line); m != nil {
				aliases = append(aliases, AliasBounds{Name: m[1], Target: m[2], Start: i + 1})
			}
		case goTypeGroupOpenRe.MatchString(line):
			inGroup = true
		default:
			if m := goAliasDeclRe.FindStringSubmatch(line); m != nil {
				aliases = append(aliases, AliasBounds{Name: m[1], Target: m[2], Start: i + 1})
			}
		}
	}
	return aliases
}

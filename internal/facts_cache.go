package internal

// A directory job runs several finders on the same lines of a file: in --all the function finder and the struct
// finder, and for Go the alias reader. Each cleaned the whole file again; the cleaning was a third of a TypeScript run,
// and half of it was repeated. A factsCache keeps the facts of the job's lines, so the later finders read them.
//
// Sharing gives the same results because CleanLinesFacts depends only on the language, the raw setting and the
// lines: it starts every file in the normal state, and the delimiter a sanitizer keeps from an earlier file is set
// again by the next literal before it is read. TestCleanLinesFactsIgnoresEarlierFiles checks that.
type factsCache struct {
	lines  []string
	config *LanguageConfig
	useRaw bool
	facts  []LineFacts
}

// factsOf returns the facts of lines for s: the kept facts when they were computed for the same lines (the same
// slice) with the same settings, or new facts, which are kept instead.
func (c *factsCache) factsOf(s *Sanitizer, lines []string) []LineFacts {
	if len(lines) == 0 {
		return s.cleanLinesFacts(lines)
	}
	if c.facts != nil && c.config == s.config && c.useRaw == s.useRaw && len(c.lines) == len(lines) && &c.lines[0] == &lines[0] {
		return c.facts
	}
	c.lines, c.config, c.useRaw = lines, s.config, s.useRaw
	c.facts = s.cleanLinesFacts(lines)
	return c.facts
}

// dirJobSharing turns prepareDirJob off, for the test that compares a directory run with and without it.
var dirJobSharing = true

// prepareDirJob sets a finder (or sanitizer) of a directory job to share the job's facts and, for a struct finder, to
// skip the fields of each type: a directory result holds only the types. Other finders are left as they are.
func prepareDirJob(finder any, cache *factsCache) {
	if !dirJobSharing {
		return
	}
	switch f := finder.(type) {
	case *Sanitizer:
		f.shared = cache
	case *Finder:
		f.sanitizer.shared = cache
	case *StructFinder:
		f.sanitizer.shared = cache
		f.noFields = true
	case *HybridStructFinder:
		f.sanitizer.shared = cache
		f.noFields = true
	case *PythonStructFinder:
		f.sanitizer.shared = cache
		f.noFields = true
	}
}

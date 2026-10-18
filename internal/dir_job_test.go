package internal

import (
	"io/fs"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// readFileLines reads the whole file at once; its lines must be forEachLine's, which keeps bufio.Scanner's rules.
func TestReadFileLinesMatchesForEachLine(t *testing.T) {
	contents := []string{
		"", "\n", "\r", "\r\n", "\r\r\n", "\n\n", "a", "a\n", "a\r", "a\r\n", "a\r\r", "a\rb\n", "a\n\nb", "a\nb\r",
		"\x00\xff\xfe\n\xc3", "é\r\nж\n", strings.Repeat("x", 200_000) + "\r\n" + strings.Repeat("y", 70_000),
		strings.Repeat("line\r\n", 5000),
	}
	for i, content := range contents {
		path := writeTempFile(t, "f.txt", []byte(content))
		got, err := readFileLines(path)
		if err != nil {
			t.Fatal(err)
		}
		var want []string
		if err := forEachLine(strings.NewReader(content), func(line string) bool {
			want = append(want, line)
			return true
		}); err != nil {
			t.Fatal(err)
		}
		if len(got) != len(want) || (len(want) > 0 && !reflect.DeepEqual(got, want)) {
			t.Errorf("case %d: readFileLines gives %d lines, forEachLine %d", i, len(got), len(want))
		}
	}
}

// fixtureFilesByLanguage returns the fixture and source files of the repository, by language.
func fixtureFilesByLanguage(t *testing.T, cfg Config) map[*LanguageConfig][][]string {
	t.Helper()
	files := map[*LanguageConfig][][]string{}
	err := filepath.WalkDir("..", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); name == "build" || name == "node_modules" || name == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		lc := cfg.GetLanguageByExtension(path)
		if lc == nil {
			return nil
		}
		lines, err := readFileLines(path)
		if err != nil {
			return err
		}
		files[lc] = append(files[lc], lines)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// A directory job shares one file's facts between its finders (factsCache), which is right only if CleanLinesFacts
// gives the same facts whatever the sanitizer cleaned before. For every language, each fixture file is cleaned after
// each other fixture file of its language and after lines that leave a literal or comment open, and compared with a
// new sanitizer's facts.
func TestCleanLinesFactsIgnoresEarlierFiles(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	openers := [][]string{{`x = "abc`}, {"x = `abc ${a"}, {`r#"abc`}, {`"""abc`}, {`'''abc`}, {`@"abc`}, {`'abc`},
		{`/* abc`}, {`/+ abc /+`}, {`q{abc`}, {`<<<EOT`, `abc`}, {`%q(abc`}, {`=begin`}, {`#[[abc`}, {`R"x(abc`}}
	checked := 0
	for lc, files := range fixtureFilesByLanguage(t, cfg) {
		earlier := append(append([][]string(nil), files...), openers...)
		for _, file := range files {
			want := NewSanitizer(lc, false).CleanLinesFacts(file)
			for _, before := range earlier {
				s := NewSanitizer(lc, false)
				s.CleanLinesFacts(before)
				if got := s.CleanLinesFacts(file); !reflect.DeepEqual(got, want) {
					t.Fatalf("%s: the facts of a file depend on the file cleaned before it (%.40q)", lc.LangKey, before[0])
				}
				checked++
			}
		}
	}
	if checked == 0 {
		t.Fatal("no file was checked")
	}
}

// A directory run that shares facts between its finders and skips struct fields gives the results of one that does
// neither, in every work mode.
func TestDirJobSharingKeepsResults(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	run := func(mode string, share bool) []DirResult {
		dirJobSharing = share
		defer func() { dirJobSharing = true }()
		var all []DirResult
		for _, dir := range []string{"../test_examples", "../test_files", "."} {
			results, err := NewDirProcessor(cfg, 1, true, false, mode).ProcessDirectory(dir)
			if err != nil {
				t.Fatal(err)
			}
			all = append(all, results...)
		}
		sort.Slice(all, func(i, j int) bool { return all[i].Path < all[j].Path })
		return all
	}
	for _, mode := range []string{"functions", "structs", "all"} {
		got, want := run(mode, true), run(mode, false)
		if len(got) != len(want) {
			t.Fatalf("%s: %d results with sharing, %d without", mode, len(got), len(want))
		}
		for i := range want {
			if !reflect.DeepEqual(got[i], want[i]) {
				t.Errorf("%s: %s differs with sharing", mode, want[i].Path)
			}
		}
	}
}

package main

import "testing"

func TestCallCountsGooseDeclarationsAndPayloads(t *testing.T) {
	counts := callCountsOf(t, "goose", "sample.goose", `fn Alpha() { } fn Beta() { }
recursive fn recur(n: i64) -> i64 { if n > 0 { return recur(n - 1); } return 0; }
fn
split() { }
extern fn native(n: i64);
let text = """
    fn Hidden() { Ghost(); }
    """;
/* outer /* nested */ Comment(); */
fn main() { Alpha(); Beta(); split(); native(1); }`)
	for name, want := range map[string]int{"Alpha": 1, "Beta": 1, "recur": 1, "split": 1, "native": 1, "main": 0, "Hidden": 0, "Ghost": 0, "Comment": 0} {
		if counts[name] != want {
			t.Errorf("%s count=%d want=%d; calls=%v", name, counts[name], want, counts)
		}
	}
}

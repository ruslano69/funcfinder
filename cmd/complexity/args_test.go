package main

import (
	"reflect"
	"testing"
)

// A flag that takes a value keeps the value next to it, wherever the positional path sits. The --breakdown limits are
// such flags: a value that is split from its flag makes the parser fail, or silently moves the number to the paths.
func TestReorderArgsKeepsFlagValues(t *testing.T) {
	in := []string{"internal/finder.go", "-len", "200", "-loop", "0", "-breakdown", "-n", "3", "-if", "12", "-deep", "5"}
	want := []string{"-len", "200", "-loop", "0", "-breakdown", "-n", "3", "-if", "12", "-deep", "5", "internal/finder.go"}
	if got := reorderArgs(in); !reflect.DeepEqual(got, want) {
		t.Errorf("reorderArgs = %v\nwant %v", got, want)
	}
}

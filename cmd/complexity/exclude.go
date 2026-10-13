package main

import "strings"

// stringList is a repeatable string flag: -exclude "*_test.go" -exclude "gen/*.go".
type stringList []string

// String is the flag.Value form of the list.
func (s *stringList) String() string {
	return strings.Join(*s, ",")
}

// Set appends one occurrence of the flag.
func (s *stringList) Set(value string) error {
	*s = append(*s, value)
	return nil
}

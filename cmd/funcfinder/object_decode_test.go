package main

import (
	"strings"
	"testing"
)

func TestDecodeCodeEditEnvelopes(t *testing.T) {
	for _, tc := range []struct {
		json   string
		legacy bool
	}{
		{`{"protocol":"funcfinder.code/v1","source":"func A() {}"}`, true},
		{`{"operation":"remove","object":{"protocol":"funcfinder.code/v1"}}`, false},
		{`{"protocol":"funcfinder.edits/v1","edits":[{"operation":"remove","object":{"protocol":"funcfinder.code/v1"}}]}`, false},
	} {
		edits, legacy, err := decodeCodeEdits(strings.NewReader(tc.json))
		if err != nil || legacy != tc.legacy || len(edits) != 1 {
			t.Fatalf("%s: %v", tc.json, err)
		}
	}
	for _, bad := range []string{
		`{"operation":"remove","object":{},"ignored":1}`,
		`{"protocol":"funcfinder.edits/v1","edits":[]}`,
		`{"protocol":"wrong","edits":[{}]}`,
		`{"protocol":"funcfinder.edits/v1","edits":[{"operation":"remove","object":{},"ignored":1}]}`,
		`{} {}`, `null`, `[]`,
	} {
		if _, _, err := decodeCodeEdits(strings.NewReader(bad)); err == nil {
			t.Fatalf("accepted %s", bad)
		}
	}
}

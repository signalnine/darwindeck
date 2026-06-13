package main

import (
	"reflect"
	"testing"
)

// TestSplitPositional verifies flags can appear before OR after positionals,
// including value-flags that consume the next token. This guards the
// `judge emit <input> --out <dir>` ordering (flag after positional), which the
// bare flag package would otherwise stop parsing at the positional.
func TestSplitPositional(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		wantPos  []string
		wantFlag []string
	}{
		{
			name:     "flag after positional",
			args:     []string{"/in", "--out", "/dir"},
			wantPos:  []string{"/in"},
			wantFlag: []string{"--out", "/dir"},
		},
		{
			name:     "flag before positional",
			args:     []string{"--out", "/dir", "/in"},
			wantPos:  []string{"/in"},
			wantFlag: []string{"--out", "/dir"},
		},
		{
			name:     "equals form",
			args:     []string{"/in", "--out=/dir"},
			wantPos:  []string{"/in"},
			wantFlag: []string{"--out=/dir"},
		},
		{
			name:     "two positionals with trailing flag (rank)",
			args:     []string{"/dossiers", "/v.json", "--out", "/r.md"},
			wantPos:  []string{"/dossiers", "/v.json"},
			wantFlag: []string{"--out", "/r.md"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pos, fl := splitPositional(tc.args)
			if !reflect.DeepEqual(pos, tc.wantPos) {
				t.Errorf("positional = %v, want %v", pos, tc.wantPos)
			}
			if !reflect.DeepEqual(fl, tc.wantFlag) {
				t.Errorf("flagArgs = %v, want %v", fl, tc.wantFlag)
			}
		})
	}
}

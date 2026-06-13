package judge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
)

func writeSeed(t *testing.T, dir, name string, g *genome.Genome) string {
	t.Helper()
	sub := filepath.Join(dir, name)
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(sub, "genome.json")
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestEmitProducesBlindDossiers verifies emit discovers genomes recursively,
// writes neutral dossiers + manifest + prompt, and returns a private answer
// key with neutral-id keys.
func TestEmitProducesBlindDossiers(t *testing.T) {
	in := t.TempDir()
	out := t.TempDir()

	// Two seeds in nested run-dir-like subdirs.
	pa := writeSeed(t, in, "alpha", seeds.CrazyEights())
	pb := writeSeed(t, in, "beta/games/rank01", seeds.GinRummy())

	sources := map[string]GameSource{
		pa: {Path: pa, Source: "classic", TrueName: "Crazy Eights", PriorVerdict: "real"},
		pb: {Path: pb, Source: "classic", TrueName: "Gin Rummy", PriorVerdict: "false-positive-degenerate"},
	}

	res, err := Emit(in, out, sources)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if len(res.IDs) != 2 {
		t.Fatalf("got %d ids, want 2", len(res.IDs))
	}

	// Dossiers exist and have Termination sections.
	for _, id := range res.IDs {
		data, err := os.ReadFile(filepath.Join(out, id+".md"))
		if err != nil {
			t.Fatalf("read dossier %s: %v", id, err)
		}
		if !strings.Contains(string(data), "## Termination") {
			t.Errorf("%s missing Termination section", id)
		}
	}

	// Manifest + prompt exist.
	if _, err := os.Stat(filepath.Join(out, "manifest.json")); err != nil {
		t.Errorf("manifest.json missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "prompt.md")); err != nil {
		t.Errorf("prompt.md missing: %v", err)
	}

	// Answer key keyed by neutral id, carrying the source + prior verdict.
	if len(res.AnswerKey) != 2 {
		t.Fatalf("answer key has %d entries, want 2", len(res.AnswerKey))
	}
	foundFalsePositive := false
	for id, rec := range res.AnswerKey {
		if !strings.HasPrefix(id, "G") {
			t.Errorf("answer key id %q is not neutral", id)
		}
		if rec.TrueName == "" {
			t.Errorf("%s missing true_name", id)
		}
		if rec.PriorValidationVerdict == "false-positive-degenerate" {
			foundFalsePositive = true
		}
	}
	if !foundFalsePositive {
		t.Error("answer key dropped the prior-validation verdict")
	}
}

// TestEmitDossiersAreLeakFree confirms the blind dossiers contain no game
// names or metric vocabulary -- the core blindness guarantee.
func TestEmitDossiersAreLeakFree(t *testing.T) {
	in := t.TempDir()
	out := t.TempDir()
	writeSeed(t, in, "g1", seeds.GinRummy())
	writeSeed(t, in, "g2", seeds.CrazyEights())
	writeSeed(t, in, "g3", seeds.Whist())

	if _, err := Emit(in, out, nil); err != nil {
		t.Fatalf("Emit: %v", err)
	}

	// Leak terms: game names + metric words. Suit/card words are allowed, so we
	// check whole-word game names and metric tokens only.
	leaks := []string{
		"gin", "knock", "crazy", "mau", "whist", "spades-game",
		"oh hell", "oh-hell", "wild union",
		"fitness", "veto", "skill=", "coverage",
	}
	matches, err := filepath.Glob(filepath.Join(out, "G*.md"))
	if err != nil || len(matches) == 0 {
		t.Fatalf("no dossiers emitted: %v", err)
	}
	for _, m := range matches {
		data, err := os.ReadFile(m)
		if err != nil {
			t.Fatal(err)
		}
		lower := strings.ToLower(string(data))
		for _, leak := range leaks {
			if strings.Contains(lower, leak) {
				t.Errorf("%s leaks %q", filepath.Base(m), leak)
			}
		}
	}
}

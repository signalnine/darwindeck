package judge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/grammar"
)

// TestBuildGrammarDossierIsLegibleAndBlind: a grammar dossier must read like a
// real game's rulebook (legibility was v2's bottleneck) AND leak nothing -- no
// composition key, no grammar internals, no metrics.
func TestBuildGrammarDossierIsLegibleAndBlind(t *testing.T) {
	spec := grammar.GameSpec{
		Players: 4, Deal: 7, Shared: 1, Move: grammar.PlayMatch, Match: grammar.MatchEither,
		End: grammar.EmptyHand, Score: grammar.FirstOut, Mods: []grammar.Modifier{grammar.ModKnock},
	}
	d, err := BuildGrammarDossier(spec, "G07")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# G07", "## How to Play", "## Special Rules", "KNOCK", "## Sample Game Traces", "## Termination"} {
		if !strings.Contains(d, want) {
			t.Errorf("dossier missing %q", want)
		}
	}
	// Blind: must NOT leak the composition key or grammar internals.
	for _, leak := range []string{spec.Composition(), "play_match", "modifier", "GameSpec", "well-typed"} {
		if strings.Contains(d, leak) {
			t.Errorf("dossier leaks %q (should be blind)", leak)
		}
	}
}

// TestEmitGrammarWritesBlindSet: EmitGrammar writes one dossier per spec plus the
// manifest, prompt, and an answer key whose true_name is the composition.
func TestEmitGrammarWritesBlindSet(t *testing.T) {
	dir := t.TempDir()
	specs := grammar.Canonical()
	res, err := EmitGrammar(specs, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.IDs) != len(specs) {
		t.Fatalf("emitted %d ids, want %d", len(res.IDs), len(specs))
	}
	for _, name := range []string{"manifest.json", "prompt.md", "G01.md"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("missing %s", name)
		}
	}
	// answer key keyed by neutral id, true_name = composition
	if rec, ok := res.AnswerKey["G01"]; !ok || rec.TrueName != specs[0].Composition() {
		t.Errorf("answer key G01 true_name = %q, want %q", rec.TrueName, specs[0].Composition())
	}
	// manifest is blind (no true_name field on disk)
	data, _ := os.ReadFile(filepath.Join(dir, "manifest.json"))
	var manifest []ManifestEntry
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest) != len(specs) {
		t.Errorf("manifest has %d entries, want %d", len(manifest), len(specs))
	}
}

// TestEmitGrammarManifestHandSize: the manifest must report the hand a player is
// actually dealt. It used to copy SpecGenome's HandSize, which is floored at 8 to
// keep the engine's turn cap sane -- so a 7-card shedding game and a 0-card
// banking game both read "hand size 8".
func TestEmitGrammarManifestHandSize(t *testing.T) {
	dir := t.TempDir()
	specs := grammar.Canonical()
	if _, err := EmitGrammar(specs, dir); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "manifest.json"))
	var manifest []ManifestEntry
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	for i, m := range manifest {
		if m.HandSize != specs[i].Deal {
			t.Errorf("%s (%s): manifest hand size = %d, want the real deal %d", m.ID, specs[i].Family(), m.HandSize, specs[i].Deal)
		}
		if m.Players != specs[i].Players {
			t.Errorf("%s: manifest players = %d, want %d", m.ID, m.Players, specs[i].Players)
		}
	}
}

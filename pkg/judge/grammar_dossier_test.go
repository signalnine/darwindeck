package judge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
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
	for _, want := range []string{"# G07", "## How to Play", "## Special Rules", "DECLARE OUT", "## Sample Game Traces", "## Termination"} {
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

// gameNameLeak matches vocabulary that names (or all but names) a published game.
// A blind novelty dossier that says "KNOCK" or "poker hand" tells the judge which
// classic it is looking at before it has read a rule. Suit names (hearts, spades)
// and generic mechanic words (trump, meld, trick, wild, raise, fold) are fine.
var gameNameLeak = regexp.MustCompile(`(?i)\b(knock\w*|gin|poker|rummy|deadwood|crazy eights|uno|scopa|casino|blackjack|big two|president|whist|bridge|euchre|pinochle|tichu)\b`)

// TestGrammarDossiersAreNameBlind: grammar dossiers wrote spec.Rulebook raw,
// skipping the neutralization every v2 dossier gets -- "KNOCK"/"knocking" in 33
// of 137 dossiers, "five-card poker hand" in every betting one. The whole dossier
// (rulebook AND traces, where the going-out event used to read "knock") must be
// free of game-name vocabulary, for every family.
func TestGrammarDossiersAreNameBlind(t *testing.T) {
	leaky := 0
	for _, spec := range grammar.EnumerateModified() {
		// Every family's rulebook, through the same neutralizer the dossier uses.
		if m := gameNameLeak.FindString(neutralizeRulebook(spec.Rulebook("G00"))); m != "" {
			if leaky++; leaky <= 5 {
				t.Errorf("%s: rulebook leaks %q", spec.Composition(), m)
			}
		}
	}
	if leaky > 5 {
		t.Errorf("... %d families leak in total", leaky)
	}
	// Full dossiers (rulebook + sample traces) for the families that used to leak.
	knock := []grammar.Modifier{grammar.ModKnock}
	for _, spec := range []grammar.GameSpec{
		{Players: 3, Deal: 7, Shared: 1, Move: grammar.PlayMatch, Match: grammar.MatchEither, End: grammar.EmptyHand, Score: grammar.FirstOut, Mods: knock},
		{Players: 3, Deal: 7, Move: grammar.BeatOrPass, End: grammar.EmptyHand, Score: grammar.FirstOut, Mods: knock},
		{Players: 2, Deal: 10, Move: grammar.Rummy, End: grammar.DeckOut, Score: grammar.FewestDeadwood, Mods: knock},
		{Players: 4, Deal: 5, Move: grammar.Vying, End: grammar.Showdown, Score: grammar.BestHand},
	} {
		d, err := BuildGrammarDossier(spec, "G00")
		if err != nil {
			t.Fatal(err)
		}
		if m := gameNameLeak.FindString(d); m != "" {
			t.Errorf("%s: dossier leaks %q", spec.Composition(), m)
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
	// answer key keyed by neutral id, true_name = composition; every spec is
	// covered exactly once.
	seen := map[string]bool{}
	for _, id := range res.IDs {
		seen[res.AnswerKey[id].TrueName] = true
	}
	for _, spec := range specs {
		if !seen[spec.Composition()] {
			t.Errorf("answer key has no id for composition %q", spec.Composition())
		}
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
	res, err := EmitGrammar(specs, dir)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "manifest.json"))
	var manifest []ManifestEntry
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	// Ids are permuted, so the answer key links a manifest row to its spec.
	byComp := map[string]grammar.GameSpec{}
	for _, s := range specs {
		byComp[s.Composition()] = s
	}
	for _, m := range manifest {
		spec, ok := byComp[res.AnswerKey[m.ID].TrueName]
		if !ok {
			t.Fatalf("%s: answer key names no emitted spec", m.ID)
		}
		if m.HandSize != spec.Deal {
			t.Errorf("%s (%s): manifest hand size = %d, want the real deal %d", m.ID, spec.Family(), m.HandSize, spec.Deal)
		}
		if m.Players != spec.Players {
			t.Errorf("%s: manifest players = %d, want %d", m.ID, m.Players, spec.Players)
		}
	}
}

// TestEmitGrammarPermutesIDs: ids used to follow the enumeration order, so
// G01..Gnn walked the families move-generator by move-generator and a judge
// could read structure off the id alone (the v2 emit leak, grammar side). Ids
// are a deterministic pseudo-random permutation of the input set, and the
// dossier written under an id is the game the answer key names for it.
func TestEmitGrammarPermutesIDs(t *testing.T) {
	specs := grammar.EnumerateModified()[:24]
	res, err := EmitGrammar(specs, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	inOrder := true
	for i, id := range res.IDs {
		if want := fmt.Sprintf("G%02d", i+1); id != want {
			t.Fatalf("ids[%d] = %s, want %s", i, id, want)
		}
		if res.AnswerKey[id].TrueName != specs[i].Composition() {
			inOrder = false
		}
	}
	if inOrder {
		t.Error("dossier ids follow the enumeration order, so the id leaks the family order")
	}

	// Deterministic for a fixed set, whatever order the caller passes it in.
	rev := make([]grammar.GameSpec, len(specs))
	for i, s := range specs {
		rev[len(specs)-1-i] = s
	}
	again, err := EmitGrammar(rev, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(res.AnswerKey, again.AnswerKey) {
		t.Error("the id assignment depends on the caller's spec order")
	}

	// The dossier under an id is the game its answer-key record names.
	byComp := map[string]grammar.GameSpec{}
	for _, s := range specs {
		byComp[s.Composition()] = s
	}
	dir := t.TempDir()
	res2, err := EmitGrammar(specs, dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range res2.IDs {
		want, err := BuildGrammarDossier(byComp[res2.AnswerKey[id].TrueName], id)
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(filepath.Join(dir, id+".md"))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != want {
			t.Errorf("%s.md is not the dossier of the composition its answer key names", id)
		}
	}
}

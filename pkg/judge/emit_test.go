package judge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/evolution"
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

// knockBorrowGenome returns a Tier-0-valid shedding genome carrying the
// MechKnock deep borrow, whose rulebook renders the knock borrow rule text --
// the path that used to leak the "knock" token into a blind dossier.
func knockBorrowGenome(t *testing.T) *genome.Genome {
	t.Helper()
	g := seeds.CrazyEights()
	g.ID = "knock-borrow"
	g.Borrowed = []genome.BorrowedMechanic{{Source: genome.Rummy, Mechanic: genome.MechKnock}}
	if errs := genome.Validate(g); len(errs) > 0 {
		t.Fatalf("knock-borrow genome fails Tier-0 validation: %v", errs)
	}
	return g
}

// scoredVyingGenome returns a Tier-0-valid vying genome with a scoring borrow,
// so the rulebook renders writeVyingRules' VyingScored branch ("A strong poker
// hand ... a poker-weak hand ..."), which the plain SimplePoker seed does not
// exercise.
func scoredVyingGenome(t *testing.T) *genome.Genome {
	t.Helper()
	g := seeds.SimplePoker()
	g.ID = "scored-vying"
	g.Borrowed = []genome.BorrowedMechanic{{Source: genome.Rummy, Mechanic: genome.MechMeldBonus}}
	if errs := genome.Validate(g); len(errs) > 0 {
		t.Fatalf("scored-vying genome fails Tier-0 validation: %v", errs)
	}
	return g
}

// TestEmitDossiersAreLeakFree confirms the blind dossiers contain no game
// names or metric vocabulary -- the core blindness guarantee.
func TestEmitDossiersAreLeakFree(t *testing.T) {
	in := t.TempDir()
	out := t.TempDir()
	writeSeed(t, in, "g1", seeds.GinRummy())
	writeSeed(t, in, "g2", seeds.CrazyEights())
	writeSeed(t, in, "g3", seeds.Whist())
	writeSeed(t, in, "g4", seeds.BigTwo())
	writeSeed(t, in, "g5", seeds.Casino())
	writeSeed(t, in, "g6", seeds.SimplePoker())
	writeSeed(t, in, "g7", knockBorrowGenome(t))
	writeSeed(t, in, "g8", scoredVyingGenome(t))

	if _, err := Emit(in, out, nil); err != nil {
		t.Fatalf("Emit: %v", err)
	}

	// Leak terms: game names + metric words. Suit/card words are allowed, so we
	// check whole-word game names and metric tokens only. The climbing-skeleton
	// rulebook used to advertise "climbing game (Big Two / Tichu / President
	// family)"; neutralizeRulebook now strips both the family names and the
	// "climbing" keyword, so guard against a regression here. Likewise the
	// casino rulebook named its "(Casino / Scopa family)", the vying rulebook
	// leaked "poker" and "big blind", and the MechKnock borrow rule leaked
	// "knock" -- all now neutralized.
	leaks := []string{
		"gin", "knock", "crazy", "mau", "whist", "spades-game",
		"oh hell", "oh-hell", "wild union",
		"big two", "tichu", "president", "climbing",
		"casino", "scopa", "poker", "blind",
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

// TestSortIDsNumericIsNumeric: IDs are G%02d, so widths diverge at 100 and a
// lexicographic compare ordered G100 before G11.
func TestSortIDsNumericIsNumeric(t *testing.T) {
	ids := []string{"G100", "G02", "G11", "G9", "G001"}
	SortIDsNumeric(ids)
	want := []string{"G001", "G02", "G9", "G11", "G100"}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("order = %v, want %v", ids, want)
		}
	}
}

// TestCleanStaleDossiers: re-emitting a smaller set into a dir that held a
// larger one left stale G(N+1)+.md dossiers beside a manifest that no longer
// listed them -- a judge globbing *.md scored dossiers whose IDs resolved to
// the wrong answer-key entries. The sweep that fixed it removed EVERY *.md in
// the output dir, so `judge emit <run> --out <run>` deleted the run's
// README.md and a re-emit deleted the judged-report.md `judge rank` writes
// there. It must remove dossiers (G<digits>.md) and nothing else.
func TestCleanStaleDossiers(t *testing.T) {
	dir := t.TempDir()
	stale := []string{"G01.md", "G07.md", "G123.md"}
	keep := []string{"README.md", "judged-report.md", "prompt.md", "Gnotes.md", "G01.md.bak", "xG01.md", "manifest.json"}
	for _, name := range append(append([]string{}, stale...), keep...) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := cleanStaleDossiers(dir); err != nil {
		t.Fatal(err)
	}
	for _, name := range stale {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			t.Errorf("stale dossier %s survived the sweep", name)
		}
	}
	for _, name := range keep {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("non-dossier file %s was removed by the sweep: %v", name, err)
		}
	}
}

// emitSeedSet writes every classic seed to <in>/<seed-id>/genome.json and
// emits the set, returning the result and the output dir.
func emitSeedSet(t *testing.T) (EmitResult, string, string) {
	t.Helper()
	in, out := t.TempDir(), t.TempDir()
	for _, g := range seeds.All() {
		writeSeed(t, in, g.ID, g)
	}
	res, err := Emit(in, out, nil)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	return res, in, out
}

// TestEmitPermutesIDsDeterministically: IDs were assigned in sorted-path
// order, so the id sequence WAS the answer key's order -- classics came out
// alphabetical by true name (G01 = big-two ... G11 = whist) and a run dir came
// out in fitness-rank order (G01 = rank01). A judge (or anyone reading the
// verdicts) could read rank and identity off the id. The order must be a
// pseudo-random function of the input set: stable across re-emits, unrelated
// to the path order.
func TestEmitPermutesIDsDeterministically(t *testing.T) {
	res, _, _ := emitSeedSet(t)
	again, _, _ := emitSeedSet(t)

	want := make([]string, len(res.IDs))
	for i := range want {
		want[i] = fmt.Sprintf("G%02d", i+1)
	}
	if !reflect.DeepEqual(res.IDs, want) {
		t.Fatalf("ids = %v, want %v", res.IDs, want)
	}

	var names []string
	for _, id := range res.IDs {
		names = append(names, res.AnswerKey[id].TrueName)
	}
	if sort.StringsAreSorted(names) {
		t.Errorf("dossier ids follow the sorted source order, so the id leaks the answer-key order: %v", names)
	}
	seen := map[string]bool{}
	for _, n := range names {
		seen[n] = true
	}
	if len(seen) != len(seeds.All()) {
		t.Errorf("answer key covers %d distinct games, want %d: %v", len(seen), len(seeds.All()), names)
	}
	if !reflect.DeepEqual(res.AnswerKey, again.AnswerKey) {
		t.Errorf("the id assignment is not deterministic for a fixed input set:\n%v\n%v", res.AnswerKey, again.AnswerKey)
	}
}

// TestEmitAnswerKeyMatchesDossierContent: with permuted ids the answer key is
// the ONLY link from an id to its game, so it must point at exactly the genome
// whose dossier was written under that id.
func TestEmitAnswerKeyMatchesDossierContent(t *testing.T) {
	res, in, out := emitSeedSet(t)
	for _, id := range res.IDs {
		rec := res.AnswerKey[id]
		// Path (relative to the input dir) is the unambiguous link: true_name
		// is only a directory name, and two run dirs can both hold a rank01.
		g, err := loadGenome(filepath.Join(in, filepath.FromSlash(rec.Path)))
		if err != nil {
			t.Fatalf("%s: answer key path %q does not resolve to a source genome: %v", id, rec.Path, err)
		}
		if want := evolution.Composition(g); rec.Composition != want {
			t.Errorf("%s: answer key composition %q, want %q (the -judge-verdicts table key)", id, rec.Composition, want)
		}
		g.ID = id
		want, err := BuildDossier(g)
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(filepath.Join(out, id+".md"))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != want {
			t.Errorf("%s.md is not the dossier of the game the answer key names (%s)", id, rec.TrueName)
		}
		if rec.Skeleton != g.Skeleton.String() {
			t.Errorf("%s: answer key skeleton %q, want %q", id, rec.Skeleton, g.Skeleton)
		}
	}
}

// TestManifestOmitsSkeleton: manifest.json sits INSIDE the blind dossier dir,
// and it listed every game's skeleton -- "casino" is a classic's name outright,
// and "climbing" / "rummy" / "shedding" / "vying" are the family labels the
// rulebook neutralizer exists to keep out of the judge's view. The skeleton
// belongs in the private answer key only.
func TestManifestOmitsSkeleton(t *testing.T) {
	res, _, out := emitSeedSet(t)
	raw, err := os.ReadFile(filepath.Join(out, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(raw))
	for _, leak := range []string{"skeleton", "shedding", "trick_taking", "rummy", "climbing", "casino", "vying"} {
		if strings.Contains(lower, leak) {
			t.Errorf("manifest.json (inside the blind dir) contains %q", leak)
		}
	}
	var manifest []ManifestEntry
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest) != len(res.IDs) {
		t.Fatalf("manifest has %d entries, want %d", len(manifest), len(res.IDs))
	}
	for i, m := range manifest {
		if m.ID != res.IDs[i] || m.Dossier != res.IDs[i]+".md" || m.Players == 0 {
			t.Errorf("manifest entry %d = %+v, want id %s with its dossier and player count", i, m, res.IDs[i])
		}
	}
	for _, id := range res.IDs {
		if res.AnswerKey[id].Skeleton == "" {
			t.Errorf("%s: the private answer key lost the skeleton", id)
		}
	}
}

// TestInferSourceLabelsClassics: with no explicit sources, every genome was
// labeled source "champion" -- including the 11 classic seeds, the ground-truth
// anchors a judged set is calibrated against. A genome that IS a built-in
// classic (whatever its id / fitness stamps / directory name) is "classic"
// with the seed's real name; anything else is not.
func TestInferSourceLabelsClassics(t *testing.T) {
	in, out := t.TempDir(), t.TempDir()
	stamped := seeds.GinRummy()
	stamped.ID = "gen12_3456"
	stamped.Generation = 12
	stamped.Fitness = 0.61
	stamped.VetoStable = true
	stamped.StableEvals = "5/5"
	writeSeed(t, in, "rank03_gen12_3456", stamped)
	writeSeed(t, in, "anchor-b", seeds.BigTwo())
	writeSeed(t, in, "rank01_gen40_77", knockBorrowGenome(t))

	res, err := Emit(in, out, nil)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	got := map[string]AnswerRec{}
	for _, rec := range res.AnswerKey {
		got[rec.TrueName] = rec
	}
	for _, name := range []string{"gin-rummy", "big-two"} {
		if rec, ok := got[name]; !ok || rec.Source != "classic" {
			t.Errorf("classic %s not labeled as such: answer key = %+v", name, res.AnswerKey)
		}
	}
	if rec, ok := got["rank01_gen40_77"]; !ok || rec.Source == "classic" {
		t.Errorf("an evolved (non-seed) genome must keep its run-dir name and a non-classic source: %+v", res.AnswerKey)
	}
}

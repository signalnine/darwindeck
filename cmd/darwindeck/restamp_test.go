package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/output"
	"github.com/darwindeck/darwindeck/pkg/seeds"
)

// restampRun builds a saved-run directory holding one published classic with
// stale stamps on it, and returns the run dir and the genome.json path.
func restampRun(t *testing.T, root string) (runDir, genomePath string) {
	t.Helper()
	runDir = filepath.Join(root, "results", "flag")
	g := seeds.CrazyEights()
	g.ID = "gen7_1234"
	g.Fitness = 0.918
	g.SharedFitness = 0.3296 // the ORIGINAL run's niche-sharing blend
	g.VetoStable = true
	g.StableEvals = "5/5"
	genomePath = putGenome(t, filepath.Join(runDir, "games"), "rank01_gen7_1234", g)
	return runDir, genomePath
}

func rankDirs(t *testing.T, bundle string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(bundle, "games"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// TestRestampRefusesInPlace: the default out dir is results/<run-basename>, so
// `restamp results/<run>` -- restamping a published bundle, the command's own
// use case -- resolved out == run and overwrote the input in place: every
// genome.json lost its original fitness and the source meta/summary were
// replaced by the restamp's.
func TestRestampRefusesInPlace(t *testing.T) {
	runDir, genomePath := restampRun(t, t.TempDir())
	before, err := os.ReadFile(genomePath)
	if err != nil {
		t.Fatal(err)
	}

	for _, out := range []string{
		runDir,
		runDir + string(filepath.Separator),
		filepath.Join(runDir, "..", "flag"),
	} {
		var stdout, stderr bytes.Buffer
		err := runRestamp(runDir, out, &stdout, &stderr)
		if err == nil {
			t.Errorf("restamp with out %q == run dir was not refused", out)
		}
	}
	after, err := os.ReadFile(genomePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Error("the input genome.json was modified by a restamp of its own directory")
	}
	if _, err := os.Stat(filepath.Join(runDir, "STABILITY.md")); err == nil {
		t.Error("the refused restamp still wrote into the run dir")
	}

	// The documented default resolves to the run dir itself for a run that
	// already lives under results/, and must be refused the same way.
	run := filepath.Join("results", "flag")
	if err := checkRestampDirs(run, restampOutDir(run, "")); err == nil {
		t.Error("the default out dir of `restamp results/<run>` is the run dir and was not refused")
	}
	if err := checkRestampDirs(filepath.Join("output", "flag"), restampOutDir(filepath.Join("output", "flag"), "")); err != nil {
		t.Errorf("the default out dir of `restamp output/<run>` (results/<run>) is a different dir: %v", err)
	}
}

// TestRestampEmptyGamesIsError: a run dir with an empty (or genome-less)
// games/ produced an empty bundle and exit 0.
func TestRestampEmptyGamesIsError(t *testing.T) {
	root := t.TempDir()
	runDir := filepath.Join(root, "run")
	if err := os.MkdirAll(filepath.Join(runDir, "games", "rank01_x"), 0o755); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, "bundle")
	var stdout, stderr bytes.Buffer

	if err := runRestamp(runDir, out, &stdout, &stderr); err == nil {
		t.Fatalf("restamp of a run with no genomes was not an error; stdout: %s", stdout.String())
	}
	if _, err := os.Stat(out); err == nil {
		t.Error("restamp of a run with no genomes still wrote a bundle")
	}
}

// TestRestampDropsStaleSharedFitness: restamp re-derives fitness and the
// stability stamps but copied shared_fitness -- the ORIGINAL run's
// niche-sharing blend, relative to a population that no longer exists --
// straight into the restamped genome.json, next to a fitness it has nothing
// to do with.
func TestRestampDropsStaleSharedFitness(t *testing.T) {
	root := t.TempDir()
	runDir, _ := restampRun(t, root)
	out := filepath.Join(root, "bundle")
	var stdout, stderr bytes.Buffer

	if err := runRestamp(runDir, out, &stdout, &stderr); err != nil {
		t.Fatalf("restamp: %v", err)
	}
	names := rankDirs(t, out)
	if len(names) != 1 {
		t.Fatalf("bundle games = %v, want one", names)
	}
	raw, err := os.ReadFile(filepath.Join(out, "games", names[0], "genome.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	if v, ok := fields["shared_fitness"]; ok {
		t.Errorf("restamped genome.json carries the source run's shared_fitness %s", v)
	}
	if string(fields["stable_evals"]) == `"5/5"` && !strings.Contains(stdout.String(), "1 veto-stable") {
		t.Errorf("stale stability stamp survived: %s", raw)
	}
	if _, ok := fields["stable_evals"]; !ok {
		t.Errorf("restamped genome.json has no stable_evals stamp: %s", raw)
	}
}

// TestRestampSweepsStaleRankDirs: a second restamp into the same bundle left
// the previous bundle's rank dirs beside the new ones (two rank01_*, a
// summary.json counting fewer games than the directory holds).
func TestRestampSweepsStaleRankDirs(t *testing.T) {
	root := t.TempDir()
	runDir, _ := restampRun(t, root)
	out := filepath.Join(root, "bundle")
	putGenome(t, filepath.Join(out, "games"), "rank01_gen1_old", seeds.GinRummy())
	putGenome(t, filepath.Join(out, "games"), "rank02_gen2_old", seeds.Whist())
	putFile(t, filepath.Join(out, "games", "NOTES.txt"), "kept")
	var stdout, stderr bytes.Buffer

	if err := runRestamp(runDir, out, &stdout, &stderr); err != nil {
		t.Fatalf("restamp: %v", err)
	}
	names := rankDirs(t, out)
	want := []string{"NOTES.txt", "rank01_gen7_1234"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("bundle games dir = %v, want %v (stale rank dirs swept, foreign files kept)", names, want)
	}
}

// TestRankRestampGamesDemotesUnstable: every veto-stable game must outrank
// every unstable game, regardless of fitness -- the publication-integrity
// invariant the Wave M restamp enforces (the r4 rank02 case: a high-fitness
// single-eval publication that fails its own veto on fresh seeds sinks to the
// bottom).
func TestRankRestampGamesDemotesUnstable(t *testing.T) {
	g := func(id string) *genome.Genome { return &genome.Genome{ID: id, Skeleton: genome.Shedding} }
	games := []restampGame{
		{genome: g("unstable-hi"), greedyMean: 0.95, stability: output.StabilityResult{ValidCount: 1, Total: 5, Stable: false}},
		{genome: g("stable-lo"), greedyMean: 0.40, stability: output.StabilityResult{ValidCount: 5, Total: 5, Stable: true}},
		{genome: g("stable-hi"), greedyMean: 0.70, stability: output.StabilityResult{ValidCount: 4, Total: 5, Stable: true}},
		{genome: g("unstable-lo"), greedyMean: 0.10, stability: output.StabilityResult{ValidCount: 0, Total: 5, Stable: false}},
	}
	rankRestampGames(games)

	wantOrder := []string{"stable-hi", "stable-lo", "unstable-hi", "unstable-lo"}
	for i, want := range wantOrder {
		if games[i].genome.ID != want {
			t.Errorf("rank %d: got %s want %s", i+1, games[i].genome.ID, want)
		}
	}
	// The high-fitness UNSTABLE game (0.95) must be below both stable games
	// (0.70, 0.40): stability outranks fitness.
	if games[0].stability.Stable != true || games[2].genome.ID != "unstable-hi" {
		t.Errorf("unstable-hi (0.95) was not demoted below the stable games")
	}
}

// TestRestampRefusesInPlaceThroughSymlink (2026-10-02 bughunt review): the
// out == run guard compared cleaned absolute paths, so an out dir that is a
// symlink to the run dir slipped through and the restamp overwrote (and, with
// the stale-rank-dir sweep, deleted from) its own input.
func TestRestampRefusesInPlaceThroughSymlink(t *testing.T) {
	root := t.TempDir()
	runDir, genomePath := restampRun(t, root)
	before, err := os.ReadFile(genomePath)
	if err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "alias")
	if err := os.Symlink(runDir, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	var stdout, stderr bytes.Buffer
	if err := runRestamp(runDir, link, &stdout, &stderr); err == nil {
		t.Error("restamp into a symlink to the run dir was not refused")
	}
	after, err := os.ReadFile(genomePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Error("the input genome.json was modified through the symlinked out dir")
	}
}

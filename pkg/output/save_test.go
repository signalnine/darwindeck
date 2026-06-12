package output

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/darwindeck/darwindeck/pkg/evolution"
	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
)

// publishedIndividual builds an Individual whose Genome.Fitness DISAGREES
// with its published Fitness.TotalFitness -- the r2 flagship's rank04 bug:
// novelty-archive entries shared Genome pointers with live individuals, so a
// later re-evaluation overwrote the genome's Fitness while the archived
// snapshot's metrics stayed frozen (report.md 0.847 vs genome.json 0.808).
func publishedIndividual() *evolution.Individual {
	g := seeds.CrazyEights()
	g.Fitness = 0.808        // stale running mean written by a later eval
	g.SharedFitness = 0.300  // stale blend
	ind := &evolution.Individual{
		Genome:     g,
		Valid:      true,
		EvalCount:  2,
		FitnessSum: 1.6, // greedy mean 0.800
		MctsSum:    0.847,
		MctsCount:  1,
	}
	ind.Fitness = fitness.Metrics{
		MeaningfulDecisions: 0.2,
		GameArc:             0.8,
		Interaction:         0.4,
		SkillGradient:       0.1,
		SessionLength:       1.0,
		TotalFitness:        0.847, // published (MCTS-mode mean)
		SharedFitness:       0.555,
	}
	return ind
}

// TestSaveResultsReportMatchesGenomeJSON (round 3 commit 5a): the fitness
// number report.md's header renders and the fitness genome.json stores MUST
// be the same number -- the published individual's TotalFitness -- no matter
// how stale the genome's own Fitness field is at save time.
func TestSaveResultsReportMatchesGenomeJSON(t *testing.T) {
	dir := t.TempDir()
	ind := publishedIndividual()

	if err := SaveResults(dir, []*evolution.Individual{ind}, evolution.Config{BaseSeed: 42}, time.Second); err != nil {
		t.Fatalf("SaveResults: %v", err)
	}

	games, err := filepath.Glob(filepath.Join(dir, "games", "rank01_*"))
	if err != nil || len(games) != 1 {
		t.Fatalf("expected one rank01 dir, got %v (%v)", games, err)
	}

	raw, err := os.ReadFile(filepath.Join(games[0], "genome.json"))
	if err != nil {
		t.Fatalf("reading genome.json: %v", err)
	}
	var saved genome.Genome
	if err := json.Unmarshal(raw, &saved); err != nil {
		t.Fatalf("parsing genome.json: %v", err)
	}

	report, err := os.ReadFile(filepath.Join(games[0], "report.md"))
	if err != nil {
		t.Fatalf("reading report.md: %v", err)
	}
	m := regexp.MustCompile(`Fitness ([0-9.]+)`).FindStringSubmatch(string(report))
	if m == nil {
		t.Fatalf("report.md has no fitness header:\n%s", report)
	}
	headerFit, _ := strconv.ParseFloat(m[1], 64)

	if want := ind.Fitness.TotalFitness; saved.Fitness != want {
		t.Errorf("genome.json fitness = %v, want the published TotalFitness %v", saved.Fitness, want)
	}
	if diff := headerFit - saved.Fitness; diff > 0.0005 || diff < -0.0005 {
		t.Errorf("report.md header %.3f != genome.json fitness %.6f", headerFit, saved.Fitness)
	}
	if saved.SharedFitness != ind.Fitness.SharedFitness {
		t.Errorf("genome.json shared_fitness = %v, want %v", saved.SharedFitness, ind.Fitness.SharedFitness)
	}

	// The caller's genome must not be mutated by publication (the save
	// stamps a clone).
	if ind.Genome.Fitness != 0.808 {
		t.Errorf("SaveResults mutated the caller's genome (fitness now %v)", ind.Genome.Fitness)
	}
}

// TestSaveResultsWritesMetaJSON (round 3 commit 5b, the Task 4 convention +
// hazard 3): every results bundle must carry a meta.json recording how it
// was produced -- commit, go version, platform, cli args, master seed,
// calibration seeds, date, the MCTS mode and knobs, the veto thresholds, and
// the fitness floor.
func TestSaveResultsWritesMetaJSON(t *testing.T) {
	dir := t.TempDir()
	cfg := evolution.Config{
		PopulationSize: 50,
		Generations:    10,
		BaseSeed:       1234,
		MCTSDecile:     0.10,
	}
	if err := SaveResults(dir, []*evolution.Individual{publishedIndividual()}, cfg, time.Second); err != nil {
		t.Fatalf("SaveResults: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "meta.json"))
	if err != nil {
		t.Fatalf("meta.json not written: %v", err)
	}
	var meta map[string]any
	if err := json.Unmarshal(raw, &meta); err != nil {
		t.Fatalf("meta.json is not valid JSON: %v", err)
	}

	for _, key := range []string{
		"commit_sha", "go_version", "platform", "cli_args", "master_seed",
		"calibration_seeds", "date", "mcts_mode", "mcts_decile",
		"veto_thresholds", "fitness_floor",
	} {
		if _, ok := meta[key]; !ok {
			t.Errorf("meta.json missing %q", key)
		}
	}
	if meta["mcts_mode"] != "top-decile" {
		t.Errorf("mcts_mode = %v, want top-decile at MCTSDecile 0.10", meta["mcts_mode"])
	}
	if meta["master_seed"] != float64(1234) {
		t.Errorf("master_seed = %v, want 1234", meta["master_seed"])
	}
	vt, ok := meta["veto_thresholds"].(map[string]any)
	if !ok || len(vt) == 0 {
		t.Errorf("veto_thresholds must be a non-empty map, got %v", meta["veto_thresholds"])
	}

	// Greedy-only mode is recorded as such.
	dir2 := t.TempDir()
	cfg.MCTSDecile = 0
	if err := SaveResults(dir2, nil, cfg, time.Second); err != nil {
		t.Fatalf("SaveResults greedy-only: %v", err)
	}
	raw2, _ := os.ReadFile(filepath.Join(dir2, "meta.json"))
	var meta2 map[string]any
	if err := json.Unmarshal(raw2, &meta2); err != nil {
		t.Fatalf("meta.json (greedy-only) invalid: %v", err)
	}
	if meta2["mcts_mode"] != "greedy-only" {
		t.Errorf("mcts_mode = %v, want greedy-only at MCTSDecile 0", meta2["mcts_mode"])
	}
}

// TestReportShowsBothFitnessMeans (round 3 commit 5c): the published fitness
// of a decile-granted genome is the MCTS-mode mean, which can exceed the
// weighted sum of the displayed component metrics by a large margin (+0.177
// measured on skill-0.00 r2 champions -- carried hazard 1 realized). For
// transparency the report must show BOTH means and the gap explicitly.
func TestReportShowsBothFitnessMeans(t *testing.T) {
	dir := t.TempDir()
	ind := publishedIndividual()
	if err := SaveResults(dir, []*evolution.Individual{ind}, evolution.Config{}, time.Second); err != nil {
		t.Fatalf("SaveResults: %v", err)
	}
	games, _ := filepath.Glob(filepath.Join(dir, "games", "rank01_*"))
	raw, err := os.ReadFile(filepath.Join(games[0], "report.md"))
	if err != nil {
		t.Fatalf("reading report.md: %v", err)
	}
	report := string(raw)

	for _, want := range []string{
		"0.847", // published / MCTS-mode mean
		"0.800", // greedy-only mean
		"+0.047", // the explicit gap
	} {
		if !strings.Contains(report, want) {
			t.Errorf("report.md missing %q (both means + gap must be explicit):\n%s", want, report)
		}
	}

	// A greedy-only individual (no MCTS evals) reports a single mean and no
	// phantom gap line.
	dir2 := t.TempDir()
	plain := publishedIndividual()
	plain.MctsSum, plain.MctsCount = 0, 0
	plain.Fitness.TotalFitness = 0.800
	if err := SaveResults(dir2, []*evolution.Individual{plain}, evolution.Config{}, time.Second); err != nil {
		t.Fatalf("SaveResults: %v", err)
	}
	games2, _ := filepath.Glob(filepath.Join(dir2, "games", "rank01_*"))
	raw2, _ := os.ReadFile(filepath.Join(games2[0], "report.md"))
	if strings.Contains(string(raw2), "MCTS-mode mean") {
		t.Errorf("greedy-only report must not claim an MCTS-mode mean:\n%s", raw2)
	}
}

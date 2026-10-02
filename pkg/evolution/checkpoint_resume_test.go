package evolution

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/seeds"
)

// engineSnapshot renders every piece of NoveltyEngine state a resumed run
// must reproduce: each individual's genome (ID + content hash), validity,
// both running-mean accumulators, descriptor, and selection scores, plus the
// archive and the adaptive admission threshold. %v on float64 is the shortest
// round-trip form, so equal strings mean bit-equal floats.
func engineSnapshot(e *NoveltyEngine) string {
	var sb strings.Builder
	line := func(tag string, i int, ind *NoveltyIndividual) {
		fmt.Fprintf(&sb, "%s[%d] %s %s valid=%v evals=%d sum=%v mcts=%d/%v beh=%v fit=%v shared=%v nov=%v cid=%v\n",
			tag, i, ind.Genome.ID, genomeHash(ind.Genome), ind.Valid, ind.EvalCount, ind.FitnessSum,
			ind.MctsCount, ind.MctsSum, ind.Behavior, ind.Fitness.TotalFitness, ind.Fitness.SharedFitness,
			ind.Novelty, ind.CID)
	}
	for i, ind := range e.Population {
		line("pop", i, ind)
	}
	for i, ind := range e.Archive {
		line("arch", i, ind)
	}
	fmt.Fprintf(&sb, "gen=%d best=%v threshold=%v\n", e.Generation, e.BestFitness, e.addThreshold)
	return sb.String()
}

// TestChunkedResumeEqualsUninterruptedRun: N generations straight must equal
// N/2 generations + checkpoint + resume in a FRESH engine (the judge-in-loop
// path: chunks are separate processes). LoadCheckpoint used to re-seed the
// mutation RNG from (BaseSeed, Generation) instead of restoring the stream,
// so the two diverged from the chunk boundary on -- a 4-generation CLI run
// reported best fitness 0.759 straight vs 0.959 chunked, with different
// top-10s. The RNG was the ONLY state lost (restoring it alone made the runs
// bit-equal), so the checkpoint now carries the PCG state.
//
// Hybrid only: the baseline and MAP-Elites engines have no checkpoint path
// (the evolve command warns and ignores -checkpoint for them).
func TestChunkedResumeEqualsUninterruptedRun(t *testing.T) {
	cfg := checkpointTestConfig()
	cfg.CrossSkeleton = true // exercise the hybrid-crossover RNG draws too

	straight := NewNoveltyEngine(cfg, allSeeds())
	if done := straight.RunChunk(nil, cfg.Generations); !done {
		t.Fatal("uninterrupted run did not complete")
	}

	first := NewNoveltyEngine(cfg, allSeeds())
	if done := first.RunChunk(nil, cfg.Generations/2); done {
		t.Fatal("first chunk reported done")
	}
	path := filepath.Join(t.TempDir(), "ck.json")
	if err := first.SaveCheckpoint(path); err != nil {
		t.Fatalf("SaveCheckpoint: %v", err)
	}
	resumed := NewNoveltyEngine(cfg, allSeeds())
	if err := resumed.LoadCheckpoint(path); err != nil {
		t.Fatalf("LoadCheckpoint: %v", err)
	}
	if done := resumed.RunChunk(nil, cfg.Generations); !done {
		t.Fatal("resumed run did not complete")
	}

	want, got := engineSnapshot(straight), engineSnapshot(resumed)
	if got != want {
		t.Errorf("chunked+resumed run diverged from the uninterrupted run\n--- uninterrupted ---\n%s--- resumed ---\n%s", want, got)
	}
}

// stripCheckpointFields rewrites a checkpoint file without the named
// top-level and Config fields, reproducing a checkpoint written before those
// fields existed.
func stripCheckpointFields(t *testing.T, path string, top, config []string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	for _, k := range top {
		delete(doc, k)
	}
	var cfg map[string]json.RawMessage
	if err := json.Unmarshal(doc["Config"], &cfg); err != nil {
		t.Fatal(err)
	}
	for _, k := range config {
		delete(cfg, k)
	}
	if doc["Config"], err = json.Marshal(cfg); err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestLoadCheckpointLegacyWithoutRNGState: a checkpoint written before the
// RNG state and the seed-pool fingerprint were persisted must still load --
// even under a seed pool the current fingerprint would reject, since a legacy
// file cannot say which pool wrote it -- and must fall back to the old
// deterministic re-seed from (BaseSeed, Generation).
func TestLoadCheckpointLegacyWithoutRNGState(t *testing.T) {
	cfg := checkpointTestConfig()
	e1 := NewNoveltyEngine(cfg, allSeeds())
	if done := e1.RunChunk(nil, 2); done {
		t.Fatal("chunk to generation 2 of 4 reported done")
	}
	path := filepath.Join(t.TempDir(), "ck.json")
	if err := e1.SaveCheckpoint(path); err != nil {
		t.Fatalf("SaveCheckpoint: %v", err)
	}
	stripCheckpointFields(t, path, []string{"RNGState"}, []string{"SeedPoolHash"})

	e2 := NewNoveltyEngine(cfg, seeds.All())
	if err := e2.LoadCheckpoint(path); err != nil {
		t.Fatalf("legacy checkpoint must stay loadable: %v", err)
	}
	if e2.Generation != 2 || len(e2.Population) != cfg.PopulationSize {
		t.Fatalf("legacy load restored generation %d / population %d", e2.Generation, len(e2.Population))
	}
	want := rand.New(rand.NewPCG(cfg.BaseSeed, 2)).Uint64()
	if got := e2.rng.Uint64(); got != want {
		t.Errorf("legacy checkpoint must fall back to the (BaseSeed, Generation) re-seed: first draw %d, want %d", got, want)
	}
	if done := e2.RunChunk(nil, cfg.Generations); !done {
		t.Error("legacy checkpoint did not run to completion after resume")
	}
}

// TestCheckpointRestoresRNGStream: a current-format checkpoint restores the
// EXACT mutation stream, not a re-seed.
func TestCheckpointRestoresRNGStream(t *testing.T) {
	cfg := checkpointTestConfig()
	e1 := NewNoveltyEngine(cfg, allSeeds())
	e1.initialize() // consumes RNG draws
	path := filepath.Join(t.TempDir(), "ck.json")
	if err := e1.SaveCheckpoint(path); err != nil {
		t.Fatalf("SaveCheckpoint: %v", err)
	}
	e2 := NewNoveltyEngine(cfg, allSeeds())
	if err := e2.LoadCheckpoint(path); err != nil {
		t.Fatalf("LoadCheckpoint: %v", err)
	}
	for i := 0; i < 4; i++ {
		if a, b := e1.rng.Uint64(), e2.rng.Uint64(); a != b {
			t.Fatalf("draw %d after resume = %d, want the saved stream's %d", i, b, a)
		}
	}
}

// TestLoadCheckpointRejectsSeedPoolMismatch: the seed pool feeds population
// init, changeSkeleton mutation and dedupParent through rng.IntN(len(seeds)),
// so it is stream-determining exactly like the knobs already fingerprinted.
// A gen-2 checkpoint used to resume happily under `-seed-dir` with a
// different pool (11 -> 21 seeds).
func TestLoadCheckpointRejectsSeedPoolMismatch(t *testing.T) {
	cfg := checkpointTestConfig()
	e1 := NewNoveltyEngine(cfg, allSeeds())
	e1.initialize()
	path := filepath.Join(t.TempDir(), "ck.json")
	if err := e1.SaveCheckpoint(path); err != nil {
		t.Fatalf("SaveCheckpoint: %v", err)
	}

	same := NewNoveltyEngine(cfg, allSeeds())
	if err := same.LoadCheckpoint(path); err != nil {
		t.Fatalf("LoadCheckpoint with the identical seed pool: %v", err)
	}

	bigger := NewNoveltyEngine(cfg, seeds.All())
	if err := bigger.LoadCheckpoint(path); err == nil {
		t.Error("larger seed pool: LoadCheckpoint succeeded, want config error")
	} else if !strings.Contains(err.Error(), "different config") {
		t.Errorf("larger seed pool: error %q does not mention the config mismatch", err)
	}

	// Same size, one seed's rules changed.
	edited := allSeeds()
	edited[0].HandSize++
	if err := NewNoveltyEngine(cfg, edited).LoadCheckpoint(path); err == nil {
		t.Error("edited seed: LoadCheckpoint succeeded, want config error")
	}

	// Same seeds in a different order index differently under rng.IntN.
	reordered := allSeeds()
	reordered[0], reordered[1] = reordered[1], reordered[0]
	if err := NewNoveltyEngine(cfg, reordered).LoadCheckpoint(path); err == nil {
		t.Error("reordered seed pool: LoadCheckpoint succeeded, want config error")
	}
}

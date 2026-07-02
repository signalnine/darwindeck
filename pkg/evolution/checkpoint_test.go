package evolution

import (
	"path/filepath"
	"strings"
	"testing"
)

// checkpointTestConfig is a small, MCTS-free config so the checkpoint tests
// run whole generations in well under a second each. MCTSDecile is 0: the
// decile pass is orthogonal to checkpointing and dominates wall time.
func checkpointTestConfig() Config {
	return Config{
		PopulationSize: 10,
		Generations:    4,
		EliteSize:      2,
		TournamentSize: 3,
		Workers:        4,
		BaseSeed:       42,
		SaveTopN:       5,
	}
}

// TestCheckpointRoundTrip: Save/Load must carry the WHOLE engine state --
// population (including the running-mean accumulators) and archive -- across
// a process boundary, since losing either is exactly the failure mode the
// checkpoint exists to prevent (the elite-only -seed-dir restart loop).
func TestCheckpointRoundTrip(t *testing.T) {
	cfg := checkpointTestConfig()
	e1 := NewNoveltyEngine(cfg, allSeeds())
	if done := e1.RunChunk(nil, 2); done {
		t.Fatal("chunk to generation 2 of 4 reported done")
	}
	// Archive admission is stochastic at this scale; guarantee at least one
	// entry so the archive round-trip is actually exercised.
	if len(e1.Archive) == 0 {
		arch := *e1.Population[0]
		arch.Genome = e1.Population[0].Genome.Clone()
		arch.Novelty = 0.75
		e1.Archive = append(e1.Archive, &arch)
	}

	path := filepath.Join(t.TempDir(), "ck.json")
	if err := e1.SaveCheckpoint(path); err != nil {
		t.Fatalf("SaveCheckpoint: %v", err)
	}

	e2 := NewNoveltyEngine(cfg, allSeeds())
	if err := e2.LoadCheckpoint(path); err != nil {
		t.Fatalf("LoadCheckpoint: %v", err)
	}

	if e2.Generation != e1.Generation {
		t.Errorf("Generation = %d, want %d", e2.Generation, e1.Generation)
	}
	if e2.BestFitness != e1.BestFitness {
		t.Errorf("BestFitness = %v, want %v", e2.BestFitness, e1.BestFitness)
	}
	if e2.addThreshold != e1.addThreshold {
		t.Errorf("addThreshold = %v, want %v", e2.addThreshold, e1.addThreshold)
	}
	if len(e2.Population) != len(e1.Population) {
		t.Fatalf("population size = %d, want %d", len(e2.Population), len(e1.Population))
	}
	for i := range e1.Population {
		a, b := e1.Population[i], e2.Population[i]
		if a.Genome.ID != b.Genome.ID {
			t.Errorf("population[%d] genome ID = %q, want %q", i, b.Genome.ID, a.Genome.ID)
		}
		if a.EvalCount != b.EvalCount || a.FitnessSum != b.FitnessSum {
			t.Errorf("population[%d] running mean = (%d, %v), want (%d, %v)",
				i, b.EvalCount, b.FitnessSum, a.EvalCount, a.FitnessSum)
		}
		if a.Behavior != b.Behavior {
			t.Errorf("population[%d] behavior = %v, want %v", i, b.Behavior, a.Behavior)
		}
		if a.Fitness.TotalFitness != b.Fitness.TotalFitness {
			t.Errorf("population[%d] fitness = %v, want %v", i, b.Fitness.TotalFitness, a.Fitness.TotalFitness)
		}
	}
	if len(e2.Archive) != len(e1.Archive) {
		t.Fatalf("archive size = %d, want %d", len(e2.Archive), len(e1.Archive))
	}
	for i := range e1.Archive {
		a, b := e1.Archive[i], e2.Archive[i]
		if a.Genome.ID != b.Genome.ID || a.Behavior != b.Behavior || a.Novelty != b.Novelty {
			t.Errorf("archive[%d] = (%q, %v, %v), want (%q, %v, %v)",
				i, b.Genome.ID, b.Behavior, b.Novelty, a.Genome.ID, a.Behavior, a.Novelty)
		}
	}
}

// TestLoadCheckpointRejectsConfigMismatch: resuming under different
// stream-determining knobs would splice two evaluation streams into one
// running mean, so LoadCheckpoint must fail loudly instead.
func TestLoadCheckpointRejectsConfigMismatch(t *testing.T) {
	cfg := checkpointTestConfig()
	e1 := NewNoveltyEngine(cfg, allSeeds())
	e1.initialize()
	path := filepath.Join(t.TempDir(), "ck.json")
	if err := e1.SaveCheckpoint(path); err != nil {
		t.Fatalf("SaveCheckpoint: %v", err)
	}

	// Same config resumes fine.
	same := NewNoveltyEngine(cfg, allSeeds())
	if err := same.LoadCheckpoint(path); err != nil {
		t.Fatalf("LoadCheckpoint with identical config: %v", err)
	}

	mismatches := map[string]Config{}
	c := cfg
	c.PopulationSize = 99
	mismatches["population size"] = c
	c = cfg
	c.Generations = 40
	mismatches["generations target"] = c
	c = cfg
	c.BaseSeed = 7
	mismatches["base seed"] = c
	c = cfg
	c.NoveltySelect = true
	mismatches["novelty-select flag"] = c
	c = cfg
	c.CrossSkeleton = true
	mismatches["cross-skeleton flag"] = c

	for name, mc := range mismatches {
		e2 := NewNoveltyEngine(mc, allSeeds())
		err := e2.LoadCheckpoint(path)
		if err == nil {
			t.Errorf("%s mismatch: LoadCheckpoint succeeded, want config error", name)
			continue
		}
		if !strings.Contains(err.Error(), "different config") {
			t.Errorf("%s mismatch: error %q does not mention the config mismatch", name, err)
		}
	}
}

// TestResumeAfterCompleteNoDoubleEval: a checkpoint saved after the final
// evaluation (Generation == Generations) must resume as a NO-OP. Re-running
// would repeat evaluatePopulation at the SAME (BaseSeed, Generation, idx)
// seeds and add a duplicate sample into every FitnessSum running mean.
func TestResumeAfterCompleteNoDoubleEval(t *testing.T) {
	cfg := checkpointTestConfig()
	cfg.Generations = 2
	e1 := NewNoveltyEngine(cfg, allSeeds())
	if done := e1.RunChunk(nil, cfg.Generations); !done {
		t.Fatal("full run did not report done")
	}
	path := filepath.Join(t.TempDir(), "ck.json")
	if err := e1.SaveCheckpoint(path); err != nil {
		t.Fatalf("SaveCheckpoint: %v", err)
	}

	e2 := NewNoveltyEngine(cfg, allSeeds())
	if err := e2.LoadCheckpoint(path); err != nil {
		t.Fatalf("LoadCheckpoint: %v", err)
	}
	type accum struct {
		evalCount  int
		fitnessSum float64
	}
	before := make([]accum, len(e2.Population))
	for i, ind := range e2.Population {
		before[i] = accum{ind.EvalCount, ind.FitnessSum}
	}

	if done := e2.RunChunk(nil, cfg.Generations); !done {
		t.Fatal("resume-after-complete did not report done")
	}
	for i, ind := range e2.Population {
		if ind.EvalCount != before[i].evalCount || ind.FitnessSum != before[i].fitnessSum {
			t.Errorf("population[%d] re-evaluated on resume-after-complete: (%d, %v) -> (%d, %v)",
				i, before[i].evalCount, before[i].fitnessSum, ind.EvalCount, ind.FitnessSum)
		}
	}
}

// TestChunkedResumeDeterminism tests what LoadCheckpoint's contract actually
// claims: the mutation RNG is re-seeded deterministically from (BaseSeed,
// Generation) and the per-genome evaluation seeds derive from
// BaseSeed+generation, so RESUMING THE SAME CHECKPOINT is bit-reproducible --
// two resumes of one checkpoint produce identical populations and best
// fitness.
//
// Deliberately NOT pinned: chunked (2+2) == unchunked (4 straight). That
// stronger property does NOT hold, by design: the resume re-seeds a FRESH PCG
// at the chunk boundary ("without repeating the previous chunk's stream"),
// while an unchunked run's selectNext keeps consuming its original stream, so
// the two mutation streams diverge from the boundary onward. Verified
// divergent at this scale when the guard below was written.
func TestChunkedResumeDeterminism(t *testing.T) {
	cfg := checkpointTestConfig()
	first := NewNoveltyEngine(cfg, allSeeds())
	if done := first.RunChunk(nil, 2); done {
		t.Fatal("chunk to generation 2 of 4 reported done")
	}
	path := filepath.Join(t.TempDir(), "ck.json")
	if err := first.SaveCheckpoint(path); err != nil {
		t.Fatalf("SaveCheckpoint: %v", err)
	}

	run := func() *NoveltyEngine {
		e := NewNoveltyEngine(cfg, allSeeds())
		if err := e.LoadCheckpoint(path); err != nil {
			t.Fatalf("LoadCheckpoint: %v", err)
		}
		if done := e.RunChunk(nil, cfg.Generations); !done {
			t.Fatal("resumed run did not complete")
		}
		return e
	}
	a, b := run(), run()

	if a.BestFitness != b.BestFitness {
		t.Errorf("BestFitness diverged across identical resumes: %v vs %v", a.BestFitness, b.BestFitness)
	}
	if len(a.Population) != len(b.Population) {
		t.Fatalf("population size diverged: %d vs %d", len(a.Population), len(b.Population))
	}
	for i := range a.Population {
		x, y := a.Population[i], b.Population[i]
		if x.Genome.ID != y.Genome.ID || x.FitnessSum != y.FitnessSum || x.EvalCount != y.EvalCount {
			t.Errorf("population[%d] diverged: (%q, %v, %d) vs (%q, %v, %d)",
				i, x.Genome.ID, x.FitnessSum, x.EvalCount, y.Genome.ID, y.FitnessSum, y.EvalCount)
		}
	}
}

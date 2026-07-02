package evolution

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"

	"github.com/darwindeck/darwindeck/pkg/genome"
)

// checkpoint is the serialized NoveltyEngine state for the judge-in-loop
// orchestration: an evolution run is split into chunks of generations across
// separate processes so an out-of-loop LLM judge can grow Config.JudgeVerdicts
// between chunks. The whole POPULATION (and the novelty archive) is carried
// across, NOT just the elite -- the failed -seed-dir restart loop
// (results/2026-06-14-judge-in-loop) lost the population's diversity by
// re-seeding only from the elite, which is why novelty pressure could not
// compound. Every field is JSON-serializable (genomes carry json tags; the
// cached eval results are plain floats/ints).
type checkpoint struct {
	Generation   int
	BestFitness  float64
	BestGenome   *genome.Genome
	AddThreshold float64
	Population   []*NoveltyIndividual
	Archive      []*NoveltyIndividual
	// Config is the fingerprint of the invocation that wrote the checkpoint
	// (see configFingerprint). LoadCheckpoint refuses to resume under a
	// different fingerprint: the per-genome evaluation seeds derive from
	// (BaseSeed, Generation, population index), so resuming with e.g. a
	// different population size or seed would splice two different evaluation
	// streams into one FitnessSum running mean -- a silent corruption, not a
	// crash.
	Config checkpointConfig
}

// checkpointConfig is the subset of Config (plus the FitnessFloor package var)
// that changes the evaluation/selection stream and therefore must match
// between the invocation that saved a checkpoint and the one resuming it.
// Config.JudgeVerdicts is deliberately EXCLUDED: growing the verdict table
// between chunks is the whole point of the judge-in-loop orchestration.
// Workers, SaveTopN, and OutputDir are excluded because they change only
// parallelism and output, never the stream.
type checkpointConfig struct {
	PopulationSize int
	Generations    int
	EliteSize      int
	TournamentSize int
	BaseSeed       uint64
	MCTSDecile     float64
	CrossSkeleton  bool
	NoveltySelect  bool
	FitnessFloor   float64
}

// configFingerprint captures the engine's stream-determining knobs for the
// checkpoint guard.
func (e *NoveltyEngine) configFingerprint() checkpointConfig {
	return checkpointConfig{
		PopulationSize: e.Config.PopulationSize,
		Generations:    e.Config.Generations,
		EliteSize:      e.Config.EliteSize,
		TournamentSize: e.Config.TournamentSize,
		BaseSeed:       e.Config.BaseSeed,
		MCTSDecile:     e.Config.MCTSDecile,
		CrossSkeleton:  e.Config.CrossSkeleton,
		NoveltySelect:  e.Config.NoveltySelect,
		FitnessFloor:   FitnessFloor,
	}
}

// SaveCheckpoint writes the engine's current state to path (atomic via a temp
// file + rename so a crashed write cannot corrupt a resumable checkpoint).
func (e *NoveltyEngine) SaveCheckpoint(path string) error {
	data, err := json.Marshal(checkpoint{
		Generation:   e.Generation,
		BestFitness:  e.BestFitness,
		BestGenome:   e.BestGenome,
		AddThreshold: e.addThreshold,
		Population:   e.Population,
		Archive:      e.Archive,
		Config:       e.configFingerprint(),
	})
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// LoadCheckpoint restores a saved state into the engine. The mutation RNG is
// re-seeded deterministically from (BaseSeed, Generation): chunks are separate
// processes, so a fresh PCG keyed on the resume generation keeps mutations
// reproducible without repeating the previous chunk's stream. The per-genome
// EVALUATION seeds are derived from BaseSeed+generation (not this RNG), so the
// fitness pipeline stays bit-reproducible across the chunk boundary.
func (e *NoveltyEngine) LoadCheckpoint(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var cp checkpoint
	if err := json.Unmarshal(data, &cp); err != nil {
		return fmt.Errorf("checkpoint %s: %w", path, err)
	}
	// Config fingerprint guard: a resume under different stream-determining
	// knobs would silently splice two evaluation streams into one running
	// mean, so it is a hard error -- relaunch with the original flags (or
	// start a fresh checkpoint). A zero-value stored fingerprint means the
	// checkpoint predates the guard (legacy file); it is accepted as-is.
	if cp.Config != (checkpointConfig{}) {
		if cur := e.configFingerprint(); cp.Config != cur {
			return fmt.Errorf("checkpoint %s was written under a different config and cannot be resumed with these flags:\n  checkpoint: %+v\n  current:    %+v",
				path, cp.Config, cur)
		}
	}
	e.Generation = cp.Generation
	e.BestFitness = cp.BestFitness
	e.BestGenome = cp.BestGenome
	e.addThreshold = cp.AddThreshold
	e.Population = cp.Population
	e.Archive = cp.Archive
	e.rng = rand.New(rand.NewPCG(e.Config.BaseSeed, uint64(e.Generation)))
	return nil
}

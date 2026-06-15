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
	e.Generation = cp.Generation
	e.BestFitness = cp.BestFitness
	e.BestGenome = cp.BestGenome
	e.addThreshold = cp.AddThreshold
	e.Population = cp.Population
	e.Archive = cp.Archive
	e.rng = rand.New(rand.NewPCG(e.Config.BaseSeed, uint64(e.Generation)))
	return nil
}

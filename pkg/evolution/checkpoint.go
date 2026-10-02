package evolution

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"

	"github.com/darwindeck/darwindeck/pkg/fitness"
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
	// RNGState is the mutation/selection PCG's exact state
	// (rand.PCG.MarshalBinary) at the chunk boundary. Restoring it is what
	// makes a chunked run EQUAL the uninterrupted run: it was the only engine
	// state the checkpoint used to drop (LoadCheckpoint re-seeded a fresh PCG
	// instead), so N/2 + resume diverged from N straight from the boundary on.
	// Absent in checkpoints written before the field existed; LoadCheckpoint
	// then falls back to the old re-seed.
	RNGState []byte `json:",omitempty"`
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
	// MCTSEval changes the MCTS evaluation stream feeding MctsSum: resuming
	// under a different search strength would splice two incommensurable MCTS
	// streams into one running mean -- exactly the corruption this fingerprint
	// exists to prevent. Old checkpoints lack the field and unmarshal to the
	// zero value, which matches the default config, so they stay resumable.
	MCTSEval      fitness.MCTSEvalConfig
	CrossSkeleton bool
	NoveltySelect bool
	FitnessFloor  float64
	// SeedPoolHash fingerprints the seed pool (seedPoolHash). The pool is
	// stream-determining like the knobs above: population init, changeSkeleton
	// mutation and dedupParent all index it through rng.IntN(len(seeds)), so a
	// resume under a different `-seed-dir` continued a different search (a
	// gen-2 checkpoint resumed happily with the pool grown from 11 to 21).
	// Empty in checkpoints written before the field existed; LoadCheckpoint
	// skips the comparison for those.
	SeedPoolHash string `json:",omitempty"`
}

// seedPoolHash is the order-sensitive content fingerprint of a seed pool: a
// SHA-256 over each seed's genomeHash (the rules, not the ID/fitness
// bookkeeping) in pool order. Order matters because the engines pick seeds by
// index.
func seedPoolHash(seeds []*genome.Genome) string {
	h := sha256.New()
	for _, s := range seeds {
		h.Write([]byte(genomeHash(s)))
		h.Write([]byte{'\n'})
	}
	return hex.EncodeToString(h.Sum(nil))
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
		MCTSEval:       e.Config.MCTSEval,
		CrossSkeleton:  e.Config.CrossSkeleton,
		NoveltySelect:  e.Config.NoveltySelect,
		FitnessFloor:   FitnessFloor,
		SeedPoolHash:   seedPoolHash(e.Seeds),
	}
}

// SaveCheckpoint writes the engine's current state to path (atomic via a temp
// file + rename so a crashed write cannot corrupt a resumable checkpoint).
func (e *NoveltyEngine) SaveCheckpoint(path string) error {
	cp := checkpoint{
		Generation:   e.Generation,
		BestFitness:  e.BestFitness,
		BestGenome:   e.BestGenome,
		AddThreshold: e.addThreshold,
		Population:   e.Population,
		Archive:      e.Archive,
		Config:       e.configFingerprint(),
	}
	// e.pcg is nil only for hand-built engines (tests); their checkpoints
	// simply resume through the legacy re-seed path.
	if e.pcg != nil {
		state, err := e.pcg.MarshalBinary()
		if err != nil {
			return fmt.Errorf("checkpoint %s: rng state: %w", path, err)
		}
		cp.RNGState = state
	}
	data, err := json.Marshal(cp)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// LoadCheckpoint restores a saved state into the engine, INCLUDING the
// mutation/selection RNG stream (checkpoint.RNGState), so a chunked run is
// bit-equal to the uninterrupted one: the per-genome EVALUATION seeds derive
// from BaseSeed+generation (not this RNG), and with the PCG state restored the
// selection/mutation draws continue exactly where the previous chunk stopped.
//
// A checkpoint written before RNGState existed falls back to the old behavior:
// a fresh PCG keyed on (BaseSeed, Generation) -- reproducible across resumes
// of the same file, but not equal to an uninterrupted run.
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
	// checkpoint predates the guard (legacy file); it is accepted as-is. A
	// stored fingerprint WITHOUT a seed-pool hash predates that field: the
	// pool cannot be checked, so the remaining knobs are compared alone.
	if cp.Config != (checkpointConfig{}) {
		cur := e.configFingerprint()
		if cp.Config.SeedPoolHash == "" {
			cur.SeedPoolHash = ""
		}
		if cp.Config != cur {
			return fmt.Errorf("checkpoint %s was written under a different config and cannot be resumed with these flags (SeedPoolHash covers -seed-dir):\n  checkpoint: %+v\n  current:    %+v",
				path, cp.Config, cur)
		}
	}
	pcg := rand.NewPCG(e.Config.BaseSeed, uint64(cp.Generation)) // legacy re-seed
	if len(cp.RNGState) > 0 {
		if err := pcg.UnmarshalBinary(cp.RNGState); err != nil {
			return fmt.Errorf("checkpoint %s: rng state: %w", path, err)
		}
	}
	e.Generation = cp.Generation
	e.BestFitness = cp.BestFitness
	e.BestGenome = cp.BestGenome
	e.addThreshold = cp.AddThreshold
	e.Population = cp.Population
	e.Archive = cp.Archive
	e.pcg = pcg
	e.rng = rand.New(pcg)
	return nil
}

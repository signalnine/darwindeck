package output

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/darwindeck/darwindeck/pkg/evolution"
	"github.com/darwindeck/darwindeck/pkg/fitness"
)

// RunSummary holds metadata about an evolution run.
type RunSummary struct {
	StartTime      string `json:"start_time"`
	Duration       string `json:"duration"`
	PopulationSize int    `json:"population_size"`
	Generations    int    `json:"generations"`
	// BestFitness is the best GREEDY-ONLY running mean across the published
	// games -- the commensurable leaderboard key (Wave K fix 1; this field
	// used to be top[0]'s published fitness, an MCTS-mode mean resting on as
	// few as one eval).
	BestFitness float64 `json:"best_fitness"`
	// MctsBest is the best MCTS-mode running mean across the published
	// games, reported alongside but never ranked. 0 when no published game
	// received an MCTS grant.
	MctsBest      float64 `json:"mcts_best"`
	TopGamesCount int     `json:"top_games_count"`
}

// RunMeta is the meta.json reproducibility record (audit Task 4 convention,
// wired in round 3 commit 5b): every published results bundle must say which
// code produced it, with which inputs, under which validity rules. Fields
// that cannot be determined at runtime are written as "unknown" rather than
// omitted -- an absent field and an unknowable one are different claims.
type RunMeta struct {
	CommitSHA        string             `json:"commit_sha"`
	CommitDirty      bool               `json:"commit_dirty"`
	GoVersion        string             `json:"go_version"`
	Platform         string             `json:"platform"`
	CLIArgs          []string           `json:"cli_args"`
	MasterSeed       uint64             `json:"master_seed"`
	CalibrationSeeds []uint64           `json:"calibration_seeds"`
	Date             string             `json:"date"`
	// MCTSMode records the skill-tier mode (round-2 hazard 3): "top-decile"
	// when the decile pass was enabled, "greedy-only" otherwise. The decile
	// and search knobs make the grant reproducible; knob value 0 means the
	// production default (200 iterations / 10 determinizations).
	MCTSMode             string             `json:"mcts_mode"`
	MCTSDecile           float64            `json:"mcts_decile"`
	MCTSIterations       int                `json:"mcts_iterations"`
	MCTSDeterminizations int                `json:"mcts_determinizations"`
	VetoThresholds       map[string]float64 `json:"veto_thresholds"`
	FitnessFloor         float64            `json:"fitness_floor"`
	PopulationSize       int                `json:"population_size"`
	Generations          int                `json:"generations"`
	Workers              int                `json:"workers"`
}

// SaveResults writes all output files for a completed evolution run.
func SaveResults(
	dir string,
	top []*evolution.Individual,
	config evolution.Config,
	elapsed time.Duration,
) error {
	// Create output directory
	gamesDir := filepath.Join(dir, "games")
	if err := os.MkdirAll(gamesDir, 0755); err != nil {
		return fmt.Errorf("creating output dir: %w", err)
	}

	// Save summary
	summary := RunSummary{
		StartTime:      time.Now().Add(-elapsed).Format(time.RFC3339),
		Duration:       elapsed.String(),
		PopulationSize: config.PopulationSize,
		Generations:    config.Generations,
		TopGamesCount:  len(top),
	}
	for _, ind := range top {
		if rank := ind.OutputRank(); rank > summary.BestFitness {
			summary.BestFitness = rank
		}
		if mctsMean, ok := ind.MCTSMean(); ok && mctsMean > summary.MctsBest {
			summary.MctsBest = mctsMean
		}
	}

	if err := writeJSON(filepath.Join(dir, "summary.json"), summary); err != nil {
		return err
	}

	// meta.json: the reproducibility record (Task 4 convention).
	if err := writeJSON(filepath.Join(dir, "meta.json"), buildRunMeta(config)); err != nil {
		return err
	}

	// Save each top genome
	for i, ind := range top {
		name := fmt.Sprintf("rank%02d_%s", i+1, sanitize(ind.Genome.ID))
		gameDir := filepath.Join(gamesDir, name)
		if err := os.MkdirAll(gameDir, 0755); err != nil {
			return err
		}

		// PUBLICATION INVARIANT (round 3 commit 5a; key changed by Wave K
		// fix 1): genome.json and report.md must state the SAME fitness --
		// the GREEDY-ONLY running mean (OutputRank) of THIS individual, the
		// commensurable leaderboard key. The MCTS-mode mean lives in the
		// report's provenance section and summary.json's mcts_best, never in
		// the headline. The genome's own Fitness field can be stale at save
		// time (the r2 flagship's rank04: a novelty-archive snapshot whose
		// live twin was re-evaluated later overwrote the shared genome's
		// field to 0.808 while the archived metrics said 0.847). Stamp a
		// clone; never mutate the caller's genome.
		published := ind.Genome.Clone()
		published.Fitness = ind.OutputRank()
		published.SharedFitness = ind.Fitness.SharedFitness

		// Genome JSON
		if err := writeJSON(filepath.Join(gameDir, "genome.json"), published); err != nil {
			return err
		}

		// Rulebook
		rulebook := GenerateRulebook(published)
		if err := os.WriteFile(filepath.Join(gameDir, "rulebook.md"), []byte(rulebook), 0644); err != nil {
			return err
		}

		// Report (both fitness means + gap, round 3 commit 5c)
		report := GenerateIndividualReport(published, ind)
		if err := os.WriteFile(filepath.Join(gameDir, "report.md"), []byte(report), 0644); err != nil {
			return err
		}
	}

	return nil
}

// buildRunMeta assembles the meta.json record. Wired from what the process
// can actually know: commit SHA and dirtiness come from the binary's
// embedded VCS stamp (debug.ReadBuildInfo; "unknown" for test binaries and
// non-VCS builds), CLI args from os.Args, validity rules from the fitness
// and evolution packages' own constants.
func buildRunMeta(config evolution.Config) RunMeta {
	meta := RunMeta{
		CommitSHA:            "unknown",
		GoVersion:            runtime.Version(),
		Platform:             runtime.GOOS + "/" + runtime.GOARCH,
		CLIArgs:              os.Args,
		MasterSeed:           config.BaseSeed,
		CalibrationSeeds:     append([]uint64(nil), fitness.CalibrationSeeds...),
		Date:                 time.Now().Format(time.RFC3339),
		MCTSMode:             "greedy-only",
		MCTSDecile:           config.MCTSDecile,
		MCTSIterations:       config.MCTSEval.Iterations,
		MCTSDeterminizations: config.MCTSEval.Determinizations,
		VetoThresholds:       fitness.DegeneracyThresholds(),
		FitnessFloor:         evolution.FitnessFloor,
		PopulationSize:       config.PopulationSize,
		Generations:          config.Generations,
		Workers:              config.Workers,
	}
	if config.MCTSDecile > 0 {
		meta.MCTSMode = "top-decile"
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			switch s.Key {
			case "vcs.revision":
				meta.CommitSHA = s.Value
			case "vcs.modified":
				meta.CommitDirty = s.Value == "true"
			}
		}
	}
	return meta
}

func writeJSON(path string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling %s: %w", path, err)
	}
	return os.WriteFile(path, data, 0644)
}

func sanitize(s string) string {
	out := make([]byte, 0, len(s))
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-' {
			out = append(out, byte(c))
		}
	}
	if len(out) == 0 {
		return "unnamed"
	}
	if len(out) > 30 {
		out = out[:30]
	}
	return string(out)
}

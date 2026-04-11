package output

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/darwindeck/darwindeck/pkg/evolution"
)

// RunSummary holds metadata about an evolution run.
type RunSummary struct {
	StartTime      string  `json:"start_time"`
	Duration       string  `json:"duration"`
	PopulationSize int     `json:"population_size"`
	Generations    int     `json:"generations"`
	BestFitness    float64 `json:"best_fitness"`
	TopGamesCount  int     `json:"top_games_count"`
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
	if len(top) > 0 {
		summary.BestFitness = top[0].Fitness.TotalFitness
	}

	if err := writeJSON(filepath.Join(dir, "summary.json"), summary); err != nil {
		return err
	}

	// Save each top genome
	for i, ind := range top {
		name := fmt.Sprintf("rank%02d_%s", i+1, sanitize(ind.Genome.ID))
		gameDir := filepath.Join(gamesDir, name)
		if err := os.MkdirAll(gameDir, 0755); err != nil {
			return err
		}

		// Genome JSON
		if err := writeJSON(filepath.Join(gameDir, "genome.json"), ind.Genome); err != nil {
			return err
		}

		// Rulebook
		rulebook := GenerateRulebook(ind.Genome)
		if err := os.WriteFile(filepath.Join(gameDir, "rulebook.md"), []byte(rulebook), 0644); err != nil {
			return err
		}

		// Report
		report := GenerateReport(ind.Genome, ind.Fitness)
		if err := os.WriteFile(filepath.Join(gameDir, "report.md"), []byte(report), 0644); err != nil {
			return err
		}
	}

	return nil
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

package evolution

import (
	"fmt"
	"testing"
	"time"
)

// TestEvolutionDemo runs a visible evolution demo with progress output.
// Run with: go test ./evolution -v -run TestEvolutionDemo -count=1
func TestEvolutionDemo(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping demo in short mode")
	}

	fmt.Println("\n" + repeat("=", 60))
	fmt.Println("  EVOLUTION DEMO - Pure Go Evolution Engine")
	fmt.Println(repeat("=", 60))

	config := &EvolutionConfig{
		PopulationSize: 10,
		MaxGenerations: 5,
		ElitismRate:    0.2,
		CrossoverRate:  0.7,
		TournamentSize: 3,
		SeedRatio:      0.8,
		RandomSeed:     time.Now().UnixNano(),
		FitnessStyle:   "balanced",
		GamesPerEval:   20,
		NumWorkers:     2,
		Verbose:        false,
	}

	fmt.Printf("\nConfiguration:\n")
	fmt.Printf("  Population:    %d\n", config.PopulationSize)
	fmt.Printf("  Generations:   %d\n", config.MaxGenerations)
	fmt.Printf("  Elitism:       %.0f%%\n", config.ElitismRate*100)
	fmt.Printf("  Crossover:     %.0f%%\n", config.CrossoverRate*100)
	fmt.Printf("  Fitness Style: %s\n", config.FitnessStyle)
	fmt.Printf("  Games/Eval:    %d\n", config.GamesPerEval)
	fmt.Printf("  Workers:       %d\n", config.NumWorkers)

	engine := NewEvolutionEngine(config)
	defer engine.Close()

	// Track progress
	startTime := time.Now()
	engine.OnGenerationComplete = func(stats GenerationStats) {
		elapsed := time.Since(startTime).Seconds()
		fmt.Printf("\n  Gen %2d | Best: %.4f | Avg: %.4f | Diversity: %.4f | Time: %.1fs",
			stats.Generation+1, stats.BestFitness, stats.AvgFitness, stats.Diversity, elapsed)
	}

	fmt.Println("\n\nStarting evolution...")
	fmt.Println(repeat("-", 60))

	err := engine.Evolve()
	if err != nil {
		t.Fatalf("Evolution failed: %v", err)
	}

	totalTime := time.Since(startTime)

	fmt.Println("\n" + repeat("-", 60))
	fmt.Println("\nEvolution Complete!")
	fmt.Printf("  Total Time:    %.2fs\n", totalTime.Seconds())
	fmt.Printf("  Best Fitness:  %.4f\n", engine.BestEver.Fitness)
	fmt.Printf("  Best Genome:   %s\n", engine.BestEver.Genome.Name)

	// Show top 5 genomes
	fmt.Println("\nTop 5 Genomes:")
	best := engine.GetBestGenomes(5)
	for i, ind := range best {
		metrics := ""
		if ind.FitnessMetrics != nil {
			m := ind.FitnessMetrics
			metrics = fmt.Sprintf(" (dec=%.2f, skill=%.2f, complex=%.2f)",
				m.DecisionDensity, m.SkillVsLuck, m.RulesComplexity)
		}
		fmt.Printf("  %d. %s: %.4f%s\n", i+1, ind.Genome.Name, ind.Fitness, metrics)
	}

	fmt.Println("\n" + repeat("=", 60))
}

// repeat returns a string repeated n times
func repeat(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}

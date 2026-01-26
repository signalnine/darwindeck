// Package main provides the darwindeck-evolve CLI for running genetic evolution of card games.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/signalnine/darwindeck/gosim/evolution"
	"github.com/signalnine/darwindeck/gosim/evolution/fitness"
	"github.com/signalnine/darwindeck/gosim/genome"
)

// Version information (set by build flags)
var (
	Version   = "dev"
	BuildTime = "unknown"
)

// CLI flags
var (
	generations       int
	populationSize    int
	style             string
	gamesPerEval      int
	seed              int64
	checkpointPath    string
	checkpointInterval int
	skipSkillEval     bool
	outputDir         string
	saveTopN          int
	workers           int
	verbose           bool
	showVersion       bool
)

func init() {
	flag.IntVar(&generations, "generations", 100, "Number of generations to evolve")
	flag.IntVar(&populationSize, "population-size", 50, "Population size")
	flag.StringVar(&style, "style", "balanced", "Fitness style preset (balanced, bluffing, strategic, party, trick-taking)")
	flag.IntVar(&gamesPerEval, "games-per-eval", 100, "Number of games per fitness evaluation")
	flag.Int64Var(&seed, "seed", 0, "Random seed (0 = use current time)")
	flag.StringVar(&checkpointPath, "checkpoint", "", "Resume from checkpoint file")
	flag.IntVar(&checkpointInterval, "checkpoint-interval", 10, "Auto-save checkpoint every N generations (0 = disabled)")
	flag.BoolVar(&skipSkillEval, "skip-skill-eval", false, "Skip MCTS skill evaluation (faster but less accurate)")
	flag.StringVar(&outputDir, "output-dir", "", "Output directory for results (default: output/evolution-TIMESTAMP)")
	flag.IntVar(&saveTopN, "save-top-n", 20, "Save top N genomes to output directory")
	flag.IntVar(&workers, "workers", 0, "Number of worker goroutines (0 = auto-detect CPU count)")
	flag.BoolVar(&verbose, "verbose", false, "Enable verbose output")
	flag.BoolVar(&showVersion, "version", false, "Show version information")
}

func main() {
	flag.Parse()

	if showVersion {
		fmt.Printf("darwindeck-evolve %s (built %s)\n", Version, BuildTime)
		os.Exit(0)
	}

	// Set default output directory
	if outputDir == "" {
		timestamp := time.Now().Format("20060102-150405")
		outputDir = filepath.Join("output", fmt.Sprintf("evolution-%s", timestamp))
	}

	// Set random seed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}

	// Print banner
	printBanner()

	// Create or resume engine
	var engine *evolution.EvolutionEngine
	var err error

	if checkpointPath != "" {
		fmt.Printf("Resuming from checkpoint: %s\n", checkpointPath)
		engine, err = evolution.ResumeFromCheckpoint(checkpointPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading checkpoint: %v\n", err)
			os.Exit(1)
		}
		// Override some settings from CLI
		engine.Config.MaxGenerations = generations
		engine.Config.NumWorkers = workers
		engine.Config.Verbose = verbose
		fmt.Printf("Resumed at generation %d\n\n", engine.Population.Generation)
	} else {
		config := &evolution.EvolutionConfig{
			PopulationSize:       populationSize,
			MaxGenerations:       generations,
			ElitismRate:          0.1,
			CrossoverRate:        0.7,
			TournamentSize:       3,
			SeedRatio:            0.5,
			RandomSeed:           seed,
			FitnessStyle:         style,
			GamesPerEval:         gamesPerEval,
			UseMCTS:              !skipSkillEval,
			NumWorkers:           workers,
			Verbose:              verbose,
			PlateauThreshold:     10,
			ImprovementThreshold: 0.001,
			DiversityThreshold:   0.05,
		}
		engine = evolution.NewEvolutionEngine(config)
	}
	defer engine.Close()

	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	// Setup auto-checkpointing
	var autoCheckpointer *evolution.AutoCheckpointer
	if checkpointInterval > 0 {
		cpPath := filepath.Join(outputDir, "checkpoint.json")
		autoCheckpointer = evolution.NewAutoCheckpointer(engine, cpPath, checkpointInterval)
	}

	// Setup signal handler for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\n\nInterrupted! Saving checkpoint...")
		if autoCheckpointer != nil {
			if err := autoCheckpointer.SaveFinal(); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving checkpoint: %v\n", err)
			} else {
				fmt.Printf("Checkpoint saved to %s\n", filepath.Join(outputDir, "checkpoint.json"))
			}
		}
		os.Exit(130)
	}()

	// Track progress
	startTime := time.Now()
	engine.OnGenerationComplete = func(stats evolution.GenerationStats) {
		elapsed := time.Since(startTime)

		// Progress bar
		progress := float64(stats.Generation+1) / float64(generations) * 100

		fmt.Printf("\rGen %3d/%d | Best: %.4f | Avg: %.4f | Div: %.4f | %s (%.0f%%)",
			stats.Generation+1, generations,
			stats.BestFitness, stats.AvgFitness, stats.Diversity,
			formatDuration(elapsed), progress)

		if verbose && engine.BestEver != nil {
			fmt.Printf("\n  Best genome: %s\n", engine.BestEver.Genome.Name)
		}

		// Auto-checkpoint
		if autoCheckpointer != nil {
			if err := autoCheckpointer.Save(stats.Generation + 1); err != nil {
				fmt.Fprintf(os.Stderr, "\nWarning: checkpoint save failed: %v\n", err)
			}
		}
	}

	// Run evolution
	fmt.Println("Starting evolution...\n")
	err = engine.Evolve()
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nEvolution failed: %v\n", err)
		os.Exit(1)
	}

	totalTime := time.Since(startTime)
	fmt.Printf("\n\nEvolution complete in %s\n", formatDuration(totalTime))

	// Save results
	fmt.Printf("\nSaving top %d genomes to %s...\n", saveTopN, outputDir)
	best := engine.GetBestGenomes(saveTopN)

	for i, ind := range best {
		filename := fmt.Sprintf("rank%02d_%s.json", i+1, sanitizeFilename(ind.Genome.Name))
		path := filepath.Join(outputDir, filename)

		if err := saveGenome(ind.Genome, ind.Fitness, ind.FitnessMetrics, path); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to save %s: %v\n", filename, err)
			continue
		}

		if verbose {
			fmt.Printf("  %2d. %s (fitness=%.4f)\n", i+1, ind.Genome.Name, ind.Fitness)
		}
	}

	// Save final checkpoint
	if autoCheckpointer != nil {
		if err := autoCheckpointer.SaveFinal(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: final checkpoint save failed: %v\n", err)
		}
	}

	// Print summary
	printSummary(engine, totalTime, outputDir)
}

func printBanner() {
	fmt.Println()
	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║           DarwinDeck Evolution Engine (Go)                 ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("Configuration:\n")
	fmt.Printf("  Population:     %d\n", populationSize)
	fmt.Printf("  Generations:    %d\n", generations)
	fmt.Printf("  Fitness Style:  %s\n", style)
	fmt.Printf("  Games/Eval:     %d\n", gamesPerEval)
	fmt.Printf("  Workers:        %d (0=auto)\n", workers)
	fmt.Printf("  Output:         %s\n", outputDir)
	if checkpointInterval > 0 {
		fmt.Printf("  Checkpoint:     every %d generations\n", checkpointInterval)
	}
	fmt.Println()
}

func printSummary(engine *evolution.EvolutionEngine, totalTime time.Duration, outputDir string) {
	fmt.Println()
	fmt.Println("════════════════════════════════════════════════════════════")
	fmt.Println("                      EVOLUTION SUMMARY")
	fmt.Println("════════════════════════════════════════════════════════════")
	fmt.Printf("  Total Time:      %s\n", formatDuration(totalTime))
	fmt.Printf("  Generations:     %d\n", len(engine.StatsHistory))

	if engine.BestEver != nil {
		fmt.Printf("  Best Fitness:    %.4f\n", engine.BestEver.Fitness)
		fmt.Printf("  Best Genome:     %s\n", engine.BestEver.Genome.Name)

		if engine.BestEver.FitnessMetrics != nil {
			m := engine.BestEver.FitnessMetrics
			fmt.Printf("  Metrics:\n")
			fmt.Printf("    Decision Density:  %.2f\n", m.DecisionDensity)
			fmt.Printf("    Skill vs Luck:     %.2f\n", m.SkillVsLuck)
			fmt.Printf("    Complexity:        %.2f\n", m.RulesComplexity)
		}
	}

	fmt.Printf("  Output:          %s\n", outputDir)
	fmt.Println("════════════════════════════════════════════════════════════")
	fmt.Println()
}

// GenomeOutput is the JSON structure for saved genomes
type GenomeOutput struct {
	Genome         *genome.GameGenome         `json:"genome"`
	Fitness        float64                    `json:"fitness"`
	FitnessMetrics map[string]float64         `json:"fitness_metrics,omitempty"`
}

func saveGenome(g *genome.GameGenome, fit float64, metrics *fitness.FitnessMetrics, path string) error {
	output := GenomeOutput{
		Genome:  g,
		Fitness: fit,
	}

	if metrics != nil {
		output.FitnessMetrics = map[string]float64{
			"decision_density":      metrics.DecisionDensity,
			"comeback_potential":    metrics.ComebackPotential,
			"tension_curve":         metrics.TensionCurve,
			"interaction_frequency": metrics.InteractionFrequency,
			"rules_complexity":      metrics.RulesComplexity,
			"skill_vs_luck":         metrics.SkillVsLuck,
			"total_fitness":         metrics.TotalFitness,
		}
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func sanitizeFilename(name string) string {
	// Replace spaces and special characters with underscores
	result := make([]byte, 0, len(name))
	for _, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			result = append(result, byte(c))
		} else if c == ' ' {
			result = append(result, '_')
		}
	}
	if len(result) == 0 {
		return "genome"
	}
	return string(result)
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Hour {
		m := int(d.Minutes())
		s := int(d.Seconds()) % 60
		return fmt.Sprintf("%dm%ds", m, s)
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh%dm", h, m)
}

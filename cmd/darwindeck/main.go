package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/darwindeck/darwindeck/pkg/evolution"
	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/output"
	"github.com/darwindeck/darwindeck/pkg/playtest"
	"github.com/darwindeck/darwindeck/pkg/seeds"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

var (
	Version   = "dev"
	BuildTime = "unknown"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "evolve":
		cmdEvolve(os.Args[2:])
	case "playtest":
		cmdPlaytest(os.Args[2:])
	case "describe":
		cmdDescribe(os.Args[2:])
	case "version":
		fmt.Printf("darwindeck %s (built %s)\n", Version, BuildTime)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Usage: darwindeck <command> [flags]

Commands:
  evolve     Run evolutionary search for novel card games
  playtest   Play a game interactively against AI
  describe   Show details of a genome JSON file
  version    Print version info
  help       Show this message`)
}

func cmdEvolve(args []string) {
	fs := flag.NewFlagSet("evolve", flag.ExitOnError)

	pop := fs.Int("population", 500, "population size")
	gens := fs.Int("generations", 100, "number of generations")
	workers := fs.Int("workers", 0, "parallel workers (0=auto)")
	seed := fs.Uint64("seed", 42, "random seed")
	topN := fs.Int("top", 20, "save top N genomes")
	outDir := fs.String("output", "", "output directory (default: auto-generated)")
	verbose := fs.Bool("verbose", false, "verbose output")

	fs.Parse(args)

	if *outDir == "" {
		*outDir = filepath.Join("output", time.Now().Format("2006-01-02_15-04-05"))
	}

	config := evolution.Config{
		PopulationSize: *pop,
		Generations:    *gens,
		EliteSize:      10,
		TournamentSize: 5,
		Workers:        *workers,
		BaseSeed:       *seed,
		SaveTopN:       *topN,
		OutputDir:      *outDir,
	}

	allSeeds := getAllSeeds()

	fmt.Printf("DarwinDeck v2 Evolution\n")
	fmt.Printf("  Population: %d\n", config.PopulationSize)
	fmt.Printf("  Generations: %d\n", config.Generations)
	fmt.Printf("  Workers: %d\n", config.Workers)
	fmt.Printf("  Seeds: %d games across 3 skeletons\n", len(allSeeds))
	fmt.Printf("  Output: %s\n\n", config.OutputDir)

	engine := evolution.NewEngine(config, allSeeds)
	startTime := time.Now()

	engine.Run(func(gen int, best, avg float64) {
		elapsed := time.Since(startTime)
		if *verbose || gen%10 == 0 || gen == config.Generations-1 {
			fmt.Printf("Gen %3d | best=%.3f avg=%.3f | %s\n",
				gen, best, avg, elapsed.Round(time.Millisecond))
		}
	})

	elapsed := time.Since(startTime)
	fmt.Printf("\nEvolution complete in %s\n", elapsed.Round(time.Millisecond))
	fmt.Printf("Best fitness: %.3f\n\n", engine.BestFitness)

	// Save results
	top := engine.TopN(config.SaveTopN)
	err := output.SaveResults(config.OutputDir, top, config, elapsed)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error saving results: %v\n", err)
		os.Exit(1)
	}

	// Print top 5
	fmt.Printf("Top %d games:\n", min(5, len(top)))
	for i, ind := range top {
		if i >= 5 {
			break
		}
		m := ind.Fitness
		fmt.Printf("  %d. %s (%.3f) [%s] decisions=%.2f arc=%.2f interact=%.2f skill=%.2f length=%.2f\n",
			i+1, ind.Genome.ID, m.TotalFitness, ind.Genome.Skeleton,
			m.MeaningfulDecisions, m.GameArc, m.Interaction, m.SkillGradient, m.SessionLength)
	}

	fmt.Printf("\nResults saved to %s\n", config.OutputDir)
}

func cmdPlaytest(args []string) {
	fs := flag.NewFlagSet("playtest", flag.ExitOnError)
	difficulty := fs.String("difficulty", "greedy", "AI difficulty: random or greedy")
	seed := fs.Uint64("seed", 0, "random seed (0=random)")
	fs.Parse(args)

	remaining := fs.Args()
	if len(remaining) == 0 {
		fmt.Fprintln(os.Stderr, "usage: darwindeck playtest <genome.json> [--difficulty random|greedy]")
		os.Exit(1)
	}

	data, err := os.ReadFile(remaining[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}

	var g genome.Genome
	if err := json.Unmarshal(data, &g); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing genome: %v\n", err)
		os.Exit(1)
	}

	errs := genome.Validate(&g)
	if len(errs) > 0 {
		fmt.Fprintf(os.Stderr, "Genome validation failed:\n")
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "  - %s\n", e)
		}
		os.Exit(1)
	}

	runner := fitness.GetRunner(&g)
	if runner == nil {
		fmt.Fprintf(os.Stderr, "No runner for skeleton: %s\n", g.Skeleton)
		os.Exit(1)
	}

	var ai sim.AIPlayer
	switch *difficulty {
	case "random":
		ai = &sim.RandomAI{}
	case "greedy":
		ai = fitness.GetGreedyAI(&g)
	default:
		fmt.Fprintf(os.Stderr, "Unknown difficulty: %s (use random or greedy)\n", *difficulty)
		os.Exit(1)
	}

	if *seed == 0 {
		*seed = uint64(time.Now().UnixNano())
	}

	session := playtest.NewSession(&g, runner, ai, *seed)
	session.Run()
}

func cmdDescribe(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: darwindeck describe <genome.json>")
		os.Exit(1)
	}

	data, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}

	var g genome.Genome
	if err := json.Unmarshal(data, &g); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing genome: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Genome: %s\n", g.ID)
	fmt.Printf("Skeleton: %s\n", g.Skeleton)
	fmt.Printf("Players: %d\n", g.Players)
	fmt.Printf("Hand Size: %d\n", g.HandSize)
	fmt.Printf("Fitness: %.3f\n", g.Fitness)
	fmt.Printf("Generation: %d\n", g.Generation)
	fmt.Printf("Params: %s\n", g.ActiveParams())
	fmt.Printf("Special Cards: %d\n", len(g.SpecialCards))
	fmt.Printf("Borrowed Mechanics: %d\n", len(g.Borrowed))
	fmt.Printf("Trump Rule: %d\n", g.TrumpRule)

	errs := genome.Validate(&g)
	if len(errs) > 0 {
		fmt.Printf("\nValidation errors:\n")
		for _, e := range errs {
			fmt.Printf("  - %s\n", e)
		}
	} else {
		fmt.Printf("\nValidation: OK\n")
	}
}

func getAllSeeds() []*genome.Genome {
	return []*genome.Genome{
		seeds.CrazyEights(),
		seeds.MauMau(),
		seeds.Whist(),
		seeds.Hearts(),
		seeds.Spades(),
		seeds.OhHell(),
		seeds.GinRummy(),
		seeds.KnockRummy(),
	}
}

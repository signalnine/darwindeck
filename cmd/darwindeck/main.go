package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	case "experiment":
		cmdExperiment(os.Args[2:])
	case "calibrate":
		cmdCalibrate(os.Args[2:])
	case "restamp":
		cmdRestamp(os.Args[2:])
	case "playtest":
		cmdPlaytest(os.Args[2:])
	case "serve":
		cmdServe(os.Args[2:])
	case "describe":
		cmdDescribe(os.Args[2:])
	case "judge":
		cmdJudge(os.Args[2:])
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
  evolve      Run evolutionary search for novel card games
  experiment  Run diversity comparison experiments
  calibrate   Report raw fitness metrics for classics + degenerate fixtures
  restamp     Re-evaluate a saved run's genomes for veto-stability and write a results bundle
  playtest    Play a game interactively against AI
  serve       Serve a genome (or a directory of them) as a browser game
  describe    Show details of a genome JSON file
  judge       LLM-as-judge: emit blind dossiers / rank verdicts
  version     Print version info
  help        Show this message`)
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
	algorithm := fs.String("algorithm", "hybrid", "algorithm: baseline, hybrid (novelty+sharing), mapelites")
	floor := fs.Float64("fitness-floor", evolution.DefaultFitnessFloor,
		"minimum fitness for QD consideration (default derived from the seed-calibration suite)")
	mctsDecile := fs.Float64("mcts-decile", 0.10,
		"fraction of each generation (ranked by greedy-only running mean) re-evaluated with MCTS; 0 disables (baseline/hybrid only)")
	crossSkeleton := fs.Bool("cross-skeleton", false,
		"enable cross-skeleton recombination: crossing two different-skeleton parents produces a HYBRID child (e.g. shed-to-win scored by tricks) and mutation may add cross-family active borrows; default OFF (baseline/hybrid only -- MAP-Elites crosses same-skeleton only)")
	noveltySelect := fs.Bool("novelty-select", false,
		"seed-aware novelty selection (hybrid only): add behavioral distance from the nearest of the 8 classic seeds into each VALID, above-floor individual's novelty score, steering the search away from the Crazy-Eights/Whist/Gin attractors; default OFF")
	seedDir := fs.String("seed-dir", "",
		"directory of custom seed genomes: load every genome.json under <dir> and AUGMENT (not replace) the built-in classic seed pool with the valid ones, so population init and changeSkeleton mutation sample from the combined pool while cross-family crossover keeps its classic partners; invalid files are skipped with a warning; default off (classics only)")
	judgeVerdicts := fs.String("judge-verdicts", "",
		"JSON file mapping a genome COMPOSITION (\"<skeleton>:<sorted borrow mechanic ids>\", e.g. \"0:1,8\") to an LLM-judge novelty score (novel > 0, variant/known < 0); adds a judge-novelty term steering the search toward certified-novel compositions and away from rediscoveries (hybrid + novelty-select only); default off")
	checkpoint := fs.String("checkpoint", "",
		"checkpoint file for judge-in-loop chunked evolution (hybrid only): resume from it if it exists, save to it after this invocation; lets an out-of-loop judge grow -judge-verdicts between chunks while the whole population persists")
	chunk := fs.Int("chunk", 0,
		"run at most this many generations this invocation, then checkpoint + emit the top genomes for judging + exit (0 = run to -generations in one process)")
	emitDir := fs.String("emit-dir", "",
		"directory to write the top genomes (genome.json per rank) at a chunk boundary, for out-of-loop novelty judging; default <output>/judge-queue")

	fs.Parse(args)
	evolution.FitnessFloor = *floor

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
		MCTSDecile:     *mctsDecile,
		CrossSkeleton:  *crossSkeleton,
		NoveltySelect:  *noveltySelect,
		JudgeVerdicts:  loadJudgeVerdicts(*judgeVerdicts),
	}

	allSeeds := seedPool(*seedDir)

	fmt.Printf("DarwinDeck v2 Evolution\n")
	fmt.Printf("  Algorithm: %s\n", *algorithm)
	fmt.Printf("  Population: %d\n", config.PopulationSize)
	fmt.Printf("  Generations: %d\n", config.Generations)
	fmt.Printf("  Workers: %d\n", config.Workers)
	fmt.Printf("  MCTS decile: %.2f\n", config.MCTSDecile)
	fmt.Printf("  Cross-skeleton: %v\n", config.CrossSkeleton)
	fmt.Printf("  Novelty-select: %v\n", config.NoveltySelect)
	if *seedDir != "" {
		fmt.Printf("  Seed pool: %d (%d classics + %d custom from %s)\n",
			len(allSeeds), len(getAllSeeds()), len(allSeeds)-len(getAllSeeds()), *seedDir)
	} else {
		skel := map[genome.SkeletonType]bool{}
		for _, s := range allSeeds {
			skel[s.Skeleton] = true
		}
		fmt.Printf("  Seeds: %d games across %d skeletons\n", len(allSeeds), len(skel))
	}
	fmt.Printf("  Output: %s\n\n", config.OutputDir)

	startTime := time.Now()

	progress := func(gen int, best, avg float64) {
		elapsed := time.Since(startTime)
		if *verbose || gen%10 == 0 || gen == config.Generations-1 {
			fmt.Printf("Gen %3d | best=%.3f avg=%.3f | %s\n",
				gen, best, avg, elapsed.Round(time.Millisecond))
		}
	}

	var top []*evolution.Individual
	var bestFitness float64

	switch *algorithm {
	case "baseline":
		engine := evolution.NewEngine(config, allSeeds)
		engine.Run(progress)
		top = engine.TopN(config.SaveTopN)
		bestFitness = engine.BestFitness
	case "hybrid", "novelty":
		engine := evolution.NewNoveltyEngine(config, allSeeds)
		// Judge-in-loop chunked mode: resume from the checkpoint (if any), run a
		// chunk, save the checkpoint, and at an intermediate boundary emit the top
		// genomes for out-of-loop judging and exit (the orchestrator judges, grows
		// -judge-verdicts, and relaunches). config.JudgeVerdicts is re-read from
		// the file each invocation, so verdicts added between chunks take effect on
		// resume.
		if *checkpoint != "" {
			if _, err := os.Stat(*checkpoint); err == nil {
				if err := engine.LoadCheckpoint(*checkpoint); err != nil {
					fmt.Fprintf(os.Stderr, "checkpoint: %v\n", err)
					os.Exit(1)
				}
				fmt.Printf("  Resumed from %s at generation %d\n", *checkpoint, engine.Generation)
			}
			until := config.Generations
			if *chunk > 0 {
				until = engine.Generation + *chunk
			}
			done := engine.RunChunk(progress, until)
			if err := engine.SaveCheckpoint(*checkpoint); err != nil {
				fmt.Fprintf(os.Stderr, "checkpoint save: %v\n", err)
				os.Exit(1)
			}
			if !done {
				queue := *emitDir
				if queue == "" {
					queue = filepath.Join(config.OutputDir, "judge-queue")
				}
				inds, _ := engine.AllQualified()
				emitForJudging(queue, engine.Generation, sortAndTrim(inds, config.SaveTopN, config.JudgeVerdicts))
				fmt.Printf("\nChunk boundary at generation %d/%d.\nJudge the new compositions in %s, append verdicts to the table, then relaunch with the same -checkpoint to resume.\n",
					engine.Generation, config.Generations, queue)
				return
			}
			inds, _ := engine.AllQualified()
			top = sortAndTrim(inds, config.SaveTopN, config.JudgeVerdicts)
			bestFitness = engine.BestFitness
			break
		}
		engine.Run(progress)
		inds, _ := engine.AllQualified()
		// Sort by raw fitness and take top N
		top = sortAndTrim(inds, config.SaveTopN, config.JudgeVerdicts)
		bestFitness = engine.BestFitness
	case "mapelites":
		engine := evolution.NewMAPElitesEngine(config, allSeeds)
		engine.Run(progress)
		top = sortAndTrim(engine.AllQualified(), config.SaveTopN, config.JudgeVerdicts)
		bestFitness = engine.BestFitness
	default:
		fmt.Fprintf(os.Stderr, "unknown algorithm: %s (must be baseline, hybrid, or mapelites)\n", *algorithm)
		os.Exit(1)
	}

	elapsed := time.Since(startTime)
	fmt.Printf("\nEvolution complete in %s\n", elapsed.Round(time.Millisecond))
	fmt.Printf("Best fitness: %.3f\n\n", bestFitness)
	// Save results
	err := output.SaveResults(config.OutputDir, top, config, elapsed)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error saving results: %v\n", err)
		os.Exit(1)
	}

	// Print top games per skeleton
	skeletonNames := []string{"shedding", "trick_taking", "rummy"}
	for _, skelName := range skeletonNames {
		fmt.Printf("\nBest %s:\n", skelName)
		count := 0
		for _, ind := range top {
			if ind.Genome.Skeleton.String() == skelName && count < 3 {
				m := ind.Fitness
				fmt.Printf("  %s (%.3f) decisions=%.2f arc=%.2f interact=%.2f skill=%.2f length=%.2f\n",
					ind.Genome.ID, m.TotalFitness,
					m.MeaningfulDecisions, m.GameArc, m.Interaction, m.SkillGradient, m.SessionLength)
				count++
			}
		}
		if count == 0 {
			fmt.Printf("  (none in top %d)\n", config.SaveTopN)
		}
	}

	fmt.Printf("\nResults saved to %s\n", config.OutputDir)
}

func cmdPlaytest(args []string) {
	fs := flag.NewFlagSet("playtest", flag.ExitOnError)
	difficulty := fs.String("difficulty", "greedy", "AI difficulty: random, greedy, or mcts")
	seed := fs.Uint64("seed", 0, "random seed (0=random)")
	fs.Parse(args)

	remaining := fs.Args()
	if len(remaining) == 0 {
		fmt.Fprintln(os.Stderr, "usage: darwindeck playtest <genome.json> [--difficulty random|greedy|mcts]")
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
	case "mcts":
		ai = playtest.NewMCTSAI(&g, runner)
	default:
		fmt.Fprintf(os.Stderr, "Unknown difficulty: %s (use random, greedy, or mcts)\n", *difficulty)
		os.Exit(1)
	}

	if *seed == 0 {
		*seed = uint64(time.Now().UnixNano())
	}

	session := playtest.NewSession(&g, runner, ai, *seed)
	outcome := session.Run()

	// Human-ratings capture (audit Task 24): the only instrument linking
	// fitness scores to human ground truth. Empty rating = skipped (null);
	// EOF on piped/non-interactive stdin skips silently.
	rating, comment := playtest.PromptRating(session.Scanner, os.Stdout)
	rec := playtest.Record{
		Timestamp:  time.Now().Format(time.RFC3339),
		GenomeID:   g.ID,
		GenomePath: remaining[0],
		Difficulty: *difficulty,
		Seed:       *seed,
		Winner:     outcome.WinnerLabel(session.HumanID),
		Turns:      outcome.Turns,
		Rating:     rating,
		Comment:    comment,
		Stuck:      outcome.Stuck,
	}
	if err := playtest.AppendRecord(playtest.ResultsFile, rec); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to record playtest result: %v\n", err)
	} else {
		fmt.Printf("Session recorded to %s\n", playtest.ResultsFile)
	}
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
	fmt.Printf("Trump Rule: %s\n", g.TrumpRule)

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

// pubJudgeWeight scales the judge verdict into the PUBLICATION ranking (v3):
// among playable games, a certified-novel composition (verdict +1) gets +0.2
// added to its leaderboard score and a certified rediscovery (verdict < 0) gets
// the verdict*0.2 subtracted, so a novel game surfaces above a somewhat-higher-
// fitness variant -- but fitness stays the base (a novel-but-weak game does not
// leapfrog a much stronger one). Fixes the v2 gap where the population explored
// novel territory but the fitness-ranked output still surfaced high-fitness
// rediscoveries. Zero effect when no verdicts are loaded.
const pubJudgeWeight = 0.2

func sortAndTrim(inds []*evolution.Individual, n int, verdicts map[string]float64) []*evolution.Individual {
	// Deduplicate by genome ID first
	seen := make(map[string]bool)
	var unique []*evolution.Individual
	for _, ind := range inds {
		if ind == nil || ind.Genome == nil || seen[ind.Genome.ID] {
			continue
		}
		seen[ind.Genome.ID] = true
		unique = append(unique, ind)
	}

	// Order by OutputRank (the greedy-only running mean -- the commensurable
	// leaderboard key, Wave K fix 1) PLUS the judge-aware publication term, so
	// certified-novel games surface above higher-fitness rediscoveries while
	// fitness stays the base. With no verdicts loaded this is exactly OutputRank.
	// Published MCTS-mode means are reported in report.md/summary.json but never
	// ranked.
	pubScore := func(ind *evolution.Individual) float64 {
		s := ind.OutputRank()
		if len(verdicts) > 0 {
			s += pubJudgeWeight * verdicts[evolution.Composition(ind.Genome)]
		}
		return s
	}
	sort.Slice(unique, func(i, j int) bool {
		return pubScore(unique[i]) > pubScore(unique[j])
	})

	// Reserve slots per skeleton for diversity in output
	perSkeleton := n / 3
	if perSkeleton < 2 {
		perSkeleton = 2
	}

	used := make(map[int]bool)
	var result []*evolution.Individual
	skelCounts := make(map[genome.SkeletonType]int)

	for i, ind := range unique {
		if skelCounts[ind.Genome.Skeleton] < perSkeleton {
			result = append(result, ind)
			used[i] = true
			skelCounts[ind.Genome.Skeleton]++
		}
		if len(result) >= n {
			break
		}
	}

	for i, ind := range unique {
		if len(result) >= n {
			break
		}
		if !used[i] {
			result = append(result, ind)
		}
	}

	return result
}

// getAllSeeds returns the built-in seed pool. Single source of truth =
// seeds.All() (the 8 classics + Big Two), so the evolution init pool, the
// calibration set, and the novelty anchors can never drift apart (they used to:
// this list and seeds.All() were maintained separately, and Big Two was in one
// but not the other).
// loadJudgeVerdicts reads the -judge-verdicts JSON (a composition -> score map)
// into Config.JudgeVerdicts. Empty path => nil (the un-judged default, which
// leaves the run byte-identical). A read or parse error is fatal: a silently
// dropped verdict file would run a different experiment than the one requested.
func loadJudgeVerdicts(path string) map[string]float64 {
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "judge-verdicts: %v\n", err)
		os.Exit(1)
	}
	var m map[string]float64
	if err := json.Unmarshal(data, &m); err != nil {
		fmt.Fprintf(os.Stderr, "judge-verdicts: invalid JSON in %s: %v\n", path, err)
		os.Exit(1)
	}
	fmt.Printf("  Judge verdicts: %d compositions from %s\n", len(m), path)
	return m
}

// emitForJudging writes the top genomes at a chunk boundary to
// <dir>/gen<NNN>/rankNN.json so the out-of-loop judge can dossier and classify
// the new compositions, then append verdicts to the table before the next chunk.
func emitForJudging(dir string, gen int, top []*evolution.Individual) {
	gdir := filepath.Join(dir, fmt.Sprintf("gen%03d", gen))
	if err := os.MkdirAll(gdir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "emit: %v\n", err)
		return
	}
	for i, ind := range top {
		data, err := json.MarshalIndent(ind.Genome, "", "  ")
		if err != nil {
			continue
		}
		os.WriteFile(filepath.Join(gdir, fmt.Sprintf("rank%02d.json", i+1)), data, 0o644)
	}
	fmt.Printf("  Emitted %d top genomes to %s\n", len(top), gdir)
}

func getAllSeeds() []*genome.Genome {
	return seeds.All()
}

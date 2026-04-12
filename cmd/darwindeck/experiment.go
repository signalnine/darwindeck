package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/darwindeck/darwindeck/pkg/evolution"
	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

// ExperimentResult holds metrics for one run.
type ExperimentResult struct {
	Config     string  `json:"config"`
	Seed       uint64  `json:"seed"`
	Coverage   float64 `json:"coverage"`    // qualified cells / total cells
	QDScore    float64 `json:"qd_score"`    // sum of fitness in qualified cells
	PairwiseDist float64 `json:"pairwise_dist"` // mean pairwise behavior distance
	MedianFit  float64 `json:"median_fitness"`
	NumGames   int     `json:"num_qualified_games"`
	Duration   string  `json:"duration"`

	// Per-skeleton breakdown
	PerSkeleton map[string]SkeletonMetrics `json:"per_skeleton"`
}

type SkeletonMetrics struct {
	Coverage   float64 `json:"coverage"`
	QDScore    float64 `json:"qd_score"`
	NumGames   int     `json:"num_games"`
}

func cmdExperiment(args []string) {
	fs := flag.NewFlagSet("experiment", flag.ExitOnError)
	numSeeds := fs.Int("seeds", 15, "number of seeds per configuration")
	population := fs.Int("population", 500, "population size")
	generations := fs.Int("generations", 100, "number of generations")
	workers := fs.Int("workers", 0, "parallel workers (0=auto)")
	outputDir := fs.String("output", "output/experiments", "output directory")
	parallel := fs.Int("parallel", 3, "number of experiments to run in parallel")
	configsFlag := fs.String("configs", "baseline,map-elites,novelty", "comma-separated list of configs to run")
	fs.Parse(args)

	configs := splitCSV(*configsFlag)

	if *workers == 0 {
		*workers = runtime.NumCPU()
	}

	allSeeds := []*genome.Genome{
		seeds.CrazyEights(), seeds.MauMau(),
		seeds.Whist(), seeds.Hearts(), seeds.Spades(), seeds.OhHell(),
		seeds.GinRummy(), seeds.KnockRummy(),
	}

	fmt.Printf("DarwinDeck Diversity Experiment\n")
	fmt.Printf("  Configs: %s\n", *configsFlag)
	fmt.Printf("  Seeds per config: %d\n", *numSeeds)
	fmt.Printf("  Population: %d, Generations: %d\n", *population, *generations)
	fmt.Printf("  Workers per run: %d, Parallel runs: %d\n", *workers, *parallel)
	fmt.Printf("  Output: %s\n\n", *outputDir)

	os.MkdirAll(*outputDir, 0755)

	type runSpec struct {
		config string
		seed   uint64
	}

	var runs []runSpec
	for i := 0; i < *numSeeds; i++ {
		for ci, cfg := range configs {
			runs = append(runs, runSpec{cfg, uint64(i + 1 + ci*1000)})
		}
	}

	var results []ExperimentResult
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, *parallel)

	startTime := time.Now()

	for _, run := range runs {
		wg.Add(1)
		sem <- struct{}{}

		go func(spec runSpec) {
			defer wg.Done()
			defer func() { <-sem }()

			config := evolution.Config{
				PopulationSize: *population,
				Generations:    *generations,
				EliteSize:      10,
				TournamentSize: 5,
				Workers:        *workers / *parallel, // divide workers among parallel runs
				BaseSeed:       spec.seed * 1000,
				SaveTopN:       20,
			}

			runStart := time.Now()
			result := runExperiment(spec.config, config, allSeeds)
			result.Config = spec.config
			result.Seed = spec.seed
			result.Duration = time.Since(runStart).Round(time.Millisecond).String()

			mu.Lock()
			results = append(results, result)
			fmt.Printf("  Done: %s seed=%d coverage=%.2f qd=%.1f pairwise=%.3f games=%d (%s)\n",
				spec.config, spec.seed, result.Coverage, result.QDScore, result.PairwiseDist, result.NumGames, result.Duration)
			mu.Unlock()
		}(run)
	}

	wg.Wait()

	elapsed := time.Since(startTime)
	fmt.Printf("\nAll experiments complete in %s\n\n", elapsed.Round(time.Second))

	// Aggregate and report
	reportResults(results, *outputDir, configs)
}

func runExperiment(configName string, config evolution.Config, allSeeds []*genome.Genome) ExperimentResult {
	var individuals []*evolution.Individual
	var behaviors []evolution.BehaviorDescriptor

	switch configName {
	case "baseline":
		engine := evolution.NewEngine(config, allSeeds)
		engine.Run(nil)

		// Collect qualified individuals and compute behaviors
		for _, ind := range engine.Population {
			if !ind.Valid || ind.Fitness.TotalFitness < evolution.FitnessFloor {
				continue
			}
			runner := fitness.GetRunner(ind.Genome)
			if runner == nil {
				continue
			}
			randomAI := &sim.RandomAI{}
			result := sim.RunBatch(ind.Genome, runner, randomAI, 50, config.BaseSeed+99999)
			behavior := evolution.ComputeBehavior(result)
			individuals = append(individuals, ind)
			behaviors = append(behaviors, behavior)
		}

	case "map-elites":
		engine := evolution.NewMAPElitesEngine(config, allSeeds)
		engine.Run(nil)
		allQ := engine.AllQualified()
		for _, ind := range allQ {
			runner := fitness.GetRunner(ind.Genome)
			if runner == nil {
				continue
			}
			randomAI := &sim.RandomAI{}
			result := sim.RunBatch(ind.Genome, runner, randomAI, 50, config.BaseSeed+99999)
			behavior := evolution.ComputeBehavior(result)
			individuals = append(individuals, ind)
			behaviors = append(behaviors, behavior)
		}

	case "novelty", "hybrid":
		engine := evolution.NewNoveltyEngine(config, allSeeds)
		engine.Run(nil)
		inds, behavs := engine.AllQualified()
		individuals = inds
		behaviors = behavs
	}

	return computeMetrics(individuals, behaviors)
}

func computeMetrics(individuals []*evolution.Individual, behaviors []evolution.BehaviorDescriptor) ExperimentResult {
	result := ExperimentResult{
		NumGames:    len(individuals),
		PerSkeleton: make(map[string]SkeletonMetrics),
	}

	if len(individuals) == 0 {
		return result
	}

	// Virtual grid placement (per skeleton)
	type cellKey struct {
		skeleton genome.SkeletonType
		row, col int
	}
	grid := make(map[cellKey]float64) // best fitness per cell

	skelCoverage := make(map[genome.SkeletonType]map[[2]int]bool)
	skelQD := make(map[genome.SkeletonType]float64)
	skelCount := make(map[genome.SkeletonType]int)

	for i, ind := range individuals {
		skel := ind.Genome.Skeleton
		b := behaviors[i]
		row, col := b.GridCell(evolution.GridSize)
		key := cellKey{skel, row, col}

		if existing, ok := grid[key]; !ok || ind.Fitness.TotalFitness > existing {
			grid[key] = ind.Fitness.TotalFitness
		}

		if skelCoverage[skel] == nil {
			skelCoverage[skel] = make(map[[2]int]bool)
		}
		skelCoverage[skel][[2]int{row, col}] = true
		skelCount[skel]++
	}

	// Compute coverage and QD-score
	totalCells := 0
	totalOccupied := 0
	totalQD := 0.0

	skelNames := map[genome.SkeletonType]string{
		genome.Shedding:    "shedding",
		genome.TrickTaking: "trick_taking",
		genome.Rummy:       "rummy",
	}

	for skel, name := range skelNames {
		cells := evolution.GridSize * evolution.GridSize
		occupied := len(skelCoverage[skel])
		qd := 0.0
		for key, fit := range grid {
			if key.skeleton == skel {
				qd += fit
			}
		}

		totalCells += cells
		totalOccupied += occupied
		totalQD += qd
		skelQD[skel] = qd

		result.PerSkeleton[name] = SkeletonMetrics{
			Coverage: float64(occupied) / float64(cells),
			QDScore:  qd,
			NumGames: skelCount[skel],
		}
	}

	result.Coverage = float64(totalOccupied) / float64(totalCells)
	result.QDScore = totalQD

	// Mean pairwise distance
	if len(behaviors) > 1 {
		totalDist := 0.0
		pairs := 0
		for i := 0; i < len(behaviors); i++ {
			for j := i + 1; j < len(behaviors); j++ {
				totalDist += behaviors[i].Distance(behaviors[j])
				pairs++
			}
		}
		if pairs > 0 {
			result.PairwiseDist = totalDist / float64(pairs)
		}
	}

	// Median fitness
	var fitnesses []float64
	for _, ind := range individuals {
		fitnesses = append(fitnesses, ind.Fitness.TotalFitness)
	}
	sort.Float64s(fitnesses)
	result.MedianFit = fitnesses[len(fitnesses)/2]

	return result
}

func splitCSV(s string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			if i > start {
				out = append(out, s[start:i])
			}
			start = i + 1
		}
	}
	return out
}

func reportResults(results []ExperimentResult, outputDir string, configs []string) {
	// Group by config
	grouped := make(map[string][]ExperimentResult)
	for _, r := range results {
		grouped[r.Config] = append(grouped[r.Config], r)
	}

	fmt.Printf("%-12s | %8s | %8s | %8s | %8s | %6s\n",
		"Config", "Coverage", "QD-Score", "PairDist", "MedFit", "Games")
	fmt.Println("-------------|----------|----------|----------|----------|-------")

	for _, configName := range configs {
		runs := grouped[configName]
		if len(runs) == 0 {
			continue
		}

		var coverages, qds, dists, fits []float64
		totalGames := 0
		for _, r := range runs {
			coverages = append(coverages, r.Coverage)
			qds = append(qds, r.QDScore)
			dists = append(dists, r.PairwiseDist)
			fits = append(fits, r.MedianFit)
			totalGames += r.NumGames
		}

		fmt.Printf("%-12s | %5.3f+%02.0f | %5.1f+%02.0f | %6.4f+%02.0f | %5.3f+%02.0f | %5.0f\n",
			configName,
			median(coverages), iqr(coverages)*100,
			median(qds), iqr(qds),
			median(dists), iqr(dists)*1000,
			median(fits), iqr(fits)*100,
			float64(totalGames)/float64(len(runs)))
	}

	// Per-skeleton breakdown
	fmt.Println("\nPer-skeleton coverage (median):")
	for _, configName := range configs {
		runs := grouped[configName]
		var shedCov, trickCov, rummyCov []float64
		for _, r := range runs {
			if s, ok := r.PerSkeleton["shedding"]; ok {
				shedCov = append(shedCov, s.Coverage)
			}
			if s, ok := r.PerSkeleton["trick_taking"]; ok {
				trickCov = append(trickCov, s.Coverage)
			}
			if s, ok := r.PerSkeleton["rummy"]; ok {
				rummyCov = append(rummyCov, s.Coverage)
			}
		}
		fmt.Printf("  %-12s shed=%.3f trick=%.3f rummy=%.3f\n",
			configName, median(shedCov), median(trickCov), median(rummyCov))
	}

	// Save raw results
	data, _ := json.MarshalIndent(results, "", "  ")
	outPath := filepath.Join(outputDir, "results.json")
	os.WriteFile(outPath, data, 0644)
	fmt.Printf("\nRaw results saved to %s\n", outPath)
}

func median(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sorted := make([]float64, len(vals))
	copy(sorted, vals)
	sort.Float64s(sorted)
	return sorted[len(sorted)/2]
}

func iqr(vals []float64) float64 {
	if len(vals) < 4 {
		return 0
	}
	sorted := make([]float64, len(vals))
	copy(sorted, vals)
	sort.Float64s(sorted)
	q1 := sorted[len(sorted)/4]
	q3 := sorted[3*len(sorted)/4]
	return q3 - q1
}

// Suppress unused import warning
var _ = math.Sqrt

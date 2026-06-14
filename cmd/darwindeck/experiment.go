package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"math/rand/v2"
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
)

// ExperimentResult holds metrics for one run.
type ExperimentResult struct {
	Config       string  `json:"config"`
	Seed         uint64  `json:"seed"`
	Coverage     float64 `json:"coverage"`      // qualified cells / total cells
	QDScore      float64 `json:"qd_score"`      // sum of fitness in qualified cells
	PairwiseDist float64 `json:"pairwise_dist"` // mean pairwise behavior distance
	MedianFit    float64 `json:"median_fitness"`
	NumGames     int     `json:"num_qualified_games"`
	Duration     string  `json:"duration"`
	DurationSec  float64 `json:"duration_sec"`

	// Per-skeleton breakdown
	PerSkeleton map[string]SkeletonMetrics `json:"per_skeleton"`
}

// knownExperimentConfigs is the config universe accepted by -configs.
// "mapelites" and "hybrid" are aliases (the audit plan and the evolve
// command use those spellings). "random" is the null-hypothesis control
// (audit Task 27): same evaluation budget, no selection.
var knownExperimentConfigs = map[string]bool{
	"baseline":   true,
	"map-elites": true,
	"mapelites":  true,
	"novelty":    true,
	"hybrid":     true,
	"random":     true,
}

// validateConfigs rejects unknown config names up front. Before this check
// existed an unknown name fell through runExperiment's switch and silently
// reported all-zero metrics.
func validateConfigs(configs []string) error {
	if len(configs) == 0 {
		return fmt.Errorf("no configs given")
	}
	for _, c := range configs {
		if !knownExperimentConfigs[c] {
			return fmt.Errorf("unknown config %q (universe: baseline, map-elites, novelty/hybrid, random)", c)
		}
	}
	return nil
}

type SkeletonMetrics struct {
	Coverage float64 `json:"coverage"`
	QDScore  float64 `json:"qd_score"`
	NumGames int     `json:"num_games"`
}

func cmdExperiment(args []string) {
	fs := flag.NewFlagSet("experiment", flag.ExitOnError)
	numSeeds := fs.Int("seeds", 15, "number of seeds per configuration")
	population := fs.Int("population", 500, "population size")
	generations := fs.Int("generations", 100, "number of generations")
	workers := fs.Int("workers", 0, "parallel workers (0=auto)")
	outputDir := fs.String("output", "output/experiments", "output directory")
	parallel := fs.Int("parallel", 3, "number of experiments to run in parallel")
	configsFlag := fs.String("configs", "baseline,map-elites,novelty,random", "comma-separated list of configs to run")
	mctsDecile := fs.Float64("mcts-decile", 0.10,
		"fraction of each generation (ranked by greedy-only running mean) re-evaluated with MCTS; 0 disables; applies to baseline/novelty (map-elites and random ignore it)")
	fs.Parse(args)

	configs := splitCSV(*configsFlag)
	if err := validateConfigs(configs); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if *workers == 0 {
		*workers = runtime.NumCPU()
	}

	allSeeds := []*genome.Genome{
		seeds.CrazyEights(), seeds.MauMau(),
		seeds.Whist(), seeds.Hearts(), seeds.Spades(), seeds.OhHell(),
		seeds.GinRummy(), seeds.KnockRummy(),
		seeds.BigTwo(), // climbing skeleton seed (novelty evolution)
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
				MCTSDecile:     *mctsDecile, // default on (0.10); map-elites and random ignore it
			}

			runStart := time.Now()
			result := runExperiment(spec.config, config, allSeeds)
			result.Config = spec.config
			result.Seed = spec.seed
			dur := time.Since(runStart)
			result.Duration = dur.Round(time.Millisecond).String()
			result.DurationSec = dur.Seconds()

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

		// Collect qualified individuals and compute behaviors.
		// evolution.BehaviorBatch is the single descriptor-batch site: it
		// runs the genome's borrowed-mechanic hooks, so the descriptor
		// describes the same hooked game fitness evaluated.
		for _, ind := range engine.Population {
			if !ind.Valid || ind.Fitness.TotalFitness < evolution.FitnessFloor {
				continue
			}
			result, ok := evolution.BehaviorBatch(ind.Genome, config.BaseSeed+99999)
			if !ok {
				continue
			}
			behavior := evolution.ComputeBehavior(result)
			individuals = append(individuals, ind)
			behaviors = append(behaviors, behavior)
		}

	case "map-elites", "mapelites":
		engine := evolution.NewMAPElitesEngine(config, allSeeds)
		engine.Run(nil)
		allQ := engine.AllQualified()
		for _, ind := range allQ {
			result, ok := evolution.BehaviorBatch(ind.Genome, config.BaseSeed+99999)
			if !ok {
				continue
			}
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

	case "random":
		individuals, behaviors = runRandomSearch(config, allSeeds)

	default:
		// validateConfigs rejects unknown names before any run starts.
		panic(fmt.Sprintf("runExperiment: unvalidated config %q", configName))
	}

	return computeMetrics(individuals, behaviors)
}

// runRandomSearch is the experiment's null-hypothesis control (audit Task
// 27): pure random genome sampling with NO selection. Every wave draws
// PopulationSize fresh mutants directly from the seed pool -- never from
// prior results -- evaluates them, and offers the valid ones to a best-seen
// archive (one best-fitness occupant per skeleton x behavior-grid cell, the
// same virtual grid computeMetrics scores). If evolution does not beat this
// on coverage/QD-score, the search space is small enough that selection adds
// nothing.
//
// Budget parity: all three evolution configs evaluate (Generations+1) *
// PopulationSize genomes (baseline/novelty run a final post-loop evaluation
// pass; MAP-Elites seeds its archive with one extra wave), so this runs
// Generations+1 waves of PopulationSize mutants with the engines' exact seed
// derivation (BaseSeed + wave*10000 + idx). Like map-elites and novelty it
// also runs one behavior batch (evolution.BehaviorBatch, +5000) per valid
// evaluation; baseline instead batches behaviors once on its final
// population.
//
// Config.MCTSDecile is deliberately ignored, as MAP-Elites ignores it: there
// is no persistent population whose greedy-only ranking could gate the MCTS
// tier, and granting it to each wave's transient top decile would change the
// reported fitness mode without any selection meaning. Archive cells keep the
// single evaluation that admitted them (no challenge re-evaluation): a
// best-seen max over (Generations+1)*PopulationSize single evaluations is
// upward-biased by luck relative to the engines' running means, which makes
// the null HARDER to beat -- conservative in the comparison's favor.
func runRandomSearch(config evolution.Config, allSeeds []*genome.Genome) ([]*evolution.Individual, []evolution.BehaviorDescriptor) {
	rng := rand.New(rand.NewPCG(config.BaseSeed, 0))

	workers := config.Workers
	if workers < 1 {
		workers = 1
	}

	type cellKey struct {
		skel     genome.SkeletonType
		row, col int
	}
	type entry struct {
		ind      *evolution.Individual
		behavior evolution.BehaviorDescriptor
	}
	archive := make(map[cellKey]entry)

	type evalSlot struct {
		ind      *evolution.Individual
		behavior evolution.BehaviorDescriptor
		ok       bool
	}

	for wave := 0; wave <= config.Generations; wave++ {
		// Mutant generation is sequential: rng is not goroutine-safe and the
		// draw order is what makes runs reproducible for a fixed BaseSeed.
		mutants := make([]*genome.Genome, config.PopulationSize)
		for i := range mutants {
			seed := allSeeds[rng.IntN(len(allSeeds))]
			g := evolution.Mutate(seed, rng, allSeeds)
			g.ID = fmt.Sprintf("rand%d_%d", wave, i)
			g.Generation = wave
			mutants[i] = g
		}

		slots := make([]evalSlot, len(mutants))
		var wg sync.WaitGroup
		sem := make(chan struct{}, workers)
		for i, g := range mutants {
			wg.Add(1)
			sem <- struct{}{}
			go func(idx int, g *genome.Genome) {
				defer wg.Done()
				defer func() { <-sem }()

				seed := config.BaseSeed + uint64(wave)*10000 + uint64(idx)
				result := fitness.Evaluate(g, seed)
				if !result.Valid {
					return
				}
				batch, ok := evolution.BehaviorBatch(g, seed+5000)
				if !ok {
					return
				}
				slots[idx] = evalSlot{
					ind: &evolution.Individual{
						Genome:     g,
						Fitness:    result.Metrics,
						Valid:      true,
						EvalCount:  1,
						FitnessSum: result.Metrics.TotalFitness,
					},
					behavior: evolution.ComputeBehavior(batch),
					ok:       true,
				}
			}(i, g)
		}
		wg.Wait()

		// Sequential insertion in index order keeps the archive deterministic.
		for _, s := range slots {
			if !s.ok {
				continue
			}
			row, col := s.behavior.GridCell(evolution.GridSize)
			key := cellKey{s.ind.Genome.Skeleton, row, col}
			if cur, exists := archive[key]; !exists || s.ind.Fitness.TotalFitness > cur.ind.Fitness.TotalFitness {
				archive[key] = entry{ind: s.ind, behavior: s.behavior}
			}
		}
	}

	// Output applies the FitnessFloor exactly like MAP-Elites' AllQualified;
	// sorted cell order keeps downstream float sums reproducible.
	keys := make([]cellKey, 0, len(archive))
	for k := range archive {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(a, b int) bool {
		if keys[a].skel != keys[b].skel {
			return keys[a].skel < keys[b].skel
		}
		if keys[a].row != keys[b].row {
			return keys[a].row < keys[b].row
		}
		return keys[a].col < keys[b].col
	})

	var individuals []*evolution.Individual
	var behaviors []evolution.BehaviorDescriptor
	for _, k := range keys {
		e := archive[k]
		if e.ind.Fitness.TotalFitness < evolution.FitnessFloor {
			continue
		}
		individuals = append(individuals, e.ind)
		behaviors = append(behaviors, e.behavior)
	}
	return individuals, behaviors
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
	result.MedianFit = median(fitnesses)

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

	// Per-config wall time. "total" sums per-run wall clocks (runs from
	// different configs interleave, so this is occupancy, not elapsed wall
	// time of the experiment).
	fmt.Println("\nPer-config wall time:")
	configSecs := make(map[string][]float64)
	for _, configName := range configs {
		runs := grouped[configName]
		if len(runs) == 0 {
			continue
		}
		var secs []float64
		total := 0.0
		for _, r := range runs {
			secs = append(secs, r.DurationSec)
			total += r.DurationSec
		}
		configSecs[configName] = secs
		fmt.Printf("  %-12s total=%s median-run=%s (n=%d)\n",
			configName,
			(time.Duration(total * float64(time.Second))).Round(time.Second),
			(time.Duration(median(secs) * float64(time.Second))).Round(time.Millisecond),
			len(runs))
	}

	// Pairwise Mann-Whitney U on the two headline metrics (audit Task 27).
	pairwise := pairwiseComparisons(grouped, configs)
	if len(pairwise) > 0 {
		fmt.Println("\nPairwise Mann-Whitney U (two-sided, normal approximation with tie + continuity correction):")
		for _, p := range pairwise {
			fmt.Printf("  %-9s %-12s vs %-12s: U=%6.1f z=%+7.3f p=%6.4f r=%+.3f (n=%d vs %d)%s\n",
				p.Metric, p.ConfigA, p.ConfigB, p.U, p.Z, p.P, p.RankBiserial, p.NA, p.NB, p.Flag)
		}
		fmt.Println("  r is the rank-biserial effect size (positive = first config larger).")
		fmt.Println("  Small-n caveat: the approximation needs n >= 8 per side, and even at the")
		fmt.Println("  default n=15 power is limited -- treat these as effect-size indications,")
		fmt.Println("  not strong claims.")
	}

	// Save raw results
	data, _ := json.MarshalIndent(results, "", "  ")
	outPath := filepath.Join(outputDir, "results.json")
	os.WriteFile(outPath, data, 0644)
	fmt.Printf("\nRaw results saved to %s\n", outPath)

	// Save aggregate statistics alongside the raw runs.
	summary := experimentSummary{
		Note: "Mann-Whitney U: two-sided normal approximation with tie + continuity correction; " +
			"adequate at n >= 8 per side. Small-n results (including the default n=15 seeds " +
			"per config) are effect-size indications, not strong claims.",
		Pairwise: pairwise,
	}
	for _, configName := range configs {
		runs := grouped[configName]
		if len(runs) == 0 {
			continue
		}
		var coverages, qds, secs []float64
		total := 0.0
		for _, r := range runs {
			coverages = append(coverages, r.Coverage)
			qds = append(qds, r.QDScore)
			secs = append(secs, r.DurationSec)
			total += r.DurationSec
		}
		summary.Configs = append(summary.Configs, configSummary{
			Config:         configName,
			Runs:           len(runs),
			MedianCoverage: median(coverages),
			IQRCoverage:    iqr(coverages),
			MedianQDScore:  median(qds),
			IQRQDScore:     iqr(qds),
			TotalWallSec:   total,
			MedianRunSec:   median(secs),
		})
	}
	sumData, _ := json.MarshalIndent(summary, "", "  ")
	sumPath := filepath.Join(outputDir, "summary_stats.json")
	os.WriteFile(sumPath, sumData, 0644)
	fmt.Printf("Aggregate statistics saved to %s\n", sumPath)
}

// experimentSummary is the persisted aggregate report (summary_stats.json).
type experimentSummary struct {
	Note     string               `json:"note"`
	Configs  []configSummary      `json:"configs"`
	Pairwise []pairwiseComparison `json:"pairwise"`
}

type configSummary struct {
	Config         string  `json:"config"`
	Runs           int     `json:"runs"`
	MedianCoverage float64 `json:"median_coverage"`
	IQRCoverage    float64 `json:"iqr_coverage"`
	MedianQDScore  float64 `json:"median_qd_score"`
	IQRQDScore     float64 `json:"iqr_qd_score"`
	TotalWallSec   float64 `json:"total_wall_sec"`
	MedianRunSec   float64 `json:"median_run_sec"`
}

type pairwiseComparison struct {
	Metric       string  `json:"metric"`
	ConfigA      string  `json:"config_a"`
	ConfigB      string  `json:"config_b"`
	NA           int     `json:"n_a"`
	NB           int     `json:"n_b"`
	U            float64 `json:"u"`
	Z            float64 `json:"z"`
	P            float64 `json:"p_two_sided"`
	RankBiserial float64 `json:"rank_biserial"`
	Flag         string  `json:"flag,omitempty"`
}

// pairwiseComparisons runs the two-sided Mann-Whitney U for every config pair
// on coverage and QD-score, in the CLI's config order.
func pairwiseComparisons(grouped map[string][]ExperimentResult, configs []string) []pairwiseComparison {
	metrics := []struct {
		name string
		get  func(ExperimentResult) float64
	}{
		{"coverage", func(r ExperimentResult) float64 { return r.Coverage }},
		{"qd-score", func(r ExperimentResult) float64 { return r.QDScore }},
	}

	var out []pairwiseComparison
	for _, m := range metrics {
		for i := 0; i < len(configs); i++ {
			for j := i + 1; j < len(configs); j++ {
				a, b := grouped[configs[i]], grouped[configs[j]]
				if len(a) == 0 || len(b) == 0 {
					continue
				}
				var xs, ys []float64
				for _, r := range a {
					xs = append(xs, m.get(r))
				}
				for _, r := range b {
					ys = append(ys, m.get(r))
				}
				res := mannWhitneyU(xs, ys)
				flag := ""
				if res.Degenerate {
					flag = " [degenerate: all values tied]"
				} else if res.N1 < 8 || res.N2 < 8 {
					flag = " [n<8 per side: normal approximation unreliable]"
				}
				out = append(out, pairwiseComparison{
					Metric:       m.name,
					ConfigA:      configs[i],
					ConfigB:      configs[j],
					NA:           res.N1,
					NB:           res.N2,
					U:            res.U,
					Z:            res.Z,
					P:            res.P,
					RankBiserial: res.RankBiserial,
					Flag:         flag,
				})
			}
		}
	}
	return out
}

// median returns the standard sample median: the middle order statistic for
// odd n, the mean of the two middle order statistics for even n (the
// previous implementation returned the upper-middle value for even n, which
// is not the median). 0 for empty input. Does not mutate vals.
func median(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sorted := make([]float64, len(vals))
	copy(sorted, vals)
	sort.Float64s(sorted)
	n := len(sorted)
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}

// iqr returns Q3 - Q1 using the simple index-based quartile convention
// Q1 = sorted[n/4], Q3 = sorted[3n/4]. At the harness default of n=15 runs
// per config this picks the 4th and 12th order statistics -- the Tukey
// quartiles for n=15. It is a coarse spread indicator, not an interpolated
// quantile estimator; returns 0 for n < 4. Does not mutate vals.
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

// mannWhitneyResult holds a two-sided Mann-Whitney U test outcome.
type mannWhitneyResult struct {
	N1, N2 int
	// U is the U statistic of the FIRST sample: the number of (x, y) pairs
	// with x > y, counting ties as 0.5. U1 + U2 = N1*N2.
	U float64
	// Z is the signed standardized statistic after continuity correction;
	// negative means the first sample ranks lower.
	Z float64
	// P is the two-sided p-value from the normal approximation.
	P float64
	// RankBiserial is the rank-biserial correlation r = 2U/(N1*N2) - 1, an
	// effect size in [-1, 1]; positive means the first sample tends larger.
	RankBiserial float64
	// Degenerate is true when the test carries no information: an empty
	// side, or every pooled value tied (sigma = 0). P is 1 in that case.
	Degenerate bool
}

// mannWhitneyU computes the two-sided Mann-Whitney U test using the NORMAL
// APPROXIMATION with tie correction and continuity correction (the same
// method as scipy.stats.mannwhitneyu(method='asymptotic'); the test fixtures
// are cross-checked against it):
//
//	U1   = R1 - n1(n1+1)/2          (R1 = rank sum of x, average ranks for ties)
//	mu   = n1*n2/2
//	s^2  = n1*n2/12 * ((N+1) - sum(t^3 - t)/(N(N-1)))   (t = tie-group sizes)
//	z    = (|U1 - mu| - 0.5)/s, clamped at 0
//	p    = 2*(1 - Phi(z)) = erfc(z/sqrt(2))
//
// APPROXIMATION NOTE (audit Task 27): the normal approximation is adequate at
// n >= 8 per side; below that the exact U distribution differs noticeably and
// callers must treat p as an effect-size indication only (the report prints
// that caveat). No exact-distribution fallback is implemented.
func mannWhitneyU(x, y []float64) mannWhitneyResult {
	res := mannWhitneyResult{N1: len(x), N2: len(y), P: 1}
	if len(x) == 0 || len(y) == 0 {
		res.Degenerate = true
		return res
	}

	type obs struct {
		v     float64
		first bool
	}
	pooled := make([]obs, 0, len(x)+len(y))
	for _, v := range x {
		pooled = append(pooled, obs{v, true})
	}
	for _, v := range y {
		pooled = append(pooled, obs{v, false})
	}
	sort.Slice(pooled, func(i, j int) bool { return pooled[i].v < pooled[j].v })

	// Assign average ranks per tie group; accumulate the tie term.
	n := len(pooled)
	r1 := 0.0
	tieTerm := 0.0
	for i := 0; i < n; {
		j := i
		for j < n && pooled[j].v == pooled[i].v {
			j++
		}
		t := float64(j - i)
		avgRank := float64(i+j+1) / 2 // 1-based ranks i+1..j
		for k := i; k < j; k++ {
			if pooled[k].first {
				r1 += avgRank
			}
		}
		tieTerm += t*t*t - t
		i = j
	}

	fn1, fn2, fN := float64(res.N1), float64(res.N2), float64(n)
	u1 := r1 - fn1*(fn1+1)/2
	res.U = u1
	res.RankBiserial = 2*u1/(fn1*fn2) - 1

	mu := fn1 * fn2 / 2
	sigma2 := fn1 * fn2 / 12 * ((fN + 1) - tieTerm/(fN*(fN-1)))
	if sigma2 <= 0 {
		// Every pooled value tied: no ordering information.
		res.Degenerate = true
		return res
	}

	diff := u1 - mu
	num := math.Abs(diff) - 0.5 // continuity correction
	if num < 0 {
		num = 0
	}
	z := num / math.Sqrt(sigma2)
	res.P = math.Erfc(z / math.Sqrt2)
	if diff < 0 {
		res.Z = -z
	} else {
		res.Z = z
	}
	return res
}

// calibrate implements the `calibrate` subcommand (audit remediation Task
// 13.5): a reporting-only view of the metric stack. It evaluates the 8
// classic seeds plus the 2 degenerate fixtures over the pinned
// fitness.CalibrationSeeds list through the REAL pipeline (fitness.Evaluate:
// Tier 0 -> Tier 1 -> Tier 2) and prints per-genome means and sds of the 5
// RAW metrics -- no weighting -- so each metric column can be sanity-
// inspected independently before Task 14 tunes weights. Any column that is
// still a skeleton constant means its Phase 2 task is incomplete.
package main

import (
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"time"

	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
)

// metricNames are the 5 raw fitness metrics in report order, matching
// rawMetrics below.
var metricNames = [5]string{"decisions", "arc", "interact", "skill", "length"}

// rawMetrics extracts the 5 unweighted metric values in metricNames order.
func rawMetrics(m fitness.Metrics) [5]float64 {
	return [5]float64{
		m.MeaningfulDecisions,
		m.GameArc,
		m.Interaction,
		m.SkillGradient,
		m.SessionLength,
	}
}

// calRow is one genome's aggregated calibration measurement.
type calRow struct {
	id          string
	skeleton    string
	tier1Passes int // evaluations that reached Tier 2
	evals       int // evaluations attempted (len of the pinned seed list)
	means, sds  [5]float64
}

// vetoStatNames are the degeneracy-detector statistics in report order,
// matching vetoStats below (audit Task 28 round 3: detector thresholds are
// derived from these numbers measured on the classics, so the calibrate
// command must print them per genome).
var vetoStatNames = [7]string{"r_meanrun", "r_minseat", "r_churn", "r_allplay", "g_meanrun", "g_minseat", "g_timeout"}

// vetoStats extracts the detector statistics from one evaluation. The greedy
// columns are only present when the greedy batch ran (greedyRan false on
// random-batch vetoes and Tier 1 kills).
func vetoStats(d fitness.DegeneracyStats) [7]float64 {
	return [7]float64{
		d.RandomMeanRun,
		d.RandomMinSeatShare,
		d.RandomDeltaShare,
		d.RandomAllPlayable,
		d.GreedyMeanRun,
		d.GreedyMinSeatShare,
		d.GreedyTimeoutShare,
	}
}

// vetoRow is one genome's aggregated veto-statistics measurement, over every
// evaluation that reached Tier 2 (vetoed evaluations included: the
// statistics are exactly what the veto decided on).
type vetoRow struct {
	id            string
	players       int
	randomSamples int
	greedySamples int
	means         [7]float64
}

// aggregateVetoRow averages each statistic over its available samples
// (greedy columns only over evaluations whose greedy batch ran).
func aggregateVetoRow(id string, players int, stats []fitness.DegeneracyStats) vetoRow {
	row := vetoRow{id: id, players: players}
	var greedy []fitness.DegeneracyStats
	for _, d := range stats {
		if d.GreedyRan {
			greedy = append(greedy, d)
		}
	}
	row.randomSamples = len(stats)
	row.greedySamples = len(greedy)
	for _, d := range stats {
		s := vetoStats(d)
		for k := 0; k < 4; k++ {
			row.means[k] += s[k]
		}
	}
	for k := 0; k < 4; k++ {
		if len(stats) > 0 {
			row.means[k] /= float64(len(stats))
		}
	}
	for _, d := range greedy {
		s := vetoStats(d)
		for k := 4; k < 7; k++ {
			row.means[k] += s[k]
		}
	}
	for k := 4; k < 7; k++ {
		if len(greedy) > 0 {
			row.means[k] /= float64(len(greedy))
		}
	}
	return row
}

// printVetoTable renders the degeneracy-detector statistics table. minseat
// columns print both the raw share and its multiple of the fair share 1/N
// (the threshold is 0.5x fair share).
func printVetoTable(w io.Writer, rows []vetoRow) {
	fmt.Fprintf(w, "%-22s %-7s %-9s", "genome", "n(r/g)", "players")
	for _, name := range vetoStatNames {
		fmt.Fprintf(w, " %-14s", name)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, strings.Repeat("-", 22+1+7+1+9+7*15))
	for _, r := range rows {
		fmt.Fprintf(w, "%-22s %-7s %-9d", r.id,
			fmt.Sprintf("%d/%d", r.randomSamples, r.greedySamples), r.players)
		fair := 1.0 / float64(r.players)
		for k, name := range vetoStatNames {
			cell := "n/a"
			n := r.randomSamples
			if k >= 4 {
				n = r.greedySamples
			}
			if n > 0 {
				switch name {
				case "r_minseat", "g_minseat":
					cell = fmt.Sprintf("%.3f (%.2fx)", r.means[k], r.means[k]/fair)
				default:
					cell = fmt.Sprintf("%.3f", r.means[k])
				}
			}
			fmt.Fprintf(w, " %-14s", cell)
		}
		fmt.Fprintln(w)
	}
}

// aggregateRow computes per-metric mean and sd over the samples (one sample
// per evaluation that passed Tier 1).
func aggregateRow(id, skeleton string, evals int, samples [][5]float64) calRow {
	row := calRow{id: id, skeleton: skeleton, evals: evals, tier1Passes: len(samples)}
	xs := make([]float64, len(samples))
	for k := range metricNames {
		for i, s := range samples {
			xs[i] = s[k]
		}
		row.means[k], row.sds[k] = meanSD(xs)
	}
	return row
}

// meanSD returns the mean and population standard deviation (divide by n,
// the calibration suite's convention). Empty input returns 0, 0.
func meanSD(xs []float64) (mean, sd float64) {
	if len(xs) == 0 {
		return 0, 0
	}
	for _, x := range xs {
		mean += x
	}
	mean /= float64(len(xs))
	var sq float64
	for _, x := range xs {
		sq += (x - mean) * (x - mean)
	}
	return mean, math.Sqrt(sq / float64(len(xs)))
}

// formatCell renders one metric column entry; n is the number of Tier 2
// samples behind the row -- with none there is nothing to report.
func formatCell(mean, sd float64, n int) string {
	if n == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.3f sd %.3f", mean, sd)
}

// printCalibrationTable renders the per-genome report table.
func printCalibrationTable(w io.Writer, rows []calRow) {
	fmt.Fprintf(w, "%-22s %-13s %-6s", "genome", "skeleton", "tier1")
	for _, name := range metricNames {
		fmt.Fprintf(w, " %-15s", name)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, strings.Repeat("-", 22+1+13+1+6+5*16))
	for _, r := range rows {
		fmt.Fprintf(w, "%-22s %-13s %-6s", r.id, r.skeleton,
			fmt.Sprintf("%d/%d", r.tier1Passes, r.evals))
		for k := range metricNames {
			fmt.Fprintf(w, " %-15s", formatCell(r.means[k], r.sds[k], r.tier1Passes))
		}
		fmt.Fprintln(w)
	}
}

func cmdCalibrate(args []string) {
	fs := flag.NewFlagSet("calibrate", flag.ExitOnError)
	fs.Parse(args)

	classics := seeds.All()
	degens := append([]*genome.Genome{seeds.InstantKnockRummy(), seeds.ForcedShedding()},
		seeds.RejectedChampions()...)
	genomes := append(classics, degens...)
	pinned := fitness.CalibrationSeeds

	fmt.Printf("DarwinDeck calibration report (raw metric means, no weighting)\n")
	fmt.Printf("%d genomes (%d classics + %d degenerate fixtures) x %d pinned seeds %v\n\n",
		len(genomes), len(classics), len(degens), len(pinned), pinned)

	start := time.Now()
	totalGames := 0
	rows := make([]calRow, 0, len(genomes))
	vetoRows := make([]vetoRow, 0, len(genomes))
	var kills []string

	for _, g := range genomes {
		var samples [][5]float64
		var detectorStats []fitness.DegeneracyStats
		for _, seed := range pinned {
			res := fitness.Evaluate(g, seed)
			if len(res.Tier0Errors) > 0 {
				fmt.Fprintf(os.Stderr, "%s: tier-0 errors (calibration genomes must be statically valid): %v\n",
					g.ID, res.Tier0Errors)
				os.Exit(1)
			}
			totalGames += fitness.GamesPerEvaluation(res)
			if res.Tier1.Passed {
				detectorStats = append(detectorStats, res.Degeneracy)
			}
			if !res.Valid {
				reason := res.Tier1.Reason
				if res.DegenerateReason != "" {
					reason = "tier-2 degeneracy veto: " + res.DegenerateReason
				}
				kills = append(kills, fmt.Sprintf("%s seed %d: %s", g.ID, seed, reason))
				continue
			}
			samples = append(samples, rawMetrics(res.Metrics))
		}
		rows = append(rows, aggregateRow(g.ID, g.Skeleton.String(), len(pinned), samples))
		vetoRows = append(vetoRows, aggregateVetoRow(g.ID, g.Players, detectorStats))
	}
	elapsed := time.Since(start)

	printCalibrationTable(os.Stdout, rows)

	fmt.Printf("\nDegeneracy-detector statistics (means over evals reaching Tier 2; vetoed evals included).\n")
	fmt.Printf("Thresholds: meanrun > 6 vetoes; minseat < 0.50x fair share vetoes; churn > 0.05 vetoes (rummy);\n")
	fmt.Printf("allplay > 0.70 vetoes (shedding); g_timeout > 0.10 vetoes. r_ = random, g_ = greedy batch.\n\n")
	printVetoTable(os.Stdout, vetoRows)

	if len(kills) > 0 {
		fmt.Printf("\nPipeline kills (%d):\n", len(kills))
		for _, k := range kills {
			fmt.Printf("  %s\n", k)
		}
	}

	fmt.Printf("\nThroughput: %d games in %s = %.0f games/sec (one calibrate process; batches use Wave I game-parallelism)\n",
		totalGames, elapsed.Round(time.Millisecond), float64(totalGames)/elapsed.Seconds())
}

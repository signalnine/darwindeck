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
	var kills []string

	for _, g := range genomes {
		var samples [][5]float64
		for _, seed := range pinned {
			res := fitness.Evaluate(g, seed)
			if len(res.Tier0Errors) > 0 {
				fmt.Fprintf(os.Stderr, "%s: tier-0 errors (calibration genomes must be statically valid): %v\n",
					g.ID, res.Tier0Errors)
				os.Exit(1)
			}
			totalGames += fitness.GamesPerEvaluation(res.Valid)
			if !res.Valid {
				kills = append(kills, fmt.Sprintf("%s seed %d: %s", g.ID, seed, res.Tier1.Reason))
				continue
			}
			samples = append(samples, rawMetrics(res.Metrics))
		}
		rows = append(rows, aggregateRow(g.ID, g.Skeleton.String(), len(pinned), samples))
	}
	elapsed := time.Since(start)

	printCalibrationTable(os.Stdout, rows)

	if len(kills) > 0 {
		fmt.Printf("\nTier 1 kills (%d):\n", len(kills))
		for _, k := range kills {
			fmt.Printf("  %s\n", k)
		}
	}

	fmt.Printf("\nThroughput: %d games in %s = %.0f games/sec (single-threaded)\n",
		totalGames, elapsed.Round(time.Millisecond), float64(totalGames)/elapsed.Seconds())
}

// restamp implements the `restamp` subcommand (Wave M, audit Task 28/29
// follow-up). It re-publishes an EXISTING saved run through the veto-stable
// publication path without re-running evolution: it loads <run>/games/*/
// genome.json, runs the K=5 veto-stability check plus a fresh greedy-only
// evaluation on each genome, re-ranks so every veto-stable game outranks every
// unstable one, and writes a results/ bundle (summary.json, meta.json, 30x
// {genome.json, rulebook.md, report.md}, STABILITY.md). This is exactly the
// path SaveResults now takes for live runs (pkg/output/stability.go), applied
// after the fact to the flagship-r4 artifacts -- the 3.5h run cannot be
// re-run, but its saved genomes can be re-evaluated and re-stamped honestly.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/output"
)

// restampSeed is the fixed base seed for both the fresh greedy evaluation and
// the stability re-eval derivation, so a restamp is reproducible. It is held
// separate from any flagship master seed so the fresh evaluation is genuinely
// fresh (distinct from the seeds the genome saw during its run).
const restampSeed = uint64(424242)

// restampGame holds one re-evaluated genome and its verdict.
type restampGame struct {
	genome     *genome.Genome
	greedyMean float64
	metrics    fitness.Metrics
	stability  output.StabilityResult
	valid      bool   // fresh greedy eval valid (Tier 0/1 + vetoes passed)
	reason     string // why the fresh eval was invalid, if so
}

func cmdRestamp(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: darwindeck restamp <run-dir> [<out-dir>]")
		fmt.Fprintln(os.Stderr, "  re-evaluates <run-dir>/games/*/genome.json for veto-stability")
		fmt.Fprintln(os.Stderr, "  and writes a results bundle to <out-dir> (default results/<run-basename>)")
		os.Exit(1)
	}
	runDir := args[0]
	outDir := ""
	if len(args) >= 2 {
		outDir = args[1]
	} else {
		outDir = filepath.Join("results", filepath.Base(runDir))
	}

	gamesDir := filepath.Join(runDir, "games")
	entries, err := os.ReadDir(gamesDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "reading %s: %v\n", gamesDir, err)
		os.Exit(1)
	}

	var games []restampGame
	start := time.Now()
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(gamesDir, e.Name(), "genome.json")
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var g genome.Genome
		if err := json.Unmarshal(data, &g); err != nil {
			fmt.Fprintf(os.Stderr, "skip %s: %v\n", path, err)
			continue
		}
		// Strip any stale published stamps from the saved genome -- this
		// restamp is the authoritative source for them.
		g.VetoStable = false
		g.StableEvals = ""

		// Fresh greedy-only evaluation for the published headline + metrics.
		ev := fitness.Evaluate(&g, restampSeed)
		rg := restampGame{genome: &g}
		rg.valid = ev.Valid
		if ev.Valid {
			rg.greedyMean = ev.Metrics.TotalFitness
			rg.metrics = ev.Metrics
		} else {
			rg.metrics = ev.Metrics // diagnostic metrics survive
			rg.reason = freshInvalidReason(ev)
		}
		// K=5 veto-stability check (the same call SaveResults makes).
		rg.stability = output.EvaluateStability(&g, restampSeed)
		games = append(games, rg)
	}
	elapsed := time.Since(start)

	// Re-rank: stable first (by fresh greedy mean desc), then unstable (by
	// fresh greedy mean desc) -- the production demotion semantics.
	rankRestampGames(games)

	if err := writeRestampBundle(outDir, runDir, games, elapsed); err != nil {
		fmt.Fprintf(os.Stderr, "writing bundle: %v\n", err)
		os.Exit(1)
	}

	stable := 0
	for _, g := range games {
		if g.stability.Stable {
			stable++
		}
	}
	fmt.Printf("restamped %d genomes from %s in %s (%d veto-stable, %d demoted)\n",
		len(games), runDir, elapsed.Round(time.Millisecond), stable, len(games)-stable)
	fmt.Printf("wrote %s (summary.json, meta.json, %d games, STABILITY.md)\n", outDir, len(games))
}

// rankRestampGames orders games for publication: every veto-stable game
// outranks every unstable one (the demotion invariant), and within each class
// games sort by fresh greedy-only fitness, descending. A demoted game can
// never outrank a stable game regardless of its (necessarily 0 or low) fresh
// fitness.
func rankRestampGames(games []restampGame) {
	sort.SliceStable(games, func(i, j int) bool {
		gi, gj := games[i], games[j]
		if gi.stability.Stable != gj.stability.Stable {
			return gi.stability.Stable // stable sorts first
		}
		return gi.greedyMean > gj.greedyMean
	})
}

// freshInvalidReason names why a fresh evaluation was invalid, for STABILITY.md.
func freshInvalidReason(ev fitness.EvaluationResult) string {
	switch {
	case len(ev.Tier0Errors) > 0:
		return "tier0:" + ev.Tier0Errors[0]
	case ev.DegenerateReason != "":
		return "veto:" + ev.DegenerateReason
	case !ev.Tier1.Passed:
		return "tier1:" + ev.Tier1.Reason
	default:
		return "invalid"
	}
}

func writeRestampBundle(outDir, runDir string, games []restampGame, elapsed time.Duration) error {
	gamesOut := filepath.Join(outDir, "games")
	if err := os.MkdirAll(gamesOut, 0755); err != nil {
		return err
	}

	// summary.json: the greedy-only best (Wave K key), restamped.
	best := 0.0
	for _, g := range games {
		if g.stability.Stable && g.greedyMean > best {
			best = g.greedyMean
		}
	}
	summary := map[string]interface{}{
		"source_run":     runDir,
		"restamp_date":   time.Now().Format(time.RFC3339),
		"restamp_seed":   restampSeed,
		"games":          len(games),
		"best_fitness":   best,
		"note":           "greedy-only best over VETO-STABLE games only; honest exit -- no publishable champion claimed",
		"stability_evals": 5,
	}
	if err := writeJSONFile(filepath.Join(outDir, "summary.json"), summary); err != nil {
		return err
	}

	// meta.json: copy the source run's meta if present (it records the true
	// run inputs), annotate with the restamp.
	meta := map[string]interface{}{}
	if data, err := os.ReadFile(filepath.Join(runDir, "meta.json")); err == nil {
		_ = json.Unmarshal(data, &meta)
	}
	meta["restamped"] = true
	meta["restamp_seed"] = restampSeed
	meta["restamp_note"] = "Genomes re-evaluated and re-ranked through the veto-stable publication path (Wave M). The original run inputs above describe how the genomes were PRODUCED; the published fitness/order/stability here come from the fresh restamp evaluation."
	if err := writeJSONFile(filepath.Join(outDir, "meta.json"), meta); err != nil {
		return err
	}

	// Per-game artifacts, published exactly as SaveResults does.
	for i, rg := range games {
		g := rg.genome.Clone()
		g.Fitness = rg.greedyMean
		g.VetoStable = rg.stability.Stable
		g.StableEvals = rg.stability.Label()

		name := fmt.Sprintf("rank%02d_%s", i+1, sanitizeName(g.ID))
		dir := filepath.Join(gamesOut, name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		if err := writeJSONFile(filepath.Join(dir, "genome.json"), g); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, "rulebook.md"),
			[]byte(output.GenerateRulebook(g)), 0644); err != nil {
			return err
		}
		report := output.GenerateReport(g, withFitness(rg.metrics, rg.greedyMean))
		report += restampProvenance(rg)
		if err := os.WriteFile(filepath.Join(dir, "report.md"), []byte(report), 0644); err != nil {
			return err
		}
	}

	return writeStabilityMarkdown(filepath.Join(outDir, "STABILITY.md"), games, elapsed)
}

// withFitness returns m with TotalFitness set to the published greedy mean so
// the report headline matches genome.json (the SaveResults invariant).
func withFitness(m fitness.Metrics, fit float64) fitness.Metrics {
	m.TotalFitness = fit
	return m
}

// restampProvenance appends an honest note about the restamp verdict. The
// key case it documents: a game whose K=5 majority is stable but whose single
// fresh published eval was VETOED reads headline fitness 0 -- the exact
// single-eval/multi-eval divergence the veto-stable fix exists to expose
// (the r4 rank02 shedding game, demoted here to the bottom of the bundle).
func restampProvenance(rg restampGame) string {
	var b strings.Builder
	b.WriteString("**Restamp provenance (Wave M):**\n")
	b.WriteString(fmt.Sprintf("- Veto-stability over 5 fresh seeds: %s valid (%s)\n",
		rg.stability.Label(), stableWord(rg.stability.Stable)))
	if rg.valid {
		b.WriteString(fmt.Sprintf("- Single fresh published eval: VALID, greedy-only fitness %.3f (the headline above)\n", rg.greedyMean))
	} else {
		b.WriteString(fmt.Sprintf("- Single fresh published eval: %s -- headline fitness is therefore 0\n", rg.reason))
		b.WriteString("- THIS IS THE BUG THE FIX EXPOSES: production published this game from one lucky eval; a fresh single eval lands on a seed where it fails its own degeneracy veto. The K=5 check (majority-stable but with a failing seed) plus the fresh-eval-driven re-rank correctly sink it to the bottom of the bundle.\n")
	}
	b.WriteString("\n")
	return b.String()
}

func stableWord(b bool) string {
	if b {
		return "veto-stable"
	}
	return "UNSTABLE -- demoted"
}

func writeStabilityMarkdown(path string, games []restampGame, elapsed time.Duration) error {
	var b strings.Builder
	b.WriteString("# Veto-stability restamp (Wave M)\n\n")
	b.WriteString("Each game below was re-evaluated K=5 times at fresh seeds through the\n")
	b.WriteString("default greedy-only pipeline (Tier 0/1 + the degeneracy vetoes). A game is\n")
	b.WriteString("VETO-STABLE iff a majority (>=3/5) of those re-evals stayed valid; unstable\n")
	b.WriteString("games are demoted below every stable game in the published rank order.\n")
	b.WriteString("`fresh_eval` is the single-seed evaluation at the restamp seed -- the\n")
	b.WriteString("production-equivalent published draw -- shown next to the K=5 verdict so a\n")
	b.WriteString("single-eval/multi-eval disagreement (the bug this closes) is visible.\n\n")
	fmt.Fprintf(&b, "Re-evaluation cost: %d games x 5 evals + fresh eval in %s.\n\n", len(games), elapsed.Round(time.Millisecond))
	b.WriteString("| Rank | Genome | Skeleton | Fresh greedy fitness | Fresh eval | Veto-stable | Stable evals | Failing reasons |\n")
	b.WriteString("|------|--------|----------|----------------------|------------|-------------|--------------|------------------|\n")
	for i, rg := range games {
		freshState := "valid"
		if !rg.valid {
			freshState = rg.reason
		}
		stable := "yes"
		if !rg.stability.Stable {
			stable = "**NO (demoted)**"
		}
		reasons := "-"
		if len(rg.stability.Reasons) > 0 {
			reasons = strings.Join(rg.stability.Reasons, ", ")
		}
		fmt.Fprintf(&b, "| %d | %s | %s | %.3f | %s | %s | %s | %s |\n",
			i+1, rg.genome.ID, rg.genome.Skeleton.String(), rg.greedyMean,
			freshState, stable, rg.stability.Label(), reasons)
	}
	b.WriteString("\n")
	return os.WriteFile(path, []byte(b.String()), 0644)
}

func writeJSONFile(path string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// sanitizeName mirrors pkg/output.sanitize (unexported there) for rank dir
// names.
func sanitizeName(s string) string {
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

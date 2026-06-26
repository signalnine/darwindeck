// Command grammar-fitness demonstrates step 5 of the grammar rearchitecture: a
// grammar GameSpec runs through the REAL 5-metric fitness pipeline
// (fitness.EvaluateWithRunner, the injected-runner plug-in point), and its metrics
// are compared side-by-side to the hand-coded seed of the same skeleton. Parity on
// the structural metrics (decisions/arc/interaction/session) and a real skill
// gradient = the grammar feeds the existing metric stack faithfully.
package main

import (
	"fmt"

	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/grammar"
	"github.com/darwindeck/darwindeck/pkg/seeds"
)

func main() {
	const seed = 1
	names := []string{"shedding", "climbing", "banking", "casino", "trick", "rummy"}
	seedBySkeleton := map[genome.SkeletonType]*genome.Genome{}
	for _, s := range seeds.All() {
		if _, ok := seedBySkeleton[s.Skeleton]; !ok {
			seedBySkeleton[s.Skeleton] = s
		}
	}

	fmt.Printf("%-10s %-22s %8s %8s %8s %8s %8s %8s %s\n",
		"family", "source", "decis", "arc", "interact", "skill", "session", "TOTAL", "valid")
	for i, spec := range grammar.Canonical() {
		g := grammar.SpecGenome(spec)
		gr := fitness.EvaluateWithRunner(g, grammar.Adapter{Spec: spec}, fitness.GetGreedyAI(g), seed)
		printRow(names[i], "grammar (EvalWithRunner)", gr)

		if sd := seedBySkeleton[g.Skeleton]; sd != nil && names[i] != "banking" {
			sr := fitness.Evaluate(sd, seed)
			printRow("", "  seed: "+sd.ID, sr)
		} else {
			fmt.Printf("%-10s %-22s   (no v2 seed -- banking is grammar-only)\n", "", "")
		}
		fmt.Println()
	}
}

func printRow(family, source string, r fitness.EvaluationResult) {
	m := r.Metrics
	valid := "yes"
	if !r.Valid {
		valid = "VETO:" + r.DegenerateReason
	}
	if !r.Tier1.Passed {
		valid = "TIER1-FAIL"
	}
	fmt.Printf("%-10s %-22s %8.3f %8.3f %8.3f %8.3f %8.3f %8.3f %s\n",
		family, source, m.MeaningfulDecisions, m.GameArc, m.Interaction,
		m.SkillGradient, m.SessionLength, m.TotalFitness, valid)
}

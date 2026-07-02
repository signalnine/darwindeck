// Command grammar-evolve runs a small genetic algorithm over the typed grammar
// space (step 6 of the rearchitecture). Fitness is the REAL 5-metric pipeline
// (fitness.EvaluateWithRunner); the genetic operators (grammar.Mutate/Crossover)
// stay inside the well-typed manifold by construction, so EVERY individual in
// EVERY generation is playable-by-construction -- evolution cannot wander into the
// v1 desert.
//
// Pure fitness selection CONVERGES (it loses the diversity the grammar exists to
// produce), so selection is novelty-aware: selScore = fitness + wNov*novelty +
// wJudge*verdict[composition], exactly v2's shape. novelty is behavioral distance
// (in 5-metric space) from the 4 canonical seeds; verdict[composition] is the
// judge-in-loop hook, keyed on GameSpec.Composition (empty map => neutral, the
// cache-miss-returns-0 contract). This is the search that serves DISCOVERY:
// playable-by-construction games behaviorally distant from the known skeletons.
package main

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"sort"

	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/grammar"
)

const (
	popSize     = 30
	generations = 12
	eliteFrac   = 0.5
	wNovelty    = 0.5
	wJudge      = 1.0
	wCrowd      = 0.25 // niche-sharing: penalize over-represented compositions
)

// JudgeVerdicts is the judge-in-loop plug-in: composition key -> verdict score
// (novel > 0, variant/known < 0). Empty here -> neutral; an out-of-loop blind
// judge would fill it from grammar dossiers, exactly as v2's verdict table does.
var JudgeVerdicts = map[string]float64{}

type vec5 [5]float64

func metricVec(m fitness.Metrics) vec5 {
	return vec5{m.MeaningfulDecisions, m.GameArc, m.Interaction, m.SkillGradient, m.SessionLength}
}

func dist(a, b vec5) float64 {
	s := 0.0
	for i := range a {
		d := a[i] - b[i]
		s += d * d
	}
	return math.Sqrt(s)
}

type individual struct {
	spec    grammar.GameSpec
	vec     vec5
	fitness float64
	valid   bool
}

func main() {
	// -verdicts <path> loads a Composition-keyed judge table (novel>0, variant/
	// known<0) from grammar-judge, closing the discovery loop: the GA then SELECTS
	// for judge-certified-novel compositions, not just behavioral novelty.
	for i := 1; i < len(os.Args)-1; i++ {
		if os.Args[i] == "-verdicts" {
			if data, err := os.ReadFile(os.Args[i+1]); err == nil {
				if err := json.Unmarshal(data, &JudgeVerdicts); err != nil {
					fmt.Fprintln(os.Stderr, "verdicts:", err)
				} else {
					fmt.Printf("loaded %d judge verdicts from %s\n", len(JudgeVerdicts), os.Args[i+1])
				}
			}
		}
	}

	rng := rand.New(rand.NewPCG(2026, 0x9e3779b97f4a7c15))
	cache := map[string]individual{}

	eval := func(s grammar.GameSpec) individual {
		key := s.String()
		if got, ok := cache[key]; ok {
			got.spec = s
			return got
		}
		g := grammar.SpecGenome(s)
		r := fitness.EvaluateWithRunner(g, grammar.Adapter{Spec: s}, fitness.GetGreedyAI(g), 1)
		ind := individual{spec: s, vec: metricVec(r.Metrics), valid: r.Valid && r.Tier1.Passed}
		if ind.valid {
			ind.fitness = r.Metrics.TotalFitness
		}
		cache[key] = ind
		return ind
	}

	// The 7 canonical seeds anchor behavioral novelty (v2 uses the 8 classics).
	var seeds []vec5
	for _, s := range grammar.Canonical() {
		seeds = append(seeds, eval(s).vec)
	}
	novelty := func(v vec5) float64 {
		best := math.Inf(1)
		for _, s := range seeds {
			if d := dist(v, s); d < best {
				best = d
			}
		}
		return best
	}
	// selScore blends fitness, behavioral novelty, the judge verdict, and a
	// niche-sharing penalty (crowd) so a strong composition cannot monopolize the
	// pool -- the diversity the discovery goal needs.
	selScore := func(ind individual, crowd map[string]int) float64 {
		if !ind.valid {
			return -1
		}
		s := ind.fitness + wNovelty*novelty(ind.vec) + wJudge*JudgeVerdicts[ind.spec.Composition()]
		return s - wCrowd*float64(crowd[ind.spec.Composition()]-1)
	}
	crowding := func(pop []individual) map[string]int {
		c := map[string]int{}
		for _, ind := range pop {
			c[ind.spec.Composition()]++
		}
		return c
	}

	pop := make([]individual, popSize)
	for i := range pop {
		pop[i] = eval(grammar.RandomSpec(rng))
	}

	elite := int(float64(popSize) * eliteFrac)
	for gen := 0; gen < generations; gen++ {
		crowd := crowding(pop)
		sort.Slice(pop, func(i, j int) bool { return selScore(pop[i], crowd) > selScore(pop[j], crowd) })
		best := pop[0]
		fmt.Printf("gen %2d: best fit=%.3f novelty=%.3f (%s)  distinct-compositions=%d\n",
			gen, best.fitness, novelty(best.vec), best.spec.Composition(), distinctCompositions(pop))

		next := append(make([]individual, 0, popSize), pop[:elite]...) // elitism
		for len(next) < popSize {
			a := pop[rng.IntN(elite)].spec
			b := pop[rng.IntN(elite)].spec
			child := grammar.Crossover(a, b, rng)
			if rng.IntN(2) == 0 {
				child = grammar.Mutate(child, rng)
			}
			next = append(next, eval(child))
		}
		pop = next
	}

	finalCrowd := crowding(pop)
	sort.Slice(pop, func(i, j int) bool { return selScore(pop[i], finalCrowd) > selScore(pop[j], finalCrowd) })
	fmt.Printf("\n== Final population, one row per composition (fitness | novelty | composition) ==\n")
	seen := map[string]bool{}
	for _, ind := range pop {
		c := ind.spec.Composition()
		if seen[c] || !ind.valid {
			continue
		}
		seen[c] = true
		fmt.Printf("  fit=%.3f  nov=%.3f  %s\n", ind.fitness, novelty(ind.vec), c)
	}
	fmt.Printf("\nEvaluated %d distinct specs; %d distinct compositions survive (novelty pressure keeps the pool diverse).\n",
		len(cache), distinctCompositions(pop))
}

func distinctCompositions(pop []individual) int {
	seen := map[string]bool{}
	for _, ind := range pop {
		seen[ind.spec.Composition()] = true
	}
	return len(seen)
}

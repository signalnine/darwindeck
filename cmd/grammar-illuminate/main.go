// Command grammar-illuminate runs MAP-Elites over the typed grammar space
// (lever 2). Where grammar-evolve OPTIMIZES (and converges, losing diversity),
// MAP-Elites ILLUMINATES: it keeps the best game in each cell of a behavior grid
// and spends its budget filling empty cells, so the output is a DIVERSE set that
// covers the space instead of a single best corner. The archive is exactly the
// composition-diverse set you then judge out-of-loop. Every spec is well-typed by
// construction (Mutate/RandomSpec never leave the manifold), so the whole archive
// is playable-by-construction.
package main

import (
	"fmt"
	"math/rand/v2"
	"sort"

	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/grammar"
)

const (
	iterations = 2500
	initN      = 80
)

type elite struct {
	spec    grammar.GameSpec
	fitness float64
}

// cell is the behavior descriptor: the structural family crossed with coarse
// buckets of two behavior axes (meaningful-decisions and interaction). Two specs
// in the same family but different behavior regions occupy different cells, so
// MAP-Elites keeps both -- that is the diversity OPTIMIZE throws away.
func cell(s grammar.GameSpec, m fitness.Metrics) string {
	b := func(x float64) int {
		v := int(x * 4)
		if v > 3 {
			v = 3
		}
		if v < 0 {
			v = 0
		}
		return v
	}
	return fmt.Sprintf("%s|d%d|i%d", s.Family(), b(m.MeaningfulDecisions), b(m.Interaction))
}

func main() {
	rng := rand.New(rand.NewPCG(2026, 0x9e3779b97f4a7c15))
	cache := map[string]struct {
		fit   float64
		cell  string
		valid bool
	}{}

	eval := func(s grammar.GameSpec) (float64, string, bool) {
		key := s.String()
		if got, ok := cache[key]; ok {
			return got.fit, got.cell, got.valid
		}
		g := grammar.SpecGenome(s)
		r := fitness.EvaluateWithRunner(g, grammar.Adapter{Spec: s}, fitness.GetGreedyAI(g), 1)
		fit, valid := 0.0, r.Valid && r.Tier1.Passed
		c := ""
		if valid {
			fit = r.Metrics.TotalFitness
			c = cell(s, r.Metrics)
		}
		cache[key] = struct {
			fit   float64
			cell  string
			valid bool
		}{fit, c, valid}
		return fit, c, valid
	}

	archive := map[string]elite{} // cell -> best elite

	place := func(s grammar.GameSpec) {
		fit, c, valid := eval(s)
		if !valid {
			return
		}
		if cur, ok := archive[c]; !ok || fit > cur.fitness {
			archive[c] = elite{spec: s, fitness: fit}
		}
	}

	for i := 0; i < initN; i++ {
		place(grammar.RandomSpec(rng))
	}

	// MAP-Elites loop: draw a random elite, mutate, try to place. Empty cells get
	// filled; occupied cells only improve. No selection pressure toward one peak.
	// Sort the pool by cell key: ranging over the archive MAP yields a random
	// order per call, so the seeded rng indexed a shuffled slice and runs were
	// not reproducible despite the fixed PCG seed.
	cells := func() []elite {
		keys := make([]string, 0, len(archive))
		for k := range archive {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := make([]elite, 0, len(keys))
		for _, k := range keys {
			out = append(out, archive[k])
		}
		return out
	}
	for it := 0; it < iterations; it++ {
		pool := cells()
		parent := pool[rng.IntN(len(pool))].spec
		place(grammar.Mutate(parent, rng))
		if (it+1)%500 == 0 {
			fmt.Printf("iter %4d | cells filled=%3d | distinct families=%2d | specs evaluated=%d\n",
				it+1, len(archive), distinctFamilies(archive), len(cache))
		}
	}

	totalFamilies := len(grammar.Families(grammar.EnumerateModified()))
	fmt.Printf("\n== MAP-Elites illumination of the typed grammar ==\n")
	fmt.Printf("  behavior cells filled        : %d  (family x decisions-bucket x interaction-bucket)\n", len(archive))
	fmt.Printf("  distinct families illuminated : %d / %d well-typed families\n", distinctFamilies(archive), totalFamilies)
	fmt.Printf("  specs evaluated               : %d\n", len(cache))

	// The archive IS the diverse judging set: best game per family, sorted.
	// Iterate cells in sorted order so equal-fitness ties break on the cell
	// key, not on map iteration order -- the printed judging set must be
	// identical across runs of the same seed.
	byFam := map[string]elite{}
	for _, e := range cells() {
		f := e.spec.Family()
		if cur, ok := byFam[f]; !ok || e.fitness > cur.fitness {
			byFam[f] = e
		}
	}
	fams := make([]string, 0, len(byFam))
	for f := range byFam {
		fams = append(fams, f)
	}
	sort.Strings(fams)
	fmt.Printf("\n== Best game per illuminated family (the out-of-loop judging set) ==\n")
	for _, f := range fams {
		fmt.Printf("  %.3f  %s\n", byFam[f].fitness, f)
	}
}

func distinctFamilies(archive map[string]elite) int {
	fams := map[string]bool{}
	for _, e := range archive {
		fams[e.spec.Family()] = true
	}
	return len(fams)
}

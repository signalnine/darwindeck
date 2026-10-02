package evolution

import (
	"fmt"
	"math/rand/v2"
	"sort"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
)

// TestOperatorsValidInValidOut is the operators' closure property: starting
// from the classic seeds and feeding every valid child back into the pool,
// 40,000 random operator applications -- mutation, raw same-skeleton
// crossover, raw hybrid crossover, and crossover followed by mutation --
// never produce a genome that
//
//   - fails Tier-0 validation (genome.Validate), or
//   - carries a DEAD borrow (genome.LiveBorrows drops it): the teeth wiring
//     must keep every grafted borrow outcome-significant.
//
// Run both with and without cross-skeleton recombination. The pool EVOLVES
// (children replace random members), so the walk reaches multi-borrow,
// multi-step states that single-step tests from a seed never visit. Each
// per-operator repair (deck budget, vying stack sufficiency, trump suit,
// card-points coupling, borrow teeth) is pinned by its own unit test; this is
// the net that catches the interaction nobody wrote a unit test for.
func TestOperatorsValidInValidOut(t *testing.T) {
	const ops = 40000
	for _, cross := range []bool{false, true} {
		t.Run(fmt.Sprintf("cross_skeleton=%v", cross), func(t *testing.T) {
			all := seeds.All()
			rng := rand.New(rand.NewPCG(7, 11))
			pool := make([]*genome.Genome, 300)
			for i := range pool {
				pool[i] = all[i%len(all)].Clone()
			}

			applied := map[string]int{}
			failures := map[string]int{} // "op | skeleton | problem" -> count
			skeletons := map[genome.SkeletonType]bool{}
			borrowed := 0

			for step := 0; step < ops; step++ {
				a, b := pool[rng.IntN(len(pool))], pool[rng.IntN(len(pool))]
				var child *genome.Genome
				var op string
				switch rng.IntN(4) {
				case 0, 1:
					op, child = "mutate", MutateWith(a, rng, all, cross)
				case 2:
					op, child = "crossover", CrossoverWith(a, b, rng, cross)
					if child != nil && a.Skeleton != b.Skeleton {
						op = "hybrid_crossover"
					}
				case 3:
					op, child = "crossover+mutate", CrossoverWith(a, b, rng, cross)
					if child != nil {
						child = MutateWith(child, rng, all, cross)
					}
				}
				if child == nil {
					continue // cross-skeleton pair with the flag off
				}
				applied[op]++

				bad := false
				for _, e := range genome.Validate(child) {
					if len(e) > 90 {
						e = e[:90]
					}
					failures[fmt.Sprintf("%s | %s | invalid: %s", op, child.Skeleton, e)]++
					bad = true
				}
				if live := child.LiveBorrows(); len(live) != len(child.Borrowed) {
					failures[fmt.Sprintf("%s | %s | dead borrow in %v (%s)", op, child.Skeleton, child.Borrowed, child.ActiveParams())]++
					bad = true
				}
				if bad {
					continue
				}
				skeletons[child.Skeleton] = true
				if len(child.Borrowed) > 0 {
					borrowed++
				}
				pool[rng.IntN(len(pool))] = child
			}

			if len(failures) > 0 {
				keys := make([]string, 0, len(failures))
				for k := range failures {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				for _, k := range keys {
					t.Errorf("%d x %s", failures[k], k)
				}
			}

			// Guard against a vacuous pass: the walk must really have covered
			// every operator, every skeleton, and borrowed genomes.
			wantOps := []string{"mutate", "crossover", "crossover+mutate"}
			if cross {
				wantOps = append(wantOps, "hybrid_crossover")
			}
			for _, op := range wantOps {
				if applied[op] < 1000 {
					t.Errorf("operator %q applied only %d times; the property is under-exercised", op, applied[op])
				}
			}
			for _, skel := range genome.AllSkeletons() {
				if !skeletons[skel] {
					t.Errorf("no valid %s child was produced; the property never exercised that skeleton", skel)
				}
			}
			if borrowed < 1000 {
				t.Errorf("only %d children carried a borrow; the dead-borrow check is under-exercised", borrowed)
			}
		})
	}
}

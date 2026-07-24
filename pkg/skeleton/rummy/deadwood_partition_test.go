package rummy

import (
	"math"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

// referenceWholeHandDP is the pre-decomposition implementation: one subset DP
// over the WHOLE hand. It is the exactness oracle for bestPartitionValue --
// component decomposition and cover compaction must be value-preserving
// identities, not approximations.
func referenceWholeHandDP(candidates []meldCandidate, handSize int) int {
	if handSize > 20 {
		panic("reference oracle is only affordable up to 20 cards")
	}
	full := uint32(1)<<uint(handSize) - 1
	value := make([]int, int(full)+1)
	for mask := uint32(1); mask <= full; mask++ {
		lowBit := mask & (^mask + 1)
		best := value[mask^lowBit]
		for _, c := range candidates {
			if c.mask&mask != c.mask {
				continue
			}
			if v := value[mask^c.mask] + c.value; v > best {
				best = v
			}
		}
		value[mask] = best
	}
	return value[full]
}

// TestBestPartitionValueMatchesWholeHandDP pins the exactness of the two
// reductions in bestPartitionValue (drop non-meldable cards from the lattice;
// solve connected components independently) against the whole-hand subset DP
// they replaced, across every meld type and the full reachable hand range.
func TestBestPartitionValueMatchesWholeHandDP(t *testing.T) {
	rng := rand.New(rand.NewPCG(20260724, 7))
	for _, meld := range []genome.MeldType{genome.MeldSets, genome.MeldRuns, genome.MeldBoth} {
		for _, minSize := range []int{3, 4} {
			params := &genome.RummyParams{MeldTypes: meld, MinMeldSize: minSize}
			for _, n := range []int{3, 5, 8, 11, 13, 16, 18, 20} {
				for trial := 0; trial < 60; trial++ {
					deck := sim.StandardDeck()
					sim.ShuffleDeck(deck, rng)
					hand := deck[:n]
					candidates := enumerateMeldCandidates(hand, params)
					if len(candidates) == 0 {
						continue
					}
					want := referenceWholeHandDP(candidates, n)
					if got := bestPartitionValue(candidates); got != want {
						t.Fatalf("meld=%d min=%d n=%d trial=%d: bestPartitionValue=%d want %d (hand %v)",
							meld, minSize, n, trial, got, want, hand)
					}
				}
			}
		}
	}
}

// TestBestPartitionValueBnBMatchesDP pins the branch-and-bound fallback (used
// for components wider than partitionDPWidth) against the exact DP on inputs
// small enough for both. The suffix-value prune must only skip branches that
// cannot beat the incumbent, so the maximum is unchanged.
func TestBestPartitionValueBnBMatchesDP(t *testing.T) {
	rng := rand.New(rand.NewPCG(99, 3))
	params := &genome.RummyParams{MeldTypes: genome.MeldBoth, MinMeldSize: 3}
	for trial := 0; trial < 300; trial++ {
		deck := sim.StandardDeck()
		sim.ShuffleDeck(deck, rng)
		hand := deck[:12]
		candidates := enumerateMeldCandidates(hand, params)
		if len(candidates) == 0 {
			continue
		}
		for _, comp := range meldComponents(candidates) {
			want := bestPartitionValueDP(comp.cands, comp.width)
			if got := bestPartitionValueBnB(comp.cands); got != want {
				t.Fatalf("trial %d: BnB=%d want DP=%d (width %d, %d candidates)",
					trial, got, want, comp.width, len(comp.cands))
			}
		}
	}
}

// TestCalcDeadwoodHasNoWidthCliff pins the defect this decomposition fixed: the
// whole-hand subset DP made a 20-card hand cost ~2.8ms per call -- about 1900x
// what the >20-card fallback cost on the same data -- so the "fast" branch was
// inverted against its own fallback across the whole 16-20 band. Progress calls
// calcDeadwood for every player after every applied move, and MechDrawPenalty on
// a rummy host grows hands right into that band.
//
// The assertion is SCALE-FREE on purpose (cost shape, not absolute nanoseconds):
// no hand size may cost materially more than the LARGEST hand size measured.
// That is exactly what a branch inverted against its fallback looks like, and it
// holds on any machine and under -race, where absolute budgets do not.
func TestCalcDeadwoodHasNoWidthCliff(t *testing.T) {
	params := &genome.RummyParams{MeldTypes: genome.MeldBoth, MinMeldSize: 3}
	rng := rand.New(rand.NewPCG(5, 5))
	deck := sim.StandardDeck()
	sim.ShuffleDeck(deck, rng)

	sizes := []int{13, 16, 18, 19, 20, 21, 24, 31}
	cost := make(map[int]time.Duration, len(sizes))
	for _, n := range sizes {
		hand := append([]sim.Card(nil), deck[:n]...)
		// Best-of-5 batches: the minimum is the least noisy estimator of a
		// deterministic routine's cost under scheduler and GC interference.
		best := time.Duration(math.MaxInt64)
		for round := 0; round < 5; round++ {
			const reps = 50
			start := time.Now()
			for i := 0; i < reps; i++ {
				calcDeadwood(hand, params)
			}
			if per := time.Since(start) / reps; per < best {
				best = per
			}
		}
		cost[n] = best
		t.Logf("hand=%2d: calcDeadwood %v/call", n, best)
	}

	widest := sizes[len(sizes)-1]
	ceiling := cost[widest] * 3 / 2 // 1.5x tolerance for measurement noise
	for _, n := range sizes {
		if n == widest {
			continue
		}
		if cost[n] > ceiling {
			t.Errorf("hand=%d costs %v/call, MORE than the widest hand (%d: %v) -- "+
				"the deadwood partition is spiking on an interior width again",
				n, cost[n], widest, cost[widest])
		}
	}
}

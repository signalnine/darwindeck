package evolution

import (
	"math/rand/v2"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
)

// TestTweakParameterReachesRoundsWithTrickScoringBorrow: tweakParameter gated
// the shedding RoundsPerGame mutation on HasScoringBorrow (meld_bonus /
// avoidance only), but the runner's multi-round predicate is
// SheddingMultiRound, which keys on HasBankingBorrow -- the wider set that
// also holds trick_scoring (the headline shed-to-win-by-tricks hybrid). On a
// trick_scoring-only genome RoundsPerGame is therefore LIVE yet was frozen at
// whatever the graft installed: 17,986 mutants, all still at 3.
func TestTweakParameterReachesRoundsWithTrickScoringBorrow(t *testing.T) {
	base := wiredHybrid(seeds.CrazyEights(), genome.TrickTaking, genome.MechTrickScoring)
	if !base.SheddingMultiRound() {
		t.Fatal("fixture: a trick_scoring shedding hybrid must be multi-round (RoundsPerGame is live)")
	}

	seen := map[int]bool{}
	for seed := uint64(0); seed < 500; seed++ {
		g := base.Clone()
		g.Shedding.RoundsPerGame = 3
		tweakParameter(g, rand.New(rand.NewPCG(seed, 0)))
		r := g.Shedding.RoundsPerGame
		if r < 2 || r > 5 {
			t.Fatalf("seed %d: RoundsPerGame %d, want 2-5 (banking borrow present)", seed, r)
		}
		seen[r] = true
	}
	for _, want := range []int{2, 3, 4} {
		if !seen[want] {
			t.Errorf("RoundsPerGame %d never produced from 3 across 500 tweaks of a trick_scoring-only genome (saw %v)", want, seen)
		}
	}
}

// TestAddSpecialCardReachesEveryRank: Tier 0 accepts special cards on any rank
// 2-14, but addSpecialCard sampled from a hand-picked list {2,7,8,10,J,Q}, so
// specials on 3-6, 9, K and A existed only if a seed carried them.
func TestAddSpecialCardReachesEveryRank(t *testing.T) {
	rng := rand.New(rand.NewPCG(11, 0))
	seen := map[uint8]bool{}
	for i := 0; i < 4000; i++ {
		g := seeds.CrazyEights()
		g.SpecialCards = nil
		addSpecialCard(g, rng)
		if len(g.SpecialCards) != 1 {
			t.Fatalf("trial %d: addSpecialCard added %d cards, want 1", i, len(g.SpecialCards))
		}
		if errs := genome.Validate(g); len(errs) > 0 {
			t.Fatalf("trial %d: addSpecialCard produced an invalid genome: %v", i, errs)
		}
		seen[g.SpecialCards[0].ByRank] = true
	}
	for rank := uint8(2); rank <= 14; rank++ {
		if !seen[rank] {
			t.Errorf("addSpecialCard never produced by_rank %d in 4000 trials (valid range is 2-14)", rank)
		}
	}
	for rank := range seen {
		if rank != 0 && (rank < 2 || rank > 14) {
			t.Errorf("addSpecialCard produced out-of-range by_rank %d", rank)
		}
	}
}

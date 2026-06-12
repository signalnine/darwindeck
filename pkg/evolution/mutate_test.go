package evolution

import (
	"math/rand/v2"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
)

func TestAddSpecialCardSkipsNonShedding(t *testing.T) {
	// Special cards are a shedding-only feature; the trick-taking and rummy
	// runners never read them, so mutation must not add them to those
	// skeletons (dd-24e).
	for seed := uint64(0); seed < 200; seed++ {
		rng := rand.New(rand.NewPCG(seed, 0))
		g := &genome.Genome{Skeleton: genome.TrickTaking}
		addSpecialCard(g, rng)
		if len(g.SpecialCards) != 0 {
			t.Fatalf("seed %d: addSpecialCard added a special card to a trick-taking genome", seed)
		}
	}
	// Sanity: shedding genomes still receive special cards.
	rng := rand.New(rand.NewPCG(1, 0))
	g := &genome.Genome{Skeleton: genome.Shedding}
	addSpecialCard(g, rng)
	if len(g.SpecialCards) != 1 {
		t.Fatalf("expected shedding genome to receive a special card, got %d", len(g.SpecialCards))
	}
}

func TestAddSpecialCardCanSetBySuit(t *testing.T) {
	// addSpecialCard must exercise the BySuit dimension of the SpecialCard
	// schema, otherwise suit-bound specials (e.g. Hearts wilds) are
	// unreachable from evolution. See dd-umc.
	sawNonZeroSuit := false
	sawZeroSuit := false
	for seed := uint64(0); seed < 500; seed++ {
		rng := rand.New(rand.NewPCG(seed, 0))
		g := &genome.Genome{}
		addSpecialCard(g, rng)
		if len(g.SpecialCards) != 1 {
			t.Fatalf("seed %d: expected 1 special card, got %d", seed, len(g.SpecialCards))
		}
		sc := g.SpecialCards[0]
		if sc.BySuit > 4 {
			t.Errorf("seed %d: BySuit=%d out of range (want 0-4)", seed, sc.BySuit)
		}
		if sc.BySuit == 0 {
			sawZeroSuit = true
		} else {
			sawNonZeroSuit = true
		}
	}
	if !sawNonZeroSuit {
		t.Errorf("addSpecialCard never produced BySuit != 0 in 500 trials; suit-bound specials unreachable")
	}
	if !sawZeroSuit {
		t.Errorf("addSpecialCard never produced BySuit == 0 in 500 trials; suit-agnostic specials unreachable")
	}
}

func TestMutateScoringNeverGeneratesInvalidRank(t *testing.T) {
	// Valid card ranks are 2-14 (per pkg/sim/card.go); Rank=0 means
	// "all ranks" wildcard. Rank=1 is invalid: it never matches any
	// card in cardPointValue, so any scoring rule with Rank=1 is dead.
	for seed := uint64(0); seed < 200; seed++ {
		rng := rand.New(rand.NewPCG(seed, 0))
		g := &genome.Genome{
			Scoring: genome.ScoringConfig{CardPoints: nil},
		}
		mutateScoring(g, rng)
		if len(g.Scoring.CardPoints) == 0 {
			t.Fatalf("seed %d: expected mutateScoring to add a card point rule", seed)
		}
		got := g.Scoring.CardPoints[0].Rank
		if got == 1 {
			t.Errorf("seed %d: mutateScoring produced invalid Rank=1", seed)
		}
		if got != 0 && (got < 2 || got > 14) {
			t.Errorf("seed %d: mutateScoring produced out-of-range Rank=%d (want 0 or 2-14)", seed, got)
		}
	}
}

func TestMutateScoringCanProduceRankZero(t *testing.T) {
	// mutateScoring must reach the Rank=0 ("all ranks") wildcard, otherwise
	// catch-all rules like "every red card is worth 2 points" are
	// unreachable from evolution -- only the 13 per-rank variants can be
	// produced. See dd-eir.
	sawRankZero := false
	sawSpecificRank := false
	for seed := uint64(0); seed < 500; seed++ {
		rng := rand.New(rand.NewPCG(seed, 0))
		g := &genome.Genome{
			Scoring: genome.ScoringConfig{CardPoints: nil},
		}
		mutateScoring(g, rng)
		if len(g.Scoring.CardPoints) == 0 {
			t.Fatalf("seed %d: expected mutateScoring to add a card point rule", seed)
		}
		r := g.Scoring.CardPoints[0].Rank
		if r == 0 {
			sawRankZero = true
		} else {
			sawSpecificRank = true
		}
	}
	if !sawRankZero {
		t.Errorf("mutateScoring never produced Rank=0 in 500 trials; catch-all rules unreachable")
	}
	if !sawSpecificRank {
		t.Errorf("mutateScoring never produced a specific rank in 500 trials; rank-specific rules unreachable")
	}
}

// TestAddSpecialCardCoversAllTypes verifies addSpecialCard samples every
// SpecialCardType. Missing SpecialDrawFour from the type slice silently made
// DrawFour unreachable through mutation despite the runner implementing it
// (dd-9w1). Mirrors the dd-eir / dd-umc fixes for catch-all coverage.
func TestAddSpecialCardCoversAllTypes(t *testing.T) {
	allTypes := []genome.SpecialCardType{
		genome.SpecialSkip,
		genome.SpecialReverse,
		genome.SpecialDrawTwo,
		genome.SpecialDrawFour,
		genome.SpecialWild,
	}
	seen := make(map[genome.SpecialCardType]bool, len(allTypes))
	for seed := uint64(0); seed < 1000; seed++ {
		rng := rand.New(rand.NewPCG(seed, 0))
		g := &genome.Genome{}
		addSpecialCard(g, rng)
		if len(g.SpecialCards) != 1 {
			t.Fatalf("seed %d: expected 1 special card, got %d", seed, len(g.SpecialCards))
		}
		seen[g.SpecialCards[0].Type] = true
	}
	for _, ty := range allTypes {
		if !seen[ty] {
			t.Errorf("addSpecialCard never produced SpecialCardType=%v in 1000 trials; type is unreachable from mutation", ty)
		}
	}
}

// TestAddSpecialCardCanProduceRankZero verifies addSpecialCard samples
// ByRank=0 ("any rank") so catch-all specials like "every Heart is wild" are
// reachable via cumulative mutation, not only via seed copy. Mirrors dd-eir
// for the special-card mutation operator (dd-g2m).
func TestAddSpecialCardCanProduceRankZero(t *testing.T) {
	sawRankZero := false
	sawSpecificRank := false
	for seed := uint64(0); seed < 1000; seed++ {
		rng := rand.New(rand.NewPCG(seed, 0))
		g := &genome.Genome{}
		addSpecialCard(g, rng)
		if len(g.SpecialCards) != 1 {
			t.Fatalf("seed %d: expected 1 special card, got %d", seed, len(g.SpecialCards))
		}
		r := g.SpecialCards[0].ByRank
		if r == 0 {
			sawRankZero = true
		} else {
			sawSpecificRank = true
		}
	}
	if !sawRankZero {
		t.Errorf("addSpecialCard never produced ByRank=0 in 1000 trials; catch-all specials unreachable from mutation")
	}
	if !sawSpecificRank {
		t.Errorf("addSpecialCard never produced a specific rank in 1000 trials")
	}
}

// TestTweakParameterReachesSheddingRounds (Task 22): RoundsPerGame is an
// evolvable shedding parameter -- tweakParameter must be able to move it,
// always landing in 1-5 (never back to the legacy 0 encoding, which would
// make a mutated multi-round genome silently single-round). The field is
// only LIVE when the genome carries a scoring borrow
// (genome.SheddingMultiRound), so the fixtures carry MechMeldBonus; the
// borrow-less case is pinned separately below.
func TestTweakParameterReachesSheddingRounds(t *testing.T) {
	scoringBorrow := []genome.BorrowedMechanic{
		{Source: genome.Rummy, Mechanic: genome.MechMeldBonus},
	}

	seen := map[int]bool{}
	for seed := uint64(0); seed < 500; seed++ {
		rng := rand.New(rand.NewPCG(seed, 0))
		g := &genome.Genome{
			Skeleton: genome.Shedding,
			Players:  2,
			HandSize: 7,
			Borrowed: scoringBorrow,
			Shedding: &genome.SheddingParams{
				MatchRule:     genome.MatchEither,
				DrawPenalty:   2,
				RoundsPerGame: 3,
			},
		}
		tweakParameter(g, rng)
		r := g.Shedding.RoundsPerGame
		if r < 1 || r > 5 {
			t.Fatalf("seed %d: tweakParameter produced RoundsPerGame %d, want 1-5", seed, r)
		}
		seen[r] = true
	}
	for _, want := range []int{2, 3, 4} {
		if !seen[want] {
			t.Errorf("RoundsPerGame %d never produced from 3 across 500 tweaks (param unreachable by mutation)", want)
		}
	}

	// A legacy genome carrying the 0 encoding must normalize into 1-5 when
	// the tweak touches the field.
	reachedOne := false
	for seed := uint64(0); seed < 200; seed++ {
		rng := rand.New(rand.NewPCG(seed, 0))
		g := &genome.Genome{
			Skeleton: genome.Shedding,
			Players:  2,
			HandSize: 7,
			Borrowed: scoringBorrow,
			Shedding: &genome.SheddingParams{MatchRule: genome.MatchEither, DrawPenalty: 1},
		}
		tweakParameter(g, rng)
		r := g.Shedding.RoundsPerGame
		if r < 0 || r > 5 {
			t.Fatalf("seed %d: RoundsPerGame %d out of range", seed, r)
		}
		if r >= 1 {
			reachedOne = true
		}
	}
	if !reachedOne {
		t.Error("tweakParameter never normalized a legacy RoundsPerGame=0 genome into 1-5")
	}
}

// TestTweakParameterSkipsRoundsWithoutScoringBorrow (reviewer finding,
// coherent-mutation principle): without a scoring borrow RoundsPerGame is
// inert (genome.SheddingMultiRound is false regardless of its value), so
// tweakParameter must not burn mutation pressure on it -- borrow-less
// shedding genomes spend the whole skeleton-param branch on DrawPenalty.
func TestTweakParameterSkipsRoundsWithoutScoringBorrow(t *testing.T) {
	for seed := uint64(0); seed < 500; seed++ {
		rng := rand.New(rand.NewPCG(seed, 0))
		g := &genome.Genome{
			Skeleton: genome.Shedding,
			Players:  2,
			HandSize: 7,
			Shedding: &genome.SheddingParams{
				MatchRule:     genome.MatchEither,
				DrawPenalty:   2,
				RoundsPerGame: 3,
			},
		}
		tweakParameter(g, rng)
		if g.Shedding.RoundsPerGame != 3 {
			t.Fatalf("seed %d: tweakParameter moved RoundsPerGame to %d on a borrow-less genome (inert field)",
				seed, g.Shedding.RoundsPerGame)
		}
	}
}

// TestAddSpecialCardNeverCatchAll (Task 28 round 3): a special card with
// ByRank == 0 AND BySuit == 0 matches every card and is Tier-0 rejected as a
// liveness violation (it deletes match_rule/draw_penalty as dead genes -- the
// round-2 flagship's shedding top 10). Mutation must never generate the
// encoding: when the rank qualifier is dropped, a suit qualifier must be
// forced, so catch-alls like "every Heart is wild" stay reachable while
// "every card is wild" is not.
func TestAddSpecialCardNeverCatchAll(t *testing.T) {
	for seed := uint64(0); seed < 2000; seed++ {
		rng := rand.New(rand.NewPCG(seed, 0))
		g := &genome.Genome{Skeleton: genome.Shedding}
		addSpecialCard(g, rng)
		if len(g.SpecialCards) != 1 {
			t.Fatalf("seed %d: expected 1 special card, got %d", seed, len(g.SpecialCards))
		}
		sc := g.SpecialCards[0]
		if sc.ByRank == 0 && sc.BySuit == 0 {
			t.Fatalf("seed %d: addSpecialCard produced a catch-all special %+v (Tier-0 rejected encoding)", seed, sc)
		}
	}
}

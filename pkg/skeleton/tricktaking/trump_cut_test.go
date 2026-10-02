package tricktaking

import (
	"math/rand/v2"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
)

// TestTrumpCutIsIndependentOfTheDeal: "cut a card to determine trump" must not
// hand a fixed seat a guaranteed trump. The cut used to read the first card of
// the shuffled deck -- which is then dealt as seat 0's first card -- so seat 0
// (who also leads the first trick) ALWAYS held a trump; reading any other fixed
// deck position just moves the gift to another seat when all 52 cards are
// dealt. The cut is now an independent draw: no dealt position matches the
// trump suit more often than chance.
func TestTrumpCutIsIndependentOfTheDeal(t *testing.T) {
	g := &genome.Genome{
		Skeleton: genome.TrickTaking, Players: 4, HandSize: 13,
		TrickTaking: &genome.TrickTakingParams{MustFollowSuit: true, TrickScoring: genome.ScorePerTrick, RoundsPerGame: 1},
		TrumpRule:   genome.TrumpCut,
	}
	const n = 800
	r := &Runner{}
	var match [4][13]int
	var suitCount [4]int
	for seed := uint64(0); seed < n; seed++ {
		st := r.Setup(g, rand.New(rand.NewPCG(seed, 99)))
		if st.TrumpSuit < 0 || st.TrumpSuit > 3 {
			t.Fatalf("seed %d: cut produced no trump suit (%d)", seed, st.TrumpSuit)
		}
		suitCount[st.TrumpSuit]++
		for p := 0; p < 4; p++ {
			for k := 0; k < 13; k++ {
				if int(st.Hands[p][k].Suit) == st.TrumpSuit {
					match[p][k]++
				}
			}
		}
	}
	for p := 0; p < 4; p++ {
		for k := 0; k < 13; k++ {
			if frac := float64(match[p][k]) / n; frac > 0.40 {
				t.Errorf("seat %d card %d is trump in %.0f%% of deals (chance is 25%%): the cut is tied to that dealt card", p, k, 100*frac)
			}
		}
	}
	for s, c := range suitCount {
		if frac := float64(c) / n; frac < 0.15 || frac > 0.35 {
			t.Errorf("suit %d is cut as trump in %.0f%% of deals, want ~25%%", s, 100*frac)
		}
	}
}

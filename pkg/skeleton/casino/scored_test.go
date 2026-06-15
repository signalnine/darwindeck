package casino

import (
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

// casinoScoredGenome is the casino fixture plus a MechMeldBonus borrow, so
// CasinoScored() is true: a Scopa-style scored fishing game.
func casinoScoredGenome() *genome.Genome {
	g := casinoGenome()
	g.ID = "casino-scored"
	g.Borrowed = []genome.BorrowedMechanic{{Source: genome.Rummy, Mechanic: genome.MechMeldBonus}}
	return g
}

func countRoundEnds(evs []sim.Event) int {
	n := 0
	for _, e := range evs {
		if e.Type == sim.EventRoundEnd {
			n++
		}
	}
	return n
}

// TestCasinoScoredEmitsExactlyOneRoundEnd: a scored casino game emits the
// banking event EXACTLY once (on the terminal move) so the accumulating meld /
// avoidance hook banks the captured pile once and never double-counts. The
// UNSCORED casino must emit ZERO -- the scored path is fully gated on
// CasinoScored, so the calibration seed is byte-identical.
func TestCasinoScoredEmitsExactlyOneRoundEnd(t *testing.T) {
	scored := casinoScoredGenome()
	plain := casinoGenome()
	for seed := uint64(0); seed < 100; seed++ {
		rs := runGame(scored, seed)
		if rs.Winner < 0 {
			t.Fatalf("scored seed %d did not complete: %s", seed, rs.Error)
		}
		if got := countRoundEnds(rs.Events); got != 1 {
			t.Fatalf("scored seed %d: expected exactly 1 EventRoundEnd, got %d", seed, got)
		}
		rp := runGame(plain, seed)
		if got := countRoundEnds(rp.Events); got != 0 {
			t.Fatalf("unscored seed %d: expected 0 EventRoundEnd (byte-identity), got %d", seed, got)
		}
	}
}

// TestCasinoScoredCheckEndCountPlusScoresFlips: with a scoring borrow, CheckEnd
// ranks by captured COUNT + banked Scores, so a meld bonus can flip the winner
// away from the raw-count leader. The same Tableau under an UNSCORED genome
// picks the count leader -- proving the borrow is outcome-significant (dd-lnh),
// not vestigial.
func TestCasinoScoredCheckEndCountPlusScoresFlips(t *testing.T) {
	runner := &Runner{}
	state := sim.NewGameState(2)
	state.Hands[0] = nil
	state.Hands[1] = nil
	state.Deck = nil // !canRedeal
	// Player 0 captured more cards; player 1 banked a meld bonus that overtakes.
	state.Tableau[0] = make([]sim.Card, 10)
	state.Tableau[1] = make([]sim.Card, 8)
	state.Scores[0] = 0
	state.Scores[1] = 5 // 8 + 5 = 13 > 10 + 0

	if w := runner.CheckEnd(state, casinoScoredGenome()); w != 1 {
		t.Fatalf("scored CheckEnd: meld bonus should flip winner to player 1 (13 vs 10), got %d", w)
	}
	if w := runner.CheckEnd(state, casinoGenome()); w != 0 {
		t.Fatalf("unscored CheckEnd: raw-count leader is player 0 (10 vs 8), got %d", w)
	}
}

// TestCasinoZeroCaptureScoredNoCrash: a terminal state where nobody ever
// captured (TrickLeader == -1, all piles empty) must resolve cleanly: the sweep
// guard skips, the hook tallies zeros, CheckEnd returns the lowest seat. No
// panic, deterministic.
func TestCasinoZeroCaptureScoredNoCrash(t *testing.T) {
	runner := &Runner{}
	state := sim.NewGameState(3)
	for i := range state.Hands {
		state.Hands[i] = nil
	}
	state.Deck = nil
	state.TrickLeader = -1
	state.Discard = []sim.Card{{Suit: 1, Rank: 5}} // leftover, but no capturer to sweep to
	if w := runner.CheckEnd(state, casinoScoredGenome()); w != 0 {
		t.Fatalf("zero-capture scored CheckEnd: want lowest seat 0, got %d", w)
	}
}

// TestCasinoScoredDeterminism: same seed, same outcome -- the early sweep and
// EventRoundEnd emission must not introduce any nondeterminism.
func TestCasinoScoredDeterminism(t *testing.T) {
	g := casinoScoredGenome()
	for seed := uint64(0); seed < 40; seed++ {
		r1, r2 := runGame(g, seed), runGame(g, seed)
		if r1.Winner != r2.Winner || r1.Turns != r2.Turns {
			t.Fatalf("seed %d non-deterministic: (%d,%d) vs (%d,%d)", seed, r1.Winner, r1.Turns, r2.Winner, r2.Turns)
		}
	}
}

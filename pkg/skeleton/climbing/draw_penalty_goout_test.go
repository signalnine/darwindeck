package climbing

import (
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/mechanic"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

// TestGoingOutOnFaceCardWinsUnderDrawPenalty: with the draw-penalty borrow, a
// player whose LAST card was a face card used to be dealt a penalty card by the
// post-move hook, so CheckEnd saw a non-empty hand and the game went on --
// contradicting "the first player to empty their hand wins". Going out is
// exempt from the penalty; an ordinary face-card play still draws.
func TestGoingOutOnFaceCardWinsUnderDrawPenalty(t *testing.T) {
	g := &genome.Genome{
		Skeleton: genome.Climbing, Players: 2, HandSize: 5,
		Climbing: &genome.ClimbingParams{},
		Borrowed: []genome.BorrowedMechanic{{Source: genome.Rummy, Mechanic: genome.MechDrawPenalty}},
	}
	if errs := genome.Validate(g); len(errs) != 0 {
		t.Fatalf("fixture invalid: %v", errs)
	}
	hooks := mechanic.HooksFor(g)
	runner := &Runner{}
	play := func(st *sim.GameState, c sim.Card) {
		for _, e := range runner.ApplyMove(st, sim.Move{Type: sim.MovePlay, Cards: []sim.Card{c}}, g) {
			for _, h := range hooks {
				h(st, g, e)
			}
		}
	}
	king := sim.Card{Suit: sim.Spades, Rank: sim.King}

	st := sim.NewGameState(2)
	st.Direction = 1
	st.Hands[0] = []sim.Card{king}
	st.Hands[1] = []sim.Card{{Suit: sim.Spades, Rank: 3}, {Suit: sim.Spades, Rank: 4}}
	st.Deck = []sim.Card{{Suit: sim.Hearts, Rank: 9}}
	play(st, king)
	runner.Upkeep(st, g)
	if len(st.Hands[0]) != 0 {
		t.Errorf("going out on a king drew a penalty card: hand %v", st.Hands[0])
	}
	if w := runner.CheckEnd(st, g); w != 0 {
		t.Fatalf("P0 played their last card: CheckEnd = %d, want 0", w)
	}

	// Not going out: the penalty still bites.
	st = sim.NewGameState(2)
	st.Direction = 1
	st.Hands[0] = []sim.Card{king, {Suit: sim.Clubs, Rank: 2}}
	st.Hands[1] = []sim.Card{{Suit: sim.Spades, Rank: 3}, {Suit: sim.Spades, Rank: 4}}
	st.Deck = []sim.Card{{Suit: sim.Hearts, Rank: 9}}
	play(st, king)
	if len(st.Hands[0]) != 2 {
		t.Errorf("mid-game king must draw 1 penalty card: hand %v", st.Hands[0])
	}
}

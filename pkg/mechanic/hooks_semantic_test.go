package mechanic

import (
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

func fireRoundEnd(state *sim.GameState, g *genome.Genome) {
	for _, h := range HooksFor(g) {
		h(state, g, sim.Event{Type: sim.EventRoundEnd})
	}
}

func trickHostWithMeldBonus(scoring genome.TrickScoring) *genome.Genome {
	return &genome.Genome{
		Skeleton: genome.TrickTaking, Players: 4, HandSize: 13,
		TrickTaking: &genome.TrickTakingParams{MustFollowSuit: true, TrickScoring: scoring, RoundsPerGame: 1},
		Scoring:     genome.ScoringConfig{CardPoints: []genome.CardScoring{{Suit: 3, Points: 1}}},
		Borrowed:    []genome.BorrowedMechanic{{Source: genome.Rummy, Mechanic: genome.MechMeldBonus}},
	}
}

// TestMeldBonusFavorableOnLowestWinsHost: on a trick-taking host scored
// Hearts-style (trick_scoring=avoidance) the LOWEST score wins, so a meld
// "bonus" that ADDS points is a penalty -- capturing three 5s (worth no card
// points) used to push a player from 0 to 15 and toward last place while the
// rulebook called it a bonus. The hook must bank the meld in the winning
// direction: subtract where the lowest score wins, add everywhere else.
func TestMeldBonusFavorableOnLowestWinsHost(t *testing.T) {
	threeFives := []sim.Card{{Suit: sim.Clubs, Rank: 5}, {Suit: sim.Diamonds, Rank: 5}, {Suit: sim.Spades, Rank: 5}}

	low := trickHostWithMeldBonus(genome.ScoreAvoidance)
	if errs := genome.Validate(low); len(errs) != 0 {
		t.Fatalf("fixture invalid: %v", errs)
	}
	st := sim.NewGameState(4)
	st.Tableau[0] = threeFives
	fireRoundEnd(st, low)
	if st.Scores[0] != -15 {
		t.Errorf("lowest-wins host: a 3-card set must bank -15 (favorable), got %d", st.Scores[0])
	}
	for p := 1; p < 4; p++ {
		if st.Scores[p] != 0 {
			t.Errorf("lowest-wins host: player %d without melds scored %d, want 0", p, st.Scores[p])
		}
	}

	for _, scoring := range []genome.TrickScoring{genome.ScorePerTrick, genome.ScoreCardPoints} {
		high := trickHostWithMeldBonus(scoring)
		st := sim.NewGameState(4)
		st.Tableau[0] = threeFives
		fireRoundEnd(st, high)
		if st.Scores[0] != 15 {
			t.Errorf("highest-wins host (%s): a 3-card set must bank +15, got %d", scoring, st.Scores[0])
		}
	}
}

func drawPenaltyHost(skel genome.SkeletonType) *genome.Genome {
	g := &genome.Genome{
		Skeleton: skel, Players: 2, HandSize: 5,
		Borrowed: []genome.BorrowedMechanic{{Source: genome.Shedding, Mechanic: genome.MechDrawPenalty}},
	}
	switch skel {
	case genome.Climbing:
		g.Climbing = &genome.ClimbingParams{}
	case genome.Rummy:
		g.Rummy = &genome.RummyParams{MeldTypes: genome.MeldBoth, MinMeldSize: 3, KnockThreshold: 10}
	}
	return g
}

// TestDrawPenaltySkippedWhenGoingOut: the draw-penalty hook fired AFTER the
// play that emptied a hand, handing the player a fresh card -- so going out on
// a face card did not win a climbing game ("the first player to empty their
// hand wins") and a rummy gin discard of a face card left a deadwood card in
// the gin hand. Going out is exempt: no draw when the play empties the hand or
// has already ended the round.
func TestDrawPenaltySkippedWhenGoingOut(t *testing.T) {
	king := sim.Card{Suit: sim.Spades, Rank: sim.King}
	played := sim.Event{Type: sim.EventCardPlayed, PlayerID: 0, Cards: []sim.Card{king}}

	// Hand emptied by the play: no penalty.
	g := drawPenaltyHost(genome.Climbing)
	st := sim.NewGameState(2)
	st.Hands[1] = []sim.Card{{Suit: sim.Hearts, Rank: 3}}
	st.Deck = []sim.Card{{Suit: sim.Hearts, Rank: 9}}
	for _, h := range HooksFor(g) {
		h(st, g, played)
	}
	if len(st.Hands[0]) != 0 || len(st.Deck) != 1 {
		t.Errorf("going out on a face card drew a penalty card: hand %v, deck %d", st.Hands[0], len(st.Deck))
	}

	// Round already over (rummy knock/gin sets PhaseEnd before hooks run): no
	// penalty even though cards remain in hand.
	r := drawPenaltyHost(genome.Rummy)
	st = sim.NewGameState(2)
	st.Hands[0] = []sim.Card{{Suit: sim.Clubs, Rank: 4}}
	st.Deck = []sim.Card{{Suit: sim.Hearts, Rank: 9}}
	st.Phase = sim.PhaseEnd
	for _, h := range HooksFor(r) {
		h(st, r, played)
	}
	if len(st.Hands[0]) != 1 || len(st.Deck) != 1 {
		t.Errorf("round-ending face-card discard drew a penalty card: hand %v, deck %d", st.Hands[0], len(st.Deck))
	}

	// Ordinary mid-game face-card play: the penalty still applies.
	st = sim.NewGameState(2)
	st.Hands[0] = []sim.Card{{Suit: sim.Clubs, Rank: 4}}
	st.Deck = []sim.Card{{Suit: sim.Hearts, Rank: 9}}
	for _, h := range HooksFor(g) {
		h(st, g, played)
	}
	if len(st.Hands[0]) != 2 || len(st.Deck) != 0 {
		t.Errorf("mid-game face-card play must still draw 1: hand %v, deck %d", st.Hands[0], len(st.Deck))
	}
}

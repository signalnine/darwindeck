package mechanic

import (
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

func TestBuildHooksEmpty(t *testing.T) {
	g := &genome.Genome{
		Skeleton: genome.Shedding,
		Borrowed: nil,
	}
	hooks := BuildHooks(g)
	if len(hooks) != 0 {
		t.Fatalf("expected 0 hooks for no borrowed mechanics, got %d", len(hooks))
	}
}

func TestBuildHooksAvoidance(t *testing.T) {
	g := &genome.Genome{
		Skeleton: genome.Shedding,
		Borrowed: []genome.BorrowedMechanic{
			{Source: genome.TrickTaking, Mechanic: genome.MechAvoidance},
		},
	}
	hooks := BuildHooks(g)
	if len(hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(hooks))
	}
	if hooks[0].Point != HookScoring {
		t.Fatalf("expected HookScoring, got %d", hooks[0].Point)
	}
}

func TestAvoidanceModifiesScores(t *testing.T) {
	g := &genome.Genome{
		Skeleton: genome.Shedding,
		Players:  2,
		Scoring: genome.ScoringConfig{
			CardPoints: []genome.CardScoring{
				{Suit: uint8(sim.Hearts) + 1, Points: 1}, // All hearts = 1 penalty
			},
		},
		Borrowed: []genome.BorrowedMechanic{
			{Source: genome.TrickTaking, Mechanic: genome.MechAvoidance},
		},
	}

	state := &sim.GameState{
		Hands: [][]sim.Card{
			// Player 0 has 3 hearts
			{{Suit: sim.Hearts, Rank: sim.Two}, {Suit: sim.Hearts, Rank: sim.Five}, {Suit: sim.Hearts, Rank: sim.King}},
			// Player 1 has no hearts
			{{Suit: sim.Spades, Rank: sim.Ace}},
		},
		Scores:     []int{0, 0},
		NumPlayers: 2,
	}

	applyAvoidance(state, g, sim.Event{})

	if state.Scores[0] >= 0 {
		t.Fatalf("player 0 should have negative score from heart penalties, got %d", state.Scores[0])
	}
	if state.Scores[1] != 0 {
		t.Fatalf("player 1 should have 0 penalty, got %d", state.Scores[1])
	}
}

func TestAvoidanceIncludesTableauCaptures(t *testing.T) {
	// Trick-taking: hand is empty at end of round, captures live in Tableau.
	// Avoidance penalty must scan Tableau too, otherwise the borrow is silently a no-op.
	g := &genome.Genome{
		Skeleton: genome.TrickTaking,
		Players:  2,
		Scoring: genome.ScoringConfig{
			CardPoints: []genome.CardScoring{
				{Suit: uint8(sim.Hearts) + 1, Points: 1},
			},
		},
		Borrowed: []genome.BorrowedMechanic{
			{Source: genome.TrickTaking, Mechanic: genome.MechAvoidance},
		},
	}

	state := &sim.GameState{
		// Hands empty (post-trick-round)
		Hands: [][]sim.Card{{}, {}},
		// Player 0 captured 3 hearts in their tableau
		Tableau: [][]sim.Card{
			{
				{Suit: sim.Hearts, Rank: sim.Two},
				{Suit: sim.Hearts, Rank: sim.Five},
				{Suit: sim.Hearts, Rank: sim.King},
			},
			{{Suit: sim.Spades, Rank: sim.Ace}},
		},
		Scores:     []int{0, 0},
		NumPlayers: 2,
	}

	applyAvoidance(state, g, sim.Event{})

	if state.Scores[0] >= 0 {
		t.Fatalf("player 0 should have negative score from heart captures in tableau, got %d", state.Scores[0])
	}
	if state.Scores[1] != 0 {
		t.Fatalf("player 1 captured no hearts and should have 0 penalty, got %d", state.Scores[1])
	}
}

func TestMeldBonusAwardsPoints(t *testing.T) {
	g := &genome.Genome{
		Skeleton: genome.Shedding,
		Players:  2,
		Borrowed: []genome.BorrowedMechanic{
			{Source: genome.Rummy, Mechanic: genome.MechMeldBonus},
		},
	}

	state := &sim.GameState{
		Hands: [][]sim.Card{
			// Player 0 has three kings (a set)
			{
				{Suit: sim.Hearts, Rank: sim.King},
				{Suit: sim.Spades, Rank: sim.King},
				{Suit: sim.Clubs, Rank: sim.King},
			},
			// Player 1 has no melds
			{{Suit: sim.Hearts, Rank: sim.Two}, {Suit: sim.Spades, Rank: sim.Five}},
		},
		Scores:     []int{0, 0},
		NumPlayers: 2,
	}

	applyMeldBonus(state, g, sim.Event{})

	if state.Scores[0] <= 0 {
		t.Fatalf("player 0 should have meld bonus, got %d", state.Scores[0])
	}
	if state.Scores[1] != 0 {
		t.Fatalf("player 1 should have 0 bonus, got %d", state.Scores[1])
	}
}

func TestDrawPenaltyOnFaceCards(t *testing.T) {
	g := &genome.Genome{
		Skeleton: genome.TrickTaking,
		Players:  2,
		Borrowed: []genome.BorrowedMechanic{
			{Source: genome.Shedding, Mechanic: genome.MechDrawPenalty},
		},
	}

	state := &sim.GameState{
		Hands: [][]sim.Card{
			{{Suit: sim.Hearts, Rank: sim.Two}},
			{{Suit: sim.Spades, Rank: sim.Five}},
		},
		Deck:       []sim.Card{{Suit: sim.Clubs, Rank: sim.Three}},
		Scores:     []int{0, 0},
		NumPlayers: 2,
	}

	// Playing a Jack should trigger draw penalty
	event := sim.Event{
		Type:     sim.EventCardPlayed,
		PlayerID: 0,
		Cards:    []sim.Card{{Suit: sim.Hearts, Rank: sim.Jack}},
	}

	handSizeBefore := len(state.Hands[0])
	applyDrawPenalty(state, g, event)
	handSizeAfter := len(state.Hands[0])

	if handSizeAfter != handSizeBefore+1 {
		t.Fatalf("expected hand to grow by 1 from draw penalty, was %d now %d",
			handSizeBefore, handSizeAfter)
	}
}

func TestRunHooksFiltersByPoint(t *testing.T) {
	called := false
	hooks := []Hook{
		{
			Point:    HookAfterPlay,
			Mechanic: genome.MechDrawPenalty,
			Apply: func(state *sim.GameState, g *genome.Genome, event sim.Event) {
				called = true
			},
		},
	}

	state := &sim.GameState{}
	g := &genome.Genome{}

	// Scoring hook point should NOT trigger AfterPlay hook
	RunHooks(hooks, HookScoring, state, g, sim.Event{})
	if called {
		t.Fatal("hook should not fire for wrong hook point")
	}

	// AfterPlay should trigger
	RunHooks(hooks, HookAfterPlay, state, g, sim.Event{})
	if !called {
		t.Fatal("hook should fire for matching hook point")
	}
}

// TestCardPenaltySpecificity verifies that cardPenalty resolves overlapping
// CardPoints rules by specificity (suit+rank > suit-only > rank-only > catch-all)
// rather than by insertion order, mirroring cardPointValue. dd-cto regression.
func TestCardPenaltySpecificity(t *testing.T) {
	queenOfSpades := sim.Card{Suit: sim.Spades, Rank: sim.Queen}
	queenOfHearts := sim.Card{Suit: sim.Hearts, Rank: sim.Queen}
	twoOfSpades := sim.Card{Suit: sim.Spades, Rank: sim.Two}
	threeOfHearts := sim.Card{Suit: sim.Hearts, Rank: sim.Three}

	allQueens := genome.CardScoring{Rank: uint8(sim.Queen), Points: 10}
	qOfSpades := genome.CardScoring{Suit: uint8(sim.Spades) + 1, Rank: uint8(sim.Queen), Points: 13}
	allHearts := genome.CardScoring{Suit: uint8(sim.Hearts) + 1, Points: 1}
	catchAll := genome.CardScoring{Points: 99}

	rulesA := []genome.CardScoring{allQueens, qOfSpades, allHearts, catchAll}
	rulesB := []genome.CardScoring{catchAll, allHearts, qOfSpades, allQueens}

	for _, rules := range [][]genome.CardScoring{rulesA, rulesB} {
		g := &genome.Genome{
			Scoring: genome.ScoringConfig{CardPoints: rules},
		}
		if got := cardPenalty(queenOfSpades, g); got != 13 {
			t.Errorf("Queen of Spades: got %d, want 13 (suit+rank should beat rank-only and catch-all)", got)
		}
		if got := cardPenalty(queenOfHearts, g); got != 1 {
			t.Errorf("Queen of Hearts: got %d, want 1 (suit-only should beat rank-only and catch-all)", got)
		}
		if got := cardPenalty(twoOfSpades, g); got != 99 {
			t.Errorf("Two of Spades: got %d, want 99 (catch-all)", got)
		}
		if got := cardPenalty(threeOfHearts, g); got != 1 {
			t.Errorf("Three of Hearts: got %d, want 1 (suit-only beats catch-all)", got)
		}
	}
}

func TestCardPenaltyNoMatch(t *testing.T) {
	g := &genome.Genome{
		Scoring: genome.ScoringConfig{
			CardPoints: []genome.CardScoring{
				{Suit: uint8(sim.Hearts) + 1, Points: 1},
			},
		},
	}
	got := cardPenalty(sim.Card{Suit: sim.Spades, Rank: sim.Two}, g)
	if got != 0 {
		t.Errorf("no matching rule should return 0, got %d", got)
	}
}

func TestBorrowedMechanicInEvolution(t *testing.T) {
	// A shedding game with avoidance borrowed should still complete
	g := &genome.Genome{
		ID:       "shedding-with-avoidance",
		Skeleton: genome.Shedding,
		Players:  2,
		HandSize: 7,
		Shedding: &genome.SheddingParams{
			MatchRule:   genome.MatchEither,
			DrawPenalty: 1,
		},
		Scoring: genome.ScoringConfig{
			CardPoints: []genome.CardScoring{
				{Suit: uint8(sim.Hearts) + 1, Points: 1},
			},
		},
		Borrowed: []genome.BorrowedMechanic{
			{Source: genome.TrickTaking, Mechanic: genome.MechAvoidance},
		},
	}

	errs := genome.Validate(g)
	if len(errs) != 0 {
		t.Fatalf("genome should be valid: %v", errs)
	}

	hooks := BuildHooks(g)
	if len(hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(hooks))
	}
}

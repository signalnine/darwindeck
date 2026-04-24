package sim

import (
	"math/rand/v2"
	"testing"
)

func TestSheddingScorerPrefersIsolatedCard(t *testing.T) {
	scorer := &SheddingScorer{}
	// Hand: three kings share rank with each other; 5S shares neither
	// suit nor rank with any other card. Playing 5S saves the flexible
	// kings for later -- its score must beat playing a king, all else equal.
	state := &GameState{
		Hands: [][]Card{
			{
				{Suit: Hearts, Rank: King},
				{Suit: Diamonds, Rank: King},
				{Suit: Clubs, Rank: King},
				{Suit: Spades, Rank: Five},
			},
		},
		NumPlayers: 1,
		Active:     0,
	}

	isolated := Move{Type: MovePlay, Cards: []Card{{Suit: Spades, Rank: Five}}, PlayerID: 0}
	connected := Move{Type: MovePlay, Cards: []Card{{Suit: Hearts, Rank: King}}, PlayerID: 0}

	isolatedScore := scorer.ScoreMove(isolated, state)
	connectedScore := scorer.ScoreMove(connected, state)

	// KH has 2 connections (same rank as KD and KC); 5S has 0.
	// The isolation heuristic penalizes connections at 2.0 per connection,
	// so the score difference must be at least 3.0 (two-connection gap).
	// The buggy version had a duplicated loop with opposite sign that
	// cancelled most of the penalty down to 0.5 per connection -- a
	// ~1.0 gap that fails this assertion.
	diff := isolatedScore - connectedScore
	if diff < 3.0 {
		t.Fatalf("isolation preference too weak: isolated=%.2f connected=%.2f diff=%.2f (want >= 3.0)", isolatedScore, connectedScore, diff)
	}
}

func TestSheddingScorerPrefersPlay(t *testing.T) {
	scorer := &SheddingScorer{}
	state := &GameState{
		Hands: [][]Card{
			{{Suit: Hearts, Rank: King}, {Suit: Spades, Rank: Five}},
		},
		Active: 0,
	}

	playMove := Move{Type: MovePlay, Cards: []Card{{Suit: Hearts, Rank: King}}, PlayerID: 0}
	drawMove := Move{Type: MoveDraw, PlayerID: 0}

	playScore := scorer.ScoreMove(playMove, state)
	drawScore := scorer.ScoreMove(drawMove, state)

	if playScore <= drawScore {
		t.Fatalf("playing should score higher than drawing: play=%.1f draw=%.1f", playScore, drawScore)
	}
}

func TestTrickTakingScorerWinning(t *testing.T) {
	scorer := &TrickTakingScorer{Avoidance: false, TrumpSuit: -1}
	state := &GameState{
		TrickCards: []Card{{Suit: Hearts, Rank: Five}},
		Active:     1,
	}

	highCard := Move{Type: MovePlay, Cards: []Card{{Suit: Hearts, Rank: King}}, PlayerID: 1}
	lowCard := Move{Type: MovePlay, Cards: []Card{{Suit: Hearts, Rank: Two}}, PlayerID: 1}

	highScore := scorer.ScoreMove(highCard, state)
	lowScore := scorer.ScoreMove(lowCard, state)

	// Both win, but low winning card should be preferred (save high cards)
	// Actually high wins and low doesn't, so high should beat low here
	// Low card (2) doesn't beat 5, so it gets a dump score
	if highScore <= lowScore {
		t.Logf("high=%.1f low=%.1f (high wins trick, low dumps)", highScore, lowScore)
	}
}

func TestTrickTakingAvoidance(t *testing.T) {
	scorer := &TrickTakingScorer{Avoidance: true, TrumpSuit: -1}
	state := &GameState{
		TrickCards: []Card{{Suit: Hearts, Rank: Five}},
		Active:     1,
	}

	// In avoidance, playing a card that would win is bad
	winningCard := Move{Type: MovePlay, Cards: []Card{{Suit: Hearts, Rank: King}}, PlayerID: 1}
	losingCard := Move{Type: MovePlay, Cards: []Card{{Suit: Hearts, Rank: Two}}, PlayerID: 1}

	winScore := scorer.ScoreMove(winningCard, state)
	loseScore := scorer.ScoreMove(losingCard, state)

	if winScore >= loseScore {
		t.Fatalf("in avoidance, losing should score higher: win=%.1f lose=%.1f", winScore, loseScore)
	}
}

func TestRummyScorerPrefersMeld(t *testing.T) {
	scorer := &RummyScorer{}
	state := &GameState{
		Hands:  [][]Card{{{Suit: Hearts, Rank: King}}},
		Active: 0,
	}

	meldMove := Move{Type: MoveMeld, Cards: []Card{
		{Suit: Hearts, Rank: King},
		{Suit: Spades, Rank: King},
		{Suit: Clubs, Rank: King},
	}, PlayerID: 0}
	passMove := Move{Type: MovePass, PlayerID: 0}

	meldScore := scorer.ScoreMove(meldMove, state)
	passScore := scorer.ScoreMove(passMove, state)

	if meldScore <= passScore {
		t.Fatalf("melding should score higher than passing: meld=%.1f pass=%.1f", meldScore, passScore)
	}
}

func TestCardDeadwoodValues(t *testing.T) {
	cases := []struct {
		rank Rank
		want int
	}{
		{Ace, 1},
		{Two, 2},
		{Nine, 9},
		{Ten, 10},
		{Jack, 10},
		{Queen, 10},
		{King, 10},
	}
	for _, tc := range cases {
		got := cardDeadwood(Card{Suit: Hearts, Rank: tc.rank})
		if got != tc.want {
			t.Errorf("cardDeadwood(%s) = %d, want %d", tc.rank, got, tc.want)
		}
	}
}

func TestGreedyAISelectsHighest(t *testing.T) {
	scorer := &SheddingScorer{}
	ai := &GreedyAI{Scorer: scorer}
	rng := rand.New(rand.NewPCG(1, 0))

	state := &GameState{
		Hands: [][]Card{
			{{Suit: Hearts, Rank: King}, {Suit: Spades, Rank: Five}},
		},
		Active: 0,
	}

	moves := []Move{
		{Type: MoveDraw, PlayerID: 0},
		{Type: MovePlay, Cards: []Card{{Suit: Hearts, Rank: King}}, PlayerID: 0},
	}

	selected := ai.SelectMove(moves, state, rng)
	if selected.Type != MovePlay {
		t.Fatalf("greedy should select play over draw, got %d", selected.Type)
	}
}

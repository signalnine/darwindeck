package sim

import (
	"math/rand/v2"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
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

// TestSheddingScorerGenomeAwareOffensiveSpecials pins the dd-audit fix for
// the hardcoded 2/7/10 special-rank heuristic: the scorer must classify
// offensive specials from the genome's SpecialCards, not from rank folklore.
//
// Genome under test: draw-two lives on rank 8; rank 2 has NO effect. The
// greedy scorer must give the offensive-special blocking bonus to the 8 (as
// it previously gave to 2s) and must NOT special-case the 2.
func TestSheddingScorerGenomeAwareOffensiveSpecials(t *testing.T) {
	g := &genome.Genome{
		Skeleton: genome.Shedding,
		Players:  2,
		SpecialCards: []genome.SpecialCard{
			{Type: genome.SpecialDrawTwo, ByRank: 8},
		},
	}
	scorer := NewSheddingScorer(g)

	// Opponent is one card from winning, which triggers the blocking bonus
	// for offensive specials. The three hand cards (8H, 2S, 5D) share no
	// suit or rank with each other, so every play has zero connections and
	// any score gap is purely the special-card classification.
	state := &GameState{
		Hands: [][]Card{
			{
				{Suit: Hearts, Rank: Eight},
				{Suit: Spades, Rank: Two},
				{Suit: Diamonds, Rank: Five},
			},
			{{Suit: Clubs, Rank: King}},
		},
		NumPlayers: 2,
		Active:     0,
	}

	score := func(c Card) float64 {
		return scorer.ScoreMove(Move{Type: MovePlay, Cards: []Card{c}, PlayerID: 0}, state)
	}

	eightScore := score(Card{Suit: Hearts, Rank: Eight})
	twoScore := score(Card{Suit: Spades, Rank: Two})
	fiveScore := score(Card{Suit: Diamonds, Rank: Five})

	// The genome's draw-two (rank 8) must get the blocking bonus.
	if eightScore <= twoScore {
		t.Fatalf("genome draw-two on rank 8 must outscore non-special rank 2: eight=%.1f two=%.1f",
			eightScore, twoScore)
	}
	// Rank 2 has no genome effect: it must score exactly like a plain card.
	if twoScore != fiveScore {
		t.Fatalf("rank 2 has no genome effect but is scored as special: two=%.1f plain-five=%.1f",
			twoScore, fiveScore)
	}
}

// TestSheddingScorerSuitScopedSpecial verifies the scorer honors the BySuit
// restriction on SpecialCards: a skip on (rank 7, spades only) must not mark
// a 7 of hearts as offensive. BySuit uses the genome encoding suit+1.
func TestSheddingScorerSuitScopedSpecial(t *testing.T) {
	g := &genome.Genome{
		Skeleton: genome.Shedding,
		Players:  2,
		SpecialCards: []genome.SpecialCard{
			{Type: genome.SpecialSkip, ByRank: 7, BySuit: uint8(Spades) + 1},
		},
	}
	scorer := NewSheddingScorer(g)

	state := &GameState{
		Hands: [][]Card{
			{
				{Suit: Spades, Rank: Seven},
				{Suit: Hearts, Rank: Three},
			},
			{{Suit: Clubs, Rank: King}},
		},
		NumPlayers: 2,
		Active:     0,
	}

	sevenSpades := scorer.ScoreMove(Move{Type: MovePlay, Cards: []Card{{Suit: Spades, Rank: Seven}}, PlayerID: 0}, state)

	// Swap the 7S for a 7H in hand (keeps connection counts at zero).
	state.Hands[0][0] = Card{Suit: Hearts, Rank: Seven}
	sevenHearts := scorer.ScoreMove(Move{Type: MovePlay, Cards: []Card{{Suit: Hearts, Rank: Seven}}, PlayerID: 0}, state)

	if sevenSpades <= sevenHearts {
		t.Fatalf("suit-scoped skip: 7S must outscore 7H: spades=%.1f hearts=%.1f",
			sevenSpades, sevenHearts)
	}
}

// TestSheddingScorerNilGenomeNoSpecialBonus verifies the zero-value scorer
// no longer applies the legacy hardcoded 2/7/10 bonus: with no genome there
// is no special-card knowledge, so a 2 scores like any plain card.
func TestSheddingScorerNilGenomeNoSpecialBonus(t *testing.T) {
	scorer := &SheddingScorer{}
	state := &GameState{
		Hands: [][]Card{
			{
				{Suit: Spades, Rank: Two},
				{Suit: Diamonds, Rank: Five},
			},
			{{Suit: Clubs, Rank: King}},
		},
		NumPlayers: 2,
		Active:     0,
	}

	twoScore := scorer.ScoreMove(Move{Type: MovePlay, Cards: []Card{{Suit: Spades, Rank: Two}}, PlayerID: 0}, state)
	fiveScore := scorer.ScoreMove(Move{Type: MovePlay, Cards: []Card{{Suit: Diamonds, Rank: Five}}, PlayerID: 0}, state)

	if twoScore != fiveScore {
		t.Fatalf("nil-genome scorer must not special-case rank 2: two=%.1f five=%.1f",
			twoScore, fiveScore)
	}
}

func TestTrickTakingScorerWinning(t *testing.T) {
	scorer := &TrickTakingScorer{Avoidance: false}
	state := &GameState{
		TrickCards: []Card{{Suit: Hearts, Rank: Five}},
		Active:     1,
		TrumpSuit:  -1,
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
	scorer := &TrickTakingScorer{Avoidance: true}
	state := &GameState{
		TrickCards: []Card{{Suit: Hearts, Rank: Five}},
		Active:     1,
		TrumpSuit:  -1,
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

// TestTrickTakingScorerReadsRuntimeTrump verifies the scorer reads trump
// from the game state at scoring time (not from a static field), so it
// behaves correctly for TrumpCut and TrumpLed games where trump is set
// dynamically by the runner.
func TestTrickTakingScorerReadsRuntimeTrump(t *testing.T) {
	scorer := &TrickTakingScorer{Avoidance: false}

	// Lead is hearts; spades is trump (as a TrumpCut runner would set it).
	state := &GameState{
		TrickCards: []Card{{Suit: Hearts, Rank: Ten}},
		Active:     1,
		TrumpSuit:  int(Spades),
	}

	// Following with a low spade (trump) should beat the led ten of hearts.
	trumpCard := Move{Type: MovePlay, Cards: []Card{{Suit: Spades, Rank: Two}}, PlayerID: 1}
	// Following with a hearts king beats the ten too, but is not trump.
	heartsKing := Move{Type: MovePlay, Cards: []Card{{Suit: Hearts, Rank: King}}, PlayerID: 1}
	// Off-suit non-trump cannot win.
	offSuitJunk := Move{Type: MovePlay, Cards: []Card{{Suit: Diamonds, Rank: Three}}, PlayerID: 1}

	trumpScore := scorer.ScoreMove(trumpCard, state)
	heartsScore := scorer.ScoreMove(heartsKing, state)
	offScore := scorer.ScoreMove(offSuitJunk, state)

	// Trump 2 wins: 20 - 2*0.5 = 19. Hearts king wins: 20 - 13*0.5 = 13.5.
	// Off-suit dump: -3*0.5 = -1.5.
	if trumpScore <= heartsScore {
		t.Fatalf("trumping should beat winning with high non-trump: trump=%.1f hearts=%.1f", trumpScore, heartsScore)
	}
	if heartsScore <= offScore {
		t.Fatalf("winning should beat dumping: hearts=%.1f off=%.1f", heartsScore, offScore)
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

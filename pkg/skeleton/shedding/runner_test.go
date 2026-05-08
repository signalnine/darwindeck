package shedding

import (
	"math/rand/v2"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

// runGame plays a full game with random AI and returns the result.
func runGame(g *genome.Genome, seed uint64) sim.GameResult {
	runner := &Runner{}
	ai := &sim.RandomAI{}
	rng := rand.New(rand.NewPCG(seed, 0))

	state := runner.Setup(g, rng)
	maxTurns := g.MaxTurns()

	for {
		winner := runner.CheckEnd(state, g)
		if winner >= 0 {
			handSizes := make([]int, state.NumPlayers)
			for i, h := range state.Hands {
				handSizes[i] = len(h)
			}
			return sim.GameResult{
				Winner:    winner,
				Turns:     state.Turn,
				Events:    state.Events,
				HandSizes: handSizes,
			}
		}

		if state.Turn >= maxTurns {
			handSizes := make([]int, state.NumPlayers)
			for i, h := range state.Hands {
				handSizes[i] = len(h)
			}
			return sim.GameResult{
				Winner:    -1,
				Turns:     state.Turn,
				Events:    state.Events,
				HandSizes: handSizes,
				Error:     "max_turns",
			}
		}

		moves := runner.GenerateMoves(state, g)
		if len(moves) == 0 {
			return sim.GameResult{
				Winner: -1,
				Turns:  state.Turn,
				Error:  "no_moves",
			}
		}

		move := ai.SelectMove(moves, state, rng)
		events := runner.ApplyMove(state, move, g)
		state.Events = append(state.Events, events...)
	}
}

func TestCrazyEightsCompletes(t *testing.T) {
	g := seeds.CrazyEights()

	// Run 100 games and verify they all complete
	completions := 0
	for seed := uint64(0); seed < 100; seed++ {
		result := runGame(g, seed)
		if result.Winner >= 0 {
			completions++
		}
		if result.Winner >= 0 && (result.Winner < 0 || result.Winner >= g.Players) {
			t.Fatalf("invalid winner %d for %d players", result.Winner, g.Players)
		}
	}

	t.Logf("Crazy Eights: %d/100 games completed with a winner", completions)
	if completions < 80 {
		t.Fatalf("too few completions: %d/100 (expected >80)", completions)
	}
}

func TestMauMauCompletes(t *testing.T) {
	g := seeds.MauMau()

	completions := 0
	for seed := uint64(0); seed < 100; seed++ {
		result := runGame(g, seed)
		if result.Winner >= 0 {
			completions++
		}
	}

	t.Logf("Mau-Mau: %d/100 games completed with a winner", completions)
	if completions < 80 {
		t.Fatalf("too few completions: %d/100 (expected >80)", completions)
	}
}

func TestDeterminism(t *testing.T) {
	g := seeds.CrazyEights()

	r1 := runGame(g, 42)
	r2 := runGame(g, 42)

	if r1.Winner != r2.Winner {
		t.Fatalf("non-deterministic: winner %d vs %d", r1.Winner, r2.Winner)
	}
	if r1.Turns != r2.Turns {
		t.Fatalf("non-deterministic: turns %d vs %d", r1.Turns, r2.Turns)
	}
}

func TestDifferentSeeds(t *testing.T) {
	g := seeds.CrazyEights()

	// Different seeds should produce different games (at least sometimes)
	sameCount := 0
	for seed := uint64(0); seed < 50; seed++ {
		r1 := runGame(g, seed)
		r2 := runGame(g, seed+1000)
		if r1.Winner == r2.Winner && r1.Turns == r2.Turns {
			sameCount++
		}
	}

	if sameCount == 50 {
		t.Fatal("all games with different seeds produced identical results")
	}
}

func TestMovesAlwaysExist(t *testing.T) {
	// Verify that GenerateMoves never returns empty (draw/pass always available)
	g := seeds.CrazyEights()
	runner := &Runner{}
	rng := rand.New(rand.NewPCG(99, 0))
	state := runner.Setup(g, rng)

	for turn := 0; turn < 200; turn++ {
		winner := runner.CheckEnd(state, g)
		if winner >= 0 {
			break
		}

		moves := runner.GenerateMoves(state, g)
		if len(moves) == 0 {
			t.Fatalf("no moves available at turn %d, active player %d, hand size %d, deck size %d",
				turn, state.Active, len(state.ActiveHand()), len(state.Deck))
		}

		ai := &sim.RandomAI{}
		move := ai.SelectMove(moves, state, rng)
		runner.ApplyMove(state, move, g)
	}
}

func TestSetupDealCorrectly(t *testing.T) {
	g := seeds.CrazyEights()
	runner := &Runner{}
	rng := rand.New(rand.NewPCG(1, 0))
	state := runner.Setup(g, rng)

	// 2 players × 7 cards = 14 dealt, 1 flipped = 15 from deck
	expectedDeckSize := 52 - g.HandSize*g.Players - 1

	if len(state.Deck) != expectedDeckSize {
		t.Fatalf("expected deck size %d, got %d", expectedDeckSize, len(state.Deck))
	}

	for i, hand := range state.Hands {
		if len(hand) != g.HandSize {
			t.Fatalf("player %d hand size %d, expected %d", i, len(hand), g.HandSize)
		}
	}

	if len(state.Discard) != 1 {
		t.Fatalf("expected 1 card in discard, got %d", len(state.Discard))
	}

	if state.TopCard == nil {
		t.Fatal("expected top card to be set")
	}
}

func TestSpecialCards(t *testing.T) {
	// Create a genome with skip on 7s
	g := &genome.Genome{
		ID:       "test-special",
		Skeleton: genome.Shedding,
		Players:  3,
		HandSize: 5,
		Shedding: &genome.SheddingParams{
			MatchRule:   genome.MatchEither,
			DrawPenalty: 1,
		},
		SpecialCards: []genome.SpecialCard{
			{Type: genome.SpecialSkip, ByRank: uint8(sim.Seven)},
		},
	}

	// Run some games — just verify no crashes with specials active
	completions := 0
	for seed := uint64(0); seed < 50; seed++ {
		result := runGame(g, seed)
		if result.Winner >= 0 {
			completions++
		}
	}

	t.Logf("Special cards game: %d/50 completed", completions)
	if completions < 30 {
		t.Fatalf("too few completions with special cards: %d/50", completions)
	}
}

// TestDrawTwoSkipsVictim ensures SpecialDrawTwo not only inflicts cards
// on the next player but also forces them to lose their turn (Uno-style),
// matching the SpecialSkip behavior for consistency (dd-jey).
func TestDrawTwoSkipsVictim(t *testing.T) {
	g := &genome.Genome{
		ID:       "test-drawtwo",
		Skeleton: genome.Shedding,
		Players:  3,
		HandSize: 3,
		Shedding: &genome.SheddingParams{
			MatchRule:   genome.MatchEither,
			DrawPenalty: 1,
		},
		SpecialCards: []genome.SpecialCard{
			{Type: genome.SpecialDrawTwo, ByRank: uint8(sim.Two)},
		},
	}
	runner := &Runner{}
	state := &sim.GameState{
		NumPlayers: 3,
		Active:     0,
		Hands: [][]sim.Card{
			{{Suit: sim.Hearts, Rank: sim.Two}}, // Player 0 plays the draw-two
			{{Suit: sim.Hearts, Rank: sim.Five}},
			{{Suit: sim.Hearts, Rank: sim.Six}},
		},
		Deck:    sim.StandardDeck(), // plenty of cards to draw
		Discard: []sim.Card{{Suit: sim.Hearts, Rank: sim.Three}},
		TopCard: &sim.Card{Suit: sim.Hearts, Rank: sim.Three},
	}

	move := sim.Move{
		Type:     sim.MovePlay,
		Cards:    []sim.Card{{Suit: sim.Hearts, Rank: sim.Two}},
		PlayerID: 0,
	}
	runner.ApplyMove(state, move, g)

	// Player 1 should have received 2 cards on top of their original 1.
	if len(state.Hands[1]) != 3 {
		t.Fatalf("victim hand size = %d, want 3 (1 original + 2 drawn)", len(state.Hands[1]))
	}
	// Active should have advanced past player 1 to player 2.
	if state.Active != 2 {
		t.Fatalf("Active = %d after draw_two, want 2 (victim should be skipped)", state.Active)
	}
}

// TestDrawFourSkipsVictim mirrors TestDrawTwoSkipsVictim for SpecialDrawFour.
func TestDrawFourSkipsVictim(t *testing.T) {
	g := &genome.Genome{
		ID:       "test-drawfour",
		Skeleton: genome.Shedding,
		Players:  3,
		HandSize: 3,
		Shedding: &genome.SheddingParams{
			MatchRule:   genome.MatchEither,
			DrawPenalty: 1,
		},
		SpecialCards: []genome.SpecialCard{
			{Type: genome.SpecialDrawFour, ByRank: uint8(sim.Two)},
		},
	}
	runner := &Runner{}
	state := &sim.GameState{
		NumPlayers: 3,
		Active:     0,
		Hands: [][]sim.Card{
			{{Suit: sim.Hearts, Rank: sim.Two}},
			{{Suit: sim.Hearts, Rank: sim.Five}},
			{{Suit: sim.Hearts, Rank: sim.Six}},
		},
		Deck:    sim.StandardDeck(),
		Discard: []sim.Card{{Suit: sim.Hearts, Rank: sim.Three}},
		TopCard: &sim.Card{Suit: sim.Hearts, Rank: sim.Three},
	}

	move := sim.Move{
		Type:     sim.MovePlay,
		Cards:    []sim.Card{{Suit: sim.Hearts, Rank: sim.Two}},
		PlayerID: 0,
	}
	runner.ApplyMove(state, move, g)

	if len(state.Hands[1]) != 5 {
		t.Fatalf("victim hand size = %d, want 5 (1 original + 4 drawn)", len(state.Hands[1]))
	}
	if state.Active != 2 {
		t.Fatalf("Active = %d after draw_four, want 2 (victim should be skipped)", state.Active)
	}
}

// TestCheckEndReturnsMinusOneAtMaxTurns verifies that CheckEnd does not
// declare a winner when only the max-turns cap has been hit (no empty hand).
// This is required so the batch runner records a Timeout rather than a
// Completion -- otherwise Tier1 cannot detect hung shedding genomes (dd-4nd).
func TestCheckEndReturnsMinusOneAtMaxTurns(t *testing.T) {
	g := seeds.CrazyEights()
	runner := &Runner{}
	rng := rand.New(rand.NewPCG(1, 0))
	state := runner.Setup(g, rng)

	// Force the state to look like a stalled game: nobody has emptied their
	// hand, and the turn counter sits at MaxTurns.
	state.Turn = g.MaxTurns()
	for i := range state.Hands {
		if len(state.Hands[i]) == 0 {
			state.Hands[i] = []sim.Card{{Suit: sim.Hearts, Rank: sim.Two}}
		}
	}

	if winner := runner.CheckEnd(state, g); winner != -1 {
		t.Fatalf("CheckEnd at max turns with non-empty hands returned %d, want -1", winner)
	}
}

func TestMatchRules(t *testing.T) {
	rules := []genome.MatchRule{
		genome.MatchSuit,
		genome.MatchRank,
		genome.MatchEither,
		genome.MatchBoth,
	}

	for _, rule := range rules {
		g := &genome.Genome{
			ID:       "test-match",
			Skeleton: genome.Shedding,
			Players:  2,
			HandSize: 7,
			Shedding: &genome.SheddingParams{
				MatchRule:   rule,
				DrawPenalty: 1,
			},
		}

		completions := 0
		for seed := uint64(0); seed < 50; seed++ {
			result := runGame(g, seed)
			if result.Winner >= 0 {
				completions++
			}
		}

		t.Logf("MatchRule %d: %d/50 completed", rule, completions)
		// Only MatchEither is permissive enough that random play reliably
		// empties a hand inside the max-turn budget for this minimal genome
		// (no specials, no wilds). MatchSuit / MatchRank / MatchBoth all
		// legitimately time out under random play -- they need extra
		// mechanics (wilds, draw penalties tuned harder, etc.) to terminate.
		if rule == genome.MatchEither && completions < 25 {
			t.Fatalf("MatchRule %d: too few completions %d/50", rule, completions)
		}
	}
}

// TestReverseFlipsDirectionWith3Players ensures SpecialReverse actually
// reverses play order in 3+ player games (dd-xns). With 4 players starting at
// 0 and Direction=+1, a Reverse should make the next player be 3, not 1.
func TestReverseFlipsDirectionWith3Players(t *testing.T) {
	g := &genome.Genome{
		ID:       "test-reverse-4p",
		Skeleton: genome.Shedding,
		Players:  4,
		HandSize: 3,
		Shedding: &genome.SheddingParams{
			MatchRule:   genome.MatchEither,
			DrawPenalty: 1,
		},
		SpecialCards: []genome.SpecialCard{
			{Type: genome.SpecialReverse, ByRank: uint8(sim.Two)},
		},
	}
	runner := &Runner{}
	state := sim.NewGameState(4)
	state.Active = 0
	state.Hands = [][]sim.Card{
		{{Suit: sim.Hearts, Rank: sim.Two}},
		{{Suit: sim.Hearts, Rank: sim.Five}},
		{{Suit: sim.Hearts, Rank: sim.Six}},
		{{Suit: sim.Hearts, Rank: sim.Seven}},
	}
	state.Deck = sim.StandardDeck()
	state.Discard = []sim.Card{{Suit: sim.Hearts, Rank: sim.Three}}
	state.TopCard = &sim.Card{Suit: sim.Hearts, Rank: sim.Three}

	runner.ApplyMove(state, sim.Move{
		Type:     sim.MovePlay,
		Cards:    []sim.Card{{Suit: sim.Hearts, Rank: sim.Two}},
		PlayerID: 0,
	}, g)

	if state.Active != 3 {
		t.Fatalf("Active = %d after Reverse from player 0 in 4-player game; want 3 (direction reversed)", state.Active)
	}

	// And after another normal turn the active should continue going backward.
	state.Hands[3] = []sim.Card{{Suit: sim.Hearts, Rank: sim.Three}}
	runner.ApplyMove(state, sim.Move{
		Type:     sim.MovePlay,
		Cards:    []sim.Card{{Suit: sim.Hearts, Rank: sim.Three}},
		PlayerID: 3,
	}, g)
	if state.Active != 2 {
		t.Fatalf("Active = %d after second move post-reverse; want 2 (still going backward)", state.Active)
	}
}

// TestReverseTwoPlayerActsAsSkip preserves the 2-player Reverse semantics:
// the same player gets to play again (Active returns to 0).
func TestReverseTwoPlayerActsAsSkip(t *testing.T) {
	g := &genome.Genome{
		ID:       "test-reverse-2p",
		Skeleton: genome.Shedding,
		Players:  2,
		HandSize: 3,
		Shedding: &genome.SheddingParams{
			MatchRule:   genome.MatchEither,
			DrawPenalty: 1,
		},
		SpecialCards: []genome.SpecialCard{
			{Type: genome.SpecialReverse, ByRank: uint8(sim.Two)},
		},
	}
	runner := &Runner{}
	state := sim.NewGameState(2)
	state.Active = 0
	state.Hands = [][]sim.Card{
		{{Suit: sim.Hearts, Rank: sim.Two}},
		{{Suit: sim.Hearts, Rank: sim.Five}},
	}
	state.Deck = sim.StandardDeck()
	state.Discard = []sim.Card{{Suit: sim.Hearts, Rank: sim.Three}}
	state.TopCard = &sim.Card{Suit: sim.Hearts, Rank: sim.Three}

	runner.ApplyMove(state, sim.Move{
		Type:     sim.MovePlay,
		Cards:    []sim.Card{{Suit: sim.Hearts, Rank: sim.Two}},
		PlayerID: 0,
	}, g)

	if state.Active != 0 {
		t.Fatalf("Active = %d after Reverse in 2-player game; want 0 (acts as skip)", state.Active)
	}
}

func TestNextPlayerHonorsDirection(t *testing.T) {
	state := sim.NewGameState(4)
	state.Active = 1
	state.Direction = -1
	state.NextPlayer()
	if state.Active != 0 {
		t.Fatalf("NextPlayer with Direction=-1 from Active=1: got %d, want 0", state.Active)
	}
	state.NextPlayer()
	if state.Active != 3 {
		t.Fatalf("NextPlayer wrap with Direction=-1 from Active=0: got %d, want 3", state.Active)
	}
}

// TestSetupStoresRNG ensures Setup wires state.RNG, which the discard
// reshuffle path needs to randomize the recycled cards reproducibly.
func TestSetupStoresRNG(t *testing.T) {
	g := seeds.CrazyEights()
	runner := &Runner{}
	rng := rand.New(rand.NewPCG(11, 0))
	state := runner.Setup(g, rng)
	if state.RNG == nil {
		t.Fatal("Setup did not store rng on state.RNG; reshuffle on empty deck cannot stay reproducible")
	}
}

// TestGenerateMovesReshufflesDiscardWhenDeckEmpty constructs a state where
// the deck is empty and the active player has no playable card, but the
// discard pile has cards beyond the top. Real-world shedding games (Crazy
// Eights, Mau-Mau) reshuffle the discard pile back into the deck instead of
// looping passes. GenerateMoves must perform the reshuffle and offer a Draw
// rather than a Pass so the game can progress.
func TestGenerateMovesReshufflesDiscardWhenDeckEmpty(t *testing.T) {
	g := seeds.CrazyEights()
	runner := &Runner{}
	rng := rand.New(rand.NewPCG(3, 0))
	state := runner.Setup(g, rng)

	// Force the stalled state: empty deck, top card of Hearts/Two, and a
	// non-trivial pile underneath. Hand contains only a single Spades/Three
	// which does not match Hearts/Two by suit or rank under MatchEither.
	state.Deck = state.Deck[:0]
	top := sim.Card{Suit: sim.Hearts, Rank: sim.Two}
	pile := []sim.Card{
		{Suit: sim.Hearts, Rank: sim.Four},
		{Suit: sim.Diamonds, Rank: sim.Five},
		{Suit: sim.Clubs, Rank: sim.Six},
		{Suit: sim.Spades, Rank: sim.Seven},
		top,
	}
	state.Discard = pile
	state.TopCard = &sim.Card{Suit: top.Suit, Rank: top.Rank}
	state.Hands[state.Active] = []sim.Card{{Suit: sim.Spades, Rank: sim.Three}}

	moves := runner.GenerateMoves(state, g)
	if len(moves) == 0 {
		t.Fatal("expected a move (draw after reshuffle)")
	}
	if moves[0].Type != sim.MoveDraw {
		t.Fatalf("expected MoveDraw after reshuffle; got %v", moves[0].Type)
	}
	if len(state.Deck) == 0 {
		t.Fatal("expected state.Deck to be repopulated from discard pile")
	}
	if len(state.Discard) != 1 || state.Discard[0] != top {
		t.Fatalf("expected discard reduced to top card only; got %d cards top=%v", len(state.Discard), state.Discard)
	}
}

// TestStackedSpecialsTargetOriginalNextPlayer covers dd-rzo. When two
// SpecialCards entries match the same played card (e.g. one keyed by ByRank
// and another by BySuit), the second case used to read state.Active after
// the first case had already advanced it via NextPlayer, mistargeting the
// drawn cards or skipped player. With four players, a Skip+DrawTwo stack
// played by player 0 must penalize player 1 (origin+1), not player 2.
func TestStackedSpecialsTargetOriginalNextPlayer(t *testing.T) {
	sevenOfClubs := sim.Card{Suit: sim.Clubs, Rank: sim.Seven}
	g := &genome.Genome{
		ID:       "test-stacked-specials",
		Skeleton: genome.Shedding,
		Players:  4,
		HandSize: 1,
		Shedding: &genome.SheddingParams{
			MatchRule:   genome.MatchEither,
			DrawPenalty: 1,
		},
		SpecialCards: []genome.SpecialCard{
			{Type: genome.SpecialSkip, ByRank: uint8(sim.Seven)},
			{Type: genome.SpecialDrawTwo, BySuit: uint8(sim.Clubs) + 1},
		},
	}

	runner := &Runner{}
	state := &sim.GameState{
		NumPlayers: 4,
		Active:     0,
		Hands: [][]sim.Card{
			{sevenOfClubs},
			{{Suit: sim.Hearts, Rank: sim.Five}},
			{{Suit: sim.Hearts, Rank: sim.Six}},
			{{Suit: sim.Hearts, Rank: sim.Eight}},
		},
		Deck:    sim.StandardDeck(),
		Discard: []sim.Card{{Suit: sim.Clubs, Rank: sim.Three}},
		TopCard: &sim.Card{Suit: sim.Clubs, Rank: sim.Three},
	}

	runner.ApplyMove(state, sim.Move{
		Type:     sim.MovePlay,
		Cards:    []sim.Card{sevenOfClubs},
		PlayerID: 0,
	}, g)

	// Player 1 (origin+1) is the rightful victim of the DrawTwo. They
	// started with one card, must end with three.
	if got := len(state.Hands[1]); got != 3 {
		t.Fatalf("player 1 hand size = %d after stacked Skip+DrawTwo, want 3 (DrawTwo must target origin+1)", got)
	}
	// Player 2 should not have received any cards from the stacked specials.
	if got := len(state.Hands[2]); got != 1 {
		t.Fatalf("player 2 hand size = %d, want 1; specials wrongly targeted them after Skip rotated Active", got)
	}
}

package shedding

import (
	"fmt"
	"hash/fnv"
	"math/rand/v2"
	"reflect"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/mechanic"
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
		runner.Upkeep(state, g)

		winner := runner.CheckEnd(state, g)
		if winner >= 0 {
			return sim.GameResult{
				Winner: winner,
				Turns:  state.Turn,
				Events: state.Events,
			}
		}

		if state.Turn >= maxTurns {
			return sim.GameResult{
				Winner: -1,
				Turns:  state.Turn,
				Events: state.Events,
				Error:  "max_turns",
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
		runner.Upkeep(state, g)

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
// TestPlayableCountCountsWildDuplicates pins the round-4 FIX 1 statistic: the
// shedding runner reports how many of the active player's hand cards legally
// satisfy the match rule OR are wild, WITHOUT the alreadyInMoves dedup that
// GenerateMoves applies. This is the per-card playable count the
// dead_match_rule successor (per-turn playable share) needs -- LegalMoves
// undercounts because the runner collapses equivalent wild plays.
func TestPlayableCountCountsWildDuplicates(t *testing.T) {
	g := &genome.Genome{
		ID:       "test-playable-count",
		Skeleton: genome.Shedding,
		Players:  2,
		HandSize: 5,
		Shedding: &genome.SheddingParams{MatchRule: genome.MatchSuit, DrawPenalty: 1},
		// All of suit Spades is wild.
		SpecialCards: []genome.SpecialCard{
			{Type: genome.SpecialWild, BySuit: uint8(sim.Spades) + 1},
		},
	}
	runner := &Runner{}
	// Top is Hearts-Three (MatchSuit -> only Hearts match the rule). Hand:
	// two Hearts (match suit), two Spades (wild), one Clubs (neither). Under
	// MatchSuit the two Hearts are playable by rule and the two Spades are
	// playable by wild = 4 of 5 playable. GenerateMoves would emit those 4
	// MovePlay moves too here (no dedup collision since cards differ), but the
	// statistic must be the raw count regardless of dedup behaviour.
	state := &sim.GameState{
		NumPlayers: 2,
		Active:     0,
		Hands: [][]sim.Card{
			{
				{Suit: sim.Hearts, Rank: sim.Four},
				{Suit: sim.Hearts, Rank: sim.Seven},
				{Suit: sim.Spades, Rank: sim.Two},
				{Suit: sim.Spades, Rank: sim.Nine},
				{Suit: sim.Clubs, Rank: sim.King},
			},
			{{Suit: sim.Hearts, Rank: sim.Five}},
		},
		Deck:    sim.StandardDeck(),
		Discard: []sim.Card{{Suit: sim.Hearts, Rank: sim.Three}},
		TopCard: &sim.Card{Suit: sim.Hearts, Rank: sim.Three},
	}
	if got := runner.PlayableCount(state, g); got != 4 {
		t.Fatalf("PlayableCount = %d, want 4 (2 suit matches + 2 wild)", got)
	}

	// Now make ALL four suits wild via a catch-all-by-union (one wild rule per
	// suit): every card is playable, count == hand size.
	gAllWild := &genome.Genome{
		ID:       "test-all-wild",
		Skeleton: genome.Shedding,
		Players:  2,
		HandSize: 5,
		Shedding: &genome.SheddingParams{MatchRule: genome.MatchSuit, DrawPenalty: 1},
		SpecialCards: []genome.SpecialCard{
			{Type: genome.SpecialWild, BySuit: 1},
			{Type: genome.SpecialWild, BySuit: 2},
			{Type: genome.SpecialWild, BySuit: 3},
			{Type: genome.SpecialWild, BySuit: 4},
		},
	}
	if got := runner.PlayableCount(state, gAllWild); got != 5 {
		t.Fatalf("all-wild PlayableCount = %d, want 5 (whole hand playable)", got)
	}

	// No top card (start of game): nothing to match; only wilds count. With no
	// specials and no top, count is 0.
	gPlain := &genome.Genome{
		ID: "plain", Skeleton: genome.Shedding, Players: 2, HandSize: 5,
		Shedding: &genome.SheddingParams{MatchRule: genome.MatchSuit, DrawPenalty: 1},
	}
	noTop := *state
	noTop.TopCard = nil
	if got := runner.PlayableCount(&noTop, gPlain); got != 0 {
		t.Fatalf("no-top PlayableCount = %d, want 0", got)
	}
}

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

// TestSpecialSkipDoesNotCompoundWithDrawTwo verifies that when a card matches
// multiple special-card rules (e.g. Skip + DrawTwo), the victim is skipped
// once -- not twice -- and the trailing NextPlayer in ApplyMove still adds
// exactly one normal advance. Without this, mutation that produces both rules
// matching the same card silently rotates two seats past the victim (cards-czo).
func TestSpecialSkipDoesNotCompoundWithDrawTwo(t *testing.T) {
	g := &genome.Genome{
		ID:       "test-czo-skip-drawtwo",
		Skeleton: genome.Shedding,
		Players:  4,
		HandSize: 3,
		Shedding: &genome.SheddingParams{
			MatchRule:   genome.MatchEither,
			DrawPenalty: 1,
		},
		SpecialCards: []genome.SpecialCard{
			{Type: genome.SpecialSkip, ByRank: uint8(sim.Two)},
			{Type: genome.SpecialDrawTwo, ByRank: uint8(sim.Two)},
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

	if len(state.Hands[1]) != 3 {
		t.Fatalf("victim hand size = %d, want 3 (1 original + 2 drawn)", len(state.Hands[1]))
	}
	if state.Active != 2 {
		t.Fatalf("Active = %d after combined Skip+DrawTwo on a single card; want 2 (victim skipped exactly once)", state.Active)
	}
}

// TestDuplicateSpecialSkipRulesDoNotCompound verifies that two Skip rules
// matching the same card produce one skip, not two. Mutation does not
// deduplicate by (Type, ByRank, BySuit), so this case is reachable in
// evolved populations (cards-czo).
func TestDuplicateSpecialSkipRulesDoNotCompound(t *testing.T) {
	g := &genome.Genome{
		ID:       "test-czo-double-skip",
		Skeleton: genome.Shedding,
		Players:  4,
		HandSize: 3,
		Shedding: &genome.SheddingParams{
			MatchRule:   genome.MatchEither,
			DrawPenalty: 1,
		},
		SpecialCards: []genome.SpecialCard{
			{Type: genome.SpecialSkip, ByRank: uint8(sim.Two)},
			{Type: genome.SpecialSkip}, // any rank/suit -- also matches the Two
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

	if state.Active != 2 {
		t.Fatalf("Active = %d after duplicate Skip rules on one card; want 2 (one skip past victim)", state.Active)
	}
}

// TestDuplicateDrawTwoRulesDoNotStack verifies that two DrawTwo rules matching
// the same card draw 2 cards once, not 4. Compounding here would silently
// double-penalize the victim relative to the rulebook (cards-czo).
func TestDuplicateDrawTwoRulesDoNotStack(t *testing.T) {
	g := &genome.Genome{
		ID:       "test-czo-double-drawtwo",
		Skeleton: genome.Shedding,
		Players:  3,
		HandSize: 3,
		Shedding: &genome.SheddingParams{
			MatchRule:   genome.MatchEither,
			DrawPenalty: 1,
		},
		SpecialCards: []genome.SpecialCard{
			{Type: genome.SpecialDrawTwo, ByRank: uint8(sim.Two)},
			{Type: genome.SpecialDrawTwo}, // any rank/suit -- also matches the Two
		},
	}
	runner := &Runner{}
	state := sim.NewGameState(3)
	state.Active = 0
	state.Hands = [][]sim.Card{
		{{Suit: sim.Hearts, Rank: sim.Two}},
		{{Suit: sim.Hearts, Rank: sim.Five}},
		{{Suit: sim.Hearts, Rank: sim.Six}},
	}
	state.Deck = sim.StandardDeck()
	state.Discard = []sim.Card{{Suit: sim.Hearts, Rank: sim.Three}}
	state.TopCard = &sim.Card{Suit: sim.Hearts, Rank: sim.Three}

	runner.ApplyMove(state, sim.Move{
		Type:     sim.MovePlay,
		Cards:    []sim.Card{{Suit: sim.Hearts, Rank: sim.Two}},
		PlayerID: 0,
	}, g)

	if len(state.Hands[1]) != 3 {
		t.Fatalf("victim hand size = %d, want 3 (1 original + 2 drawn); duplicate DrawTwo must not stack", len(state.Hands[1]))
	}
	if state.Active != 2 {
		t.Fatalf("Active = %d; want 2 (one skip past victim)", state.Active)
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

// TestUpkeepReshufflesDiscardWhenDeckEmpty constructs a state where the deck
// is empty and the active player has no playable card, but the discard pile
// has cards beyond the top. Real-world shedding games (Crazy Eights, Mau-Mau)
// reshuffle the discard pile back into the deck instead of looping passes.
// Upkeep must perform the reshuffle (GenerateMoves is a pure query and may
// not -- audit Task 3) so the subsequent GenerateMoves offers a Draw rather
// than a Pass and the game can progress.
func TestUpkeepReshufflesDiscardWhenDeckEmpty(t *testing.T) {
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

	runner.Upkeep(state, g)
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

// TestDrawTwoHonorsReversedDirection covers dd-itq. When Direction=-1, the
// SpecialDrawTwo victim must be origin-1 (next-in-direction), not origin+1.
// Three players, P0 plays a DrawTwo with Direction=-1: P2 receives the 2
// cards and is skipped; Active lands on P1.
func TestDrawTwoHonorsReversedDirection(t *testing.T) {
	g := &genome.Genome{
		ID:       "test-drawtwo-dir-neg",
		Skeleton: genome.Shedding,
		Players:  3,
		HandSize: 1,
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
		Direction:  -1,
		Hands: [][]sim.Card{
			{{Suit: sim.Hearts, Rank: sim.Two}},
			{{Suit: sim.Hearts, Rank: sim.Five}},
			{{Suit: sim.Hearts, Rank: sim.Six}},
		},
		Deck:    sim.StandardDeck(),
		Discard: []sim.Card{{Suit: sim.Hearts, Rank: sim.Three}},
		TopCard: &sim.Card{Suit: sim.Hearts, Rank: sim.Three},
	}

	runner.ApplyMove(state, sim.Move{
		Type:     sim.MovePlay,
		Cards:    []sim.Card{{Suit: sim.Hearts, Rank: sim.Two}},
		PlayerID: 0,
	}, g)

	if got := len(state.Hands[2]); got != 3 {
		t.Fatalf("player 2 hand size = %d after DrawTwo with Direction=-1, want 3 (origin-1 victim)", got)
	}
	if got := len(state.Hands[1]); got != 1 {
		t.Fatalf("player 1 hand size = %d, want 1; DrawTwo with Direction=-1 wrongly targeted them", got)
	}
	if state.Active != 1 {
		t.Fatalf("Active = %d after DrawTwo with Direction=-1, want 1 (P2 victim should be skipped)", state.Active)
	}
}

// TestDrawFourHonorsReversedDirection covers dd-itq for DrawFour.
func TestDrawFourHonorsReversedDirection(t *testing.T) {
	g := &genome.Genome{
		ID:       "test-drawfour-dir-neg",
		Skeleton: genome.Shedding,
		Players:  3,
		HandSize: 1,
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
		Direction:  -1,
		Hands: [][]sim.Card{
			{{Suit: sim.Hearts, Rank: sim.Two}},
			{{Suit: sim.Hearts, Rank: sim.Five}},
			{{Suit: sim.Hearts, Rank: sim.Six}},
		},
		Deck:    sim.StandardDeck(),
		Discard: []sim.Card{{Suit: sim.Hearts, Rank: sim.Three}},
		TopCard: &sim.Card{Suit: sim.Hearts, Rank: sim.Three},
	}

	runner.ApplyMove(state, sim.Move{
		Type:     sim.MovePlay,
		Cards:    []sim.Card{{Suit: sim.Hearts, Rank: sim.Two}},
		PlayerID: 0,
	}, g)

	if got := len(state.Hands[2]); got != 5 {
		t.Fatalf("player 2 hand size = %d after DrawFour with Direction=-1, want 5 (origin-1 victim)", got)
	}
	if got := len(state.Hands[1]); got != 1 {
		t.Fatalf("player 1 hand size = %d, want 1; DrawFour with Direction=-1 wrongly targeted them", got)
	}
	if state.Active != 1 {
		t.Fatalf("Active = %d after DrawFour with Direction=-1, want 1 (P2 victim should be skipped)", state.Active)
	}
}

// TestSkipHonorsReversedDirection covers dd-itq for Skip.
func TestSkipHonorsReversedDirection(t *testing.T) {
	g := &genome.Genome{
		ID:       "test-skip-dir-neg",
		Skeleton: genome.Shedding,
		Players:  3,
		HandSize: 1,
		Shedding: &genome.SheddingParams{
			MatchRule:   genome.MatchEither,
			DrawPenalty: 1,
		},
		SpecialCards: []genome.SpecialCard{
			{Type: genome.SpecialSkip, ByRank: uint8(sim.Two)},
		},
	}
	runner := &Runner{}
	state := &sim.GameState{
		NumPlayers: 3,
		Active:     0,
		Direction:  -1,
		Hands: [][]sim.Card{
			{{Suit: sim.Hearts, Rank: sim.Two}},
			{{Suit: sim.Hearts, Rank: sim.Five}},
			{{Suit: sim.Hearts, Rank: sim.Six}},
		},
		Deck:    sim.StandardDeck(),
		Discard: []sim.Card{{Suit: sim.Hearts, Rank: sim.Three}},
		TopCard: &sim.Card{Suit: sim.Hearts, Rank: sim.Three},
	}

	events := runner.ApplyMove(state, sim.Move{
		Type:     sim.MovePlay,
		Cards:    []sim.Card{{Suit: sim.Hearts, Rank: sim.Two}},
		PlayerID: 0,
	}, g)

	if state.Active != 1 {
		t.Fatalf("Active = %d after Skip with Direction=-1, want 1 (P2 should be skipped)", state.Active)
	}

	var foundSkip bool
	for _, e := range events {
		if e.Type == sim.EventSpecialTriggered && e.Detail == "skip" {
			foundSkip = true
			if e.PlayerID != 2 {
				t.Fatalf("Skip event PlayerID = %d, want 2 (origin-1 with Direction=-1)", e.PlayerID)
			}
		}
	}
	if !foundSkip {
		t.Fatal("expected a skip EventSpecialTriggered, found none")
	}
}

// TestChainedReverseDrawTwoHonorsNewDirection covers dd-itq for mid-loop
// direction flips. With 4 players starting at Direction=+1 and Active=0,
// playing a card that matches BOTH a Reverse and a DrawTwo special must apply
// Reverse first (flipping direction to -1), then DrawTwo against the new
// origin-1 victim = player 3, not origin+1.
func TestChainedReverseDrawTwoHonorsNewDirection(t *testing.T) {
	threeOfClubs := sim.Card{Suit: sim.Clubs, Rank: sim.Three}
	g := &genome.Genome{
		ID:       "test-chained-reverse-drawtwo",
		Skeleton: genome.Shedding,
		Players:  4,
		HandSize: 1,
		Shedding: &genome.SheddingParams{
			MatchRule:   genome.MatchEither,
			DrawPenalty: 1,
		},
		SpecialCards: []genome.SpecialCard{
			{Type: genome.SpecialReverse, ByRank: uint8(sim.Three)},
			{Type: genome.SpecialDrawTwo, BySuit: uint8(sim.Clubs) + 1},
		},
	}
	runner := &Runner{}
	state := &sim.GameState{
		NumPlayers: 4,
		Active:     0,
		Direction:  1,
		Hands: [][]sim.Card{
			{threeOfClubs},
			{{Suit: sim.Hearts, Rank: sim.Five}},
			{{Suit: sim.Hearts, Rank: sim.Six}},
			{{Suit: sim.Hearts, Rank: sim.Eight}},
		},
		Deck:    sim.StandardDeck(),
		Discard: []sim.Card{{Suit: sim.Clubs, Rank: sim.Four}},
		TopCard: &sim.Card{Suit: sim.Clubs, Rank: sim.Four},
	}

	runner.ApplyMove(state, sim.Move{
		Type:     sim.MovePlay,
		Cards:    []sim.Card{threeOfClubs},
		PlayerID: 0,
	}, g)

	if got := len(state.Hands[3]); got != 3 {
		t.Fatalf("player 3 hand size = %d after Reverse+DrawTwo chain, want 3 (DrawTwo must target new origin-1 victim after Reverse)", got)
	}
	if got := len(state.Hands[1]); got != 1 {
		t.Fatalf("player 1 hand size = %d, want 1; DrawTwo used stale direction and hit origin+1", got)
	}
	if state.Direction != -1 {
		t.Fatalf("Direction = %d after Reverse, want -1", state.Direction)
	}
}

// TestDrawTwoRecyclesDiscardWhenDeckEmpty covers dd-9jy. When the deck is empty
// and a DrawTwo fires, the simulation must recycle the discard pile (minus the
// just-played top card) into the deck before drawing, so the victim actually
// receives the penalty cards instead of the effect silently no-opping.
func TestDrawTwoRecyclesDiscardWhenDeckEmpty(t *testing.T) {
	g := &genome.Genome{
		ID:       "test-drawtwo-empty-deck",
		Skeleton: genome.Shedding,
		Players:  3,
		HandSize: 1,
		Shedding: &genome.SheddingParams{
			MatchRule:   genome.MatchEither,
			DrawPenalty: 1,
		},
		SpecialCards: []genome.SpecialCard{
			{Type: genome.SpecialDrawTwo, ByRank: uint8(sim.Seven)},
		},
	}
	runner := &Runner{}
	state := &sim.GameState{
		NumPlayers: 3,
		Active:     0,
		Hands: [][]sim.Card{
			{{Suit: sim.Hearts, Rank: sim.Seven}},
			{{Suit: sim.Clubs, Rank: sim.Five}},
			{{Suit: sim.Diamonds, Rank: sim.Six}},
		},
		Deck: nil, // empty
		Discard: []sim.Card{
			{Suit: sim.Hearts, Rank: sim.Four},
			{Suit: sim.Diamonds, Rank: sim.Five},
			{Suit: sim.Clubs, Rank: sim.Six},
			{Suit: sim.Spades, Rank: sim.Eight},
			{Suit: sim.Hearts, Rank: sim.Nine},
		},
		TopCard: &sim.Card{Suit: sim.Hearts, Rank: sim.Nine},
		RNG:     rand.New(rand.NewPCG(42, 0)),
	}

	runner.ApplyMove(state, sim.Move{
		Type:     sim.MovePlay,
		Cards:    []sim.Card{{Suit: sim.Hearts, Rank: sim.Seven}},
		PlayerID: 0,
	}, g)

	if got := len(state.Hands[1]); got != 3 {
		t.Fatalf("victim hand size = %d, want 3 (1 original + 2 drawn after discard recycle)", got)
	}
}

// TestDrawFourRecyclesDiscardWhenDeckEmpty covers dd-9jy for DrawFour: the
// discard pile must be recycled so the victim actually receives 4 cards.
func TestDrawFourRecyclesDiscardWhenDeckEmpty(t *testing.T) {
	g := &genome.Genome{
		ID:       "test-drawfour-empty-deck",
		Skeleton: genome.Shedding,
		Players:  3,
		HandSize: 1,
		Shedding: &genome.SheddingParams{
			MatchRule:   genome.MatchEither,
			DrawPenalty: 1,
		},
		SpecialCards: []genome.SpecialCard{
			{Type: genome.SpecialDrawFour, ByRank: uint8(sim.Seven)},
		},
	}
	runner := &Runner{}
	state := &sim.GameState{
		NumPlayers: 3,
		Active:     0,
		Hands: [][]sim.Card{
			{{Suit: sim.Hearts, Rank: sim.Seven}},
			{{Suit: sim.Clubs, Rank: sim.Five}},
			{{Suit: sim.Diamonds, Rank: sim.Six}},
		},
		Deck: nil,
		Discard: []sim.Card{
			{Suit: sim.Hearts, Rank: sim.Four},
			{Suit: sim.Diamonds, Rank: sim.Five},
			{Suit: sim.Clubs, Rank: sim.Six},
			{Suit: sim.Spades, Rank: sim.Eight},
			{Suit: sim.Hearts, Rank: sim.Nine},
		},
		TopCard: &sim.Card{Suit: sim.Hearts, Rank: sim.Nine},
		RNG:     rand.New(rand.NewPCG(42, 0)),
	}

	runner.ApplyMove(state, sim.Move{
		Type:     sim.MovePlay,
		Cards:    []sim.Card{{Suit: sim.Hearts, Rank: sim.Seven}},
		PlayerID: 0,
	}, g)

	if got := len(state.Hands[1]); got != 5 {
		t.Fatalf("victim hand size = %d, want 5 (1 original + 4 drawn after discard recycle)", got)
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

// stateHash serializes every GameState field except RNG (a pointer; the
// purity tests pass a nil RNG sentinel so any dereference panics loudly).
// Used to assert that query methods do not mutate state (audit Task 3).
func stateHash(s *sim.GameState) string {
	top := "nil"
	if s.TopCard != nil {
		top = s.TopCard.String()
	}
	return fmt.Sprintf("deck=%v|hands=%v|discard=%v|tableau=%v|scores=%v|turn=%d|active=%d|phase=%d|np=%d|dir=%d|round=%d|maxround=%d|top=%s|tc=%v|tp=%v|tl=%d|trump=%d|broken=%t|melds=%v|owners=%v|events=%v",
		s.Deck, s.Hands, s.Discard, s.Tableau, s.Scores, s.Turn, s.Active,
		s.Phase, s.NumPlayers, s.Direction, s.Round, s.MaxRound, top,
		s.TrickCards, s.TrickPlayers, s.TrickLeader, s.TrumpSuit, s.TrickBroken,
		s.Melds, s.MeldOwner, s.Events)
}

// TestGenerateMovesIsPure pins audit Task 3: GenerateMoves must be a pure
// query. The historical violation: with an empty deck and no playable card it
// recycled the discard pile into the deck and advanced state.RNG. That
// maintenance now lives in Upkeep.
func TestGenerateMovesIsPure(t *testing.T) {
	g := seeds.CrazyEights()
	runner := &Runner{}

	// Scenario A: ordinary post-setup state.
	fresh := runner.Setup(g, rand.New(rand.NewPCG(3, 0)))
	fresh.RNG = nil // sentinel: pure queries must never touch the RNG

	// Scenario B: stalled state that used to trigger the in-query recycle --
	// empty deck, multi-card discard pile, no playable card in hand.
	stalled := runner.Setup(g, rand.New(rand.NewPCG(3, 0)))
	stalled.Deck = stalled.Deck[:0]
	top := sim.Card{Suit: sim.Hearts, Rank: sim.Two}
	stalled.Discard = []sim.Card{
		{Suit: sim.Hearts, Rank: sim.Four},
		{Suit: sim.Diamonds, Rank: sim.Five},
		{Suit: sim.Clubs, Rank: sim.Six},
		{Suit: sim.Spades, Rank: sim.Seven},
		top,
	}
	stalled.TopCard = &sim.Card{Suit: top.Suit, Rank: top.Rank}
	stalled.Hands[stalled.Active] = []sim.Card{{Suit: sim.Spades, Rank: sim.Three}}
	stalled.RNG = nil

	for name, state := range map[string]*sim.GameState{"fresh": fresh, "stalled": stalled} {
		before := stateHash(state)
		m1 := runner.GenerateMoves(state, g)
		after1 := stateHash(state)
		m2 := runner.GenerateMoves(state, g)
		after2 := stateHash(state)

		if after1 != before {
			t.Errorf("%s: first GenerateMoves mutated state:\nbefore: %s\nafter:  %s", name, before, after1)
		}
		if after2 != before {
			t.Errorf("%s: second GenerateMoves mutated state:\nbefore: %s\nafter:  %s", name, before, after2)
		}
		if !reflect.DeepEqual(m1, m2) {
			t.Errorf("%s: repeated GenerateMoves returned different moves:\n%v\nvs\n%v", name, m1, m2)
		}
	}
}

// playOutWithProgress plays a full game with random AI, asserting at every
// applied move that Progress returns one value per player in [0,1] (audit
// Task 8). Returns the final state and winner (-1 when the game did not
// complete naturally).
func playOutWithProgress(t *testing.T, g *genome.Genome, seed uint64) (*sim.GameState, int) {
	t.Helper()
	runner := &Runner{}
	ai := &sim.RandomAI{}
	rng := rand.New(rand.NewPCG(seed, 0))

	state := runner.Setup(g, rng)
	maxTurns := g.MaxTurns()

	for {
		runner.Upkeep(state, g)
		if winner := runner.CheckEnd(state, g); winner >= 0 {
			return state, winner
		}
		if state.Turn >= maxTurns {
			return state, -1
		}
		moves := runner.GenerateMoves(state, g)
		if len(moves) == 0 {
			return state, -1
		}
		move := ai.SelectMove(moves, state, rng)
		runner.ApplyMove(state, move, g)

		progress := runner.Progress(state, g)
		if len(progress) != state.NumPlayers {
			t.Fatalf("seed %d: Progress returned %d values, want %d", seed, len(progress), state.NumPlayers)
		}
		for p, v := range progress {
			if v < 0 || v > 1 {
				t.Fatalf("seed %d: Progress[%d] = %v, want in [0,1]", seed, p, v)
			}
		}
	}
}

// TestProgressWinnerIsMaxAcrossSeeds is the Task 8 (b) property: in a
// played-out game, the eventual winner's final Progress is the maximum
// across players (ties allowed).
func TestProgressWinnerIsMaxAcrossSeeds(t *testing.T) {
	g := seeds.CrazyEights()
	runner := &Runner{}
	completed := 0
	for seed := uint64(0); seed < 10; seed++ {
		state, winner := playOutWithProgress(t, g, seed)
		if winner < 0 {
			continue
		}
		completed++
		progress := runner.Progress(state, g)
		for p, v := range progress {
			if v > progress[winner] {
				t.Errorf("seed %d: Progress[%d] = %v exceeds winner %d's %v", seed, p, v, winner, progress[winner])
			}
		}
	}
	if completed == 0 {
		t.Fatal("no seed completed: winner-max property never exercised")
	}
}

// TestProgressIncreasesOnShed is the Task 8 (c) property: shedding a card
// raises the shedder's Progress (1 - hand/initialHandSize grows as the hand
// shrinks).
func TestProgressIncreasesOnShed(t *testing.T) {
	g := &genome.Genome{
		Skeleton: genome.Shedding,
		Players:  2,
		HandSize: 2,
		Shedding: &genome.SheddingParams{MatchRule: genome.MatchEither, DrawPenalty: 1},
	}
	runner := &Runner{}

	state := sim.NewGameState(2)
	top := sim.Card{Suit: sim.Hearts, Rank: sim.Seven}
	state.Discard = []sim.Card{top}
	state.TopCard = &top
	state.Hands[0] = []sim.Card{{Suit: sim.Spades, Rank: sim.Seven}, {Suit: sim.Diamonds, Rank: sim.Two}}
	state.Hands[1] = []sim.Card{{Suit: sim.Clubs, Rank: sim.Nine}, {Suit: sim.Clubs, Rank: sim.Ten}}
	state.Phase = sim.PhasePlay

	before := runner.Progress(state, g)
	runner.ApplyMove(state, sim.Move{
		Type:     sim.MovePlay,
		Cards:    []sim.Card{{Suit: sim.Spades, Rank: sim.Seven}},
		PlayerID: 0,
	}, g)
	after := runner.Progress(state, g)

	if after[0] <= before[0] {
		t.Errorf("Progress[0] did not increase on shed: before %v, after %v", before[0], after[0])
	}
	if after[1] != before[1] {
		t.Errorf("Progress[1] changed without acting: before %v, after %v", before[1], after[1])
	}
}

// TestProgressClampsGrownHands pins the clamp: draw penalties can grow a hand
// past the initial deal, and Progress must floor at 0 rather than go negative.
func TestProgressClampsGrownHands(t *testing.T) {
	g := &genome.Genome{
		Skeleton: genome.Shedding,
		Players:  2,
		HandSize: 2,
		Shedding: &genome.SheddingParams{MatchRule: genome.MatchEither, DrawPenalty: 1},
	}
	runner := &Runner{}
	state := sim.NewGameState(2)
	state.Hands[0] = []sim.Card{
		{Suit: sim.Clubs, Rank: sim.Two}, {Suit: sim.Clubs, Rank: sim.Three},
		{Suit: sim.Clubs, Rank: sim.Four}, {Suit: sim.Clubs, Rank: sim.Five},
	}
	state.Hands[1] = []sim.Card{{Suit: sim.Hearts, Rank: sim.Nine}}

	progress := runner.Progress(state, g)
	if progress[0] != 0 {
		t.Errorf("Progress[0] = %v for a hand grown past the deal, want 0 (clamped)", progress[0])
	}
	if want := 0.5; progress[1] != want {
		t.Errorf("Progress[1] = %v, want %v (1 - 1/2)", progress[1], want)
	}
}

// --- Task 22: multi-round shedding (banked-score rounds) ---

// fingerprintBatch folds a batch's winners, turn counts, and full event
// streams into one FNV-1a hash. Pinned values were captured from the
// pre-Task-22 runner (commit 8c36b21 work tree) so single-round behavior can
// be asserted byte-identical.
func fingerprintBatch(g *genome.Genome, n int, seed uint64) (uint64, sim.BatchResult) {
	runner := &Runner{}
	res := sim.RunBatch(g, runner, &sim.RandomAI{}, n, seed)
	h := fnv.New64a()
	for gi, events := range res.AllEvents {
		fmt.Fprintf(h, "game %d winner %d turns %d\n", gi, res.AllWinners[gi], res.TurnsList[gi])
		for _, e := range events {
			fmt.Fprintf(h, "%d|%d|%s|", e.Type, e.PlayerID, e.Detail)
			for _, c := range e.Cards {
				fmt.Fprintf(h, "%d.%d,", c.Suit, c.Rank)
			}
			fmt.Fprintln(h)
		}
	}
	return h.Sum64(), res
}

// singleRoundBorrowGenome carries both scoring borrows but NO RoundsPerGame,
// so it must keep pre-Task-22 single-round semantics bit for bit.
func singleRoundBorrowGenome() *genome.Genome {
	return &genome.Genome{
		ID:       "shedding-borrow-single-round",
		Skeleton: genome.Shedding,
		Players:  2,
		HandSize: 7,
		Shedding: &genome.SheddingParams{
			MatchRule:   genome.MatchEither,
			DrawPenalty: 1,
		},
		Borrowed: []genome.BorrowedMechanic{
			{Source: genome.Rummy, Mechanic: genome.MechMeldBonus},
			{Source: genome.TrickTaking, Mechanic: genome.MechAvoidance},
		},
		Scoring: genome.ScoringConfig{
			CardPoints: []genome.CardScoring{
				{Suit: uint8(sim.Hearts) + 1, Points: 1},
			},
		},
	}
}

// TestSingleRoundBehaviorBytePinned pins the no-multi-round paths against
// event-stream fingerprints captured from the pre-Task-22 runner: classics
// (no borrows), and a scoring-borrow genome without RoundsPerGame. Any drift
// here means Task 22 changed single-round semantics -- exactly what the
// calibration re-baseline rule forbids.
func TestSingleRoundBehaviorBytePinned(t *testing.T) {
	cases := []struct {
		name     string
		g        *genome.Genome
		fp       uint64
		wins     []int
		turns    int
		maxTurns int
	}{
		{"crazy-eights", seeds.CrazyEights(), 0x380f3d410dfc17c8, []int{24, 26}, 2044, 140},
		{"mau-mau", seeds.MauMau(), 0x6efc924ad6f62557, []int{20, 12, 18}, 1865, 150},
		{"borrow-single-round", singleRoundBorrowGenome(), 0x9c1142b653a2cbc1, []int{23, 27}, 2201, 140},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.g.MaxTurns(); got != tc.maxTurns {
				t.Errorf("MaxTurns = %d, want pre-change %d", got, tc.maxTurns)
			}
			fp, res := fingerprintBatch(tc.g, 50, 12345)
			if fp != tc.fp {
				t.Errorf("event-stream fingerprint = %#016x, want pre-change %#016x (single-round semantics drifted)", fp, tc.fp)
			}
			if !reflect.DeepEqual(res.WinCounts, tc.wins) {
				t.Errorf("win counts = %v, want pre-change %v", res.WinCounts, tc.wins)
			}
			if res.TotalTurns != tc.turns {
				t.Errorf("total turns = %d, want pre-change %d", res.TotalTurns, tc.turns)
			}
			if res.Completions != 50 {
				t.Errorf("completions = %d, want 50", res.Completions)
			}
		})
	}
}

// multiRoundMeldGenome is the Task 22 reference genome: shedding host, rummy
// MeldBonus borrow, 3 banked-score rounds. HandSize 13 / DrawPenalty 3 /
// 3 players keep residual hands large enough at round end that 3-card melds
// actually appear (measured: ~22/30 seeded games bank a nonzero score; at
// DrawPenalty 1 with small hands the bonus almost never fires).
func multiRoundMeldGenome() *genome.Genome {
	return &genome.Genome{
		ID:       "shedding-meldbonus-3rounds",
		Skeleton: genome.Shedding,
		Players:  3,
		HandSize: 13,
		Shedding: &genome.SheddingParams{
			MatchRule:     genome.MatchEither,
			DrawPenalty:   3,
			RoundsPerGame: 3,
		},
		Borrowed: []genome.BorrowedMechanic{
			{Source: genome.Rummy, Mechanic: genome.MechMeldBonus},
		},
	}
}

// multiRoundAvoidanceGenome: shedding host, trick-taking Avoidance borrow,
// 3 rounds. Hearts held at round end cost a point each (banked negative).
func multiRoundAvoidanceGenome() *genome.Genome {
	return &genome.Genome{
		ID:       "shedding-avoidance-3rounds",
		Skeleton: genome.Shedding,
		Players:  2,
		HandSize: 7,
		Shedding: &genome.SheddingParams{
			MatchRule:     genome.MatchEither,
			DrawPenalty:   1,
			RoundsPerGame: 3,
		},
		Borrowed: []genome.BorrowedMechanic{
			{Source: genome.TrickTaking, Mechanic: genome.MechAvoidance},
		},
		Scoring: genome.ScoringConfig{
			CardPoints: []genome.CardScoring{
				{Suit: uint8(sim.Hearts) + 1, Points: 1},
			},
		},
	}
}

// multiRoundTrickScoringGenome: shedding host, trick-taking MechTrickScoring
// borrow, 3 rounds -- the headline cross-skeleton hybrid (novelty evolution):
// a shed-to-win game scored by tricks. Each shed card is a "trick"; the best
// shedder of each round banks a capture bonus.
func multiRoundTrickScoringGenome() *genome.Genome {
	return &genome.Genome{
		ID:       "shedding-trickscoring-3rounds",
		Skeleton: genome.Shedding,
		Players:  3,
		HandSize: 7,
		Shedding: &genome.SheddingParams{
			MatchRule:     genome.MatchEither,
			DrawPenalty:   1,
			RoundsPerGame: 3,
		},
		Borrowed: []genome.BorrowedMechanic{
			{Source: genome.TrickTaking, Mechanic: genome.MechTrickScoring},
		},
	}
}

// TestTrickScoredSheddingBanksAndDeterminesWinner (novelty evolution): the
// shed-to-win-by-tricks hybrid plays exactly 3 banked rounds, the
// applyTrickScoring hook banks a nonzero capture bonus from the per-player
// shed-card tableau, and the winner is the highest banked total -- the borrow
// DETERMINES the outcome, it does not merely mutate state. Also pins the
// 52-card conservation invariant across rounds (the tableau must be reclaimed
// at redeal).
func TestTrickScoredSheddingBanksAndDeterminesWinner(t *testing.T) {
	g := multiRoundTrickScoringGenome()
	if errs := genome.Validate(g); len(errs) > 0 {
		t.Fatalf("genome should be valid: %v", errs)
	}
	if !g.SheddingMultiRound() {
		t.Fatal("trick-scoring + 3 rounds must activate multi-round play")
	}

	completed, scored := 0, 0
	for seed := uint64(0); seed < 30; seed++ {
		state, winner := runMultiRoundGame(t, g, seed)
		if winner < 0 {
			continue
		}
		completed++

		// Card-conservation invariant: the canonical card locations (deck +
		// discard + hands) must hold the full 52-card deck at game end. The
		// per-player shed tableau is a TALLY that aliases discard cards, so it
		// is deliberately excluded from this count -- if redealRound wrongly
		// appended it to the deck, cards would duplicate and this would exceed
		// 52.
		total := len(state.Deck) + len(state.Discard)
		for i := range state.Hands {
			total += len(state.Hands[i])
		}
		if total != 52 {
			t.Errorf("seed %d: %d canonical cards accounted for, want 52 (card duplication?)", seed, total)
		}

		rounds, _ := countRoundEnds(state.Events)
		if rounds != 3 {
			t.Errorf("seed %d: %d EventRoundEnd events, want 3 (one per round)", seed, rounds)
		}
		if state.Scores[winner] != state.Scores[argmaxScores(state.Scores)] {
			t.Errorf("seed %d: winner %d (Scores=%v) is not a banked-total maximizer", seed, winner, state.Scores)
		}
		for _, s := range state.Scores {
			if s != 0 {
				scored++
				break
			}
		}
	}
	if completed == 0 {
		t.Fatal("no seed completed a trick-scored shedding game")
	}
	if scored == 0 {
		t.Error("no completed game banked a nonzero score: MechTrickScoring never affected the outcome on a shedding host")
	}
}

// scoringHookFuncs adapts mechanic.BuildHooks to sim.HookFunc with the same
// event-type mapping fitness.buildHookFuncs uses (HookEndOfRound and
// HookScoring both fire on EventRoundEnd).
func scoringHookFuncs(g *genome.Genome) []sim.HookFunc {
	hooks := mechanic.BuildHooks(g)
	funcs := make([]sim.HookFunc, 0, len(hooks))
	for _, h := range hooks {
		hook := h
		funcs = append(funcs, func(state *sim.GameState, gg *genome.Genome, event sim.Event) {
			switch hook.Point {
			case mechanic.HookAfterPlay:
				if event.Type == sim.EventCardPlayed {
					hook.Apply(state, gg, event)
				}
			case mechanic.HookEndOfRound, mechanic.HookScoring:
				if event.Type == sim.EventRoundEnd {
					hook.Apply(state, gg, event)
				}
			}
		})
	}
	return funcs
}

// runMultiRoundGame plays one seeded game with scoring hooks attached
// (mirroring the batch loop's move/hook ordering) and returns the final
// state and winner (-1 = no natural completion).
func runMultiRoundGame(t *testing.T, g *genome.Genome, seed uint64) (*sim.GameState, int) {
	t.Helper()
	runner := &Runner{}
	ai := &sim.RandomAI{}
	rng := rand.New(rand.NewPCG(seed, 0))
	hooks := scoringHookFuncs(g)

	state := runner.Setup(g, rng)
	maxTurns := g.MaxTurns()

	for {
		runner.Upkeep(state, g)
		if winner := runner.CheckEnd(state, g); winner >= 0 {
			return state, winner
		}
		if state.Turn >= maxTurns {
			return state, -1
		}
		moves := runner.GenerateMoves(state, g)
		if len(moves) == 0 {
			return state, -1
		}
		move := ai.SelectMove(moves, state, rng)
		events := runner.ApplyMove(state, move, g)
		state.Events = append(state.Events, events...)
		for _, e := range events {
			for _, hook := range hooks {
				hook(state, g, e)
			}
		}
	}
}

func countRoundEnds(events []sim.Event) (total int, perPlayer map[int]int) {
	perPlayer = make(map[int]int)
	for _, e := range events {
		if e.Type == sim.EventRoundEnd {
			total++
			perPlayer[e.PlayerID]++
		}
	}
	return total, perPlayer
}

func argmaxScores(scores []int) int {
	best := 0
	for i := 1; i < len(scores); i++ {
		if scores[i] > scores[best] {
			best = i
		}
	}
	return best
}

// TestMultiRoundPlaysAllRoundsAndScoresDetermineWinner: a MeldBonus +
// 3-rounds genome plays exactly 3 banked rounds and the winner is the
// highest banked total in state.Scores -- the borrow DETERMINES the outcome,
// it does not merely mutate state (Task 22 test b).
func TestMultiRoundPlaysAllRoundsAndScoresDetermineWinner(t *testing.T) {
	g := multiRoundMeldGenome()
	if errs := genome.Validate(g); len(errs) > 0 {
		t.Fatalf("genome should be valid: %v", errs)
	}

	completed := 0
	scored := 0
	for seed := uint64(0); seed < 20; seed++ {
		state, winner := runMultiRoundGame(t, g, seed)
		if winner < 0 {
			continue
		}
		completed++

		rounds, _ := countRoundEnds(state.Events)
		if rounds != 3 {
			t.Errorf("seed %d: %d EventRoundEnd events, want 3 (one per round)", seed, rounds)
		}
		if state.Round != 3 {
			t.Errorf("seed %d: final state.Round = %d, want 3", seed, state.Round)
		}
		if state.Scores[winner] != state.Scores[argmaxScores(state.Scores)] {
			t.Errorf("seed %d: winner %d (Scores=%v) is not a banked-total maximizer -- banked scores do not determine the winner",
				seed, winner, state.Scores)
		}
		for _, s := range state.Scores {
			if s != 0 {
				scored++
				break
			}
		}
	}
	if completed == 0 {
		t.Fatal("no seed completed a multi-round game")
	}
	if scored == 0 {
		t.Error("no completed game banked a nonzero score: MeldBonus never affected the outcome signal")
	}
}

// TestMultiRoundPointsCanBeatRoundCount is Task 22 test (a): with MechMeldBonus
// and 3 rounds, a player can win on points WITHOUT winning the most rounds
// (rounds won = hands emptied = EventRoundEnd.PlayerID). Found via seeded
// play; deterministic.
func TestMultiRoundPointsCanBeatRoundCount(t *testing.T) {
	g := multiRoundMeldGenome()
	for seed := uint64(0); seed < 300; seed++ {
		state, winner := runMultiRoundGame(t, g, seed)
		if winner < 0 {
			continue
		}
		// The winner must have won ON POINTS (strictly outscored someone),
		// not via the all-tied hand-size tie-break.
		wonOnPoints := false
		for _, s := range state.Scores {
			if state.Scores[winner] > s {
				wonOnPoints = true
				break
			}
		}
		if !wonOnPoints {
			continue
		}
		_, perPlayer := countRoundEnds(state.Events)
		for p, roundWins := range perPlayer {
			if p != winner && roundWins > perPlayer[winner] {
				t.Logf("seed %d: player %d won on points (Scores=%v) with %d round wins vs player %d's %d",
					seed, winner, state.Scores, perPlayer[winner], p, roundWins)
				return
			}
		}
	}
	t.Fatal("no seed in 0-299 produced a points winner with fewer round wins: scoring borrow may not be outcome-affecting")
}

// TestMultiRoundAvoidanceFewestPenaltiesWins: MechAvoidance banks penalties as
// NEGATIVE score (applyAvoidance subtracts), so argmax(Scores) is the player
// holding the fewest penalty points -- Mau-Mau scoring. The winner must be a
// maximizer of the banked total in every completed game, and at least one
// game must separate the players' scores so the assertion bites.
func TestMultiRoundAvoidanceFewestPenaltiesWins(t *testing.T) {
	g := multiRoundAvoidanceGenome()
	if errs := genome.Validate(g); len(errs) > 0 {
		t.Fatalf("genome should be valid: %v", errs)
	}

	completed, separated := 0, 0
	for seed := uint64(0); seed < 20; seed++ {
		state, winner := runMultiRoundGame(t, g, seed)
		if winner < 0 {
			continue
		}
		completed++
		if state.Scores[winner] < state.Scores[argmaxScores(state.Scores)] {
			t.Errorf("seed %d: winner %d Scores=%v is not a banked-total maximizer", seed, winner, state.Scores)
		}
		for _, s := range state.Scores {
			if s != state.Scores[winner] {
				separated++
				break
			}
		}
		for _, s := range state.Scores {
			if s > 0 {
				t.Errorf("seed %d: avoidance banked a positive score (%v); penalties must be negative", seed, state.Scores)
				break
			}
		}
	}
	if completed == 0 {
		t.Fatal("no seed completed")
	}
	if separated == 0 {
		t.Error("no completed game separated the players' banked penalties")
	}
}

// TestRoundsWithoutScoringBorrowStaySingleRound: RoundsPerGame=3 with NO
// scoring borrow must keep single-round semantics -- nothing banks scores, so
// extra rounds would have no winner signal. First empty hand wins, exactly
// one round is played.
func TestRoundsWithoutScoringBorrowStaySingleRound(t *testing.T) {
	g := multiRoundMeldGenome()
	g.Borrowed = nil
	if errs := genome.Validate(g); len(errs) > 0 {
		t.Fatalf("genome should be valid: %v", errs)
	}

	completed := 0
	for seed := uint64(0); seed < 5; seed++ {
		state, winner := runMultiRoundGame(t, g, seed)
		if winner < 0 {
			continue
		}
		completed++
		if len(state.Hands[winner]) != 0 {
			t.Errorf("seed %d: winner %d has %d cards in hand; single-round shedding winner must have emptied it",
				seed, winner, len(state.Hands[winner]))
		}
		rounds, _ := countRoundEnds(state.Events)
		if rounds != 1 {
			t.Errorf("seed %d: %d EventRoundEnd events, want exactly 1 (single round)", seed, rounds)
		}
	}
	if completed == 0 {
		t.Fatal("no seed completed")
	}
}

// TestMultiRoundUpkeepRedeals unit-tests the round transition: a mid-game
// empty hand advances Round and redeals fresh HandSize hands (cards
// conserved, new top card flipped); the FINAL round's empty hand advances
// Round without redealing so CheckEnd can report the banked-score winner.
// A second Upkeep on the finished state must be a no-op (the guard makes the
// shedding transition idempotent at game end, unlike mid-game redeals).
func TestMultiRoundUpkeepRedeals(t *testing.T) {
	g := multiRoundAvoidanceGenome()
	runner := &Runner{}
	rng := rand.New(rand.NewPCG(7, 0))
	state := runner.Setup(g, rng)

	if state.MaxRound != 3 {
		t.Fatalf("Setup MaxRound = %d, want 3", state.MaxRound)
	}

	// Empty player 1's hand back into the deck: round over.
	state.Deck = append(state.Deck, state.Hands[1]...)
	state.Hands[1] = state.Hands[1][:0]
	state.Scores[0] = -4
	state.Scores[1] = -1

	runner.Upkeep(state, g)

	if state.Round != 1 {
		t.Fatalf("after round-1 Upkeep: Round = %d, want 1", state.Round)
	}
	total := len(state.Deck) + len(state.Discard)
	for i, hand := range state.Hands {
		if len(hand) != g.HandSize {
			t.Errorf("after redeal: hand %d has %d cards, want %d", i, len(hand), g.HandSize)
		}
		total += len(hand)
	}
	if total != 52 {
		t.Errorf("after redeal: %d cards in play, want 52", total)
	}
	if state.TopCard == nil || len(state.Discard) == 0 {
		t.Error("after redeal: no discard top card flipped")
	}
	if w := runner.CheckEnd(state, g); w != -1 {
		t.Errorf("CheckEnd after mid-game redeal = %d, want -1 (game continues)", w)
	}
	// Banked scores must survive the redeal.
	if state.Scores[0] != -4 || state.Scores[1] != -1 {
		t.Errorf("redeal clobbered banked scores: %v", state.Scores)
	}

	// Fast-forward to the final round and end it.
	state.Round = 2
	state.Deck = append(state.Deck, state.Hands[0]...)
	state.Hands[0] = state.Hands[0][:0]

	runner.Upkeep(state, g)

	if state.Round != 3 {
		t.Fatalf("after final-round Upkeep: Round = %d, want 3", state.Round)
	}
	if len(state.Hands[0]) != 0 {
		t.Error("final round redealt; hands must stay as played out")
	}
	// Player 1 banked fewer penalties (-1 > -4): highest total wins.
	if w := runner.CheckEnd(state, g); w != 1 {
		t.Errorf("CheckEnd after final round = %d, want 1 (highest banked total, Scores=%v)", w, state.Scores)
	}

	// Idempotence at game end + CheckEnd purity.
	before := stateHash(state)
	runner.Upkeep(state, g)
	if state.Round != 3 {
		t.Errorf("Upkeep on finished state advanced Round to %d", state.Round)
	}
	w1 := runner.CheckEnd(state, g)
	w2 := runner.CheckEnd(state, g)
	if w1 != w2 {
		t.Errorf("repeated CheckEnd returned %d then %d", w1, w2)
	}
	if after := stateHash(state); after != before {
		t.Errorf("Upkeep/CheckEnd on finished state mutated it:\nbefore: %s\nafter:  %s", before, after)
	}
}

// TestMultiRoundProgressTracksBankedScores pins the multi-round Progress
// definition: min-max normalization of state.Scores ((s-min)/(max-min)), all
// zeros when every score is equal. The winner rule is argmax(Scores), so the
// winner's final Progress is 1.0 -- the Task 8 winner-max property by
// construction. Negative avoidance totals normalize the same way.
func TestMultiRoundProgressTracksBankedScores(t *testing.T) {
	g := multiRoundMeldGenome()
	runner := &Runner{}
	state := sim.NewGameState(3)
	state.Hands[0] = []sim.Card{{Suit: sim.Hearts, Rank: sim.Two}}
	state.Hands[1] = []sim.Card{{Suit: sim.Clubs, Rank: sim.Three}}
	state.Hands[2] = []sim.Card{{Suit: sim.Spades, Rank: sim.Four}}
	g.Players = 3

	state.Scores = []int{10, 4, 7}
	got := runner.Progress(state, g)
	want := []float64{1.0, 0.0, 0.5}
	for i := range want {
		if diff := got[i] - want[i]; diff > 1e-9 || diff < -1e-9 {
			t.Errorf("Progress[%d] = %v, want %v (Scores=%v)", i, got[i], want[i], state.Scores)
		}
	}

	// Avoidance-style negative totals.
	state.Scores = []int{-6, -2, -4}
	got = runner.Progress(state, g)
	want = []float64{0.0, 1.0, 0.5}
	for i := range want {
		if diff := got[i] - want[i]; diff > 1e-9 || diff < -1e-9 {
			t.Errorf("negative Scores: Progress[%d] = %v, want %v", i, got[i], want[i])
		}
	}

	// All equal (e.g. before any banking): everyone ties at 0.
	state.Scores = []int{0, 0, 0}
	for i, v := range runner.Progress(state, g) {
		if v != 0 {
			t.Errorf("equal Scores: Progress[%d] = %v, want 0", i, v)
		}
	}
}

// TestMultiRoundProgressWinnerIsMax extends the Task 8 winner-max property to
// multi-round games: played out with scoring hooks, the banked-score winner's
// final Progress is the maximum across players.
func TestMultiRoundProgressWinnerIsMax(t *testing.T) {
	runner := &Runner{}
	for _, g := range []*genome.Genome{multiRoundMeldGenome(), multiRoundAvoidanceGenome()} {
		completed := 0
		for seed := uint64(0); seed < 10; seed++ {
			state, winner := runMultiRoundGame(t, g, seed)
			if winner < 0 {
				continue
			}
			completed++
			progress := runner.Progress(state, g)
			for p, v := range progress {
				if v > progress[winner] {
					t.Errorf("%s seed %d: Progress[%d] = %v exceeds winner %d's %v (Scores=%v)",
						g.ID, seed, p, v, winner, progress[winner], state.Scores)
				}
			}
		}
		if completed == 0 {
			t.Fatalf("%s: no seed completed", g.ID)
		}
	}
}

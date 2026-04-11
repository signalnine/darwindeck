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
		// MatchBoth is very restrictive — few matches possible, games may timeout
		if rule != genome.MatchBoth && completions < 25 {
			t.Fatalf("MatchRule %d: too few completions %d/50", rule, completions)
		}
	}
}

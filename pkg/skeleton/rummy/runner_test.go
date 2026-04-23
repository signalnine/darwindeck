package rummy

import (
	"math/rand/v2"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

func runGame(g *genome.Genome, seed uint64) sim.GameResult {
	runner := &Runner{}
	ai := &sim.RandomAI{}
	rng := rand.New(rand.NewPCG(seed, 0))

	state := runner.Setup(g, rng)
	maxTurns := g.MaxTurns()

	for {
		winner := runner.CheckEnd(state, g)
		if winner >= 0 {
			return sim.GameResult{
				Winner: winner,
				Turns:  state.Turn,
				Events: state.Events,
			}
		}

		if state.Turn >= maxTurns {
			// Force end at max turns
			return sim.GameResult{
				Winner: scoreRound(state, g),
				Turns:  state.Turn,
				Events: state.Events,
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

func TestGinRummyCompletes(t *testing.T) {
	g := seeds.GinRummy()
	completions := 0
	for seed := uint64(0); seed < 100; seed++ {
		result := runGame(g, seed)
		if result.Winner >= 0 {
			completions++
		}
		if result.Error == "no_moves" {
			t.Fatalf("seed %d: no moves", seed)
		}
	}
	t.Logf("Gin Rummy: %d/100 completed", completions)
	if completions < 80 {
		t.Fatalf("too few completions: %d/100", completions)
	}
}

func TestKnockRummyCompletes(t *testing.T) {
	g := seeds.KnockRummy()
	completions := 0
	for seed := uint64(0); seed < 100; seed++ {
		result := runGame(g, seed)
		if result.Winner >= 0 {
			completions++
		}
	}
	t.Logf("Knock Rummy: %d/100 completed", completions)
	if completions < 80 {
		t.Fatalf("too few completions: %d/100", completions)
	}
}

func TestRummyDeterminism(t *testing.T) {
	g := seeds.GinRummy()
	r1 := runGame(g, 42)
	r2 := runGame(g, 42)

	if r1.Winner != r2.Winner {
		t.Fatalf("non-deterministic: winner %d vs %d", r1.Winner, r2.Winner)
	}
	if r1.Turns != r2.Turns {
		t.Fatalf("non-deterministic: turns %d vs %d", r1.Turns, r2.Turns)
	}
}

func TestFindSets(t *testing.T) {
	hand := []sim.Card{
		{Suit: sim.Hearts, Rank: sim.King},
		{Suit: sim.Spades, Rank: sim.King},
		{Suit: sim.Clubs, Rank: sim.King},
		{Suit: sim.Hearts, Rank: sim.Five},
	}

	sets := findSets(hand, 3)
	if len(sets) != 1 {
		t.Fatalf("expected 1 set, got %d", len(sets))
	}
	if len(sets[0]) != 3 {
		t.Fatalf("expected set of 3, got %d", len(sets[0]))
	}
}

func TestFindRuns(t *testing.T) {
	hand := []sim.Card{
		{Suit: sim.Hearts, Rank: sim.Five},
		{Suit: sim.Hearts, Rank: sim.Six},
		{Suit: sim.Hearts, Rank: sim.Seven},
		{Suit: sim.Hearts, Rank: sim.Eight},
		{Suit: sim.Spades, Rank: sim.Two},
	}

	runs := findRuns(hand, 3)
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}
	if len(runs[0]) < 3 {
		t.Fatalf("expected run of at least 3, got %d", len(runs[0]))
	}
}

func TestDeadwood(t *testing.T) {
	params := &genome.RummyParams{
		MeldTypes:   genome.MeldBoth,
		MinMeldSize: 3,
	}

	// Hand with one set of kings (0 deadwood for those) + a 5
	hand := []sim.Card{
		{Suit: sim.Hearts, Rank: sim.King},
		{Suit: sim.Spades, Rank: sim.King},
		{Suit: sim.Clubs, Rank: sim.King},
		{Suit: sim.Hearts, Rank: sim.Five},
	}

	dw := calcDeadwood(hand, params)
	if dw != 5 {
		t.Fatalf("expected deadwood 5, got %d", dw)
	}
}

func TestDeadwoodAceWorthOne(t *testing.T) {
	// In rummy, Ace is worth 1 deadwood point, not 10. Regression:
	// earlier code checked `Rank >= Ten` first, so Ace (rank 14) fell
	// through the face-card case and scored 10.
	params := &genome.RummyParams{MeldTypes: genome.MeldBoth, MinMeldSize: 3}

	// Hand with a lone Ace and two unrelated cards — no melds possible.
	// Expected deadwood: 1 (Ace) + 2 (Two) + 5 (Five) = 8.
	hand := []sim.Card{
		{Suit: sim.Hearts, Rank: sim.Ace},
		{Suit: sim.Clubs, Rank: sim.Two},
		{Suit: sim.Spades, Rank: sim.Five},
	}
	if dw := calcDeadwood(hand, params); dw != 8 {
		t.Fatalf("expected deadwood 8 (Ace=1 + 2 + 5), got %d", dw)
	}
}

func TestDeadwoodEmpty(t *testing.T) {
	params := &genome.RummyParams{MeldTypes: genome.MeldBoth, MinMeldSize: 3}
	dw := calcDeadwood(nil, params)
	if dw != 0 {
		t.Fatalf("expected 0 deadwood for empty hand, got %d", dw)
	}
}

func TestAllRummySeedsValid(t *testing.T) {
	seedGames := []*genome.Genome{
		seeds.GinRummy(),
		seeds.KnockRummy(),
	}

	for _, g := range seedGames {
		errs := genome.Validate(g)
		if len(errs) != 0 {
			t.Errorf("%s failed validation: %v", g.ID, errs)
		}
	}
}

func TestPhaseProgression(t *testing.T) {
	// Verify the draw → meld → discard phase cycle
	g := seeds.GinRummy()
	runner := &Runner{}
	rng := rand.New(rand.NewPCG(1, 0))
	state := runner.Setup(g, rng)

	if state.Phase != sim.PhaseDraw {
		t.Fatalf("expected PhaseDraw, got %d", state.Phase)
	}

	// Draw
	moves := runner.GenerateMoves(state, g)
	for _, m := range moves {
		if m.Type == sim.MoveDraw {
			runner.ApplyMove(state, m, g)
			break
		}
	}
	if state.Phase != sim.PhaseMeld {
		t.Fatalf("after draw, expected PhaseMeld, got %d", state.Phase)
	}

	// Pass melding
	runner.ApplyMove(state, sim.Move{Type: sim.MovePass, PlayerID: state.Active}, g)
	if state.Phase != sim.PhaseDiscard {
		t.Fatalf("after pass meld, expected PhaseDiscard, got %d", state.Phase)
	}
}

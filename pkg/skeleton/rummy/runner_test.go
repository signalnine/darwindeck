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

func TestDeadwoodOverlappingSetAndRun(t *testing.T) {
	params := &genome.RummyParams{
		MeldTypes:   genome.MeldBoth,
		MinMeldSize: 3,
	}

	// Hand where 5H can belong to either a set of three 5s or a run 5H-6H-7H.
	// A card can only be used in one meld, so at most one of these can form.
	// Optimal partition: use the run (saves 5+6+7=18), leaves 5D+5C as deadwood (5+5=10).
	// Set-only partition leaves 6H+7H as deadwood (6+7=13).
	// Buggy behavior (over-marking): deadwood = 0.
	hand := []sim.Card{
		{Suit: sim.Hearts, Rank: sim.Five},
		{Suit: sim.Diamonds, Rank: sim.Five},
		{Suit: sim.Clubs, Rank: sim.Five},
		{Suit: sim.Hearts, Rank: sim.Six},
		{Suit: sim.Hearts, Rank: sim.Seven},
	}

	dw := calcDeadwood(hand, params)
	if dw != 10 {
		t.Fatalf("expected deadwood 10 (optimal run partition), got %d", dw)
	}
}

func TestDeadwoodEmpty(t *testing.T) {
	params := &genome.RummyParams{MeldTypes: genome.MeldBoth, MinMeldSize: 3}
	dw := calcDeadwood(nil, params)
	if dw != 0 {
		t.Fatalf("expected 0 deadwood for empty hand, got %d", dw)
	}
}

func TestCardValueAce(t *testing.T) {
	// Rummy convention: Ace is worth 1 point as deadwood (low),
	// even though it's ranked high (14) for run ordering.
	ace := sim.Card{Suit: sim.Hearts, Rank: sim.Ace}
	if got := cardValue(ace); got != 1 {
		t.Fatalf("expected Ace value 1, got %d", got)
	}
}

func TestCardValueFaceCards(t *testing.T) {
	// Face cards (10, J, Q, K) are worth 10 each.
	cases := []sim.Rank{sim.Ten, sim.Jack, sim.Queen, sim.King}
	for _, r := range cases {
		c := sim.Card{Suit: sim.Clubs, Rank: r}
		if got := cardValue(c); got != 10 {
			t.Fatalf("expected %s value 10, got %d", r, got)
		}
	}
}

func TestCardValueNumberCards(t *testing.T) {
	// Number cards (2-9) are worth their face value.
	for r := sim.Two; r <= sim.Nine; r++ {
		c := sim.Card{Suit: sim.Diamonds, Rank: r}
		if got := cardValue(c); got != int(r) {
			t.Fatalf("expected %s value %d, got %d", r, int(r), got)
		}
	}
}

func TestDeadwoodWithAces(t *testing.T) {
	// Hand with unmelded aces should score them at 1 each, not 10.
	params := &genome.RummyParams{MeldTypes: genome.MeldBoth, MinMeldSize: 3}
	hand := []sim.Card{
		{Suit: sim.Hearts, Rank: sim.Ace},
		{Suit: sim.Spades, Rank: sim.Ace},
		{Suit: sim.Hearts, Rank: sim.Five},
	}
	// 1 + 1 + 5 = 7 (not 10 + 10 + 5 = 25)
	if dw := calcDeadwood(hand, params); dw != 7 {
		t.Fatalf("expected deadwood 7 (Ace=1, Ace=1, Five=5), got %d", dw)
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

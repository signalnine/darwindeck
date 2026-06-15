package vying

import (
	"math/rand/v2"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

func vyingGenome() *genome.Genome {
	return &genome.Genome{
		ID: "vying-fixture", Skeleton: genome.Vying, Players: 4, HandSize: 5,
		Vying: &genome.VyingParams{StartingChips: 1000, MinBet: 10, MaxRaises: 3, RoundsPerGame: 12},
	}
}

// playOut runs a full game with the given AI, asserting the playability
// invariant (moves never empty) and chip conservation (stacks + pot is constant)
// at every step. Returns (winner, turns).
func playOut(t *testing.T, g *genome.Genome, ai sim.AIPlayer, seed uint64) (int, int) {
	t.Helper()
	runner := &Runner{}
	rng := rand.New(rand.NewPCG(seed, 0))
	state := runner.Setup(g, rng)
	totalChips := g.Vying.StartingChips * g.Players
	maxTurns := g.MaxTurns()

	for {
		runner.Upkeep(state, g)
		if w := runner.CheckEnd(state, g); w >= 0 {
			return w, state.Turn
		}
		if state.Turn >= maxTurns {
			t.Fatalf("seed %d: hit max turns %d without completing (termination broken)", seed, maxTurns)
		}
		// Chip conservation: nothing is created or destroyed.
		sum := state.Pot
		for _, s := range state.Scores {
			sum += s
		}
		if sum != totalChips {
			t.Fatalf("seed %d turn %d: chips not conserved: stacks+pot=%d, want %d", seed, state.Turn, sum, totalChips)
		}
		moves := runner.GenerateMoves(state, g)
		if len(moves) == 0 {
			t.Fatalf("seed %d turn %d: GenerateMoves empty (playability invariant violated)", seed, state.Turn)
		}
		runner.ApplyMove(state, ai.SelectMove(moves, state, rng), g)
	}
}

// TestVyingCompletesAndConserves: random games run to completion, never stall,
// and never create or destroy chips.
func TestVyingCompletesAndConserves(t *testing.T) {
	g := vyingGenome()
	ai := &sim.RandomAI{}
	for seed := uint64(0); seed < 200; seed++ {
		w, _ := playOut(t, g, ai, seed)
		if w < 0 {
			t.Fatalf("seed %d did not complete", seed)
		}
	}
}

// TestVyingDeterminism: same seed, same outcome.
func TestVyingDeterminism(t *testing.T) {
	g := vyingGenome()
	ai := &sim.RandomAI{}
	for seed := uint64(0); seed < 40; seed++ {
		w1, tn1 := playOut(t, g, ai, seed)
		w2, tn2 := playOut(t, g, ai, seed)
		if w1 != w2 || tn1 != tn2 {
			t.Fatalf("seed %d non-deterministic: (%d,%d) vs (%d,%d)", seed, w1, tn1, w2, tn2)
		}
	}
}

// TestVyingShowdownBestHandWins: at showdown the better poker hand takes the pot.
func TestVyingShowdownBestHandWins(t *testing.T) {
	runner := &Runner{}
	state := sim.NewGameState(2)
	state.Committed = []int{0, 0}
	state.Folded = []bool{false, false}
	state.Pot = 100
	state.Scores = []int{0, 0}
	// Player 0: a flush. Player 1: a pair.
	state.Hands[0] = []sim.Card{c(sim.Two, sim.Clubs), c(sim.Five, sim.Clubs), c(sim.Eight, sim.Clubs), c(sim.Jack, sim.Clubs), c(sim.King, sim.Clubs)}
	state.Hands[1] = []sim.Card{c(sim.Ace, sim.Hearts), c(sim.Ace, sim.Spades), c(sim.Three, sim.Diamonds), c(sim.Six, sim.Hearts), c(sim.Nine, sim.Spades)}
	runner.resolveShowdown(state)
	if state.Scores[0] != 100 || state.Scores[1] != 0 {
		t.Fatalf("flush must beat a pair for the whole pot: scores %v", state.Scores)
	}
}

// TestVyingFoldUncontested: when all but one fold, that player takes the pot
// with no showdown.
func TestVyingFoldUncontested(t *testing.T) {
	runner := &Runner{}
	state := sim.NewGameState(3)
	state.Committed = []int{0, 0, 0}
	state.Folded = []bool{true, false, true} // only player 1 remains
	state.Pot = 60
	state.Scores = []int{0, 0, 0}
	state.Hands[1] = []sim.Card{c(sim.Two, sim.Clubs), c(sim.Three, sim.Hearts)} // weak, irrelevant
	runner.resolveShowdown(state)
	if state.Scores[1] != 60 {
		t.Fatalf("lone non-folded player must take the pot uncontested: scores %v", state.Scores)
	}
}

// TestVyingGreedyBeatsRandom: tight-aggressive greedy (seat 0) should win more
// than its fair share against random opponents -- the skill the metric needs.
func TestVyingGreedyBeatsRandom(t *testing.T) {
	g := vyingGenome()
	runner := &Runner{}
	greedy := &sim.GreedyAI{Scorer: &VyingScorer{}}
	random := &sim.RandomAI{}
	players := []sim.AIPlayer{greedy, random, random, random}
	ai := &sim.PerPlayerAI{Players: players, Fallback: random}
	wins0 := 0
	const games = 300
	for seed := uint64(0); seed < games; seed++ {
		rng := rand.New(rand.NewPCG(seed, 99))
		state := runner.Setup(g, rng)
		maxTurns := g.MaxTurns()
		for state.Turn < maxTurns {
			runner.Upkeep(state, g)
			if w := runner.CheckEnd(state, g); w >= 0 {
				if w == 0 {
					wins0++
				}
				break
			}
			moves := runner.GenerateMoves(state, g)
			runner.ApplyMove(state, ai.SelectMove(moves, state, rng), g)
		}
	}
	rate := float64(wins0) / float64(games)
	t.Logf("greedy seat-0 win rate: %.3f (fair share = %.3f)", rate, 1.0/float64(g.Players))
	if rate <= 1.0/float64(g.Players) {
		t.Fatalf("greedy must beat its fair share %.3f, got %.3f", 1.0/float64(g.Players), rate)
	}
}

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

// TestBeginDealKeepsUndealtStock: the undealt remainder must land in state.Deck.
// Determinize builds its hidden pool from Deck + other hands; an empty Deck
// hands ISMCTS the opponents' exact hole cards (omniscient search).
func TestBeginDealKeepsUndealtStock(t *testing.T) {
	g := vyingGenome()
	runner := &Runner{}
	state := runner.Setup(g, rand.New(rand.NewPCG(7, 0)))

	want := 52 - g.Players*g.HandSize
	if len(state.Deck) != want {
		t.Fatalf("state.Deck has %d cards after the deal, want %d", len(state.Deck), want)
	}
	seen := map[sim.Card]bool{}
	for _, c := range state.Deck {
		seen[c] = true
	}
	for i := 0; i < g.Players; i++ {
		for _, c := range state.Hands[i] {
			if seen[c] {
				t.Fatalf("card %v is in both a hand and the stock", c)
			}
			seen[c] = true
		}
	}
	if len(seen) != 52 {
		t.Fatalf("hands+stock cover %d distinct cards, want 52", len(seen))
	}
}

// TestDeterminizeHidesHoleCards: from seat 0's perspective a determinization
// must resample the opponents' hidden hands from the unknown pool, not hand
// back their real cards. Seat 0's own hand is public to itself and must survive.
func TestDeterminizeHidesHoleCards(t *testing.T) {
	g := vyingGenome()
	runner := &Runner{}
	state := runner.Setup(g, rand.New(rand.NewPCG(7, 0)))
	rng := rand.New(rand.NewPCG(11, 0))

	differed := false
	for i := 0; i < 10 && !differed; i++ {
		det := sim.Determinize(state, 0, rng)
		if len(det.Hands[0]) != len(state.Hands[0]) {
			t.Fatalf("own hand size changed under determinization")
		}
		for j, c := range state.Hands[0] {
			if det.Hands[0][j] != c {
				t.Fatalf("own hand changed under determinization")
			}
		}
		for j, c := range state.Hands[1] {
			if det.Hands[1][j] != c {
				differed = true
				break
			}
		}
	}
	if !differed {
		t.Fatalf("10 determinizations reproduced the opponent's exact hole cards -- hidden pool is empty (omniscient MCTS)")
	}
}

// TestNegativeStackPostsNothing: a VyingScored avoidance penalty can leave a
// stack negative at a deal boundary; the blind clamp must floor at zero rather
// than post a negative blind (negative pot, calls that mint chips).
func TestNegativeStackPostsNothing(t *testing.T) {
	g := vyingGenome()
	runner := &Runner{}
	state := runner.Setup(g, rand.New(rand.NewPCG(7, 0)))

	state.Round = 1 // big blind seat = 1
	state.Scores[1] = -50
	runner.beginDeal(state, g)

	if state.Pot != 0 || state.CurrentBet != 0 || state.Committed[1] != 0 {
		t.Fatalf("negative stack posted a blind: pot=%d bet=%d committed=%d, want all 0",
			state.Pot, state.CurrentBet, state.Committed[1])
	}
	if state.Scores[1] != -50 {
		t.Fatalf("negative stack changed by posting: %d, want -50", state.Scores[1])
	}
}

// TestSetupNilParamsDoesNotPanic: a hand-built genome that bypasses Tier-0
// validation must degrade to defaults, not nil-panic the batch worker (every
// other runner honors the same contract).
func TestSetupNilParamsDoesNotPanic(t *testing.T) {
	g := &genome.Genome{Skeleton: genome.Vying, Players: 3, HandSize: 5}
	r := &Runner{}
	state := r.Setup(g, rand.New(rand.NewPCG(1, 0)))
	if state == nil || len(state.Hands) != 3 {
		t.Fatalf("Setup with nil Vying params must produce a playable state")
	}
	if moves := r.GenerateMoves(state, g); len(moves) == 0 {
		t.Fatal("no legal moves from the default-params setup")
	}
}

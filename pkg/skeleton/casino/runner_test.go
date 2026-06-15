package casino

import (
	"math/rand/v2"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

func casinoGenome() *genome.Genome {
	return &genome.Genome{
		ID: "casino-fixture", Skeleton: genome.Casino, Players: 2, HandSize: 4,
		Casino: &genome.CasinoParams{TableSize: 4, AllowSumCapture: true},
	}
}

func runGame(g *genome.Genome, seed uint64) sim.GameResult {
	runner := &Runner{}
	ai := &sim.RandomAI{}
	rng := rand.New(rand.NewPCG(seed, 0))
	state := runner.Setup(g, rng)
	maxTurns := g.MaxTurns()
	for {
		runner.Upkeep(state, g)
		if w := runner.CheckEnd(state, g); w >= 0 {
			return sim.GameResult{Winner: w, Turns: state.Turn, Events: state.Events}
		}
		if state.Turn >= maxTurns {
			return sim.GameResult{Winner: -1, Turns: state.Turn, Error: "max_turns"}
		}
		moves := runner.GenerateMoves(state, g)
		if len(moves) == 0 {
			return sim.GameResult{Winner: -1, Turns: state.Turn, Error: "no_moves"}
		}
		move := ai.SelectMove(moves, state, rng)
		state.Events = append(state.Events, runner.ApplyMove(state, move, g)...)
	}
}

// TestCasinoCompletes: the game terminates by construction (every turn removes a
// card from a hand; hands refill only from the finite stock).
func TestCasinoCompletes(t *testing.T) {
	g := casinoGenome()
	completed := 0
	for seed := uint64(0); seed < 200; seed++ {
		if runGame(g, seed).Winner >= 0 {
			completed++
		}
	}
	t.Logf("Casino: %d/200 games completed with a winner", completed)
	if completed < 190 {
		t.Fatalf("Casino completes too rarely: %d/200", completed)
	}
}

// TestCasinoMovesNeverEmpty: trail is always legal, so a non-empty active hand
// always yields a move (the playability floor).
func TestCasinoMovesNeverEmpty(t *testing.T) {
	g := casinoGenome()
	runner := &Runner{}
	for seed := uint64(0); seed < 100; seed++ {
		rng := rand.New(rand.NewPCG(seed, 0))
		state := runner.Setup(g, rng)
		for turn := 0; turn < g.MaxTurns(); turn++ {
			runner.Upkeep(state, g)
			if runner.CheckEnd(state, g) >= 0 {
				break
			}
			if len(state.ActiveHand()) == 0 {
				t.Fatalf("seed %d turn %d: active hand empty but game not ended", seed, turn)
			}
			moves := runner.GenerateMoves(state, g)
			if len(moves) == 0 {
				t.Fatalf("seed %d turn %d: zero legal moves with a non-empty hand", seed, turn)
			}
			runner.ApplyMove(state, (&sim.RandomAI{}).SelectMove(moves, state, rng), g)
		}
	}
}

// TestCasinoSumCapture: 5 + 3 on the table is captured by playing an 8 (sum),
// and the move moves all three cards into the player's pile.
func TestCasinoSumCapture(t *testing.T) {
	g := casinoGenome()
	runner := &Runner{}
	state := sim.NewGameState(2)
	state.Active = 0
	eight := sim.Card{Suit: 1, Rank: 8}
	five := sim.Card{Suit: 2, Rank: 5}
	three := sim.Card{Suit: 3, Rank: 3}
	king := sim.Card{Suit: 4, Rank: sim.King}
	state.Hands[0] = []sim.Card{eight}
	state.Hands[1] = []sim.Card{{Suit: 1, Rank: 2}}
	state.Discard = []sim.Card{five, three, king}
	state.TrickLeader = -1

	moves := runner.GenerateMoves(state, g)
	var capture *sim.Move
	for i := range moves {
		if moves[i].Type == sim.MoveCapture {
			m := moves[i]
			capture = &m
			break
		}
	}
	if capture == nil {
		t.Fatalf("expected a sum capture (8 = 5+3), got moves %+v", moves)
	}
	if len(capture.Cards) != 3 {
		t.Fatalf("sum capture should take the 8 + 5 + 3 (3 cards), got %d", len(capture.Cards))
	}
	runner.ApplyMove(state, *capture, g)
	if len(state.Hands[0]) != 0 {
		t.Fatalf("played card should leave the hand, hand=%d", len(state.Hands[0]))
	}
	if len(state.Tableau[0]) != 3 {
		t.Fatalf("captured pile should hold 8,5,3 (3 cards), got %d", len(state.Tableau[0]))
	}
	if len(state.Discard) != 1 || state.Discard[0] != king {
		t.Fatalf("only the King should remain on the table, got %+v", state.Discard)
	}
	if state.TrickLeader != 0 {
		t.Fatalf("last-capturer should be player 0, got %d", state.TrickLeader)
	}
}

// TestCasinoDeterminism guards that captureGroups' enumeration order (built from
// sorted table cards) is deterministic.
func TestCasinoDeterminism(t *testing.T) {
	g := casinoGenome()
	for seed := uint64(0); seed < 30; seed++ {
		r1, r2 := runGame(g, seed), runGame(g, seed)
		if r1.Winner != r2.Winner || r1.Turns != r2.Turns {
			t.Fatalf("seed %d non-deterministic: (%d,%d) vs (%d,%d)", seed, r1.Winner, r1.Turns, r2.Winner, r2.Turns)
		}
	}
}

// TestCasinoSeedValidates: the seed genome passes Tier-0 validation.
func TestCasinoSeedValidates(t *testing.T) {
	if errs := genome.Validate(casinoGenome()); len(errs) > 0 {
		t.Fatalf("casino seed has Tier-0 violations: %v", errs)
	}
}

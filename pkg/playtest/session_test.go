package playtest

import (
	"math/rand/v2"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

// stubRunner advances the active player on every move, mirroring how real
// shedding/trick-taking runners mutate state.Active inside ApplyMove.
type stubRunner struct{}

func (stubRunner) Setup(g *genome.Genome, rng *rand.Rand) *sim.GameState { return nil }
func (stubRunner) Upkeep(state *sim.GameState, g *genome.Genome)         {}
func (stubRunner) GenerateMoves(state *sim.GameState, g *genome.Genome) []sim.Move {
	return nil
}
func (stubRunner) ApplyMove(state *sim.GameState, move sim.Move, g *genome.Genome) []sim.Event {
	state.NextPlayer()
	return nil
}
func (stubRunner) CheckEnd(state *sim.GameState, g *genome.Genome) int { return -1 }
func (stubRunner) Progress(state *sim.GameState, g *genome.Genome) []float64 {
	return nil
}

// stubAI returns a fixed move regardless of state.
type stubAI struct{ move sim.Move }

func (s stubAI) SelectMove(moves []sim.Move, state *sim.GameState, rng *rand.Rand) sim.Move {
	return s.move
}

// TestAITurnReportsActorBeforeMove verifies that aiTurn attributes the action
// to the player who actually moved, not the next player. The bug was that
// aiTurn read s.State.Active AFTER ApplyMove, which had already advanced it.
func TestAITurnReportsActorBeforeMove(t *testing.T) {
	state := sim.NewGameState(3)
	state.Active = 1
	state.Hands[0] = []sim.Card{}
	state.Hands[1] = []sim.Card{}
	state.Hands[2] = []sim.Card{}

	move := sim.Move{Type: sim.MovePass, PlayerID: 1}

	s := &Session{
		Genome:  &genome.Genome{},
		Runner:  stubRunner{},
		AI:      stubAI{move: move},
		State:   state,
		RNG:     rand.New(rand.NewPCG(1, 2)),
		HumanID: 0,
	}

	preActive := s.State.Active
	actor := s.aiTurn([]sim.Move{move})

	if actor != preActive {
		t.Fatalf("aiTurn reported actor=%d, want %d (the player who moved)", actor, preActive)
	}
	if s.State.Active == preActive {
		t.Fatalf("expected ApplyMove to advance state.Active away from %d, but it did not — test setup broken", preActive)
	}
}

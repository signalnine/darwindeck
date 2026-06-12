package playtest

import (
	"bufio"
	"math/rand/v2"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
	"github.com/darwindeck/darwindeck/pkg/sim"
	"github.com/darwindeck/darwindeck/pkg/skeleton/rummy"
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

// TestNewMCTSAIIsFullyWired pins the dangerous failure mode of the mcts
// difficulty: sim.MCTSAI with a nil Runner or Genome silently degrades to
// uniform random (a deliberate batch-safety fallback), so a half-wired
// constructor would ship a "mcts" opponent that actually plays randomly.
func TestNewMCTSAIIsFullyWired(t *testing.T) {
	g := seeds.GinRummy()
	runner := &rummy.Runner{}

	ai := NewMCTSAI(g, runner)

	if ai.Runner == nil {
		t.Fatal("NewMCTSAI returned nil Runner: MCTSAI would degrade to random play")
	}
	if ai.Genome == nil {
		t.Fatal("NewMCTSAI returned nil Genome: MCTSAI would degrade to random play")
	}
}

// timedAI wraps an AIPlayer to count decisions and total decision latency.
type timedAI struct {
	inner sim.AIPlayer
	calls int
	total time.Duration
}

func (a *timedAI) SelectMove(moves []sim.Move, state *sim.GameState, rng *rand.Rand) sim.Move {
	start := time.Now()
	mv := a.inner.SelectMove(moves, state, rng)
	a.total += time.Since(start)
	a.calls++
	return mv
}

// TestMCTSSessionCompletesGame runs a fully scripted playtest session against
// the mcts difficulty: the "human" (seat 0) always picks move 1 from stdin
// while the MCTS opponent plays the other seat. Gin rummy is used because the
// rummy runner is the most expensive movegen MCTS can face — this doubles as
// the interactive-latency check (sub-second per AI move, Task 21).
func TestMCTSSessionCompletesGame(t *testing.T) {
	g := seeds.GinRummy()
	runner := &rummy.Runner{}
	ai := &timedAI{inner: NewMCTSAI(g, runner)}

	s := NewSession(g, runner, ai, 7)
	// One stdin line per human decision; a rummy game is bounded by
	// MaxTurns (208) at a few moves per turn, so 4096 lines can never run
	// out (EOF would os.Exit the test binary).
	s.Scanner = bufio.NewScanner(strings.NewReader(strings.Repeat("1\n", 4096)))

	// Silence the interactive transcript for the duration of Run.
	oldStdout := os.Stdout
	devnull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open %s: %v", os.DevNull, err)
	}
	defer func() {
		os.Stdout = oldStdout
		devnull.Close()
	}()
	os.Stdout = devnull

	s.Run()

	os.Stdout = oldStdout

	if s.State == nil {
		t.Fatal("Run never set up a game state")
	}
	if s.State.Turn == 0 {
		t.Fatal("game made no progress")
	}
	// Run returns on exactly three paths: winner, max-turns cap, or the
	// no-legal-moves stuck path. CheckEnd is pure (audit Wave B), so
	// re-querying it distinguishes a real ending from a stuck session.
	winner := runner.CheckEnd(s.State, g)
	if winner < 0 && s.State.Turn < g.MaxTurns() {
		t.Fatalf("session got stuck at turn %d: no winner and below max turns %d",
			s.State.Turn, g.MaxTurns())
	}
	if ai.calls == 0 {
		t.Fatal("MCTS opponent was never asked for a move")
	}

	avg := ai.total / time.Duration(ai.calls)
	t.Logf("mcts difficulty: %d AI moves, avg %v per move (winner=%d, turns=%d)",
		ai.calls, avg, winner, s.State.Turn)
	// Interactive budget: sub-second per move. Measured ~10-30ms on the
	// worst-case skeleton, so 1s gives wide headroom against slow CI.
	if avg >= time.Second {
		t.Fatalf("mcts difficulty too slow for interactive play: avg %v per move (budget < 1s)", avg)
	}
}

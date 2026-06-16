package webplay

import (
	"testing"

	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

// playToEnd drives a session to a terminal state by always submitting the first
// legal move for the human seat (a legal move by construction). Termination is
// guaranteed by the max-turns cap in advance() regardless of the choice.
func playToEnd(t *testing.T, g *genome.Genome) *WebSession {
	t.Helper()
	runner := fitness.GetRunner(g)
	if runner == nil {
		t.Fatalf("no runner for skeleton %s", g.Skeleton)
	}
	ws := NewWebSession("test", g, runner, &sim.RandomAI{}, 12345, "random", "test.json")
	// The cap is a runaway guard; a healthy game ends far sooner (winner or
	// max-turns). It must exceed any single game's human decisions.
	for i := 0; i < 100000; i++ {
		ws.mu.Lock()
		st := ws.status
		ws.mu.Unlock()
		if st != StatusHumanTurn {
			break
		}
		if err := ws.submitMove(0); err != nil {
			t.Fatalf("submitMove(0): %v", err)
		}
	}
	return ws
}

// Every classic seed must play start-to-finish through the web step machine and
// land in a terminal state -- exercising all six skeleton runners (and the
// betting/forced-turn auto-advance) over the HTTP-shaped loop.
func TestWebSessionPlaysEverySeedToTermination(t *testing.T) {
	for _, g := range seeds.All() {
		g := g
		t.Run(g.ID, func(t *testing.T) {
			ws := playToEnd(t, g)
			if ws.status != StatusGameOver && ws.status != StatusStuck {
				t.Fatalf("%s ended in non-terminal status %q", g.ID, ws.status)
			}
			v := ws.view(false)
			if v.Status != ws.status {
				t.Errorf("view status %q != session status %q", v.Status, ws.status)
			}
			// A game_over view must name a winner field (>=0 seat or -1 no-winner).
			if ws.status == StatusGameOver && v.WinnerLabel == "" {
				t.Errorf("%s game_over view has empty WinnerLabel", g.ID)
			}
		})
	}
}

// submitMove must reject out-of-range indices and any submission once the game
// is no longer awaiting a human move -- the server turns these into 4xx, so they
// must never panic or mutate state.
func TestSubmitMoveValidation(t *testing.T) {
	g := firstShedding(t)
	runner := fitness.GetRunner(g)
	ws := NewWebSession("test", g, runner, &sim.RandomAI{}, 99, "random", "test.json")

	if ws.status != StatusHumanTurn {
		t.Fatalf("expected first decision to be the human's, got %q", ws.status)
	}
	if err := ws.submitMove(-1); err == nil {
		t.Error("expected error for index -1")
	}
	if err := ws.submitMove(1 << 20); err == nil {
		t.Error("expected error for out-of-range index")
	}

	// Drive to a terminal state, then any submission must error.
	for i := 0; i < 100000; i++ {
		ws.mu.Lock()
		st := ws.status
		ws.mu.Unlock()
		if st != StatusHumanTurn {
			break
		}
		_ = ws.submitMove(0)
	}
	if err := ws.submitMove(0); err == nil {
		t.Errorf("expected error submitting after terminal status %q", ws.status)
	}
}

func firstShedding(t *testing.T) *genome.Genome {
	t.Helper()
	for _, g := range seeds.All() {
		if g.Skeleton == genome.Shedding {
			return g
		}
	}
	t.Fatal("no shedding seed found")
	return nil
}

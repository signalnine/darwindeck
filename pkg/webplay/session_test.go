package webplay

import (
	"encoding/json"
	"fmt"
	"strings"
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
		ver := ws.moveVersion
		ws.mu.Unlock()
		if st != StatusHumanTurn {
			break
		}
		if err := ws.submitMove(0, ver); err != nil {
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
	if err := ws.submitMove(-1, ws.moveVersion); err == nil {
		t.Error("expected error for index -1")
	}
	if err := ws.submitMove(1<<20, ws.moveVersion); err == nil {
		t.Error("expected error for out-of-range index")
	}
	// A stale version (double-click against a regenerated list) is a distinct,
	// retryable rejection: errStaleMove, no state mutation.
	if err := ws.submitMove(0, ws.moveVersion-1); err != errStaleMove {
		t.Errorf("expected errStaleMove for stale version, got %v", err)
	}
	if ws.status != StatusHumanTurn {
		t.Fatalf("stale submission mutated session status to %q", ws.status)
	}

	// Drive to a terminal state, then any submission must error.
	for i := 0; i < 100000; i++ {
		ws.mu.Lock()
		st := ws.status
		ver := ws.moveVersion
		ws.mu.Unlock()
		if st != StatusHumanTurn {
			break
		}
		_ = ws.submitMove(0, ver)
	}
	if err := ws.submitMove(0, ws.moveVersion); err == nil {
		t.Errorf("expected error submitting after terminal status %q", ws.status)
	}
}

// The casino "discard" IS the face-up table you capture from, and the captured
// piles (state.Tableau) are the win condition: the view must surface both or
// the game is unplayable by sight (ported from the CLI printState casino fix).
func TestCasinoViewExposesTableAndCaptured(t *testing.T) {
	g := seedWithSkeleton(t, genome.Casino)
	runner := fitness.GetRunner(g)
	ws := NewWebSession("test", g, runner, &sim.RandomAI{}, 7, "random", "test.json")

	ws.mu.Lock()
	ws.state.Discard = []sim.Card{{Suit: sim.Hearts, Rank: sim.Five}, {Suit: sim.Spades, Rank: sim.Nine}}
	ws.state.Tableau[HumanSeat] = []sim.Card{
		{Suit: sim.Clubs, Rank: sim.Two}, {Suit: sim.Clubs, Rank: sim.Three}, {Suit: sim.Clubs, Rank: sim.Four},
	}
	ws.state.Tableau[1] = []sim.Card{{Suit: sim.Diamonds, Rank: sim.Ace}}
	v := ws.view(false)
	ws.mu.Unlock()

	want := []string{"5H", "9S"}
	if len(v.Table.TableCards) != len(want) || v.Table.TableCards[0] != want[0] || v.Table.TableCards[1] != want[1] {
		t.Errorf("TableCards = %v, want %v", v.Table.TableCards, want)
	}
	if v.YourCaptured != 3 {
		t.Errorf("YourCaptured = %d, want 3", v.YourCaptured)
	}
	if len(v.Opponents) == 0 || v.Opponents[0].Captured != 1 {
		t.Errorf("opponents = %+v, want first opponent Captured=1", v.Opponents)
	}
}

// Non-casino skeletons keep the discard pile hidden (count only) -- exposing
// its contents would leak information the game's rules don't grant.
func TestNonCasinoViewOmitsTableCards(t *testing.T) {
	g := firstShedding(t)
	runner := fitness.GetRunner(g)
	ws := NewWebSession("test", g, runner, &sim.RandomAI{}, 7, "random", "test.json")

	ws.mu.Lock()
	v := ws.view(false)
	ws.mu.Unlock()

	if v.Table.TableCards != nil {
		t.Errorf("shedding view leaked TableCards = %v", v.Table.TableCards)
	}
	if v.YourCaptured != 0 {
		t.Errorf("shedding view set YourCaptured = %d", v.YourCaptured)
	}
}

// vyingAtHumanDecision builds a poker session and pins the betting state to a
// human decision with the given current bet and chips already committed by the
// human (the opponent has matched the bet). The legal moves are regenerated by
// the real runner, so the view is exactly what the server would send there.
func vyingAtHumanDecision(t *testing.T, currentBet, humanCommitted int) *WebSession {
	t.Helper()
	g := seedWithSkeleton(t, genome.Vying)
	ws := NewWebSession("test", g, fitness.GetRunner(g), &sim.RandomAI{}, 0, "random", "test.json")
	st := ws.state
	for i := range st.Folded {
		st.Folded[i] = false
	}
	for i := range st.Committed {
		st.Committed[i] = currentBet
	}
	st.Committed[HumanSeat] = humanCommitted
	st.CurrentBet = currentBet
	st.RaiseCount = 0
	st.Active = HumanSeat
	ws.status = StatusHumanTurn
	ws.legalMoves = ws.runner.GenerateMoves(st, g)
	return ws
}

func moveLabels(v View) []string {
	labels := make([]string, len(v.LegalMoves))
	for i, m := range v.LegalMoves {
		labels[i] = m.Label
	}
	return labels
}

// The poker table's "to call" must be what the HUMAN still owes, not the full
// current bet. As the big blind with the bet matched around (current bet 10,
// already in for 10) the legal moves are Check / Raise -- there is nothing to
// call -- yet the page rendered "to call 10" from table.currentBet.
func TestVyingViewToCallIsWhatHumanOwes(t *testing.T) {
	g := seedWithSkeleton(t, genome.Vying)
	bet := g.Vying.MinBet

	t.Run("bet already matched", func(t *testing.T) {
		ws := vyingAtHumanDecision(t, bet, bet)
		v := ws.view(false)
		if got := moveLabels(v); len(got) == 0 || got[0] != "Check" {
			t.Fatalf("precondition: want a Check-first move list, got %v", got)
		}
		if v.Table.CurrentBet != bet {
			t.Fatalf("precondition: table.currentBet = %d, want %d", v.Table.CurrentBet, bet)
		}
		if v.YourToCall != 0 {
			t.Errorf("YourToCall = %d with moves %v, want 0 (nothing owed)", v.YourToCall, moveLabels(v))
		}
		if data, _ := json.Marshal(v); strings.Contains(string(data), `"yourToCall"`) {
			t.Errorf("a zero owed amount should be omitted from the JSON: %s", data)
		}
	})

	t.Run("facing a raise", func(t *testing.T) {
		ws := vyingAtHumanDecision(t, 2*bet, bet)
		v := ws.view(false)
		if v.YourToCall != bet {
			t.Errorf("YourToCall = %d, want %d (current bet %d - %d already in)", v.YourToCall, bet, 2*bet, bet)
		}
		wantCall := fmt.Sprintf("Call %d", bet)
		found := false
		for _, l := range moveLabels(v) {
			found = found || l == wantCall
		}
		if !found {
			t.Errorf("moves %v: want a %q button agreeing with YourToCall", moveLabels(v), wantCall)
		}
		if data, _ := json.Marshal(v); !strings.Contains(string(data), fmt.Sprintf(`"yourToCall":%d`, bet)) {
			t.Errorf("view JSON lacks yourToCall:%d: %s", bet, data)
		}
	})

	t.Run("facing the full bet", func(t *testing.T) {
		ws := vyingAtHumanDecision(t, bet, 0)
		if v := ws.view(false); v.YourToCall != bet {
			t.Errorf("YourToCall = %d, want %d", v.YourToCall, bet)
		}
	})
}

// The owed amount is the human's own, never another seat's, and is only
// reported while the human has a decision to make: a finished game whose last
// hand the human folded must not keep saying "to call N".
func TestVyingViewToCallIgnoresOtherSeats(t *testing.T) {
	g := seedWithSkeleton(t, genome.Vying)
	bet := g.Vying.MinBet

	t.Run("opponent owes, human does not", func(t *testing.T) {
		ws := vyingAtHumanDecision(t, 2*bet, 2*bet)
		ws.state.Committed[1] = 0 // seat 1 owes the full bet
		ws.state.Active = 1       // and is the active seat
		if v := ws.view(false); v.YourToCall != 0 {
			t.Errorf("YourToCall = %d, want 0: that is seat 1's debt, not the human's", v.YourToCall)
		}
	})

	t.Run("game over", func(t *testing.T) {
		ws := vyingAtHumanDecision(t, 2*bet, bet) // human would owe bet...
		ws.status = StatusGameOver                // ...but the game has ended
		ws.legalMoves = nil
		if v := ws.view(false); v.YourToCall != 0 {
			t.Errorf("YourToCall = %d on a finished game, want 0", v.YourToCall)
		}
	})
}

// Skeletons without betting never carry the field.
func TestNonVyingViewOmitsToCall(t *testing.T) {
	g := firstShedding(t)
	ws := NewWebSession("test", g, fitness.GetRunner(g), &sim.RandomAI{}, 7, "random", "test.json")
	v := ws.view(false)
	if v.YourToCall != 0 {
		t.Errorf("shedding view set YourToCall = %d", v.YourToCall)
	}
	if data, _ := json.Marshal(v); strings.Contains(string(data), `"yourToCall"`) {
		t.Errorf("shedding view JSON carries yourToCall: %s", data)
	}
}

func firstShedding(t *testing.T) *genome.Genome {
	t.Helper()
	return seedWithSkeleton(t, genome.Shedding)
}

func seedWithSkeleton(t *testing.T, sk genome.SkeletonType) *genome.Genome {
	t.Helper()
	for _, g := range seeds.All() {
		if g.Skeleton == sk {
			return g
		}
	}
	t.Fatalf("no %s seed found", sk)
	return nil
}

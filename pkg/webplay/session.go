// Package webplay serves an evolved DarwinDeck game over HTTP so a human can
// play it in a browser. It wraps the same skeleton runner + borrowed-mechanic
// hooks the fitness pipeline uses (mechanic.HooksFor), so the browser game is
// byte-for-byte the game evolution scored -- the playtest-parity contract
// (audit Task 24) carried over from the CLI session.
//
// The CLI playtest.Session blocks on stdin inside one Run() loop. The web flow
// can't block: each HTTP request must return. WebSession therefore splits that
// loop into a re-entrant step machine -- advance() runs the shared
// Upkeep->CheckEnd->GenerateMoves loop, auto-playing every AI/forced seat, and
// stops at the next human decision (or game end). submitMove() applies one human
// move and advances again.
package webplay

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"sync"
	"time"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/mechanic"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

// HumanSeat is the seat the browser player controls. Fixed to 0 to match the
// CLI session (playtest.Session.HumanID), so web and CLI playtests of the same
// genome see the same seat.
const HumanSeat = 0

// Session status values surfaced to the client.
const (
	StatusHumanTurn = "human_turn" // waiting on the human to pick a legal move
	StatusGameOver  = "game_over"  // a seat won, or max-turns reached (winner -1)
	StatusStuck     = "stuck"      // no legal moves (a broken skeleton; should be rare)
)

// WebSession is one in-progress browser game. All mutation goes through advance
// and submitMove under mu, so concurrent requests on the same session serialize.
type WebSession struct {
	mu sync.Mutex

	ID         string // opaque session token (crypto/rand), the client's handle
	Genome     *genome.Genome
	GenomePath string // source file, recorded in the ratings log (never client-supplied)
	Difficulty string // random | greedy | mcts, recorded in the ratings log
	Seed       uint64

	runner   sim.GenericRunner
	ai       sim.AIPlayer
	state    *sim.GameState
	rng      *rand.Rand
	hooks    []sim.HookFunc
	maxTurns int

	status      string
	winner      int        // valid when status == game_over; -1 = no winner (max turns / draw)
	legalMoves  []sim.Move // the canonical move list for the current human decision; submit by index
	moveVersion int        // bumped every time legalMoves is regenerated; the client must echo it
	log         []string   // append-only narration; the view sends the tail
	rules       string     // rendered rulebook markdown; set by the server at creation
	rated       bool       // set by the first /api/rate; later calls get 409 (one rating per game)
	lastActive  time.Time  // last state/move/rate touch; the janitor evicts idle sessions
}

// errStaleMove distinguishes a moveVersion mismatch (client raced its own move,
// e.g. a double-click against a regenerated move list) from plain bad input, so
// the handler can answer 409 and the client can just refresh state.
var errStaleMove = errors.New("stale move version (the move list changed); refresh state")

// NewWebSession sets up a game and advances to the first human decision. runner,
// ai and the genome's hooks are wired exactly as the CLI session wires them.
func NewWebSession(id string, g *genome.Genome, runner sim.GenericRunner, ai sim.AIPlayer, seed uint64, difficulty, genomePath string) *WebSession {
	rng := rand.New(rand.NewPCG(seed, 0))
	ws := &WebSession{
		ID:         id,
		Genome:     g,
		GenomePath: genomePath,
		Difficulty: difficulty,
		Seed:       seed,
		runner:     runner,
		ai:         ai,
		rng:        rng,
		hooks:      mechanic.HooksFor(g),
		maxTurns:   g.MaxTurns(),
	}
	ws.state = runner.Setup(g, rng)
	ws.advance()
	return ws
}

// advance runs the shared game loop, auto-playing every AI/forced seat, and
// returns when it reaches the human's decision or the game ends. Mirrors
// playtest.Session.Run's loop body (Upkeep -> CheckEnd -> max-turns -> Generate
// -> apply) so a browser game is identical to a simulated one. Caller holds mu
// (or the session is not yet shared, as in NewWebSession).
func (ws *WebSession) advance() {
	// Absolute guard against a pathological non-terminating skeleton: the batch
	// loop uses the same shape (pkg/sim/batch.go). Turn-based termination is the
	// primary cap; this only bounds AI/forced turns that don't advance Turn.
	iterCap := 100 * (ws.maxTurns + 1)
	if iterCap < 10000 {
		iterCap = 10000
	}
	iter := 0
	for {
		ws.runner.Upkeep(ws.state, ws.Genome)

		if w := ws.runner.CheckEnd(ws.state, ws.Genome); w >= 0 {
			ws.status = StatusGameOver
			ws.winner = w
			ws.legalMoves = nil
			ws.logf(ws.winnerLine(w))
			return
		}
		if ws.state.Turn >= ws.maxTurns {
			ws.status = StatusGameOver
			ws.winner = -1
			ws.legalMoves = nil
			ws.logf("Game ended at the turn limit (%d) with no winner.", ws.maxTurns)
			return
		}
		// Runaway guard, positioned exactly as in pkg/sim/batch.go (after the
		// max-turns cap, before move generation) so a hit exits in the same
		// state the simulator would -- preserving the parity contract on the
		// error path too.
		if iter >= iterCap {
			ws.status = StatusStuck
			ws.legalMoves = nil
			ws.logf("Game exceeded its iteration budget -- stopping.")
			return
		}
		iter++

		moves := ws.runner.GenerateMoves(ws.state, ws.Genome)
		if len(moves) == 0 {
			ws.status = StatusStuck
			ws.legalMoves = nil
			ws.logf("No legal moves -- game stuck.")
			return
		}

		if ws.state.Active == HumanSeat {
			ws.status = StatusHumanTurn
			ws.legalMoves = moves
			ws.moveVersion++
			return
		}

		// AI / non-human seat: pick, apply, fire hooks, narrate.
		actor := ws.state.Active
		mv := ws.ai.SelectMove(moves, ws.state, ws.rng)
		events := ws.applyMove(mv)
		ws.logf("Player %d: %s%s", actor, moveLabel(mv, ws.state, ws.Genome), eventSuffix(events))
	}
}

// submitMove applies the human's chosen move (by index into the move list shown
// to them) and advances to the next decision. version must match the
// moveVersion the client saw with that list -- a bare index against a
// regenerated list would apply an unintended move (double-click race); a
// mismatch returns errStaleMove (handler: 409). Returns an error the handler
// turns into a 4xx; it never panics on bad input.
func (ws *WebSession) submitMove(index, version int) error {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	if ws.status != StatusHumanTurn {
		return fmt.Errorf("not awaiting a move (status %q)", ws.status)
	}
	if version != ws.moveVersion {
		return errStaleMove
	}
	if index < 0 || index >= len(ws.legalMoves) {
		return fmt.Errorf("move index %d out of range [0,%d)", index, len(ws.legalMoves))
	}
	mv := ws.legalMoves[index]
	events := ws.applyMove(mv)
	ws.logf("You: %s%s", moveLabel(mv, ws.state, ws.Genome), eventSuffix(events))
	ws.legalMoves = nil
	ws.advance()
	return nil
}

// applyMove applies one move and fires the borrowed-mechanic hooks on its
// events, exactly as playtest.Session.afterMove does (hook parity).
func (ws *WebSession) applyMove(mv sim.Move) []sim.Event {
	events := ws.runner.ApplyMove(ws.state, mv, ws.Genome)
	ws.state.Events = append(ws.state.Events, events...)
	for _, e := range events {
		for _, hook := range ws.hooks {
			hook(ws.state, ws.Genome, e)
		}
	}
	return events
}

// touch records client activity; the server calls it on every session lookup
// (state/move/rate) so the janitor's idle clock resets while a game is played.
func (ws *WebSession) touch(t time.Time) {
	ws.mu.Lock()
	ws.lastActive = t
	ws.mu.Unlock()
}

func (ws *WebSession) logf(format string, args ...interface{}) {
	ws.log = append(ws.log, fmt.Sprintf(format, args...))
}

func (ws *WebSession) winnerLine(w int) string {
	if w == HumanSeat {
		return "You win!"
	}
	return fmt.Sprintf("Player %d wins.", w)
}

// eventSuffix renders notable events (special-card effects, trick wins) as a
// short trailing annotation on a move log line.
func eventSuffix(events []sim.Event) string {
	var parts []string
	for _, e := range events {
		switch e.Type {
		case sim.EventSpecialTriggered:
			if e.Detail != "" {
				parts = append(parts, e.Detail)
			}
		case sim.EventTrickWon:
			parts = append(parts, fmt.Sprintf("wins trick"))
		case sim.EventMeldLaid:
			parts = append(parts, "melds")
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return " [" + strings.Join(parts, ", ") + "]"
}

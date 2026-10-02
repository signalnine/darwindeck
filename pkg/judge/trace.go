package judge

import (
	"fmt"
	"math/rand/v2"
	"strings"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

// The dossier's sample traces come from a judge-local game loop instead of
// sim.RunBatch's event log, because the event log does not contain every
// decision: a move that changes no cards emits no event. The unscored vying
// runner emits NO events at all (a whole game rendered as an empty code
// block), and a climbing pass is invisible (the player who kept the lead
// appeared to act several times in a row -- the "long uninterrupted
// single-player run" the rubric reads as degenerate). sim.GameResult exposes
// events and TurnRecords but not the moves themselves, and adding runner
// events would feed the fitness metrics (attack events, trace-derived stats),
// so the moves are recorded here, in a loop that is pinned winner-for-winner
// to the production engine (TestPlayTracedMatchesRunBatch).

// moveNames labels a move that needs its own trace line. MoveKnock renders as
// DECLARE_OUT for the same reason neutralizeDetail rewrites "knock".
var moveNames = map[sim.MoveType]string{
	sim.MovePlay:    "PLAY",
	sim.MoveDraw:    "DRAW",
	sim.MovePass:    "PASS",
	sim.MoveKnock:   "DECLARE_OUT",
	sim.MoveMeld:    "MELD",
	sim.MoveDiscard: "DISCARD",
	sim.MoveCapture: "CAPTURE",
	sim.MoveCheck:   "CHECK",
	sim.MoveCall:    "CALL",
	sim.MoveRaise:   "RAISE",
	sim.MoveFold:    "FOLD",
	sim.MoveBid:     "BID",
}

// tracedGame is one instrumented game: its outcome and its rendered trace
// lines (without the "turn N:" numbering, which renderTraceLines adds).
type tracedGame struct {
	winner int // -1 when the game did not reach a winner (turn cap / no moves)
	lines  []string
}

// playTraced plays one game to maxTurns and records a trace line for every
// decision. Its control flow is sim.runSingleGame's, step for step (Upkeep ->
// CheckEnd -> turn cap -> iteration cap -> GenerateMoves -> SelectMove ->
// ApplyMove -> hooks), with the same iteration guard, so at a given rng and
// hook set it plays the identical game.
func playTraced(g *genome.Genome, runner sim.GenericRunner, ai sim.AIPlayer, rng *rand.Rand, maxTurns int, hooks ...sim.HookFunc) tracedGame {
	game := tracedGame{winner: -1}
	state := runner.Setup(g, rng)

	iterCap := (maxTurns + 1) * 100
	if iterCap < 10000 {
		iterCap = 10000
	}
	iter := 0
	round := state.Round

	for {
		runner.Upkeep(state, g)

		// A betting deal resolves in Upkeep with no event (the pot is awarded
		// and the next hand dealt), so without a marker a vying trace is one
		// unbroken stream of bets with no way to see where a deal ends or who
		// took the pot. Emitted only for hosts that track a pot.
		if state.Round != round {
			round = state.Round
			if line, ok := dealEndLine(state); ok {
				game.lines = append(game.lines, line)
			}
		}

		if winner := runner.CheckEnd(state, g); winner >= 0 {
			game.winner = winner
			return game
		}
		if state.Turn >= maxTurns || iter >= iterCap {
			return game
		}
		iter++

		moves := runner.GenerateMoves(state, g)
		if len(moves) == 0 {
			return game
		}

		move := ai.SelectMove(moves, state, rng)
		mover := state.Active

		events := runner.ApplyMove(state, move, g)
		state.Events = append(state.Events, events...)

		if needsMoveLine(len(moves), state.Active == mover, events) {
			game.lines = append(game.lines, moveLine(mover, move))
		}
		for _, e := range events {
			game.lines = append(game.lines, eventLine(e))
		}

		for _, e := range events {
			for _, hook := range hooks {
				hook(state, g, e)
			}
		}
	}
}

// needsMoveLine reports whether a move must get its own synthesized trace
// line. A move whose events already show the action (a card played, drawn or
// melded) is rendered by those events. Any other move is rendered from the
// move itself -- a pass, a bet, a fold, a declaration -- with one exception:
// a FORCED move (the only legal one) that also KEEPS the turn is a phase
// skip, not a decision and not turn-taking information (rummy's meld phase
// with nothing to meld fires every single turn), so it is left out.
func needsMoveLine(numMoves int, turnKept bool, events []sim.Event) bool {
	for _, e := range events {
		switch e.Type {
		case sim.EventCardPlayed, sim.EventCardDrawn, sim.EventMeldLaid:
			return false
		}
	}
	return !(numMoves == 1 && turnKept)
}

// moveLine renders an event-less move: "P{player} {MOVE} [cards] [amount]".
func moveLine(mover int, m sim.Move) string {
	name := moveNames[m.Type]
	if name == "" {
		name = fmt.Sprintf("MOVE(%d)", int(m.Type))
	}
	line := fmt.Sprintf("P%d %s", mover, name)
	if cards := renderCards(m.Cards); cards != "" {
		line += " " + cards
	}
	if m.Amount != 0 {
		line += fmt.Sprintf(" %d", m.Amount)
	}
	return line
}

// eventLine renders one engine event: "P{player} {EVENT} [cards] [detail]".
func eventLine(e sim.Event) string {
	name := eventNames[e.Type]
	if name == "" {
		name = fmt.Sprintf("EVENT(%d)", int(e.Type))
	}
	line := fmt.Sprintf("P%d %s", e.PlayerID, name)
	if cards := renderCards(e.Cards); cards != "" {
		line += " " + cards
	}
	if detail := neutralizeDetail(e.Detail); detail != "" {
		line += " " + detail
	}
	return line
}

// dealEndLine renders the table-level line that closes a betting deal: every
// player's chip total. A stack is state.Scores plus whatever that player has
// already committed to the deal just dealt (the next forced stake is posted in
// the same Upkeep that resolves the showdown), so the totals are the standings
// BETWEEN deals and, in an unscored game, always sum to the chips in play.
// After the LAST deal no new deal is dealt and Committed still holds the
// finished deal's (already paid out) wagers, so it is not added then.
// ok is false for hosts with no pot (they mark round ends with an event).
func dealEndLine(state *sim.GameState) (string, bool) {
	if len(state.Committed) != state.NumPlayers || len(state.Scores) != state.NumPlayers {
		return "", false
	}
	nextDealt := state.Round < state.MaxRound
	var b strings.Builder
	b.WriteString("-- DEAL_END chips")
	for p := 0; p < state.NumPlayers; p++ {
		chips := state.Scores[p]
		if nextDealt {
			chips += state.Committed[p]
		}
		fmt.Fprintf(&b, " P%d=%d", p, chips)
	}
	return b.String(), true
}

// sampleTraces plays up to n games at seeds base, base+1, ... (the seeds
// sim.RunBatch would use) and returns the first `want` completed games with
// distinct traces, plus how many games were played and how many of those
// reached a winner. It stops as soon as `want` games are found, so a game that
// completes reliably costs `want` games, not n.
func sampleTraces(g *genome.Genome, runner sim.GenericRunner, ai sim.AIPlayer, n int, base uint64, want int, hooks ...sim.HookFunc) (shown []tracedGame, played, completed int) {
	seen := map[string]bool{}
	for i := 0; i < n && len(shown) < want; i++ {
		rng := rand.New(rand.NewPCG(base+uint64(i), 0))
		game := playTraced(g, runner, ai, rng, g.MaxTurns(), hooks...)
		played++
		if game.winner < 0 {
			continue
		}
		completed++
		sig := strings.Join(game.lines, "\n")
		if seen[sig] {
			continue
		}
		seen[sig] = true
		shown = append(shown, game)
	}
	return shown, played, completed
}

// traceNote explains a dossier that shows fewer than two sample games. It is
// driven by how many sampled games REALLY completed: the note used to key on
// the number of distinct traces shown, so a game whose traces de-duplicated to
// one was reported as "the rest hit the turn cap" while every game completed.
// shown < 2 implies the whole sample was played, so completed is exact.
func traceNote(shown, completed, sampled int) string {
	switch {
	case shown >= 2:
		return ""
	case completed == 0:
		return "_No games completed in the sampled batch; the automated players run out of fast progress and hit the turn cap. This is a SPEED observation, not a design verdict -- see the Termination section below for whether the win condition is reachable by the rules._\n\n"
	case completed == 1:
		return "_Only one of the sampled games reached a winner; the rest hit the turn cap without resolving. See the Termination section below for whether the win condition is reachable by the rules._\n\n"
	default:
		return fmt.Sprintf("_%d of the %d sampled games reached a winner, but they played out action for action identically, so only one is shown._\n\n", completed, sampled)
	}
}

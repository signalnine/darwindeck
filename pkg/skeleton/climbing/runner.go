// Package climbing implements the climbing / ladder game skeleton (Big Two /
// Tichu / President family) -- the fourth DarwinDeck skeleton (novelty
// evolution).
//
// CLIMBING RULES. Each player holds a hand. There is a "current combination"
// on the table that must be beaten. On your turn you either:
//   - play a combination of the SAME TYPE as the current one but STRICTLY
//     HIGHER in rank, or
//   - PASS.
//
// Leading (the table is clear): play ANY valid combination. When every OTHER
// player passes in succession, the last player who played leads a fresh
// combination and the table clears. The first player to empty their hand wins.
// For more than two players this stays a single-winner race (first to empty),
// matching the other skeletons' single-winner model.
//
// PLAYABILITY BY CONSTRUCTION. A legal move ALWAYS exists, so GenerateMoves
// never returns empty:
//   - When NOT leading you can always PASS.
//   - When leading (table clear) you hold >= 1 card, and singles are always a
//     valid combination type, so you can always play a single.
// This invariant is pinned by TestGenerateMovesNeverEmpty over many random
// reachable states.
//
// PURITY. GenerateMoves and CheckEnd are pure queries (no state mutation). The
// table-clear / lead-rotation transition lives in Upkeep; per-move hand and
// combination updates live in ApplyMove. TrickCards/TrickLeader/PassCount on
// the shared GameState carry the climbing-specific state (see state.go).
//
// TERMINATION. Every PLAY strictly shrinks the mover's hand, and a round of
// passes resolves to a fresh lead within at most NumPlayers consecutive moves,
// so the empty-hand win is always reachable; the batch turn-cap (g.MaxTurns) is
// the backstop against pathological pass cycles.
package climbing

import (
	"math/rand/v2"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

// Runner implements the climbing skeleton. It is a stateless empty struct (like
// the other skeleton runners) so the batch loop can share one instance across
// parallel games.
type Runner struct{}

// Type alias for sim.Card used internally.
type Card = sim.Card

func (r *Runner) Setup(g *genome.Genome, rng *rand.Rand) *sim.GameState {
	deck := sim.StandardDeck()
	sim.ShuffleDeck(deck, rng)

	state := sim.NewGameState(g.Players)

	for i := 0; i < g.Players; i++ {
		hand, rest := sim.DrawN(deck, g.HandSize)
		state.Hands[i] = make([]sim.Card, len(hand))
		copy(state.Hands[i], hand)
		deck = rest
	}

	state.Deck = deck
	state.Phase = sim.PhasePlay
	state.RNG = rng

	// Table starts clear: the active player (seat 0) leads any valid combo.
	state.TrickCards = make([]sim.Card, 0, g.HandSize)
	state.TrickPlayers = make([]int, 0, g.Players)
	state.TrickLeader = 0
	state.PassCount = 0
	state.Direction = 1

	state.Round = 0
	state.MaxRound = 1

	return state
}

// Upkeep owns the table-clear / lead-rotation transition (the only climbing
// state change outside ApplyMove). When every OTHER player has passed in
// succession (PassCount >= NumPlayers-1), the player who played the current
// combination wins the round: the table clears and that player leads fresh.
//
// Mutates state, so it must be called exactly once at the top of each game-loop
// iteration (the GenericRunner contract). GenerateMoves/CheckEnd stay pure.
func (r *Runner) Upkeep(state *sim.GameState, g *genome.Genome) {
	if len(state.TrickCards) == 0 {
		return // table already clear; nothing to resolve
	}
	if state.PassCount >= state.NumPlayers-1 {
		// Everyone else passed: the leader takes control of a clear table.
		state.TrickCards = state.TrickCards[:0]
		state.TrickPlayers = state.TrickPlayers[:0]
		state.PassCount = 0
		state.Active = state.TrickLeader
	}
}

// GenerateMoves returns the legal moves for the active player. PURE: reads
// state and the genome, mutates nothing. NEVER returns empty (the playability
// invariant -- see the package doc).
func (r *Runner) GenerateMoves(state *sim.GameState, g *genome.Genome) []sim.Move {
	hand := state.ActiveHand()
	if len(hand) == 0 {
		// Defensive: an empty-handed player is the winner and CheckEnd fires
		// first, so this is unreachable in a well-formed loop. Return nil rather
		// than fabricate a move on an empty hand.
		return nil
	}

	params := g.Climbing
	if params == nil {
		// No params: degrade to singles-only leading / pass, still playable.
		params = &genome.ClimbingParams{}
	}

	leading := len(state.TrickCards) == 0

	var moves []sim.Move
	if leading {
		// Leading a clear table: play ANY valid combination. Singles are always
		// available (>= 1 card in hand), so moves is never empty.
		moves = r.allCombinations(hand, params, state.Active)
		// allCombinations always includes every single, so len(moves) >= 1.
	} else {
		// Following: play a SAME-TYPE, strictly-higher combination, or pass. Pass
		// is always legal, so moves is never empty even with no beating combo.
		current := classify(state.TrickCards, params)
		for _, combo := range r.allCombinations(hand, params, state.Active) {
			cand := classify(combo.Cards, params)
			if beats(cand, current) {
				moves = append(moves, combo)
			}
		}
		moves = append(moves, sim.Move{Type: sim.MovePass, PlayerID: state.Active})
	}

	// MechKnock (DEEP cross-skeleton borrow: rummy's knock -> climbing). Once
	// the hand is small you may KNOCK to end the game immediately instead of
	// racing to empty; the fewest-cards player then wins (CheckEnd). Climbing is
	// an empty-hand race like shedding, so "fewest cards" is a meaningful lead. It
	// is ADDITIVE (appended after the plays/pass above, never replacing them), so
	// the move set is never emptied and a knock can only END the game sooner --
	// playability and termination hold. A wrong knock hands the win away, the
	// risk that makes the declare a real decision. Acts in the runner, not a hook.
	if g.Knockable() && len(hand) >= 1 && len(hand) <= knockThreshold {
		moves = append(moves, sim.Move{Type: sim.MoveKnock, PlayerID: state.Active})
	}

	return moves
}

// knockThreshold is the hand size at or below which a MechKnock host may knock.
// Small enough that a knock sharpens the endgame (most cards already shed)
// rather than ending the game on turn one. Mirrors the shedding runner.
const knockThreshold = 3

// ApplyMove applies a play or pass and advances to the next player. The
// table-clear transition is deferred to Upkeep (next iteration).
func (r *Runner) ApplyMove(state *sim.GameState, move sim.Move, g *genome.Genome) []sim.Event {
	var events []sim.Event

	switch move.Type {
	case sim.MovePlay:
		player := state.Active
		for _, c := range move.Cards {
			state.Hands[player] = removeCard(state.Hands[player], c)
		}
		// The played combination becomes the table's current combination.
		state.TrickCards = append(state.TrickCards[:0], move.Cards...)
		state.TrickPlayers = append(state.TrickPlayers[:0], player)
		state.TrickLeader = player
		state.PassCount = 0

		cardsCopy := append([]sim.Card(nil), move.Cards...)
		events = append(events, sim.Event{
			Type:     sim.EventCardPlayed,
			PlayerID: player,
			Cards:    cardsCopy,
			Detail:   "climb",
		})

	case sim.MovePass:
		state.PassCount++

	case sim.MoveKnock:
		// MechKnock: the active player declares. Flag the game over by setting
		// Phase=PhaseEnd; CheckEnd reads that and awards the win to the
		// fewest-cards player. Emit an interactive event (it ends everyone's
		// game). Setup leaves Phase=PhasePlay and nothing else touches it, so
		// PhaseEnd uniquely means "knocked".
		state.Phase = sim.PhaseEnd
		events = append(events, sim.Event{
			Type:     sim.EventSpecialTriggered,
			PlayerID: state.Active,
			Detail:   "knock",
		})
	}

	state.Turn++
	state.NextPlayer()
	return events
}

// Progress returns each player's progress toward winning in [0,1] (audit
// Task 8): 1 - handSize/initialHandSize, where initialHandSize is the dealt
// g.HandSize. Climbing never grows a hand (the only optional borrow,
// MechDrawPenalty, can push a hand past the deal), so the value is floored at 0.
// An empty hand (the win condition) scores exactly 1.0.
//
// The climbing winner is first-to-empty-hand (CheckEnd), and Progress ranks by
// (1 - hand share), so the eventual winner's final Progress is the maximum --
// the Task 8 winner-max property by construction (pinned by
// TestProgressWinnerIsMax). Must be pure and allocation-light: the batch loop
// calls it after every applied move.
func (r *Runner) Progress(state *sim.GameState, g *genome.Genome) []float64 {
	out := make([]float64, state.NumPlayers)
	initial := g.HandSize
	if initial < 1 {
		initial = 1
	}
	for i := 0; i < state.NumPlayers; i++ {
		p := 1 - float64(len(state.Hands[i]))/float64(initial)
		if p < 0 {
			p = 0
		}
		out[i] = p
	}
	return out
}

// CheckEnd returns the first player to empty their hand, or -1. PURE.
//
// At max turns the batch runner sees no winner (-1) and classifies the game as
// a genuine timeout rather than a completion -- the same convention the other
// skeletons use, so a hung climbing genome is not masked from Tier-1 timeout
// detection.
func (r *Runner) CheckEnd(state *sim.GameState, g *genome.Genome) int {
	// MechKnock: a knock set Phase=PhaseEnd. The game ends immediately and the
	// fewest-cards-in-hand player wins (ties break to the lowest seat). Checked
	// before the first-to-empty path so a knock decides the winner. A knock when
	// you are not actually fewest hands the win to someone else -- the risk
	// behind the declare.
	if state.Phase == sim.PhaseEnd {
		winner := 0
		for i := 1; i < state.NumPlayers; i++ {
			if len(state.Hands[i]) < len(state.Hands[winner]) {
				winner = i
			}
		}
		return winner
	}

	for i, hand := range state.Hands {
		if len(hand) == 0 {
			return i
		}
	}
	return -1
}

// --- Combination generation and classification ---

// comboKind identifies a combination type for same-type comparison.
type comboKind uint8

const (
	kindSingle comboKind = iota
	kindPair
	kindTriple
	kindRun
)

// comboClass is the classified identity of a combination: its kind, the rank it
// is compared on, and (for runs) its length. Two combinations are the same TYPE
// iff their kind matches AND (for runs) their length matches.
type comboClass struct {
	kind   comboKind
	rank   sim.Rank // comparison rank (the rank for sets; the top rank for runs)
	length int      // run length (1 for non-runs)
}

// classify identifies what type of combination a card slice is, under params.
// The slice is assumed to already be a valid combination (produced by
// allCombinations or the current table combo, which was a valid play). For
// runs, rank is the highest rank in the run.
func classify(cards []sim.Card, params *genome.ClimbingParams) comboClass {
	switch len(cards) {
	case 1:
		return comboClass{kind: kindSingle, rank: cards[0].Rank, length: 1}
	case 2:
		if sameRank(cards) {
			return comboClass{kind: kindPair, rank: cards[0].Rank, length: 1}
		}
	case 3:
		if sameRank(cards) {
			return comboClass{kind: kindTriple, rank: cards[0].Rank, length: 1}
		}
	}
	// Otherwise it is a run (consecutive ranks). Its comparison rank is the top.
	return comboClass{kind: kindRun, rank: topRank(cards), length: len(cards)}
}

// beats reports whether candidate combination cand legally beats the current
// table combination cur: same type (same kind, and for runs same length) and
// strictly higher comparison rank.
func beats(cand, cur comboClass) bool {
	if cand.kind != cur.kind {
		return false
	}
	if cand.kind == kindRun && cand.length != cur.length {
		return false
	}
	return cand.rank > cur.rank
}

// allCombinations enumerates every valid combination the active player can form
// from hand under params, as MovePlay moves. The result is deterministic in
// hand order (move generation must be deterministic for MCTS key stability).
// Singles are ALWAYS included, so the result is never empty for a non-empty
// hand -- the leading-half of the playability invariant.
func (r *Runner) allCombinations(hand []sim.Card, params *genome.ClimbingParams, player int) []sim.Move {
	moves := make([]sim.Move, 0, len(hand))

	// Singles: one per card, in hand order. Always present.
	for i := range hand {
		moves = append(moves, sim.Move{
			Type:     sim.MovePlay,
			Cards:    []sim.Card{hand[i]},
			PlayerID: player,
		})
	}

	// Group hand indices by rank, preserving first-seen order, so set
	// enumeration is deterministic.
	rankOrder := make([]sim.Rank, 0, len(hand))
	byRank := make(map[sim.Rank][]int, len(hand))
	for i, c := range hand {
		if _, seen := byRank[c.Rank]; !seen {
			rankOrder = append(rankOrder, c.Rank)
		}
		byRank[c.Rank] = append(byRank[c.Rank], i)
	}

	if params.AllowPairs {
		for _, rk := range rankOrder {
			idx := byRank[rk]
			if len(idx) >= 2 {
				moves = append(moves, sim.Move{
					Type:     sim.MovePlay,
					Cards:    []sim.Card{hand[idx[0]], hand[idx[1]]},
					PlayerID: player,
				})
			}
		}
	}

	if params.AllowTriples {
		for _, rk := range rankOrder {
			idx := byRank[rk]
			if len(idx) >= 3 {
				moves = append(moves, sim.Move{
					Type:     sim.MovePlay,
					Cards:    []sim.Card{hand[idx[0]], hand[idx[1]], hand[idx[2]]},
					PlayerID: player,
				})
			}
		}
	}

	if params.AllowRuns {
		minLen := params.MinRunLen
		if minLen < 3 {
			minLen = 3 // defensive: runs are always length >= 3
		}
		moves = append(moves, runMoves(hand, minLen, player)...)
	}

	return moves
}

// runMoves enumerates consecutive-rank runs of length >= minLen from hand.
// A run uses one card per rank (the first hand card of that rank), so each
// distinct maximal consecutive rank-window of length minLen produces exactly
// one run move (the lowest-starting window of that length). Deterministic in
// rank order.
func runMoves(hand []sim.Card, minLen, player int) []sim.Move {
	// Map each rank to the first hand card holding it (deterministic: first in
	// hand order).
	firstOfRank := make(map[sim.Rank]int, len(hand))
	for i, c := range hand {
		if _, ok := firstOfRank[c.Rank]; !ok {
			firstOfRank[c.Rank] = i
		}
	}

	var moves []sim.Move
	// Walk ranks 2..Ace; for each consecutive window of length minLen where
	// every rank is present, emit one run move. Enumerate every starting rank so
	// runs of exactly minLen are all reachable (longer runs are reachable as a
	// sequence of these; keeping generation to a fixed length keeps the type
	// system simple and the move list bounded).
	for start := int(sim.Two); start+minLen-1 <= int(sim.Ace); start++ {
		ok := true
		cards := make([]sim.Card, 0, minLen)
		for r := start; r < start+minLen; r++ {
			idx, present := firstOfRank[sim.Rank(r)]
			if !present {
				ok = false
				break
			}
			cards = append(cards, hand[idx])
		}
		if ok {
			moves = append(moves, sim.Move{
				Type:     sim.MovePlay,
				Cards:    cards,
				PlayerID: player,
			})
		}
	}
	return moves
}

// --- helpers ---

func sameRank(cards []sim.Card) bool {
	for i := 1; i < len(cards); i++ {
		if cards[i].Rank != cards[0].Rank {
			return false
		}
	}
	return true
}

func topRank(cards []sim.Card) sim.Rank {
	top := cards[0].Rank
	for _, c := range cards[1:] {
		if c.Rank > top {
			top = c.Rank
		}
	}
	return top
}

// removeCard removes the first occurrence of card from hand.
func removeCard(hand []sim.Card, card sim.Card) []sim.Card {
	for i, c := range hand {
		if c == card {
			return append(hand[:i], hand[i+1:]...)
		}
	}
	return hand
}

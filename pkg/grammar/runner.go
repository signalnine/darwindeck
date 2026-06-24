package grammar

import (
	"math/rand/v2"

	"github.com/darwindeck/darwindeck/pkg/sim"
)

// Runner interprets a GameSpec over sim.GameState. It is the single generic
// engine that plays ANY composition -- the whole point of the grammar.
type Runner struct{ Spec GameSpec }

func cardValue(r sim.Rank) int {
	v := int(r)
	switch {
	case v >= 11 && v <= 13: // J, Q, K
		return 10
	case v == 14: // Ace high
		return 11
	default:
		return v // 2-10
	}
}

func lastCard(cards []sim.Card) (sim.Card, bool) {
	if len(cards) == 0 {
		return sim.Card{}, false
	}
	return cards[len(cards)-1], true
}

func matches(c, top sim.Card, ok bool, rule MatchRule) bool {
	if !ok {
		return true // empty discard: anything plays
	}
	switch rule {
	case MatchSuit:
		return c.Suit == top.Suit
	case MatchRank:
		return c.Rank == top.Rank
	default: // MatchEither
		return c.Suit == top.Suit || c.Rank == top.Rank
	}
}

func removeCard(hand []sim.Card, c sim.Card) []sim.Card {
	for i, h := range hand {
		if h.Suit == c.Suit && h.Rank == c.Rank {
			return append(hand[:i:i], hand[i+1:]...)
		}
	}
	return hand
}

func mv(t sim.MoveType, player int, cards ...sim.Card) sim.Move {
	return sim.Move{Type: t, PlayerID: player, Cards: cards}
}

// Setup deals a fresh game per the spec.
func (rr Runner) Setup(rng *rand.Rand) *sim.GameState {
	s := rr.Spec
	deck := sim.StandardDeck()
	sim.ShuffleDeck(deck, rng)
	gs := sim.NewGameState(s.Players)
	gs.RNG = rng
	gs.NumPlayers = s.Players
	for p := 0; p < s.Players; p++ {
		h, rem := sim.DrawN(deck, s.Deal)
		gs.Hands[p] = h
		deck = rem
	}
	if s.Shared > 0 {
		sh, rem := sim.DrawN(deck, s.Shared)
		gs.Discard = sh
		deck = rem
	}
	gs.Deck = deck
	gs.Folded = make([]bool, s.Players)
	gs.Active = 0
	return gs
}

// LegalMoves returns the legal moves for the active player. INVARIANT: never
// empty -- every generator carries an unconditional fallback.
func (rr Runner) LegalMoves(gs *sim.GameState) []sim.Move {
	p := gs.Active
	hand := gs.Hands[p]
	switch rr.Spec.Move {
	case PlayMatch:
		top, ok := lastCard(gs.Discard)
		var moves []sim.Move
		for _, c := range hand {
			if matches(c, top, ok, rr.Spec.Match) {
				moves = append(moves, mv(sim.MovePlay, p, c))
			}
		}
		if len(moves) == 0 { // fallback: draw, or pass if the deck is gone
			if len(gs.Deck) > 0 {
				moves = append(moves, mv(sim.MoveDraw, p))
			} else {
				moves = append(moves, mv(sim.MovePass, p))
			}
		}
		return moves

	case BeatOrPass:
		leading := len(gs.Discard) == 0
		var moves []sim.Move
		top, ok := lastCard(gs.Discard)
		for _, c := range hand {
			if leading || (ok && c.Rank > top.Rank) {
				moves = append(moves, mv(sim.MovePlay, p, c))
			}
		}
		if !leading {
			moves = append(moves, mv(sim.MovePass, p)) // pass: always legal when following
		}
		if len(moves) == 0 { // leading with an empty hand: end will fire; pass is the floor
			moves = append(moves, mv(sim.MovePass, p))
		}
		return moves

	case Accumulate:
		var moves []sim.Move
		if top, ok := lastCard(gs.Discard); ok {
			moves = append(moves, mv(sim.MovePlay, p, top)) // take the face-up card
		}
		if len(gs.Deck) > 0 {
			moves = append(moves, mv(sim.MoveDraw, p)) // take a blind card
		}
		moves = append(moves, mv(sim.MovePass, p)) // STICK: always legal, the fallback
		return moves

	case Capture:
		var moves []sim.Move
		for _, c := range hand {
			moves = append(moves, mv(sim.MovePlay, p, c)) // capture-or-trail, decided in Apply
		}
		if len(moves) == 0 {
			moves = append(moves, mv(sim.MovePass, p)) // empty hand: pass (refill in Upkeep)
		}
		return moves
	}
	return []sim.Move{mv(sim.MovePass, p)}
}

// Apply mutates the state for the chosen move and advances the turn.
func (rr Runner) Apply(gs *sim.GameState, m sim.Move) {
	p := gs.Active
	s := rr.Spec
	switch s.Move {
	case PlayMatch:
		switch m.Type {
		case sim.MovePlay:
			c := m.Cards[0]
			gs.Hands[p] = removeCard(gs.Hands[p], c)
			gs.Discard = append(gs.Discard, c)
		case sim.MoveDraw:
			drawn, rem := sim.DrawN(gs.Deck, 1)
			gs.Hands[p] = append(gs.Hands[p], drawn...)
			gs.Deck = rem
		}
		gs.Active = (p + 1) % gs.NumPlayers

	case BeatOrPass:
		switch m.Type {
		case sim.MovePlay:
			c := m.Cards[0]
			gs.Hands[p] = removeCard(gs.Hands[p], c)
			gs.Discard = []sim.Card{c} // new top to beat
			gs.PassCount = 0
		case sim.MovePass:
			gs.PassCount++
			if gs.PassCount >= gs.NumPlayers-1 { // all others passed: table clears
				gs.Discard = nil
				gs.PassCount = 0
			}
		}
		gs.Active = (p + 1) % gs.NumPlayers

	case Accumulate:
		switch m.Type {
		case sim.MovePlay: // take the face-up card
			c := m.Cards[0]
			if top, ok := lastCard(gs.Discard); ok && top == c {
				gs.Discard = gs.Discard[:len(gs.Discard)-1]
			}
			gs.Scores[p] += cardValue(c.Rank)
			rr.refillMarket(gs)
			if gs.Scores[p] > s.Target {
				gs.Folded[p] = true // bust
			}
		case sim.MoveDraw: // take a blind card
			drawn, rem := sim.DrawN(gs.Deck, 1)
			gs.Deck = rem
			if len(drawn) > 0 {
				gs.Scores[p] += cardValue(drawn[0].Rank)
				if gs.Scores[p] > s.Target {
					gs.Folded[p] = true
				}
			}
		case sim.MovePass: // stick
			gs.Folded[p] = true
		}
		gs.Active = rr.nextActive(gs)

	case Capture:
		if m.Type == sim.MovePlay {
			c := m.Cards[0]
			gs.Hands[p] = removeCard(gs.Hands[p], c)
			var capt []sim.Card
			var rest []sim.Card
			for _, t := range gs.Discard {
				if t.Rank == c.Rank {
					capt = append(capt, t)
				} else {
					rest = append(rest, t)
				}
			}
			if len(capt) > 0 { // capture
				gs.Discard = rest
				gs.Scores[p] += len(capt) + 1
				gs.Tableau[p] = append(gs.Tableau[p], append(capt, c)...)
			} else { // trail
				gs.Discard = append(gs.Discard, c)
			}
		}
		gs.Active = (p + 1) % gs.NumPlayers
	}
	gs.Turn++
}

// Upkeep runs once per loop iteration before the end check (Capture refill).
func (rr Runner) Upkeep(gs *sim.GameState) {
	if rr.Spec.Move != Capture {
		return
	}
	allEmpty := true
	for p := 0; p < gs.NumPlayers; p++ {
		if len(gs.Hands[p]) > 0 {
			allEmpty = false
			break
		}
	}
	if allEmpty && len(gs.Deck) >= gs.NumPlayers {
		for p := 0; p < gs.NumPlayers; p++ {
			drawn, rem := sim.DrawN(gs.Deck, rr.Spec.Deal)
			gs.Hands[p] = append(gs.Hands[p], drawn...)
			gs.Deck = rem
		}
	}
}

func (rr Runner) refillMarket(gs *sim.GameState) {
	if rr.Spec.Shared > 0 && len(gs.Discard) < rr.Spec.Shared && len(gs.Deck) > 0 {
		drawn, rem := sim.DrawN(gs.Deck, 1)
		gs.Discard = append(gs.Discard, drawn...)
		gs.Deck = rem
	}
}

func (rr Runner) nextActive(gs *sim.GameState) int {
	for i := 1; i <= gs.NumPlayers; i++ {
		q := (gs.Active + i) % gs.NumPlayers
		if !gs.Folded[q] {
			return q
		}
	}
	return gs.Active // all folded; CheckEnd handles it
}

// CheckEnd returns the winner seat (>=0) or -1 if the game continues. A returned
// winner of -1 from a terminal state (e.g. everyone busted) is reported as a
// drawn-but-TERMINATED game by the harness, not a hang.
func (rr Runner) CheckEnd(gs *sim.GameState) (winner int, done bool) {
	s := rr.Spec
	switch s.End {
	case EmptyHand:
		for p := 0; p < gs.NumPlayers; p++ {
			if len(gs.Hands[p]) == 0 {
				return rr.score(gs), true
			}
		}
	case DeckOut:
		allEmpty := true
		for p := 0; p < gs.NumPlayers; p++ {
			if len(gs.Hands[p]) > 0 {
				allEmpty = false
			}
		}
		if allEmpty && len(gs.Deck) == 0 {
			return rr.score(gs), true
		}
	case Bust:
		for p := 0; p < gs.NumPlayers; p++ {
			if !gs.Folded[p] {
				return -1, false
			}
		}
		return rr.score(gs), true // all stuck or busted
	}
	return -1, false
}

func (rr Runner) score(gs *sim.GameState) int {
	s := rr.Spec
	best, bestVal := -1, 0
	switch s.Score {
	case FirstOut:
		for p := 0; p < gs.NumPlayers; p++ {
			if len(gs.Hands[p]) == 0 {
				return p
			}
		}
	case FewestCards:
		best, bestVal = 0, len(gs.Hands[0])
		for p := 1; p < gs.NumPlayers; p++ {
			if len(gs.Hands[p]) < bestVal {
				best, bestVal = p, len(gs.Hands[p])
			}
		}
		return best
	case ClosestTarget:
		best = -1
		for p := 0; p < gs.NumPlayers; p++ {
			if gs.Scores[p] <= s.Target && gs.Scores[p] >= bestVal {
				best, bestVal = p, gs.Scores[p]
			}
		}
		return best // -1 if everyone busted
	case MostCaptured, HighScore:
		best, bestVal = 0, gs.Scores[0]
		for p := 1; p < gs.NumPlayers; p++ {
			if gs.Scores[p] > bestVal {
				best, bestVal = p, gs.Scores[p]
			}
		}
		return best
	}
	return best
}

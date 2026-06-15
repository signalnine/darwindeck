// Package casino implements the Casino / Scopa fishing-capture skeleton.
//
// On your turn you play one card from hand and either CAPTURE table cards (any
// table card of the same rank, or -- with AllowSumCapture -- a subset of number
// cards whose pip values sum to your card's value) into your pile, or TRAIL the
// card face-up onto the table. Trailing is always legal, so a legal move always
// exists (the playability floor). Hands refill from the stock between rounds
// until the stock can no longer deal a full round; the last player to capture
// then sweeps any cards left on the table. Most captured cards wins.
//
// State mapping onto sim.GameState: state.Deck is the stock, state.Discard is
// the face-up table, state.Hands are the hands, state.Tableau[i] is player i's
// captured pile, and state.TrickLeader records the last capturer (for the
// end-of-game table sweep).
package casino

import (
	"math/rand/v2"
	"sort"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

type Card = sim.Card

// Runner implements the casino skeleton.
type Runner struct{}

func (r *Runner) Setup(g *genome.Genome, rng *rand.Rand) *sim.GameState {
	deck := sim.StandardDeck()
	sim.ShuffleDeck(deck, rng)

	state := sim.NewGameState(g.Players)
	for i := 0; i < g.Players; i++ {
		hand, rest := sim.DrawN(deck, g.HandSize)
		state.Hands[i] = append([]sim.Card(nil), hand...)
		deck = rest
	}

	tableN := 0
	if g.Casino != nil {
		tableN = g.Casino.TableSize
	}
	if tableN > len(deck) {
		tableN = len(deck)
	}
	table, rest := sim.DrawN(deck, tableN)
	state.Discard = append([]sim.Card(nil), table...)
	state.Deck = rest

	state.Phase = sim.PhasePlay
	state.RNG = rng
	state.Active = 0
	state.TrickLeader = -1 // last capturer; -1 = nobody has captured yet
	return state
}

func allHandsEmpty(state *sim.GameState) bool {
	for _, h := range state.Hands {
		if len(h) > 0 {
			return false
		}
	}
	return true
}

// canRedeal reports whether the stock still holds a full round (one HandSize
// hand per player). Dealing only full rounds keeps every hand equal, so the
// active player always has a card when GenerateMoves is called (no mid-round
// empty-handed seat) -- the invariant the game loop relies on.
func canRedeal(state *sim.GameState, g *genome.Genome) bool {
	return len(state.Deck) >= g.HandSize*state.NumPlayers
}

// Upkeep redeals a full round from the stock when all hands are empty, or, when
// no full round remains, ends the game by sweeping the table to the last
// capturer (standard Casino end rule). Mutating; called once per loop iteration.
func (r *Runner) Upkeep(state *sim.GameState, g *genome.Genome) {
	if !allHandsEmpty(state) {
		return
	}
	if canRedeal(state, g) {
		for i := 0; i < state.NumPlayers; i++ {
			hand, rest := sim.DrawN(state.Deck, g.HandSize)
			state.Hands[i] = append(state.Hands[i][:0], hand...)
			state.Deck = rest
		}
		return
	}
	// Game ending: last capturer takes whatever is still on the table. CheckEnd
	// (pure) then reports the winner from the captured piles.
	if len(state.Discard) > 0 && state.TrickLeader >= 0 && state.TrickLeader < state.NumPlayers {
		state.Tableau[state.TrickLeader] = append(state.Tableau[state.TrickLeader], state.Discard...)
		state.Discard = state.Discard[:0]
	}
}

func (r *Runner) GenerateMoves(state *sim.GameState, g *genome.Genome) []sim.Move {
	hand := state.ActiveHand()
	if len(hand) == 0 {
		return nil
	}
	allowSum := g.Casino != nil && g.Casino.AllowSumCapture

	var moves []sim.Move
	for i := range hand {
		c := hand[i]
		// Trail is always legal -- the playability floor.
		moves = append(moves, sim.Move{Type: sim.MovePlay, Cards: []sim.Card{c}, PlayerID: state.Active})
		for _, grp := range captureGroups(c, state.Discard, allowSum) {
			cards := make([]sim.Card, 0, 1+len(grp))
			cards = append(cards, c)
			cards = append(cards, grp...)
			moves = append(moves, sim.Move{Type: sim.MoveCapture, Cards: cards, PlayerID: state.Active})
		}
	}
	return moves
}

func (r *Runner) ApplyMove(state *sim.GameState, move sim.Move, g *genome.Genome) []sim.Event {
	player := state.Active
	var events []sim.Event

	switch move.Type {
	case sim.MovePlay: // trail
		c := move.Cards[0]
		state.Hands[player] = removeCard(state.Hands[player], c)
		state.Discard = append(state.Discard, c)
		events = append(events, sim.Event{
			Type: sim.EventCardPlayed, PlayerID: player, Cards: []sim.Card{c}, Detail: "trail",
		})

	case sim.MoveCapture:
		c := move.Cards[0]
		captured := move.Cards[1:]
		state.Hands[player] = removeCard(state.Hands[player], c)
		for _, cap := range captured {
			state.Discard = removeCard(state.Discard, cap)
		}
		if player < len(state.Tableau) {
			state.Tableau[player] = append(state.Tableau[player], c)
			state.Tableau[player] = append(state.Tableau[player], captured...)
		}
		state.TrickLeader = player // last capturer sweeps the table at game end
		events = append(events, sim.Event{
			Type: sim.EventCardPlayed, PlayerID: player, Cards: append([]sim.Card(nil), move.Cards...), Detail: "capture",
		})
	}

	state.Turn++
	state.NextPlayer()
	return events
}

func (r *Runner) CheckEnd(state *sim.GameState, g *genome.Genome) int {
	// Over once all hands are empty and the stock can't deal another full round.
	// Upkeep has already swept the table to the last capturer by this point, so
	// the captured piles are final. Most captured cards wins; ties to lowest seat.
	if allHandsEmpty(state) && !canRedeal(state, g) {
		winner := 0
		for i := 1; i < state.NumPlayers; i++ {
			if len(state.Tableau[i]) > len(state.Tableau[winner]) {
				winner = i
			}
		}
		return winner
	}
	return -1
}

// Progress is each player's share of all captured cards in [0,1]. argmax is the
// winner rule (most captured), so the winner's final Progress is the max.
func (r *Runner) Progress(state *sim.GameState, g *genome.Genome) []float64 {
	out := make([]float64, state.NumPlayers)
	total := 0
	for i := 0; i < state.NumPlayers; i++ {
		total += len(state.Tableau[i])
	}
	if total == 0 {
		return out
	}
	for i := 0; i < state.NumPlayers; i++ {
		out[i] = float64(len(state.Tableau[i])) / float64(total)
	}
	return out
}

// captureGroups returns the capturable groups for playing card c against the
// table: the same-rank match (all table cards of c's rank, as one group) and,
// when allowSum is set, every subset of 2+ number cards whose pip values sum to
// c's value. Output is DETERMINISTIC (match first, then sum subsets in sorted
// index order) so the seeded batch stays reproducible.
func captureGroups(c sim.Card, table []sim.Card, allowSum bool) [][]sim.Card {
	var groups [][]sim.Card

	// Rank match: capture every same-rank table card together.
	var match []sim.Card
	for _, t := range table {
		if t.Rank == c.Rank {
			match = append(match, t)
		}
	}
	if len(match) > 0 {
		groups = append(groups, match)
	}

	if !allowSum {
		return groups
	}
	v := cardValue(c.Rank)
	if v < 1 || v > 10 {
		return groups // face cards capture by rank only
	}

	// Number cards on the table (value 1..10), sorted for determinism.
	var nums []sim.Card
	for _, t := range table {
		if cv := cardValue(t.Rank); cv >= 1 && cv <= 10 {
			nums = append(nums, t)
		}
	}
	sortCards(nums)

	// Enumerate subsets summing exactly to v; record those of size >= 2 (a
	// size-1 subset summing to v is the same-rank card, already in `match`).
	var cur []sim.Card
	var rec func(start, remaining int)
	rec = func(start, remaining int) {
		if remaining == 0 {
			if len(cur) >= 2 {
				groups = append(groups, append([]sim.Card(nil), cur...))
			}
			return
		}
		for j := start; j < len(nums); j++ {
			cv := cardValue(nums[j].Rank)
			if cv > remaining {
				continue
			}
			cur = append(cur, nums[j])
			rec(j+1, remaining-cv)
			cur = cur[:len(cur)-1]
		}
	}
	rec(0, v)
	return groups
}

// cardValue is a card's Casino pip value: Ace=1, 2..10 = pip, face cards = 0
// (they have no numeric value and capture only by rank).
func cardValue(r sim.Rank) int {
	switch {
	case r == sim.Ace:
		return 1
	case r >= 2 && r <= 10:
		return int(r)
	default:
		return 0
	}
}

func sortCards(cs []sim.Card) {
	sort.Slice(cs, func(i, j int) bool {
		if cs[i].Rank != cs[j].Rank {
			return cs[i].Rank < cs[j].Rank
		}
		return cs[i].Suit < cs[j].Suit
	})
}

func removeCard(cards []sim.Card, c sim.Card) []sim.Card {
	for i, x := range cards {
		if x == c {
			return append(cards[:i], cards[i+1:]...)
		}
	}
	return cards
}

package rummy

import (
	"math/rand/v2"
	"sort"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

// Runner implements the rummy game skeleton.
// Draw, form melds (sets/runs), discard. Knock or go gin to end round.
type Runner struct{}

func (r *Runner) Setup(g *genome.Genome, rng *rand.Rand) *sim.GameState {
	deck := sim.StandardDeck()
	sim.ShuffleDeck(deck, rng)

	state := sim.NewGameState(g.Players)

	// Deal hands
	for i := 0; i < g.Players; i++ {
		hand, rest := sim.DrawN(deck, g.HandSize)
		state.Hands[i] = make([]sim.Card, len(hand))
		copy(state.Hands[i], hand)
		deck = rest
	}

	// Flip one card to discard pile
	if len(deck) > 0 {
		top := deck[0]
		deck = deck[1:]
		state.Discard = []sim.Card{top}
	}

	state.Deck = deck
	state.Phase = sim.PhaseDraw
	state.Melds = make([][]sim.Card, 0)
	state.MeldOwner = make([]int, 0)

	return state
}

func (r *Runner) GenerateMoves(state *sim.GameState, g *genome.Genome) []sim.Move {
	params := g.Rummy
	if params == nil {
		return []sim.Move{{Type: sim.MovePass, PlayerID: state.Active}}
	}

	switch state.Phase {
	case sim.PhaseDraw:
		return r.generateDrawMoves(state, params)
	case sim.PhaseMeld:
		return r.generateMeldMoves(state, params, g)
	case sim.PhaseDiscard:
		return r.generateDiscardMoves(state)
	default:
		return []sim.Move{{Type: sim.MovePass, PlayerID: state.Active}}
	}
}

func (r *Runner) generateDrawMoves(state *sim.GameState, params *genome.RummyParams) []sim.Move {
	var moves []sim.Move

	switch params.DrawFrom {
	case genome.DrawDeck:
		if len(state.Deck) > 0 {
			moves = append(moves, sim.Move{Type: sim.MoveDraw, PlayerID: state.Active})
		}
	case genome.DrawDiscard:
		if len(state.Discard) > 0 {
			moves = append(moves, sim.Move{
				Type:     sim.MoveDraw,
				Cards:    []sim.Card{state.Discard[len(state.Discard)-1]},
				PlayerID: state.Active,
			})
		}
	case genome.DrawEither:
		if len(state.Deck) > 0 {
			moves = append(moves, sim.Move{Type: sim.MoveDraw, PlayerID: state.Active})
		}
		if len(state.Discard) > 0 {
			moves = append(moves, sim.Move{
				Type:     sim.MoveDraw,
				Cards:    []sim.Card{state.Discard[len(state.Discard)-1]},
				PlayerID: state.Active,
			})
		}
	}

	// If no draws possible, pass
	if len(moves) == 0 {
		moves = append(moves, sim.Move{Type: sim.MovePass, PlayerID: state.Active})
	}

	return moves
}

func (r *Runner) generateMeldMoves(state *sim.GameState, params *genome.RummyParams, g *genome.Genome) []sim.Move {
	hand := state.ActiveHand()
	var moves []sim.Move

	// Find all valid melds in hand
	melds := findMelds(hand, params)
	for _, meld := range melds {
		moves = append(moves, sim.Move{
			Type:     sim.MoveMeld,
			Cards:    meld,
			PlayerID: state.Active,
		})
	}

	// Knock option (if deadwood is low enough)
	deadwood := calcDeadwood(hand, params)
	if deadwood <= params.KnockThreshold {
		moves = append(moves, sim.Move{
			Type:     sim.MoveKnock,
			PlayerID: state.Active,
		})
	}

	// Can always pass (skip melding, go to discard)
	moves = append(moves, sim.Move{Type: sim.MovePass, PlayerID: state.Active})

	return moves
}

func (r *Runner) generateDiscardMoves(state *sim.GameState) []sim.Move {
	hand := state.ActiveHand()
	if len(hand) == 0 {
		return []sim.Move{{Type: sim.MovePass, PlayerID: state.Active}}
	}

	moves := make([]sim.Move, len(hand))
	for i, card := range hand {
		moves[i] = sim.Move{
			Type:     sim.MoveDiscard,
			Cards:    []sim.Card{card},
			PlayerID: state.Active,
		}
	}
	return moves
}

func (r *Runner) ApplyMove(state *sim.GameState, move sim.Move, g *genome.Genome) []sim.Event {
	var events []sim.Event

	switch move.Type {
	case sim.MoveDraw:
		if len(move.Cards) > 0 {
			// Draw from discard
			card := state.Discard[len(state.Discard)-1]
			state.Discard = state.Discard[:len(state.Discard)-1]
			state.Hands[state.Active] = append(state.Hands[state.Active], card)
			events = append(events, sim.Event{
				Type:     sim.EventCardDrawn,
				PlayerID: state.Active,
				Cards:    []sim.Card{card},
				Detail:   "discard",
			})
		} else {
			// Draw from deck
			if len(state.Deck) > 0 {
				drawn, rest := sim.DrawN(state.Deck, 1)
				state.Deck = rest
				state.Hands[state.Active] = append(state.Hands[state.Active], drawn...)
				events = append(events, sim.Event{
					Type:     sim.EventCardDrawn,
					PlayerID: state.Active,
					Cards:    drawn,
					Detail:   "deck",
				})
			}
		}
		state.Phase = sim.PhaseMeld

	case sim.MoveMeld:
		// Remove meld cards from hand
		for _, card := range move.Cards {
			state.Hands[state.Active] = removeCard(state.Hands[state.Active], card)
		}
		meldCopy := make([]sim.Card, len(move.Cards))
		copy(meldCopy, move.Cards)
		state.Melds = append(state.Melds, meldCopy)
		state.MeldOwner = append(state.MeldOwner, state.Active)
		events = append(events, sim.Event{
			Type:     sim.EventMeldLaid,
			PlayerID: state.Active,
			Cards:    move.Cards,
		})
		// Stay in meld phase (can lay multiple melds)

	case sim.MoveKnock:
		// Knock: lay down all melds, then score
		params := g.Rummy
		if params != nil {
			hand := state.Hands[state.Active]
			melds := findMelds(hand, params)
			for _, meld := range melds {
				for _, card := range meld {
					state.Hands[state.Active] = removeCard(state.Hands[state.Active], card)
				}
				meldCopy := make([]sim.Card, len(meld))
				copy(meldCopy, meld)
				state.Melds = append(state.Melds, meldCopy)
				state.MeldOwner = append(state.MeldOwner, state.Active)
			}
		}
		state.Phase = sim.PhaseEnd
		events = append(events, sim.Event{
			Type:     sim.EventRoundEnd,
			PlayerID: state.Active,
			Detail:   "knock",
		})

	case sim.MoveDiscard:
		card := move.Cards[0]
		state.Hands[state.Active] = removeCard(state.Hands[state.Active], card)
		state.Discard = append(state.Discard, card)
		events = append(events, sim.Event{
			Type:     sim.EventCardPlayed,
			PlayerID: state.Active,
			Cards:    []sim.Card{card},
			Detail:   "discard",
		})

		// Check for gin (empty hand after discard)
		if len(state.Hands[state.Active]) == 0 {
			state.Phase = sim.PhaseEnd
			events = append(events, sim.Event{
				Type:     sim.EventRoundEnd,
				PlayerID: state.Active,
				Detail:   "gin",
			})
		} else {
			// Next player's turn
			state.Turn++
			state.NextPlayer()
			state.Phase = sim.PhaseDraw

			// If deck is empty, reshuffle discard (keep top card)
			if len(state.Deck) == 0 && len(state.Discard) > 1 {
				top := state.Discard[len(state.Discard)-1]
				state.Deck = state.Discard[:len(state.Discard)-1]
				state.Discard = []sim.Card{top}
				// Shuffle the new deck
				// Use turn as entropy since we don't have rng here
				for i := len(state.Deck) - 1; i > 0; i-- {
					j := (state.Turn*31 + i*17) % (i + 1)
					state.Deck[i], state.Deck[j] = state.Deck[j], state.Deck[i]
				}
			}
		}
		return events

	case sim.MovePass:
		if state.Phase == sim.PhaseMeld {
			state.Phase = sim.PhaseDiscard
		} else if state.Phase == sim.PhaseDraw {
			state.Phase = sim.PhaseMeld
		}
	}

	if move.Type != sim.MoveDiscard {
		state.Turn++
	}

	return events
}

func (r *Runner) CheckEnd(state *sim.GameState, g *genome.Genome) int {
	if state.Phase == sim.PhaseEnd {
		return scoreRound(state, g)
	}

	// Max turns check
	if state.Turn >= g.MaxTurns() {
		return scoreRound(state, g)
	}

	return -1
}

func scoreRound(state *sim.GameState, g *genome.Genome) int {
	params := g.Rummy
	if params == nil {
		return 0
	}

	// Score each player's deadwood
	for i := 0; i < state.NumPlayers; i++ {
		deadwood := calcDeadwood(state.Hands[i], params)
		state.Scores[i] = -deadwood // Negative deadwood = better
	}

	// Highest score (least deadwood) wins
	best := 0
	for i := 1; i < state.NumPlayers; i++ {
		if state.Scores[i] > state.Scores[best] {
			best = i
		}
	}
	return best
}

// findMelds finds all valid melds in a hand (greedy, largest first).
func findMelds(hand []sim.Card, params *genome.RummyParams) [][]sim.Card {
	var melds [][]sim.Card

	if params.MeldTypes == genome.MeldSets || params.MeldTypes == genome.MeldBoth {
		melds = append(melds, findSets(hand, params.MinMeldSize)...)
	}

	if params.MeldTypes == genome.MeldRuns || params.MeldTypes == genome.MeldBoth {
		melds = append(melds, findRuns(hand, params.MinMeldSize)...)
	}

	return melds
}

// findSets finds groups of cards with the same rank.
func findSets(hand []sim.Card, minSize int) [][]sim.Card {
	byRank := make(map[sim.Rank][]sim.Card)
	for _, c := range hand {
		byRank[c.Rank] = append(byRank[c.Rank], c)
	}

	var sets [][]sim.Card
	for _, cards := range byRank {
		if len(cards) >= minSize {
			set := make([]sim.Card, len(cards))
			copy(set, cards)
			sets = append(sets, set)
		}
	}
	return sets
}

// findRuns finds sequences of consecutive cards in the same suit.
func findRuns(hand []sim.Card, minSize int) [][]sim.Card {
	bySuit := make(map[sim.Suit][]sim.Card)
	for _, c := range hand {
		bySuit[c.Suit] = append(bySuit[c.Suit], c)
	}

	var runs [][]sim.Card
	for _, cards := range bySuit {
		if len(cards) < minSize {
			continue
		}

		// Sort by rank
		sort.Slice(cards, func(i, j int) bool {
			return cards[i].Rank < cards[j].Rank
		})

		// Find consecutive sequences
		run := []sim.Card{cards[0]}
		for i := 1; i < len(cards); i++ {
			if cards[i].Rank == cards[i-1].Rank+1 {
				run = append(run, cards[i])
			} else {
				if len(run) >= minSize {
					runCopy := make([]sim.Card, len(run))
					copy(runCopy, run)
					runs = append(runs, runCopy)
				}
				run = []sim.Card{cards[i]}
			}
		}
		if len(run) >= minSize {
			runCopy := make([]sim.Card, len(run))
			copy(runCopy, run)
			runs = append(runs, runCopy)
		}
	}
	return runs
}

// calcDeadwood calculates the total deadwood points in a hand.
// Cards not part of any meld count as deadwood.
// Face cards = 10, Ace = 1, others = face value.
func calcDeadwood(hand []sim.Card, params *genome.RummyParams) int {
	if len(hand) == 0 {
		return 0
	}

	// Candidate melds may share cards (e.g. 5H appears in both a 5-set and a
	// 5H-6H-7H run). A single card can only sit in one meld, so we greedily
	// assign cards to the highest-value meld first and skip any meld whose
	// cards are no longer all available.
	melds := findMelds(hand, params)
	sort.SliceStable(melds, func(i, j int) bool {
		return meldValue(melds[i]) > meldValue(melds[j])
	})

	used := make(map[int]bool)
	for _, meld := range melds {
		claim := make([]int, 0, len(meld))
		ok := true
		for _, mc := range meld {
			idx := -1
			for i, hc := range hand {
				if hc == mc && !used[i] && !containsInt(claim, i) {
					idx = i
					break
				}
			}
			if idx < 0 {
				ok = false
				break
			}
			claim = append(claim, idx)
		}
		if !ok {
			continue
		}
		for _, i := range claim {
			used[i] = true
		}
	}

	total := 0
	for i, card := range hand {
		if !used[i] {
			total += cardValue(card)
		}
	}
	return total
}

func meldValue(meld []sim.Card) int {
	total := 0
	for _, c := range meld {
		total += cardValue(c)
	}
	return total
}

func containsInt(xs []int, v int) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func cardValue(c sim.Card) int {
	switch {
	case c.Rank >= sim.Ten:
		return 10 // 10, J, Q, K
	case c.Rank == sim.Ace:
		return 1 // Ace (low in rummy deadwood)
	default:
		return int(c.Rank) // 2-9
	}
}

// Ace is high for runs but low for deadwood — handle in rank ordering
// For simplicity, Ace is only high (14) in our rank system.
// Runs with Ace-low (A-2-3) would need special handling.

func removeCard(hand []sim.Card, card sim.Card) []sim.Card {
	for i, c := range hand {
		if c == card {
			return append(hand[:i], hand[i+1:]...)
		}
	}
	return hand
}

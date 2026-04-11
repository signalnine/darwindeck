package shedding

import (
	"math/rand/v2"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

// Runner implements the shedding game skeleton.
// Shedding games: play cards matching the top of the discard pile by suit/rank.
// If you can't play, draw. First to empty hand wins.
type Runner struct{}

func (r *Runner) Setup(g *genome.Genome, rng *rand.Rand) *sim.GameState {
	deck := sim.StandardDeck()
	sim.ShuffleDeck(deck, rng)

	state := sim.NewGameState(g.Players)

	// Deal hands
	for i := 0; i < g.Players; i++ {
		hand, rest := sim.DrawN(deck, g.HandSize)
		state.Hands[i] = make([]Card, len(hand))
		copy(state.Hands[i], hand)
		deck = rest
	}

	// Flip top card to start discard pile
	if len(deck) > 0 {
		top := deck[0]
		deck = deck[1:]
		state.Discard = []sim.Card{top}
		state.TopCard = &sim.Card{Suit: top.Suit, Rank: top.Rank}
	}

	state.Deck = deck
	state.Phase = sim.PhasePlay
	return state
}

func (r *Runner) GenerateMoves(state *sim.GameState, g *genome.Genome) []sim.Move {
	hand := state.ActiveHand()
	if len(hand) == 0 {
		return nil
	}

	params := g.Shedding
	if params == nil {
		return []sim.Move{{Type: sim.MovePass, PlayerID: state.Active}}
	}

	var moves []sim.Move

	if state.TopCard != nil {
		for i, card := range hand {
			if matchesTop(card, *state.TopCard, params.MatchRule) {
				moves = append(moves, sim.Move{
					Type:    sim.MovePlay,
					Cards:   []sim.Card{hand[i]},
					PlayerID: state.Active,
				})
			}
		}
	}

	// Check for special wild cards that can always be played
	for i, card := range hand {
		if isWild(card, g.SpecialCards) && !alreadyInMoves(moves, card) {
			moves = append(moves, sim.Move{
				Type:    sim.MovePlay,
				Cards:   []sim.Card{hand[i]},
				PlayerID: state.Active,
			})
		}
	}

	// If no playable cards, must draw
	if len(moves) == 0 {
		if len(state.Deck) > 0 {
			moves = append(moves, sim.Move{
				Type:    sim.MoveDraw,
				PlayerID: state.Active,
			})
		} else {
			// No deck and no plays — pass
			moves = append(moves, sim.Move{
				Type:    sim.MovePass,
				PlayerID: state.Active,
			})
		}
	}

	return moves
}

func (r *Runner) ApplyMove(state *sim.GameState, move sim.Move, g *genome.Genome) []sim.Event {
	var events []sim.Event

	switch move.Type {
	case sim.MovePlay:
		card := move.Cards[0]
		// Remove card from hand
		state.Hands[state.Active] = removeCard(state.Hands[state.Active], card)
		// Add to discard pile
		state.Discard = append(state.Discard, card)
		state.TopCard = &sim.Card{Suit: card.Suit, Rank: card.Rank}

		events = append(events, sim.Event{
			Type:     sim.EventCardPlayed,
			PlayerID: state.Active,
			Cards:    []sim.Card{card},
		})

		// Apply special card effects
		effects := applySpecialEffects(state, card, g)
		events = append(events, effects...)

	case sim.MoveDraw:
		penalty := 1
		if g.Shedding != nil {
			penalty = g.Shedding.DrawPenalty
		}
		drawn, rest := sim.DrawN(state.Deck, penalty)
		state.Deck = rest
		state.Hands[state.Active] = append(state.Hands[state.Active], drawn...)

		events = append(events, sim.Event{
			Type:     sim.EventCardDrawn,
			PlayerID: state.Active,
			Cards:    drawn,
		})

	case sim.MovePass:
		// Nothing happens
	}

	// Advance to next player (unless a skip effect already did)
	state.Turn++
	state.NextPlayer()

	return events
}

func (r *Runner) CheckEnd(state *sim.GameState, g *genome.Genome) int {
	// First player to empty hand wins
	for i, hand := range state.Hands {
		if len(hand) == 0 {
			return i
		}
	}

	// Check max turns
	if state.Turn >= g.MaxTurns() {
		// Game over — player with fewest cards wins
		minCards := len(state.Hands[0])
		winner := 0
		for i := 1; i < state.NumPlayers; i++ {
			if len(state.Hands[i]) < minCards {
				minCards = len(state.Hands[i])
				winner = i
			}
		}
		return winner
	}

	return -1
}

// matchesTop checks if a card matches the top of the discard pile.
func matchesTop(card, top sim.Card, rule genome.MatchRule) bool {
	switch rule {
	case genome.MatchSuit:
		return card.Suit == top.Suit
	case genome.MatchRank:
		return card.Rank == top.Rank
	case genome.MatchEither:
		return card.Suit == top.Suit || card.Rank == top.Rank
	case genome.MatchBoth:
		return card.Suit == top.Suit && card.Rank == top.Rank
	default:
		return false
	}
}

// isWild checks if a card is designated as wild.
func isWild(card sim.Card, specials []genome.SpecialCard) bool {
	for _, sc := range specials {
		if sc.Type != genome.SpecialWild {
			continue
		}
		if sc.ByRank != 0 && sc.ByRank != uint8(card.Rank) {
			continue
		}
		if sc.BySuit != 0 && sc.BySuit != uint8(card.Suit)+1 {
			continue
		}
		return true
	}
	return false
}

func alreadyInMoves(moves []sim.Move, card sim.Card) bool {
	for _, m := range moves {
		if len(m.Cards) > 0 && m.Cards[0] == card {
			return true
		}
	}
	return false
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

// applySpecialEffects handles special card effects when played.
func applySpecialEffects(state *sim.GameState, card sim.Card, g *genome.Genome) []sim.Event {
	var events []sim.Event

	for _, sc := range g.SpecialCards {
		if !cardMatchesSpecial(card, sc) {
			continue
		}

		switch sc.Type {
		case genome.SpecialSkip:
			// Skip is handled by advancing an extra player
			state.NextPlayer()
			events = append(events, sim.Event{
				Type:     sim.EventSpecialTriggered,
				PlayerID: state.Active,
				Detail:   "skip",
			})

		case genome.SpecialReverse:
			// In 2-player, reverse is the same as skip
			if state.NumPlayers == 2 {
				state.NextPlayer()
			}
			// For 3+, we'd need a direction field — simplified for now
			events = append(events, sim.Event{
				Type:   sim.EventSpecialTriggered,
				Detail: "reverse",
			})

		case genome.SpecialDrawTwo:
			nextPlayer := (state.Active + 1) % state.NumPlayers
			drawn, rest := sim.DrawN(state.Deck, 2)
			state.Deck = rest
			state.Hands[nextPlayer] = append(state.Hands[nextPlayer], drawn...)
			events = append(events, sim.Event{
				Type:     sim.EventSpecialTriggered,
				PlayerID: nextPlayer,
				Cards:    drawn,
				Detail:   "draw_two",
			})

		case genome.SpecialDrawFour:
			nextPlayer := (state.Active + 1) % state.NumPlayers
			drawn, rest := sim.DrawN(state.Deck, 4)
			state.Deck = rest
			state.Hands[nextPlayer] = append(state.Hands[nextPlayer], drawn...)
			events = append(events, sim.Event{
				Type:     sim.EventSpecialTriggered,
				PlayerID: nextPlayer,
				Cards:    drawn,
				Detail:   "draw_four",
			})

		case genome.SpecialWild:
			// Wild effect is handled in move generation (always playable)
		}
	}

	return events
}

// cardMatchesSpecial checks if a card triggers a special effect.
func cardMatchesSpecial(card sim.Card, sc genome.SpecialCard) bool {
	if sc.ByRank != 0 && sc.ByRank != uint8(card.Rank) {
		return false
	}
	if sc.BySuit != 0 && sc.BySuit != uint8(card.Suit)+1 {
		return false
	}
	return true
}

// Type alias for sim.Card used in Setup
type Card = sim.Card

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
	state.RNG = rng
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

	// If no playable cards, must draw. If the deck is empty, recycle the
	// discard pile (minus the top card) before falling through to Pass --
	// otherwise the game pingpongs MovePass deterministically until MaxTurns
	// and gets misclassified as a degenerate timeout. Mirrors the rummy
	// runner's behavior in ApplyMove around MoveDiscard.
	if len(moves) == 0 {
		if len(state.Deck) == 0 && len(state.Discard) > 1 {
			refillDeckFromDiscard(state)
		}
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

// refillDeckFromDiscard moves all but the top discard card into the deck and
// shuffles using state.RNG. Called when the deck has emptied so shedding
// games can recover instead of stalling on an unreachable discard pile.
func refillDeckFromDiscard(state *sim.GameState) {
	if len(state.Discard) <= 1 {
		return
	}
	top := state.Discard[len(state.Discard)-1]
	recycled := state.Discard[:len(state.Discard)-1]
	state.Deck = append(state.Deck[:0], recycled...)
	state.Discard = []sim.Card{top}
	if state.RNG != nil {
		sim.ShuffleDeck(state.Deck, state.RNG)
	}
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
			Detail:   "discard", // Shedding plays go to shared discard pile
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

	// At max turns, return -1 so the batch runner classifies the game as a
	// genuine timeout rather than a completion. Awarding the smallest hand
	// here would mask hung shedding genomes from Tier1 timeout detection.
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
//
// All victim/skip computations resolve against the player who actually
// played the card (originActive). When two SpecialCards entries match the
// same card (e.g. one by ByRank and another by BySuit), each effect's
// "next player" must still mean origin+1, not origin+1+(prior advances).
// We accumulate the total NextPlayer advances and apply them once at the
// end so chained effects target the rightful victim.
func applySpecialEffects(state *sim.GameState, card sim.Card, g *genome.Genome) []sim.Event {
	var events []sim.Event
	originActive := state.Active
	advances := 0
	nextOf := func() int {
		return (originActive + 1) % state.NumPlayers
	}

	for _, sc := range g.SpecialCards {
		if !cardMatchesSpecial(card, sc) {
			continue
		}

		switch sc.Type {
		case genome.SpecialSkip:
			skipped := nextOf()
			advances++
			events = append(events, sim.Event{
				Type:     sim.EventSpecialTriggered,
				PlayerID: skipped,
				Detail:   "skip",
			})

		case genome.SpecialReverse:
			// In 2-player, reverse is functionally a skip: flip direction so
			// the trailing NextPlayer in ApplyMove lands back on the original
			// player, mirroring Uno semantics.
			// For 3+, flip the direction so subsequent NextPlayer calls walk
			// the play order backward.
			if state.Direction == 0 {
				state.Direction = 1
			}
			state.Direction = -state.Direction
			if state.NumPlayers == 2 {
				advances++
			}
			events = append(events, sim.Event{
				Type:   sim.EventSpecialTriggered,
				Detail: "reverse",
			})

		case genome.SpecialDrawTwo:
			victim := nextOf()
			drawn, rest := sim.DrawN(state.Deck, 2)
			state.Deck = rest
			state.Hands[victim] = append(state.Hands[victim], drawn...)
			// Standard Uno-style draw also forces the victim to lose their
			// turn. Advance once here so the trailing NextPlayer in
			// ApplyMove rotates past them.
			advances++
			events = append(events, sim.Event{
				Type:     sim.EventSpecialTriggered,
				PlayerID: victim,
				Cards:    drawn,
				Detail:   "draw_two",
			})

		case genome.SpecialDrawFour:
			victim := nextOf()
			drawn, rest := sim.DrawN(state.Deck, 4)
			state.Deck = rest
			state.Hands[victim] = append(state.Hands[victim], drawn...)
			// Standard Uno-style draw also forces the victim to lose their
			// turn. Advance once here so the trailing NextPlayer in
			// ApplyMove rotates past them.
			advances++
			events = append(events, sim.Event{
				Type:     sim.EventSpecialTriggered,
				PlayerID: victim,
				Cards:    drawn,
				Detail:   "draw_four",
			})

		case genome.SpecialWild:
			// Wild effect is handled in move generation (always playable)
		}
	}

	for i := 0; i < advances; i++ {
		state.NextPlayer()
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

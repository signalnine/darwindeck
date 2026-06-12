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

// Upkeep performs start-of-turn maintenance: when the deck has emptied and
// the discard pile still holds recyclable cards, shuffle them back into the
// deck (minus the top card) so players who cannot play can still draw.
// Otherwise the game pingpongs MovePass deterministically until MaxTurns and
// gets misclassified as a degenerate timeout. This used to live inside
// GenerateMoves; it was moved here so queries stay pure (the recycle
// shuffles with state.RNG, advancing it).
func (r *Runner) Upkeep(state *sim.GameState, g *genome.Genome) {
	if len(state.Deck) == 0 && len(state.Discard) > 1 {
		refillDeckFromDiscard(state)
	}
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
					Type:     sim.MovePlay,
					Cards:    []sim.Card{hand[i]},
					PlayerID: state.Active,
				})
			}
		}
	}

	// Check for special wild cards that can always be played
	for i, card := range hand {
		if isWild(card, g.SpecialCards) && !alreadyInMoves(moves, card) {
			moves = append(moves, sim.Move{
				Type:     sim.MovePlay,
				Cards:    []sim.Card{hand[i]},
				PlayerID: state.Active,
			})
		}
	}

	// If no playable cards, must draw. The deck is replenished from the
	// discard pile by Upkeep (start of each loop iteration), never here:
	// GenerateMoves is a pure query (audit Task 3). If both the deck and
	// the recyclable discard are exhausted, fall through to Pass.
	if len(moves) == 0 {
		if len(state.Deck) > 0 {
			moves = append(moves, sim.Move{
				Type:     sim.MoveDraw,
				PlayerID: state.Active,
			})
		} else {
			// No deck and no plays — pass
			moves = append(moves, sim.Move{
				Type:     sim.MovePass,
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
		player := state.Active
		// Remove card from hand
		state.Hands[player] = removeCard(state.Hands[player], card)
		// Add to discard pile
		state.Discard = append(state.Discard, card)
		state.TopCard = &sim.Card{Suit: card.Suit, Rank: card.Rank}

		events = append(events, sim.Event{
			Type:     sim.EventCardPlayed,
			PlayerID: player,
			Cards:    []sim.Card{card},
			Detail:   "discard", // Shedding plays go to shared discard pile
		})

		// Apply special card effects
		effects := applySpecialEffects(state, card, g)
		events = append(events, effects...)

		// If this play emptied a hand, the game is over (CheckEnd returns
		// that player as winner on the next loop). Emit EventRoundEnd so
		// borrowed-mechanic hooks gated on HookScoring/HookEndOfRound (e.g.
		// MechAvoidance, MechMeldBonus) actually fire -- without this they
		// silently no-op on Shedding hosts (dd-4ql).
		for i, hand := range state.Hands {
			if len(hand) == 0 {
				events = append(events, sim.Event{
					Type:     sim.EventRoundEnd,
					PlayerID: i,
					Detail:   "hand_empty",
				})
				break
			}
		}

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
// Multiple SpecialCard rules can match the same played card -- mutation
// freely appends specials and never deduplicates by (Type, ByRank, BySuit).
// To keep the simulation outcome aligned with what the rulebook describes,
// each effect category is collected and applied at most once: duplicate
// rules of the same type collapse into a single effect, and combinations
// like Skip + DrawTwo skip the victim exactly once rather than rotating
// two seats past them (cards-czo). This subsumes the partial fix from
// dd-rzo which still allowed advances to accumulate per matching rule.
//
// Victim selection reads state.Direction *after* Reverse has been applied,
// so chained Reverse+DrawN / Reverse+Skip combos target the new
// origin-adjacent player, and starts in Direction=-1 honor the reversed
// play order rather than always targeting origin+1 (dd-itq).
func applySpecialEffects(state *sim.GameState, card sim.Card, g *genome.Genome) []sim.Event {
	var (
		skip      bool
		reverse   bool
		drawCount int
	)

	for _, sc := range g.SpecialCards {
		if !cardMatchesSpecial(card, sc) {
			continue
		}
		switch sc.Type {
		case genome.SpecialSkip:
			skip = true
		case genome.SpecialReverse:
			reverse = true
		case genome.SpecialDrawTwo:
			if drawCount == 0 {
				drawCount = 2
			}
		case genome.SpecialDrawFour:
			if drawCount == 0 {
				drawCount = 4
			}
		case genome.SpecialWild:
			// Wild effect is handled in move generation (always playable)
		}
	}

	// Apply Reverse first so victim/skip lookups use the new direction.
	if reverse {
		if state.Direction == 0 {
			state.Direction = 1
		}
		state.Direction = -state.Direction
	}

	// Compute the victim using the current direction. ApplyMove's trailing
	// NextPlayer is what rotates *to* the victim normally; an extra advance
	// here is what makes them get "skipped".
	dir := state.Direction
	if dir == 0 {
		dir = 1
	}
	victim := ((state.Active+dir)%state.NumPlayers + state.NumPlayers) % state.NumPlayers

	var events []sim.Event

	if drawCount > 0 {
		// If the deck cannot cover the penalty, recycle the discard pile
		// (minus the just-played top card) so the special card actually
		// inflicts cards on the victim. Without this the effect silently
		// no-ops late game when the deck has been exhausted (dd-9jy).
		if len(state.Deck) < drawCount && len(state.Discard) > 1 {
			refillDeckFromDiscard(state)
		}
		drawn, rest := sim.DrawN(state.Deck, drawCount)
		state.Deck = rest
		state.Hands[victim] = append(state.Hands[victim], drawn...)
		detail := "draw_two"
		if drawCount == 4 {
			detail = "draw_four"
		}
		events = append(events, sim.Event{
			Type:     sim.EventSpecialTriggered,
			PlayerID: victim,
			Cards:    drawn,
			Detail:   detail,
		})
	}

	// Skip the victim if any skip-style effect fired. Skip, DrawN (Uno-style
	// "draw and lose your turn"), and Reverse-in-2-player all collapse to one
	// extra advance regardless of how many matching rules contributed.
	skipVictim := skip || drawCount > 0 || (reverse && state.NumPlayers == 2)
	if skipVictim {
		state.NextPlayer()
	}

	if skip {
		events = append(events, sim.Event{
			Type:     sim.EventSpecialTriggered,
			PlayerID: victim,
			Detail:   "skip",
		})
	}
	if reverse {
		events = append(events, sim.Event{
			Type:   sim.EventSpecialTriggered,
			Detail: "reverse",
		})
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

package mechanic

import (
	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

// HookPoint identifies when a hook fires.
type HookPoint int

const (
	HookAfterPlay    HookPoint = iota // After a card is played
	HookEndOfRound                     // After a round/hand ends
	HookScoring                        // During scoring phase
)

// Hook is a borrowed mechanic's behavior injected into a skeleton runner.
type Hook struct {
	Point    HookPoint
	Mechanic genome.MechanicType
	Apply    func(state *sim.GameState, g *genome.Genome, event sim.Event)
}

// BuildHooks creates the hook functions for a genome's borrowed mechanics.
func BuildHooks(g *genome.Genome) []Hook {
	var hooks []Hook

	for _, bm := range g.Borrowed {
		switch bm.Mechanic {
		case genome.MechAvoidance:
			hooks = append(hooks, Hook{
				Point:    HookScoring,
				Mechanic: genome.MechAvoidance,
				Apply:    applyAvoidance,
			})

		case genome.MechMeldBonus:
			hooks = append(hooks, Hook{
				Point:    HookEndOfRound,
				Mechanic: genome.MechMeldBonus,
				Apply:    applyMeldBonus,
			})

		case genome.MechDrawPenalty:
			hooks = append(hooks, Hook{
				Point:    HookAfterPlay,
				Mechanic: genome.MechDrawPenalty,
				Apply:    applyDrawPenalty,
			})

		case genome.MechTrump:
			// Trump is handled structurally (card comparison), not via hook
			// The genome's TrumpRule field already controls this

		case genome.MechTrickScoring:
			hooks = append(hooks, Hook{
				Point:    HookEndOfRound,
				Mechanic: genome.MechTrickScoring,
				Apply:    applyTrickScoring,
			})

		case genome.MechPlayMultiple:
			// Handled in move generation, not via hook
		}
	}

	return hooks
}

// RunHooks executes all hooks matching the given point.
func RunHooks(hooks []Hook, point HookPoint, state *sim.GameState, g *genome.Genome, event sim.Event) {
	for _, h := range hooks {
		if h.Point == point {
			h.Apply(state, g, event)
		}
	}
}

// applyAvoidance adds penalty points for certain cards collected.
// In shedding: cards still in hand at end are penalties.
// In rummy: same as deadwood but with card-specific multipliers.
func applyAvoidance(state *sim.GameState, g *genome.Genome, event sim.Event) {
	if len(g.Scoring.CardPoints) == 0 {
		return
	}

	for i := 0; i < state.NumPlayers; i++ {
		penalty := 0
		// Check cards in hand (shedding/rummy) and captured cards (trick-taking tableau)
		for _, card := range state.Hands[i] {
			penalty += cardPenalty(card, g)
		}
		if i < len(state.Tableau) {
			for _, card := range state.Tableau[i] {
				penalty += cardPenalty(card, g)
			}
		}
		state.Scores[i] -= penalty // Negative = bad
	}
}

// applyMeldBonus awards bonus points for sets/runs in a player's hand or tableau.
func applyMeldBonus(state *sim.GameState, g *genome.Genome, event sim.Event) {
	for i := 0; i < state.NumPlayers; i++ {
		hand := state.Hands[i]
		bonus := 0

		// Check for sets (3+ same rank)
		rankCount := make(map[sim.Rank]int)
		for _, c := range hand {
			rankCount[c.Rank]++
		}
		for _, count := range rankCount {
			if count >= 3 {
				bonus += count * 5 // 5 points per card in a set
			}
		}

		// Check for runs (3+ consecutive same suit)
		suitCards := make(map[sim.Suit][]int)
		for _, c := range hand {
			suitCards[c.Suit] = append(suitCards[c.Suit], int(c.Rank))
		}
		for _, ranks := range suitCards {
			if len(ranks) < 3 {
				continue
			}
			// Sort and find consecutive
			sortInts(ranks)
			run := 1
			for j := 1; j < len(ranks); j++ {
				if ranks[j] == ranks[j-1]+1 {
					run++
				} else {
					if run >= 3 {
						bonus += run * 3 // 3 points per card in a run
					}
					run = 1
				}
			}
			if run >= 3 {
				bonus += run * 3
			}
		}

		state.Scores[i] += bonus
	}
}

// applyDrawPenalty forces the active player to draw extra cards after certain plays.
func applyDrawPenalty(state *sim.GameState, g *genome.Genome, event sim.Event) {
	if event.Type != sim.EventCardPlayed {
		return
	}
	// Draw 1 extra card on high-value plays (face cards)
	if len(event.Cards) > 0 && event.Cards[0].Rank >= sim.Jack {
		if len(state.Deck) > 0 {
			drawn, rest := sim.DrawN(state.Deck, 1)
			state.Deck = rest
			state.Hands[event.PlayerID] = append(state.Hands[event.PlayerID], drawn...)
		}
	}
}

// applyTrickScoring adds trick-like scoring to non-trick games.
// Awards points for playing the highest card each turn cycle.
func applyTrickScoring(state *sim.GameState, g *genome.Genome, event sim.Event) {
	// At end of round, player with most cards captured (in tableau) gets bonus
	maxCapture := 0
	captureWinner := 0
	for i := 0; i < state.NumPlayers; i++ {
		if len(state.Tableau[i]) > maxCapture {
			maxCapture = len(state.Tableau[i])
			captureWinner = i
		}
	}
	if maxCapture > 0 {
		state.Scores[captureWinner] += maxCapture
	}
}

// cardPenalty returns the penalty points for card under g's scoring rules.
// Delegates to genome.MatchCardPoints so penalty resolution stays in lockstep
// with cardPointValue in pkg/skeleton/tricktaking/runner.go (dd-cto).
func cardPenalty(card sim.Card, g *genome.Genome) int {
	return genome.MatchCardPoints(g.Scoring.CardPoints, uint8(card.Rank), uint8(card.Suit))
}

func sortInts(a []int) {
	// Simple insertion sort for small slices
	for i := 1; i < len(a); i++ {
		key := a[i]
		j := i - 1
		for j >= 0 && a[j] > key {
			a[j+1] = a[j]
			j--
		}
		a[j+1] = key
	}
}

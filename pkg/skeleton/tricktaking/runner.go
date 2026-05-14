package tricktaking

import (
	"math/rand/v2"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

// Runner implements the trick-taking game skeleton.
// Players play one card each per trick. Highest card of led suit wins,
// unless trumped. Score based on tricks won or card points.
type Runner struct{}

// TrickState tracks the current trick in progress.
type TrickState struct {
	CardsPlayed []sim.Card // Cards played this trick, indexed by play order
	Players     []int      // Player IDs in play order
	LeadSuit    sim.Suit   // Suit of the first card played
	Leader      int        // Player who led
}

func (r *Runner) Setup(g *genome.Genome, rng *rand.Rand) *sim.GameState {
	deck := sim.StandardDeck()
	sim.ShuffleDeck(deck, rng)

	state := sim.NewGameState(g.Players)

	// Determine trump from the full pre-deal deck so TrumpCut still picks a
	// real suit when HandSize*Players == 52 empties the post-deal remainder
	// (cards-6u5). Other TrumpRule values don't read the deck, so the early
	// call is harmless for them.
	state.TrumpSuit = determineTrump(g, deck, rng)

	// Deal hands
	for i := 0; i < g.Players; i++ {
		hand, rest := sim.DrawN(deck, g.HandSize)
		state.Hands[i] = make([]sim.Card, len(hand))
		copy(state.Hands[i], hand)
		deck = rest
	}

	state.Deck = deck
	state.Phase = sim.PhaseTrick
	state.Round = 0
	if g.TrickTaking != nil {
		state.MaxRound = g.TrickTaking.RoundsPerGame
	} else {
		state.MaxRound = 1
	}
	state.RNG = rng

	// Initialize trick state
	state.TrickCards = make([]sim.Card, 0, g.Players)
	state.TrickPlayers = make([]int, 0, g.Players)
	state.TrickLeader = 0
	state.TrickBroken = false

	return state
}

func determineTrump(g *genome.Genome, deck []sim.Card, rng *rand.Rand) int {
	switch g.TrumpRule {
	case genome.TrumpNone:
		return -1
	case genome.TrumpFixed:
		return int(g.Scoring.TrumpSuit) - 1 // 1-indexed to 0-indexed
	case genome.TrumpCut:
		if len(deck) > 0 {
			return int(deck[0].Suit)
		}
		return -1
	case genome.TrumpLed:
		return -2 // Sentinel: set to first suit led
	default:
		return -1
	}
}

func (r *Runner) GenerateMoves(state *sim.GameState, g *genome.Genome) []sim.Move {
	hand := state.ActiveHand()
	if len(hand) == 0 {
		return nil
	}

	params := g.TrickTaking
	if params == nil {
		// Fallback: play anything
		return playAnyCard(hand, state.Active)
	}

	var moves []sim.Move

	// If leading a new trick
	if len(state.TrickCards) == 0 {
		for _, card := range hand {
			// Check lead restrictions
			if canLead(card, state, params) {
				moves = append(moves, sim.Move{
					Type:     sim.MovePlay,
					Cards:    []sim.Card{card},
					PlayerID: state.Active,
				})
			}
		}
		// If no legal leads (e.g., only trump but not broken), allow any
		if len(moves) == 0 {
			return playAnyCard(hand, state.Active)
		}
		return moves
	}

	// Following — must follow suit if possible
	if params.MustFollowSuit {
		leadSuit := state.TrickCards[0].Suit
		for _, card := range hand {
			if card.Suit == leadSuit {
				moves = append(moves, sim.Move{
					Type:     sim.MovePlay,
					Cards:    []sim.Card{card},
					PlayerID: state.Active,
				})
			}
		}
		if len(moves) > 0 {
			return moves
		}
	}

	// Can't follow suit (or not required) — play anything
	return playAnyCard(hand, state.Active)
}

func canLead(card sim.Card, state *sim.GameState, params *genome.TrickTakingParams) bool {
	if params.LeadRestriction != genome.LeadNoTrumpUntilBroken {
		return true
	}
	if state.TrumpSuit < 0 {
		return true
	}
	if state.TrickBroken {
		return true
	}
	// Can't lead trump until broken
	return int(card.Suit) != state.TrumpSuit
}

func playAnyCard(hand []sim.Card, playerID int) []sim.Move {
	moves := make([]sim.Move, len(hand))
	for i, card := range hand {
		moves[i] = sim.Move{
			Type:     sim.MovePlay,
			Cards:    []sim.Card{card},
			PlayerID: playerID,
		}
	}
	return moves
}

func (r *Runner) ApplyMove(state *sim.GameState, move sim.Move, g *genome.Genome) []sim.Event {
	var events []sim.Event
	card := move.Cards[0]

	// Remove card from hand
	state.Hands[state.Active] = removeCard(state.Hands[state.Active], card)

	// Add to trick
	state.TrickCards = append(state.TrickCards, card)
	state.TrickPlayers = append(state.TrickPlayers, state.Active)

	// Set lead suit on first card
	if len(state.TrickCards) == 1 {
		// Handle trump-led rule
		if state.TrumpSuit == -2 {
			state.TrumpSuit = int(card.Suit)
		}
	}

	// Check if trump was broken (player played trump on a non-trump lead).
	// Following trump on a trump-led trick is not "breaking" trump.
	if state.TrumpSuit >= 0 && int(card.Suit) == state.TrumpSuit {
		if len(state.TrickCards) > 1 && int(state.TrickCards[0].Suit) != state.TrumpSuit {
			state.TrickBroken = true
		}
	}

	events = append(events, sim.Event{
		Type:     sim.EventCardPlayed,
		PlayerID: state.Active,
		Cards:    []sim.Card{card},
	})

	state.Turn++

	// Check if trick is complete
	if len(state.TrickCards) == state.NumPlayers {
		winner := resolveTrick(state, g)
		state.Tableau[winner] = append(state.Tableau[winner], state.TrickCards...)

		// Score the trick
		scoreTrick(state, winner, g)

		trickCardsCopy := make([]sim.Card, len(state.TrickCards))
		copy(trickCardsCopy, state.TrickCards)
		events = append(events, sim.Event{
			Type:     sim.EventTrickWon,
			PlayerID: winner,
			Cards:    trickCardsCopy,
		})

		// Reset trick
		state.TrickCards = state.TrickCards[:0]
		state.TrickPlayers = state.TrickPlayers[:0]
		state.Active = winner
		state.TrickLeader = winner
	} else {
		state.NextPlayer()
	}

	return events
}

func resolveTrick(state *sim.GameState, g *genome.Genome) int {
	leadSuit := state.TrickCards[0].Suit
	bestIdx := 0
	bestRank := state.TrickCards[0].Rank
	bestIsTrump := state.TrumpSuit >= 0 && int(leadSuit) == state.TrumpSuit

	for i := 1; i < len(state.TrickCards); i++ {
		card := state.TrickCards[i]
		isTrump := state.TrumpSuit >= 0 && int(card.Suit) == state.TrumpSuit

		if isTrump && !bestIsTrump {
			// Trump beats non-trump
			bestIdx = i
			bestRank = card.Rank
			bestIsTrump = true
		} else if isTrump && bestIsTrump {
			// Both trump — higher rank wins
			if card.Rank > bestRank {
				bestIdx = i
				bestRank = card.Rank
			}
		} else if !isTrump && !bestIsTrump && card.Suit == leadSuit {
			// Both following suit — higher rank wins
			if card.Rank > bestRank {
				bestIdx = i
				bestRank = card.Rank
			}
		}
		// Off-suit non-trump card can't win
	}

	return state.TrickPlayers[bestIdx]
}

func scoreTrick(state *sim.GameState, winner int, g *genome.Genome) {
	if g.TrickTaking == nil {
		return
	}

	switch g.TrickTaking.TrickScoring {
	case genome.ScorePerTrick:
		state.Scores[winner]++

	case genome.ScoreCardPoints, genome.ScoreAvoidance:
		points := 0
		for _, card := range state.TrickCards {
			points += cardPointValue(card, g)
		}
		state.Scores[winner] += points
	}
}

// cardPointValue returns the point value of card under g's scoring rules.
// Delegates to genome.MatchCardPoints; the shared helper resolves overlapping
// rules by specificity (suit+rank > suit-only > rank-only > catch-all) so
// mutators or crossover that permute the slice cannot change a card's score.
func cardPointValue(card sim.Card, g *genome.Genome) int {
	return genome.MatchCardPoints(g.Scoring.CardPoints, uint8(card.Rank), uint8(card.Suit))
}

func (r *Runner) CheckEnd(state *sim.GameState, g *genome.Genome) int {
	// Check if all players have empty hands (round over)
	allEmpty := true
	for _, hand := range state.Hands {
		if len(hand) > 0 {
			allEmpty = false
			break
		}
	}

	if !allEmpty {
		// At max turns with hands still in play, return -1 so the batch runner
		// classifies this as a genuine timeout rather than a completion.
		// Awarding findWinner here would mask hung trick-taking genomes from
		// Tier1 timeout detection (matches shedding and rummy behavior).
		return -1
	}

	// Round complete
	state.Round++
	if state.Round >= state.MaxRound {
		return findWinner(state, g)
	}

	redealRound(state, g)
	return -1
}

// redealRound prepares state for the next round of a multi-round game:
// gather all played cards, shuffle, deal fresh hands, and reset trick
// state. Cumulative scores carry across rounds; per-round trick piles do not.
func redealRound(state *sim.GameState, g *genome.Genome) {
	deck := make([]sim.Card, 0, 52)
	for i := range state.Tableau {
		deck = append(deck, state.Tableau[i]...)
		state.Tableau[i] = state.Tableau[i][:0]
	}
	deck = append(deck, state.Deck...)
	deck = append(deck, state.Discard...)
	state.Discard = state.Discard[:0]

	if state.RNG != nil {
		sim.ShuffleDeck(deck, state.RNG)
	}

	// Cut for trump from the full pre-deal deck so TrumpCut keeps a real
	// suit when the round-end re-deal empties the deck (cards-6u5).
	if state.RNG != nil {
		state.TrumpSuit = determineTrump(g, deck, state.RNG)
	}

	for i := 0; i < g.Players; i++ {
		hand, rest := sim.DrawN(deck, g.HandSize)
		state.Hands[i] = append(state.Hands[i][:0], hand...)
		deck = rest
	}
	state.Deck = deck

	state.TrickCards = state.TrickCards[:0]
	state.TrickPlayers = state.TrickPlayers[:0]
	state.TrickBroken = false
	state.TrickLeader = state.Round % state.NumPlayers
	state.Active = state.TrickLeader
}

func findWinner(state *sim.GameState, g *genome.Genome) int {
	if g.TrickTaking != nil && g.TrickTaking.TrickScoring == genome.ScoreAvoidance {
		// Lowest score wins (Hearts-style)
		minScore := state.Scores[0]
		winner := 0
		for i := 1; i < state.NumPlayers; i++ {
			if state.Scores[i] < minScore {
				minScore = state.Scores[i]
				winner = i
			}
		}
		return winner
	}

	// Highest score wins
	maxScore := state.Scores[0]
	winner := 0
	for i := 1; i < state.NumPlayers; i++ {
		if state.Scores[i] > maxScore {
			maxScore = state.Scores[i]
			winner = i
		}
	}
	return winner
}

func removeCard(hand []sim.Card, card sim.Card) []sim.Card {
	for i, c := range hand {
		if c == card {
			return append(hand[:i], hand[i+1:]...)
		}
	}
	return hand
}

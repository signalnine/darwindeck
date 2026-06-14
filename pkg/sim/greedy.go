package sim

import (
	"math/rand/v2"

	"github.com/darwindeck/darwindeck/pkg/genome"
)

// GreedyAI selects moves using skeleton-aware heuristics.
// Each move is scored and the highest-scoring move is chosen,
// with random tie-breaking.
type GreedyAI struct {
	Scorer MoveScorer
}

// MoveScorer scores a move in context. Higher = better.
type MoveScorer interface {
	ScoreMove(move Move, state *GameState) float64
}

func (ai *GreedyAI) SelectMove(moves []Move, state *GameState, rng *rand.Rand) Move {
	if len(moves) == 0 {
		return Move{Type: MovePass}
	}
	if len(moves) == 1 {
		return moves[0]
	}

	bestScore := ai.Scorer.ScoreMove(moves[0], state)
	bestMoves := []Move{moves[0]}

	for _, m := range moves[1:] {
		score := ai.Scorer.ScoreMove(m, state)
		if score > bestScore {
			bestScore = score
			bestMoves = bestMoves[:0]
			bestMoves = append(bestMoves, m)
		} else if score == bestScore {
			bestMoves = append(bestMoves, m)
		}
	}

	if len(bestMoves) == 1 {
		return bestMoves[0]
	}
	return bestMoves[rng.IntN(len(bestMoves))]
}

// --- Shedding Greedy Scorer ---

// SheddingScorer scores shedding moves.
// Strategy: play cards that minimize future draws by keeping flexible cards
// (those that share suit/rank with many others). Time specials strategically.
type SheddingScorer struct {
	// Genome supplies the SpecialCards rules used to classify offensive
	// specials (skip/reverse/draw-N). A nil Genome means the scorer has no
	// special-card knowledge and applies no special-timing bonus -- it does
	// NOT fall back to the old hardcoded 2/7/10 rank heuristic, which the
	// 2026-06-11 audit flagged as genome-blind.
	Genome *genome.Genome
}

// NewSheddingScorer builds a shedding scorer that classifies special cards
// from the genome's SpecialCards instead of hardcoded ranks.
func NewSheddingScorer(g *genome.Genome) *SheddingScorer {
	return &SheddingScorer{Genome: g}
}

func (s *SheddingScorer) ScoreMove(move Move, state *GameState) float64 {
	switch move.Type {
	case MovePlay:
		if len(move.Cards) == 0 {
			return 0
		}
		card := move.Cards[0]
		score := 10.0 // Base: playing is better than drawing

		hand := state.Hands[move.PlayerID]

		// Count how many OTHER cards in hand this card connects to.
		// High connections = flexible card = keep it for later.
		// Low connections = isolated card = play it now.
		connections := 0
		for _, h := range hand {
			if h == card {
				continue
			}
			if h.Suit == card.Suit || h.Rank == card.Rank {
				connections++
			}
		}
		// Play isolated cards first (saves flexible ones for later)
		score -= float64(connections) * 2.0

		// If opponent is close to winning (few cards), prefer specials
		// that disrupt them (draw-two, skip).
		// Check if this is a special card that hurts opponent.
		if s.isOffensiveSpecial(card) {
			for i := 0; i < state.NumPlayers; i++ {
				if i == move.PlayerID {
					continue
				}
				opponentCards := len(state.Hands[i])
				if opponentCards <= 2 {
					score += 15.0 // High priority: block their win
				} else if opponentCards <= 4 {
					score += 5.0
				}
			}
		}

		return score

	case MoveDraw:
		return 0 // Drawing is worst option

	case MovePass:
		return -1

	default:
		return 0
	}
}

// isOffensiveSpecial reports whether the genome assigns this card an effect
// that disrupts opponents (skip, reverse, draw-two, draw-four). Wild is
// excluded: it is flexibility for the holder, not an attack. Matching
// mirrors the shedding runner's cardMatchesSpecial semantics: ByRank 0 = any
// rank, BySuit 0 = any suit, otherwise BySuit is suit+1.
func (s *SheddingScorer) isOffensiveSpecial(card Card) bool {
	if s.Genome == nil {
		return false
	}
	for _, sc := range s.Genome.SpecialCards {
		if sc.ByRank != 0 && sc.ByRank != uint8(card.Rank) {
			continue
		}
		if sc.BySuit != 0 && sc.BySuit != uint8(card.Suit)+1 {
			continue
		}
		switch sc.Type {
		case genome.SpecialSkip, genome.SpecialReverse,
			genome.SpecialDrawTwo, genome.SpecialDrawFour:
			return true
		}
	}
	return false
}

// --- Trick-Taking Greedy Scorer ---

// TrickTakingScorer scores trick-taking moves.
//
// Trump suit is read from state.TrumpSuit at scoring time rather than
// configured statically. This is required for TrumpCut and TrumpLed games
// where trump is determined at runtime, not from the genome.
type TrickTakingScorer struct {
	Avoidance bool // True for Hearts-style (don't want to win tricks with point cards)
}

// effectiveTrump returns the trump suit known to the scorer at scoring time.
// state.TrumpSuit uses -1 for "no trump" and -2 for "pending" (TrumpLed before
// the first card is led); both collapse to -1 here, since the scorer cannot
// reason about an unknown trump.
func (s *TrickTakingScorer) effectiveTrump(state *GameState) int {
	if state.TrumpSuit < 0 {
		return -1
	}
	return state.TrumpSuit
}

func (s *TrickTakingScorer) ScoreMove(move Move, state *GameState) float64 {
	if move.Type != MovePlay || len(move.Cards) == 0 {
		return 0
	}

	card := move.Cards[0]
	isLeading := len(state.TrickCards) == 0

	if s.Avoidance {
		return s.scoreAvoidance(card, state, isLeading)
	}
	return s.scoreWinning(card, state, isLeading)
}

func (s *TrickTakingScorer) scoreWinning(card Card, state *GameState, isLeading bool) float64 {
	trump := s.effectiveTrump(state)
	isTrump := trump >= 0 && int(card.Suit) == trump

	if isLeading {
		// Lead high cards to win tricks
		score := float64(card.Rank) * 0.5
		if isTrump {
			score += 5 // Lead trump to pull them out
		}
		return score
	}

	// Following: try to win with lowest winning card
	if s.wouldWin(card, state) {
		// Win, but prefer winning with low cards (save high cards)
		return 20.0 - float64(card.Rank)*0.5
	}

	// Can't win: dump lowest card
	return -float64(card.Rank) * 0.5
}

func (s *TrickTakingScorer) scoreAvoidance(card Card, state *GameState, isLeading bool) float64 {
	if isLeading {
		// Lead low cards, avoid leading suits with penalty cards
		return -float64(card.Rank) * 0.5
	}

	// Following: try NOT to win if trick has penalty cards
	if s.wouldWin(card, state) {
		// Winning is bad in avoidance — heavily penalize
		return -20.0 - float64(card.Rank)*0.5
	}

	// Can't win: dump high cards (get rid of dangerous cards)
	return float64(card.Rank) * 0.5
}

func (s *TrickTakingScorer) wouldWin(card Card, state *GameState) bool {
	if len(state.TrickCards) == 0 {
		return true // Leading always "wins" so far
	}

	trump := s.effectiveTrump(state)
	leadSuit := state.TrickCards[0].Suit
	isTrump := trump >= 0 && int(card.Suit) == trump

	// Find current best
	bestRank := state.TrickCards[0].Rank
	bestIsTrump := trump >= 0 && int(state.TrickCards[0].Suit) == trump

	for _, tc := range state.TrickCards[1:] {
		tcTrump := trump >= 0 && int(tc.Suit) == trump
		if tcTrump && !bestIsTrump {
			bestRank = tc.Rank
			bestIsTrump = true
		} else if tcTrump && bestIsTrump && tc.Rank > bestRank {
			bestRank = tc.Rank
		} else if !tcTrump && !bestIsTrump && tc.Suit == leadSuit && tc.Rank > bestRank {
			bestRank = tc.Rank
		}
	}

	if isTrump && !bestIsTrump {
		return true
	}
	if isTrump && bestIsTrump {
		return card.Rank > bestRank
	}
	if card.Suit == leadSuit && !bestIsTrump {
		return card.Rank > bestRank
	}
	return false
}

// --- Rummy Greedy Scorer ---

// RummyScorer scores rummy moves.
type RummyScorer struct{}

func (s *RummyScorer) ScoreMove(move Move, state *GameState) float64 {
	switch move.Type {
	case MoveMeld:
		// Always meld if possible — reduces hand, reduces deadwood
		return 100.0 + float64(len(move.Cards))*10

	case MoveKnock:
		// Knock is great — ends the round when you're ahead
		return 200.0

	case MoveDraw:
		if len(move.Cards) > 0 {
			// Drawing from discard: prefer if card helps form melds
			card := move.Cards[0]
			hand := state.Hands[state.Active]
			helpfulness := s.cardMeldPotential(card, hand)
			return 5.0 + helpfulness
		}
		// Drawing from deck: baseline
		return 3.0

	case MoveDiscard:
		if len(move.Cards) == 0 {
			return 0
		}
		card := move.Cards[0]
		hand := state.Hands[state.Active]

		// Discard highest deadwood card that doesn't contribute to melds
		potential := s.cardMeldPotential(card, hand)
		deadwood := float64(cardDeadwood(card))

		// High deadwood + low meld potential = good discard
		return deadwood - potential*5

	case MovePass:
		return -1

	default:
		return 0
	}
}

// cardMeldPotential estimates how useful a card is for forming melds.
func (s *RummyScorer) cardMeldPotential(card Card, hand []Card) float64 {
	score := 0.0

	for _, h := range hand {
		if h == card {
			continue
		}
		// Same rank = potential set
		if h.Rank == card.Rank {
			score += 2.0
		}
		// Same suit + adjacent rank = potential run
		if h.Suit == card.Suit {
			diff := int(h.Rank) - int(card.Rank)
			if diff < 0 {
				diff = -diff
			}
			if diff == 1 {
				score += 2.0 // Adjacent
			} else if diff == 2 {
				score += 0.5 // Gap of 1
			}
		}
	}

	return score
}

func cardDeadwood(c Card) int {
	switch {
	case c.Rank == Ace:
		return 1
	case c.Rank >= Ten:
		return 10
	default:
		return int(c.Rank)
	}
}

// --- Climbing Greedy Scorer ---

// ClimbingScorer scores climbing/ladder moves with a conserve-high-cards
// heuristic (the strategy the task specifies):
//
//   - When LEADING a clear table, dump the LOWEST combination first -- play out
//     weak cards while you control the table, and prefer playing more cards at
//     once (bigger combos shrink the hand faster) lightly over the rank
//     preference.
//   - When FOLLOWING, play the LOWEST beating combination (just clear the bar,
//     save high cards for later), and treat PASS as a low-value fallback so a
//     play is preferred when one is cheap. The pass baseline sits just below a
//     low beat but above an expensive high-card beat, so the AI passes rather
//     than burn its strongest cards "when behind".
//
// State is read from GameState.TrickCards (empty => leading) at scoring time,
// like the other scorers. Higher score = better.
type ClimbingScorer struct{}

func (s *ClimbingScorer) ScoreMove(move Move, state *GameState) float64 {
	switch move.Type {
	case MovePlay:
		if len(move.Cards) == 0 {
			return 0
		}
		top := comboTopRank(move.Cards)
		leading := len(state.TrickCards) == 0
		if leading {
			// Dump low first; reward shedding more cards at once slightly.
			return 30.0 - float64(top) + float64(len(move.Cards))*0.25
		}
		// Following: play the lowest beating combo. Lower top rank => higher
		// score, so the cheapest legal beat wins. The constant keeps every beat
		// above the pass baseline UNLESS the beat needs a very high card, at
		// which point passing (conserve) becomes preferable.
		return 20.0 - float64(top)

	case MovePass:
		// Conserve: passing beats burning a high card (top rank ~>= 12), but a
		// cheap low beat (top rank <= 11) is preferred over passing.
		return 8.0

	default:
		return 0
	}
}

// comboTopRank returns the highest rank in a combination (its comparison rank
// for runs, the shared rank for sets, the card rank for singles).
func comboTopRank(cards []Card) Rank {
	top := cards[0].Rank
	for _, c := range cards[1:] {
		if c.Rank > top {
			top = c.Rank
		}
	}
	return top
}

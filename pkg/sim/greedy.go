package sim

import "math/rand/v2"

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
type SheddingScorer struct{}

func (s *SheddingScorer) ScoreMove(move Move, state *GameState) float64 {
	switch move.Type {
	case MovePlay:
		card := move.Cards[0]
		score := 10.0 // Base: playing is better than drawing

		// Prefer playing high-rank cards (harder to match later)
		score += float64(card.Rank) * 0.5

		// Prefer cards that aren't "connectors" (share suit/rank with other hand cards)
		// This keeps more flexible cards for later
		hand := state.Hands[move.PlayerID]
		connections := 0
		for _, h := range hand {
			if h == card {
				continue
			}
			if h.Suit == card.Suit || h.Rank == card.Rank {
				connections++
			}
		}
		score -= float64(connections) * 0.3

		return score

	case MoveDraw:
		return 0 // Drawing is worst option

	case MovePass:
		return -1

	default:
		return 0
	}
}

// --- Trick-Taking Greedy Scorer ---

// TrickTakingScorer scores trick-taking moves.
type TrickTakingScorer struct {
	Avoidance bool // True for Hearts-style (don't want to win tricks with point cards)
	TrumpSuit int  // -1 = no trump
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
	isTrump := s.TrumpSuit >= 0 && int(card.Suit) == s.TrumpSuit

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

	leadSuit := state.TrickCards[0].Suit
	isTrump := s.TrumpSuit >= 0 && int(card.Suit) == s.TrumpSuit

	// Find current best
	bestRank := state.TrickCards[0].Rank
	bestIsTrump := s.TrumpSuit >= 0 && int(state.TrickCards[0].Suit) == s.TrumpSuit

	for _, tc := range state.TrickCards[1:] {
		tcTrump := s.TrumpSuit >= 0 && int(tc.Suit) == s.TrumpSuit
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
	case c.Rank >= Ten:
		return 10
	case c.Rank == Ace:
		return 1
	default:
		return int(c.Rank)
	}
}

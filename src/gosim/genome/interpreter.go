package genome

import (
	"github.com/signalnine/darwindeck/gosim/engine"
)

// GenerateLegalMovesTyped generates legal moves using typed phases directly.
// This is the direct AST interpretation approach - no bytecode parsing needed.
func GenerateLegalMovesTyped(state *engine.GameState, genome *GameGenome) []engine.LegalMove {
	moves := make([]engine.LegalMove, 0, 10)
	currentPlayer := state.CurrentPlayer

	for phaseIdx, phase := range genome.TurnStructure.Phases {
		switch p := phase.(type) {
		case *DrawPhase:
			moves = appendDrawMoves(moves, state, currentPlayer, phaseIdx, p)

		case *PlayPhase:
			moves = appendPlayMoves(moves, state, currentPlayer, phaseIdx, p, genome)

		case *DiscardPhase:
			moves = appendDiscardMoves(moves, state, currentPlayer, phaseIdx, p)

		case *TrickPhase:
			moves = appendTrickMoves(moves, state, currentPlayer, phaseIdx, p)

		case *BettingPhase:
			moves = appendBettingMoves(moves, state, currentPlayer, phaseIdx, p)

		case *ClaimPhase:
			moves = appendClaimMoves(moves, state, currentPlayer, phaseIdx)

		case *BiddingPhase:
			moves = appendBiddingMoves(moves, state, currentPlayer, phaseIdx, p)
		}
	}

	return moves
}

// appendDrawMoves adds legal draw moves for a DrawPhase.
// Compare to movegen.go case 1 - this reads struct fields directly instead of phase.Data bytes.
func appendDrawMoves(moves []engine.LegalMove, state *engine.GameState, currentPlayer uint8, phaseIdx int, p *DrawPhase) []engine.LegalMove {
	// Skip if player has already stood (blackjack)
	if int(currentPlayer) < len(state.HasStood) && state.HasStood[currentPlayer] {
		return moves
	}

	// Check phase condition if present
	if p.Condition != nil {
		conditionMet := evaluateConditionTyped(state, currentPlayer, p.Condition)
		if !conditionMet {
			return moves // Skip this phase if condition not met
		}
	}

	// Check if can draw, with automatic deck reshuffling
	canDraw := false
	source := engine.Location(p.Source)
	switch source {
	case engine.LocationDeck:
		// If deck is empty but discard has cards, reshuffle would happen
		if len(state.Deck) == 0 && len(state.Discard) > 1 {
			// reshuffleDeck would be called - for now just check
			canDraw = true // Would reshuffle
		}
		canDraw = canDraw || len(state.Deck) > 0
	case engine.LocationDiscard:
		canDraw = len(state.Discard) > 0
	case engine.LocationOpponentHand:
		opponentID := (currentPlayer + 1) % state.NumPlayers
		canDraw = len(state.Players[opponentID].Hand) > 0
	}

	if canDraw {
		moves = append(moves, engine.LegalMove{
			PhaseIndex: phaseIdx,
			CardIndex:  engine.MoveDraw, // -1 = draw (hit)
			TargetLoc:  source,
		})
	}

	// Add pass/stand option when drawing is not mandatory
	if !p.Mandatory && canDraw {
		moves = append(moves, engine.LegalMove{
			PhaseIndex: phaseIdx,
			CardIndex:  engine.MoveDrawPass, // -3 = pass (stand)
			TargetLoc:  source,
		})
	}

	return moves
}

// appendPlayMoves adds legal play moves for a PlayPhase.
// Compare to movegen.go case 2 - reads p.Target, p.MinCards, etc. directly.
func appendPlayMoves(moves []engine.LegalMove, state *engine.GameState, currentPlayer uint8, phaseIdx int, p *PlayPhase, genome *GameGenome) []engine.LegalMove {
	target := engine.Location(p.Target)
	hand := state.Players[currentPlayer].Hand
	if len(hand) == 0 {
		return moves
	}

	playMoveCount := 0

	// SEQUENCE mode: special handling for tableau plays
	if state.TableauMode == 3 && target == engine.LocationTableau {
		moves, playMoveCount = appendSequenceMoves(moves, state, currentPlayer, phaseIdx, p, hand, target)

		// If no valid plays but pass_if_unable is set, add pass move
		if playMoveCount == 0 && p.PassIfUnable {
			moves = append(moves, engine.LegalMove{
				PhaseIndex: phaseIdx,
				CardIndex:  engine.MovePlayPass,
				TargetLoc:  target,
			})
		}
		return moves
	}

	// Single-card plays (standard)
	if p.MinCards <= 1 && p.MaxCards >= 1 {
		for cardIdx, card := range hand {
			// Evaluate valid_play_condition if present
			if p.ValidPlayCondition != nil {
				if !evaluateCardConditionTyped(state, currentPlayer, card, p.ValidPlayCondition) {
					continue
				}
			}
			moves = append(moves, engine.LegalMove{
				PhaseIndex: phaseIdx,
				CardIndex:  cardIdx,
				TargetLoc:  target,
			})
			playMoveCount++
		}
	}

	// Multi-card plays (Go Fish sets)
	if p.MinCards > 1 {
		rankCounts := make(map[uint8]int)
		for _, card := range hand {
			rankCounts[card.Rank]++
		}

		for rank, count := range rankCounts {
			if count >= p.MinCards && count <= p.MaxCards {
				moves = append(moves, engine.LegalMove{
					PhaseIndex: phaseIdx,
					CardIndex:  -int(rank) - 100,
					TargetLoc:  target,
				})
				playMoveCount++
			}
		}
	}

	// If no valid plays but pass_if_unable is set, add pass move
	if playMoveCount == 0 && p.PassIfUnable {
		moves = append(moves, engine.LegalMove{
			PhaseIndex: phaseIdx,
			CardIndex:  engine.MovePlayPass,
			TargetLoc:  target,
		})
	}

	return moves
}

func appendSequenceMoves(moves []engine.LegalMove, state *engine.GameState, currentPlayer uint8, phaseIdx int, p *PlayPhase, hand []engine.Card, target engine.Location) ([]engine.LegalMove, int) {
	playMoveCount := 0

	// Check if all piles are empty
	allPilesEmpty := true
	for _, pile := range state.Tableau {
		if len(pile) > 0 {
			allPilesEmpty = false
			break
		}
	}

	if allPilesEmpty || len(state.Tableau) == 0 {
		// Empty tableau: any card can start a new pile
		for cardIdx, card := range hand {
			if p.ValidPlayCondition != nil {
				if !evaluateCardConditionTyped(state, currentPlayer, card, p.ValidPlayCondition) {
					continue
				}
			}
			moves = append(moves, engine.LegalMove{
				PhaseIndex: phaseIdx,
				CardIndex:  cardIdx,
				TargetLoc:  target,
			})
			playMoveCount++
		}
	} else {
		// Non-empty tableau: check each card against all piles
		addedCards := make(map[int]bool)

		for cardIdx, card := range hand {
			if p.ValidPlayCondition != nil {
				if !evaluateCardConditionTyped(state, currentPlayer, card, p.ValidPlayCondition) {
					continue
				}
			}

			canPlayOnExisting := false
			for _, pile := range state.Tableau {
				if len(pile) > 0 {
					topCard := pile[len(pile)-1]
					if isValidSequencePlayTyped(card, topCard, state.SequenceDirection) {
						canPlayOnExisting = true
						break
					}
				}
			}

			canStartNewPile := false
			for _, pile := range state.Tableau {
				if len(pile) == 0 {
					canStartNewPile = true
					break
				}
			}

			if (canPlayOnExisting || canStartNewPile) && !addedCards[cardIdx] {
				moves = append(moves, engine.LegalMove{
					PhaseIndex: phaseIdx,
					CardIndex:  cardIdx,
					TargetLoc:  target,
				})
				addedCards[cardIdx] = true
				playMoveCount++
			}
		}
	}

	return moves, playMoveCount
}

func appendDiscardMoves(moves []engine.LegalMove, state *engine.GameState, currentPlayer uint8, phaseIdx int, p *DiscardPhase) []engine.LegalMove {
	if len(state.Players[currentPlayer].Hand) > 0 {
		for cardIdx := range state.Players[currentPlayer].Hand {
			moves = append(moves, engine.LegalMove{
				PhaseIndex: phaseIdx,
				CardIndex:  cardIdx,
				TargetLoc:  engine.LocationDiscard,
			})
		}
	}
	return moves
}

func appendTrickMoves(moves []engine.LegalMove, state *engine.GameState, currentPlayer uint8, phaseIdx int, p *TrickPhase) []engine.LegalMove {
	hand := state.Players[currentPlayer].Hand
	if len(hand) == 0 {
		return moves
	}

	isLeading := len(state.CurrentTrick) == 0

	if isLeading {
		for cardIdx, card := range hand {
			if p.BreakingSuit != 255 && card.Suit == p.BreakingSuit && !state.HeartsBroken {
				hasOther := false
				for _, c := range hand {
					if c.Suit != p.BreakingSuit {
						hasOther = true
						break
					}
				}
				if hasOther {
					continue
				}
			}
			moves = append(moves, engine.LegalMove{
				PhaseIndex: phaseIdx,
				CardIndex:  cardIdx,
				TargetLoc:  engine.LocationTableau,
			})
		}
	} else {
		leadSuit := state.CurrentTrick[0].Card.Suit

		if p.LeadSuitRequired {
			hasLeadSuit := false
			for _, card := range hand {
				if card.Suit == leadSuit {
					hasLeadSuit = true
					break
				}
			}

			if hasLeadSuit {
				for cardIdx, card := range hand {
					if card.Suit == leadSuit {
						moves = append(moves, engine.LegalMove{
							PhaseIndex: phaseIdx,
							CardIndex:  cardIdx,
							TargetLoc:  engine.LocationTableau,
						})
					}
				}
			} else {
				for cardIdx := range hand {
					moves = append(moves, engine.LegalMove{
						PhaseIndex: phaseIdx,
						CardIndex:  cardIdx,
						TargetLoc:  engine.LocationTableau,
					})
				}
			}
		} else {
			for cardIdx := range hand {
				moves = append(moves, engine.LegalMove{
					PhaseIndex: phaseIdx,
					CardIndex:  cardIdx,
					TargetLoc:  engine.LocationTableau,
				})
			}
		}
	}

	return moves
}

func appendBettingMoves(moves []engine.LegalMove, state *engine.GameState, currentPlayer uint8, phaseIdx int, p *BettingPhase) []engine.LegalMove {
	if state.BettingComplete {
		return moves
	}

	activePlayers := engine.CountActivePlayers(state)
	if activePlayers <= 1 {
		state.BettingComplete = true
		return moves
	}

	if engine.AllBetsMatched(state) && engine.CountActingPlayers(state) == 0 {
		state.BettingComplete = true
		return moves
	}

	// Convert typed BettingPhase to engine.BettingPhaseData for compatibility
	bettingData := &engine.BettingPhaseData{
		MinBet:    p.MinBet,
		MaxRaises: p.MaxRaises,
	}

	bettingMoves := engine.GenerateBettingMoves(state, bettingData, int(currentPlayer))

	for _, action := range bettingMoves {
		moves = append(moves, engine.LegalMove{
			PhaseIndex: phaseIdx,
			CardIndex:  -10 - int(action),
			TargetLoc:  engine.LocationDeck,
		})
	}

	return moves
}

func appendClaimMoves(moves []engine.LegalMove, state *engine.GameState, currentPlayer uint8, phaseIdx int) []engine.LegalMove {
	if state.CurrentClaim == nil {
		hand := state.Players[currentPlayer].Hand
		if len(hand) > 0 {
			for cardIdx := range hand {
				moves = append(moves, engine.LegalMove{
					PhaseIndex: phaseIdx,
					CardIndex:  cardIdx,
					TargetLoc:  engine.LocationDiscard,
				})
			}
		}
	} else {
		if currentPlayer != state.CurrentClaim.ClaimerID {
			moves = append(moves, engine.LegalMove{
				PhaseIndex: phaseIdx,
				CardIndex:  engine.MoveChallenge,
				TargetLoc:  engine.LocationDiscard,
			})
			moves = append(moves, engine.LegalMove{
				PhaseIndex: phaseIdx,
				CardIndex:  engine.MovePass,
				TargetLoc:  engine.LocationDiscard,
			})
		}
	}
	return moves
}

func appendBiddingMoves(moves []engine.LegalMove, state *engine.GameState, currentPlayer uint8, phaseIdx int, p *BiddingPhase) []engine.LegalMove {
	if state.BiddingComplete {
		return moves
	}

	if state.Players[currentPlayer].CurrentBid >= 0 {
		return moves
	}

	// Convert to engine.BiddingPhase for compatibility
	enginePhase := engine.BiddingPhase{
		MinBid:   p.MinBid,
		MaxBid:   p.MaxBid,
		AllowNil: p.AllowNil,
	}

	handSize := len(state.Players[currentPlayer].Hand)
	bidMoves := engine.GenerateBidMoves(enginePhase, handSize)

	for _, bid := range bidMoves {
		cardIndex := engine.MoveBidOffset - bid.Value
		targetLoc := engine.LocationDeck
		if bid.IsNil {
			targetLoc = engine.LocationDiscard
		}
		moves = append(moves, engine.LegalMove{
			PhaseIndex: phaseIdx,
			CardIndex:  cardIndex,
			TargetLoc:  targetLoc,
		})
	}

	return moves
}

// evaluateConditionTyped evaluates a condition using typed struct instead of bytes.
func evaluateConditionTyped(state *engine.GameState, playerID uint8, cond *Condition) bool {
	if cond == nil {
		return true
	}

	// Build condition bytes for existing EvaluateCondition function
	// This is a temporary bridge during the transition
	condBytes := make([]byte, 7)
	condBytes[0] = cond.OpCode
	condBytes[1] = cond.Operator
	// Value as 4 bytes big-endian
	condBytes[2] = byte(cond.Value >> 24)
	condBytes[3] = byte(cond.Value >> 16)
	condBytes[4] = byte(cond.Value >> 8)
	condBytes[5] = byte(cond.Value)
	condBytes[6] = cond.RefLoc

	return engine.EvaluateCondition(state, playerID, condBytes)
}

// evaluateCardConditionTyped evaluates a card condition using typed struct.
func evaluateCardConditionTyped(state *engine.GameState, playerID uint8, card engine.Card, cond *Condition) bool {
	if cond == nil {
		return true
	}

	condBytes := make([]byte, 7)
	condBytes[0] = cond.OpCode
	condBytes[1] = cond.Operator
	condBytes[2] = byte(cond.Value >> 24)
	condBytes[3] = byte(cond.Value >> 16)
	condBytes[4] = byte(cond.Value >> 8)
	condBytes[5] = byte(cond.Value)
	condBytes[6] = cond.RefLoc

	return engine.EvaluateCardCondition(state, playerID, card, condBytes)
}

// isValidSequencePlayTyped checks sequence validity using typed direction.
func isValidSequencePlayTyped(card engine.Card, topCard engine.Card, direction uint8) bool {
	if card.Suit != topCard.Suit {
		return false
	}

	switch direction {
	case 0: // ASCENDING
		if topCard.Rank == 13 {
			return false
		}
		return card.Rank == topCard.Rank+1
	case 1: // DESCENDING
		if topCard.Rank == 2 {
			return false
		}
		return card.Rank == topCard.Rank-1
	case 2: // BOTH
		canAscend := topCard.Rank != 13 && card.Rank == topCard.Rank+1
		canDescend := topCard.Rank != 2 && card.Rank == topCard.Rank-1
		return canAscend || canDescend
	}
	return false
}

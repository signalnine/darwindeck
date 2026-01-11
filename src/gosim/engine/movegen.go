package engine

import "encoding/binary"

// LegalMove represents a possible action
type LegalMove struct {
	PhaseIndex int
	CardIndex  int // -1 if not card-specific
	TargetLoc  Location
}

// GenerateLegalMoves returns all valid moves for current player
func GenerateLegalMoves(state *GameState, genome *Genome) []LegalMove {
	moves := make([]LegalMove, 0, 10)
	currentPlayer := state.CurrentPlayer

	for phaseIdx, phase := range genome.TurnPhases {
		switch phase.PhaseType {
		case 1: // DrawPhase
			if len(phase.Data) < 6 {
				continue
			}
			source := Location(phase.Data[0])
			mandatory := phase.Data[5] == 1

			// Check if can draw
			canDraw := false
			switch source {
			case LocationDeck:
				canDraw = len(state.Deck) > 0
			case LocationDiscard:
				canDraw = len(state.Discard) > 0
			case LocationOpponentHand:
				opponentID := 1 - currentPlayer
				canDraw = len(state.Players[opponentID].Hand) > 0
			}

			if canDraw || mandatory {
				moves = append(moves, LegalMove{
					PhaseIndex: phaseIdx,
					CardIndex:  -1,
					TargetLoc:  source,
				})
			}

		case 2: // PlayPhase
			if len(phase.Data) < 3 {
				continue
			}
			target := Location(phase.Data[0])
			minCards := int(phase.Data[1])
			maxCards := int(phase.Data[2])

			// For now, only support single-card plays
			if minCards <= 1 && maxCards >= 1 {
				// Check each card in hand
				for cardIdx := range state.Players[currentPlayer].Hand {
					// TODO: Evaluate valid_play_condition from phase.Data
					// For now, allow all cards
					moves = append(moves, LegalMove{
						PhaseIndex: phaseIdx,
						CardIndex:  cardIdx,
						TargetLoc:  target,
					})
				}
			}

		case 3: // DiscardPhase
			// Always allow discard if have cards
			if len(state.Players[currentPlayer].Hand) > 0 {
				for cardIdx := range state.Players[currentPlayer].Hand {
					moves = append(moves, LegalMove{
						PhaseIndex: phaseIdx,
						CardIndex:  cardIdx,
						TargetLoc:  LocationDiscard,
					})
				}
			}

		case 4: // TrickPhase
			if len(phase.Data) < 4 {
				continue
			}
			leadSuitRequired := phase.Data[0] == 1
			// trumpSuit := phase.Data[1]  // 255 = none
			// highCardWins := phase.Data[2] == 1
			breakingSuit := phase.Data[3] // 255 = none

			hand := state.Players[currentPlayer].Hand
			if len(hand) == 0 {
				continue
			}

			// Determine if we're leading or following
			isLeading := len(state.CurrentTrick) == 0

			if isLeading {
				// Leading: can play any card, except breaking suit until broken
				for cardIdx, card := range hand {
					// If breaking suit (e.g., Hearts) and not broken yet, can't lead it
					if breakingSuit != 255 && card.Suit == breakingSuit && !state.HeartsBroken {
						// Check if player has any non-breaking suit cards
						hasOther := false
						for _, c := range hand {
							if c.Suit != breakingSuit {
								hasOther = true
								break
							}
						}
						if hasOther {
							continue // Can't lead breaking suit
						}
						// If only breaking suit cards, can lead them
					}
					moves = append(moves, LegalMove{
						PhaseIndex: phaseIdx,
						CardIndex:  cardIdx,
						TargetLoc:  LocationTableau, // Use tableau as trick area
					})
				}
			} else {
				// Following: must follow suit if able
				leadSuit := state.CurrentTrick[0].Card.Suit

				if leadSuitRequired {
					// Check if we have cards of lead suit
					hasLeadSuit := false
					for _, card := range hand {
						if card.Suit == leadSuit {
							hasLeadSuit = true
							break
						}
					}

					if hasLeadSuit {
						// Must follow suit
						for cardIdx, card := range hand {
							if card.Suit == leadSuit {
								moves = append(moves, LegalMove{
									PhaseIndex: phaseIdx,
									CardIndex:  cardIdx,
									TargetLoc:  LocationTableau,
								})
							}
						}
					} else {
						// Can't follow suit - can play any card
						for cardIdx := range hand {
							moves = append(moves, LegalMove{
								PhaseIndex: phaseIdx,
								CardIndex:  cardIdx,
								TargetLoc:  LocationTableau,
							})
						}
					}
				} else {
					// No suit following required - can play any card
					for cardIdx := range hand {
						moves = append(moves, LegalMove{
							PhaseIndex: phaseIdx,
							CardIndex:  cardIdx,
							TargetLoc:  LocationTableau,
						})
					}
				}
			}
		}
	}

	return moves
}

// ApplyMove executes a legal move, mutating state
func ApplyMove(state *GameState, move *LegalMove, genome *Genome) {
	if move.PhaseIndex >= len(genome.TurnPhases) {
		return
	}

	phase := genome.TurnPhases[move.PhaseIndex]
	currentPlayer := state.CurrentPlayer

	switch phase.PhaseType {
	case 1: // DrawPhase
		if len(phase.Data) >= 5 {
			count := int(binary.BigEndian.Uint32(phase.Data[1:5]))
			for i := 0; i < count; i++ {
				state.DrawCard(currentPlayer, move.TargetLoc)
			}
		}

	case 2: // PlayPhase
		if move.CardIndex >= 0 {
			state.PlayCard(currentPlayer, move.CardIndex, move.TargetLoc)

			// War-specific logic: if playing to tableau in 2-player game
			if move.TargetLoc == LocationTableau && len(state.Players) == 2 {
				resolveWarBattle(state)
			}
		}

	case 3: // DiscardPhase
		if move.CardIndex >= 0 {
			state.PlayCard(currentPlayer, move.CardIndex, LocationDiscard)
		}

	case 4: // TrickPhase
		if move.CardIndex >= 0 && move.CardIndex < len(state.Players[currentPlayer].Hand) {
			card := state.Players[currentPlayer].Hand[move.CardIndex]

			// Remove card from hand
			state.Players[currentPlayer].Hand = append(
				state.Players[currentPlayer].Hand[:move.CardIndex],
				state.Players[currentPlayer].Hand[move.CardIndex+1:]...,
			)

			// Add to current trick
			state.CurrentTrick = append(state.CurrentTrick, TrickCard{
				PlayerID: currentPlayer,
				Card:     card,
			})

			// Check if this card breaks hearts (or other breaking suit)
			if len(phase.Data) >= 4 {
				breakingSuit := phase.Data[3]
				if breakingSuit != 255 && card.Suit == breakingSuit {
					state.HeartsBroken = true
				}
			}

			// Check if trick is complete
			numPlayers := int(state.NumPlayers)
			if numPlayers == 0 {
				numPlayers = 2 // Default to 2 players
			}
			if len(state.CurrentTrick) >= numPlayers {
				// Resolve trick
				resolveTrick(state, genome, phase)
				return // Don't advance turn normally - resolveTrick sets next player
			}
		}
	}

	// Advance turn
	state.CurrentPlayer = (state.CurrentPlayer + 1) % state.NumPlayers
	if state.NumPlayers == 0 {
		state.CurrentPlayer = 1 - currentPlayer // Fallback for 2 players
	}
	state.TurnNumber++
}

// resolveTrick determines the winner and scores points
func resolveTrick(state *GameState, genome *Genome, phase PhaseDescriptor) {
	if len(state.CurrentTrick) == 0 {
		return
	}

	// Parse phase data
	trumpSuit := uint8(255) // None
	highCardWins := true
	breakingSuit := uint8(255)
	if len(phase.Data) >= 4 {
		trumpSuit = phase.Data[1]
		highCardWins = phase.Data[2] == 1
		breakingSuit = phase.Data[3]
	}

	leadSuit := state.CurrentTrick[0].Card.Suit
	winnerIdx := 0
	winningCard := state.CurrentTrick[0].Card

	for i := 1; i < len(state.CurrentTrick); i++ {
		tc := state.CurrentTrick[i]
		card := tc.Card

		// Determine if this card beats the current winner
		beats := false

		if trumpSuit != 255 {
			// Trump game rules
			winnerIsTrump := winningCard.Suit == trumpSuit
			cardIsTrump := card.Suit == trumpSuit

			if cardIsTrump && !winnerIsTrump {
				// Trump beats non-trump
				beats = true
			} else if cardIsTrump && winnerIsTrump {
				// Both trump - compare ranks
				if highCardWins {
					beats = card.Rank > winningCard.Rank
				} else {
					beats = card.Rank < winningCard.Rank
				}
			} else if !cardIsTrump && !winnerIsTrump && card.Suit == leadSuit {
				// Neither trump - must follow suit to win
				if winningCard.Suit == leadSuit {
					if highCardWins {
						beats = card.Rank > winningCard.Rank
					} else {
						beats = card.Rank < winningCard.Rank
					}
				} else {
					// Current winner didn't follow suit, this card does
					beats = true
				}
			}
		} else {
			// No trump - only lead suit counts
			if card.Suit == leadSuit {
				if winningCard.Suit != leadSuit {
					beats = true
				} else if highCardWins {
					beats = card.Rank > winningCard.Rank
				} else {
					beats = card.Rank < winningCard.Rank
				}
			}
		}

		if beats {
			winnerIdx = i
			winningCard = card
		}
	}

	winner := state.CurrentTrick[winnerIdx].PlayerID

	// Score points for Hearts-style games
	points := int32(0)
	for _, tc := range state.CurrentTrick {
		if breakingSuit != 255 && tc.Card.Suit == breakingSuit {
			points++ // Each Heart = 1 point
		}
		// Queen of Spades = 13 points in Hearts
		if tc.Card.Suit == 3 && tc.Card.Rank == 10 { // Spades (3), Queen (10)
			points += 13
		}
	}
	state.Players[winner].Score += points

	// Track tricks won
	if len(state.TricksWon) <= int(winner) {
		// Extend TricksWon slice if needed
		for len(state.TricksWon) <= int(winner) {
			state.TricksWon = append(state.TricksWon, 0)
		}
	}
	state.TricksWon[winner]++

	// Clear current trick
	state.CurrentTrick = state.CurrentTrick[:0]

	// Winner leads next trick
	state.CurrentPlayer = winner
	state.TrickLeader = winner
	state.TurnNumber++
}

// resolveWarBattle handles War game card comparison
func resolveWarBattle(state *GameState) {
	// Check if both players have played (tableau has 2 cards)
	if len(state.Tableau) == 0 || len(state.Tableau[0]) < 2 {
		return
	}

	tableau := state.Tableau[0]
	card1 := tableau[len(tableau)-2] // Second-to-last card (player 0's card)
	card2 := tableau[len(tableau)-1] // Last card (player 1's card)

	// Compare ranks (Ace high: A=12, K=11, ..., 2=0)
	var winner uint8
	if card1.Rank > card2.Rank {
		winner = 0
	} else if card2.Rank > card1.Rank {
		winner = 1
	} else {
		// Tie - in simplified War, alternate who wins ties
		winner = state.CurrentPlayer
	}

	// Winner takes all cards from tableau
	for _, card := range tableau {
		state.Players[winner].Hand = append(state.Players[winner].Hand, card)
	}

	// Clear tableau
	state.Tableau[0] = state.Tableau[0][:0]
}

// CheckWinConditions evaluates win conditions, returns winner ID or -1
// Exported so mcts package can use it
func CheckWinConditions(state *GameState, genome *Genome) int8 {
	for _, wc := range genome.WinConditions {
		switch wc.WinType {
		case 0: // empty_hand
			for playerID, player := range state.Players {
				if len(player.Hand) == 0 {
					return int8(playerID)
				}
			}
		case 1: // high_score (highest score wins, triggers when anyone reaches threshold)
			maxScore := int32(-1)
			winner := int8(-1)
			triggered := false
			for playerID, player := range state.Players {
				if player.Score >= wc.Threshold {
					triggered = true
				}
				if player.Score > maxScore {
					maxScore = player.Score
					winner = int8(playerID)
				}
			}
			if triggered && winner >= 0 {
				return winner
			}
		case 2: // first_to_score
			for playerID, player := range state.Players {
				if player.Score >= wc.Threshold {
					return int8(playerID)
				}
			}
		case 3: // capture_all
			for playerID, player := range state.Players {
				if len(player.Hand) == 52 {
					return int8(playerID)
				}
			}
		case 4: // low_score (Hearts: lowest score wins when anyone reaches threshold)
			minScore := int32(999999)
			winner := int8(-1)
			triggered := false
			for playerID, player := range state.Players {
				if player.Score >= wc.Threshold {
					triggered = true
				}
				if player.Score < minScore {
					minScore = player.Score
					winner = int8(playerID)
				}
			}
			if triggered && winner >= 0 {
				return winner
			}
		case 5: // all_hands_empty (trick-taking: hand ends when all empty)
			allEmpty := true
			for _, player := range state.Players {
				if len(player.Hand) > 0 {
					allEmpty = false
					break
				}
			}
			if allEmpty {
				// In trick-taking games, lowest score wins when hand ends
				minScore := int32(999999)
				winner := int8(-1)
				for playerID, player := range state.Players {
					if player.Score < minScore {
						minScore = player.Score
						winner = int8(playerID)
					}
				}
				return winner
			}
		}
	}
	return -1
}

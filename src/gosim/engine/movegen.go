package engine

import "encoding/binary"

// UpdateTeamScore updates the team score when a player scores.
// This should be called whenever a player's score changes.
// It's a no-op if teams are not configured.
func UpdateTeamScore(state *GameState, playerIdx int, pointsDelta int32) {
	// Skip if teams not configured
	if state.PlayerToTeam == nil || state.TeamScores == nil {
		return
	}

	// Bounds check
	if playerIdx < 0 || playerIdx >= len(state.PlayerToTeam) {
		return
	}

	teamIdx := state.PlayerToTeam[playerIdx]
	if teamIdx >= 0 && int(teamIdx) < len(state.TeamScores) {
		state.TeamScores[teamIdx] += pointsDelta
	}
}

// Special CardIndex values for ClaimPhase
const (
	MoveChallenge = -1 // Challenge the current claim
	MovePass      = -2 // Accept the claim without challenging
)

// Special CardIndex values for DrawPhase
const (
	MoveDraw     = -1 // Draw a card (hit in blackjack)
	MoveDrawPass = -3 // Skip drawing (stand in blackjack)
)

// Special CardIndex values for PlayPhase
const (
	MovePlayPass = -4 // Pass/skip playing (used in President when can't beat top card)
)

// Special CardIndex values for BettingPhase
const (
	MoveBettingCheck = -10
	MoveBettingBet   = -11
	MoveBettingCall  = -12
	MoveBettingRaise = -13
	MoveBettingAllIn = -14
	MoveBettingFold  = -15
)

// LegalMove represents a possible action
type LegalMove struct {
	PhaseIndex int
	CardIndex  int // -1 if not card-specific, -1=Challenge, -2=Pass for ClaimPhase
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

			// Skip if player has already stood (blackjack)
			if int(currentPlayer) < len(state.HasStood) && state.HasStood[currentPlayer] {
				continue
			}

			source := Location(phase.Data[0])
			mandatory := phase.Data[5] == 1

			// Check phase condition if present
			// Data layout: source:1, count:4, mandatory:1, has_condition:1, [condition:7]
			hasCondition := len(phase.Data) > 6 && phase.Data[6] == 1
			if hasCondition && len(phase.Data) >= 14 {
				// Condition is at bytes 7-13: opcode:1, operator:1, value:4, ref:1
				conditionMet := EvaluateCondition(state, currentPlayer, phase.Data[7:14])
				if !conditionMet {
					continue // Skip this phase if condition not met
				}
			}

			// Check if can draw, with automatic deck reshuffling
			canDraw := false
			switch source {
			case LocationDeck:
				// If deck is empty but discard has cards, reshuffle discard into deck
				if len(state.Deck) == 0 && len(state.Discard) > 1 {
					reshuffleDeck(state)
				}
				canDraw = len(state.Deck) > 0
			case LocationDiscard:
				canDraw = len(state.Discard) > 0
			case LocationOpponentHand:
				// Pick next player for N-player support
				opponentID := (currentPlayer + 1) % state.NumPlayers
				canDraw = len(state.Players[opponentID].Hand) > 0
			}

			if canDraw {
				moves = append(moves, LegalMove{
					PhaseIndex: phaseIdx,
					CardIndex:  MoveDraw, // -1 = draw (hit)
					TargetLoc:  source,
				})
			}

			// Add pass/stand option when drawing is not mandatory
			if !mandatory && canDraw {
				moves = append(moves, LegalMove{
					PhaseIndex: phaseIdx,
					CardIndex:  MoveDrawPass, // -3 = pass (stand)
					TargetLoc:  source,
				})
			}

		case 2: // PlayPhase
			if len(phase.Data) < 9 {
				continue
			}
			target := Location(phase.Data[0])
			minCards := int(phase.Data[1])
			maxCards := int(phase.Data[2])
			// phase.Data[3] is mandatory flag
			passIfUnable := phase.Data[4] == 1
			conditionLen := int(binary.BigEndian.Uint32(phase.Data[5:9]))

			// Extract condition bytes if present
			var conditionBytes []byte
			if conditionLen > 0 && len(phase.Data) >= 9+conditionLen {
				conditionBytes = phase.Data[9 : 9+conditionLen]
			}

			hand := state.Players[currentPlayer].Hand
			if len(hand) == 0 {
				continue
			}

			playMoveCount := 0

			// SEQUENCE mode: special handling for tableau plays
			if state.TableauMode == 3 && target == LocationTableau {
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
						// Apply any existing condition
						if len(conditionBytes) > 0 {
							if !EvaluateCardCondition(state, currentPlayer, card, conditionBytes) {
								continue
							}
						}
						moves = append(moves, LegalMove{
							PhaseIndex: phaseIdx,
							CardIndex:  cardIdx,
							TargetLoc:  target,
						})
						playMoveCount++
					}
				} else {
					// Non-empty tableau: check each card against all piles
					// Track which cards have been added to avoid duplicates
					addedCards := make(map[int]bool)

					for cardIdx, card := range hand {
						// Apply any existing condition first
						if len(conditionBytes) > 0 {
							if !EvaluateCardCondition(state, currentPlayer, card, conditionBytes) {
								continue
							}
						}

						// Check if card can play on any existing pile
						canPlayOnExisting := false
						for _, pile := range state.Tableau {
							if len(pile) > 0 {
								topCard := pile[len(pile)-1]
								if isValidSequencePlay(card, topCard, state.SequenceDirection) {
									canPlayOnExisting = true
									break
								}
							}
						}

						// Check if card can start a new pile on an empty slot
						canStartNewPile := false
						for _, pile := range state.Tableau {
							if len(pile) == 0 {
								canStartNewPile = true
								break
							}
						}

						// Add move if card can be played somewhere
						if (canPlayOnExisting || canStartNewPile) && !addedCards[cardIdx] {
							moves = append(moves, LegalMove{
								PhaseIndex: phaseIdx,
								CardIndex:  cardIdx,
								TargetLoc:  target,
							})
							addedCards[cardIdx] = true
							playMoveCount++
						}
					}
				}

				// If no valid plays but pass_if_unable is set, add pass move
				if playMoveCount == 0 && passIfUnable {
					moves = append(moves, LegalMove{
						PhaseIndex: phaseIdx,
						CardIndex:  MovePlayPass,
						TargetLoc:  target,
					})
				}
				continue // Skip normal play logic
			}

			// Single-card plays (standard)
			if minCards <= 1 && maxCards >= 1 {
				// Check each card in hand
				for cardIdx, card := range hand {
					// Evaluate valid_play_condition if present
					if len(conditionBytes) > 0 {
						if !EvaluateCardCondition(state, currentPlayer, card, conditionBytes) {
							continue // Card doesn't satisfy condition
						}
					}
					moves = append(moves, LegalMove{
						PhaseIndex: phaseIdx,
						CardIndex:  cardIdx,
						TargetLoc:  target,
					})
					playMoveCount++
				}
			}

			// Multi-card plays (Go Fish sets)
			// When min_cards > 1, we need a complete set of matching rank
			// CardIndex encodes the rank to play (all cards of that rank)
			if minCards > 1 {
				// Count cards by rank
				rankCounts := make(map[uint8]int)
				for _, card := range hand {
					rankCounts[card.Rank]++
				}

				// Find ranks with enough cards
				for rank, count := range rankCounts {
					if count >= minCards && count <= maxCards {
						// Use negative CardIndex to encode rank + 100
						// CardIndex = -(rank + 100) to distinguish from single plays
						moves = append(moves, LegalMove{
							PhaseIndex: phaseIdx,
							CardIndex:  -int(rank) - 100, // Negative rank encoding
							TargetLoc:  target,
						})
						playMoveCount++
					}
				}
			}

			// If no valid plays but pass_if_unable is set, add pass move
			if playMoveCount == 0 && passIfUnable {
				moves = append(moves, LegalMove{
					PhaseIndex: phaseIdx,
					CardIndex:  MovePlayPass,
					TargetLoc:  target,
				})
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

		case 5: // BettingPhase
			// Skip if betting already completed this hand (for games like blackjack)
			if state.BettingComplete {
				continue
			}

			// Check if only one player remains (everyone else folded)
			activePlayers := CountActivePlayers(state)
			if activePlayers <= 1 {
				// Betting round is effectively over - only one player left
				// Mark betting complete so the game can proceed
				state.BettingComplete = true
				continue
			}

			// Parse betting phase data
			bettingPhase, err := ParseBettingPhaseData(phase.Data)
			if err != nil || bettingPhase == nil {
				continue
			}

			// Check if all bets are matched and no one can act (betting round complete)
			if AllBetsMatched(state) && CountActingPlayers(state) == 0 {
				state.BettingComplete = true
				continue
			}

			// Generate betting moves for current player
			bettingMoves := GenerateBettingMoves(state, bettingPhase, int(currentPlayer))

			// Map BettingAction to LegalMove using negative CardIndex encoding
			// -10=Check, -11=Bet, -12=Call, -13=Raise, -14=AllIn, -15=Fold
			for _, action := range bettingMoves {
				moves = append(moves, LegalMove{
					PhaseIndex: phaseIdx,
					CardIndex:  -10 - int(action), // BettingCheck=0 -> -10, etc.
					TargetLoc:  LocationDeck,      // Unused but required
				})
			}

		case 6: // ClaimPhase - Bluffing/Cheat
			if state.CurrentClaim == nil {
				// No active claim - current player makes a claim
				// For simplicity, play 1 card at a time
				hand := state.Players[currentPlayer].Hand
				if len(hand) > 0 {
					for cardIdx := range hand {
						moves = append(moves, LegalMove{
							PhaseIndex: phaseIdx,
							CardIndex:  cardIdx,
							TargetLoc:  LocationDiscard, // Cards go face-down to discard
						})
					}
				}
			} else {
				// Active claim exists - opponent responds
				if currentPlayer != state.CurrentClaim.ClaimerID {
					// Can challenge or pass
					moves = append(moves, LegalMove{
						PhaseIndex: phaseIdx,
						CardIndex:  MoveChallenge, // -1 = Challenge
						TargetLoc:  LocationDiscard,
					})
					moves = append(moves, LegalMove{
						PhaseIndex: phaseIdx,
						CardIndex:  MovePass, // -2 = Pass (accept claim)
						TargetLoc:  LocationDiscard,
					})
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
		// MoveDrawPass (-3) = stand/pass, mark player as stood (for Blackjack-style games)
		// MoveDraw (-1) = hit/draw
		if move.CardIndex == MoveDraw && len(phase.Data) >= 5 {
			count := int(binary.BigEndian.Uint32(phase.Data[1:5]))
			for i := 0; i < count; i++ {
				state.DrawCard(currentPlayer, move.TargetLoc)
			}
		} else if move.CardIndex == MoveDrawPass {
			// Mark player as having stood - but only for non-shedding games
			// In shedding games (empty_hand win condition), passing is just skipping a draw
			// In Blackjack-style games, passing means "stand" for the rest of the hand
			isShedding := false
			for _, wc := range genome.WinConditions {
				if wc.WinType == 0 { // WinTypeEmptyHand = shedding game
					isShedding = true
					break
				}
			}
			if !isShedding && int(currentPlayer) < len(state.HasStood) {
				state.HasStood[currentPlayer] = true
			}
		}

	case 2: // PlayPhase
		if move.CardIndex == MovePlayPass {
			// Player passes - can't or won't play a card
			state.ConsecutivePasses++

			// If all other players have passed (N-1 passes), clear the tableau
			// The last player to play can now play any card
			if state.ConsecutivePasses >= int(state.NumPlayers)-1 {
				if len(state.Tableau) > 0 {
					// Move tableau cards to discard
					for _, pile := range state.Tableau {
						state.Discard = append(state.Discard, pile...)
					}
					state.Tableau = nil
				}
				state.ConsecutivePasses = 0
			}
		} else if move.CardIndex >= 0 {
			// Single-card play - reset pass counter
			state.ConsecutivePasses = 0

			playedCard := state.Players[currentPlayer].Hand[move.CardIndex]
			state.PlayCard(currentPlayer, move.CardIndex, move.TargetLoc)

			if move.TargetLoc == LocationTableau {
				// Use explicit TableauMode switch for clarity
				switch state.TableauMode {
				case 0: // NONE
					// No special handling - card just sits on tableau
				case 1: // WAR
					// War-style battle: compare ranks, winner takes both
					resolveWarBattle(state)
				case 2: // MATCH_RANK
					// Scopa-style capture: match by rank
					resolveMatchRankCapture(state, currentPlayer, playedCard)
				case 3: // SEQUENCE
					// Sequence validation done in move generation; card just added to pile
					// No additional resolution needed here
				}
			}

			// Check for special effect after playing a card
			if genome != nil && genome.Effects != nil {
				if effect, ok := genome.Effects[playedCard.Rank]; ok {
					ApplyEffect(state, &effect, nil) // nil RNG for now
				}
			}
		} else if move.CardIndex <= -100 {
			// Multi-card play (Go Fish sets)
			// CardIndex encodes rank as -(rank + 100)
			targetRank := uint8(-(move.CardIndex + 100))

			// Find and remove all cards of this rank from hand
			cardsToPlay := make([]Card, 0, 4)
			newHand := make([]Card, 0, len(state.Players[currentPlayer].Hand))
			for _, card := range state.Players[currentPlayer].Hand {
				if card.Rank == targetRank {
					cardsToPlay = append(cardsToPlay, card)
				} else {
					newHand = append(newHand, card)
				}
			}
			state.Players[currentPlayer].Hand = newHand

			// Play cards to target location
			switch move.TargetLoc {
			case LocationDiscard:
				state.Discard = append(state.Discard, cardsToPlay...)
				// Score point for completing a set (Go Fish scoring)
				state.Players[currentPlayer].Score++
				UpdateTeamScore(state, int(currentPlayer), 1)
			case LocationTableau:
				if len(state.Tableau) == 0 {
					state.Tableau = make([][]Card, 1)
				}
				state.Tableau[0] = append(state.Tableau[0], cardsToPlay...)
			}

			// Check for special effect after playing cards (multi-card play)
			if genome != nil && genome.Effects != nil {
				if effect, ok := genome.Effects[targetRank]; ok {
					ApplyEffect(state, &effect, nil) // nil RNG for now
				}
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

	case 5: // BettingPhase
		// Decode betting action from CardIndex
		// -10=Check, -11=Bet, -12=Call, -13=Raise, -14=AllIn, -15=Fold
		if move.CardIndex <= MoveBettingCheck && move.CardIndex >= MoveBettingFold {
			action := BettingAction(-(move.CardIndex + 10))

			bettingPhase, err := ParseBettingPhaseData(phase.Data)
			if err == nil && bettingPhase != nil {
				ApplyBettingAction(state, bettingPhase, int(currentPlayer), action)
			}
		}
		// Note: Turn advancement handled by caller for betting rounds

	case 6: // ClaimPhase - Bluffing/Cheat
		if move.CardIndex >= 0 {
			// Making a claim - play card and create claim
			if move.CardIndex < len(state.Players[currentPlayer].Hand) {
				card := state.Players[currentPlayer].Hand[move.CardIndex]

				// Remove card from hand
				state.Players[currentPlayer].Hand = append(
					state.Players[currentPlayer].Hand[:move.CardIndex],
					state.Players[currentPlayer].Hand[move.CardIndex+1:]...,
				)

				// Add to discard pile (face-down conceptually)
				state.Discard = append(state.Discard, card)

				// Create claim - claimed rank is sequential based on turn number
				claimedRank := uint8(state.TurnNumber % 13) // A, 2, 3, ..., K, A, 2, ...
				state.CurrentClaim = &Claim{
					ClaimerID:    currentPlayer,
					ClaimedRank:  claimedRank,
					ClaimedCount: 1,
					CardsPlayed:  []Card{card},
					Challenged:   false,
				}
			}
		} else if move.CardIndex == MoveChallenge {
			// Challenge the claim
			if state.CurrentClaim != nil {
				resolveChallenge(state, currentPlayer)
				// After challenge resolves, this player makes the next claim
				// Don't advance turn - current player will claim
				state.TurnNumber++
				return
			}
		} else if move.CardIndex == MovePass {
			// Accept claim - clear it, cards stay in discard
			state.CurrentClaim = nil
			// After pass, this player makes the next claim
			// Don't advance turn - current player will claim
			state.TurnNumber++
			return
		}
	}

	// Advance turn
	state.CurrentPlayer = (state.CurrentPlayer + 1) % state.NumPlayers
	if state.NumPlayers == 0 {
		state.CurrentPlayer = 1 - currentPlayer // Fallback for 2 players
	}
	state.TurnNumber++
}

// calculateTrickPoints calculates points for cards in current trick.
// Uses explicit CardScoring rules from genome if available, otherwise
// falls back to implicit Hearts scoring for backwards compatibility.
func calculateTrickPoints(state *GameState, genome *Genome, breakingSuit uint8) int32 {
	points := int32(0)

	// Use explicit scoring rules if available
	if len(genome.CardScoring) > 0 {
		for _, tc := range state.CurrentTrick {
			for _, rule := range genome.CardScoring {
				if rule.Trigger != TriggerTrickWin {
					continue
				}
				// Check if card matches condition
				suitMatch := rule.Suit == 255 || rule.Suit == tc.Card.Suit
				rankMatch := rule.Rank == 255 || rule.Rank == tc.Card.Rank
				if suitMatch && rankMatch {
					points += int32(rule.Points)
				}
			}
		}
		return points
	}

	// Fallback to implicit Hearts scoring for backwards compatibility
	for _, tc := range state.CurrentTrick {
		if breakingSuit != 255 && tc.Card.Suit == breakingSuit {
			points++ // Each breaking suit card = 1 point
		}
		// Queen of Spades = 13 points in Hearts
		if tc.Card.Suit == 3 && tc.Card.Rank == 10 { // Spades (3), Queen (10)
			points += 13
		}
	}

	return points
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

	// Calculate and award points for trick
	points := calculateTrickPoints(state, genome, breakingSuit)
	state.Players[winner].Score += points
	UpdateTeamScore(state, int(winner), points)

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
		// Tie - alternate who wins ties based on battle number
		// Each battle takes 2 turns, so divide TurnNumber by 2
		battleNum := state.TurnNumber / 2
		winner = uint8(battleNum % 2)
	}

	// Winner takes all cards from tableau
	for _, card := range tableau {
		state.Players[winner].Hand = append(state.Players[winner].Hand, card)
	}

	// Clear tableau
	state.Tableau[0] = state.Tableau[0][:0]
}

// resolveMatchRankCapture handles rank-matching capture (Scopa-style)
// When playing a card to tableau, capture any card with matching rank
func resolveMatchRankCapture(state *GameState, playerID uint8, playedCard Card) {
	if len(state.Tableau) == 0 || len(state.Tableau[0]) == 0 {
		return
	}

	tableau := state.Tableau[0]

	// Find matching card in tableau (by rank)
	matchIdx := -1
	for i, card := range tableau {
		// Skip the card we just played (it's the last one)
		if i == len(tableau)-1 {
			continue
		}
		if card.Rank == playedCard.Rank {
			matchIdx = i
			break
		}
	}

	if matchIdx >= 0 {
		// Capture both cards - add to player's score
		capturedCard := tableau[matchIdx]

		// Remove the matched card from tableau
		state.Tableau[0] = append(tableau[:matchIdx], tableau[matchIdx+1:]...)

		// Remove the played card from tableau (it's now at len-1 after removal)
		if len(state.Tableau[0]) > 0 {
			state.Tableau[0] = state.Tableau[0][:len(state.Tableau[0])-1]
		}

		// Score captures (each captured card = 1 point)
		state.Players[playerID].Score += 2 // Both captured card and played card
		UpdateTeamScore(state, int(playerID), 2)

		// For a more complete Scopa implementation, we'd track captured cards
		// in a separate pile, but for scoring purposes, just increment Score
		_ = capturedCard // Used for capture
	}
	// If no match, played card stays on tableau (already added by PlayCard)
}

// setWinnerWithTeam sets the winner ID and also sets WinningTeam if teams are configured.
// Returns the winner ID for convenience in return statements.
func setWinnerWithTeam(state *GameState, winnerID int8) int8 {
	if winnerID < 0 {
		return winnerID
	}

	// Set WinningTeam if teams are configured
	if state.PlayerToTeam != nil && int(winnerID) < len(state.PlayerToTeam) {
		state.WinningTeam = state.PlayerToTeam[winnerID]
	}
	// If player is not in team map, WinningTeam stays at its current value (typically -1)

	return winnerID
}

// CheckWinConditions evaluates win conditions, returns winner ID or -1
// Exported so mcts package can use it
// When a winner is found and teams are configured, also sets state.WinningTeam
func CheckWinConditions(state *GameState, genome *Genome) int8 {
	numPlayers := int(state.NumPlayers)
	if numPlayers == 0 {
		numPlayers = 2 // Default fallback
	}

	for _, wc := range genome.WinConditions {
		switch wc.WinType {
		case 0: // empty_hand
			for playerID := 0; playerID < numPlayers; playerID++ {
				if len(state.Players[playerID].Hand) == 0 {
					return setWinnerWithTeam(state, int8(playerID))
				}
			}
		case 1: // high_score (highest score wins, triggers when anyone reaches threshold)
			maxScore := int32(-1)
			winner := int8(-1)
			triggered := false
			for playerID := 0; playerID < numPlayers; playerID++ {
				player := state.Players[playerID]
				if player.Score >= wc.Threshold {
					triggered = true
				}
				if player.Score > maxScore {
					maxScore = player.Score
					winner = int8(playerID)
				}
			}
			if triggered && winner >= 0 {
				return setWinnerWithTeam(state, winner)
			}
		case 2: // first_to_score
			for playerID := 0; playerID < numPlayers; playerID++ {
				if state.Players[playerID].Score >= wc.Threshold {
					return setWinnerWithTeam(state, int8(playerID))
				}
			}
		case 3: // capture_all
			for playerID := 0; playerID < numPlayers; playerID++ {
				if len(state.Players[playerID].Hand) == 52 {
					return setWinnerWithTeam(state, int8(playerID))
				}
			}
		case 4: // low_score (Hearts: lowest score wins when anyone reaches threshold)
			minScore := int32(999999)
			winner := int8(-1)
			triggered := false
			for playerID := 0; playerID < numPlayers; playerID++ {
				player := state.Players[playerID]
				if player.Score >= wc.Threshold {
					triggered = true
				}
				if player.Score < minScore {
					minScore = player.Score
					winner = int8(playerID)
				}
			}
			if triggered && winner >= 0 {
				return setWinnerWithTeam(state, winner)
			}
		case 5: // all_hands_empty (trick-taking: hand ends when all empty)
			allEmpty := true
			for playerID := 0; playerID < numPlayers; playerID++ {
				if len(state.Players[playerID].Hand) > 0 {
					allEmpty = false
					break
				}
			}
			if allEmpty {
				// In trick-taking games, lowest score wins when hand ends
				minScore := int32(999999)
				winner := int8(-1)
				for playerID := 0; playerID < numPlayers; playerID++ {
					if state.Players[playerID].Score < minScore {
						minScore = state.Players[playerID].Score
						winner = int8(playerID)
					}
				}
				return setWinnerWithTeam(state, winner)
			}

		case 6: // best_hand (poker: compare hands at end of game)
			// This is checked when max_turns is reached
			// For poker, we evaluate immediately when all players have 5 cards
			// and the draw phase is complete
			allHaveFive := true
			for playerID := 0; playerID < numPlayers; playerID++ {
				if len(state.Players[playerID].Hand) != 5 {
					allHaveFive = false
					break
				}
			}
			// Only trigger after some turns have passed (draw phase complete)
			if allHaveFive && state.TurnNumber >= uint32(numPlayers*2) {
				winner := FindBestPokerWinner(state, numPlayers)
				return setWinnerWithTeam(state, winner)
			}

		case 7: // most_captured (Scopa: player with most captured cards wins)
			// Check if game should end (deck empty and hands empty)
			deckEmpty := len(state.Deck) == 0
			handsEmpty := true
			for playerID := 0; playerID < numPlayers; playerID++ {
				if len(state.Players[playerID].Hand) > 0 {
					handsEmpty = false
					break
				}
			}
			if deckEmpty && handsEmpty {
				// Compare captured card counts (stored in Score)
				maxScore := int32(-1)
				winner := int8(-1)
				for playerID := 0; playerID < numPlayers; playerID++ {
					if state.Players[playerID].Score > maxScore {
						maxScore = state.Players[playerID].Score
						winner = int8(playerID)
					}
				}
				return setWinnerWithTeam(state, winner)
			}
		}
	}
	return -1
}

// resolveChallenge handles a challenge in ClaimPhase
// If claim was TRUE (cards match claimed rank), challenger takes pile
// If claim was FALSE (cards don't match), claimer takes pile
func resolveChallenge(state *GameState, challengerID uint8) {
	if state.CurrentClaim == nil {
		return
	}

	claim := state.CurrentClaim
	claimerID := claim.ClaimerID

	// Check if the claim was truthful
	truthful := true
	for _, card := range claim.CardsPlayed {
		if card.Rank != claim.ClaimedRank {
			truthful = false
			break
		}
	}

	var loserID uint8
	if truthful {
		// Claim was true - challenger was wrong, takes the pile
		loserID = challengerID
	} else {
		// Claim was false - claimer was lying, takes the pile
		loserID = claimerID
	}

	// Loser takes entire discard pile
	for _, card := range state.Discard {
		state.Players[loserID].Hand = append(state.Players[loserID].Hand, card)
	}
	state.Discard = state.Discard[:0]

	// Clear the claim
	state.CurrentClaim = nil
}

// reshuffleDeck moves all discard cards except the top one into the deck and shuffles.
// Used for shedding games like Uno when the deck runs out.
func reshuffleDeck(state *GameState) {
	if len(state.Discard) <= 1 {
		return // Nothing to reshuffle
	}

	// Keep the top card, move the rest to deck
	topCard := state.Discard[len(state.Discard)-1]
	for i := 0; i < len(state.Discard)-1; i++ {
		state.Deck = append(state.Deck, state.Discard[i])
	}
	state.Discard = state.Discard[:1]
	state.Discard[0] = topCard

	// Shuffle the deck using turn number as seed for determinism
	state.ShuffleDeck(uint64(state.TurnNumber))
}

// isValidSequencePlay checks if card can be played on top of topCard according to sequence rules.
// Rules:
// - Cards must match suit
// - Direction determines valid ranks:
//   - ASCENDING (0): card.Rank must be exactly topCard.Rank + 1
//   - DESCENDING (1): card.Rank must be exactly topCard.Rank - 1
//   - BOTH (2): either direction is valid
//
// No wrapping: K (13) can't go to A (14) in ascending, 2 can't go to A in descending.
// Rank encoding: 2-10 are face value, 11=J, 12=Q, 13=K, 14=A (Ace high)
// Valid ascending range: 2 -> 3 -> ... -> 13 (K is the end, can't go to A which is 14)
// Valid descending range: 13 -> 12 -> ... -> 2 (2 is the end)
func isValidSequencePlay(card Card, topCard Card, direction uint8) bool {
	// Must match suit
	if card.Suit != topCard.Suit {
		return false
	}

	switch direction {
	case 0: // ASCENDING - card must be exactly 1 rank higher
		// King (13) is the highest in ascending sequences - can't go to Ace (14)
		if topCard.Rank == 13 {
			return false // K is the end of ascending sequence
		}
		return card.Rank == topCard.Rank+1
	case 1: // DESCENDING - card must be exactly 1 rank lower
		// 2 is the lowest in descending sequences - can't go lower
		if topCard.Rank == 2 {
			return false // 2 is the end of descending sequence
		}
		return card.Rank == topCard.Rank-1
	case 2: // BOTH - either direction is valid
		// Apply both boundary checks
		canAscend := topCard.Rank != 13 && card.Rank == topCard.Rank+1
		canDescend := topCard.Rank != 2 && card.Rank == topCard.Rank-1
		return canAscend || canDescend
	}
	return false
}

// BidMove represents a bid action in a bidding phase
type BidMove struct {
	Value int
	IsNil bool
}

// GenerateBidMoves generates valid bid options for the current player
func GenerateBidMoves(phase BiddingPhase, handSize int) []BidMove {
	moves := []BidMove{}

	// Validate max_bid against actual hand size
	effectiveMax := phase.MaxBid
	if handSize < effectiveMax {
		effectiveMax = handSize
	}

	// Add Nil option if allowed and min_bid > 0
	if phase.AllowNil && phase.MinBid > 0 {
		moves = append(moves, BidMove{Value: 0, IsNil: true})
	}

	// Generate valid bid range
	for bid := phase.MinBid; bid <= effectiveMax; bid++ {
		moves = append(moves, BidMove{Value: bid, IsNil: false})
	}

	return moves
}

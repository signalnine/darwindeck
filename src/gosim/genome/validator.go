// Package genome provides typed game genome structures and validation.
package genome

import (
	"fmt"
)

// StandardDeckSize is the number of cards in a standard deck.
const StandardDeckSize = 52

// DefaultPlayerCount is used when player count isn't specified.
const DefaultPlayerCount = 2

// ValidationError represents a genome validation error.
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("%s: %s", e.Field, e.Message)
	}
	return e.Message
}

// GenomeValidator validates genome consistency.
type GenomeValidator struct{}

// Validate returns a list of validation errors (empty = valid).
func (v *GenomeValidator) Validate(genome *GameGenome) []ValidationError {
	var errors []ValidationError

	// Get player count (default to 2 if not specified)
	playerCount := DefaultPlayerCount

	// Check 0: Setup requires valid number of cards
	cardsNeeded := genome.Setup.CardsPerPlayer * playerCount
	if cardsNeeded > StandardDeckSize {
		errors = append(errors, ValidationError{
			Field:   "setup.cards_per_player",
			Message: fmt.Sprintf("Setup requires %d cards but deck only has %d", cardsNeeded, StandardDeckSize),
		})
	}

	// Collect win condition types
	winTypes := make(map[WinConditionType]bool)
	for _, wc := range genome.WinConditions {
		winTypes[wc.Type] = true
	}

	// Check 1: Score-based wins require scoring rules
	scoreWins := map[WinConditionType]bool{
		WinTypeHighScore:    true,
		WinTypeLowScore:     true,
		WinTypeFirstToScore: true,
	}
	hasScoreWin := false
	for wt := range winTypes {
		if scoreWins[wt] {
			hasScoreWin = true
			break
		}
	}
	if hasScoreWin {
		hasScoring := len(genome.CardScoring) > 0
		if !hasScoring {
			errors = append(errors, ValidationError{
				Field:   "win_conditions",
				Message: "Score-based win condition requires card_scoring",
			})
		}
	}

	// Check 2: best_hand win requires hand_evaluation with PATTERN_MATCH
	if winTypes[WinTypeBestHand] {
		hasPatternEval := genome.HandEval != nil &&
			genome.HandEval.Method == EvalMethodPatternMatch
		if !hasPatternEval {
			errors = append(errors, ValidationError{
				Field:   "win_conditions",
				Message: "best_hand win condition requires hand_evaluation with PATTERN_MATCH",
			})
		}
	}

	// Check 3: Betting phase requires starting_chips > 0
	hasBetting := false
	for _, phase := range genome.TurnStructure.Phases {
		if _, ok := phase.(*BettingPhase); ok {
			hasBetting = true
			break
		}
	}
	if hasBetting && genome.Setup.StartingChips <= 0 {
		errors = append(errors, ValidationError{
			Field:   "setup.starting_chips",
			Message: "BettingPhase requires setup.starting_chips > 0",
		})
	}

	// Check 5: Capture wins require capture mechanic
	captureWins := map[WinConditionType]bool{
		WinTypeCaptureAll:   true,
		WinTypeMostCaptured: true,
	}
	hasCaptureWin := false
	for wt := range winTypes {
		if captureWins[wt] {
			hasCaptureWin = true
			break
		}
	}
	if hasCaptureWin {
		hasCapture := genome.TurnStructure.TableauMode == TableauModeWar ||
			genome.TurnStructure.TableauMode == TableauModeMatchRank
		if !hasCapture {
			errors = append(errors, ValidationError{
				Field:   "turn_structure.tableau_mode",
				Message: "Capture win condition requires tableau_mode WAR or MATCH_RANK",
			})
		}
	}

	// Check 6: HandPattern constraints must be internally consistent
	if genome.HandEval != nil && len(genome.HandEval.Patterns) > 0 {
		for _, pattern := range genome.HandEval.Patterns {
			if len(pattern.SameRankGroups) > 0 && pattern.RequiredCount > 0 {
				groupSum := uint8(0)
				for _, g := range pattern.SameRankGroups {
					groupSum += g
				}
				if groupSum > pattern.RequiredCount {
					errors = append(errors, ValidationError{
						Field:   "hand_evaluation.patterns",
						Message: fmt.Sprintf("HandPattern '%s': same_rank_groups sum (%d) exceeds required_count (%d)",
							pattern.Name, groupSum, pattern.RequiredCount),
					})
				}
			}
		}
	}

	// Check 7: Game must have card play phases (not just betting)
	hasCardPlay := false
	for _, phase := range genome.TurnStructure.Phases {
		switch phase.(type) {
		case *PlayPhase, *DrawPhase, *DiscardPhase, *TrickPhase:
			hasCardPlay = true
			break
		}
	}
	if !hasCardPlay {
		errors = append(errors, ValidationError{
			Field:   "turn_structure.phases",
			Message: "Game has no card play phases (needs PlayPhase, DrawPhase, DiscardPhase, or TrickPhase)",
		})
	}

	// Check 8: Betting min_bet should allow meaningful play
	for _, phase := range genome.TurnStructure.Phases {
		if bp, ok := phase.(*BettingPhase); ok {
			starting := genome.Setup.StartingChips
			if starting > 0 && bp.MinBet > 0 {
				// If min_bet > starting_chips / 2, players can only bet once
				if bp.MinBet > starting/2 {
					errors = append(errors, ValidationError{
						Field:   "betting_phase.min_bet",
						Message: fmt.Sprintf("BettingPhase min_bet (%d) is too high relative to starting_chips (%d) - limits meaningful betting",
							bp.MinBet, starting),
					})
				}
			}
		}
	}

	// Check 9: Team configuration validation
	errors = append(errors, v.validateTeams(genome)...)

	// Check 10: Bidding configuration validation
	errors = append(errors, v.validateBidding(genome)...)

	return errors
}

// validateTeams validates team configuration.
func (v *GenomeValidator) validateTeams(genome *GameGenome) []ValidationError {
	var errors []ValidationError

	if genome.Teams == nil || !genome.Teams.Enabled {
		return errors
	}

	playerCount := DefaultPlayerCount

	// Must have at least 2 teams
	if len(genome.Teams.Teams) < 2 {
		errors = append(errors, ValidationError{
			Field:   "teams",
			Message: fmt.Sprintf("Team mode requires at least 2 teams, got %d", len(genome.Teams.Teams)),
		})
		return errors
	}

	// Collect all player indices
	allPlayers := make(map[int]bool)
	for teamIdx, team := range genome.Teams.Teams {
		if len(team) == 0 {
			errors = append(errors, ValidationError{
				Field:   "teams",
				Message: fmt.Sprintf("Team %d is empty", teamIdx),
			})
			continue
		}
		for _, playerIdx := range team {
			// Check for out-of-range
			if playerIdx < 0 || playerIdx >= playerCount {
				errors = append(errors, ValidationError{
					Field:   "teams",
					Message: fmt.Sprintf("Player index %d out of range [0, %d)", playerIdx, playerCount),
				})
			}
			// Check for duplicates
			if allPlayers[playerIdx] {
				errors = append(errors, ValidationError{
					Field:   "teams",
					Message: fmt.Sprintf("Duplicate player %d appears in multiple teams", playerIdx),
				})
			}
			allPlayers[playerIdx] = true
		}
	}

	// Check all players are assigned
	for i := 0; i < playerCount; i++ {
		if !allPlayers[i] {
			errors = append(errors, ValidationError{
				Field:   "teams",
				Message: fmt.Sprintf("Player %d not assigned to any team", i),
			})
		}
	}

	return errors
}

// validateBidding validates bidding phase configuration.
func (v *GenomeValidator) validateBidding(genome *GameGenome) []ValidationError {
	var errors []ValidationError

	hasBiddingPhase := false
	hasTrickPhase := false
	for _, phase := range genome.TurnStructure.Phases {
		if _, ok := phase.(*BiddingPhase); ok {
			hasBiddingPhase = true
		}
		if _, ok := phase.(*TrickPhase); ok {
			hasTrickPhase = true
		}
	}

	if hasBiddingPhase && !hasTrickPhase {
		errors = append(errors, ValidationError{
			Field:   "turn_structure.phases",
			Message: "BiddingPhase requires at least one TrickPhase (contracts need tricks)",
		})
	}

	return errors
}

// ValidateGenome is a convenience function that validates a genome.
func ValidateGenome(genome *GameGenome) []ValidationError {
	v := &GenomeValidator{}
	return v.Validate(genome)
}

// IsValid returns true if the genome has no validation errors.
func IsValid(genome *GameGenome) bool {
	return len(ValidateGenome(genome)) == 0
}

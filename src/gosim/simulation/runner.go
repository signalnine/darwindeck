package simulation

import (
	"encoding/binary"
	"math/rand"
	"time"

	"github.com/signalnine/darwindeck/gosim/engine"
	"github.com/signalnine/darwindeck/gosim/mcts"
)

// AIPlayerType specifies which AI to use
type AIPlayerType uint8

const (
	RandomAI    AIPlayerType = 0
	GreedyAI    AIPlayerType = 1
	MCTS100AI   AIPlayerType = 2
	MCTS500AI   AIPlayerType = 3
	MCTS1000AI  AIPlayerType = 4
	MCTS2000AI  AIPlayerType = 5
)

// GameMetrics holds Phase 1 instrumentation counters
type GameMetrics struct {
	TotalDecisions    uint64 // Decision points (when player chooses move)
	TotalValidMoves   uint64 // Sum of valid moves at each decision
	ForcedDecisions   uint64 // Decisions with only 1 valid move
	TotalInteractions uint64 // Actions affecting opponent state
	TotalActions      uint64 // Total actions taken
	TotalHandSize     uint64 // Sum of hand sizes at each decision (for filtering ratio)

	// Solitaire detection metrics (interaction quality)
	MoveDisruptionEvents uint64 // Opponent turns that changed waiting player's legal moves
	ContentionEvents     uint64 // Times players competed for same resource
	ForcedResponseEvents uint64 // Turns where legal moves significantly constrained
	OpponentTurnCount    uint64 // Total opponent turns (denominator for rates)

	// Bluffing metrics (ClaimPhase games)
	TotalClaims       uint64 // Number of claims made
	TotalBluffs       uint64 // Claims where cards didn't match claimed rank
	TotalChallenges   uint64 // Number of challenges made
	SuccessfulBluffs  uint64 // Bluffs that weren't challenged (opponent passed)
	SuccessfulCatches uint64 // Challenges that caught a bluff

	// Betting metrics (BettingPhase games)
	TotalBets     uint64 // Count of Bet/Raise/AllIn actions
	BettingBluffs uint64 // Bet/Raise/AllIn with weak hand (hand_strength < 0.3)
	FoldWins      uint64 // Wins where opponent(s) folded (no showdown)
	ShowdownWins  uint64 // Wins that went to showdown
	AllInCount    uint64 // Number of all-in actions

	// Tension curve metrics
	LeadChanges       uint32  // Number of times the lead changed hands
	DecisiveTurnPct   float32 // Fraction of turns with margin >= 50% of max possible
	ClosestMargin     float32 // Smallest margin observed (normalized 0-1)
	WinnerWasTrailing bool    // True if winner was behind at midpoint (comeback win)
}

// GameResult holds the outcome of a single game
type GameResult struct {
	WinnerID       int8
	WinningTeam    int8   // -1 = no teams or no winner, 0+ = winning team index
	TurnCount      uint32
	DurationNs     uint64
	Error          string
	Metrics        GameMetrics // Phase 1 instrumentation
}

// AggregatedStats summarizes multiple game results
type AggregatedStats struct {
	TotalGames    uint32
	Wins          []uint32 // Wins per player (index = player ID)
	Draws         uint32
	AvgTurns      float32
	MedianTurns   uint32
	AvgDurationNs uint64
	Errors        uint32

	// Phase 1 instrumentation: aggregated across all games
	TotalDecisions    uint64
	TotalValidMoves   uint64
	ForcedDecisions   uint64
	TotalInteractions uint64
	TotalActions      uint64
	TotalHandSize     uint64 // For filtering ratio calculation

	// Bluffing metrics: aggregated across all games
	TotalClaims       uint64
	TotalBluffs       uint64
	TotalChallenges   uint64
	SuccessfulBluffs  uint64
	SuccessfulCatches uint64

	// Betting metrics: aggregated across all games
	TotalBets     uint64
	BettingBluffs uint64
	FoldWins      uint64
	ShowdownWins  uint64
	AllInCount    uint64

	// Tension metrics: aggregated across all games
	LeadChanges     uint32  // Sum of lead changes across all games
	DecisiveTurnPct float32 // Average decisive turn percentage
	ClosestMargin   float32 // Average closest margin
	TrailingWinners uint32  // Games where winner was behind at midpoint

	// Solitaire detection metrics (interaction quality)
	MoveDisruptionEvents uint64 // Opponent turns that changed waiting player's legal moves
	ContentionEvents     uint64 // Times players competed for same resource
	ForcedResponseEvents uint64 // Turns where legal moves significantly constrained
	OpponentTurnCount    uint64 // Total opponent turns (denominator for rates)

	// Team play metrics
	TeamWins []uint32 // Win count per team (nil if no teams)
}

// RunBatch simulates multiple games with the same genome and AI configuration
func RunBatch(genome *engine.Genome, numGames int, aiType AIPlayerType, mctsIterations int, seed uint64) AggregatedStats {
	results := make([]GameResult, numGames)

	// Use seed for determinism
	rng := rand.New(rand.NewSource(int64(seed)))

	for i := 0; i < numGames; i++ {
		gameSeed := rng.Uint64()
		results[i] = RunSingleGame(genome, aiType, mctsIterations, gameSeed)
	}

	return aggregateResults(results)
}

// RunSingleGame plays one complete game to termination
func RunSingleGame(genome *engine.Genome, aiType AIPlayerType, mctsIterations int, seed uint64) GameResult {
	start := time.Now()
	var metrics GameMetrics

	// Initialize game state
	state := engine.GetState()
	defer engine.PutState(state)

	// Setup deck and deal cards
	setupDeck(state, seed)

	// Read setup section from genome bytecode
	// Format: cards_per_player:4 + initial_discard_count:4 + starting_chips:4
	cardsPerPlayer := 26 // Default for War
	initialDiscardCount := 0
	startingChips := 0

	if genome.Header.SetupOffset > 0 && genome.Header.SetupOffset+12 <= int32(len(genome.Bytecode)) {
		setupOffset := genome.Header.SetupOffset
		cardsPerPlayer = int(int32(binary.BigEndian.Uint32(genome.Bytecode[setupOffset : setupOffset+4])))
		initialDiscardCount = int(int32(binary.BigEndian.Uint32(genome.Bytecode[setupOffset+4 : setupOffset+8])))
		startingChips = int(int32(binary.BigEndian.Uint32(genome.Bytecode[setupOffset+8 : setupOffset+12])))
	}

	// Determine number of players from genome header
	numPlayers := int(genome.Header.PlayerCount)
	if numPlayers == 0 || numPlayers > 4 {
		numPlayers = 2 // Default to 2 players
	}

	// Initialize trick-taking state
	state.NumPlayers = uint8(numPlayers)
	state.CardsPerPlayer = cardsPerPlayer

	// Set tableau mode from genome header
	state.TableauMode = genome.Header.TableauMode
	state.SequenceDirection = genome.Header.SequenceDirection

	// Initialize teams if configured
	if genome.Header.TeamMode && genome.Header.TeamCount > 0 && genome.Header.TeamDataOffset > 0 {
		teamDataOffset := genome.Header.TeamDataOffset
		if teamDataOffset < len(genome.Bytecode) {
			teams := engine.ParseTeams(genome.Bytecode[teamDataOffset:])
			state.InitializeTeams(teams)
		}
	}

	// Deal cards to each player
	for i := 0; i < cardsPerPlayer; i++ {
		for p := 0; p < numPlayers; p++ {
			state.DrawCard(uint8(p), engine.LocationDeck)
		}
	}

	// Deal initial cards to discard pile (for Uno-style games)
	// The first card goes face-up to start the discard pile
	if initialDiscardCount > 0 && len(state.Deck) >= initialDiscardCount {
		for i := 0; i < initialDiscardCount; i++ {
			if len(state.Deck) > 0 {
				card := state.Deck[len(state.Deck)-1]
				state.Deck = state.Deck[:len(state.Deck)-1]
				state.Discard = append(state.Discard, card)
			}
		}
	}

	// Initialize chips if this genome uses betting
	if startingChips > 0 {
		state.InitializeChips(startingChips)
	}

	// Initialize tension tracking
	detector := engine.SelectLeaderDetector(genome)
	tensionMetrics := engine.NewTensionMetrics(int(state.NumPlayers))

	// Game loop with turn limit protection
	maxTurns := genome.Header.MaxTurns
	for state.TurnNumber < maxTurns {
		// Check win conditions
		winner := engine.CheckWinConditions(state, genome)
		if winner >= 0 {
			tensionMetrics.Finalize(int(winner))
			metrics.LeadChanges = uint32(tensionMetrics.LeadChanges)
			metrics.DecisiveTurnPct = tensionMetrics.DecisiveTurnPct()
			metrics.ClosestMargin = tensionMetrics.ClosestMargin
			metrics.WinnerWasTrailing = tensionMetrics.WinnerWasTrailing
			return GameResult{
				WinnerID:    winner,
				WinningTeam: state.WinningTeam,
				TurnCount:   state.TurnNumber,
				DurationNs:  uint64(time.Since(start).Nanoseconds()),
				Metrics:     metrics,
			}
		}

		// Generate legal moves
		moves := engine.GenerateLegalMoves(state, genome)

		// Check if this is a betting phase
		if hasBettingPhase(moves) {
			bettingPhase := getBettingPhaseData(genome)
			if bettingPhase != nil {
				err := runBettingRound(state, genome, bettingPhase, aiType, &metrics, tensionMetrics, detector)
				if err != "" {
					tensionMetrics.Finalize(-1)
					metrics.LeadChanges = uint32(tensionMetrics.LeadChanges)
					metrics.DecisiveTurnPct = tensionMetrics.DecisiveTurnPct()
					metrics.ClosestMargin = tensionMetrics.ClosestMargin
					metrics.WinnerWasTrailing = tensionMetrics.WinnerWasTrailing
					return GameResult{
						WinnerID:    -1,
						WinningTeam: -1,
						TurnCount:   state.TurnNumber,
						DurationNs:  uint64(time.Since(start).Nanoseconds()),
						Error:       err,
						Metrics:     metrics,
					}
				}

				// Mark betting as complete for this hand
				state.BettingComplete = true

				// After betting round, check if we should resolve showdown
				// For blackjack-style games, betting is at the start - continue game
				// For poker-style games, betting is at the end - resolve showdown
				if engine.IsBlackjackGame(genome) {
					// Blackjack: just continue to draw phase
					// Only resolve showdown if someone folded
					winners := engine.ResolveShowdown(state)
					if len(winners) == 1 {
						// Single winner (opponent folded)
						engine.AwardPot(state, winners)
						metrics.FoldWins++
						state.ResetHand()
					}
					// Otherwise continue to draw phase
					continue
				}

				// Poker-style: resolve showdown after betting
				winners := engine.ResolveShowdown(state)
				if len(winners) == 1 {
					// Single winner (others folded)
					engine.AwardPot(state, winners)
					metrics.FoldWins++ // Track fold win
				} else if len(winners) > 1 {
					// Multiple players - use poker hand comparison
					winner := engine.FindBestPokerWinner(state, int(state.NumPlayers))
					if winner >= 0 {
						engine.AwardPot(state, []int{int(winner)})
						metrics.ShowdownWins++ // Track showdown win
					}
				}

				// Reset for next hand
				state.ResetHand()
				continue // Skip normal move application
			}
		}

		// Check if this is a bidding phase
		if hasBiddingMoves(moves) {
			// Create AI type array for all players
			aiTypes := make([]AIPlayerType, state.NumPlayers)
			for i := range aiTypes {
				aiTypes[i] = aiType
			}
			runBiddingRound(state, genome, aiTypes)
			continue // Skip normal move application, re-evaluate moves after bidding
		}

		if len(moves) == 0 {
			// No legal moves
			// For blackjack, this means players can't draw anymore - determine winner
			if engine.IsBlackjackGame(genome) {
				winner := engine.FindBestBlackjackWinner(state, int(state.NumPlayers))
				if winner >= 0 {
					metrics.ShowdownWins++
				}
				tensionMetrics.Finalize(int(winner))
				metrics.LeadChanges = uint32(tensionMetrics.LeadChanges)
				metrics.DecisiveTurnPct = tensionMetrics.DecisiveTurnPct()
				metrics.ClosestMargin = tensionMetrics.ClosestMargin
				metrics.WinnerWasTrailing = tensionMetrics.WinnerWasTrailing
				return GameResult{
					WinnerID:    winner,
					WinningTeam: state.WinningTeam,
					TurnCount:   state.TurnNumber,
					DurationNs:  uint64(time.Since(start).Nanoseconds()),
					Metrics:     metrics,
				}
			}
			// For other games, no legal moves means stuck
			tensionMetrics.Finalize(-1)
			metrics.LeadChanges = uint32(tensionMetrics.LeadChanges)
			metrics.DecisiveTurnPct = tensionMetrics.DecisiveTurnPct()
			metrics.ClosestMargin = tensionMetrics.ClosestMargin
			metrics.WinnerWasTrailing = tensionMetrics.WinnerWasTrailing
			return GameResult{
				WinnerID:    -1,
				WinningTeam: -1,
				TurnCount:   state.TurnNumber,
				DurationNs:  uint64(time.Since(start).Nanoseconds()),
				Error:       "no legal moves",
				Metrics:     metrics,
			}
		}

		// Phase 1 instrumentation: decision counting
		metrics.TotalDecisions++
		metrics.TotalValidMoves += uint64(len(moves))
		metrics.TotalHandSize += uint64(len(state.Players[state.CurrentPlayer].Hand))
		if len(moves) == 1 {
			metrics.ForcedDecisions++
		}

		// BEFORE selecting/applying move: snapshot state for disruption tracking
		numPlayers := int(state.NumPlayers)
		actingPlayer := int(state.CurrentPlayer) // Capture BEFORE ApplyMove changes it
		var nextPlayerIdx int
		var movesBefore []engine.LegalMove
		if numPlayers > 1 {
			// Track the NEXT player who will act (their options may change)
			nextPlayerIdx = (actingPlayer + 1) % numPlayers
			movesBefore = getLegalMovesForPlayer(state, genome, nextPlayerIdx)
		}

		// Select and apply move based on AI type
		var move *engine.LegalMove

		// Use blackjack strategy for blackjack games with draw phase moves
		isBlackjack := engine.IsBlackjackGame(genome)
		hasBlackjackDrawMoves := false
		if isBlackjack && len(moves) > 0 {
			hasBlackjackDrawMoves = engine.IsBlackjackDrawMove(&moves[0])
		}

		// Optimization: skip MCTS search if only one legal move
		if len(moves) == 1 {
			move = &moves[0]
		} else if hasBlackjackDrawMoves {
			// Use basic blackjack strategy (hit <17, stand >=17)
			idx := engine.SelectBlackjackMove(state, moves)
			if idx >= 0 && idx < len(moves) {
				move = &moves[idx]
			} else {
				move = &moves[0]
			}
		} else {
			switch aiType {
			case RandomAI:
				move = &moves[rand.Intn(len(moves))]
			case GreedyAI:
				move = selectGreedyMove(state, genome, moves)
			case MCTS100AI:
				move = mcts.Search(state, genome, 100, mcts.DefaultExplorationParam)
			case MCTS500AI:
				move = mcts.Search(state, genome, 500, mcts.DefaultExplorationParam)
			case MCTS1000AI:
				move = mcts.Search(state, genome, 1000, mcts.DefaultExplorationParam)
			case MCTS2000AI:
				move = mcts.Search(state, genome, 2000, mcts.DefaultExplorationParam)
			default:
				move = &moves[0]
			}
		}

		if move == nil {
			tensionMetrics.Finalize(-1)
			metrics.LeadChanges = uint32(tensionMetrics.LeadChanges)
			metrics.DecisiveTurnPct = tensionMetrics.DecisiveTurnPct()
			metrics.ClosestMargin = tensionMetrics.ClosestMargin
			metrics.WinnerWasTrailing = tensionMetrics.WinnerWasTrailing
			return GameResult{
				WinnerID:    -1,
				WinningTeam: -1,
				TurnCount:   state.TurnNumber,
				DurationNs:  uint64(time.Since(start).Nanoseconds()),
				Error:       "AI returned nil move",
				Metrics:     metrics,
			}
		}

		// Phase 1 instrumentation: action and interaction counting
		metrics.TotalActions++
		if isInteraction(state, move, genome) {
			metrics.TotalInteractions++
		}

		// Track bluffing metrics before ApplyMove changes state
		trackBluffingMetrics(state, move, genome, &metrics)

		// Track resource contention - could opponents have made similar move?
		if isContentionEvent(state, move, genome, actingPlayer) {
			metrics.ContentionEvents++
		}

		engine.ApplyMove(state, move, genome)

		// Track move disruption - did this turn change next player's options?
		// Note: actingPlayer and nextPlayerIdx captured BEFORE ApplyMove
		if numPlayers > 1 && movesBefore != nil {
			movesAfter := getLegalMovesForPlayer(state, genome, nextPlayerIdx)

			// Move disruption: any change in available moves
			if movesDisrupted(movesBefore, movesAfter) {
				metrics.MoveDisruptionEvents++
			}

			// Forced response: moves dropped by >30%
			// This indicates the opponent MUST react (fewer options available)
			beforeCount := len(movesBefore)
			afterCount := len(movesAfter)
			if beforeCount > 0 && afterCount < beforeCount {
				ratio := float64(afterCount) / float64(beforeCount)
				if ratio < 0.7 {
					metrics.ForcedResponseEvents++
				}
			}

			metrics.OpponentTurnCount++
		}

		// Update tension tracking after move
		tensionMetrics.Update(state, detector)
	}

	// Max turns reached - draw
	tensionMetrics.Finalize(-1)
	metrics.LeadChanges = uint32(tensionMetrics.LeadChanges)
	metrics.DecisiveTurnPct = tensionMetrics.DecisiveTurnPct()
	metrics.ClosestMargin = tensionMetrics.ClosestMargin
	metrics.WinnerWasTrailing = tensionMetrics.WinnerWasTrailing
	return GameResult{
		WinnerID:    -1,
		WinningTeam: -1,
		TurnCount:   state.TurnNumber,
		DurationNs:  uint64(time.Since(start).Nanoseconds()),
		Metrics:     metrics,
	}
}

// RunBatchAsymmetric simulates games with different AI types for each player.
// Used for skill gap measurement (e.g., MCTS vs Random).
func RunBatchAsymmetric(genome *engine.Genome, numGames int, p0AIType AIPlayerType, p1AIType AIPlayerType, mctsIterations int, seed uint64) AggregatedStats {
	results := make([]GameResult, numGames)
	rng := rand.New(rand.NewSource(int64(seed)))

	for i := 0; i < numGames; i++ {
		gameSeed := rng.Uint64()
		results[i] = RunSingleGameAsymmetric(genome, p0AIType, p1AIType, mctsIterations, gameSeed)
	}

	return aggregateResults(results)
}

// RunSingleGameAsymmetric plays one game with different AI for each player.
func RunSingleGameAsymmetric(genome *engine.Genome, p0AIType AIPlayerType, p1AIType AIPlayerType, mctsIterations int, seed uint64) GameResult {
	start := time.Now()
	var metrics GameMetrics

	state := engine.GetState()
	defer engine.PutState(state)

	setupDeck(state, seed)

	// Read setup section from genome bytecode
	// Format: cards_per_player:4 + initial_discard_count:4 + starting_chips:4
	cardsPerPlayer := 26
	initialDiscardCount := 0
	startingChips := 0

	if genome.Header.SetupOffset > 0 && genome.Header.SetupOffset+12 <= int32(len(genome.Bytecode)) {
		setupOffset := genome.Header.SetupOffset
		cardsPerPlayer = int(int32(binary.BigEndian.Uint32(genome.Bytecode[setupOffset : setupOffset+4])))
		initialDiscardCount = int(int32(binary.BigEndian.Uint32(genome.Bytecode[setupOffset+4 : setupOffset+8])))
		startingChips = int(int32(binary.BigEndian.Uint32(genome.Bytecode[setupOffset+8 : setupOffset+12])))
	}

	numPlayers := int(genome.Header.PlayerCount)
	if numPlayers == 0 || numPlayers > 4 {
		numPlayers = 2
	}

	state.NumPlayers = uint8(numPlayers)
	state.CardsPerPlayer = cardsPerPlayer

	// Set tableau mode from genome header
	state.TableauMode = genome.Header.TableauMode
	state.SequenceDirection = genome.Header.SequenceDirection

	// Initialize teams if configured
	if genome.Header.TeamMode && genome.Header.TeamCount > 0 && genome.Header.TeamDataOffset > 0 {
		teamDataOffset := genome.Header.TeamDataOffset
		if teamDataOffset < len(genome.Bytecode) {
			teams := engine.ParseTeams(genome.Bytecode[teamDataOffset:])
			state.InitializeTeams(teams)
		}
	}

	for i := 0; i < cardsPerPlayer; i++ {
		for p := 0; p < numPlayers; p++ {
			state.DrawCard(uint8(p), engine.LocationDeck)
		}
	}

	// Deal initial cards to discard pile (for Uno-style games)
	// The first card goes face-up to start the discard pile
	if initialDiscardCount > 0 && len(state.Deck) >= initialDiscardCount {
		for i := 0; i < initialDiscardCount; i++ {
			if len(state.Deck) > 0 {
				card := state.Deck[len(state.Deck)-1]
				state.Deck = state.Deck[:len(state.Deck)-1]
				state.Discard = append(state.Discard, card)
			}
		}
	}

	// Initialize chips if this genome uses betting
	if startingChips > 0 {
		state.InitializeChips(startingChips)
	}

	// Initialize tension tracking
	detector := engine.SelectLeaderDetector(genome)
	tensionMetrics := engine.NewTensionMetrics(int(state.NumPlayers))

	maxTurns := genome.Header.MaxTurns
	for state.TurnNumber < maxTurns {
		winner := engine.CheckWinConditions(state, genome)
		if winner >= 0 {
			tensionMetrics.Finalize(int(winner))
			metrics.LeadChanges = uint32(tensionMetrics.LeadChanges)
			metrics.DecisiveTurnPct = tensionMetrics.DecisiveTurnPct()
			metrics.ClosestMargin = tensionMetrics.ClosestMargin
			metrics.WinnerWasTrailing = tensionMetrics.WinnerWasTrailing
			return GameResult{
				WinnerID:    winner,
				WinningTeam: state.WinningTeam,
				TurnCount:   state.TurnNumber,
				DurationNs:  uint64(time.Since(start).Nanoseconds()),
				Metrics:     metrics,
			}
		}

		moves := engine.GenerateLegalMoves(state, genome)

		// Check if this is a betting phase
		if hasBettingPhase(moves) {
			bettingPhase := getBettingPhaseData(genome)
			if bettingPhase != nil {
				err := runBettingRoundAsymmetric(state, genome, bettingPhase, p0AIType, p1AIType, &metrics)
				if err != "" {
					tensionMetrics.Finalize(-1)
					metrics.LeadChanges = uint32(tensionMetrics.LeadChanges)
					metrics.DecisiveTurnPct = tensionMetrics.DecisiveTurnPct()
					metrics.ClosestMargin = tensionMetrics.ClosestMargin
					metrics.WinnerWasTrailing = tensionMetrics.WinnerWasTrailing
					return GameResult{
						WinnerID:    -1,
						WinningTeam: -1,
						TurnCount:   state.TurnNumber,
						DurationNs:  uint64(time.Since(start).Nanoseconds()),
						Error:       err,
						Metrics:     metrics,
					}
				}

				// Mark betting as complete for this hand
				state.BettingComplete = true

				// After betting round, check if we should resolve showdown
				// For blackjack-style games, betting is at the start - continue game
				// For poker-style games, betting is at the end - resolve showdown
				if engine.IsBlackjackGame(genome) {
					// Blackjack: just continue to draw phase
					// Only resolve showdown if someone folded
					winners := engine.ResolveShowdown(state)
					if len(winners) == 1 {
						// Single winner (opponent folded)
						engine.AwardPot(state, winners)
						metrics.FoldWins++
						state.ResetHand()
					}
					// Otherwise continue to draw phase
					continue
				}

				// Poker-style: resolve showdown after betting
				winners := engine.ResolveShowdown(state)
				if len(winners) == 1 {
					// Single winner (others folded)
					engine.AwardPot(state, winners)
					metrics.FoldWins++ // Track fold win
				} else if len(winners) > 1 {
					// Multiple players - use poker hand comparison
					winner := engine.FindBestPokerWinner(state, int(state.NumPlayers))
					if winner >= 0 {
						engine.AwardPot(state, []int{int(winner)})
						metrics.ShowdownWins++ // Track showdown win
					}
				}

				// Reset for next hand
				state.ResetHand()
				continue // Skip normal move application
			}
		}

		// Check if this is a bidding phase
		if hasBiddingMoves(moves) {
			runBiddingRoundAsymmetric(state, genome, p0AIType, p1AIType)
			continue // Skip normal move application, re-evaluate moves after bidding
		}

		if len(moves) == 0 {
			// No legal moves
			// For blackjack, this means players can't draw anymore - determine winner
			if engine.IsBlackjackGame(genome) {
				winner := engine.FindBestBlackjackWinner(state, int(state.NumPlayers))
				if winner >= 0 {
					metrics.ShowdownWins++
				}
				tensionMetrics.Finalize(int(winner))
				metrics.LeadChanges = uint32(tensionMetrics.LeadChanges)
				metrics.DecisiveTurnPct = tensionMetrics.DecisiveTurnPct()
				metrics.ClosestMargin = tensionMetrics.ClosestMargin
				metrics.WinnerWasTrailing = tensionMetrics.WinnerWasTrailing
				return GameResult{
					WinnerID:    winner,
					WinningTeam: state.WinningTeam,
					TurnCount:   state.TurnNumber,
					DurationNs:  uint64(time.Since(start).Nanoseconds()),
					Metrics:     metrics,
				}
			}
			// For other games, no legal moves means stuck
			tensionMetrics.Finalize(-1)
			metrics.LeadChanges = uint32(tensionMetrics.LeadChanges)
			metrics.DecisiveTurnPct = tensionMetrics.DecisiveTurnPct()
			metrics.ClosestMargin = tensionMetrics.ClosestMargin
			metrics.WinnerWasTrailing = tensionMetrics.WinnerWasTrailing
			return GameResult{
				WinnerID:    -1,
				WinningTeam: -1,
				TurnCount:   state.TurnNumber,
				DurationNs:  uint64(time.Since(start).Nanoseconds()),
				Error:       "no legal moves",
				Metrics:     metrics,
			}
		}

		metrics.TotalDecisions++
		metrics.TotalValidMoves += uint64(len(moves))
		metrics.TotalHandSize += uint64(len(state.Players[state.CurrentPlayer].Hand))
		if len(moves) == 1 {
			metrics.ForcedDecisions++
		}

		// BEFORE selecting/applying move: snapshot state for disruption tracking
		numPlayers := int(state.NumPlayers)
		actingPlayer := int(state.CurrentPlayer) // Capture BEFORE ApplyMove changes it
		var nextPlayerIdx int
		var movesBefore []engine.LegalMove
		if numPlayers > 1 {
			// Track the NEXT player who will act (their options may change)
			nextPlayerIdx = (actingPlayer + 1) % numPlayers
			movesBefore = getLegalMovesForPlayer(state, genome, nextPlayerIdx)
		}

		// Select AI based on current player
		var aiType AIPlayerType
		if state.CurrentPlayer == 0 {
			aiType = p0AIType
		} else {
			aiType = p1AIType
		}

		var move *engine.LegalMove

		// Optimization: skip MCTS search if only one legal move
		if len(moves) == 1 {
			move = &moves[0]
		} else {
			switch aiType {
			case RandomAI:
				move = &moves[rand.Intn(len(moves))]
			case GreedyAI:
				move = selectGreedyMove(state, genome, moves)
			case MCTS100AI:
				move = mcts.Search(state, genome, 100, mcts.DefaultExplorationParam)
			case MCTS500AI:
				move = mcts.Search(state, genome, 500, mcts.DefaultExplorationParam)
			case MCTS1000AI:
				move = mcts.Search(state, genome, 1000, mcts.DefaultExplorationParam)
			case MCTS2000AI:
				move = mcts.Search(state, genome, 2000, mcts.DefaultExplorationParam)
			default:
				move = &moves[0]
			}
		}

		if move == nil {
			tensionMetrics.Finalize(-1)
			metrics.LeadChanges = uint32(tensionMetrics.LeadChanges)
			metrics.DecisiveTurnPct = tensionMetrics.DecisiveTurnPct()
			metrics.ClosestMargin = tensionMetrics.ClosestMargin
			metrics.WinnerWasTrailing = tensionMetrics.WinnerWasTrailing
			return GameResult{
				WinnerID:    -1,
				WinningTeam: -1,
				TurnCount:   state.TurnNumber,
				DurationNs:  uint64(time.Since(start).Nanoseconds()),
				Error:       "AI returned nil move",
				Metrics:     metrics,
			}
		}

		metrics.TotalActions++
		if isInteraction(state, move, genome) {
			metrics.TotalInteractions++
		}

		// Track bluffing metrics before ApplyMove changes state
		trackBluffingMetrics(state, move, genome, &metrics)

		// Track resource contention - could opponents have made similar move?
		if isContentionEvent(state, move, genome, actingPlayer) {
			metrics.ContentionEvents++
		}

		engine.ApplyMove(state, move, genome)

		// Track move disruption - did this turn change next player's options?
		// Note: actingPlayer and nextPlayerIdx captured BEFORE ApplyMove
		if numPlayers > 1 && movesBefore != nil {
			movesAfter := getLegalMovesForPlayer(state, genome, nextPlayerIdx)

			// Move disruption: any change in available moves
			if movesDisrupted(movesBefore, movesAfter) {
				metrics.MoveDisruptionEvents++
			}

			// Forced response: moves dropped by >30%
			// This indicates the opponent MUST react (fewer options available)
			beforeCount := len(movesBefore)
			afterCount := len(movesAfter)
			if beforeCount > 0 && afterCount < beforeCount {
				ratio := float64(afterCount) / float64(beforeCount)
				if ratio < 0.7 {
					metrics.ForcedResponseEvents++
				}
			}

			metrics.OpponentTurnCount++
		}

		// Update tension tracking after move
		tensionMetrics.Update(state, detector)
	}

	// Max turns reached - draw
	tensionMetrics.Finalize(-1)
	metrics.LeadChanges = uint32(tensionMetrics.LeadChanges)
	metrics.DecisiveTurnPct = tensionMetrics.DecisiveTurnPct()
	metrics.ClosestMargin = tensionMetrics.ClosestMargin
	metrics.WinnerWasTrailing = tensionMetrics.WinnerWasTrailing
	return GameResult{
		WinnerID:    -1,
		WinningTeam: -1,
		TurnCount:   state.TurnNumber,
		DurationNs:  uint64(time.Since(start).Nanoseconds()),
		Metrics:     metrics,
	}
}

// trackBluffingMetrics records claim/challenge/bluff events
func trackBluffingMetrics(state *engine.GameState, move *engine.LegalMove, genome *engine.Genome, metrics *GameMetrics) {
	// Only track for ClaimPhase (phase type 6)
	if move.PhaseIndex >= len(genome.TurnPhases) {
		return
	}
	phase := genome.TurnPhases[move.PhaseIndex]
	if phase.PhaseType != 6 { // 6 = ClaimPhase
		return
	}

	// Check if there's an active claim
	claim := state.CurrentClaim

	if move.CardIndex == engine.MoveChallenge {
		// This is a challenge
		metrics.TotalChallenges++

		// Check if the claim was a bluff (cards don't match claimed rank)
		if claim != nil {
			isBluff := false
			for _, card := range claim.CardsPlayed {
				if card.Rank != claim.ClaimedRank {
					isBluff = true
					break
				}
			}
			if isBluff {
				metrics.SuccessfulCatches++ // Caught a bluff
			}
		}
	} else if move.CardIndex == engine.MovePass {
		// Opponent passed on the claim
		if claim != nil {
			// Check if this was a bluff that succeeded
			isBluff := false
			for _, card := range claim.CardsPlayed {
				if card.Rank != claim.ClaimedRank {
					isBluff = true
					break
				}
			}
			if isBluff {
				metrics.SuccessfulBluffs++ // Bluff wasn't challenged
			}
		}
	} else if move.CardIndex >= 0 && claim == nil {
		// This is a new claim (no active claim, playing cards)
		// We'll check if it's a bluff after the move is applied
		// For now, just count the claim
		metrics.TotalClaims++

		// Check if it's a bluff by looking at the cards being played
		// The claimed rank will be based on turn number (sequential)
		claimedRank := uint8(state.TurnNumber % 13)
		hand := state.Players[state.CurrentPlayer].Hand

		if move.CardIndex < len(hand) {
			card := hand[move.CardIndex]
			if card.Rank != claimedRank {
				metrics.TotalBluffs++
			}
		}
	}
}

// isInteraction determines if a move affects the opponent's state
func isInteraction(state *engine.GameState, move *engine.LegalMove, genome *engine.Genome) bool {
	if move.PhaseIndex >= len(genome.TurnPhases) {
		return false
	}

	phase := genome.TurnPhases[move.PhaseIndex]

	switch phase.PhaseType {
	case 1: // DrawPhase
		// Drawing from opponent's hand is an interaction
		if move.TargetLoc == engine.LocationOpponentHand {
			return true
		}
	case 2: // PlayPhase
		// Playing to tableau triggers War battle resolution which affects opponent
		if move.TargetLoc == engine.LocationTableau {
			return true
		}
		// Playing to opponent's locations is an interaction
		if move.TargetLoc == engine.LocationOpponentHand ||
			move.TargetLoc == engine.LocationOpponentDiscard {
			return true
		}
		// Check if playing this card triggers a special effect targeting opponents
		if move.CardIndex >= 0 && move.CardIndex < len(state.Players[state.CurrentPlayer].Hand) {
			card := state.Players[state.CurrentPlayer].Hand[move.CardIndex]
			if effect, ok := genome.Effects[card.Rank]; ok {
				// Effects targeting opponents are interactions
				// TARGET_NEXT_PLAYER=0, TARGET_PREV_PLAYER=1, TARGET_PLAYER_CHOICE=2,
				// TARGET_RANDOM_OPPONENT=3, TARGET_ALL_OPPONENTS=4
				if effect.Target <= 4 { // All targets except self
					return true
				}
			}
		}
	case 3: // DiscardPhase
		// Regular discard doesn't affect opponent
		return false
	case 4: // TrickPhase
		// Trick-taking is inherently interactive - every card played
		// affects the trick outcome and impacts all players
		return true
	case 6: // ClaimPhase
		// Bluffing/Cheat is highly interactive - claims affect opponent decisions
		// and challenges affect who picks up the pile
		return true
	}

	return false
}

// movesDisrupted compares two move slices to detect if options changed.
// Returns true if the available moves are different (disrupted).
// Uses a hash-based approach for efficiency with large move sets.
func movesDisrupted(before, after []engine.LegalMove) bool {
	// Quick length check
	if len(before) != len(after) {
		return true
	}
	if len(before) == 0 {
		return false // Both empty = no disruption
	}

	// Build a simple signature for each move set
	// Signature: count moves by (phaseIndex, targetLoc) pairs
	beforeSig := make(map[uint32]int)
	afterSig := make(map[uint32]int)

	for _, m := range before {
		key := uint32(m.PhaseIndex)<<16 | uint32(m.TargetLoc)
		beforeSig[key]++
	}
	for _, m := range after {
		key := uint32(m.PhaseIndex)<<16 | uint32(m.TargetLoc)
		afterSig[key]++
	}

	// Compare signatures
	if len(beforeSig) != len(afterSig) {
		return true
	}
	for k, v := range beforeSig {
		if afterSig[k] != v {
			return true
		}
	}
	return false
}

// isContentionEvent detects when a player takes an action that opponents
// could also have taken - indicating competition for shared resources.
// This is generic across game types.
func isContentionEvent(state *engine.GameState, move *engine.LegalMove, genome *engine.Genome, actingPlayer int) bool {
	// Generic contention: could any opponent have made a similar move?
	// "Similar" = same phase and target location

	for playerIdx := range state.Players {
		if playerIdx == actingPlayer {
			continue
		}

		// Get opponent's legal moves (without mutating state)
		opponentMoves := getLegalMovesForPlayer(state, genome, playerIdx)

		for _, oppMove := range opponentMoves {
			// Contention if opponent could target the same location in same phase
			if oppMove.PhaseIndex == move.PhaseIndex && oppMove.TargetLoc == move.TargetLoc {
				// For shared locations (tableau, discard, deck draws), this is contention
				if move.TargetLoc == engine.LocationTableau ||
					move.TargetLoc == engine.LocationDiscard ||
					move.TargetLoc == engine.LocationDeck {
					return true
				}
			}
		}
	}

	return false
}

// getLegalMovesForPlayer generates legal moves for a specific player
// without mutating the game state's CurrentPlayer field.
func getLegalMovesForPlayer(state *engine.GameState, genome *engine.Genome, playerIdx int) []engine.LegalMove {
	// Save and restore CurrentPlayer to avoid side effects
	originalPlayer := state.CurrentPlayer
	state.CurrentPlayer = uint8(playerIdx)
	moves := engine.GenerateLegalMoves(state, genome)
	state.CurrentPlayer = originalPlayer
	return moves
}

// setupDeck creates and shuffles a standard 52-card deck
func setupDeck(state *engine.GameState, seed uint64) {
	// Create standard 52-card deck
	for suit := uint8(0); suit < 4; suit++ {
		for rank := uint8(0); rank < 13; rank++ {
			state.Deck = append(state.Deck, engine.Card{Rank: rank, Suit: suit})
		}
	}

	// Shuffle with seed
	state.ShuffleDeck(seed)
}

// selectGreedyMove picks the move that maximizes immediate score
func selectGreedyMove(state *engine.GameState, genome *engine.Genome, moves []engine.LegalMove) *engine.LegalMove {
	// Greedy heuristic: prefer moves that:
	// 1. Reduce hand size (get closer to winning)
	// 2. Play higher ranked cards (might matter for War-like games)

	bestMove := &moves[0]
	bestScore := scoreMove(state, &moves[0])

	for i := 1; i < len(moves); i++ {
		score := scoreMove(state, &moves[i])
		if score > bestScore {
			bestScore = score
			bestMove = &moves[i]
		}
	}

	return bestMove
}

// scoreMove assigns a heuristic value to a move
func scoreMove(state *engine.GameState, move *engine.LegalMove) float64 {
	score := 0.0

	// Prefer moves that reduce hand size
	if move.CardIndex >= 0 {
		score += 10.0
	}

	// Prefer playing higher ranked cards
	if move.CardIndex >= 0 && move.CardIndex < len(state.Players[state.CurrentPlayer].Hand) {
		card := state.Players[state.CurrentPlayer].Hand[move.CardIndex]
		score += float64(card.Rank)
	}

	return score
}

// aggregateResults computes summary statistics
func aggregateResults(results []GameResult) AggregatedStats {
	stats := AggregatedStats{
		TotalGames: uint32(len(results)),
		Wins:       make([]uint32, 4), // Support up to 4 players
	}

	turnCounts := make([]uint32, 0, len(results))
	totalDuration := uint64(0)

	// Detect team count by scanning ALL results for the maximum winning team index.
	// This handles the case where one team never wins in the sample.
	var teamWins []uint32
	maxTeamIdx := int8(-1)
	for _, r := range results {
		if r.WinningTeam > maxTeamIdx {
			maxTeamIdx = r.WinningTeam
		}
	}
	if maxTeamIdx >= 0 {
		// Found at least one team game, allocate team wins slice
		// Use maxTeamIdx + 1 as the minimum, but this only counts teams that have won.
		// For proper team count, we need to know the expected number of teams.
		// Heuristic: check if this looks like a 2-team game (indices 0 and 1 expected)
		teamCount := int(maxTeamIdx) + 1
		if teamCount < 2 {
			// If only team 0 has won, assume there are at least 2 teams
			teamCount = 2
		}
		if teamCount > 4 {
			teamCount = 4
		}
		teamWins = make([]uint32, teamCount)
	}

	for _, result := range results {
		if result.Error != "" {
			stats.Errors++
			continue
		}

		// Track wins by player ID (supports N players)
		if result.WinnerID >= 0 && int(result.WinnerID) < len(stats.Wins) {
			stats.Wins[result.WinnerID]++
		} else {
			stats.Draws++
		}

		// Track team wins
		if teamWins != nil && result.WinningTeam >= 0 && int(result.WinningTeam) < len(teamWins) {
			teamWins[result.WinningTeam]++
		}

		turnCounts = append(turnCounts, result.TurnCount)
		totalDuration += result.DurationNs

		// Phase 1 instrumentation: aggregate metrics from each game
		stats.TotalDecisions += result.Metrics.TotalDecisions
		stats.TotalValidMoves += result.Metrics.TotalValidMoves
		stats.ForcedDecisions += result.Metrics.ForcedDecisions
		stats.TotalInteractions += result.Metrics.TotalInteractions
		stats.TotalActions += result.Metrics.TotalActions
		stats.TotalHandSize += result.Metrics.TotalHandSize

		// Bluffing metrics
		stats.TotalClaims += result.Metrics.TotalClaims
		stats.TotalBluffs += result.Metrics.TotalBluffs
		stats.TotalChallenges += result.Metrics.TotalChallenges
		stats.SuccessfulBluffs += result.Metrics.SuccessfulBluffs
		stats.SuccessfulCatches += result.Metrics.SuccessfulCatches

		// Betting metrics
		stats.TotalBets += result.Metrics.TotalBets
		stats.BettingBluffs += result.Metrics.BettingBluffs
		stats.FoldWins += result.Metrics.FoldWins
		stats.ShowdownWins += result.Metrics.ShowdownWins
		stats.AllInCount += result.Metrics.AllInCount

		// Tension metrics (aggregate for averaging later)
		stats.LeadChanges += result.Metrics.LeadChanges
		stats.DecisiveTurnPct += result.Metrics.DecisiveTurnPct
		stats.ClosestMargin += result.Metrics.ClosestMargin
		if result.Metrics.WinnerWasTrailing {
			stats.TrailingWinners++
		}

		// Solitaire detection metrics
		stats.MoveDisruptionEvents += result.Metrics.MoveDisruptionEvents
		stats.ContentionEvents += result.Metrics.ContentionEvents
		stats.ForcedResponseEvents += result.Metrics.ForcedResponseEvents
		stats.OpponentTurnCount += result.Metrics.OpponentTurnCount
	}

	// Calculate averages
	validGames := len(turnCounts) // Games without errors
	if validGames > 0 {
		// Tension metrics: compute averages
		stats.DecisiveTurnPct = stats.DecisiveTurnPct / float32(validGames)
		stats.ClosestMargin = stats.ClosestMargin / float32(validGames)
	}

	if validGames > 0 {
		sum := uint64(0)
		for _, tc := range turnCounts {
			sum += uint64(tc)
		}
		stats.AvgTurns = float32(sum) / float32(len(turnCounts))

		// Median (simple sort-based approach)
		// For production, use quickselect
		stats.MedianTurns = median(turnCounts)
	}

	if stats.TotalGames > 0 {
		stats.AvgDurationNs = totalDuration / uint64(stats.TotalGames)
	}

	// Set team wins if this was a team game
	stats.TeamWins = teamWins

	return stats
}

// median calculates the median of a slice
func median(values []uint32) uint32 {
	if len(values) == 0 {
		return 0
	}

	// Simple bubble sort (fine for small batches)
	sorted := make([]uint32, len(values))
	copy(sorted, values)

	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	mid := len(sorted) / 2
	if len(sorted)%2 == 0 {
		return (sorted[mid-1] + sorted[mid]) / 2
	}
	return sorted[mid]
}

// hasBettingPhase checks if moves are from a betting phase
func hasBettingPhase(moves []engine.LegalMove) bool {
	for _, m := range moves {
		if m.CardIndex <= engine.MoveBettingCheck && m.CardIndex >= engine.MoveBettingFold {
			return true
		}
	}
	return false
}

// hasBiddingMoves checks if moves are from a bidding phase
func hasBiddingMoves(moves []engine.LegalMove) bool {
	for _, m := range moves {
		// Bidding moves have CardIndex <= MoveBidOffset (-50)
		if m.CardIndex <= engine.MoveBidOffset {
			return true
		}
	}
	return false
}

// getBettingPhaseData finds and parses the betting phase from genome
func getBettingPhaseData(genome *engine.Genome) *engine.BettingPhaseData {
	for _, phase := range genome.TurnPhases {
		if phase.PhaseType == 5 { // BettingPhase
			data, _ := engine.ParseBettingPhaseData(phase.Data)
			return data
		}
	}
	return nil
}

// anyNeedsToAct checks if any player still needs to act in betting round
func anyNeedsToAct(needsToAct []bool) bool {
	for _, needs := range needsToAct {
		if needs {
			return true
		}
	}
	return false
}

// runBettingRound executes a complete betting round
// Returns error string if round fails, empty string on success
func runBettingRound(state *engine.GameState, genome *engine.Genome, bettingPhase *engine.BettingPhaseData, aiType AIPlayerType, metrics *GameMetrics, tensionMetrics *engine.TensionMetrics, detector engine.LeaderDetector) string {
	// Track who needs to act
	needsToAct := make([]bool, state.NumPlayers)
	for i := 0; i < int(state.NumPlayers); i++ {
		p := &state.Players[i]
		needsToAct[i] = !p.HasFolded && !p.IsAllIn && p.Chips > 0
	}

	// Ensure starting player is in bounds (BettingStartPlayer may exceed NumPlayers after rotation)
	currentPlayer := state.BettingStartPlayer % int(state.NumPlayers)
	maxActions := int(state.NumPlayers) * (bettingPhase.MaxRaises + 2) * 2 // Safety limit

	for actionCount := 0; actionCount < maxActions; actionCount++ {
		// Check termination: only one player remains
		if engine.CountActivePlayers(state) <= 1 {
			break
		}

		// Check termination: all remaining players are all-in
		if engine.CountActingPlayers(state) == 0 {
			break
		}

		// Check termination: round complete (all acted and matched)
		if !anyNeedsToAct(needsToAct) && engine.AllBetsMatched(state) {
			break
		}

		// Find next player who needs to act
		startSearch := currentPlayer
		for !needsToAct[currentPlayer] {
			currentPlayer = (currentPlayer + 1) % int(state.NumPlayers)
			if currentPlayer == startSearch {
				// Wrapped around, no one needs to act
				break
			}
		}
		if !needsToAct[currentPlayer] {
			break
		}

		// Generate betting moves
		moves := engine.GenerateBettingMoves(state, bettingPhase, currentPlayer)
		if len(moves) == 0 {
			needsToAct[currentPlayer] = false
			currentPlayer = (currentPlayer + 1) % int(state.NumPlayers)
			continue
		}

		// Metrics tracking
		metrics.TotalDecisions++
		metrics.TotalValidMoves += uint64(len(moves))
		if len(moves) == 1 {
			metrics.ForcedDecisions++
		}

		// Select betting action based on AI type
		var action engine.BettingAction
		switch aiType {
		case GreedyAI:
			handStrength := engine.EvaluateHandStrength(state.Players[currentPlayer].Hand)
			action = engine.SelectGreedyBettingAction(state, moves, handStrength)
		default: // RandomAI and MCTS use random for betting
			action = engine.SelectRandomBettingAction(moves, rand.Intn)
		}

		// Track betting metrics before applying action
		handStrength := engine.EvaluateHandStrength(state.Players[currentPlayer].Hand)

		// Count betting actions (Bet=1, Raise=3, AllIn=4)
		if action == engine.BettingBet || action == engine.BettingRaise || action == engine.BettingAllIn {
			metrics.TotalBets++
			// Count as bluff if betting with weak hand (< 0.3)
			if handStrength < 0.3 {
				metrics.BettingBluffs++
			}
		}
		if action == engine.BettingAllIn {
			metrics.AllInCount++
		}

		oldCurrentBet := state.CurrentBet
		engine.ApplyBettingAction(state, bettingPhase, currentPlayer, action)
		metrics.TotalActions++
		metrics.TotalInteractions++ // Betting is always interactive

		// Update tension tracking after each betting action
		if tensionMetrics != nil && detector != nil {
			tensionMetrics.Update(state, detector)
		}

		// If bet increased, everyone else needs to act again
		if state.CurrentBet > oldCurrentBet {
			for i := 0; i < int(state.NumPlayers); i++ {
				p := &state.Players[i]
				if !p.HasFolded && !p.IsAllIn && p.Chips > 0 && i != currentPlayer {
					needsToAct[i] = true
				}
			}
		}

		needsToAct[currentPlayer] = false
		currentPlayer = (currentPlayer + 1) % int(state.NumPlayers)
		state.TurnNumber++
	}

	return "" // Success
}

// runBettingRoundAsymmetric executes a complete betting round with different AI per player
// Returns error string if round fails, empty string on success
func runBettingRoundAsymmetric(state *engine.GameState, genome *engine.Genome, bettingPhase *engine.BettingPhaseData, p0AIType AIPlayerType, p1AIType AIPlayerType, metrics *GameMetrics) string {
	// Track who needs to act
	needsToAct := make([]bool, state.NumPlayers)
	for i := 0; i < int(state.NumPlayers); i++ {
		p := &state.Players[i]
		needsToAct[i] = !p.HasFolded && !p.IsAllIn && p.Chips > 0
	}

	// Ensure starting player is in bounds (BettingStartPlayer may exceed NumPlayers after rotation)
	currentPlayer := state.BettingStartPlayer % int(state.NumPlayers)
	maxActions := int(state.NumPlayers) * (bettingPhase.MaxRaises + 2) * 2 // Safety limit

	for actionCount := 0; actionCount < maxActions; actionCount++ {
		// Check termination: only one player remains
		if engine.CountActivePlayers(state) <= 1 {
			break
		}

		// Check termination: all remaining players are all-in
		if engine.CountActingPlayers(state) == 0 {
			break
		}

		// Check termination: round complete (all acted and matched)
		if !anyNeedsToAct(needsToAct) && engine.AllBetsMatched(state) {
			break
		}

		// Find next player who needs to act
		startSearch := currentPlayer
		for !needsToAct[currentPlayer] {
			currentPlayer = (currentPlayer + 1) % int(state.NumPlayers)
			if currentPlayer == startSearch {
				// Wrapped around, no one needs to act
				break
			}
		}
		if !needsToAct[currentPlayer] {
			break
		}

		// Generate betting moves
		moves := engine.GenerateBettingMoves(state, bettingPhase, currentPlayer)
		if len(moves) == 0 {
			needsToAct[currentPlayer] = false
			currentPlayer = (currentPlayer + 1) % int(state.NumPlayers)
			continue
		}

		// Metrics tracking
		metrics.TotalDecisions++
		metrics.TotalValidMoves += uint64(len(moves))
		if len(moves) == 1 {
			metrics.ForcedDecisions++
		}

		// Select AI based on current player
		var aiType AIPlayerType
		if currentPlayer == 0 {
			aiType = p0AIType
		} else {
			aiType = p1AIType
		}

		// Select betting action based on AI type
		var action engine.BettingAction
		switch aiType {
		case GreedyAI:
			handStrength := engine.EvaluateHandStrength(state.Players[currentPlayer].Hand)
			action = engine.SelectGreedyBettingAction(state, moves, handStrength)
		default: // RandomAI and MCTS use random for betting
			action = engine.SelectRandomBettingAction(moves, rand.Intn)
		}

		// Track betting metrics before applying action
		handStrength := engine.EvaluateHandStrength(state.Players[currentPlayer].Hand)

		// Count betting actions (Bet=1, Raise=3, AllIn=4)
		if action == engine.BettingBet || action == engine.BettingRaise || action == engine.BettingAllIn {
			metrics.TotalBets++
			// Count as bluff if betting with weak hand (< 0.3)
			if handStrength < 0.3 {
				metrics.BettingBluffs++
			}
		}
		if action == engine.BettingAllIn {
			metrics.AllInCount++
		}

		oldCurrentBet := state.CurrentBet
		engine.ApplyBettingAction(state, bettingPhase, currentPlayer, action)
		metrics.TotalActions++
		metrics.TotalInteractions++ // Betting is always interactive

		// If bet increased, everyone else needs to act again
		if state.CurrentBet > oldCurrentBet {
			for i := 0; i < int(state.NumPlayers); i++ {
				p := &state.Players[i]
				if !p.HasFolded && !p.IsAllIn && p.Chips > 0 && i != currentPlayer {
					needsToAct[i] = true
				}
			}
		}

		needsToAct[currentPlayer] = false
		currentPlayer = (currentPlayer + 1) % int(state.NumPlayers)
		state.TurnNumber++
	}

	return "" // Success
}

// hasBiddingPhase checks if the genome has a BiddingPhase (phase type 7)
func hasBiddingPhase(genome *engine.Genome) bool {
	for _, phase := range genome.TurnPhases {
		if phase.PhaseType == engine.PhaseTypeBidding {
			return true
		}
	}
	return false
}

// getBiddingPhaseData returns the BiddingPhase data from the genome, or nil if none exists
func getBiddingPhaseData(genome *engine.Genome) []byte {
	for _, phase := range genome.TurnPhases {
		if phase.PhaseType == engine.PhaseTypeBidding {
			return phase.Data
		}
	}
	return nil
}

// selectGreedyBid estimates tricks and returns a bid value for greedy AI
func selectGreedyBid(state *engine.GameState, biddingPhase engine.BiddingPhase, playerIdx int) engine.BidMove {
	hand := state.Players[playerIdx].Hand
	handSize := len(hand)

	// Estimate tricks based on high cards
	estimate := 0
	emptyCard := engine.Card{}
	for _, card := range hand {
		if card == emptyCard { // Empty slot
			continue
		}
		// Count high cards (Q=10, K=11, A=12 in 0-indexed)
		if card.Rank >= 10 { // Queen or higher
			estimate++
		}
		// Bonus for spades (trump) - assuming spades is suit 3
		if card.Suit == 3 { // Spades
			estimate++
		}
	}

	// Cap estimate at hand size and adjust to be conservative
	estimate = estimate / 2 // Be conservative
	if estimate > handSize {
		estimate = handSize
	}

	// Ensure within valid bid range
	bid := estimate
	if bid < int(biddingPhase.MinBid) {
		bid = int(biddingPhase.MinBid)
	}
	effectiveMax := int(biddingPhase.MaxBid)
	if handSize < effectiveMax {
		effectiveMax = handSize
	}
	if bid > effectiveMax {
		bid = effectiveMax
	}

	// Consider Nil bid for weak hands
	if biddingPhase.AllowNil && estimate == 0 && biddingPhase.MinBid > 0 {
		return engine.BidMove{Value: 0, IsNil: true}
	}

	return engine.BidMove{Value: bid, IsNil: false}
}

// runBiddingRound executes a complete bidding round for all players
func runBiddingRound(state *engine.GameState, genome *engine.Genome, aiTypes []AIPlayerType) {
	biddingData := getBiddingPhaseData(genome)
	if biddingData == nil {
		return
	}
	biddingPhase, _, bytesRead := engine.ParseBiddingPhase(biddingData)
	if bytesRead == 0 {
		return
	}

	// Reset bidding state
	state.BiddingComplete = false
	for i := 0; i < int(state.NumPlayers); i++ {
		state.Players[i].CurrentBid = -1
		state.Players[i].IsNilBid = false
	}

	// Each player bids in order starting from current player
	startPlayer := int(state.CurrentPlayer)
	for i := 0; i < int(state.NumPlayers); i++ {
		playerIdx := (startPlayer + i) % int(state.NumPlayers)

		// Get bid based on AI type
		var bid engine.BidMove
		aiType := aiTypes[playerIdx]
		switch aiType {
		case GreedyAI:
			bid = selectGreedyBid(state, biddingPhase, playerIdx)
		default: // RandomAI and MCTS use random for bidding
			handSize := len(state.Players[playerIdx].Hand)
			bidMoves := engine.GenerateBidMoves(biddingPhase, handSize)
			if len(bidMoves) > 0 {
				bid = bidMoves[rand.Intn(len(bidMoves))]
			} else {
				bid = engine.BidMove{Value: 1, IsNil: false}
			}
		}

		engine.ApplyBidMove(state, playerIdx, bid)
		state.TurnNumber++
	}
}

// runBiddingRoundAsymmetric executes a complete bidding round with different AI per player (for skill evaluation)
func runBiddingRoundAsymmetric(state *engine.GameState, genome *engine.Genome, p0AIType AIPlayerType, p1AIType AIPlayerType) {
	biddingData := getBiddingPhaseData(genome)
	if biddingData == nil {
		return
	}
	biddingPhase, _, bytesRead := engine.ParseBiddingPhase(biddingData)
	if bytesRead == 0 {
		return
	}

	// Reset bidding state
	state.BiddingComplete = false
	for i := 0; i < int(state.NumPlayers); i++ {
		state.Players[i].CurrentBid = -1
		state.Players[i].IsNilBid = false
	}

	// Each player bids in order starting from current player
	startPlayer := int(state.CurrentPlayer)
	for i := 0; i < int(state.NumPlayers); i++ {
		playerIdx := (startPlayer + i) % int(state.NumPlayers)

		// Select AI based on current player
		var aiType AIPlayerType
		if playerIdx == 0 {
			aiType = p0AIType
		} else {
			aiType = p1AIType
		}

		// Get bid based on AI type
		var bid engine.BidMove
		switch aiType {
		case GreedyAI:
			bid = selectGreedyBid(state, biddingPhase, playerIdx)
		default: // RandomAI and MCTS use random for bidding
			handSize := len(state.Players[playerIdx].Hand)
			bidMoves := engine.GenerateBidMoves(biddingPhase, handSize)
			if len(bidMoves) > 0 {
				bid = bidMoves[rand.Intn(len(bidMoves))]
			} else {
				bid = engine.BidMove{Value: 1, IsNil: false}
			}
		}

		engine.ApplyBidMove(state, playerIdx, bid)
		state.TurnNumber++
	}
}

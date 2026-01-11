package simulation

import (
	"encoding/binary"
	"math/rand"
	"time"

	"github.com/signalnine/cards-evolve/gosim/engine"
	"github.com/signalnine/cards-evolve/gosim/mcts"
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
}

// GameResult holds the outcome of a single game
type GameResult struct {
	WinnerID       int8
	TurnCount      uint32
	DurationNs     uint64
	Error          string
	Metrics        GameMetrics // Phase 1 instrumentation
}

// AggregatedStats summarizes multiple game results
type AggregatedStats struct {
	TotalGames    uint32
	Player0Wins   uint32
	Player1Wins   uint32
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

	// Read cards_per_player from genome setup section
	cardsPerPlayer := 26 // Default for War
	if genome.Header.SetupOffset > 0 && genome.Header.SetupOffset+8 <= int32(len(genome.Bytecode)) {
		setupOffset := genome.Header.SetupOffset
		cardsPerPlayer = int(int32(binary.BigEndian.Uint32(genome.Bytecode[setupOffset : setupOffset+4])))
	}

	// Determine number of players from genome header
	numPlayers := int(genome.Header.PlayerCount)
	if numPlayers == 0 || numPlayers > 4 {
		numPlayers = 2 // Default to 2 players
	}

	// Initialize trick-taking state
	state.NumPlayers = uint8(numPlayers)
	state.CardsPerPlayer = cardsPerPlayer

	// Deal cards to each player
	for i := 0; i < cardsPerPlayer; i++ {
		for p := 0; p < numPlayers; p++ {
			state.DrawCard(uint8(p), engine.LocationDeck)
		}
	}

	// Game loop with turn limit protection
	maxTurns := genome.Header.MaxTurns
	for state.TurnNumber < maxTurns {
		// Check win conditions
		winner := engine.CheckWinConditions(state, genome)
		if winner >= 0 {
			return GameResult{
				WinnerID:   winner,
				TurnCount:  state.TurnNumber,
				DurationNs: uint64(time.Since(start).Nanoseconds()),
				Metrics:    metrics,
			}
		}

		// Generate legal moves
		moves := engine.GenerateLegalMoves(state, genome)
		if len(moves) == 0 {
			// No legal moves - game stuck
			return GameResult{
				WinnerID:   -1,
				TurnCount:  state.TurnNumber,
				DurationNs: uint64(time.Since(start).Nanoseconds()),
				Error:      "no legal moves",
				Metrics:    metrics,
			}
		}

		// Phase 1 instrumentation: decision counting
		metrics.TotalDecisions++
		metrics.TotalValidMoves += uint64(len(moves))
		if len(moves) == 1 {
			metrics.ForcedDecisions++
		}

		// Select and apply move based on AI type
		var move *engine.LegalMove
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

		if move == nil {
			return GameResult{
				WinnerID:   -1,
				TurnCount:  state.TurnNumber,
				DurationNs: uint64(time.Since(start).Nanoseconds()),
				Error:      "AI returned nil move",
				Metrics:    metrics,
			}
		}

		// Phase 1 instrumentation: action and interaction counting
		metrics.TotalActions++
		if isInteraction(state, move, genome) {
			metrics.TotalInteractions++
		}

		engine.ApplyMove(state, move, genome)
	}

	// Max turns reached - draw
	return GameResult{
		WinnerID:   -1,
		TurnCount:  state.TurnNumber,
		DurationNs: uint64(time.Since(start).Nanoseconds()),
		Metrics:    metrics,
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

	cardsPerPlayer := 26
	if genome.Header.SetupOffset > 0 && genome.Header.SetupOffset+8 <= int32(len(genome.Bytecode)) {
		setupOffset := genome.Header.SetupOffset
		cardsPerPlayer = int(int32(binary.BigEndian.Uint32(genome.Bytecode[setupOffset : setupOffset+4])))
	}

	numPlayers := int(genome.Header.PlayerCount)
	if numPlayers == 0 || numPlayers > 4 {
		numPlayers = 2
	}

	state.NumPlayers = uint8(numPlayers)
	state.CardsPerPlayer = cardsPerPlayer

	for i := 0; i < cardsPerPlayer; i++ {
		for p := 0; p < numPlayers; p++ {
			state.DrawCard(uint8(p), engine.LocationDeck)
		}
	}

	maxTurns := genome.Header.MaxTurns
	for state.TurnNumber < maxTurns {
		winner := engine.CheckWinConditions(state, genome)
		if winner >= 0 {
			return GameResult{
				WinnerID:   winner,
				TurnCount:  state.TurnNumber,
				DurationNs: uint64(time.Since(start).Nanoseconds()),
				Metrics:    metrics,
			}
		}

		moves := engine.GenerateLegalMoves(state, genome)
		if len(moves) == 0 {
			return GameResult{
				WinnerID:   -1,
				TurnCount:  state.TurnNumber,
				DurationNs: uint64(time.Since(start).Nanoseconds()),
				Error:      "no legal moves",
				Metrics:    metrics,
			}
		}

		metrics.TotalDecisions++
		metrics.TotalValidMoves += uint64(len(moves))
		if len(moves) == 1 {
			metrics.ForcedDecisions++
		}

		// Select AI based on current player
		var aiType AIPlayerType
		if state.CurrentPlayer == 0 {
			aiType = p0AIType
		} else {
			aiType = p1AIType
		}

		var move *engine.LegalMove
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

		if move == nil {
			return GameResult{
				WinnerID:   -1,
				TurnCount:  state.TurnNumber,
				DurationNs: uint64(time.Since(start).Nanoseconds()),
				Error:      "AI returned nil move",
				Metrics:    metrics,
			}
		}

		metrics.TotalActions++
		if isInteraction(state, move, genome) {
			metrics.TotalInteractions++
		}

		engine.ApplyMove(state, move, genome)
	}

	return GameResult{
		WinnerID:   -1,
		TurnCount:  state.TurnNumber,
		DurationNs: uint64(time.Since(start).Nanoseconds()),
		Metrics:    metrics,
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
	case 3: // DiscardPhase
		// Regular discard doesn't affect opponent
		return false
	}

	return false
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
	}

	turnCounts := make([]uint32, 0, len(results))
	totalDuration := uint64(0)

	for _, result := range results {
		if result.Error != "" {
			stats.Errors++
			continue
		}

		switch result.WinnerID {
		case 0:
			stats.Player0Wins++
		case 1:
			stats.Player1Wins++
		default:
			stats.Draws++
		}

		turnCounts = append(turnCounts, result.TurnCount)
		totalDuration += result.DurationNs

		// Phase 1 instrumentation: aggregate metrics from each game
		stats.TotalDecisions += result.Metrics.TotalDecisions
		stats.TotalValidMoves += result.Metrics.TotalValidMoves
		stats.ForcedDecisions += result.Metrics.ForcedDecisions
		stats.TotalInteractions += result.Metrics.TotalInteractions
		stats.TotalActions += result.Metrics.TotalActions
	}

	// Calculate averages
	if len(turnCounts) > 0 {
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

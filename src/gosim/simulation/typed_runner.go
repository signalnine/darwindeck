package simulation

import (
	"math/rand"
	"runtime"
	"sync"
	"time"

	"github.com/signalnine/darwindeck/gosim/engine"
	"github.com/signalnine/darwindeck/gosim/genome"
	"github.com/signalnine/darwindeck/gosim/mcts"
)

// TypedGameJob represents a simulation job for typed genomes.
type TypedGameJob struct {
	SimID int
	Seed  uint64
}

// RunBatchTyped simulates multiple games with a typed genome and AI configuration.
// This is the new entry point for the pure Go evolution system.
// NOTE: This is the serial version. Use RunBatchTypedParallel for parallel execution.
func RunBatchTyped(g *genome.GameGenome, numGames int, aiType AIPlayerType, mctsIterations int, seed uint64) AggregatedStats {
	results := make([]GameResult, numGames)
	rng := rand.New(rand.NewSource(int64(seed)))

	for i := 0; i < numGames; i++ {
		gameSeed := rng.Uint64()
		results[i] = RunSingleGameTyped(g, aiType, mctsIterations, gameSeed)
	}

	return aggregateResults(results)
}

// RunBatchTypedParallel simulates multiple games in parallel using typed genomes.
// This achieves significant speedup on multi-core systems.
func RunBatchTypedParallel(g *genome.GameGenome, numGames int, aiType AIPlayerType, mctsIterations int, seed uint64) AggregatedStats {
	numWorkers := runtime.NumCPU()
	return RunBatchTypedParallelN(g, numGames, aiType, mctsIterations, seed, numWorkers)
}

// RunBatchTypedParallelN simulates multiple games in parallel with a specified number of workers.
func RunBatchTypedParallelN(g *genome.GameGenome, numGames int, aiType AIPlayerType, mctsIterations int, seed uint64, numWorkers int) AggregatedStats {
	if numWorkers <= 0 {
		numWorkers = runtime.NumCPU()
	}

	jobs := make(chan TypedGameJob, numGames)
	results := make(chan GameResult, numGames)

	var wg sync.WaitGroup

	// Start workers
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go typedWorker(&wg, jobs, results, g, aiType, mctsIterations)
	}

	// Generate deterministic seeds
	rng := rand.New(rand.NewSource(int64(seed)))

	// Queue all simulation jobs
	for i := 0; i < numGames; i++ {
		gameSeed := rng.Uint64()
		jobs <- TypedGameJob{
			SimID: i,
			Seed:  gameSeed,
		}
	}
	close(jobs)

	// Wait for all workers to complete, then close results
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect and aggregate results
	allResults := make([]GameResult, 0, numGames)
	for result := range results {
		allResults = append(allResults, result)
	}

	return aggregateResults(allResults)
}

// typedWorker processes typed simulation jobs from the jobs channel.
func typedWorker(wg *sync.WaitGroup, jobs <-chan TypedGameJob, results chan<- GameResult, g *genome.GameGenome, aiType AIPlayerType, mctsIterations int) {
	defer wg.Done()

	for job := range jobs {
		result := RunSingleGameTyped(g, aiType, mctsIterations, job.Seed)
		results <- result
	}
}

// GameTimeout is the maximum duration for a single game (prevents infinite loops)
const GameTimeout = 100 * time.Millisecond

// RunSingleGameTyped plays one complete game using a typed genome.
func RunSingleGameTyped(g *genome.GameGenome, aiType AIPlayerType, mctsIterations int, seed uint64) GameResult {
	start := time.Now()
	var metrics GameMetrics

	// Initialize game state
	state := engine.GetState()
	defer engine.PutState(state)

	// Setup deck and shuffle
	setupDeck(state, seed)

	// Read setup from typed genome
	cardsPerPlayer := g.Setup.CardsPerPlayer
	if cardsPerPlayer <= 0 {
		cardsPerPlayer = 26 // Default for War
	}

	initialDiscardCount := g.Setup.DealToTableau
	startingChips := g.Setup.StartingChips

	// Determine number of players (default to 2)
	numPlayers := 2 // TODO: Add PlayerCount to GameGenome if needed

	state.NumPlayers = uint8(numPlayers)
	state.CardsPerPlayer = cardsPerPlayer

	// Set tableau mode from typed genome
	state.TableauMode = uint8(g.TurnStructure.TableauMode)
	state.SequenceDirection = uint8(g.TurnStructure.SequenceDirection)

	// Initialize teams if configured
	if g.Teams != nil && g.Teams.Enabled && len(g.Teams.Teams) > 0 {
		teams := make([][]int, len(g.Teams.Teams))
		for i, team := range g.Teams.Teams {
			teams[i] = make([]int, len(team))
			copy(teams[i], team)
		}
		state.InitializeTeams(teams)
	}

	// Deal cards to each player
	for i := 0; i < cardsPerPlayer; i++ {
		for p := 0; p < numPlayers; p++ {
			state.DrawCard(uint8(p), engine.LocationDeck)
		}
	}

	// Deal initial cards to discard/tableau
	if initialDiscardCount > 0 && len(state.Deck) >= initialDiscardCount {
		// Initialize tableau pile if needed for TableauMode games
		if state.TableauMode != 0 && len(state.Tableau) == 0 {
			state.Tableau = make([][]engine.Card, 1)
			state.Tableau[0] = make([]engine.Card, 0, initialDiscardCount)
		}
		for i := 0; i < initialDiscardCount; i++ {
			if len(state.Deck) > 0 {
				card := state.Deck[len(state.Deck)-1]
				state.Deck = state.Deck[:len(state.Deck)-1]
				if state.TableauMode != 0 {
					state.Tableau[0] = append(state.Tableau[0], card)
				} else {
					state.Discard = append(state.Discard, card)
				}
			}
		}
	}

	// Initialize chips if this genome uses betting
	if startingChips > 0 {
		state.InitializeChips(startingChips)
	}

	// Create bytecode genome for compatibility with existing win condition checks
	// TODO: Implement typed win condition checking
	bytecodeGenome := createCompatGenome(g)

	// Initialize tension tracking
	detector := engine.SelectLeaderDetector(bytecodeGenome)
	tensionMetrics := engine.NewTensionMetrics(int(state.NumPlayers))

	// Game loop with turn limit protection
	maxTurns := uint32(g.TurnStructure.MaxTurns)
	if maxTurns == 0 {
		maxTurns = 1000 // Default
	}

	for state.TurnNumber < maxTurns {
		// Check timeout to prevent infinite loops from bad genomes
		if time.Since(start) > GameTimeout {
			tensionMetrics.Finalize(-1)
			return GameResult{
				WinnerID:    -1,
				WinningTeam: -1,
				TurnCount:   state.TurnNumber,
				DurationNs:  uint64(time.Since(start).Nanoseconds()),
				Error:       "timeout",
				Metrics:     metrics,
			}
		}

		// Check win conditions
		winner := checkWinConditionsTyped(state, g)
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

		// Generate legal moves using typed interpreter
		moves := genome.GenerateLegalMovesTyped(state, g)

		// Check if this is a betting phase
		if hasBettingMoves(moves) {
			bettingPhase := findBettingPhase(g)
			if bettingPhase != nil {
				err := runBettingRoundTyped(state, g, bettingPhase, aiType, &metrics, tensionMetrics, detector)
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

				state.BettingComplete = true

				// Resolve showdown after betting
				winners := engine.ResolveShowdown(state)
				if len(winners) == 1 {
					engine.AwardPot(state, winners)
					metrics.FoldWins++
				} else if len(winners) > 1 {
					winner := engine.FindBestPokerWinner(state, int(state.NumPlayers))
					if winner >= 0 {
						engine.AwardPot(state, []int{int(winner)})
						metrics.ShowdownWins++
					}
				}

				state.ResetHand()
				continue
			}
		}

		// Check if this is a bidding phase
		if hasBiddingMoves(moves) {
			aiTypes := make([]AIPlayerType, state.NumPlayers)
			for i := range aiTypes {
				aiTypes[i] = aiType
			}
			runBiddingRoundTyped(state, g, aiTypes)
			continue
		}

		if len(moves) == 0 {
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

		// Phase 1 instrumentation
		metrics.TotalDecisions++
		metrics.TotalValidMoves += uint64(len(moves))
		metrics.TotalHandSize += uint64(len(state.Players[state.CurrentPlayer].Hand))
		if len(moves) == 1 {
			metrics.ForcedDecisions++
		}

		// Select and apply move
		var move *engine.LegalMove

		if len(moves) == 1 {
			move = &moves[0]
		} else {
			switch aiType {
			case RandomAI:
				move = &moves[rand.Intn(len(moves))]
			case GreedyAI:
				move = selectGreedyMoveTyped(state, g, moves)
			case MCTS100AI, MCTS500AI, MCTS1000AI, MCTS2000AI:
				// Use bytecode genome for MCTS (requires existing infrastructure)
				move = mcts.Search(state, bytecodeGenome, mctsIterations, mcts.DefaultExplorationParam)
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

		// Instrumentation
		metrics.TotalActions++
		if isInteractionTyped(state, move, g) {
			metrics.TotalInteractions++
		}

		applyMoveTyped(state, move, g)

		// Update tension tracking
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

// checkWinConditionsTyped checks win conditions from typed genome.
func checkWinConditionsTyped(state *engine.GameState, g *genome.GameGenome) int8 {
	for _, wc := range g.WinConditions {
		switch wc.Type {
		case genome.WinTypeEmptyHand:
			// First player to empty hand wins
			for i := 0; i < int(state.NumPlayers); i++ {
				if len(state.Players[i].Hand) == 0 {
					return int8(i)
				}
			}

		case genome.WinTypeCaptureAll:
			// Player who captures all cards wins
			for i := 0; i < int(state.NumPlayers); i++ {
				otherHandsEmpty := true
				for j := 0; j < int(state.NumPlayers); j++ {
					if j != i && len(state.Players[j].Hand) > 0 {
						otherHandsEmpty = false
						break
					}
				}
				if otherHandsEmpty && len(state.Players[i].Hand) > 0 {
					return int8(i)
				}
			}

		case genome.WinTypeAllHandsEmpty:
			// All hands empty - determine winner by score/tricks
			allEmpty := true
			for i := 0; i < int(state.NumPlayers); i++ {
				if len(state.Players[i].Hand) > 0 {
					allEmpty = false
					break
				}
			}
			if allEmpty {
				// Find winner by score
				bestPlayer := -1
				bestScore := int32(-1000000)
				for i := 0; i < int(state.NumPlayers); i++ {
					if state.Players[i].Score > bestScore {
						bestScore = state.Players[i].Score
						bestPlayer = i
					}
				}
				if bestPlayer >= 0 {
					return int8(bestPlayer)
				}
			}

		case genome.WinTypeHighScore:
			// First to reach threshold wins
			for i := 0; i < int(state.NumPlayers); i++ {
				if state.Players[i].Score >= wc.Threshold {
					return int8(i)
				}
			}

		case genome.WinTypeLowScore:
			// Lowest score when someone hits threshold loses, others win
			anyHitThreshold := false
			for i := 0; i < int(state.NumPlayers); i++ {
				if state.Players[i].Score >= wc.Threshold {
					anyHitThreshold = true
					break
				}
			}
			if anyHitThreshold {
				lowestScore := int32(1000000)
				winner := int8(-1)
				for i := 0; i < int(state.NumPlayers); i++ {
					if state.Players[i].Score < lowestScore {
						lowestScore = state.Players[i].Score
						winner = int8(i)
					}
				}
				return winner
			}

		case genome.WinTypeMostCaptured:
			// When all hands empty, most tricks won wins (used as captured proxy)
			allEmpty := true
			for i := 0; i < int(state.NumPlayers); i++ {
				if len(state.Players[i].Hand) > 0 {
					allEmpty = false
					break
				}
			}
			if allEmpty && len(state.Deck) == 0 {
				bestPlayer := -1
				mostTricks := int8(0)
				for i := 0; i < int(state.NumPlayers); i++ {
					if state.Players[i].TricksWon > mostTricks {
						mostTricks = state.Players[i].TricksWon
						bestPlayer = i
					}
				}
				if bestPlayer >= 0 {
					return int8(bestPlayer)
				}
			}

		case genome.WinTypeBestHand:
			// Handled by showdown - not a turn-based win condition
			continue

		case genome.WinTypeFirstToScore:
			// Same as high score
			for i := 0; i < int(state.NumPlayers); i++ {
				if state.Players[i].Score >= wc.Threshold {
					return int8(i)
				}
			}
		}
	}

	return -1 // No winner yet
}

// findBettingPhase returns the first BettingPhase in the genome, or nil.
func findBettingPhase(g *genome.GameGenome) *genome.BettingPhase {
	for _, phase := range g.TurnStructure.Phases {
		if bp, ok := phase.(*genome.BettingPhase); ok {
			return bp
		}
	}
	return nil
}

// findBiddingPhase returns the first BiddingPhase in the genome, or nil.
func findBiddingPhase(g *genome.GameGenome) *genome.BiddingPhase {
	for _, phase := range g.TurnStructure.Phases {
		if bp, ok := phase.(*genome.BiddingPhase); ok {
			return bp
		}
	}
	return nil
}

// hasBettingMoves checks if any moves are betting actions.
func hasBettingMoves(moves []engine.LegalMove) bool {
	for _, m := range moves {
		if m.CardIndex <= engine.MoveBettingCheck && m.CardIndex >= engine.MoveBettingFold {
			return true
		}
	}
	return false
}

// runBettingRoundTyped executes a betting round using typed genome.
func runBettingRoundTyped(state *engine.GameState, g *genome.GameGenome, bettingPhase *genome.BettingPhase, aiType AIPlayerType, metrics *GameMetrics, tensionMetrics *engine.TensionMetrics, detector engine.LeaderDetector) string {
	// Convert to engine type for compatibility
	engineBettingPhase := &engine.BettingPhaseData{
		MinBet:    bettingPhase.MinBet,
		MaxRaises: bettingPhase.MaxRaises,
	}

	// Track who needs to act
	needsToAct := make([]bool, state.NumPlayers)
	for i := 0; i < int(state.NumPlayers); i++ {
		p := &state.Players[i]
		needsToAct[i] = !p.HasFolded && !p.IsAllIn && p.Chips > 0
	}

	currentPlayer := state.BettingStartPlayer % int(state.NumPlayers)
	maxActions := int(state.NumPlayers) * (bettingPhase.MaxRaises + 2) * 2

	for actionCount := 0; actionCount < maxActions; actionCount++ {
		if engine.CountActivePlayers(state) <= 1 {
			break
		}
		if engine.CountActingPlayers(state) == 0 {
			break
		}
		if !anyNeedsToAct(needsToAct) && engine.AllBetsMatched(state) {
			break
		}

		startSearch := currentPlayer
		for !needsToAct[currentPlayer] {
			currentPlayer = (currentPlayer + 1) % int(state.NumPlayers)
			if currentPlayer == startSearch {
				break
			}
		}
		if !needsToAct[currentPlayer] {
			break
		}

		moves := engine.GenerateBettingMoves(state, engineBettingPhase, currentPlayer)
		if len(moves) == 0 {
			needsToAct[currentPlayer] = false
			currentPlayer = (currentPlayer + 1) % int(state.NumPlayers)
			continue
		}

		metrics.TotalDecisions++
		metrics.TotalValidMoves += uint64(len(moves))
		if len(moves) == 1 {
			metrics.ForcedDecisions++
		}

		var action engine.BettingAction
		switch aiType {
		case GreedyAI:
			handStrength := engine.EvaluateHandStrength(state.Players[currentPlayer].Hand)
			action = engine.SelectGreedyBettingAction(state, moves, handStrength)
		default:
			action = engine.SelectRandomBettingAction(moves, rand.Intn)
		}

		handStrength := engine.EvaluateHandStrength(state.Players[currentPlayer].Hand)
		if action == engine.BettingBet || action == engine.BettingRaise || action == engine.BettingAllIn {
			metrics.TotalBets++
			if handStrength < 0.3 {
				metrics.BettingBluffs++
			}
		}
		if action == engine.BettingAllIn {
			metrics.AllInCount++
		}

		oldCurrentBet := state.CurrentBet
		engine.ApplyBettingAction(state, engineBettingPhase, currentPlayer, action)
		metrics.TotalActions++
		metrics.TotalInteractions++

		if tensionMetrics != nil && detector != nil {
			tensionMetrics.Update(state, detector)
		}

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

	return ""
}

// runBiddingRoundTyped executes a bidding round using typed genome.
func runBiddingRoundTyped(state *engine.GameState, g *genome.GameGenome, aiTypes []AIPlayerType) {
	biddingPhase := findBiddingPhase(g)
	if biddingPhase == nil {
		return
	}

	// Convert to engine type
	engineBiddingPhase := engine.BiddingPhase{
		MinBid:   biddingPhase.MinBid,
		MaxBid:   biddingPhase.MaxBid,
		AllowNil: biddingPhase.AllowNil,
	}

	// Reset bidding state
	state.BiddingComplete = false
	for i := 0; i < int(state.NumPlayers); i++ {
		state.Players[i].CurrentBid = -1
		state.Players[i].IsNilBid = false
	}

	startPlayer := int(state.CurrentPlayer)
	for i := 0; i < int(state.NumPlayers); i++ {
		playerIdx := (startPlayer + i) % int(state.NumPlayers)

		var bid engine.BidMove
		aiType := aiTypes[playerIdx]
		switch aiType {
		case GreedyAI:
			bid = selectGreedyBid(state, engineBiddingPhase, playerIdx)
		default:
			handSize := len(state.Players[playerIdx].Hand)
			bidMoves := engine.GenerateBidMoves(engineBiddingPhase, handSize)
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

// selectGreedyMoveTyped picks the move that maximizes immediate score.
func selectGreedyMoveTyped(state *engine.GameState, g *genome.GameGenome, moves []engine.LegalMove) *engine.LegalMove {
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

// isInteractionTyped determines if a move affects opponent state.
func isInteractionTyped(state *engine.GameState, move *engine.LegalMove, g *genome.GameGenome) bool {
	if move.PhaseIndex >= len(g.TurnStructure.Phases) {
		return false
	}

	phase := g.TurnStructure.Phases[move.PhaseIndex]

	switch phase.(type) {
	case *genome.DrawPhase:
		if move.TargetLoc == engine.LocationOpponentHand {
			return true
		}
	case *genome.PlayPhase:
		if move.TargetLoc == engine.LocationTableau {
			return true
		}
	case *genome.TrickPhase:
		return true
	case *genome.ClaimPhase:
		return true
	case *genome.BettingPhase:
		return true
	}

	return false
}

// applyMoveTyped applies a move using typed phase information.
func applyMoveTyped(state *engine.GameState, move *engine.LegalMove, g *genome.GameGenome) {
	// Use existing engine.ApplyMove with a compatibility wrapper
	bytecodeGenome := createCompatGenome(g)
	engine.ApplyMove(state, move, bytecodeGenome)
}

// createCompatGenome creates a bytecode genome for compatibility with existing engine functions.
// This is a temporary bridge during the transition to pure typed genomes.
func createCompatGenome(g *genome.GameGenome) *engine.Genome {
	// Create minimal bytecode genome for compatibility
	result := &engine.Genome{
		Header: &engine.BytecodeHeader{
			MaxTurns:          uint32(g.TurnStructure.MaxTurns),
			TableauMode:       uint8(g.TurnStructure.TableauMode),
			SequenceDirection: uint8(g.TurnStructure.SequenceDirection),
			PlayerCount:       2, // Default
		},
		TurnPhases:    make([]engine.PhaseDescriptor, len(g.TurnStructure.Phases)),
		WinConditions: make([]engine.WinCondition, len(g.WinConditions)),
		Effects:       make(map[uint8]engine.SpecialEffect),
	}

	// Convert phases to descriptors
	for i, phase := range g.TurnStructure.Phases {
		result.TurnPhases[i] = engine.PhaseDescriptor{
			PhaseType: phase.PhaseType(),
			// Data is not needed for basic compatibility
		}
	}

	// Convert win conditions
	for i, wc := range g.WinConditions {
		result.WinConditions[i] = engine.WinCondition{
			WinType:   uint8(wc.Type),
			Threshold: wc.Threshold,
		}
	}

	// Convert effects
	for _, effect := range g.Effects {
		result.Effects[effect.TriggerRank] = engine.SpecialEffect{
			TriggerRank: effect.TriggerRank,
			EffectType:  uint8(effect.Effect),
			Target:      effect.Target,
			Value:       effect.Value,
		}
	}

	return result
}

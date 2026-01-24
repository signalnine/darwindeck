package main

/*
#include <stdlib.h>
#include <string.h>
*/
import "C"
import (
	"unsafe"

	flatbuffers "github.com/google/flatbuffers/go"
	"github.com/signalnine/darwindeck/gosim/bindings/cardsim"
	"github.com/signalnine/darwindeck/gosim/engine"
	"github.com/signalnine/darwindeck/gosim/simulation"
)

// AggStats holds aggregated simulation results
type AggStats struct {
	TotalGames    uint32
	Wins          []uint32 // Wins per player (index = player ID)
	PlayerCount   uint8    // Number of players (2-4)
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
	TotalHandSize     uint64

	// Bluffing metrics
	TotalClaims       uint64
	TotalBluffs       uint64
	TotalChallenges   uint64
	SuccessfulBluffs  uint64
	SuccessfulCatches uint64

	// Betting metrics
	TotalBets     uint64
	BettingBluffs uint64
	FoldWins      uint64
	ShowdownWins  uint64
	AllInCount    uint64

	// Tension metrics
	LeadChanges      uint32
	DecisiveTurnPct  float32
	ClosestMargin    float32
	TrailingWinners  uint32

	// Solitaire detection metrics
	MoveDisruptionEvents uint64
	ContentionEvents     uint64
	ForcedResponseEvents uint64
	OpponentTurnCount    uint64
}

//export SimulateBatch
func SimulateBatch(requestPtr unsafe.Pointer, requestLen C.int, responseLen *C.int) unsafe.Pointer {
	// Parse Flatbuffers request
	requestBytes := C.GoBytes(requestPtr, requestLen)
	batchRequest := cardsim.GetRootAsBatchRequest(requestBytes, 0)

	// Create response builder
	builder := flatbuffers.NewBuilder(1024)

	// Process each simulation request
	requestCount := batchRequest.RequestsLength()
	resultOffsets := make([]flatbuffers.UOffsetT, requestCount)

	for i := 0; i < requestCount; i++ {
		req := new(cardsim.SimulationRequest)
		if !batchRequest.Requests(req, i) {
			continue
		}

		// Parse genome bytecode
		genomeBytecode := req.GenomeBytecodeBytes()
		genome, err := engine.ParseGenome(genomeBytecode)
		if err != nil {
			// Return error stats
			stats := &AggStats{
				TotalGames: req.NumGames(),
				Errors:     req.NumGames(),
			}
			resultOffsets[i] = serializeStats(builder, stats)
			continue
		}

		// Determine AI types
		aiType := simulation.AIPlayerType(req.AiPlayerType())
		mctsIter := int(req.MctsIterations())
		seed := req.RandomSeed()

		// Get player count (default to 2 for backward compatibility)
		playerCount := int(req.PlayerCount())
		if playerCount == 0 || playerCount < 2 || playerCount > 4 {
			playerCount = 2
		}

		// Build per-player AI types array
		// Priority: ai_types array > legacy player0/player1_ai_type > default ai_player_type
		aiTypes := make([]simulation.AIPlayerType, playerCount)
		for p := 0; p < playerCount; p++ {
			aiTypes[p] = aiType // Default
		}

		// Check for new ai_types array (preferred)
		if req.AiTypesLength() > 0 {
			for p := 0; p < playerCount && p < req.AiTypesLength(); p++ {
				override := req.AiTypes(p)
				if override > 0 {
					aiTypes[p] = simulation.AIPlayerType(override - 1)
				}
			}
		} else {
			// Fallback to legacy player0/player1_ai_type fields
			p0Override := req.Player0AiType()
			p1Override := req.Player1AiType()
			if p0Override > 0 {
				aiTypes[0] = simulation.AIPlayerType(p0Override - 1)
			}
			if p1Override > 0 && playerCount > 1 {
				aiTypes[1] = simulation.AIPlayerType(p1Override - 1)
			}
		}

		// Check if all players have the same AI type (symmetric)
		symmetric := true
		for p := 1; p < playerCount; p++ {
			if aiTypes[p] != aiTypes[0] {
				symmetric = false
				break
			}
		}

		// Run batch simulation sequentially.
		// Go-level parallelism is disabled because it causes issues with
		// Python multiprocessing + 'spawn' context + CGo:
		// - Even limited parallelism (4 Go workers) causes hangs
		// - The goroutines don't play well with Python process management
		// Python handles parallelism at the genome level (many workers),
		// so Go just needs to simulate quickly without spawning goroutines.
		var simStats simulation.AggregatedStats
		if symmetric {
			simStats = simulation.RunBatch(genome, int(req.NumGames()), aiTypes[0], mctsIter, seed)
		} else {
			// For now, asymmetric only supports 2 players
			simStats = simulation.RunBatchAsymmetric(genome, int(req.NumGames()), aiTypes[0], aiTypes[1], mctsIter, seed)
		}

		// Convert to AggStats
		// Copy wins slice, trimming to actual player count
		wins := make([]uint32, playerCount)
		for p := 0; p < playerCount && p < len(simStats.Wins); p++ {
			wins[p] = simStats.Wins[p]
		}

		stats := &AggStats{
			TotalGames:        simStats.TotalGames,
			Wins:              wins,
			PlayerCount:       uint8(playerCount),
			Draws:             simStats.Draws,
			AvgTurns:          simStats.AvgTurns,
			MedianTurns:       simStats.MedianTurns,
			AvgDurationNs:     simStats.AvgDurationNs,
			Errors:            simStats.Errors,
			TotalDecisions:    simStats.TotalDecisions,
			TotalValidMoves:   simStats.TotalValidMoves,
			ForcedDecisions:   simStats.ForcedDecisions,
			TotalInteractions: simStats.TotalInteractions,
			TotalActions:      simStats.TotalActions,
			TotalHandSize:     simStats.TotalHandSize,
			// Bluffing metrics
			TotalClaims:       simStats.TotalClaims,
			TotalBluffs:       simStats.TotalBluffs,
			TotalChallenges:   simStats.TotalChallenges,
			SuccessfulBluffs:  simStats.SuccessfulBluffs,
			SuccessfulCatches: simStats.SuccessfulCatches,
			// Betting metrics
			TotalBets:     simStats.TotalBets,
			BettingBluffs: simStats.BettingBluffs,
			FoldWins:      simStats.FoldWins,
			ShowdownWins:  simStats.ShowdownWins,
			AllInCount:    simStats.AllInCount,
			// Tension metrics (aggregated from individual games)
			LeadChanges:      simStats.LeadChanges,
			DecisiveTurnPct:  simStats.DecisiveTurnPct,
			ClosestMargin:    simStats.ClosestMargin,
			TrailingWinners:  simStats.TrailingWinners,
			// Solitaire detection metrics
			MoveDisruptionEvents: simStats.MoveDisruptionEvents,
			ContentionEvents:     simStats.ContentionEvents,
			ForcedResponseEvents: simStats.ForcedResponseEvents,
			OpponentTurnCount:    simStats.OpponentTurnCount,
		}

		// Serialize result
		resultOffsets[i] = serializeStats(builder, stats)
	}

	// Build response
	cardsim.BatchResponseStartResultsVector(builder, requestCount)
	for i := requestCount - 1; i >= 0; i-- {
		builder.PrependUOffsetT(resultOffsets[i])
	}
	resultsVec := builder.EndVector(requestCount)

	cardsim.BatchResponseStart(builder)
	cardsim.BatchResponseAddBatchId(builder, batchRequest.BatchId())
	cardsim.BatchResponseAddResults(builder, resultsVec)
	response := cardsim.BatchResponseEnd(builder)

	builder.Finish(response)

	// Get response bytes
	responseBytes := builder.FinishedBytes()
	*responseLen = C.int(len(responseBytes))

	// Allocate C memory for response (caller must free)
	cBytes := C.malloc(C.size_t(len(responseBytes)))
	if cBytes == nil {
		*responseLen = 0
		return nil
	}

	// Copy Go bytes to C memory
	C.memcpy(cBytes, unsafe.Pointer(&responseBytes[0]), C.size_t(len(responseBytes)))

	return cBytes
}

//export FreeResponse
func FreeResponse(ptr unsafe.Pointer) {
	C.free(ptr)
}

func serializeStats(builder *flatbuffers.Builder, stats *AggStats) flatbuffers.UOffsetT {
	// Build wins vector first (must be created before table)
	var winsOffset flatbuffers.UOffsetT
	if len(stats.Wins) > 0 {
		cardsim.AggregatedStatsStartWinsVector(builder, len(stats.Wins))
		// Add in reverse order (FlatBuffers convention)
		for i := len(stats.Wins) - 1; i >= 0; i-- {
			builder.PrependUint32(stats.Wins[i])
		}
		winsOffset = builder.EndVector(len(stats.Wins))
	}

	// Get player0/player1 wins for backward compatibility
	player0Wins := uint32(0)
	player1Wins := uint32(0)
	if len(stats.Wins) > 0 {
		player0Wins = stats.Wins[0]
	}
	if len(stats.Wins) > 1 {
		player1Wins = stats.Wins[1]
	}

	cardsim.AggregatedStatsStart(builder)
	cardsim.AggregatedStatsAddTotalGames(builder, stats.TotalGames)
	// Deprecated fields for backward compatibility
	cardsim.AggregatedStatsAddPlayer0Wins(builder, player0Wins)
	cardsim.AggregatedStatsAddPlayer1Wins(builder, player1Wins)
	cardsim.AggregatedStatsAddDraws(builder, stats.Draws)
	cardsim.AggregatedStatsAddAvgTurns(builder, stats.AvgTurns)
	cardsim.AggregatedStatsAddMedianTurns(builder, stats.MedianTurns)
	cardsim.AggregatedStatsAddAvgDurationNs(builder, stats.AvgDurationNs)
	cardsim.AggregatedStatsAddErrors(builder, stats.Errors)
	// N-player support
	if winsOffset > 0 {
		cardsim.AggregatedStatsAddWins(builder, winsOffset)
	}
	cardsim.AggregatedStatsAddPlayerCount(builder, stats.PlayerCount)
	// Phase 1 instrumentation fields
	cardsim.AggregatedStatsAddTotalDecisions(builder, stats.TotalDecisions)
	cardsim.AggregatedStatsAddTotalValidMoves(builder, stats.TotalValidMoves)
	cardsim.AggregatedStatsAddForcedDecisions(builder, stats.ForcedDecisions)
	cardsim.AggregatedStatsAddTotalHandSize(builder, stats.TotalHandSize)
	cardsim.AggregatedStatsAddTotalInteractions(builder, stats.TotalInteractions)
	cardsim.AggregatedStatsAddTotalActions(builder, stats.TotalActions)
	// Bluffing metrics
	cardsim.AggregatedStatsAddTotalClaims(builder, stats.TotalClaims)
	cardsim.AggregatedStatsAddTotalBluffs(builder, stats.TotalBluffs)
	cardsim.AggregatedStatsAddTotalChallenges(builder, stats.TotalChallenges)
	cardsim.AggregatedStatsAddSuccessfulBluffs(builder, stats.SuccessfulBluffs)
	cardsim.AggregatedStatsAddSuccessfulCatches(builder, stats.SuccessfulCatches)
	// Betting metrics
	cardsim.AggregatedStatsAddTotalBets(builder, stats.TotalBets)
	cardsim.AggregatedStatsAddBettingBluffs(builder, stats.BettingBluffs)
	cardsim.AggregatedStatsAddFoldWins(builder, stats.FoldWins)
	cardsim.AggregatedStatsAddShowdownWins(builder, stats.ShowdownWins)
	cardsim.AggregatedStatsAddAllInCount(builder, stats.AllInCount)
	// Tension metrics
	cardsim.AggregatedStatsAddLeadChanges(builder, stats.LeadChanges)
	cardsim.AggregatedStatsAddDecisiveTurnPct(builder, stats.DecisiveTurnPct)
	cardsim.AggregatedStatsAddClosestMargin(builder, stats.ClosestMargin)
	cardsim.AggregatedStatsAddTrailingWinners(builder, stats.TrailingWinners)
	// Solitaire detection metrics
	cardsim.AggregatedStatsAddMoveDisruptionEvents(builder, stats.MoveDisruptionEvents)
	cardsim.AggregatedStatsAddContentionEvents(builder, stats.ContentionEvents)
	cardsim.AggregatedStatsAddForcedResponseEvents(builder, stats.ForcedResponseEvents)
	cardsim.AggregatedStatsAddOpponentTurnCount(builder, stats.OpponentTurnCount)
	return cardsim.AggregatedStatsEnd(builder)
}

func main() {} // Required for CGo

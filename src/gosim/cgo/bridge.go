package main

/*
#include <stdlib.h>
#include <string.h>
*/
import "C"
import (
	"unsafe"

	flatbuffers "github.com/google/flatbuffers/go"
	"github.com/signalnine/cards-evolve/gosim/bindings/cardsim"
	"github.com/signalnine/cards-evolve/gosim/engine"
	"github.com/signalnine/cards-evolve/gosim/simulation"
)

// AggStats holds aggregated simulation results
type AggStats struct {
	TotalGames    uint32
	Player0Wins   uint32
	Player1Wins   uint32
	Draws         uint32
	AvgTurns      float32
	MedianTurns   uint32
	AvgDurationNs uint64
	Errors        uint32
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

		// Run batch simulation
		aiType := simulation.AIPlayerType(req.AiPlayerType())
		mctsIter := int(req.MctsIterations())
		seed := req.RandomSeed()

		simStats := simulation.RunBatch(genome, int(req.NumGames()), aiType, mctsIter, seed)

		// Convert to AggStats
		stats := &AggStats{
			TotalGames:    simStats.TotalGames,
			Player0Wins:   simStats.Player0Wins,
			Player1Wins:   simStats.Player1Wins,
			Draws:         simStats.Draws,
			AvgTurns:      simStats.AvgTurns,
			MedianTurns:   simStats.MedianTurns,
			AvgDurationNs: simStats.AvgDurationNs,
			Errors:        simStats.Errors,
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
	cardsim.AggregatedStatsStart(builder)
	cardsim.AggregatedStatsAddTotalGames(builder, stats.TotalGames)
	cardsim.AggregatedStatsAddPlayer0Wins(builder, stats.Player0Wins)
	cardsim.AggregatedStatsAddPlayer1Wins(builder, stats.Player1Wins)
	cardsim.AggregatedStatsAddDraws(builder, stats.Draws)
	cardsim.AggregatedStatsAddAvgTurns(builder, stats.AvgTurns)
	cardsim.AggregatedStatsAddMedianTurns(builder, stats.MedianTurns)
	cardsim.AggregatedStatsAddAvgDurationNs(builder, stats.AvgDurationNs)
	cardsim.AggregatedStatsAddErrors(builder, stats.Errors)
	return cardsim.AggregatedStatsEnd(builder)
}

func main() {} // Required for CGo

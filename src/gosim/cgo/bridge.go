package main

/*
#include <stdlib.h>
*/
import "C"
import (
	"unsafe"

	flatbuffers "github.com/google/flatbuffers/go"
	"github.com/signalnine/cards-evolve/gosim/bindings/cardsim"
)

// AggStats holds aggregated simulation results (stub for now)
type AggStats struct {
	TotalGames   uint32
	Player0Wins  uint32
	Player1Wins  uint32
	Draws        uint32
	AvgTurns     float32
	MedianTurns  uint32
	Errors       uint32
}

//export SimulateBatch
func SimulateBatch(requestPtr unsafe.Pointer, requestLen C.int) *C.char {
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

		// TODO: Run actual simulations (Task 8)
		// For now, return stub stats
		stats := &AggStats{
			TotalGames:  req.NumGames(),
			Player0Wins: req.NumGames() / 2,
			Player1Wins: req.NumGames() / 2,
			Draws:       0,
			AvgTurns:    50.0,
			MedianTurns: 50,
			Errors:      0,
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

	// Return as C string (caller must free)
	responseBytes := builder.FinishedBytes()
	return C.CString(string(responseBytes))
}

//export FreeCString
func FreeCString(s *C.char) {
	C.free(unsafe.Pointer(s))
}

func serializeStats(builder *flatbuffers.Builder, stats *AggStats) flatbuffers.UOffsetT {
	cardsim.AggregatedStatsStart(builder)
	cardsim.AggregatedStatsAddTotalGames(builder, stats.TotalGames)
	cardsim.AggregatedStatsAddPlayer0Wins(builder, stats.Player0Wins)
	cardsim.AggregatedStatsAddPlayer1Wins(builder, stats.Player1Wins)
	cardsim.AggregatedStatsAddDraws(builder, stats.Draws)
	cardsim.AggregatedStatsAddAvgTurns(builder, stats.AvgTurns)
	cardsim.AggregatedStatsAddMedianTurns(builder, stats.MedianTurns)
	cardsim.AggregatedStatsAddErrors(builder, stats.Errors)
	return cardsim.AggregatedStatsEnd(builder)
}

func main() {} // Required for CGo

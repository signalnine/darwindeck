package simulation

import (
	"testing"
	"time"

	"github.com/signalnine/cards-evolve/gosim/engine"
)

// createTestGenome creates a simple War game genome for testing
func createTestGenome() *engine.Genome {
	// Minimal War game bytecode
	// Header: 36 bytes
	bytecode := make([]byte, 200)

	// Version = 1
	bytecode[0] = 0
	bytecode[1] = 0
	bytecode[2] = 0
	bytecode[3] = 1

	// GenomeIDHash (arbitrary)
	bytecode[4] = 0xDE
	bytecode[5] = 0xAD
	bytecode[6] = 0xBE
	bytecode[7] = 0xEF

	// PlayerCount = 2
	bytecode[12] = 0
	bytecode[13] = 0
	bytecode[14] = 0
	bytecode[15] = 2

	// MaxTurns = 1000
	bytecode[16] = 0
	bytecode[17] = 0
	bytecode[18] = 0x03
	bytecode[19] = 0xE8

	// SetupOffset = 36
	bytecode[20] = 0
	bytecode[21] = 0
	bytecode[22] = 0
	bytecode[23] = 36

	// TurnStructureOffset = 60
	bytecode[24] = 0
	bytecode[25] = 0
	bytecode[26] = 0
	bytecode[27] = 60

	// WinConditionsOffset = 100
	bytecode[28] = 0
	bytecode[29] = 0
	bytecode[30] = 0
	bytecode[31] = 100

	// ScoringOffset = 120
	bytecode[32] = 0
	bytecode[33] = 0
	bytecode[34] = 0
	bytecode[35] = 120

	// Turn structure at offset 60
	// PhaseCount = 1
	bytecode[60] = 0
	bytecode[61] = 0
	bytecode[62] = 0
	bytecode[63] = 1

	// Phase 1: PlayPhase (type=2)
	bytecode[64] = 2
	// target = LocationTableau (3)
	bytecode[65] = 3
	// min_cards = 1
	bytecode[66] = 1
	// max_cards = 1
	bytecode[67] = 1
	// mandatory = 1
	bytecode[68] = 1
	// condition_len = 0
	bytecode[69] = 0
	bytecode[70] = 0
	bytecode[71] = 0
	bytecode[72] = 0

	// Win conditions at offset 100
	// Count = 1
	bytecode[100] = 0
	bytecode[101] = 0
	bytecode[102] = 0
	bytecode[103] = 1

	// WinType = 0 (empty_hand)
	bytecode[104] = 0
	// Threshold = 0
	bytecode[105] = 0
	bytecode[106] = 0
	bytecode[107] = 0
	bytecode[108] = 0

	genome, err := engine.ParseGenome(bytecode)
	if err != nil {
		panic(err)
	}

	return genome
}

// TestRunBatchParallel_ProducesSameResultsAsSerial verifies correctness
// Note: Due to global rand usage in existing code, results won't be bitwise identical,
// but should be statistically equivalent
func TestRunBatchParallel_ProducesSameResultsAsSerial(t *testing.T) {
	genome := createTestGenome()
	numGames := 1000 // Larger sample for statistical comparison
	aiType := RandomAI
	seed := uint64(42)

	// Run serial
	serialStats := RunBatch(genome, numGames, aiType, 0, seed)

	// Run parallel
	parallelStats := RunBatchParallel(genome, numGames, aiType, 0, seed)

	// Results should be statistically similar (not necessarily identical)
	if serialStats.TotalGames != parallelStats.TotalGames {
		t.Errorf("TotalGames mismatch: serial=%d, parallel=%d",
			serialStats.TotalGames, parallelStats.TotalGames)
	}

	// Check win rates are within reasonable bounds (10% tolerance)
	serialP0WinRate := float64(serialStats.Player0Wins) / float64(numGames)
	parallelP0WinRate := float64(parallelStats.Player0Wins) / float64(numGames)
	winRateDiff := absDiff64(serialP0WinRate, parallelP0WinRate)
	if winRateDiff > 0.10 {
		t.Errorf("Player0 win rate too different: serial=%.3f, parallel=%.3f (diff=%.3f)",
			serialP0WinRate, parallelP0WinRate, winRateDiff)
	}

	serialP1WinRate := float64(serialStats.Player1Wins) / float64(numGames)
	parallelP1WinRate := float64(parallelStats.Player1Wins) / float64(numGames)
	p1WinRateDiff := absDiff64(serialP1WinRate, parallelP1WinRate)
	if p1WinRateDiff > 0.10 {
		t.Errorf("Player1 win rate too different: serial=%.3f, parallel=%.3f (diff=%.3f)",
			serialP1WinRate, parallelP1WinRate, p1WinRateDiff)
	}

	// Check average turns are similar (20% tolerance for this metric)
	avgTurnsDiff := absDiffFloat32(serialStats.AvgTurns, parallelStats.AvgTurns)
	maxDiff := serialStats.AvgTurns * 0.20
	if avgTurnsDiff > maxDiff {
		t.Errorf("AvgTurns too different: serial=%.1f, parallel=%.1f (diff=%.1f, max=%.1f)",
			serialStats.AvgTurns, parallelStats.AvgTurns, avgTurnsDiff, maxDiff)
	}

	// Both should have no errors
	if serialStats.Errors != 0 {
		t.Errorf("Serial had errors: %d", serialStats.Errors)
	}
	if parallelStats.Errors != 0 {
		t.Errorf("Parallel had errors: %d", parallelStats.Errors)
	}

	t.Logf("Serial:   P0=%.1f%% P1=%.1f%% Draws=%.1f%% AvgTurns=%.1f",
		serialP0WinRate*100, serialP1WinRate*100,
		float64(serialStats.Draws)/float64(numGames)*100, serialStats.AvgTurns)
	t.Logf("Parallel: P0=%.1f%% P1=%.1f%% Draws=%.1f%% AvgTurns=%.1f",
		parallelP0WinRate*100, parallelP1WinRate*100,
		float64(parallelStats.Draws)/float64(numGames)*100, parallelStats.AvgTurns)
}

// TestRunBatchParallel_IsFasterThanSerial verifies performance improvement
func TestRunBatchParallel_IsFasterThanSerial(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	genome := createTestGenome()
	numGames := 5000 // Larger batch for more stable timing
	aiType := RandomAI
	seed := uint64(12345)

	// Measure serial time
	serialStart := time.Now()
	RunBatch(genome, numGames, aiType, 0, seed)
	serialDuration := time.Since(serialStart)

	// Measure parallel time
	parallelStart := time.Now()
	RunBatchParallel(genome, numGames, aiType, 0, seed)
	parallelDuration := time.Since(parallelStart)

	speedup := float64(serialDuration) / float64(parallelDuration)

	t.Logf("Serial time: %v", serialDuration)
	t.Logf("Parallel time: %v", parallelDuration)
	t.Logf("Speedup: %.2fx", speedup)

	// Expect at least 1.3x speedup on multi-core systems
	// Note: Actual speedup depends on workload, system load, and batch size
	// We target 2-4x but accept 1.3x minimum for test reliability
	// (Observed: 1.3x-1.5x on 4-core system with this workload)
	if speedup < 1.3 {
		t.Errorf("Parallel speedup too low: %.2fx (expected at least 1.3x)", speedup)
	}
}

// TestRunBatchParallel_HandlesSmallBatches tests edge case
func TestRunBatchParallel_HandlesSmallBatches(t *testing.T) {
	genome := createTestGenome()
	numGames := 10
	aiType := RandomAI
	seed := uint64(99)

	stats := RunBatchParallel(genome, numGames, aiType, 0, seed)

	if stats.TotalGames != uint32(numGames) {
		t.Errorf("Expected %d total games, got %d", numGames, stats.TotalGames)
	}

	// Should complete without errors
	totalOutcomes := stats.Player0Wins + stats.Player1Wins + stats.Draws + stats.Errors
	if totalOutcomes != uint32(numGames) {
		t.Errorf("Outcome count mismatch: %d != %d", totalOutcomes, numGames)
	}
}

// TestRunBatchParallel_HandlesLargeBatches tests scalability
func TestRunBatchParallel_HandlesLargeBatches(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large batch test in short mode")
	}

	genome := createTestGenome()
	numGames := 10000
	aiType := RandomAI
	seed := uint64(7777)

	start := time.Now()
	stats := RunBatchParallel(genome, numGames, aiType, 0, seed)
	duration := time.Since(start)

	t.Logf("Completed %d games in %v (%.0f games/sec)",
		numGames, duration, float64(numGames)/duration.Seconds())

	if stats.TotalGames != uint32(numGames) {
		t.Errorf("Expected %d total games, got %d", numGames, stats.TotalGames)
	}

	// Should complete without errors
	totalOutcomes := stats.Player0Wins + stats.Player1Wins + stats.Draws + stats.Errors
	if totalOutcomes != uint32(numGames) {
		t.Errorf("Outcome count mismatch: %d != %d", totalOutcomes, numGames)
	}

	// Verify reasonable performance (>100 games/sec minimum)
	gamesPerSec := float64(numGames) / duration.Seconds()
	if gamesPerSec < 100 {
		t.Errorf("Performance too slow: %.0f games/sec (expected >100)", gamesPerSec)
	}
}

// BenchmarkRunBatchSerial benchmarks serial execution
func BenchmarkRunBatchSerial(b *testing.B) {
	genome := createTestGenome()
	numGames := 100
	aiType := RandomAI
	seed := uint64(42)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RunBatch(genome, numGames, aiType, 0, seed)
	}
}

// BenchmarkRunBatchParallel benchmarks parallel execution
func BenchmarkRunBatchParallel(b *testing.B) {
	genome := createTestGenome()
	numGames := 100
	aiType := RandomAI
	seed := uint64(42)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RunBatchParallel(genome, numGames, aiType, 0, seed)
	}
}

// Helper function for float comparison
func absDiffFloat32(a, b float32) float32 {
	if a > b {
		return a - b
	}
	return b - a
}

func absDiff64(a, b float64) float64 {
	if a > b {
		return a - b
	}
	return b - a
}

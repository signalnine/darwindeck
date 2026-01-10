package simulation

import (
	"runtime"
	"testing"
)

// ===================================================================
// SERIAL BASELINE BENCHMARKS
// ===================================================================

func BenchmarkSerial_Batch10(b *testing.B) {
	genome := createTestGenome()
	seed := uint64(42)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RunBatch(genome, 10, RandomAI, 0, seed)
	}
}

func BenchmarkSerial_Batch100(b *testing.B) {
	genome := createTestGenome()
	seed := uint64(42)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RunBatch(genome, 100, RandomAI, 0, seed)
	}
}

func BenchmarkSerial_Batch1000(b *testing.B) {
	genome := createTestGenome()
	seed := uint64(42)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RunBatch(genome, 1000, RandomAI, 0, seed)
	}
}

func BenchmarkSerial_Batch10000(b *testing.B) {
	genome := createTestGenome()
	seed := uint64(42)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RunBatch(genome, 10000, RandomAI, 0, seed)
	}
}

// ===================================================================
// PARALLEL BENCHMARKS (MATCHING BATCH SIZES)
// ===================================================================

func BenchmarkParallel_Batch10(b *testing.B) {
	genome := createTestGenome()
	seed := uint64(42)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RunBatchParallel(genome, 10, RandomAI, 0, seed)
	}
}

func BenchmarkParallel_Batch100(b *testing.B) {
	genome := createTestGenome()
	seed := uint64(42)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RunBatchParallel(genome, 100, RandomAI, 0, seed)
	}
}

func BenchmarkParallel_Batch1000(b *testing.B) {
	genome := createTestGenome()
	seed := uint64(42)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RunBatchParallel(genome, 1000, RandomAI, 0, seed)
	}
}

func BenchmarkParallel_Batch10000(b *testing.B) {
	genome := createTestGenome()
	seed := uint64(42)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RunBatchParallel(genome, 10000, RandomAI, 0, seed)
	}
}

// ===================================================================
// AI TYPE COMPARISON BENCHMARKS (BATCH SIZE 1000)
// ===================================================================

func BenchmarkSerial_AIRandom_Batch1000(b *testing.B) {
	genome := createTestGenome()
	seed := uint64(42)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RunBatch(genome, 1000, RandomAI, 0, seed)
	}
}

func BenchmarkParallel_AIRandom_Batch1000(b *testing.B) {
	genome := createTestGenome()
	seed := uint64(42)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RunBatchParallel(genome, 1000, RandomAI, 0, seed)
	}
}

func BenchmarkSerial_AIGreedy_Batch1000(b *testing.B) {
	genome := createTestGenome()
	seed := uint64(42)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RunBatch(genome, 1000, GreedyAI, 0, seed)
	}
}

func BenchmarkParallel_AIGreedy_Batch1000(b *testing.B) {
	genome := createTestGenome()
	seed := uint64(42)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RunBatchParallel(genome, 1000, GreedyAI, 0, seed)
	}
}

// Note: MCTS benchmarks commented out as they may be too slow
// Uncomment to test if MCTS parallelization efficiency differs
/*
func BenchmarkSerial_AIMCTS100_Batch100(b *testing.B) {
	genome := createTestGenome()
	seed := uint64(42)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RunBatch(genome, 100, MCTS100AI, 100, seed)
	}
}

func BenchmarkParallel_AIMCTS100_Batch100(b *testing.B) {
	genome := createTestGenome()
	seed := uint64(42)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RunBatchParallel(genome, 100, MCTS100AI, 100, seed)
	}
}
*/

// ===================================================================
// WORKER COUNT VARIATION BENCHMARKS (BATCH SIZE 1000)
// ===================================================================

func BenchmarkParallel_1Worker_Batch1000(b *testing.B) {
	genome := createTestGenome()
	seed := uint64(42)

	// Save original GOMAXPROCS
	oldMaxProcs := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(oldMaxProcs)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RunBatchParallel(genome, 1000, RandomAI, 0, seed)
	}
}

func BenchmarkParallel_2Workers_Batch1000(b *testing.B) {
	genome := createTestGenome()
	seed := uint64(42)

	// Save original GOMAXPROCS
	oldMaxProcs := runtime.GOMAXPROCS(2)
	defer runtime.GOMAXPROCS(oldMaxProcs)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RunBatchParallel(genome, 1000, RandomAI, 0, seed)
	}
}

func BenchmarkParallel_4Workers_Batch1000(b *testing.B) {
	genome := createTestGenome()
	seed := uint64(42)

	// Save original GOMAXPROCS
	oldMaxProcs := runtime.GOMAXPROCS(4)
	defer runtime.GOMAXPROCS(oldMaxProcs)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RunBatchParallel(genome, 1000, RandomAI, 0, seed)
	}
}

func BenchmarkParallel_8Workers_Batch1000(b *testing.B) {
	genome := createTestGenome()
	seed := uint64(42)

	// Save original GOMAXPROCS
	oldMaxProcs := runtime.GOMAXPROCS(8)
	defer runtime.GOMAXPROCS(oldMaxProcs)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RunBatchParallel(genome, 1000, RandomAI, 0, seed)
	}
}

// ===================================================================
// ADDITIONAL BATCH SIZE GRANULARITY (for detailed analysis)
// ===================================================================

func BenchmarkSerial_Batch50(b *testing.B) {
	genome := createTestGenome()
	seed := uint64(42)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RunBatch(genome, 50, RandomAI, 0, seed)
	}
}

func BenchmarkParallel_Batch50(b *testing.B) {
	genome := createTestGenome()
	seed := uint64(42)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RunBatchParallel(genome, 50, RandomAI, 0, seed)
	}
}

func BenchmarkSerial_Batch500(b *testing.B) {
	genome := createTestGenome()
	seed := uint64(42)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RunBatch(genome, 500, RandomAI, 0, seed)
	}
}

func BenchmarkParallel_Batch500(b *testing.B) {
	genome := createTestGenome()
	seed := uint64(42)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RunBatchParallel(genome, 500, RandomAI, 0, seed)
	}
}

func BenchmarkSerial_Batch5000(b *testing.B) {
	genome := createTestGenome()
	seed := uint64(42)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RunBatch(genome, 5000, RandomAI, 0, seed)
	}
}

func BenchmarkParallel_Batch5000(b *testing.B) {
	genome := createTestGenome()
	seed := uint64(42)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RunBatchParallel(genome, 5000, RandomAI, 0, seed)
	}
}

// ===================================================================
// THROUGHPUT BENCHMARKS (for games/sec measurement)
// ===================================================================

func BenchmarkThroughput_Serial(b *testing.B) {
	genome := createTestGenome()
	seed := uint64(42)

	// Report games per operation
	b.ReportMetric(0, "games/op")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RunBatch(genome, 1000, RandomAI, 0, seed)
	}
	b.StopTimer()

	// Calculate and report throughput
	totalGames := float64(b.N * 1000)
	gamesPerSec := totalGames / b.Elapsed().Seconds()
	b.ReportMetric(gamesPerSec, "games/sec")
	b.ReportMetric(1000, "games/op")
}

func BenchmarkThroughput_Parallel(b *testing.B) {
	genome := createTestGenome()
	seed := uint64(42)

	// Report games per operation
	b.ReportMetric(0, "games/op")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RunBatchParallel(genome, 1000, RandomAI, 0, seed)
	}
	b.StopTimer()

	// Calculate and report throughput
	totalGames := float64(b.N * 1000)
	gamesPerSec := totalGames / b.Elapsed().Seconds()
	b.ReportMetric(gamesPerSec, "games/sec")
	b.ReportMetric(1000, "games/op")
}

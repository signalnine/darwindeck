package simulation

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/signalnine/darwindeck/gosim/engine"
)

func TestRunSingleGameWithGoldenGenome(t *testing.T) {
	// Load golden War genome bytecode
	goldenPath := filepath.Join("..", "..", "..", "tests", "golden", "war_genome.bin")
	bytecode, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("Failed to read golden file: %v", err)
	}

	// Parse genome
	genome, err := engine.ParseGenome(bytecode)
	if err != nil {
		t.Fatalf("Failed to parse genome: %v", err)
	}

	// Run single game
	result := RunSingleGame(genome, RandomAI, 0, 42)

	if result.Error != "" {
		t.Errorf("Game failed: %s", result.Error)
	}

	if result.WinnerID < -1 || result.WinnerID > 1 {
		t.Errorf("Invalid winner ID: %d", result.WinnerID)
	}

	if result.TurnCount == 0 {
		t.Error("Game should have at least one turn")
	}

	t.Logf("Game completed: winner=%d, turns=%d, duration=%dns",
		result.WinnerID, result.TurnCount, result.DurationNs)
}

func TestRunBatchWithGoldenGenome(t *testing.T) {
	// Load golden War genome bytecode
	goldenPath := filepath.Join("..", "..", "..", "tests", "golden", "war_genome.bin")
	bytecode, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("Failed to read golden file: %v", err)
	}

	// Parse genome
	genome, err := engine.ParseGenome(bytecode)
	if err != nil {
		t.Fatalf("Failed to parse genome: %v", err)
	}

	// Run batch
	stats := RunBatch(genome, 10, RandomAI, 0, 12345)

	if stats.TotalGames != 10 {
		t.Errorf("Expected 10 games, got %d", stats.TotalGames)
	}

	if stats.Errors > 0 {
		t.Errorf("Got %d errors", stats.Errors)
	}

	totalWins := stats.Wins[0] + stats.Wins[1] + stats.Draws
	if totalWins != 10 {
		t.Errorf("Wins don't add up: %d+%d+%d = %d",
			stats.Wins[0], stats.Wins[1], stats.Draws, totalWins)
	}

	t.Logf("Batch results: P0=%d P1=%d Draws=%d, Avg turns=%.1f",
		stats.Wins[0], stats.Wins[1], stats.Draws, stats.AvgTurns)
}

func BenchmarkRunSingleGame(b *testing.B) {
	goldenPath := filepath.Join("..", "..", "..", "tests", "golden", "war_genome.bin")
	bytecode, err := os.ReadFile(goldenPath)
	if err != nil {
		b.Fatalf("Failed to read golden file: %v", err)
	}

	genome, err := engine.ParseGenome(bytecode)
	if err != nil {
		b.Fatalf("Failed to parse genome: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RunSingleGame(genome, RandomAI, 0, uint64(i))
	}
}

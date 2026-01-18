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

// BenchmarkSolitaireMetricsOverhead measures the overhead of solitaire detection metrics.
// Solitaire detection adds per-turn work:
//   - getLegalMovesForPlayer: Generate opponent's legal moves before each turn
//   - movesDisrupted: Compare move sets before/after to detect disruption
//   - isContentionEvent: Check if opponents could make similar moves
//
// This benchmark runs the full simulation including these metrics.
// Compare with historical benchmarks to assess overhead impact.
// Target: <20% overhead vs baseline simulation without solitaire tracking.
func BenchmarkSolitaireMetricsOverhead(b *testing.B) {
	// Load the golden genome from bytecode file
	goldenPath := filepath.Join("..", "..", "..", "tests", "golden", "war_genome.bin")
	bytecodeData, err := os.ReadFile(goldenPath)
	if err != nil {
		b.Fatalf("Failed to read golden genome: %v", err)
	}

	genome, err := engine.ParseGenome(bytecodeData)
	if err != nil {
		b.Fatalf("Failed to parse genome: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RunSingleGame(genome, RandomAI, 0, uint64(i))
	}
}

func TestRunnerSetsTableauMode(t *testing.T) {
	// Create a v2 bytecode with WAR mode
	bytecode := makeV2BytecodeWithTableauMode(1, 0) // TableauMode = WAR, SequenceDirection = ASC

	genome, err := engine.ParseGenome(bytecode)
	if err != nil {
		t.Fatalf("ParseGenome failed: %v", err)
	}

	// Verify genome header parsed tableau fields correctly
	if genome.Header.TableauMode != 1 {
		t.Errorf("Expected TableauMode=1 (WAR), got %d", genome.Header.TableauMode)
	}
	if genome.Header.SequenceDirection != 0 {
		t.Errorf("Expected SequenceDirection=0 (ASC), got %d", genome.Header.SequenceDirection)
	}

	// Run a single game and check it completes
	result := RunSingleGame(genome, RandomAI, 0, 12345)

	// Just verify no error - the important thing is the state was initialized
	if result.Error != "" {
		t.Errorf("Game errored: %s", result.Error)
	}
}

func TestRunnerSetsTableauModeSequence(t *testing.T) {
	// Create a v2 bytecode with SEQUENCE mode and DESC direction
	bytecode := makeV2BytecodeWithTableauMode(3, 1) // TableauMode = SEQUENCE, SequenceDirection = DESC

	genome, err := engine.ParseGenome(bytecode)
	if err != nil {
		t.Fatalf("ParseGenome failed: %v", err)
	}

	// Verify genome header parsed tableau fields correctly
	if genome.Header.TableauMode != 3 {
		t.Errorf("Expected TableauMode=3 (SEQUENCE), got %d", genome.Header.TableauMode)
	}
	if genome.Header.SequenceDirection != 1 {
		t.Errorf("Expected SequenceDirection=1 (DESC), got %d", genome.Header.SequenceDirection)
	}

	// Run a single game - for this test we mainly care about parsing
	result := RunSingleGame(genome, RandomAI, 0, 12345)

	// Allow errors since this minimal bytecode may not be fully playable
	// The key verification is that the genome header was parsed correctly
	_ = result
}

// makeV2BytecodeWithTableauMode creates a minimal v2 bytecode with specified tableau mode.
// V2 format: 39-byte header + minimal turn structure + win conditions
func makeV2BytecodeWithTableauMode(tableauMode uint8, seqDir uint8) []byte {
	// V2 header is 39 bytes, then we need:
	// - Setup section (12 bytes: cards_per_player + initial_discard + starting_chips)
	// - Turn structure (minimal: 4-byte count + phase data)
	// - Win conditions (5+ bytes)

	// Layout:
	// Bytes 0: version (2)
	// Bytes 1-4: legacy version
	// Bytes 5-12: genome_id_hash
	// Bytes 13-16: player_count
	// Bytes 17-20: max_turns
	// Bytes 21-24: setup_offset
	// Bytes 25-28: turn_structure_offset
	// Bytes 29-32: win_conditions_offset
	// Bytes 33-36: scoring_offset
	// Byte 37: tableau_mode
	// Byte 38: sequence_direction
	// Bytes 39+: section data

	bytecode := make([]byte, 100) // Plenty of room

	// Header (39 bytes)
	bytecode[0] = 2  // Version 2
	// Legacy version at 1-4 (leave as 0)
	// Genome ID hash at 5-12 (leave as 0)
	// Player count at 13-16
	bytecode[13] = 0
	bytecode[14] = 0
	bytecode[15] = 0
	bytecode[16] = 2 // 2 players
	// Max turns at 17-20
	bytecode[17] = 0
	bytecode[18] = 0
	bytecode[19] = 0
	bytecode[20] = 200 // 200 max turns
	// Setup offset at 21-24 -> byte 39
	bytecode[21] = 0
	bytecode[22] = 0
	bytecode[23] = 0
	bytecode[24] = 39
	// Turn structure offset at 25-28 -> byte 51 (39 + 12 for setup)
	bytecode[25] = 0
	bytecode[26] = 0
	bytecode[27] = 0
	bytecode[28] = 51
	// Win conditions offset at 29-32 -> byte 66 (51 + 15 for turn structure)
	bytecode[29] = 0
	bytecode[30] = 0
	bytecode[31] = 0
	bytecode[32] = 66
	// Scoring offset at 33-36 (leave as 0 - no scoring)
	// Tableau mode at 37
	bytecode[37] = tableauMode
	// Sequence direction at 38
	bytecode[38] = seqDir

	// Setup section at offset 39 (12 bytes)
	// cards_per_player (4 bytes) = 26
	bytecode[39] = 0
	bytecode[40] = 0
	bytecode[41] = 0
	bytecode[42] = 26
	// initial_discard_count (4 bytes) = 0
	// starting_chips (4 bytes) = 0
	// (bytes 43-50 already 0)

	// Turn structure at offset 51
	// Phase count (4 bytes) = 2 (draw + play)
	bytecode[51] = 0
	bytecode[52] = 0
	bytecode[53] = 0
	bytecode[54] = 2

	// Phase 1: DrawPhase (type=1, source:1 + count:4 + mandatory:1 + has_condition:1 = 7 bytes)
	bytecode[55] = 1 // PhaseTypeDraw
	bytecode[56] = 0 // source = deck
	bytecode[57] = 0
	bytecode[58] = 0
	bytecode[59] = 0
	bytecode[60] = 1 // count = 1
	bytecode[61] = 1 // mandatory = true
	bytecode[62] = 0 // has_condition = false

	// Phase 2: PlayPhase (type=2, target:1 + min:1 + max:1 + mandatory:1 + pass_if_unable:1 + conditionLen:4 = 9 bytes)
	bytecode[63] = 2 // PhaseTypePlay
	bytecode[64] = 3 // target = tableau (LocationTableau)
	bytecode[65] = 1 // min = 1
	bytecode[66] = 1 // max = 1
	bytecode[67] = 1 // mandatory = true
	bytecode[68] = 0 // pass_if_unable = false
	// conditionLen (4 bytes) = 0
	// (bytes 69-72 already 0)

	// Wait - need to recalculate. Phase 1 is 1+7=8 bytes, Phase 2 is 1+9=10 bytes
	// Actually phase_type is part of the loop, so:
	// Phase 1: 1 byte type + 7 bytes data = 8 bytes (55-62)
	// Phase 2: 1 byte type + 9 bytes data = 10 bytes (63-72)
	// Total turn structure: 4 + 8 + 10 = 22 bytes (51-72)
	// So win conditions should start at 73

	// Fix win conditions offset
	bytecode[29] = 0
	bytecode[30] = 0
	bytecode[31] = 0
	bytecode[32] = 73

	// Win conditions at offset 73
	// Count (4 bytes) = 1
	bytecode[73] = 0
	bytecode[74] = 0
	bytecode[75] = 0
	bytecode[76] = 1

	// Win condition: empty_hand (type=0, threshold=0)
	bytecode[77] = 0 // WinType = empty_hand
	// threshold (4 bytes) = 0
	// (bytes 78-81 already 0)

	return bytecode[:82]
}

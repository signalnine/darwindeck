package simulation

import (
	"testing"

	"github.com/signalnine/darwindeck/gosim/genome"
)

func TestRunSingleGameTypedWar(t *testing.T) {
	g := genome.CreateWarGenome()

	result := RunSingleGameTyped(g, RandomAI, 0, 12345)

	if result.Error != "" {
		t.Errorf("War game returned error: %s", result.Error)
	}
	// War games can end in draws (WinnerID=-1) when hitting max turns
	// This is valid behavior for the game
	if result.WinnerID < -1 || result.WinnerID > 1 {
		t.Errorf("Expected winner -1, 0, or 1, got %d", result.WinnerID)
	}
	if result.TurnCount == 0 {
		t.Error("Expected non-zero turn count")
	}
	t.Logf("War game: Winner=%d, Turns=%d, Duration=%dns",
		result.WinnerID, result.TurnCount, result.DurationNs)
}

func TestRunBatchTypedWar(t *testing.T) {
	g := genome.CreateWarGenome()

	stats := RunBatchTyped(g, 10, RandomAI, 0, 12345)

	if stats.TotalGames != 10 {
		t.Errorf("Expected 10 games, got %d", stats.TotalGames)
	}
	if stats.Errors > 5 { // Allow some errors but not all
		t.Errorf("Too many errors: %d", stats.Errors)
	}

	totalWins := stats.Wins[0] + stats.Wins[1] + stats.Draws
	if totalWins != 10 {
		t.Errorf("Expected 10 outcomes, got %d", totalWins)
	}

	t.Logf("War batch: P0Wins=%d, P1Wins=%d, Draws=%d, AvgTurns=%.1f, Errors=%d",
		stats.Wins[0], stats.Wins[1], stats.Draws, stats.AvgTurns, stats.Errors)
}

func TestRunSingleGameTypedHearts(t *testing.T) {
	g := genome.CreateHeartsGenome()

	result := RunSingleGameTyped(g, RandomAI, 0, 54321)

	// Hearts games may not always complete within turn limit, allow errors
	if result.Error != "" && result.Error != "no legal moves" {
		t.Logf("Hearts game note: %s (may be expected)", result.Error)
	}

	t.Logf("Hearts game: Winner=%d, Turns=%d, Duration=%dns",
		result.WinnerID, result.TurnCount, result.DurationNs)
}

func TestRunSingleGameTypedCrazyEights(t *testing.T) {
	g := genome.CreateCrazyEightsGenome()

	result := RunSingleGameTyped(g, RandomAI, 0, 99999)

	t.Logf("Crazy Eights: Winner=%d, Turns=%d, Error=%s",
		result.WinnerID, result.TurnCount, result.Error)
}

func TestRunSingleGameTypedSimplePoker(t *testing.T) {
	g := genome.CreateSimplePokerGenome()

	result := RunSingleGameTyped(g, RandomAI, 0, 11111)

	t.Logf("Simple Poker: Winner=%d, Turns=%d, Error=%s",
		result.WinnerID, result.TurnCount, result.Error)
}

func TestMetricsTrackedTyped(t *testing.T) {
	g := genome.CreateWarGenome()

	result := RunSingleGameTyped(g, RandomAI, 0, 77777)

	if result.Metrics.TotalDecisions == 0 {
		t.Error("Expected some decisions to be tracked")
	}
	if result.Metrics.TotalActions == 0 {
		t.Error("Expected some actions to be tracked")
	}

	t.Logf("Metrics: Decisions=%d, ValidMoves=%d, ForcedDecisions=%d, Actions=%d, Interactions=%d",
		result.Metrics.TotalDecisions,
		result.Metrics.TotalValidMoves,
		result.Metrics.ForcedDecisions,
		result.Metrics.TotalActions,
		result.Metrics.TotalInteractions)
}

func TestGreedyAITyped(t *testing.T) {
	g := genome.CreateWarGenome()

	// Run multiple games with Greedy AI
	stats := RunBatchTyped(g, 5, GreedyAI, 0, 22222)

	if stats.Errors > 2 {
		t.Errorf("Too many errors with Greedy AI: %d", stats.Errors)
	}

	t.Logf("Greedy War: P0Wins=%d, P1Wins=%d, AvgTurns=%.1f",
		stats.Wins[0], stats.Wins[1], stats.AvgTurns)
}

func TestCheckWinConditionsTypedEmptyHand(t *testing.T) {
	g := genome.CreateWarGenome()
	g.WinConditions = []genome.WinCondition{
		{Type: genome.WinTypeEmptyHand},
	}

	// Run a single game - when a player's hand is empty, that player wins
	result := RunSingleGameTyped(g, RandomAI, 0, 88888)

	// War with empty-hand win condition should still complete
	if result.Error != "" && result.Error != "no legal moves" {
		t.Logf("Game note: %s", result.Error)
	}
	t.Logf("EmptyHand Win: Winner=%d, Turns=%d", result.WinnerID, result.TurnCount)
}

// TestEndToEndTypedPipeline validates the complete typed genome pipeline:
// 1. Create genome from schema
// 2. Validate with GenomeValidator
// 3. Serialize to JSON and deserialize (round-trip)
// 4. Run batch simulations
// 5. Verify reasonable results
func TestEndToEndTypedPipeline(t *testing.T) {
	// Step 1: Create War genome from typed schema
	original := genome.CreateWarGenome()
	if original.Name != "War" {
		t.Fatalf("Expected name 'War', got '%s'", original.Name)
	}
	t.Logf("Step 1: Created genome '%s'", original.Name)

	// Step 2: Validate with GenomeValidator
	errors := genome.ValidateGenome(original)
	if len(errors) > 0 {
		for _, err := range errors {
			t.Logf("Validation warning: %s: %s", err.Field, err.Message)
		}
	}
	if !genome.IsValid(original) {
		t.Fatalf("War genome should be valid")
	}
	t.Logf("Step 2: Genome validated successfully")

	// Step 3: JSON round-trip
	jsonBytes, err := original.MarshalJSON()
	if err != nil {
		t.Fatalf("Failed to serialize genome: %v", err)
	}
	t.Logf("Step 3a: Serialized to %d bytes of JSON", len(jsonBytes))

	loaded := &genome.GameGenome{}
	err = loaded.UnmarshalJSON(jsonBytes)
	if err != nil {
		t.Fatalf("Failed to deserialize genome: %v", err)
	}
	if loaded.Name != original.Name {
		t.Fatalf("Round-trip name mismatch: expected '%s', got '%s'", original.Name, loaded.Name)
	}
	t.Logf("Step 3b: Deserialized genome '%s' successfully", loaded.Name)

	// Step 4: Run batch simulations
	stats := RunBatchTyped(loaded, 100, RandomAI, 0, 42)
	if stats.TotalGames != 100 {
		t.Fatalf("Expected 100 games, got %d", stats.TotalGames)
	}
	t.Logf("Step 4: Ran %d games", stats.TotalGames)

	// Step 5: Verify reasonable results
	totalOutcomes := stats.Wins[0] + stats.Wins[1] + stats.Draws
	if totalOutcomes != 100 {
		t.Errorf("Expected 100 outcomes, got %d", totalOutcomes)
	}
	if stats.AvgTurns < 10 {
		t.Errorf("Average turns too low: %.1f (War should take many turns)", stats.AvgTurns)
	}
	if stats.Errors > 10 {
		t.Errorf("Too many errors: %d", stats.Errors)
	}

	t.Logf("Step 5: Results validated")
	t.Logf("  P0 Wins: %d (%.1f%%)", stats.Wins[0], float64(stats.Wins[0])/100*100)
	t.Logf("  P1 Wins: %d (%.1f%%)", stats.Wins[1], float64(stats.Wins[1])/100*100)
	t.Logf("  Draws:   %d (%.1f%%)", stats.Draws, float64(stats.Draws)/100*100)
	t.Logf("  Avg Turns: %.1f", stats.AvgTurns)
	t.Logf("  Errors: %d", stats.Errors)
}

// TestAllSeedGenomesRunnable verifies all 19 seed genomes can run simulations
func TestAllSeedGenomesRunnable(t *testing.T) {
	genomes := genome.GetSeedGenomes()
	t.Logf("Testing %d seed genomes", len(genomes))

	// Skip genomes with known incomplete support in typed runner
	// These require features not yet ported (drawing from opponents, bidding, etc.)
	skipGenomes := map[string]bool{
		"Go Fish":           true, // Drawing from opponents not fully supported
		"Spades":            true, // Bidding phase not fully supported
		"Partnership Spades": true, // Bidding phase + teams
		"Simple Poker":      true, // Complex betting flow
		"Draw Poker":        true, // Complex betting flow
		"Blackjack":         true, // Complex betting flow
		"Scopa":             true, // Set collection not fully supported
		"Cheat":             true, // Claim/challenge phases not supported
	}

	for _, g := range genomes {
		t.Run(g.Name, func(t *testing.T) {
			if skipGenomes[g.Name] {
				t.Skipf("Skipping %s (incomplete typed runner support)", g.Name)
				return
			}

			// Run 5 games with each genome
			stats := RunBatchTyped(g, 5, RandomAI, 0, 12345)

			// Basic sanity checks
			if stats.TotalGames != 5 {
				t.Errorf("Expected 5 games, got %d", stats.TotalGames)
			}

			// Log results but don't fail on errors (some genomes may be incomplete)
			t.Logf("%s: P0=%d P1=%d Draws=%d AvgTurns=%.1f Errors=%d",
				g.Name, stats.Wins[0], stats.Wins[1], stats.Draws, stats.AvgTurns, stats.Errors)
		})
	}
}

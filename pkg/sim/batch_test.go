package sim

import (
	"math/rand/v2"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
)

// stubRunner satisfies GenericRunner but is never invoked when n==0.
type stubRunner struct{}

func (stubRunner) Setup(g *genome.Genome, rng *rand.Rand) *GameState         { return nil }
func (stubRunner) GenerateMoves(state *GameState, g *genome.Genome) []Move   { return nil }
func (stubRunner) ApplyMove(state *GameState, move Move, g *genome.Genome) []Event {
	return nil
}
func (stubRunner) CheckEnd(state *GameState, g *genome.Genome) int { return -1 }

// TestRunBatchEmptyDoesNotLeakSentinelMinTurns guards against dd-d80:
// when n==0 the loop never runs, so MinTurns must not surface as the
// initialization sentinel (~2.1B) to downstream metrics.
func TestRunBatchEmptyDoesNotLeakSentinelMinTurns(t *testing.T) {
	g := &genome.Genome{Players: 2, HandSize: 5, Skeleton: genome.Shedding}
	result := RunBatch(g, stubRunner{}, &RandomAI{}, 0, 1)

	if result.GamesPlayed != 0 {
		t.Fatalf("GamesPlayed = %d, want 0", result.GamesPlayed)
	}
	if result.MinTurns != 0 {
		t.Errorf("MinTurns = %d on empty batch, want 0 (sentinel leaked to caller)", result.MinTurns)
	}
	if result.MaxTurns != 0 {
		t.Errorf("MaxTurns = %d on empty batch, want 0", result.MaxTurns)
	}
}

package evolution

import (
	"math"

	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

// BehaviorDescriptor is a 2D point in behavior space.
// X = decision density, Y = interaction.
type BehaviorDescriptor [2]float64

// ComputeBehavior extracts the behavior descriptor from simulation results.
// X-axis: decision density (fraction of turns where the acting player had
// >= 2 legal moves). Y-axis: interaction (fraction of turns that perturbed
// the next player's options or carried a direct attack).
//
// These replace the original axes (normalized AvgTurns x win entropy): win
// entropy under random play is ~1.0 for almost any non-broken game, so 91%
// of mutants landed in the top grid row and only 16/100 MAP-Elites cells
// were reachable (audit Task 17). Decision density and interaction are real
// per-genome variables with measured within-skeleton spread (post Tasks 9/11),
// enforced by TestDescriptorSpread.
//
// Both axes are computed by the canonical pkg/fitness implementations via
// the exported ComputeFitness -- a single source of truth, so the descriptor
// can never drift from the fitness metrics. The empty greedy batch and zero
// player count zero out the metrics this descriptor does not read (skill
// gradient, session length); decision density and interaction depend only on
// the random batch's TurnRecords.
func ComputeBehavior(result sim.BatchResult) BehaviorDescriptor {
	m := fitness.ComputeFitness(result, sim.BatchResult{}, 0)
	return BehaviorDescriptor{m.MeaningfulDecisions, m.Interaction}
}

// GridCell returns the (row, col) cell indices for a given grid size.
func (b BehaviorDescriptor) GridCell(gridSize int) (int, int) {
	col := int(b[0] * float64(gridSize))
	row := int(b[1] * float64(gridSize))
	if col >= gridSize {
		col = gridSize - 1
	}
	if row >= gridSize {
		row = gridSize - 1
	}
	if col < 0 {
		col = 0
	}
	if row < 0 {
		row = 0
	}
	return row, col
}

// Distance returns Euclidean distance between two behavior descriptors.
func (b BehaviorDescriptor) Distance(other BehaviorDescriptor) float64 {
	dx := b[0] - other[0]
	dy := b[1] - other[1]
	return math.Sqrt(dx*dx + dy*dy)
}

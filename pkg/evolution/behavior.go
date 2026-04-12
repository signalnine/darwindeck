package evolution

import (
	"math"

	"github.com/darwindeck/darwindeck/pkg/sim"
)

// BehaviorDescriptor is a 2D point in behavior space.
// X = normalized average turns, Y = win entropy.
type BehaviorDescriptor [2]float64

// ComputeBehavior extracts the behavior descriptor from simulation results.
// X-axis: AvgTurns normalized to [0, 1] over range [5, 100].
// Y-axis: WinEntropy (Shannon entropy of win distribution, normalized by max entropy).
func ComputeBehavior(result sim.BatchResult) BehaviorDescriptor {
	// X: normalize AvgTurns to [0, 1]
	x := (result.AvgTurns - 5.0) / 95.0 // maps [5, 100] -> [0, 1]
	x = clamp(x, 0, 1)

	// Y: win entropy
	y := computeWinEntropy(result)

	return BehaviorDescriptor{x, y}
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

func computeWinEntropy(result sim.BatchResult) float64 {
	numPlayers := len(result.WinCounts)
	if numPlayers <= 1 {
		return 0
	}

	totalWins := 0
	for _, w := range result.WinCounts {
		totalWins += w
	}
	if totalWins == 0 {
		return 0
	}

	maxEntropy := math.Log2(float64(numPlayers))
	if maxEntropy == 0 {
		return 0
	}

	entropy := 0.0
	for _, w := range result.WinCounts {
		if w == 0 {
			continue
		}
		p := float64(w) / float64(totalWins)
		entropy -= p * math.Log2(p)
	}

	return entropy / maxEntropy
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

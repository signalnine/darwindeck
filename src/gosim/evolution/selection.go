package evolution

import (
	"math/rand"
	"sort"
)

// TournamentSelection selects an individual via tournament selection.
// k is the tournament size (number of candidates to sample).
func TournamentSelection(pop *Population, k int, rng *rand.Rand) *Individual {
	if pop == nil || len(pop.Individuals) == 0 {
		return nil
	}

	if k > len(pop.Individuals) {
		k = len(pop.Individuals)
	}
	if k < 1 {
		k = 1
	}

	// Sample k individuals
	indices := rng.Perm(len(pop.Individuals))[:k]
	candidates := make([]*Individual, k)
	for i, idx := range indices {
		candidates[i] = pop.Individuals[idx]
	}

	// Return best
	best := candidates[0]
	for _, ind := range candidates[1:] {
		if ind.Fitness > best.Fitness {
			best = ind
		}
	}
	return best
}

// SelectElite returns the top n individuals by fitness.
func SelectElite(pop *Population, n int) []*Individual {
	if pop == nil || len(pop.Individuals) == 0 {
		return nil
	}

	if n > len(pop.Individuals) {
		n = len(pop.Individuals)
	}
	if n < 1 {
		return nil
	}

	// Sort by fitness (descending)
	sorted := make([]*Individual, len(pop.Individuals))
	copy(sorted, pop.Individuals)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Fitness > sorted[j].Fitness
	})

	return sorted[:n]
}

// SelectEliteByRate returns the top percentage of individuals.
// elitismRate should be in range [0.0, 1.0].
func SelectEliteByRate(pop *Population, elitismRate float64) []*Individual {
	if pop == nil || len(pop.Individuals) == 0 {
		return nil
	}

	n := int(float64(len(pop.Individuals)) * elitismRate)
	if n < 1 {
		n = 1
	}
	return SelectElite(pop, n)
}

// RouletteWheelSelection selects an individual using fitness-proportionate selection.
// Higher fitness = higher probability of selection.
func RouletteWheelSelection(pop *Population, rng *rand.Rand) *Individual {
	if pop == nil || len(pop.Individuals) == 0 {
		return nil
	}

	// Calculate total fitness
	var totalFitness float64
	for _, ind := range pop.Individuals {
		if ind.Fitness > 0 {
			totalFitness += ind.Fitness
		}
	}

	// If all fitnesses are zero or negative, use uniform selection
	if totalFitness <= 0 {
		return pop.Individuals[rng.Intn(len(pop.Individuals))]
	}

	// Spin the wheel
	spin := rng.Float64() * totalFitness
	var cumulative float64
	for _, ind := range pop.Individuals {
		if ind.Fitness > 0 {
			cumulative += ind.Fitness
			if cumulative >= spin {
				return ind
			}
		}
	}

	// Fallback (shouldn't happen with proper math)
	return pop.Individuals[len(pop.Individuals)-1]
}

// RankSelection selects an individual using rank-based selection.
// Better individuals have higher probability but not proportional to fitness.
// This reduces selection pressure compared to fitness-proportionate.
func RankSelection(pop *Population, rng *rand.Rand) *Individual {
	if pop == nil || len(pop.Individuals) == 0 {
		return nil
	}

	n := len(pop.Individuals)

	// Sort by fitness (ascending - worst first)
	sorted := make([]*Individual, n)
	copy(sorted, pop.Individuals)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Fitness < sorted[j].Fitness
	})

	// Assign ranks: worst=1, best=n
	// Total rank sum = n*(n+1)/2
	totalRank := float64(n * (n + 1) / 2)

	// Spin based on rank
	spin := rng.Float64() * totalRank
	var cumulative float64
	for rank, ind := range sorted {
		cumulative += float64(rank + 1)
		if cumulative >= spin {
			return ind
		}
	}

	return sorted[n-1]
}

// TruncationSelection returns the top percentage of the population.
// truncationRate should be in range (0.0, 1.0].
func TruncationSelection(pop *Population, truncationRate float64) []*Individual {
	if pop == nil || len(pop.Individuals) == 0 {
		return nil
	}

	n := int(float64(len(pop.Individuals)) * truncationRate)
	if n < 1 {
		n = 1
	}
	return SelectElite(pop, n)
}

// SelectDiverse selects n individuals maximizing diversity.
// Uses greedy selection starting from the best individual.
func SelectDiverse(pop *Population, n int) []*Individual {
	if pop == nil || len(pop.Individuals) == 0 {
		return nil
	}

	if n >= len(pop.Individuals) {
		return pop.Individuals
	}
	if n < 1 {
		return nil
	}

	// Start with best individual
	sorted := SelectElite(pop, len(pop.Individuals))
	selected := []*Individual{sorted[0]}
	remaining := sorted[1:]

	// Greedily add individuals that maximize minimum distance to selected set
	for len(selected) < n && len(remaining) > 0 {
		bestIdx := 0
		bestMinDist := -1.0

		for i, candidate := range remaining {
			// Calculate minimum distance to already selected individuals
			minDist := 1.0
			for _, sel := range selected {
				dist := GenomeDistance(candidate.Genome, sel.Genome)
				if dist < minDist {
					minDist = dist
				}
			}

			// Keep track of candidate with highest minimum distance
			if minDist > bestMinDist {
				bestMinDist = minDist
				bestIdx = i
			}
		}

		selected = append(selected, remaining[bestIdx])
		remaining = append(remaining[:bestIdx], remaining[bestIdx+1:]...)
	}

	return selected
}

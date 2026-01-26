package evolution

import (
	"math"
	"math/rand"

	"github.com/signalnine/darwindeck/gosim/evolution/fitness"
	"github.com/signalnine/darwindeck/gosim/genome"
)

// DiversityThreshold is the threshold below which diversity is considered critical.
const DiversityThreshold = 0.1

// Individual represents a single genome with its fitness score.
type Individual struct {
	Genome         *genome.GameGenome
	Fitness        float64
	Evaluated      bool
	FitnessMetrics *fitness.FitnessMetrics // Full metrics breakdown
}

// Clone creates a deep copy of the individual.
func (ind *Individual) Clone() *Individual {
	clone := &Individual{
		Genome:    ind.Genome.Clone(),
		Fitness:   ind.Fitness,
		Evaluated: ind.Evaluated,
	}
	if ind.FitnessMetrics != nil {
		metricsCopy := *ind.FitnessMetrics
		clone.FitnessMetrics = &metricsCopy
	}
	return clone
}

// Population represents a collection of individuals.
type Population struct {
	Individuals []*Individual
	Generation  int
}

// NewPopulation creates a new population from a list of individuals.
func NewPopulation(individuals []*Individual) *Population {
	return &Population{
		Individuals: individuals,
		Generation:  0,
	}
}

// Size returns the number of individuals in the population.
func (p *Population) Size() int {
	return len(p.Individuals)
}

// GetBestIndividual returns the individual with the highest fitness.
func (p *Population) GetBestIndividual() *Individual {
	if len(p.Individuals) == 0 {
		return nil
	}

	best := p.Individuals[0]
	for _, ind := range p.Individuals[1:] {
		if ind.Fitness > best.Fitness {
			best = ind
		}
	}
	return best
}

// GetAverageFitness returns the average fitness of evaluated individuals.
func (p *Population) GetAverageFitness() float64 {
	if len(p.Individuals) == 0 {
		return 0.0
	}

	var sum float64
	var count int
	for _, ind := range p.Individuals {
		if ind.Evaluated {
			sum += ind.Fitness
			count++
		}
	}

	if count == 0 {
		return 0.0
	}
	return sum / float64(count)
}

// ComputeDiversity calculates population diversity using pairwise distances.
// Higher = more diverse, Lower = converged.
// Returns diversity score in range [0.0, 1.0].
func (p *Population) ComputeDiversity() float64 {
	if len(p.Individuals) < 2 {
		return 0.0
	}

	var totalDistance float64
	var pairCount int

	if len(p.Individuals) <= 50 {
		// Small population: check all pairs
		for i := 0; i < len(p.Individuals); i++ {
			for j := i + 1; j < len(p.Individuals); j++ {
				totalDistance += GenomeDistance(
					p.Individuals[i].Genome,
					p.Individuals[j].Genome,
				)
				pairCount++
			}
		}
	} else {
		// Large population: sample 100 random pairs
		for k := 0; k < 100; k++ {
			i := rand.Intn(len(p.Individuals))
			j := rand.Intn(len(p.Individuals))
			if i == j {
				j = (i + 1) % len(p.Individuals)
			}
			totalDistance += GenomeDistance(
				p.Individuals[i].Genome,
				p.Individuals[j].Genome,
			)
			pairCount++
		}
	}

	if pairCount == 0 {
		return 0.0
	}

	return totalDistance / float64(pairCount)
}

// CheckDiversityCrisis returns true if diversity has collapsed.
func (p *Population) CheckDiversityCrisis() bool {
	return p.ComputeDiversity() < DiversityThreshold
}

// GetUnevaluated returns all individuals that haven't been evaluated.
func (p *Population) GetUnevaluated() []*Individual {
	var unevaluated []*Individual
	for _, ind := range p.Individuals {
		if !ind.Evaluated {
			unevaluated = append(unevaluated, ind)
		}
	}
	return unevaluated
}

// SortByFitness returns individuals sorted by fitness (descending).
func (p *Population) SortByFitness() []*Individual {
	sorted := make([]*Individual, len(p.Individuals))
	copy(sorted, p.Individuals)

	// Simple insertion sort (stable, works well for partially sorted data)
	for i := 1; i < len(sorted); i++ {
		j := i
		for j > 0 && sorted[j-1].Fitness < sorted[j].Fitness {
			sorted[j-1], sorted[j] = sorted[j], sorted[j-1]
			j--
		}
	}
	return sorted
}

// GenomeDistance computes distance between two genomes (0.0 = identical, 1.0 = maximally different).
// Uses Hamming distance on key structural features.
func GenomeDistance(g1, g2 *genome.GameGenome) float64 {
	var distance float64
	var totalFeatures float64

	// 1. Turn structure phase count
	phaseDiff := math.Abs(float64(len(g1.TurnStructure.Phases) - len(g2.TurnStructure.Phases)))
	distance += math.Min(1.0, phaseDiff/5.0) // Normalize by max expected diff
	totalFeatures++

	// 2. Special effects count
	effectDiff := math.Abs(float64(len(g1.Effects) - len(g2.Effects)))
	distance += math.Min(1.0, effectDiff/3.0)
	totalFeatures++

	// 3. Win conditions count
	winDiff := math.Abs(float64(len(g1.WinConditions) - len(g2.WinConditions)))
	distance += math.Min(1.0, winDiff/2.0)
	totalFeatures++

	// 4. Max turns (normalized)
	turnsDiff := math.Abs(float64(g1.TurnStructure.MaxTurns-g2.TurnStructure.MaxTurns)) / 1000.0
	distance += math.Min(1.0, turnsDiff)
	totalFeatures++

	// 5. Setup differences (cards per player)
	cardDiff := math.Abs(float64(g1.Setup.CardsPerPlayer - g2.Setup.CardsPerPlayer))
	distance += math.Min(1.0, cardDiff/26.0)
	totalFeatures++

	return distance / totalFeatures
}

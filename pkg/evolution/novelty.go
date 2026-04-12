package evolution

import (
	"fmt"
	"math/rand/v2"
	"sort"
	"sync"

	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

const (
	NoveltyK         = 15   // k-nearest neighbors for novelty
	NoveltyWeight    = 0.5  // weight of novelty vs fitness
	NoveltyThreshold = 0.3  // min novelty to enter archive
	FitnessFloor     = 0.70 // minimum fitness to be considered
)

// NoveltyIndividual extends Individual with behavior and novelty score.
type NoveltyIndividual struct {
	Individual
	Behavior BehaviorDescriptor
	Novelty  float64
}

// NoveltyEngine runs novelty search with a fitness floor.
type NoveltyEngine struct {
	Config     Config
	Seeds      []*genome.Genome
	Population []*NoveltyIndividual
	Archive    []*NoveltyIndividual // Novelty archive (memory of exploration)
	Generation int
	rng        *rand.Rand

	// Stats
	BestFitness float64
	BestGenome  *genome.Genome
}

// NewNoveltyEngine creates a novelty search engine.
func NewNoveltyEngine(config Config, seeds []*genome.Genome) *NoveltyEngine {
	return &NoveltyEngine{
		Config: config,
		Seeds:  seeds,
		rng:    rand.New(rand.NewPCG(config.BaseSeed, 0)),
	}
}

// Run executes the novelty search loop.
func (e *NoveltyEngine) Run(progress func(gen int, best float64, avg float64)) {
	e.initialize()

	for gen := 0; gen < e.Config.Generations; gen++ {
		e.Generation = gen
		e.evaluatePopulation()
		e.computeNovelty()

		// Stats
		totalFit := 0.0
		validCount := 0
		for _, ind := range e.Population {
			if ind.Valid && ind.Fitness.TotalFitness >= FitnessFloor {
				totalFit += ind.Fitness.TotalFitness
				validCount++
			}
		}
		avgFit := 0.0
		if validCount > 0 {
			avgFit = totalFit / float64(validCount)
		}

		if progress != nil {
			progress(gen, e.BestFitness, avgFit)
		}

		e.Population = e.selectNext()
	}

	// Final evaluation
	e.evaluatePopulation()
	e.computeNovelty()
}

func (e *NoveltyEngine) initialize() {
	e.Population = make([]*NoveltyIndividual, e.Config.PopulationSize)

	for i := 0; i < e.Config.PopulationSize; i++ {
		seed := e.Seeds[e.rng.IntN(len(e.Seeds))]
		g := Mutate(seed, e.rng, e.Seeds)
		g.ID = fmt.Sprintf("init_%d", i)
		g.Generation = 0
		e.Population[i] = &NoveltyIndividual{
			Individual: Individual{Genome: g},
		}
	}
}

func (e *NoveltyEngine) evaluatePopulation() {
	var wg sync.WaitGroup
	sem := make(chan struct{}, e.Config.Workers)

	for i, ind := range e.Population {
		if ind.Valid {
			continue
		}

		wg.Add(1)
		sem <- struct{}{}

		go func(idx int, ind *NoveltyIndividual) {
			defer wg.Done()
			defer func() { <-sem }()

			seed := e.Config.BaseSeed + uint64(e.Generation)*10000 + uint64(idx)
			result := fitness.Evaluate(ind.Genome, seed)

			ind.Valid = result.Valid
			ind.Fitness = result.Metrics

			if result.Valid {
				// Compute behavior
				runner := fitness.GetRunner(ind.Genome)
				if runner != nil {
					randomAI := &sim.RandomAI{}
					batchResult := sim.RunBatch(ind.Genome, runner, randomAI, 50, seed+5000)
					ind.Behavior = ComputeBehavior(batchResult)
				}

				if result.Metrics.TotalFitness > e.BestFitness {
					e.BestFitness = result.Metrics.TotalFitness
					e.BestGenome = ind.Genome
				}
			}
		}(i, ind)
	}

	wg.Wait()
}

// computeNovelty calculates novelty score for each individual based on
// k-nearest neighbor distance in behavior space.
func (e *NoveltyEngine) computeNovelty() {
	// Collect all behavior points (population + archive)
	var allBehaviors []BehaviorDescriptor
	for _, ind := range e.Population {
		if ind.Valid && ind.Fitness.TotalFitness >= FitnessFloor {
			allBehaviors = append(allBehaviors, ind.Behavior)
		}
	}
	for _, arch := range e.Archive {
		allBehaviors = append(allBehaviors, arch.Behavior)
	}

	if len(allBehaviors) == 0 {
		return
	}

	// Compute novelty for each qualified individual
	maxNovelty := 0.0
	for _, ind := range e.Population {
		if !ind.Valid || ind.Fitness.TotalFitness < FitnessFloor {
			ind.Novelty = 0
			continue
		}

		// Compute distances to all other points
		var distances []float64
		for _, b := range allBehaviors {
			d := ind.Behavior.Distance(b)
			if d > 0 { // exclude self
				distances = append(distances, d)
			}
		}

		if len(distances) == 0 {
			ind.Novelty = 0
			continue
		}

		// k-nearest neighbor average distance
		sort.Float64s(distances)
		k := NoveltyK
		if k > len(distances) {
			k = len(distances)
		}

		sum := 0.0
		for i := 0; i < k; i++ {
			sum += distances[i]
		}
		ind.Novelty = sum / float64(k)

		if ind.Novelty > maxNovelty {
			maxNovelty = ind.Novelty
		}
	}

	// Normalize novelty to [0, 1] and compute combined fitness
	for _, ind := range e.Population {
		if !ind.Valid || ind.Fitness.TotalFitness < FitnessFloor {
			ind.Fitness.SharedFitness = 0
			continue
		}

		normalizedNovelty := 0.0
		if maxNovelty > 0 {
			normalizedNovelty = ind.Novelty / maxNovelty
		}

		// Combined: 50% fitness + 50% novelty
		ind.Fitness.SharedFitness = (1-NoveltyWeight)*ind.Fitness.TotalFitness + NoveltyWeight*normalizedNovelty
		ind.Genome.Fitness = ind.Fitness.SharedFitness

		// Add to archive if sufficiently novel
		if normalizedNovelty >= NoveltyThreshold {
			e.Archive = append(e.Archive, &NoveltyIndividual{
				Individual: ind.Individual,
				Behavior:   ind.Behavior,
				Novelty:    ind.Novelty,
			})
		}
	}

	// Cap archive size to prevent unbounded growth
	if len(e.Archive) > 1000 {
		e.Archive = e.Archive[len(e.Archive)-1000:]
	}
}

func (e *NoveltyEngine) selectNext() []*NoveltyIndividual {
	// Sort by shared fitness (novelty-adjusted)
	sort.Slice(e.Population, func(i, j int) bool {
		return e.Population[i].Fitness.SharedFitness > e.Population[j].Fitness.SharedFitness
	})

	nextGen := make([]*NoveltyIndividual, e.Config.PopulationSize)

	// Elitism
	elite := min(e.Config.EliteSize, len(e.Population))
	for i := 0; i < elite; i++ {
		nextGen[i] = &NoveltyIndividual{
			Individual: Individual{
				Genome:  e.Population[i].Genome,
				Fitness: e.Population[i].Fitness,
				Valid:   true,
			},
			Behavior: e.Population[i].Behavior,
		}
	}

	// Tournament selection + mutation
	for i := elite; i < e.Config.PopulationSize; i++ {
		parent := e.tournament()

		var child *genome.Genome
		if e.rng.Float64() < 0.3 {
			parent2 := e.tournament()
			child = Crossover(parent.Genome, parent2.Genome, e.rng)
		}
		if child == nil {
			child = Mutate(parent.Genome, e.rng, e.Seeds)
		} else {
			child = Mutate(child, e.rng, e.Seeds)
		}

		child.ID = fmt.Sprintf("gen%d_%d", e.Generation+1, e.rng.IntN(100000))
		child.Generation = e.Generation + 1
		nextGen[i] = &NoveltyIndividual{
			Individual: Individual{Genome: child},
		}
	}

	return nextGen
}

func (e *NoveltyEngine) tournament() *NoveltyIndividual {
	best := e.Population[e.rng.IntN(len(e.Population))]
	for i := 1; i < e.Config.TournamentSize; i++ {
		candidate := e.Population[e.rng.IntN(len(e.Population))]
		if candidate.Fitness.SharedFitness > best.Fitness.SharedFitness {
			best = candidate
		}
	}
	return best
}

// AllQualified returns all individuals meeting the fitness floor.
func (e *NoveltyEngine) AllQualified() ([]*Individual, []BehaviorDescriptor) {
	var individuals []*Individual
	var behaviors []BehaviorDescriptor

	// From population
	for _, ind := range e.Population {
		if ind.Valid && ind.Fitness.TotalFitness >= FitnessFloor {
			individuals = append(individuals, &ind.Individual)
			behaviors = append(behaviors, ind.Behavior)
		}
	}

	// From archive (deduplicate by genome ID)
	seen := make(map[string]bool)
	for _, ind := range individuals {
		seen[ind.Genome.ID] = true
	}
	for _, arch := range e.Archive {
		if !seen[arch.Genome.ID] && arch.Fitness.TotalFitness >= FitnessFloor {
			individuals = append(individuals, &arch.Individual)
			behaviors = append(behaviors, arch.Behavior)
			seen[arch.Genome.ID] = true
		}
	}

	return individuals, behaviors
}

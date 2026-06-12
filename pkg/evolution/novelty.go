package evolution

import (
	"fmt"
	"math/rand/v2"
	"runtime"
	"sort"
	"sync"

	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

const (
	NoveltyK         = 15  // k-nearest neighbors for novelty
	NoveltyWeight    = 0.5 // weight of novelty vs fitness
	NoveltyThreshold = 0.3 // min novelty to enter archive

	// DefaultFitnessFloor is derived from the seed-calibration suite
	// (Task 15): worst classic survivor-conditioned mean (crazy-eights
	// 0.475 at the Task 14 commit) minus 0.05. The previous folklore value
	// 0.70 sat ABOVE every trick-taking classic, zeroing the selection
	// gradient for human-validated games. If the calibration suite's
	// worst-classic mean moves, re-derive this constant
	// (TestCalibrationClassicsAboveFloor enforces the relationship).
	DefaultFitnessFloor = 0.42
)

// FitnessFloor is the minimum fitness for QD consideration (novelty archive
// admission, sharing, output ranking). Overridable via the evolve command's
// -fitness-floor flag; everything else should treat it as a constant.
var FitnessFloor = DefaultFitnessFloor

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
	if config.Workers == 0 {
		config.Workers = runtime.NumCPU()
	}
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
			}
		}(i, ind)
	}

	wg.Wait()

	for _, ind := range e.Population {
		if ind.Valid && ind.Fitness.TotalFitness > e.BestFitness {
			e.BestFitness = ind.Fitness.TotalFitness
			e.BestGenome = ind.Genome
		}
	}
}

// computeNovelty calculates novelty score for each individual based on
// k-nearest neighbor distance in behavior space, computed WITHIN-SKELETON only.
// Also applies fitness sharing by skeleton niche to prevent monopoly.
func (e *NoveltyEngine) computeNovelty() {
	// Collect behavior points per skeleton (population + archive). Track the
	// owning *NoveltyIndividual so each individual can skip its own entry by
	// identity instead of filtering zero-valued distances -- otherwise
	// converged clusters drop their intra-cluster zeros and pull k-NN from
	// faraway outliers, which would reward (not penalize) duplication.
	type behaviorPoint struct {
		Behavior BehaviorDescriptor
		Owner    *NoveltyIndividual // nil for archive entries
	}
	perSkeleton := make(map[genome.SkeletonType][]behaviorPoint)
	for _, ind := range e.Population {
		if ind.Valid && ind.Fitness.TotalFitness >= FitnessFloor {
			skel := ind.Genome.Skeleton
			perSkeleton[skel] = append(perSkeleton[skel], behaviorPoint{Behavior: ind.Behavior, Owner: ind})
		}
	}
	for _, arch := range e.Archive {
		skel := arch.Genome.Skeleton
		perSkeleton[skel] = append(perSkeleton[skel], behaviorPoint{Behavior: arch.Behavior, Owner: nil})
	}

	if len(perSkeleton) == 0 {
		return
	}

	// Count valid individuals per skeleton for fitness sharing
	nicheCounts := make(map[genome.SkeletonType]int)
	totalValid := 0
	for _, ind := range e.Population {
		if ind.Valid && ind.Fitness.TotalFitness >= FitnessFloor {
			nicheCounts[ind.Genome.Skeleton]++
			totalValid++
		}
	}

	// Compute novelty per skeleton (k-NN within-skeleton only)
	maxNoveltyPerSkel := make(map[genome.SkeletonType]float64)
	for _, ind := range e.Population {
		if !ind.Valid || ind.Fitness.TotalFitness < FitnessFloor {
			ind.Novelty = 0
			continue
		}

		skel := ind.Genome.Skeleton
		points := perSkeleton[skel]

		var distances []float64
		for _, p := range points {
			if p.Owner == ind {
				continue // skip self by identity, not by zero distance
			}
			distances = append(distances, ind.Behavior.Distance(p.Behavior))
		}

		if len(distances) == 0 {
			ind.Novelty = 0
			continue
		}

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

		if ind.Novelty > maxNoveltyPerSkel[skel] {
			maxNoveltyPerSkel[skel] = ind.Novelty
		}
	}

	// Expected share per skeleton for fitness sharing
	numNiches := len(nicheCounts)
	expectedPerNiche := 0.0
	if numNiches > 0 {
		expectedPerNiche = float64(totalValid) / float64(numNiches)
	}

	// Normalize novelty within each skeleton (so rummy variations compete
	// against rummy, not against all games) and apply fitness sharing.
	for _, ind := range e.Population {
		if !ind.Valid || ind.Fitness.TotalFitness < FitnessFloor {
			ind.Fitness.SharedFitness = 0
			continue
		}

		skel := ind.Genome.Skeleton
		maxNov := maxNoveltyPerSkel[skel]
		normalizedNovelty := 0.0
		if maxNov > 0 {
			normalizedNovelty = ind.Novelty / maxNov
		}

		// Combined: 50% fitness + 50% within-skeleton novelty
		combined := (1-NoveltyWeight)*ind.Fitness.TotalFitness + NoveltyWeight*normalizedNovelty

		// Apply fitness sharing by skeleton niche
		count := float64(nicheCounts[skel])
		ratio := count / expectedPerNiche
		if ratio < 1 {
			// Boost underrepresented niches (same as main engine)
			boost := 1.0 / ratio
			if boost > 3.0 {
				boost = 3.0
			}
			combined *= boost
		} else {
			combined /= ratio
		}

		ind.Fitness.SharedFitness = combined
		ind.Genome.Fitness = ind.Fitness.SharedFitness

		// Add to archive if sufficiently novel (within-skeleton normalized)
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

package evolution

import (
	"fmt"
	"math/rand/v2"
	"sync"

	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

const GridSize = 10

// ArchiveCell holds the best individual for a behavior niche.
type ArchiveCell struct {
	Individual *Individual
	Behavior   BehaviorDescriptor
}

// MAPElitesEngine runs MAP-Elites quality-diversity search.
// Maintains one 10x10 archive per skeleton type.
type MAPElitesEngine struct {
	Config   Config
	Seeds    []*genome.Genome
	Archives map[genome.SkeletonType]*Archive
	rng      *rand.Rand

	// Stats
	BestFitness float64
	BestGenome  *genome.Genome
}

// Archive is a 2D grid of behavior niches.
type Archive struct {
	Cells    [GridSize][GridSize]*ArchiveCell
	Occupied int
	QDScore  float64
}

// NewMAPElitesEngine creates a MAP-Elites engine.
func NewMAPElitesEngine(config Config, seeds []*genome.Genome) *MAPElitesEngine {
	return &MAPElitesEngine{
		Config: config,
		Seeds:  seeds,
		Archives: map[genome.SkeletonType]*Archive{
			genome.Shedding:    {},
			genome.TrickTaking: {},
			genome.Rummy:       {},
		},
		rng: rand.New(rand.NewPCG(config.BaseSeed, 0)),
	}
}

// Run executes the MAP-Elites algorithm.
func (e *MAPElitesEngine) Run(progress func(gen int, best float64, avg float64)) {
	// Seed the archives with initial evaluations
	e.seedArchives()

	// Main loop: generate offspring, evaluate, attempt insertion
	for gen := 0; gen < e.Config.Generations; gen++ {
		e.generation(gen)

		if progress != nil {
			occupied, qdScore := e.totalStats()
			progress(gen, e.BestFitness, qdScore/float64(max(occupied, 1)))
		}
	}
}

// seedArchives evaluates seed genomes and mutants to initialize archives.
func (e *MAPElitesEngine) seedArchives() {
	var candidates []*genome.Genome

	// Create initial candidates from seeds + mutations
	for i := 0; i < e.Config.PopulationSize; i++ {
		seed := e.Seeds[e.rng.IntN(len(e.Seeds))]
		g := Mutate(seed, e.rng, e.Seeds)
		g.ID = fmt.Sprintf("init_%d", i)
		g.Generation = 0
		candidates = append(candidates, g)
	}

	// Evaluate and insert
	e.evaluateAndInsert(candidates, 0)
}

// generation produces PopulationSize offspring and attempts archive insertion.
func (e *MAPElitesEngine) generation(gen int) {
	var offspring []*genome.Genome

	for i := 0; i < e.Config.PopulationSize; i++ {
		// Pick a random occupied cell from a random archive
		parent := e.randomArchiveOccupant()
		if parent == nil {
			// No occupants yet, mutate a seed
			seed := e.Seeds[e.rng.IntN(len(e.Seeds))]
			child := Mutate(seed, e.rng, e.Seeds)
			child.ID = fmt.Sprintf("gen%d_%d", gen+1, e.rng.IntN(100000))
			child.Generation = gen + 1
			offspring = append(offspring, child)
			continue
		}

		var child *genome.Genome
		if e.rng.Float64() < 0.3 {
			// Crossover with another archive occupant of same skeleton
			parent2 := e.randomArchiveOccupantOfSkeleton(parent.Skeleton)
			if parent2 != nil {
				child = Crossover(parent, parent2, e.rng)
			}
		}
		if child == nil {
			child = Mutate(parent, e.rng, e.Seeds)
		} else {
			child = Mutate(child, e.rng, e.Seeds)
		}

		child.ID = fmt.Sprintf("gen%d_%d", gen+1, e.rng.IntN(100000))
		child.Generation = gen + 1
		offspring = append(offspring, child)
	}

	e.evaluateAndInsert(offspring, gen+1)
}

// evaluateAndInsert evaluates genomes in parallel and inserts qualifying ones into archives.
func (e *MAPElitesEngine) evaluateAndInsert(genomes []*genome.Genome, gen int) {
	type evalResult struct {
		genome   *genome.Genome
		metrics  fitness.Metrics
		behavior BehaviorDescriptor
		valid    bool
	}

	results := make([]evalResult, len(genomes))
	var wg sync.WaitGroup
	sem := make(chan struct{}, e.Config.Workers)

	for i, g := range genomes {
		wg.Add(1)
		sem <- struct{}{}

		go func(idx int, g *genome.Genome) {
			defer wg.Done()
			defer func() { <-sem }()

			seed := e.Config.BaseSeed + uint64(gen)*10000 + uint64(idx)
			result := fitness.Evaluate(g, seed)

			if !result.Valid || result.Metrics.TotalFitness < 0.70 {
				return
			}

			// Compute behavior from a fresh batch
			runner := fitness.GetRunner(g)
			if runner == nil {
				return
			}
			randomAI := &sim.RandomAI{}
			batchResult := sim.RunBatch(g, runner, randomAI, 50, seed+5000)
			behavior := ComputeBehavior(batchResult)

			results[idx] = evalResult{
				genome:   g,
				metrics:  result.Metrics,
				behavior: behavior,
				valid:    true,
			}
		}(i, g)
	}

	wg.Wait()

	// Insert into archives (sequential to avoid races)
	for _, r := range results {
		if !r.valid {
			continue
		}

		archive := e.Archives[r.genome.Skeleton]
		row, col := r.behavior.GridCell(GridSize)

		cell := archive.Cells[row][col]
		if cell == nil || r.metrics.TotalFitness > cell.Individual.Fitness.TotalFitness {
			if cell == nil {
				archive.Occupied++
			} else {
				archive.QDScore -= cell.Individual.Fitness.TotalFitness
			}

			archive.Cells[row][col] = &ArchiveCell{
				Individual: &Individual{
					Genome:  r.genome,
					Fitness: r.metrics,
					Valid:   true,
				},
				Behavior: r.behavior,
			}
			archive.QDScore += r.metrics.TotalFitness

			if r.metrics.TotalFitness > e.BestFitness {
				e.BestFitness = r.metrics.TotalFitness
				e.BestGenome = r.genome
			}
		}
	}
}

// randomArchiveOccupant returns a random genome from any occupied archive cell.
func (e *MAPElitesEngine) randomArchiveOccupant() *genome.Genome {
	// Iterate skeletons in a stable order so seeded runs are reproducible;
	// Go map iteration order is randomized per process and would otherwise
	// shuffle the index space passed to rng.IntN.
	var occupants []*genome.Genome
	for _, skel := range []genome.SkeletonType{genome.Shedding, genome.TrickTaking, genome.Rummy} {
		archive, ok := e.Archives[skel]
		if !ok {
			continue
		}
		for r := 0; r < GridSize; r++ {
			for c := 0; c < GridSize; c++ {
				if archive.Cells[r][c] != nil {
					occupants = append(occupants, archive.Cells[r][c].Individual.Genome)
				}
			}
		}
	}
	if len(occupants) == 0 {
		return nil
	}
	return occupants[e.rng.IntN(len(occupants))]
}

// randomArchiveOccupantOfSkeleton returns a random genome from the given skeleton's archive.
func (e *MAPElitesEngine) randomArchiveOccupantOfSkeleton(skel genome.SkeletonType) *genome.Genome {
	archive := e.Archives[skel]
	var occupants []*genome.Genome
	for r := 0; r < GridSize; r++ {
		for c := 0; c < GridSize; c++ {
			if archive.Cells[r][c] != nil {
				occupants = append(occupants, archive.Cells[r][c].Individual.Genome)
			}
		}
	}
	if len(occupants) == 0 {
		return nil
	}
	return occupants[e.rng.IntN(len(occupants))]
}

// totalStats returns total occupied cells and QD-score across all archives.
func (e *MAPElitesEngine) totalStats() (int, float64) {
	occupied := 0
	qdScore := 0.0
	for _, archive := range e.Archives {
		occupied += archive.Occupied
		qdScore += archive.QDScore
	}
	return occupied, qdScore
}

// AllQualified returns all genomes in the archives with their behaviors.
func (e *MAPElitesEngine) AllQualified() []*Individual {
	var result []*Individual
	for _, archive := range e.Archives {
		for r := 0; r < GridSize; r++ {
			for c := 0; c < GridSize; c++ {
				if archive.Cells[r][c] != nil {
					result = append(result, archive.Cells[r][c].Individual)
				}
			}
		}
	}
	return result
}

package evolution

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"runtime"
	"sort"
	"sync"

	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/genome"
)

// Config controls the evolution run.
type Config struct {
	PopulationSize int
	Generations    int
	EliteSize      int // Top N carried forward unchanged
	TournamentSize int
	Workers        int // 0 = auto-detect
	BaseSeed       uint64
	SaveTopN       int
	OutputDir      string
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		PopulationSize: 500,
		Generations:    100,
		EliteSize:      10,
		TournamentSize: 5,
		Workers:        0,
		BaseSeed:       42,
		SaveTopN:       20,
		OutputDir:      "output",
	}
}

// Individual wraps a genome with its fitness evaluation.
type Individual struct {
	Genome  *genome.Genome
	Fitness fitness.Metrics
	Valid   bool
}

// Engine runs the evolutionary algorithm.
type Engine struct {
	Config     Config
	Seeds      []*genome.Genome
	Population []*Individual
	Generation int
	rng        *rand.Rand

	// Stats
	BestFitness float64
	BestGenome  *genome.Genome
}

// NewEngine creates an evolution engine with the given config and seed genomes.
func NewEngine(config Config, seeds []*genome.Genome) *Engine {
	if config.Workers == 0 {
		config.Workers = runtime.NumCPU()
	}

	return &Engine{
		Config: config,
		Seeds:  seeds,
		rng:    rand.New(rand.NewPCG(config.BaseSeed, 0)),
	}
}

// Initialize creates the initial population from seed genomes.
func (e *Engine) Initialize() {
	e.Population = make([]*Individual, e.Config.PopulationSize)

	for i := 0; i < e.Config.PopulationSize; i++ {
		// Pick a random seed and mutate it
		seed := e.Seeds[e.rng.IntN(len(e.Seeds))]
		g := Mutate(seed, e.rng, e.Seeds)
		g.ID = fmt.Sprintf("init_%d", i)
		g.Generation = 0
		e.Population[i] = &Individual{Genome: g}
	}
}

// EvaluatePopulation runs fitness evaluation on all individuals in parallel.
func (e *Engine) EvaluatePopulation() {
	var wg sync.WaitGroup
	sem := make(chan struct{}, e.Config.Workers)

	for i, ind := range e.Population {
		if ind.Valid {
			continue // Already evaluated (elite carried forward)
		}

		wg.Add(1)
		sem <- struct{}{} // Acquire worker slot

		go func(idx int, ind *Individual) {
			defer wg.Done()
			defer func() { <-sem }() // Release worker slot

			seed := e.Config.BaseSeed + uint64(e.Generation)*10000 + uint64(idx)
			result := fitness.Evaluate(ind.Genome, seed)

			ind.Valid = result.Valid
			ind.Fitness = result.Metrics
			if result.Valid {
				ind.Genome.Fitness = result.Metrics.TotalFitness
			}
		}(i, ind)
	}

	wg.Wait()
}

// Select performs tournament selection to create the next generation.
func (e *Engine) Select() []*Individual {
	// Sort by fitness (descending)
	sort.Slice(e.Population, func(i, j int) bool {
		return e.Population[i].Fitness.TotalFitness > e.Population[j].Fitness.TotalFitness
	})

	// Track best
	if len(e.Population) > 0 && e.Population[0].Fitness.TotalFitness > e.BestFitness {
		e.BestFitness = e.Population[0].Fitness.TotalFitness
		e.BestGenome = e.Population[0].Genome
	}

	nextGen := make([]*Individual, e.Config.PopulationSize)

	// Elitism: top N carry forward
	elite := min(e.Config.EliteSize, len(e.Population))
	for i := 0; i < elite; i++ {
		nextGen[i] = &Individual{
			Genome:  e.Population[i].Genome,
			Fitness: e.Population[i].Fitness,
			Valid:   true, // Skip re-evaluation
		}
	}

	// Fill rest via tournament selection + mutation/crossover
	for i := elite; i < e.Config.PopulationSize; i++ {
		parent := e.tournament()

		if e.rng.Float64() < 0.3 {
			// Crossover
			parent2 := e.tournament()
			child := Crossover(parent.Genome, parent2.Genome, e.rng)
			if child != nil {
				child = Mutate(child, e.rng, e.Seeds)
				nextGen[i] = &Individual{Genome: child}
				continue
			}
		}

		// Mutation only
		child := Mutate(parent.Genome, e.rng, e.Seeds)
		nextGen[i] = &Individual{Genome: child}
	}

	// Diversity: kill duplicate genomes in top 50
	dedup(nextGen)

	return nextGen
}

func (e *Engine) tournament() *Individual {
	best := e.Population[e.rng.IntN(len(e.Population))]
	for i := 1; i < e.Config.TournamentSize; i++ {
		candidate := e.Population[e.rng.IntN(len(e.Population))]
		if candidate.Fitness.TotalFitness > best.Fitness.TotalFitness {
			best = candidate
		}
	}
	return best
}

// dedup removes duplicate genomes in the top 50 by hash.
func dedup(pop []*Individual) {
	seen := make(map[string]bool)
	top := min(50, len(pop))
	for i := 0; i < top; i++ {
		if pop[i].Genome == nil {
			continue
		}
		hash := genomeHash(pop[i].Genome)
		if seen[hash] {
			// Replace with a fresh mutation of a random valid individual
			for j := 0; j < top; j++ {
				if pop[j].Genome != nil && !seen[genomeHash(pop[j].Genome)] {
					pop[i].Valid = false // Force re-evaluation
					break
				}
			}
		}
		seen[hash] = true
	}
}

func genomeHash(g *genome.Genome) string {
	// Hash key params (not ID/generation/fitness)
	return fmt.Sprintf("%d_%d_%d_%d_%v_%v_%v_%d",
		g.Skeleton, g.Players, g.HandSize, g.TrumpRule,
		g.Shedding, g.TrickTaking, g.Rummy, len(g.SpecialCards))
}

// Run executes the full evolution loop.
func (e *Engine) Run(progress func(gen int, best float64, avg float64)) {
	e.Initialize()

	for gen := 0; gen < e.Config.Generations; gen++ {
		e.Generation = gen
		e.EvaluatePopulation()

		// Compute stats
		totalFit := 0.0
		validCount := 0
		for _, ind := range e.Population {
			if ind.Valid {
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

		// Select and create next generation
		e.Population = e.Select()
	}

	// Final evaluation
	e.EvaluatePopulation()
}

// TopN returns the top N genomes sorted by fitness.
func (e *Engine) TopN(n int) []*Individual {
	sort.Slice(e.Population, func(i, j int) bool {
		return e.Population[i].Fitness.TotalFitness > e.Population[j].Fitness.TotalFitness
	})

	top := min(n, len(e.Population))
	result := make([]*Individual, top)
	copy(result, e.Population[:top])
	return result
}

// SavePopulation writes the full population to a JSON file.
func (e *Engine) SavePopulation(path string) error {
	var genomes []*genome.Genome
	for _, ind := range e.Population {
		if ind.Genome != nil {
			genomes = append(genomes, ind.Genome)
		}
	}

	data, err := json.MarshalIndent(genomes, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

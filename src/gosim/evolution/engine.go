package evolution

import (
	"fmt"
	"log"
	"math/rand"
	"runtime"
	"time"

	"github.com/signalnine/darwindeck/gosim/evolution/fitness"
	"github.com/signalnine/darwindeck/gosim/evolution/operators"
	"github.com/signalnine/darwindeck/gosim/genome"
)

// EvolutionConfig holds configuration for an evolutionary run.
type EvolutionConfig struct {
	PopulationSize       int     // Number of individuals per generation
	MaxGenerations       int     // Maximum generations to run
	ElitismRate          float64 // Top percentage preserved (0.1 = 10%)
	CrossoverRate        float64 // Probability of crossover (0.7 = 70%)
	TournamentSize       int     // Tournament selection size
	PlateauThreshold     int     // Generations without improvement before stopping (0 = disabled)
	ImprovementThreshold float64 // Minimum improvement to not be a plateau (0.005 = 0.5%)
	DiversityThreshold   float64 // Diversity below this triggers aggressive mutation
	SeedRatio            float64 // Ratio of known games to mutants (0.7 = 70% known)
	RandomSeed           int64   // Random seed (0 = use time)
	FitnessStyle         string  // Fitness weight preset (balanced, bluffing, strategic, party, trick-taking)
	NumWorkers           int     // Number of parallel workers (0 = auto)
	GamesPerEval         int     // Games per fitness evaluation
	UseMCTS              bool    // Use MCTS for evaluation (slower but more accurate)
	Verbose              bool    // Enable verbose logging
}

// DefaultConfig returns a default evolution configuration.
func DefaultConfig() *EvolutionConfig {
	return &EvolutionConfig{
		PopulationSize:       100,
		MaxGenerations:       100,
		ElitismRate:          0.1,
		CrossoverRate:        0.7,
		TournamentSize:       3,
		PlateauThreshold:     0, // Disabled by default
		ImprovementThreshold: 0.005,
		DiversityThreshold:   0.1,
		SeedRatio:            0.7,
		RandomSeed:           0,
		FitnessStyle:         "balanced",
		NumWorkers:           0, // Auto-detect
		GamesPerEval:         100,
		UseMCTS:              false,
		Verbose:              false,
	}
}

// GenerationStats holds statistics for a single generation.
type GenerationStats struct {
	Generation  int
	BestFitness float64
	AvgFitness  float64
	Diversity   float64
	Evaluations int
	Timestamp   time.Time
}

// EvolutionEngine runs the evolutionary algorithm.
type EvolutionEngine struct {
	Config           *EvolutionConfig
	Population       *Population
	StatsHistory     []GenerationStats
	BestEver         *Individual
	Rng              *rand.Rand
	Evaluator        *ParallelEvaluator
	MutationPipeline *operators.MutationPipeline
	Crossover        *UniformCrossover
	UseAggressive    bool // Switch to aggressive mutation when diversity drops

	// Callbacks for progress reporting
	OnGenerationComplete func(stats GenerationStats)
}

// NewEvolutionEngine creates a new evolution engine.
func NewEvolutionEngine(config *EvolutionConfig) *EvolutionEngine {
	if config == nil {
		config = DefaultConfig()
	}

	// Initialize RNG
	seed := config.RandomSeed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	rng := rand.New(rand.NewSource(seed))

	// Initialize workers
	numWorkers := config.NumWorkers
	if numWorkers <= 0 {
		numWorkers = runtime.NumCPU()
	}

	// Create mutation pipeline
	mutationPipeline := operators.NewDefaultPipeline(rng)

	return &EvolutionEngine{
		Config:           config,
		Rng:              rng,
		Evaluator:        NewParallelEvaluator(config.FitnessStyle, numWorkers),
		MutationPipeline: mutationPipeline,
		Crossover:        NewUniformCrossover(config.CrossoverRate),
		StatsHistory:     make([]GenerationStats, 0, config.MaxGenerations),
	}
}

// Close releases resources.
func (e *EvolutionEngine) Close() {
	if e.Evaluator != nil {
		e.Evaluator.Close()
	}
}

// InitializePopulation creates the initial population from seed genomes.
func (e *EvolutionEngine) InitializePopulation() error {
	if e.Config.Verbose {
		log.Printf("Initializing population of size %d", e.Config.PopulationSize)
	}

	// Get seed genomes
	seedGenomes := genome.GetSeedGenomes()
	if len(seedGenomes) == 0 {
		return fmt.Errorf("no seed genomes available")
	}

	// Calculate how many should be seeds vs mutants
	numSeeds := int(float64(e.Config.PopulationSize) * e.Config.SeedRatio)
	if numSeeds > len(seedGenomes) {
		numSeeds = len(seedGenomes)
	}

	individuals := make([]*Individual, 0, e.Config.PopulationSize)

	// Add seed genomes (with cloning)
	for i := 0; i < numSeeds; i++ {
		seedIdx := i % len(seedGenomes)
		cloned := seedGenomes[seedIdx].Clone()
		individuals = append(individuals, &Individual{
			Genome:    cloned,
			Fitness:   0.0,
			Evaluated: false,
		})
	}

	// Fill rest with mutated copies
	for len(individuals) < e.Config.PopulationSize {
		// Pick random seed and clone
		seedIdx := e.Rng.Intn(len(seedGenomes))
		cloned := seedGenomes[seedIdx].Clone()

		// Apply mutations
		e.MutationPipeline.Apply(cloned, e.Rng)

		individuals = append(individuals, &Individual{
			Genome:    cloned,
			Fitness:   0.0,
			Evaluated: false,
		})
	}

	e.Population = NewPopulation(individuals)

	if e.Config.Verbose {
		log.Printf("Population initialized with %d individuals (%d seeds, %d mutants)",
			len(individuals), numSeeds, len(individuals)-numSeeds)
	}

	return nil
}

// EvaluatePopulation evaluates fitness for all unevaluated individuals.
func (e *EvolutionEngine) EvaluatePopulation() {
	if e.Population == nil {
		return
	}

	unevaluated := e.Population.GetUnevaluated()
	if len(unevaluated) == 0 {
		return
	}

	if e.Config.Verbose {
		log.Printf("Evaluating %d individuals...", len(unevaluated))
	}

	// Evaluate in parallel
	e.Evaluator.EvaluateIndividuals(unevaluated, e.Config.GamesPerEval, e.Config.UseMCTS)

	if e.Config.Verbose {
		log.Printf("Evaluation complete. Avg fitness: %.3f", e.Population.GetAverageFitness())
	}
}

// CreateOffspring creates the next generation via selection, crossover, and mutation.
func (e *EvolutionEngine) CreateOffspring() []*Individual {
	offspring := make([]*Individual, 0, e.Config.PopulationSize)

	// 1. Elitism - preserve top individuals
	nElite := int(float64(e.Config.PopulationSize) * e.Config.ElitismRate)
	elite := SelectElite(e.Population, nElite)
	for _, ind := range elite {
		offspring = append(offspring, ind.Clone())
	}

	// 2. Create remaining offspring via selection + crossover + mutation
	for len(offspring) < e.Config.PopulationSize {
		// Select two parents
		parent1 := TournamentSelection(e.Population, e.Config.TournamentSize, e.Rng)
		parent2 := TournamentSelection(e.Population, e.Config.TournamentSize, e.Rng)

		// Crossover
		child1, child2 := e.Crossover.Crossover(parent1.Genome, parent2.Genome, e.Rng)

		// Mutation
		e.MutationPipeline.Apply(child1, e.Rng)
		e.MutationPipeline.Apply(child2, e.Rng)

		// Add to offspring (unevaluated)
		offspring = append(offspring, &Individual{
			Genome:    child1,
			Fitness:   0.0,
			Evaluated: false,
		})

		if len(offspring) < e.Config.PopulationSize {
			offspring = append(offspring, &Individual{
				Genome:    child2,
				Fitness:   0.0,
				Evaluated: false,
			})
		}
	}

	return offspring[:e.Config.PopulationSize]
}

// CheckPlateau returns true if evolution has plateaued.
func (e *EvolutionEngine) CheckPlateau() bool {
	if e.Config.PlateauThreshold <= 0 {
		return false
	}

	if len(e.StatsHistory) < e.Config.PlateauThreshold {
		return false
	}

	// Get recent stats
	recent := e.StatsHistory[len(e.StatsHistory)-e.Config.PlateauThreshold:]
	bestRecent := recent[0].BestFitness
	oldestRecent := recent[0].BestFitness

	for _, s := range recent {
		if s.BestFitness > bestRecent {
			bestRecent = s.BestFitness
		}
	}

	if oldestRecent == 0 {
		return false
	}

	improvement := (bestRecent - oldestRecent) / oldestRecent
	return improvement < e.Config.ImprovementThreshold
}

// Evolve runs the evolutionary loop.
func (e *EvolutionEngine) Evolve() error {
	if e.Config.Verbose {
		log.Println("Starting evolutionary loop...")
	}

	// Initialize population if not already done
	if e.Population == nil {
		if err := e.InitializePopulation(); err != nil {
			return err
		}
	}

	// Evaluate initial population
	e.EvaluatePopulation()

	// Evolution loop
	for generation := 0; generation < e.Config.MaxGenerations; generation++ {
		if e.Config.Verbose {
			log.Printf("\n============================================================")
			log.Printf("Generation %d/%d", generation+1, e.Config.MaxGenerations)
			log.Printf("============================================================")
		}

		// Compute statistics
		best := e.Population.GetBestIndividual()
		avgFitness := e.Population.GetAverageFitness()
		diversity := e.Population.ComputeDiversity()

		// Update best ever
		if e.BestEver == nil || best.Fitness > e.BestEver.Fitness {
			e.BestEver = best.Clone()
			if e.Config.Verbose {
				log.Printf("New best fitness: %.4f", best.Fitness)
			}
		}

		// Store stats
		stats := GenerationStats{
			Generation:  generation,
			BestFitness: best.Fitness,
			AvgFitness:  avgFitness,
			Diversity:   diversity,
			Evaluations: len(e.Population.Individuals),
			Timestamp:   time.Now(),
		}
		e.StatsHistory = append(e.StatsHistory, stats)

		// Callback
		if e.OnGenerationComplete != nil {
			e.OnGenerationComplete(stats)
		}

		if e.Config.Verbose {
			modeIndicator := ""
			if e.UseAggressive {
				modeIndicator = " [AGGRESSIVE]"
			}
			log.Printf("Best fitness: %.4f", best.Fitness)
			log.Printf("Avg fitness: %.4f", avgFitness)
			log.Printf("Diversity: %.4f%s", diversity, modeIndicator)
		}

		// Check diversity and switch mutation mode
		if diversity < e.Config.DiversityThreshold {
			if !e.UseAggressive {
				if e.Config.Verbose {
					log.Printf("WARNING: Low diversity (%.4f) - switching to AGGRESSIVE mutation mode", diversity)
				}
				e.UseAggressive = true
				e.MutationPipeline = operators.NewAggressivePipeline(e.Rng)
			}
		} else if diversity > e.Config.DiversityThreshold*1.5 {
			if e.UseAggressive {
				if e.Config.Verbose {
					log.Printf("Diversity recovered (%.4f) - switching back to normal mutation mode", diversity)
				}
				e.UseAggressive = false
				e.MutationPipeline = operators.NewDefaultPipeline(e.Rng)
			}
		}

		// Check plateau
		if e.CheckPlateau() {
			if e.Config.Verbose {
				log.Println("Stopping due to plateau")
			}
			break
		}

		// Create next generation
		offspring := e.CreateOffspring()
		e.Population = NewPopulation(offspring)
		e.Population.Generation = generation + 1

		// Evaluate new individuals
		e.EvaluatePopulation()
	}

	if e.Config.Verbose {
		log.Println("\n============================================================")
		log.Println("Evolution complete!")
		if e.BestEver != nil {
			log.Printf("Best fitness: %.4f", e.BestEver.Fitness)
		}
		log.Println("============================================================")
	}

	return nil
}

// GetBestGenomes returns the top N unique genomes.
func (e *EvolutionEngine) GetBestGenomes(n int) []*Individual {
	if e.Population == nil {
		return nil
	}

	// Sort population by fitness
	sorted := e.Population.SortByFitness()

	// Deduplicate by name (simple approach)
	seen := make(map[string]bool)
	unique := make([]*Individual, 0, n)

	// Add best ever first
	if e.BestEver != nil {
		seen[e.BestEver.Genome.Name] = true
		unique = append(unique, e.BestEver)
	}

	for _, ind := range sorted {
		if seen[ind.Genome.Name] {
			continue
		}
		seen[ind.Genome.Name] = true
		unique = append(unique, ind)
		if len(unique) >= n {
			break
		}
	}

	return unique
}

// GetStats returns the stats history.
func (e *EvolutionEngine) GetStats() []GenerationStats {
	return e.StatsHistory
}

// GetFitnessEvaluator returns the fitness evaluator for metrics inspection.
func (e *EvolutionEngine) GetFitnessEvaluator() *fitness.Evaluator {
	return e.Evaluator.Evaluator
}

package evolution

import (
	"runtime"
	"sync"

	"github.com/signalnine/darwindeck/gosim/evolution/fitness"
	"github.com/signalnine/darwindeck/gosim/genome"
	"github.com/signalnine/darwindeck/gosim/simulation"
)

// AIType constants for convenience.
const (
	AITypeRandom   = simulation.RandomAI
	AITypeGreedy   = simulation.GreedyAI
	AITypeMCTS100  = simulation.MCTS100AI
	AITypeMCTS500  = simulation.MCTS500AI
	AITypeMCTS1000 = simulation.MCTS1000AI
	AITypeMCTS2000 = simulation.MCTS2000AI
)

// EvaluationTask represents a single genome evaluation task.
type EvaluationTask struct {
	Index          int
	Genome         *genome.GameGenome
	NumSimulations int
	UseMCTS        bool
}

// EvaluationResult holds the result of a genome evaluation.
type EvaluationResult struct {
	Index   int
	Metrics *fitness.FitnessMetrics
}

// ParallelEvaluator evaluates genomes in parallel using goroutines.
type ParallelEvaluator struct {
	NumWorkers int
	Evaluator  *fitness.Evaluator
	Style      string
}

// NewParallelEvaluator creates a new parallel evaluator.
func NewParallelEvaluator(style string, numWorkers int) *ParallelEvaluator {
	if numWorkers <= 0 {
		numWorkers = runtime.NumCPU()
	}

	return &ParallelEvaluator{
		NumWorkers: numWorkers,
		Evaluator:  fitness.NewEvaluator(style, nil),
		Style:      style,
	}
}

// EvaluatePopulation evaluates all genomes in parallel.
func (pe *ParallelEvaluator) EvaluatePopulation(
	genomes []*genome.GameGenome,
	numSimulations int,
	useMCTS bool,
) []*fitness.FitnessMetrics {
	if len(genomes) == 0 {
		return nil
	}

	// Create task channel
	tasks := make(chan EvaluationTask, len(genomes))
	results := make(chan EvaluationResult, len(genomes))

	// Start worker pool
	var wg sync.WaitGroup
	for i := 0; i < pe.NumWorkers; i++ {
		wg.Add(1)
		go pe.worker(tasks, results, &wg, numSimulations, useMCTS)
	}

	// Submit tasks
	for i, g := range genomes {
		tasks <- EvaluationTask{
			Index:          i,
			Genome:         g,
			NumSimulations: numSimulations,
			UseMCTS:        useMCTS,
		}
	}
	close(tasks)

	// Wait for workers and close results
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results in order
	metricsArray := make([]*fitness.FitnessMetrics, len(genomes))
	for result := range results {
		metricsArray[result.Index] = result.Metrics
	}

	return metricsArray
}

// worker processes evaluation tasks.
func (pe *ParallelEvaluator) worker(
	tasks <-chan EvaluationTask,
	results chan<- EvaluationResult,
	wg *sync.WaitGroup,
	numSimulations int,
	useMCTS bool,
) {
	defer wg.Done()

	for task := range tasks {
		metrics := pe.evaluateGenome(task.Genome, numSimulations, useMCTS)
		results <- EvaluationResult{
			Index:   task.Index,
			Metrics: metrics,
		}
	}
}

// evaluateGenome evaluates a single genome.
func (pe *ParallelEvaluator) evaluateGenome(
	g *genome.GameGenome,
	numSimulations int,
	useMCTS bool,
) *fitness.FitnessMetrics {
	// Validate genome first
	if !genome.IsValid(g) {
		return &fitness.FitnessMetrics{
			Valid:        false,
			TotalFitness: 0.0,
		}
	}

	// Select AI type
	aiType := simulation.RandomAI
	if useMCTS {
		aiType = simulation.MCTS100AI
	}

	// Run simulations using typed genome runner (direct AST interpretation)
	simResults := simulation.RunBatchTyped(g, numSimulations, aiType, 0, 0)

	// Convert to fitness.SimulationResults
	fitnessResults := convertAggregatedStats(&simResults, genome.DefaultPlayerCount)

	// Evaluate fitness
	return pe.Evaluator.Evaluate(g, fitnessResults)
}

// convertAggregatedStats converts simulation.AggregatedStats to fitness.SimulationResults.
func convertAggregatedStats(stats *simulation.AggregatedStats, playerCount int) *fitness.SimulationResults {
	if stats == nil {
		return &fitness.SimulationResults{
			TotalGames:  0,
			PlayerCount: playerCount,
		}
	}

	// Convert wins array
	wins := make([]int, len(stats.Wins))
	for i, w := range stats.Wins {
		wins[i] = int(w)
	}

	return &fitness.SimulationResults{
		TotalGames:  int(stats.TotalGames),
		Wins:        wins,
		PlayerCount: playerCount,
		Draws:       int(stats.Draws),
		AvgTurns:    float64(stats.AvgTurns),
		Errors:      int(stats.Errors),
		// Bluffing metrics
		TotalClaims:       int(stats.TotalClaims),
		TotalBluffs:       int(stats.TotalBluffs),
		TotalChallenges:   int(stats.TotalChallenges),
		SuccessfulBluffs:  int(stats.SuccessfulBluffs),
		SuccessfulCatches: int(stats.SuccessfulCatches),
		// Betting metrics
		TotalBets:    int(stats.TotalBets),
		AllInCount:   int(stats.AllInCount),
		ShowdownWins: int(stats.ShowdownWins),
		FoldWins:     int(stats.FoldWins),
	}
}

// EvaluateIndividuals evaluates a slice of individuals in parallel.
// Returns the same individuals with fitness scores updated.
func (pe *ParallelEvaluator) EvaluateIndividuals(
	individuals []*Individual,
	numSimulations int,
	useMCTS bool,
) {
	if len(individuals) == 0 {
		return
	}

	// Extract genomes
	genomes := make([]*genome.GameGenome, len(individuals))
	for i, ind := range individuals {
		genomes[i] = ind.Genome
	}

	// Evaluate in parallel
	metrics := pe.EvaluatePopulation(genomes, numSimulations, useMCTS)

	// Update individuals
	for i, m := range metrics {
		individuals[i].Fitness = m.TotalFitness
		individuals[i].FitnessMetrics = m
		individuals[i].Evaluated = true
	}
}

// Close releases any resources (no-op for goroutine-based implementation).
func (pe *ParallelEvaluator) Close() {
	// No resources to release with goroutine pool
}

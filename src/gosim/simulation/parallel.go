package simulation

import (
	"math/rand"
	"runtime"
	"sync"

	"github.com/signalnine/darwindeck/gosim/engine"
)

// GameJob represents a single simulation job
type GameJob struct {
	SimID int
	Seed  uint64
}

// RunBatchParallelN executes batch simulations using a specified number of workers.
// Use this when running under Python multiprocessing to avoid thread over-subscription.
func RunBatchParallelN(genome *engine.Genome, numGames int, aiType AIPlayerType, mctsIterations int, seed uint64, numWorkers int) AggregatedStats {
	if numWorkers <= 0 {
		numWorkers = runtime.NumCPU()
	}
	runtime.GOMAXPROCS(numWorkers)

	jobs := make(chan GameJob, numGames)
	results := make(chan GameResult, numGames)

	var wg sync.WaitGroup

	// Start workers
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go worker(&wg, jobs, results, genome, aiType, mctsIterations)
	}

	// Use seed for deterministic game seeds
	rng := rand.New(rand.NewSource(int64(seed)))

	// Queue all simulation jobs with deterministic seeds
	for i := 0; i < numGames; i++ {
		gameSeed := rng.Uint64()
		jobs <- GameJob{
			SimID: i,
			Seed:  gameSeed,
		}
	}
	close(jobs)

	// Wait for all workers to complete, then close results
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect and aggregate results
	return aggregateParallelResults(results, numGames)
}

// RunBatchParallel executes batch simulations using worker pool
// Achieves ~4x speedup on 4-core systems
func RunBatchParallel(genome *engine.Genome, numGames int, aiType AIPlayerType, mctsIterations int, seed uint64) AggregatedStats {
	numWorkers := runtime.NumCPU()
	runtime.GOMAXPROCS(numWorkers)

	jobs := make(chan GameJob, numGames)
	results := make(chan GameResult, numGames)

	var wg sync.WaitGroup

	// Start workers
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go worker(&wg, jobs, results, genome, aiType, mctsIterations)
	}

	// Use seed for deterministic game seeds (same as serial version)
	// We need to generate all seeds up front to match serial execution
	rng := rand.New(rand.NewSource(int64(seed)))

	// Queue all simulation jobs with deterministic seeds
	for i := 0; i < numGames; i++ {
		gameSeed := rng.Uint64()
		jobs <- GameJob{
			SimID: i,
			Seed:  gameSeed,
		}
	}
	close(jobs)

	// Wait for all workers to complete, then close results
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect and aggregate results
	return aggregateParallelResults(results, numGames)
}

// worker processes simulation jobs from the jobs channel
func worker(wg *sync.WaitGroup, jobs <-chan GameJob, results chan<- GameResult, genome *engine.Genome, aiType AIPlayerType, mctsIterations int) {
	defer wg.Done()

	for job := range jobs {
		result := RunSingleGame(genome, aiType, mctsIterations, job.Seed)
		results <- result
	}
}

// aggregateParallelResults collects all results and computes aggregate statistics
func aggregateParallelResults(results <-chan GameResult, numGames int) AggregatedStats {
	allResults := make([]GameResult, 0, numGames)

	for result := range results {
		allResults = append(allResults, result)
	}

	// Reuse existing aggregation logic
	return aggregateResults(allResults)
}

// RunBatchAsymmetricParallelN executes asymmetric batch simulations with specified workers.
// Use this when running under Python multiprocessing to avoid thread over-subscription.
func RunBatchAsymmetricParallelN(genome *engine.Genome, numGames int, p0AIType AIPlayerType, p1AIType AIPlayerType, mctsIterations int, seed uint64, numWorkers int) AggregatedStats {
	if numWorkers <= 0 {
		numWorkers = runtime.NumCPU()
	}
	runtime.GOMAXPROCS(numWorkers)

	jobs := make(chan GameJob, numGames)
	results := make(chan GameResult, numGames)

	var wg sync.WaitGroup

	// Start workers
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go workerAsymmetric(&wg, jobs, results, genome, p0AIType, p1AIType, mctsIterations)
	}

	// Generate deterministic seeds
	rng := rand.New(rand.NewSource(int64(seed)))

	// Queue all simulation jobs
	for i := 0; i < numGames; i++ {
		gameSeed := rng.Uint64()
		jobs <- GameJob{
			SimID: i,
			Seed:  gameSeed,
		}
	}
	close(jobs)

	// Wait for all workers to complete, then close results
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect and aggregate results
	return aggregateParallelResults(results, numGames)
}

// RunBatchAsymmetricParallel executes asymmetric batch simulations using worker pool.
// Used for MCTS skill evaluation where different AI types play against each other.
func RunBatchAsymmetricParallel(genome *engine.Genome, numGames int, p0AIType AIPlayerType, p1AIType AIPlayerType, mctsIterations int, seed uint64) AggregatedStats {
	numWorkers := runtime.NumCPU()
	runtime.GOMAXPROCS(numWorkers)

	jobs := make(chan GameJob, numGames)
	results := make(chan GameResult, numGames)

	var wg sync.WaitGroup

	// Start workers
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go workerAsymmetric(&wg, jobs, results, genome, p0AIType, p1AIType, mctsIterations)
	}

	// Generate deterministic seeds
	rng := rand.New(rand.NewSource(int64(seed)))

	// Queue all simulation jobs
	for i := 0; i < numGames; i++ {
		gameSeed := rng.Uint64()
		jobs <- GameJob{
			SimID: i,
			Seed:  gameSeed,
		}
	}
	close(jobs)

	// Wait for all workers to complete, then close results
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect and aggregate results
	return aggregateParallelResults(results, numGames)
}

// workerAsymmetric processes asymmetric simulation jobs
func workerAsymmetric(wg *sync.WaitGroup, jobs <-chan GameJob, results chan<- GameResult, genome *engine.Genome, p0AIType AIPlayerType, p1AIType AIPlayerType, mctsIterations int) {
	defer wg.Done()

	for job := range jobs {
		result := RunSingleGameAsymmetric(genome, p0AIType, p1AIType, mctsIterations, job.Seed)
		results <- result
	}
}

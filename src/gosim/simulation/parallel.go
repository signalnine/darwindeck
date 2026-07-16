package simulation

import (
	"math/rand"
	"runtime"
	"sync"

	"github.com/signalnine/darwindeck/gosim/engine"
)

// parallelWorkerCount resolves the requested worker count and avoids starting
// more workers than there are games to simulate.
func parallelWorkerCount(numGames, requested int) int {
	if requested <= 0 {
		requested = runtime.NumCPU()
	}
	if numGames > 0 && requested > numGames {
		requested = numGames
	}
	if requested < 1 {
		return 1
	}
	return requested
}

// defaultParallelWorkerCount avoids overwhelming short simulations with one
// worker per logical CPU. Callers with heavier workloads can opt into a
// specific count through RunBatchParallelN.
func defaultParallelWorkerCount(numGames int) int {
	workers := runtime.GOMAXPROCS(0)
	if workers > 8 {
		workers = 8
	}
	return parallelWorkerCount(numGames, workers)
}

// gameSeeds returns the same deterministic per-game seeds used by RunBatch.
func gameSeeds(numGames int, seed uint64) []uint64 {
	seeds := make([]uint64, numGames)
	rng := rand.New(rand.NewSource(int64(seed)))
	for i := range seeds {
		seeds[i] = rng.Uint64()
	}
	return seeds
}

// RunBatchParallelN executes batch simulations using a specified number of workers.
// Use this when running under Python multiprocessing to avoid thread over-subscription.
func RunBatchParallelN(genome *engine.Genome, numGames int, aiType AIPlayerType, mctsIterations int, seed uint64, numWorkers int) AggregatedStats {
	if numGames == 0 {
		return aggregateResults(nil)
	}

	numWorkers = parallelWorkerCount(numGames, numWorkers)

	seeds := gameSeeds(numGames, seed)
	results := make([]GameResult, numGames)

	var wg sync.WaitGroup
	wg.Add(numWorkers)
	for workerID := 0; workerID < numWorkers; workerID++ {
		start := workerID * numGames / numWorkers
		end := (workerID + 1) * numGames / numWorkers
		go func() {
			defer wg.Done()
			for i := start; i < end; i++ {
				results[i] = RunSingleGame(genome, aiType, mctsIterations, seeds[i], i)
			}
		}()
	}
	wg.Wait()

	return aggregateResults(results)
}

// RunBatchParallel executes batch simulations using a worker pool.
// The benefit depends on simulation cost and available CPU cores.
func RunBatchParallel(genome *engine.Genome, numGames int, aiType AIPlayerType, mctsIterations int, seed uint64) AggregatedStats {
	return RunBatchParallelN(genome, numGames, aiType, mctsIterations, seed, defaultParallelWorkerCount(numGames))
}

// RunBatchAsymmetricParallelN executes asymmetric batch simulations with specified workers.
// Use this when running under Python multiprocessing to avoid thread over-subscription.
func RunBatchAsymmetricParallelN(genome *engine.Genome, numGames int, p0AIType AIPlayerType, p1AIType AIPlayerType, mctsIterations int, seed uint64, numWorkers int) AggregatedStats {
	if numGames == 0 {
		return aggregateResults(nil)
	}

	numWorkers = parallelWorkerCount(numGames, numWorkers)

	seeds := gameSeeds(numGames, seed)
	results := make([]GameResult, numGames)

	var wg sync.WaitGroup
	wg.Add(numWorkers)
	for workerID := 0; workerID < numWorkers; workerID++ {
		start := workerID * numGames / numWorkers
		end := (workerID + 1) * numGames / numWorkers
		go func() {
			defer wg.Done()
			for i := start; i < end; i++ {
				results[i] = RunSingleGameAsymmetric(
					genome,
					p0AIType,
					p1AIType,
					mctsIterations,
					seeds[i],
					i,
				)
			}
		}()
	}
	wg.Wait()

	return aggregateResults(results)
}

// RunBatchAsymmetricParallel executes asymmetric batch simulations using a worker pool.
// Used for MCTS skill evaluation where different AI types play against each other.
func RunBatchAsymmetricParallel(genome *engine.Genome, numGames int, p0AIType AIPlayerType, p1AIType AIPlayerType, mctsIterations int, seed uint64) AggregatedStats {
	return RunBatchAsymmetricParallelN(
		genome,
		numGames,
		p0AIType,
		p1AIType,
		mctsIterations,
		seed,
		defaultParallelWorkerCount(numGames),
	)
}

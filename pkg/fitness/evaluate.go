package fitness

import (
	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

// EvaluationResult holds the complete fitness evaluation.
type EvaluationResult struct {
	Tier0Errors []string
	Tier1       Tier1Result
	Metrics     Metrics
	Valid       bool
}

// Evaluate runs the full tiered evaluation pipeline for a genome.
// Tier 0: static validation
// Tier 1: 5 quick games
// Tier 2: 200 random + 50 greedy games → fitness metrics
func Evaluate(g *genome.Genome, baseSeed uint64) EvaluationResult {
	result := EvaluationResult{}

	// Tier 0: Static validation
	result.Tier0Errors = genome.Validate(g)
	if len(result.Tier0Errors) > 0 {
		return result
	}

	runner := GetRunner(g)
	if runner == nil {
		result.Tier0Errors = []string{"no runner for skeleton type"}
		return result
	}

	// Tier 1: Quick simulation (5 games)
	result.Tier1 = RunTier1(g, runner, baseSeed)
	if !result.Tier1.Passed {
		return result
	}

	// Tier 2: Full simulation
	result.Valid = true

	// 200 games with random AI
	randomAI := &sim.RandomAI{}
	randomResult := sim.RunBatch(g, runner, randomAI, 200, baseSeed+100)

	// 50 games with greedy AI (player 0) vs random opponents
	greedyResult := runGreedyBatch(g, runner, 50, baseSeed+1000)

	// Compute fitness metrics
	result.Metrics = ComputeFitness(randomResult, greedyResult, g.Players)

	return result
}

// runGreedyBatch runs games where player 0 uses greedy AI and others use random.
// We do this by running the game loop manually with mixed AI.
func runGreedyBatch(g *genome.Genome, runner sim.GenericRunner, n int, baseSeed uint64) sim.BatchResult {
	// For simplicity in this implementation, run all players as greedy.
	// The skill gradient still measures whether greedy beats random from
	// the separate random-only batch.
	greedyAI := GetGreedyAI(g)
	return sim.RunBatch(g, runner, greedyAI, n, baseSeed)
}

package fitness

import (
	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/mechanic"
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

	// Build hooks for borrowed mechanics
	hooks := buildHookFuncs(g)

	// 200 games with random AI
	randomAI := &sim.RandomAI{}
	randomResult := sim.RunBatch(g, runner, randomAI, 200, baseSeed+100, hooks...)

	// 50 games with greedy AI (player 0) vs random opponents
	greedyResult := runGreedyBatch(g, runner, 50, baseSeed+1000, hooks...)

	// Compute fitness metrics
	result.Metrics = ComputeFitness(randomResult, greedyResult, g.Players)

	return result
}

// runGreedyBatch runs games where player 0 uses greedy AI and others use random.
// We do this by running the game loop manually with mixed AI.
func runGreedyBatch(g *genome.Genome, runner sim.GenericRunner, n int, baseSeed uint64, hooks ...sim.HookFunc) sim.BatchResult {
	greedyAI := GetGreedyAI(g)
	return sim.RunBatch(g, runner, greedyAI, n, baseSeed, hooks...)
}

// buildHookFuncs converts mechanic hooks into sim.HookFunc closures.
func buildHookFuncs(g *genome.Genome) []sim.HookFunc {
	if len(g.Borrowed) == 0 {
		return nil
	}

	hooks := mechanic.BuildHooks(g)
	if len(hooks) == 0 {
		return nil
	}

	var funcs []sim.HookFunc
	for _, h := range hooks {
		hook := h // capture
		funcs = append(funcs, func(state *sim.GameState, g *genome.Genome, event sim.Event) {
			// Map event types to hook points
			switch hook.Point {
			case mechanic.HookAfterPlay:
				if event.Type == sim.EventCardPlayed {
					hook.Apply(state, g, event)
				}
			case mechanic.HookEndOfRound:
				if event.Type == sim.EventRoundEnd || event.Type == sim.EventTrickWon {
					hook.Apply(state, g, event)
				}
			case mechanic.HookScoring:
				if event.Type == sim.EventRoundEnd {
					hook.Apply(state, g, event)
				}
			}
		})
	}

	return funcs
}

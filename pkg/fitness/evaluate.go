package fitness

import (
	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/mechanic"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

// Tier 2 sample sizes. Greedy was raised from 50 to 200 (Task 13.2): at 50
// games the standard error on the seat-0 win rate is ~0.07, which drowns the
// skill-gradient signal; 200 games cut it to ~0.035.
const (
	tier2RandomGames = 200
	tier2GreedyGames = 200
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
// Tier 2: 200 random + 200 greedy games → fitness metrics
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

	// Games with random AI
	randomAI := &sim.RandomAI{}
	randomResult := sim.RunBatch(g, runner, randomAI, tier2RandomGames, baseSeed+100, hooks...)

	// Games with greedy AI (player 0) vs random opponents
	greedyResult := runGreedyBatch(g, runner, tier2GreedyGames, baseSeed+1000, hooks...)

	// Compute fitness metrics
	result.Metrics = ComputeFitness(randomResult, greedyResult, g.Players)

	return result
}

// runGreedyBatch runs games where player 0 uses greedy AI and others use
// random. computeSkillGradient reads player 0's win rate, so mixing AIs per
// seat is the only way the metric can distinguish skill from symmetry.
func runGreedyBatch(g *genome.Genome, runner sim.GenericRunner, n int, baseSeed uint64, hooks ...sim.HookFunc) sim.BatchResult {
	players := make([]sim.AIPlayer, g.Players)
	players[0] = GetGreedyAI(g)
	random := &sim.RandomAI{}
	for i := 1; i < g.Players; i++ {
		players[i] = random
	}
	ai := &sim.PerPlayerAI{Players: players, Fallback: random}
	return sim.RunBatch(g, runner, ai, n, baseSeed, hooks...)
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
				if event.Type == sim.EventRoundEnd {
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

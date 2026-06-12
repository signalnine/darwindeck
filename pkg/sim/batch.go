package sim

import (
	"math/rand/v2"

	"github.com/darwindeck/darwindeck/pkg/genome"
)

// BatchResult holds aggregated results from running multiple games.
type BatchResult struct {
	GamesPlayed int
	Completions int   // Games that ended with a winner
	Errors      int   // Games that errored
	Timeouts    int   // Games that hit max turns
	WinCounts   []int // Wins per player
	TotalTurns  int   // Sum of all turns
	MinTurns    int
	MaxTurns    int
	AvgTurns    float64
	TurnsList   []int     // All turn counts for distribution analysis
	AllEvents   [][]Event // Events from each game (for fitness analysis)
}

// GenericRunner is the interface all skeleton runners implement.
// The genome is passed to each method so the runner can read parameters.
type GenericRunner interface {
	Setup(g *genome.Genome, rng *rand.Rand) *GameState
	// Upkeep performs start-of-turn state maintenance (deck recycling,
	// round transitions/redeals). It is the ONLY method besides ApplyMove
	// allowed to mutate state. Game loops must call it exactly once at the
	// top of each iteration, before CheckEnd.
	Upkeep(state *GameState, g *genome.Genome)
	GenerateMoves(state *GameState, g *genome.Genome) []Move // must be pure
	ApplyMove(state *GameState, move Move, g *genome.Genome) []Event
	CheckEnd(state *GameState, g *genome.Genome) int // must be pure
}

// HookFunc is called after each move with the resulting events.
type HookFunc func(state *GameState, g *genome.Genome, event Event)

// RunBatch plays n games with the given genome, runner, and AI.
// Returns aggregated statistics.
func RunBatch(g *genome.Genome, runner GenericRunner, ai AIPlayer, n int, baseSeed uint64, hooks ...HookFunc) BatchResult {
	result := BatchResult{
		GamesPlayed: n,
		WinCounts:   make([]int, g.Players),
		TurnsList:   make([]int, 0, n),
	}

	maxTurns := g.MaxTurns()

	for i := 0; i < n; i++ {
		rng := rand.New(rand.NewPCG(baseSeed+uint64(i), 0))
		gr := runSingleGame(g, runner, ai, rng, maxTurns, hooks...)

		result.TurnsList = append(result.TurnsList, gr.Turns)
		result.TotalTurns += gr.Turns

		if i == 0 || gr.Turns < result.MinTurns {
			result.MinTurns = gr.Turns
		}
		if gr.Turns > result.MaxTurns {
			result.MaxTurns = gr.Turns
		}

		if gr.Error != "" {
			if gr.Error == "max_turns" {
				result.Timeouts++
			} else {
				result.Errors++
			}
		} else if gr.Winner >= 0 && gr.Winner < g.Players {
			result.Completions++
			result.WinCounts[gr.Winner]++
		}

		result.AllEvents = append(result.AllEvents, gr.Events)
	}

	if n > 0 {
		result.AvgTurns = float64(result.TotalTurns) / float64(n)
	}

	return result
}

func runSingleGame(g *genome.Genome, runner GenericRunner, ai AIPlayer, rng *rand.Rand, maxTurns int, hooks ...HookFunc) GameResult {
	state := runner.Setup(g, rng)

	// Cap inner iterations independently of maxTurns: some skeletons (rummy)
	// run multiple ApplyMove calls per turn (draw, meld, discard) and a buggy
	// runner/genome combo can stall in a phase without ever bumping Turn.
	// Without this guard the game loop runs forever and EvaluatePopulation
	// hangs across all workers (see dd-505).
	iterCap := (maxTurns + 1) * 100
	if iterCap < 10000 {
		iterCap = 10000
	}
	iter := 0

	for {
		runner.Upkeep(state, g)

		winner := runner.CheckEnd(state, g)
		if winner >= 0 {
			return GameResult{
				Winner: winner,
				Turns:  state.Turn,
				Events: state.Events,
			}
		}

		if state.Turn >= maxTurns {
			return GameResult{
				Winner: -1,
				Turns:  state.Turn,
				Events: state.Events,
				Error:  "max_turns",
			}
		}

		if iter >= iterCap {
			return GameResult{
				Winner: -1,
				Turns:  state.Turn,
				Events: state.Events,
				Error:  "stuck",
			}
		}
		iter++

		moves := runner.GenerateMoves(state, g)
		if len(moves) == 0 {
			return GameResult{
				Winner: -1,
				Turns:  state.Turn,
				Events: state.Events,
				Error:  "no_moves",
			}
		}

		move := ai.SelectMove(moves, state, rng)
		events := runner.ApplyMove(state, move, g)
		state.Events = append(state.Events, events...)

		// Run hooks after each move
		for _, event := range events {
			for _, hook := range hooks {
				hook(state, g, event)
			}
		}
	}
}

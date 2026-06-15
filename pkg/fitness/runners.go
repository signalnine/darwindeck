package fitness

import (
	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/sim"
	"github.com/darwindeck/darwindeck/pkg/skeleton/casino"
	"github.com/darwindeck/darwindeck/pkg/skeleton/climbing"
	"github.com/darwindeck/darwindeck/pkg/skeleton/rummy"
	"github.com/darwindeck/darwindeck/pkg/skeleton/shedding"
	"github.com/darwindeck/darwindeck/pkg/skeleton/tricktaking"
	"github.com/darwindeck/darwindeck/pkg/skeleton/vying"
)

// GetRunner returns the appropriate skeleton runner for a genome.
func GetRunner(g *genome.Genome) sim.GenericRunner {
	switch g.Skeleton {
	case genome.Shedding:
		return &shedding.Runner{}
	case genome.TrickTaking:
		return &tricktaking.Runner{}
	case genome.Rummy:
		return &rummy.Runner{}
	case genome.Climbing:
		return &climbing.Runner{}
	case genome.Casino:
		return &casino.Runner{}
	case genome.Vying:
		return &vying.Runner{}
	default:
		return nil
	}
}

// GetGreedyAI returns a greedy AI configured for the genome's skeleton.
func GetGreedyAI(g *genome.Genome) sim.AIPlayer {
	switch g.Skeleton {
	case genome.Shedding:
		return &sim.GreedyAI{Scorer: sim.NewSheddingScorer(g)}
	case genome.TrickTaking:
		avoidance := g.TrickTaking != nil && g.TrickTaking.TrickScoring == genome.ScoreAvoidance
		return &sim.GreedyAI{Scorer: &sim.TrickTakingScorer{
			Avoidance: avoidance,
		}}
	case genome.Rummy:
		return &sim.GreedyAI{Scorer: &sim.RummyScorer{}}
	case genome.Climbing:
		return &sim.GreedyAI{Scorer: &sim.ClimbingScorer{}}
	case genome.Casino:
		return &sim.GreedyAI{Scorer: &sim.CasinoScorer{}}
	case genome.Vying:
		return &sim.GreedyAI{Scorer: &vying.VyingScorer{}}
	default:
		return &sim.RandomAI{}
	}
}

package fitness

import (
	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/sim"
	"github.com/darwindeck/darwindeck/pkg/skeleton/rummy"
	"github.com/darwindeck/darwindeck/pkg/skeleton/shedding"
	"github.com/darwindeck/darwindeck/pkg/skeleton/tricktaking"
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
	default:
		return nil
	}
}

// GetGreedyAI returns a greedy AI configured for the genome's skeleton.
func GetGreedyAI(g *genome.Genome) sim.AIPlayer {
	switch g.Skeleton {
	case genome.Shedding:
		return &sim.GreedyAI{Scorer: &sim.SheddingScorer{}}
	case genome.TrickTaking:
		avoidance := g.TrickTaking != nil && g.TrickTaking.TrickScoring == genome.ScoreAvoidance
		trumpSuit := -1
		if g.TrumpRule == genome.TrumpFixed {
			trumpSuit = int(g.Scoring.TrumpSuit) - 1
		}
		return &sim.GreedyAI{Scorer: &sim.TrickTakingScorer{
			Avoidance: avoidance,
			TrumpSuit: trumpSuit,
		}}
	case genome.Rummy:
		return &sim.GreedyAI{Scorer: &sim.RummyScorer{}}
	default:
		return &sim.RandomAI{}
	}
}

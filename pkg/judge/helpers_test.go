package judge

import (
	"testing"

	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

func mustRunner(t *testing.T, g *genome.Genome) sim.GenericRunner {
	t.Helper()
	r := fitness.GetRunner(g)
	if r == nil {
		t.Fatalf("no runner for %v", g.Skeleton)
	}
	return r
}

func mustAI(g *genome.Genome) sim.AIPlayer {
	return fitness.GetGreedyAI(g)
}

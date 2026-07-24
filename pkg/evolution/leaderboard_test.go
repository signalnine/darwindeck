package evolution

import (
	"testing"

	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/genome"
)

// Commensurable-leaderboard tests (Wave K fix 1). The round-3 designer
// review caught the top-N sort mixing MCTS-mode published means (top-decile
// genomes, +0.085..+0.145 uplift) with greedy-only means (everyone else) in
// ONE ranking -- the MCTS-grant boundary WAS the top-10 boundary. The
// leaderboard must rank every individual by the greedy-only running mean
// (the number every genome has, measured the same way); the MCTS-mode mean
// is reported separately and never ranked.

// mkEvaluated builds an individual with explicit running-mean accumulators,
// the shape every valid individual has after the evaluation pass.
func mkEvaluated(g *genome.Genome, greedySum float64, greedyN int, mctsSum float64, mctsN int) *Individual {
	ind := &Individual{
		Genome:     g,
		Valid:      true,
		EvalCount:  greedyN,
		FitnessSum: greedySum,
		MctsSum:    mctsSum,
		MctsCount:  mctsN,
	}
	ind.Fitness = fitness.Metrics{TotalFitness: ind.publishedFitness(), SharedFitness: ind.publishedFitness()}
	ind.Genome.Fitness = ind.Fitness.TotalFitness
	return ind
}

func TestOutputRankIsGreedyOnlyMean(t *testing.T) {
	ind := mkEvaluated(distinctShedding(0), 3.90, 5, 0.92, 1)
	if got := ind.OutputRank(); got != 0.78 {
		t.Errorf("OutputRank = %v, want the greedy-only mean 0.78 (NOT the 0.92 MCTS mean)", got)
	}

	// Fixture fallback: with no running-mean history the published
	// TotalFitness stands in (hand-built individuals; production always has
	// EvalCount >= 1 when Valid).
	plain := mkInd(distinctShedding(1), 0.55)
	if got := plain.OutputRank(); got != 0.55 {
		t.Errorf("OutputRank fallback = %v, want 0.55", got)
	}
}

// TestTopNRanksByGreedyOnlyMean: an MCTS-granted individual with a high
// published mean but a LOWER greedy mean must rank below an ungranted
// individual with a higher greedy mean. Pre-fix, the published (MCTS-mode)
// 0.92 would have outranked the greedy-only 0.80.
func TestTopNRanksByGreedyOnlyMean(t *testing.T) {
	granted := mkEvaluated(distinctShedding(0), 3.90, 5, 0.92, 1) // greedy 0.78, published 0.92
	ungranted := mkEvaluated(distinctShedding(1), 4.00, 5, 0, 0)  // greedy 0.80, published 0.80
	third := mkEvaluated(distinctShedding(2), 3.50, 5, 0, 0)      // greedy 0.70

	e := &Engine{Population: []*Individual{granted, ungranted, third}}
	top := e.TopN(3)
	if len(top) != 3 {
		t.Fatalf("TopN(3) returned %d", len(top))
	}
	wantOrder := []string{"distinct-1", "distinct-0", "distinct-2"}
	for i, want := range wantOrder {
		if top[i].Genome.ID != want {
			t.Errorf("top[%d] = %s (rank %.3f), want %s -- leaderboard must order by greedy-only mean",
				i, top[i].Genome.ID, top[i].OutputRank(), want)
		}
	}
}

// TestAllQualifiedKeepsGreedyBestCloneMember: when a clone group collapses,
// the kept member is the best by the COMMENSURABLE key, not by the published
// (possibly MCTS-inflated) mean.
func TestAllQualifiedKeepsGreedyBestCloneMember(t *testing.T) {
	base := distinctShedding(0)
	mkNov := func(id string, greedySum float64, greedyN int, mctsSum float64, mctsN int) *NoveltyIndividual {
		return &NoveltyIndividual{Individual: *mkEvaluated(cloneWithID(base, id), greedySum, greedyN, mctsSum, mctsN)}
	}
	e := &NoveltyEngine{
		Population: []*NoveltyIndividual{
			mkNov("granted", 3.50, 5, 0.95, 1), // greedy 0.70, published 0.95
			mkNov("ungranted", 4.00, 5, 0, 0),  // greedy 0.80, published 0.80
		},
	}

	inds, _ := e.AllQualified()
	if len(inds) != 1 {
		t.Fatalf("clone group must collapse to 1, got %d", len(inds))
	}
	if inds[0].Genome.ID != "ungranted" {
		t.Errorf("kept %s, want the greedy-best member (ungranted, 0.80 > 0.70)", inds[0].Genome.ID)
	}
}

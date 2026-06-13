package main

import (
	"testing"

	"github.com/darwindeck/darwindeck/pkg/evolution"
	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/genome"
)

// sortAndTrim must order by the greedy-only running mean (OutputRank), the
// commensurable leaderboard key -- not by the published fitness, which is an
// MCTS-mode mean for decile-granted individuals only (Wave K fix 1: the
// MCTS-grant boundary WAS the flagship-r3 top-10 boundary).
func TestSortAndTrimRanksByGreedyOnlyMean(t *testing.T) {
	mk := func(id string, handSize int, greedySum float64, greedyN int, mctsSum float64, mctsN int) *evolution.Individual {
		ind := &evolution.Individual{
			Genome: &genome.Genome{
				ID:       id,
				Skeleton: genome.Shedding,
				Players:  2,
				HandSize: handSize,
				Shedding: &genome.SheddingParams{MatchRule: genome.MatchEither, DrawPenalty: 1},
			},
			Valid:      true,
			EvalCount:  greedyN,
			FitnessSum: greedySum,
			MctsSum:    mctsSum,
			MctsCount:  mctsN,
		}
		published := ind.OutputRank()
		if mctsN > 0 {
			published, _ = ind.MCTSMean()
		}
		ind.Fitness = fitness.Metrics{TotalFitness: published, SharedFitness: published}
		return ind
	}

	granted := mk("granted", 5, 3.90, 5, 0.92, 1)   // greedy 0.78, published 0.92
	ungranted := mk("ungranted", 6, 4.00, 5, 0, 0)  // greedy 0.80
	third := mk("third", 7, 3.50, 5, 0, 0)          // greedy 0.70

	top := sortAndTrim([]*evolution.Individual{granted, third, ungranted}, 3)
	if len(top) != 3 {
		t.Fatalf("sortAndTrim returned %d", len(top))
	}
	for i, want := range []string{"ungranted", "granted", "third"} {
		if top[i].Genome.ID != want {
			t.Errorf("top[%d] = %s, want %s (greedy-only ordering)", i, top[i].Genome.ID, want)
		}
	}
}

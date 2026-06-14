package seeds

import "github.com/darwindeck/darwindeck/pkg/genome"

// BigTwo returns the climbing-skeleton seed genome -- a sensible Big-Two-like
// default. It is EVOLUTION SEED MATERIAL and a PLAYABILITY REFERENCE for the
// climbing runner; it is deliberately NOT part of the calibration ground-truth
// (seeds.All()), because the project has no human fun-rating for a climbing game
// to calibrate against.
//
// Big Two: 4 players, 13-card hands, singles + pairs + triples + runs (length
// >= 3). You beat the current combination with a same-type, higher combination
// or pass; when everyone else passes the leader leads fresh. First to empty
// their hand wins.
func BigTwo() *genome.Genome {
	return &genome.Genome{
		ID:       "big-two",
		Skeleton: genome.Climbing,
		Players:  4,
		HandSize: 13,
		Climbing: &genome.ClimbingParams{
			AllowPairs:   true,
			AllowTriples: true,
			AllowRuns:    true,
			MinRunLen:    3,
		},
	}
}

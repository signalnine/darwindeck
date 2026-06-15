package seeds

import "github.com/darwindeck/darwindeck/pkg/genome"

// BigTwo returns the climbing-skeleton seed genome -- a sensible Big-Two-like
// default. It is a calibration ground-truth seed (in seeds.All()), the climbing
// playability reference, and evolution seed material.
//
// HISTORY: Big Two was once excluded from seeds.All() on the stated grounds of
// "no human fun-rating for climbing." That was wrong on two counts: Big Two is a
// hugely popular real game (a game still in circulation is fun by survival, the
// same implicit ground truth the other classics rest on), and the actual blocker
// was a MEASUREMENT artifact -- the Interaction metric was blind to climbing, so
// Big Two scored interact=0.000 and TotalFitness ~0.401 (a hair above the floor)
// despite passing every degeneracy veto. Extending the metric to measure
// climbing's beat/pass constraint (deltaModeClimbing in pkg/sim/batch.go) raised
// it to interact~0.76 / TotalFitness~0.55, on par with Gin Rummy, and it then
// passed the full calibration gate -- so it was promoted into seeds.All().
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

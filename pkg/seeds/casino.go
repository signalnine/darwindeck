package seeds

import "github.com/darwindeck/darwindeck/pkg/genome"

// Casino returns the casino-skeleton seed genome: a simplified 2-player Casino
// (no builds, no point scoring for cards/spades/aces/sweeps -- most captured
// cards wins, closer to Scopa-without-scope than to tournament Casino rules).
// Deal 4 to each hand and 4 face-up to the table; capture by rank-match or by
// summing number cards to your played card, else trail; refill hands from the
// stock until it runs out. A recognizable member of a real, time-tested
// published family, so it anchors the fishing skeleton in the fun ground truth
// (popular-and-surviving = fun) provided the metrics can measure its play.
func Casino() *genome.Genome {
	return &genome.Genome{
		ID:       "casino",
		Skeleton: genome.Casino,
		Players:  2,
		HandSize: 4,
		Casino: &genome.CasinoParams{
			TableSize:       4,
			AllowSumCapture: true,
		},
	}
}

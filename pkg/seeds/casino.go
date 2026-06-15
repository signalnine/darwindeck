package seeds

import "github.com/darwindeck/darwindeck/pkg/genome"

// Casino returns the casino-skeleton seed genome: standard 2-player Casino.
// Deal 4 to each hand and 4 face-up to the table; capture by rank-match or by
// summing number cards to your played card, else trail; refill hands from the
// stock until it runs out; most captured cards wins. A real, time-tested
// published game, so it belongs in the fun ground truth (popular-and-surviving =
// fun) provided the metrics can measure its play.
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

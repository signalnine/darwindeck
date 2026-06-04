package seeds

import "github.com/darwindeck/darwindeck/pkg/genome"

// GinRummy returns the Gin Rummy seed genome.
// Sets and runs, draw from deck or discard, knock at 10 deadwood or gin.
func GinRummy() *genome.Genome {
	return &genome.Genome{
		ID:       "gin-rummy",
		Skeleton: genome.Rummy,
		Players:  2,
		HandSize: 10,
		Rummy: &genome.RummyParams{
			MeldTypes:      genome.MeldBoth,
			MinMeldSize:    3,
			DrawFrom:       genome.DrawEither,
			KnockThreshold: 10,
		},
	}
}

// KnockRummy returns the Knock Rummy seed genome.
// Simpler than Gin — everyone pays deadwood, no laying off.
func KnockRummy() *genome.Genome {
	return &genome.Genome{
		ID:       "knock-rummy",
		Skeleton: genome.Rummy,
		Players:  3,
		HandSize: 7,
		Rummy: &genome.RummyParams{
			MeldTypes:      genome.MeldBoth,
			MinMeldSize:    3,
			DrawFrom:       genome.DrawEither,
			KnockThreshold: 15,
		},
	}
}

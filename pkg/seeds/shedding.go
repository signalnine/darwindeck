package seeds

import "github.com/darwindeck/darwindeck/pkg/genome"

// CrazyEights returns the Crazy Eights seed genome.
// Match suit or rank, draw 1 on miss, 8s are wild.
func CrazyEights() *genome.Genome {
	return &genome.Genome{
		ID:       "crazy-eights",
		Skeleton: genome.Shedding,
		Players:  2,
		HandSize: 7,
		Shedding: &genome.SheddingParams{
			MatchRule:   genome.MatchEither,
			DrawPenalty: 1,
		},
		SpecialCards: []genome.SpecialCard{
			{Type: genome.SpecialWild, ByRank: uint8(8)}, // 8s are wild
		},
	}
}

// MauMau returns the Mau-Mau seed genome.
// Like Crazy Eights but with special card effects.
func MauMau() *genome.Genome {
	return &genome.Genome{
		ID:       "mau-mau",
		Skeleton: genome.Shedding,
		Players:  3,
		HandSize: 5,
		Shedding: &genome.SheddingParams{
			MatchRule:   genome.MatchEither,
			DrawPenalty: 1,
		},
		SpecialCards: []genome.SpecialCard{
			{Type: genome.SpecialWild, ByRank: uint8(8)},        // 8s are wild
			{Type: genome.SpecialSkip, ByRank: uint8(7)},        // 7s skip
			{Type: genome.SpecialDrawTwo, ByRank: uint8(2)},     // 2s draw two
			{Type: genome.SpecialReverse, ByRank: uint8(10)},    // 10s reverse
		},
	}
}

package seeds

import "github.com/darwindeck/darwindeck/pkg/genome"

// SimplePoker returns the vying-skeleton seed: a stripped single-street poker
// session. Each deal every player gets 5 hidden cards, one rotating player posts
// the big blind, a single fold/call/raise round runs (raises capped at 3), and
// the best poker hand among the non-folded players takes the pot. Chips carry
// across 12 deals and the largest stack wins. It is the minimal real game of the
// vying / betting family (wager on hidden hands, showdown by hand rank) -- the
// one decision axis the other five skeletons do not touch. StartingChips covers
// the worst-case commitment across all deals, so no all-in / side pot arises.
func SimplePoker() *genome.Genome {
	return &genome.Genome{
		ID:       "simple_poker",
		Skeleton: genome.Vying,
		Players:  4,
		HandSize: 5,
		Vying: &genome.VyingParams{
			StartingChips: 1000,
			MinBet:        10,
			MaxRaises:     3,
			RoundsPerGame: 12,
		},
	}
}

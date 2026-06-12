// Degenerate fixtures: known-bad genomes used as negative ground truth for
// fitness calibration (audit remediation Task 2). These must always score
// below every classic seed game; a fitness function that ranks one of these
// above a classic is falsified (see pkg/fitness/calibration_test.go).
//
// Per Task 28's failed-review loop, every degenerate champion rejected at
// designer review gets encoded here as a new fixture -- rejected champions
// are the most valuable ground truth the project can get.
package seeds

import "github.com/darwindeck/darwindeck/pkg/genome"

// All returns fresh copies of the 8 classic seed genomes -- the canonical
// human-validated "fun" registry used by the calibration suite. Degenerate
// fixtures are deliberately NOT included: they are negative ground truth.
func All() []*genome.Genome {
	return []*genome.Genome{
		CrazyEights(),
		MauMau(),
		Whist(),
		Hearts(),
		Spades(),
		OhHell(),
		GinRummy(),
		KnockRummy(),
	}
}

// InstantKnockRummy reproduces the degenerate flagship champion
// (rank01_gen200_70015): hand 3 with min meld size 4 makes melds unreachable,
// and knock threshold 27 makes knocking nearly always legal on the first
// turn -- the game is an instant coin flip with no meaningful decisions.
func InstantKnockRummy() *genome.Genome {
	return &genome.Genome{
		ID:       "instant-knock-rummy",
		Skeleton: genome.Rummy,
		Players:  2,
		HandSize: 3,
		Rummy: &genome.RummyParams{
			MeldTypes:      genome.MeldSets,
			MinMeldSize:    4,
			DrawFrom:       genome.DrawDiscard,
			KnockThreshold: 27,
		},
	}
}

// ForcedShedding is a minimal-agency shedding game: MatchEither with
// DrawPenalty 1, no special cards, and the smallest legal hand (3 -- Tier 0
// requires hand_size >= 3). Almost every turn has exactly one sensible line:
// play the single matching card or draw one.
func ForcedShedding() *genome.Genome {
	return &genome.Genome{
		ID:       "forced-shedding",
		Skeleton: genome.Shedding,
		Players:  2,
		HandSize: 3,
		Shedding: &genome.SheddingParams{
			MatchRule:   genome.MatchEither,
			DrawPenalty: 1,
		},
	}
}

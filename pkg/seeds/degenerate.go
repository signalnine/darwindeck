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

// --- Task 28 step 4 failed-review fixtures (round 2, 2026-06-12) ---
//
// The three constructors below are byte-faithful clones (IDs/fitness/
// generation stripped) of the post-fix flagship champions rejected at
// designer review (output/2026-06-12-flagship-postfix). Top 30 of that run
// collapsed to exactly these three gamed archetypes; per the failed-review
// loop they are now permanent negative ground truth.

// CatchAllSkipShedding reproduces archetype A1 (flagship ranks 1-10, cloned
// from rank01_gen200_97457): a 2-player, 13-card shedding game whose first
// special rule is a CATCH-ALL skip ({Type: SpecialSkip} with ByRank=0 and
// BySuit=0 matches every card in cardMatchesSpecial), with three of the four
// suits wild (39/52 cards always playable). In 2-player, skip == play-again,
// so one player plays until stuck while the opponent spectates (live
// playtest: 13 consecutive plays, opponent acted 0 times). Gamed metrics:
// interaction pinned 1.00 (every play emitted a "skip" attack event) and
// decisions 0.86-0.88 (legal-move COUNT inflated by wilds whose choice has
// near-zero impact).
func CatchAllSkipShedding() *genome.Genome {
	return &genome.Genome{
		ID:       "catch-all-skip-shedding",
		Skeleton: genome.Shedding,
		Players:  2,
		HandSize: 13,
		Shedding: &genome.SheddingParams{
			MatchRule:     genome.MatchEither,
			DrawPenalty:   2,
			RoundsPerGame: 1,
		},
		Borrowed: []genome.BorrowedMechanic{
			{Source: genome.TrickTaking, Mechanic: genome.MechAvoidance},
			{Source: genome.Rummy, Mechanic: genome.MechMeldBonus},
		},
		SpecialCards: []genome.SpecialCard{
			{Type: genome.SpecialSkip},                // catch-all: matches EVERY card
			{Type: genome.SpecialWild, BySuit: 2},     // suit 2 wild
			{Type: genome.SpecialWild, BySuit: 3},     // suit 3 wild
			{Type: genome.SpecialDrawTwo, BySuit: 3},  // suit 3 also draw-two
			{Type: genome.SpecialWild, BySuit: 4},     // suit 4 wild
			{Type: genome.SpecialDrawFour, ByRank: 7}, // 7s draw-four
		},
		Scoring: genome.ScoringConfig{
			CardPoints: []genome.CardScoring{
				{Rank: 0, Suit: 2, Points: 6, Event: genome.ScoreOnTrickWin},
			},
		},
	}
}

// NoFollowAvoidanceTrick reproduces archetype A2 (flagship ranks 11-20,
// cloned from rank11_gen200_23872): a 2-player, 12-card trick-taking game
// with no follow-suit constraint, flat avoidance scoring (every card worth 6
// penalty points via the catch-all card_points rule), and winner-leads. With
// no trump and no follow requirement, an off-suit card can never win a trick
// (resolveTrick), so the follower always ducks; perfect play reduces to seat
// parity. Gamed metric: decisions 0.92 -- no follow constraint means maximum
// legal moves every turn, all of them equivalent.
func NoFollowAvoidanceTrick() *genome.Genome {
	return &genome.Genome{
		ID:       "no-follow-avoidance-trick",
		Skeleton: genome.TrickTaking,
		Players:  2,
		HandSize: 12,
		TrickTaking: &genome.TrickTakingParams{
			MustFollowSuit: false,
			TrickScoring:   genome.ScoreAvoidance,
			// The champion carried lead_restriction 2 (winner_leads), which
			// was the INERT encoding of the skeleton's hardcoded turn order
			// (byte-identical traces; the value is reserved and Tier-0
			// rejected since the round-2 commit 6). Encoded as LeadNone so
			// the fixture stays Tier-0 valid; behavior is unchanged.
			LeadRestriction: genome.LeadNone,
			RoundsPerGame:   2,
		},
		Scoring: genome.ScoringConfig{
			CardPoints: []genome.CardScoring{
				{Rank: 0, Suit: 0, Points: 6, Event: genome.ScoreOnTrickWin},
			},
			TrumpSuit: 4, // inert: TrumpRule is TrumpNone
		},
	}
}

// PairMeldKnockRummy reproduces archetype A3 (flagship ranks 21-29, cloned
// from rank21_gen200_52056): a 5-player, 10-card rummy game with
// min_meld_size 2 (two-card runs count as melds) and knock threshold 15 --
// a pair-meld knock race over a ~1-card stock (5x10 dealt + upcard leaves 1
// in stock). Milder than A1/A2 but scored 0.688-0.696, ABOVE the rummy
// classics (gin 0.548, knock 0.578).
func PairMeldKnockRummy() *genome.Genome {
	return &genome.Genome{
		ID:       "pair-meld-knock-rummy",
		Skeleton: genome.Rummy,
		Players:  5,
		HandSize: 10,
		Rummy: &genome.RummyParams{
			MeldTypes:      genome.MeldRuns,
			MinMeldSize:    2,
			DrawFrom:       genome.DrawEither,
			KnockThreshold: 15,
		},
		Scoring: genome.ScoringConfig{
			CardPoints: []genome.CardScoring{
				{Rank: 14, Suit: 4, Points: 3, Event: genome.ScoreOnTrickWin},
			},
		},
	}
}

// RejectedChampions returns the Task 28 round-2 failed-review fixtures as a
// group, in flagship rank order. The calibration gate and the calibrate
// subcommand both consume this list so the two can never drift.
func RejectedChampions() []*genome.Genome {
	return []*genome.Genome{
		CatchAllSkipShedding(),
		NoFollowAvoidanceTrick(),
		PairMeldKnockRummy(),
	}
}

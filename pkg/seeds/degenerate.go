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
// All returns the calibration ground-truth seeds: real, time-tested published
// card games (a popular game still in circulation is fun by survival). These are
// the calibration anchors AND the novelty seed-distance anchors. Big Two
// (climbing) joined this set once the Interaction metric was extended to measure
// the climbing skeleton (deltaModeClimbing): before that it scored interact=0.0
// and barely cleared the floor purely as a measurement artifact; with the metric
// fixed it scores ~0.55 and passes the full calibration gate.
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
		BigTwo(),
		Casino(),
		SimplePoker(),
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
//
// ROUND 3 STATUS: now a NEGATIVE TIER-0 SPECIMEN (see CatchAllChampions).
// The catch-all skip is statically rejected by genome.Validate as a liveness
// violation, so this fixture no longer reaches the metrics and is no longer
// metric ground truth; TestTier0RejectsCatchAllChampions pins the rejection.
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
//
// ROUND 4 STATUS: now a NEGATIVE TIER-0 SPECIMEN (see TrivialMeldChampions).
// min_meld_size 2 is statically rejected by genome.Validate as a trivial-meld
// liveness violation, so this fixture no longer reaches the metrics or the
// draw_supply_churn veto; TestTier0RejectsTrivialMeldChampions pins the
// rejection. (Previously a round-2 draw_supply_churn veto specimen.)
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

// CatchAllWildShedding is the round-3 catch-all encoding: a byte-faithful
// clone (ID/fitness/generation stripped) of the round-2 flagship champion
// rank01_gen185_15818 (output/2026-06-12-flagship-r2). Its first special rule
// is a CATCH-ALL WILD ({Type: SpecialWild}, ByRank=0/BySuit=0 matches every
// card), which statically deletes match_rule and draw_penalty as dead genes:
// every card is always playable, so the "shedding" skeleton's matching game
// never happens. The archetype owned the r2 shedding top 10 (density
// 0.86-0.98 from inflict-vs-plain profile mixing, greedy skill 0.00; this
// genome even cycled to the 390-turn cap under greedy play).
//
// NEGATIVE TIER-0 SPECIMEN: genome.Validate rejects the catch-all encoding
// outright (Task 28 round 3), so this fixture is ground truth for STATIC
// rejection, not for the metrics -- see CatchAllChampions and
// TestTier0RejectsCatchAllChampions.
func CatchAllWildShedding() *genome.Genome {
	return &genome.Genome{
		ID:       "catch-all-wild-shedding",
		Skeleton: genome.Shedding,
		Players:  3,
		HandSize: 13,
		Shedding: &genome.SheddingParams{
			MatchRule:     genome.MatchEither,
			DrawPenalty:   2,
			RoundsPerGame: 1,
		},
		SpecialCards: []genome.SpecialCard{
			{Type: genome.SpecialWild},                            // catch-all: EVERY card wild
			{Type: genome.SpecialDrawTwo, ByRank: 11, BySuit: 3},  // J of hearts draw-two
			{Type: genome.SpecialDrawFour, BySuit: 2},             // suit 2 draw-four
			{Type: genome.SpecialDrawFour, ByRank: 8},             // 8s draw-four
			{Type: genome.SpecialDrawTwo, BySuit: 3},              // suit 3 draw-two
		},
		Scoring: genome.ScoringConfig{
			CardPoints: []genome.CardScoring{
				{Rank: 11, Suit: 4, Points: 9, Event: genome.ScoreOnTrickWin},
			},
		},
	}
}

// --- Task 28 step 4 failed-review fixtures (round 3, 2026-06-12) ---
//
// The three constructors below encode the round-2 flagship champions
// (output/2026-06-12-flagship-r2) rejected at the round-3 designer review.
// They are clones of the published genomes with ONE deliberate re-encoding:
// each carried the catch-all wild ({type:4}, ByRank=0/BySuit=0), which is
// Tier-0 rejected since round-3 commit 1 -- here it is re-encoded as FOUR
// suit-bound wilds (BySuit 1-4), a SEMANTICALLY IDENTICAL union (every card
// is wild either way; isWild ranges over all rules) that keeps the fixture
// statically valid. That bypass is exactly why these fixtures matter: they
// prove the DYNAMIC vetoes catch the archetype even when the static
// catch-all rule is evaded by encoding.

// ReverseLockoutShedding reproduces r2 rank03 (gen200_44517): a 4-player,
// 11-card shedding game with every card wild and ~18 reverse cards (suit 4,
// all Jacks, Q of clubs, 10 of hearts). Adjacent-pair reverse ping-pong
// locks 2 of the 4 seats out of the game almost entirely (same-player runs
// stay ~1, invisible to tempo_monopoly -- the round-3 seat_participation
// detector encodes exactly this designer rejection).
func ReverseLockoutShedding() *genome.Genome {
	return &genome.Genome{
		ID:       "reverse-lockout-shedding",
		Skeleton: genome.Shedding,
		Players:  4,
		HandSize: 11,
		Shedding: &genome.SheddingParams{
			MatchRule:     genome.MatchEither,
			DrawPenalty:   3,
			RoundsPerGame: 1,
		},
		SpecialCards: []genome.SpecialCard{
			// Catch-all wild, re-encoded as the four suit wilds (see above).
			{Type: genome.SpecialWild, BySuit: 1},
			{Type: genome.SpecialWild, BySuit: 2},
			{Type: genome.SpecialWild, BySuit: 3},
			{Type: genome.SpecialWild, BySuit: 4},
			{Type: genome.SpecialDrawFour, BySuit: 1},             // clubs draw-four
			{Type: genome.SpecialReverse, ByRank: 10, BySuit: 3},  // 10 of hearts reverses
			{Type: genome.SpecialReverse, BySuit: 4},              // all spades reverse
			{Type: genome.SpecialReverse, ByRank: 12, BySuit: 1},  // Q of clubs reverses
			{Type: genome.SpecialReverse, ByRank: 11},             // all Jacks reverse
		},
		Scoring: genome.ScoringConfig{
			CardPoints: []genome.CardScoring{
				{Rank: 11, Suit: 1, Points: 15, Event: genome.ScoreOnTrickWin},
			},
		},
	}
}

// HeartEngineShedding reproduces r2 rank04 (gen192_87771): a 2-player,
// 6-card shedding game with every card wild, two draw-two suits, and a skip
// suit. In 2-player, the draw-penalty-skip (shedding/runner.go: drawCount>0
// advances past the victim) makes every attack card a play-again -- skilled
// play chains attacks into tempo monopolies the random batch never finds
// (greedy-batch detectors, round 3). Published density 0.86-0.98 via
// inflict-vs-plain profile mixing with greedy skill 0.00.
func HeartEngineShedding() *genome.Genome {
	return &genome.Genome{
		ID:       "heart-engine-shedding",
		Skeleton: genome.Shedding,
		Players:  2,
		HandSize: 6,
		Shedding: &genome.SheddingParams{
			MatchRule:     genome.MatchEither,
			DrawPenalty:   3,
			RoundsPerGame: 1,
		},
		SpecialCards: []genome.SpecialCard{
			// Catch-all wild, re-encoded as the four suit wilds (see above);
			// the champion also carried an explicit suit-1 wild, subsumed.
			{Type: genome.SpecialWild, BySuit: 1},
			{Type: genome.SpecialWild, BySuit: 2},
			{Type: genome.SpecialWild, BySuit: 3},
			{Type: genome.SpecialWild, BySuit: 4},
			{Type: genome.SpecialDrawTwo, BySuit: 2},  // diamonds draw-two
			{Type: genome.SpecialDrawTwo, BySuit: 3},  // hearts draw-two
			{Type: genome.SpecialSkip, BySuit: 3},     // hearts also skip
		},
		Scoring: genome.ScoringConfig{
			CardPoints: []genome.CardScoring{
				{Rank: 11, Suit: 1, Points: 15, Event: genome.ScoreOnTrickWin},
			},
		},
	}
}

// PairMeldStockRummy reproduces r2 rank22 (gen200_56926): a 4-player,
// 12-card pair-meld rummy (min_meld_size 2, DrawEither, knock 21) over a
// 3-card stock -- the veto-adjacent cousin of round 2's PairMeldKnockRummy
// (A3): where A3's 1-card stock pushed draw-supply churn to 0.292 (vetoed),
// this one parks churn just UNDER the 0.10 cliff and instead rode the
// count-based density exception (pinned 0.80 > gin 0.69). The round-3
// deadwood-consequence probe is what kills the archetype's score.
//
// ROUND 4 STATUS: now a NEGATIVE TIER-0 SPECIMEN (see TrivialMeldChampions).
// Its min_meld_size 2 is statically rejected, so the dynamic stack never sees
// it -- a strictly earlier kill than the churn/density measures it used to
// need. The round-4 runs-only pair-meld champions (r3 rank23/rank27) share
// the same min_meld_size 2 vector and are subsumed by the same static rule;
// this fixture stays as the SETS-side specimen, proving the Tier-0 rule
// covers both meld types.
func PairMeldStockRummy() *genome.Genome {
	return &genome.Genome{
		ID:       "pair-meld-stock-rummy",
		Skeleton: genome.Rummy,
		Players:  4,
		HandSize: 12,
		Rummy: &genome.RummyParams{
			MeldTypes:      genome.MeldSets,
			MinMeldSize:    2,
			DrawFrom:       genome.DrawEither,
			KnockThreshold: 21,
		},
		Scoring: genome.ScoringConfig{
			CardPoints: []genome.CardScoring{
				{Rank: 11, Suit: 3, Points: 10, Event: genome.ScoreOnTrickWin},
			},
		},
	}
}

// --- Task 28 step 4 failed-review fixtures (round 4, 2026-06-12) ---
//
// The round-3 flagship (output/2026-06-12-flagship-r3) was designer-reviewed:
// 0 publishable / 19 borderline / 11 degenerate. Three exploits slipped the
// frozen round-3 stack and become fixtures here. Round 4 is an authorized
// EXTRA swing past the budgeted 3 rounds, adding three NEW detectors (a
// per-turn playable-share veto, a longest-run monopoly veto, and the
// trivial-meld Tier-0 rule above) -- no weighted-metric scale constant moved.

// WildUnionShedding reproduces r3 rank01 (gen192_16965): a 2-player, 13-card
// shedding game with THREE of four suits wild (suits 2/3/4) -- 39/52 cards
// playable on any card -- so the "match suit or rank" core is dead for 75% of
// the deck. It is Tier-0 VALID (each wild is suit-qualified; no catch-all),
// so it is a TIER-2 METRIC FIXTURE. It slipped the round-3 stack because
// dead_match_rule uses the WHOLE-HAND-playable share (can the whole hand dump
// at once), which at hand 13 stays ~0.03-0.19 even though per-card 75% of the
// deck ignores the match rule. The round-4 per-turn playable-share veto
// (pkg/fitness/degeneracy.go playable_share) measures the per-card share
// directly: rank01 sits at ~0.62 vs classic shedding max ~0.30.
func WildUnionShedding() *genome.Genome {
	return &genome.Genome{
		ID:       "wild-union-shedding",
		Skeleton: genome.Shedding,
		Players:  2,
		HandSize: 13,
		Shedding: &genome.SheddingParams{
			MatchRule:   genome.MatchEither,
			DrawPenalty: 3,
		},
		SpecialCards: []genome.SpecialCard{
			{Type: genome.SpecialDrawTwo, ByRank: 2}, // 2s draw-two
			{Type: genome.SpecialReverse, ByRank: 10},// 10s reverse
			{Type: genome.SpecialWild, BySuit: 2},    // suit 2 wild
			{Type: genome.SpecialWild, BySuit: 3},    // suit 3 wild
			{Type: genome.SpecialWild, BySuit: 4},    // suit 4 wild
			{Type: genome.SpecialDrawTwo, BySuit: 1}, // suit 1 draw-two
		},
		Scoring: genome.ScoringConfig{
			CardPoints: []genome.CardScoring{
				{Rank: 13, Suit: 4, Points: 3, Event: genome.ScoreOnTrickWin},
			},
		},
	}
}

// HeartEngine2SuitShedding reproduces r3 rank04 (gen200_59040): a milder
// 2-player, 13-card shedding game with TWO of four suits wild (suits 3/4) --
// 26/52 cards always playable, draw_penalty 2. The round-3 review called it
// "one fix from a real game" -- it is the JUDGMENT fixture, deliberately NOT
// in RejectedChampions: the calibration gate does not require it to die.
// TestRound4JudgmentFixtureLanding measures where it lands and records
// whether the new detectors flag it. Its per-turn playable-share (~0.44)
// sits BETWEEN the classics (~0.30) and rank01 (~0.62); the playable-share
// threshold (0.45) is set to spare it. Its longest-run (~6.5) DOES trip the
// round-4 monopoly veto, which is the documented limitation: the longest-run
// detector cannot distinguish rank04 from its degenerate cousins.
func HeartEngine2SuitShedding() *genome.Genome {
	return &genome.Genome{
		ID:       "heart-engine-2suit-shedding",
		Skeleton: genome.Shedding,
		Players:  2,
		HandSize: 13,
		Shedding: &genome.SheddingParams{
			MatchRule:   genome.MatchEither,
			DrawPenalty: 2,
		},
		SpecialCards: []genome.SpecialCard{
			{Type: genome.SpecialReverse, ByRank: 10},            // 10s reverse
			{Type: genome.SpecialWild, BySuit: 3},                // suit 3 wild
			{Type: genome.SpecialWild, BySuit: 4},                // suit 4 wild
			{Type: genome.SpecialDrawTwo, BySuit: 1},             // suit 1 draw-two
			{Type: genome.SpecialDrawTwo, ByRank: 8, BySuit: 1},  // 8 of suit 1 draw-two
			{Type: genome.SpecialDrawFour, ByRank: 2},            // 2s draw-four
		},
		Scoring: genome.ScoringConfig{
			CardPoints: []genome.CardScoring{
				{Rank: 13, Suit: 4, Points: 3, Event: genome.ScoreOnTrickWin},
			},
		},
	}
}

// RunsOnlyPairMeldRummy reproduces r3 rank23 (gen200_2700): a 2-player,
// 13-card runs-only rummy with min_meld_size 2 (two-card runs count as
// melds), DrawEither, knock 16, carrying a draw-penalty + avoidance borrow.
// Deadwood reaches ~0 by turn 7 because five 2-card runs come off the deal,
// so melding is consequence-free. r3 rank27 is the byte-near twin (no
// borrows, catch-all-rank card scoring). It is a NEGATIVE TIER-0 SPECIMEN:
// min_meld_size 2 is statically rejected by the round-4 trivial-meld rule
// (genome.Validate); see TrivialMeldChampions.
func RunsOnlyPairMeldRummy() *genome.Genome {
	return &genome.Genome{
		ID:       "runs-only-pair-meld-rummy",
		Skeleton: genome.Rummy,
		Players:  2,
		HandSize: 13,
		Rummy: &genome.RummyParams{
			MeldTypes:      genome.MeldRuns,
			MinMeldSize:    2,
			DrawFrom:       genome.DrawEither,
			KnockThreshold: 16,
		},
		Borrowed: []genome.BorrowedMechanic{
			{Source: genome.Shedding, Mechanic: genome.MechDrawPenalty},
			{Source: genome.TrickTaking, Mechanic: genome.MechAvoidance},
		},
		Scoring: genome.ScoringConfig{
			CardPoints: []genome.CardScoring{
				{Rank: 6, Suit: 4, Points: 3, Event: genome.ScoreOnTrickWin},
			},
		},
	}
}

// RejectedChampions returns the failed-review fixtures that are TIER-2 METRIC
// ground truth: statically valid genomes the dynamic pipeline (Tier 1 +
// degeneracy vetoes + metrics) must rank below every classic. The calibration
// gate and the calibrate subcommand both consume this list so the two can
// never drift.
//
// RESTRUCTURED in round 3: fixtures whose degeneracy became a Tier-0 rule (the
// catch-all specials) moved to CatchAllChampions -- a Tier-0-rejected genome
// never reaches the metrics, so it cannot serve as metric ground truth.
//
// RESTRUCTURED AGAIN in round 4: the two pair-meld fixtures (min_meld_size 2)
// moved to TrivialMeldChampions for the same reason -- the round-4 Tier-0
// trivial-meld rule rejects them statically. The round-4 ADDITION is the
// wild-union shedding champion (r3 rank01), Tier-0 valid and killed
// dynamically by the new per-turn playable-share veto.
func RejectedChampions() []*genome.Genome {
	return []*genome.Genome{
		NoFollowAvoidanceTrick(),
		ReverseLockoutShedding(),
		HeartEngineShedding(),
		WildUnionShedding(), // round 4: r3 rank01, killed by playable_share veto
	}
}

// CatchAllChampions returns the rejected champions whose shared degeneracy
// vector -- a catch-all special card ({ByRank: 0, BySuit: 0} matches every
// card) -- is rejected STATICALLY by genome.Validate (Task 28 round 3). They
// are negative Tier-0 specimens: TestTier0RejectsCatchAllChampions asserts
// each one fails static validation, the inverse of the Tier-2 fixtures'
// contract.
func CatchAllChampions() []*genome.Genome {
	return []*genome.Genome{
		CatchAllSkipShedding(), // round-1 A1: catch-all SKIP
		CatchAllWildShedding(), // round-2 rank01: catch-all WILD
	}
}

// TrivialMeldChampions returns the rejected champions whose shared degeneracy
// vector -- a 2-card meld (min_meld_size 2) -- is rejected STATICALLY by
// genome.Validate (Task 28 round 4 trivial-meld liveness rule). Like
// CatchAllChampions they are negative Tier-0 specimens (not metric ground
// truth): TestTier0RejectsTrivialMeldChampions asserts each fails static
// validation. The list spans both meld types so the test proves the Tier-0
// rule is not runs-specific.
func TrivialMeldChampions() []*genome.Genome {
	return []*genome.Genome{
		RunsOnlyPairMeldRummy(), // round 4: r3 rank23/rank27, runs min 2
		PairMeldKnockRummy(),    // round 2 A3: runs min 2 (now Tier-0)
		PairMeldStockRummy(),    // round 3 r2 rank22: sets min 2 (now Tier-0)
	}
}

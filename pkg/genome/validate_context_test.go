package genome

import (
	"strings"
	"testing"
)

// The contextual Tier-0 rules: combinations that are individually whitelisted
// but degenerate together (measured seat-0 sweeps / all-draw timeouts) must be
// rejected statically, not left for Tier-1 to burn evaluations on.

func trickTakingWithAvoidanceBorrow(scoring TrickScoring) *Genome {
	return &Genome{
		ID:       "tt-avoid",
		Skeleton: TrickTaking,
		Players:  4,
		HandSize: 13,
		TrickTaking: &TrickTakingParams{
			MustFollowSuit:  true,
			TrickScoring:    scoring,
			LeadRestriction: LeadNone,
			RoundsPerGame:   3,
		},
		Scoring: ScoringConfig{
			CardPoints: []CardScoring{{Suit: 3, Points: 1}}, // hearts = 1 penalty
		},
		Borrowed: []BorrowedMechanic{{Source: Rummy, Mechanic: MechAvoidance}},
	}
}

func TestValidateRejectsAvoidanceBorrowOnCardPointScoring(t *testing.T) {
	// scoreTrick banks MatchCardPoints per captured card; applyAvoidance
	// subtracts the identical values over the identical tableau -- exact
	// cancellation, every score 0, seat 0 wins the tiebreak 100% of games.
	for _, scoring := range []TrickScoring{ScoreCardPoints, ScoreAvoidance} {
		g := trickTakingWithAvoidanceBorrow(scoring)
		errs := Validate(g)
		found := false
		for _, e := range errs {
			if strings.Contains(e, "avoidance borrow on trick_taking requires trick_scoring=per_trick") {
				found = true
			}
		}
		if !found {
			t.Errorf("trick_scoring=%v + avoidance borrow should be rejected, got: %v", scoring, errs)
		}
	}
}

func TestValidateAcceptsAvoidanceBorrowOnPerTrickScoring(t *testing.T) {
	g := trickTakingWithAvoidanceBorrow(ScorePerTrick)
	if errs := Validate(g); len(errs) != 0 {
		t.Fatalf("per_trick + avoidance borrow is the live cross-family combo and must validate, got: %v", errs)
	}
}

func sheddingWithFollowSuitBorrow(rule MatchRule) *Genome {
	return &Genome{
		ID:       "shed-follow",
		Skeleton: Shedding,
		Players:  4,
		HandSize: 7,
		Shedding: &SheddingParams{MatchRule: rule, DrawPenalty: 1},
		Borrowed: []BorrowedMechanic{{Source: TrickTaking, Mechanic: MechFollowSuit}},
	}
}

func TestValidateRejectsFollowSuitOnMatchRank(t *testing.T) {
	// Under MatchRank a held top-suit card is never itself playable, so the
	// follow-suit obligation erases every legal play: all-draw to the timeout
	// (measured 0/200 completions).
	g := sheddingWithFollowSuitBorrow(MatchRank)
	errs := Validate(g)
	found := false
	for _, e := range errs {
		if strings.Contains(e, "follow_suit borrow on shedding requires match_rule suit or either") {
			found = true
		}
	}
	if !found {
		t.Fatalf("match_rule=rank + follow_suit borrow should be rejected, got: %v", errs)
	}
}

func TestValidateAcceptsFollowSuitOnPermissiveMatchRules(t *testing.T) {
	for _, rule := range []MatchRule{MatchSuit, MatchEither} {
		g := sheddingWithFollowSuitBorrow(rule)
		if errs := Validate(g); len(errs) != 0 {
			t.Errorf("match_rule=%v + follow_suit borrow should validate, got: %v", rule, errs)
		}
	}
}

func TestValidateRejectsMatchBoth(t *testing.T) {
	// The only suit+rank match of the top card is the top card itself, which
	// no hand can hold in a single deck: no non-wild play is ever legal.
	g := &Genome{
		ID:       "shed-both",
		Skeleton: Shedding,
		Players:  2,
		HandSize: 7,
		Shedding: &SheddingParams{MatchRule: MatchBoth, DrawPenalty: 1},
	}
	errs := Validate(g)
	found := false
	for _, e := range errs {
		if strings.Contains(e, "statically unplayable") {
			found = true
		}
	}
	if !found {
		t.Fatalf("match_rule=both should be rejected as statically unplayable, got: %v", errs)
	}
}

package genome

import "testing"

// TestCrossFamilyBorrowsWhitelisted pins the novelty-evolution whitelist
// expansion: shedding may borrow the active cross-family MechTrickScoring
// (shed-to-win scored by tricks) and trick-taking may borrow the active
// cross-family MechAvoidance (penalty-card scoring). Both have working hooks
// that affect the winner; this guards against the entries silently dropping
// out of the whitelist.
func TestCrossFamilyBorrowsWhitelisted(t *testing.T) {
	if !validBorrows[Shedding][MechTrickScoring] {
		t.Error("shedding must be able to borrow MechTrickScoring (shed-to-win-by-tricks hybrid)")
	}
	if !validBorrows[TrickTaking][MechAvoidance] {
		t.Error("trick-taking must be able to borrow MechAvoidance (penalty-card scoring hybrid)")
	}

	// Reserved/no-hook mechanics must NEVER be whitelisted (the expansion must
	// not relax the dd-lnh guard).
	for skel, allowed := range validBorrows {
		if allowed[MechTrump] || allowed[MechPlayMultiple] {
			t.Errorf("reserved mechanic whitelisted on %s skeleton", skel)
		}
	}
}

// TestSheddingTrickScoringBorrowValidates: a shedding host carrying a
// MechTrickScoring borrow (multi-round, the coherent hybrid form) passes Tier 0.
func TestSheddingTrickScoringBorrowValidates(t *testing.T) {
	g := &Genome{
		Skeleton: Shedding,
		Players:  3,
		HandSize: 7,
		Shedding: &SheddingParams{MatchRule: MatchEither, DrawPenalty: 1, RoundsPerGame: 3},
		Borrowed: []BorrowedMechanic{{Source: TrickTaking, Mechanic: MechTrickScoring}},
	}
	if errs := Validate(g); len(errs) > 0 {
		t.Fatalf("shed-to-win-by-tricks hybrid should be Tier-0 valid, got: %v", errs)
	}
	if !g.SheddingMultiRound() {
		t.Error("MechTrickScoring borrow + RoundsPerGame>1 should activate multi-round play")
	}
	if !g.SheddingTrickScored() {
		t.Error("SheddingTrickScored() should be true for a shedding host with a MechTrickScoring borrow")
	}
}

// TestTrickTakingAvoidanceBorrowValidates: a trick-taking host carrying a
// MechAvoidance borrow with CardPoints passes Tier 0.
func TestTrickTakingAvoidanceBorrowValidates(t *testing.T) {
	g := &Genome{
		Skeleton: TrickTaking,
		Players:  4,
		HandSize: 13,
		TrickTaking: &TrickTakingParams{
			MustFollowSuit:  true,
			TrickScoring:    ScorePerTrick,
			LeadRestriction: LeadNone,
			RoundsPerGame:   1,
		},
		Borrowed: []BorrowedMechanic{{Source: Shedding, Mechanic: MechAvoidance}},
		Scoring:  ScoringConfig{CardPoints: []CardScoring{{Suit: 3, Points: 1}}},
	}
	if errs := Validate(g); len(errs) > 0 {
		t.Fatalf("trick-taking + avoidance borrow should be Tier-0 valid, got: %v", errs)
	}
	live := g.LiveBorrows()
	if len(live) != 1 || live[0].Mechanic != MechAvoidance {
		t.Errorf("avoidance borrow on trick-taking should be live, got %v", live)
	}
}

// TestHasBankingBorrow pins the broadened banking predicate.
func TestHasBankingBorrow(t *testing.T) {
	cases := []struct {
		name string
		mech MechanicType
		want bool
	}{
		{"meld bonus", MechMeldBonus, true},
		{"avoidance", MechAvoidance, true},
		{"trick scoring", MechTrickScoring, true},
		{"draw penalty", MechDrawPenalty, false},
	}
	for _, tc := range cases {
		g := &Genome{Borrowed: []BorrowedMechanic{{Source: Rummy, Mechanic: tc.mech}}}
		if got := g.HasBankingBorrow(); got != tc.want {
			t.Errorf("%s: HasBankingBorrow() = %v, want %v", tc.name, got, tc.want)
		}
	}
	// HasScoringBorrow stays NARROW: trick-scoring is a banking borrow but not
	// a "scoring borrow" in the LiveBorrows multi-round-required sense.
	ts := &Genome{Borrowed: []BorrowedMechanic{{Source: TrickTaking, Mechanic: MechTrickScoring}}}
	if ts.HasScoringBorrow() {
		t.Error("HasScoringBorrow() must stay narrow (MeldBonus/Avoidance only); trick-scoring is banking-only")
	}
}

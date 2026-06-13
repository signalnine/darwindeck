package genome

import "testing"

// Liveness predicate tests (Wave K fix 2 prerequisite). The predicates were
// born in pkg/output/rulebook.go (round 3 commit 6: no dead-rule text) and
// move here so the output-ranking dedup in pkg/evolution can reuse the EXACT
// same rules without an import cycle. These tests pin the semantics at the
// new home.

func sheddingHost(rounds int, borrows ...BorrowedMechanic) *Genome {
	return &Genome{
		Skeleton: Shedding,
		Players:  2,
		HandSize: 7,
		Shedding: &SheddingParams{MatchRule: MatchEither, DrawPenalty: 1, RoundsPerGame: rounds},
		Borrowed: borrows,
	}
}

func TestLiveBorrowsSheddingScoringBorrowNeedsMultiRound(t *testing.T) {
	meld := BorrowedMechanic{Source: Rummy, Mechanic: MechMeldBonus}

	single := sheddingHost(1, meld)
	if got := single.LiveBorrows(); len(got) != 0 {
		t.Errorf("single-round shedding meld-bonus borrow must be dead, got %v", got)
	}

	multi := sheddingHost(3, meld)
	if got := multi.LiveBorrows(); len(got) != 1 || got[0] != meld {
		t.Errorf("multi-round shedding meld-bonus borrow must be live, got %v", got)
	}
}

func TestLiveBorrowsAvoidanceNeedsCardPoints(t *testing.T) {
	avoid := BorrowedMechanic{Source: TrickTaking, Mechanic: MechAvoidance}

	// Multi-round shedding host, but no CardPoints: the applyAvoidance hook
	// no-ops, so the borrow is dead.
	noPoints := sheddingHost(3, avoid)
	if got := noPoints.LiveBorrows(); len(got) != 0 {
		t.Errorf("avoidance borrow without CardPoints must be dead, got %v", got)
	}

	withPoints := sheddingHost(3, avoid)
	withPoints.Scoring.CardPoints = []CardScoring{{Suit: 2, Points: 1}}
	if got := withPoints.LiveBorrows(); len(got) != 1 || got[0] != avoid {
		t.Errorf("avoidance borrow with CardPoints on multi-round host must be live, got %v", got)
	}
}

func TestLiveBorrowsDirectActingAlwaysLive(t *testing.T) {
	direct := []BorrowedMechanic{
		{Source: TrickTaking, Mechanic: MechTrickScoring},
		{Source: Shedding, Mechanic: MechDrawPenalty},
	}
	g := sheddingHost(1, direct...) // single round: scoring borrows would be dead here
	got := g.LiveBorrows()
	if len(got) != 2 {
		t.Fatalf("direct-acting borrows must always be live, got %v", got)
	}
}

func TestLiveCardPoints(t *testing.T) {
	pts := []CardScoring{{Suit: 2, Points: 1}}

	cases := []struct {
		name string
		g    *Genome
		want bool
	}{
		{
			name: "empty card points never live",
			g:    &Genome{Skeleton: TrickTaking, TrickTaking: &TrickTakingParams{TrickScoring: ScoreCardPoints}},
			want: false,
		},
		{
			name: "trick-taking card_points scoring reads them",
			g: &Genome{Skeleton: TrickTaking,
				TrickTaking: &TrickTakingParams{TrickScoring: ScoreCardPoints},
				Scoring:     ScoringConfig{CardPoints: pts}},
			want: true,
		},
		{
			name: "trick-taking avoidance scoring reads them",
			g: &Genome{Skeleton: TrickTaking,
				TrickTaking: &TrickTakingParams{TrickScoring: ScoreAvoidance},
				Scoring:     ScoringConfig{CardPoints: pts}},
			want: true,
		},
		{
			name: "trick-taking per_trick scoring ignores them",
			g: &Genome{Skeleton: TrickTaking,
				TrickTaking: &TrickTakingParams{TrickScoring: ScorePerTrick},
				Scoring:     ScoringConfig{CardPoints: pts}},
			want: false,
		},
		{
			name: "borrow-less single-round shedding ignores them (flagship-r3 ranks 1/2/3)",
			g: func() *Genome {
				g := sheddingHost(1)
				g.Scoring.CardPoints = pts
				return g
			}(),
			want: false,
		},
		{
			name: "live avoidance borrow reads them",
			g: func() *Genome {
				g := sheddingHost(3, BorrowedMechanic{Source: TrickTaking, Mechanic: MechAvoidance})
				g.Scoring.CardPoints = pts
				return g
			}(),
			want: true,
		},
		{
			name: "borrow-less rummy ignores them",
			g: &Genome{Skeleton: Rummy,
				Rummy:   &RummyParams{MeldTypes: MeldBoth, MinMeldSize: 3},
				Scoring: ScoringConfig{CardPoints: pts}},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.g.LiveCardPoints(); got != tc.want {
				t.Errorf("LiveCardPoints() = %v, want %v", got, tc.want)
			}
		})
	}
}

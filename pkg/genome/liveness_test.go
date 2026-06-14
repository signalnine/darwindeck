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
	// MechDrawPenalty acts directly (appends cards on play), so it is live on a
	// single-round shedding host. MechTrickScoring is NOT direct-acting on a
	// shedding host: its applyTrickScoring hook BANKS into state.Scores at round
	// end, which a single-round shedding game never reads (it ends at the first
	// empty hand) -- so it is live only in multi-round mode, like the other
	// banking borrows (novelty evolution: the shed-to-win-by-tricks hybrid).
	directOnly := sheddingHost(1, BorrowedMechanic{Source: Shedding, Mechanic: MechDrawPenalty})
	if got := directOnly.LiveBorrows(); len(got) != 1 || got[0].Mechanic != MechDrawPenalty {
		t.Fatalf("direct-acting draw_penalty must be live on a single-round host, got %v", got)
	}

	// Trick-scoring on a SINGLE-round shedding host is inert (banks scores
	// nothing reads).
	tsSingle := sheddingHost(1, BorrowedMechanic{Source: TrickTaking, Mechanic: MechTrickScoring})
	if got := tsSingle.LiveBorrows(); len(got) != 0 {
		t.Errorf("trick-scoring on single-round shedding must be inert (banks unread scores), got %v", got)
	}

	// Trick-scoring on a MULTI-round shedding host is live (the headline
	// hybrid). sheddingHost(3, ...) gives RoundsPerGame 3, and MechTrickScoring
	// is a banking borrow, so SheddingMultiRound() is true.
	tsMulti := sheddingHost(3, BorrowedMechanic{Source: TrickTaking, Mechanic: MechTrickScoring})
	if got := tsMulti.LiveBorrows(); len(got) != 1 || got[0].Mechanic != MechTrickScoring {
		t.Errorf("trick-scoring on multi-round shedding must be live, got %v", got)
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

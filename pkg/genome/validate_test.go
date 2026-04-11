package genome

import "testing"

func TestValidGenome(t *testing.T) {
	g := &Genome{
		ID:       "test",
		Skeleton: Shedding,
		Players:  2,
		HandSize: 7,
		Shedding: &SheddingParams{
			MatchRule:   MatchEither,
			DrawPenalty: 1,
		},
	}
	errs := Validate(g)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %v", errs)
	}
}

func TestInvalidPlayerCount(t *testing.T) {
	g := &Genome{
		Skeleton: Shedding,
		Players:  8,
		HandSize: 7,
		Shedding: &SheddingParams{MatchRule: MatchEither, DrawPenalty: 1},
	}
	errs := Validate(g)
	if len(errs) == 0 {
		t.Fatal("expected errors for 8 players")
	}
}

func TestDeckOverflow(t *testing.T) {
	g := &Genome{
		Skeleton: Shedding,
		Players:  6,
		HandSize: 10,
		Shedding: &SheddingParams{MatchRule: MatchEither, DrawPenalty: 1},
	}
	errs := Validate(g)
	found := false
	for _, e := range errs {
		if e == "hand_size(10) * players(6) = 60 exceeds 52-card deck" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected deck overflow error, got: %v", errs)
	}
}

func TestMissingSkeletonParams(t *testing.T) {
	g := &Genome{
		Skeleton: TrickTaking,
		Players:  4,
		HandSize: 13,
	}
	errs := Validate(g)
	found := false
	for _, e := range errs {
		if e == "trick_taking skeleton requires trick_taking params" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected missing params error, got: %v", errs)
	}
}

func TestInvalidBorrow(t *testing.T) {
	g := &Genome{
		Skeleton: Shedding,
		Players:  2,
		HandSize: 7,
		Shedding: &SheddingParams{MatchRule: MatchEither, DrawPenalty: 1},
		Borrowed: []BorrowedMechanic{
			{Source: TrickTaking, Mechanic: MechFollowSuit}, // Not allowed for shedding
		},
	}
	errs := Validate(g)
	if len(errs) == 0 {
		t.Fatal("expected error for invalid borrow")
	}
}

func TestSelfBorrow(t *testing.T) {
	g := &Genome{
		Skeleton: Shedding,
		Players:  2,
		HandSize: 7,
		Shedding: &SheddingParams{MatchRule: MatchEither, DrawPenalty: 1},
		Borrowed: []BorrowedMechanic{
			{Source: Shedding, Mechanic: MechDrawPenalty},
		},
	}
	errs := Validate(g)
	found := false
	for _, e := range errs {
		if e == "cannot borrow from own skeleton: shedding" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected self-borrow error, got: %v", errs)
	}
}

func TestTrumpInRummy(t *testing.T) {
	g := &Genome{
		Skeleton:  Rummy,
		Players:   2,
		HandSize:  10,
		TrumpRule: TrumpFixed,
		Rummy: &RummyParams{
			MeldTypes:      MeldBoth,
			MinMeldSize:    3,
			DrawFrom:       DrawEither,
			KnockThreshold: 10,
		},
		Scoring: ScoringConfig{TrumpSuit: 1},
	}
	errs := Validate(g)
	found := false
	for _, e := range errs {
		if e == "trump rule not applicable to rummy skeleton" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected trump-in-rummy error, got: %v", errs)
	}
}

func TestValidTrickTaking(t *testing.T) {
	g := &Genome{
		ID:       "whist",
		Skeleton: TrickTaking,
		Players:  4,
		HandSize: 13,
		TrickTaking: &TrickTakingParams{
			MustFollowSuit:  true,
			TrickScoring:    ScorePerTrick,
			LeadRestriction: LeadWinnerLeads,
			RoundsPerGame:   1,
		},
		TrumpRule: TrumpCut,
	}
	errs := Validate(g)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %v", errs)
	}
}

func TestValidRummy(t *testing.T) {
	g := &Genome{
		ID:       "gin",
		Skeleton: Rummy,
		Players:  2,
		HandSize: 10,
		Rummy: &RummyParams{
			MeldTypes:      MeldBoth,
			MinMeldSize:    3,
			DrawFrom:       DrawEither,
			CanLayOff:      false,
			KnockThreshold: 10,
		},
	}
	errs := Validate(g)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %v", errs)
	}
}

func TestCardPointsScoringRequiresConfig(t *testing.T) {
	g := &Genome{
		Skeleton: TrickTaking,
		Players:  4,
		HandSize: 13,
		TrickTaking: &TrickTakingParams{
			MustFollowSuit:  true,
			TrickScoring:    ScoreCardPoints,
			LeadRestriction: LeadNone,
			RoundsPerGame:   1,
		},
	}
	errs := Validate(g)
	found := false
	for _, e := range errs {
		if e == "card_points/avoidance scoring requires card_points in scoring config" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected scoring config error, got: %v", errs)
	}
}

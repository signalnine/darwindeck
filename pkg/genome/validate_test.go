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
			KnockThreshold: 10,
		},
	}
	errs := Validate(g)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %v", errs)
	}
}

func TestValidateRejectsSpecialCardsOnNonShedding(t *testing.T) {
	// Only the shedding runner reads g.SpecialCards. A trick-taking or rummy
	// genome carrying special cards advertises skip/draw effects that never
	// fire, so Tier 0 must reject them (dd-24e).
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
		SpecialCards: []SpecialCard{{Type: SpecialSkip, ByRank: 7}},
	}
	if errs := Validate(g); len(errs) == 0 {
		t.Fatal("expected validation to reject SpecialCards on a trick-taking skeleton")
	}

	r := &Genome{
		Skeleton:     Rummy,
		Players:      2,
		HandSize:     10,
		Rummy:        &RummyParams{MeldTypes: MeldBoth, MinMeldSize: 3, DrawFrom: DrawEither, KnockThreshold: 10},
		SpecialCards: []SpecialCard{{Type: SpecialDrawTwo, ByRank: 2}},
	}
	if errs := Validate(r); len(errs) == 0 {
		t.Fatal("expected validation to reject SpecialCards on a rummy skeleton")
	}
}

func TestSheddingTrumpBorrowRejected(t *testing.T) {
	// MechTrump in shedding had no runner-side implementation (shedding
	// runner never reads TrumpRule/TrumpSuit). Allowing it via the
	// validBorrows whitelist let evolution waste a search dimension on a
	// no-op. See dd-lnh.
	g := &Genome{
		Skeleton: Shedding,
		Players:  2,
		HandSize: 7,
		Shedding: &SheddingParams{MatchRule: MatchEither, DrawPenalty: 1},
		Borrowed: []BorrowedMechanic{
			{Source: TrickTaking, Mechanic: MechTrump},
		},
	}
	errs := Validate(g)
	if len(errs) == 0 {
		t.Fatal("expected validation to reject MechTrump borrow in shedding (no runner support)")
	}
}

func TestTrickTakingPlayMultipleBorrowRejected(t *testing.T) {
	// MechPlayMultiple in trick-taking had no runner-side implementation
	// (tricktaking move-gen only ever produces single-card plays). See
	// dd-lnh.
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
		Borrowed: []BorrowedMechanic{
			{Source: Shedding, Mechanic: MechPlayMultiple},
		},
	}
	errs := Validate(g)
	if len(errs) == 0 {
		t.Fatal("expected validation to reject MechPlayMultiple borrow in trick-taking (no runner support)")
	}
}

func TestReservedMechanicsNeverWhitelisted(t *testing.T) {
	// MechTrump and MechPlayMultiple are reserved enum values with no hook or
	// runner implementation (dd-lnh; audit remediation Task 23). They must
	// never appear in the validBorrows whitelist for any skeleton. The enum
	// values themselves are kept (marked reserved) because MechanicType
	// serializes as a bare number: renumbering would corrupt every existing
	// serialized genome.
	reserved := []MechanicType{MechTrump, MechPlayMultiple}
	for skel, allowed := range validBorrows {
		for _, m := range reserved {
			if allowed[m] {
				t.Errorf("reserved mechanic %s whitelisted for skeleton %s", m, skel)
			}
		}
	}
}

func TestValidateRejectsReservedBorrowsOnEverySkeleton(t *testing.T) {
	hosts := []*Genome{
		{
			Skeleton: Shedding,
			Players:  2,
			HandSize: 7,
			Shedding: &SheddingParams{MatchRule: MatchEither, DrawPenalty: 1},
		},
		{
			Skeleton: TrickTaking,
			Players:  4,
			HandSize: 13,
			TrickTaking: &TrickTakingParams{
				MustFollowSuit:  true,
				TrickScoring:    ScorePerTrick,
				LeadRestriction: LeadNone,
				RoundsPerGame:   1,
			},
		},
		{
			Skeleton: Rummy,
			Players:  2,
			HandSize: 10,
			Rummy:    &RummyParams{MeldTypes: MeldBoth, MinMeldSize: 3, DrawFrom: DrawEither, KnockThreshold: 10},
		},
	}

	for _, host := range hosts {
		if errs := Validate(host); len(errs) > 0 {
			t.Fatalf("baseline %s genome should be valid: %v", host.Skeleton, errs)
		}

		source := Shedding
		if host.Skeleton == Shedding {
			source = Rummy
		}

		for _, m := range []MechanicType{MechTrump, MechPlayMultiple} {
			g := host.Clone()
			g.Borrowed = []BorrowedMechanic{{Source: source, Mechanic: m}}
			if errs := Validate(g); len(errs) == 0 {
				t.Errorf("Validate accepted reserved mechanic %s on %s skeleton", m, host.Skeleton)
			}
		}
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

// --- Task 22: SheddingParams.RoundsPerGame (multi-round shedding) ---

// sheddingGenomeWithRounds builds a minimal valid shedding genome with the
// given RoundsPerGame value.
func sheddingGenomeWithRounds(rounds int) *Genome {
	return &Genome{
		ID:       "rounds-test",
		Skeleton: Shedding,
		Players:  2,
		HandSize: 7,
		Shedding: &SheddingParams{
			MatchRule:     MatchEither,
			DrawPenalty:   1,
			RoundsPerGame: rounds,
		},
	}
}

// TestSheddingRoundsPerGameRange: valid values are 0-5. Zero is the legacy
// "unset" encoding carried by every pre-Task-22 genome (seeds, serialized
// output, test fixtures) and is treated as 1 round; 1-5 are the evolvable
// values.
func TestSheddingRoundsPerGameRange(t *testing.T) {
	for _, rounds := range []int{0, 1, 3, 5} {
		if errs := Validate(sheddingGenomeWithRounds(rounds)); len(errs) != 0 {
			t.Errorf("RoundsPerGame=%d should be valid, got: %v", rounds, errs)
		}
	}
	for _, rounds := range []int{-1, 6, 100} {
		if errs := Validate(sheddingGenomeWithRounds(rounds)); len(errs) == 0 {
			t.Errorf("RoundsPerGame=%d should be rejected", rounds)
		}
	}
}

// TestSheddingMultiRoundPredicate pins the activation rule for multi-round
// shedding: RoundsPerGame > 1 AND a scoring borrow (MechMeldBonus or
// MechAvoidance) present. Without a scoring borrow nothing ever writes
// state.Scores, so a multi-round game would have no winner signal -- the
// rounds parameter is only meaningful with the borrows that bank points.
func TestSheddingMultiRoundPredicate(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Genome)
		want   bool
	}{
		{"no borrow, 3 rounds", func(g *Genome) {}, false},
		{"meld bonus, 1 round", func(g *Genome) {
			g.Shedding.RoundsPerGame = 1
			g.Borrowed = []BorrowedMechanic{{Source: Rummy, Mechanic: MechMeldBonus}}
		}, false},
		{"meld bonus, 0 rounds (legacy unset)", func(g *Genome) {
			g.Shedding.RoundsPerGame = 0
			g.Borrowed = []BorrowedMechanic{{Source: Rummy, Mechanic: MechMeldBonus}}
		}, false},
		{"meld bonus, 3 rounds", func(g *Genome) {
			g.Borrowed = []BorrowedMechanic{{Source: Rummy, Mechanic: MechMeldBonus}}
		}, true},
		{"avoidance, 3 rounds", func(g *Genome) {
			g.Borrowed = []BorrowedMechanic{{Source: TrickTaking, Mechanic: MechAvoidance}}
			g.Scoring.CardPoints = []CardScoring{{Suit: 3, Points: 1}}
		}, true},
	}
	for _, tc := range cases {
		g := sheddingGenomeWithRounds(3)
		tc.mutate(g)
		if got := g.SheddingMultiRound(); got != tc.want {
			t.Errorf("%s: SheddingMultiRound() = %v, want %v", tc.name, got, tc.want)
		}
	}

	// Non-shedding skeletons are never multi-round-shedding, even with a
	// scoring borrow and a rounds param present.
	tt := &Genome{
		Skeleton: TrickTaking,
		Players:  4,
		HandSize: 13,
		TrickTaking: &TrickTakingParams{
			MustFollowSuit: true,
			RoundsPerGame:  3,
		},
		Borrowed: []BorrowedMechanic{{Source: Rummy, Mechanic: MechMeldBonus}},
	}
	if tt.SheddingMultiRound() {
		t.Error("trick-taking genome reported SheddingMultiRound() = true")
	}
}

// TestSheddingMultiRoundScalesMaxTurns: a 3-round game needs ~3x the turn
// budget or every multi-round genome dies as a Tier-1 timeout. Single-round
// genomes (and rounds without a scoring borrow) keep the original cap so
// pre-Task-22 timeout detection is unchanged.
func TestSheddingMultiRoundScalesMaxTurns(t *testing.T) {
	base := sheddingGenomeWithRounds(0)
	single := base.MaxTurns()

	multi := sheddingGenomeWithRounds(3)
	multi.Borrowed = []BorrowedMechanic{{Source: Rummy, Mechanic: MechMeldBonus}}
	if got, want := multi.MaxTurns(), single*3; got != want {
		t.Errorf("3-round MaxTurns = %d, want %d (3x single-round %d)", got, want, single)
	}

	// Rounds WITHOUT a scoring borrow: single-round semantics, single-round cap.
	inert := sheddingGenomeWithRounds(3)
	if got := inert.MaxTurns(); got != single {
		t.Errorf("rounds-without-borrow MaxTurns = %d, want unchanged %d", got, single)
	}
}

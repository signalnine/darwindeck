package genome

import "testing"

func TestCloneDeepCopyAllPointers(t *testing.T) {
	original := &Genome{
		ID:       "orig",
		Skeleton: Shedding,
		Players:  2,
		HandSize: 7,
		Shedding: &SheddingParams{
			MatchRule:   MatchEither,
			DrawPenalty: 1,
		},
		TrickTaking: &TrickTakingParams{
			MustFollowSuit: true,
			RoundsPerGame:  4,
		},
		Rummy: &RummyParams{
			MeldTypes:      MeldBoth,
			MinMeldSize:    3,
			DrawFrom:       DrawEither,
			KnockThreshold: 10,
		},
		Borrowed: []BorrowedMechanic{
			{Source: TrickTaking, Mechanic: MechFollowSuit},
		},
		SpecialCards: []SpecialCard{
			{Type: SpecialWild, ByRank: 8},
		},
		Scoring: ScoringConfig{
			CardPoints: []CardScoring{
				{Rank: 11, Suit: 0, Points: 10, Event: ScoreOnTrickWin},
			},
			TrumpSuit: 2,
		},
	}

	clone := original.Clone()

	if clone == original {
		t.Fatal("Clone returned same pointer")
	}
	if clone.Shedding == original.Shedding {
		t.Fatal("Shedding pointer not deep-copied")
	}
	if clone.TrickTaking == original.TrickTaking {
		t.Fatal("TrickTaking pointer not deep-copied")
	}
	if clone.Rummy == original.Rummy {
		t.Fatal("Rummy pointer not deep-copied")
	}
	if &clone.Borrowed[0] == &original.Borrowed[0] {
		t.Fatal("Borrowed slice not deep-copied")
	}
	if &clone.SpecialCards[0] == &original.SpecialCards[0] {
		t.Fatal("SpecialCards slice not deep-copied")
	}
	if &clone.Scoring.CardPoints[0] == &original.Scoring.CardPoints[0] {
		t.Fatal("Scoring.CardPoints slice not deep-copied")
	}

	clone.ID = "modified"
	clone.Players = 6
	clone.Shedding.DrawPenalty = 99
	clone.TrickTaking.RoundsPerGame = 99
	clone.Rummy.KnockThreshold = 99
	clone.Borrowed[0].Mechanic = MechTrump
	clone.SpecialCards[0].ByRank = 99
	clone.Scoring.CardPoints[0].Points = 999
	clone.Scoring.TrumpSuit = 4

	if original.ID == "modified" {
		t.Fatal("clone modified original ID")
	}
	if original.Players == 6 {
		t.Fatal("clone modified original Players")
	}
	if original.Shedding.DrawPenalty == 99 {
		t.Fatal("clone modified original Shedding.DrawPenalty")
	}
	if original.TrickTaking.RoundsPerGame == 99 {
		t.Fatal("clone modified original TrickTaking.RoundsPerGame")
	}
	if original.Rummy.KnockThreshold == 99 {
		t.Fatal("clone modified original Rummy.KnockThreshold")
	}
	if original.Borrowed[0].Mechanic == MechTrump {
		t.Fatal("clone modified original Borrowed mechanic")
	}
	if original.SpecialCards[0].ByRank == 99 {
		t.Fatal("clone modified original SpecialCards entry")
	}
	if original.Scoring.CardPoints[0].Points == 999 {
		t.Fatal("clone modified original Scoring.CardPoints entry")
	}
	if original.Scoring.TrumpSuit == 4 {
		t.Fatal("clone modified original Scoring.TrumpSuit")
	}
}

func TestCloneNilSafe(t *testing.T) {
	var g *Genome
	if got := g.Clone(); got != nil {
		t.Fatalf("Clone of nil genome should be nil, got %#v", got)
	}
}

func TestCloneOmitsOptionalFields(t *testing.T) {
	original := &Genome{
		ID:       "minimal",
		Skeleton: Shedding,
		Players:  2,
		HandSize: 7,
	}
	clone := original.Clone()
	if clone.Shedding != nil || clone.TrickTaking != nil || clone.Rummy != nil {
		t.Fatal("nil skeleton params should remain nil after Clone")
	}
	if clone.Borrowed != nil || clone.SpecialCards != nil {
		t.Fatal("nil slices should remain nil after Clone")
	}
	if clone.Scoring.CardPoints != nil {
		t.Fatal("nil CardPoints should remain nil after Clone")
	}
}

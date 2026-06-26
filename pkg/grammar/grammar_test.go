package grammar

import "testing"

// The grammar's whole claim is playable-by-construction: across the ENTIRE typed
// composition space, random play never gets stuck (safety) and always terminates
// (liveness). These tests pin that claim so a future primitive/modifier that
// breaks it fails loudly.

const testTrials = 20

// TestExpressiveness: the grammar reproduces the hand-coded skeletons it covers.
func TestExpressiveness(t *testing.T) {
	for i, s := range Canonical() {
		sm := Runner{Spec: s}.Playability(testTrials, 1)
		if !sm.Playable() {
			t.Errorf("canonical[%d] %s not playable: term=%d/%d stuck=%d cap=%d agency=%.2f",
				i, s.Family(), sm.Terminated, sm.Trials, sm.Stuck, sm.HitCap, sm.AgencyFrac)
		}
	}
}

// TestWellTypedGrammarIsPlayable: every well-typed base spec is playable AND ends
// by its own end condition (no stalemate reliance).
func TestWellTypedGrammarIsPlayable(t *testing.T) {
	specs := Enumerate()
	if len(specs) == 0 {
		t.Fatal("Enumerate returned no specs")
	}
	for _, s := range specs {
		if !s.WellTyped() {
			t.Errorf("Enumerate yielded a non-well-typed spec: %s", s)
		}
		sm := Runner{Spec: s}.Playability(testTrials, 7)
		if !sm.Playable() {
			t.Errorf("well-typed %s not playable: stuck=%d cap=%d agency=%.2f",
				s.Family(), sm.Stuck, sm.HitCap, sm.AgencyFrac)
		}
		if !sm.NaturalEnd() {
			t.Errorf("well-typed %s relies on the stalemate fallback (%d/%d via stalemate)",
				s.Family(), sm.Stalemate, sm.Trials)
		}
	}
}

// TestBaseFamiliesAreCanonical: the well-typed BASE grammar reproduces exactly the
// skeletons it covers -- no spurious novel base families (the score axis is inert
// on these ends). All novelty must come from the modifier axis.
func TestBaseFamiliesAreCanonical(t *testing.T) {
	canon := CanonicalFamilies()
	base := Families(Enumerate())
	if len(base) != len(canon) {
		t.Errorf("base grammar has %d families, want %d (canonical only)", len(base), len(canon))
	}
	for f := range base {
		if !canon[f] {
			t.Errorf("base grammar yielded a non-canonical family %q (score axis should be inert)", f)
		}
	}
}

// TestSafetyAndLivenessHoldUntyped: even the LOOSE cross-product (mis-typed and
// agency-dead included) never gets stuck and always terminates -- the two
// structural guarantees are independent of the coherence type.
func TestSafetyAndLivenessHoldUntyped(t *testing.T) {
	for _, s := range EnumerateAll() {
		sm := Runner{Spec: s}.Playability(testTrials, 7)
		if sm.Stuck > 0 {
			t.Errorf("SAFETY VIOLATION: %s got stuck (%d/%d)", s, sm.Stuck, sm.Trials)
		}
		if sm.HitCap > 0 {
			t.Errorf("LIVENESS VIOLATION: %s hit the turn cap (%d/%d)", s, sm.HitCap, sm.Trials)
		}
	}
}

// TestModifiersPreservePlayability: every modified family stays playable-by-
// construction -- modifiers never introduce a stuck or non-terminating spec.
func TestModifiersPreservePlayability(t *testing.T) {
	mod := EnumerateModified()
	baseFams := len(Families(Enumerate()))
	modFams := len(Families(mod))
	if modFams <= baseFams {
		t.Fatalf("modifier axis did not expand the family space: %d <= %d families", modFams, baseFams)
	}
	for _, s := range mod {
		if !s.WellTyped() {
			t.Errorf("EnumerateModified yielded a non-well-typed spec: %s", s)
		}
		sm := Runner{Spec: s}.Playability(testTrials, 13)
		if sm.Stuck > 0 || sm.HitCap > 0 {
			t.Errorf("modifier broke playable-by-construction: %s stuck=%d cap=%d", s, sm.Stuck, sm.HitCap)
		}
		if !sm.Playable() {
			t.Errorf("modified family not playable: %s agency=%.2f", s.Family(), sm.AgencyFrac)
		}
	}
}

// TestModifierTyping pins the compatibility rules (the lifted v2 whitelist).
func TestModifierTyping(t *testing.T) {
	shedding := GameSpec{Move: PlayMatch, Match: MatchEither, End: EmptyHand, Score: FirstOut}
	climbing := GameSpec{Move: BeatOrPass, End: EmptyHand, Score: FirstOut}
	banking := GameSpec{Move: Accumulate, End: Bust, Score: ClosestTarget}
	casinoCap := GameSpec{Move: Capture, End: DeckOut, Score: MostCaptured}
	trick := GameSpec{Move: Trick, End: DeckOut, Score: MostCaptured}
	rummy := GameSpec{Move: Rummy, End: DeckOut, Score: FewestDeadwood}

	cases := []struct {
		m    Modifier
		spec GameSpec
		want bool
	}{
		{ModRunPlay, shedding, true},
		{ModRunPlay, climbing, false},
		{ModFollowSuit, shedding, true},
		{ModFollowSuit, banking, false},
		{ModKnock, shedding, true},
		{ModKnock, climbing, true},
		{ModKnock, rummy, true},    // Gin go-out by deadwood
		{ModKnock, banking, false}, // bust end, not a hand race
		{ModDrawPenalty, shedding, true},
		{ModDrawPenalty, climbing, false}, // v2 allows it; grammar scopes it to the match-shed gen
		{ModMeldBonus, casinoCap, true},   // banks set/run bonus on top of the capture count (v2 casino)
		{ModMeldBonus, trick, true},       // win tricks AND form melds from the won pile
		{ModMeldBonus, shedding, false},
		{ModAvoidance, trick, true},     // Hearts: penalty cards in won tricks count against you
		{ModAvoidance, casinoCap, true}, // Scopa penalty-suit
		{ModAvoidance, shedding, false}, // no won pile to penalize
		{ModWild, rummy, true},          // deuces/eights-wild: completes melds
		{ModWild, shedding, false},      // still non-productive on shedding
		{ModTrump, trick, true},         // Spades/Bridge trump
		{ModTrump, shedding, false},
		{ModSkip, shedding, true}, // Uno skip
		{ModSkip, trick, false},
		{ModForceDraw, shedding, true}, // Uno draw-two
		{ModForceDraw, casinoCap, false},
	}
	for _, c := range cases {
		if got := c.m.CompatibleWith(c.spec); got != c.want {
			t.Errorf("%s.CompatibleWith(%s) = %v, want %v", c.m, c.spec.Family(), got, c.want)
		}
	}
}

// TestKnockChangesWinCondition: with ModKnock a game can end by fewest-cards
// (the Phase=PhaseEnd path), not only by emptying a hand.
func TestKnockTerminatesByFewest(t *testing.T) {
	s := GameSpec{Players: 4, Deal: 7, Shared: 1, Move: PlayMatch, Match: MatchEither,
		End: EmptyHand, Score: FirstOut, Mods: []Modifier{ModKnock}}
	if !s.WellTyped() {
		t.Fatal("shedding+knock should be well-typed")
	}
	sm := Runner{Spec: s}.Playability(testTrials, 5)
	if !sm.Playable() {
		t.Errorf("shedding+knock not playable: stuck=%d cap=%d agency=%.2f", sm.Stuck, sm.HitCap, sm.AgencyFrac)
	}
}

// TestModCapEnforced: WellTyped rejects more modifiers than the cap.
func TestModCapEnforced(t *testing.T) {
	s := GameSpec{Move: PlayMatch, Match: MatchEither, End: EmptyHand, Score: FirstOut,
		Mods: []Modifier{ModRunPlay, ModFollowSuit, ModDrawPenalty, ModKnock}}
	if s.WellTyped() {
		t.Errorf("spec with %d > %d modifiers should not be well-typed", len(s.Mods), modCap)
	}
}

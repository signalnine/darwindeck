package grammar

import (
	"testing"

	"github.com/darwindeck/darwindeck/pkg/sim"
)

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
	trick4 := GameSpec{Players: 4, Move: Trick, End: DeckOut, Score: MostCaptured}
	rummy := GameSpec{Move: Rummy, End: DeckOut, Score: FewestDeadwood}

	cases := []struct {
		m    Modifier
		spec GameSpec
		want bool
	}{
		{ModRunPlay, shedding, true},
		{ModRunPlay, climbing, true}, // Big Two: lead a set, beat with a higher same-size set
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
		{ModBid, trick, true}, // Spades/Oh Hell contract bid
		{ModBid, shedding, false},
		{ModTeams, trick4, true}, // 2v2 partnerships
		{ModTeams, trick, false}, // needs exactly 4 seats
		{ModTeams, shedding, false},
		{ModSkip, shedding, true}, // Uno skip
		{ModSkip, trick, false},
		{ModForceDraw, shedding, true}, // Uno draw-two
		{ModForceDraw, casinoCap, false},
		{ModNominate, shedding, true}, // Crazy Eights: 8 names the suit
		{ModNominate, trick, false},
		{ModReverse, shedding, true}, // Uno reverse
		{ModReverse, trick, false},
		{ModSumCapture, casinoCap, true}, // Scopa building capture
		{ModSumCapture, shedding, false},
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

// TestBidComposesWithScoringMods: on a trick+bid host the co-typed scoring
// modifiers must COMPOSE with the contract score, never be deadened by it
// (Pinochle precedent: bidding and melds coexist). Pinned on a constructed
// terminal state where the adjustment flips the winner vs the bid-only spec.
func TestBidComposesWithScoringMods(t *testing.T) {
	base := GameSpec{Players: 2, Deal: 13, Move: Trick, End: DeckOut, Score: MostCaptured}
	bidOnly, bidMeld, bidAvoid := base, base, base
	bidOnly.Mods = []Modifier{ModBid}
	bidMeld.Mods = []Modifier{ModBid, ModMeldBonus}
	bidAvoid.Mods = []Modifier{ModBid, ModAvoidance}
	for _, s := range []GameSpec{bidOnly, bidMeld, bidAvoid} {
		if !s.WellTyped() {
			t.Fatalf("%s should be well-typed", s.Family())
		}
	}
	// p0: bid 2, took 2 tricks (banked 4 cards) -> contract 20; pile has the
	// Queen of Spades + a heart (avoidance penalty 14) and no melds.
	// p1: bid 1, took 1 trick -> contract 10; pile is a four-of-a-kind
	// (meld bonus 20) holding one heart (avoidance penalty 1).
	state := func() *sim.GameState {
		return &sim.GameState{
			NumPlayers: 2,
			Scores:     []int{4, 2},
			Bids:       []int{2, 1},
			Tableau: [][]sim.Card{
				{{Rank: 12, Suit: sim.Spades}, {Rank: 13, Suit: sim.Hearts}, {Rank: 2, Suit: sim.Clubs}, {Rank: 9, Suit: sim.Diamonds}},
				{{Rank: 5, Suit: sim.Clubs}, {Rank: 5, Suit: sim.Diamonds}, {Rank: 5, Suit: sim.Hearts}, {Rank: 5, Suit: sim.Spades}},
			},
		}
	}
	if w := (Runner{bidOnly}).score(state()); w != 0 {
		t.Fatalf("bid-only winner = %d, want 0 (contract 20 vs 10)", w)
	}
	if w := (Runner{bidMeld}).score(state()); w != 1 {
		t.Errorf("bid+meld_bonus winner = %d, want 1 -- meld_bonus is inert on the bid spec", w)
	}
	if w := (Runner{bidAvoid}).score(state()); w != 1 {
		t.Errorf("bid+avoidance winner = %d, want 1 -- avoidance is inert on the bid spec", w)
	}
}

// TestNominateComboSuitChoice: under run_play+nominate a combo whose LAST card
// is an 8 must carry the 4-suit nomination choice (Move.Amount), exactly like
// single-card 8s -- otherwise Apply nominates Amount's zero value (always clubs).
func TestNominateComboSuitChoice(t *testing.T) {
	s := GameSpec{Players: 2, Deal: 5, Shared: 1, Move: PlayMatch, Match: MatchEither,
		End: EmptyHand, Score: FirstOut, Mods: []Modifier{ModRunPlay, ModNominate}}
	if !s.WellTyped() {
		t.Fatal("shedding+run_play+nominate should be well-typed")
	}
	top := sim.Card{Rank: 3, Suit: sim.Clubs}
	gs := &sim.GameState{
		NumPlayers: 2,
		TopCard:    &top, // 8C matches by suit, so the 8-pair combo is generated
		Hands: [][]sim.Card{
			{{Rank: 8, Suit: sim.Clubs}, {Rank: 8, Suit: sim.Diamonds}},
			{{Rank: 4, Suit: sim.Hearts}},
		},
	}
	amounts := map[int]bool{}
	for _, m := range (Runner{s}).LegalMoves(gs) {
		if m.Type == sim.MovePlay && len(m.Cards) == 2 {
			amounts[m.Amount] = true
		}
	}
	if len(amounts) != 4 {
		t.Errorf("8-pair combo carries %d nomination variants, want 4 (suits)", len(amounts))
	}
}

// TestFollowSuitExemptsNominateEights: with follow_suit+nominate the rulebook
// promises "you may play an eight on anything" -- rank-8 plays must survive the
// follow-suit filter even when the player holds the led suit.
func TestFollowSuitExemptsNominateEights(t *testing.T) {
	s := GameSpec{Players: 2, Deal: 5, Shared: 1, Move: PlayMatch, Match: MatchEither,
		End: EmptyHand, Score: FirstOut, Mods: []Modifier{ModFollowSuit, ModNominate}}
	if !s.WellTyped() {
		t.Fatal("shedding+follow_suit+nominate should be well-typed")
	}
	top := sim.Card{Rank: 13, Suit: sim.Hearts}
	gs := &sim.GameState{
		NumPlayers: 2,
		TopCard:    &top,
		Hands: [][]sim.Card{
			{{Rank: 5, Suit: sim.Hearts}, {Rank: 8, Suit: sim.Clubs}}, // holds the led suit AND an off-suit 8
			{{Rank: 4, Suit: sim.Diamonds}},
		},
	}
	sawEight, sawHeart := false, false
	for _, m := range (Runner{s}).LegalMoves(gs) {
		if m.Type != sim.MovePlay {
			continue
		}
		switch {
		case int(m.Cards[0].Rank) == wildRank:
			sawEight = true
		case m.Cards[0].Suit == sim.Hearts:
			sawHeart = true
		}
	}
	if !sawEight {
		t.Error("follow-suit filtered the off-suit 8 -- nominate 8s must be exempt (wild)")
	}
	if !sawHeart {
		t.Error("the led-suit play should still be offered")
	}
}

// TestModifiedFamilyCountPinned pins the modified-family census the coverage
// survey reports (results/2026-06-26-grammar-coverage). A drift means a
// primitive/typing change altered the reachable space -- make it deliberate.
func TestModifiedFamilyCountPinned(t *testing.T) {
	if n := len(Families(EnumerateModified())); n != 137 {
		t.Errorf("modified family count = %d, want 137", n)
	}
}

// TestModifiedFamilySweepFullDomain runs EVERY modified family across the FULL
// players x deal domains from enumerate.go (EnumerateModified keeps only one
// representative point per family), asserting never-stuck and terminated at
// every combination. Skipped under -short: it plays the whole ~680-combo grid.
func TestModifiedFamilySweepFullDomain(t *testing.T) {
	if testing.Short() {
		t.Skip("full players x deal sweep across every modified family; run without -short")
	}
	const trials = 8
	combos := 0
	for _, rep := range EnumerateModified() {
		for _, players := range enumPlayers {
			for _, deal := range dealFor(rep.Move) {
				s := rep
				s.Players, s.Deal = players, deal
				if !s.WellTyped() { // e.g. teams needs exactly 4 seats
					continue
				}
				combos++
				sm := Runner{Spec: s}.Playability(trials, uint64(combos)*0x10000+31)
				if sm.Stuck > 0 {
					t.Errorf("SAFETY VIOLATION: %s stuck %d/%d", s, sm.Stuck, sm.Trials)
				}
				if sm.HitCap > 0 {
					t.Errorf("LIVENESS VIOLATION: %s hit the turn cap %d/%d", s, sm.HitCap, sm.Trials)
				}
				if sm.Terminated != sm.Trials {
					t.Errorf("%s terminated only %d/%d trials", s, sm.Terminated, sm.Trials)
				}
			}
		}
	}
	if min := len(Families(EnumerateModified())); combos < min {
		t.Errorf("sweep covered %d combos, fewer than the %d families", combos, min)
	}
}

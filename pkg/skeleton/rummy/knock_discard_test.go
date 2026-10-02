package rummy

import (
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

// Regression tests for the 2026-10 bughunt rummy findings: the knock is the
// real rule (discard, THEN knock -- judged on the hand you keep), ties follow
// the undercut rule, and the density probe uses the same knockable test.

func knockGenome(threshold int) *genome.Genome {
	return &genome.Genome{
		Skeleton: genome.Rummy, Players: 2, HandSize: 10,
		Rummy: &genome.RummyParams{MeldTypes: genome.MeldBoth, MinMeldSize: 3, DrawFrom: genome.DrawDeck, KnockThreshold: threshold},
	}
}

func card(r sim.Rank, s sim.Suit) sim.Card { return sim.Card{Rank: r, Suit: s} }

// elevenCardHand: nine melded cards (2-3-4 of hearts, three 7s, J-Q-K of
// spades) + the 5 of diamonds (deadwood 5) + a just-drawn king of clubs (10).
// Deadwood on all eleven cards is 15; after discarding the king it is 5.
func elevenCardHand() []sim.Card {
	return []sim.Card{
		card(2, sim.Hearts), card(3, sim.Hearts), card(4, sim.Hearts),
		card(7, sim.Clubs), card(7, sim.Diamonds), card(7, sim.Spades),
		card(sim.Jack, sim.Spades), card(sim.Queen, sim.Spades), card(sim.King, sim.Spades),
		card(5, sim.Diamonds), card(sim.King, sim.Clubs),
	}
}

func meldPhaseState(hand []sim.Card) *sim.GameState {
	st := sim.NewGameState(2)
	st.Hands[0] = append([]sim.Card(nil), hand...)
	st.Hands[1] = []sim.Card{card(9, sim.Clubs), card(sim.Queen, sim.Hearts)}
	st.Discard = []sim.Card{card(6, sim.Clubs)}
	st.Deck = []sim.Card{card(8, sim.Hearts), card(8, sim.Clubs)}
	st.Phase = sim.PhaseMeld
	st.Melds = [][]sim.Card{}
	st.MeldOwner = []int{}
	return st
}

func knockMove(moves []sim.Move) (sim.Move, bool) {
	for _, m := range moves {
		if m.Type == sim.MoveKnock {
			return m, true
		}
	}
	return sim.Move{}, false
}

// TestKnockJudgedOnPostDiscardHand: the knock used to be offered in the meld
// phase only when the deadwood of ALL eleven cards (the drawn card included)
// met the threshold, and the knocker kept the eleventh card as deadwood. In
// real knock rummy you draw, discard, and knock on the hand you KEEP: with the
// hand above and threshold 10 you throw the king and knock on 5.
func TestKnockJudgedOnPostDiscardHand(t *testing.T) {
	g := knockGenome(10)
	st := meldPhaseState(elevenCardHand())
	mv, ok := knockMove((&Runner{}).GenerateMoves(st, g))
	if !ok {
		t.Fatal("deadwood after discarding the king is 5 (<= 10): a knock must be offered")
	}
	if len(mv.Cards) != 1 || mv.Cards[0] != card(sim.King, sim.Clubs) {
		t.Fatalf("the knock must carry its discard (the king of clubs, the least-deadwood discard), got %v", mv.Cards)
	}
}

// TestKnockDiscardNotCountedAsDeadwood: applying the knock moves the discard to
// the pile, lays the melds, and banks only the kept hand's deadwood.
func TestKnockDiscardNotCountedAsDeadwood(t *testing.T) {
	g := knockGenome(10)
	r := &Runner{}
	st := meldPhaseState(elevenCardHand())
	mv, ok := knockMove(r.GenerateMoves(st, g))
	if !ok {
		t.Fatal("knock not offered")
	}
	events := r.ApplyMove(st, mv, g)

	if top := st.Discard[len(st.Discard)-1]; top != card(sim.King, sim.Clubs) {
		t.Errorf("knock discard must land on the discard pile, top = %v", top)
	}
	if len(st.Hands[0]) != 1 || st.Hands[0][0] != card(5, sim.Diamonds) {
		t.Errorf("knocker keeps only the unmelded 5 of diamonds, hand = %v", st.Hands[0])
	}
	if len(st.Melds) != 3 {
		t.Errorf("the three melds must be laid down, got %d", len(st.Melds))
	}
	if st.Phase != sim.PhaseEnd {
		t.Errorf("knock must end the round, phase = %v", st.Phase)
	}
	var sawDiscard, sawEnd bool
	for _, e := range events {
		if e.Type == sim.EventCardPlayed && e.Detail == "discard" && len(e.Cards) == 1 && e.Cards[0] == card(sim.King, sim.Clubs) {
			sawDiscard = true
		}
		if e.Type == sim.EventRoundEnd && e.Detail == "knock" {
			sawEnd = true
		}
	}
	if !sawDiscard || !sawEnd {
		t.Errorf("knock must emit the discard and the round end, events = %+v", events)
	}

	r.Upkeep(st, g)
	if st.Scores[0] != -5 {
		t.Errorf("knocker banks the kept hand's deadwood (5), score = %d", st.Scores[0])
	}
	if st.Scores[1] != -19 {
		t.Errorf("opponent banks 9+10 = 19 deadwood, score = %d", st.Scores[1])
	}
	if w := r.CheckEnd(st, g); w != 0 {
		t.Errorf("knocker (deadwood 5 vs 19) must win, got %d", w)
	}
}

// TestKnockNotOfferedAboveThresholdAfterBestDiscard: no discard gets the hand
// to the threshold -> no knock, however small the threshold gap.
func TestKnockNotOfferedAboveThresholdAfterBestDiscard(t *testing.T) {
	g := knockGenome(4) // best discard leaves 5
	st := meldPhaseState(elevenCardHand())
	if _, ok := knockMove((&Runner{}).GenerateMoves(st, g)); ok {
		t.Fatal("best post-discard deadwood is 5 > threshold 4: knock must NOT be offered")
	}

	// Gin-only (threshold 0): offered exactly when a discard leaves no
	// deadwood at all.
	gin := knockGenome(0)
	if _, ok := knockMove((&Runner{}).GenerateMoves(st, gin)); ok {
		t.Fatal("threshold 0: 5 deadwood after the best discard is not gin")
	}
	ginHand := elevenCardHand()
	ginHand[9] = card(8, sim.Diamonds) // replaces the 5: still deadwood
	ginHand[10] = card(5, sim.Hearts)  // extends 2-3-4 of hearts to 2-3-4-5
	st = meldPhaseState(ginHand)
	mv, ok := knockMove((&Runner{}).GenerateMoves(st, gin))
	if !ok || mv.Cards[0] != card(8, sim.Diamonds) {
		t.Fatalf("threshold 0: discarding the 8 leaves ten melded cards (gin); want a knock carrying the 8, got %v ok=%v", mv.Cards, ok)
	}
	r := &Runner{}
	events := r.ApplyMove(st, mv, gin)
	if len(st.Hands[0]) != 0 {
		t.Errorf("gin: every kept card melds, hand = %v", st.Hands[0])
	}
	if last := events[len(events)-1]; last.Type != sim.EventRoundEnd || last.Detail != "gin" {
		t.Errorf("a knock that leaves no deadwood is gin; last event = %+v", last)
	}
}

// TestKnockDiscardBreaksTiesDeterministically: among discards that leave the
// same (least) deadwood the knock takes the highest-ranked card, then the
// lowest suit -- independent of hand order, so the rulebook can state it.
func TestKnockDiscardBreaksTiesDeterministically(t *testing.T) {
	g := knockGenome(30)
	// No melds: discarding either king leaves 10+2+3 = 15; the queen ties on
	// value (10) but ranks lower.
	hands := [][]sim.Card{
		{card(sim.King, sim.Spades), card(sim.Queen, sim.Hearts), card(sim.King, sim.Clubs), card(2, sim.Diamonds), card(3, sim.Hearts)},
		{card(3, sim.Hearts), card(sim.King, sim.Clubs), card(2, sim.Diamonds), card(sim.Queen, sim.Hearts), card(sim.King, sim.Spades)},
	}
	for _, hand := range hands {
		st := meldPhaseState(hand)
		mv, ok := knockMove((&Runner{}).GenerateMoves(st, g))
		if !ok {
			t.Fatalf("knock must be offered for hand %v", hand)
		}
		if mv.Cards[0] != card(sim.King, sim.Clubs) {
			t.Errorf("hand %v: knock discard = %v, want the king of clubs (highest rank, then lowest suit)", hand, mv.Cards[0])
		}
	}
}

// TestBareKnockStillAppliesTheRule: a knock applied without an explicit discard
// (older callers, hand-built moves) discards the same least-deadwood card
// rather than keeping the extra card as deadwood.
func TestBareKnockStillAppliesTheRule(t *testing.T) {
	g := knockGenome(10)
	r := &Runner{}
	st := meldPhaseState(elevenCardHand())
	r.ApplyMove(st, sim.Move{Type: sim.MoveKnock, PlayerID: 0}, g)
	r.Upkeep(st, g)
	if st.Scores[0] != -5 {
		t.Errorf("bare knock: knocker banks %d, want -5 (the king discarded, not kept)", st.Scores[0])
	}
}

// TestRummyTieUndercut: equal deadwood no longer goes to the lowest seat. A
// knocker who is tied loses (the undercut: the tied opponent nearest after the
// knocker wins); a player who goes out with NO deadwood (gin) wins any tie.
func TestRummyTieUndercut(t *testing.T) {
	g := knockGenome(10)
	g.Players = 3
	r := &Runner{}
	run := func(knocker int, hands [][]sim.Card) int {
		st := sim.NewGameState(3)
		for i := range hands {
			st.Hands[i] = append([]sim.Card(nil), hands[i]...)
		}
		st.Discard = []sim.Card{card(6, sim.Clubs)}
		st.Phase = sim.PhaseMeld
		st.Active = knocker
		mv, ok := knockMove(r.GenerateMoves(st, g))
		if !ok {
			t.Fatalf("knocker %d: knock not offered for %v", knocker, st.Hands[knocker])
		}
		r.ApplyMove(st, mv, g)
		r.Upkeep(st, g)
		return r.CheckEnd(st, g)
	}
	// Each "kept" hand is 5 deadwood; the knocker also holds a king to discard.
	five := func(s sim.Suit) []sim.Card { return []sim.Card{card(2, s), card(3, sim.Suit((int(s)+1)%4))} }
	withKing := func(h []sim.Card, s sim.Suit) []sim.Card {
		return append(append([]sim.Card(nil), h...), card(sim.King, s))
	}
	high := []sim.Card{card(9, sim.Clubs), card(sim.Queen, sim.Hearts)} // 19

	if w := run(0, [][]sim.Card{withKing(five(sim.Clubs), sim.Clubs), five(sim.Diamonds), high}); w != 1 {
		t.Errorf("knocker P0 tied at 5 with P1: winner = %d, want 1 (undercut)", w)
	}
	if w := run(1, [][]sim.Card{five(sim.Clubs), withKing(five(sim.Diamonds), sim.Clubs), high}); w != 0 {
		t.Errorf("knocker P1 tied at 5 with P0: winner = %d, want 0 (undercut; seat order must not rescue the knocker)", w)
	}
	if w := run(1, [][]sim.Card{five(sim.Clubs), withKing(five(sim.Diamonds), sim.Clubs), five(sim.Hearts)}); w != 2 {
		t.Errorf("three-way tie, P1 knocks: winner = %d, want 2 (nearest after the knocker)", w)
	}
	if w := run(2, [][]sim.Card{high, high, withKing(five(sim.Hearts), sim.Clubs)}); w != 2 {
		t.Errorf("knocker strictly lowest: winner = %d, want 2", w)
	}
	if w := run(0, [][]sim.Card{withKing(five(sim.Clubs), sim.Clubs), {card(2, sim.Hearts)}, high}); w != 1 {
		t.Errorf("opponent strictly lower than the knocker: winner = %d, want 1", w)
	}

	// Gin wins ties: the knocker keeps three 7s (no deadwood) while P1 holds a
	// complete meld too (deadwood 0).
	ginHand := []sim.Card{card(7, sim.Clubs), card(7, sim.Diamonds), card(7, sim.Spades), card(sim.King, sim.Clubs)}
	p1 := []sim.Card{card(9, sim.Clubs), card(9, sim.Diamonds), card(9, sim.Spades)}
	if w := run(0, [][]sim.Card{ginHand, p1, high}); w != 0 {
		t.Errorf("gin tied at 0 deadwood with P1: winner = %d, want 0 (gin cannot be undercut)", w)
	}
}

// TestChoiceMattersVoidsKnockableDiscard: the density probe's end-at-will
// voiding must use the SAME knockable test as the move generator. With the
// eleven-card hand above the player could have knocked this turn (5 after the
// best discard, threshold 10), so the discard micro-choice is subordinate --
// even though the eleven cards' own deadwood (15) exceeds the threshold.
func TestChoiceMattersVoidsKnockableDiscard(t *testing.T) {
	g := knockGenome(10)
	r := &Runner{}
	st := meldPhaseState(elevenCardHand())
	st.Phase = sim.PhaseDiscard
	moves := r.GenerateMoves(st, g)
	if len(moves) != 11 {
		t.Fatalf("premise: 11 discards, got %d", len(moves))
	}
	if r.ChoiceMatters(st, g, moves) {
		t.Error("knock was available this turn: the discard choice must be voided (end-at-will)")
	}
	// Not knockable (threshold 4): the discard choice is live again.
	tight := knockGenome(4)
	if !r.ChoiceMatters(st, tight, moves) {
		t.Error("no knock available: a discard that changes deadwood must be meaningful")
	}
}

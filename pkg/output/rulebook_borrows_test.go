package output

import (
	"strings"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
)

// TestRulebookMeldBonusOnAvoidanceHost: where the lowest score wins
// (trick_scoring=avoidance) the meld hook SUBTRACTS the meld points, so the
// rulebook must say they come off your score -- "score points" next to
// "Lowest score wins" described a penalty as a bonus.
func TestRulebookMeldBonusOnAvoidanceHost(t *testing.T) {
	g := seeds.Hearts()
	g.Borrowed = []genome.BorrowedMechanic{{Source: genome.Rummy, Mechanic: genome.MechMeldBonus}}
	if errs := genome.Validate(g); len(errs) != 0 {
		t.Fatalf("fixture invalid: %v", errs)
	}
	sec := additionalRules(t, GenerateRulebook(g))
	if !strings.Contains(sec, "subtracted from your score") {
		t.Errorf("lowest-wins host: meld bonus text must say the points are subtracted:\n%s", sec)
	}

	// Highest-wins host: plain "score points", no subtraction clause.
	w := seeds.Whist()
	w.Borrowed = g.Borrowed
	sec = additionalRules(t, GenerateRulebook(w))
	if strings.Contains(sec, "subtracted") {
		t.Errorf("highest-wins host must not claim meld points are subtracted:\n%s", sec)
	}
	if !strings.Contains(sec, "Meld bonus") || !strings.Contains(sec, "5 points per card") {
		t.Errorf("meld bonus magnitudes missing:\n%s", sec)
	}
}

// TestRulebookCaptureBonusSheddingWording: a shedding game has no tricks and no
// laid-down melds. The trick-scoring borrow counts the cards a player SHED this
// round (the runner records each discard in the player's tableau), so the
// rulebook must not talk about won tricks.
func TestRulebookCaptureBonusSheddingWording(t *testing.T) {
	g := sheddingBase()
	g.Shedding.RoundsPerGame = 3
	g.Borrowed = []genome.BorrowedMechanic{{Source: genome.TrickTaking, Mechanic: genome.MechTrickScoring}}
	sec := additionalRules(t, GenerateRulebook(g))
	for _, banned := range []string{"won tricks", "laid-down melds", "captured"} {
		if strings.Contains(sec, banned) {
			t.Errorf("shedding capture-bonus rule mentions %q, which does not exist in this game:\n%s", banned, sec)
		}
	}
	for _, want := range []string{"shed the most cards", "equal to the number of cards", "split the bonus evenly"} {
		if !strings.Contains(sec, want) {
			t.Errorf("shedding capture-bonus rule must state %q:\n%s", want, sec)
		}
	}

	// Rummy host: melds only, no tricks.
	r := seeds.KnockRummy()
	r.Borrowed = []genome.BorrowedMechanic{{Source: genome.TrickTaking, Mechanic: genome.MechTrickScoring}}
	sec = additionalRules(t, GenerateRulebook(r))
	if strings.Contains(sec, "won tricks") {
		t.Errorf("rummy capture-bonus rule mentions tricks, which do not exist in this game:\n%s", sec)
	}
	if !strings.Contains(sec, "laid down the most cards in melds") {
		t.Errorf("rummy capture-bonus rule must count laid-down melds:\n%s", sec)
	}
}

// TestRulebookRunPlayStatesNewTop: after a combination play the runner makes the
// group's LAST card the discard top (the highest card of a run; for a set, the
// last such card in hand order) and fires only that card's special effect. The
// next player has to match that card, so the rulebook must say which it is.
func TestRulebookRunPlayStatesNewTop(t *testing.T) {
	g := sheddingBase()
	g.Borrowed = []genome.BorrowedMechanic{{Source: genome.Climbing, Mechanic: genome.MechRunPlay}}
	sec := additionalRules(t, GenerateRulebook(g))
	for _, want := range []string{"last card becomes the new top", "highest card of a run", "only that card's special effect"} {
		if !strings.Contains(sec, want) {
			t.Errorf("run-play rule must state %q:\n%s", want, sec)
		}
	}
}

// TestRulebookDrawPenaltyGoingOutException: the draw-penalty hook exempts a
// play that goes out (empties the hand or ends the round). The rulebook must
// state the exception, and on a rummy host the trigger is a DISCARD.
func TestRulebookDrawPenaltyGoingOutException(t *testing.T) {
	c := seeds.BigTwo()
	c.HandSize = 10 // leave a draw pile
	c.Borrowed = []genome.BorrowedMechanic{{Source: genome.Rummy, Mechanic: genome.MechDrawPenalty}}
	sec := additionalRules(t, GenerateRulebook(c))
	for _, want := range []string{"face card (Jack or higher)", "draw 1 extra card", "Going out is exempt", "deck is empty"} {
		if !strings.Contains(sec, want) {
			t.Errorf("climbing draw-penalty rule must state %q:\n%s", want, sec)
		}
	}

	r := seeds.KnockRummy()
	r.Borrowed = []genome.BorrowedMechanic{{Source: genome.Shedding, Mechanic: genome.MechDrawPenalty}}
	sec = additionalRules(t, GenerateRulebook(r))
	if !strings.Contains(sec, "discard a face card (Jack or higher)") {
		t.Errorf("rummy draw-penalty rule must be triggered by a discard:\n%s", sec)
	}
	if !strings.Contains(sec, "Going out is exempt") {
		t.Errorf("rummy draw-penalty rule must state the going-out exception:\n%s", sec)
	}
}

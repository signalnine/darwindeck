package webplay

import (
	"testing"

	"github.com/darwindeck/darwindeck/pkg/seeds"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

// TestKnockLabelNamesTheDiscard: the rummy knock is "discard X and knock" (the
// runner scores the hand KEPT after the knock discard, and the move carries
// that discard in Cards[0]). The button must say which card goes; a
// shedding/climbing knock carries no card and stays "Knock".
func TestKnockLabelNamesTheDiscard(t *testing.T) {
	g := seeds.GinRummy()
	st := sim.NewGameState(2)
	rummyKnock := sim.Move{Type: sim.MoveKnock, Cards: []sim.Card{{Suit: sim.Clubs, Rank: sim.King}}}
	if got, want := moveLabel(rummyKnock, st, g), "Discard "+cardList(rummyKnock.Cards)+" and knock"; got != want {
		t.Errorf("rummy knock label = %q, want %q", got, want)
	}
	if got := moveLabel(sim.Move{Type: sim.MoveKnock}, st, g); got != "Knock" {
		t.Errorf("bare knock label = %q, want \"Knock\"", got)
	}
}

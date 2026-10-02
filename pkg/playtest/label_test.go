package playtest

import (
	"testing"

	"github.com/darwindeck/darwindeck/pkg/sim"
)

// TestDescribeKnockWithDiscard: a rummy knock is "discard X and knock" (the
// move carries its discard in Cards[0]); the human must see WHICH card the
// knock throws away. A shedding/climbing knock carries no card and stays a
// plain "Knock".
func TestDescribeKnockWithDiscard(t *testing.T) {
	rummy := sim.Move{Type: sim.MoveKnock, Cards: []sim.Card{{Suit: sim.Clubs, Rank: sim.King}}}
	if got, want := describeMoveShort(rummy), "Discard KC and knock"; got != want {
		t.Errorf("rummy knock label = %q, want %q", got, want)
	}
	if got, want := describeMoveShort(sim.Move{Type: sim.MoveKnock}), "Knock"; got != want {
		t.Errorf("bare knock label = %q, want %q", got, want)
	}
}

package output

import (
	"strings"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
)

// TestRulebookSheddingBlockedText: the shedding turn rules must match the
// runner -- you must play if you can, a draw ends your turn, the discard pile
// is recycled when the deck runs out, and a game that is truly blocked (no
// deck, nothing to recycle, nobody can play) has no winner. The multi-round
// rulebook never mentioned the blocked case at all.
func TestRulebookSheddingBlockedText(t *testing.T) {
	single := GenerateRulebook(seeds.CrazyEights())
	for _, want := range []string{
		"If you can play, you must",
		"your turn ends",
		"shuffled to form a new deck",
		"blocked",
		"ends in a draw",
	} {
		if !strings.Contains(single, want) {
			t.Errorf("single-round shedding rulebook must state %q:\n%s", want, single)
		}
	}

	multi := sheddingBase()
	multi.Shedding.RoundsPerGame = 3
	multi.Borrowed = []genome.BorrowedMechanic{{Source: genome.Rummy, Mechanic: genome.MechMeldBonus}}
	rb := GenerateRulebook(multi)
	for _, want := range []string{"shuffled to form a new deck", "blocked", "no winner"} {
		if !strings.Contains(rb, want) {
			t.Errorf("multi-round shedding rulebook must state %q:\n%s", want, rb)
		}
	}
}

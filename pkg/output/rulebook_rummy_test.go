package output

import (
	"strings"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
)

// TestRulebookRummyAceHighRuns: the runner ranks the ace HIGH in runs (Q-K-A
// melds, A-2-3 does not) while counting it 1 point as deadwood. The rulebook
// only said "Ace: 1 point", which reads as ace-low everywhere.
func TestRulebookRummyAceHighRuns(t *testing.T) {
	for _, g := range []*genome.Genome{seeds.GinRummy(), seeds.KnockRummy()} {
		rb := GenerateRulebook(g)
		for _, want := range []string{"Q-K-A", "A-2-3", "Ace: 1 point"} {
			if !strings.Contains(rb, want) {
				t.Errorf("%s rulebook must state %q (ace high in runs, 1 point as deadwood):\n%s", g.ID, want, rb)
			}
		}
	}
	// Sets-only game: there are no runs, so no run note.
	setsOnly := seeds.GinRummy()
	setsOnly.Rummy.MeldTypes = genome.MeldSets
	if rb := GenerateRulebook(setsOnly); strings.Contains(rb, "Q-K-A") {
		t.Errorf("sets-only rummy must not describe runs:\n%s", rb)
	}
}

// TestRulebookRummyKnockProcedure: the rulebook must describe the knock the
// runner implements -- it replaces the ordinary discard, is judged on the hand
// kept AFTER the discard, throws the least-deadwood card, and lays the melds.
// "You may knock when your deadwood is N or less" alone never said which hand
// is measured.
func TestRulebookRummyKnockProcedure(t *testing.T) {
	rb := GenerateRulebook(seeds.KnockRummy())
	for _, want := range []string{
		"You may **knock** when",
		"15 points or less",
		"takes the place of your discard",
		"the hand you keep after that discard",
		"least deadwood",
		"highest-ranked",
	} {
		if !strings.Contains(rb, want) {
			t.Errorf("knock rummy rulebook must state %q:\n%s", want, rb)
		}
	}

	// Threshold 0 is gin-only: no knock threshold sentence, but the going-out
	// procedure is still stated.
	gin := seeds.GinRummy()
	gin.Rummy.KnockThreshold = 0
	rb = GenerateRulebook(gin)
	if !strings.Contains(rb, "You can only go out with **Gin** (no deadwood at all).") {
		t.Errorf("gin-only rulebook lost its gin sentence:\n%s", rb)
	}
	if strings.Contains(rb, "You may **knock** when") {
		t.Errorf("gin-only rulebook must not offer a knock threshold:\n%s", rb)
	}
	if !strings.Contains(rb, "the hand you keep after that discard") {
		t.Errorf("gin-only rulebook must still say the kept hand is what counts:\n%s", rb)
	}
}

// TestRulebookRummyTieRule: equal deadwood is resolved by the undercut rule
// (rummy.roundWinner), which the rulebook must state -- it used to be decided
// silently by seat order.
func TestRulebookRummyTieRule(t *testing.T) {
	rb := GenerateRulebook(seeds.KnockRummy())
	for _, want := range []string{"tied", "undercut", "next in turn order", "no deadwood at all wins any tie"} {
		if !strings.Contains(rb, want) {
			t.Errorf("rummy rulebook must state the tie rule (%q):\n%s", want, rb)
		}
	}
	// Scoring is on best possible melds, laid or not.
	if !strings.Contains(rb, "whether or not") {
		t.Errorf("rummy rulebook must say deadwood counts after the best melds, laid or not:\n%s", rb)
	}
}

package output

import (
	"fmt"
	"strings"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
)

func additionalRules(t *testing.T, rb string) string {
	t.Helper()
	i := strings.Index(rb, "## Additional Rules")
	if i < 0 {
		t.Fatalf("rulebook has no Additional Rules section:\n%s", rb)
	}
	rest := rb[i+len("## Additional Rules"):]
	if j := strings.Index(rest, "\n## "); j >= 0 {
		rest = rest[:j]
	}
	return rest
}

// TestRulebookKnockStatesThresholdAndTieRule: the knock rule used to say "once
// your hand is down to a few cards" and "whoever holds the fewest cards wins"
// -- no number, no tie rule. The runner offers the knock at
// genome.KnockHandThreshold cards and resolves ties by the undercut rule
// (genome.KnockWinner); the rulebook must state both.
func TestRulebookKnockStatesThresholdAndTieRule(t *testing.T) {
	threshold := fmt.Sprintf("%d cards or fewer", genome.KnockHandThreshold)
	knock := genome.BorrowedMechanic{Source: genome.Rummy, Mechanic: genome.MechKnock}

	shed := sheddingBase()
	shed.Borrowed = []genome.BorrowedMechanic{knock}
	climb := seeds.BigTwo()
	climb.Borrowed = []genome.BorrowedMechanic{knock}

	for _, g := range []*genome.Genome{shed, climb} {
		sec := additionalRules(t, GenerateRulebook(g))
		for _, want := range []string{threshold, "end the game at once", "strictly fewer", "soonest after you"} {
			if !strings.Contains(sec, want) {
				t.Errorf("%s knock rule must state %q:\n%s", g.Skeleton, want, sec)
			}
		}
		if strings.Contains(sec, "a few cards") {
			t.Errorf("%s knock rule still says the vague \"a few cards\":\n%s", g.Skeleton, sec)
		}
	}
}

// TestRulebookKnockEndsRoundInMultiRoundGame: on a banked-score multi-round
// shedding host a knock ends the ROUND (hands are scored, the next round is
// dealt). The rulebook must say so, and must not promise that a knock ends the
// game or that the fewest-cards player wins.
func TestRulebookKnockEndsRoundInMultiRoundGame(t *testing.T) {
	g := sheddingBase()
	g.Shedding.RoundsPerGame = 3
	g.Borrowed = []genome.BorrowedMechanic{
		{Source: genome.Rummy, Mechanic: genome.MechMeldBonus},
		{Source: genome.Rummy, Mechanic: genome.MechKnock},
	}
	rb := GenerateRulebook(g)
	sec := additionalRules(t, rb)
	for _, want := range []string{
		fmt.Sprintf("%d cards or fewer", genome.KnockHandThreshold),
		"end the round at once",
		"exactly as if a player had gone out",
	} {
		if !strings.Contains(sec, want) {
			t.Errorf("multi-round knock rule must state %q:\n%s", want, sec)
		}
	}
	for _, banned := range []string{"end the game at once", "whoever holds the fewest cards wins"} {
		if strings.Contains(sec, banned) {
			t.Errorf("multi-round knock rule must not say %q (a knock only ends the round):\n%s", banned, sec)
		}
	}
	// The Rounds paragraph names both ways a round ends.
	i := strings.Index(rb, "### Rounds")
	j := strings.Index(rb, "### Winning")
	if i < 0 || j < i {
		t.Fatalf("no Rounds section:\n%s", rb)
	}
	if rounds := rb[i:j]; !strings.Contains(rounds, "knock") {
		t.Errorf("Rounds section must mention that a knock also ends the round:\n%s", rounds)
	}
}

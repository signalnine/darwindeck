package output

import (
	"strings"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
)

// TestRulebookStatesTieRules: every skeleton whose winner is a score comparison
// states how a tie is resolved, in the words of the rule the runner applies
// (tricktaking.findWinner, casino CheckEnd, vying CheckEnd/resolveShowdown).
// The ties used to be decided silently by seat order.
func TestRulebookStatesTieRules(t *testing.T) {
	cases := []struct {
		g     *genome.Genome
		wants []string
	}{
		{seeds.Whist(), []string{"If scores are tied", "won the final trick", "next in turn order after"}},
		{seeds.Hearts(), []string{"If scores are tied", "won the final trick"}},
		{seeds.Casino(), []string{"If the count is tied", "made the last capture", "next in turn order after"}},
		{seeds.SimplePoker(), []string{"If chips are tied", "won the most recent pot", "next in seat order after"}},
	}
	for _, tc := range cases {
		rb := GenerateRulebook(tc.g)
		for _, want := range tc.wants {
			if !strings.Contains(rb, want) {
				t.Errorf("%s rulebook must state its tie rule (%q):\n%s", tc.g.ID, want, rb)
			}
		}
	}
}

func showdownSection(t *testing.T, rb string) string {
	t.Helper()
	i := strings.Index(rb, "### Showdown")
	j := strings.Index(rb, "### Winning")
	if i < 0 || j < i {
		t.Fatalf("no Showdown section:\n%s", rb)
	}
	return rb[i:j]
}

// TestRulebookVyingHandCategoriesByHandSize: the hand evaluator needs five
// cards for a straight or a flush (vying.eval5), so with 3- or 4-card hands
// those categories cannot occur -- a 3-card 5-6-7 of hearts is just "high
// card" and loses to a pair of twos. The rulebook listed all nine categories
// for every hand size.
func TestRulebookVyingHandCategoriesByHandSize(t *testing.T) {
	g := seeds.SimplePoker()

	g.HandSize = 3
	sec := showdownSection(t, GenerateRulebook(g))
	for _, banned := range []string{"straight", "flush", "full house", "two pair", "four of a kind"} {
		if strings.Contains(sec, banned+",") || strings.Contains(sec, banned+" —") || strings.Contains(sec, ", "+banned) {
			t.Errorf("3-card hands cannot make a %s; rulebook lists it as a winning hand:\n%s", banned, sec)
		}
	}
	for _, want := range []string{"pair", "three of a kind", "no straights or flushes"} {
		if !strings.Contains(sec, want) {
			t.Errorf("3-card rulebook must state %q:\n%s", want, sec)
		}
	}

	g.HandSize = 4
	sec = showdownSection(t, GenerateRulebook(g))
	for _, want := range []string{"pair, two pair, three of a kind, four of a kind", "no straights or flushes"} {
		if !strings.Contains(sec, want) {
			t.Errorf("4-card rulebook must state %q:\n%s", want, sec)
		}
	}
	if strings.Contains(sec, "full house") {
		t.Errorf("4-card hands cannot make a full house:\n%s", sec)
	}

	g.HandSize = 5
	sec = showdownSection(t, GenerateRulebook(g))
	if !strings.Contains(sec, "pair, two pair, three of a kind, straight, flush, full house, four of a kind, straight flush") {
		t.Errorf("5-card rulebook must list all nine categories:\n%s", sec)
	}
	if strings.Contains(sec, "no straights or flushes") || strings.Contains(sec, "best five") {
		t.Errorf("5-card rulebook needs neither the short-hand nor the best-five note:\n%s", sec)
	}
}

// TestRulebookVyingSplitPotAndBestFive: equal hands split the pot (odd chips to
// the tied winner who acts first), and with more than five cards only the best
// five count (vying.HandStrength). Neither was stated.
func TestRulebookVyingSplitPotAndBestFive(t *testing.T) {
	g := seeds.SimplePoker()
	g.HandSize = 7
	sec := showdownSection(t, GenerateRulebook(g))
	for _, want := range []string{"best five", "7 cards", "split", "odd chip", "acts first"} {
		if !strings.Contains(sec, want) {
			t.Errorf("7-card rulebook must state %q:\n%s", want, sec)
		}
	}
	g.HandSize = 5
	sec = showdownSection(t, GenerateRulebook(g))
	for _, want := range []string{"split", "odd chip", "acts first"} {
		if !strings.Contains(sec, want) {
			t.Errorf("5-card rulebook must state the split-pot rule (%q):\n%s", want, sec)
		}
	}
}

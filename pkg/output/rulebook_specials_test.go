package output

import (
	"strings"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
)

func specialSection(t *testing.T, rb string) string {
	t.Helper()
	i := strings.Index(rb, "## Special Cards")
	if i < 0 {
		t.Fatalf("rulebook has no Special Cards section:\n%s", rb)
	}
	rest := rb[i+len("## Special Cards"):]
	if j := strings.Index(rest, "\n## "); j >= 0 {
		rest = rest[:j]
	}
	return rest
}

// TestRulebookStatesOverlappingDrawRule: a card matched by both a draw_two and
// a draw_four rule makes the victim draw 4 (the runner takes the larger). The
// rulebook listed both effects with no word on which fires.
func TestRulebookStatesOverlappingDrawRule(t *testing.T) {
	g := sheddingBase()
	g.SpecialCards = []genome.SpecialCard{
		{Type: genome.SpecialDrawTwo, ByRank: 9},
		{Type: genome.SpecialDrawFour, ByRank: 9, BySuit: 3},
	}
	sec := specialSection(t, GenerateRulebook(g))
	if !strings.Contains(sec, "larger") || !strings.Contains(sec, "not added together") {
		t.Errorf("overlapping draw rules: rulebook must state the larger draw applies and they do not stack:\n%s", sec)
	}

	// No overlap -> no note.
	g.SpecialCards = []genome.SpecialCard{
		{Type: genome.SpecialDrawTwo, ByRank: 9},
		{Type: genome.SpecialDrawFour, ByRank: 4},
	}
	if sec := specialSection(t, GenerateRulebook(g)); strings.Contains(sec, "larger") {
		t.Errorf("disjoint draw rules must not print the overlap note:\n%s", sec)
	}
}

// TestRulebookDrawEffectsMentionLostTurn: draw_two / draw_four also cost the
// victim their turn (applySpecialEffects: skipVictim), which the rulebook's
// "Next player draws N cards" never said.
func TestRulebookDrawEffectsMentionLostTurn(t *testing.T) {
	g := sheddingBase()
	g.SpecialCards = []genome.SpecialCard{
		{Type: genome.SpecialDrawTwo, ByRank: 9},
		{Type: genome.SpecialDrawFour, ByRank: 4},
	}
	sec := specialSection(t, GenerateRulebook(g))
	for _, want := range []string{"draws 2 cards and loses their turn", "draws 4 cards and loses their turn"} {
		if !strings.Contains(sec, want) {
			t.Errorf("rulebook must say %q:\n%s", want, sec)
		}
	}
}

// TestRulebookReverseTwoPlayerText: with two players a reverse hands the turn
// straight back to the player who played it (and so does a skip, and a draw
// card). "Reverse play direction" alone reads as a no-op in a 2-player game.
func TestRulebookReverseTwoPlayerText(t *testing.T) {
	g := sheddingBase()
	g.Players = 2
	g.SpecialCards = []genome.SpecialCard{
		{Type: genome.SpecialReverse, ByRank: 5},
		{Type: genome.SpecialSkip, ByRank: 6},
	}
	sec := specialSection(t, GenerateRulebook(g))
	if strings.Count(sec, "play again") < 2 {
		t.Errorf("2-player reverse and skip must both say the player plays again:\n%s", sec)
	}

	g.Players = 4
	sec = specialSection(t, GenerateRulebook(g))
	if strings.Contains(sec, "play again") {
		t.Errorf("4-player reverse/skip must not claim an extra turn:\n%s", sec)
	}
	if !strings.Contains(sec, "Reverse play direction") || !strings.Contains(sec, "Skip the next player") {
		t.Errorf("4-player reverse/skip text missing:\n%s", sec)
	}
}

// TestRulebookSpecialCardsNoDuplicateLines: mutation appends special rules
// without deduplicating, and the runner collapses identical rules into one
// effect -- the rulebook must not print the same rule twice.
func TestRulebookSpecialCardsNoDuplicateLines(t *testing.T) {
	g := sheddingBase()
	g.SpecialCards = []genome.SpecialCard{
		{Type: genome.SpecialSkip, ByRank: 6},
		{Type: genome.SpecialSkip, ByRank: 6},
	}
	if sec := specialSection(t, GenerateRulebook(g)); strings.Count(sec, "any 6") != 1 {
		t.Errorf("duplicate special rule printed more than once:\n%s", sec)
	}
}

// TestRulebookWildStatesWhatFollows: the runner has no suit nomination -- after
// a wild, the next player matches the wild card itself. The rulebook must say
// so or a Crazy Eights player assumes they may name a suit.
func TestRulebookWildStatesWhatFollows(t *testing.T) {
	g := sheddingBase()
	g.SpecialCards = []genome.SpecialCard{{Type: genome.SpecialWild, ByRank: 8}}
	sec := specialSection(t, GenerateRulebook(g))
	if !strings.Contains(sec, "no suit is named") {
		t.Errorf("wild text must state no suit is nominated:\n%s", sec)
	}
}

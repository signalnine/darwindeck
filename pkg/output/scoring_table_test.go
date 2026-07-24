package output

import (
	"strings"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
)

// TestUnqualifiedScoringRuleIsNamed pins the rendering of MatchCardPoints'
// lowest-specificity tier. A rank=0 + suit=0 rule scores every card the more
// specific rules do not claim -- a real mechanic (Uno's "every card left in your
// hand costs you") -- but scoringCardName fell through to the generic branch and
// printed "Any of any suit", which names nothing.
func TestUnqualifiedScoringRuleIsNamed(t *testing.T) {
	g := seeds.Hearts()
	g.Scoring.CardPoints = []genome.CardScoring{{Rank: 0, Suit: 0, Points: 3}}
	if errs := genome.Validate(g); len(errs) > 0 {
		t.Fatalf("genome invalid: %v", errs)
	}

	rb := GenerateRulebook(g)
	if strings.Contains(rb, "Any of any suit") {
		t.Errorf("rulebook still renders the unqualified rule as \"Any of any suit\":\n%s", scoringSection(rb))
	}
	if !strings.Contains(rb, "| Every card | 3 |") {
		t.Errorf("rulebook does not name the unqualified rule; scoring section:\n%s", scoringSection(rb))
	}
}

// TestOverlappingScoringRulesStatePrecedence pins that the table states the rule
// the engine applies. genome.MatchCardPoints resolves overlap by SPECIFICITY,
// not slice order, and never adds values together; a penalty suit plus a named
// high card (the standard Hearts shape) left a reader to guess.
func TestOverlappingScoringRulesStatePrecedence(t *testing.T) {
	g := seeds.Hearts()
	g.Scoring.CardPoints = []genome.CardScoring{
		{Suit: 3, Points: 1},            // all Hearts
		{Suit: 4, Rank: 12, Points: 13}, // Queen of Spades
		{Rank: 0, Suit: 0, Points: 2},   // every other card
	}
	if errs := genome.Validate(g); len(errs) > 0 {
		t.Fatalf("genome invalid: %v", errs)
	}
	rb := GenerateRulebook(g)
	if !strings.Contains(rb, "more specific") {
		t.Errorf("overlapping scoring rules render without a precedence note:\n%s", scoringSection(rb))
	}
	if !strings.Contains(rb, "never added together") && !strings.Contains(rb, "never added") {
		t.Errorf("precedence note does not rule out stacking:\n%s", scoringSection(rb))
	}

	// The note must match the engine: the Queen of Spades scores 13, not 16.
	if got := genome.MatchCardPoints(g.Scoring.CardPoints, 12, 3); got != 13 {
		t.Errorf("Queen of Spades scores %d, want 13 (suit+rank is the most specific rule)", got)
	}
}

// TestNonOverlappingScoringRulesOmitPrecedenceNote keeps the note from becoming
// boilerplate: a table whose rules cannot both apply to any card does not need
// it, and the classic seeds must not gain unexplained text.
func TestNonOverlappingScoringRulesOmitPrecedenceNote(t *testing.T) {
	g := seeds.Hearts()
	g.Scoring.CardPoints = []genome.CardScoring{{Suit: 3, Points: 1}}
	if errs := genome.Validate(g); len(errs) > 0 {
		t.Fatalf("genome invalid: %v", errs)
	}
	if rb := GenerateRulebook(g); strings.Contains(rb, "more specific") {
		t.Errorf("single-rule scoring table gained a precedence note it does not need:\n%s", scoringSection(rb))
	}
}

func scoringSection(rb string) string {
	i := strings.Index(rb, "## Card Point Values")
	if i < 0 {
		return "(no scoring table rendered)"
	}
	return rb[i:]
}

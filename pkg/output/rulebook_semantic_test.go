package output

import (
	"strings"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
)

// Regression tests for the 2026-10 runner-vs-rulebook bughunt: every sentence
// the rulebook prints must describe something the runner does, and every
// outcome-relevant thing the runner does must be stated.

func sheddingBase() *genome.Genome {
	return &genome.Genome{
		Skeleton: genome.Shedding, Players: 3, HandSize: 7,
		Shedding: &genome.SheddingParams{MatchRule: genome.MatchEither, DrawPenalty: 1},
	}
}

func quickTakeLine(t *testing.T, g *genome.Genome) string {
	t.Helper()
	for _, ln := range strings.Split(GenerateReport(g, fitness.Metrics{}), "\n") {
		if strings.Contains(ln, "Quick Take") {
			return ln
		}
	}
	t.Fatal("report has no Quick Take line")
	return ""
}

// TestQuickTakeTrumpOnlyOnTrickTaking: only the trick-taking runner reads
// TrumpRule. The report advertised "with trump cards" for any skeleton
// carrying the bit (e.g. a hand-edited or legacy shedding genome).
func TestQuickTakeTrumpOnlyOnTrickTaking(t *testing.T) {
	shed := sheddingBase()
	shed.TrumpRule = genome.TrumpFixed
	shed.Scoring.TrumpSuit = 4
	if ln := quickTakeLine(t, shed); strings.Contains(ln, "trump") {
		t.Errorf("shedding Quick Take advertises trump the runner never reads: %q", ln)
	}
	if ln := quickTakeLine(t, seeds.Spades()); !strings.Contains(ln, "trump cards") {
		t.Errorf("trick-taking Quick Take must still mention trump: %q", ln)
	}
}

// TestRulebookOmitsInertLeadRestriction: no_trump_until_broken restricts
// nothing without a trump suit (the Hearts seed), so the rulebook must not
// print a trump lead restriction for a game with no trump.
func TestRulebookOmitsInertLeadRestriction(t *testing.T) {
	hearts := GenerateRulebook(seeds.Hearts())
	if strings.Contains(hearts, "Lead restriction") || strings.Contains(strings.ToLower(hearts), "trump has been played") {
		t.Errorf("Hearts (no trump) rulebook prints an inert trump lead restriction:\n%s", hearts)
	}
	spades := GenerateRulebook(seeds.Spades())
	if !strings.Contains(spades, "**Lead restriction:** Cannot lead trump until") {
		t.Errorf("Spades (fixed trump) rulebook must keep its live lead restriction:\n%s", spades)
	}
}

// TestRulebookNeverReferencesMissingAdditionalRules: an avoidance borrow with
// no card_points banks nothing. The rulebook used to render the multi-round
// structure ("see Additional Rules", "fewest penalty points") while
// LiveBorrows pruned the Additional Rules section it pointed at.
func TestRulebookNeverReferencesMissingAdditionalRules(t *testing.T) {
	g := sheddingBase()
	g.Shedding.RoundsPerGame = 3
	g.Borrowed = []genome.BorrowedMechanic{{Source: genome.TrickTaking, Mechanic: genome.MechAvoidance}}
	rb := GenerateRulebook(g)
	if strings.Contains(rb, "see Additional Rules") && !strings.Contains(rb, "## Additional Rules") {
		t.Errorf("rulebook references an Additional Rules section it does not contain:\n%s", rb)
	}
	if strings.Contains(rb, "penalty points") {
		t.Errorf("rulebook promises penalty-point scoring no hook banks:\n%s", rb)
	}
	if !strings.Contains(rb, "The first player to play all their cards wins") {
		t.Errorf("a dead banking borrow leaves the game single-round; rulebook must say first-to-empty wins:\n%s", rb)
	}

	// Meld bonus (live) next to a dead avoidance borrow: the winning text must
	// not blame penalty cards that are never scored.
	g.Borrowed = append(g.Borrowed, genome.BorrowedMechanic{Source: genome.Rummy, Mechanic: genome.MechMeldBonus})
	rb = GenerateRulebook(g)
	if strings.Contains(rb, "penalty") {
		t.Errorf("dead avoidance borrow leaked penalty text next to a live meld bonus:\n%s", rb)
	}
	if !strings.Contains(rb, "## Additional Rules") {
		t.Errorf("live meld bonus must render its Additional Rules section:\n%s", rb)
	}
}

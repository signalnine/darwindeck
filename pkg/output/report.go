package output

import (
	"fmt"
	"strings"

	"github.com/darwindeck/darwindeck/pkg/evolution"
	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/genome"
)

// GenerateIndividualReport produces the published per-genome report,
// including the fitness provenance section (round 3 commit 5c): the
// published fitness of a decile-granted genome is the MCTS-mode running
// mean, which can exceed the weighted sum of the displayed (last-eval)
// component metrics by a large margin -- the r2 flagship published +0.177
// of silent uplift on skill-0.00 champions. Both means and the gap are
// therefore explicit.
func GenerateIndividualReport(g *genome.Genome, ind *evolution.Individual) string {
	report := GenerateReport(g, ind.Fitness)

	var b strings.Builder
	b.WriteString(report)
	b.WriteString("**Fitness provenance:**\n")
	if mctsMean, ok := ind.MCTSMean(); ok {
		greedy := ind.GreedyMean()
		b.WriteString(fmt.Sprintf("- Published fitness %.3f is the MCTS-mode mean (%d two-tier evals)\n",
			mctsMean, ind.MctsCount))
		b.WriteString(fmt.Sprintf("- Greedy-only mean: %.3f (%d evals -- the selection/decile ranking key)\n",
			greedy, ind.EvalCount))
		b.WriteString(fmt.Sprintf("- MCTS uplift: %+.3f (the second skill tier and fresh batches; large gaps on low-skill games are the knock-timing hazard -- see pkg/fitness)\n",
			mctsMean-greedy))
	} else {
		b.WriteString(fmt.Sprintf("- Published fitness %.3f is the greedy-only mean (%d evals); no MCTS tier was granted\n",
			ind.Fitness.TotalFitness, ind.EvalCount))
	}
	b.WriteString("- Component metrics above are last-evaluation values while the published fitness is a running mean over all evaluations: the weighted component sum will NOT reconcile exactly with the headline number\n\n")
	return b.String()
}

// GenerateReport produces a playtest analysis report.
func GenerateReport(g *genome.Genome, m fitness.Metrics) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("## %s — Fitness %.3f\n\n", gameName(g), m.TotalFitness))

	// Quick take
	b.WriteString(fmt.Sprintf("**Quick Take:** %s\n\n", quickTake(g)))

	// Stats
	b.WriteString("**Stats:**\n")
	b.WriteString(fmt.Sprintf("- Skeleton: %s\n", g.Skeleton))
	b.WriteString(fmt.Sprintf("- Players: %d, Hand size: %d\n", g.Players, g.HandSize))
	b.WriteString(fmt.Sprintf("- Decision density: %.2f (%.0f%% of turns have meaningful choices)\n",
		m.MeaningfulDecisions, m.MeaningfulDecisions*100))
	b.WriteString(fmt.Sprintf("- Game arc: %.2f\n", m.GameArc))
	b.WriteString(fmt.Sprintf("- Interaction: %.2f\n", m.Interaction))
	b.WriteString(fmt.Sprintf("- Skill gradient: %.2f\n", m.SkillGradient))
	b.WriteString(fmt.Sprintf("- Session length: %.2f\n", m.SessionLength))
	b.WriteString(fmt.Sprintf("- Generation: %d\n\n", g.Generation))

	// What makes it interesting
	b.WriteString("**What makes it interesting:**\n")
	insights := generateInsights(g, m)
	for _, insight := range insights {
		b.WriteString(fmt.Sprintf("- %s\n", insight))
	}
	b.WriteString("\n")

	return b.String()
}

func quickTake(g *genome.Genome) string {
	base := ""
	switch g.Skeleton {
	case genome.Shedding:
		base = "A shedding game"
	case genome.TrickTaking:
		base = "A trick-taking game"
	case genome.Rummy:
		base = "A rummy-style game"
	}

	modifiers := []string{}
	// Only LIVE borrows are advertised (round 3 commit 6b): a scoring borrow
	// on single-round shedding banks scores nothing reads.
	for _, bm := range liveBorrows(g) {
		modifiers = append(modifiers, borrowedQuickDesc(bm))
	}
	if g.Skeleton == genome.Shedding && len(g.SpecialCards) > 0 {
		modifiers = append(modifiers, fmt.Sprintf("%d special card effects", len(g.SpecialCards)))
	}
	if g.TrumpRule != genome.TrumpNone {
		modifiers = append(modifiers, "trump cards")
	}

	if len(modifiers) > 0 {
		return fmt.Sprintf("%s with %s", base, strings.Join(modifiers, ", "))
	}
	return base
}

func borrowedQuickDesc(bm genome.BorrowedMechanic) string {
	switch bm.Mechanic {
	case genome.MechMeldBonus:
		return "meld bonuses"
	case genome.MechTrickScoring:
		return "trick scoring"
	case genome.MechAvoidance:
		return "penalty cards"
	case genome.MechDrawPenalty:
		return "draw penalties"
	default:
		return "borrowed mechanics"
	}
}

func generateInsights(g *genome.Genome, m fitness.Metrics) []string {
	var insights []string

	if m.MeaningfulDecisions > 0.7 {
		insights = append(insights, "High decision density — most turns involve a real choice")
	} else if m.MeaningfulDecisions < 0.3 {
		insights = append(insights, "Low decision density — many forced plays")
	}

	if m.GameArc > 0.8 {
		insights = append(insights, "Strong game arc — outcomes are uncertain and varied")
	}

	if m.Interaction > 0.5 {
		insights = append(insights, "High player interaction — your plays frequently affect opponents")
	} else if m.Interaction < 0.1 {
		insights = append(insights, "Low interaction — plays are mostly independent")
	}

	if m.SkillGradient > 0.3 {
		insights = append(insights, "Good skill gradient — better play is rewarded")
	} else if m.SkillGradient < 0.05 {
		insights = append(insights, "Mostly luck-driven — skill has little impact")
	}

	if m.SessionLength > 0.8 {
		insights = append(insights, "Well-paced game length")
	} else if m.SessionLength < 0.2 {
		insights = append(insights, "Game length outside target range (too short or too long)")
	}

	for _, bm := range liveBorrows(g) {
		insights = append(insights, fmt.Sprintf("Borrows %s from %s skeleton",
			borrowedQuickDesc(bm), bm.Source))
	}

	if len(insights) == 0 {
		insights = append(insights, "A straightforward variant of its base skeleton")
	}

	return insights
}

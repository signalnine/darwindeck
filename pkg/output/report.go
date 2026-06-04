package output

import (
	"fmt"
	"strings"

	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/genome"
)

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
	if len(g.Borrowed) > 0 {
		for _, bm := range g.Borrowed {
			modifiers = append(modifiers, borrowedQuickDesc(bm))
		}
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

	for _, bm := range g.Borrowed {
		insights = append(insights, fmt.Sprintf("Borrows %s from %s skeleton",
			borrowedQuickDesc(bm), bm.Source))
	}

	if len(insights) == 0 {
		insights = append(insights, "A straightforward variant of its base skeleton")
	}

	return insights
}

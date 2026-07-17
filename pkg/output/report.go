package output

import (
	"fmt"
	"strings"

	"github.com/darwindeck/darwindeck/pkg/evolution"
	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/genome"
)

// minMctsEvalsForUplift is the sample floor below which the report refuses
// to state an MCTS uplift (Wave K fix 3): the flagship-r3 headline 0.918
// rested on ONE two-tier eval, which a reviewer reproduced as 0.73-0.82 over
// fresh seeds -- a textbook winner's curse. n >= 3 is not a significance
// claim, just the minimum for the word "mean" to carry any information
// beyond a single draw.
const minMctsEvalsForUplift = 3

// GenerateIndividualReport produces the published per-genome report. The
// headline fitness is the GREEDY-ONLY running mean -- OutputRank, the
// commensurable leaderboard key (Wave K fix 1) -- matching genome.json and
// the leaderboard order. The provenance section (round 3 commit 5c,
// retargeted here) reports the MCTS-mode mean separately with its sample
// count, and states the uplift only at MctsCount >= minMctsEvalsForUplift.
func GenerateIndividualReport(g *genome.Genome, ind *evolution.Individual) string {
	headline := ind.Fitness
	headline.TotalFitness = ind.OutputRank()
	report := GenerateReport(g, headline)

	var b strings.Builder
	b.WriteString(report)
	b.WriteString("**Fitness provenance:**\n")
	b.WriteString(fmt.Sprintf("- Headline fitness %.3f is the greedy-only running mean (%d evals) -- the leaderboard ranking key for every published game\n",
		ind.OutputRank(), ind.EvalCount))
	if mctsMean, ok := ind.MCTSMean(); ok {
		b.WriteString(fmt.Sprintf("- MCTS-mode mean: %.3f (n=%d two-tier evals; reported separately, never ranked)\n",
			mctsMean, ind.MctsCount))
		if ind.MctsCount >= minMctsEvalsForUplift {
			b.WriteString(fmt.Sprintf("- MCTS uplift: %+.3f (the second skill tier and fresh batches; large gaps on low-skill games are the knock-timing hazard -- see pkg/fitness)\n",
				mctsMean-ind.GreedyMean()))
		} else {
			b.WriteString(fmt.Sprintf("- MCTS uplift: insufficient samples (n=%d)\n", ind.MctsCount))
		}
	} else {
		b.WriteString("- No MCTS tier was granted to this genome\n")
	}
	b.WriteString("- Component metrics above are last-evaluation values while the headline fitness is a running mean over all evaluations: the weighted component sum will NOT reconcile exactly with the headline number\n\n")
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
	b.WriteString(fmt.Sprintf("- Generation: %d\n", g.Generation))
	// Veto-stability (Wave M): published only when the stability check ran
	// (StableEvals set). An unstable game is here as demoted evidence, not a
	// recommendation -- say so plainly.
	if g.StableEvals != "" {
		if g.VetoStable {
			b.WriteString(fmt.Sprintf("- Veto-stable: yes (%s fresh-seed re-evals valid)\n", g.StableEvals))
		} else {
			b.WriteString(fmt.Sprintf("- Veto-stable: NO -- only %s fresh-seed re-evals stayed valid; DEMOTED below stable games (this game fails its own degeneracy veto / Tier-1 on a majority of fresh seeds and should not be treated as a publishable result)\n", g.StableEvals))
		}
	}
	b.WriteString("\n")

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
	// Every skeleton needs a case: a missing one published a report whose
	// Quick Take line was literally empty ("**Quick Take:** ") for casino and
	// vying genomes.
	base := "A card game"
	switch g.Skeleton {
	case genome.Shedding:
		base = "A shedding game"
	case genome.TrickTaking:
		base = "A trick-taking game"
	case genome.Rummy:
		base = "A rummy-style game"
	case genome.Climbing:
		base = "A climbing/ladder game"
	case genome.Casino:
		base = "A fishing/capture game"
	case genome.Vying:
		base = "A betting/showdown game"
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
	// The deep borrows are the headline hybrids -- the generic "borrowed
	// mechanics" fallback erased exactly the mechanics the discovery work is
	// about from Quick Takes and insights.
	case genome.MechRunPlay:
		return "multi-card combination plays"
	case genome.MechFollowSuit:
		return "a follow-suit obligation"
	case genome.MechKnock:
		return "knocking to end the game early"
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

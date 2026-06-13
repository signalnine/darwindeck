package judge

import (
	"fmt"
	"strings"

	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/output"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

// dossierSeed is the reproducible PRNG / shuffle seed used for every dossier's
// trace and stats batches, so re-running emit on the same input is stable.
const dossierSeed uint64 = 0x5EED1234CAFEBABE

// BuildDossier produces the BLIND dossier markdown for one game. g.ID must
// ALREADY be set to the neutral code (e.g. "G01") by the caller, so the
// rulebook title is neutral and no original ID leaks. The dossier contains
// ONLY:
//   - the neutral rulebook,
//   - two sample greedy-vs-greedy traces,
//   - a Termination section (the new reachable-win evidence).
//
// It contains NO fitness/metric scores, NO veto names, NO skill numbers, NO
// rank, NO source category, and NO true game name.
func BuildDossier(g *genome.Genome) (string, error) {
	runner := fitness.GetRunner(g)
	if runner == nil {
		return "", fmt.Errorf("no runner for skeleton %s", g.Skeleton)
	}
	ai := fitness.GetGreedyAI(g)

	var b strings.Builder

	// 1. Neutral rulebook (title is g.ID, already set to the neutral code).
	b.WriteString(neutralizeRulebook(output.GenerateRulebook(g)))
	b.WriteString("\n---\n\n")

	// 2. Two sample greedy-vs-greedy traces. A larger batch (distinct seed) is
	// run so two COMPLETED games can be shown even for games that rarely finish
	// under greedy self-play (e.g. gin-style rummy times out most games).
	const traceN = 400
	traceRes := sim.RunBatch(g, runner, ai, traceN, dossierSeed+1)

	b.WriteString("## Sample Game Traces\n\n")
	b.WriteString("Two complete games played by identical automated players (greedy strategy on both sides). Each line is one game event in order, so you can see who acts and when, and spot long uninterrupted single-player runs.\n\n")

	picked := pickDistinctCompleted(traceRes, 2)
	switch len(picked) {
	case 0:
		b.WriteString("_No games completed in the sampled batch; the automated players run out of fast progress and hit the turn cap. This is a SPEED observation, not a design verdict -- see the Termination section below for whether the win condition is reachable by the rules._\n\n")
	case 1:
		b.WriteString("_Only one of the sampled games reached a winner; the rest hit the turn cap without resolving. See the Termination section below for whether the win condition is reachable by the rules._\n\n")
	}
	for n, idx := range picked {
		b.WriteString(fmt.Sprintf("### Game %d\n\n", n+1))
		b.WriteString(renderTrace(traceRes.AllEvents[idx]))
		b.WriteString(fmt.Sprintf("\n**Winner:** Player %d\n\n", traceRes.AllWinners[idx]))
	}

	// 3. Termination section (THE FIX). A larger sample makes the boolean
	// reachable-win signal robust to the greedy AI's slow, seed-sensitive
	// descent toward the win condition -- a sound-but-slow rummy game (Gin)
	// reaches a legal knock in only a few percent of greedy games, so the
	// sample must be large enough to observe it reliably.
	const termN = 150
	term := computeTermination(g, runner, ai, termN, dossierSeed+2)
	b.WriteString(renderTermination(term))

	return b.String(), nil
}

// renderTermination writes the Termination section: completion at the standard
// cap, completion at a 4x extended cap, and the skeleton-specific
// WIN-CONDITION-REACHABLE signal. This section is the corrective evidence that
// lets a judge distinguish "slow AI play" from "degenerate design".
func renderTermination(t TerminationInfo) string {
	var b strings.Builder
	b.WriteString("---\n\n## Termination\n\n")
	b.WriteString("This section reports whether the game can END -- separating how FAST the automated players reach a win from whether a win is reachable BY THE RULES. A low completion rate alone does NOT mean the game is broken: it may just be slow.\n\n")

	b.WriteString(fmt.Sprintf("- Completion at the standard turn cap (%d turns): **%.0f%%** of %d sampled games ended with a winner.\n",
		t.CapStd, t.CompletionStdPct, t.GamesSampled))
	b.WriteString(fmt.Sprintf("- Completion at a 4x extended turn cap (%d turns): **%.0f%%**. ",
		t.CapExt, t.CompletionExtPct))
	if t.CompletionExtPct > t.CompletionStdPct+5 {
		b.WriteString("Completion CLIMBS when the players are given more turns, which indicates the game is SLOW to resolve, not unable to resolve.\n")
	} else {
		b.WriteString("Completion is roughly flat at the extended cap.\n")
	}

	b.WriteString("\n**Win-condition reachable (by design):**\n\n")
	switch t.Skeleton {
	case genome.Rummy:
		if t.ReachableKnock {
			b.WriteString(fmt.Sprintf("- A going-out move became LEGAL in sampled games (a player's leftover-card total dropped to the threshold), first becoming legal at a median of **turn %d**. The terminal state IS reachable by the rules.\n", t.MedianTurnsToKnockLegal))
		} else {
			b.WriteString("- No sampled game reached a state where a going-out move was legal within the cap. The terminal state may be hard or impossible to reach -- weigh this against the rules.\n")
		}
		if t.AnyKnockOrGin {
			b.WriteString("- At least one sampled game actually ended by going out (a player declared out or reached zero leftover cards).\n")
		} else {
			b.WriteString("- No sampled game actually ended by going out within the cap (the automated players were slow to do so).\n")
		}
	case genome.Shedding:
		if t.MedianTurnsToEmptyHand > 0 {
			b.WriteString(fmt.Sprintf("- A player emptied their hand (the win condition) in completed sampled games, at a median of **turn %d**. The terminal state IS reachable by the rules.\n", t.MedianTurnsToEmptyHand))
		} else {
			b.WriteString("- No sampled game saw a player empty their hand within the cap. The terminal state may be hard or impossible to reach -- weigh this against the rules.\n")
		}
	case genome.TrickTaking:
		if t.RoundsComplete {
			b.WriteString("- Rounds reached completion in sampled games (all tricks were played out and scored). The terminal state IS reachable by the rules.\n")
		} else {
			b.WriteString("- No sampled round reached completion within the cap. The terminal state may be hard or impossible to reach -- weigh this against the rules.\n")
		}
	default:
		b.WriteString("- (No skeleton-specific reachable-win signal available.)\n")
	}

	yn := func(v bool) string {
		if v {
			return "yes"
		}
		return "no"
	}
	b.WriteString(fmt.Sprintf("\n- Has bidding/contract scoring: **%s**\n", yn(t.HasContractScoring)))
	b.WriteString("\n")
	return b.String()
}

// pickDistinctCompleted returns up to want indices of completed games from res,
// skipping any whose event trace is identical to one already picked (so two
// runs of the same deterministic game are not shown twice).
func pickDistinctCompleted(res sim.BatchResult, want int) []int {
	var out []int
	seen := map[string]bool{}
	for i := 0; i < len(res.AllWinners) && len(out) < want; i++ {
		if res.AllWinners[i] < 0 {
			continue
		}
		sig := traceSignature(res.AllEvents[i])
		if seen[sig] {
			continue
		}
		seen[sig] = true
		out = append(out, i)
	}
	return out
}

// traceSignature is a cheap content hash of a game's event list, used only to
// dedup identical traces.
func traceSignature(events []sim.Event) string {
	var b strings.Builder
	for _, e := range events {
		fmt.Fprintf(&b, "%d/%d/%s/%s;", e.Type, e.PlayerID, renderCards(e.Cards), e.Detail)
	}
	return b.String()
}

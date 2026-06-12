// Degeneracy veto (audit remediation Task 28 step 4, round 2 -- the Task 14
// exit condition (a), instantiated): three dynamic detectors computed from
// the Tier 2 random batch that reject game-shaped non-games the five weighted
// metrics genuinely like. The metric weights are frozen at
// 0.25/0.25/0.20/0.20/0.10, so the added measurement is a VALIDITY veto, not
// a sixth weighted term: a game that fails any detector is invalid (fitness
// 0 in the pipeline), exactly like a Tier 1 kill.
//
// Why a veto is unavoidable (the round-2 record): after the interaction and
// choice-impact fixes, the three rejected flagship champions still measured
// 0.625-0.759 weighted vs 0.428-0.578 for the human-validated classics --
// the catch-all-skip champion in particular Pareto-dominates several
// classics on the five metrics (high arc, real greedy gradient, frequent
// draw-two attacks, in-band length). No monotone scale change separates a
// dominating pair; the missing dimensions are below.
//
// Each detector encodes a DESIGNER's stated rejection reason, with at least
// a 2x measured margin between every classic and the champion it targets
// (means over CalibrationSeeds, 50-game probes; see the calibration suite's
// ROUND 2 block for the full table):
//
//	non_agentic       -- meaningful-decision density < 0.05. The v1 key
//	                     constraint ("games must contain non-random decision
//	                     points"), measurable since the choice-impact fix.
//	                     Targets NoFollowAvoidanceTrick (density 0.000;
//	                     classic minimum 0.171).
//	tempo_monopoly    -- mean consecutive same-player move-run length > 6.
//	                     The spectator problem: "13 consecutive plays,
//	                     opponent acted 0 times" was the literal review
//	                     verdict on the catch-all-skip champion (measured
//	                     mean run 15.4; classic maximum 3.04 -- rummy's
//	                     structural draw-meld-discard cycle).
//	draw_supply_churn -- rummy only: share of moves carrying a nonzero
//	                     OptionDelta > 0.10. Rummy deltas attach only to
//	                     discards probing the next player's DRAW options
//	                     (Task 7 table), so this share measures how often a
//	                     hand-off toggles the opponent's ability to draw at
//	                     all: a healthy two-supply economy is stable (gin
//	                     0.010, knock 0.010), while the rejected pair-meld
//	                     champion's ~1-card stock turns nearly every
//	                     hand-off into supply whiplash (0.292).
package fitness

import (
	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

const (
	// degAgencyFloor: minimum meaningful-decision density. Classic minimum
	// 0.171 (oh-hell) = 3.4x margin.
	degAgencyFloor = 0.05
	// degTempoMonopolyMeanRun: maximum mean consecutive same-player run.
	// Classic maximum 3.04 (gin/knock structural turn cycles) = 2x margin
	// below; the catch-all-skip champion's 15.4 = 2.6x margin above.
	degTempoMonopolyMeanRun = 6.0
	// degRummyChurnMax: maximum share of moves with nonzero OptionDelta,
	// rummy skeleton only. Classics 0.010 = 10x margin below; the pair-meld
	// champion's 0.292 = 2.9x margin above.
	degRummyChurnMax = 0.10
)

// CheckDegeneracy inspects a batch's turn records for the degeneracy
// signatures above. Returns "" for a healthy game or the failed detector's
// reason string. The empty batch is vacuously healthy: batch-size sanity is
// Tier 1's job.
func CheckDegeneracy(result sim.BatchResult, g *genome.Genome) string {
	totalTurns := 0
	for _, turns := range result.AllTurns {
		totalTurns += len(turns)
	}
	if totalTurns == 0 {
		return ""
	}

	if computeDecisionDensity(result) < degAgencyFloor {
		return "non_agentic"
	}
	if meanConsecutiveRun(result) > degTempoMonopolyMeanRun {
		return "tempo_monopoly"
	}
	if g.Skeleton == genome.Rummy && optionDeltaShare(result) > degRummyChurnMax {
		return "draw_supply_churn"
	}
	return ""
}

// meanConsecutiveRun returns the mean length of maximal consecutive
// same-player record runs across all games in the batch.
func meanConsecutiveRun(result sim.BatchResult) float64 {
	runs, sum := 0, 0
	for _, turns := range result.AllTurns {
		prev := -1
		cur := 0
		for _, tr := range turns {
			if tr.Player == prev {
				cur++
				continue
			}
			if cur > 0 {
				runs++
				sum += cur
			}
			prev = tr.Player
			cur = 1
		}
		if cur > 0 {
			runs++
			sum += cur
		}
	}
	if runs == 0 {
		return 0
	}
	return float64(sum) / float64(runs)
}

// optionDeltaShare returns the fraction of all recorded moves carrying a
// nonzero OptionDelta.
func optionDeltaShare(result sim.BatchResult) float64 {
	total, nonzero := 0, 0
	for _, turns := range result.AllTurns {
		for _, tr := range turns {
			total++
			if tr.OptionDelta != 0 {
				nonzero++
			}
		}
	}
	if total == 0 {
		return 0
	}
	return float64(nonzero) / float64(total)
}

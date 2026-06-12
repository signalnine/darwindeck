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
	// ROUND 3: also enforced on the greedy batch (CheckGreedyDegeneracy);
	// measured greedy-batch classic maximum 3.064 (gin's structural
	// draw-meld-discard cycle; shedding/trick-taking classics 1.00-1.06),
	// preserving the ~2x margin on both batches.
	degTempoMonopolyMeanRun = 6.0
	// degRummyChurnMax: maximum share of moves with nonzero OptionDelta,
	// rummy skeleton only. Classics 0.010 = 10x margin below; the pair-meld
	// champion's 0.292 = 2.9x margin above.
	degRummyChurnMax = 0.10
	// degSeatShareFraction (round 3): minimum mean min-seat share of turns,
	// as a fraction of the fair share 1/numPlayers -- vetoed strictly below
	// 0.5/numPlayers on EITHER batch. Encodes the r2 rank03 designer
	// rejection (adjacent-pair reverse ping-pong locked 2 of 4 seats out;
	// same-player runs stayed ~1, invisible to tempo_monopoly). Measured
	// classic minima over CalibrationSeeds (calibrate veto table): the
	// least-fair classic on both batches is mau-mau at 0.89x fair share
	// (random 0.295, greedy 0.298 of the 3p fair 0.333); every other classic
	// sits at 0.94-1.00x. That is a 1.78x margin to the 0.5x threshold; the
	// r2 rank03 lockout measures ~0.07x (locked seats act a handful of times
	// per game), ~7x below it.
	degSeatShareFraction = 0.5
	// degGreedyTimeoutShare (round 3): maximum share of greedy-batch games
	// hitting the turn cap. The r2 rank01 class cycles to the 390-turn cap
	// under GREEDY play while completing under random -- non-termination
	// invisible to the random-batch Tier 1. Strict-above 0.10: measured
	// classic maximum over CalibrationSeeds is crazy-eights 0.014 (mau-mau
	// 0.005, all six others 0.000) = 7x margin, and Tier 1 already tolerates
	// <= 2/10 random timeouts as noise, so 10% is the established noise
	// ceiling. Measured side effect, accepted as a strengthening: the two
	// ORIGINAL degenerate fixtures lose their last Tier-2-surviving seeds to
	// this detector (instant-knock seed 44 at 0.110, forced-shedding's four
	// surviving seeds at mean 0.141) -- greedy play fails to terminate >10%
	// of their games, which is exactly the non-termination class this
	// detector encodes. Both now read pipeline-effective 0 on every
	// calibration seed.
	degGreedyTimeoutShare = 0.10
)

// CheckDegeneracy inspects the RANDOM batch's turn records for the degeneracy
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
	if meanMinSeatShare(result, g.Players) < degSeatShareFraction/float64(g.Players) {
		return "seat_participation"
	}
	if g.Skeleton == genome.Rummy && optionDeltaShare(result) > degRummyChurnMax {
		return "draw_supply_churn"
	}
	return ""
}

// CheckGreedyDegeneracy inspects the GREEDY batch (seat 0 = greedy AI) for
// degeneracy only skilled play exposes -- the round-2 blind spot, exploited
// by the r2 flagship: every detector ran on random play only, so a genome
// healthy under random but degenerate under skilled play (greedy-discovered
// tempo monopolies, greedy-play non-termination) escaped every veto and
// reached designer review. Reasons carry the greedy_ prefix so kill listings
// name the batch that fired.
//
//	greedy_timeout            -- timeout share > 0.10: skilled play cycles
//	                             the game to the turn cap (the r2 rank01
//	                             390-turn class; random Tier 1 cannot see it)
//	greedy_tempo_monopoly     -- same threshold as the random batch
//	greedy_seat_participation -- same threshold as the random batch
//
// The agency floor and churn deliberately do NOT run here: meaningfulness
// under a deterministic AI is not the density metric's semantics (one fixed
// policy collapses choice diversity by construction), and draw-supply churn
// is a supply-economy statistic defined on random play.
func CheckGreedyDegeneracy(result sim.BatchResult, g *genome.Genome) string {
	if result.GamesPlayed > 0 &&
		float64(result.Timeouts)/float64(result.GamesPlayed) > degGreedyTimeoutShare {
		return "greedy_timeout"
	}

	totalTurns := 0
	for _, turns := range result.AllTurns {
		totalTurns += len(turns)
	}
	if totalTurns == 0 {
		return ""
	}

	if meanConsecutiveRun(result) > degTempoMonopolyMeanRun {
		return "greedy_tempo_monopoly"
	}
	if meanMinSeatShare(result, g.Players) < degSeatShareFraction/float64(g.Players) {
		return "greedy_seat_participation"
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

// meanMinSeatShare returns the mean over games of the least-active seat's
// share of that game's turn records. A fair game gives every seat ~1/n;
// a lockout (r2 rank03: reverse ping-pong between an adjacent pair) starves
// some seat toward 0. Games without records are skipped; an empty batch
// returns 1 (vacuously fair -- batch-size sanity is Tier 1's job).
func meanMinSeatShare(result sim.BatchResult, numPlayers int) float64 {
	if numPlayers <= 0 {
		return 1
	}
	games := 0
	var sum float64
	counts := make([]int, numPlayers)
	for _, turns := range result.AllTurns {
		if len(turns) == 0 {
			continue
		}
		for i := range counts {
			counts[i] = 0
		}
		for _, tr := range turns {
			if tr.Player >= 0 && tr.Player < numPlayers {
				counts[tr.Player]++
			}
		}
		minCount := counts[0]
		for _, c := range counts[1:] {
			if c < minCount {
				minCount = c
			}
		}
		sum += float64(minCount) / float64(len(turns))
		games++
	}
	if games == 0 {
		return 1
	}
	return sum / float64(games)
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

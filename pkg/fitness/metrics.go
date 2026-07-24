package fitness

import (
	"math"

	"github.com/darwindeck/darwindeck/pkg/sim"
)

// Metrics holds the 5 fitness metrics, each 0.0-1.0.
type Metrics struct {
	MeaningfulDecisions float64 // Weight 0.25
	GameArc             float64 // Weight 0.25
	Interaction         float64 // Weight 0.20
	SkillGradient       float64 // Weight 0.20
	SessionLength       float64 // Weight 0.10
	TotalFitness        float64
	SharedFitness       float64 // After niche sharing adjustment
}

const (
	WeightDecisions = 0.25
	WeightArc       = 0.25
	WeightInteract  = 0.20
	WeightSkill     = 0.20
	WeightLength    = 0.10
)

// ComputeFitness calculates all 5 metrics from simulation results, with no
// MCTS batch: the skill gradient is greedy-only (capped at the scaled 0.4
// term, see computeSkillGradient). This is the default-mode entry point --
// kept at the 3-argument signature for callers that never run MCTS
// (pkg/evolution/behavior.go, the default Evaluate path).
func ComputeFitness(
	randomResult sim.BatchResult,
	greedyResult sim.BatchResult,
	numPlayers int,
) Metrics {
	return ComputeFitnessWithMCTS(randomResult, greedyResult, sim.BatchResult{}, numPlayers)
}

// ComputeFitnessWithMCTS calculates all 5 metrics including the two-tier
// skill gradient (audit Task 20). mctsResult may be the zero value when no
// MCTS batch was run; the MCTS skill term is then 0.
func ComputeFitnessWithMCTS(
	randomResult sim.BatchResult,
	greedyResult sim.BatchResult,
	mctsResult sim.BatchResult,
	numPlayers int,
) Metrics {
	m := Metrics{
		MeaningfulDecisions: computeDecisionDensity(randomResult),
		GameArc:             computeGameArc(randomResult),
		Interaction:         computeInteraction(randomResult),
		SkillGradient:       computeSkillGradient(randomResult, greedyResult, mctsResult, numPlayers),
		SessionLength:       computeSessionLength(randomResult, numPlayers),
	}

	m.TotalFitness = m.MeaningfulDecisions*WeightDecisions +
		m.GameArc*WeightArc +
		m.Interaction*WeightInteract +
		m.SkillGradient*WeightSkill +
		m.SessionLength*WeightLength

	return m
}

// computeDecisionDensity: fraction of decision points whose choice plausibly
// MATTERED (TurnRecord.Meaningful, Task 28 round 2): the acting player had
// >= 2 legal moves AND the batch runner's choice-impact sampling found moves
// differing in type, special-effect profile, or next-player option impact
// (see turnIsMeaningful in pkg/sim/batch.go for per-skeleton semantics).
//
// Forced turns (1 legal move) are never decisions regardless of event type.
// Raw LegalMoves >= 2 counting -- the previous definition -- was the
// archetype A1/A2 inflation vector: all-wild shedding hands and no-follow
// trick hands scored 0.86-0.92 density while their choices had near-zero
// impact (the rejected flagship champions, now pinned as fixtures in
// pkg/seeds/degenerate.go).
func computeDecisionDensity(result sim.BatchResult) float64 {
	total, meaningful := 0, 0
	for _, turns := range result.AllTurns {
		for _, tr := range turns {
			total++
			if tr.Meaningful {
				meaningful++
			}
		}
	}
	if total == 0 {
		return 0
	}
	return float64(meaningful) / float64(total)
}

// computeGameArc: a good arc = early uncertainty (the eventual winner was not
// already leading at midgame) + late resolution (the leader near the end
// wins). Computed from the per-game leader tracks (audit Tasks 8 + 10):
//
//	comeback    = P(winner != leader at the 50% sample), target ~0.5, score peaks there
//	resolution  = P(winner == leader at the 90% sample, clamped to the
//	              second-to-last sample so short tracks never sample the
//	              final, winner-correlated entry -- audit Wave D fix 2)
//	leadChanges = mean lead changes per game (-1 tie samples ignored), saturating at 3
//
//	arc = 0.4*tent(comeback, 0.5) + 0.4*resolution + 0.2*min(leadChanges/3, 1)
//
// A pure coin flip decided on the last move scores low on resolution; a
// foregone conclusion (wire-to-wire leader, or a midgame leader who ALWAYS
// loses) scores 0 on comeback's tent. The old seat-entropy + turn-CV metric
// gave both ~1.0. The previous turn-CV term is gone entirely: duration spread
// is SessionLength's concern (plan Task 10 decision).
//
// The per-game winner is the REAL batch winner (BatchResult.AllWinners, from
// GameResult.Winner), never the final leader sample: the batch runner appends
// leader tracks for ALL games including max_turns/stuck/no_moves exits, whose
// tracks end with a leader but no winner; and a rummy scoring borrow can hand
// the CheckEnd win (state.Scores incl. hook contributions) to a player who
// never led on live deadwood (audit Wave D fix 1). Games with fewer than 5
// leader samples (the minimum where the resolution index lands strictly
// after the midgame sample) or without a winner (Winner < 0: non-completion)
// are skipped; a batch with no qualifying game scores 0.
func computeGameArc(result sim.BatchResult) float64 {
	comeback, resolution, leadChanges, counted := arcStats(result)
	if counted == 0 {
		return 0
	}
	return 0.4*tent(comeback, 0.5) + 0.4*resolution + 0.2*math.Min(leadChanges/3, 1)
}

// arcStats aggregates the three arc components over qualifying games (>= 4
// leader samples and a real winner: AllWinners[i] >= 0, i.e. the game
// completed). comeback and resolution are batch-level probabilities;
// leadChanges is the mean per qualifying game.
func arcStats(result sim.BatchResult) (comeback, resolution, leadChanges float64, counted int) {
	comebacks, resolutions, changes := 0, 0, 0
	for i, track := range result.AllLeaders {
		n := len(track)
		// Minimum 5 samples, not 4: the resolution index clamps to n-2, and
		// only for n >= 5 is n-2 strictly AFTER the midgame sample (n/2). At
		// n == 4 both read track[2], so "resolution" and "comeback" measured
		// the same sample and were mechanically anticorrelated -- a 4-sample
		// game has no probe-able late-game at all.
		if n < 5 {
			continue
		}
		// Defensive: a hand-built BatchResult without winners carries no
		// completion information, so none of its games qualify.
		if i >= len(result.AllWinners) || result.AllWinners[i] < 0 {
			continue
		}
		winner := int8(result.AllWinners[i])
		counted++
		// A -1 (tie) at a sample point means the winner was NOT strictly
		// leading there: it counts toward comeback and against resolution.
		if track[n/2] != winner {
			comebacks++
		}
		// Resolution sample: nominally the 90% mark, clamped to n-2 so it is
		// never the final sample itself. For n <= 10, n*9/10 == n-1, and the
		// final leader correlates with the winner, so ultra-short games (the
		// instant-knock degenerate class the calibration suite targets) got
		// resolution ~1 for free (audit Wave D fix 2). The >= 5 sample
		// minimum above keeps the index strictly after the midgame sample.
		resIdx := n * 9 / 10
		if resIdx > n-2 {
			resIdx = n - 2
		}
		if track[resIdx] == winner {
			resolutions++
		}
		changes += countLeadChanges(track)
	}
	if counted == 0 {
		return 0, 0, 0, 0
	}
	games := float64(counted)
	return float64(comebacks) / games, float64(resolutions) / games, float64(changes) / games, counted
}

// countLeadChanges counts transitions between distinct non-tie leaders.
// Tie samples (-1) are skipped entirely: 0,-1,0 is no change; 0,-1,1 is one.
func countLeadChanges(track []int8) int {
	changes := 0
	prev := int8(-1)
	seen := false
	for _, leader := range track {
		if leader < 0 {
			continue
		}
		if seen && leader != prev {
			changes++
		}
		prev = leader
		seen = true
	}
	return changes
}

// tent scores x against a target c > 0: 1 at x == c, falling linearly to 0 at
// 0 and 2c.
func tent(x, c float64) float64 {
	return clamp(1-math.Abs(x-c)/c, 0, 1)
}

// computeInteraction: exact fraction of INTERACTIVE TURNS -- turns whose move
// perturbed the next player's legal options (TurnRecord.OptionDelta != 0,
// audit Task 7) or carried a direct attack (TurnRecord.Attack, set at record
// time from the move's own events; the whitelist lives in sim.IsAttackEvent,
// the single source of truth -- audit Wave D fix 3). A discard that does not
// change what the opponent can legally do is NOT interaction -- that was the
// old event-taxonomy metric's central flaw: it counted every shedding discard
// and every rummy meld as interactive and pinned hearts-4p at a deterministic
// 0.657 (13 TrickWon / 66 events / 0.3).
//
// The previous max(deltaTurns, attackEvents) approximation undercounted
// disjoint mixed games (2 delta turns + 2 distinct attack turns scored as 2,
// not 4) and could multi-count stacked specials (one card matching
// skip+reverse+draw rules emits up to 3 attack events from a single move).
// The per-turn union is exact on both: each TurnRecord is one turn, counted
// once, interactive iff it perturbed options OR attacked.
//
// Per-skeleton signal sources (Task 7 table): shedding = OptionDelta from the
// discard-top perturbation plus skip/draw/reverse attack turns; rummy =
// OptionDelta attached to the turn-passing MoveDiscard; trick-taking =
// trick-completing attack turns (one per EventTrickWon) plus lead-constraint
// OptionDelta on trick-leading plays (audit Wave D fix 4: nonzero when
// MustFollowSuit binds the follower, so trick-taking interaction is no longer
// the closed-form constant 2/N).
//
// Scale: clamp(ratio/0.5) is provisional; Task 14 recalibrates the
// denominator from the seed-game spread, not assumption.
func computeInteraction(result sim.BatchResult) float64 {
	totalTurns, interactive := 0, 0
	for _, turns := range result.AllTurns {
		totalTurns += len(turns)
		for _, tr := range turns {
			if tr.OptionDelta != 0 || tr.Attack {
				interactive++
			}
		}
	}
	if totalTurns == 0 {
		return 0
	}
	ratio := float64(interactive) / float64(totalTurns)
	return clamp(ratio/0.5, 0, 1)
}

// Two-tier skill constants (audit Task 20, the v2 design's formula):
//
//	raw = 0.4*max(0, greedyWR - randomBaselineWR)/(1-randomBaselineWR)
//	    + 0.6*max(0, mctsWR  - greedyWR)         /(1-greedyWR)
//
// The 0.4/0.6 split is plan-fixed: skill a 1-ply greedy can detect saturates
// at 0.4 of the raw scale; the top 0.6 is reachable only by ISMCTS outplaying
// greedy. skillScale is the metric's raw-to-[0,1] divisor -- the ONE constant
// Task 20 may adjust (weights stay at 0.25/0.25/0.20/0.20/0.10).
//
// skillScale = 0.5, justified by the measured classic spread over
// CalibrationSeeds (post-Task-20 block in calibration_test.go): the
// strongest classic greedy term is gin rummy's 0.905, the cross-seed MEAN
// of the normalized term over CalibrationSeeds (sd 0.011, Task 13.5 table).
// The single-seed rates measured at seed 44 -- greedyWR 0.945 over a 0.484
// empirical baseline -- give (0.945-0.484)/(1-0.484) = 0.893 for THAT seed,
// about 1 sd below the mean; an earlier revision of this comment glued the
// seed-44 rates onto the cross-seed mean as if 0.905 followed from them.
// Either way raw ~= 0.4*0.90 ~= 0.36 with the MCTS term 0
// (greedy outplays 20-determinization ISMCTS at gin: mctsWR 0.900). At raw
// scale (1.0) every real game would compress below ~0.45 and gin would lose
// its calibration margin over the instant-knock fixture (measured: gin
// total 0.475 vs the required 0.527); /0.5 restores the gate's Task 14
// margins while preserving the cap structure: greedy-only-detectable skill
// tops out at 0.4/0.5 = 0.8, and the top 20% of the metric is reachable
// only through the MCTS tier. A scale of 0.4 or lower would let greedy-only
// games saturate the metric at 1.0, neutering the second tier.
const (
	skillGreedyWeight = 0.4
	skillMCTSWeight   = 0.6
	skillScale        = 0.5
)

// computeSkillGradient measures whether better play leads to better results,
// on two tiers: greedy AI over the empirical random baseline, and ISMCTS
// over the empirical greedy baseline (audit Task 20). All win rates are
// seat-0 rates; every baseline is an empirical seat-0 baseline from an
// INDEPENDENT random batch (dd-qt7) -- the batches are NOT same-seed paired
// (the pipeline seeds them at distinct offsets: random +100, greedy +1000,
// MCTS +2000). A structural first-player advantage appears in all three
// rates IN EXPECTATION and cancels out of both difference terms, so a
// zero-skill game scores ~0 regardless of FPA; but unpaired differencing
// roughly doubles the variance of each difference vs a paired design.
// Future win: paired (same-seed) batch seeding plus a calibration
// re-baseline.
//
// mctsResult is the zero value in default (greedy-only) mode -- Task 19's
// 2s/genome MCTS budget FAILED (~14.5s measured, see pkg/sim/mcts.go), so
// the 20-game MCTS batch runs only for selected genomes (EvaluateWithMCTS;
// MCTS-for-top-decile is the plan's production fallback). Without MCTS data
// the second term is 0 and skill is capped at the scaled 0.4 greedy term.
func computeSkillGradient(randomResult, greedyResult, mctsResult sim.BatchResult, numPlayers int) float64 {
	if greedyResult.Completions == 0 || numPlayers == 0 {
		return 0
	}

	// Baseline = the *empirical* seat-0 win rate under all-random play. Greedy
	// always occupies seat 0, so any structural first-player advantage already
	// shows up in randomResult.WinCounts[0]; measuring against the theoretical
	// 1/numPlayers would miscredit that seat edge as skill (dd-qt7). Fall back to
	// the theoretical rate only when the random batch produced no completions.
	baselineWR := 1.0 / float64(numPlayers)
	if randomResult.Completions > 0 && len(randomResult.WinCounts) > 0 {
		baselineWR = float64(randomResult.WinCounts[0]) / float64(randomResult.Completions)
	}

	// In greedy games, player 0 uses greedy AI, rest use random.
	// Greedy win rate = player 0's wins / total completions.
	greedyWR := 0.0
	if len(greedyResult.WinCounts) > 0 {
		greedyWR = float64(greedyResult.WinCounts[0]) / float64(greedyResult.Completions)
	}

	raw := 0.0

	// Tier 1: greedy over the empirical random seat-0 baseline. Degenerate
	// denominator (random seat 0 already wins 100%) leaves no detectable
	// greedy headroom: term is 0.
	if maxDiff := 1.0 - baselineWR; maxDiff > 0 {
		raw += skillGreedyWeight * math.Max(0, greedyWR-baselineWR) / maxDiff
	}

	// Tier 2: ISMCTS over the empirical greedy seat-0 baseline, same shape.
	// Skipped when no MCTS batch ran (greedy-only mode) or when greedy
	// already wins every game (no headroom left to detect, never NaN).
	if mctsResult.Completions > 0 && len(mctsResult.WinCounts) > 0 {
		mctsWR := float64(mctsResult.WinCounts[0]) / float64(mctsResult.Completions)
		if maxDiff := 1.0 - greedyWR; maxDiff > 0 {
			raw += skillMCTSWeight * math.Max(0, mctsWR-greedyWR) / maxDiff
		}
	}

	return clamp(raw/skillScale, 0, 1)
}

// computeSessionLength scores game length measured in DECISIONS PER PLAYER
// against the target band. The old unit was state.Turn, whose meaning varied
// by skeleton -- per-move for shedding, per-card for trick-taking, per-cycle
// for rummy -- so a 52-card whist deal counted as "52 turns" and was silently
// pushed down the long-game falloff while each player actually made only 13
// decisions (audit Task 12). TurnRecords give one record per applied move,
// attributed to the acting player, so the unit is identical across skeletons.
//
// Target band: flat 1.0 in [6, 60] decisions per player, linear ramps
// 3..6 and 60..170, hard zero below 3 or above 170. Calibrated (Task 14)
// from the measured spread of the 8 classic seeds under random play over
// the pinned CalibrationSeeds (mean decisions/player, 200 games each):
//
//	oh-hell 7.0 | mau-mau 12.2 | whist/hearts/spades 13.0 |
//	crazy-eights 18.8 | knock-rummy 77.1 | gin-rummy 150.6
//
// The original band (15-40 flat, zero >100) hard-zeroed gin rummy -- a
// human-validated classic -- while degenerate fixtures sat at 1.0.
//
// ROUND 2 RE-DERIVATION (Task 28 step 4, the one session-scale change this
// round): the flat band's low edge moved 10 -> 6 (ramp 3..6, previously
// 4..10). Justification from the same measured table: oh-hell sits at 7.0
// decisions/player -- a human-validated classic INSIDE the previous ramp,
// paying a 0.5 length penalty for its natural deal size, which left it the
// worst classic (0.428) and 0.003 BELOW the instant-knock fixture's
// single-surviving-seed mean after the round-2 decisions fix. The Task 14
// mandate is "set the band to cover all 8 classics with margin"; 6 covers
// oh-hell with ~15% margin while instant-knock's class is killed by Tier 1
// (too-short AVERAGE games), not by this band -- its surviving seed already
// scored length 1.0, so no degenerate gains from the wider flat. The
// random-play-marathon end stays discounted (gin 0.18): 150 random-play
// decision cycles IS a real playability cost.
func computeSessionLength(result sim.BatchResult, numPlayers int) float64 {
	avg := avgDecisionsPerPlayer(result, numPlayers)
	if avg < 3 || avg > 170 {
		return 0
	}
	if avg >= 6 && avg <= 60 {
		return 1.0
	}
	if avg < 6 {
		return (avg - 3) / 3 // Linear from 3→6
	}
	// avg > 60
	return (170 - avg) / 110 // Linear from 60→170
}

// avgDecisionsPerPlayer returns the batch mean of each game's decisions per
// player: count(TurnRecords where Player==p) averaged over the numPlayers
// seats, which reduces to len(TurnRecords)/numPlayers per game since every
// record belongs to exactly one seat. Computed per game, then averaged over
// the batch. Returns 0 for an empty batch or a nonpositive player count.
func avgDecisionsPerPlayer(result sim.BatchResult, numPlayers int) float64 {
	games := len(result.AllTurns)
	if numPlayers <= 0 || games == 0 {
		return 0
	}
	total := 0
	for _, turns := range result.AllTurns {
		total += len(turns)
	}
	return float64(total) / float64(numPlayers) / float64(games)
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

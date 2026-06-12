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

// ComputeFitness calculates all 5 metrics from simulation results.
func ComputeFitness(
	randomResult sim.BatchResult,
	greedyResult sim.BatchResult,
	numPlayers int,
) Metrics {
	m := Metrics{
		MeaningfulDecisions: computeDecisionDensity(randomResult),
		GameArc:             computeGameArc(randomResult),
		Interaction:         computeInteraction(randomResult),
		SkillGradient:       computeSkillGradient(randomResult, greedyResult, numPlayers),
		SessionLength:       computeSessionLength(randomResult, numPlayers),
	}

	m.TotalFitness = m.MeaningfulDecisions*WeightDecisions +
		m.GameArc*WeightArc +
		m.Interaction*WeightInteract +
		m.SkillGradient*WeightSkill +
		m.SessionLength*WeightLength

	return m
}

// computeDecisionDensity: fraction of decision points where the acting player
// had >= 2 legal moves. This is the metric CLAUDE.md always claimed.
// Forced turns (1 legal move) are not decisions regardless of event type.
func computeDecisionDensity(result sim.BatchResult) float64 {
	total, meaningful := 0, 0
	for _, turns := range result.AllTurns {
		for _, tr := range turns {
			total++
			if tr.LegalMoves >= 2 {
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
//	resolution  = P(winner == leader at the 90% sample)
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
// never led on live deadwood (audit Wave D fix 1). Games with fewer than 4
// leader samples or without a winner (Winner < 0: non-completion) are
// skipped; a batch with no qualifying game scores 0.
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
		if n < 4 {
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
		if track[n*9/10] == winner {
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

// attackDetails enumerates the EventSpecialTriggered Detail strings that
// directly affect an opponent. This is the complete set the shedding runner's
// applySpecialEffects emits (the only EventSpecialTriggered emitter in the
// codebase): draw penalties inflicted on the next player, a skip, and a
// reverse. A future self-targeted special (e.g. a wild-suit choice) must NOT
// be added here.
var attackDetails = map[string]bool{
	"skip":      true,
	"draw_two":  true,
	"draw_four": true,
	"reverse":   true,
}

// computeInteraction: fraction of turns whose move perturbed the next
// player's legal options (TurnRecord.OptionDelta != 0, audit Task 7) or
// carried a direct-attack event (EventTrickWon, or EventSpecialTriggered with
// an opponent-affecting detail). A discard that does not change what the
// opponent can legally do is NOT interaction -- that was the old
// event-taxonomy metric's central flaw: it counted every shedding discard and
// every rummy meld as interactive and pinned hearts-4p at a deterministic
// 0.657 (13 TrickWon / 66 events / 0.3).
//
// Events and TurnRecords are parallel per game but not per-turn indexed, so
// rather than correlating event groups to turn boundaries we count
// option-perturbing turns and attack events separately and take the larger
// count over total turns (the plan's "max ratio" option -- the simplest
// correct approach). Each attack event is emitted by exactly one applied move
// (= one TurnRecord), so each count is a valid lower bound on the number of
// interactive turns, and max never double-counts a turn that both perturbed
// options and attacked. The clamp absorbs the rare move emitting two attack
// events (a card matching both a draw and a skip rule).
//
// Per-skeleton signal sources (Task 7 table): shedding = OptionDelta from the
// discard-top perturbation plus skip/draw/reverse specials; rummy =
// OptionDelta attached to the turn-passing MoveDiscard; trick-taking =
// EventTrickWon only (OptionDelta is 0 BY DESIGN -- follow-suit legality is
// set by the acting player, so the counterfactual is ill-defined).
//
// Scale: clamp(ratio/0.5) is provisional; Task 14 recalibrates the
// denominator from the seed-game spread, not assumption.
func computeInteraction(result sim.BatchResult) float64 {
	totalTurns, deltaTurns := 0, 0
	for _, turns := range result.AllTurns {
		totalTurns += len(turns)
		for _, tr := range turns {
			if tr.OptionDelta != 0 {
				deltaTurns++
			}
		}
	}
	if totalTurns == 0 {
		return 0
	}

	attackEvents := 0
	for _, events := range result.AllEvents {
		for _, e := range events {
			switch e.Type {
			case sim.EventTrickWon:
				attackEvents++
			case sim.EventSpecialTriggered:
				if attackDetails[e.Detail] {
					attackEvents++
				}
			}
		}
	}

	interactive := deltaTurns
	if attackEvents > interactive {
		interactive = attackEvents
	}
	ratio := float64(interactive) / float64(totalTurns)
	return clamp(ratio/0.5, 0, 1)
}

// computeSkillGradient measures whether better play leads to better results.
// Compares greedy AI win rate vs random AI expected win rate.
func computeSkillGradient(randomResult, greedyResult sim.BatchResult, numPlayers int) float64 {
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
	if greedyResult.Completions > 0 && len(greedyResult.WinCounts) > 0 {
		greedyWR = float64(greedyResult.WinCounts[0]) / float64(greedyResult.Completions)
	}

	// Skill = how much better greedy does vs the measured random baseline.
	skillDiff := greedyWR - baselineWR
	if skillDiff < 0 {
		skillDiff = 0 // Greedy no better than random seat 0 = no skill signal
	}

	// Normalize linearly from the baseline (0.0) to a 100% greedy win rate (1.0).
	// Saturates only when greedy wins every game.
	maxDiff := 1.0 - baselineWR
	if maxDiff == 0 {
		return 0
	}

	return clamp(skillDiff/maxDiff, 0, 1)
}

// computeSessionLength scores game length measured in DECISIONS PER PLAYER
// against the target band. The old unit was state.Turn, whose meaning varied
// by skeleton -- per-move for shedding, per-card for trick-taking, per-cycle
// for rummy -- so a 52-card whist deal counted as "52 turns" and was silently
// pushed down the long-game falloff while each player actually made only 13
// decisions (audit Task 12). TurnRecords give one record per applied move,
// attributed to the acting player, so the unit is identical across skeletons.
//
// Target band: 15-40 decisions per player, linear falloff outside, hard zero
// below 5 or above 100. The band and falloff shape carry over from the old
// curve unchanged; only the unit moved. The band is provisional -- Task 14
// recalibrates it from the measured spread of the 8 classic seeds.
func computeSessionLength(result sim.BatchResult, numPlayers int) float64 {
	avg := avgDecisionsPerPlayer(result, numPlayers)
	if avg < 5 || avg > 100 {
		return 0
	}
	if avg >= 15 && avg <= 40 {
		return 1.0
	}
	if avg < 15 {
		return (avg - 5) / 10 // Linear from 5→15
	}
	// avg > 40
	return (100 - avg) / 60 // Linear from 40→100
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

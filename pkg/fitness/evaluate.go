package fitness

import (
	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/mechanic"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

// Tier 2 sample sizes. Greedy was raised from 50 to 200 (Task 13.2): at 50
// games the standard error on the seat-0 win rate is ~0.07, which drowns the
// skill-gradient signal; 200 games cut it to ~0.035. MCTS is 20 games (the
// v2 design's number, Task 20) -- it runs only in EvaluateWithMCTS, never in
// the default Evaluate (see the mode note there), because Task 19 measured
// ~14.5s per 20-game batch at production search strength vs the 2s budget.
const (
	tier2RandomGames = 200
	tier2GreedyGames = 200
	tier2MCTSGames   = 20
)

// MCTSEvalConfig carries the search-strength knobs for the Tier 2 MCTS skill
// batch. Zero values fall back to the sim defaults (200 iterations, 10
// determinizations -- production strength). The batch size itself is fixed
// at tier2MCTSGames and is not a knob.
type MCTSEvalConfig struct {
	Iterations       int
	Determinizations int
}

// EvaluationResult holds the complete fitness evaluation.
type EvaluationResult struct {
	Tier0Errors []string
	Tier1       Tier1Result
	Metrics     Metrics
	Valid       bool
	// DegenerateReason is non-empty when the Tier 2 degeneracy veto rejected
	// the genome (see degeneracy.go): the game passed Tier 1 and produced
	// metrics, but one of its Tier 2 batches carries a degeneracy signature.
	// Random-batch reasons: non_agentic, tempo_monopoly, seat_participation,
	// draw_supply_churn. Greedy-batch reasons (round 3, greedy_ prefix):
	// greedy_timeout, greedy_tempo_monopoly, greedy_seat_participation.
	// Valid is false either way.
	DegenerateReason string
	// Degeneracy carries the raw detector statistics behind the veto
	// decision, for diagnosis and the calibrate command's threshold table
	// (round 3: thresholds are derived from these numbers measured on the
	// classics). Zero-valued for tiers that never ran.
	Degeneracy DegeneracyStats
}

// DegeneracyStats are the per-batch detector inputs (see degeneracy.go for
// each statistic's definition and threshold).
type DegeneracyStats struct {
	RandomMeanRun      float64 // mean consecutive same-player run, random batch
	RandomMinSeatShare float64 // mean min-seat turn share, random batch
	RandomDeltaShare   float64 // share of moves with nonzero OptionDelta (rummy churn)
	RandomAllPlayable  float64 // share of HandSize>=2 records with the whole hand playable (shedding dead_match_rule)
	GreedyMeanRun      float64 // mean consecutive same-player run, greedy batch
	GreedyMinSeatShare float64 // mean min-seat turn share, greedy batch
	GreedyTimeoutShare float64 // share of greedy-batch games hitting the turn cap
	GreedyRan          bool    // false when a random-batch veto skipped the greedy batch
}

// Evaluate runs the default tiered evaluation pipeline for a genome.
// Tier 0: static validation
// Tier 1: 10 quick games
// Tier 2: 200 random + 200 greedy games → fitness metrics
//
// MODE NOTE (audit Task 20): the default pipeline runs NO MCTS batch, so the
// skill gradient is greedy-only (capped at the scaled 0.4 term). Task 19's
// hard budget -- 20 MCTS games per genome in <= 2s -- FAILED by ~7x
// (~14.5s measured, >95% of it in rummy move generation; see pkg/sim/mcts.go),
// so per the plan the production default is MCTS-FOR-TOP-DECILE: rank a
// generation by this greedy two-tier evaluation, then re-evaluate only the
// top decile with EvaluateWithMCTS. Results published from either mode must
// record it in meta.json (Task 28).
func Evaluate(g *genome.Genome, baseSeed uint64) EvaluationResult {
	return evaluate(g, baseSeed, nil)
}

// EvaluateWithMCTS runs the full two-tier pipeline: everything Evaluate does
// plus a 20-game MCTS batch (seat 0 = ISMCTS, others random) feeding the
// skill gradient's second term. cfg tunes search strength only; the
// calibration suite uses reduced knobs (on the record in
// calibration_test.go), production callers should pass the zero value.
func EvaluateWithMCTS(g *genome.Genome, baseSeed uint64, cfg MCTSEvalConfig) EvaluationResult {
	return evaluate(g, baseSeed, &cfg)
}

// evaluate is the shared pipeline; mcts == nil selects default (greedy-only)
// mode.
func evaluate(g *genome.Genome, baseSeed uint64, mcts *MCTSEvalConfig) EvaluationResult {
	result := EvaluationResult{}

	// Tier 0: Static validation
	result.Tier0Errors = genome.Validate(g)
	if len(result.Tier0Errors) > 0 {
		return result
	}

	runner := GetRunner(g)
	if runner == nil {
		result.Tier0Errors = []string{"no runner for skeleton type"}
		return result
	}

	// Build hooks for borrowed mechanics ONCE, before Tier 1: both tiers
	// must simulate the same game (audit Task 16). mechanic.HooksFor is the
	// single shared constructor (audit Task 24) -- the playtest session uses
	// it too, so humans play the same game this pipeline evaluates. Never
	// hand Tier 1 a different hook set.
	hooks := mechanic.HooksFor(g)

	// Tier 1: Quick simulation (10 games)
	result.Tier1 = RunTier1(g, runner, baseSeed, hooks...)
	if !result.Tier1.Passed {
		return result
	}

	// Tier 2: Full simulation

	// Games with random AI
	randomAI := &sim.RandomAI{}
	randomResult := sim.RunBatch(g, runner, randomAI, tier2RandomGames, baseSeed+100, hooks...)

	// Degeneracy veto, random batch (Task 28 round 2): the random batch's
	// turn records are checked for game-shaped non-game signatures BEFORE
	// the genome is declared valid. A vetoed genome's metrics are still
	// computed into EvaluationResult.Metrics (the calibrate subcommand,
	// however, prints n/a for vetoed evals) but it is fitness 0 in the
	// pipeline, exactly like a Tier 1 kill. Both modes share this path, so
	// an MCTS grant can never resurrect a vetoed genome.
	result.Degeneracy.RandomMeanRun = meanConsecutiveRun(randomResult)
	result.Degeneracy.RandomMinSeatShare = meanMinSeatShare(randomResult, g.Players)
	result.Degeneracy.RandomDeltaShare = optionDeltaShare(randomResult)
	result.Degeneracy.RandomAllPlayable = allPlayableShare(randomResult)
	if reason := CheckDegeneracy(randomResult, g); reason != "" {
		result.DegenerateReason = reason
		// No greedy batch for a dead genome: metrics are reported for
		// diagnosis (calibrate) from the random batch alone; skill reads 0.
		result.Metrics = ComputeFitness(randomResult, sim.BatchResult{}, g.Players)
		return result
	}

	// Games with greedy AI (player 0) vs random opponents
	greedyResult := runGreedyBatch(g, runner, tier2GreedyGames, baseSeed+1000, hooks...)

	// Degeneracy veto, greedy batch (Task 28 round 3): the round-2 detectors
	// ran on random play only -- the documented blind spot the r2 flagship
	// exploited (skilled-play-only tempo monopolies, greedy-play cycles to
	// the turn cap). The greedy batch already exists for the skill gradient;
	// it now also feeds tempo/seat/timeout detectors. Same contract as the
	// random veto: metrics survive for diagnosis, fitness 0 in the pipeline,
	// no MCTS resurrection.
	result.Degeneracy.GreedyRan = true
	result.Degeneracy.GreedyMeanRun = meanConsecutiveRun(greedyResult)
	result.Degeneracy.GreedyMinSeatShare = meanMinSeatShare(greedyResult, g.Players)
	if greedyResult.GamesPlayed > 0 {
		result.Degeneracy.GreedyTimeoutShare = float64(greedyResult.Timeouts) / float64(greedyResult.GamesPlayed)
	}
	if reason := CheckGreedyDegeneracy(greedyResult, g); reason != "" {
		result.DegenerateReason = reason
		result.Metrics = ComputeFitness(randomResult, sim.BatchResult{}, g.Players)
		return result
	}

	result.Valid = true

	// Optional MCTS batch (player 0) vs random opponents. Seed offset +2000
	// keeps the tier-1 (+0), random (+100), and greedy (+1000) batches
	// bit-identical between the two modes: EvaluateWithMCTS can only ADD the
	// second skill term, never perturb the other metrics.
	var mctsResult sim.BatchResult
	if mcts != nil {
		mctsResult = runMCTSBatch(g, runner, tier2MCTSGames, baseSeed+2000, *mcts, hooks...)
	}

	// Compute fitness metrics
	result.Metrics = ComputeFitnessWithMCTS(randomResult, greedyResult, mctsResult, g.Players)

	return result
}

// runGreedyBatch runs games where player 0 uses greedy AI and others use
// random. computeSkillGradient reads player 0's win rate, so mixing AIs per
// seat is the only way the metric can distinguish skill from symmetry.
func runGreedyBatch(g *genome.Genome, runner sim.GenericRunner, n int, baseSeed uint64, hooks ...sim.HookFunc) sim.BatchResult {
	players := make([]sim.AIPlayer, g.Players)
	players[0] = GetGreedyAI(g)
	random := &sim.RandomAI{}
	for i := 1; i < g.Players; i++ {
		players[i] = random
	}
	ai := &sim.PerPlayerAI{Players: players, Fallback: random}
	return sim.RunBatch(g, runner, ai, n, baseSeed, hooks...)
}

// runMCTSBatch runs games where player 0 uses the determinized ISMCTS player
// and others use random -- the second skill tier's measurement batch, shaped
// exactly like runGreedyBatch so the two tiers' seat-0 rates are comparable.
// Known model limitation carried from Task 19: the search's internal model is
// hook-blind (borrowed-mechanic hooks apply only in the outer batch loop),
// mirroring the greedy scorer's situation.
func runMCTSBatch(g *genome.Genome, runner sim.GenericRunner, n int, baseSeed uint64, cfg MCTSEvalConfig, hooks ...sim.HookFunc) sim.BatchResult {
	players := make([]sim.AIPlayer, g.Players)
	players[0] = &sim.MCTSAI{
		Runner:           runner,
		Genome:           g,
		Iterations:       cfg.Iterations,
		Determinizations: cfg.Determinizations,
	}
	random := &sim.RandomAI{}
	for i := 1; i < g.Players; i++ {
		players[i] = random
	}
	ai := &sim.PerPlayerAI{Players: players, Fallback: random}
	return sim.RunBatch(g, runner, ai, n, baseSeed, hooks...)
}


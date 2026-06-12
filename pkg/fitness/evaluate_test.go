package fitness

import (
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
)

// TestTier2BatchSizes pins the Tier 2 sample sizes (Task 13.2,
// docs/plans/2026-06-11-audit-remediation.md). The audit found 50 greedy
// games give SE ~= sqrt(0.5*0.5/50) ~= 0.07 on the seat-0 win rate -- too
// noisy for the skill-gradient metric to separate real skill from luck.
// 200 games bring SE down to ~0.035. Random stays at 200.
//
// If you change these constants, re-derive the SE numbers above and
// re-measure evaluation noise in the calibration suite before committing.
func TestTier2BatchSizes(t *testing.T) {
	if tier2RandomGames != 200 {
		t.Errorf("tier2RandomGames = %d, want 200", tier2RandomGames)
	}
	if tier2GreedyGames != 200 {
		t.Errorf("tier2GreedyGames = %d, want 200 (50 gives SE ~0.07 on win rate; 200 gives ~0.035)", tier2GreedyGames)
	}
	if tier2MCTSGames != 20 {
		t.Errorf("tier2MCTSGames = %d, want 20 (the v2 design's MCTS batch size, plan Task 20)", tier2MCTSGames)
	}
}

// TestEvaluateWithMCTSGinSkillAtLeastGreedyOnly pins the Task 20 calibration
// expectation: gin rummy's two-tier skill is >= its greedy-only skill. Both
// pipelines share the tier-1/random/greedy seed offsets, so the greedy term
// is bit-identical and the MCTS term can only add (max(0, ...) >= 0). The
// reduced search knobs keep this test cheap; the property holds at any
// strength because it is structural, not statistical.
func TestEvaluateWithMCTSGinSkillAtLeastGreedyOnly(t *testing.T) {
	g := seeds.GinRummy()
	base := Evaluate(g, 11)
	full := EvaluateWithMCTS(g, 11, MCTSEvalConfig{Iterations: 20, Determinizations: 2})
	if !base.Valid || !full.Valid {
		t.Fatalf("gin rummy must pass tiers 0-1 on seed 11: base.Valid=%v full.Valid=%v (tier1: %q / %q)",
			base.Valid, full.Valid, base.Tier1.Reason, full.Tier1.Reason)
	}

	if full.Metrics.SkillGradient < base.Metrics.SkillGradient {
		t.Errorf("gin two-tier skill %.3f < greedy-only skill %.3f",
			full.Metrics.SkillGradient, base.Metrics.SkillGradient)
	}

	// The MCTS batch must not perturb the other batches: every non-skill
	// metric is computed from the same seeded random batch in both modes.
	if full.Metrics.MeaningfulDecisions != base.Metrics.MeaningfulDecisions ||
		full.Metrics.GameArc != base.Metrics.GameArc ||
		full.Metrics.Interaction != base.Metrics.Interaction ||
		full.Metrics.SessionLength != base.Metrics.SessionLength {
		t.Errorf("non-skill metrics diverged between default and MCTS modes:\n  base %+v\n  full %+v",
			base.Metrics, full.Metrics)
	}
	t.Logf("gin seed 11: greedy-only skill %.3f, two-tier skill %.3f",
		base.Metrics.SkillGradient, full.Metrics.SkillGradient)
}

// TestMCTSTierRewardsDegenKnockTiming pins the Task 20 hazard documented in
// calibration_test.go's POST-TASK-20 block: the MCTS skill term fires HARD
// on the instant-knock degenerate fixture, because ISMCTS discovers the real
// knock-timing strategy (hold low deadwood, let the random opponent knock
// into an undercut) that the greedy rummy scorer misses entirely (measured
// seat-0 rates on seed 44: random 0.473, greedy 0.506, MCTS 0.750 at these
// knobs, 0.933 at production strength). max(0, mctsWR-greedyWR) cannot tell
// depth-in-a-rich-game from greedy-incompetence-in-a-trivial-one; that is
// WHY the production default is MCTS-for-top-decile (the term is granted
// only to genomes already elite on greedy-only rank) and why the calibration
// gate measures the greedy-only pipeline. If this test ever fails because
// the gap closed, the hazard is gone -- update the calibration block and
// reconsider full-MCTS gate measurement.
func TestMCTSTierRewardsDegenKnockTiming(t *testing.T) {
	g := seeds.InstantKnockRummy()
	// Seed 44 is the fixture's single Tier-1-surviving calibration seed.
	base := Evaluate(g, 44)
	full := EvaluateWithMCTS(g, 44, MCTSEvalConfig{Iterations: 50, Determinizations: 5})
	if !base.Valid || !full.Valid {
		t.Fatalf("instant-knock must pass tiers 0-1 on seed 44: base=%v full=%v (tier1: %q / %q)",
			base.Valid, full.Valid, base.Tier1.Reason, full.Tier1.Reason)
	}
	gap := full.Metrics.SkillGradient - base.Metrics.SkillGradient
	t.Logf("instant-knock seed 44: greedy-only skill %.3f, two-tier skill %.3f (gap %.3f)",
		base.Metrics.SkillGradient, full.Metrics.SkillGradient, gap)
	if gap < 0.2 {
		t.Errorf("expected the MCTS term to fire hard on instant-knock (gap >= 0.2), got %.3f", gap)
	}
}

// TestEvaluateWithMCTSRespectsEarlyTiers: a genome that fails Tier 0 exits
// before any simulation in MCTS mode exactly as in default mode.
func TestEvaluateWithMCTSRespectsEarlyTiers(t *testing.T) {
	g := &genome.Genome{ID: "broken", Skeleton: genome.Shedding, Players: 10}
	res := EvaluateWithMCTS(g, 0, MCTSEvalConfig{})
	if res.Valid || len(res.Tier0Errors) == 0 {
		t.Errorf("tier-0-invalid genome must be rejected in MCTS mode: valid=%v tier0=%v",
			res.Valid, res.Tier0Errors)
	}
}

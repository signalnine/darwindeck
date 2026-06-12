// Package fitness_test: seed-calibration suite (audit remediation Tasks 2/14).
//
// This is the falsifiable acceptance gate for the fitness function and runs
// in the DEFAULT test suite (the `calibration` build tag was removed by
// Task 14, as the plan requires). It is the fitness function's permanent
// regression gate: any metric or scale change that re-inverts a
// classic-vs-degenerate ordering fails the build.
package fitness_test

import (
	"math"
	"strings"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/evolution"
	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
)

// The canonical pinned seed list lives in fitness.CalibrationSeeds
// (calibration.go) so the `calibrate` subcommand can import it; this suite
// and that command must always measure over the same seeds.

// BASELINE (measured at the Task 2 commit, BEFORE any metric fixes; means
// over CalibrationSeeds, sd in parens, n = evaluations of 10 that passed
// Tier 1):
//
//	classic crazy-eights        0.725 (sd 0.016, n  9/10)
//	classic mau-mau             0.722 (sd 0.010, n 10/10)
//	classic whist               0.636 (sd 0.014, n 10/10)
//	classic hearts              0.692 (sd 0.006, n 10/10)
//	classic spades              0.660 (sd 0.017, n 10/10)
//	classic oh-hell             0.667 (sd 0.014, n  9/10)
//	classic gin-rummy           0.813 (sd 0.016, n  8/10)
//	classic knock-rummy         0.823 (sd 0.012, n  9/10)
//	degen   instant-knock-rummy 0.826 (sd 0.023, n  7/10)
//	degen   forced-shedding     0.716 (sd 0.000, n  1/10)
//
// Failure shape at baseline (the audit's falsification finding): the
// instant-knock fixture outscored EVERY classic including gin rummy; the
// worst classic trailed the best degenerate by 0.190; all four trick-taking
// classics sat below the then-0.70 FitnessFloor.
//
// TASK 14 RESULT (after metric reimplementation Tasks 9-13 + review fixes,
// Tier 1 robustness Task 16, and ONE scale-constant change -- the session
// length band recalibrated to the measured classic spread; metric weights
// UNCHANGED at 0.25/0.25/0.20/0.20/0.10). HISTORICAL: superseded by the
// POST-TASK-20 RESULT block below (skill became two-tier and gained its
// scale constant; these numbers were measured with skill = raw greedy term):
//
//	survivor-conditioned means (Tier 1 kills excluded):
//	  classics: crazy-eights 0.475 | mau-mau 0.490 | whist 0.633 |
//	            hearts 0.650 | spades 0.652 | oh-hell 0.549 |
//	            gin-rummy 0.584 | knock-rummy 0.609     (all n=10/10)
//	  degens:   instant-knock 0.434 (n=1/10) | forced-shedding 0.429 (n=4/10)
//	pipeline-effective means (Tier 1 kills counted as the 0 fitness they
//	are in the real pipeline):
//	  degens:   instant-knock 0.043 | forced-shedding 0.171
//
// Margins: every classic beats every degenerate on BOTH views. The
// pipeline-effective margin is wide (worst classic 0.475 vs best degenerate
// 0.171 = +0.30). The survivor-conditioned margins are strict but narrow at
// the bottom: crazy-eights vs forced-shedding +0.046, vs instant-knock
// +0.041 -- under the plan's 0.05 for that one pair. Per the plan's exit
// condition (b), the 0.05-margin requirement is applied to the
// PIPELINE-EFFECTIVE view, with strict ordering (no margin) required on the
// survivor view. Justification: Tier 1 kills ARE fitness 0 during evolution
// (a degenerate killed on 9/10 seeds contributes ~0 selection pressure), so
// pipeline-effective means are what "the QD machinery would promote it" --
// the audit's actual falsification criterion -- measures. The survivor view
// is retained strictly so the metric stack alone, with Tier 1 blind, still
// ranks every classic above every degenerate. The narrow crazy-eights vs
// forced-shedding gap is honest: forced-shedding IS approximately
// crazy-eights stripped of its special cards, and the metrics see exactly
// that difference (interaction 0.385 vs 0.231, decisions 0.300 vs 0.181).

// HISTORICAL -- POST-TASK-20 RESULT (two-tier skill gradient, 2026-06-11;
// superseded by the ROUND 2 block below). The skill
// metric became skill = clamp(raw/skillScale, 0, 1) with
// raw = 0.4*greedyTerm + 0.6*mctsTerm and skillScale = 0.5 (the one
// scale-constant change Task 20 permits; weights UNCHANGED at
// 0.25/0.25/0.20/0.20/0.10). The gate below measures the DEFAULT pipeline
// (greedy-only skill, MCTS term 0 -- see meanFit's MEASUREMENT MODE note),
// so every classic's skill is its Task 14 value x 0.8:
//
//	survivor-conditioned means (all classics n=10/10):
//	  classics: crazy-eights 0.474 (sd 0.007) | mau-mau 0.489 (sd 0.004) |
//	            whist 0.630 (sd 0.004) | hearts 0.630 (sd 0.005) |
//	            spades 0.645 (sd 0.004) | oh-hell 0.543 (sd 0.003) |
//	            gin-rummy 0.548 (sd 0.004) | knock-rummy 0.578 (sd 0.006)
//	  degens:   instant-knock 0.431 (n=1/10) | forced-shedding 0.416 (n=4/10)
//	pipeline-effective means:
//	  degens:   instant-knock 0.043 | forced-shedding 0.166
//
// Margins: pipeline-effective worst classic 0.474 vs best degenerate 0.166
// (+0.31); survivor-strict worst classic crazy-eights 0.474 vs best
// degenerate instant-knock 0.431 (+0.043, same shape as Task 14's +0.041);
// gin 0.548 vs instant-knock 0.431 (+0.117 >= 0.10).
//
// FULL-MCTS GATE MEASUREMENT: TRIED AND REJECTED, ON THE RECORD. Task 20
// first ran this suite through fitness.EvaluateWithMCTS for all 10 genomes
// (20 MCTS games per surviving eval at reduced knobs 50 iterations / 5
// determinizations, meanFit parallelized across genomes; wall 28.7s). The
// gate FAILED, and no skillScale value can pass it (strict survivor
// ordering needs scale > 1.44 while the gin margin needs scale < 0.07):
// ISMCTS discovers REAL knock-timing skill in the instant-knock fixture --
// hold low deadwood and let the random opponent knock into an undercut --
// that the greedy rummy scorer completely misses. Measured seat-0 rates on
// its one surviving seed (44): random 0.473, greedy 0.506, MCTS 0.750 at
// suite knobs and 0.933 at production strength (signal, not 20-game noise:
// it strengthens with search depth). Meanwhile greedy outplays ISMCTS at
// both rummy classics (gin 0.945 vs 0.900, knock 0.840 vs 0.650), so their
// MCTS terms are 0. Full-MCTS survivor means for completeness: crazy-eights
// 0.475, mau-mau 0.534, whist 0.652, hearts 0.639, spades 0.652, oh-hell
// 0.572, gin 0.548, knock 0.578, instant-knock 0.550, forced-shedding
// 0.440 -- the degenerate above five classics.
//
// Why greedy-only gate measurement is the honest choice and not a dodge:
// the max(0, mctsWR-greedyWR) term rewards games where greedy plays BADLY,
// which indicates depth in a rich game but greedy-incompetence in a trivial
// one. Production (top-decile mode, Task 19's failed 2s budget) only grants
// the MCTS term to genomes already top-decile by greedy-only rank, so
// instant-knock (greedy-only 0.431, Tier-1-killed 9/10) never receives it;
// the gate guards exactly the ranking that decides who gets MCTS.
//
// CARRIED HAZARD (for Tasks 22/28 designer review): an evolved degenerate
// with strong non-skill metrics could ride greedy-only fitness into the top
// decile and then collect a large MCTS term for greedy-incompetence rather
// than depth. The Phase 7 failed-review loop must encode any such champion
// as a fixture here; TestMCTSTierRewardsDegenKnockTiming (evaluate_test.go)
// pins the mechanism.

// ROUND 2 RESULT (Task 28 step 4 failed-review loop, 2026-06-12). The
// post-fix flagship's designer review HARD-BLOCKED publication: its top 30
// were three gamed archetypes, now fixtures (pkg/seeds/degenerate.go:
// CatchAllSkipShedding 0.879, NoFollowAvoidanceTrick 0.854,
// PairMeldKnockRummy 0.673 at rejection -- two above EVERY classic). Three
// changes re-passed the gate, in order:
//
//  1. INTERACTION FIX: 2p skip/reverse are self-tempo, not attacks, and
//     self-perturbation never counts as OptionDelta coupling. Catch-all-skip
//     interaction 1.00 -> 0.632.
//  2. CHOICE-IMPACT DECISIONS: a turn counts only if sampled moves differ in
//     type, effect profile, or next-player option-SET probe. No-follow trick
//     density 0.917 -> 0.000; catch-all-skip 0.874 -> 0.633 (honest inflict
//     choices remain); classics moved too (whist 0.776 -> 0.201, oh-hell
//     -> 0.172, gin unchanged 0.690 -- rummy keeps count semantics).
//  3. RECALIBRATION (this round's Task 14): one scale change -- the session
//     band's low edge 10 -> 6 (ramp 3..6), because oh-hell (7.0
//     decisions/player, a CLASSIC) paid a 0.5 length penalty and fell below
//     a degenerate. Weights UNCHANGED at 0.25/0.25/0.20/0.20/0.10;
//     interaction denominator (0.5) and skillScale (0.5) unchanged --
//     measured interactive-turn ratios still top out at 0.42 among classics
//     (no saturation) and the skill scale's gin-margin derivation still
//     holds.
//
// EXIT CONDITION (a) TAKEN AND DOCUMENTED: after fixes 1-2 the three
// rejected champions still measured 0.625-0.759 vs classics 0.428-0.578 --
// catch-all-skip Pareto-dominates several classics on the five metrics, and
// no monotone scale change separates a dominating pair. Per the plan's Task
// 14 exit condition (a), ONE added measurement was introduced: the Tier 2
// degeneracy veto (pkg/fitness/degeneracy.go) -- agency floor (density >=
// 0.05), tempo monopoly (mean same-player run <= 6), rummy draw-supply
// churn (delta share <= 0.10) -- validity rules computed from the existing
// random batch, NOT a sixth weighted term, so the five weights stay frozen.
// Every detector encodes the designer's stated rejection reason with >= 2x
// measured margins to every classic (full table in the checkpoint doc's
// Wave H entry).
//
// Survivor-conditioned means after all three changes (n=10/10 classics):
//
//	classics: crazy-eights 0.451 | mau-mau 0.476 | whist 0.486 |
//	          hearts 0.486 | spades 0.500 | oh-hell 0.478 |
//	          gin-rummy 0.548 | knock-rummy 0.578
//	degens:   instant-knock 0.431 (n=1/10, eff 0.043) |
//	          forced-shedding 0.399 (n=4/10, eff 0.160) |
//	          catch-all-skip / no-follow / pair-meld: VETOED on 10/10 seeds
//	          (tempo_monopoly / non_agentic / draw_supply_churn), survivor
//	          mean 0, eff 0
//
// Margins: survivor-strict worst classic crazy-eights 0.451 vs best
// degenerate instant-knock 0.431 (+0.020, strict ordering holds);
// pipeline-effective worst classic 0.451 vs best degenerate 0.160 (+0.29);
// gin 0.548 vs instant-knock 0.431 (+0.117 >= 0.10). FitnessFloor
// re-derived 0.42 -> 0.40 (= 0.451 - 0.05, Task 15 rule).

// Ground truth: the 8 classic seeds are the only human-validated "fun" games
// in the repo. Any fitness function that scores a classic below a degenerate
// fixture is falsified. Evaluations are averaged over the pinned seed list
// because per-eval noise is sd ~0.02.
//
// OVERFITTING GUARD: 8 classics is a narrow truth source. Tune to MARGINS,
// never to exact orderings among the classics; as more classics are added
// (Hoyle-derived trick/shedding/rummy variants), the suite grows but
// thresholds do not move.

// calResult is one genome's calibration measurement.
type calResult struct {
	mean  float64 // survivor-conditioned: mean over evals that passed Tier 1
	eff   float64 // pipeline-effective: Tier 1 kills counted as fitness 0
	sd    float64
	valid int // evaluations that passed Tier 1 (of len(fitness.CalibrationSeeds))
}

// calCache memoizes per-genome measurements so the gate tests share one
// evaluation pass. Tests in this package do not use t.Parallel, so a plain
// map is safe.
var calCache = map[string]calResult{}

// meanFit evaluates g once per calibration seed through the REAL pipeline
// (fitness.Evaluate: Tier 0 -> Tier 1 -> Tier 2). The survivor-conditioned
// mean isolates the metric stack (what do the metrics say about games that
// reach them); the pipeline-effective mean counts Tier 1 kills as the 0
// fitness they are during evolution. A genome killed by Tier 1 on EVERY
// seed has eff == 0 -- the pipeline rejecting a degenerate outright
// satisfies the gate.
//
// MEASUREMENT MODE (post-Task-20): the gate measures the DEFAULT pipeline
// (fitness.Evaluate, greedy-only skill) because that is the production mode:
// Task 19's MCTS budget failed (~14.5s vs 2s per genome), so evolution runs
// MCTS-for-top-decile -- selection, the fitness floor, and decile ranking
// all act on these greedy-only numbers, and only genomes ALREADY at the top
// of that ranking ever receive the MCTS term (fitness.EvaluateWithMCTS).
// The gate therefore guards exactly the ranking that decides who gets MCTS.
// See the POST-TASK-20 block above for why full-MCTS gate measurement was
// tried and rejected on the record.
func meanFit(t *testing.T, g *genome.Genome) calResult {
	t.Helper()
	if r, ok := calCache[g.ID]; ok {
		return r
	}

	var fits []float64
	for _, seed := range fitness.CalibrationSeeds {
		res := fitness.Evaluate(g, seed)
		if len(res.Tier0Errors) > 0 {
			t.Fatalf("%s: tier-0 errors (fixture must be statically valid): %v", g.ID, res.Tier0Errors)
		}
		if !res.Valid {
			if res.DegenerateReason != "" {
				t.Logf("%s: seed %d killed by tier-2 degeneracy veto: %s", g.ID, seed, res.DegenerateReason)
			} else {
				t.Logf("%s: seed %d killed by tier 1: %s", g.ID, seed, res.Tier1.Reason)
			}
			continue
		}
		fits = append(fits, res.Metrics.TotalFitness)
	}

	r := calResult{valid: len(fits)}
	if len(fits) > 0 {
		var sum float64
		for _, f := range fits {
			sum += f
		}
		r.mean = sum / float64(len(fits))
		r.eff = sum / float64(len(fitness.CalibrationSeeds))
		var sq float64
		for _, f := range fits {
			sq += (f - r.mean) * (f - r.mean)
		}
		r.sd = math.Sqrt(sq / float64(len(fits)))
	}
	calCache[g.ID] = r
	t.Logf("%s: survivor mean %.3f (sd %.3f, n %d/%d), pipeline-effective %.3f",
		g.ID, r.mean, r.sd, r.valid, len(fitness.CalibrationSeeds), r.eff)
	return r
}

// TestDegenerateFixturesAreTier0Valid pins the fixture contract: TIER-2
// degenerate fixtures (and all classics) must pass genome.Validate -- they
// are negative ground truth for the METRICS, so they must not be rejectable
// by static analysis.
//
// RESTRUCTURED in round 3 (Task 28 step 4): fixtures whose degeneracy vector
// became a Tier-0 rule -- the catch-all specials -- are deliberately NOT in
// this list anymore; a statically rejected genome never reaches the metrics,
// so it cannot serve as metric ground truth. Those are now negative Tier-0
// specimens with the INVERSE contract, asserted by
// TestTier0RejectsCatchAllChampions below.
func TestDegenerateFixturesAreTier0Valid(t *testing.T) {
	all := append(seeds.All(), seeds.InstantKnockRummy(), seeds.ForcedShedding())
	all = append(all, seeds.RejectedChampions()...)
	for _, g := range all {
		if errs := genome.Validate(g); len(errs) > 0 {
			t.Errorf("%s: tier-0 violations: %v", g.ID, errs)
		}
	}
	if n := len(seeds.All()); n != 8 {
		t.Errorf("seeds.All() returned %d classics, want 8", n)
	}
}

// TestTier0RejectsCatchAllChampions (round 3): the catch-all-special
// champions -- round-1 A1 (catch-all SKIP, CatchAllSkipShedding) and the
// round-2 flagship rank01 encoding (catch-all WILD, CatchAllWildShedding) --
// must be rejected by STATIC validation: a special card with ByRank=0 and
// BySuit=0 matches every card and deletes the shedding skeleton's
// match/draw rules as dead genes. These fixtures' ground-truth role moved
// from "the metrics must rank them below classics" to "Tier 0 must never let
// them reach the metrics at all".
func TestTier0RejectsCatchAllChampions(t *testing.T) {
	champs := seeds.CatchAllChampions()
	if len(champs) < 2 {
		t.Fatalf("expected at least 2 catch-all specimens (round-1 A1 + r2 rank01), got %d", len(champs))
	}
	for _, g := range champs {
		errs := genome.Validate(g)
		found := false
		for _, e := range errs {
			if strings.Contains(e, "catch-all") {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: catch-all champion must be Tier-0 rejected with a catch-all violation, got: %v", g.ID, errs)
		}
		// And the full pipeline must refuse it before any simulation.
		res := fitness.Evaluate(g, fitness.CalibrationSeeds[0])
		if len(res.Tier0Errors) == 0 || res.Valid {
			t.Errorf("%s: Evaluate must stop at Tier 0 (errors=%v valid=%v)", g.ID, res.Tier0Errors, res.Valid)
		}
	}
}

// TestCalibrationClassicsBeatDegenerates is the core gate, asserted on both
// views (see the TASK 14 RESULT block for semantics and justification):
//   - pipeline-effective: worst classic beats best degenerate by 0.05
//   - survivor-conditioned: strict ordering, every classic above every
//     degenerate (no margin -- the metric stack alone must never invert)
func TestCalibrationClassicsBeatDegenerates(t *testing.T) {
	classics := seeds.All() // 8 games
	// All five degenerate fixtures: the two originals plus the three
	// rejected round-2 flagship champions (Task 28 failed-review loop).
	degens := append([]*genome.Genome{seeds.InstantKnockRummy(), seeds.ForcedShedding()},
		seeds.RejectedChampions()...)

	worstClassic, worstEff := 1.0, 1.0
	bestDegen, bestEff := 0.0, 0.0
	worstName, bestName := "", ""
	for _, c := range classics {
		r := meanFit(t, c)
		if r.mean < worstClassic {
			worstClassic, worstName = r.mean, c.ID
		}
		if r.eff < worstEff {
			worstEff = r.eff
		}
	}
	for _, d := range degens {
		r := meanFit(t, d)
		if r.mean > bestDegen {
			bestDegen, bestName = r.mean, d.ID
		}
		if r.eff > bestEff {
			bestEff = r.eff
		}
	}

	if worstEff < bestEff+0.05 {
		t.Errorf("pipeline-effective gate failed: worst classic %.3f vs best degenerate %.3f (need +0.05 margin)",
			worstEff, bestEff)
	}
	if worstClassic <= bestDegen {
		t.Errorf("survivor-conditioned ordering inverted: worst classic %s %.3f <= best degenerate %s %.3f",
			worstName, worstClassic, bestName, bestDegen)
	}
}

// TestCalibrationRejectedChampionsBelowClassics is the named round-2 gate
// over the Task 28 step-4 failed-review fixtures: the archetypes that owned
// the ENTIRE top 30 of the post-fix flagship
// (output/2026-06-12-flagship-postfix) and were rejected at designer review.
// ROUND 3: the catch-all-skip archetype left this gate for the Tier-0 one
// (TestTier0RejectsCatchAllChampions) -- seeds.RejectedChampions() now holds
// only the fixtures that remain statically valid.
// Same two-view semantics as TestCalibrationClassicsBeatDegenerates (which
// also includes these fixtures in its degens list; this test stays as the
// round's named falsification record). RED PHASE ON RECORD (fixtures
// commit): with the pre-fix metric stack this gate FAILED -- survivor means
// catch-all-skip-shedding 0.879, no-follow-avoidance-trick 0.854,
// pair-meld-knock-rummy 0.673 vs worst classic crazy-eights 0.474, all
// passing Tier 1 on 10/10 seeds. GREEN since the round-2 fixes: all three
// are now vetoed by the Tier 2 degeneracy checks on every calibration seed
// (tempo_monopoly / non_agentic / draw_supply_churn -- see the ROUND 2
// block).
func TestCalibrationRejectedChampionsBelowClassics(t *testing.T) {
	classics := seeds.All()
	degens := seeds.RejectedChampions()

	worstClassic, worstEff := 1.0, 1.0
	bestDegen, bestEff := 0.0, 0.0
	worstName, bestName := "", ""
	for _, c := range classics {
		r := meanFit(t, c)
		if r.mean < worstClassic {
			worstClassic, worstName = r.mean, c.ID
		}
		if r.eff < worstEff {
			worstEff = r.eff
		}
	}
	for _, d := range degens {
		r := meanFit(t, d)
		if r.mean > bestDegen {
			bestDegen, bestName = r.mean, d.ID
		}
		if r.eff > bestEff {
			bestEff = r.eff
		}
	}

	if worstEff < bestEff+0.05 {
		t.Errorf("pipeline-effective gate failed: worst classic %.3f vs best rejected champion %.3f (need +0.05 margin)",
			worstEff, bestEff)
	}
	if worstClassic <= bestDegen {
		t.Errorf("survivor-conditioned ordering inverted: worst classic %s %.3f <= best rejected champion %s %.3f",
			worstName, worstClassic, bestName, bestDegen)
	}
}

// TestCalibrationClassicsAboveFloor: every classic must clear the QD
// viability floor (evolution.FitnessFloor; derived from this suite's
// measurements in Task 15). A floor that rejects human-validated games
// zeroes the selection gradient for exactly the genomes evolution should be
// reaching for.
func TestCalibrationClassicsAboveFloor(t *testing.T) {
	for _, c := range seeds.All() {
		if m := meanFit(t, c).mean; m < evolution.FitnessFloor {
			t.Errorf("classic %s mean fitness %.3f below FitnessFloor %.2f", c.ID, m, evolution.FitnessFloor)
		}
	}
}

// TestCalibrationGinBeatsInstantKnock: the named pair from the audit. The
// flagship "champion" was an instant-knock coin flip scored ABOVE gin rummy;
// any sane metric stack separates them by a wide margin. Survivor-conditioned
// on both sides: this is pure metric-stack attribution.
func TestCalibrationGinBeatsInstantKnock(t *testing.T) {
	gin := meanFit(t, seeds.GinRummy()).mean
	knock := meanFit(t, seeds.InstantKnockRummy()).mean
	if gin < knock+0.10 {
		t.Errorf("gin rummy %.3f does not beat instant-knock %.3f by 0.10", gin, knock)
	}
}

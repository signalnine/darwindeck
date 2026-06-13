# Audit Remediation -- Checkpoint Log

Tracking doc required by Task 13.5. Baselines first, per-metric calibration tables appended as phases land.

## Throughput baseline (pre-Task-7, commit 8c36b21-dirty, 2026-06-11)

Command: `./bin/darwindeck evolve -population 50 -generations 10 -workers 8`

- Wall: 1.92s, CPU: 12.1s user + 0.9s sys
- ~500 genome evaluations (Tier 1 + Tier 2 ~255 games each where passed) => roughly 65k games / 12s CPU = ~5,400 games/sec/core-equivalent
- Regression gate per plan: >3x drop at any later checkpoint triggers a profiling sub-task

Top-of-run metric fingerprint (pre-remediation, for before/after comparison):
`gen4_47426 (0.916) decisions=0.79 arc=1.00 interact=1.00 skill=0.84 length=1.00`
(arc and interact pinned at 1.00 -- the audit's structural-constant finding, expected to disappear after Phase 2)

## Calibration baseline (Task 2 commit a9ecaf3, pre-metric-fix, means over CalibrationSeeds, n = evals passing Tier 1)

Classics: crazy-eights 0.725 (9), mau-mau 0.722 (10), whist 0.636 (10), hearts 0.692 (10), spades 0.660 (10), oh-hell 0.667 (9), gin-rummy 0.813 (8), knock-rummy 0.823 (9).
Degenerates: instant-knock-rummy 0.826 (7), forced-shedding 0.716 (1; Tier 1-killed 9/10).

Falsification confirmed numerically: the instant-knock degenerate outscores every classic; worst classic (whist) trails the best degenerate by 0.190; all four trick-taking classics sit below FitnessFloor 0.70.

## Wave log

- Wave A (Tasks 1, 4, 5-safe, 6, 23, 25): done 2026-06-11 -- commits 8eda14b, afa283e, 4127df5, 1ce1b79, 631e198, 4f4854b. Epub history rewrite COMPLETED 2026-06-12 with user approval: filter-repo across master + 4 stale remote branches (rewritten, not deleted), force-pushed, all remote histories verified clean; pre-rewrite bundle retained locally in /tmp.
- Wave B (Tasks 2, 3): done 2026-06-11 -- commits a9ecaf3, bec2e3e. Code review: APPROVE. Bonus finding fixed: rummy CheckEnd double-banked deadwood (winner-flipping); banking moved to Upkeep. Hazard comment pinned in f119093. NOTE for Task 19 (MCTS): rummy/tricktaking Upkeep is NOT idempotent -- tree search must preserve the once-per-iteration contract on cloned states.
- Wave C (Tasks 7, 8): done 2026-06-11 -- commits fe3635e, 78ece7d. Rummy OptionDelta implemented analytically (full-union probe was 4.15x; cancellation argument documented in batch.go with invalidation condition: lay-off-style mechanics break it). Rummy hot path optimized (fixed-array buckets + value-only deadwood DP) after Progress pushed it to 2.96x baseline; now 1.006x.
- Wave D (Tasks 9-13): done 2026-06-11 -- commits 47b1e03, 4aff683, 3fa2ecd, ac77034, 5989c2f, d2c1dcc, f882a67. Metrics un-pinned: whist density 0.775 (was 1.0), arc varies 0.418-0.864 across seeds (was ~1.0), hearts-4p interaction off the old 0.657 pin. Code review: FIX-REQUIRED (narrow) -> Wave D.1 fixes in flight (phantom winners in arc, short-track resolution freebie, TurnRecord.Attack union semantics, TT lead-constraint OptionDelta -- plan table amended in a7fe81c).
- CARRIED FINDINGS: (a) Task 18 must also remove the winner's-curse skip in novelty.go:109 and MAP-Elites best-ever cells (engine.go got the fix in f882a67; the other two engines did not). (b) Phase 7 docs: Individual.Fitness components are last-eval values while TotalFitness is a running mean -- champion metric breakdowns will not reconcile with weights; document when publishing. (c) Hook effects (e.g. DrawPenalty borrow) are invisible to OptionDelta/leader sampling (recorded pre-hook) -- relevant to Task 22's evolvability sub-check.

## Task 13.5 per-metric table (post Wave D.1)

Command: `./bin/darwindeck calibrate` (commit 22872f4 + calibrate subcommand; single-threaded; raw metrics, no weighting; tier1 = evals reaching Tier 2 over the 10 pinned CalibrationSeeds).

```
genome                 skeleton      tier1  decisions       arc             interact        skill           length
---------------------------------------------------------------------------------------------------------------------------
crazy-eights           shedding      10/10  0.300 sd 0.003  0.866 sd 0.015  0.385 sd 0.003  0.033 sd 0.033  1.000 sd 0.000
mau-mau                shedding      9/10   0.288 sd 0.002  0.798 sd 0.020  0.564 sd 0.005  0.030 sd 0.026  0.728 sd 0.021
whist                  trick_taking  10/10  0.776 sd 0.000  0.609 sd 0.007  0.844 sd 0.002  0.091 sd 0.020  0.800 sd 0.000
hearts                 trick_taking  10/10  0.777 sd 0.001  0.359 sd 0.009  0.844 sd 0.001  0.484 sd 0.017  0.800 sd 0.000
spades                 trick_taking  10/10  0.769 sd 0.001  0.631 sd 0.021  0.835 sd 0.001  0.176 sd 0.022  0.800 sd 0.000
oh-hell                trick_taking  9/10   0.634 sd 0.001  0.634 sd 0.033  0.783 sd 0.003  0.125 sd 0.041  0.200 sd 0.000
gin-rummy              rummy         8/10   0.690 sd 0.000  0.858 sd 0.010  0.022 sd 0.000  0.905 sd 0.011  0.000 sd 0.000
knock-rummy            rummy         9/10   0.687 sd 0.000  0.814 sd 0.009  0.021 sd 0.000  0.764 sd 0.019  0.239 sd 0.020
instant-knock-rummy    rummy         7/10   0.355 sd 0.002  0.915 sd 0.014  0.000 sd 0.000  0.056 sd 0.035  1.000 sd 0.000
forced-shedding        shedding      1/10   0.182 sd 0.000  0.712 sd 0.000  0.231 sd 0.000  0.289 sd 0.000  1.000 sd 0.000
```

Tier 1 kills: 17 total -- forced-shedding 9/10 (timeouts, the Task 16 false-reject item), instant-knock 3/10, gin 2/10, mau-mau/oh-hell/knock-rummy 1/10 each. Kill shape matches the Task 2 baseline.

Per-column sanity verdicts (does the metric VARY within every skeleton?):

- **decisions: PASS.** Varies within all three skeletons (shedding 0.182-0.300, trick-taking 0.634-0.777, rummy 0.355-0.690). The audit's structural pins (1.0 for all trick-taking) are gone; whist/hearts/spades cluster within 0.008 of each other (same follow-suit structure) but oh-hell separates, so it is not a constant. Watch item "trick-taking decision density" cleared.
- **arc: PASS with a direction flag.** Varies within all skeletons (shedding 0.712-0.866, trick-taking 0.359-0.634, rummy 0.814-0.915). Gin vs instant-knock DO differ (0.858 vs 0.915) -- but instant-knock is the table's HIGHEST arc: 3-card coin-flip games produce cheap lead changes + clean resolution. Separation of that pair must come from decisions/interact/skill/length (all of which point the right way); a Task 14 scale/weight concern, not a Task 10 constant.
- **interact: PASS for shedding/trick-taking; FLAG rummy as an effective skeleton constant.** Shedding varies 0.231-0.564. Trick-taking is off the old 2/N closed form and varies 0.783-0.844, though compressed and sitting at ratio ~0.39-0.42 against the 0.5-ratio clamp denominator -- near-saturated, not clamped; Task 14's denominator recalibration should widen it. Rummy: gin 0.022, knock 0.021 (classic spread 0.001, sd 0.000), instant-knock 0.000 -- the discard-OptionDelta signal barely fires; column is floor-pinned for rummy and gives Task 14 almost no within-skeleton gradient. Revisit under Task 11/14 if rummy separation needs it.
- **skill: PASS.** Strong variation within every skeleton (shedding 0.030-0.289, trick-taking 0.091-0.484, rummy 0.056-0.905). Gin 0.905 vs instant-knock 0.056 is the single strongest classic-vs-degenerate signal in the table.
- **length: PASS with two flags.** Varies within all skeletons (shedding 0.728-1.000, trick-taking 0.200-0.800, rummy 0.000-1.000). Flags: (1) whist/hearts/spades are deterministically 0.800 (fixed 13 decisions/player by deal -- structural, expected, not a bug); (2) gin-rummy hard-zeros (decisions/player > 100 cap) while instant-knock banks a free 1.000 -- the 15-40 band recalibration is squarely Task 14's first item.

Throughput: 33,700 games in 4.245s = **7,939 games/sec single-threaded**, vs the pre-Task-7 baseline ~5,400 games/sec/core-equivalent. No regression (gate: >3x drop, i.e. <1,800/sec); the baseline figure included evolution-engine overhead so the comparison is conservative in both directions, but the instrumentation hot path is clearly fine.

Verdict: no column is a within-skeleton constant for the metrics' own tasks; the two flagged compressions (rummy interaction floor, trick-taking interaction near-saturation) and the two ordering inversions (instant-knock arc + length) are Task 14 scale/weight work, not Phase 2 rework. Proceed to Task 14.

## Phase 3 complete: THE CALIBRATION GATE PASSES (2026-06-11)

Wave D.1 review fixes (68a9ccf, f0d3e93, 941f5e5, 22872f4) + Task 16 reordered first (bf2a513: Tier 1 10 games, completed-games avg-turns basis at cutoff 5 -- classics 29-30/30, instant-knock killed 23/30) + Task 14 (233174c: session band [4,10,60,170] from measured classic DPP, weights unchanged, two-view gate semantics per exit condition (b)) + Task 15 (db83815: floor 0.70 -> 0.42 derived).

Final survivor-conditioned means (n=10/10 for all classics): crazy-eights 0.475, mau-mau 0.490, whist 0.633, hearts 0.650, spades 0.652, oh-hell 0.549, gin 0.584, knock 0.609; degenerates instant-knock 0.434 (n=1/10, pipeline-effective 0.043), forced-shedding 0.428 (n=4/10, effective 0.171). Every classic beats every degenerate on both views; gin beats instant-knock by 0.150. The audit's falsification finding is inverted; the suite now runs untagged as a permanent regression gate.

Remaining for Task 18 besides archive semantics: drop FitnessFloor from MAP-Elites archive admission; remove winner's-curse Valid skip in novelty.go (carried finding a).

## Wave F (Tasks 17, 18, 19, 20, 21, 22, 24, 26): done 2026-06-11

Commits: a613afc (Task 17 descriptor on decision-density x interaction), 3cdecf9 + 5bac854 (Task 18 archive semantics: absolute admission threshold + adaptive control, uniform-random eviction, MAP-Elites admission floor-free with floor moved to output, novelty winner's-curse Valid skip removed), f8410c2 + 489466e + ae0f264 (Task 19: Clone + Move.Key, determinization, determinized ISMCTS player), 4423b0a (Task 20 two-tier skill gradient), 68ff418 (Task 21 mcts playtest difficulty), 619b33d (Task 22 multi-round shedding, run strictly serial per the plan; calibration re-baseline clean -- classics carry no scoring borrows and none moved > 0.02), 418b901 (Task 24 playtest hooks + ratings capture via mechanic.HooksFor single site), cd0e895 (Task 26 borrow tests derived from validBorrows).

**THE MCTS BUDGET FAILED -> TOP-DECILE IS THE PRODUCTION MODE.** Task 19's hard constraint (Tier 2 incl. 20 MCTS games <= 2s/genome single-threaded) failed by ~7x: ~14.5s measured at production strength (200 iterations / 10 determinizations), >95% of it in rummy move generation. Per the plan's pre-registered fallback, the default pipeline (fitness.Evaluate) runs NO MCTS batch and the production mode is MCTS-for-top-decile; EvaluateWithMCTS carries the second tier for selected genomes only.

**FULL-MCTS CALIBRATION MEASUREMENT: TRIED AND REJECTED, ON THE RECORD** (full numbers in the POST-TASK-20 block of pkg/fitness/calibration_test.go). Running the calibration gate through EvaluateWithMCTS for all 10 genomes fails unfixably: no skillScale satisfies both gate criteria (strict survivor ordering needs scale > 1.44, the gin margin needs < 0.07), because ISMCTS discovers REAL knock-timing skill in the instant-knock fixture (seat-0 rates on its one surviving seed 44: random 0.473, greedy 0.506, MCTS 0.933 at production strength) while greedy outplays ISMCTS at both rummy classics (MCTS terms 0). The gate therefore measures the DEFAULT (greedy-only) pipeline -- exactly the ranking that decides who receives MCTS. The mechanism is pinned by TestMCTSTierRewardsDegenKnockTiming.

## Wave F.1 (code-review follow-ups): done 2026-06-12

Commits: 047583b (Task 20b -- the Important finding: EvaluateWithMCTS had NO production caller; MCTS-for-top-decile wired into engine.go AND novelty.go via the two-accumulator design -- FitnessSum/EvalCount stay the pure greedy-only running mean and decile ranking key, MctsSum/MctsCount accumulate the granted EvaluateWithMCTS results at seed offset +5000, published TotalFitness = MCTS mean once MctsCount > 0; -mcts-decile flag default 0.10, threaded through evolve and experiment; resets wherever EvalCount resets. Cost: TestSmallEvolution 1.55s -> 2.31s with the decile enabled at reduced search knobs), ebc2921 (reviewer finding 3 -- MAP-Elites winner's curse: challengers to an occupied cell trigger one fresh-seed incumbent re-evaluation into an EvalCount/FitnessSum running mean; comparison uses running means on both sides; QDScore tracks published means; bounded cost, collisions only), 63b72a4 (findings 2/4/6/7/8: skill-gradient comment corrected to unpaired-batches truth with the paired-seeding future win recorded; skillScale arithmetic made self-consistent (0.905 = cross-seed mean, seed-44 rates give 0.893); descriptor batches now run hooks via the single-site evolution.BehaviorBatch in novelty/MAP-Elites/experiment; batch.go round-boundary probe comment; RoundsPerGame mutation gated on HasScoringBorrow per the coherent-mutation principle; multi-round rulebook states the full score -> fewest-cards -> earliest-seat tiebreak chain; bonus: .gitignore output/ anchored to /output/ -- the unanchored pattern blocked git add on the tracked pkg/output package).

**Carried-findings status:**
- (a) CLOSED: novelty winner's-curse skip removed in 5bac854; MAP-Elites best-ever cells fixed by challenge re-evaluation in ebc2921 (cells now hold running means that lucky evals cannot defend).
- (b) STILL OPEN, carried to Task 29 docs: Individual.Fitness COMPONENT metrics are last-eval values while TotalFitness is a running mean (and, for decile-granted genomes, an MCTS-mode mean over different batches than the components) -- champion metric breakdowns will NOT reconcile with the 0.25/0.25/0.20/0.20/0.10 weights. Document prominently when republishing.
- (c) CLOSED: hook effects are now visible to evolution on both fronts -- Task 22 (619b33d) made shedding scoring borrows outcome-affecting with the evolvability sub-check confirming fitness-visibility, and 63b72a4 made every descriptor batch run hooks. Residual (accepted, documented in batch.go): a single move's TurnRecord is captured before that move's own hooks run; hook effects land in all subsequent records.

**CARRIED HAZARDS for Task 28 designer review:**
1. Knock-timing champions: max(0, mctsWR - greedyWR) cannot tell depth-in-a-rich-game from greedy-incompetence-in-a-trivial-one, and a degenerate with strong non-skill metrics could ride greedy-only fitness into the top decile and then collect a large MCTS term. The decile gate (enforced in evaluateTopDecileMCTS since 047583b) is the mitigation, not a cure. Review every rummy champion's knock threshold/meld reachability as a designer; the failed-review loop encodes rejects as fixtures.
2. Hook-blind MCTS internal model on multi-round shedding genomes: MCTSAI's search model does not run borrowed-mechanic hooks (hooks apply only in the outer batch loop), and multi-round shedding winners are decided by hook-banked Scores -- on those genomes the MCTS tier measures an agent whose internal model is wrong about the win condition. Treat MCTS-term-driven multi-round shedding champions with suspicion; if one reaches the top 10, the fix is hook-aware search, not publication.
3. meta.json (Task 28) must record the MCTS mode: `mcts_mode: top-decile`, the -mcts-decile value, and the search knobs.

## Wave G, Task 27 (experiment harness -- null control + tested statistics): done 2026-06-12

Commit d930dae. The harness's aggregation math backed published comparisons with zero tests; now every aggregation function has hand-computed fixtures (cmd/darwindeck/experiment_test.go).

- **`random` null config:** pure random genome sampling -- mutate-from-seeds with NO selection, every wave drawn fresh from the seed pool -- at the engines' exact budget ((Generations+1) x PopulationSize evaluations, same BaseSeed + wave*10000 + idx derivation), feeding a best-seen per-(skeleton x grid-cell) archive scored by the same coverage/QD/median/pairwise metrics. Reporting bias is documented and conservative: best-seen single-eval maxima are luck-inflated relative to the engines' running means, making the null HARDER to beat. Ignores -mcts-decile (like MAP-Elites; documented in runRandomSearch).
- **Statistics fixes under test:** median() returned the upper-middle value for even n -- now the standard midpoint mean; iqr's index-based quartile convention documented (exact Tukey quartiles at the default n=15); computeMetrics' MedianFit routed through median(); unknown -configs names used to fall through the switch and silently report all-zero metrics -- now rejected up front (aliases mapelites/hybrid accepted, matching the Task 28 command in the plan).
- **Mann-Whitney U:** two-sided, normal approximation with tie + continuity correction (scipy's 'asymptotic' method; fixtures cross-checked against scipy to 1e-9), documented as adequate at n >= 8 per side, plus rank-biserial effect size. Wired into the report for every pairwise config comparison on coverage and QD-score, printing n per side and the small-n caveat (effect-size indications, not strong claims); persisted with per-config aggregates to summary_stats.json.
- **Verified (Wave F.1 claim):** experiment.go threads -mcts-decile into evolution.Config (experiment.go:145). Per-run wall time now recorded (duration_sec) and per-config totals/medians printed.

## Wave H (Task 28 step 4, failed-review loop ROUND 2 of the Task 14 procedure): done 2026-06-12

The post-fix flagship (output/2026-06-12-flagship-postfix) was designer-reviewed; publication HARD-BLOCKED -- the entire top 30 collapsed to three gamed archetypes. Per the failed-review loop, each became a permanent fixture and the metric stack was re-falsified and re-calibrated. Six commits:

1. **Fixtures + red on record** (36d3b79): CatchAllSkipShedding (A1, ranks 1-10: catch-all skip special matches EVERY card + 3 suits wild; in 2p skip == play-again, the opponent spectates), NoFollowAvoidanceTrick (A2, ranks 11-20: no-follow + flat avoidance + winner-leads; off-suit never wins, follower always ducks), PairMeldKnockRummy (A3, ranks 21-29: min_meld_size 2 knock race over a ~1-card stock). Pre-fix gate measurement (the falsification): A1 0.879, A2 0.854, A3 0.673 survivor means, ALL n=10/10, vs worst classic 0.474 -- two fixtures above every classic.
2. **Interaction fix** (8a042bb): 2p skip/reverse are self-tempo, not attacks (IsAttackEvent now takes the player count; draw penalties stay attacks at any count), and self-perturbation never counts as OptionDelta coupling (the rummy rule, now applied to shedding). A1 interaction 1.00 -> 0.632.
3. **Choice-impact decisions** (75778a9): TurnRecord.Meaningful -- a turn with >= 2 legal moves counts only if up to 4 deterministically sampled moves differ in (type, special-effect profile, next-player option-SET hash probe). A2 density 0.917 -> 0.000; A1 0.874 -> 0.633 (the remainder is honest inflict-vs-plain profile mixing); whist 0.776 -> 0.201 (leads stay meaningful, follows/completions collapse); rummy keeps count semantics (a count/set probe cannot capture hidden-information discard value and would collapse gin's core decision as hard as a degenerate's). Throughput gate: shedding bench 1.24x, rummy unchanged (gate 3x).
4. **Recalibration, Task 14 round 2** (this commit): see below.
5. (pending) Output pipeline dedup + fitness-field fixes.
6. (pending) lead_restriction inert-param resolution.

**Round-2 recalibration decisions (commit 4):**

- ONE scale-constant change: session band low edge 10 -> 6 (ramp 3..6). Justification: oh-hell measures 7.0 decisions/player -- a human-validated classic paying a 0.5 length penalty for its natural deal size, which left it 0.003 BELOW instant-knock's single-surviving-seed mean after the decisions fix. Weights unchanged (0.25/0.25/0.20/0.20/0.10); interaction denominator 0.5 unchanged (classic interactive-turn ratios top out at 0.42 -- no saturation, and the A2 ratio pin is structural 1/N, not a denominator problem); skillScale 0.5 unchanged (the gin-margin derivation still holds).
- **EXIT CONDITION (a) TAKEN** (plan Task 14 step 2b, anticipated by the round-2 brief): after the interaction+decisions fixes the three rejected champions still measured 0.625-0.759 vs classics 0.428-0.578; A1 in particular Pareto-dominates several classics on the five metrics (real greedy gradient 0.709, frequent draw-two attacks 0.632, arc 0.874, in-band length), and no monotone scale change separates a dominating pair. The added measurement is the **Tier 2 degeneracy veto** (pkg/fitness/degeneracy.go) -- a validity rule on the existing 200-game random batch, NOT a sixth weighted term (the weight vector stays frozen): non_agentic (meaningful density < 0.05; A2 = 0.000, classic min 0.171), tempo_monopoly (mean consecutive same-player run > 6; A1 = 15.4 -- the designer's literal rejection note "13 consecutive plays, opponent acted 0 times" -- classic max 3.04, rummy's structural turn cycle), draw_supply_churn (rummy-only, OptionDelta share > 0.10; A3 = 0.292 from its starved 1-card stock, gin/knock 0.010). Every threshold has >= 2x measured margin to every classic. Vetoed genomes skip the greedy batch and read fitness 0 in the pipeline, exactly like Tier 1 kills.
- FitnessFloor re-derived 0.42 -> 0.40 (worst classic crazy-eights 0.451 - 0.05, the Task 15 rule).

**Round-2 calibrate table** (./bin/darwindeck calibrate, raw metric means over the 10 pinned CalibrationSeeds; weighted survivor means in the rightmost notes):

```
genome                 skeleton      tier1  decisions       arc             interact        skill           length
---------------------------------------------------------------------------------------------------------------------------
crazy-eights           shedding      10/10  0.211 sd 0.003  0.866 sd 0.015  0.385 sd 0.003  0.026 sd 0.026  1.000 sd 0.000   -> 0.451
mau-mau                shedding      10/10  0.237 sd 0.002  0.797 sd 0.019  0.565 sd 0.004  0.024 sd 0.020  1.000 sd 0.000   -> 0.476
whist                  trick_taking  10/10  0.201 sd 0.001  0.609 sd 0.007  0.844 sd 0.002  0.073 sd 0.016  1.000 sd 0.000   -> 0.486
hearts                 trick_taking  10/10  0.200 sd 0.001  0.359 sd 0.009  0.844 sd 0.001  0.387 sd 0.014  1.000 sd 0.000   -> 0.486
spades                 trick_taking  10/10  0.188 sd 0.001  0.631 sd 0.021  0.835 sd 0.001  0.141 sd 0.018  1.000 sd 0.000   -> 0.500
oh-hell                trick_taking  10/10  0.172 sd 0.001  0.630 sd 0.034  0.783 sd 0.003  0.105 sd 0.034  1.000 sd 0.000   -> 0.478
gin-rummy              rummy         10/10  0.690 sd 0.000  0.859 sd 0.010  0.022 sd 0.000  0.725 sd 0.009  0.114 sd 0.022   -> 0.548
knock-rummy            rummy         10/10  0.687 sd 0.000  0.813 sd 0.009  0.021 sd 0.000  0.611 sd 0.014  0.765 sd 0.011   -> 0.578
instant-knock-rummy    rummy         1/10   0.356           0.930           0.000           0.049           1.000            -> 0.431 (eff 0.043)
forced-shedding        shedding      4/10   0.112           0.706           0.231           0.242           1.000            -> 0.399 (eff 0.160)
catch-all-skip-shedding shedding     0/10   vetoed 10/10: tempo_monopoly                                                     -> 0 (eff 0)
no-follow-avoidance-trick trick_takg 0/10   vetoed 10/10: non_agentic                                                        -> 0 (eff 0)
pair-meld-knock-rummy  rummy         0/10   vetoed 10/10: draw_supply_churn                                                  -> 0 (eff 0)
```

Gate (both views): survivor-strict worst classic crazy-eights 0.451 > best degenerate instant-knock 0.431 (+0.020); pipeline-effective worst classic 0.451 vs best degenerate 0.160 (+0.29 >= 0.05); gin 0.548 > instant-knock 0.431 + 0.10. All four calibration tests green, untagged, in the default suite.

Throughput: 41,300 games in 10.3s = 3,991 games/sec single-threaded (Task 13.5 measured 7,939; ratio 1.99x, under the 3x regression gate -- the cost is the choice-impact probes, measured 1.24x on the shedding bench, plus 30 extra fixture evaluations in the report).

**Round-2 hazards carried to Task 28 step 5 (the re-run):**
- The veto thresholds are specimen-derived (2-29x margins, but from three specimens). Round 3 of 3 remains: if the next flagship's champions are veto-adjacent cousins (e.g. 5p hand-9 pair-meld rummy at churn 0.08, or mean-run 5.5 shedding), encode them and tighten ONLY from new measured tables.
- ~~Vetoed genomes still occupy descriptor space~~ CORRECTED by the Wave H review: all four BehaviorBatch call sites run only after a result.Valid check (novelty.go:183-202, mapelites.go:169-175, experiment.go:187-204 and :305-308), so vetoed genomes never get descriptors or archive entries. No round-3 work needed on this path.
- VETO BLIND SPOT (Wave H review finding 1, round-3 watch item): all three degeneracy detectors run on the Tier 2 RANDOM batch only. A genome healthy under random play but degenerate under skilled play (e.g. a tempo monopoly only greedy/MCTS discovers) escapes every detector; designer review is the only backstop. This is the obvious shape for a round-3 rejected champion.
- The decisions metric's classic band moved from [0.30, 0.78] to [0.17, 0.69]; anything downstream that hardcoded the old spread (descriptor grids are unit-square, so they are fine) should be re-checked at republish time.

**Task 28 budgeting baseline (this 28-core machine):** one run, one config (baseline), one seed, harness defaults pop=500 / gens=100, -parallel 1, -mcts-decile 0: **10m58s wall**, 10,955s CPU (1663% = ~59% of 28 cores), max RSS 422MB; produced coverage 0.07, QD 13.3, 466 qualified games. Projection for the 4-config x 15-seed matrix at -mcts-decile 0: ~657k CPU-seconds ~= 7-11h wall (random/map-elites/novelty add a 50-game behavior batch per valid eval, ~+12% games over baseline). WARNING: at the default -mcts-decile 0.10, baseline/novelty runs additionally grant ~50 EvaluateWithMCTS per generation x ~14.5s CPU each ~= 73k CPU-seconds per run -- a ~7x multiplier that puts the default-mode matrix in multi-day territory on this box. Either budget for that, restrict the matrix to -mcts-decile 0 and record `mcts_mode: greedy-only` in meta.json (hazard 3), or run the decile mode on bigger iron.

## Round 3 (Task 28 step 4, failed-review loop ROUND 3 of 3): 2026-06-12

The r2 flagship (output/2026-06-12-flagship-r2) was designer-reviewed; publication HARD-BLOCKED. Findings: shedding ranks 1-10 rode a catch-all WILD ({type:4}, ByRank=0/BySuit=0) that deletes match_rule/draw_penalty as dead genes (density 0.86-0.98 from inflict-vs-plain profile mixing, greedy skill 0.00; rank01 cycling to the 390-turn cap under greedy -- invisible to random Tier 1); rank03 locked 2 of 4 seats out via adjacent-pair reverse ping-pong (same-player runs ~1, invisible to tempo_monopoly); rummy ranks 21-30 were the PREDICTED pair-meld count-density archetype (rank22 churn parked at 0.088, just under the 0.10 cliff); publication bugs (report-vs-genome fitness divergence, no meta.json, MCTS-mean inflation over component sums, dead rulebook text). Six commits:

1. **Tier-0 catch-all liveness** (76945ad): genome.Validate rejects ANY special with ByRank=0 AND BySuit=0; addSpecialCard forces a suit qualifier when dropping the rank. Calibration restructure: Tier-0-rejected fixtures cannot be metric ground truth -- seeds.CatchAllChampions (round-1 A1 + new CatchAllWildShedding, the byte-faithful r2 rank01 encoding) are negative Tier-0 specimens (TestTier0RejectsCatchAllChampions); seeds.RejectedChampions keeps only statically valid Tier-2 fixtures.
2. **Greedy-batch vetoes** (ac820db): the round-2 blind spot (all detectors random-batch-only) closed. tempo_monopoly + NEW seat_participation (mean min-seat turn share < 0.5/N) run on BOTH batches; NEW greedy_timeout (share > 0.10). Thresholds from the measured classics (calibrate now prints the per-genome veto-statistics table): classic margins -- meanrun max 3.064 (gin, structural), minseat min 0.89x fair (mau-mau), g_timeout max 0.014 (crazy-eights). Measured strengthening: instant-knock (seed 44, 0.110) and forced-shedding (4 seeds, mean 0.141) lose their last survivors to greedy_timeout -- greedy play fails to terminate >10% of their games. Both originals now 0/10.
3. **Rummy deadwood-consequence density** (65b4819): the count exception is dead. rummy.Runner implements sim.ChoiceConsequenceProber: draw live iff the KNOWN top beats the sampled deck best lexicographically on (meld gain, proto-meld gain); meld phase NEVER meaningful (consequence-free on hands; knock timing is the skill metric's domain); discard live iff sampled deadwood-deltas differ; END-AT-WILL voiding (knockable hands' draw/discard are subordinate to the knock decision). gin 0.690 -> 0.369, knock 0.687 -> 0.377, pair-meld archetype 0.869 -> 0.276 (TestDecisionDensityRummyDeadwoodConsequence pins gin >= pair + 0.05 and gin >= 0.30). Throughput: rummy bench 2.3 -> 3.7ms (1.6x, gate 3x) after a no-DP participation prefilter and an O(hand) proto-meld surrogate (the MinMeldSize-1 DP was 56% of the batch profile).
4. **Round-3 fixtures + recalibration** (this commit): ReverseLockoutShedding (rank03) and HeartEngineShedding (rank04) encode the catch-all wild as the FOUR-SUIT-WILD UNION -- semantically identical, statically valid, proving the dynamic stack catches the class when the static rule is bypassed; PairMeldStockRummy (rank22). RED on record: 0.775 / 0.787 / 0.484 survivor means vs worst classic 0.451. Killed by (a) NEW dead_match_rule -- the dynamic twin of commit 1's static rule: share of whole-hand-playable shedding records (LegalMoves >= HandSize, HandSize >= 2 -- TurnRecord gained HandSize) > 0.70; wild-union fixtures 1.000 flat vs classic max 0.033 (21x margin below, 1.43x above) -- and (b) the churn tightening 0.10 -> 0.05 that the round-2 hazard note pre-sanctioned for exactly this cousin (classics 0.011 = 4.5x below; rank22 0.088 = 1.8x above; A3 0.293). NO metric scale/weight changed this round; the one moved constant is the churn veto threshold.
5. **Publication reconciliation** (ddf33b7): (a) the report-vs-genome fitness divergence root-caused -- novelty ARCHIVE entries shared live genome pointers, so later elite re-evaluations overwrote the archived genome's Fitness under frozen metrics (rank04: 0.847 vs 0.808); admission now snapshots (clones) the genome, AND SaveResults stamps a clone with the published TotalFitness/SharedFitness and renders genome.json + report.md + rulebook.md from that single source. (b) meta.json per the Task 4 convention: commit_sha + dirty bit (VCS build stamp; "unknown" when unstamped), go_version, platform, cli_args, master_seed, calibration_seeds, date, mcts_mode/decile/knobs (hazard 3), veto_thresholds (fitness.DegeneracyThresholds), fitness_floor, pop/gens/workers. (c) report.md fitness-provenance section: published mean, greedy-only mean, explicit MCTS uplift (+0.177 silent on r2 skill-0 champions), and the carried-finding-(b) running-mean-vs-components caveat. Selection semantics untouched.
6. **Rulebook truth** (this commit): (a) the card-point table renders only with a live consumer (TT card_points/avoidance scoring, or a live MechAvoidance borrow) -- r2 printed dead tables under ScorePerTrick and on borrow-less rummy; (b) scoring borrows are advertised (rulebook AND report quick-take/insights) only when live -- single-round shedding scoring borrows are inert (rank05); mutation coupling: addBorrowedMechanic forces RoundsPerGame >= 2 when adding a scoring borrow to shedding (+ CardPoints for avoidance), tweakParameter's rounds floor is 2 (the branch requires the borrow), and repairCrossoverInvariants restores both after independent coin flips; (c) TestRulebookNoDeadRuleTextAcrossAllFixtures renders every seed + every fixture rulebook with genome-derived liveness assertions.

**Round-3 calibrate table** (./bin/darwindeck calibrate, raw metric means over the 10 pinned CalibrationSeeds; weighted survivor means in the notes):

```
genome                 skeleton      tier1  decisions       arc             interact        skill           length
---------------------------------------------------------------------------------------------------------------------------
crazy-eights           shedding      10/10  0.211 sd 0.003  0.866 sd 0.015  0.385 sd 0.003  0.026 sd 0.026  1.000 sd 0.000   -> 0.451
mau-mau                shedding      10/10  0.237 sd 0.002  0.797 sd 0.019  0.565 sd 0.004  0.024 sd 0.020  1.000 sd 0.000   -> 0.476
whist                  trick_taking  10/10  0.201 sd 0.001  0.609 sd 0.007  0.844 sd 0.002  0.073 sd 0.016  1.000 sd 0.000   -> 0.486
hearts                 trick_taking  10/10  0.200 sd 0.001  0.359 sd 0.009  0.844 sd 0.001  0.387 sd 0.014  1.000 sd 0.000   -> 0.486
spades                 trick_taking  10/10  0.188 sd 0.001  0.631 sd 0.021  0.835 sd 0.001  0.141 sd 0.018  1.000 sd 0.000   -> 0.500
oh-hell                trick_taking  10/10  0.172 sd 0.001  0.630 sd 0.034  0.783 sd 0.003  0.105 sd 0.034  1.000 sd 0.000   -> 0.478
gin-rummy              rummy         10/10  0.369 sd 0.000  0.859 sd 0.010  0.022 sd 0.000  0.725 sd 0.009  0.114 sd 0.022   -> 0.468
knock-rummy            rummy         10/10  0.377 sd 0.000  0.813 sd 0.009  0.021 sd 0.000  0.611 sd 0.014  0.765 sd 0.011   -> 0.500
instant-knock-rummy    rummy         0/10   killed: 9x tier-1 too-quick, 1x greedy_timeout                                   -> 0 (eff 0)
forced-shedding        shedding      0/10   killed: 6x tier-1 timeouts, 4x greedy_timeout                                    -> 0 (eff 0)
no-follow-avoidance-trick trick_takg 0/10   vetoed 10/10: non_agentic                                                        -> 0 (eff 0)
pair-meld-knock-rummy  rummy         0/10   vetoed 10/10: draw_supply_churn                                                  -> 0 (eff 0)
reverse-lockout-shedding shedding    0/10   vetoed 10/10: dead_match_rule (r_allplay 1.000)                                  -> 0 (eff 0)
heart-engine-shedding  shedding      0/10   vetoed 10/10: dead_match_rule (r_allplay 1.000)                                  -> 0 (eff 0)
pair-meld-stock-rummy  rummy         0/10   vetoed 10/10: draw_supply_churn (0.088 > 0.05)                                   -> 0 (eff 0)
```

Gate (both views): every one of the SEVEN Tier-2 fixtures reads 0 on every calibration seed -- the survivor view is vacuously strict (no degenerate survives to be compared) and stays asserted as a regression tripwire; pipeline-effective worst classic 0.451 vs best degenerate 0.000. Worst classic crazy-eights 0.451 -> FitnessFloor stays 0.40. Weights frozen at 0.25/0.25/0.20/0.20/0.10; no metric scale constant moved this round.

Throughput: 45,500 games in 2.79s = 16,290 games/sec (calibrate process, Wave I game-parallel batches; round-2 measured 3,991/s pre-Wave-I -- no regression, gate >3x drop).

**Round-3 hazards carried forward:**
- dead_match_rule is shedding-only; trick-taking's all-playable analogue (no-follow) is covered by non_agentic, but a future skeleton with a match rule needs its own liveness twin.
- The seat_participation threshold (0.5x fair) did NOT fire on rank03 under random/1-greedy play (measured 0.77-0.78x; the designer-observed lockout manifests under stronger play than the batches contain). rank03 dies via dead_match_rule instead. If a future lockout champion has a LIVE match rule, the detector as-is may miss it; an all-greedy or per-game-distribution variant is the known next step.
- instant-knock's knock-timing MCTS hazard (round-2 hazard 1) is now moot in-pipeline (the fixture never reaches the MCTS tier; TestMCTSTierRewardsDegenKnockTiming pins both the veto and the metric-level mechanism).
- THIN-MARGIN WATCH (round-3 review finding 5): instant-knock's seed-44 death rides greedy_timeout at 0.110 vs the 0.10 threshold (1.1x). Any RNG-affecting change could flip that seed back to surviving, where its survivor mean (0.431) sits only ~3 sd under the binding classic (crazy-eights 0.451). The gate would still pass via the pipeline-effective view, but expect this to resurface as the suite's thinnest margin after any sim-layer change.

- Wave I (sim.RunBatch game-parallelism): done 2026-06-12 -- commit 06f832b. Games within a batch now play on a bounded worker pool (min(BatchGameParallelism=8, GOMAXPROCS, n)) with sequential in-order reduction, BIT-IDENTICAL to the retained serial reference runBatchSerial (permanent golden test: 3 skeletons x 7 genomes incl. hooked borrows + greedy + shared-MCTSAI, 5 seeds, n=20; -race clean across sim/fitness/evolution; hooks audited stateless, guard comments on HooksFor and MCTSAI). Benches on this 28-thread box: Shedding200 12.0ms -> 1.62ms, Rummy20 13.6ms -> 2.10ms, 20-game MCTS grant (new BenchmarkMCTSBatch20) 12.7s -> 2.0s (~6.4-7.4x) -- the per-grant serial pole behind the EPYC's 16% load is gone, and the Task 19 2s MCTS budget now holds, shrinking the Wave H decile-mode (~7x) matrix warning accordingly.

## Round-3 review outcome + HONEST EXIT (Task 28 step 4 verdict; Wave K): 2026-06-12

The round-3 flagship (output/2026-06-12-flagship-r3: pop 2000, gen 200, seed 42, -mcts-decile 0.02, commit 1bd5e5d-dirty) was designer-reviewed -- the THIRD and FINAL round of the failed-review loop. Verdict: **0 publishable / 19 borderline / 11 degenerate.** No champion gamed the stack in the round-1/round-2 sense (the vetoes held); the panel found playable-but-unremarkable games and weak-but-not-vetoed failure shapes, plus three publication-integrity findings:

1. **Incommensurable leaderboard:** the top-N sort mixed MCTS-mode published means (decile grantees) with greedy-only means (everyone else) in one ranking. Ranks 1-10 were exactly the grant set, carrying +0.085..+0.145 uplift -- the MCTS-grant boundary WAS the top-10 boundary.
2. **Functional duplicates:** ranks 1/2/3 were one game differing only in DEAD card_points genes (no consumer in borrow-less single-round shedding); all three rendered identical rulebooks. Byte-level output dedup (Task 28 round 2) cannot see this.
3. **Winner's-curse headline:** every one of the ten published MCTS-mode means rested on a SINGLE two-tier eval; the 0.918 headline reproduced at 0.73-0.82 over fresh seeds.

**HONEST EXIT DECLARED.** The loop budget (3 rounds) is spent and the metric stack is FROZEN -- no metric, veto, weight, or threshold moved in response to this review, and none will without a new plan. The project claims NO publishable champion from the remediated pipeline. The run is preserved as judged in results/2026-06-12-flagship-r3/ (REVIEW.md there records all three rounds' verdicts); README republish (Task 29) happens after the experiment matrix lands and must present the honest exit, not a champion table.

**Wave K (output/reporting fixes only, from the three findings):**

- 9a1600e: gene-liveness predicates (LiveBorrows/LiveCardPoints) hoisted from pkg/output to pkg/genome so dedup and rulebook share one truth.
- 2a699d8 (finding 2): output-ranking dedup hashes the genome's LIVE view -- dead scoring borrows, unread card_points, non-shedding special cards are zeroed before hashing -- at all three output sites (Engine.TopN, Novelty/MAP-Elites AllQualified). POPULATION dedup stays byte-level on purpose (dead genes remain evolutionary material).
- 11e9b26 (findings 1+3): Individual.OutputRank() = the greedy-only running mean is THE leaderboard key everywhere (TopN, sortAndTrim, clone-group keeps); genome.json fitness + report.md headline = OutputRank; summary.json best_fitness = greedy-only best with explicit mcts_best alongside; report provenance always prints n=MctsCount and replaces the uplift line with "insufficient samples (n=N)" below n=3. Selection inside the run still uses publishedFitness -- search policy untouched.
- fc70b22: the results bundle + REVIEW.md.

**Carried round-4 detector candidates** (recorded for a FUTURE plan; the frozen stack does not change):

- Per-turn playable-share statistic (hand fraction playable per turn -- the dead_match_rule generalization beyond shedding).
- Chain-length / decisive-swing statistic on the greedy batch (the round-2/3 lockout and tempo shapes that only skilled play exposes).
- Gameplay-level dedup (behavioral equivalence beyond gene liveness) -- PARTIALLY LANDED via 2a699d8's liveness-aware output dedup; full behavioral dedup (e.g. trace-distribution hashing) remains future work.
- Plus the standing round-3 hazards above (dead_match_rule skeleton coverage, seat_participation under stronger play, the greedy_timeout thin margin).

## Round 4 (Task 28 step 4, AUTHORIZED EXTRA swing past the budgeted 3 rounds): 2026-06-12

The round-3 flagship (output/2026-06-12-flagship-r3) review returned **0 publishable / 19 borderline / 11 degenerate**. Round 4 encodes the three exploits that slipped the frozen round-3 stack and adds three NEW VALIDITY measures. Metric WEIGHTS and ALL weighted-metric scale constants stay FROZEN (0.25/0.25/0.20/0.20/0.10) -- the additions are two vetoes and one Tier-0 rule, all validity rules (the round-1 exit-condition-(a) instrument). Three commits:

1. **FIX 3 -- Tier-0 trivial-meld liveness** (99ad0e4): genome.Validate rejects min_meld_size < 3 for ANY meld type (a 2-card set or 2-run is trivially formable, so melding is consequence-free -- the runs-only-pair-meld champions r3 rank23/rank27 reached deadwood ~0 by turn 7). Parallel to the round-3 catch-all-special rule. mutate clamp 2-4 -> 3-4. Verified against all 10 r3 rummy champions: rejects exactly rank23/rank27; preserves the 8 genuinely-borderline keepers (all MeldBoth min 3) and both classics (gin/knock MeldBoth min 3). Fixture restructure: the three min-2 pair-meld fixtures (RunsOnlyPairMeldRummy = r3 rank23, PairMeldKnockRummy = r2 A3 runs, PairMeldStockRummy = r2 rank22 sets) become seeds.TrivialMeldChampions -- negative Tier-0 specimens (TestTier0RejectsTrivialMeldChampions, spanning both meld types), like CatchAllChampions; they leave RejectedChampions and the draw_supply_churn integration cases.

2. **FIX 1/2 -- playable_share + longest_run vetoes** (b78a356):
   - **playable_share** (shedding, random batch): the round-3 dead_match_rule uses the WHOLE-HAND-playable share (LegalMoves >= HandSize), which at hand 13 stays ~0.16 even when 3 of 4 suits are wild (r3 rank01: 39/52 cards playable on any card -- r_allplay 0.159, never near the 0.70 cliff). The new per-card share is computed DIRECTLY by the shedding runner (new sim.PlayableShareProber -> TurnRecord.PlayableCount), NOT from LegalMoves (GenerateMoves dedups equivalent wild plays via alreadyInMoves, undercounting wild duplicates). Mean per-card playable share over choice-turns (HandSize >= 2) > 0.45 vetoes. Measured (calibrate r_playshare): classics 0.275 (crazy-eights) / 0.299 (mau-mau) / 0.223 (forced-shedding) = 1.51x below; r3 rank01 0.629 = 1.40x above.
   - **longest_run** (greedy batch): meanConsecutiveRun averages ALL runs and misses an EPISODIC monopoly -- a held attack-card chain firing in ONE mega-turn (6-13 consecutive plays, opponent never acts) on a game otherwise alternating (mean run ~1.4). New meanLongestRun = mean over games of each game's per-game MAXIMUM same-player run; > 5.0 vetoes. Measured (calibrate g_longest): gin 4.074 / knock 3.958 (the structural rummy draw-meld-discard cycle, the table's legitimate maximum) = 1.23x below; all other classics 1.0-2.0. NOTE (round-4 review correction): the once-claimed "~5.84 headroom" champion is now Tier-0 rejected (min_meld<3) and never reaches this veto, and the wild-union champion dies to playable_share (its g_longest ~4.84 is under 5.0) -- so longest_run is the unique killer of NO gate fixture. It is a PROSPECTIVE detector for the future episodic-monopoly shape (a held chain firing in one mega-turn on an otherwise-alternating game) that meanConsecutiveRun/dead_match_rule/playable_share all miss; its only current specimen kill is the spared 2-suit judgment fixture (rank04). The 1.23x margin is safe because gin's per-seed mean spans only 4.06-4.09 (the draw-meld-discard cycle pins it <=5). Run on the GREEDY batch because the chains are a skilled-play phenomenon.
   - Both statistics added to DegeneracyStats, DegeneracyThresholds (meta.json), and the calibrate veto table (new r_playshare, g_longest columns -- step 6 satisfied).

3. **Round-4 fixtures + recalibration** (this commit): WildUnionShedding (r3 rank01, 3-suit wild, Tier-0 VALID so a TIER-2 metric fixture) joins RejectedChampions. RED on record (measured before the FIX-1 veto, survivor means over CalibrationSeeds): wild-union-shedding 0.734, heart-engine-2suit 0.733, runs-only-pair-meld 0.529, ALL n=10/10 -- every one above worst classic 0.451; wild-union and the 2-suit cousin above EVERY classic. GREEN after: wild-union vetoed 10/10 by playable_share.

**JUDGMENT FIXTURE (r3 rank04, HeartEngine2SuitShedding -- the review's "one fix from a real game"):** a milder 2-of-4-suit wild shedding game. The playable_share veto is deliberately SPARED on it (per-card share ~0.44 < 0.45 threshold -- the intended judgment call). But longest_run KILLS it (greedy longest-run ~6.5 > 5.0): in 2-player its wild + draw-penalty chains produce the same episodic bursts as the fully degenerate cousins, so it reads 0 on all 10 seeds. **DOCUMENTED LIMITATION:** the longest_run detector cannot distinguish rank04's one-fix-from-real tempo from a degenerate monopoly. It is NOT in RejectedChampions (the gate does not require it to die); TestRound4JudgmentFixtureLanding pins the landing (survived 0, killed-by-longest_run 10).

**Round-4 calibrate table** (./bin/darwindeck calibrate, raw metric means over the 10 pinned CalibrationSeeds; weighted survivor means in the notes; NO metric/scale constant moved -- classic metric columns are identical to round 3):

```
genome                 skeleton      tier1  decisions       arc             interact        skill           length
---------------------------------------------------------------------------------------------------------------------------
crazy-eights           shedding      10/10  0.211 sd 0.003  0.866 sd 0.015  0.385 sd 0.003  0.026 sd 0.026  1.000 sd 0.000   -> 0.451
mau-mau                shedding      10/10  0.237 sd 0.002  0.797 sd 0.019  0.565 sd 0.004  0.024 sd 0.020  1.000 sd 0.000   -> 0.476
whist                  trick_taking  10/10  0.201 sd 0.001  0.609 sd 0.007  0.844 sd 0.002  0.073 sd 0.016  1.000 sd 0.000   -> 0.486
hearts                 trick_taking  10/10  0.200 sd 0.001  0.359 sd 0.009  0.844 sd 0.001  0.387 sd 0.014  1.000 sd 0.000   -> 0.486
spades                 trick_taking  10/10  0.188 sd 0.001  0.631 sd 0.021  0.835 sd 0.001  0.141 sd 0.018  1.000 sd 0.000   -> 0.500
oh-hell                trick_taking  10/10  0.172 sd 0.001  0.630 sd 0.034  0.783 sd 0.003  0.105 sd 0.034  1.000 sd 0.000   -> 0.478
gin-rummy              rummy         10/10  0.369 sd 0.000  0.859 sd 0.010  0.022 sd 0.000  0.725 sd 0.009  0.114 sd 0.022   -> 0.468
knock-rummy            rummy         10/10  0.377 sd 0.000  0.813 sd 0.009  0.021 sd 0.000  0.611 sd 0.014  0.765 sd 0.011   -> 0.501
instant-knock-rummy    rummy         0/10   killed: 9x tier-1 too-quick, 1x greedy_timeout                                   -> 0 (eff 0)
forced-shedding        shedding      0/10   killed: 6x tier-1 timeouts, 4x greedy_timeout                                    -> 0 (eff 0)
no-follow-avoidance-trick trick_takg 0/10   vetoed 10/10: non_agentic                                                        -> 0 (eff 0)
reverse-lockout-shedding shedding    0/10   vetoed 10/10: dead_match_rule (r_allplay 1.000)                                  -> 0 (eff 0)
heart-engine-shedding  shedding      0/10   vetoed 10/10: dead_match_rule (r_allplay 1.000)                                  -> 0 (eff 0)
wild-union-shedding    shedding      0/10   vetoed 10/10: playable_share (r_playshare 0.629 > 0.45, r_allplay only 0.159)    -> 0 (eff 0)
```

Veto-statistic margins (calibrate veto table): r_playshare classic max mau-mau 0.299 (1.51x below the 0.45 threshold); wild-union 0.629 (1.40x above). g_longest classic max gin 4.074 (1.23x below the 5.0 threshold). r_allplay confirms the round-3 dead_match_rule would NOT have caught wild-union (0.159, far under 0.70) -- the new per-card veto is what closes it.

Gate (both views): every one of the SIX Tier-2 fixtures reads 0 on every calibration seed -- survivor view vacuously strict (no degenerate survives), pipeline-effective worst classic 0.451 vs best degenerate 0.000. Worst classic crazy-eights 0.451 -> FitnessFloor stays 0.40 (Task 15 rule; 0.451 - 0.05 = 0.401). gin 0.468 > instant-knock 0 + 0.10. All calibration tests green, untagged, in the default suite. Weights frozen at 0.25/0.25/0.20/0.20/0.10; no metric scale constant moved this round (the three additions are validity rules).

Throughput: 43,400 games in 1.17s = ~37,100 games/sec (calibrate; the new shedding fixtures die at the veto before the 200-greedy batch, so fewer batches run). Round 3 measured 16,290 g/s pre-this-machine-load -- no regression (gate: >3x drop).

**Round-4 hazards carried forward:**
- The longest_run veto cannot distinguish the 2-suit "one-fix-from-real" judgment fixture (rank04) from its degenerate cousins (both ~6.5 greedy longest-run). A future plan wanting to RESCUE rank04-class games needs a richer monopoly signal (e.g. victim-acted-at-all share, or distinguishing self-tempo chains from opponent-locking chains), not a threshold nudge.
- playable_share is shedding-only (it needs the runner's match predicate). A future skeleton with a match rule needs its own per-card-liveness twin, same as the dead_match_rule note.
- The trivial-meld Tier-0 rule subsumes both PairMeld fixtures' dynamic vetoes (draw_supply_churn). draw_supply_churn now has NO live fixture among the trivial-meld cousins -- a future min-3 rummy with a starved stock would be its only remaining target; keep the synthetic unit coverage.

## LOOP CLOSED -- round-4 review verdict + Wave M (2026-06-13)

The round-4 flagship (output/2026-06-12-flagship-r4: pop 2000, gen 200, seed 42, -mcts-decile 0.02, commit ccf6df5-dirty) was designer-reviewed -- the AUTHORIZED EXTRA swing past the budgeted three rounds. **Verdict: 0 publishable.** The top 30 is exactly 10 shedding (wild-union residue parked just under the round-4 vetoes; the original published rank02 actually FAILS its own greedy_longest_run veto on 1/10 seeds, published only because production does a SINGLE eval per genome), 10 trick-taking (genuine multi-round Whist -- a real game but a public-domain REDISCOVERY, not novel), 10 rummy (genuine Gin/Knock after the min_meld>=3 fix -- also rediscoveries). Best honest greedy-only fitness 0.739 (down from round-3's inflated 0.918).

**The four-round arc (the headline methodology result):**

| Round | Run | Verdict | What gamed the stack | Validity rules added (weights/scales FROZEN from round 1) |
|-------|-----|---------|----------------------|-----------------------------------------------------------|
| 1 | flagship-postfix | HARD-BLOCKED | catch-all-skip shedding, no-follow avoidance trick, pair-meld knock rummy | non_agentic, tempo_monopoly, draw_supply_churn; interaction + choice-impact decision-density metric fixes |
| 2 | flagship-r2 | HARD-BLOCKED | catch-all WILD shedding (dead match_rule), reverse-lockout, pair-meld at churn 0.088 under the 0.10 cliff | Tier-0 catch-all liveness; greedy-batch vetoes (seat_participation, greedy_timeout, greedy tempo); rummy deadwood-consequence density; churn 0.10 -> 0.05 |
| 3 | flagship-r3 | 0 publishable / 19 borderline / 11 degenerate | no NEW exploit (vetoes held); only playable-unremarkable games + publication bugs | Wave K output-path fixes: greedy-only leaderboard key, functional output dedup, MCTS-provenance n-floor |
| 4 | flagship-r4 | 0 publishable; 10 wild-union shedding / 10 Whist / 10 Gin-Knock | wild-union shedding (statically valid), trivial-meld rummy, episodic monopoly | Tier-0 trivial-meld liveness (min_meld >= 3); playable_share (per-card) + longest_run (episodic monopoly) vetoes |

Honest fitness ceiling per round as exploit corners closed: **0.97 -> 0.91 -> 0.92-inflated -> 0.739-honest** (the round-3 0.92 was inflated by the incommensurable MCTS/greedy leaderboard + single-eval winner's curse).

**FINAL VERDICT -- HONEST EXIT.** Correct, calibrated, four-times-adversarially-hardened proxy metrics still do not discover novel fun. Evolution either games the newest veto or rediscovers an existing game; the most game-like outputs are faithful Whist/Gin reimplementations. The project claims **no novel publishable game** from the remediated pipeline. This is an honest negative result with a real lesson: automated fun-proxies are exploitable by construction; novel-fun discovery needs a human in the loop or a fundamentally richer signal, not more vetoes. The metric stack is FROZEN -- no metric, veto, weight, or threshold moved in response to the round-4 review, and none will without a new plan.

**Wave M (loop-closure and honest-exit publication, four commits -- output path only, metric stack untouched):**

1. **Veto-stable publication fix** (the bug rank02 exposed): single-eval publication let a genome that fails its own veto on 10% of seeds publish as rank 2. SaveResults now RE-EVALUATES each top-N genome K=5 times at distinct fresh seeds (pkg/output/stability.go); a genome valid on a majority (>= 3/5) is veto_stable and keeps its leaderboard place, the rest are demoted below all stable games. Every published genome.json/report carries veto_stable + stable_evals ("N/5"). Output-path only (Wave K spirit): selection/evolution dynamics/frozen metric stack untouched; input greedy-only leaderboard order preserved within each stability class. Throughput: ~11ms/eval, 30 top-N x 5 = ~1.65s, trivial vs a run. Tests: planted degenerate is unstable + demoted, clean classic keeps rank, determinism under fixed seed, majority boundary (2/5 demoted, 3/5 stable).

2. **Re-published flagship-r4 through the fixed path**: new `darwindeck restamp` subcommand (cmd/darwindeck/restamp.go) loads a saved run's games/*/genome.json, runs the K=5 stability check + a fresh greedy-only eval on each, re-ranks stable-first, and writes results/2026-06-12-flagship-r4/ (summary.json, meta.json copied+annotated, 30x {genome.json, rulebook.md, report.md}, STABILITY.md, REVIEW.md). The original rank02 (gen200_55718) demotes to rank 29 with honest fitness 0 -- its single fresh published eval lands on its failing greedy_longest_run seed while K=5 reads 4/5 -- the exact single-eval/multi-eval divergence the fix exposes. REVIEW.md records the full four-round arc + honest exit.

3. **README honest-exit republish** (Task 29 narrative): the true story -- playable games + correct ranking of Whist/Gin rediscoveries, NO novel fun across 4 rounds; the failed-review-loop arc as the headline; the lesson; the current fitness/validation sections (5 rebuilt metrics, Tier-0 liveness, degeneracy veto stack, calibration gate as permanent regression test, two-tier greedy+top-decile-MCTS skill, veto-stable publication); a clearly-marked PLACEHOLDER for the algorithm-comparison table; stale pre-fix numbers removed.

4. **This checkpoint finale + plan closure**.

**Tasks 28/29 status:**
- **Task 28 (full re-runs on fixed code): COMPLETE via the failed-review loop -> honest exit.** The flagship was re-run on remediated code four times (postfix, r2, r3, r4); step 3 designer review and step 4 failed-review loop ran to their full budget + one authorized extra; the loop's verdict is 0 publishable. The reproducible flagship bundle is results/2026-06-12-flagship-r4/ (re-published through the veto-stable path); meta.json records mcts_mode/decile/knobs, veto thresholds, fitness floor, pop/gens. NOT done: the 4-config x 15-seed experiment matrix (step 2), running on a separate machine; its results.json lands later.
- **Task 29 (republish + close the loop): DONE except the matrix table.** README/checkpoint tell the honest exit; the algorithm-comparison table is a marked placeholder for the experiment matrix. CLAUDE.md/ROADMAP truth-pass for the honest exit can fold into the matrix-landing commit.

**What remains (carried-forward research directions, for a FUTURE plan -- the frozen stack does not change):**
- The **experiment-matrix table** (Task 29 step 2): fill the README placeholder from the 4-config x 15-seed matrix's results.json when it lands.
- **Optional NSGA-II (Task 30)**: Pareto selection vs weighted sum over the 5 raw metrics; pre-registered criterion in the plan.
- **Human-in-the-loop fitness**: the only signal that can see fun the simulation cannot (the rank04-class "one fix from a real game" judgment no threshold can make). Task 24's playtest ratings instrument exists but is unvalidated (no N >= 10 rated sessions yet).
- **Richer victim-acted / decision-impact signals**: distinguish self-tempo chains from opponent-locking chains; a "victim acted at all" share to rescue the rank04-class games longest_run cannot tell from degenerate monopolies.
- **Novelty-vs-existing-games detection**: the system has no way to know its best trick-taking output IS Whist; a rediscovery detector turns these false positives into an explicit "rediscovered a classic" label.
- Standing round-4 hazards (playable_share is shedding-only; draw_supply_churn has no live fixture after the trivial-meld Tier-0 rule -- keep synthetic coverage; the greedy_timeout thin margin).

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

- Wave A (Tasks 1, 4, 5-safe, 6, 23, 25): done 2026-06-11 -- commits 8eda14b, afa283e, 4127df5, 1ce1b79, 631e198, 4f4854b. Epub history rewrite still gated on user approval (0 forks, no open PRs).
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

# DarwinDeck Audit Remediation Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use conclave:executing-plans to implement this plan task-by-task.

**Goal:** Fix every verified finding from the 2026-06-11 deep-dive audit so that DarwinDeck's fitness function is falsifiable against human ground truth, its published results are reproducible on current code, and its showcased games are real.

**Architecture:** Eight phases ordered by dependency: (0) ground truth + hygiene, (1) sim-layer instrumentation, (2) metric reimplementation, (3) calibration gate, (4) QD layer, (5) MCTS skill instrumentation, (6) borrows/playtest parity/prevention, (7) re-run + republish. The keystone is the seed-calibration suite (Task 2): the 8 classic seed games are the only human-validated "fun" ground truth in the repo, so every metric change is judged by whether classics outrank known-degenerate fixtures. Until Phase 3 completes, the calibration tests live behind a `calibration` build tag so CI stays green.

**Tech Stack:** Pure Go (v2 tree: `pkg/`, `cmd/`). No new dependencies. `git filter-repo` for the one history-rewrite task.

**Key audit findings this plan remediates (verified 2026-06-11):**
- 3 of 5 fitness metrics are skeleton-identity constants (sd 0.000-0.006); MeaningfulDecisions is structurally 1.0 for trick-taking; GameArc measures seat balance, not arc; Interaction counts every discard as interactive.
- Fitness function rejects its own ground truth: whist 0.668, hearts 0.684, spades 0.671, oh-hell 0.651 -- all below `FitnessFloor = 0.70` (`pkg/evolution/novelty.go:19`) -- while the flagship champion is an instant-knock coin flip (rummy, hand 3, min meld 4, knock 27) reported at 0.919.
- No MCTS in v2; skill ceiling is 1-ply greedy with hardcoded special ranks; 50 greedy games give SE ~0.07; elites are never re-evaluated (winner's curse).
- QD descriptor degenerate: 91% of mutants land in the top win-entropy row; 16/100 MAP-Elites cells reachable; novelty archive uses per-generation-max-normalized threshold + FIFO eviction; MAP-Elites/novelty/descriptor at 0% test coverage.
- Rummy nondeterministic under fixed seed (Go map iteration); shedding `GenerateMoves` and tricktaking `CheckEnd` mutate state.
- Tier 1 runs without borrowed-mechanic hooks; playtest never runs hooks; both shedding borrows are outcome-inert; MechPlayMultiple/MechTrump have empty hook cases.
- Flagship run and README experiment table predate every fitness fix (Apr 12 run vs Apr 24-Jun 3 fixes); never re-run.
- Copyrighted Hoyle's epub committed and pushed to public GitHub; flagship evidence untracked while 1,237 obsolete v1 output files (~64MB) are committed; stray `darwindeck` ELF binary at repo root.

---

## Phase 0: Ground truth and safety rails

### Task 1: Fix rummy nondeterminism (map iteration)

**Files:**
- Modify: `pkg/skeleton/rummy/runner.go:341,359,449,464` (the four `map[sim.Rank]`/`map[sim.Suit]` iteration sites)
- Test: `pkg/skeleton/rummy/runner_test.go`

**Dependencies:** none

**Step 1: Write the failing test**

```go
// TestMeldMoveOrderDeterministic pins dd-audit-1: meld-move generation must not
// depend on Go map iteration order. Same genome+seed => byte-identical event streams.
func TestMeldMoveOrderDeterministic(t *testing.T) {
	g := seeds.GinRummy() // any rummy genome with melds reachable
	for seed := uint64(1); seed <= 20; seed++ {
		r1 := sim.RunBatch(g, &Runner{}, &sim.RandomAI{}, 1, seed)
		r2 := sim.RunBatch(g, &Runner{}, &sim.RandomAI{}, 1, seed)
		if !reflect.DeepEqual(r1.AllEvents, r2.AllEvents) {
			t.Fatalf("seed %d: event streams differ", seed)
		}
	}
}
```

Note: an existing determinism test passes only because RandomAI masks ordering when move *sets* are equal. This test will flake rather than fail deterministically; run it with `-count=20`. If it does not fail after 20 counts, instrument `generateMeldMoves` to log move order and assert order equality directly instead.

**Step 2: Run to verify it fails**

Run: `go test ./pkg/skeleton/rummy/ -run TestMeldMoveOrderDeterministic -count=20 -v`
Expected: FAIL (order divergence) or proceed with the order-assert variant.

**Step 3: Implement**

At each of the four sites, after building the map, extract keys, sort, and iterate sorted keys:

```go
ranks := make([]sim.Rank, 0, len(byRank))
for r := range byRank {
	ranks = append(ranks, r)
}
slices.Sort(ranks)
for _, r := range ranks {
	cards := byRank[r]
	...
}
```

(Same pattern for `bySuit` with `slices.Sort` on the suit values.)

**Step 4: Run tests**

Run: `go test ./pkg/skeleton/rummy/ -count=20 -v`
Expected: PASS, all existing tests still green.

**Step 5: Commit**

```bash
git add pkg/skeleton/rummy/
git commit -m "fix(rummy): sort map keys in meld generation for per-seed determinism"
```

---

### Task 2: Seed-calibration suite (behind build tag)

**Files:**
- Create: `pkg/seeds/degenerate.go`
- Create: `pkg/fitness/calibration_test.go` (build tag `calibration`)

**Dependencies:** Task 1

This is the falsifiable acceptance test for the whole plan. It is committed now, gated by `//go:build calibration`, and the gate is removed in Task 14 when it must pass.

**Step 1: Create degenerate fixtures**

```go
// Package seeds: known-bad genomes used as negative ground truth for fitness
// calibration. These must always score below every classic seed game.
package seeds

// InstantKnockRummy reproduces the degenerate flagship champion
// (rank01_gen200_70015): hand 3, melds unreachable, knock nearly always legal.
func InstantKnockRummy() *genome.Genome {
	return &genome.Genome{
		Skeleton: genome.SkeletonRummy,
		Players:  2,
		HandSize: 3,
		Rummy: genome.RummyParams{
			MeldTypes:      genome.MeldSets,
			MinMeldSize:    4,
			DrawFrom:       genome.DrawDiscard,
			KnockThreshold: 27,
		},
	}
}

// ForcedShedding: MatchEither with DrawPenalty 1 and no special cards on a
// 2-card hand -- almost every turn has exactly one sensible line.
func ForcedShedding() *genome.Genome { ... } // analogous construction
```

(Adjust field names/required fields against `pkg/genome/genome.go:45-140` and `genome.Validate`; fixtures must pass Tier 0.)

**Step 2: Write the calibration test**

```go
//go:build calibration

package fitness_test

// CalibrationSeeds is the canonical pinned seed list for ALL calibration
// evaluations. Every task that measures seed-game fitness uses this list --
// never ad-hoc seeds -- so numbers are comparable across the whole plan.
var CalibrationSeeds = []uint64{11, 22, 33, 44, 55, 66, 77, 88, 99, 110}

// Ground truth: the 8 classic seeds are the only human-validated "fun" games
// in the repo. Any fitness function that scores a classic below a degenerate
// fixture is falsified. Evaluations are averaged over the pinned seed list
// because per-eval noise is sd ~0.02.
//
// OVERFITTING GUARD: 8 classics is a narrow truth source. Tune to MARGINS
// (worst classic must beat best degenerate by 0.05), never to exact
// orderings among the classics; as more classics are added (Hoyle-derived
// trick/shedding/rummy variants), the suite grows but thresholds do not move.
func TestCalibrationClassicsBeatDegenerates(t *testing.T) {
	classics := seeds.All() // 8 games
	degens := []*genome.Genome{seeds.InstantKnockRummy(), seeds.ForcedShedding()}
	meanFit := func(g *genome.Genome) float64 { /* 10x EvaluateTier2, mean of TotalFitness */ }

	worstClassic, bestDegen := 1.0, 0.0
	for _, c := range classics { worstClassic = min(worstClassic, meanFit(c)) }
	for _, d := range degens   { bestDegen = max(bestDegen, meanFit(d)) }

	if worstClassic < bestDegen+0.05 {
		t.Errorf("calibration failed: worst classic %.3f vs best degenerate %.3f", worstClassic, bestDegen)
	}
}

func TestCalibrationClassicsAboveFloor(t *testing.T) {
	// every classic must clear the QD viability floor (see Task 15)
}

func TestCalibrationGinBeatsInstantKnock(t *testing.T) {
	// gin rummy mean fitness >= instant-knock + 0.10
}
```

**Step 3: Run and record the current (failing) numbers**

Run: `go test -tags calibration ./pkg/fitness/ -run TestCalibration -v`
Expected: FAIL. Record the printed means in the test file as a comment block (baseline as of this commit: whist 0.668, hearts 0.684, spades 0.671, oh-hell 0.651, gin 0.814, instant-knock ~0.807).

**Step 4: Verify untagged suite is unaffected**

Run: `go test ./pkg/...`
Expected: PASS (calibration excluded without the tag).

**Step 5: Commit**

```bash
git add pkg/seeds/degenerate.go pkg/fitness/calibration_test.go
git commit -m "test(fitness): add seed-calibration suite behind calibration tag (currently failing by design)"
```

---

### Task 3: Purify side-effecting queries

**Files:**
- Modify: `pkg/skeleton/shedding/runner.go:84-99` (GenerateMoves recycles discard + advances RNG)
- Modify: `pkg/skeleton/tricktaking/runner.go:303-329` (CheckEnd does Round++ and redeals)
- Modify: `pkg/sim/batch.go` (game loop), `pkg/sim/batch.go:26-31` (GenericRunner interface)
- Test: all three `pkg/skeleton/*/runner_test.go`

**Dependencies:** Task 1

Queries must be pure so Task 7 (counterfactual interaction measurement) and Task 19 (MCTS tree search) can call them repeatedly. Add an explicit upkeep step to the interface:

```go
type GenericRunner interface {
	Setup(g *genome.Genome, rng *rand.Rand) *GameState
	// Upkeep performs start-of-turn state maintenance (deck recycling,
	// round transitions/redeals). It is the ONLY method besides ApplyMove
	// allowed to mutate state.
	Upkeep(state *GameState, g *genome.Genome)
	GenerateMoves(state *GameState, g *genome.Genome) []Move // must be pure
	ApplyMove(state *GameState, move Move, g *genome.Genome) []Event
	CheckEnd(state *GameState, g *genome.Genome) int // must be pure
}
```

**Steps (TDD):**
1. Write `TestGenerateMovesIsPure` per skeleton: hash full state (deck, hands, discard, RNG unused -- pass a sentinel), call GenerateMoves twice, assert hash unchanged and both move slices deep-equal. Write `TestCheckEndIsPure` for tricktaking the same way. Run; shedding and tricktaking FAIL.
2. Move discard-recycling from shedding GenerateMoves and round-transition/redeal from tricktaking CheckEnd into new `Upkeep` methods; rummy and shedding get the same treatment for any reshuffle logic; batch loop calls `runner.Upkeep(state, g)` at the top of each iteration before `CheckEnd`. NOTE: the RNG advance moves with the recycle, so per-seed game traces WILL change -- update any pinned-trace tests, and re-run Task 1's determinism tests.
3. Run full suite + the borrow integration tests (`pkg/fitness/borrowed_hooks_test.go` asserts a 70% completion floor -- it will catch upkeep-ordering mistakes).
4. Commit: `refactor(sim): make GenerateMoves/CheckEnd pure, add explicit Upkeep step`.

---

### Task 4: Repo hygiene -- tracked/untracked inversion

**Files:**
- Modify: `.gitignore`
- Delete from index: `output/**` (1,237 committed v1 files), stray root binary `darwindeck`
- Create: `results/` (tracked, small artifacts only)

**Dependencies:** none

**Steps:**
1. Add to `.gitignore`: `output/`, `/darwindeck`, `bin/`.
2. `git rm -r --cached output/ && git rm --cached darwindeck` (working tree preserved).
3. Create `results/README.md` explaining the convention: every published claim gets its backing artifact committed here (summary.json, results.json, top-N rulebooks -- JSON/MD only, no binaries), and **every results bundle MUST contain a `meta.json`**: `{commit_sha, go_version, platform, cli_args, master_seed, calibration_seeds, date}`. Without this, "reproducible" is hollow. Copy in: `output/2026-04-12_19-43-11/summary.json` + 30 rulebooks/reports, and `output/experiments/{full,large}/results.json`, labeled `pre-fix-flagship/` and `pre-fix-experiments/`, each with a `meta.json` whose fields are filled only where knowable from git history and otherwise `"unknown"`, plus `"non_reproducible": true` -- these are preserved as evidence of the *old* results, explicitly NOT as reproducible artifacts; Phase 7 adds the reproducible ones.
4. `go build ./... && go test ./pkg/...` (nothing depends on committed outputs); commit: `chore(repo): ignore outputs, track result artifacts under results/`.

### Task 5: Remove copyrighted epub from git history

**Files:**
- Delete: `Hoyle's Encyclopedia of Card Game by Walter B. Gibson.epub` (tracked, 5.8MB, on origin/master, public repo, MIT README)

**Dependencies:** Task 4 (do all index surgery first so history is rewritten once)

**GATE: requires explicit user approval before execution -- this rewrites public history and force-pushes.**

**Steps:**
1. Operational pre-check: `gh api repos/signalnine/darwindeck/forks` and `gh pr list` -- if forks or open PRs exist, they retain the old history; note them for follow-up (the file stays in their copies regardless; the goal is to stop *this* repo distributing it). Local clones on other machines need `git fetch && git reset --hard origin/master` -- record that instruction in the commit message.
2. `git rm "Hoyle's Encyclopedia of Card Game by Walter B. Gibson.epub" && git commit -m "chore: remove copyrighted epub"`.
3. With user approval: `git filter-repo --invert-paths --path "Hoyle's Encyclopedia of Card Game by Walter B. Gibson.epub"` (note: filter-repo removes origin remote; re-add it), then `git push --force-with-lease origin master`.
4. Verify: `git log --all --oneline -- "Hoyle*"` empty; `git cat-file -e origin/master:"Hoyle's Encyclopedia of Card Game by Walter B. Gibson.epub"` fails.
5. `docs/hoyles-game-examples.md` stays (original prose referencing the book is fine); add a one-line source citation instead of the file.

### Task 6: Documentation truth pass

**Files:**
- Modify: `CLAUDE.md`, `ROADMAP.md`, `README.md`

**Dependencies:** none

**Steps:**
1. CLAUDE.md: delete or mark-as-historical the v1 performance claims contradicted by code (the "~4x parallel Python speedup" class runs serially; the golden equivalence tests are skipped). Fix the MeaningfulDecisions description ("fraction of turns with >1 legal move" -- false until Task 9 makes it true; reword to match current code now, re-update in Task 9).
2. README.md: add the MAP-Elites row to the results table (it has a reportable outcome: baseline-level coverage, ~5x fewer output games, highest median fitness 0.862); add a dated caveat that all current numbers predate the Apr-Jun fitness fixes and will be regenerated (Phase 7 removes the caveat).
3. ROADMAP.md: record dropped-scope decisions explicitly: MCTS absent from v2 (restored in Phase 5), Pareto/NSGA-II open question, betting/bidding/teams/web UI are v1-only.
4. Commit: `docs: align CLAUDE/README/ROADMAP claims with code reality`.

---

## Phase 1: Sim-layer instrumentation

### Task 7: Per-turn decision and interaction recording

**Files:**
- Modify: `pkg/sim/batch.go` (game loop ~line 125, `GameResult`, `BatchResult`)
- Modify: `pkg/sim/state.go` (no changes to GameState; new types only)
- Test: `pkg/sim/batch_test.go`

**Dependencies:** Task 3 (pure queries)

Add per-turn records, captured in the loop where `moves := runner.GenerateMoves(state, g)` already exists:

```go
// TurnRecord captures per-turn decision data for fitness analysis.
type TurnRecord struct {
	Player      int
	LegalMoves  uint8 // capped at 255
	OptionDelta int8  // change in next player's legal-move count caused by this move
}

type GameResult struct {
	...
	Turns   []TurnRecord
	Leaders []int8 // leader after each turn, -1 = tie (filled by Task 8)
}
```

`OptionDelta` is defined PER SKELETON, explicitly, before any implementation (consensus review flagged a generic definition as brittle across game families). The contract: `OptionDelta = options(next player, post-move state) - options(next player, pre-move reference state)`, where both terms are computable with the now-pure `GenerateMoves` on cloned/explicit states, and the reference state is skeleton-specific:

| Skeleton | options(next, ...) means | pre-move reference | rationale |
|---|---|---|---|
| shedding | legal plays+draw for next player against the discard top | pre-move state with same next player | discard top is the entire coupling surface |
| rummy | legal draws+melds+discards for next player | pre-move state | discard top + table melds are the coupling surface |
| tricktaking | defined for trick-LEADING plays only: `OptionDelta = legalMoves(next, post-lead) - len(hand(next))` (the constraint the lead imposes; <= 0, nonzero only when follow rules bind). Follows and trick-completing plays: 0 | next player's unconstrained hand size | mid-trick counterfactuals are ill-defined (the leader sets follow legality), but the lead's constraining power IS well-defined and genome-linked: MustFollowSuit genomes produce negative deltas, free-play genomes produce 0 -- a real within-skeleton gradient. AMENDED 2026-06-11 after Wave D review found the original always-0 rule made Interaction a closed-form constant (2/N) for trick-taking, recreating the audit's skeleton-constant pathology |
| climbing (added with the skeleton, 2026-06; table row added 2026-07-17) | defined for MovePlay with next != mover: `OptionDelta = legalPLAYS(next, combo on table) - legalPLAYS(next, counterfactual clear table)`. Passes: 0 | counterfactual clear-table free lead for the same next player | how much the played combination constrains the follower vs leading freely; <= 0 by construction since every beat is also a legal lead. AMENDED 2026-07-17: both terms count PLAYS only (`probePlayOptionCount`) -- the original raw `GenerateMoves` count included the follow position's always-legal Pass but not a lead's, a +1 floor bias that produced impossible positive deltas and read "removed exactly one option" as 0 |
| casino (added with the skeleton, 2026-06) | legal moves for next player (captures shift with the shared table) | pre-move baseline, refreshed per iteration (shedding-style) | a capture removes table cards and a trail adds one; the shared table is the coupling surface |
| vying (added with the skeleton, 2026-06) | by move TYPE, not option count: raise/fold record delta 1, check/call 0 | n/a | betting interaction is move-type: a raise pressures every opponent and a fold removes a contender; an option-count probe under-measures it |

If during implementation a skeleton's definition proves incoherent, STOP and update this table first -- do not improvise in code. Each skeleton's definition gets its own unit test with a hand-constructed state where the delta is computed by hand.

`BatchResult` gains `AllTurns [][]TurnRecord` and `AllLeaders [][]int8` parallel to `AllEvents`.

**Steps (TDD):** failing test first -- run a 5-game shedding batch, assert every game has `len(Turns) > 0`, every `LegalMoves >= 1`, and that a forced-play fixture (1-card hands) records `LegalMoves == 1`; implement; full suite; commit `feat(sim): record per-turn legal-move counts and option deltas`.

### Task 8: Progress/lead tracking

**Files:**
- Modify: `pkg/sim/batch.go` (GenericRunner interface + loop)
- Modify: all three `pkg/skeleton/*/runner.go`
- Test: per-skeleton runner tests

**Dependencies:** Task 3

Add to GenericRunner:

```go
// Progress returns each player's progress toward winning in [0,1].
// Monotonicity is NOT required; it is a snapshot ranking signal.
// shedding:     1 - len(hand)/initialHandSize
// tricktaking:  playerScore / max(1, totalScorePossibleSoFar)  (avoidance: inverted)
// rummy:        clamp(1 - deadwood/initialDeadwoodEstimate, 0, 1)
Progress(state *GameState, g *genome.Genome) []float64
```

Batch loop appends `argmax(Progress)` (or -1 on tie) to `Leaders` after each applied move.

**Steps (TDD):** per skeleton, test that (a) Progress returns one value per player in [0,1], (b) the eventual winner's final Progress is the maximum in a played-out game across 10 seeds (allow ties), (c) shedding progress increases when a player sheds. Implement, suite, commit `feat(sim): per-player progress tracking for arc metrics`.

---

## Phase 2: Metric reimplementation

Each metric task follows the same discipline: value-level unit tests on synthetic `BatchResult` fixtures FIRST (the v1 sanity-check culture: "War must score ~0"), then implementation, then re-run the calibration suite (`-tags calibration`) and record the new numbers in the test's comment block. Weights are NOT tuned here -- that is Task 14.

### Task 9: MeaningfulDecisions -- implement the spec

**Files:**
- Modify: `pkg/fitness/metrics.go:54-99` (replace `computeDecisionDensity`)
- Test: `pkg/fitness/metrics_test.go`

**Dependencies:** Task 7

```go
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
	if total == 0 { return 0 }
	return float64(meaningful) / float64(total)
}
```

**Unit tests:** all-forced fixture (every LegalMoves=1) => 0.0; all-choice fixture => 1.0; mixed 30/70 => 0.3. Integration assertion: evaluate the whist seed and assert density is NOT 1.0 and NOT 0.0 (the old structurally-pinned value was 1.0 for all trick-taking).

Commit: `feat(fitness): decision density from real legal-move counts (implements spec, un-pins trick-taking)`.

### Task 10: GameArc -- within-game trajectory

**Files:**
- Modify: `pkg/fitness/metrics.go:104-169`
- Test: `pkg/fitness/metrics_test.go`

**Dependencies:** Task 8

Replace seat-entropy + turn-CV with a real arc, computed from `AllLeaders` + winners:

```go
// computeGameArc: a good arc = early uncertainty (the eventual winner was not
// already leading at midgame) + late resolution (the leader near the end wins).
//   comeback   = P(winner != leader at 50% of turns), target ~0.5, score peaks there
//   resolution = P(winner == leader at 90% of turns)
//   leadChanges = mean lead changes per game, saturating at 3
// arc = 0.4*tent(comeback, 0.5) + 0.4*resolution + 0.2*min(leadChanges/3, 1)
// tent(x, c) = 1 - |x-c|/c  (peaks at c, 0 at 0 and 2c)
```

A pure coin flip decided on the last turn scores low on resolution; a foregone conclusion scores 0 on comeback's tent. The old metric gave both ~1.0.

**Unit tests:** synthetic leader-tracks -- (a) wire-to-wire leader who wins: resolution 1, comeback 0, arc ~0.4+; (b) random leader every turn with random winner: resolution ~1/N => arc low; (c) winner trails at 50%, leads from 75%: arc high (>0.7). Plus: keep the CV scale-invariance regression test for whatever turn-spread term remains, or delete it with a comment if turn-CV is fully removed (decision: remove turn-CV; session-length already covers duration).

Commit: `feat(fitness): game arc from per-game lead trajectories (comeback + resolution + lead changes)`.

### Task 11: Interaction -- option-perturbation

**Files:**
- Modify: `pkg/fitness/metrics.go:174-210`
- Test: `pkg/fitness/metrics_test.go`

**Dependencies:** Task 7

```go
// computeInteraction: fraction of turns whose move perturbed the next player's
// options (OptionDelta != 0) or carried a direct-attack event
// (EventSpecialTriggered with skip/draw/reverse detail, EventTrickWon).
// A discard that does not change what the opponent can legally do is NOT
// interaction -- that was the old metric's central flaw.
```

Scale: `clamp(ratio/0.5, 0, 1)` -- recalibrate the denominator in Task 14 from seed-game spread, not assumption.

**Unit tests:** solitaire-like fixture (all OptionDelta 0, no attack events) => 0; draw-2-heavy shedding fixture => high; assert hearts-4p no longer pins to the old deterministic 0.657.

Commit: `feat(fitness): interaction from option perturbation, not event taxonomy`.

### Task 12: SessionLength -- uniform turn semantics

**Files:**
- Modify: `pkg/fitness/metrics.go:254+`, three runners if needed
- Test: `pkg/fitness/metrics_test.go`

**Dependencies:** Task 7

Define the unit as **decisions per player**: `decisionsPerPlayer = len(TurnRecords where player==p) averaged over p`, computed from Task 7 data -- identical semantics across skeletons (the old `state.Turn` meant per-move for shedding, per-card for trick-taking, per-cycle for rummy, silently capping Whist/Hearts). Target band: keep 15-40 *decisions per player* provisionally; Task 14 recalibrates from the classics (measure each seed's decisions/player, set the band to cover all 8 classics with margin).

**Unit tests:** fixture with known TurnRecords => exact decisions/player; whist no longer scores in the falloff/zero region purely due to turn-unit inflation.

Commit: `fix(fitness): session length measured in decisions-per-player uniformly across skeletons`.

### Task 13: Skill measurement hardening (pre-MCTS)

**Files:**
- Modify: `pkg/sim/greedy.go:13-15` (MoveScorer interface), shedding scorer (hardcoded ranks 2/7/10), `pkg/fitness/evaluate.go:48-53`, `pkg/evolution/engine.go:96-97,216-226`
- Test: `pkg/sim/greedy_test.go`, `pkg/evolution/engine_test.go`

**Dependencies:** Task 3

Three sub-fixes:
1. **Genome-aware greedy:** `ScoreMove(move Move, state *GameState, g *genome.Genome) float64`; shedding scorer reads `g.SpecialCards` for offensive/defensive classification instead of hardcoding ranks 2/7/10. Test: a genome whose draw-2 special is on rank 8 -- greedy must value holding 8s as it valued 2s before.
2. **Statistical power:** greedy batch 50 -> 200 games in `evaluate.go` (SE on win rate ~0.035 instead of ~0.07). Keep random at 200. Re-measure eval noise in the calibration suite comment block.
3. **Elite re-evaluation (winner's curse):** in `engine.go`, elites no longer carry `Valid: true` forever; re-evaluate elites every generation with a fresh seed and set `Fitness.TotalFitness` to the running mean (store `EvalCount`, `FitnessSum` on Individual; both initialize to 1/first-eval at creation, reset on any mutation/crossover since the genome changed). Expect transient instability in BestFitness trajectories vs old runs -- that is the correction working, not a regression. Test: an individual evaluated 5 times reports the mean, not the max; `BestFitness` over a noisy constant-genome population converges toward the true mean instead of max-of-noise.

Commit each sub-fix separately: `feat(sim): genome-aware greedy scorer`, `feat(fitness): 200 greedy games for skill power`, `fix(evolution): re-evaluate elites, report mean fitness (kills winner's curse)`.

---

## Phase 3: Calibration gate

### Task 13.5: Stabilization checkpoint -- reporting-only calibration

**Files:**
- Create: `cmd/darwindeck/calibrate.go` (a `calibrate` subcommand)

**Dependencies:** Tasks 9, 10, 11, 12, 13

Tasks 1-13 change the measurement stack in seven places at once; if Task 14's gate then fails, root cause is combinatorially ambiguous. Before any weight tuning, make every metric individually inspectable:

**Steps:**
1. Implement `./bin/darwindeck calibrate`: evaluates the 8 classics + 2 degenerate fixtures over `CalibrationSeeds`, prints a per-genome table of all 5 raw metric means with sd (no weighting), plus games/sec throughput.
2. Run it; commit the output table into the plan-tracking doc (`docs/plans/2026-06-11-audit-remediation-checkpoint.md`). Sanity-inspect each COLUMN independently: decision density must vary within every skeleton (the old pinned values 1.0/1.0/0.657 must be gone); arc must differ between gin rummy and the instant-knock fixture; interaction must not be 1.0 everywhere. Any column that is still a skeleton constant means its task is incomplete -- fix THAT task before proceeding.
3. Record throughput: if games/sec dropped more than 3x from pre-plan baseline (measure baseline before Task 7 with the same command against old code via `git stash` or a worktree), profile (`go test -bench`, pprof) and optimize the instrumentation hot path before continuing.
4. Commit: `feat(cli): calibrate subcommand -- per-metric reporting for attribution`.

### Task 14: Make the calibration suite pass; recalibrate weights and scales

**Files:**
- Modify: `pkg/fitness/metrics.go` (weights at ~line 24-41, scale constants), `pkg/fitness/calibration_test.go` (remove `//go:build calibration` tag)
- Test: the calibration suite itself

**Dependencies:** Task 13.5 (checkpoint table must be clean first)

**Steps:**
1. Run `go test -tags calibration ./pkg/fitness/ -v`; record all 10 means (8 classics + 2 degenerates).
2. Tune ONLY scale constants and the 5 weights until: every classic > every degenerate + 0.05; gin > instant-knock + 0.10; classic spread is sane (gin and hearts should plausibly top the classics). Tune scale constants before touching weights. Document each constant chosen with the measured seed values that justify it. If no setting satisfies the gate, a metric is still broken -- go back to its task; do NOT weaken the gate.
2b. **Exit condition (prevents an infinite tuning loop):** if after 3 rounds of metric-fix-then-retune the separation criterion still cannot be met, the permissible escalations are, in order: (a) add a sixth metric that targets the specific unseparated pair (documented rationale); (b) reduce the margin for that ONE pair with written justification in the test; (c) replace a classic that no reasonable metric combination separates (requires noting it in README -- it means that classic's fun is invisible to simulation, itself a publishable finding). Never silently loosen the global gate.
3. Remove the build tag so calibration runs in the default suite (runtime guard: keep total runtime < 60s by reducing eval repeats to 5 with a fixed seed list).
4. Full suite green. Commit: `feat(fitness): calibrated metrics -- classics now outrank degenerate fixtures (closes the audit's falsification finding)`.

### Task 15: Fitness floor derived from calibration

**Files:**
- Modify: `pkg/evolution/novelty.go:19` and all 8 `FitnessFloor` uses; `cmd/darwindeck/main.go` (flag)
- Test: `pkg/evolution/novelty_test.go`

**Dependencies:** Task 14

Replace the hardcoded 0.70 (which zeroed the selection gradient for 47% of mutants and all four trick-taking classics) with a config field `FitnessFloor` defaulting to `minClassicCalibration - 0.05` (a named constant updated by Task 14's measurements, with a comment pointing at the calibration suite). Add `TestFloorAdmitsAllClassics`: every classic seed's mean fitness clears the default floor. Commit: `fix(evolution): fitness floor derived from classic-seed calibration, not folklore`.

### Task 16: Tier 1 robustness + hooks

**Files:**
- Modify: `pkg/fitness/tier1.go` (RunTier1 signature, thresholds), call sites in `pkg/evolution/engine.go`, `pkg/fitness/evaluate.go`
- Test: `pkg/fitness/tier1_test.go`

**Dependencies:** Task 3

Two fixes:
1. **Hooks:** `RunTier1(g, runner, baseSeed, hooks ...sim.HookFunc)` and pass the same borrowed-mechanic hooks Tier 2 uses -- the gate must validate the same game being evolved (the dd-wfi borrow that caused 198/200 timeouts sailed through hook-less Tier 1).
2. **False-reject rate:** 5 games with kill-on-single-timeout rejects healthy rummy seeds 13-20% of the time. Change to 10 games, fail if `errors > 0 || timeouts >= 3 || completions < 7` (keep the existing 3+-player sweep check and avg-turns check). Add `TestTier1AcceptsClassics`: each of the 8 classics passes Tier 1 on >= 28/30 seeds; `TestTier1KillsInstantKnock`: the degenerate fixture fails on a majority of seeds.

Commit: `fix(fitness): tier-1 gate runs hooks and tolerates single-timeout noise`.

---

## Phase 4: QD layer fixes

### Task 17: Replace the degenerate behavior descriptor

**Files:**
- Modify: `pkg/evolution/behavior.go`
- Test: create `pkg/evolution/behavior_test.go`

**Dependencies:** Tasks 9, 11 (needs the un-pinned metrics)

New axes -- X: decision density, Y: interaction (both now real per-genome variables with measured spread; the old Y, win entropy, put 91% of genomes in one row of 10). Keep `[2]float64` shape so novelty/MAP-Elites code is unchanged.

**Tests (this file is currently 0% covered):**
- `TestDescriptorSpread`: descriptors of the 8 classics + 50 mutants occupy >= 4 distinct rows AND >= 4 distinct columns of the 10x10 grid (the empirical anti-degeneracy gate; the old descriptor fails it).
- `TestGridCellBounds`: corners and out-of-range clamp correctly.
- `TestDistanceMetric`: symmetry, identity, known values.

Commit: `feat(evolution): behavior descriptor on decision-density x interaction (old entropy axis was 91% degenerate)`.

### Task 18: Novelty-archive semantics + engine tests

**Files:**
- Modify: `pkg/evolution/novelty.go:185-245` (admission/eviction)
- Test: `pkg/evolution/novelty_test.go`, `pkg/evolution/mapelites_test.go`

**Dependencies:** Task 17

1. **Admission:** replace per-generation-max-normalized threshold (which re-admits persisting elites every generation) with an absolute distance threshold: admit iff distance to nearest archive entry > `NoveltyAddThreshold` (initialize from measured descriptor spread; adaptive: halve/double to keep admissions ~2-5% per generation).
2. **Eviction:** replace FIFO with uniform-random eviction at cap (preserves coverage memory; FIFO turned the archive into a sliding window).
3. **MAP-Elites admission:** drop the `FitnessFloor` gate from *archive admission* (cells should hold their best occupant regardless of global floor); keep the floor for selection/output only.
4. **Engine tests (currently 0%):** MAP-Elites `Run`/insertion/elite-replacement -- insert two genomes mapping to the same cell, assert the fitter one holds the cell; novelty engine end-to-end smoke on 3 generations asserting archive growth and no re-admission of identical descriptors.

Commit: `fix(evolution): real archive semantics for novelty search; test MAP-Elites and novelty engines`.

---

## Phase 5: MCTS skill instrumentation

### Task 19: GameState.Clone + determinized MCTS player

**Files:**
- Create: `pkg/sim/clone.go`, `pkg/sim/mcts.go`
- Test: `pkg/sim/mcts_test.go`

**Dependencies:** Task 3 (pure queries are mandatory for tree search)

v2 has no Clone and no MCTS (the design promised it in four places). v1's MCTS was omniscient -- it cloned hidden hands; do NOT copy it.

**Performance budget (hard constraint, set before writing code):** Tier 2 evaluation of one genome including 20 MCTS games must complete in <= 2s single-threaded at the default settings below. Benchmark first (`BenchmarkMCTSGame`); if the budget cannot be met, the DEFAULT path is MCTS-for-top-decile-only (greedy two-tier for the rest) -- defined now, not as an afterthought.

**Allocation strategy is benchmark-determined, not assumed:** ~40k clone+rollout state copies per genome will create GC pressure. Step 1 of this task is `BenchmarkCloneRollout` measuring allocs/op and GC time at target iteration counts on the naive heap-allocating Clone. If GC exceeds ~10% of benchmark wall time, adopt `sync.Pool` for GameState/slices (v1 precedent: `src/gosim/engine/types.go`) or value-semantic flat arrays BEFORE building the tree -- the architecture follows the benchmark result.

0. **Move identity stability (prerequisite test):** MCTS aggregates statistics for "the same move" across determinizations and clones, so `Move` must have stable identity: assert `reflect.DeepEqual`-comparable or add a canonical `Move.Key() string`. Test: generate moves on a state, clone the state, generate again -- the move lists must be element-wise identical (order AND content); generate on two different determinizations of the same info-state -- moves referring to the player's own (known) cards must have equal keys. This test lands BEFORE any MCTS code (Task 1's map-sort is necessary but not sufficient).
1. **Clone:** deep-copy GameState (Deck, Hands, Discard, Tableau, Scores, Events excluded -- document why: events are observational, cloning them wastes memory in rollouts). Test: mutate clone, assert original unchanged, field-by-field.
2. **Determinization:** from player p's perspective, hidden cards = deck + all other hands. `Determinize(state, p, rng)`: pool hidden cards, shuffle, redeal preserving each zone's count and p's own hand. Test: p's hand and all public zones identical after determinization; hidden zone sizes preserved; the union of all cards is exactly the original multiset.
3. **ISMCTS player:**

```go
type MCTSAI struct {
	Iterations       int // default 200
	Determinizations int // default 10
	RolloutCap       int // max rollout turns, default 200
}
// SelectMove: for each of D determinizations run Iterations/D UCT iterations
// (UCB1, c = 1.4) on the determinized clone using runner.GenerateMoves/
// ApplyMove/Upkeep/CheckEnd, random rollouts to terminal or cap; aggregate
// root-child visit counts across determinizations; return most-visited move.
```

Tests: (a) MCTS picks the winning move in a constructed one-move-to-win shedding state; (b) MCTS vs random on gin rummy wins >= 60% over 50 games (sanity, generous bound); (c) determinism given fixed rng seed; (d) `go test -race` clean.

Commit in three steps: `feat(sim): GameState.Clone`, `feat(sim): hidden-info determinization`, `feat(sim): determinized ISMCTS player`.

### Task 20: Two-tier skill gradient

**Files:**
- Modify: `pkg/fitness/evaluate.go`, `pkg/fitness/metrics.go:214+`
- Test: `pkg/fitness/metrics_test.go`, calibration suite

**Dependencies:** Tasks 13, 19

Implement the design's formula with the dd-qt7 lesson (empirical seat-0 baselines everywhere):

```go
// skill = 0.4*max(0, greedyWR - randomBaselineWR) /(1-randomBaselineWR)
//       + 0.6*max(0, mctsWR  - greedyWR)          /(1-greedyWR)
// All win rates are seat-0; baselines are EMPIRICAL from the same-seed
// random batch (dd-qt7). 20 MCTS games per Tier 2 (design's number) --
// expensive, so MCTS batch runs only for genomes that pass Tier 1, and only
// if Task 19's 2s/genome budget held; otherwise the default is
// MCTS-for-top-decile (per-generation, after greedy two-tier ranking), with
// the mode recorded in results meta.json.
```

Tests: zero-skill game (greedy==random==mcts seat-0 rates) => 0 regardless of FPA; greedy-detectable-only game => capped at 0.4 term; calibration expectation: gin rummy two-tier skill >= its greedy-only skill, war-like fixture ~0. Re-run calibration suite (Task 14 gate must still pass; adjust the skill scale constant if needed, weights stay).

Commit: `feat(fitness): two-tier skill gradient (greedy + ISMCTS) per v2 design`.

### Task 21: MCTS playtest difficulty

**Files:**
- Modify: `cmd/darwindeck/main.go:173,217`, `pkg/playtest/`
- Test: manual + existing playtest tests

**Dependencies:** Task 19

Accept `--difficulty mcts`. Commit: `feat(playtest): mcts opponent`.

---

## Phase 6: Borrows, playtest parity, structural prevention

### Task 22: Make shedding scoring borrows outcome-affecting (multi-round shedding)

**Files:**
- Modify: `pkg/skeleton/shedding/runner.go`, `pkg/genome/genome.go` (SheddingParams gains `RoundsPerGame int // 1-5; >1 only meaningful with scoring borrows`)
- Test: `pkg/skeleton/shedding/runner_test.go`, `pkg/fitness/borrowed_hooks_test.go`

**Dependencies:** Task 3

**Scope acknowledgment: this is a DESIGN CHANGE, not a bug fix** -- it adds a genome parameter and changes shedding game semantics mid-remediation. The alternative (removing both borrows) shrinks cross-skeleton novelty, the v2 design's flagship feature, to 4 pairs; the design change is preferred but must carry its own calibration sub-check (below).

**Structural decision (consensus-required):** Task 22 runs STRICTLY SERIAL -- no other task in flight while it lands (it touches all six genome surfaces: schema, mutation, crossover, validation, rulebook, runner; highest merge-conflict risk in the plan). After it merges, re-run the full calibration suite and Task 13.5's `calibrate` report as a re-baseline; if any classic's score moved > 0.02, investigate before proceeding (classics carry no scoring borrows, so movement indicates an unintended semantics change). If this task stalls, the ring-fenced fallback is borrow removal (Task 23 pattern) so it can never block core remediation.

Both whitelisted shedding borrows (MechMeldBonus, MechAvoidance) write `state.Scores`, which nothing reads -- they are advertised in rulebooks but cannot affect outcomes. Fix with real game design (Mau-Mau scoring): when the genome has a scoring borrow AND `RoundsPerGame > 1`, emptying your hand ends the *round* and banks Scores via the hooks; after R rounds, highest (or lowest, for MechAvoidance) total Scores wins. Round transition happens in `Upkeep` (Task 3's home for redeals). When no scoring borrow is present, behavior is unchanged (first empty hand wins).

Tests: (a) with MechMeldBonus and 3 rounds, a player can win on points without winning the most rounds (construct via seeded play); (b) borrow integration test now asserts Scores *determine the winner*, not merely that they mutate; (c) rulebook renders the round structure; (d) **evolvability sub-check**: run the Task 13.5 `calibrate` command on a MeldBonus+3-rounds shedding genome and assert its metrics differ measurably from the same genome without the borrow -- if the borrow is fitness-invisible, it is still inert from evolution's perspective and the task is not done. Update mutation/crossover/validate/rulebook for the new param in the same commit (the audit's six-surface lesson -- never add a field to fewer than all surfaces; Task 25 adds the structural guard).

Commit: `feat(shedding): multi-round scoring makes MeldBonus/Avoidance borrows outcome-affecting`.

### Task 23: Remove unimplemented borrow types

**Files:**
- Modify: `pkg/mechanic/hooks.go:51-64` (empty MechPlayMultiple/MechTrump cases), `pkg/genome/validate.go` (whitelist), `pkg/evolution/mutate.go` (candidate lists), seeds if any
- Test: `pkg/genome/validate_test.go`

**Dependencies:** none

MechPlayMultiple and MechTrump have no hook implementation; flagship ranks 11/14/15/16/18/19 carried the inert (shedding, PlayMultiple) borrow. Remove both from the whitelist and the MechanicType enum (or mark reserved), per the project's own fix-by-removal pattern (dd-027 precedent). Verify: `grep -rn "MechPlayMultiple\|MechTrump" pkg/` returns only historical comments.

Commit: `fix(mechanic): remove unimplemented PlayMultiple/Trump borrows from search space`.

### Task 24: Playtest parity -- hooks + ratings capture

**Files:**
- Modify: `pkg/playtest/session.go` (or equivalent), `cmd/darwindeck/main.go`
- Create: ratings append to `playtest_results.jsonl` (v2 schema: timestamp, genome path, difficulty, seed, winner, turns, rating 1-5, comment, stuck flag)
- Test: `pkg/playtest/session_test.go`

**Dependencies:** Tasks 21, 22

Playtest currently never constructs borrowed-mechanic hooks, so humans play a different game than fitness evaluated -- the stated validation loop is broken. Build hooks exactly as `pkg/fitness/evaluate.go` does (extract a shared helper `mechanic.HooksFor(g)` so the two call sites cannot drift). After each session, prompt for a 1-5 rating and append the JSONL record. Test: a scripted session against a borrow-carrying genome fires hook events; helper is the single hook-construction site (grep-test).

Commit: `fix(playtest): run borrowed-mechanic hooks and capture human ratings (closes evaluation/playtest divergence)`.

### Task 25: Structural inert-parameter prevention

**Files:**
- Create: `pkg/genome/consumers_test.go`

**Dependencies:** none (strengthens everything after)

The audit counted 9+ inert-parameter bugs across six hand-synchronized surfaces. Add a structural test using `go/parser`: walk the AST of `pkg/skeleton/...`, `pkg/sim/...`, `pkg/fitness/...`, `pkg/mechanic/...` collecting selector identifiers; assert every exported field of `SheddingParams`, `TrickTakingParams`, `RummyParams`, and every genome field that mutation can touch (cross-reference the field list in `pkg/evolution/mutate.go`) appears as a selector in at least one consuming package OUTSIDE `pkg/genome`, `pkg/evolution`, `pkg/output`. "Appears as selector" is defined narrowly and cheaply: any `ast.SelectorExpr` whose `Sel.Name` equals the field name, in any file of the consuming packages -- field names are unique enough across the genome structs that receiver/alias resolution is unnecessary (verify uniqueness in the test; if two fields collide, qualify by struct type via `go/types` only for those). This proves "read somewhere," not "semantically affects outcomes" -- it is a tripwire for the dd-027 class, not full prevention; Task 22's evolvability sub-check pattern is the semantic complement. The allowlist (with mandatory `// why:` comment) is capped: if it exceeds 3 entries, the test has gone toothless -- treat that as a failure.

Test the test: temporarily add a dummy field to RummyParams + mutate.go, confirm failure, remove.

Commit: `test(genome): structural guard -- every evolvable param must be consumed by a runner/fitness (prevents dd-027 class)`.

### Task 26: Derive borrow integration tests from the whitelist

**Files:**
- Modify: `pkg/fitness/borrowed_hooks_test.go:20-21` (hardcoded case list)

**Dependencies:** Task 23

The borrow test's case list is hand-coded and can drift from `validBorrows`. Export or test-expose the whitelist and generate test cases from it, so a newly whitelisted borrow is automatically covered (failing until its hook behaves). Verify by temporarily whitelisting a fake borrow: test must fail.

Commit: `test(fitness): borrow integration cases derived from validBorrows whitelist`.

---

## Phase 7: Re-run and republish

### Task 27: Experiment harness -- null control + tested statistics

**Files:**
- Modify: `cmd/darwindeck/experiment.go` (`configs` at :51), `cmd/darwindeck/main.go:117-129`
- Create: `cmd/darwindeck/experiment_test.go`

**Dependencies:** Tasks 14-18

1. Add a `random` config: pure random genome sampling (mutate-from-seeds with no selection) at the same evaluation budget -- the null hypothesis for a deliberately small search space.
2. Unit-test the aggregation math (median, IQR, coverage, QD-score) on hand-computed fixtures -- it currently backs published comparisons with zero tests.
3. Add a Mann-Whitney U implementation + test (the audit hand-computed it; make it part of the report output).

Commit: `feat(experiment): random-search null config and tested statistics`.

### Task 28: Full re-runs on fixed code

**STATUS (2026-06-13): COMPLETE via the failed-review loop -> HONEST EXIT.** The flagship was re-run on remediated code four times (postfix, r2, r3, r4); step 3 designer review and step 4 failed-review loop ran the budgeted 3 rounds + 1 authorized extra. Verdict: 0 publishable -- evolution gamed the newest veto each round or rediscovered an existing game (round-4 top 30 = 10 wild-union shedding + 10 Whist + 10 Gin/Knock; best honest fitness 0.739). The reproducible flagship bundle is `results/2026-06-12-flagship-r4/` (re-published through the Wave M veto-stable path; meta.json complete). NOT done: the 4-config x 15-seed experiment matrix (step 2), running on a separate machine -- results.json lands later and fills the README table placeholder. See the LOOP CLOSED section of the checkpoint doc.

**Files:**
- Create: `results/2026-MM-DD-flagship/` and `results/2026-MM-DD-experiments/` (tracked artifacts per Task 4 convention)

**Dependencies:** ALL of Phases 0-6 (this is the payoff)

**Steps:**
1. `make build-v2`; run `./bin/darwindeck evolve -population 2000 -generations 200 -workers 256` (~15-25 min; MCTS batches will add time -- if wall time exceeds 60 min, reduce MCTS to top-decile genomes only and document the mode in meta.json).
2. Run the experiment: `./bin/darwindeck experiment -configs baseline,hybrid,mapelites,random -seeds 15` (~2h).
3. Manually review the top 10 rulebooks AS A DESIGNER before publishing: melds reachable? knock thresholds meaningful? rules in the rulebook all real? Do not publish a champion you would not deal out at a table.
4. **Failed-review loop (explicit):** if any reviewed champion is degenerate, publication is HARD-BLOCKED. Re-entry point is Phase 3: (a) encode the rejected champion as a new degenerate fixture in `pkg/seeds/degenerate.go`, (b) re-run Task 14 until the calibration gate rejects it (adjusting metrics/scales, not the gate), (c) re-run this task from step 1. Each loop iteration strengthens the ground-truth suite -- rejected champions are the most valuable fixtures the project can get.
5. Write `meta.json` per the Task 4 convention (commit SHA, go version, platform, CLI args, master seed, calibration seeds, MCTS mode); copy summary.json, results.json, top-10 rulebooks/reports into `results/`; commit: `results: post-remediation flagship + experiment artifacts`.

### Task 29: Republish numbers and close the loop

**STATUS (2026-06-13): DONE except the matrix table.** README rewritten to the honest exit (Wave M commit 3): playable games + correct ranking of Whist/Gin rediscoveries, NO novel fun across 4 rounds, the failed-review-loop arc as the headline, the lesson, current fitness/validation sections, calibration suite noted as the permanent regression harness. The algorithm-comparison table is a marked PLACEHOLDER (`<!-- TABLE: pending final experiment matrix -->`) -- the experiment matrix fills it later; no numbers invented. Pre-fix numbers removed. CLAUDE.md/ROADMAP truth-pass for the honest exit folds into the matrix-landing commit.

**Files:**
- Modify: `README.md`, `CLAUDE.md`, `ROADMAP.md`

**Dependencies:** Task 28

Regenerate the README results table from the new `results.json` (including the random null and MAP-Elites rows), remove Task 6's caveat, update the fitness-metric table to the new definitions, note the calibration suite as the fitness function's regression harness, and update ROADMAP's human-playtesting section: next milestone is N >= 10 rated human sessions of post-remediation games via Task 24's instrument (the only remaining unvalidated claim).

Commit: `docs: republish results from post-remediation runs`.

---

## Phase 8 (optional, after everything above): NSGA-II experiment

### Task 30: Pareto selection vs weighted sum

**Files:**
- Create: `pkg/evolution/nsga2.go`, test file
- Modify: `cmd/darwindeck/main.go` (algorithm flag), experiment configs

**Dependencies:** Task 28

The day-one open question (`evolved-card-games.md:118`); weighted-sum saturation caused both the v1 0.8403 and v2 0.919 ceilings. Implement NSGA-II (fast non-dominated sort + crowding distance) over the 5 raw metrics, add as `-algorithm nsga2`, and run the Task 27 experiment harness against hybrid. **Pre-register the evaluation criterion in this plan doc BEFORE the runs** (proposed: NSGA-II wins if behavioral coverage >= hybrid's AND >= as many of its top-10 pass designer review; note n=15 seeds gives the Mann-Whitney test limited power -- report effect sizes, do not overclaim). Publish whichever wins; this is an experiment, not a commitment.

---

## Execution notes

- **Waves for parallel execution:** Wave A = Tasks 1, 4, 5, 6, 23, 25 (independent). Wave B = Tasks 2, 3. Wave C = Tasks 7, 8 (after 3). Wave D = Tasks 9-13 (after 7/8). Wave E = Task 13.5, then 14, then 15, 16. Wave F = Tasks 17, 18, 19 (after their deps), then 20, 21, then **Task 22 strictly alone** (see its structural decision), then 24, 26. Wave G = 27, 28, 29. Optional H = 30.
- **Every task:** full `go build ./... && go vet ./... && go test ./pkg/... ./cmd/...` before its commit (Completion Gate). After Phase 3, that includes the calibration suite -- it is the project's conscience now.
- **Calibration build tag removal is non-negotiable:** the tag exists only between Task 2 and Task 14; Task 14 is not complete while the tag exists. No other task may depend on "calibration is optional".
- **Performance checkpoints:** measure evaluation throughput (games/sec) before Task 7 (baseline), at Task 13.5, and after Task 20. A >3x regression at any checkpoint triggers a profiling/optimization sub-task before proceeding -- instrumentation and MCTS are the two expected hot spots.
- **Seed discipline:** all calibration measurements use the pinned `CalibrationSeeds` list (Task 2); per-game batch seeds remain the existing deterministic derivation (`BaseSeed + gen*10000 + idx`) -- add a doc-test in `pkg/sim/batch_test.go` asserting two games with the same derived seed produce identical event streams regardless of batch position.
- **Do not tune weights outside Task 14.** If a later task breaks calibration, the task is wrong, not the gate.
- **Task 5 (history rewrite) requires explicit user approval at execution time.**
- **CI (optional, user-gated):** if the user wants CI enforcement of the calibration gate and structural tests, that is a `.github/workflows` change -- sensitive file, requires explicit approval; not on the critical path.

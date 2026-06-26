# Generative grammar de-risk prototype (2026-06-23)

`pkg/grammar` + `cmd/grammar-proto`. One question: can a game be a composition of
typed primitives run by ONE generic interpreter, and does that composition space
stay playable-by-construction -- or does it rediscover the v1 desert (generative
but mostly garbage)?

Raw run: `run-2026-06-23.txt`.

## The bet

A game = `setup x move-generator x end-condition x scoring` (`GameSpec`, `spec.go`),
plus a set of typed *modifiers* (`modifier.go`, lifted from v2's borrows). One
interpreter (`runner.go`) plays any composition. Two structural guarantees:

- **Safety (never stuck):** every move-generator carries an unconditional fallback
  (PlayMatch -> draw/pass, BeatOrPass -> pass, Accumulate -> stick, Capture -> pass),
  so `LegalMoves` is never empty. No dead states.
- **Liveness (always terminates):** a monotone non-increasing progress potential
  (`progressSig` = deck + hands + not-yet-folded, bounded below by 0) plus a
  stalemate rule that ends the game when the potential is flat for `3p+6` turns.
  So even a composition whose own end-condition is unreachable still terminates.

## Result

Safety and liveness hold across the ENTIRE enumerated space. Under random AI:

| Property | Untyped cross-product | Well-typed grammar | + modifier axis |
|---|---|---|---|
| specs / families | 114 / 20 | 21 / 4 | 20 / 20 |
| ever STUCK (safety) | **0** | 0 | 0 |
| ever non-terminating (liveness) | **0** | 0 | 0 |
| playable (terminates + agency) | 70 (61%) | **21/21 (100%)** | **20/20 (100%)** |

The bet is confirmed: **playable-by-construction holds across the whole composition
space** -- safety and liveness are 0-violation even on the loose cross-product. No
v1 desert. All 4 hand-coded skeletons the grammar covers (shedding, climbing,
banking, casino) reproduce and play.

## The coherence type: 20 families -> 4, all canonical

The 16 dropped families are not noise -- they are the signal that the loose
enumeration is under-typed. They fall into three physical patterns (`GameSpec.WellTyped`):

1. **`deck_out` is unreachable for `play_match` / `beat_or_pass`** (8 families).
   Those move-gens empty *hands*, not the deck (the empty-hand->draw fallback
   refills a spent hand), so the deck only drains via the stalemate path. `deck_out`
   belongs to `capture`.
2. **rank-only / suit-only matching is agency-dead** (8 families). Too few legal
   plays; random play collapses to forced draws (agency ~ 0). Only `MatchEither`
   (Crazy Eights' rank-OR-suit) has real choice. A modest wild does NOT rescue it
   (see below).
3. **the score axis is inert on these ends** (4 families). On an `empty_hand` end
   the player who emptied is uniquely determined, so `fewest_cards` == `first_out`.
   On `capture` both `most_captured` and `high_score` argmax the same `Scores`
   tally. On `bust`, `high_score` would reward the biggest *bust* (a total over the
   target). So each move-gen has exactly ONE non-inert base score.

Apply all three and the **20 families collapse to the 4 canonical skeletons, and
all 4 survive**:

```
play_match/either|empty_hand|first_out   [shedding]
beat_or_pass|empty_hand|first_out        [climbing]
accumulate|bust|closest_target           [banking]
capture|deck_out|most_captured           [casino]
```

There are **NO spurious novel base families** -- the base grammar reproduces
exactly the skeletons it covers. That is the grammar's promise made concrete: the
type rules are few and physical, and tightening them makes illegal compositions
*unrepresentable* rather than caught at runtime.

## The modifier axis (where novelty actually comes from)

Thousands needs the orthogonal **modifier** axis: typed addons lifted from v2's
borrow mechanics (`pkg/mechanic`). v2's per-host whitelist
(`validBorrows[skeleton][mech]`) is an ad-hoc, hand-maintained version of what the
grammar makes a type rule -- `Modifier.CompatibleWith(spec)` -- and v2's "don't
whitelist a no-op borrow" rule (dd-lnh) is exactly "a modifier is only well-typed
when the spec's move/end/score actually consumes the signal it produces."

Five productive modifiers ported, spanning all the v2 hook phases:

| Modifier | phase | type rule (CompatibleWith) | v2 borrow |
|---|---|---|---|
| run_play | move-expand | `Move==PlayMatch` | MechRunPlay (deep, id 8) |
| follow_suit | move-restrict | `Move==PlayMatch` | MechFollowSuit (deep, id 7) |
| draw_penalty | after-move | `PlayMatch && End==EmptyHand` | MechDrawPenalty (id 2) |
| knock | win-override | `(PlayMatch\|BeatOrPass) && End==EmptyHand` | MechKnock (deep, id 3) |
| meld_bonus | score-adjust | `Move==Capture && Score==MostCaptured` | MechMeldBonus (id 1) |

Crossing the 4 base families with every compatible modifier subset (size <=3,
v2's borrow cap) gives **20 families from 4 (5.0x), of which 16 are novel** (carry
>=1 modifier), and **every one is playable**: 0 stuck, 0 non-terminating, 20/20
clear the agency bar. The modifier algebra multiplies the family space *without
leaving the playable-by-construction manifold* -- the core result. ALL novelty is
modifier-driven; the base grammar contributes the 4 skeletons, the modifiers
contribute every new game. Adding the 2 missing move-gens (trick-taking, rummy)
and more modifiers compounds this multiplicatively toward the hundreds / thousands.

## Honest caveats

- **A modest wild does NOT rescue agency-dead match rules.** An earlier draft
  claimed a wild-card modifier revives rank/suit-only matching. It doesn't: a
  single wild rank is too sparse (rescue experiment: agency 0.01 -> 0.02, still
  dead). The base agency rule (require rank-OR-suit matching) stands, and the type
  system correctly keeps non-either matching off-type. Wild is left defined but
  non-productive (never enumerated).
- **meld_bonus fidelity** (caught in adversarial review): the scoring now banks
  both same-rank sets AND same-suit runs, matching v2 `applyMeldBonus` (the first
  cut scored only sets, contradicting its own comment).

## Step 4: it runs in the REAL engine (and liveness has to live in the runner)

`adapter.go` makes a `GameSpec` satisfy `sim.GenericRunner`, so a grammar
composition runs through the SAME `sim.RunBatch` engine the hand-coded skeletons
use. Verified: **24 specs (4 canonical + 20 modified), 4800 games, 0 timeouts,
0 errors, 0 stuck** -- every game completes with a real winner and emits the event
taxonomy the metrics read.

Getting there surfaced a real bug the prototype harness had hidden: **the grammar's
liveness guarantee lived in the HARNESS, not the runner.** `PlayRandom` had a
stalemate net (progress-potential flat -> end); the real engine has only a turn
cap. So `draw_penalty` specs (which grow hands and drain the deck) and the
deck-remainder edge of `capture` would stall on an all-pass loop and time out in
`sim.RunBatch` while passing in the harness. Fixed by moving termination INTO the
runner (`CheckEnd`): a PlayMatch all-pass deadlock and a "can't deal another round"
capture end. Now the runner terminates by its own rules -- the harness stalemate
net is a redundant backstop, and the modifier axis reads 20/20 natural-end (was
19/1). Lesson: playable-by-construction has to be a property of the RUNNER to
survive contact with the real engine, not of the test harness.

## Recommendation / status

Not a full rewrite. Grow v2's borrow system into a typed-composition layer on top
of the generic runner:

1. Lift move-gen x end x score into the typed spec. **DONE** (`spec.go`).
2. Tighten the coherence typing (move/end/match/score). **DONE**
   (`GameSpec.WellTyped`; 20 families -> 4 canonical, all surviving).
3. Port the borrow hooks as typed *modifiers* gated against the spec. **DONE**
   (`modifier.go`; 5 productive modifiers, 5.0x families, 16 novel, playability
   preserved). Reviewed adversarially (3 lenses) + verified.
4. Run through the real simulation engine. **DONE** (`adapter.go`, `sim.RunBatch`,
   4800 games clean) -- and the liveness-in-runner fix above.
5. Full FITNESS parity (the 5 metrics). **MOSTLY DONE.** Added the injection
   point `fitness.EvaluateWithRunner(g, runner, greedy, seed)` (skips Tier 0,
   reports the veto without zeroing metrics), so a grammar spec runs through the
   SAME 5-metric pipeline. Parity vs the seed of the same skeleton (`cmd/grammar-
   fitness`, `fitness-parity-2026-06-23.txt`):

   | family | metrics that matched the seed | gap |
   |---|---|---|
   | shedding | decisions, interaction, skill, session (total 0.423 vs 0.447) | arc slightly low |
   | climbing | decisions, arc, interaction, session (total 0.512 vs 0.558) | skill (grammar climbing is structurally simpler than Big Two) |
   | casino | decisions, arc, session | interaction + skill read 0 |

   THE COUPLING, with a worked fix: the metrics read SKELETON-SPECIFIC state
   fields the generic runner stored in generic ones. The shedding choice-impact
   probe swaps `state.TopCard`; the climbing OptionDelta probe + greedy scorer read
   `state.TrickCards`. Pointing the grammar runner at those conventional fields
   (PlayMatch->TopCard, BeatOrPass->TrickCards) took shedding from a `non_agentic`
   VETO to parity and climbing's interaction from 0.000 to 0.676. **Casino is a
   deeper, different mismatch:** the grammar's coarse capture move-gen ("play any
   card, resolve in Apply") makes the legal-move COUNT table-independent, so the
   OptionDelta interaction metric reads 0 structurally and the scorer can't grade
   captures -- a move-GRANULARITY difference, not field-aliasing. Fix = enumerate
   capture targets explicitly in the runner (next).

6. Evolution + judge keying over specs. **DONE** (`evolve.go`, `cmd/grammar-
   evolve`). Genetic operators `Mutate`/`Crossover`/`RandomSpec` stay inside the
   well-typed manifold BY CONSTRUCTION (every output re-checked against
   `WellTyped`), so every individual in every generation is playable -- evolution
   literally cannot reach the v1 desert. Fitness is the real pipeline
   (`EvaluateWithRunner`). `GameSpec.Composition()` is the verdict-table key (the
   structural identity), and `JudgeVerdicts[Composition]` is the judge-in-loop
   plug-in (empty -> neutral, v2's cache-miss-returns-0).

   A pure-fitness GA CONVERGES (12 -> 3 compositions) -- the discovery anti-goal.
   Novelty-aware selection (behavioral distance in 5-metric space from the 4
   canonical seeds) + niche-sharing (a composition-crowding penalty) keeps the pool
   diverse: **16 of the 20 well-typed compositions survive the final generation,
   and the high-novelty survivors are exactly the modifier-driven fusions** the
   grammar exists to find (`beat_or_pass+knock` nov=0.875; `run_play,draw_penalty,
   knock` stacks). Raw run: `evolve-2026-06-23.txt`.

7. The blind judge -- closing the loop. **DONE** (`pkg/grammar/rulebook.go`,
   `pkg/judge/grammar_dossier.go`, `cmd/grammar-judge`). `GameSpec.Rulebook`
   renders a composition as natural-language rules (no grammar internals leak --
   the legibility v2 learned is the bottleneck); `EmitGrammar` writes one BLIND
   dossier per well-typed family (rulebook + greedy-vs-greedy traces via the
   adapter + a termination note), keyed on `Composition`.

   All 20 compositions were blind-judged (one judge each, `judge-verdicts-
   2026-06-23.txt`). The judge is calibrated and the result mirrors v2 exactly:
   the bare canonical bases land as **known** (shedding 0.95, casino 0.82) or
   variant, the modifier fusions read mostly **variant** (a move-tweak doesn't
   change the core decision), and ONE rich fusion -- `run_play + follow_suit +
   knock` -- reads **NOVEL** ("a dual win-path timing decision found in no single
   standard published game"). 1 novel / 17 variant / 2 known.

   Folding the verdicts into a `Composition`-keyed `JudgeVerdicts` table
   (`judge-verdicts.json`) and re-running the novelty-select GA with it
   (`cmd/grammar-evolve -verdicts`, `evolve-judged-2026-06-23.txt`) **surfaces the
   judge-certified-novel composition to the #1 selection slot and demotes the two
   "known" bare bases to the bottom.** The loop is closed: emit -> blind judge ->
   verdict table -> the GA selects for certified novelty -- the original DarwinDeck
   goal, now over the typed grammar space.

This is the whole point: thousands of playable-by-construction families feeding
the *same* judge loop already in use -- the grammar serves the discovery goal, it
is not the goal.

## 2026-06-26: growing the space (lever 1) and illuminating it (lever 2)

The discovery loop saturates because the *space* is small: 6 skeletons x a few
borrows = 66 reachable compositions, and the search only ever visits ~20. You get
more distinct games by enlarging the primitive product and by filling it, not by
searching the same 66 harder. Two changes:

**Lever 1 -- a new move-generator.** Added the `Trick` move-gen (follow-suit
trick-taking: play one card, follow the led suit if held, highest of the led suit
wins the trick and leads next) and the `ModAvoidance` scoring modifier (penalty
cards in your won pile count against you). The space grows **4 -> 5 base families
and 20 -> 26 modified (5.2x)**, and it stays playable-by-construction: tricks empty
hands in lockstep so `deck_out` fires deterministically, the move set is never
empty, **0 stuck / 0 non-terminating** across the enlarged space. New families:
`trick` (Whist), `trick+avoidance` (Hearts), `trick+meld_bonus`,
`capture+avoidance`. Trick reaches FULL fitness parity with the Whist seed through
the real pipeline (`cmd/grammar-fitness`) once the adapter emits `EventTrickWon`.
Each added generator multiplies the family count; rummy (draw-meld-discard) is the
remaining one.

**Lever 2 -- illuminate instead of optimize.** `cmd/grammar-illuminate` runs
MAP-Elites over the typed space: it keeps the best game per cell of a behavior
grid (`family x decisions-bucket x interaction-bucket`) and spends its budget
filling empty cells, never converging. Result (`illuminate-2026-06-26.txt`):
**49 behavior cells filled, ALL 26/26 well-typed families illuminated**, in 130
evaluations -- versus the optimizing GA (`cmd/grammar-evolve`) which converges to
~16 families and throws the rest away. The archive IS the diverse out-of-loop
judging set (best game per family). The two levers compose: a bigger space, and a
search that fills all of it instead of collapsing onto its best corner.

Remaining sharpening (not blockers): casino's coarse capture move-gen
(under-measures interaction/skill), the last move-gen (rummy), and a 3-judge
majority + more compositions for a production verdict table.

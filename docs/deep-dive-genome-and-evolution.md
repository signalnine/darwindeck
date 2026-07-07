# How DarwinDeck represents and evolves card games

DarwinDeck searches for new card games the way you'd search for anything with a
genetic algorithm: encode candidates as data, score them, breed the winners.
The hard parts are all in the details of those three clauses. This doc covers
the two load-bearing ones -- what a game genome actually is, and what the
evolution loop actually does -- with the design lessons that shaped both.

Everything here refers to the v2 pure-Go system (`cmd/`, `pkg/`). Numbers and
names come from the code as of July 2026.

## The genome: a game as parameters, never as logic

v1 encoded games as near-arbitrary programs: bytecode phases, condition
opcodes, free-form win conditions. It could represent almost anything, and
that was the problem. Random points in that space are overwhelmingly broken --
games that deadlock, never end, or contain no decisions -- so the GA spent its
budget rediscovering "don't be broken" instead of searching for "be fun."
Validation-as-a-filter can't fix this; when 99% of the space is desert, the
filter is the bottleneck.

v2 inverts the representation. A genome never encodes game *logic*. It picks
one of six hand-written **skeletons** -- game loops that are correct by
construction -- and parameterizes it:

```go
type Genome struct {
    Skeleton SkeletonType   // shedding | trick_taking | rummy | climbing | casino | vying

    Players  int            // 2-6
    HandSize int            // 3-13, hand_size * players <= 52 enforced

    // Exactly one of these is active, matching Skeleton:
    Shedding    *SheddingParams
    TrickTaking *TrickTakingParams
    Rummy       *RummyParams
    Climbing    *ClimbingParams
    Casino      *CasinoParams
    Vying       *VyingParams

    Borrowed     []BorrowedMechanic // cross-skeleton genes, see below
    SpecialCards []SpecialCard      // rank-triggered effects (skip, reverse, draw-two...)
    Scoring      ScoringConfig      // per-card points, triggers
    TrumpRule    TrumpRule
}
```

Each skeleton is a runner implementing `Setup / GenerateMoves / ApplyMove /
Upkeep / CheckEnd` over a shared `sim.GameState`. The runner guarantees the
two properties evolution must never be able to break: a legal move always
exists (shedding falls back to draw-or-pass, casino can always trail, vying
can always fold), and the game terminates (turn caps are a backstop, but each
skeleton has a real ending -- deck drain, rounds-per-game, max raises).
Parameters control *what* happens on a turn. Whether the game works at all is
not up for mutation.

The six skeletons each contribute a different core decision:

| Skeleton | Core decision | Classic anchor |
|---|---|---|
| shedding | which matching card to dump | Crazy Eights, Mau-Mau |
| trick_taking | which card wins or ducks the trick | Whist, Hearts, Spades |
| rummy | draw source + which deadwood to keep | Gin Rummy |
| climbing | beat the table or pass and keep ammo | Big Two |
| casino | capture the table or trail into it | Casino/Scopa |
| vying | wager on a hidden hand: fold/call/raise | poker |

A `VyingParams{StartingChips: 1000, MinBet: 10, MaxRaises: 3, RoundsPerGame: 12}`
is a complete poker variant. Mutating `MaxRaises` to 1 gives you a tighter,
faster game; mutating `RoundsPerGame` changes the arc from a sprint to a
session. None of those mutations can produce a game that hangs.

The obvious objection: doesn't this collapse the search space to "six games
with sliders"? Two mechanisms push back against that, and they're where the
interesting genetics live.

## Borrowed mechanics: cross-family genes

`Genome.Borrowed` carries mechanics lifted from *other* skeletons. There are
two kinds, and the distinction turned out to matter more than anything else in
the project.

**Shallow borrows** are scoring hooks. Runners emit typed events
(`EventCardPlayed`, `EventTrickWon`, `EventRoundEnd`...), and a borrow like
`MechMeldBonus` or `MechAvoidance` is a stateless function over
`(state, genome, event)` that adjusts scores when events fire. A shedding game
with a rummy meld bonus banks set/run bonuses each round; a vying game with
avoidance penalizes a suit at showdown, so the best hand can net fewer chips.
Hooks can only re-weight outcomes. They never touch the legal-move set.

**Deep borrows** live inside the runners and change what you can *do*.
`MechRunPlay` (climbing combos grafted into shedding) is consulted inside
`GenerateMoves` -- suddenly you can dump a whole same-suit run in one turn.
`MechFollowSuit` restricts the move set; `MechKnock` adds a new move that ends
the game. Blind LLM judges consistently rate hook-only hybrids as "variant"
and move-changing hybrids as candidates for "novel," which matches intuition:
a new way to score an old decision is a house rule, a new decision is a new
game.

Three invariants keep borrows honest, all enforced at the operator level
rather than by filtering:

1. **Whitelisting.** `genome.ValidBorrows` is a per-host table -- e.g. casino
   rejects `MechDrawPenalty` because growing one hand breaks its equal-hands
   redeal invariant. An illegal (host, mechanic) pair fails Tier-0 validation.
2. **Teeth.** `giveBorrowTeeth` rewires supporting parameters when a borrow is
   grafted (run_play needs `HandSize >= 6` and a permissive match rule or
   combos never fire), and it re-runs after *every* mutation, because a later
   `HandSize` tweak can silently defang a borrow. An inert gene is worse than
   no gene: it reads novel in a rulebook while doing nothing in play, and it
   fooled the blind judges exactly once before we made inertness unrepresentable.
3. **Dedup by mechanic.** Two `MeldBonus` entries under different source
   skeletons would double-apply the hook (the bonus silently doubles while the
   rulebook states the single value). Validation and both breeding operators
   key duplicates on mechanic alone.

## The grammar: a second genome, for structure

The skeleton genome is a *dense* space: every point is playable, and the
search explores parameter neighborhoods around six known-good structures plus
their borrow hybrids. `pkg/grammar` is the complementary representation -- a
*combinatorial* space of game structures that keeps the same guarantee.

A `GameSpec` composes typed primitives: one of 7 move-generators (play_match,
beat_or_pass, accumulate, capture, trick, rummy, vying), an end condition, a
scoring rule, and up to 3 of 15 modifiers (trump, bid, teams, nominate,
sum_capture, run_play...). One generic interpreter runs any spec. A coherence
type, `GameSpec.WellTyped()`, makes degenerate compositions unrepresentable --
an unreachable end condition or an inert score rule fails to type-check, so
the search never sees it. `Modifier.CompatibleWith(spec)` is the lift of the
per-host borrow whitelist into a total function.

The payoff is the same playable-by-construction property at combinatorial
scale: 137 modified families, all 137 playable under random play -- zero stuck
states, zero non-termination -- covering roughly 81% of a surveyed corpus of
known card games. Grammar specs run through the identical fitness pipeline via
an adapter (`fitness.EvaluateWithRunner`), so both genome representations
compete on the same metrics.

One rule from this layer is worth engraving: **termination is a property of
the runner, never of the harness**. Early on, a harness-level stalemate
detector made everything look fine while games quietly relied on it; the same
specs then timed out in the real engine. Every move-generator now owns its own
ending (play_match ends on an all-pass round, capture ends when the stock
can't deal a full round, vying caps raises). The test harness is allowed to
observe termination, never to cause it.

## Scoring a genome: three tiers and five metrics

Evaluation is a funnel, ordered by cost:

- **Tier 0 (free):** static checks on the struct -- deck overflow, parameter
  ranges, borrow whitelist, duplicate mechanics.
- **Tier 1 (5 games):** random-AI smoke test. Any hang, error, or degenerate
  outcome kills the genome.
- **Tier 2 (~400 games):** 200 random-AI games plus 200 greedy-AI games at the
  same seeds; the top fitness decile also gets a 20-game MCTS batch.

Tier 2 feeds a battery of **degeneracy vetoes** before any metric is computed:
`seat_participation` (a seat that never acts), `tempo_monopoly` (one player
takes long uninterrupted runs), `non_agentic` (too few real choices),
`dead_match_rule` / `playable_share` (a match rule so tight nothing is ever
playable), `draw_supply_churn` (the game is mostly reshuffling). A vetoed
genome scores zero regardless of its metrics. Because a genome is published
from a single evaluation, publication re-runs each top-N candidate at K fresh
seeds and records `VetoStable` -- a genome that fails its own veto on a
minority of seeds gets demoted rather than shipped.

Five metrics, fixed weights:

| Metric | Weight | What it actually measures |
|---|---|---|
| Meaningful Decisions | 0.25 | fraction of move events that are chosen plays vs forced draws |
| Game Arc | 0.25 | win-entropy across seats (0.6) + turn-count variation (0.4) |
| Interaction | 0.20 | share of events that touch opponents, via per-skeleton OptionDelta probes |
| Skill Gradient | 0.20 | two-tier: greedy over random (0.4 of scale) + MCTS over greedy (0.6) |
| Session Length | 0.10 | target 15-40 turns, linear falloff |

The skill formula is the one worth spelling out:

```
raw = 0.4 * max(0, greedyWR - randomWR) / (1 - randomWR)
    + 0.6 * max(0, mctsWR  - greedyWR)  / (1 - greedyWR)
```

with `randomWR` measured empirically from the same-seed random batch (never
assumed to be 1/players -- seat advantages are real) and the MCTS term
available only to genomes that earned an MCTS grant. Skill a one-ply greedy
can detect saturates at 0.4; the top 0.6 is reachable only by lookahead
outplaying greed. That structure exists because "greedy crushes random" is
cheap to fake with a big obvious heuristic, while "search beats greedy" is
hard to fake.

The metrics are proxies and the code says so in comments. The recurring trap
is measurement blindness: when the climbing skeleton first landed, its seed
(Big Two) scored Interaction 0.000 and total ~0.40 despite passing every veto,
because no probe knew how to measure a beat-or-pass constraint. The number was
an artifact. Every skeleton since ships as a package deal -- runner, an
OptionDelta mode for Interaction, and a greedy scorer for the skill gradient --
because a skeleton the metrics can't see gets bred *against*, silently.

## The evolution loop

The production engine (`evolve -algorithm hybrid`) runs generations of:

```
evaluate population (parallel)  ->  MCTS grants for top decile
  ->  compute novelty + fitness sharing  ->  select next generation
```

**Evaluation.** Each individual gets seed
`BaseSeed + generation*10000 + index`, so every generation is a fresh sample
and any run is reproducible. Survivors accumulate a running mean
(`FitnessSum / EvalCount`) across generations rather than a single point
estimate -- with 400-game batches the per-eval noise on TotalFitness is a few
hundredths, enough to shuffle ranks. A genome that fails validation on
re-evaluation loses its accumulated history entirely: flakiness under some
seeds means the mean was describing a genome we can no longer trust.

**Mutation** (`MutateWith`) fires operators independently per child:

| Operator | p | Effect |
|---|---|---|
| tweakParameter | 0.40 | nudge one numeric param (hand size, min bet, rounds...) |
| flipBool | 0.15 | toggle a rule flag |
| changeEnum | 0.15 | switch a categorical (match rule, draw rule...) |
| addSpecialCard | 0.08 | new rank-triggered effect |
| removeSpecialCard | 0.07 | drop one |
| addBorrowedMechanic | 0.05 | graft a whitelisted cross-skeleton gene |
| removeBorrowedMechanic | 0.05 | excise one |
| mutateScoring | 0.03 | reweight card points / triggers |
| changeSkeleton | 0.02 | re-seat the params on a different skeleton |

then repairs invariants: `hand_size * players <= 52`, and teeth re-applied to
every carried borrow. Repair beats rejection here -- a rejected child wastes a
tournament slot, a repaired one keeps the mutation's intent.

**Crossover** happens for 30% of non-elite slots: two tournament parents,
`hybridCrossover` blends parameters within a skeleton, and with
`-cross-skeleton` enabled it can graft a borrow representing the other
parent's family (the child stays on one skeleton; the foreign parent
contributes a gene, never half a game loop). The child is then mutated too.

**Selection** is elitism plus tournament, ranked not by raw fitness but by
`SharedFitness` -- which is where the interesting machinery lives.

## Novelty: how it avoids re-evolving Crazy Eights forever

Left alone, the GA converges: the fitness function has attractors, and the
attractors are the classics the seeds came from. The hybrid engine counters
with four additive novelty signals, computed per generation, gated hard on
validity (`Valid && TotalFitness >= 0.40` -- a broken game can never be
interestingly weird):

1. **Behavioral k-NN.** Each genome gets a 2D behavior descriptor --
   (MeaningfulDecisions, Interaction) from a 50-game random batch -- and its
   novelty starts as the mean distance to its 15 nearest same-skeleton
   neighbors in population + archive. Within-skeleton only: a rummy variant
   should be unlike other rummies, and gets no credit for being unlike poker.
2. **Seed distance** (`-novelty-select`). Add the distance to the nearest of
   the 11 classic seeds' descriptors. k-NN alone rewards being unlike your
   *current* neighbors, which a population huddled on Crazy Eights satisfies
   by micro-differentiation; this term pays for actually leaving.
3. **Counterfactual integration (CID)**, weight 1.5. For each borrow, re-run
   the same 50-game batch at the same seed with that borrow removed, and
   measure divergence: total-variation of the win distribution (does it change
   who wins?), game-length shift (does it change tempo?), and mean legal-move
   count shift (does it change the move set?). The genome's CID is the *max*
   over its borrows' marginals, so one genuinely deep gene scores high while a
   pile of shallow tallies scores low. Paired common-random-numbers keeps the
   comparison tight. This is the term that makes deep fusion *selected for*
   rather than an accident we hope to spot later.
4. **Judge verdicts**, weight 1.5. An LLM judge periodically reads blind
   dossiers (neutralized rulebook + play traces, no names) for new
   compositions and files verdicts into a table keyed on
   `Composition(genome)` -- the structural identity (skeleton + borrow set),
   so one verdict covers a whole lineage. Certified-novel compositions get a
   novelty boost; certified rediscoveries get suppressed (clamped at zero so
   suppression can't invert the normalization). The chunked
   checkpoint/resume machinery exists for this loop: evolve N generations,
   emit the frontier for judging, append verdicts, resume.

The final selection score blends 50% raw fitness with 50% within-skeleton
normalized novelty, then applies niche sharing across skeletons (underpopulated
families get up to a 3x boost) so six families coexist instead of the easiest
one eating the population. Raw fitness and the blended score are stored in
separate fields (`Fitness` vs `SharedFitness`) after an early bug where
published files showed one and reports showed the other.

Timing matters as much as the terms. An earlier design judged only at round
boundaries and restarted evolution from judged survivors; novelty did not
compound (a 1 -> 2 -> 0 collapse across rounds). Selection pressure needs
generation granularity -- CID and the verdict table push on every selection
event, and that's the version that produced judge-certified novel games under
selection rather than by luck.

## What actually generalizes

Rules of thumb this project paid for:

- **Constrain the representation, never the repair.** Every guarantee that
  started as a filter (playability checks, coherence checks, teeth) got
  cheaper and more reliable when moved into the representation or the
  operators. The grammar's `WellTyped` is the endpoint: illegal games you
  can't even write down.
- **A win-condition *gate* is a termination bug; win-condition *points* are
  fine.** A rummy-style "you may only go out with a full meld" grafted onto
  shedding terminates ~7% of random games. The same idea expressed as
  meld-points over multiple rounds terminates always and reads just as novel.
  Random play must be able to finish the game by accident.
- **The validated novelty recipe** is one move-changing borrow plus a
  multi-round points win condition. Blind judges certified such combos 4/4
  novel; pure scoring tweaks judged 2/2 variant. Three-borrow stacks read
  novel but incoherent -- incentive clash is a real failure mode.
- **Audit the instruments before believing the organisms.** The three worst
  bugs found in review were all measurement bugs wearing discovery costumes:
  dossiers that leaked family names to a "blind" judge, an MCTS whose
  determinization accidentally read opponents' hidden cards (inflating the
  skill tier it gates), and metrics that scored an unmeasured skeleton at
  zero. Selection amplifies whatever the instruments reward, including their
  bugs.

The system's one-sentence summary: make broken games unrepresentable, make
fun measurable enough to rank, and make novelty a selection pressure instead
of a hope.

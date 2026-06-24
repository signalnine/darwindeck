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
5. Full FITNESS parity (the 5 metrics) + judge keying. **NEXT** -- the seam:
   the simulation layer is genuinely generic (the adapter is all it took), but the
   fitness layer is still skeleton-coupled. `fitness.Evaluate` calls `GetRunner(g)`
   (no runner injection), `optionDeltaModeFor` switches on `g.Skeleton` for the
   Interaction metric, and the greedy scorers are per-skeleton and don't understand
   the grammar's state layout. So full parity needs either a `GetRunner` injection
   point (+ a grammar genome it maps to) or generic delta-mode/greedy-scorer
   versions. `SpecGenome` already carries a best-fit skeleton so the existing modes
   approximately apply.

The win is thousands of playable-by-construction families feeding the *same* judge
loop already in use -- the grammar serves the discovery goal, it is not the goal.

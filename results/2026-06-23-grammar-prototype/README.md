# Generative grammar de-risk prototype (2026-06-23)

`pkg/grammar` + `cmd/grammar-proto`. One question: can a game be a composition of
typed primitives run by ONE generic interpreter, and does that composition space
stay playable-by-construction -- or does it rediscover the v1 desert (generative
but mostly garbage)?

Raw run: `run-2026-06-23.txt`.

## The bet

A game = `setup x move-generator x end-condition x scoring` (`GameSpec`, `spec.go`).
One interpreter (`runner.go`) plays any composition. Two structural guarantees:

- **Safety (never stuck):** every move-generator carries an unconditional fallback
  (PlayMatch -> draw/pass, BeatOrPass -> pass, Accumulate -> stick, Capture -> pass),
  so `LegalMoves` is never empty. No dead states.
- **Liveness (always terminates):** a monotone non-increasing progress potential
  (`progressSig` = deck + hands + not-yet-folded, bounded below by 0) plus a
  stalemate rule that ends the game when the potential is flat for `3p+6` turns.
  So even a composition whose own end-condition is unreachable still terminates.

## Result

Across all 114 enumerated specs (20 distinct families), under random AI, 40 trials each:

| Property | Result |
|---|---|
| ever STUCK (empty move set) | **0 / 114** -- safety holds 100% |
| ever hit the 2000-turn cap (non-termination) | **0 / 114** -- liveness holds 100% |
| PLAYABLE (terminates + never stuck + has agency) | 70 / 114 (61%) |
| of which natural-end (own end fires, not stalemate) | 42 |
| stalemate-only (end unreachable) | 28 |

All 4 hand-coded skeletons the grammar covers (shedding, climbing, banking, casino)
reproduce and play. The bet is confirmed: **playable-by-construction holds across
the entire enumerated space.** No v1 desert.

## The typing diagnostic (what drives the next step)

The 28 stalemate-only + 16 agency-dead specs are not noise -- they are the signal
that the enumeration's coherence rules are too loose. The per-family rollup
(bottom of the raw run) shows two clean, physical patterns:

1. **`deck_out` is unreachable for `play_match` and `beat_or_pass`.** Those
   move-gens empty *hands*, not the deck -- the empty-hand->draw fallback refills a
   spent hand, so the deck only drains via the slow stalemate path. `deck_out`
   belongs to `capture`. This accounts for every `*|deck_out` mis-typed family.
2. **`play_match` with rank-only or suit-only matching is agency-dead.** Too few
   legal plays; random play collapses to forced draws (agency ~ 0). Only
   `MatchEither` (Crazy Eights' rank-OR-suit) has real choice in the base grammar.
   Rank/suit-only need a wild-card relaxation to have agency -- which is exactly a
   *modifier* (see below).

Apply both rules and the 20 families collapse to the **8 well-typed families, all
of which survive** (4 canonical + 4 novel):

```
accumulate|bust|closest_target      [canonical: banking]
accumulate|bust|high_score          [novel]
beat_or_pass|empty_hand|first_out   [canonical: climbing]
beat_or_pass|empty_hand|fewest_cards[novel]
capture|deck_out|most_captured      [canonical: casino]
capture|deck_out|high_score         [novel]
play_match/either|empty_hand|first_out    [canonical: shedding]
play_match/either|empty_hand|fewest_cards [novel]
```

That is the grammar's promise made concrete: the type rules are few and physical
(a move-gen determines which end is reachable; match-strictness determines agency),
and tightening them makes illegal compositions *unrepresentable* rather than
caught at runtime. Untyped yields 8/20 good families; typed yields 8/8.

## Honest caveats

- **A 4-move-gen toy yields ~dozens of families, not thousands.** Thousands needs
  (a) the 2 remaining move-gens (trick-taking follow-suit, rummy draw-meld-discard)
  and, mainly, (b) the orthogonal **modifier axis**.
- **The multiplicative lever is modifiers, not move-gens.** v2 already has the
  ingredients: the borrow hooks (meld_bonus, trick_scoring, avoidance, run_play,
  ...). As *typed* modifiers gated against the base spec, each independent modifier
  ~doubles the family count, and -- critically -- a wild-card modifier *rescues*
  the agency-dead rank/suit match families. Modifiers don't only multiply; they
  revive families that are dead in the base grammar.

## Recommendation / next step

Not a full rewrite. Grow v2's borrow system into a typed-composition layer on top
of the generic runner:

1. Lift move-gen x end x score into the typed spec (done here).
2. Tighten the coherence typing per the two rules above (kills the 28 + 16).
3. Port the existing borrow hooks as typed *modifiers* gated against the spec.
4. Wire the existing fitness + judge-in-loop pipeline onto the generic runner so
   the same evolution loop searches the typed, playable-by-construction space.

The win is thousands of playable-by-construction families feeding the *same* judge
loop already in use -- the grammar serves the discovery goal, it is not the goal.

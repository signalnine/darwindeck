# Vying-as-scoring-host: hybrids sweep the leaderboard but judge VARIANT

Date: 2026-06-15

First evolve with vying (poker) hosting the scoring borrows MechMeldBonus /
MechAvoidance (seed 42, pop 400, gen 100). Then blind LLM-judge novelty
assessment, 3 judges/game, with SimplePoker and Casino controls.

## Leaderboard

Vying-scoring hybrids SWEPT the top 6: vying + meld + avoidance, fitness
0.90-0.92, skill 0.63-0.79 -- overtaking every prior pattern (the ~0.89 casino
ceiling). The borrows are outcome-significant in the simulation: they bank into
the chip stacks and decide the chip winner, which is why they rank so high.

## Verdicts

| Game | Composition | Verdict |
|------|-------------|---------|
| A | vying + meld + avoidance (0.921) | **variant 3/3** |
| B | vying + meld (0.920) | **variant 3/3** |
| C | climbing + knock + draw_penalty | variant 2/1 |
| D | casino + meld + avoidance | novel 2/1 |
| E | SimplePoker seed (CONTROL) | known 2/1 |
| F | Casino seed (CONTROL) | variant 3/3 |

Controls calibrated (plain poker = known, Casino = variant), so the verdicts
discriminate.

## The finding: vying scoring borrows are strategically shallow

The vying-scoring hybrids top the leaderboard but judge VARIANT, and the judges
are right. The poker hand is FROZEN at the deal -- there is no draw, discard, or
exchange phase -- so a player has no agency to chase melds or dump penalty cards.
The meld bonus and avoidance penalty are real chip adjustments (not a separate
track; the simulation banks them into the stacks), but the player cannot ACT on
them: the only decision is fold/call/raise, and the meld/heart count is fixed the
moment the cards are dealt. A scoring overlay that does not change the core
decisions reads as a variant -- the exact shallow-borrow problem the deep borrows
(run_play, knock) were built to solve.

LESSON: a borrow is novel only when it changes a DECISION, not just a tally
(the divergence principle, again). On shedding/climbing the deep move/win borrows
change the legal moves or the win trigger. On vying, a scoring overlay on a
frozen hand changes neither -- it only reweights an outcome the player cannot
influence. For vying to produce novel games the borrow must give AGENCY over the
hand (a draw / exchange to pursue melds or shed penalties -- but a draw is
published draw poker) or change the betting structure itself. The scoring-host
direction makes vying a borrow participant and a strong fitness performer, but
not a novelty source.

Also fixed: the rulebook presented the meld/avoidance as "points" separate from
the "most chips wins" condition, so judges read them as inert; writeVyingRules
now states the scored game adjusts CHIPS at showdown.

Raw per-judge verdicts in `blind-judge-verdicts.json`; dossiers game_A.md ..
game_F.md.

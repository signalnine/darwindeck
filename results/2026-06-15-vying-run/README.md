# First vying-inclusive evolve batch: blind novelty judging

Date: 2026-06-15

First full `evolve -cross-skeleton -novelty-select` run (seed 42, pop 400, gen
100) with all SIX skeletons in the pool, including the new vying / betting
(poker) family. Then blind LLM-judge novelty assessment of the top candidates,
3 judges/game, rulebook-only dossiers, with a Casino control.

## Leaderboard

Vying TOPS the run: rank 1 is plain SimplePoker (fit 0.858, skill 0.45). Below
it, casino-scored fusions (0.83-0.84), shedding knock+run_play (0.82-0.83), and
climbing draw_penalty+knock stacks (0.81-0.84). Vying has no borrows and is not a
borrow source, so it only appears as standalone poker (the `meld, vying` tags on
casino games are crossover provenance -- the mechanic is meld, sourced from a
vying parent).

## Verdicts

| Game | Composition | Verdict | Note |
|------|-------------|---------|------|
| A | plain vying (poker) | **known 3/3** | "it simply IS poker" -- the new family is a faithful rediscovery, a calibration anchor, not a novel find |
| B | casino + meld + avoidance | variant 3/3 | passive scoring overlays on a Casino loop |
| C | shedding + knock + run_play | variant 3/3 | Crazy Eights with bolt-ons |
| D | climbing + draw_penalty + knock | **novel 2/3** | see below |
| E | trick + avoidance + meld | variant 2/3 | Hearts rediscovery (the CID trap) |
| F | Casino seed (CONTROL) | variant 3/3 | correct |

## The find: climbing + draw_penalty + knock (novel 2/3)

This confirms and extends the divergence principle from the prior judge pass.
Plain climbing + knock was 3/3 VARIANT: climbing is monotonic (you only shed), so
knock's "fewest cards wins" just tracks "closest to going out" -- redundant with
the empty-hand race, no real new decision.

Adding `draw_penalty` changes that. Face cards force an extra draw, so playing
your strongest cards GROWS your hand in a hand-emptying race. Now "fewest cards
now" diverges from "closest to empty," and knock's declare-to-end becomes a
genuine new decision -- the exact divergence that makes knock novel on Crazy
Eights (which has forced draws). The judges nailed it: "the face-card draw
penalty is anti-thematic and inverted: in a hand-emptying race, playing your
strongest cards makes your hand GROW" + knock fewest-wins = a fusion no published
climbing game has.

PRINCIPLE (confirmed): a win-condition borrow (knock) is novel only where the
host's win DIVERGES from a monotonic shed. Knock is novel on shedding (forced
draws diverge), variant on plain climbing (monotonic), and novel again on
climbing+draw_penalty (the draw penalty re-introduces divergence). Novelty is a
property of the COMBINATION, not the borrow alone.

## Calibration

Both controls land correctly (Casino seed = variant, plain vying = known-poker
at high confidence), so the "novel" verdict on game D means something. Vying
integrated cleanly and is a strong, faithful classic (0.844 in calibration); it
adds a new family and a new borrow-divergence source, not a novel game on its own.

Raw per-judge verdicts in `blind-judge-verdicts.json`; dossiers `game_A.md` ..
`game_F.md`.

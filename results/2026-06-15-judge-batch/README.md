# 2026-06-15 evolve batch: blind novelty judging (mostly variant)

Date: 2026-06-15

A full `evolve -cross-skeleton -novelty-select` run (seed 42, pop 400, gen 100)
with the full borrow set (run_play, follow_suit, knock on shedding+climbing,
casino-as-scoring-host), then blind LLM-judge novelty assessment of the top
candidates. 3 judges/game, rulebook-only dossiers (no fitness, no provenance).
Two controls check calibration; a clean Big-Two+knock-only genome (G) isolates
climbing-knock novelty.

## Verdicts

| Game | Composition | Verdict | Note |
|------|-------------|---------|------|
| A | casino + meld + avoidance | variant (2/1) | meld weakly integrated; flat "all cards = 1pt" penalty ~cancels capture reward |
| B | climbing + knock + draw_penalty | split (1 novel / 1 variant, 2 judges) | knock is a real win-condition change, but stacked draw_penalty grows the hand, fighting the empty-hand goal -- incoherent |
| C | shedding + knock + run_play | variant (3/3) | borrows buried under heavy standard Crazy-Eights command cards |
| D | trick + avoidance + meld | variant (3/3, Hearts) | the documented CID trap: integrated but a Hearts rediscovery |
| E | plain trick (CONTROL) | variant (2 variant / 1 known, Whist) | correct |
| F | Big Two seed (CONTROL) | variant (3/3, Big Two) | correct |
| G | Big Two + knock ONLY | variant (3/3, Big Two) | climbing-knock isolated -- see below |

## What it says

The judges are calibrated: both controls land correctly (Whist, Big Two), so a
"novel" verdict means something. This run's top fusions came back mostly
**variant** -- the judge separating novel from rediscovery, as designed.

The same patterns that were 3/3 **novel** on 2026-06-14 (casino+meld,
shedding+knock+run_play) are **variant** here. The difference is integration
coherence, not the pattern: seed-42's top genomes stack incoherent bolt-ons (a
near-degenerate flat penalty, a hand-growing draw_penalty on top of an
empty-hand race, heavy standard special-card decoration) that read as
recognizable variants. Fitness + CID rank these highly; the judge correctly
declines to call them novel.

**Climbing knock is a variant, not novel (G, 3/3).** Knock generalized cleanly
to climbing mechanically (it works, terminates, carries a skill gradient of
0.4-0.8), but in a pure climbing game you only ever shed, so "fewest cards wins"
via knock just tracks "closest to going out" -- near-redundant with the
empty-hand win, no new move set, no bluff/deadwood tension. Knock's novelty on
shedding came from Crazy Eights' forced-draw dynamic, where your hand can grow,
so a "fewest cards now" snapshot genuinely diverges from "will empty first."
That divergence does not exist in climbing. Lesson: a win-condition borrow is
novel only where it diverges from the host's native win, not merely where it is
mechanically legal.

Raw per-judge verdicts in `blind-judge-verdicts.json`; dossiers `game_A.md` ..
`game_G.md`.

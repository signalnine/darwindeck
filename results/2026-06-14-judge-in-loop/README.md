# Judge-in-the-loop novelty experiment (2026-06-14)

**Question:** can putting the LLM judge *in the selection loop* make novelty
**robust / selected-for**, instead of the incidental novelty waves 1-3 produced?

**Mechanism tested:** a judge-gated restart loop. Each round: evolve with
`-cross-skeleton -novelty-select -seed-dir <prev survivors>`, judge the elite
blind (3 reps/game, controls included), select the games judged
**novel + playable + not-degenerate**, and seed the next round from those
survivors. The LLM judge is the selection operator at each round boundary.
(The judge has no in-process credential, so it cannot run per-generation; this
tests it at round granularity.)

## Result: it does NOT reliably compound novelty

| Round | Seeded from | Novel + playable survivors |
|-------|-------------|----------------------------|
| 1 | wave-1's 4 novel games | **1** (shedding scored-by-tricks hybrid) |
| 2 | round-1 survivor | **2** (a trick-taker with conflicting per-trick + Hearts-penalty + meld-bonus scoring; a shedding hybrid — *different lineages*) |
| 3 | the 3 cumulative novels | **0** (collapsed to Whist / Crazy-Eights / Gin variants) |

Trajectory **1 → 2 → 0**: it grew, then collapsed. Controls (Gin Rummy, Crazy
Eights) were correctly flagged as rediscoveries in all three rounds, so the
counts are trustworthy.

**Why round 3 collapsed — the real finding.** The in-round evolution fitness
rewards playability (decision density, arc, interaction, skill, session length),
**not novelty**. The novel hybrids the judge selects are cross-mechanic games
with *muddled* scoring (e.g. "win tricks" pulling against "avoid hearts"), which
makes them *lower* fitness than a clean Whist clone. So within a round the GA
erodes the seeded novelty faster than the between-round judge gate can
accumulate it. Round-*boundary* judge selection is too coarse to win that tug
of war.

**The honest conclusion.** The cross-skeleton machinery genuinely *produces*
judge-certified novel playable games (wave 1: 4; this experiment: 3 more across
rounds 1-2, several distinct lineages, all in `games/`). But producing them
**reliably / compounding** is still unsolved. Novelty pressure has to live at
**generation granularity** — a strong novelty term, or the LLM judge itself,
*inside* the per-generation fitness/selection, not just at round boundaries.
That is the heavier, still-unbuilt lever (the judge-per-generation version is
expensive: an LLM call per elite per generation).

## The cumulative novel games found (judge-certified, all playable, non-degenerate)

`games/` holds 3 distinct novel hybrids surfaced by the judge across rounds 1-2,
on top of wave-1's 4 (`results/2026-06-13-novel-games`). All are
**borderline-quality** — genuinely novel cross-mechanic combinations the judge
declined to map to any single published game, but with coherence problems
(conflicting scoring layers). None reached the publishable-novel tier that
wave-1's G14 hit. So: more novelty and more diverse lineages than wave 1, but
lower coherence — quality/coherence of novel hybrids is the frontier alongside
reliable selection.

## What shipped to master from this line of work

- `evolve -seed-dir <dir>`: seed a run from a custom genome set (augments the
  classic pool). The capability the restart loop needs; reusable.
- `pkg/judge` dossier neutralizer now strips the climbing game-name
  parenthetical (4th-skeleton blind-judging fix).

The experiment itself was driven manually (Bash evolve/emit/select + a
judge-only workflow per round) because the orchestration harness intermittently
failed on bash-running structured-output agents; the judge agents themselves
were reliable.

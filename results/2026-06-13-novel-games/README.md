# Novel playable games (2026-06-13)

This bundle holds the first **judge-certified novel playable card games** DarwinDeck
has produced by evolution. "Novel" here means: a card-game designer LLM judge,
run blind (no metric scores, no source labels) and validated against controls,
classified the game as a genuinely novel design — *not* a faithful rediscovery
of a known published game — while also rating it playable and not degenerate.

## The result

Goal: **evolve novel playable card games.** Achieved — narrowly and via cross-skeleton
recombination. Four games cleared the bar (majority of 3 blind judge reps: novelty=novel
AND playable AND quality != degenerate), from the wave-1 run
(`output/2026-06-13-novelty-w1`, `-cross-skeleton`, seed 7). The judge's controls
in that run passed (Crazy Eights and Gin Rummy were correctly flagged as
rediscoveries), so the novel count is not a leniency artifact.

All four are one lineage: a **trick-taking core grafted with rummy-style meld
bonuses *and* shedding/Hearts-style avoidance penalties** — a three-way
cross-family combination that maps to no single published card game.

### The strongest: `rank12_gen50_17399` (unanimous 3/3 novel + 3/3 publishable)

- Skeleton: trick-taking (trump, must-follow-suit), 2 players, hand 8, 5 rounds.
- Borrowed mechanics: `{Rummy, MeldBonus}` (set/run bonuses on captured cards)
  + `{Shedding, Avoidance}` (points-are-bad penalty scoring).
- Judge (rep 3, verbatim): *"Sound contested trump trick-taker (strict P0/P1
  alternation, 100% completion, both players win tricks and winners differ
  across games) that grafts rummy-style set/run bonuses and Hearts-style penalty
  cards plus contract scoring onto the trick core, a combination that matches no
  single published game."*
- report.md metrics: decision density 0.37, interaction 1.00, skill gradient
  0.54, veto-stable 5/5.

The other three (`rank11`, `rank16`, `rank18`) are the same recipe at different
player counts / hand sizes / trump rules; each was 2/3 novel.

## The honest limitation

The novelty was **incidental, not selected-for.** The evolution fitness rewards
playability proxies (decision density, arc, interaction, skill, session length),
never novelty — so these four games slipped through rather than being the search's
target, and the same run produced 26 rediscoveries.

Two deliberate attempts to make novelty **robust** both produced **zero** novel games:

| Wave | Lever | Novel playable games |
|------|-------|----------------------|
| 1 | cross-skeleton recombination (no novelty selection) | **4** (1 lineage, 1 publishable) |
| 2 | + seed-aware novelty selection | 0 — vestigial borrows: a hybrid carried a borrow in its genome but *played* like a classic, so the behavioral novelty signal saw nothing to push toward, and the population collapsed back to Whist clones |
| 3 | + borrow "teeth" (borrows made outcome-significant) + climbing skeleton | 0 — borrows now genuinely fire and decide outcomes (verified: winner-flips 12-49/60), but the judge still reads "shedding scored by tricks" as a known family, and aggressive borrows go degenerate, not novel |

**Lesson:** the cheap structural/behavioral novelty signal (distance from seeds
in decision-density × interaction space) cannot drive the search to genuine
novelty. The only signal that reliably distinguishes novel from rediscovery is
the LLM judge (validated separately, see `results/2026-06-13-llm-judge-tryout`).
The clear next lever is **the LLM judge *in* the selection loop** (the F.1
top-decile slot) so the GA is rewarded for what a designer actually perceives as
novel — not yet built (it needs orchestrated Go-evolution / agent-judging
alternation, since the engine has no in-process LLM credential).

## Machinery added this session (all on master, additive, gate-green)

- `-cross-skeleton`: cross-family recombination producing hybrids.
- `-novelty-select`: seed-aware novelty in the hybrid algorithm.
- A 4th skeleton: **climbing / ladder** (Big Two family), playable by construction.
- Cross-family borrow "teeth": borrows made outcome-significant.

None of these touched the frozen playability metric stack (weights, vetoes,
calibration gate all unchanged).

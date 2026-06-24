# Judge-in-loop: operating procedure

The LLM judge is the only signal that separates a novel game from a rediscovery
of a published game outside the 11-seed set (every structural signal -- k-NN
behavior distance, seed distance, CID -- anchors on the seeds). This doc is how
to run it inside evolution. Mechanism and validation:
`results/2026-06-15-judge-in-loop-v2/`, `results/2026-06-16-judge-in-loop-modes/`.

## Model policy: Sonnet in-loop, Opus to certify

**Use Sonnet (4.6) for the in-loop chunk judging. Reserve Opus (4.8) for final
certification of the published top-N.**

A controlled check (3 Sonnet + 3 Opus judges on the same 6 dossiers spanning
known / variant / novel, `results/2026-06-16-judge-in-loop-modes/` notes):

| dossier | Sonnet (3) | Opus (3) | |
|---------|-----------|----------|--|
| plain poker | known 3/3 | known 3/3 | match |
| vying+meld | variant 3/3 | variant 3/3 | match |
| vying+meld+avoidance | variant 3/3 | variant 3/3 | match |
| casino+meld+avoidance | variant 3/3 | variant 3/3 | match |
| climb+knock+draw_penalty | variant 3/3 | novel 2/1 | adjacent |
| Casino seed | known 3/3 | variant 3/3 | adjacent |

4/6 exact majority, 6/6 within one category, ZERO known<->novel flips. The two
disagreements are the genuinely-borderline cases, and Sonnet is the MORE
CONSERVATIVE one -- the safe direction for an in-loop SUPPRESSION signal (it will
not over-promote a borderline game into the published top). Sonnet's closest-game
IDs were often sharper than Opus's (it named Chinese Poker / Pusoy / Teen Patti /
Guts where Opus said generic "Straight Poker"). The composition-keying means a
single noisy verdict only mis-steers one lineage, so the loop tolerates Sonnet's
slight conservatism. Opus's more generous borderline read is worth its cost only
on the final published top-N.

The earlier finding that LOCAL models under-credit novelty does NOT extend to
Sonnet; do not substitute a local model for the in-loop judge without re-running
this agreement check.

## The three pieces (`pkg/evolution`, `cmd/darwindeck`)

- **Selection** -- `computeNovelty` adds `JudgeWeight(1.5) * verdict(Composition(g))`,
  behind the Valid+FitnessFloor gate, keyed by `Composition` (skeleton + sorted
  borrow-mechanic set, e.g. `0:1,8`). Novel > 0 explores more, rediscovery < 0 is
  starved (clamped at novelty 0).
- **Compounding** -- chunked checkpoint/resume (`-checkpoint`, `-chunk`) carries
  the WHOLE population across processes, so the verdict table can grow between
  chunks and the pressure compounds (the elite-only `-seed-dir` restart loop went
  1->2->0 because it lost the population).
- **Publication** -- the leaderboard ranks by `OutputRank + 0.2*verdict`, so a
  certified-novel game surfaces above a higher-fitness rediscovery (a shedding
  knock-alone game at fitness 0.72 reached rank 9 on its +1.0 verdict).

Everything is byte-identical with no verdict table loaded.

## The verdict table (distillation)

The judge's input is `Composition(g)` (skeleton + sorted borrow set) -- a small
finite categorical space, so rather than train a local model to approximate the
judge, you label that space with the teacher once and read it back as a lookup.
`results/2026-06-18-complete-verdict-table/verdicts.json` labels the compositions
the search has produced so far (44 entries: ~35 of the 66 reachable + 9 legacy;
5 novel, 33 variant, 6 known). Loaded as `-judge-verdicts`, a labeled composition
is a **zero-cost lookup** -- no LLM call. It is NOT complete over the full 66
reachable space (31 mostly-shedding compositions are unlabeled), but that is fine
by design: a **miss is neutral** (`JudgeVerdicts[composition]` returns 0, so an
unseen composition gets a 0 judge term, not an error), and `judge backfill`
labels whatever a run actually produces.

So the table self-heals for explored compositions rather than statically covering
all 66. Run `darwindeck judge backfill -table <verdicts.json> -dir <run-output>
-out <dossiers>` at a chunk boundary: it emits a blind dossier per composition
present in the run but absent from the table; judge those (the workflow below),
append `composition -> score`. Adding a skeleton or a borrow opens new
compositions to backfill.

## The loop (one chunk)

```bash
# 1. run a chunk: resume the checkpoint, run -chunk gens, emit the top genomes, exit
darwindeck evolve -algorithm hybrid -cross-skeleton -novelty-select \
  -population 250 -generations 60 -chunk 20 -mcts-decile 0 \
  -judge-verdicts run/verdicts.json -checkpoint run/ck.json -emit-dir run/queue \
  -output run/out

# 2. build blind dossiers for the emitted top genomes (real tool, not a throwaway)
darwindeck judge emit run/queue/gen020 --out run/dossiers/gen020

# 3. judge the NEW compositions with Sonnet (3-judge majority) -- see the workflow below.
#    Append { "<composition>": <score> } to run/verdicts.json:
#      clear variant/known -> negative (-0.5 .. -1.0), clear novel -> positive (+0.5 .. +1.0),
#      split panels -> small magnitude (+/-0.3). Composition = "<skeleton>:<sorted mech ids>".

# 4. relaunch step 1 with the same -checkpoint to resume; the grown table takes effect.
#    Repeat until the run completes (it writes run/out, judge-ranked, on the last chunk).
```

Start the table empty (`{}`) for a DISCOVERY run (the search surfaces compositions
and the judge classifies them from scratch) or from a prior table for an
EXPLOITATION run (it pre-steers onto known-novel). Both are the system working;
see `results/2026-06-16-judge-in-loop-modes/`.

## The Sonnet in-loop judge (canonical workflow)

Invoke via the Workflow tool. `model: 'sonnet'` is the wired-in default for the
loop; only set `model: 'opus'` for the final-certification pass on the published
top-N. A copy lives at `.claude/workflows/judge-novelty.js` (invoke by name).

```js
// args: { dir: "run/dossiers/gen020", ids: ["g001","g002",...], model: "sonnet" }
export const meta = {
  name: 'judge-novelty',
  description: 'Blind novelty judge (Sonnet in-loop default), 3-judge majority per dossier',
  phases: [{ title: 'Judge' }],
}
const model = (args && args.model) || 'sonnet'   // Sonnet in-loop; pass "opus" to certify
const SCHEMA = {
  type: 'object', additionalProperties: false,
  required: ['verdict', 'closest_game'],
  properties: {
    verdict: { type: 'string', enum: ['known', 'variant', 'novel'] },
    closest_game: { type: 'string' },
  },
}
const prompt = (id) =>
  `Card-game historian and designer. Read the dossier at ${args.dir}/${id}.md (Read tool). ` +
  `Is this a NOVEL card game or a rediscovery of an existing published game? ` +
  `verdict: "known" (essentially IS a published game), "variant" (a known game with minor ` +
  `tweaks, keeps its core identity), or "novel" (a combination of mechanics not found ` +
  `together in any published card game). Tough, well-read skeptic: a scoring overlay that ` +
  `does not change the core decisions is a variant, not novel. Name the closest published game.`

phase('Judge')
const out = await parallel(args.ids.map(id => () =>
  parallel([1, 2, 3].map(j => () =>
    agent(prompt(id), { label: `${model}:${id}#${j}`, phase: 'Judge', schema: SCHEMA, model })
  )).then(v => ({ id, verdicts: v.filter(Boolean) }))
))
return out.filter(Boolean)
```

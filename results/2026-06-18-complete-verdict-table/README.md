# Judge verdict table (composition-keyed), 2026-06-18

`verdicts.json` is the composition -> score table the in-loop judge term reads
(`evolve -judge-verdicts`). The judge's input is `Composition(g)` =
`<skeleton> : <sorted borrow-mechanic ids>`, a small finite categorical space, so
the "distillation" here is to label that space with the LLM teacher once and read
it back as a lookup -- no model, no per-generation LLM call. A 4B fine-tune would
be a slower, noisier approximation of a lookup you can just build.

## Reachable vs labeled (read this before trusting "complete")

The reachable space is **66 compositions** under the current rules (per-skeleton
borrow whitelist in `pkg/genome/validate.go`, max 3 borrows):

| skeleton | whitelist | reachable |
|----------|-----------|-----------|
| shedding | 6 | 42 |
| rummy    | 3 | 8  |
| trick / climbing / casino / vying | 2 each | 4 each |
| **total** | | **66** |

This table has **44 entries: ~35 of the reachable 66, plus 9 legacy** that use
Trump/PlayMultiple (mechanics removed from the whitelist 2026-06-11; they can no
longer occur). So it is **NOT complete over the reachable space** -- **31
reachable compositions (almost all shedding) are still unlabeled.** They are
simply ones evolution hasn't produced in the 738-genome corpus yet.

This is fine operationally, by design: the judge only queries compositions that
appear in a population, and a **miss is neutral** -- `JudgeVerdicts[composition]`
returns 0, so an unlabeled composition gets a 0 (neither novel nor penalized)
judge term, not an error. `judge backfill` then labels whatever a run actually
produces. So the table is **self-healing for explored compositions**, not a
static lock over all 66. To eliminate even first-contact misses you would
pre-label all 66 (synthesize a genome per missing composition), but most of the
31 will be variant and the search backfills them on contact, so it is low value.

## How it was built

- 42 distinct compositions appear across the 738 evolved genomes in `output/` +
  `results/` (9 of them legacy). 17 were already labeled by prior judge runs.
- The remaining 27 observed-and-unlabeled were labeled 2026-06-18 with the Sonnet
  teacher, 3-judge majority per blind dossier (`judge emit` -> judge-novelty
  workflow). All 27 came back **variant** -- the novel recipes (e.g. `0:1,8`)
  were already in the 17. `backfill-raw-2026-06-18.json` is the per-composition
  audit (3 votes + closest published game).
- Merge: 17 + 27 = 44 entries. Score scale: novel `+0.5..+1.0`, variant
  `-0.5..-0.6`, known `-0.8..-1.0`.

## What the labels say

Of 44 labeled: **5 novel, 33 variant, 6 known.** The novel frontier is small --
most of what the search produces is a rediscovery, which is why a semantic judge
(now a table) is the only thing that separates the 5. The recipe holds: `0:8`
(run_play alone) variant, `0:1,8` (run_play + multi-round meld points) novel,
`0:1,5,8` (3-borrow stack) back to variant.

## Use it

```bash
evolve -algorithm hybrid -novelty-select \
  -judge-verdicts results/2026-06-18-complete-verdict-table/verdicts.json ...
```

## Keep it complete (for what actually occurs)

Run at a chunk boundary (or after any run) to label compositions the search
produced that aren't in the table yet -- and after adding a skeleton or borrow,
which opens new compositions:

```bash
darwindeck judge backfill -table results/2026-06-18-complete-verdict-table/verdicts.json \
  -dir <run-output> -out /tmp/new-dossiers
# -> emits a blind dossier per unlabeled composition; judge them with the
#    judge-novelty workflow, then append composition->score to verdicts.json.
```

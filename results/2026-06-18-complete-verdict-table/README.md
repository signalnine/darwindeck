# Complete verdict table (judge distillation), 2026-06-18

`verdicts.json` is a **complete** composition -> score table: every novelty
composition the search can currently reach is labeled. With it loaded, the
in-loop judge term (`evolve -judge-verdicts`) is a **zero-cost lookup** -- no LLM
call per generation. That is the whole "distillation" here: the judge's input is
`Composition(g)` = `<skeleton> : <sorted borrow-mechanic ids>`, a small finite
categorical space, so completing the lookup beats training a model to
approximate it (a 4B fine-tune would be a slower, noisier version of a 44-row
table).

## How it was built

- The in-loop judge keys on composition (`pkg/evolution` `Composition`), which
  drops params -- two genomes with the same skeleton+borrow-set share a verdict.
- 42 distinct compositions are present across the 738 evolved genomes in
  `output/` + `results/`. 17 were already labeled by prior judge runs.
- The remaining 27 were labeled on 2026-06-18 with the Sonnet teacher, 3-judge
  majority per blind dossier (`judge emit` -> judge-novelty workflow). All 27
  came back **variant** -- the novel recipes (e.g. `0:1,8`) were already in the
  17. `backfill-raw-2026-06-18.json` is the per-composition audit (3 votes +
  closest published game).
- Merge: 17 + 27 = 44 entries (a few labeled compositions no longer have a live
  genome, hence 44 > 42). Score scale: novel `+0.5..+1.0`, variant `-0.5..-0.6`,
  known `-0.8..-1.0`.

## What the table says

Of 44 labeled compositions: **5 novel, 33 variant, 6 known.** The novel frontier
is small -- the overwhelming majority of what the search produces is a
rediscovery, which is exactly why the LLM judge (and now this table) is the only
thing that can separate the 5 from the rest. Notable: `0:8` (run_play alone) is
variant, `0:1,8` (run_play + multi-round meld points) is novel, `0:1,5,8` (the
3-borrow stack) falls back to variant -- the recipe holds.

## Use it

```bash
evolve -algorithm hybrid -novelty-select \
  -judge-verdicts results/2026-06-18-complete-verdict-table/verdicts.json ...
```

## Keep it complete

The table is complete only w.r.t. the current skeletons and borrow whitelist.
Add a skeleton or a borrow and new compositions open up. To find and label them:

```bash
darwindeck judge backfill -table results/2026-06-18-complete-verdict-table/verdicts.json \
  -dir <new-run-output> -out /tmp/new-dossiers
# -> emits a blind dossier per unlabeled composition; judge them with the
#    judge-novelty workflow, then append composition->score to verdicts.json.
```

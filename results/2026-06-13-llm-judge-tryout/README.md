# LLM-Judge Try-Out (2026-06-13)

Blind LLM-as-judge run over 15 neutralized card-game dossiers, validating the
**termination-decoupling fix** to the judge dossier format.

## What the tool is

`darwindeck judge` (in `pkg/judge/`, CLI in `cmd/darwindeck/judge.go`) is a
two-stage LLM-as-judge harness:

- `judge emit <input> --out <dir>` builds **blind** dossiers: one per genome,
  each containing only a neutral-titled rulebook, two greedy-vs-greedy sample
  traces, and a `Termination` section. Game names, sources, R4 ranks, and numeric
  fitness scores are stripped. A PRIVATE `answer-key.json` is written OUTSIDE the
  dossier dir so the judge stays blind.
- `judge rank <dossier-dir> <verdicts.json> --out <report.md>` ingests the
  judge's verdicts (3 reps per game), aggregates **majority-of-3** per id,
  applies a **rediscovery demotion** (majority `variant_of_known` drops one
  quality band and is labeled with the classic family), re-ranks by judged
  quality, and writes `judged-report.md` + `judged.json`.

Aggregation rules: quality tie -> worse band (conservative); novelty tie ->
`variant_of_known` (a rediscovery suspicion is not dropped on a split); playable
tie -> playable (do not condemn on a split); confidence is the mean.

## The termination fix being validated

In prior validation, completion rate (how FAST greedy AIs finish) was conflated
with whether a win is reachable BY THE RULES. Slow-but-sound rummy games were
condemned as degenerate. The fix adds a **Termination** section to each dossier
that separates the two: it reports standard-cap and 4x-extended-cap completion
AND whether the win condition became legally reachable (go-out legal, hand
emptied, rounds completed). The rubric (`dossiers/prompt.md`) instructs the judge
to call a game degenerate ONLY for (a) no reachable terminal state or (b) a
one-sided runaway visible in the traces -- NOT for low AI completion alone.

## This run

- 15 dossiers (`dossiers/`): 11 R4 champions (7 shedding, 2 trick, 2 rummy),
  3 classics (Crazy Eights, Gin Rummy, Knock Rummy), 1 degenerate fixture
  (WildUnionShedding).
- 45 verdicts (`verdicts.json`), 3 reps each.
- Output: `judged-report.md` (ranked table) + `judged.json`.

Reproduce:

```bash
make build-v2
./bin/darwindeck judge rank results/2026-06-13-llm-judge-tryout/dossiers \
    results/2026-06-13-llm-judge-tryout/verdicts.json \
    --out /tmp/judged-report.md
```

## Tool fix landed in this branch

The verdict ingestion shape (`Verdict.Confidence`) only accepted a JSON number,
but an LLM judge naturally emits a qualitative label (`"high"`/`"medium"`/
`"low"`). Running `judge rank` on label-form verdicts failed with:

```
parse verdicts: json: cannot unmarshal string into Go struct field Verdict.confidence of type float64
```

Fix (`pkg/judge/rank.go`): a custom `Verdict.UnmarshalJSON` now accepts
`confidence` as either a number (passthrough) or a string label
(low=0.3, medium=0.6, high=0.9), case-insensitive; unknown/empty -> 0 so one
malformed field cannot reject a whole batch. Covered by
`TestVerdictConfidenceAcceptsStringOrNumber`.

## Result: termination fix WORKED

KEY TEST -- the two prior FALSE POSITIVES cleared:

| Game | Prior validation | This run (majority verdict) | Final band |
|------|------------------|-----------------------------|------------|
| G13 Gin Rummy | 3/3 degenerate | 3/3 **publishable**, playable=true | borderline* |
| G14 Knock Rummy | 3/3 degenerate | **publishable** (2 pub / 1 borderline), playable=true | borderline* |

*Their final band is borderline only because of the rediscovery demotion
(publishable -> borderline for being Gin/Knock-Rummy variants), NOT because of
degeneracy. Both are judged playable and sound. Each reason explicitly cites the
Termination section ("the low AI completion is mere slowness, not degeneracy").

Degeneracy detection stayed sharp -- the negative control was still caught:

- G15 WildUnionShedding: 3/3 **degenerate** (raw judge call), stays degenerate.
  The fix did not blind the judge to real degeneracy.

Rediscoveries still flagged with correct families:

- Crazy Eights: G12 (classic) + shedding champions G01-G07, G15.
- Gin / Knock Rummy: G13, G14 (classics) + rummy champions G10, G11.
- Whist / Spades: trick champions G08, G09.

## Re-ranking vs original R4 greedy-fitness order

Judge order among the 11 champions vs their R4 greedy-fitness rank:

- **G08** (R4 #11, trick): rises to judge **#2** -- the only champion the judge
  rated genuinely sound (publishable -> borderline after rediscovery demotion).
  Biggest mover up.
- **G01** (R4 #1, top greedy-fitness shedding champion): falls to judge **#9**.
  The judge demoted it: borderline raw quality, then rediscovery-demoted to
  degenerate for being an over-wilded Crazy Eights clone. Biggest mover down.
- The two classics that were prior false positives, **G13/G14**, sit at judge
  **#1 and #3** -- above every shedding champion.
- Shedding champions G01-G07 collapse into the bottom (all borderline -> demoted
  to degenerate via the rediscovery cascade: ~37-50% of the deck wild). Greedy
  fitness ranked them top; the judge ranks them last.

Net: the judge inverts the greedy-fitness ordering. Greedy fitness rewarded the
over-wilded shedding champions; the judge sees their thin decision surface,
flags them as Crazy Eights rediscoveries, and elevates the one trick game with
real follow-suit decisions plus the two cleared rummy classics.

## Verdict

**fix_worked_ship_it.** The termination-decoupled dossiers cleared both prior
false positives (Gin/Knock Rummy now playable + not-degenerate, citing the
Termination section), the negative control (WildUnionShedding) is still caught as
degenerate, and rediscoveries are correctly named. One tool bug (string
confidence) was fixed to ingest the natural LLM verdict shape.

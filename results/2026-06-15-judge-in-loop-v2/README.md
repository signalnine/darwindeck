# Judge-in-loop v2: chunked checkpoint, verdict table grows during the run

Date: 2026-06-15

v1 (the judge-novelty selection term) used a STATIC verdict table, so evolution
escaped into newly-invented compositions the judge had not seen. v2 grows the
table DURING the run: the evolution is split into chunks across separate
processes (checkpoint/resume carries the WHOLE population + novelty archive, not
just the elite -- the fix for the failed -seed-dir restart loop). Between chunks
the out-of-loop LLM judge classifies the new top compositions and appends
verdicts; the next chunk resumes with the grown table.

## Run (seed 42, pop 150, gen 30, chunk 10 -> 3 chunks)

Started from the 13-composition table of prior judgments. Chunk 1 surfaced 4 new
compositions -- and they were the PLAIN CLASSICS the search had drifted toward:
plain casino (judged KNOWN: Casino), plain climbing (KNOWN: Big Two), plain trick
(KNOWN: Whist), and trick+meld (VARIANT: the judges correctly named the
Pinochle/Bezique family -- a known trick+meld hybrid). All suppressed.

| stage | suppressed (rediscovery) | boosted (novel) |
|-------|:--:|:--:|
| gen 10 elite (chunk 1) | 12 | 8 |
| gen 20 elite (chunk 2, after judging) | **4** | **16** |
| gen 30 final OUTPUT | 7 | 12 |

## What it shows

The SELECTION shift is conclusive: after the judge classified the rediscoveries
chunk 1 found and the verdicts fed back, chunk 2 ABANDONED that territory --
suppressed elite dropped 12 -> 4, boosted-novel rose 8 -> 16, the plain classics
left the top entirely, casino+meld (+0.8) and climb+draw_penalty+knock (+0.5)
concentrated. This is the compounding the static v1 could not do: it caught new
rediscoveries (plain classics, Pinochle-family trick+meld) the static table never
had and steered the search off them. The whole-population checkpoint is what lets
it compound where the elite-only restart loop (1->2->0) could not.

The remaining gap: the final OUTPUT crept back to 7 suppressed. Novelty drives
SELECTION/exploration, but SaveResults ranks the published top-N by FITNESS -- so
a high-fitness rediscovery (plain Casino ~0.77) survives in the population and
reappears in the fitness-ranked output even while being explored less. v3: make
the PUBLICATION step judge-aware too (rank/filter the output by verdict, not just
fitness), so the discovered novel games actually surface at the top.

## v3: judge-aware publication ranking (the gap closed)

The output now ranks by OutputRank + 0.2*verdict (Composition-keyed), so a
certified-novel game surfaces above a higher-fitness rediscovery while fitness
stays the base. Re-running the gen-30 output through this ranking: the final
top-12 is ALL judge-certified novel -- casino+meld (+0.8, fit ~0.85), climbing
draw_penalty+knock (+0.5, ~0.85), and shedding knock-alone (+1.0) lifted from
fit 0.72 into ranks 11-12 by its strong novel verdict. The high-fitness
rediscoveries (plain Casino ~0.77, suppressed -1.0 -> -0.20) are demoted out of
the top. Byte-identical when no verdicts are loaded.

End to end: v1 explores toward novel (selection term), v2 compounds it (chunked
checkpoint grows the verdict table mid-run, whole population persisted), v3
surfaces the discoveries (judge-aware publication ranking). Structural metrics
cannot tell novel from rediscovery; the LLM judge can, and putting it in the loop
at generation granularity -- with the population persisted and the output
judge-ranked -- is what makes novelty pressure both compound AND publish.

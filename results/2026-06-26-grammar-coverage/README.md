# Card-game corpus coverage of the grammar (2026-06-26)

Goal: represent MOST known card games with the playable-by-construction grammar.
This is the survey that maps the corpus against the grammar's primitives and
ranks the gaps by how many games each unlocks. Raw: `corpus-survey.json` (60
games across 8 families, 8 expert agents + a synthesis, via a Workflow).

## GOAL REACHED: most known card games are representable

**Final in-scope coverage = 21/26 = 80.8% (covered + partial); 11 games FULLY
covered; all 7 major families represented.** Trajectory across this session:
**14% -> 67% -> 80.8%.** Measured by three blind expert-agent surveys
(`coverage-final.json`).

The grammar now has **7 move-generators** -- play_match, beat_or_pass, accumulate,
capture, trick, rummy, and **vying (poker)** -- and **15 modifiers**: run_play
(shedding + climbing combos), follow_suit, draw_penalty, knock (shedding/climbing
fewest-cards + rummy Gin go-out), meld_bonus, avoidance (Hearts), trump (Spades),
bid (contract), teams (2v2), skip, force_draw, reverse (the Uno set), nominate
(Crazy Eights), wild (rummy melds), sum_capture (Scopa building). Base families 7,
**modified families 137 (22.8x), all 137/137 playable-by-construction** (0 stuck,
0 non-terminating); MAP-Elites illuminates 133/137. The new modifiers were
adversarially reviewed (safety/distinctness/fidelity/legibility) -- 0 confirmed bugs.

FULLY COVERED (11): Crazy Eights, Mau-Mau, Switch, Last Card (shedding); Whist,
Spades, Hearts (trick); Casino, Scopa, Cassino, Thirty-One (fishing/banking).
PARTIAL (10, core decision present, a secondary rule short): Uno (108-card deck),
Big Two / President / Tien Len (poker-combos/finish-order), Oh Hell, Gin / Knock
Rummy (draw-from-discard, multi-round), Blackjack (banker/soft-ace), Five-Card Draw
Poker, Three-Card Brag.
NOT COVERED (5): Euchre / Napoleon / Bridge-play (bid-named trump, bowers, dummy),
Texas Hold'em / Stud (community board / up-cards) -- each needs a distinct,
non-shared primitive, so no single add recovers them.

The original key finding held: the two MOVE-GENERATORS (bidding-as-a-trick-phase
and vying) were the gates, not modifiers -- both now built, both terminate by
construction (a finite bid round; a max-raises cap).

## The gap roadmap (ranked by games unlocked)

| # | gap | type | unlocks | playable-by-construction? |
|---|-----|------|---------|---------------------------|
| 1 | **Bidding / contract phase** (declare a target, score make-or-set / closeness) | move-gen | **11** -- Spades, Oh Hell, Euchre, 500, Skat, Bridge(play), Napoleon, Rook, Pinochle, Klaberjass | yes: a one-shot finite bid round terminates trivially; contract score is an end tally, not a gate |
| 2 | **Vying / betting** (check/bet/call/raise/fold over a pot, hidden-hand showdown) | move-gen | **8** -- Poker family, Brag, Blackjack-as-betting, Baccarat | yes WITH a max-raises cap per round (mirrors v2 max_raises); needs hidden-hand + hand-rank score |
| 3 | **Multi-card combo-beat on climbing** (pair beats pair, run beats run) | move-gen | **6** -- Big Two, President, Tien Len, Daihinmin, ... | yes: a strictly-higher same-type superset; pass always legal; combos shed faster |
| 4 | **Suit-nomination** (a wild rank lets you CHOOSE the next required suit) | modifier | **6** -- Crazy Eights, Uno, Mau-Mau, Switch, Whot, Last Card | yes: the draw fallback survives, no deadlock |
| 5 | force-opponent-draw (Uno draw-two) | modifier | 5 | **DONE 2026-06-26** |
| 6 | **Pip-sum capture** (capture a table subset summing to the played card) | modifier | **5** -- Casino, Scopa, Cassino, Cuarenta, Sweep | yes; already exists as AllowSumCapture in pkg/skeleton/casino -- needs the explicit capture-target enumeration in the grammar's coarse capture gen |
| 7 | partnerships / team scoring | score-rule | 6 (cross-cuts bidding) | yes: pure seat->team aggregation |
| 8 | in-hand declaration melds (marriages/bezique) | score-rule | 5 (gated by bidding) | yes: end-of-trick tally on the held hand |
| 9 | trump-NAMING via bid + bowers | modifier | 5 (gated by bidding) | yes, once bidding exists |

Out-of-scope by design (~10 games): real-time/dexterity (Spit, Egyptian Ratscrew),
no-decision feeds (War, Beggar-my-Neighbour), and multi-primitive stacks (Tichu,
Canasta) that pile several independent gaps at once.

## What this says about the goal

To reach "most known card games," the order is clear: **(1) the bidding/contract
move-gen (+11) and (3) climbing combo-beat (+6) are the cleanest big unlocks and
both stay playable-by-construction by construction; (2) the vying/betting move-gen
(+8) is the next, needing a max-raises cap + hand-rank showdown.** The remaining
clean modifiers (suit-nomination +6, pip-sum capture +5, teams +6) are cheaper and
fill the shedding/fishing/partnership tails. The hard limit is intrinsic: a handful
of games are real-time or no-decision and don't belong in a turn-based agency
grammar at all.

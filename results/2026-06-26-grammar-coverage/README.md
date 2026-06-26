# Card-game corpus coverage of the grammar (2026-06-26)

Goal: represent MOST known card games with the playable-by-construction grammar.
This is the survey that maps the corpus against the grammar's primitives and
ranks the gaps by how many games each unlocks. Raw: `corpus-survey.json` (60
games across 8 families, 8 expert agents + a synthesis, via a Workflow).

## Where we are

The grammar now has **6 move-generators** (play_match, beat_or_pass, accumulate,
capture, trick, rummy) and **11 productive modifiers**: run_play, follow_suit,
draw_penalty, knock, meld_bonus, avoidance, trump, skip, force_draw, wild(rummy),
gin-knock. Base families 6, **modified families 61 (10.2x), all 61/61
playable-by-construction** (0 stuck, 0 non-terminating). The five newest modifiers
(trump/skip/force_draw/wild-rummy/gin-knock) were adversarially reviewed across
safety/distinctness/fidelity/legibility -- **0 confirmed bugs**.

Honest coverage: of ~42 in-scope multiplayer games surveyed, the grammar covers or
nearly-covers a modest slice today (Whist, Thirty-One, Crazy Eights, Hearts, Gin /
Knock Rummy, plus Spades-without-bidding, Uno-ish, Big-Two-singles). **The ceiling
is gated by two MOVE-GENERATORS, not modifiers** -- which is the key finding: you
cannot modifier your way to "most games."

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

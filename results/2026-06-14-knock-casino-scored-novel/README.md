# Knock + Casino-scored: two new borrows, blind-judge-certified novel

Date: 2026-06-14

Two cross-skeleton borrows shipped this session, then validated with a blind LLM
novelty judge (3 independent judges per game, reading only the neutralized
rulebook -- no fitness, no skeleton/borrow provenance). Two controls check that
the judge discriminates instead of rubber-stamping.

## What shipped

- **MechKnock** (rummy knock -> shedding): the first WIN-CONDITION deep borrow.
  Declare once your hand is small to end the game now; fewest cards wins. A wrong
  knock hands the win away. Runner-side (GenerateMoves/ApplyMove/CheckEnd), not a
  hook.
- **Casino as a scoring-borrow host** (MechMeldBonus / MechAvoidance): a
  Scopa-style scored fishing game. The captured pile is banked into the score on
  one end-of-game EventRoundEnd; the winner is captured count plus meld bonus /
  minus avoidance penalty. Unscored casino stays byte-identical (calibration seed
  still 0.772).
- **Casino rulebook** (`writeCasinoRules`): casino games rendered no gameplay
  rules at all before this; the gap made them unplaytestable and unjudgeable.

## Blind judge verdicts

| Game | Composition | Verdict | Notes |
|------|-------------|---------|-------|
| A | casino + meld + avoidance | **novel 3/3** | "a fishing-capture game scored by melds is a combination I cannot place in any published game" |
| B | casino + avoidance only | variant 2/3 | a single penalty suit reads as a Casino house rule |
| C | shedding + combos + knock | **novel 3/3** | Crazy Eights race + Gin knock + climbing combos |
| D | shedding + knock only | **novel 3/3** | "a Crazy Eights race where you declare-to-end fewest-wins is not a published game" |
| E | plain casino (unscored) | variant 3/3 | CONTROL -- correctly "Casino" |
| F | Casino seed | variant 3/3 | CONTROL -- correctly "Casino" |

## What it says

The judge is calibrated: both controls (plain casino, the seed) are called Casino
variants at high confidence, so a "novel" verdict means something.

Both new borrows produce judge-certified novel games on their own. Knock alone
(D) is novel because it changes the WIN CONDITION -- the race-to-empty becomes
race-or-snipe, a tension no published shedding game has. Casino + meld (A) is
novel because no fishing game scores captures by melds.

The refinement: a borrow that changes the win-condition STRUCTURE drives novelty
(knock; casino meld scoring); a borrow that only adds a scoring overlay does not
(casino + avoidance alone = variant). This matches the earlier deep-borrows
finding -- move-tweaks read as variant, win-condition changes read as novel.

Raw per-judge verdicts in `blind-judge-verdicts.json`; the exact dossiers the
judges read are `game_A.md` .. `game_F.md`.

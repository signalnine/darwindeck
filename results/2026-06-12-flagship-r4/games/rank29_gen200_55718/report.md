## gen200_55718 — Fitness 0.000

**Quick Take:** A shedding game with 6 special card effects

**Stats:**
- Skeleton: shedding
- Players: 2, Hand size: 11
- Decision density: 0.55 (55% of turns have meaningful choices)
- Game arc: 0.87
- Interaction: 1.00
- Skill gradient: 0.00
- Session length: 1.00
- Generation: 200
- Veto-stable: yes (4/5 fresh-seed re-evals valid)

**What makes it interesting:**
- Strong game arc — outcomes are uncertain and varied
- High player interaction — your plays frequently affect opponents
- Mostly luck-driven — skill has little impact
- Well-paced game length

**Restamp provenance (Wave M):**
- Veto-stability over 5 fresh seeds: 4/5 valid (veto-stable)
- Single fresh published eval: veto:greedy_longest_run -- headline fitness is therefore 0
- THIS IS THE BUG THE FIX EXPOSES: production published this game from one lucky eval; a fresh single eval lands on a seed where it fails its own degeneracy veto. The K=5 check (majority-stable but with a failing seed) plus the fresh-eval-driven re-rank correctly sink it to the bottom of the bundle.


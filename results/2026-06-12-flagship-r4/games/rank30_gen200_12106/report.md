## gen200_12106 — Fitness 0.000

**Quick Take:** A rummy-style game with draw penalties, penalty cards

**Stats:**
- Skeleton: rummy
- Players: 3, Hand size: 12
- Decision density: 0.36 (36% of turns have meaningful choices)
- Game arc: 0.87
- Interaction: 0.10
- Skill gradient: 0.00
- Session length: 1.00
- Generation: 200
- Veto-stable: yes (4/5 fresh-seed re-evals valid)

**What makes it interesting:**
- Strong game arc — outcomes are uncertain and varied
- Mostly luck-driven — skill has little impact
- Well-paced game length
- Borrows draw penalties from shedding skeleton
- Borrows penalty cards from trick_taking skeleton

**Restamp provenance (Wave M):**
- Veto-stability over 5 fresh seeds: 4/5 valid (veto-stable)
- Single fresh published eval: veto:draw_supply_churn -- headline fitness is therefore 0
- THIS IS THE BUG THE FIX EXPOSES: production published this game from one lucky eval; a fresh single eval lands on a seed where it fails its own degeneracy veto. The K=5 check (majority-stable but with a failing seed) plus the fresh-eval-driven re-rank correctly sink it to the bottom of the bundle.


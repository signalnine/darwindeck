## gen173_5460 — Fitness 0.868

**Quick Take:** A shedding game with 6 special card effects

**Stats:**
- Skeleton: shedding
- Players: 2, Hand size: 13
- Decision density: 0.82 (82% of turns have meaningful choices)
- Game arc: 0.83
- Interaction: 0.93
- Skill gradient: 0.28
- Session length: 1.00
- Generation: 173

**What makes it interesting:**
- High decision density — most turns involve a real choice
- Strong game arc — outcomes are uncertain and varied
- High player interaction — your plays frequently affect opponents
- Well-paced game length

**Fitness provenance:**
- Published fitness 0.868 is the MCTS-mode mean (1 two-tier evals)
- Greedy-only mean: 0.757 (1 evals -- the selection/decile ranking key)
- MCTS uplift: +0.111 (the second skill tier and fresh batches; large gaps on low-skill games are the knock-timing hazard -- see pkg/fitness)
- Component metrics above are last-evaluation values while the published fitness is a running mean over all evaluations: the weighted component sum will NOT reconcile exactly with the headline number


## gen199_73801 — Fitness 0.862

**Quick Take:** A shedding game with 5 special card effects

**Stats:**
- Skeleton: shedding
- Players: 2, Hand size: 13
- Decision density: 0.73 (73% of turns have meaningful choices)
- Game arc: 0.89
- Interaction: 0.84
- Skill gradient: 0.41
- Session length: 1.00
- Generation: 199

**What makes it interesting:**
- High decision density — most turns involve a real choice
- Strong game arc — outcomes are uncertain and varied
- High player interaction — your plays frequently affect opponents
- Good skill gradient — better play is rewarded
- Well-paced game length

**Fitness provenance:**
- Published fitness 0.862 is the MCTS-mode mean (1 two-tier evals)
- Greedy-only mean: 0.756 (1 evals -- the selection/decile ranking key)
- MCTS uplift: +0.106 (the second skill tier and fresh batches; large gaps on low-skill games are the knock-timing hazard -- see pkg/fitness)
- Component metrics above are last-evaluation values while the published fitness is a running mean over all evaluations: the weighted component sum will NOT reconcile exactly with the headline number


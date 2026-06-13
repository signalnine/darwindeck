## gen200_2700 — Fitness 0.532

**Quick Take:** A rummy-style game with draw penalties, penalty cards

**Stats:**
- Skeleton: rummy
- Players: 2, Hand size: 13
- Decision density: 0.33 (33% of turns have meaningful choices)
- Game arc: 0.88
- Interaction: 0.05
- Skill gradient: 0.62
- Session length: 0.97
- Generation: 200

**What makes it interesting:**
- Strong game arc — outcomes are uncertain and varied
- Low interaction — plays are mostly independent
- Good skill gradient — better play is rewarded
- Well-paced game length
- Borrows draw penalties from shedding skeleton
- Borrows penalty cards from trick_taking skeleton

**Fitness provenance:**
- Published fitness 0.532 is the greedy-only mean (1 evals); no MCTS tier was granted
- Component metrics above are last-evaluation values while the published fitness is a running mean over all evaluations: the weighted component sum will NOT reconcile exactly with the headline number


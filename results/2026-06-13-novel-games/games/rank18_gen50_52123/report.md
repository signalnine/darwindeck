## gen50_52123 — Fitness 0.653

**Quick Take:** A trick-taking game with meld bonuses, penalty cards, trump cards

**Stats:**
- Skeleton: trick_taking
- Players: 3, Hand size: 13
- Decision density: 0.27 (27% of turns have meaningful choices)
- Game arc: 0.75
- Interaction: 1.00
- Skill gradient: 0.57
- Session length: 0.84
- Generation: 50
- Veto-stable: yes (5/5 fresh-seed re-evals valid)

**What makes it interesting:**
- Low decision density — many forced plays
- High player interaction — your plays frequently affect opponents
- Good skill gradient — better play is rewarded
- Well-paced game length
- Borrows meld bonuses from rummy skeleton
- Borrows penalty cards from shedding skeleton

**Fitness provenance:**
- Headline fitness 0.653 is the greedy-only running mean (1 evals) -- the leaderboard ranking key for every published game
- No MCTS tier was granted to this genome
- Component metrics above are last-evaluation values while the headline fitness is a running mean over all evaluations: the weighted component sum will NOT reconcile exactly with the headline number


# Top 5 Evolved Games

Run: 2026-01-11_10-12-01

## 1. RapidBoar

**Fitness:** 0.8556
**Skill Evaluation:**
- Greedy vs Random: 80.0%
- MCTS vs Random: 97.0%
- Combined Skill Score: 0.89

**Fitness Metrics:**
| Metric | Score |
|--------|-------|
| Decision Density | 0.650 |
| Comeback Potential | 1.000 |
| Tension Curve | 1.000 |
| Interaction Frequency | 1.000 |
| Rules Complexity | 0.505 |
| Skill vs Luck | 0.887 |
| Bluffing Depth | 0.814 |
| Session Length | 0.342 |

Players engage in a complex multi-phase turn where they must play cards to a central tableau, participate in a trick-taking round with diamonds as trump (where low cards surprisingly win), and potentially play additional cards if their hand is large enough. The goal is to be the first player to empty your hand completely. This evolved game creates an intriguing hybrid of tableau-building and trick-taking mechanics, and with MCTS achieving a 97% win rate against random play, strategic decision-making is crucial for success.

---

## 2. InnerArch

**Fitness:** 0.8545
**Skill Evaluation:**
- Greedy vs Random: 95.0%
- MCTS vs Random: 95.0%
- Combined Skill Score: 0.95

**Fitness Metrics:**
| Metric | Score |
|--------|-------|
| Decision Density | 0.650 |
| Comeback Potential | 1.000 |
| Tension Curve | 1.000 |
| Interaction Frequency | 1.000 |
| Rules Complexity | 0.580 |
| Skill vs Luck | 0.850 |
| Bluffing Depth | 0.810 |
| Session Length | 0.279 |

Players engage in a unique two-phase card game where they first participate in trick-taking rounds (with clubs as trump and an interesting twist where low cards win), followed by strategic tableau play where they can place multiple cards when holding 3+ cards, then must play exactly one card to the tableau. The goal is to be the first player to empty your hand of all cards. What makes this game particularly intriguing is its blend of trick-taking and shedding mechanics, combined with the strategic depth evidenced by AI players vastly outperforming random play (95% win rate), indicating that skillful decision-making is crucial for success.

---

## 3. SmallPhoenix

**Fitness:** 0.8545
**Skill Evaluation:**
- Greedy vs Random: 84.0%
- MCTS vs Random: 96.0%
- Combined Skill Score: 0.90

**Fitness Metrics:**
| Metric | Score |
|--------|-------|
| Decision Density | 0.650 |
| Comeback Potential | 0.980 |
| Tension Curve | 1.000 |
| Interaction Frequency | 1.000 |
| Rules Complexity | 0.505 |
| Skill vs Luck | 0.881 |
| Bluffing Depth | 0.819 |
| Session Length | 0.590 |

Players engage in a complex multi-phase turn where they must play cards to a central tableau under varying conditions - first playing one card when they have any in hand, then playing 1-4 cards when exactly one card is on the tableau, followed by another single card play when holding more than four cards. The goal is to be the first player to empty your hand of all 26 cards. This unusual phased structure creates a highly strategic game where advanced AI significantly outperforms random play (96% vs 84% for simpler strategies), indicating that careful planning and tactical decision-making are crucial for success.

---

## 4. ArcaneDragon

**Fitness:** 0.8545
**Skill Evaluation:**
- Greedy vs Random: 55.0%
- MCTS vs Random: 97.0%
- Combined Skill Score: 0.76
- **First Player Advantage: -10.0%** 

**Fitness Metrics:**
| Metric | Score |
|--------|-------|
| Decision Density | 0.650 |
| Comeback Potential | 0.980 |
| Tension Curve | 1.000 |
| Interaction Frequency | 1.000 |
| Rules Complexity | 0.505 |
| Skill vs Luck | 0.881 |
| Bluffing Depth | 0.819 |
| Session Length | 0.299 |

This hybrid card game combines trick-taking with tableau building across multiple phases per turn. Players first engage in two trick-taking rounds (the second using Hearts as trump), then must play cards to a central tableau area, with the second tableau play only allowed if they have more than 3 cards in hand. Victory comes from either emptying your hand completely or maintaining a low score below 50 points, creating multiple strategic paths to success. The high MCTS win rate (97%) indicates this is a deeply strategic game where careful planning and tactical decisions significantly outweigh luck.

---

## 5. TwilightRiver

**Fitness:** 0.8543
**Skill Evaluation:**
- Greedy vs Random: 98.0%
- MCTS vs Random: 93.0%
- Combined Skill Score: 0.96

**Fitness Metrics:**
| Metric | Score |
|--------|-------|
| Decision Density | 0.650 |
| Comeback Potential | 1.000 |
| Tension Curve | 1.000 |
| Interaction Frequency | 1.000 |
| Rules Complexity | 0.580 |
| Skill vs Luck | 0.850 |
| Bluffing Depth | 0.809 |
| Session Length | 0.289 |

This strategic card game combines trick-taking with tableau building, where players must navigate a complex turn structure that includes playing tricks (with clubs as trump and an unusual low-card-wins rule) followed by mandatory card plays to a shared tableau. Victory goes to the first player to empty their hand completely, creating tension between participating in tricks and managing your hand size. The game demands serious strategic thinking, as evidenced by the 93% win rate of advanced AI against random play, and features an interesting balance where neither player has a significant first-move advantage.

---


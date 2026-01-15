# Top 5 Evolved Games

Run: 2026-01-14_15-01-32

## 1. WesternRidge

**Fitness:** 0.5939
**Skill Evaluation:**
- Greedy vs Random: 8.0%
- MCTS vs Random: 50.0%
- Combined Skill Score: 0.29
- **First Player Advantage: +26.0%** 

**Fitness Metrics:**
| Metric | Score |
|--------|-------|
| Decision Density | 0.330 |
| Comeback Potential | 0.824 |
| Tension Curve | 0.867 |
| Interaction Frequency | 1.000 |
| Rules Complexity | 0.620 |
| Skill vs Luck | 0.363 |
| Bluffing Depth | 0.000 |
| Session Length | 0.156 |

Players take turns playing single cards to a central tableau, following a "two high" beating system where each card must either be higher than the previous card by at least two ranks, or be played to an empty tableau. The goal is to be the first player to empty your 12-card hand. With a significant first-player advantage and minimal strategic depth (as shown by the low skill differentiation), this creates a fast-paced shedding game that relies more on card distribution and turn order than deep tactical planning.

---

## 2. ArcaneTiger

**Fitness:** 0.5947
**Skill Evaluation:**
- Greedy vs Random: 10.0%
- MCTS vs Random: 49.0%
- Combined Skill Score: 0.29
- **First Player Advantage: +17.0%** 

**Fitness Metrics:**
| Metric | Score |
|--------|-------|
| Decision Density | 0.341 |
| Comeback Potential | 0.832 |
| Tension Curve | 0.871 |
| Interaction Frequency | 1.000 |
| Rules Complexity | 0.620 |
| Skill vs Luck | 0.333 |
| Bluffing Depth | 0.000 |
| Session Length | 0.169 |

In this card game, players take turns playing single cards to a shared tableau, but can only play if the tableau is empty or if their card beats the top card by having a rank that's two or higher. The goal is simple: be the first player to empty your hand of all 13 cards. While the core mechanic seems straightforward, the game appears to be mostly luck-driven with limited strategic depth, as even advanced AI players struggle significantly against random play, suggesting that card draw and timing matter more than tactical decision-making.

---

## 3. OldClash

**Fitness:** 0.5940
**Skill Evaluation:**
- Greedy vs Random: 10.0%
- MCTS vs Random: 52.0%
- Combined Skill Score: 0.31
- **First Player Advantage: +28.0%** 

**Fitness Metrics:**
| Metric | Score |
|--------|-------|
| Decision Density | 0.339 |
| Comeback Potential | 0.828 |
| Tension Curve | 0.875 |
| Interaction Frequency | 1.000 |
| Rules Complexity | 0.620 |
| Skill vs Luck | 0.330 |
| Bluffing Depth | 0.000 |
| Session Length | 0.171 |

Players take turns playing single cards to a central tableau, either starting the pile when empty or playing a card that beats the current top card by at least two ranks (like playing a 7 on a 5 or lower). The first player to empty their hand wins this shedding-style game. The significant first-player advantage (+28%) and moderate strategic depth make timing and card sequencing crucial, though the game favors tactical play over deep planning.

---

## 4. ScarletTide

**Fitness:** 0.5967
**Skill Evaluation:**
- Greedy vs Random: 15.0%
- MCTS vs Random: 53.0%
- Combined Skill Score: 0.34
- **First Player Advantage: +24.0%** 

**Fitness Metrics:**
| Metric | Score |
|--------|-------|
| Decision Density | 0.340 |
| Comeback Potential | 0.856 |
| Tension Curve | 0.845 |
| Interaction Frequency | 1.000 |
| Rules Complexity | 0.620 |
| Skill vs Luck | 0.325 |
| Bluffing Depth | 0.000 |
| Session Length | 0.169 |

Players take turns playing single cards to a shared tableau, following a climbing mechanic where each card must beat the previous one by at least two ranks (or they can start fresh on an empty tableau). The goal is to be the first player to empty your hand of all 13 cards. This creates a moderately strategic game where the MCTS AI significantly outperforms random play (53% vs 15% for greedy strategy), indicating that careful card sequencing and timing decisions meaningfully impact success.

---

## 5. SecretDagger

**Fitness:** 0.6352
**Skill Evaluation:**
- Greedy vs Random: 50.0%
- MCTS vs Random: 50.0%
- Combined Skill Score: 0.50

**Fitness Metrics:**
| Metric | Score |
|--------|-------|
| Decision Density | 0.340 |
| Comeback Potential | 0.656 |
| Tension Curve | 0.865 |
| Interaction Frequency | 1.000 |
| Rules Complexity | 0.740 |
| Skill vs Luck | 0.385 |
| Bluffing Depth | 0.000 |
| Session Length | 0.169 |

Players start with 13 cards each and can optionally draw up to 5 cards from the discard pile on their turn, creating an unusual reverse-draw mechanic where the discard pile becomes a resource rather than waste. The goal is to be the first player to empty your hand completely. With both advanced AI and simple strategies performing equally well (50% win rates), this appears to be a largely luck-based game where the optional drawing mechanic may not provide enough strategic depth to reward skillful play.

---


# Top 5 Evolved Games

Run: 2026-01-11_11-36-16

## 1. InnerWave

**Fitness:** 0.7082
**Skill Evaluation:**
- Greedy vs Random: 50.0%
- MCTS vs Random: 63.0%
- Combined Skill Score: 0.56
- **First Player Advantage: +15.0%** 

**Fitness Metrics:**
| Metric | Score |
|--------|-------|
| Decision Density | 0.650 |
| Comeback Potential | 0.690 |
| Tension Curve | 0.568 |
| Interaction Frequency | 1.000 |
| Rules Complexity | 0.655 |
| Skill vs Luck | 0.617 |
| Bluffing Depth | 0.000 |
| Session Length | 0.116 |

This is a trick-taking game where players follow a unique three-phase structure each round: first playing traditional high-card tricks while avoiding hearts, then switching to a hearts-trump phase where low cards actually win, and finally returning to high-card play with hearts still forbidden. Players aim to keep their cumulative score below 100 points to avoid elimination, with the last player standing declared the winner. The alternating phases create an intriguing tactical puzzle where card values flip between rounds, though the moderate AI performance suggests luck plays a significant role alongside strategy.

---

## 2. FirstLynx

**Fitness:** 0.6743
**Skill Evaluation:**
- Greedy vs Random: 38.0%
- MCTS vs Random: 57.0%
- Combined Skill Score: 0.47

**Fitness Metrics:**
| Metric | Score |
|--------|-------|
| Decision Density | 0.650 |
| Comeback Potential | 0.580 |
| Tension Curve | 0.568 |
| Interaction Frequency | 1.000 |
| Rules Complexity | 0.730 |
| Skill vs Luck | 0.546 |
| Bluffing Depth | 0.000 |
| Session Length | 0.116 |

This is a trick-taking card game where 4 players compete over 13 rounds, following suit when possible with the highest card winning each trick, and hearts serving as a special "breaking suit." Players aim to keep their scores low to avoid reaching 100 points, creating a penalty-avoidance dynamic similar to Hearts. The game shows moderate strategic depth, as evidenced by the MCTS algorithm's 57% win rate against random play, though the duplicate trick phases in the genome suggest some developmental quirks in this evolved ruleset.

---

## 3. EmeraldFeint

**Fitness:** 0.6421
**Skill Evaluation:**
- Greedy vs Random: 43.0%
- MCTS vs Random: 52.0%
- Combined Skill Score: 0.47

**Fitness Metrics:**
| Metric | Score |
|--------|-------|
| Decision Density | 0.455 |
| Comeback Potential | 0.640 |
| Tension Curve | 0.568 |
| Interaction Frequency | 1.000 |
| Rules Complexity | 0.805 |
| Skill vs Luck | 0.527 |
| Bluffing Depth | 0.000 |
| Session Length | 0.116 |

Players engage in a 13-trick card game where they must follow the lead suit if possible, with the highest card winning each trick, and hearts serving as a special "breaking suit" that likely carries penalties. The goal is to achieve the lowest score possible under 100 points across multiple hands, creating a strategic avoidance game similar to Hearts. With MCTS AI only achieving a 52% win rate against random play, this appears to be a relatively luck-dependent game where basic card-following rules matter more than deep strategic planning.

---

## 4. YoungBond

**Fitness:** 0.5920
**Skill Evaluation:**
- Greedy vs Random: 39.0%
- MCTS vs Random: 56.0%
- Combined Skill Score: 0.48

**Fitness Metrics:**
| Metric | Score |
|--------|-------|
| Decision Density | 0.416 |
| Comeback Potential | 0.560 |
| Tension Curve | 0.456 |
| Interaction Frequency | 1.000 |
| Rules Complexity | 0.805 |
| Skill vs Luck | 0.463 |
| Bluffing Depth | 0.000 |
| Session Length | 0.098 |

This is a trick-taking game where four players compete in 13 rounds, following suit when possible with clubs as trump and hearts as the breaking suit. Players aim to keep their scores low to be the first to reach under 100 points, though the genome doesn't specify how points are actually accumulated. The game shows moderate strategic depth with AI performing significantly better than random play (56% vs random), suggesting that careful card management and trick-taking decisions meaningfully impact success.

---

## 5. ProwlingRaise

**Fitness:** 0.4803
**Skill Evaluation:**
- Greedy vs Random: 0.0%
- MCTS vs Random: 8.0%
- Combined Skill Score: 0.04

**Fitness Metrics:**
| Metric | Score |
|--------|-------|
| Decision Density | 0.650 |
| Comeback Potential | 0.020 |
| Tension Curve | 0.568 |
| Interaction Frequency | 0.590 |
| Rules Complexity | 0.505 |
| Skill vs Luck | 0.491 |
| Bluffing Depth | 0.000 |
| Session Length | 0.116 |

This is a hybrid trick-taking game where players start each turn by discarding a card, then play through multiple trick-taking rounds where they must follow suit and high cards win. Victory goes to whoever has the lowest score under 100 points, has emptied their hand, or has captured the most tricks - creating multiple competing strategies. The game shows very low strategic depth, with even advanced AI strategies barely outperforming random play, suggesting that luck heavily dominates over skill in determining outcomes.

---


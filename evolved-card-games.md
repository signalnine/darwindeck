---
apple_notes_id: x-coredata://975E8692-B6B0-48E2-97C5-0E5E6F550F8B/ICNote/p1321
---

# Evolved Card Games

Created: 2026-01-09
Status: Idea
Priority: Low

## Overview

Use large-scale Monte Carlo simulations and genetic algorithms to evolve novel card games playable with a standard 52-card deck. Optimize for configurable parameters like complexity, session length, skill/luck ratio, and player count.

## Core Concept

### Game Representation (Genome)

A card game can be encoded as a set of rules:
- **Setup**: Deal pattern, hand sizes, tableau configuration
- **Turn structure**: Draw rules, play rules, pass rules
- **Valid moves**: Card matching (suit, rank, sequence), special card effects
- **Win conditions**: Empty hand, point threshold, capture goals
- **Scoring**: Point values, bonuses, penalties

### Fitness Functions

Configurable optimization targets:
- **Rules complexity**: Number of distinct rules, decision tree depth
- **Session length**: Average turns to completion
- **Skill vs luck ratio**: Correlation between "optimal play" and win rate
- **Player count**: 2-player, 3-4 player, party size (5+)
- **Decision density**: Meaningful choices per turn
- **Comeback potential**: Can trailing players recover?
- **Engagement**: Minimize "dead" turns with no real choice

### Simulation Engine

Monte Carlo approach:
1. Generate random game rules (or mutate existing)
2. Simulate thousands of games with AI players (random, greedy, MCTS)
3. Measure fitness metrics
4. Select, crossover, mutate top performers
5. Repeat for N generations

### AI Players for Simulation

- **Random**: Baseline, validates game is playable
- **Greedy**: Simple heuristics, measures obvious strategy
- **MCTS**: Tree search, approximates skilled play
- **Compare outcomes**: Skill gap = MCTS win rate vs random

## Technical Approach

### Rule DSL

Define a domain-specific language for card game rules:
```
setup:
  deal: 7 each
  tableau: none

turn:
  draw: 1 from deck
  play: 1..n matching suit OR rank of discard top

special:
  8: wild, player chooses suit
  2: next player draws 2

win:
  first to empty hand
```

### Genetic Operators

- **Mutation**: Change a rule parameter, add/remove special card effect
- **Crossover**: Combine turn structure from game A with win condition from game B
- **Elitism**: Preserve top 10% unchanged

### Constraints

- Must be playable (no infinite loops, unreachable win states)
- Must terminate (enforce max turns)
- Must have agency (some non-random decisions)

## Interesting Dimensions to Explore

1. **Known games as seeds**: Start from Crazy 8s, Rummy, War - see what evolves
2. **Pareto frontier**: Map the tradeoff space (quick+lucky vs long+skillful)
3. **Human playtesting**: Top evolved games get human trials
4. **Explainability**: Generate natural language rules from genome

## Prior Art / Inspiration

- Evolutionary game design research
- AI Dungeon / procedural game generation
- CFR (Counterfactual Regret Minimization) for poker
- BoardGameGeek complexity/weight ratings as training signal

## Proxy Metrics for "Fun"

Since fun is subjective and requires human feedback, use measurable proxies:

- **Decision density**: Ratio of turns with meaningful choices vs forced/obvious plays
- **Comeback potential**: Probability that trailing player can still win (avoid runaway leaders)
- **Tension curve**: Game state uncertainty over time (want drama near end, not foregone conclusions)
- **Minimal dead turns**: Avoid "draw and pass" situations
- **Information asymmetry**: Hidden info creates anticipation (but too much → pure luck)
- **Pacing variance**: Mix of quick tactical plays and longer strategic arcs
- **Interaction frequency**: How often do your actions affect opponents?

Validate proxies with human playtesting on evolved games - correlate proxy scores with enjoyment ratings to refine the fitness function.

## Questions

- Balance novelty vs familiarity?
- Multi-objective optimization (Pareto) vs weighted sum?
- How many generations/simulations needed for convergence?

## Next Steps

1. [ ] Research existing card game DSLs / formalizations
2. [ ] Prototype simple rule genome + random generation
3. [ ] Build simulation harness with random AI
4. [ ] Implement basic genetic operators
5. [ ] Run first evolution experiments
6. [ ] Add MCTS player for skill measurement

---
*Created: 2026-01-09*

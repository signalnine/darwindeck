# ModernRaid

## Overview
ModernRaid is a fast-paced 4-player card game where you'll strategically draw and play cards across five distinct phases, racing to be the first to empty your hand completely. Master the timing of when to draw more cards versus when to play them as you compete to achieve the ultimate goal of having no cards left.

## Components
- Standard 52-card deck (4 players)

## Setup
1. Shuffle the deck
2. Deal 12 cards to each player
3. Place 1 card face-up to start the discard pile
4. Place remaining cards face-down as the draw pile

## Objective
Meet the all_hands_empty condition

## Turn Structure
Each turn consists of 5 phase(s):

### Phase 1: Draw
Draw 1 card from the deck

### Phase 2: Draw
Draw 1 card from the deck

### Phase 3: Play
Play up to 10 cards to the tableau (optional)

### Phase 4: Draw
Draw 1 card from the deck

### Phase 5: Play
Play up to 10 cards to the tableau (optional)

## Edge Cases
**Empty deck:** Shuffle the discard pile (except top card) to form a new deck. If still empty, skip the draw.

**Tie:** If multiple players meet win conditions simultaneously, the active player wins.

**Hand limit:** If your hand exceeds 15 cards, discard down to 15 at end of turn.

**Turn limit:** If max turns reached, highest score wins (or draw if no scoring).

# HighStone

## Components
- Standard 52-card deck (4 players)

## Setup
1. Shuffle the deck
2. Deal 12 cards to each player
3. Place remaining cards face-down as the draw pile

## Objective
Meet the all_hands_empty condition

## Turn Structure
Each turn consists of 5 phase(s):

### Phase 1: Draw
Draw 1 card from the deck

### Phase 2: Play
Play up to 10 cards to the tableau (optional)

### Phase 3: Draw
Draw 1 card from the deck

### Phase 4: Play
Play exactly 1 card to the tableau

### Phase 5: Draw
Draw 1 card from the deck

## Edge Cases
**Empty deck:** Shuffle the discard pile (except top card) to form a new deck. If still empty, skip the draw.

**Tie:** If multiple players meet win conditions simultaneously, the active player wins.

**Hand limit:** If your hand exceeds 15 cards, discard down to 15 at end of turn.

**Turn limit:** If max turns reached, highest score wins (or draw if no scoring).

package engine

// Effect type constants
const (
	EFFECT_SKIP_NEXT = iota
	EFFECT_REVERSE
	EFFECT_DRAW_CARDS
	EFFECT_EXTRA_TURN
	EFFECT_FORCE_DISCARD
)

// Target constants
const (
	TARGET_NEXT_PLAYER = iota
	TARGET_PREV_PLAYER
	TARGET_PLAYER_CHOICE
	TARGET_RANDOM_OPPONENT
	TARGET_ALL_OPPONENTS
	TARGET_LEFT_OPPONENT
	TARGET_RIGHT_OPPONENT
)

// SpecialEffect represents a card-triggered effect
type SpecialEffect struct {
	TriggerRank uint8
	EffectType  uint8
	Target      uint8
	Value       uint8
}

// RNG interface for deterministic random (nil = no random effects)
type RNG interface {
	Intn(n int) int
}

// ApplyEffect executes a special effect on the game state
func ApplyEffect(state *GameState, effect *SpecialEffect, rng RNG) {
	switch effect.EffectType {
	case EFFECT_SKIP_NEXT:
		state.SkipCount += effect.Value
		// Cap at NumPlayers-1 to prevent degenerate infinite turns
		maxSkip := state.NumPlayers - 1
		if state.SkipCount > maxSkip {
			state.SkipCount = maxSkip
		}

	case EFFECT_REVERSE:
		state.PlayDirection *= -1

	case EFFECT_DRAW_CARDS:
		applyToTargets(state, effect.Target, rng, func(targetID int) {
			for i := uint8(0); i < effect.Value && len(state.Deck) > 0; i++ {
				card := state.Deck[0]
				state.Deck = state.Deck[1:]
				state.Players[targetID].Hand = append(state.Players[targetID].Hand, card)
			}
		})

	case EFFECT_EXTRA_TURN:
		// Skip everyone else = current player goes again
		state.SkipCount = state.NumPlayers - 1

	case EFFECT_FORCE_DISCARD:
		applyToTargets(state, effect.Target, rng, func(targetID int) {
			hand := &state.Players[targetID].Hand
			toDiscard := int(effect.Value)
			if toDiscard > len(*hand) {
				toDiscard = len(*hand)
			}
			for i := 0; i < toDiscard; i++ {
				card := (*hand)[len(*hand)-1]
				*hand = (*hand)[:len(*hand)-1]
				state.Discard = append(state.Discard, card)
			}
		})

	default:
		// Unknown effect type - ignore for forward compatibility
	}
}

// resolveTarget determines which player(s) an effect targets
func resolveTarget(state *GameState, target uint8) int {
	current := int(state.CurrentPlayer)
	numPlayers := int(state.NumPlayers)
	direction := int(state.PlayDirection)

	switch target {
	case TARGET_NEXT_PLAYER:
		return (current + direction + numPlayers) % numPlayers
	case TARGET_PREV_PLAYER:
		return (current - direction + numPlayers) % numPlayers
	case TARGET_ALL_OPPONENTS:
		// Returns -1 to signal caller must loop over all opponents
		return -1
	default:
		return (current + 1) % numPlayers
	}
}

// applyToTargets handles single target or ALL_OPPONENTS
func applyToTargets(state *GameState, target uint8, rng RNG, action func(int)) {
	targetID := resolveTarget(state, target)
	if targetID == -1 {
		// ALL_OPPONENTS: apply to everyone except current player
		for i := 0; i < int(state.NumPlayers); i++ {
			if i != int(state.CurrentPlayer) {
				action(i)
			}
		}
	} else {
		action(targetID)
	}
}

// AdvanceTurn moves to the next player, respecting direction and skips
func AdvanceTurn(state *GameState) {
	step := int(state.PlayDirection)
	next := int(state.CurrentPlayer)
	numPlayers := int(state.NumPlayers)

	// Always advance at least once, plus any skips
	for i := 0; i <= int(state.SkipCount); i++ {
		next = (next + step + numPlayers) % numPlayers
	}

	state.CurrentPlayer = uint8(next)
	state.SkipCount = 0 // Reset after applying
}

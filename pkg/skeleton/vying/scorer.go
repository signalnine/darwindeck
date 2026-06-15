package vying

import "github.com/darwindeck/darwindeck/pkg/sim"

// VyingScorer is the greedy poker player: it bets by hand strength. It reads the
// active player's hidden hand, ranks it (HandStrength -> category), and prefers
// to raise strong hands, call mediocre ones, fold weak ones to a bet, and check
// weak ones for free. This is the tight-aggressive policy random play lacks (a
// random bettor folds monsters and raises air), so it supplies the skill
// gradient. Implements sim.MoveScorer.
type VyingScorer struct{}

func (s *VyingScorer) ScoreMove(move sim.Move, state *sim.GameState) float64 {
	// Hand category: 0 = high card, 1 = pair, 2 = two pair, ... 8 = straight
	// flush. The category dominates the decision; kickers refine within it but
	// the betting policy keys on category strength.
	cat := float64(CategoryOf(HandStrength(state.Hands[move.PlayerID])))

	switch move.Type {
	case sim.MoveRaise:
		// Love raising strong hands, hate raising air.
		return cat*10.0 - 5.0 // high card -5, pair 5, two pair 15, trips 25, ...
	case sim.MoveCall:
		// Call with at least mediocre strength.
		return cat*6.0 + 2.0 // high card 2, pair 8, two pair 14, ...
	case sim.MoveCheck:
		// Free card: mildly positive, but below a raise once the hand is strong
		// so two-pair-plus bets rather than checks.
		return 8.0
	case sim.MoveFold:
		// Fold weak hands to a bet; folding a made hand is bad.
		return 7.0 - cat*6.0 // high card 7 (fold), pair 1, two pair -5, ...
	}
	return 0
}

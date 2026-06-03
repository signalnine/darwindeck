package sim

import "testing"

// A MovePlay whose Cards slice is empty must not panic. All current runners
// populate Cards for MovePlay, but a borrowed-mechanic move generator could
// emit an empty one; a panic here would abort the whole fitness evaluation
// worker. TrickTakingScorer already guards this case; SheddingScorer must too.
func TestSheddingScorerEmptyCardsNoPanic(t *testing.T) {
	scorer := &SheddingScorer{}
	state := &GameState{
		Hands:      [][]Card{{}},
		NumPlayers: 1,
		Active:     0,
	}
	move := Move{Type: MovePlay, Cards: nil, PlayerID: 0}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("SheddingScorer.ScoreMove panicked on empty Cards: %v", r)
		}
	}()
	if got := scorer.ScoreMove(move, state); got != 0 {
		t.Errorf("empty-Cards MovePlay score = %.2f, want neutral 0", got)
	}
}

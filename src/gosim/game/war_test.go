package game

import (
	"testing"
)

func TestWarGame_PlayBattle(t *testing.T) {
	game := NewWarGame(42)

	if len(game.Player1Hand) != 26 {
		t.Errorf("Player1 has %d cards, want 26", len(game.Player1Hand))
	}
	if len(game.Player2Hand) != 26 {
		t.Errorf("Player2 has %d cards, want 26", len(game.Player2Hand))
	}

	game.PlayBattle()

	total := len(game.Player1Hand) + len(game.Player2Hand)
	if total != 52 {
		t.Errorf("Total cards = %d, want 52", total)
	}
}

func TestPlayWarGame(t *testing.T) {
	result := PlayWarGame(42, 1000)

	if result.Winner != 1 && result.Winner != 2 {
		t.Errorf("Winner = %d, want 1 or 2", result.Winner)
	}
	if result.Turns < 1 || result.Turns > 1000 {
		t.Errorf("Turns = %d, want 1-1000", result.Turns)
	}
}

func BenchmarkPlayWarGame(b *testing.B) {
	for i := 0; i < b.N; i++ {
		PlayWarGame(int64(i), 1000)
	}
}

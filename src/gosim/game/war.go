package game

import (
	"math/rand"
)

// WarGame represents the game state for War
type WarGame struct {
	Player1Hand []int
	Player2Hand []int
	Turns       int
	rng         *rand.Rand
}

// WarResult contains game outcome
type WarResult struct {
	Winner int
	Turns  int
}

// NewWarGame creates a new War game
func NewWarGame(seed int64) *WarGame {
	rng := rand.New(rand.NewSource(seed))

	// Create deck (ranks 1-13, four of each)
	deck := make([]int, 52)
	idx := 0
	for suit := 0; suit < 4; suit++ {
		for rank := 1; rank <= 13; rank++ {
			deck[idx] = rank
			idx++
		}
	}

	// Shuffle
	rng.Shuffle(len(deck), func(i, j int) {
		deck[i], deck[j] = deck[j], deck[i]
	})

	return &WarGame{
		Player1Hand: deck[:26],
		Player2Hand: deck[26:],
		Turns:       0,
		rng:         rng,
	}
}

// PlayBattle plays one battle
func (g *WarGame) PlayBattle() {
	if len(g.Player1Hand) == 0 || len(g.Player2Hand) == 0 {
		return
	}

	p1Card := g.Player1Hand[0]
	p2Card := g.Player2Hand[0]
	g.Player1Hand = g.Player1Hand[1:]
	g.Player2Hand = g.Player2Hand[1:]

	if p1Card > p2Card {
		g.Player1Hand = append(g.Player1Hand, p1Card, p2Card)
	} else if p2Card > p1Card {
		g.Player2Hand = append(g.Player2Hand, p2Card, p1Card)
	} else {
		// War!
		if len(g.Player1Hand) >= 4 && len(g.Player2Hand) >= 4 {
			warPile := []int{p1Card, p2Card}
			warPile = append(warPile, g.Player1Hand[:4]...)
			warPile = append(warPile, g.Player2Hand[:4]...)
			g.Player1Hand = g.Player1Hand[4:]
			g.Player2Hand = g.Player2Hand[4:]

			// Winner takes all
			if warPile[len(warPile)-4] > warPile[len(warPile)-1] {
				g.Player1Hand = append(g.Player1Hand, warPile...)
			} else {
				g.Player2Hand = append(g.Player2Hand, warPile...)
			}
		} else {
			// Not enough cards, return them
			g.Player1Hand = append(g.Player1Hand, p1Card)
			g.Player2Hand = append(g.Player2Hand, p2Card)
		}
	}

	g.Turns++
}

// IsGameOver checks if game has ended
func (g *WarGame) IsGameOver() bool {
	return len(g.Player1Hand) == 0 || len(g.Player2Hand) == 0
}

// GetWinner returns winner (1 or 2)
func (g *WarGame) GetWinner() int {
	if len(g.Player1Hand) > len(g.Player2Hand) {
		return 1
	}
	return 2
}

// PlayWarGame plays a complete game
func PlayWarGame(seed int64, maxTurns int) WarResult {
	game := NewWarGame(seed)

	for !game.IsGameOver() && game.Turns < maxTurns {
		game.PlayBattle()
	}

	return WarResult{
		Winner: game.GetWinner(),
		Turns:  game.Turns,
	}
}

package sim

// GameResult holds the outcome of a single game.
type GameResult struct {
	Winner    int // Player ID who won, or -1 for draw
	Turns     int
	Events    []Event
	Error     string
	HandSizes []int // Final hand sizes per player
}

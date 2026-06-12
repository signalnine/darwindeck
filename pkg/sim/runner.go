package sim

// GameResult holds the outcome of a single game.
type GameResult struct {
	Winner    int // Player ID who won, or -1 for draw
	Turns     int
	Events    []Event
	Error     string
	HandSizes []int // Final hand sizes per player

	// TurnRecords holds one record per applied move (audit Task 7). Named
	// TurnRecords rather than the plan's "Turns" because the Turns count
	// field above predates it.
	TurnRecords []TurnRecord
	// Leaders is the leader after each applied move, -1 = tie. Left nil by
	// the batch runner until Task 8 (progress tracking) fills it.
	Leaders []int8
}

package sim

// GameResult holds the outcome of a single game.
// (A HandSizes field once lived here but was never populated by the batch
// runner -- only a shedding test helper filled it -- so any consumer read nil
// silently; removed rather than left as a trap.)
type GameResult struct {
	Winner int // Player ID who won, or -1 for draw
	Turns  int
	Events []Event
	Error  string

	// TurnRecords holds one record per applied move (audit Task 7). Named
	// TurnRecords rather than the plan's "Turns" because the Turns count
	// field above predates it.
	TurnRecords []TurnRecord
	// Leaders is the leader after each applied move (argmax of the runner's
	// Progress snapshot), -1 = tie; parallel to TurnRecords (audit Task 8).
	Leaders []int8
}

package engine

// TensionMetrics tracks tension curve data during simulation
type TensionMetrics struct {
	LeadChanges   int     // Number of times leader switched
	DecisiveTurn  int     // Turn when winner took PERMANENT lead
	ClosestMargin float32 // Smallest normalized gap between 1st and 2nd (0 = tied)
	TotalTurns    int     // For computing decisive turn percentage

	// Internal tracking (not serialized)
	currentLeader int   // Player ID of current leader (-1 for tie)
	leaderHistory []int // Leader at each turn (for permanent lead calculation)
}

// LeaderDetector interface for game-type-specific leader detection
type LeaderDetector interface {
	GetLeader(state *GameState) int     // Returns player ID or -1 for tie
	GetMargin(state *GameState) float32 // Normalized gap (0-1), 0 = tied, 1 = max gap
}

// NewTensionMetrics creates initialized tension tracker
func NewTensionMetrics(numPlayers int) *TensionMetrics {
	return &TensionMetrics{
		currentLeader: -1,
		ClosestMargin: 1.0,
		leaderHistory: make([]int, 0, 100),
	}
}

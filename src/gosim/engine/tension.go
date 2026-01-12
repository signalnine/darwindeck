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

// ScoreLeaderDetector - for score-based games (Gin Rummy, Scopa)
// Higher score = winning
type ScoreLeaderDetector struct{}

func (d *ScoreLeaderDetector) GetLeader(state *GameState) int {
	if len(state.Players) < 2 {
		return -1
	}
	maxScore := state.Players[0].Score
	leader := 0
	tied := false
	for i := 1; i < len(state.Players); i++ {
		if state.Players[i].Score > maxScore {
			maxScore = state.Players[i].Score
			leader = i
			tied = false
		} else if state.Players[i].Score == maxScore {
			tied = true
		}
	}
	if tied {
		return -1
	}
	return leader
}

func (d *ScoreLeaderDetector) GetMargin(state *GameState) float32 {
	if len(state.Players) < 2 {
		return 0
	}
	var first, second int32 = 0, 0
	for _, p := range state.Players {
		if p.Score > first {
			second = first
			first = p.Score
		} else if p.Score > second {
			second = p.Score
		}
	}
	if first == 0 {
		return 0
	}
	return float32(first-second) / float32(first)
}

// HandSizeLeaderDetector - for shedding games (Crazy 8s, President)
// Fewer cards = winning
type HandSizeLeaderDetector struct{}

func (d *HandSizeLeaderDetector) GetLeader(state *GameState) int {
	if len(state.Players) < 2 {
		return -1
	}
	minCards := len(state.Players[0].Hand)
	leader := 0
	tied := false
	for i := 1; i < len(state.Players); i++ {
		cards := len(state.Players[i].Hand)
		if cards < minCards {
			minCards = cards
			leader = i
			tied = false
		} else if cards == minCards {
			tied = true
		}
	}
	if tied {
		return -1
	}
	return leader
}

func (d *HandSizeLeaderDetector) GetMargin(state *GameState) float32 {
	if len(state.Players) < 2 {
		return 0
	}
	first, second := 999, 999
	maxCards := 0
	for _, p := range state.Players {
		cards := len(p.Hand)
		if cards > maxCards {
			maxCards = cards
		}
		if cards < first {
			second = first
			first = cards
		} else if cards < second {
			second = cards
		}
	}
	if maxCards == 0 || second == 999 {
		return 0
	}
	return float32(second-first) / float32(maxCards)
}

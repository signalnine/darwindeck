package engine

// WinType constants for tension detection
// These map to win condition types in bytecode
const (
	WinTypeEmptyHand    uint8 = 0 // Shedding games - empty hand wins
	WinTypeHighScore    uint8 = 1 // Score-based - highest score wins
	WinTypeFirstToScore uint8 = 2 // Race to threshold
	WinTypeCaptureAll   uint8 = 3 // War-style capture
	WinTypeLowScore     uint8 = 4 // Avoidance games (Hearts) - lowest score wins
	WinTypeAllHandEmpty uint8 = 5 // Trick-taking hand end
	WinTypeBestHand     uint8 = 6 // Poker-style hand comparison
	WinTypeMostCaptured uint8 = 7 // Scopa-style most cards
	WinTypeMostTricks   uint8 = 8 // Trick-collecting games (Spades)
	WinTypeFewestTricks uint8 = 9 // Trick-avoidance games (Hearts)
	WinTypeMostChips    uint8 = 10 // Poker cash games
)

// TensionMetrics tracks tension curve data during simulation
type TensionMetrics struct {
	LeadChanges      int     // Number of times leader switched
	DecisiveTurn     int     // Turn when winner took PERMANENT lead
	ClosestMargin    float32 // Smallest normalized gap between 1st and 2nd (0 = tied)
	TotalTurns       int     // For computing decisive turn percentage
	WinnerWasTrailing bool   // True if winner was behind at midpoint (comeback win)

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

// HandSizeMaxLeaderDetector - for capture/collection games (War)
// More cards in hand = winning (opposite of shedding)
type HandSizeMaxLeaderDetector struct{}

func (d *HandSizeMaxLeaderDetector) GetLeader(state *GameState) int {
	if len(state.Players) < 2 {
		return -1
	}
	maxCards := len(state.Players[0].Hand)
	leader := 0
	tied := false
	for i := 1; i < len(state.Players); i++ {
		cards := len(state.Players[i].Hand)
		if cards > maxCards {
			maxCards = cards
			leader = i
			tied = false
		} else if cards == maxCards {
			tied = true
		}
	}
	if tied {
		return -1
	}
	return leader
}

func (d *HandSizeMaxLeaderDetector) GetMargin(state *GameState) float32 {
	if len(state.Players) < 2 {
		return 0
	}
	var first, second int = 0, 0
	var totalCards int = 0
	for _, p := range state.Players {
		cards := len(p.Hand)
		totalCards += cards
		if cards > first {
			second = first
			first = cards
		} else if cards > second {
			second = cards
		}
	}
	if totalCards == 0 {
		return 0
	}
	return float32(first-second) / float32(totalCards)
}

// TrickLeaderDetector - for trick-COLLECTING games (Spades, Whist)
// More tricks = winning
type TrickLeaderDetector struct{}

func (d *TrickLeaderDetector) GetLeader(state *GameState) int {
	if len(state.TricksWon) < 2 {
		return -1
	}
	maxTricks := state.TricksWon[0]
	leader := 0
	tied := false
	for i := 1; i < len(state.TricksWon); i++ {
		if state.TricksWon[i] > maxTricks {
			maxTricks = state.TricksWon[i]
			leader = i
			tied = false
		} else if state.TricksWon[i] == maxTricks {
			tied = true
		}
	}
	if tied {
		return -1
	}
	return leader
}

func (d *TrickLeaderDetector) GetMargin(state *GameState) float32 {
	if len(state.TricksWon) < 2 {
		return 0
	}
	var first, second uint8 = 0, 0
	var totalTricks uint8 = 0
	for _, tricks := range state.TricksWon {
		totalTricks += tricks
		if tricks > first {
			second = first
			first = tricks
		} else if tricks > second {
			second = tricks
		}
	}
	if totalTricks == 0 {
		return 0
	}
	return float32(first-second) / float32(totalTricks)
}

// TrickAvoidanceLeaderDetector - for trick-AVOIDANCE games (Hearts)
// Fewer tricks = winning
type TrickAvoidanceLeaderDetector struct{}

func (d *TrickAvoidanceLeaderDetector) GetLeader(state *GameState) int {
	if len(state.TricksWon) < 2 {
		return -1
	}
	minTricks := state.TricksWon[0]
	leader := 0
	tied := false
	for i := 1; i < len(state.TricksWon); i++ {
		if state.TricksWon[i] < minTricks {
			minTricks = state.TricksWon[i]
			leader = i
			tied = false
		} else if state.TricksWon[i] == minTricks {
			tied = true
		}
	}
	if tied {
		return -1
	}
	return leader
}

func (d *TrickAvoidanceLeaderDetector) GetMargin(state *GameState) float32 {
	if len(state.TricksWon) < 2 {
		return 0
	}
	var first, second uint8 = 255, 255
	var totalTricks uint8 = 0
	for _, tricks := range state.TricksWon {
		totalTricks += tricks
		if tricks < first {
			second = first
			first = tricks
		} else if tricks < second {
			second = tricks
		}
	}
	if totalTricks == 0 || second == 255 {
		return 0
	}
	return float32(second-first) / float32(totalTricks)
}

// ChipLeaderDetector - for betting games (Poker variants)
// More chips = winning
type ChipLeaderDetector struct{}

func (d *ChipLeaderDetector) GetLeader(state *GameState) int {
	if len(state.Players) < 2 {
		return -1
	}
	maxChips := state.Players[0].Chips
	leader := 0
	tied := false
	for i := 1; i < len(state.Players); i++ {
		if state.Players[i].Chips > maxChips {
			maxChips = state.Players[i].Chips
			leader = i
			tied = false
		} else if state.Players[i].Chips == maxChips {
			tied = true
		}
	}
	if tied {
		return -1
	}
	return leader
}

func (d *ChipLeaderDetector) GetMargin(state *GameState) float32 {
	if len(state.Players) < 2 {
		return 0
	}
	var first, second int64 = 0, 0
	var totalChips int64 = 0
	for _, p := range state.Players {
		totalChips += p.Chips
		if p.Chips > first {
			second = first
			first = p.Chips
		} else if p.Chips > second {
			second = p.Chips
		}
	}
	if totalChips == 0 {
		return 0
	}
	return float32(first-second) / float32(totalChips)
}

// Update called after each turn in the game loop
func (tm *TensionMetrics) Update(state *GameState, detector LeaderDetector) {
	newLeader := detector.GetLeader(state)
	margin := detector.GetMargin(state)

	// Track closest margin seen (smaller = more tension)
	if margin < tm.ClosestMargin {
		tm.ClosestMargin = margin
	}

	// Track lead changes (ignore ties)
	if newLeader != -1 && tm.currentLeader != -1 && newLeader != tm.currentLeader {
		tm.LeadChanges++
	}

	// Update current leader
	if newLeader != -1 {
		tm.currentLeader = newLeader
	}

	// Record leader for permanent lead calculation
	tm.leaderHistory = append(tm.leaderHistory, tm.currentLeader)
	tm.TotalTurns++
}

// Finalize computes DecisiveTurn and WinnerWasTrailing based on winner
// DecisiveTurn = first turn where winner took lead and NEVER lost it
// WinnerWasTrailing = true if winner was behind at game midpoint
func (tm *TensionMetrics) Finalize(winnerID int) {
	// Handle invalid winner (draw or error)
	if winnerID < 0 {
		tm.DecisiveTurn = tm.TotalTurns
		tm.WinnerWasTrailing = false
		return
	}

	// Empty history - game ended immediately
	if len(tm.leaderHistory) == 0 {
		tm.DecisiveTurn = tm.TotalTurns
		tm.WinnerWasTrailing = false
		return
	}

	// Calculate WinnerWasTrailing: check who was leading at midpoint
	midpoint := len(tm.leaderHistory) / 2
	if midpoint > 0 && midpoint < len(tm.leaderHistory) {
		midpointLeader := tm.leaderHistory[midpoint]
		// Winner was trailing if someone ELSE was leading at midpoint
		// (not a tie, and not the winner)
		tm.WinnerWasTrailing = midpointLeader != -1 && midpointLeader != winnerID
	} else {
		tm.WinnerWasTrailing = false
	}

	// Scan backwards to find when winner took permanent lead
	// We're looking for the last turn where someone OTHER than the winner was leading
	tm.DecisiveTurn = tm.TotalTurns
	foundOtherLeader := false
	lastWinnerLead := -1 // Track last turn winner was leading (scanning backwards)

	for i := len(tm.leaderHistory) - 1; i >= 0; i-- {
		if tm.leaderHistory[i] != winnerID && tm.leaderHistory[i] != -1 {
			// Found a turn where someone else was leading
			// Winner took permanent lead after this
			foundOtherLeader = true
			if i+1 < len(tm.leaderHistory) {
				tm.DecisiveTurn = i + 1
			}
			break
		}
		// Track when winner was leading (for the no-other-leader case)
		if tm.leaderHistory[i] == winnerID && lastWinnerLead == -1 {
			lastWinnerLead = i
		}
	}

	// If no other player was ever leading, find when winner FIRST took lead
	if !foundOtherLeader {
		winnerFirstLead := -1
		for i, leader := range tm.leaderHistory {
			if leader == winnerID {
				winnerFirstLead = i
				break
			}
		}

		if winnerFirstLead >= 0 {
			// Winner was leading from turn winnerFirstLead onwards
			tm.DecisiveTurn = winnerFirstLead
		} else {
			// Winner was NEVER explicitly leading (all ties until final resolution)
			// This means maximum tension - outcome was uncertain until the very end
			tm.DecisiveTurn = tm.TotalTurns
		}
	}
}

// DecisiveTurnPct returns decisive turn as percentage of game
func (tm *TensionMetrics) DecisiveTurnPct() float32 {
	if tm.TotalTurns == 0 {
		return 0
	}
	return float32(tm.DecisiveTurn) / float32(tm.TotalTurns)
}

// SelectLeaderDetector chooses the appropriate detector based on genome's win conditions and phases.
// Priority: WinConditions first (most reliable), then phase types, then default to ScoreLeaderDetector.
func SelectLeaderDetector(genome *Genome) LeaderDetector {
	// Check win conditions first - most reliable indicator of game type
	for _, wc := range genome.WinConditions {
		switch wc.WinType {
		case WinTypeEmptyHand:
			return &HandSizeLeaderDetector{}
		case WinTypeHighScore, WinTypeFirstToScore:
			return &ScoreLeaderDetector{}
		case WinTypeLowScore, WinTypeFewestTricks:
			return &TrickAvoidanceLeaderDetector{}
		case WinTypeMostTricks:
			return &TrickLeaderDetector{}
		case WinTypeMostChips, WinTypeBestHand:
			return &ChipLeaderDetector{}
		case WinTypeCaptureAll:
			// War-style: captured cards go back to hand, more cards = winning
			return &HandSizeMaxLeaderDetector{}
		case WinTypeMostCaptured:
			// Scopa-style: captured cards tracked via Score
			return &ScoreLeaderDetector{}
		}
	}

	// Check for betting games (have BettingPhase)
	for _, phase := range genome.TurnPhases {
		if phase.PhaseType == PhaseTypeBetting {
			return &ChipLeaderDetector{}
		}
	}

	// Check phases for trick-taking hints
	for _, phase := range genome.TurnPhases {
		if phase.PhaseType == PhaseTypeTrick {
			return &TrickLeaderDetector{}
		}
	}

	// Default to score-based
	return &ScoreLeaderDetector{}
}

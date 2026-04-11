package sim

// PhaseType identifies the current phase of a game turn.
type PhaseType uint8

const (
	PhasePlay    PhaseType = iota
	PhaseDraw
	PhaseDiscard
	PhaseTrick
	PhaseMeld
	PhaseEnd
)

// MoveType identifies what kind of action a move represents.
type MoveType uint8

const (
	MovePlay    MoveType = iota // Play a card from hand
	MoveDraw                    // Draw from deck
	MovePass                    // Pass / can't play
	MoveKnock                   // Knock (rummy)
	MoveMeld                    // Lay down a meld (rummy)
	MoveDiscard                 // Discard a card
)

// Move represents a player action.
type Move struct {
	Type    MoveType
	Cards   []Card // Cards involved (for play, meld, discard)
	PlayerID int
}

// EventType identifies game events for logging and fitness analysis.
type EventType uint8

const (
	EventCardPlayed EventType = iota
	EventCardDrawn
	EventTrickWon
	EventMeldLaid
	EventSpecialTriggered
	EventRoundEnd
)

// Event records something that happened during a game.
type Event struct {
	Type     EventType
	PlayerID int
	Cards    []Card
	Detail   string
}

// GameState holds the complete mutable state of a game in progress.
type GameState struct {
	Deck    []Card
	Hands   [][]Card
	Discard []Card
	Tableau [][]Card // Per-player tableau (tricks won, melds, etc.)
	Scores  []int

	Turn       int
	Active     int // Active player index
	Phase      PhaseType
	NumPlayers int

	// Round tracking (for multi-round games like trick-taking)
	Round    int
	MaxRound int

	// Shedding-specific
	TopCard *Card // Current top of discard pile for matching

	// Events log for fitness analysis
	Events []Event
}

// NewGameState creates a fresh game state for the given number of players.
func NewGameState(numPlayers int) *GameState {
	return &GameState{
		Hands:      make([][]Card, numPlayers),
		Tableau:    make([][]Card, numPlayers),
		Scores:     make([]int, numPlayers),
		NumPlayers: numPlayers,
		Events:     make([]Event, 0, 100),
	}
}

// ActiveHand returns the active player's hand.
func (gs *GameState) ActiveHand() []Card {
	return gs.Hands[gs.Active]
}

// NextPlayer advances to the next player.
func (gs *GameState) NextPlayer() {
	gs.Active = (gs.Active + 1) % gs.NumPlayers
}

// AddEvent records a game event.
func (gs *GameState) AddEvent(e Event) {
	gs.Events = append(gs.Events, e)
}

// SkeletonRunner is the interface that all skeleton game loops implement.
type SkeletonRunner interface {
	// Setup initializes the game state from a genome.
	Setup(genome interface{}, rng interface{}) *GameState

	// GenerateMoves returns all legal moves for the active player.
	GenerateMoves(state *GameState) []Move

	// ApplyMove applies a move and returns events that occurred.
	ApplyMove(state *GameState, move Move) []Event

	// CheckEnd checks if the game is over. Returns winner ID or -1.
	CheckEnd(state *GameState) int
}

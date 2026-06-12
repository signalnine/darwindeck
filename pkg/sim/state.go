package sim

import "math/rand/v2"

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

// TurnRecord captures per-turn decision data for fitness analysis (audit
// Task 7). The batch runner appends exactly one record per applied move.
type TurnRecord struct {
	Player     int   // Acting player at the decision point
	LegalMoves uint8 // Legal moves available to the acting player, capped at 255
	// OptionDelta is the change in the next actor's legal-move count caused
	// by this move: options(next, post-move) - options(next, pre-move).
	// Semantics are defined PER SKELETON (see the table in
	// docs/plans/2026-06-11-audit-remediation.md Task 7 and the probe helpers
	// in batch.go); trick-taking is always 0 by design.
	OptionDelta int8
	// Attack is true iff this move emitted at least one attack event
	// (IsAttackEvent: EventTrickWon, or EventSpecialTriggered with an
	// opponent-affecting detail). Set at record time by the batch runner so a
	// stacked special -- one card matching skip+reverse+draw rules emits up
	// to 3 attack events -- still counts as exactly ONE interactive turn
	// (audit Wave D fix 3).
	Attack bool
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

// attackSpecialDetails enumerates the EventSpecialTriggered Detail strings
// that directly affect an opponent. This is the complete set the shedding
// runner's applySpecialEffects emits (the only EventSpecialTriggered emitter
// in the codebase): draw penalties inflicted on the next player, a skip, and
// a reverse. A future self-targeted special (e.g. a wild-suit choice) must
// NOT be added here. SINGLE SOURCE OF TRUTH: fitness consumes attacks via
// TurnRecord.Attack / IsAttackEvent -- do not duplicate this whitelist
// elsewhere (audit Wave D fix 3).
var attackSpecialDetails = map[string]bool{
	"skip":      true,
	"draw_two":  true,
	"draw_four": true,
	"reverse":   true,
}

// IsAttackEvent reports whether e is a direct attack on an opponent: a trick
// win, or a special trigger with an opponent-affecting detail.
func IsAttackEvent(e Event) bool {
	switch e.Type {
	case EventTrickWon:
		return true
	case EventSpecialTriggered:
		return attackSpecialDetails[e.Detail]
	}
	return false
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
	// Direction is the play-order delta (+1 forward, -1 reversed). Zero is
	// treated as +1 for backwards compatibility with states that do not set
	// it explicitly.
	Direction int

	// Round tracking (for multi-round games like trick-taking)
	Round    int
	MaxRound int

	// Shedding-specific
	TopCard *Card // Current top of discard pile for matching

	// Trick-taking-specific
	TrickCards   []Card // Cards played in current trick
	TrickPlayers []int  // Player IDs corresponding to TrickCards
	TrickLeader  int    // Player who led the current trick
	TrumpSuit    int    // Trump suit (0-3), -1 = none, -2 = pending
	TrickBroken  bool   // Has trump been played off-suit?

	// Rummy-specific
	Melds     [][]Card // All melds on the table
	MeldOwner []int    // Owner of each meld

	// Events log for fitness analysis
	Events []Event

	// RNG used by runners that need to perform mid-game randomization
	// (e.g. trick-taking round re-deals, rummy stock reshuffles). Set in
	// Setup so CheckEnd/ApplyMove can reuse the game's seeded source.
	RNG *rand.Rand
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

// NextPlayer advances to the next player in the current play direction.
// Direction == 0 is treated as +1 so states constructed without explicit
// initialisation continue to advance forward.
func (gs *GameState) NextPlayer() {
	dir := gs.Direction
	if dir == 0 {
		dir = 1
	}
	gs.Active = ((gs.Active+dir)%gs.NumPlayers + gs.NumPlayers) % gs.NumPlayers
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

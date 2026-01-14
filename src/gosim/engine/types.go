package engine

import (
	"sync"
)

// Card represents a playing card (1 byte)
type Card struct {
	Rank uint8 // 0-12 (A,2-10,J,Q,K)
	Suit uint8 // 0-3 (H,D,C,S)
}

// Location enum
type Location uint8

const (
	LocationDeck Location = iota
	LocationHand
	LocationDiscard
	LocationTableau
	// Optional extensions
	LocationOpponentHand
	LocationOpponentDiscard
)

// PlayerState is mutable for performance
type PlayerState struct {
	Hand   []Card
	Score  int32
	Active bool // Still in the game (not folded/eliminated)
	// Optional extensions for betting games
	Chips      int64 // Chip/token count for betting games (int64 for precision)
	CurrentBet int64 // Current bet in this round (int64 for precision)
	HasFolded  bool  // Folded this round
	IsAllIn    bool  // Track all-in status (can't act but still in hand)
}

// Claim represents a bluffing claim for games like I Doubt It, Cheat, BS
type Claim struct {
	ClaimerID    uint8   // Who made the claim
	ClaimedRank  uint8   // Claimed rank (0-12 for A-K)
	ClaimedCount uint8   // Number of cards claimed
	CardsPlayed  []Card  // Actual cards played (for verification)
	Challenged   bool    // Has this claim been challenged?
	ChallengerID uint8   // Who challenged (if Challenged=true)
}

// TrickCard represents a card played to the current trick
type TrickCard struct {
	PlayerID uint8
	Card     Card
}

// GameState is mutable and pooled
type GameState struct {
	Players       []PlayerState
	Deck          []Card
	Discard       []Card
	Tableau       [][]Card // For games like War, Gin Rummy
	CurrentPlayer uint8
	TurnNumber    uint32
	WinnerID      int8 // -1 = no winner yet, 0/1 = player ID
	// Optional extensions for betting games
	Pot                int64 // Current pot size (int64 for precision)
	CurrentBet         int64 // Highest bet in current round (int64 for precision)
	RaiseCount         int   // Raises this round
	BettingStartPlayer int   // Rotates each hand for position fairness
	BettingComplete    bool  // True after betting round finishes (for blackjack: betting before draw)
	// Optional extensions for bluffing games
	CurrentClaim *Claim // nil if no active claim
	// Trick-taking game state
	CurrentTrick   []TrickCard // Cards played in current trick
	TrickLeader    uint8       // Who leads the current trick
	TricksWon      []uint8     // Count of tricks won by each player
	HeartsBroken   bool        // For Hearts: whether hearts have been played
	NumPlayers     uint8       // Number of players (for trick completion check)
	CardsPerPlayer int         // Cards dealt to each player (for hand size check)
	// Scopa/capture game state
	CaptureMode    bool        // If true, use Scopa capture mechanics instead of War
	// Special effects state
	PlayDirection int8  // 1 = clockwise, -1 = counter-clockwise
	SkipCount     uint8 // Number of players to skip (capped at NumPlayers-1)
	// Blackjack-specific state
	HasStood []bool // Track which players have stood (for blackjack)
	// President/climbing game state
	ConsecutivePasses int // Track consecutive passes (for clearing tableau)
}

// StatePool manages GameState memory
var StatePool = sync.Pool{
	New: func() interface{} {
		return &GameState{
			Players:      make([]PlayerState, 4), // Support up to 4 players
			Deck:         make([]Card, 0, 52),
			Discard:      make([]Card, 0, 52),
			Tableau:      make([][]Card, 0, 10),
			CurrentTrick: make([]TrickCard, 0, 4), // Max 4 players per trick
			TricksWon:    make([]uint8, 0, 4),     // Max 4 players
			HasStood:     make([]bool, 4),         // Max 4 players for blackjack
		}
	},
}

// GetState acquires a GameState from pool
func GetState() *GameState {
	state := StatePool.Get().(*GameState)
	state.Reset()
	return state
}

// PutState returns a GameState to pool
func PutState(state *GameState) {
	StatePool.Put(state)
}

// Reset clears state for reuse
func (s *GameState) Reset() {
	// Reset all 4 potential players
	for i := 0; i < len(s.Players); i++ {
		s.Players[i].Hand = s.Players[i].Hand[:0]
		s.Players[i].Score = 0
		s.Players[i].Active = true
		s.Players[i].Chips = 0
		s.Players[i].CurrentBet = 0
		s.Players[i].HasFolded = false
		s.Players[i].IsAllIn = false
	}

	s.Deck = s.Deck[:0]
	s.Discard = s.Discard[:0]
	s.Tableau = s.Tableau[:0]
	s.CurrentPlayer = 0
	s.TurnNumber = 0
	s.WinnerID = -1
	s.Pot = 0
	s.CurrentBet = 0
	s.RaiseCount = 0
	s.BettingComplete = false
	s.BettingStartPlayer = 0
	s.CurrentClaim = nil
	// Trick-taking state
	s.CurrentTrick = s.CurrentTrick[:0]
	s.TrickLeader = 0
	s.TricksWon = s.TricksWon[:0]
	s.HeartsBroken = false
	s.NumPlayers = 2
	s.CardsPerPlayer = 0
	s.CaptureMode = false
	s.PlayDirection = 1
	s.SkipCount = 0
	// Blackjack state
	for i := 0; i < len(s.HasStood); i++ {
		s.HasStood[i] = false
	}
	// President state
	s.ConsecutivePasses = 0
}

// Clone creates a deep copy for MCTS tree search
func (s *GameState) Clone() *GameState {
	clone := GetState()

	// Clone all active players
	numPlayers := int(s.NumPlayers)
	if numPlayers == 0 {
		numPlayers = 2 // Default fallback
	}
	for i := 0; i < numPlayers && i < len(s.Players); i++ {
		clone.Players[i].Hand = append(clone.Players[i].Hand, s.Players[i].Hand...)
		clone.Players[i].Score = s.Players[i].Score
		clone.Players[i].Active = s.Players[i].Active
		clone.Players[i].Chips = s.Players[i].Chips
		clone.Players[i].CurrentBet = s.Players[i].CurrentBet
		clone.Players[i].HasFolded = s.Players[i].HasFolded
		clone.Players[i].IsAllIn = s.Players[i].IsAllIn
	}

	clone.Deck = append(clone.Deck, s.Deck...)
	clone.Discard = append(clone.Discard, s.Discard...)

	for _, pile := range s.Tableau {
		tableuClone := make([]Card, len(pile))
		copy(tableuClone, pile)
		clone.Tableau = append(clone.Tableau, tableuClone)
	}

	clone.CurrentPlayer = s.CurrentPlayer
	clone.TurnNumber = s.TurnNumber
	clone.WinnerID = s.WinnerID
	clone.Pot = s.Pot
	clone.CurrentBet = s.CurrentBet
	clone.RaiseCount = s.RaiseCount
	clone.BettingStartPlayer = s.BettingStartPlayer

	// Clone claim if present
	if s.CurrentClaim != nil {
		clone.CurrentClaim = &Claim{
			ClaimerID:    s.CurrentClaim.ClaimerID,
			ClaimedRank:  s.CurrentClaim.ClaimedRank,
			ClaimedCount: s.CurrentClaim.ClaimedCount,
			CardsPlayed:  append([]Card{}, s.CurrentClaim.CardsPlayed...),
			Challenged:   s.CurrentClaim.Challenged,
			ChallengerID: s.CurrentClaim.ChallengerID,
		}
	}

	// Clone trick-taking state
	clone.CurrentTrick = append(clone.CurrentTrick, s.CurrentTrick...)
	clone.TrickLeader = s.TrickLeader
	clone.TricksWon = append(clone.TricksWon, s.TricksWon...)
	clone.HeartsBroken = s.HeartsBroken
	clone.NumPlayers = s.NumPlayers
	clone.CardsPerPlayer = s.CardsPerPlayer
	clone.CaptureMode = s.CaptureMode
	clone.PlayDirection = s.PlayDirection
	clone.SkipCount = s.SkipCount
	// Clone blackjack state
	for i := 0; i < len(s.HasStood) && i < len(clone.HasStood); i++ {
		clone.HasStood[i] = s.HasStood[i]
	}
	// Clone President state
	clone.ConsecutivePasses = s.ConsecutivePasses

	return clone
}

// InitializeChips sets up starting chips for all players
func (gs *GameState) InitializeChips(startingChips int) {
	for i := range gs.Players {
		gs.Players[i].Chips = int64(startingChips)
		gs.Players[i].CurrentBet = 0
		gs.Players[i].HasFolded = false
		gs.Players[i].IsAllIn = false
	}
	gs.Pot = 0
	gs.CurrentBet = 0
	gs.RaiseCount = 0
	gs.BettingStartPlayer = 0
}

// ResetHand resets betting state for a new hand while preserving chips
func (gs *GameState) ResetHand() {
	for i := range gs.Players {
		gs.Players[i].CurrentBet = 0
		gs.Players[i].HasFolded = false
		gs.Players[i].IsAllIn = false
	}
	gs.Pot = 0
	gs.CurrentBet = 0
	gs.RaiseCount = 0
	gs.BettingComplete = false
	gs.BettingStartPlayer = (gs.BettingStartPlayer + 1) % len(gs.Players)
}

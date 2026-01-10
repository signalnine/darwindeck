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
	Pot        int64 // Current pot size (int64 for precision)
	CurrentBet int64 // Highest bet in current round (int64 for precision)
	// Optional extensions for bluffing games
	CurrentClaim *Claim // nil if no active claim
}

// StatePool manages GameState memory
var StatePool = sync.Pool{
	New: func() interface{} {
		return &GameState{
			Players: make([]PlayerState, 2),
			Deck:    make([]Card, 0, 52),
			Discard: make([]Card, 0, 52),
			Tableau: make([][]Card, 0, 10),
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
	s.Players[0].Hand = s.Players[0].Hand[:0]
	s.Players[0].Score = 0
	s.Players[0].Active = true
	s.Players[0].Chips = 0
	s.Players[0].CurrentBet = 0
	s.Players[0].HasFolded = false

	s.Players[1].Hand = s.Players[1].Hand[:0]
	s.Players[1].Score = 0
	s.Players[1].Active = true
	s.Players[1].Chips = 0
	s.Players[1].CurrentBet = 0
	s.Players[1].HasFolded = false

	s.Deck = s.Deck[:0]
	s.Discard = s.Discard[:0]
	s.Tableau = s.Tableau[:0]
	s.CurrentPlayer = 0
	s.TurnNumber = 0
	s.WinnerID = -1
	s.Pot = 0
	s.CurrentBet = 0
	s.CurrentClaim = nil
}

// Clone creates a deep copy for MCTS tree search
func (s *GameState) Clone() *GameState {
	clone := GetState()

	clone.Players[0].Hand = append(clone.Players[0].Hand, s.Players[0].Hand...)
	clone.Players[0].Score = s.Players[0].Score
	clone.Players[0].Active = s.Players[0].Active
	clone.Players[0].Chips = s.Players[0].Chips
	clone.Players[0].CurrentBet = s.Players[0].CurrentBet
	clone.Players[0].HasFolded = s.Players[0].HasFolded

	clone.Players[1].Hand = append(clone.Players[1].Hand, s.Players[1].Hand...)
	clone.Players[1].Score = s.Players[1].Score
	clone.Players[1].Active = s.Players[1].Active
	clone.Players[1].Chips = s.Players[1].Chips
	clone.Players[1].CurrentBet = s.Players[1].CurrentBet
	clone.Players[1].HasFolded = s.Players[1].HasFolded

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

	return clone
}

package engine

import (
	"testing"
)

func TestStatePool(t *testing.T) {
	// Acquire and release
	s1 := GetState()
	if len(s1.Players) != 4 {
		t.Errorf("Expected 4 players, got %d", len(s1.Players))
	}

	PutState(s1)

	// Should get same instance back
	s2 := GetState()
	if &s1.Players[0] != &s2.Players[0] {
		t.Error("Pool did not reuse memory")
	}

	PutState(s2)
}

func TestGameStateClone(t *testing.T) {
	s1 := GetState()
	s1.Players[0].Hand = append(s1.Players[0].Hand, Card{Rank: 0, Suit: 0})
	s1.Deck = append(s1.Deck, Card{Rank: 1, Suit: 1})

	s2 := s1.Clone()

	// Modify original
	s1.Players[0].Hand[0].Rank = 12
	s1.Deck[0].Suit = 3

	// Clone should be unchanged
	if s2.Players[0].Hand[0].Rank != 0 {
		t.Error("Clone was not deep copied")
	}
	if s2.Deck[0].Suit != 1 {
		t.Error("Clone deck was not deep copied")
	}

	PutState(s1)
	PutState(s2)
}

func TestDrawAndPlay(t *testing.T) {
	s := GetState()
	s.Deck = append(s.Deck, Card{Rank: 5, Suit: 2})

	// Draw card
	if !s.DrawCard(0, LocationDeck) {
		t.Error("Failed to draw card")
	}

	if len(s.Players[0].Hand) != 1 {
		t.Errorf("Expected 1 card in hand, got %d", len(s.Players[0].Hand))
	}

	// Play card
	if !s.PlayCard(0, 0, LocationDiscard) {
		t.Error("Failed to play card")
	}

	if len(s.Players[0].Hand) != 0 {
		t.Error("Hand should be empty after playing")
	}

	if len(s.Discard) != 1 {
		t.Errorf("Expected 1 card in discard, got %d", len(s.Discard))
	}

	PutState(s)
}

func TestGameStateHasEffectFields(t *testing.T) {
	state := GetState()
	defer PutState(state)

	// New fields should exist and have defaults
	if state.PlayDirection != 1 {
		t.Errorf("PlayDirection should default to 1, got %d", state.PlayDirection)
	}
	if state.SkipCount != 0 {
		t.Errorf("SkipCount should default to 0, got %d", state.SkipCount)
	}
}

func TestGameStateClonePreservesEffectFields(t *testing.T) {
	state := GetState()
	state.PlayDirection = -1
	state.SkipCount = 2

	clone := state.Clone()
	defer PutState(state)
	defer PutState(clone)

	if clone.PlayDirection != -1 {
		t.Errorf("Clone PlayDirection should be -1, got %d", clone.PlayDirection)
	}
	if clone.SkipCount != 2 {
		t.Errorf("Clone SkipCount should be 2, got %d", clone.SkipCount)
	}
}

func TestGameStateHasTableauMode(t *testing.T) {
	state := NewGameState(2)

	// Default should be 0 (NONE)
	if state.TableauMode != 0 {
		t.Errorf("Expected TableauMode 0, got %d", state.TableauMode)
	}

	// Should be settable
	state.TableauMode = 1 // WAR
	if state.TableauMode != 1 {
		t.Errorf("Expected TableauMode 1, got %d", state.TableauMode)
	}

	state.SequenceDirection = 2 // BOTH
	if state.SequenceDirection != 2 {
		t.Errorf("Expected SequenceDirection 2, got %d", state.SequenceDirection)
	}
}

func TestGameStateTeamScores(t *testing.T) {
	state := &GameState{
		NumPlayers:   4,
		TeamScores:   []int32{0, 0},
		PlayerToTeam: []int8{0, 1, 0, 1}, // Players 0,2 on team 0; Players 1,3 on team 1
		WinningTeam:  -1,
	}

	if len(state.TeamScores) != 2 {
		t.Errorf("Expected 2 team scores, got %d", len(state.TeamScores))
	}
	if state.PlayerToTeam[0] != 0 || state.PlayerToTeam[2] != 0 {
		t.Error("Players 0 and 2 should be on team 0")
	}
	if state.PlayerToTeam[1] != 1 || state.PlayerToTeam[3] != 1 {
		t.Error("Players 1 and 3 should be on team 1")
	}
	if state.WinningTeam != -1 {
		t.Errorf("WinningTeam should be -1 initially, got %d", state.WinningTeam)
	}
}

func TestBuildPlayerToTeamLookup(t *testing.T) {
	teams := [][]int{{0, 2}, {1, 3}}
	lookup := BuildPlayerToTeamLookup(teams, 4)

	if len(lookup) != 4 {
		t.Errorf("Expected lookup for 4 players, got %d", len(lookup))
	}
	if lookup[0] != 0 || lookup[2] != 0 {
		t.Error("Players 0 and 2 should map to team 0")
	}
	if lookup[1] != 1 || lookup[3] != 1 {
		t.Error("Players 1 and 3 should map to team 1")
	}
}

func TestBuildPlayerToTeamLookupEmpty(t *testing.T) {
	// Test with empty teams
	lookup := BuildPlayerToTeamLookup([][]int{}, 4)

	if len(lookup) != 4 {
		t.Errorf("Expected lookup for 4 players, got %d", len(lookup))
	}
	for i, v := range lookup {
		if v != -1 {
			t.Errorf("Player %d should map to -1 (no team), got %d", i, v)
		}
	}
}

func TestBuildPlayerToTeamLookupOutOfBounds(t *testing.T) {
	// Test with out-of-bounds player indices (should be ignored)
	teams := [][]int{{0, 10}, {1, -5}} // 10 and -5 are out of bounds for 4 players
	lookup := BuildPlayerToTeamLookup(teams, 4)

	if len(lookup) != 4 {
		t.Errorf("Expected lookup for 4 players, got %d", len(lookup))
	}
	if lookup[0] != 0 {
		t.Errorf("Player 0 should map to team 0, got %d", lookup[0])
	}
	if lookup[1] != 1 {
		t.Errorf("Player 1 should map to team 1, got %d", lookup[1])
	}
	// Out-of-bounds indices should be ignored, leaving default -1
	if lookup[2] != -1 {
		t.Errorf("Player 2 should be -1 (no team), got %d", lookup[2])
	}
	if lookup[3] != -1 {
		t.Errorf("Player 3 should be -1 (no team), got %d", lookup[3])
	}
}

func TestInitializeTeams(t *testing.T) {
	state := &GameState{
		NumPlayers: 4,
	}
	teams := [][]int{{0, 2}, {1, 3}}
	state.InitializeTeams(teams)

	if len(state.TeamScores) != 2 {
		t.Errorf("Expected 2 team scores, got %d", len(state.TeamScores))
	}
	if len(state.PlayerToTeam) != 4 {
		t.Errorf("Expected PlayerToTeam for 4 players, got %d", len(state.PlayerToTeam))
	}
	if state.WinningTeam != -1 {
		t.Errorf("WinningTeam should be -1 after init, got %d", state.WinningTeam)
	}
}

func TestInitializeTeamsEmpty(t *testing.T) {
	state := &GameState{
		NumPlayers: 4,
	}
	state.InitializeTeams([][]int{})

	if state.TeamScores != nil {
		t.Errorf("Expected nil TeamScores for empty teams, got %v", state.TeamScores)
	}
	if state.PlayerToTeam != nil {
		t.Errorf("Expected nil PlayerToTeam for empty teams, got %v", state.PlayerToTeam)
	}
	if state.WinningTeam != -1 {
		t.Errorf("WinningTeam should be -1, got %d", state.WinningTeam)
	}
}

func TestGameStateCloneWithTeams(t *testing.T) {
	original := &GameState{
		NumPlayers:   4,
		Players:      make([]PlayerState, 4),
		TeamScores:   []int32{10, 20},
		PlayerToTeam: []int8{0, 1, 0, 1},
		WinningTeam:  -1,
	}

	clone := original.Clone()

	// Modify clone
	clone.TeamScores[0] = 100
	clone.PlayerToTeam[0] = 1

	// Original should be unchanged
	if original.TeamScores[0] != 10 {
		t.Error("Clone modified original TeamScores")
	}
	if original.PlayerToTeam[0] != 0 {
		t.Error("Clone modified original PlayerToTeam")
	}
}

func TestGameStateCloneWithNilTeams(t *testing.T) {
	original := &GameState{
		NumPlayers:   2,
		Players:      make([]PlayerState, 4),
		TeamScores:   nil,
		PlayerToTeam: nil,
		WinningTeam:  -1,
	}

	clone := original.Clone()

	if clone.TeamScores != nil {
		t.Errorf("Clone should have nil TeamScores, got %v", clone.TeamScores)
	}
	if clone.PlayerToTeam != nil {
		t.Errorf("Clone should have nil PlayerToTeam, got %v", clone.PlayerToTeam)
	}
	if clone.WinningTeam != -1 {
		t.Errorf("Clone WinningTeam should be -1, got %d", clone.WinningTeam)
	}
}

func TestGameStateResetClearsTeams(t *testing.T) {
	state := GetState()
	state.TeamScores = []int32{10, 20}
	state.PlayerToTeam = []int8{0, 1, 0, 1}
	state.WinningTeam = 1

	state.Reset()

	// After reset, team fields should be cleared
	if state.TeamScores != nil {
		t.Errorf("Expected nil TeamScores after reset, got %v", state.TeamScores)
	}
	if state.PlayerToTeam != nil {
		t.Errorf("Expected nil PlayerToTeam after reset, got %v", state.PlayerToTeam)
	}
	if state.WinningTeam != -1 {
		t.Errorf("WinningTeam should be -1 after reset, got %d", state.WinningTeam)
	}

	PutState(state)
}

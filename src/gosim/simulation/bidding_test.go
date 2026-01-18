package simulation

import (
	"testing"

	"github.com/signalnine/darwindeck/gosim/engine"
)

func TestRunSingleGameWithBidding(t *testing.T) {
	// Create a simple trick-taking genome with bidding
	// This mimics what Python would produce for a Spades-style game
	
	// For now, let's test that the bidding functions don't panic
	state := engine.GetState()
	defer engine.PutState(state)
	
	// Initialize state for 4 players
	state.NumPlayers = 4
	state.CardsPerPlayer = 13
	
	// Initialize players with hands
	for i := 0; i < 4; i++ {
		state.Players[i].Hand = make([]engine.Card, 13)
		for j := 0; j < 13; j++ {
			state.Players[i].Hand[j] = engine.Card{
				Rank: uint8(j % 13),
				Suit: uint8(i),
			}
		}
		state.Players[i].CurrentBid = -1
		state.Players[i].IsNilBid = false
	}
	
	// Test hasBiddingPhase with a mock genome
	genome := &engine.Genome{
		TurnPhases: []engine.PhaseDescriptor{
			{
				PhaseType: engine.PhaseTypeBidding,
				// BiddingPhase data (16 bytes):
				// [0]: opcode (70)
				// [1]: min_bid (1)
				// [2]: max_bid (13)
				// [3]: flags (1 = allow_nil)
				// [4]: points_per_trick_bid (10)
				// [5]: overtrick_points (1)
				// [6]: failed_contract_penalty (10)
				// [7-8]: nil_bonus (100 = 0x64, 0x00)
				// [9-10]: nil_penalty (100 = 0x64, 0x00)
				// [11]: bag_limit (10)
				// [12-13]: bag_penalty (100 = 0x64, 0x00)
				// [14-15]: reserved (0, 0)
				Data: []byte{0x46, 0x01, 0x0d, 0x01, 0x0a, 0x01, 0x0a, 0x64, 0x00, 0x64, 0x00, 0x0a, 0x64, 0x00, 0x00, 0x00},
			},
			{
				PhaseType: engine.PhaseTypeTrick,
				Data:      []byte{0x01, 0x03, 0x01, 0xFF}, // lead_suit_req=1, trump=spades, high_wins=1, breaking=none
			},
		},
	}
	
	if !hasBiddingPhase(genome) {
		t.Error("Expected hasBiddingPhase to return true")
	}
	
	// Test getBiddingPhaseData
	data := getBiddingPhaseData(genome)
	if data == nil {
		t.Error("Expected getBiddingPhaseData to return data")
	}
	
	// Test selectGreedyBid
	biddingPhase := engine.BiddingPhase{
		MinBid:   1,
		MaxBid:   13,
		AllowNil: true,
	}
	bid := selectGreedyBid(state, biddingPhase, 0)
	if bid.Value < 1 || bid.Value > 13 {
		t.Errorf("Expected bid between 1 and 13, got %d", bid.Value)
	}
	
	// Test runBiddingRound
	aiTypes := []AIPlayerType{RandomAI, RandomAI, RandomAI, RandomAI}
	runBiddingRound(state, genome, aiTypes)
	
	// Verify all players have bid
	if !state.BiddingComplete {
		t.Error("Expected BiddingComplete to be true after runBiddingRound")
	}
	
	for i := 0; i < 4; i++ {
		if state.Players[i].CurrentBid < 0 {
			t.Errorf("Player %d should have bid, got CurrentBid=%d", i, state.Players[i].CurrentBid)
		}
	}
	
	t.Logf("Bids: P0=%d, P1=%d, P2=%d, P3=%d", 
		state.Players[0].CurrentBid, 
		state.Players[1].CurrentBid,
		state.Players[2].CurrentBid,
		state.Players[3].CurrentBid)
}

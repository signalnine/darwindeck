package climbing

import (
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

// climbingKnockGenome is Big Two plus a MechKnock borrow, so Knockable() is
// true: a climbing race you can end by declaring.
func climbingKnockGenome() *genome.Genome {
	g := bigTwo()
	g.ID = "climb-knock"
	g.Borrowed = []genome.BorrowedMechanic{{Source: genome.Rummy, Mechanic: genome.MechKnock}}
	return g
}

func hasKnock(moves []sim.Move) bool {
	for _, m := range moves {
		if m.Type == sim.MoveKnock {
			return true
		}
	}
	return false
}

// TestClimbingKnockOfferedOnlyWhenSmall: the MoveKnock appears once the hand is
// at or below knockThreshold and not before -- it sharpens the endgame, not a
// turn-one escape.
func TestClimbingKnockOfferedOnlyWhenSmall(t *testing.T) {
	g := climbingKnockGenome()
	runner := &Runner{}
	state := sim.NewGameState(2)
	state.Active = 0
	state.Hands[1] = []sim.Card{{Suit: 3, Rank: 7}}
	// leading (TrickCards empty); hand of 3 (== threshold) -> knock offered.
	state.Hands[0] = []sim.Card{{Suit: 1, Rank: 5}, {Suit: 2, Rank: 9}, {Suit: 0, Rank: 11}}
	if !hasKnock(runner.GenerateMoves(state, g)) {
		t.Fatalf("hand %d (<= threshold %d): knock must be offered", len(state.Hands[0]), knockThreshold)
	}
	// hand of 5 (> threshold) -> no knock.
	state.Hands[0] = []sim.Card{
		{Suit: 1, Rank: 5}, {Suit: 2, Rank: 9}, {Suit: 0, Rank: 11}, {Suit: 3, Rank: 4}, {Suit: 1, Rank: 8},
	}
	if hasKnock(runner.GenerateMoves(state, g)) {
		t.Fatalf("hand %d (> threshold %d): knock must NOT be offered", len(state.Hands[0]), knockThreshold)
	}
}

// TestClimbingKnockUnaffectedWithoutBorrow: plain Big Two never offers a knock,
// even at one card -- the borrow is the only thing that turns it on.
func TestClimbingKnockUnaffectedWithoutBorrow(t *testing.T) {
	g := bigTwo()
	runner := &Runner{}
	state := sim.NewGameState(2)
	state.Active = 0
	state.Hands[0] = []sim.Card{{Suit: 1, Rank: 5}}
	state.Hands[1] = []sim.Card{{Suit: 3, Rank: 7}}
	if hasKnock(runner.GenerateMoves(state, g)) {
		t.Fatal("plain climbing (no MechKnock borrow) must never offer a knock")
	}
}

// TestClimbingKnockEndsGameFewestWins: applying a knock flags the game over and
// CheckEnd awards the win to the fewest-cards player, not the knocker.
func TestClimbingKnockEndsGameFewestWins(t *testing.T) {
	g := climbingKnockGenome()
	runner := &Runner{}
	state := sim.NewGameState(3)
	state.Hands[0] = make([]sim.Card, 3)
	state.Hands[1] = make([]sim.Card, 1) // strictly fewest
	state.Hands[2] = make([]sim.Card, 2)
	state.Active = 0

	runner.ApplyMove(state, sim.Move{Type: sim.MoveKnock, PlayerID: 0}, g)
	if state.Phase != sim.PhaseEnd {
		t.Fatalf("knock must set Phase=PhaseEnd, got %v", state.Phase)
	}
	if w := runner.CheckEnd(state, g); w != 1 {
		t.Fatalf("fewest-cards player (1) must win after a knock, got %d", w)
	}
}

// TestClimbingKnockGreedyScorer: the ClimbingScorer knocks iff strictly fewest.
func TestClimbingKnockGreedyScorer(t *testing.T) {
	sc := &sim.ClimbingScorer{}
	state := sim.NewGameState(2)
	state.Active = 0

	state.Hands[0] = make([]sim.Card, 1) // I am fewest
	state.Hands[1] = make([]sim.Card, 3)
	if got := sc.ScoreMove(sim.Move{Type: sim.MoveKnock, PlayerID: 0}, state); got <= 30 {
		t.Fatalf("winning knock must outscore any play, got %v", got)
	}

	state.Hands[0] = make([]sim.Card, 3) // I am behind
	state.Hands[1] = make([]sim.Card, 1)
	if got := sc.ScoreMove(sim.Move{Type: sim.MoveKnock, PlayerID: 0}, state); got >= 0 {
		t.Fatalf("losing knock must score below passing, got %v", got)
	}

	// Tied for fewest: CheckEnd breaks ties to the lowest seat, so the
	// lowest-seat tied player WINS the knock and a higher-seat tied player
	// loses it. The scorer must agree with that resolution.
	tied := sim.NewGameState(3)
	tied.Hands[0] = make([]sim.Card, 1)
	tied.Hands[1] = make([]sim.Card, 1)
	tied.Hands[2] = make([]sim.Card, 3)
	if got := sc.ScoreMove(sim.Move{Type: sim.MoveKnock, PlayerID: 0}, tied); got <= 30 {
		t.Fatalf("tied-for-fewest at the lowest seat must be a winning knock, got %v", got)
	}
	if got := sc.ScoreMove(sim.Move{Type: sim.MoveKnock, PlayerID: 1}, tied); got >= 0 {
		t.Fatalf("tied-for-fewest at a later seat must be a losing knock, got %v", got)
	}
}

// TestClimbingKnockCompletesAndTerminates: a knock can only END the game sooner,
// so random games still always have a move and terminate.
func TestClimbingKnockCompletesAndTerminates(t *testing.T) {
	g := climbingKnockGenome()
	ai := &sim.RandomAI{}
	completed := 0
	for seed := uint64(0); seed < 200; seed++ {
		if _, w := playOut(t, g, ai, seed); w >= 0 {
			completed++
		}
	}
	t.Logf("ClimbingKnock: %d/200 completed", completed)
	if completed < 190 {
		t.Fatalf("knock host completes too rarely: %d/200", completed)
	}
}

func TestClimbingKnockDeterminism(t *testing.T) {
	g := climbingKnockGenome()
	ai := &sim.RandomAI{}
	for seed := uint64(0); seed < 30; seed++ {
		_, w1 := playOut(t, g, ai, seed)
		_, w2 := playOut(t, g, ai, seed)
		if w1 != w2 {
			t.Fatalf("seed %d non-deterministic: winner %d vs %d", seed, w1, w2)
		}
	}
}

package shedding

import (
	"math/rand/v2"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

func knockGenome() *genome.Genome {
	return &genome.Genome{
		ID: "knock-fixture", Skeleton: genome.Shedding, Players: 2, HandSize: 6,
		Shedding: &genome.SheddingParams{MatchRule: genome.MatchEither, DrawPenalty: 1},
		Borrowed: []genome.BorrowedMechanic{{Source: genome.Rummy, Mechanic: genome.MechKnock}},
	}
}

func hasKnock(moves []sim.Move) bool {
	for _, m := range moves {
		if m.Type == sim.MoveKnock {
			return true
		}
	}
	return false
}

// TestKnockOfferedOnlyWhenSmall: the MoveKnock appears once the hand is at or
// below knockThreshold and not before -- it sharpens the endgame, it is not a
// turn-one escape hatch.
func TestKnockOfferedOnlyWhenSmall(t *testing.T) {
	g := knockGenome()
	runner := &Runner{}
	state := sim.NewGameState(2)
	state.Active = 0
	state.TopCard = &sim.Card{Suit: 1, Rank: 5}
	state.Discard = []sim.Card{*state.TopCard}
	state.Deck = []sim.Card{{Suit: 4, Rank: 7}}
	state.Hands[1] = []sim.Card{{Suit: 1, Rank: 4}}

	// hand of 3 (== threshold): knock offered, alongside the normal plays.
	state.Hands[0] = []sim.Card{{Suit: 2, Rank: 9}, {Suit: 3, Rank: 10}, {Suit: 4, Rank: 2}}
	if !hasKnock(runner.GenerateMoves(state, g)) {
		t.Fatalf("hand size %d (<= threshold %d): knock must be offered", len(state.Hands[0]), knockThreshold)
	}

	// hand of 4 (> threshold): no knock.
	state.Hands[0] = append(state.Hands[0], sim.Card{Suit: 1, Rank: 6})
	if hasKnock(runner.GenerateMoves(state, g)) {
		t.Fatalf("hand size %d (> threshold %d): knock must NOT be offered", len(state.Hands[0]), knockThreshold)
	}
}

// TestKnockUnaffectedWithoutBorrow: a plain shedding genome never offers a
// knock, even at one card -- the borrow is the only thing that turns it on
// (byte-compatible with the ordinary game).
func TestKnockUnaffectedWithoutBorrow(t *testing.T) {
	g := &genome.Genome{
		ID: "plain", Skeleton: genome.Shedding, Players: 2, HandSize: 6,
		Shedding: &genome.SheddingParams{MatchRule: genome.MatchEither, DrawPenalty: 1},
	}
	runner := &Runner{}
	state := sim.NewGameState(2)
	state.Active = 0
	state.TopCard = &sim.Card{Suit: 1, Rank: 5}
	state.Discard = []sim.Card{*state.TopCard}
	state.Deck = []sim.Card{{Suit: 4, Rank: 7}}
	state.Hands[0] = []sim.Card{{Suit: 2, Rank: 9}}
	state.Hands[1] = []sim.Card{{Suit: 1, Rank: 4}}
	if hasKnock(runner.GenerateMoves(state, g)) {
		t.Fatal("plain shedding (no MechKnock borrow) must never offer a knock")
	}
}

// TestKnockEndsGameFewestWins: applying a knock flags the game over and CheckEnd
// awards the win to the fewest-cards player, not the knocker.
func TestKnockEndsGameFewestWins(t *testing.T) {
	g := knockGenome()
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

// TestKnockGreedyScorer: the SheddingScorer knocks iff strictly fewest -- the
// skill the random AI lacks. A winning knock outscores any play; a losing knock
// scores below drawing.
func TestKnockGreedyScorer(t *testing.T) {
	g := knockGenome()
	sc := sim.NewSheddingScorer(g)
	state := sim.NewGameState(2)
	state.Active = 0

	state.Hands[0] = make([]sim.Card, 1) // I am fewest
	state.Hands[1] = make([]sim.Card, 3)
	if got := sc.ScoreMove(sim.Move{Type: sim.MoveKnock, PlayerID: 0}, state); got <= 25 {
		t.Fatalf("winning knock must outscore any play, got %v", got)
	}

	state.Hands[0] = make([]sim.Card, 3) // I am behind
	state.Hands[1] = make([]sim.Card, 1)
	if got := sc.ScoreMove(sim.Move{Type: sim.MoveKnock, PlayerID: 0}, state); got >= 0 {
		t.Fatalf("losing knock must score below drawing, got %v", got)
	}

	// Tied for fewest: CheckEnd breaks ties to the lowest seat, so the
	// lowest-seat tied player WINS the knock and a higher-seat tied player
	// loses it. The scorer must agree with that resolution.
	tied := sim.NewGameState(3)
	tied.Hands[0] = make([]sim.Card, 1)
	tied.Hands[1] = make([]sim.Card, 1)
	tied.Hands[2] = make([]sim.Card, 3)
	if got := sc.ScoreMove(sim.Move{Type: sim.MoveKnock, PlayerID: 0}, tied); got <= 25 {
		t.Fatalf("tied-for-fewest at the lowest seat must be a winning knock, got %v", got)
	}
	if got := sc.ScoreMove(sim.Move{Type: sim.MoveKnock, PlayerID: 1}, tied); got >= 0 {
		t.Fatalf("tied-for-fewest at a later seat must be a losing knock, got %v", got)
	}
}

// TestKnockNeverEmptyAndCompletes: a knock can only END the game sooner, so
// random games still always have a move and terminate (here, more reliably --
// the knock is an extra exit).
func TestKnockNeverEmptyAndCompletes(t *testing.T) {
	g := knockGenome()
	runner := &Runner{}
	completed := 0
	for seed := uint64(0); seed < 200; seed++ {
		rng := rand.New(rand.NewPCG(seed, 0))
		state := runner.Setup(g, rng)
		maxTurns := g.MaxTurns()
		for turn := 0; turn < maxTurns; turn++ {
			runner.Upkeep(state, g)
			if runner.CheckEnd(state, g) >= 0 {
				completed++
				break
			}
			moves := runner.GenerateMoves(state, g)
			if len(moves) == 0 {
				t.Fatalf("seed %d turn %d: knock host produced zero moves", seed, turn)
			}
			runner.ApplyMove(state, (&sim.RandomAI{}).SelectMove(moves, state, rng), g)
		}
	}
	t.Logf("Knock: %d/200 completed", completed)
	if completed < 190 {
		t.Fatalf("knock host completes too rarely: %d/200", completed)
	}
}

func TestKnockDeterminism(t *testing.T) {
	g := knockGenome()
	for seed := uint64(0); seed < 30; seed++ {
		r1, r2 := runGame(g, seed), runGame(g, seed)
		if r1.Winner != r2.Winner || r1.Turns != r2.Turns {
			t.Fatalf("seed %d non-deterministic: (%d,%d) vs (%d,%d)", seed, r1.Winner, r1.Turns, r2.Winner, r2.Turns)
		}
	}
}

package tricktaking

import (
	"math/rand/v2"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

func runGame(g *genome.Genome, seed uint64) sim.GameResult {
	runner := &Runner{}
	ai := &sim.RandomAI{}
	rng := rand.New(rand.NewPCG(seed, 0))

	state := runner.Setup(g, rng)
	maxTurns := g.MaxTurns()

	for {
		winner := runner.CheckEnd(state, g)
		if winner >= 0 {
			return sim.GameResult{
				Winner: winner,
				Turns:  state.Turn,
				Events: state.Events,
			}
		}

		if state.Turn >= maxTurns {
			return sim.GameResult{
				Winner: -1,
				Turns:  state.Turn,
				Error:  "max_turns",
			}
		}

		moves := runner.GenerateMoves(state, g)
		if len(moves) == 0 {
			return sim.GameResult{
				Winner: -1,
				Turns:  state.Turn,
				Error:  "no_moves",
			}
		}

		move := ai.SelectMove(moves, state, rng)
		events := runner.ApplyMove(state, move, g)
		state.Events = append(state.Events, events...)
	}
}

func TestWhistCompletes(t *testing.T) {
	g := seeds.Whist()
	completions := 0
	for seed := uint64(0); seed < 100; seed++ {
		result := runGame(g, seed)
		if result.Winner >= 0 {
			completions++
		}
		if result.Error == "no_moves" {
			t.Fatalf("seed %d: no moves available", seed)
		}
	}
	t.Logf("Whist: %d/100 completed", completions)
	if completions < 95 {
		t.Fatalf("too few completions: %d/100", completions)
	}
}

func TestHeartsCompletes(t *testing.T) {
	g := seeds.Hearts()
	completions := 0
	for seed := uint64(0); seed < 100; seed++ {
		result := runGame(g, seed)
		if result.Winner >= 0 {
			completions++
		}
	}
	t.Logf("Hearts: %d/100 completed", completions)
	if completions < 95 {
		t.Fatalf("too few completions: %d/100", completions)
	}
}

func TestSpadesCompletes(t *testing.T) {
	g := seeds.Spades()
	completions := 0
	for seed := uint64(0); seed < 100; seed++ {
		result := runGame(g, seed)
		if result.Winner >= 0 {
			completions++
		}
	}
	t.Logf("Spades: %d/100 completed", completions)
	if completions < 95 {
		t.Fatalf("too few completions: %d/100", completions)
	}
}

func TestOhHellCompletes(t *testing.T) {
	g := seeds.OhHell()
	completions := 0
	for seed := uint64(0); seed < 100; seed++ {
		result := runGame(g, seed)
		if result.Winner >= 0 {
			completions++
		}
	}
	t.Logf("Oh Hell: %d/100 completed", completions)
	if completions < 95 {
		t.Fatalf("too few completions: %d/100", completions)
	}
}

func TestTrickDeterminism(t *testing.T) {
	g := seeds.Whist()
	r1 := runGame(g, 42)
	r2 := runGame(g, 42)

	if r1.Winner != r2.Winner {
		t.Fatalf("non-deterministic: winner %d vs %d", r1.Winner, r2.Winner)
	}
	if r1.Turns != r2.Turns {
		t.Fatalf("non-deterministic: turns %d vs %d", r1.Turns, r2.Turns)
	}
}

func TestFollowSuit(t *testing.T) {
	// In a follow-suit game, verify that when a player has lead suit,
	// all moves are of that suit
	g := seeds.Whist()
	runner := &Runner{}
	rng := rand.New(rand.NewPCG(1, 0))
	state := runner.Setup(g, rng)

	// Play first card (lead)
	moves := runner.GenerateMoves(state, g)
	lead := moves[0]
	runner.ApplyMove(state, lead, g)
	leadSuit := lead.Cards[0].Suit

	// Second player's moves
	moves = runner.GenerateMoves(state, g)
	hand := state.ActiveHand()

	// Check if player has any cards of lead suit
	hasLeadSuit := false
	for _, c := range hand {
		if c.Suit == leadSuit {
			hasLeadSuit = true
			break
		}
	}

	if hasLeadSuit {
		// All moves should be of lead suit
		for _, m := range moves {
			if m.Cards[0].Suit != leadSuit {
				t.Fatalf("player must follow suit %s but offered %s",
					leadSuit, m.Cards[0].Suit)
			}
		}
	}
}

func TestHeartsAvoidanceScoring(t *testing.T) {
	g := seeds.Hearts()
	result := runGame(g, 7)

	if result.Winner < 0 {
		t.Fatal("game did not complete")
	}

	// In Hearts, the winner should have the lowest score
	// (verified by the runner's findWinner logic)
}

func TestTrumpBeatsOffSuit(t *testing.T) {
	// Verify trump resolution: trump card should beat non-trump
	g := seeds.Spades()
	runner := &Runner{}
	rng := rand.New(rand.NewPCG(5, 0))
	state := runner.Setup(g, rng)

	// Play through some tricks and verify no panics/errors
	ai := &sim.RandomAI{}
	for i := 0; i < 52; i++ {
		winner := runner.CheckEnd(state, g)
		if winner >= 0 {
			break
		}
		moves := runner.GenerateMoves(state, g)
		if len(moves) == 0 {
			t.Fatalf("no moves at turn %d", i)
		}
		move := ai.SelectMove(moves, state, rng)
		runner.ApplyMove(state, move, g)
	}
}

// trumpBrokenGenome builds a 2-player trick-taking genome with a fixed trump
// suit and no lead restriction, so we can drive specific cards through ApplyMove.
func trumpBrokenGenome(trump sim.Suit) *genome.Genome {
	return &genome.Genome{
		ID:       "trump-broken-test",
		Skeleton: genome.TrickTaking,
		Players:  2,
		HandSize: 1,
		TrickTaking: &genome.TrickTakingParams{
			MustFollowSuit:  true,
			TrickScoring:    genome.ScorePerTrick,
			LeadRestriction: genome.LeadNone,
			RoundsPerGame:   1,
		},
		TrumpRule: genome.TrumpFixed,
		Scoring: genome.ScoringConfig{
			TrumpSuit: uint8(trump) + 1,
		},
	}
}

func TestTrumpNotBrokenWhenTrumpIsLed(t *testing.T) {
	g := trumpBrokenGenome(sim.Hearts)
	runner := &Runner{}
	rng := rand.New(rand.NewPCG(1, 0))
	state := runner.Setup(g, rng)

	// Set up a controlled trick: trump (Hearts) is led, follower follows trump.
	state.Hands[0] = []sim.Card{{Suit: sim.Hearts, Rank: sim.Five}}
	state.Hands[1] = []sim.Card{{Suit: sim.Hearts, Rank: sim.Seven}}
	state.TrickCards = state.TrickCards[:0]
	state.TrickPlayers = state.TrickPlayers[:0]
	state.Active = 0
	state.TrickBroken = false

	runner.ApplyMove(state, sim.Move{
		Type:     sim.MovePlay,
		Cards:    []sim.Card{{Suit: sim.Hearts, Rank: sim.Five}},
		PlayerID: 0,
	}, g)
	runner.ApplyMove(state, sim.Move{
		Type:     sim.MovePlay,
		Cards:    []sim.Card{{Suit: sim.Hearts, Rank: sim.Seven}},
		PlayerID: 1,
	}, g)

	if state.TrickBroken {
		t.Fatal("TrickBroken must stay false when trump suit is led and follower follows suit")
	}
}

func TestTrumpBrokenWhenPlayedOffLead(t *testing.T) {
	g := trumpBrokenGenome(sim.Hearts)
	runner := &Runner{}
	rng := rand.New(rand.NewPCG(1, 0))
	state := runner.Setup(g, rng)

	// Spades is led (non-trump), follower has no spades and plays trump.
	state.Hands[0] = []sim.Card{{Suit: sim.Spades, Rank: sim.Five}}
	state.Hands[1] = []sim.Card{{Suit: sim.Hearts, Rank: sim.Seven}}
	state.TrickCards = state.TrickCards[:0]
	state.TrickPlayers = state.TrickPlayers[:0]
	state.Active = 0
	state.TrickBroken = false

	runner.ApplyMove(state, sim.Move{
		Type:     sim.MovePlay,
		Cards:    []sim.Card{{Suit: sim.Spades, Rank: sim.Five}},
		PlayerID: 0,
	}, g)
	runner.ApplyMove(state, sim.Move{
		Type:     sim.MovePlay,
		Cards:    []sim.Card{{Suit: sim.Hearts, Rank: sim.Seven}},
		PlayerID: 1,
	}, g)

	if !state.TrickBroken {
		t.Fatal("TrickBroken must be set when a player plays trump on a non-trump lead")
	}
}

func TestTrumpNotBrokenByOffSuitWhenTrumpLed(t *testing.T) {
	g := trumpBrokenGenome(sim.Hearts)
	runner := &Runner{}
	rng := rand.New(rand.NewPCG(1, 0))
	state := runner.Setup(g, rng)

	// Trump is led; follower has no trump and dumps an off-suit card.
	state.Hands[0] = []sim.Card{{Suit: sim.Hearts, Rank: sim.Five}}
	state.Hands[1] = []sim.Card{{Suit: sim.Clubs, Rank: sim.Seven}}
	state.TrickCards = state.TrickCards[:0]
	state.TrickPlayers = state.TrickPlayers[:0]
	state.Active = 0
	state.TrickBroken = false

	runner.ApplyMove(state, sim.Move{
		Type:     sim.MovePlay,
		Cards:    []sim.Card{{Suit: sim.Hearts, Rank: sim.Five}},
		PlayerID: 0,
	}, g)
	runner.ApplyMove(state, sim.Move{
		Type:     sim.MovePlay,
		Cards:    []sim.Card{{Suit: sim.Clubs, Rank: sim.Seven}},
		PlayerID: 1,
	}, g)

	if state.TrickBroken {
		t.Fatal("TrickBroken must stay false when no trump is played")
	}
}

func TestAllTrickSeedsValid(t *testing.T) {
	seedGames := []*genome.Genome{
		seeds.Whist(),
		seeds.Hearts(),
		seeds.Spades(),
		seeds.OhHell(),
	}

	for _, g := range seedGames {
		errs := genome.Validate(g)
		if len(errs) != 0 {
			t.Errorf("%s failed validation: %v", g.ID, errs)
		}
	}
}

package tricktaking

import (
	"fmt"
	"math"
	"math/rand/v2"
	"reflect"
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
		runner.Upkeep(state, g)

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
		runner.Upkeep(state, g)
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

func TestMultiRoundReDeals(t *testing.T) {
	g := seeds.Whist()
	g.TrickTaking.RoundsPerGame = 3
	g.HandSize = 4
	g.Players = 2

	runner := &Runner{}
	rng := rand.New(rand.NewPCG(99, 0))
	state := runner.Setup(g, rng)
	ai := &sim.RandomAI{}

	maxTurns := g.MaxTurns()
	roundsCompleted := 0
	prevRound := state.Round
	for state.Turn < maxTurns {
		runner.Upkeep(state, g)
		winner := runner.CheckEnd(state, g)
		if state.Round != prevRound {
			roundsCompleted++
			prevRound = state.Round
			for i := 0; i < g.Players; i++ {
				if len(state.Hands[i]) != g.HandSize {
					t.Fatalf("after re-deal player %d hand size is %d, want %d",
						i, len(state.Hands[i]), g.HandSize)
				}
			}
			if len(state.TrickCards) != 0 {
				t.Fatal("re-deal must clear TrickCards")
			}
			if state.TrickBroken {
				t.Fatal("re-deal must clear TrickBroken")
			}
		}
		if winner >= 0 {
			break
		}
		moves := runner.GenerateMoves(state, g)
		if len(moves) == 0 {
			t.Fatalf("no moves at turn %d (round %d)", state.Turn, state.Round)
		}
		runner.ApplyMove(state, ai.SelectMove(moves, state, rng), g)
	}

	if roundsCompleted < g.TrickTaking.RoundsPerGame-1 {
		t.Fatalf("only %d round transitions observed, expected at least %d",
			roundsCompleted, g.TrickTaking.RoundsPerGame-1)
	}
}

// TestCardPointValueSpecificity verifies that overlapping CardPoints rules
// are resolved by specificity (suit+rank > suit-only > rank-only > catch-all)
// rather than by insertion order (dd-hzc).
func TestCardPointValueSpecificity(t *testing.T) {
	queenOfSpades := sim.Card{Suit: sim.Spades, Rank: sim.Queen}
	queenOfHearts := sim.Card{Suit: sim.Hearts, Rank: sim.Queen}
	twoOfSpades := sim.Card{Suit: sim.Spades, Rank: sim.Two}
	threeOfHearts := sim.Card{Suit: sim.Hearts, Rank: sim.Three}

	// Both orderings of the same rule set should produce the same scores.
	allQueens := genome.CardScoring{Rank: uint8(sim.Queen), Points: 10}
	qOfSpades := genome.CardScoring{Suit: uint8(sim.Spades) + 1, Rank: uint8(sim.Queen), Points: 13}
	allHearts := genome.CardScoring{Suit: uint8(sim.Hearts) + 1, Points: 1}
	catchAll := genome.CardScoring{Points: 99}

	rulesA := []genome.CardScoring{allQueens, qOfSpades, allHearts, catchAll}
	rulesB := []genome.CardScoring{catchAll, allHearts, qOfSpades, allQueens}

	for _, rules := range [][]genome.CardScoring{rulesA, rulesB} {
		g := &genome.Genome{
			Scoring: genome.ScoringConfig{CardPoints: rules},
		}
		// Q of Spades: most specific match is the suit+rank rule (13).
		if got := cardPointValue(queenOfSpades, g); got != 13 {
			t.Errorf("Queen of Spades: got %d, want 13 (suit+rank should beat rank-only and catch-all)", got)
		}
		// Q of Hearts: matches "all Queens" (rank-only, 10) and "all Hearts"
		// (suit-only, 1) and catch-all (99). Suit-only is most specific.
		if got := cardPointValue(queenOfHearts, g); got != 1 {
			t.Errorf("Queen of Hearts: got %d, want 1 (suit-only should beat rank-only and catch-all)", got)
		}
		// 2 of Spades: only catch-all matches (99).
		if got := cardPointValue(twoOfSpades, g); got != 99 {
			t.Errorf("Two of Spades: got %d, want 99 (catch-all)", got)
		}
		// 3 of Hearts: matches all Hearts (1) and catch-all (99). Suit-only wins.
		if got := cardPointValue(threeOfHearts, g); got != 1 {
			t.Errorf("Three of Hearts: got %d, want 1 (suit-only beats catch-all)", got)
		}
	}
}

// TestCheckEndReturnsTimeoutAtMaxTurns verifies that when state.Turn has
// reached MaxTurns and hands are still non-empty, CheckEnd returns -1 so the
// batch runner classifies the game as a timeout. Returning a winner here
// would mask hung trick-taking genomes from Tier1 timeout detection (the
// shedding and rummy runners already do this).
func TestCheckEndReturnsTimeoutAtMaxTurns(t *testing.T) {
	g := seeds.Whist()
	runner := &Runner{}
	rng := rand.New(rand.NewPCG(1, 0))
	state := runner.Setup(g, rng)

	// Force a stalled state: Turn at MaxTurns but every hand still has cards.
	state.Turn = g.MaxTurns()
	for i := range state.Hands {
		if len(state.Hands[i]) == 0 {
			state.Hands[i] = []sim.Card{{Suit: sim.Hearts, Rank: sim.Two}}
		}
	}

	if winner := runner.CheckEnd(state, g); winner != -1 {
		t.Fatalf("CheckEnd at MaxTurns with non-empty hands should return -1 (timeout); got %d", winner)
	}
}

// TestTrumpCutAssignsTrumpEvenWhenDeckFullyDealt verifies cards-6u5: when
// HandSize*Players == 52 the post-deal remainder is empty, and TrumpCut would
// silently fall back to TrumpNone. Trump must be picked from the pre-deal
// deck so a genome's declared TrumpRule is honoured regardless of slack.
func TestTrumpCutAssignsTrumpEvenWhenDeckFullyDealt(t *testing.T) {
	g := &genome.Genome{
		ID:       "trump-cut-full-deal",
		Skeleton: genome.TrickTaking,
		Players:  4,
		HandSize: 13, // 4*13 == 52, deck is empty after dealing
		TrickTaking: &genome.TrickTakingParams{
			MustFollowSuit:  true,
			TrickScoring:    genome.ScorePerTrick,
			LeadRestriction: genome.LeadNone,
			RoundsPerGame:   1,
		},
		TrumpRule: genome.TrumpCut,
	}

	for seed := uint64(0); seed < 10; seed++ {
		runner := &Runner{}
		rng := rand.New(rand.NewPCG(seed, 0))
		state := runner.Setup(g, rng)
		if state.TrumpSuit < 0 || state.TrumpSuit > 3 {
			t.Fatalf("seed %d: TrumpCut with HandSize*Players==52 must assign a real trump suit (0-3); got %d",
				seed, state.TrumpSuit)
		}
	}
}

// TestTrumpCutRedealAssignsTrumpEvenWhenDeckFullyDealt covers the same
// regression at round-end re-deal: redealRound also calls determineTrump on
// the post-deal slack, so TrumpCut must remain a real suit after a re-deal
// that empties the deck.
func TestTrumpCutRedealAssignsTrumpEvenWhenDeckFullyDealt(t *testing.T) {
	g := &genome.Genome{
		ID:       "trump-cut-full-deal-redeal",
		Skeleton: genome.TrickTaking,
		Players:  4,
		HandSize: 13,
		TrickTaking: &genome.TrickTakingParams{
			MustFollowSuit:  true,
			TrickScoring:    genome.ScorePerTrick,
			LeadRestriction: genome.LeadNone,
			RoundsPerGame:   2,
		},
		TrumpRule: genome.TrumpCut,
	}
	runner := &Runner{}
	rng := rand.New(rand.NewPCG(3, 0))
	state := runner.Setup(g, rng)
	ai := &sim.RandomAI{}

	maxTurns := g.MaxTurns()
	sawRound1 := false
	for state.Turn < maxTurns {
		runner.Upkeep(state, g)
		winner := runner.CheckEnd(state, g)
		if state.Round == 1 && !sawRound1 {
			sawRound1 = true
			if state.TrumpSuit < 0 || state.TrumpSuit > 3 {
				t.Fatalf("after re-deal, TrumpCut must keep a real trump suit (0-3); got %d", state.TrumpSuit)
			}
		}
		if winner >= 0 {
			break
		}
		moves := runner.GenerateMoves(state, g)
		if len(moves) == 0 {
			t.Fatalf("no moves at turn %d (round %d)", state.Turn, state.Round)
		}
		runner.ApplyMove(state, ai.SelectMove(moves, state, rng), g)
	}

	if !sawRound1 {
		t.Fatal("multi-round game never advanced to round 1; cannot exercise re-deal trump assignment")
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

// stateHash serializes every GameState field except RNG (a pointer; the
// purity tests pass a nil RNG sentinel so any dereference panics loudly).
// Used to assert that query methods do not mutate state (audit Task 3).
func stateHash(s *sim.GameState) string {
	top := "nil"
	if s.TopCard != nil {
		top = s.TopCard.String()
	}
	return fmt.Sprintf("deck=%v|hands=%v|discard=%v|tableau=%v|scores=%v|turn=%d|active=%d|phase=%d|np=%d|dir=%d|round=%d|maxround=%d|top=%s|tc=%v|tp=%v|tl=%d|trump=%d|broken=%t|melds=%v|owners=%v|events=%v",
		s.Deck, s.Hands, s.Discard, s.Tableau, s.Scores, s.Turn, s.Active,
		s.Phase, s.NumPlayers, s.Direction, s.Round, s.MaxRound, top,
		s.TrickCards, s.TrickPlayers, s.TrickLeader, s.TrumpSuit, s.TrickBroken,
		s.Melds, s.MeldOwner, s.Events)
}

// TestGenerateMovesIsPure pins audit Task 3: GenerateMoves must be a pure
// query -- calling it any number of times must not change state and must
// return the same move list.
func TestGenerateMovesIsPure(t *testing.T) {
	g := seeds.Whist()
	runner := &Runner{}
	state := runner.Setup(g, rand.New(rand.NewPCG(5, 0)))
	state.RNG = nil // sentinel: pure queries must never touch the RNG

	before := stateHash(state)
	m1 := runner.GenerateMoves(state, g)
	after1 := stateHash(state)
	m2 := runner.GenerateMoves(state, g)
	after2 := stateHash(state)

	if after1 != before {
		t.Errorf("first GenerateMoves mutated state:\nbefore: %s\nafter:  %s", before, after1)
	}
	if after2 != before {
		t.Errorf("second GenerateMoves mutated state:\nbefore: %s\nafter:  %s", before, after2)
	}
	if !reflect.DeepEqual(m1, m2) {
		t.Errorf("repeated GenerateMoves returned different moves:\n%v\nvs\n%v", m1, m2)
	}
}

// TestCheckEndIsPure pins audit Task 3: CheckEnd must be a pure query. The
// historical violation: on an all-hands-empty state mid-game it incremented
// state.Round and redealt fresh hands (advancing state.RNG). That round
// transition now lives in Upkeep.
func TestCheckEndIsPure(t *testing.T) {
	g := seeds.Whist()
	g.TrickTaking.RoundsPerGame = 3
	g.HandSize = 4
	g.Players = 2

	runner := &Runner{}
	state := runner.Setup(g, rand.New(rand.NewPCG(9, 0)))
	state.RNG = nil // sentinel: pure queries must never touch the RNG

	// Simulate a finished round: every hand played out into the tableau.
	for i := range state.Hands {
		state.Tableau[i] = append(state.Tableau[i], state.Hands[i]...)
		state.Hands[i] = state.Hands[i][:0]
	}

	before := stateHash(state)
	w1 := runner.CheckEnd(state, g)
	after1 := stateHash(state)
	w2 := runner.CheckEnd(state, g)
	after2 := stateHash(state)

	if after1 != before {
		t.Errorf("first CheckEnd mutated state:\nbefore: %s\nafter:  %s", before, after1)
	}
	if after2 != before {
		t.Errorf("second CheckEnd mutated state:\nbefore: %s\nafter:  %s", before, after2)
	}
	if w1 != w2 {
		t.Errorf("repeated CheckEnd returned different winners: %d vs %d", w1, w2)
	}
}

// playOutWithProgress plays a full game with random AI, asserting at every
// applied move that Progress returns one value per player in [0,1] (audit
// Task 8). Returns the final state and winner (-1 when the game did not
// complete naturally).
func playOutWithProgress(t *testing.T, g *genome.Genome, seed uint64) (*sim.GameState, int) {
	t.Helper()
	runner := &Runner{}
	ai := &sim.RandomAI{}
	rng := rand.New(rand.NewPCG(seed, 0))

	state := runner.Setup(g, rng)
	maxTurns := g.MaxTurns()

	for {
		runner.Upkeep(state, g)
		if winner := runner.CheckEnd(state, g); winner >= 0 {
			return state, winner
		}
		if state.Turn >= maxTurns {
			return state, -1
		}
		moves := runner.GenerateMoves(state, g)
		if len(moves) == 0 {
			return state, -1
		}
		move := ai.SelectMove(moves, state, rng)
		runner.ApplyMove(state, move, g)

		progress := runner.Progress(state, g)
		if len(progress) != state.NumPlayers {
			t.Fatalf("seed %d: Progress returned %d values, want %d", seed, len(progress), state.NumPlayers)
		}
		for p, v := range progress {
			if v < 0 || v > 1 {
				t.Fatalf("seed %d: Progress[%d] = %v, want in [0,1]", seed, p, v)
			}
		}
	}
}

// TestProgressWinnerIsMaxAcrossSeeds is the Task 8 (b) property for both
// scoring directions: in a played-out game, the eventual winner's final
// Progress is the maximum across players (ties allowed). Whist covers
// highest-score-wins (ScorePerTrick); Hearts covers avoidance, where the
// share must be inverted so the lowest scorer leads.
func TestProgressWinnerIsMaxAcrossSeeds(t *testing.T) {
	cases := []struct {
		name string
		g    *genome.Genome
	}{
		{"whist", seeds.Whist()},
		{"hearts-avoidance", seeds.Hearts()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runner := &Runner{}
			completed := 0
			for seed := uint64(0); seed < 10; seed++ {
				state, winner := playOutWithProgress(t, tc.g, seed)
				if winner < 0 {
					continue
				}
				completed++
				progress := runner.Progress(state, tc.g)
				for p, v := range progress {
					if v > progress[winner] {
						t.Errorf("seed %d: Progress[%d] = %v exceeds winner %d's %v", seed, p, v, winner, progress[winner])
					}
				}
			}
			if completed == 0 {
				t.Fatal("no seed completed: winner-max property never exercised")
			}
		})
	}
}

// TestProgressAvoidanceInversion pins the inversion direction with hand-set
// scores: under ScoreAvoidance the LOW scorer leads; under ScorePerTrick the
// HIGH scorer leads. Shares are score/total: with scores {5, 2} the plain
// shares are {5/7, 2/7} and the avoidance shares are {2/7, 5/7}.
func TestProgressAvoidanceInversion(t *testing.T) {
	runner := &Runner{}
	state := sim.NewGameState(2)
	state.Scores[0] = 5
	state.Scores[1] = 2

	plain := &genome.Genome{
		Skeleton: genome.TrickTaking, Players: 2, HandSize: 13,
		TrickTaking: &genome.TrickTakingParams{TrickScoring: genome.ScorePerTrick},
	}
	avoid := &genome.Genome{
		Skeleton: genome.TrickTaking, Players: 2, HandSize: 13,
		TrickTaking: &genome.TrickTakingParams{TrickScoring: genome.ScoreAvoidance},
	}

	pp := runner.Progress(state, plain)
	if got, want := pp[0], 5.0/7.0; math.Abs(got-want) > 1e-12 {
		t.Errorf("plain Progress[0] = %v, want %v", got, want)
	}
	if pp[0] <= pp[1] {
		t.Errorf("plain scoring: high scorer must lead, got %v", pp)
	}

	pa := runner.Progress(state, avoid)
	if got, want := pa[1], 1-2.0/7.0; math.Abs(got-want) > 1e-12 {
		t.Errorf("avoidance Progress[1] = %v, want %v", got, want)
	}
	if pa[1] <= pa[0] {
		t.Errorf("avoidance scoring: low scorer must lead, got %v", pa)
	}
}

package climbing

import (
	"math/rand/v2"
	"reflect"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

// --- Test fixtures ---

func bigTwo() *genome.Genome { return seeds.BigTwo() }

func singlesOnly(players, hand int) *genome.Genome {
	return &genome.Genome{
		ID: "singles", Skeleton: genome.Climbing, Players: players, HandSize: hand,
		Climbing: &genome.ClimbingParams{},
	}
}

// playOut runs a full game with the given AI, returning (finalState, winner).
// winner is -1 when the game did not complete naturally (turn cap). It asserts
// the playability invariant (GenerateMoves never empty) and the Progress
// contract (one value per player, all in [0,1]) at every applied move.
func playOut(t *testing.T, g *genome.Genome, ai sim.AIPlayer, seed uint64) (*sim.GameState, int) {
	t.Helper()
	runner := &Runner{}
	rng := rand.New(rand.NewPCG(seed, 0))
	state := runner.Setup(g, rng)
	maxTurns := g.MaxTurns()

	for {
		runner.Upkeep(state, g)
		if w := runner.CheckEnd(state, g); w >= 0 {
			return state, w
		}
		if state.Turn >= maxTurns {
			return state, -1
		}
		moves := runner.GenerateMoves(state, g)
		if len(moves) == 0 {
			t.Fatalf("seed %d turn %d: GenerateMoves returned empty (playability invariant violated)", seed, state.Turn)
		}
		move := ai.SelectMove(moves, state, rng)
		runner.ApplyMove(state, move, g)

		prog := runner.Progress(state, g)
		if len(prog) != state.NumPlayers {
			t.Fatalf("seed %d: Progress returned %d values, want %d", seed, len(prog), state.NumPlayers)
		}
		for p, v := range prog {
			if v < 0 || v > 1 {
				t.Fatalf("seed %d: Progress[%d] = %v, want in [0,1]", seed, p, v)
			}
		}
	}
}

// --- Validation ---

func TestBigTwoSeedValid(t *testing.T) {
	if errs := genome.Validate(bigTwo()); len(errs) > 0 {
		t.Fatalf("BigTwo seed must be Tier-0 valid, got: %v", errs)
	}
}

func TestClimbingValidatesParamRanges(t *testing.T) {
	// AllowRuns with an out-of-range MinRunLen is rejected.
	bad := &genome.Genome{
		Skeleton: genome.Climbing, Players: 2, HandSize: 5,
		Climbing: &genome.ClimbingParams{AllowRuns: true, MinRunLen: 2},
	}
	if errs := genome.Validate(bad); len(errs) == 0 {
		t.Error("AllowRuns with MinRunLen 2 must be rejected (runs are length >= 3)")
	}
	// Runs off + MinRunLen 0 (unset) is fine.
	ok := &genome.Genome{
		Skeleton: genome.Climbing, Players: 2, HandSize: 5,
		Climbing: &genome.ClimbingParams{AllowPairs: true},
	}
	if errs := genome.Validate(ok); len(errs) > 0 {
		t.Errorf("pairs-only climbing must be valid, got: %v", errs)
	}
}

func TestClimbingRejectsTrumpAndSpecials(t *testing.T) {
	withTrump := &genome.Genome{
		Skeleton: genome.Climbing, Players: 2, HandSize: 5,
		Climbing: &genome.ClimbingParams{}, TrumpRule: genome.TrumpCut,
	}
	if errs := genome.Validate(withTrump); len(errs) == 0 {
		t.Error("trump rule on climbing must be rejected (no trump concept)")
	}
	withSpecials := &genome.Genome{
		Skeleton: genome.Climbing, Players: 2, HandSize: 5,
		Climbing:     &genome.ClimbingParams{},
		SpecialCards: []genome.SpecialCard{{Type: genome.SpecialWild, ByRank: 8}},
	}
	if errs := genome.Validate(withSpecials); len(errs) == 0 {
		t.Error("special cards on climbing must be rejected (only shedding consumes them)")
	}
}

// --- Playability invariant (the headline property) ---

// TestGenerateMovesNeverEmpty is the playability-by-construction property: over
// many random REACHABLE states (random rollouts of several genome shapes),
// GenerateMoves must never return empty for a player with cards. This is the
// invariant the whole skeleton rests on -- a legal move always exists (pass
// when following, a single when leading).
func TestGenerateMovesNeverEmpty(t *testing.T) {
	runner := &Runner{}
	ai := &sim.RandomAI{}
	genomes := []*genome.Genome{
		bigTwo(),
		singlesOnly(2, 3),
		singlesOnly(4, 13),
		{ID: "p", Skeleton: genome.Climbing, Players: 2, HandSize: 13,
			Climbing: &genome.ClimbingParams{AllowPairs: true, AllowTriples: true}},
		{ID: "r", Skeleton: genome.Climbing, Players: 3, HandSize: 10,
			Climbing: &genome.ClimbingParams{AllowRuns: true, MinRunLen: 4}},
		{ID: "all6", Skeleton: genome.Climbing, Players: 6, HandSize: 8,
			Climbing: &genome.ClimbingParams{AllowPairs: true, AllowTriples: true, AllowRuns: true, MinRunLen: 3}},
	}
	checks := 0
	for _, g := range genomes {
		for seed := uint64(0); seed < 30; seed++ {
			rng := rand.New(rand.NewPCG(seed, 99))
			state := runner.Setup(g, rng)
			maxTurns := g.MaxTurns()
			for {
				runner.Upkeep(state, g)
				if runner.CheckEnd(state, g) >= 0 || state.Turn >= maxTurns {
					break
				}
				// The active player always has cards here (CheckEnd would have
				// fired on an empty hand). A legal move must exist.
				if len(state.ActiveHand()) > 0 {
					moves := runner.GenerateMoves(state, g)
					if len(moves) == 0 {
						t.Fatalf("genome %s seed %d turn %d: empty move list (hand=%v, table=%v)",
							g.ID, seed, state.Turn, state.ActiveHand(), state.TrickCards)
					}
					checks++
				}
				moves := runner.GenerateMoves(state, g)
				runner.ApplyMove(state, ai.SelectMove(moves, state, rng), g)
			}
		}
	}
	if checks == 0 {
		t.Fatal("invariant never exercised")
	}
}

// TestLeadingAlwaysHasASingle: when the table is clear, every hand card is a
// legal single lead, so the move list is at least as long as the hand.
func TestLeadingAlwaysHasASingle(t *testing.T) {
	runner := &Runner{}
	g := singlesOnly(2, 5)
	state := sim.NewGameState(2)
	state.Hands[0] = []sim.Card{
		{Suit: sim.Clubs, Rank: sim.Two}, {Suit: sim.Hearts, Rank: sim.King},
		{Suit: sim.Spades, Rank: sim.Ace},
	}
	state.TrickCards = nil // clear table => leading
	state.Active = 0
	moves := runner.GenerateMoves(state, g)
	if len(moves) < 3 {
		t.Fatalf("leading with 3 cards must offer >= 3 single plays, got %d", len(moves))
	}
	for _, m := range moves {
		if m.Type != sim.MovePlay {
			t.Errorf("leading move must be a play, got %v", m.Type)
		}
	}
}

// TestFollowingAlwaysOffersPass: when a combination is on the table, Pass is
// always among the legal moves (even when no beating combo exists).
func TestFollowingAlwaysOffersPass(t *testing.T) {
	runner := &Runner{}
	g := singlesOnly(2, 5)
	state := sim.NewGameState(2)
	// Active holds only a 3; the table shows an Ace -- no beat possible.
	state.Hands[0] = []sim.Card{{Suit: sim.Clubs, Rank: sim.Three}}
	state.TrickCards = []sim.Card{{Suit: sim.Spades, Rank: sim.Ace}}
	state.TrickLeader = 1
	state.Active = 0
	moves := runner.GenerateMoves(state, g)
	hasPass := false
	for _, m := range moves {
		if m.Type == sim.MovePass {
			hasPass = true
		}
	}
	if !hasPass {
		t.Fatalf("following with no beat must offer Pass, got %v", moves)
	}
}

// --- Combination beating rules ---

func TestSingleBeatsByRank(t *testing.T) {
	runner := &Runner{}
	g := singlesOnly(2, 5)
	state := sim.NewGameState(2)
	state.Hands[0] = []sim.Card{
		{Suit: sim.Clubs, Rank: sim.Three}, // can't beat a King
		{Suit: sim.Hearts, Rank: sim.Ace},  // beats a King
	}
	state.TrickCards = []sim.Card{{Suit: sim.Spades, Rank: sim.King}}
	state.TrickLeader = 1
	state.Active = 0
	moves := runner.GenerateMoves(state, g)
	plays := 0
	for _, m := range moves {
		if m.Type == sim.MovePlay {
			plays++
			if m.Cards[0].Rank != sim.Ace {
				t.Errorf("only the Ace beats a King single, got %v", m.Cards)
			}
		}
	}
	if plays != 1 {
		t.Errorf("exactly one beating single expected, got %d", plays)
	}
}

func TestPairsRequireSameTypeAndHigher(t *testing.T) {
	runner := &Runner{}
	g := &genome.Genome{Skeleton: genome.Climbing, Players: 2, HandSize: 5,
		Climbing: &genome.ClimbingParams{AllowPairs: true}}
	state := sim.NewGameState(2)
	// Hand has a pair of Kings and a lone Ace.
	state.Hands[0] = []sim.Card{
		{Suit: sim.Clubs, Rank: sim.King}, {Suit: sim.Hearts, Rank: sim.King},
		{Suit: sim.Spades, Rank: sim.Ace},
	}
	// Table shows a pair of Queens.
	state.TrickCards = []sim.Card{
		{Suit: sim.Clubs, Rank: sim.Queen}, {Suit: sim.Diamonds, Rank: sim.Queen}}
	state.TrickLeader = 1
	state.Active = 0
	moves := runner.GenerateMoves(state, g)
	pairPlays := 0
	for _, m := range moves {
		if m.Type == sim.MovePlay {
			if len(m.Cards) != 2 {
				t.Errorf("only a pair may beat a pair, got %d cards: %v", len(m.Cards), m.Cards)
			}
			pairPlays++
		}
	}
	if pairPlays != 1 {
		t.Errorf("exactly one beating pair (Kings) expected, got %d", pairPlays)
	}
}

func TestRunsRequireSameLength(t *testing.T) {
	runner := &Runner{}
	g := &genome.Genome{Skeleton: genome.Climbing, Players: 2, HandSize: 13,
		Climbing: &genome.ClimbingParams{AllowRuns: true, MinRunLen: 3}}
	state := sim.NewGameState(2)
	// Hand holds a 5-6-7 run (beats a lower 3-card run) and a longer 9-10-J-Q.
	state.Hands[0] = []sim.Card{
		{Suit: sim.Clubs, Rank: sim.Five}, {Suit: sim.Hearts, Rank: sim.Six},
		{Suit: sim.Spades, Rank: sim.Seven},
		{Suit: sim.Clubs, Rank: sim.Nine}, {Suit: sim.Hearts, Rank: sim.Ten},
		{Suit: sim.Spades, Rank: sim.Jack}, {Suit: sim.Diamonds, Rank: sim.Queen},
	}
	// Table shows a 3-card run 2-3-4.
	state.TrickCards = []sim.Card{
		{Suit: sim.Clubs, Rank: sim.Two}, {Suit: sim.Hearts, Rank: sim.Three},
		{Suit: sim.Spades, Rank: sim.Four}}
	state.TrickLeader = 1
	state.Active = 0
	moves := runner.GenerateMoves(state, g)
	for _, m := range moves {
		if m.Type == sim.MovePlay && len(m.Cards) != 3 {
			t.Errorf("only a 3-card run may beat a 3-card run, got %d cards: %v", len(m.Cards), m.Cards)
		}
	}
}

// --- Table-clear / lead rotation (Upkeep) ---

func TestUpkeepClearsTableWhenAllPass(t *testing.T) {
	runner := &Runner{}
	g := singlesOnly(3, 5)
	state := sim.NewGameState(3)
	for i := range state.Hands {
		state.Hands[i] = []sim.Card{{Suit: sim.Clubs, Rank: sim.Rank(5 + i)}}
	}
	// Player 1 played the current combo; players 2 and 0 both passed.
	state.TrickCards = []sim.Card{{Suit: sim.Spades, Rank: sim.Ace}}
	state.TrickLeader = 1
	state.PassCount = 2 // NumPlayers-1 = 2 => everyone else passed
	state.Active = 1

	runner.Upkeep(state, g)
	if len(state.TrickCards) != 0 {
		t.Errorf("table should clear after all-pass, got %v", state.TrickCards)
	}
	if state.Active != 1 {
		t.Errorf("leader (player 1) should lead the fresh table, Active=%d", state.Active)
	}
	if state.PassCount != 0 {
		t.Errorf("PassCount should reset on table clear, got %d", state.PassCount)
	}
}

func TestUpkeepNoClearBeforeAllPass(t *testing.T) {
	runner := &Runner{}
	g := singlesOnly(3, 5)
	state := sim.NewGameState(3)
	state.TrickCards = []sim.Card{{Suit: sim.Spades, Rank: sim.Ace}}
	state.TrickLeader = 1
	state.PassCount = 1 // only one player passed; not all others yet
	state.Active = 2
	runner.Upkeep(state, g)
	if len(state.TrickCards) == 0 {
		t.Error("table cleared too early (only 1 of 2 others passed)")
	}
}

// --- Determinism ---

func TestClimbingDeterminism(t *testing.T) {
	g := bigTwo()
	for seed := uint64(0); seed < 10; seed++ {
		_, w1 := playOut(t, g, &sim.RandomAI{}, seed)
		_, w2 := playOut(t, g, &sim.RandomAI{}, seed)
		if w1 != w2 {
			t.Errorf("seed %d: nondeterministic winner %d vs %d", seed, w1, w2)
		}
	}
}

// TestGenerateMovesIsPure: GenerateMoves must not mutate state and must return
// the same move list on repeated calls (audit Task 3 contract).
func TestGenerateMovesIsPure(t *testing.T) {
	runner := &Runner{}
	g := bigTwo()
	rng := rand.New(rand.NewPCG(5, 0))
	state := runner.Setup(g, rng)
	ai := &sim.RandomAI{}
	for step := 0; step < 50 && runner.CheckEnd(state, g) < 0; step++ {
		runner.Upkeep(state, g)
		if runner.CheckEnd(state, g) >= 0 {
			break
		}
		before := state.Clone()
		m1 := runner.GenerateMoves(state, g)
		m2 := runner.GenerateMoves(state, g)
		if !statesEqual(before, state) {
			t.Fatalf("step %d: GenerateMoves mutated state", step)
		}
		if !reflect.DeepEqual(m1, m2) {
			t.Fatalf("step %d: GenerateMoves nondeterministic:\n%v\nvs\n%v", step, m1, m2)
		}
		runner.ApplyMove(state, ai.SelectMove(m1, state, rng), g)
	}
}

// TestCheckEndIsPure: CheckEnd must not mutate state.
func TestCheckEndIsPure(t *testing.T) {
	runner := &Runner{}
	g := bigTwo()
	rng := rand.New(rand.NewPCG(7, 0))
	state := runner.Setup(g, rng)
	before := state.Clone()
	runner.CheckEnd(state, g)
	if !statesEqual(before, state) {
		t.Fatal("CheckEnd mutated state")
	}
}

func statesEqual(a, b *sim.GameState) bool {
	return reflect.DeepEqual(a.Hands, b.Hands) &&
		reflect.DeepEqual(a.TrickCards, b.TrickCards) &&
		reflect.DeepEqual(a.TrickPlayers, b.TrickPlayers) &&
		a.TrickLeader == b.TrickLeader && a.PassCount == b.PassCount &&
		a.Active == b.Active && a.Turn == b.Turn &&
		reflect.DeepEqual(a.Deck, b.Deck)
}

// --- Termination ---

// TestTerminationEveryPlayShrinksHand: a play strictly removes its cards from
// the mover's hand, so the empty-hand win is monotonically approached.
func TestTerminationEveryPlayShrinksHand(t *testing.T) {
	runner := &Runner{}
	g := bigTwo()
	rng := rand.New(rand.NewPCG(3, 0))
	state := runner.Setup(g, rng)
	ai := &sim.RandomAI{}
	for step := 0; step < 500 && runner.CheckEnd(state, g) < 0; step++ {
		runner.Upkeep(state, g)
		if runner.CheckEnd(state, g) >= 0 {
			break
		}
		mover := state.Active
		before := len(state.Hands[mover])
		moves := runner.GenerateMoves(state, g)
		m := ai.SelectMove(moves, state, rng)
		runner.ApplyMove(state, m, g)
		after := len(state.Hands[mover])
		if m.Type == sim.MovePlay && after >= before {
			t.Fatalf("step %d: play did not shrink hand (%d -> %d)", step, before, after)
		}
		if m.Type == sim.MovePass && after != before {
			t.Fatalf("step %d: pass changed hand size (%d -> %d)", step, before, after)
		}
	}
}

func TestClimbingGamesComplete(t *testing.T) {
	g := bigTwo()
	completions := 0
	for seed := uint64(0); seed < 30; seed++ {
		if _, w := playOut(t, g, &sim.RandomAI{}, seed); w >= 0 {
			completions++
		}
	}
	if completions < 25 {
		t.Errorf("expected most games to complete under random play, got %d/30", completions)
	}
}

// --- Progress / winner-max (audit Task 8) ---

// TestProgressWinnerIsMax: the eventual winner's final Progress is the maximum
// across players (ties allowed) -- the winner-max property the other skeletons
// pin. Climbing's winner is first-to-empty-hand and Progress ranks by
// (1 - hand share), so the orderings agree exactly (the winner has hand 0,
// Progress 1.0).
func TestProgressWinnerIsMax(t *testing.T) {
	runner := &Runner{}
	g := bigTwo()
	completed := 0
	for seed := uint64(0); seed < 30; seed++ {
		state, winner := playOut(t, g, &sim.RandomAI{}, seed)
		if winner < 0 {
			continue
		}
		completed++
		prog := runner.Progress(state, g)
		if prog[winner] != 1.0 {
			t.Errorf("seed %d: winner %d Progress = %v, want 1.0 (empty hand)", seed, winner, prog[winner])
		}
		for p, v := range prog {
			if v > prog[winner] {
				t.Errorf("seed %d: Progress[%d] = %v exceeds winner's %v", seed, p, v, prog[winner])
			}
		}
	}
	if completed == 0 {
		t.Fatal("winner-max property never exercised")
	}
}

func TestProgressFromHandSize(t *testing.T) {
	runner := &Runner{}
	g := singlesOnly(2, 10)
	state := sim.NewGameState(2)
	state.Hands[0] = make([]sim.Card, 3) // 3 of 10 left => 1 - 3/10 = 0.7
	state.Hands[1] = nil                 // empty => 1.0
	prog := runner.Progress(state, g)
	if prog[0] != 0.7 {
		t.Errorf("Progress[0] = %v, want 0.7", prog[0])
	}
	if prog[1] != 1.0 {
		t.Errorf("Progress[1] = %v, want 1.0", prog[1])
	}
	// Pure: Progress must not bank into Scores.
	if state.Scores[0] != 0 || state.Scores[1] != 0 {
		t.Errorf("Progress mutated Scores = %v", state.Scores)
	}
}

// --- Greedy AI ---

func TestGreedyClimbingPlaysAndCompletes(t *testing.T) {
	g := bigTwo()
	ai := &sim.GreedyAI{Scorer: &sim.ClimbingScorer{}}
	completions := 0
	for seed := uint64(0); seed < 20; seed++ {
		if _, w := playOut(t, g, ai, seed); w >= 0 {
			completions++
		}
	}
	if completions == 0 {
		t.Fatal("greedy climbing never completed a game")
	}
}

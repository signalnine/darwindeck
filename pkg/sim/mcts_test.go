// ISMCTS player tests (audit Task 19 step 3). External package so the real
// skeleton runners can be driven (they import sim).
package sim_test

import (
	"fmt"
	"math/rand/v2"
	"reflect"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
	"github.com/darwindeck/darwindeck/pkg/sim"
	"github.com/darwindeck/darwindeck/pkg/skeleton/rummy"
)

// testMCTS returns an MCTSAI with small-but-meaningful settings so the suite
// stays fast (including under -race). Production defaults (200/10/200) are
// exercised by BenchmarkMCTSGame.
func testMCTS(g *genome.Genome, runner sim.GenericRunner) *sim.MCTSAI {
	return &sim.MCTSAI{
		Runner:           runner,
		Genome:           g,
		Iterations:       60,
		Determinizations: 4,
		RolloutCap:       80,
	}
}

// ginKnockState builds a rummy state where the active player can KNOCK in
// PhaseMeld and immediately win: their hand has deadwood 7 (<= the gin seed's
// threshold 10) but contains NO complete meld, so the legal moves are exactly
// {knock, pass}. Knock ends the round now (player 0 banks -7, the opponent
// banks far more); pass wanders into a long uncertain game. The values are
// well separated, so the most-visited root child must be knock.
//
// Plan note (deviation): the plan asks for a "one-move-to-win shedding
// state", but the shedding move generator cannot offer a CHOICE that
// includes an immediate win -- a 1-card hand with a matching card yields
// exactly one legal move, and multi-card hands never win on a single play.
// The rummy knock construct is the same test with an actual decision in it.
// An earlier variant gave player 0 a perfect gin hand, but laying a meld
// from a gin hand is also worth ~1.0 (the player keeps acting until gin), so
// most-visited was legitimately ambiguous; deadwood-without-melds removes
// the near-equal alternatives instead of papering over them with iterations.
func ginKnockState(t *testing.T) (*genome.Genome, *sim.GameState) {
	t.Helper()
	g := seeds.GinRummy() // KnockThreshold 10, MinMeldSize 3, 2 players

	st := sim.NewGameState(2)
	// Deadwood 1+1+2+3 = 7; the ace pair is not a meld (MinMeldSize 3), no
	// runs (suits all differ on adjacent ranks).
	st.Hands[0] = []sim.Card{
		{Suit: sim.Clubs, Rank: sim.Ace}, {Suit: sim.Diamonds, Rank: sim.Ace},
		{Suit: sim.Hearts, Rank: sim.Two}, {Suit: sim.Spades, Rank: sim.Three},
	}
	// Deadwood 10+10+10+10+8+7+6+4+3+9 = 77; distinct ranks, no same-suit
	// adjacencies => no melds.
	st.Hands[1] = []sim.Card{
		{Suit: sim.Clubs, Rank: sim.King}, {Suit: sim.Diamonds, Rank: sim.Queen},
		{Suit: sim.Hearts, Rank: sim.Jack}, {Suit: sim.Spades, Rank: sim.Ten},
		{Suit: sim.Clubs, Rank: sim.Eight}, {Suit: sim.Diamonds, Rank: sim.Seven},
		{Suit: sim.Hearts, Rank: sim.Six}, {Suit: sim.Spades, Rank: sim.Four},
		{Suit: sim.Clubs, Rank: sim.Three}, {Suit: sim.Diamonds, Rank: sim.Nine},
	}
	st.Discard = []sim.Card{{Suit: sim.Diamonds, Rank: sim.Three}}

	// Deck = remainder of the standard 52 so rollouts can draw.
	used := make(map[sim.Card]bool)
	for _, h := range st.Hands {
		for _, c := range h {
			used[c] = true
		}
	}
	used[st.Discard[0]] = true
	for _, c := range sim.StandardDeck() {
		if !used[c] {
			st.Deck = append(st.Deck, c)
		}
	}

	st.Active = 0
	st.Phase = sim.PhaseMeld
	st.Melds = [][]sim.Card{}
	st.MeldOwner = []int{}
	st.RNG = rand.New(rand.NewPCG(99, 0))
	return g, st
}

// TestMCTSPicksWinningMove: plan test (a). The knock move wins immediately in
// every determinization; MCTS must return it over the pass alternative.
// Repeated over 5 rng seeds so a uniform-random chooser (p = 1/32 to pass by
// luck) cannot sneak through.
func TestMCTSPicksWinningMove(t *testing.T) {
	g, st := ginKnockState(t)
	runner := &rummy.Runner{}
	ai := testMCTS(g, runner)

	moves := runner.GenerateMoves(st, g)
	hasKnock, alternatives := false, 0
	for _, m := range moves {
		if m.Type == sim.MoveKnock {
			hasKnock = true
		} else {
			alternatives++
		}
	}
	if !hasKnock || alternatives == 0 {
		t.Fatalf("bad construct: moves = %v, want knock plus alternatives", moves)
	}

	for seed := uint64(1); seed <= 5; seed++ {
		got := ai.SelectMove(moves, st, rand.New(rand.NewPCG(seed, 0)))
		if got.Type != sim.MoveKnock {
			t.Fatalf("seed %d: MCTS chose %v (key %s), want MoveKnock", seed, got, got.Key())
		}
	}
}

// TestMCTSKnockBanksDeadwoodOnce: the plan's no-double-banking regression,
// end-to-end with known numbers. After MCTS picks knock and the move is
// applied through the real loop shape (ApplyMove, then Upkeep exactly once,
// then CheckEnd), the final scores must be exactly [-7, -77]. A search that
// leaked an extra Upkeep onto the REAL state -- or a Clone that aliased
// Scores -- would surface here as [-14, -154] or other corruption.
func TestMCTSKnockBanksDeadwoodOnce(t *testing.T) {
	g, st := ginKnockState(t)
	runner := &rummy.Runner{}
	ai := testMCTS(g, runner)

	moves := runner.GenerateMoves(st, g)
	mv := ai.SelectMove(moves, st, rand.New(rand.NewPCG(1, 0)))
	if mv.Type != sim.MoveKnock {
		t.Fatalf("MCTS chose %v, want MoveKnock", mv)
	}

	runner.ApplyMove(st, mv, g)
	runner.Upkeep(st, g) // the game loop's once-per-iteration Upkeep banks deadwood
	winner := runner.CheckEnd(st, g)

	if winner != 0 {
		t.Errorf("winner = %d, want 0", winner)
	}
	if want := []int{-7, -77}; !reflect.DeepEqual(st.Scores, want) {
		t.Errorf("Scores = %v, want %v (double-banked deadwood would show as [-14 -154])", st.Scores, want)
	}
}

// decisionPoint advances a fresh game until the active player faces >= 2
// legal moves, returning the state (already Upkeep'd, like the game loop
// leaves it before calling the AI) and the move list. Fixed step counts are
// brittle: a particular seed can land on a forced-move point.
func decisionPoint(t *testing.T, tc skeletonCase, seed uint64) (*sim.GameState, []sim.Move) {
	t.Helper()
	rng := rand.New(rand.NewPCG(seed, 0))
	st := tc.runner.Setup(tc.g, rng)
	for step := 0; step < 200; step++ {
		tc.runner.Upkeep(st, tc.g)
		if tc.runner.CheckEnd(st, tc.g) >= 0 {
			break
		}
		moves := tc.runner.GenerateMoves(st, tc.g)
		if len(moves) == 0 {
			break
		}
		if len(moves) >= 2 && step >= 6 { // skip the opening so zones are populated
			return st, moves
		}
		tc.runner.ApplyMove(st, moves[rng.IntN(len(moves))], tc.g)
	}
	t.Fatalf("seed %d: no mid-game decision point with >= 2 moves found", seed)
	return nil, nil
}

// TestMCTSDoesNotMutateState: SelectMove searches clones and determinizations
// only; the real state (and the move list it was handed) must be
// bit-identical afterwards.
func TestMCTSDoesNotMutateState(t *testing.T) {
	tc := skeletonCase{"rummy/gin-rummy", seeds.GinRummy(), &rummy.Runner{}}
	st, moves := decisionPoint(t, tc, 5)

	snapshot := st.Clone()
	movesCopy := make([]sim.Move, len(moves))
	for i, m := range moves {
		movesCopy[i] = sim.Move{Type: m.Type, Cards: append([]sim.Card(nil), m.Cards...), PlayerID: m.PlayerID}
	}

	ai := testMCTS(tc.g, tc.runner)
	ai.SelectMove(moves, st, rand.New(rand.NewPCG(8, 0)))

	if !reflect.DeepEqual(st.Clone(), snapshot) { // Clone normalizes Events/RNG, both nil
		t.Error("SelectMove mutated the real game state")
	}
	if !reflect.DeepEqual(moves, movesCopy) {
		t.Error("SelectMove mutated the caller's move list")
	}
}

// upkeepContractSpy wraps a real runner and enforces the non-idempotent
// Upkeep contract on every state the search touches: Upkeep must run exactly
// once after each ApplyMove and NEVER before the first one (the game loop
// already ran Upkeep on the decision-point state the search starts from --
// rummy would double-bank deadwood, tricktaking would double-advance
// Round/redeal).
type upkeepContractSpy struct {
	inner      sim.GenericRunner
	applies    map[*sim.GameState]int
	upkeeps    map[*sim.GameState]int
	violations []string
}

func newUpkeepContractSpy(inner sim.GenericRunner) *upkeepContractSpy {
	return &upkeepContractSpy{
		inner:   inner,
		applies: make(map[*sim.GameState]int),
		upkeeps: make(map[*sim.GameState]int),
	}
}

func (s *upkeepContractSpy) Setup(g *genome.Genome, rng *rand.Rand) *sim.GameState {
	return s.inner.Setup(g, rng)
}

func (s *upkeepContractSpy) Upkeep(st *sim.GameState, g *genome.Genome) {
	if s.upkeeps[st] != s.applies[st]-1 {
		s.violations = append(s.violations, fmt.Sprintf(
			"Upkeep with applies=%d upkeeps=%d (want upkeeps == applies-1: exactly one Upkeep per ApplyMove, none before the first)",
			s.applies[st], s.upkeeps[st]))
	}
	s.upkeeps[st]++
	s.inner.Upkeep(st, g)
}

func (s *upkeepContractSpy) GenerateMoves(st *sim.GameState, g *genome.Genome) []sim.Move {
	return s.inner.GenerateMoves(st, g)
}

func (s *upkeepContractSpy) ApplyMove(st *sim.GameState, mv sim.Move, g *genome.Genome) []sim.Event {
	if s.applies[st] != s.upkeeps[st] {
		s.violations = append(s.violations, fmt.Sprintf(
			"ApplyMove with applies=%d upkeeps=%d (a previous apply was not followed by exactly one Upkeep)",
			s.applies[st], s.upkeeps[st]))
	}
	s.applies[st]++
	return s.inner.ApplyMove(st, mv, g)
}

func (s *upkeepContractSpy) CheckEnd(st *sim.GameState, g *genome.Genome) int {
	return s.inner.CheckEnd(st, g)
}

func (s *upkeepContractSpy) Progress(st *sim.GameState, g *genome.Genome) []float64 {
	return s.inner.Progress(st, g)
}

// TestMCTSUpkeepExactlyOncePerSimulatedMove drives MCTS decisions along a
// real gin-rummy game with the spy installed. Any deviation from the
// once-per-applied-move Upkeep contract inside the search is a violation.
// This is the search-loop half of the plan's double-banked-deadwood
// regression (TestMCTSKnockBanksDeadwoodOnce is the end-to-end half).
func TestMCTSUpkeepExactlyOncePerSimulatedMove(t *testing.T) {
	g := seeds.GinRummy()
	real := &rummy.Runner{}
	spy := newUpkeepContractSpy(real)
	ai := &sim.MCTSAI{Runner: spy, Genome: g, Iterations: 30, Determinizations: 3, RolloutCap: 60}

	rng := rand.New(rand.NewPCG(21, 0))
	st := real.Setup(g, rng)
	decisions := 0
	for decisions < 8 {
		// The REAL game loop (raw runner, not the spy: the spy's books track
		// search-internal states only).
		real.Upkeep(st, g)
		if real.CheckEnd(st, g) >= 0 || st.Turn >= g.MaxTurns() {
			break
		}
		moves := real.GenerateMoves(st, g)
		if len(moves) == 0 {
			break
		}
		mv := ai.SelectMove(moves, st, rng)
		real.ApplyMove(st, mv, g)
		decisions++
	}

	if decisions == 0 {
		t.Fatal("game ended before any MCTS decision")
	}
	if len(spy.violations) > 0 {
		t.Fatalf("Upkeep contract violated %d time(s) inside the search; first: %s",
			len(spy.violations), spy.violations[0])
	}
	if len(spy.applies) == 0 {
		t.Fatal("spy saw no search activity -- test wiring broken")
	}
}

// TestMCTSSelfPlayGinRummyValidGame: plan regression -- an MCTS self-play gin
// rummy game stepped with the same loop shape and seed structure RunBatch
// uses (rng = PCG(baseSeed+i, 0)) must complete with consistent final scores:
// deadwood only ever subtracts (all scores <= 0, gin seed has no scoring
// hooks) and the reported winner is the score argmax, exactly like a RunBatch
// completion. Double-banked deadwood breaks the winner/argmax consistency
// whenever it flips the comparison -- the constructed-state test pins the
// exact numbers; this one proves the property on a full organic game.
func TestMCTSSelfPlayGinRummyValidGame(t *testing.T) {
	g := seeds.GinRummy()
	runner := &rummy.Runner{}
	ai := &sim.MCTSAI{Runner: runner, Genome: g, Iterations: 20, Determinizations: 2, RolloutCap: 40}

	rng := rand.New(rand.NewPCG(3, 0)) // RunBatch seed structure: PCG(baseSeed+i, 0)
	st := runner.Setup(g, rng)
	winner := -1
	for {
		runner.Upkeep(st, g)
		if w := runner.CheckEnd(st, g); w >= 0 {
			winner = w
			break
		}
		if st.Turn >= g.MaxTurns() {
			break
		}
		moves := runner.GenerateMoves(st, g)
		if len(moves) == 0 {
			break
		}
		runner.ApplyMove(st, ai.SelectMove(moves, st, rng), g)
	}

	if winner < 0 || winner >= g.Players {
		t.Fatalf("self-play game did not complete with a valid winner: %d", winner)
	}
	best := 0
	for i := 1; i < len(st.Scores); i++ {
		if st.Scores[i] > st.Scores[best] {
			best = i
		}
	}
	if winner != best {
		t.Errorf("winner %d is not the score argmax %d (scores %v)", winner, best, st.Scores)
	}
	for i, s := range st.Scores {
		if s > 0 {
			t.Errorf("Scores[%d] = %d > 0; gin rummy only banks negative deadwood", i, s)
		}
	}
}

// TestMCTSDeterministic: plan test (c). Same state, same fresh rng seed =>
// same selected move.
func TestMCTSDeterministic(t *testing.T) {
	tc := skeletonCase{"rummy/gin-rummy", seeds.GinRummy(), &rummy.Runner{}}
	st, moves := decisionPoint(t, tc, 9)

	ai := testMCTS(tc.g, tc.runner)
	first := ai.SelectMove(moves, st, rand.New(rand.NewPCG(77, 0)))
	for i := 0; i < 3; i++ {
		again := ai.SelectMove(moves, st, rand.New(rand.NewPCG(77, 0)))
		if !reflect.DeepEqual(first, again) {
			t.Fatalf("run %d: nondeterministic under fixed rng: %v vs %v", i, first, again)
		}
	}
}

// TestMCTSBeatsRandomGinRummy: plan test (b). Seat 0 MCTS vs random over 50
// games must win >= 60% (generous bound; greedy hits ~90% on this seed).
// Settings below the testMCTS defaults keep the 50-game batch fast enough
// for the -race run; measured win rate at these settings was 0.78 (60/4/80
// gave 0.80), comfortably above the bound.
func TestMCTSBeatsRandomGinRummy(t *testing.T) {
	g := seeds.GinRummy()
	runner := &rummy.Runner{}
	mcts := &sim.MCTSAI{Runner: runner, Genome: g, Iterations: 24, Determinizations: 3, RolloutCap: 50}
	ai := &sim.PerPlayerAI{
		Players:  []sim.AIPlayer{mcts, &sim.RandomAI{}},
		Fallback: &sim.RandomAI{},
	}

	result := sim.RunBatch(g, runner, ai, 50, 4242)
	if result.Completions < 45 {
		t.Fatalf("only %d/50 games completed (errors %d, timeouts %d)",
			result.Completions, result.Errors, result.Timeouts)
	}
	winRate := float64(result.WinCounts[0]) / float64(result.Completions)
	t.Logf("MCTS seat-0 win rate: %.2f (%d/%d)", winRate, result.WinCounts[0], result.Completions)
	if winRate < 0.60 {
		t.Errorf("MCTS win rate %.2f < 0.60 over %d completed games", winRate, result.Completions)
	}
}

// TestMCTSFallbacks: degenerate inputs must not panic the batch worker.
func TestMCTSFallbacks(t *testing.T) {
	g := seeds.GinRummy()
	runner := &rummy.Runner{}
	ai := testMCTS(g, runner)
	rng := rand.New(rand.NewPCG(1, 0))

	if got := ai.SelectMove(nil, nil, rng); got.Type != sim.MovePass {
		t.Errorf("empty move list: got %v, want MovePass", got)
	}

	only := []sim.Move{{Type: sim.MoveDraw, PlayerID: 0}}
	if got := ai.SelectMove(only, nil, rng); !reflect.DeepEqual(got, only[0]) {
		t.Errorf("single move: got %v, want %v", got, only[0])
	}

	// Nil Runner/Genome: must degrade to a uniform choice, never crash.
	tc := skeletonCase{"rummy", seeds.GinRummy(), runner}
	st, moves := decisionPoint(t, tc, 2)
	bare := &sim.MCTSAI{}
	got := bare.SelectMove(moves, st, rng)
	found := false
	for _, m := range moves {
		if reflect.DeepEqual(m, got) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("nil-config MCTS returned a move outside the legal list: %v", got)
	}
}

// BenchmarkMCTSGame measures one full gin-rummy game with seat 0 at the
// PRODUCTION defaults (Iterations 200, Determinizations 10, RolloutCap 200)
// vs a random opponent -- the unit Task 20 schedules 20x per Tier 2 genome.
// Budget check: 20 * ns/op must be <= 2s single-threaded. Measured
// 2026-06-11: ~720-775ms/op => ~14.5s per 20 games, budget FAILED ~7x; see
// the performance note in mcts.go (cost is rummy-runner move generation, not
// the sim layer; Task 20 takes the top-decile fallback).
func BenchmarkMCTSGame(b *testing.B) {
	g := seeds.GinRummy()
	runner := &rummy.Runner{}
	ai := &sim.PerPlayerAI{
		Players:  []sim.AIPlayer{&sim.MCTSAI{Runner: runner, Genome: g}, &sim.RandomAI{}},
		Fallback: &sim.RandomAI{},
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sim.RunBatch(g, runner, ai, 1, uint64(1000+i))
	}
}

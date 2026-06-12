// Clone + determinization tests live in sim_test (external) so they can drive
// the REAL skeleton runners; importing them from an in-package test would be
// an import cycle (the skeletons import sim). Audit Task 19.
package sim_test

import (
	"math/rand/v2"
	"reflect"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
	"github.com/darwindeck/darwindeck/pkg/sim"
	"github.com/darwindeck/darwindeck/pkg/skeleton/rummy"
	"github.com/darwindeck/darwindeck/pkg/skeleton/shedding"
	"github.com/darwindeck/darwindeck/pkg/skeleton/tricktaking"
)

// stepOnce mirrors one iteration of sim.runSingleGame's loop shape: Upkeep
// exactly once, CheckEnd, GenerateMoves, apply one random move. Returns false
// when the game is over (winner, no moves, or max turns). Upkeep is NOT
// idempotent (rummy banks deadwood, tricktaking advances Round/redeals), so
// every test that advances a state MUST go through this helper rather than
// calling runner methods ad hoc.
func stepOnce(t *testing.T, runner sim.GenericRunner, g *genome.Genome, st *sim.GameState, rng *rand.Rand) bool {
	t.Helper()
	runner.Upkeep(st, g)
	if runner.CheckEnd(st, g) >= 0 {
		return false
	}
	if st.Turn >= g.MaxTurns() {
		return false
	}
	moves := runner.GenerateMoves(st, g)
	if len(moves) == 0 {
		return false
	}
	runner.ApplyMove(st, moves[rng.IntN(len(moves))], g)
	return true
}

// skeletonCase pairs each skeleton's seed genome with its runner for
// table-driven coverage of all three game loops.
type skeletonCase struct {
	name   string
	g      *genome.Genome
	runner sim.GenericRunner
}

func allSkeletonCases() []skeletonCase {
	return []skeletonCase{
		{"shedding/crazy-eights", seeds.CrazyEights(), &shedding.Runner{}},
		{"tricktaking/whist", seeds.Whist(), &tricktaking.Runner{}},
		{"rummy/gin-rummy", seeds.GinRummy(), &rummy.Runner{}},
	}
}

// TestMoveIdentityStableAcrossClones is Task 19 Step 0 (prerequisite for any
// MCTS code): MCTS aggregates statistics for "the same move" across clones,
// so GenerateMoves on a state and on its clone must return element-wise
// identical move lists (order AND content), and Move.Key must agree pairwise.
// Checked at every decision point of a 30-step game per skeleton, not just at
// Setup, so mid-game phases (rummy meld/discard, trick follows) are covered.
func TestMoveIdentityStableAcrossClones(t *testing.T) {
	for _, tc := range allSkeletonCases() {
		t.Run(tc.name, func(t *testing.T) {
			rng := rand.New(rand.NewPCG(42, 0))
			st := tc.runner.Setup(tc.g, rng)
			for step := 0; step < 30; step++ {
				tc.runner.Upkeep(st, tc.g)
				if tc.runner.CheckEnd(st, tc.g) >= 0 {
					break
				}
				moves := tc.runner.GenerateMoves(st, tc.g)
				if len(moves) == 0 {
					break
				}

				cl := st.Clone()
				cloneMoves := tc.runner.GenerateMoves(cl, tc.g)
				if !reflect.DeepEqual(moves, cloneMoves) {
					t.Fatalf("step %d: moves diverge across clone:\n original: %v\n clone:    %v", step, moves, cloneMoves)
				}
				seen := make(map[string]int, len(moves))
				for i := range moves {
					k1, k2 := moves[i].Key(), cloneMoves[i].Key()
					if k1 != k2 {
						t.Fatalf("step %d move %d: Key mismatch %q vs %q", step, i, k1, k2)
					}
					if j, dup := seen[k1]; dup {
						t.Fatalf("step %d: duplicate move key %q at indices %d and %d -- MCTS children maps need unique keys", step, k1, j, i)
					}
					seen[k1] = i
				}

				tc.runner.ApplyMove(st, moves[rng.IntN(len(moves))], tc.g)
			}
		})
	}
}

// TestMoveKeyDistinguishesMoves pins Key's discriminating power: type, player,
// and card content must all be visible in the key, with no separator
// ambiguity between rank/suit digits.
func TestMoveKeyDistinguishesMoves(t *testing.T) {
	c1 := sim.Card{Suit: sim.Hearts, Rank: sim.Ten}
	c2 := sim.Card{Suit: sim.Spades, Rank: sim.Two}
	base := sim.Move{Type: sim.MovePlay, Cards: []sim.Card{c1}, PlayerID: 0}

	variants := []sim.Move{
		{Type: sim.MoveDiscard, Cards: []sim.Card{c1}, PlayerID: 0}, // type differs
		{Type: sim.MovePlay, Cards: []sim.Card{c2}, PlayerID: 0},    // card differs
		{Type: sim.MovePlay, Cards: []sim.Card{c1}, PlayerID: 1},    // player differs
		{Type: sim.MovePlay, Cards: []sim.Card{c1, c2}, PlayerID: 0}, // extra card
		{Type: sim.MovePlay, PlayerID: 0},                            // no cards
	}
	for i, v := range variants {
		if v.Key() == base.Key() {
			t.Errorf("variant %d: Key %q collides with base move", i, v.Key())
		}
	}

	same := sim.Move{Type: sim.MovePlay, Cards: []sim.Card{{Suit: sim.Hearts, Rank: sim.Ten}}, PlayerID: 0}
	if same.Key() != base.Key() {
		t.Errorf("equal moves produced different keys: %q vs %q", same.Key(), base.Key())
	}
}

// fullyPopulatedState returns a GameState with every field set to a non-zero
// value so the clone test can verify each one field-by-field.
func fullyPopulatedState() *sim.GameState {
	return &sim.GameState{
		Deck:    []sim.Card{{Suit: sim.Clubs, Rank: sim.Two}, {Suit: sim.Diamonds, Rank: sim.Nine}},
		Hands:   [][]sim.Card{{{Suit: sim.Hearts, Rank: sim.Ace}}, {{Suit: sim.Spades, Rank: sim.King}, {Suit: sim.Clubs, Rank: sim.Five}}},
		Discard: []sim.Card{{Suit: sim.Diamonds, Rank: sim.Three}},
		Tableau: [][]sim.Card{{{Suit: sim.Clubs, Rank: sim.Jack}}, {}},
		Scores:  []int{3, -7},

		Turn:       9,
		Active:     1,
		Phase:      sim.PhaseMeld,
		NumPlayers: 2,
		Direction:  -1,

		Round:    2,
		MaxRound: 5,

		TopCard: &sim.Card{Suit: sim.Hearts, Rank: sim.Queen},

		TrickCards:   []sim.Card{{Suit: sim.Spades, Rank: sim.Four}},
		TrickPlayers: []int{0},
		TrickLeader:  1,
		TrumpSuit:    2,
		TrickBroken:  true,

		Melds:     [][]sim.Card{{{Suit: sim.Hearts, Rank: sim.Six}, {Suit: sim.Hearts, Rank: sim.Seven}, {Suit: sim.Hearts, Rank: sim.Eight}}},
		MeldOwner: []int{0},

		Events: []sim.Event{{Type: sim.EventCardPlayed, PlayerID: 1, Detail: "discard"}},
		RNG:    rand.New(rand.NewPCG(7, 0)),
	}
}

// TestCloneFieldByField: every copied field of the clone equals the original,
// Events are excluded (observational; cloning them per rollout wastes memory),
// and RNG is not shared (a clone advancing the original game's RNG would make
// AI search non-pure).
func TestCloneFieldByField(t *testing.T) {
	st := fullyPopulatedState()
	cp := st.Clone()

	if cp == st {
		t.Fatal("Clone returned the same pointer")
	}

	checks := []struct {
		name       string
		got, want  interface{}
	}{
		{"Deck", cp.Deck, st.Deck},
		{"Hands", cp.Hands, st.Hands},
		{"Discard", cp.Discard, st.Discard},
		{"Tableau", cp.Tableau, st.Tableau},
		{"Scores", cp.Scores, st.Scores},
		{"Turn", cp.Turn, st.Turn},
		{"Active", cp.Active, st.Active},
		{"Phase", cp.Phase, st.Phase},
		{"NumPlayers", cp.NumPlayers, st.NumPlayers},
		{"Direction", cp.Direction, st.Direction},
		{"Round", cp.Round, st.Round},
		{"MaxRound", cp.MaxRound, st.MaxRound},
		{"TrickCards", cp.TrickCards, st.TrickCards},
		{"TrickPlayers", cp.TrickPlayers, st.TrickPlayers},
		{"TrickLeader", cp.TrickLeader, st.TrickLeader},
		{"TrumpSuit", cp.TrumpSuit, st.TrumpSuit},
		{"TrickBroken", cp.TrickBroken, st.TrickBroken},
		{"Melds", cp.Melds, st.Melds},
		{"MeldOwner", cp.MeldOwner, st.MeldOwner},
	}
	for _, c := range checks {
		if !reflect.DeepEqual(c.got, c.want) {
			t.Errorf("%s: clone = %v, want %v", c.name, c.got, c.want)
		}
	}

	if cp.TopCard == nil || *cp.TopCard != *st.TopCard {
		t.Errorf("TopCard: clone = %v, want %v", cp.TopCard, st.TopCard)
	}
	if cp.TopCard == st.TopCard {
		t.Error("TopCard pointer is shared with the original")
	}
	if len(cp.Events) != 0 {
		t.Errorf("Events: clone carries %d events, want 0 (excluded by design)", len(cp.Events))
	}
	if cp.RNG != nil {
		t.Error("RNG: clone shares an RNG; must be nil so clones cannot advance the original game's randomness")
	}
}

// TestCloneIsDeep mutates every mutable cell of the clone and asserts the
// original is bit-identical afterwards (no aliased backing arrays).
func TestCloneIsDeep(t *testing.T) {
	st := fullyPopulatedState()
	snapshot := st.Clone() // second clone as the reference snapshot

	cp := st.Clone()
	cp.Deck[0] = sim.Card{Suit: sim.Spades, Rank: sim.Ace}
	cp.Deck = append(cp.Deck, sim.Card{Suit: sim.Clubs, Rank: sim.Ten})
	cp.Hands[0][0] = sim.Card{Suit: sim.Clubs, Rank: sim.Two}
	cp.Hands[1] = append(cp.Hands[1], sim.Card{Suit: sim.Diamonds, Rank: sim.Queen})
	cp.Discard[0] = sim.Card{Suit: sim.Hearts, Rank: sim.Two}
	cp.Tableau[0][0] = sim.Card{Suit: sim.Diamonds, Rank: sim.Ace}
	cp.Scores[0] = 999
	cp.TopCard.Rank = sim.Two
	cp.TrickCards[0] = sim.Card{Suit: sim.Clubs, Rank: sim.Three}
	cp.TrickPlayers[0] = 1
	cp.Melds[0][0] = sim.Card{Suit: sim.Spades, Rank: sim.Nine}
	cp.MeldOwner[0] = 1
	cp.Turn = 1000
	cp.Active = 0
	cp.Phase = sim.PhaseEnd
	cp.Direction = 1
	cp.Round = 99
	cp.TrumpSuit = 0
	cp.TrickBroken = false

	for name, pair := range map[string][2]interface{}{
		"Deck":         {st.Deck, snapshot.Deck},
		"Hands":        {st.Hands, snapshot.Hands},
		"Discard":      {st.Discard, snapshot.Discard},
		"Tableau":      {st.Tableau, snapshot.Tableau},
		"Scores":       {st.Scores, snapshot.Scores},
		"TrickCards":   {st.TrickCards, snapshot.TrickCards},
		"TrickPlayers": {st.TrickPlayers, snapshot.TrickPlayers},
		"Melds":        {st.Melds, snapshot.Melds},
		"MeldOwner":    {st.MeldOwner, snapshot.MeldOwner},
	} {
		if !reflect.DeepEqual(pair[0], pair[1]) {
			t.Errorf("%s: original mutated through clone:\n got  %v\n want %v", name, pair[0], pair[1])
		}
	}
	if *st.TopCard != *snapshot.TopCard {
		t.Errorf("TopCard: original mutated through clone: got %v, want %v", st.TopCard, snapshot.TopCard)
	}
	if st.Turn != snapshot.Turn || st.Active != snapshot.Active || st.Phase != snapshot.Phase ||
		st.Direction != snapshot.Direction || st.Round != snapshot.Round ||
		st.TrumpSuit != snapshot.TrumpSuit || st.TrickBroken != snapshot.TrickBroken {
		t.Error("scalar fields of original mutated through clone")
	}
}

// TestGameStateFieldCountPinsClone is the maintenance tripwire: if a field is
// added to (or removed from) GameState, this fails until GameState.Clone
// (pkg/sim/clone.go), Determinize (if the field carries hidden information),
// the field-by-field tests above, and this constant are all updated together.
func TestGameStateFieldCountPinsClone(t *testing.T) {
	const want = 22
	if got := reflect.TypeOf(sim.GameState{}).NumField(); got != want {
		t.Fatalf("GameState has %d fields, want %d -- update Clone/Determinize/clone_test.go for the new field, then bump this constant", got, want)
	}
}

// BenchmarkCloneRollout is Task 19's benchmark-first gate: it measures the
// naive heap-allocating Clone plus one random rollout (the unit MCTS performs
// ~40k times per genome: Iterations 200 x Determinizations 10 x 20 games).
// Run with -benchmem; if GC exceeds ~10% of wall time at that volume, Clone
// must move to sync.Pool or value-semantic state BEFORE the tree is built.
// Measured result is recorded in pkg/sim/mcts.go's allocation-strategy note.
func BenchmarkCloneRollout(b *testing.B) {
	g := seeds.GinRummy()
	runner := &rummy.Runner{}
	setupRNG := rand.New(rand.NewPCG(1, 0))
	base := runner.Setup(g, setupRNG)
	// Advance to a mid-game state so rollouts exercise meld/discard phases.
	for i := 0; i < 12; i++ {
		runner.Upkeep(base, g)
		if runner.CheckEnd(base, g) >= 0 {
			break
		}
		moves := runner.GenerateMoves(base, g)
		if len(moves) == 0 {
			break
		}
		runner.ApplyMove(base, moves[setupRNG.IntN(len(moves))], g)
	}

	rng := rand.New(rand.NewPCG(2, 0))
	maxTurns := g.MaxTurns()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		st := base.Clone()
		st.RNG = rng
		for applied := 0; applied < 200; applied++ {
			runner.Upkeep(st, g)
			if runner.CheckEnd(st, g) >= 0 || st.Turn >= maxTurns {
				break
			}
			moves := runner.GenerateMoves(st, g)
			if len(moves) == 0 {
				break
			}
			runner.ApplyMove(st, moves[rng.IntN(len(moves))], g)
		}
	}
}

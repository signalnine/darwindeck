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
		PassCount:    1,

		Melds:     [][]sim.Card{{{Suit: sim.Hearts, Rank: sim.Six}, {Suit: sim.Hearts, Rank: sim.Seven}, {Suit: sim.Hearts, Rank: sim.Eight}}},
		MeldOwner: []int{0},

		Pot:        50,
		CurrentBet: 20,
		Committed:  []int{20, 10},
		Folded:     []bool{false, true},
		RaiseCount: 1,
		ToAct:      1,

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
		{"PassCount", cp.PassCount, st.PassCount},
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
	cp.Committed[0] = 777
	cp.Folded[0] = true
	cp.Turn = 1000
	cp.Active = 0
	cp.Phase = sim.PhaseEnd
	cp.Direction = 1
	cp.Round = 99
	cp.TrumpSuit = 0
	cp.TrickBroken = false
	cp.PassCount = 0
	cp.Pot = 0
	cp.CurrentBet = 0
	cp.RaiseCount = 9
	cp.ToAct = 9

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
		"Committed":    {st.Committed, snapshot.Committed},
		"Folded":       {st.Folded, snapshot.Folded},
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
		st.TrumpSuit != snapshot.TrumpSuit || st.TrickBroken != snapshot.TrickBroken ||
		st.PassCount != snapshot.PassCount ||
		st.Pot != snapshot.Pot || st.CurrentBet != snapshot.CurrentBet ||
		st.RaiseCount != snapshot.RaiseCount || st.ToAct != snapshot.ToAct {
		t.Error("scalar fields of original mutated through clone")
	}
}

// TestGameStateFieldCountPinsClone is the maintenance tripwire: if a field is
// added to (or removed from) GameState, this fails until GameState.Clone
// (pkg/sim/clone.go), Determinize (if the field carries hidden information),
// the field-by-field tests above, and this constant are all updated together.
func TestGameStateFieldCountPinsClone(t *testing.T) {
	const want = 29 // +6 vying betting fields: Pot, CurrentBet, Committed, Folded, RaiseCount, ToAct
	if got := reflect.TypeOf(sim.GameState{}).NumField(); got != want {
		t.Fatalf("GameState has %d fields, want %d -- update Clone/Determinize/clone_test.go for the new field, then bump this constant", got, want)
	}
}

// --- Determinization (Task 19 step 2) ---

// cardMultiset counts every card in every zone of the state. Determinization
// must conserve this exactly: it may move hidden cards between hidden zones
// but can never create, destroy, or duplicate a card.
func cardMultiset(st *sim.GameState) map[sim.Card]int {
	ms := make(map[sim.Card]int, 52)
	add := func(cards []sim.Card) {
		for _, c := range cards {
			ms[c]++
		}
	}
	add(st.Deck)
	for _, h := range st.Hands {
		add(h)
	}
	add(st.Discard)
	for _, tb := range st.Tableau {
		add(tb)
	}
	for _, m := range st.Melds {
		add(m)
	}
	add(st.TrickCards)
	return ms
}

// midGameState advances a fresh game `steps` applied moves so determinization
// tests run against states with populated discard/tableau/meld zones.
func midGameState(t *testing.T, tc skeletonCase, seed uint64, steps int) *sim.GameState {
	t.Helper()
	rng := rand.New(rand.NewPCG(seed, 0))
	st := tc.runner.Setup(tc.g, rng)
	for i := 0; i < steps; i++ {
		if !stepOnce(t, tc.runner, tc.g, st, rng) {
			break
		}
	}
	return st
}

// TestDeterminizePreservesPublicInfo is the hidden-info contract from the
// plan: from player p's perspective the hidden cards are the deck plus all
// OTHER hands; p's own hand and every public zone must be byte-identical
// after determinization, hidden zone SIZES must be preserved, and the full
// card multiset must be conserved. v1's MCTS was omniscient (it cloned hidden
// hands verbatim); this test plus TestDeterminizeActuallyShuffles guards
// against recreating that.
func TestDeterminizePreservesPublicInfo(t *testing.T) {
	for _, tc := range allSkeletonCases() {
		t.Run(tc.name, func(t *testing.T) {
			for seed := uint64(1); seed <= 5; seed++ {
				st := midGameState(t, tc, seed, 20)
				p := st.Active
				before := st.Clone() // reference snapshot
				wantMS := cardMultiset(st)

				det := sim.Determinize(st, p, rand.New(rand.NewPCG(seed*100, 1)))

				// The original must be untouched.
				detOriginal := st.Clone()
				detOriginal.Events, before.Events = nil, nil
				if !reflect.DeepEqual(detOriginal, before) {
					t.Fatalf("seed %d: Determinize mutated the original state", seed)
				}

				// p's own hand and all public zones identical.
				if !reflect.DeepEqual(det.Hands[p], st.Hands[p]) {
					t.Errorf("seed %d: player %d's own hand changed: %v -> %v", seed, p, st.Hands[p], det.Hands[p])
				}
				public := []struct {
					name      string
					got, want interface{}
				}{
					{"Discard", det.Discard, st.Discard},
					{"Tableau", det.Tableau, st.Tableau},
					{"Melds", det.Melds, st.Melds},
					{"MeldOwner", det.MeldOwner, st.MeldOwner},
					{"TrickCards", det.TrickCards, st.TrickCards},
					{"TrickPlayers", det.TrickPlayers, st.TrickPlayers},
					{"Scores", det.Scores, st.Scores},
					{"Turn", det.Turn, st.Turn},
					{"Active", det.Active, st.Active},
					{"Phase", det.Phase, st.Phase},
					{"Direction", det.Direction, st.Direction},
					{"Round", det.Round, st.Round},
					{"TrumpSuit", det.TrumpSuit, st.TrumpSuit},
					{"TrickBroken", det.TrickBroken, st.TrickBroken},
				}
				for _, c := range public {
					if !reflect.DeepEqual(c.got, c.want) {
						t.Errorf("seed %d: public field %s changed: %v -> %v", seed, c.name, c.want, c.got)
					}
				}
				if (det.TopCard == nil) != (st.TopCard == nil) ||
					(det.TopCard != nil && *det.TopCard != *st.TopCard) {
					t.Errorf("seed %d: TopCard changed: %v -> %v", seed, st.TopCard, det.TopCard)
				}

				// Hidden zone sizes preserved.
				if len(det.Deck) != len(st.Deck) {
					t.Errorf("seed %d: deck size %d -> %d", seed, len(st.Deck), len(det.Deck))
				}
				for i := range st.Hands {
					if len(det.Hands[i]) != len(st.Hands[i]) {
						t.Errorf("seed %d: hand %d size %d -> %d", seed, i, len(st.Hands[i]), len(det.Hands[i]))
					}
				}

				// Full multiset conservation.
				if got := cardMultiset(det); !reflect.DeepEqual(got, wantMS) {
					t.Errorf("seed %d: card multiset not conserved", seed)
				}
			}
		})
	}
}

// TestDeterminizeActuallyShuffles: across many determinizations the hidden
// zones must actually vary -- otherwise Determinize is a glorified Clone and
// MCTS would search the true hidden state (v1's omniscience bug).
func TestDeterminizeActuallyShuffles(t *testing.T) {
	tc := skeletonCase{"rummy/gin-rummy", seeds.GinRummy(), &rummy.Runner{}}
	st := midGameState(t, tc, 3, 6) // early game: big deck, full opponent hand
	p := st.Active
	opp := (p + 1) % st.NumPlayers

	changed := false
	for i := uint64(0); i < 20 && !changed; i++ {
		det := sim.Determinize(st, p, rand.New(rand.NewPCG(i, 7)))
		if !reflect.DeepEqual(det.Hands[opp], st.Hands[opp]) {
			changed = true
		}
	}
	if !changed {
		t.Fatal("20 determinizations never changed the opponent's hand -- hidden info is leaking into the search")
	}
}

// TestMoveIdentityStableAcrossDeterminizations is the second half of Task 19
// Step 0: moves generated on two different determinizations of the same
// info-state refer only to the acting player's own (known) cards and public
// zones, so the move lists -- and their keys -- must be element-wise
// identical. Checked at every decision point along a 30-step game.
func TestMoveIdentityStableAcrossDeterminizations(t *testing.T) {
	for _, tc := range allSkeletonCases() {
		t.Run(tc.name, func(t *testing.T) {
			rng := rand.New(rand.NewPCG(11, 0))
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

				det1 := sim.Determinize(st, st.Active, rand.New(rand.NewPCG(uint64(step), 1)))
				det2 := sim.Determinize(st, st.Active, rand.New(rand.NewPCG(uint64(step), 2)))
				m1 := tc.runner.GenerateMoves(det1, tc.g)
				m2 := tc.runner.GenerateMoves(det2, tc.g)
				if !reflect.DeepEqual(m1, moves) || !reflect.DeepEqual(m2, moves) {
					t.Fatalf("step %d: determinized move lists diverge from original\n original: %v\n det1:     %v\n det2:     %v", step, moves, m1, m2)
				}
				for i := range moves {
					if k, k1, k2 := moves[i].Key(), m1[i].Key(), m2[i].Key(); k != k1 || k != k2 {
						t.Fatalf("step %d move %d: keys diverge: %q / %q / %q", step, i, k, k1, k2)
					}
				}

				tc.runner.ApplyMove(st, moves[rng.IntN(len(moves))], tc.g)
			}
		})
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

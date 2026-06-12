// Package sim_test exercises the batch runner from outside the sim package so
// it can drive the REAL skeleton runners (shedding, rummy, tricktaking).
// Importing them from an in-package test would be an import cycle: the
// skeletons import sim.
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

// stubRunner satisfies GenericRunner but is never invoked when n==0.
type stubRunner struct{}

func (stubRunner) Setup(g *genome.Genome, rng *rand.Rand) *sim.GameState { return nil }
func (stubRunner) Upkeep(state *sim.GameState, g *genome.Genome)         {}
func (stubRunner) GenerateMoves(state *sim.GameState, g *genome.Genome) []sim.Move {
	return nil
}
func (stubRunner) ApplyMove(state *sim.GameState, move sim.Move, g *genome.Genome) []sim.Event {
	return nil
}
func (stubRunner) CheckEnd(state *sim.GameState, g *genome.Genome) int { return -1 }
func (stubRunner) Progress(state *sim.GameState, g *genome.Genome) []float64 {
	return nil
}

// TestRunBatchEmptyDoesNotLeakSentinelMinTurns guards against dd-d80:
// when n==0 the loop never runs, so MinTurns must not surface as the
// initialization sentinel (~2.1B) to downstream metrics.
func TestRunBatchEmptyDoesNotLeakSentinelMinTurns(t *testing.T) {
	g := &genome.Genome{Players: 2, HandSize: 5, Skeleton: genome.Shedding}
	result := sim.RunBatch(g, stubRunner{}, &sim.RandomAI{}, 0, 1)

	if result.GamesPlayed != 0 {
		t.Fatalf("GamesPlayed = %d, want 0", result.GamesPlayed)
	}
	if result.MinTurns != 0 {
		t.Errorf("MinTurns = %d on empty batch, want 0 (sentinel leaked to caller)", result.MinTurns)
	}
	if result.MaxTurns != 0 {
		t.Errorf("MaxTurns = %d on empty batch, want 0", result.MaxTurns)
	}
}

// fixedSetupRunner wraps a real skeleton runner but substitutes a
// hand-constructed state from Setup, so OptionDelta semantics can be verified
// against values computed by hand (audit Task 7).
type fixedSetupRunner struct {
	sim.GenericRunner
	build func() *sim.GameState
}

func (f fixedSetupRunner) Setup(g *genome.Genome, rng *rand.Rand) *sim.GameState {
	st := f.build()
	st.RNG = rng
	return st
}

func card(s sim.Suit, r sim.Rank) sim.Card { return sim.Card{Suit: s, Rank: r} }

// TestBatchRecordsPerTurnData is the Task 7 entry test: a 5-game shedding
// batch must record one TurnRecord per applied move, with a sane player index
// and at least one legal move, and AllTurns/AllLeaders must stay parallel to
// AllEvents.
func TestBatchRecordsPerTurnData(t *testing.T) {
	g := seeds.CrazyEights()
	result := sim.RunBatch(g, &shedding.Runner{}, &sim.RandomAI{}, 5, 1)

	if len(result.AllTurns) != result.GamesPlayed {
		t.Fatalf("len(AllTurns) = %d, want %d (parallel to games)", len(result.AllTurns), result.GamesPlayed)
	}
	if len(result.AllTurns) != len(result.AllEvents) {
		t.Fatalf("len(AllTurns) = %d, len(AllEvents) = %d: must be parallel", len(result.AllTurns), len(result.AllEvents))
	}
	if len(result.AllLeaders) != result.GamesPlayed {
		t.Fatalf("len(AllLeaders) = %d, want %d (parallel to games)", len(result.AllLeaders), result.GamesPlayed)
	}
	for i, turns := range result.AllTurns {
		if len(turns) == 0 {
			t.Errorf("game %d: no turn records", i)
		}
		for j, tr := range turns {
			if tr.LegalMoves < 1 {
				t.Errorf("game %d turn %d: LegalMoves = %d, want >= 1", i, j, tr.LegalMoves)
			}
			if tr.Player < 0 || tr.Player >= g.Players {
				t.Errorf("game %d turn %d: Player = %d out of range", i, j, tr.Player)
			}
		}
	}
	// Leaders (Task 8): one argmax-of-Progress entry per applied move,
	// parallel to the TurnRecords, each either a player index or -1 (tie).
	for i, leaders := range result.AllLeaders {
		if len(leaders) != len(result.AllTurns[i]) {
			t.Errorf("game %d: len(Leaders) = %d, want %d (parallel to TurnRecords)", i, len(leaders), len(result.AllTurns[i]))
		}
		for j, l := range leaders {
			if l < -1 || int(l) >= g.Players {
				t.Errorf("game %d move %d: leader = %d, want -1..%d", i, j, l, g.Players-1)
			}
		}
	}
}

// TestForcedDrawRecordsOneLegalMove pins the plan's forced-play fixture: a
// hand that cannot play anything has exactly one legal move (draw), and that
// must be recorded as LegalMoves == 1.
func TestForcedDrawRecordsOneLegalMove(t *testing.T) {
	g := &genome.Genome{
		Skeleton: genome.Shedding,
		Players:  2,
		HandSize: 1,
		Shedding: &genome.SheddingParams{MatchRule: genome.MatchEither, DrawPenalty: 1},
	}
	runner := fixedSetupRunner{
		GenericRunner: &shedding.Runner{},
		build: func() *sim.GameState {
			st := sim.NewGameState(2)
			top := card(sim.Hearts, sim.Seven)
			st.Discard = []sim.Card{top}
			st.TopCard = &top
			// P0 cannot play 9D on 7H (no suit or rank match): forced draw.
			st.Hands[0] = []sim.Card{card(sim.Diamonds, sim.Nine)}
			// P1 holds a rank match and wins on their turn.
			st.Hands[1] = []sim.Card{card(sim.Diamonds, sim.Seven)}
			st.Deck = []sim.Card{card(sim.Clubs, sim.Two), card(sim.Clubs, sim.Three)}
			st.Phase = sim.PhasePlay
			return st
		},
	}

	result := sim.RunBatch(g, runner, &sim.RandomAI{}, 1, 1)
	turns := result.AllTurns[0]
	if len(turns) == 0 {
		t.Fatal("no turn records")
	}
	first := turns[0]
	if first.Player != 0 {
		t.Errorf("first record Player = %d, want 0", first.Player)
	}
	if first.LegalMoves != 1 {
		t.Errorf("forced draw: LegalMoves = %d, want 1", first.LegalMoves)
	}
	// The draw leaves the discard top unchanged, so P1's options are
	// untouched: 7D matched rank-7 before and after. Delta is 0.
	if first.OptionDelta != 0 {
		t.Errorf("forced draw OptionDelta = %d, want 0 (top card unchanged)", first.OptionDelta)
	}
}

// TestSheddingOptionDeltaHandComputed verifies the shedding OptionDelta
// definition from the Task 7 table: options(next) = legal plays+draw for the
// next player against the discard top, before vs after the move.
//
// Hand-computed: top is 7H. P1 holds {9S, 5H, 6H}: against 7H both hearts
// match by suit => 2 options. P0's only legal play is 7S (rank match). After
// it, the top is 7S and P1's only match is 9S (suit) => 1 option.
// OptionDelta = 1 - 2 = -1.
func TestSheddingOptionDeltaHandComputed(t *testing.T) {
	g := &genome.Genome{
		Skeleton: genome.Shedding,
		Players:  2,
		HandSize: 3,
		Shedding: &genome.SheddingParams{MatchRule: genome.MatchEither, DrawPenalty: 1},
	}
	runner := fixedSetupRunner{
		GenericRunner: &shedding.Runner{},
		build: func() *sim.GameState {
			st := sim.NewGameState(2)
			top := card(sim.Hearts, sim.Seven)
			st.Discard = []sim.Card{top}
			st.TopCard = &top
			// 7S matches rank; 2D matches nothing => exactly one legal move,
			// and the hand stays non-empty so the game continues.
			st.Hands[0] = []sim.Card{card(sim.Spades, sim.Seven), card(sim.Diamonds, sim.Two)}
			st.Hands[1] = []sim.Card{card(sim.Spades, sim.Nine), card(sim.Hearts, sim.Five), card(sim.Hearts, sim.Six)}
			st.Deck = []sim.Card{card(sim.Clubs, sim.Three), card(sim.Clubs, sim.Four)}
			st.Phase = sim.PhasePlay
			return st
		},
	}

	result := sim.RunBatch(g, runner, &sim.RandomAI{}, 1, 1)
	turns := result.AllTurns[0]
	if len(turns) == 0 {
		t.Fatal("no turn records")
	}
	first := turns[0]
	if first.Player != 0 || first.LegalMoves != 1 {
		t.Fatalf("first record = %+v, want Player 0 with exactly 1 legal move", first)
	}
	if first.OptionDelta != -1 {
		t.Errorf("shedding OptionDelta = %d, want -1 (P1 went from 2 matches to 1)", first.OptionDelta)
	}
}

// TestRummyOptionDeltaHandComputed verifies the rummy OptionDelta definition
// from the Task 7 table: options(next) = legal draws+melds+discards for the
// next player (the union across the three turn phases), before vs after the
// turn-passing move. Deltas attach only to MoveDiscard -- draw/meld/pass keep
// the mover acting, and self-perturbation is not coupling, so those record 0.
//
// Hand-computed for P1 = {2H, 2D, 7S} with MeldSets/min 3, DrawEither,
// knock at 10 (deadwood 2+2+7 = 11, so no knock; no 3-of-a-kind, so no melds):
//
//	before P0's discard (discard pile EMPTY): draw 1 (deck only) + meld 1
//	(pass only) + discard 3 = 5
//	after P0 discards a card:                 draw 2 (deck + discard) + 1 + 3 = 6
//
// OptionDelta = 6 - 5 = +1.
func TestRummyOptionDeltaHandComputed(t *testing.T) {
	g := &genome.Genome{
		Skeleton: genome.Rummy,
		Players:  2,
		HandSize: 3,
		Rummy: &genome.RummyParams{
			MeldTypes:      genome.MeldSets,
			MinMeldSize:    3,
			DrawFrom:       genome.DrawEither,
			KnockThreshold: 10,
		},
	}
	runner := fixedSetupRunner{
		GenericRunner: &rummy.Runner{},
		build: func() *sim.GameState {
			st := sim.NewGameState(2)
			st.Phase = sim.PhaseDiscard // P0 is mid-turn, about to discard
			st.Hands[0] = []sim.Card{card(sim.Hearts, sim.Five), card(sim.Clubs, sim.Nine)}
			st.Hands[1] = []sim.Card{card(sim.Hearts, sim.Two), card(sim.Diamonds, sim.Two), card(sim.Spades, sim.Seven)}
			st.Discard = nil // empty: P0's discard CREATES P1's discard-draw option
			st.Deck = []sim.Card{card(sim.Clubs, sim.King), card(sim.Clubs, sim.Queen), card(sim.Clubs, sim.Jack)}
			st.Melds = [][]sim.Card{}
			st.MeldOwner = []int{}
			return st
		},
	}

	result := sim.RunBatch(g, runner, &sim.RandomAI{}, 1, 1)
	turns := result.AllTurns[0]
	if len(turns) == 0 {
		t.Fatal("no turn records")
	}
	first := turns[0]
	if first.Player != 0 || first.LegalMoves != 2 {
		t.Fatalf("first record = %+v, want Player 0 with 2 legal discards", first)
	}
	if first.OptionDelta != 1 {
		t.Errorf("rummy OptionDelta = %d, want +1 (P1 union 5 -> 6)", first.OptionDelta)
	}

	// P1's next decision is the draw (2 legal moves: deck or discard). It
	// does not pass the turn, so it must record OptionDelta 0.
	if len(turns) < 2 {
		t.Fatal("expected a second turn record (P1's draw)")
	}
	second := turns[1]
	if second.Player != 1 || second.LegalMoves != 2 {
		t.Fatalf("second record = %+v, want Player 1 with 2 legal draws", second)
	}
	if second.OptionDelta != 0 {
		t.Errorf("self-handoff (draw) OptionDelta = %d, want 0 (not a coupling move)", second.OptionDelta)
	}
}

// TestTrickTakingOptionDeltaAlwaysZero pins the Task 7 table BY DESIGN:
// trick-taking OptionDelta is not move-count based, because follow-suit
// legality depends on the led card, which the acting player sets. It is 0 on
// every record; interaction signal comes from EventTrickWon/specials instead.
func TestTrickTakingOptionDeltaAlwaysZero(t *testing.T) {
	g := seeds.Whist()
	result := sim.RunBatch(g, &tricktaking.Runner{}, &sim.RandomAI{}, 5, 7)

	sawChoice := false
	for i, turns := range result.AllTurns {
		if len(turns) == 0 {
			t.Errorf("game %d: no turn records", i)
		}
		for j, tr := range turns {
			if tr.OptionDelta != 0 {
				t.Fatalf("game %d turn %d: OptionDelta = %d, want 0 always for trick-taking", i, j, tr.OptionDelta)
			}
			if tr.LegalMoves < 1 {
				t.Errorf("game %d turn %d: LegalMoves = %d, want >= 1", i, j, tr.LegalMoves)
			}
			if tr.LegalMoves > 1 {
				sawChoice = true
			}
		}
	}
	if !sawChoice {
		t.Error("no record with LegalMoves > 1: recording looks dead (whist leads offer a full hand)")
	}
}

// TestBatchLeadersTrackArgmaxProgress verifies the Task 8 leader track by
// hand on the forced-draw fixture (g.HandSize = 1, so shedding Progress is
// 1 - hand/1 floored at 0):
//
//	move 0: P0 forced draw -> P0 holds 2 cards (progress 0, clamped), P1
//	        holds 1 (progress 0). Tied at the max => leader -1.
//	move 1: P1 plays 7D on 7H -> P1 empty (progress 1), P0 at 0 => leader 1,
//	        and P1 wins the game.
func TestBatchLeadersTrackArgmaxProgress(t *testing.T) {
	g := &genome.Genome{
		Skeleton: genome.Shedding,
		Players:  2,
		HandSize: 1,
		Shedding: &genome.SheddingParams{MatchRule: genome.MatchEither, DrawPenalty: 1},
	}
	runner := fixedSetupRunner{
		GenericRunner: &shedding.Runner{},
		build: func() *sim.GameState {
			st := sim.NewGameState(2)
			top := card(sim.Hearts, sim.Seven)
			st.Discard = []sim.Card{top}
			st.TopCard = &top
			// P0 cannot play 9D on 7H: forced draw.
			st.Hands[0] = []sim.Card{card(sim.Diamonds, sim.Nine)}
			// P1 holds a rank match and wins on their turn.
			st.Hands[1] = []sim.Card{card(sim.Diamonds, sim.Seven)}
			st.Deck = []sim.Card{card(sim.Clubs, sim.Two), card(sim.Clubs, sim.Three)}
			st.Phase = sim.PhasePlay
			return st
		},
	}

	result := sim.RunBatch(g, runner, &sim.RandomAI{}, 1, 1)
	want := []int8{-1, 1}
	if !reflect.DeepEqual(result.AllLeaders[0], want) {
		t.Errorf("Leaders = %v, want %v (tie after forced draw, then winner leads)", result.AllLeaders[0], want)
	}
	if result.WinCounts[1] != 1 {
		t.Fatalf("WinCounts = %v, want P1 to win (fixture broken)", result.WinCounts)
	}
}

// TestSameDerivedSeedSameEventStream is the Execution-notes doc-test: per-game
// seeds derive as baseSeed+index, so game i of one batch must be byte-identical
// (events AND turn records) to game 0 of a batch whose baseSeed is shifted by
// i. Batch position must not leak into game outcomes.
func TestSameDerivedSeedSameEventStream(t *testing.T) {
	cases := []struct {
		name   string
		g      *genome.Genome
		runner sim.GenericRunner
	}{
		{"shedding", seeds.CrazyEights(), &shedding.Runner{}},
		{"rummy", seeds.GinRummy(), &rummy.Runner{}}, // exercises mid-game RNG reshuffles
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			const baseSeed, idx = 40, 3
			r1 := sim.RunBatch(tc.g, tc.runner, &sim.RandomAI{}, 5, baseSeed)
			r2 := sim.RunBatch(tc.g, tc.runner, &sim.RandomAI{}, 1, baseSeed+idx)
			if !reflect.DeepEqual(r1.AllEvents[idx], r2.AllEvents[0]) {
				t.Errorf("event streams differ between batch position %d (base %d) and position 0 (base %d)", idx, baseSeed, baseSeed+idx)
			}
			if !reflect.DeepEqual(r1.AllTurns[idx], r2.AllTurns[0]) {
				t.Errorf("turn records differ between batch position %d (base %d) and position 0 (base %d)", idx, baseSeed, baseSeed+idx)
			}
			if !reflect.DeepEqual(r1.AllLeaders[idx], r2.AllLeaders[0]) {
				t.Errorf("leader tracks differ between batch position %d (base %d) and position 0 (base %d)", idx, baseSeed, baseSeed+idx)
			}
		})
	}
}

// BenchmarkRunBatchShedding200 is the Task 7 instrumentation hot-loop gate:
// a 200-game shedding batch, compared before/after per-turn recording.
func BenchmarkRunBatchShedding200(b *testing.B) {
	g := seeds.CrazyEights()
	runner := &shedding.Runner{}
	ai := &sim.RandomAI{}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sim.RunBatch(g, runner, ai, 200, 12345)
	}
}

// BenchmarkRunBatchRummy20 tracks the rummy probe cost (rummy OptionDelta
// probes are the expensive ones: meld generation runs per probe).
func BenchmarkRunBatchRummy20(b *testing.B) {
	g := seeds.GinRummy()
	runner := &rummy.Runner{}
	ai := &sim.RandomAI{}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sim.RunBatch(g, runner, ai, 20, 12345)
	}
}

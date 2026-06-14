package shedding

import (
	"math/rand/v2"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

// runPlayGenome is the hand-built fixture for the MechRunPlay deep borrow:
// a shedding host where you may dump same-rank sets / same-suit runs of 2+
// cards in one turn (climbing's combinations -> shedding).
func runPlayGenome() *genome.Genome {
	return &genome.Genome{
		ID:       "runplay-fixture",
		Skeleton: genome.Shedding,
		Players:  2,
		HandSize: 6,
		Shedding: &genome.SheddingParams{MatchRule: genome.MatchEither, DrawPenalty: 1},
		Borrowed: []genome.BorrowedMechanic{{Source: genome.Climbing, Mechanic: genome.MechRunPlay}},
	}
}

// plainGenome is runPlayGenome without the borrow -- the baseline whose move
// set MechRunPlay must be a SUPERSET of.
func plainGenome() *genome.Genome {
	g := runPlayGenome()
	g.Borrowed = nil
	return g
}

// TestRunPlayMovesNeverEmpty is the playability-floor property: the move-adding
// borrow keeps GenerateMoves non-empty from every reachable state (trivially
// true as a superset, but guarded since it is a deep move-level change).
func TestRunPlayMovesNeverEmpty(t *testing.T) {
	g := runPlayGenome()
	runner := &Runner{}
	for seed := uint64(0); seed < 200; seed++ {
		rng := rand.New(rand.NewPCG(seed, 0))
		state := runner.Setup(g, rng)
		maxTurns := g.MaxTurns()
		for turn := 0; turn < maxTurns; turn++ {
			runner.Upkeep(state, g)
			if runner.CheckEnd(state, g) >= 0 {
				break
			}
			moves := runner.GenerateMoves(state, g)
			if len(moves) == 0 {
				t.Fatalf("seed %d turn %d: RunPlay produced ZERO legal moves", seed, turn)
			}
			move := (&sim.RandomAI{}).SelectMove(moves, state, rng)
			runner.ApplyMove(state, move, g)
		}
	}
}

// TestRunPlayIsSupersetAndFires checks the borrow is (a) a strict superset of
// the plain move set in every reachable state, and (b) actually FIRES -- it
// must offer at least one multi-card combo somewhere, or it is a no-op borrow
// (dd-lnh forbids whitelisting those).
func TestRunPlayIsSupersetAndFires(t *testing.T) {
	combo := runPlayGenome()
	plain := plainGenome()
	runner := &Runner{}
	combosSeen := 0
	for seed := uint64(0); seed < 100; seed++ {
		rng := rand.New(rand.NewPCG(seed, 0))
		state := runner.Setup(combo, rng)
		maxTurns := combo.MaxTurns()
		for turn := 0; turn < maxTurns; turn++ {
			runner.Upkeep(state, combo)
			if runner.CheckEnd(state, combo) >= 0 {
				break
			}
			cm := runner.GenerateMoves(state, combo)
			pm := runner.GenerateMoves(state, plain)
			// superset: every plain move appears in the combo move set
			for _, p := range pm {
				if !containsMove(cm, p) {
					t.Fatalf("seed %d turn %d: combo move set missing a plain move %v", seed, turn, p)
				}
			}
			if len(cm) < len(pm) {
				t.Fatalf("seed %d turn %d: combo move set smaller than plain (%d < %d)", seed, turn, len(cm), len(pm))
			}
			for _, m := range cm {
				if m.Type == sim.MovePlay && len(m.Cards) >= 2 {
					combosSeen++
				}
			}
			move := (&sim.RandomAI{}).SelectMove(cm, state, rng)
			runner.ApplyMove(state, move, combo)
		}
	}
	if combosSeen == 0 {
		t.Fatal("MechRunPlay never offered a multi-card combo in 100 games -- it is a no-op borrow (dd-lnh)")
	}
	t.Logf("MechRunPlay offered %d multi-card combos across 100 games", combosSeen)
}

func containsMove(moves []sim.Move, want sim.Move) bool {
	for _, m := range moves {
		if m.Type != want.Type || len(m.Cards) != len(want.Cards) {
			continue
		}
		same := true
		for i := range m.Cards {
			if m.Cards[i] != want.Cards[i] {
				same = false
				break
			}
		}
		if same {
			return true
		}
	}
	return false
}

// TestRunPlayMultiCardApply checks a multi-card discard removes ALL its cards
// from hand, pushes them all to discard, and makes the LAST card the new top.
func TestRunPlayMultiCardApply(t *testing.T) {
	g := runPlayGenome()
	runner := &Runner{}
	state := sim.NewGameState(2)
	state.Active = 0
	state.Hands[0] = []sim.Card{
		{Suit: 1, Rank: 5}, {Suit: 2, Rank: 5}, {Suit: 3, Rank: 5}, // a set of three 5s
		{Suit: 1, Rank: 9},
	}
	state.Hands[1] = []sim.Card{{Suit: 4, Rank: 2}}
	state.TopCard = &sim.Card{Suit: 1, Rank: 7} // 5 of suit-1 matches by suit
	state.Discard = []sim.Card{*state.TopCard}

	combo := []sim.Card{{Suit: 1, Rank: 5}, {Suit: 2, Rank: 5}, {Suit: 3, Rank: 5}}
	runner.ApplyMove(state, sim.Move{Type: sim.MovePlay, Cards: combo, PlayerID: 0}, g)

	if len(state.Hands[0]) != 1 {
		t.Fatalf("after 3-card combo, hand should have 1 card, got %d", len(state.Hands[0]))
	}
	if state.TopCard.Rank != 5 || state.TopCard.Suit != 3 {
		t.Fatalf("new top should be the last combo card (5 of suit 3), got %+v", *state.TopCard)
	}
	// all three combo cards must be in the discard pile
	if len(state.Discard) != 4 { // original top + 3 combo cards
		t.Fatalf("discard should hold 4 cards, got %d", len(state.Discard))
	}
}

// TestRunPlayDeterminism guards that findComboPlays' output order (built from
// maps) is deterministic.
func TestRunPlayDeterminism(t *testing.T) {
	g := runPlayGenome()
	for seed := uint64(0); seed < 30; seed++ {
		r1 := runGame(g, seed)
		r2 := runGame(g, seed)
		if r1.Winner != r2.Winner || r1.Turns != r2.Turns {
			t.Fatalf("seed %d non-deterministic: (winner %d turns %d) vs (winner %d turns %d)",
				seed, r1.Winner, r1.Turns, r2.Winner, r2.Turns)
		}
	}
}

// TestRunPlayCompletes is the Gate-2 health check: as a pure move superset the
// borrow must complete at least as reliably as plain shedding (combos only
// empty hands faster), so it is not a degenerate timeout machine.
func TestRunPlayCompletes(t *testing.T) {
	g := runPlayGenome()
	completed := 0
	for seed := uint64(0); seed < 200; seed++ {
		if runGame(g, seed).Winner >= 0 {
			completed++
		}
	}
	t.Logf("RunPlay fixture: %d/200 games completed with a winner", completed)
	if completed < 160 {
		t.Fatalf("RunPlay fixture completes too rarely: %d/200 (<80%%)", completed)
	}
}

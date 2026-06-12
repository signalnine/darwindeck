package evolution

import (
	"reflect"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

// TestBehaviorBatchRunsHooks (reviewer finding 6): descriptor batches must
// simulate the HOOKED game -- the same game the fitness pipeline evaluates
// and humans playtest (mechanic.HooksFor single-construction-site
// discipline, audit Task 24). A rummy genome borrowing shedding's
// DrawPenalty draws extra cards on face-card plays; with hooks the game
// traces must diverge from a hook-less batch at the same seed.
func TestBehaviorBatchRunsHooks(t *testing.T) {
	g := seeds.KnockRummy()
	g.Borrowed = append(g.Borrowed, genome.BorrowedMechanic{
		Source:   genome.Shedding,
		Mechanic: genome.MechDrawPenalty,
	})

	hooked, ok := BehaviorBatch(g, 99)
	if !ok {
		t.Fatal("BehaviorBatch found no runner for a rummy genome")
	}
	bare := sim.RunBatch(g, fitness.GetRunner(g), &sim.RandomAI{}, behaviorBatchGames, 99)

	if reflect.DeepEqual(hooked.AllEvents, bare.AllEvents) {
		t.Error("BehaviorBatch traces identical to a hook-less batch: borrowed-mechanic hooks are not running, so the descriptor describes a different game than fitness evaluates")
	}
}

// TestBehaviorBatchNoRunner: a genome with an unknown skeleton reports ok ==
// false instead of panicking.
func TestBehaviorBatchNoRunner(t *testing.T) {
	g := &genome.Genome{ID: "no-skel", Skeleton: genome.SkeletonType(99), Players: 2}
	if _, ok := BehaviorBatch(g, 1); ok {
		t.Error("BehaviorBatch reported ok for a genome with no runner")
	}
}

// TestBehaviorBatchDeterministic: same genome + seed => identical batch
// (the descriptor sampling must stay reproducible).
func TestBehaviorBatchDeterministic(t *testing.T) {
	g := seeds.CrazyEights()
	a, okA := BehaviorBatch(g, 7)
	b, okB := BehaviorBatch(g, 7)
	if !okA || !okB {
		t.Fatal("BehaviorBatch found no runner for a shedding genome")
	}
	if !reflect.DeepEqual(a.AllEvents, b.AllEvents) {
		t.Error("BehaviorBatch is not deterministic for a fixed seed")
	}
}

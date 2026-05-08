package evolution

import (
	"fmt"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/seeds"
)

// TestComputeNoveltyConvergedClusterScoresZero verifies that when a
// population converges on a single behavior descriptor, every member of
// that cluster gets Novelty ~= 0 -- the cluster should be penalized for
// duplication, not rewarded. The buggy implementation filtered "if d > 0"
// inside the k-NN loop, which dropped intra-cluster zeros and pulled all
// k distances from faraway outliers. The fix skips the genome's own slot
// by identity, preserving zero distances from other identical individuals.
//
// We seed the cluster with NoveltyK+1 members so the k-NN mean over the
// nearest NoveltyK neighbors is exactly zero, matching dd-8z7 acceptance
// criteria.
func TestComputeNoveltyConvergedClusterScoresZero(t *testing.T) {
	cfg := Config{Workers: 1, BaseSeed: 1, PopulationSize: NoveltyK + 2}
	e := NewNoveltyEngine(cfg, allSeeds())

	clusterBehavior := BehaviorDescriptor{0.5, 0.5}
	outlierBehavior := BehaviorDescriptor{0.95, 0.05}

	g := seeds.CrazyEights()
	mk := func(b BehaviorDescriptor, id string) *NoveltyIndividual {
		clone := cloneGenome(g)
		clone.ID = id
		return &NoveltyIndividual{
			Individual: Individual{
				Genome:  clone,
				Valid:   true,
				Fitness: fitness.Metrics{TotalFitness: 0.9},
			},
			Behavior: b,
		}
	}

	pop := make([]*NoveltyIndividual, 0, NoveltyK+2)
	for i := 0; i < NoveltyK+1; i++ {
		pop = append(pop, mk(clusterBehavior, fmt.Sprintf("cluster_%d", i)))
	}
	pop = append(pop, mk(outlierBehavior, "outlier"))
	e.Population = pop

	e.computeNovelty()

	for i := 0; i < NoveltyK+1; i++ {
		if got := e.Population[i].Novelty; got != 0 {
			t.Fatalf("converged cluster member %d got Novelty=%v; want 0 (k-NN must keep zero distances from other individuals and skip self by identity, not by zero-distance filter)",
				i, got)
		}
	}
	// Sanity: the outlier still sees the cluster's behavior as far away, so
	// its novelty must be strictly positive. This guards against an over-
	// aggressive fix that would also zero-out the outlier.
	outlier := e.Population[NoveltyK+1]
	if outlier.Novelty <= 0 {
		t.Fatalf("outlier got Novelty=%v; want >0 (faraway behavior should still register novelty)", outlier.Novelty)
	}
}

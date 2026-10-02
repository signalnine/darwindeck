package evolution

import (
	"fmt"
	"strings"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
)

// A seeded run must not depend on the worker count. Every engine evaluates
// its population on a goroutine pool; the invariant that makes that safe is
// that each evaluation's seed derives from (BaseSeed, generation, slot) and
// each goroutine writes only its own slot, while all RNG-driven selection and
// mutation stays on the calling goroutine. These tests pin it end to end by
// running each engine with 1 worker and with 8 and comparing every piece of
// state selection or output reads. (Verified at the CLI as well: byte-identical
// genome output for -workers 1 vs 16 on all three algorithms.)

// workerDeterminismConfig is small enough for a multi-second budget yet
// exercises both evaluation pools (the greedy pass and the MCTS decile pass,
// at reduced search strength -- production strength costs ~15s per genome).
func workerDeterminismConfig(workers int) Config {
	return Config{
		PopulationSize: 8,
		Generations:    2,
		EliteSize:      2,
		TournamentSize: 3,
		Workers:        workers,
		BaseSeed:       7,
		CrossSkeleton:  true,
		NoveltySelect:  true,
		MCTSDecile:     0.25,
		MCTSEval:       fitness.MCTSEvalConfig{Iterations: 10, Determinizations: 2},
	}
}

func individualLine(sb *strings.Builder, tag string, ind *Individual) {
	fmt.Fprintf(sb, "%s %s %s valid=%v evals=%d sum=%v mcts=%d/%v fit=%v shared=%v\n",
		tag, ind.Genome.ID, genomeHash(ind.Genome), ind.Valid, ind.EvalCount, ind.FitnessSum,
		ind.MctsCount, ind.MctsSum, ind.Fitness.TotalFitness, ind.Fitness.SharedFitness)
}

func TestBaselineDeterministicAcrossWorkerCounts(t *testing.T) {
	run := func(workers int) string {
		e := NewEngine(workerDeterminismConfig(workers), seeds.All())
		e.Run(nil)
		var sb strings.Builder
		for i, ind := range e.Population {
			individualLine(&sb, fmt.Sprintf("pop[%d]", i), ind)
		}
		for i, ind := range e.TopN(5) {
			fmt.Fprintf(&sb, "top[%d] %s\n", i, ind.Genome.ID)
		}
		fmt.Fprintf(&sb, "best=%v\n", e.BestFitness)
		return sb.String()
	}
	if one, many := run(1), run(8); one != many {
		t.Errorf("baseline engine depends on the worker count\n--- workers=1 ---\n%s--- workers=8 ---\n%s", one, many)
	}
}

func TestHybridDeterministicAcrossWorkerCounts(t *testing.T) {
	run := func(workers int) string {
		e := NewNoveltyEngine(workerDeterminismConfig(workers), seeds.All())
		e.Run(nil)
		var sb strings.Builder
		sb.WriteString(engineSnapshot(e))
		inds, behaviors := e.AllQualified()
		for i, ind := range inds {
			fmt.Fprintf(&sb, "qualified[%d] %s %v\n", i, ind.Genome.ID, behaviors[i])
		}
		return sb.String()
	}
	if one, many := run(1), run(8); one != many {
		t.Errorf("hybrid engine depends on the worker count\n--- workers=1 ---\n%s--- workers=8 ---\n%s", one, many)
	}
}

func TestMAPElitesDeterministicAcrossWorkerCounts(t *testing.T) {
	run := func(workers int) string {
		e := NewMAPElitesEngine(workerDeterminismConfig(workers), seeds.All())
		e.Run(nil)
		var sb strings.Builder
		for _, skel := range genome.AllSkeletons() {
			archive := e.Archives[skel]
			for r := 0; r < GridSize; r++ {
				for c := 0; c < GridSize; c++ {
					if cell := archive.Cells[r][c]; cell != nil {
						individualLine(&sb, fmt.Sprintf("%s[%d][%d] beh=%v", skel, r, c, cell.Behavior), cell.Individual)
					}
				}
			}
			fmt.Fprintf(&sb, "%s occupied=%d qd=%v\n", skel, archive.Occupied, archive.QDScore)
		}
		fmt.Fprintf(&sb, "best=%v challenges=%d\n", e.BestFitness, e.challenges)
		return sb.String()
	}
	if one, many := run(1), run(8); one != many {
		t.Errorf("MAP-Elites engine depends on the worker count\n--- workers=1 ---\n%s--- workers=8 ---\n%s", one, many)
	}
}

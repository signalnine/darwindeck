package evolution

import (
	"math"

	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/mechanic"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

// behaviorBatchGames is the sample size for descriptor batches.
const behaviorBatchGames = 50

// BehaviorBatch runs the canonical descriptor batch for g: 50 random-AI
// games WITH the genome's borrowed-mechanic hooks. Hooks are mandatory
// (reviewer finding 6): the descriptor must describe the HOOKED game -- the
// same game the fitness pipeline evaluates and humans playtest --
// and mechanic.HooksFor is the single hook-construction site (audit Task
// 24). This is the only place QD code may build a behavior batch; the
// novelty engine, MAP-Elites, and the experiment harness all call it, so a
// hook-construction change lands in every descriptor at once. ok is false
// when g has no runner.
func BehaviorBatch(g *genome.Genome, seed uint64) (result sim.BatchResult, ok bool) {
	runner := fitness.GetRunner(g)
	if runner == nil {
		return sim.BatchResult{}, false
	}
	return sim.RunBatch(g, runner, &sim.RandomAI{}, behaviorBatchGames, seed, mechanic.HooksFor(g)...), true
}

// BehaviorDescriptor is a 2D point in behavior space.
// X = decision density, Y = interaction.
type BehaviorDescriptor [2]float64

// ComputeBehavior extracts the behavior descriptor from simulation results.
// X-axis: decision density (fraction of turns where the acting player had
// >= 2 legal moves). Y-axis: interaction (fraction of turns that perturbed
// the next player's options or carried a direct attack).
//
// These replace the original axes (normalized AvgTurns x win entropy): win
// entropy under random play is ~1.0 for almost any non-broken game, so 91%
// of mutants landed in the top grid row and only 16/100 MAP-Elites cells
// were reachable (audit Task 17). Decision density and interaction are real
// per-genome variables with measured within-skeleton spread (post Tasks 9/11),
// enforced by TestDescriptorSpread.
//
// Both axes are computed by the canonical pkg/fitness implementations via
// the exported ComputeFitness -- a single source of truth, so the descriptor
// can never drift from the fitness metrics. The empty greedy batch and zero
// player count zero out the metrics this descriptor does not read (skill
// gradient, session length); decision density and interaction depend only on
// the random batch's TurnRecords.
func ComputeBehavior(result sim.BatchResult) BehaviorDescriptor {
	m := fitness.ComputeFitness(result, sim.BatchResult{}, 0)
	return BehaviorDescriptor{m.MeaningfulDecisions, m.Interaction}
}

// GridCell returns the (row, col) cell indices for a given grid size.
func (b BehaviorDescriptor) GridCell(gridSize int) (int, int) {
	col := int(b[0] * float64(gridSize))
	row := int(b[1] * float64(gridSize))
	if col >= gridSize {
		col = gridSize - 1
	}
	if row >= gridSize {
		row = gridSize - 1
	}
	if col < 0 {
		col = 0
	}
	if row < 0 {
		row = 0
	}
	return row, col
}

// Distance returns Euclidean distance between two behavior descriptors.
func (b BehaviorDescriptor) Distance(other BehaviorDescriptor) float64 {
	dx := b[0] - other[0]
	dy := b[1] - other[1]
	return math.Sqrt(dx*dx + dy*dy)
}

// CounterfactualIntegration measures how much a genome's BORROWED mechanics
// actually change its play, vs the same genome with the borrows removed -- a
// paired (common-random-numbers) counterfactual. This is a MECHANIC-AWARE
// novelty signal, unlike the 2-D behavior descriptor (a blind playstyle
// shadow): a borrow that genuinely fuses families (changes the legal-move set
// AND/OR the win condition) moves play a lot when removed; an inert or bolt-on
// borrow moves it ~nothing. Returns 0 for a borrowless genome (no
// counterfactual) and in [0,1] otherwise, the mean of three divergences
// between the HOOKED and NEUTERED batches:
//   - win-distribution total-variation (does the borrow change WHO wins?)
//   - game-length shift (does it change tempo/length?)
//   - option-flow shift (mean legal-move count -- does it change the MOVE set?
//     a move-adding borrow like MechRunPlay raises this; a pure end-of-round
//     scoring tally leaves it ~0, which is exactly the discrimination we want)
//
// `hooked` is the genome's own already-run descriptor batch (so the hooked arm
// is not re-run); `seed` MUST be the seed used for `hooked` (paired CRN). The
// neutered arm is the genome with Borrowed=nil at the same params -- because
// the rounds machinery and ComboPlay are gated on the borrow, clearing Borrowed
// removes both the move-change and the win-condition-change while holding hand
// size etc. fixed, isolating the borrow's contribution. Guard: if either arm
// cannot complete a game, the borrow may be load-bearing for non-brokenness, so
// return 0 (degeneracy is the vetoes' job, not a novelty reward).
func CounterfactualIntegration(g *genome.Genome, hooked sim.BatchResult, seed uint64) float64 {
	if len(g.Borrowed) == 0 {
		return 0
	}
	runner := fitness.GetRunner(g)
	if runner == nil {
		return 0
	}
	neut := g.Clone()
	neut.Borrowed = nil
	neutered := sim.RunBatch(neut, runner, &sim.RandomAI{}, behaviorBatchGames, seed, mechanic.HooksFor(neut)...)
	if hooked.Completions == 0 || neutered.Completions == 0 {
		return 0
	}
	cid := (winDistTV(hooked, neutered) + lengthShift(hooked, neutered) + optionFlowShift(hooked, neutered)) / 3.0
	if cid > 1 {
		cid = 1
	}
	return cid
}

// winDistTV is the total-variation distance between the two batches' normalized
// win distributions (in [0,1]).
func winDistTV(a, b sim.BatchResult) float64 {
	n := len(a.WinCounts)
	if len(b.WinCounts) < n {
		n = len(b.WinCounts)
	}
	ta, tb := 0, 0
	for i := 0; i < n; i++ {
		ta += a.WinCounts[i]
		tb += b.WinCounts[i]
	}
	if ta == 0 || tb == 0 {
		return 0
	}
	tv := 0.0
	for i := 0; i < n; i++ {
		tv += math.Abs(float64(a.WinCounts[i])/float64(ta) - float64(b.WinCounts[i])/float64(tb))
	}
	return tv / 2
}

// lengthShift is the normalized absolute change in average game length ([0,1]).
func lengthShift(a, b sim.BatchResult) float64 {
	m := math.Max(a.AvgTurns, b.AvgTurns)
	if m < 1 {
		return 0
	}
	d := math.Abs(a.AvgTurns-b.AvgTurns) / m
	if d > 1 {
		d = 1
	}
	return d
}

// optionFlowShift is the normalized change in the mean per-turn legal-move
// count ([0,1]) -- the cheap proxy for "the borrow changes the move set".
func optionFlowShift(a, b sim.BatchResult) float64 {
	la, lb := meanLegalMoves(a), meanLegalMoves(b)
	m := math.Max(la, lb)
	if m < 1 {
		return 0
	}
	d := math.Abs(la-lb) / m
	if d > 1 {
		d = 1
	}
	return d
}

func meanLegalMoves(r sim.BatchResult) float64 {
	sum, cnt := 0.0, 0
	for _, game := range r.AllTurns {
		for _, t := range game {
			sum += float64(t.LegalMoves)
			cnt++
		}
	}
	if cnt == 0 {
		return 0
	}
	return sum / float64(cnt)
}

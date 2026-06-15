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
// actually change its play -- a paired (common-random-numbers) counterfactual,
// the MECHANIC-AWARE novelty signal the 2-D behavior descriptor (a blind
// playstyle shadow) lacks. Returns 0 for a borrowless genome and in [0,1]
// otherwise.
//
// LEAVE-ONE-OUT (max single-borrow marginal): for each borrow it removes ONLY
// that borrow (keeping the rest), measures how much play diverges from the full
// hooked genome, and returns the MAX over borrows. This is the marginal
// contribution of the single most integrated borrow, which is what we want to
// reward: a genome with ONE genuinely deep borrow (e.g. MechRunPlay changing
// the move set) scores high, while a PILE-ON of shallow scoring tallies scores
// low (each tally's marginal is small, and the max ignores the rest) -- the
// remove-ALL-borrows version conflated those two by summing divergences. It
// also resists rewarding an inert borrow carried alongside a live one (the
// inert borrow's marginal is ~0). A judge blind-test caught exactly that
// failure (a dead trick_scoring borrow inflating a genome's apparent depth).
//
// Each marginal is the mean of three divergences between the hooked and the
// borrow-removed batch:
//   - win-distribution total-variation (does the borrow change WHO wins?)
//   - game-length shift (does it change tempo/length?)
//   - option-flow shift (mean legal-move count -- does it change the MOVE set?
//     a move-adding borrow like MechRunPlay raises this; a pure end-of-round
//     scoring tally leaves it ~0, the discrimination we want)
//
// `hooked` is the genome's own already-run descriptor batch (the hooked arm is
// not re-run); `seed` MUST be the seed used for `hooked` (paired CRN). A
// borrow whose removal BREAKS the game (the variant can't complete) is skipped,
// not counted -- being load-bearing for non-brokenness is the vetoes' concern,
// not a novelty reward.
func CounterfactualIntegration(g *genome.Genome, hooked sim.BatchResult, seed uint64) float64 {
	if len(g.Borrowed) == 0 {
		return 0
	}
	runner := fitness.GetRunner(g)
	if runner == nil || hooked.Completions == 0 {
		return 0
	}
	best := 0.0
	for i := range g.Borrowed {
		variant := g.Clone()
		// drop only borrow i, keep the rest
		kept := make([]genome.BorrowedMechanic, 0, len(g.Borrowed)-1)
		kept = append(kept, g.Borrowed[:i]...)
		kept = append(kept, g.Borrowed[i+1:]...)
		variant.Borrowed = kept
		res := sim.RunBatch(variant, runner, &sim.RandomAI{}, behaviorBatchGames, seed, mechanic.HooksFor(variant)...)
		if res.Completions == 0 {
			continue // removing this borrow breaks the game -> don't credit it
		}
		marginal := (winDistTV(hooked, res) + lengthShift(hooked, res) + optionFlowShift(hooked, res)) / 3.0
		if marginal > best {
			best = marginal
		}
	}
	if best > 1 {
		best = 1
	}
	return best
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

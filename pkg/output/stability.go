package output

import (
	"fmt"

	"github.com/darwindeck/darwindeck/pkg/evolution"
	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/genome"
)

// Wave M -- veto-stable publication (audit Task 28/29 follow-up).
//
// THE BUG this closes (motivating example, r4 rank02): production publishes
// each top-N genome from a SINGLE evaluation. A genome that fails its own
// degeneracy veto (or a Tier-1 kill) on a MINORITY of seeds is fitness-0 on
// those seeds but can still ride a lucky published eval into the top-N. The
// r4 flagship's rank02 shedding genome fails greedy_longest_run on 1/10 fresh
// seeds, yet published as rank 2. A reviewer who runs it on the failing seed
// sees a degenerate game where the published report claims a healthy one.
//
// THE FIX (output path only, in the spirit of Wave K): before SaveResults
// writes the top-N, re-evaluate each genome K times at distinct fresh seeds.
// A genome is VETO-STABLE iff a majority of those re-evals stay valid (pass
// Tier 0/1 and every degeneracy veto). Unstable genomes are DEMOTED below all
// stable genomes in the published order, and every published genome.json /
// report carries veto_stable + stable_evals. This changes ONLY what is
// published and in what order; selection, evolution dynamics, and the frozen
// metric stack are untouched.

const (
	// stabilityEvals is K -- the number of fresh-seed re-evaluations per
	// top-N genome. 30 top-N * 5 = 150 evaluations, trivial against a run
	// (millions of games); measured in the restamp tooling.
	stabilityEvals = 5

	// stabilityMajority is the validity floor for the veto_stable flag. A
	// genome valid on >= 3/5 re-evals is published as stable; below that it is
	// demoted. Majority (not unanimity) is chosen deliberately: per-eval
	// validity carries genuine seed noise (one unlucky random-AI deal can trip
	// greedy_timeout on an otherwise-healthy game, the very false-reject the
	// Tier-1 tolerance band exists to absorb), so a single failing re-eval
	// flags-but-does-not-condemn. A genome that fails the MAJORITY of fresh
	// seeds is degenerate-in-expectation, not unlucky. r4 rank02 (fails 1/10)
	// would publish as stable-with-a-flag under this rule; a fixture that
	// fails most seeds (e.g. wild-union, vetoed 10/10) is demoted.
	stabilityMajority = (stabilityEvals / 2) + 1

	// stabilitySeedBase keeps the K re-eval seeds clear of every seed the run
	// itself used. The run derives per-genome seeds as BaseSeed + gen*10000 +
	// idx (and the engine's elite re-evals walk forward from there), so a base
	// far above any plausible Generations*10000 guarantees fresh draws. The
	// per-genome offset below makes the check deterministic under fixed inputs.
	stabilitySeedBase = uint64(1_000_000_000)
)

// StabilityResult is the outcome of the fresh-seed re-evaluation of one
// genome.
type StabilityResult struct {
	ValidCount int      // re-evals that passed Tier 0/1 and every veto
	Total      int      // K
	Stable     bool     // ValidCount >= stabilityMajority
	Reasons    []string // per-failing-eval reason (Tier-1 reason or veto name)
}

// Label renders the published "N/K" string.
func (s StabilityResult) Label() string {
	return fmt.Sprintf("%d/%d", s.ValidCount, s.Total)
}

// EvaluateStability re-evaluates g at stabilityEvals distinct fresh seeds
// derived deterministically from baseSeed and the genome's identity, and
// reports how many re-evals stayed valid. It uses the DEFAULT (greedy-only)
// pipeline -- fitness.Evaluate -- because that is the validity gate every
// genome faces in production (Tier 0/1 + the degeneracy vetoes); the MCTS tier
// is a skill measurement, not a validity gate, and is irrelevant to whether a
// genome is degenerate. Deterministic under fixed (g, baseSeed).
func EvaluateStability(g *genome.Genome, baseSeed uint64) StabilityResult {
	res := StabilityResult{Total: stabilityEvals}
	// Per-genome offset keeps two genomes in the same top-N on disjoint seed
	// streams (so they cannot share a coincidentally-lucky or -unlucky draw)
	// while staying reproducible. genomeStabilityOffset is a stable hash of the
	// published genome content.
	off := genomeStabilityOffset(g)
	for k := 0; k < stabilityEvals; k++ {
		// baseSeed is mixed in (golden-ratio multiplier to spread nearby run
		// seeds far apart) so the doc contract "deterministic under fixed
		// (g, baseSeed)" is real: the parameter was previously unused, which
		// made stability identical across runs regardless of -seed AND let a
		// run seeded near stabilitySeedBase overlap the stability stream.
		seed := stabilitySeedBase + baseSeed*0x9e3779b97f4a7c15 + off + uint64(k)*7919 // 7919: a prime stride
		ev := fitness.Evaluate(g, seed)
		switch {
		case ev.Valid:
			res.ValidCount++
		case len(ev.Tier0Errors) > 0:
			res.Reasons = append(res.Reasons, "tier0:"+ev.Tier0Errors[0])
		case ev.DegenerateReason != "":
			res.Reasons = append(res.Reasons, "veto:"+ev.DegenerateReason)
		case !ev.Tier1.Passed:
			res.Reasons = append(res.Reasons, "tier1:"+ev.Tier1.Reason)
		default:
			res.Reasons = append(res.Reasons, "invalid")
		}
	}
	res.Stable = res.ValidCount >= stabilityMajority
	return res
}

// reRankByStability re-evaluates every individual in `top` and returns the
// publication order: all veto-stable genomes first (in their original
// leaderboard order), then all unstable genomes (also in their original
// order, preserved as demoted evidence). The returned StabilityResult slice is
// aligned index-for-index with the returned individuals. A STABLE partition
// is the rank-1-eligible set; an unstable genome can never outrank a stable
// one regardless of fitness. This is a pure reordering -- nothing is dropped,
// so the published bundle still documents every reviewed game (the unstable
// ones flagged 1/5, 2/5, ...). Order within each class is the caller's input
// order, which is the greedy-only leaderboard (Wave K OutputRank).
func reRankByStability(top []*evolution.Individual, baseSeed uint64) ([]*evolution.Individual, []StabilityResult) {
	type entry struct {
		ind *evolution.Individual
		st  StabilityResult
	}
	stable := make([]entry, 0, len(top))
	unstable := make([]entry, 0)
	for _, ind := range top {
		st := EvaluateStability(ind.Genome, baseSeed)
		e := entry{ind: ind, st: st}
		if st.Stable {
			stable = append(stable, e)
		} else {
			unstable = append(unstable, e)
		}
	}
	ordered := append(stable, unstable...)
	inds := make([]*evolution.Individual, len(ordered))
	sts := make([]StabilityResult, len(ordered))
	for i, e := range ordered {
		inds[i] = e.ind
		sts[i] = e.st
	}
	return inds, sts
}

// genomeStabilityOffset is a small stable hash of the genome's identity, used
// only to give each genome its own re-eval seed stream. It need not be
// cryptographic; it must be deterministic for a fixed genome. The genome ID is
// generation+counter (unique within a run) so it suffices.
func genomeStabilityOffset(g *genome.Genome) uint64 {
	var h uint64 = 1469598103934665603 // FNV-1a offset basis
	for _, c := range g.ID {
		h ^= uint64(c)
		h *= 1099511628211
	}
	// Spread across a modest range so streams stay below the next decade of
	// the seed space and never approach the run's BaseSeed+gen*10000 region.
	return h % 1_000_000
}

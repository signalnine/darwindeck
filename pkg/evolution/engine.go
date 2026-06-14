package evolution

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"runtime"
	"sort"
	"sync"

	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/genome"
)

// Config controls the evolution run.
type Config struct {
	PopulationSize int
	Generations    int
	EliteSize      int // Top N carried forward unchanged
	TournamentSize int
	Workers        int // 0 = auto-detect
	BaseSeed       uint64
	SaveTopN       int
	OutputDir      string

	// MCTSDecile enables MCTS-for-top-decile mode (audit Task 20b, the
	// plan's production fallback after Task 19's 2s/genome MCTS budget
	// FAILED by ~7x): after each generation's greedy evaluation pass, the
	// top MCTSDecile fraction of valid individuals -- ranked by the
	// greedy-only running mean -- each receive one fitness.EvaluateWithMCTS
	// at a distinct seed offset, accumulated into the separate
	// MctsSum/MctsCount running mean. 0 disables (zero-value Config =
	// greedy-only, the pre-Task-20b pipeline); the production default is
	// 0.10 (DefaultConfig, the evolve command's -mcts-decile flag). Applies
	// to Engine and NoveltyEngine; MAP-Elites instead re-evaluates
	// incumbents on cell challenge (see mapelites.go).
	MCTSDecile float64
	// MCTSEval tunes the decile pass's search strength. The zero value is
	// production strength (200 iterations / 10 determinizations); tests
	// dial it down.
	MCTSEval fitness.MCTSEvalConfig

	// CrossSkeleton enables cross-skeleton recombination (novelty evolution).
	// Default OFF (zero value): crossing two DIFFERENT-skeleton parents returns
	// nil and the caller falls back to mutation, exactly the pre-novelty
	// behavior v2 shipped with (cross-breeding was hard-disabled because the
	// hybrids it produced were unplayable). When ON, such a cross produces a
	// HYBRID child instead -- one parent's skeleton + core params PLUS an
	// active cross-family borrowed mechanic from the other parent's family,
	// made outcome-affecting and repaired to validity (hybridCrossover). The
	// degeneracy vetoes + calibration gate + LLM judge are the safety net that
	// makes re-enabling this safe. Same-skeleton crossover is unaffected by
	// this flag. Threaded into Engine and NoveltyEngine via the engine's
	// crossover dispatch; MAP-Elites only crosses same-skeleton archive
	// occupants, so the flag is inert there.
	CrossSkeleton bool
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		PopulationSize: 500,
		Generations:    100,
		EliteSize:      10,
		TournamentSize: 5,
		Workers:        0,
		BaseSeed:       42,
		SaveTopN:       20,
		OutputDir:      "output",
		MCTSDecile:     0.10,
	}
}

// Individual wraps a genome with its fitness evaluation.
//
// EvalCount/FitnessSum implement the running-mean fitness scheme (Task 13.3
// of the 2026-06-11 audit remediation): elites are re-evaluated every
// generation with a fresh seed, and Fitness.TotalFitness holds the mean of
// all evaluations of this exact genome. This kills the winner's curse, where
// a single lucky evaluation was carried forward as `Valid: true` forever.
// Both fields reset to zero whenever the genome changes (mutation,
// crossover, dedup replacement) -- prior evaluations describe a different
// game.
type Individual struct {
	Genome  *genome.Genome
	Fitness fitness.Metrics
	Valid   bool

	// EvalCount is the number of valid evaluations of this exact genome.
	EvalCount int
	// FitnessSum is the sum of raw TotalFitness over those evaluations.
	FitnessSum float64

	// MctsSum/MctsCount are the SECOND accumulator of the two-accumulator
	// design (audit Task 20b): a running mean over full two-tier
	// (fitness.EvaluateWithMCTS) evaluations, granted once per generation to
	// individuals in the top Config.MCTSDecile of the greedy-only ranking.
	// MCTS-mode samples NEVER mix into FitnessSum/EvalCount: that greedy-only
	// mean stays pure as the decile ranking key (mixing modes would corrupt
	// the ranking that decides who gets MCTS -- the reviewer's mode-mixing
	// warning). Published fitness is MctsSum/MctsCount once MctsCount > 0
	// (see publishedFitness). Both fields reset wherever EvalCount resets:
	// mutation/crossover offspring, dedup replacement, invalid re-evaluation.
	MctsSum   float64
	MctsCount int
}

// greedyMean returns the greedy-only running mean -- the decile ranking key
// and the published fitness while no MCTS evaluations exist. 0 before any
// valid evaluation.
func (ind *Individual) greedyMean() float64 {
	if ind.EvalCount == 0 {
		return 0
	}
	return ind.FitnessSum / float64(ind.EvalCount)
}

// publishedFitness is the selection/output fitness: the MCTS running mean
// once any two-tier evaluations exist, else the greedy-only running mean.
// SharedFitness, novelty, BestFitness, and TopN all flow from this value
// exactly as they previously flowed from the greedy mean.
func (ind *Individual) publishedFitness() float64 {
	if ind.MctsCount > 0 {
		return ind.MctsSum / float64(ind.MctsCount)
	}
	return ind.greedyMean()
}

// GreedyMean exposes the greedy-only running mean to publication code
// (pkg/output): report.md prints it next to the published fitness so the
// MCTS-mode uplift is explicit instead of silent (round 3 commit 5c).
func (ind *Individual) GreedyMean() float64 { return ind.greedyMean() }

// MCTSMean returns the MCTS-mode running mean and whether any two-tier
// evaluations exist. When ok is false the published fitness IS the greedy
// mean and reports must not claim an MCTS-mode number.
func (ind *Individual) MCTSMean() (mean float64, ok bool) {
	if ind.MctsCount == 0 {
		return 0, false
	}
	return ind.MctsSum / float64(ind.MctsCount), true
}

// OutputRank is the LEADERBOARD ranking key (Wave K fix 1): the greedy-only
// running mean, the one number every individual has, measured the same way.
// The round-3 review caught the top-N sort mixing MCTS-mode published means
// (decile-granted genomes only, +0.085..+0.145 uplift) with greedy-only
// means in one ranking -- the MCTS-grant boundary WAS the top-10 boundary,
// and the headline numbers rested on as few as one MCTS eval. All published
// ordering (TopN, sortAndTrim, AllQualified clone-group keep) and the
// report/summary headline numbers flow from this; the MCTS-mode mean is
// reported alongside, never ranked. Selection inside the run still uses
// publishedFitness -- that is search policy, not publication.
//
// Falls back to the published TotalFitness when no running-mean history
// exists (EvalCount == 0): production individuals always carry EvalCount >=
// 1 while Valid, so the fallback only serves hand-built fixtures and legacy
// snapshots.
func (ind *Individual) OutputRank() float64 {
	if ind.EvalCount > 0 {
		return ind.greedyMean()
	}
	return ind.Fitness.TotalFitness
}

// Engine runs the evolutionary algorithm.
type Engine struct {
	Config     Config
	Seeds      []*genome.Genome
	Population []*Individual
	Generation int
	rng        *rand.Rand

	// Stats
	BestFitness float64
	BestGenome  *genome.Genome
}

// NewEngine creates an evolution engine with the given config and seed genomes.
func NewEngine(config Config, seeds []*genome.Genome) *Engine {
	if config.Workers == 0 {
		config.Workers = runtime.NumCPU()
	}

	return &Engine{
		Config: config,
		Seeds:  seeds,
		rng:    rand.New(rand.NewPCG(config.BaseSeed, 0)),
	}
}

// Initialize creates the initial population from seed genomes.
func (e *Engine) Initialize() {
	e.Population = make([]*Individual, e.Config.PopulationSize)

	for i := 0; i < e.Config.PopulationSize; i++ {
		// Pick a random seed and mutate it
		seed := e.Seeds[e.rng.IntN(len(e.Seeds))]
		g := e.mutate(seed)
		g.ID = fmt.Sprintf("init_%d", i)
		g.Generation = 0
		e.Population[i] = &Individual{Genome: g}
	}
}

// EvaluatePopulation runs fitness evaluation on all individuals in parallel,
// then applies fitness sharing to reward niche diversity.
//
// Every individual is evaluated every generation -- including elites, which
// previously skipped re-evaluation via `Valid: true` and so carried a single
// (possibly lucky) estimate forever. The per-generation seed term in the
// derivation below guarantees each re-evaluation samples fresh games, and
// TotalFitness becomes the running mean over all evaluations of the genome.
func (e *Engine) EvaluatePopulation() {
	var wg sync.WaitGroup
	sem := make(chan struct{}, e.Config.Workers)

	for i, ind := range e.Population {
		wg.Add(1)
		sem <- struct{}{} // Acquire worker slot

		go func(idx int, ind *Individual) {
			defer wg.Done()
			defer func() { <-sem }() // Release worker slot

			seed := e.Config.BaseSeed + uint64(e.Generation)*10000 + uint64(idx)
			result := fitness.Evaluate(ind.Genome, seed)

			ind.Valid = result.Valid
			ind.Fitness = result.Metrics
			if !result.Valid {
				// A genome that fails Tier 0/1 on re-evaluation is flaky
				// (e.g. degenerate under some seeds). Drop its history so
				// it must re-qualify from scratch if it ever passes again.
				// Both accumulators go: the MCTS mean describes the same
				// now-discredited genome.
				ind.EvalCount = 0
				ind.FitnessSum = 0
				ind.MctsSum = 0
				ind.MctsCount = 0
				return
			}

			ind.FitnessSum += result.Metrics.TotalFitness
			ind.EvalCount++
			ind.Fitness.TotalFitness = ind.publishedFitness()
			ind.Genome.Fitness = ind.Fitness.TotalFitness
		}(i, ind)
	}

	wg.Wait()

	// MCTS-for-top-decile (audit Task 20b): grant one two-tier evaluation to
	// the top of the greedy-only ranking, BEFORE sharing so SharedFitness
	// flows from the published (possibly MCTS-mean) fitness.
	e.runMCTSTopDecile()

	// Apply fitness sharing: divide fitness by niche count.
	// Niches are defined by skeleton type. This prevents a single skeleton
	// from monopolizing the population.
	e.applyFitnessSharing()
}

// runMCTSTopDecile applies the shared decile pass to this engine's
// population.
func (e *Engine) runMCTSTopDecile() {
	cands := make([]mctsCandidate, 0, len(e.Population))
	for i, ind := range e.Population {
		if ind != nil && ind.Valid {
			cands = append(cands, mctsCandidate{ind: ind, idx: i})
		}
	}
	evaluateTopDecileMCTS(e.Config, e.Generation, cands)
}

// mctsSeedOffset places the decile pass's evaluation seeds in a band
// disjoint from the greedy pass's. The greedy pass evaluates individual idx
// at BaseSeed + gen*10000 + idx, whose internal batches derive game seeds at
// +0..9 (Tier 1), +100..299 (random), and +1000..1199 (greedy); the decile
// pass adds +2000..2019 (MCTS batch) on top of its own base. With this
// offset the two passes' derived seed bands cannot overlap for populations
// up to ~2900, so the MCTS running mean never re-samples the exact games the
// greedy running mean already consumed. The novelty engine's behavior batch
// (seed+5000, 50 games) shares part of this band by construction; that is
// harmless -- behavior is a descriptor, not a fitness sample, so no
// statistical coupling enters either running mean.
const mctsSeedOffset = 5000

// mctsCandidate pairs a valid individual with its population index, which
// the decile pass needs for seed derivation.
type mctsCandidate struct {
	ind *Individual
	idx int
}

// evaluateTopDecileMCTS implements MCTS-for-top-decile (audit Task 20b),
// shared by Engine and NoveltyEngine: rank the valid individuals by
// greedy-only running mean, give the top ceil(decile*N) one
// fitness.EvaluateWithMCTS each at a distinct seed, and accumulate the
// result into the separate MCTS running mean. The greedy accumulator is
// never touched (mode purity); published fitness flips to the MCTS mean via
// publishedFitness. cfg.MCTSDecile <= 0 disables the pass entirely.
//
// HAZARD (pinned by TestMCTSTierRewardsDegenKnockTiming, pkg/fitness): the
// MCTS skill term cannot tell depth-in-a-rich-game from greedy-incompetence-
// in-a-trivial-one -- it fires hard on the instant-knock degenerate. The
// decile gate is the mitigation: only genomes already elite on greedy-only
// rank ever receive the term. This function is where that gate is ENFORCED
// in code rather than by convention.
func evaluateTopDecileMCTS(cfg Config, gen int, cands []mctsCandidate) {
	if cfg.MCTSDecile <= 0 || len(cands) == 0 {
		return
	}

	// Stable sort: ties keep population order, so the granted set is
	// deterministic for a fixed seed.
	sort.SliceStable(cands, func(a, b int) bool {
		return cands[a].ind.greedyMean() > cands[b].ind.greedyMean()
	})

	n := int(math.Ceil(cfg.MCTSDecile * float64(len(cands))))
	if n < 1 {
		n = 1
	}
	if n > len(cands) {
		n = len(cands)
	}

	workers := cfg.Workers
	if workers < 1 {
		workers = 1
	}
	var wg sync.WaitGroup
	sem := make(chan struct{}, workers)
	for _, c := range cands[:n] {
		wg.Add(1)
		sem <- struct{}{}
		go func(c mctsCandidate) {
			defer wg.Done()
			defer func() { <-sem }()

			seed := cfg.BaseSeed + uint64(gen)*10000 + uint64(c.idx) + mctsSeedOffset
			result := fitness.EvaluateWithMCTS(c.ind.Genome, seed, cfg.MCTSEval)
			if !result.Valid {
				// Flaky at this seed: skip the sample. Validity resets are
				// owned by the main evaluation pass, which already passed
				// this individual this generation.
				return
			}

			c.ind.MctsSum += result.Metrics.TotalFitness
			c.ind.MctsCount++
			c.ind.Fitness.TotalFitness = c.ind.publishedFitness()
			c.ind.Genome.Fitness = c.ind.Fitness.TotalFitness
		}(c)
	}
	wg.Wait()
}

// applyFitnessSharing divides each genome's fitness by the count of valid
// genomes sharing the same skeleton type (niche). Uses square root of niche
// count for softer pressure than strict division.
func (e *Engine) applyFitnessSharing() {
	// Count valid individuals per skeleton
	nicheCounts := make(map[genome.SkeletonType]int)
	for _, ind := range e.Population {
		if ind.Valid {
			nicheCounts[ind.Genome.Skeleton]++
		}
	}

	totalValid := 0
	for _, c := range nicheCounts {
		totalValid += c
	}
	if totalValid == 0 {
		return
	}

	// Expected share per niche if evenly distributed
	numNiches := len(nicheCounts)
	if numNiches <= 1 {
		// Only one skeleton type present, no sharing needed
		for _, ind := range e.Population {
			if ind.Valid {
				ind.Fitness.SharedFitness = ind.Fitness.TotalFitness
				ind.Genome.SharedFitness = ind.Fitness.SharedFitness
			}
		}
		return
	}

	expectedPerNiche := float64(totalValid) / float64(numNiches)

	// Apply sharing: fitness / (nicheCount / expectedCount)
	// Linear division forces strong balance across skeleton types.
	// Overrepresented niches get proportionally penalized.
	// Underrepresented niches get a boost to prevent extinction.
	for _, ind := range e.Population {
		if !ind.Valid {
			continue
		}
		count := float64(nicheCounts[ind.Genome.Skeleton])
		ratio := count / expectedPerNiche
		if ratio < 1 {
			// Boost underrepresented niches: inverse ratio as multiplier
			// A niche at 50% expected gets 1.5x boost, at 25% gets 2x, etc.
			boost := 1.0 / ratio
			if boost > 3.0 {
				boost = 3.0 // Cap boost to avoid runaway
			}
			ind.Fitness.SharedFitness = ind.Fitness.TotalFitness * boost
		} else {
			ind.Fitness.SharedFitness = ind.Fitness.TotalFitness / ratio
		}
		// genome.json contract (Task 28 round 2): Fitness stays the RAW
		// TotalFitness report.md shows; the blended selection score goes to
		// the explicit SharedFitness field.
		ind.Genome.Fitness = ind.Fitness.TotalFitness
		ind.Genome.SharedFitness = ind.Fitness.SharedFitness
	}
}

// updateBestFitness recomputes BestFitness/BestGenome from the CURRENT
// population's (running-mean) TotalFitness. It deliberately does NOT keep a
// sticky historical max: with elite re-evaluation an elite's mean drifts
// toward its true value, and freezing the highest-ever noisy estimate would
// reintroduce the winner's curse this scheme exists to kill. Elitism keeps
// the best genome in the population, so the current best is the honest
// best. Expect BestFitness to move down as well as up across generations --
// that is the correction working. If no valid individual exists, the
// previous best is retained.
func (e *Engine) updateBestFitness() {
	var best *Individual
	for _, ind := range e.Population {
		if !ind.Valid {
			continue
		}
		if best == nil || ind.Fitness.TotalFitness > best.Fitness.TotalFitness {
			best = ind
		}
	}
	if best != nil {
		e.BestFitness = best.Fitness.TotalFitness
		e.BestGenome = best.Genome
	}
}

// Select performs tournament selection to create the next generation.
func (e *Engine) Select() []*Individual {
	// Sort by shared fitness (descending) -- niche-adjusted
	sort.Slice(e.Population, func(i, j int) bool {
		return e.Population[i].Fitness.SharedFitness > e.Population[j].Fitness.SharedFitness
	})

	// Track best by raw TotalFitness across the whole population -- the
	// SharedFitness-sorted leader is not necessarily the highest raw fitness,
	// because niche sharing can demote an overrepresented-niche genome that
	// would otherwise be the best.
	e.updateBestFitness()

	nextGen := make([]*Individual, e.Config.PopulationSize)

	// Elitism: top N carry forward, including their running-mean state.
	// Elites are re-evaluated with a fresh seed every generation (see
	// EvaluatePopulation); carrying EvalCount/FitnessSum is what turns the
	// next evaluation into a running mean instead of a fresh point estimate.
	elite := min(e.Config.EliteSize, len(e.Population))
	for i := 0; i < elite; i++ {
		nextGen[i] = &Individual{
			Genome:     e.Population[i].Genome,
			Fitness:    e.Population[i].Fitness,
			Valid:      true,
			EvalCount:  e.Population[i].EvalCount,
			FitnessSum: e.Population[i].FitnessSum,
			MctsSum:    e.Population[i].MctsSum,
			MctsCount:  e.Population[i].MctsCount,
		}
	}

	// Fill rest via tournament selection + mutation/crossover
	for i := elite; i < e.Config.PopulationSize; i++ {
		parent := e.tournament()

		if e.rng.Float64() < 0.3 {
			// Crossover (cross-skeleton produces a hybrid when enabled)
			parent2 := e.tournament()
			child := CrossoverWith(parent.Genome, parent2.Genome, e.rng, e.Config.CrossSkeleton)
			if child != nil {
				child = e.mutate(child)
				nextGen[i] = &Individual{Genome: child}
				continue
			}
		}

		// Mutation only
		child := e.mutate(parent.Genome)
		nextGen[i] = &Individual{Genome: child}
	}

	// Diversity: kill duplicate genomes in top 50
	e.dedup(nextGen)

	return nextGen
}

// mutate applies MutateWith threaded with this engine's cross-skeleton flag,
// so cross-family borrow mutations are reachable whenever -cross-skeleton is on.
func (e *Engine) mutate(g *genome.Genome) *genome.Genome {
	return MutateWith(g, e.rng, e.Seeds, e.Config.CrossSkeleton)
}

func (e *Engine) tournament() *Individual {
	best := e.Population[e.rng.IntN(len(e.Population))]
	for i := 1; i < e.Config.TournamentSize; i++ {
		candidate := e.Population[e.rng.IntN(len(e.Population))]
		if candidate.Fitness.SharedFitness > best.Fitness.SharedFitness {
			best = candidate
		}
	}
	return best
}

// dedup removes duplicate genomes in the top 50 by replacing each duplicate
// with a fresh mutation. Preference order for the parent: a non-duplicate
// individual already in pop, then a random seed, then the duplicate itself.
func (e *Engine) dedup(pop []*Individual) {
	seen := make(map[string]bool)
	top := min(50, len(pop))
	for i := 0; i < top; i++ {
		if pop[i].Genome == nil {
			continue
		}
		hash := genomeHash(pop[i].Genome)
		if seen[hash] {
			parent := e.dedupParent(pop, top, seen, hash, i)
			if parent != nil {
				child := e.mutate(parent)
				pop[i].Genome = child
				pop[i].Valid = false
				// The genome changed: prior evaluations are meaningless --
				// in both modes.
				pop[i].EvalCount = 0
				pop[i].FitnessSum = 0
				pop[i].MctsSum = 0
				pop[i].MctsCount = 0
				hash = genomeHash(child)
			}
		}
		seen[hash] = true
	}
}

// dedupParent picks a parent genome for replacing a duplicate at pop[i]:
// first a pop entry whose hash differs from the duplicate, then a random
// seed, and finally the duplicate itself as a last resort.
func (e *Engine) dedupParent(pop []*Individual, top int, seen map[string]bool, dupHash string, i int) *genome.Genome {
	for j := 0; j < top; j++ {
		if j == i || pop[j].Genome == nil {
			continue
		}
		if genomeHash(pop[j].Genome) != dupHash {
			return pop[j].Genome
		}
	}
	if len(e.Seeds) > 0 {
		return e.Seeds[e.rng.IntN(len(e.Seeds))]
	}
	return pop[i].Genome
}

// genomeHash returns a dedup key covering every field that mutation or
// crossover can change. ID, Generation, and Fitness are deliberately omitted
// so semantically identical genomes still collapse. Borrowed, SpecialCards,
// and Scoring.CardPoints are canonicalized into a deterministic order so
// permutations that don't change meaning still hash equally.
func genomeHash(g *genome.Genome) string {
	canon := struct {
		Skeleton     genome.SkeletonType
		Players      int
		HandSize     int
		TrumpRule    genome.TrumpRule
		Shedding     *genome.SheddingParams
		TrickTaking  *genome.TrickTakingParams
		Rummy        *genome.RummyParams
		Borrowed     []genome.BorrowedMechanic
		SpecialCards []genome.SpecialCard
		Scoring      genome.ScoringConfig
	}{
		Skeleton:     g.Skeleton,
		Players:      g.Players,
		HandSize:     g.HandSize,
		TrumpRule:    g.TrumpRule,
		Shedding:     g.Shedding,
		TrickTaking:  g.TrickTaking,
		Rummy:        g.Rummy,
		Borrowed:     sortedBorrowed(g.Borrowed),
		SpecialCards: sortedSpecialCards(g.SpecialCards),
		Scoring: genome.ScoringConfig{
			CardPoints: sortedCardPoints(g.Scoring.CardPoints),
			TrumpSuit:  g.Scoring.TrumpSuit,
		},
	}
	b, err := json.Marshal(canon)
	if err != nil {
		return fmt.Sprintf("err:%v", err)
	}
	return string(b)
}

// outputHash is the OUTPUT-ranking dedup key (Wave K fix 2): genomeHash with
// genes that are DEAD under the rulebook's own liveness rules zeroed out --
// dead scoring borrows (genome.LiveBorrows), unread card_points blocks
// (genome.LiveCardPoints), and special cards on non-shedding skeletons
// (which no runner simulates; dd-24e). The flagship-r3 leaderboard's ranks
// 1/2/3 were one game differing only in dead card_points genes: same rules,
// same rulebook, three slots. Two genomes whose LIVE views match are the
// same published game and must take one output slot.
//
// POPULATION dedup (enforceDiversity/dedupParent) deliberately keeps the
// byte-level genomeHash: dead genes are still evolutionary material (a later
// mutation can revive them), so the search may carry the variants -- only
// publication collapses them.
func outputHash(g *genome.Genome) string {
	c := g.Clone()
	c.Borrowed = g.LiveBorrows()
	if !g.LiveCardPoints() {
		c.Scoring.CardPoints = nil
	}
	if g.Skeleton != genome.Shedding {
		c.SpecialCards = nil
	}
	return genomeHash(c)
}

func sortedBorrowed(in []genome.BorrowedMechanic) []genome.BorrowedMechanic {
	if len(in) == 0 {
		return nil
	}
	out := append([]genome.BorrowedMechanic(nil), in...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Source != out[j].Source {
			return out[i].Source < out[j].Source
		}
		return out[i].Mechanic < out[j].Mechanic
	})
	return out
}

func sortedSpecialCards(in []genome.SpecialCard) []genome.SpecialCard {
	if len(in) == 0 {
		return nil
	}
	out := append([]genome.SpecialCard(nil), in...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Type != out[j].Type {
			return out[i].Type < out[j].Type
		}
		if out[i].ByRank != out[j].ByRank {
			return out[i].ByRank < out[j].ByRank
		}
		return out[i].BySuit < out[j].BySuit
	})
	return out
}

func sortedCardPoints(in []genome.CardScoring) []genome.CardScoring {
	if len(in) == 0 {
		return nil
	}
	out := append([]genome.CardScoring(nil), in...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Rank != out[j].Rank {
			return out[i].Rank < out[j].Rank
		}
		if out[i].Suit != out[j].Suit {
			return out[i].Suit < out[j].Suit
		}
		if out[i].Event != out[j].Event {
			return out[i].Event < out[j].Event
		}
		return out[i].Points < out[j].Points
	})
	return out
}

// Run executes the full evolution loop.
func (e *Engine) Run(progress func(gen int, best float64, avg float64)) {
	e.Initialize()

	for gen := 0; gen < e.Config.Generations; gen++ {
		e.Generation = gen
		e.EvaluatePopulation()

		// Compute stats
		totalFit := 0.0
		validCount := 0
		for _, ind := range e.Population {
			if ind.Valid {
				totalFit += ind.Fitness.TotalFitness
				validCount++
			}
		}
		avgFit := 0.0
		if validCount > 0 {
			avgFit = totalFit / float64(validCount)
		}

		if progress != nil {
			progress(gen, e.BestFitness, avgFit)
		}

		// Select and create next generation
		e.Population = e.Select()
	}

	// Final evaluation. Select never runs on this population, so update
	// BestFitness directly to capture any post-final-Select offspring whose
	// raw fitness beats the prior best. Bump Generation past the loop range
	// so elites get a fresh seed here too instead of repeating the last
	// generation's evaluation.
	e.Generation = e.Config.Generations
	e.EvaluatePopulation()
	e.updateBestFitness()
}

// TopN returns the top N genomes ensuring skeleton diversity.
// Reserves slots per skeleton proportionally, then fills remaining with best overall.
//
// Ordering is by OutputRank -- the greedy-only running mean, the
// commensurable leaderboard key (Wave K fix 1; sorting by published fitness
// mixed MCTS-mode and greedy-only means in one ranking).
//
// Functionally identical genomes are deduplicated by outputHash (Task 28
// round 2 caught byte-identical clones under different IDs; Wave K fix 2
// extends the key to ignore DEAD genes after flagship-r3 published one game
// as ranks 1/2/3 differing only in unread card_points). The population is
// sorted by rank first, so the kept member of each clone group is its best
// and the freed slots flow to the next-best DISTINCT genomes.
func (e *Engine) TopN(n int) []*Individual {
	sort.Slice(e.Population, func(i, j int) bool {
		return e.Population[i].OutputRank() > e.Population[j].OutputRank()
	})

	// Reserve at least n/numSkeletons slots per skeleton type
	perSkeleton := n / 3
	if perSkeleton < 2 {
		perSkeleton = 2
	}

	used := make(map[int]bool)
	seen := make(map[string]bool)
	var result []*Individual

	// First pass: take top perSkeleton from each skeleton
	skeletonCounts := make(map[genome.SkeletonType]int)
	for i, ind := range e.Population {
		if !ind.Valid || ind.Genome == nil {
			continue
		}
		skel := ind.Genome.Skeleton
		if skeletonCounts[skel] < perSkeleton {
			if hash := outputHash(ind.Genome); !seen[hash] {
				seen[hash] = true
				result = append(result, ind)
				used[i] = true
				skeletonCounts[skel]++
			}
		}
		if len(result) >= n {
			break
		}
	}

	// Second pass: fill remaining slots with best overall
	for i, ind := range e.Population {
		if len(result) >= n {
			break
		}
		if used[i] || !ind.Valid || ind.Genome == nil {
			continue
		}
		if hash := outputHash(ind.Genome); !seen[hash] {
			seen[hash] = true
			result = append(result, ind)
		}
	}

	return result
}

// SavePopulation writes the full population to a JSON file.
func (e *Engine) SavePopulation(path string) error {
	var genomes []*genome.Genome
	for _, ind := range e.Population {
		if ind.Genome != nil {
			genomes = append(genomes, ind.Genome)
		}
	}

	data, err := json.MarshalIndent(genomes, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

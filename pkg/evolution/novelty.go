package evolution

import (
	"fmt"
	"math"
	"math/rand/v2"
	"runtime"
	"sort"
	"sync"

	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/genome"
)

const (
	NoveltyK      = 15  // k-nearest neighbors for novelty
	NoveltyWeight = 0.5 // weight of novelty vs fitness

	// NoveltyAddThreshold is the INITIAL absolute archive-admission
	// threshold: an individual enters the archive iff its behavior is
	// farther than this from every same-skeleton archive entry, including
	// entries admitted earlier in the same pass. This replaces the old
	// per-generation-max-normalized threshold, which re-admitted
	// persisting elites every generation (audit Task 18). Initialized from
	// the measured descriptor spread of the Task 17 population (8 classics
	// + 50 Tier-0-valid mutants, 2026-06-11): within-skeleton
	// nearest-neighbor distance medians were shedding 0.014, trick-taking
	// 0.002, rummy 0.001 with p90s 0.040/0.034/0.006, so 0.02 sits in the
	// upper NN tail -- the right order of magnitude for a low-single-digit
	// admission rate. The engine adapts it from there (noveltyAdmitLo/Hi).
	NoveltyAddThreshold = 0.02

	// NoveltyArchiveCap bounds archive growth. Eviction at the cap is
	// uniform-random: FIFO eviction turned the archive into a sliding
	// window that forgot early coverage (audit Task 18).
	NoveltyArchiveCap = 1000

	// Adaptive admission control: after each admission pass the threshold
	// doubles if more than noveltyAdmitHi of the population was admitted
	// and halves below noveltyAdmitLo, keeping admissions at roughly 2-5%
	// per generation. Clamped to [noveltyThresholdMin, noveltyThresholdMax]
	// (the max distance in the unit behavior square is sqrt(2) ~ 1.414).
	noveltyAdmitLo      = 0.02
	noveltyAdmitHi      = 0.05
	noveltyThresholdMin = 1e-3
	noveltyThresholdMax = 1.5

	// DefaultFitnessFloor is derived from the seed-calibration suite
	// (Task 15): worst classic survivor-conditioned mean minus 0.05. The
	// original folklore value 0.70 sat ABOVE every trick-taking classic,
	// zeroing the selection gradient for human-validated games; Task 14
	// derived 0.42 from crazy-eights' then-mean 0.475. RE-DERIVED in round 2
	// (Task 28 step 4): the choice-impact decisions fix moved the classics
	// to 0.451-0.578 (worst: crazy-eights 0.451, ROUND 2 block in
	// pkg/fitness/calibration_test.go), so the floor is 0.451 - 0.05 = 0.40.
	// If the calibration suite's worst-classic mean moves, re-derive this
	// constant (TestCalibrationClassicsAboveFloor enforces the relationship).
	DefaultFitnessFloor = 0.40
)

// FitnessFloor is the minimum fitness for QD consideration (novelty archive
// admission, sharing, output ranking). Overridable via the evolve command's
// -fitness-floor flag; everything else should treat it as a constant.
var FitnessFloor = DefaultFitnessFloor

// NoveltyIndividual extends Individual with behavior and novelty score.
type NoveltyIndividual struct {
	Individual
	Behavior BehaviorDescriptor
	Novelty  float64
}

// NoveltyEngine runs novelty search with a fitness floor.
type NoveltyEngine struct {
	Config     Config
	Seeds      []*genome.Genome
	Population []*NoveltyIndividual
	Archive    []*NoveltyIndividual // Novelty archive (memory of exploration)
	Generation int
	rng        *rand.Rand

	// addThreshold is the current absolute archive-admission threshold,
	// adapted each generation (see NoveltyAddThreshold).
	addThreshold float64

	// Stats
	BestFitness float64
	BestGenome  *genome.Genome
}

// NewNoveltyEngine creates a novelty search engine.
func NewNoveltyEngine(config Config, seeds []*genome.Genome) *NoveltyEngine {
	if config.Workers == 0 {
		config.Workers = runtime.NumCPU()
	}
	return &NoveltyEngine{
		Config:       config,
		Seeds:        seeds,
		rng:          rand.New(rand.NewPCG(config.BaseSeed, 0)),
		addThreshold: NoveltyAddThreshold,
	}
}

// Run executes the novelty search loop.
func (e *NoveltyEngine) Run(progress func(gen int, best float64, avg float64)) {
	e.initialize()

	for gen := 0; gen < e.Config.Generations; gen++ {
		e.Generation = gen
		e.evaluatePopulation()
		e.computeNovelty()

		// Stats
		totalFit := 0.0
		validCount := 0
		for _, ind := range e.Population {
			if ind.Valid && ind.Fitness.TotalFitness >= FitnessFloor {
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

		e.Population = e.selectNext()
	}

	// Final evaluation. Bump Generation past the loop range so elites get
	// fresh seeds here instead of repeating the last generation's
	// evaluation, which would double-count the same sample in the running
	// mean (mirrors engine.go, Task 13.3).
	e.Generation = e.Config.Generations
	e.evaluatePopulation()
	e.computeNovelty()
}

func (e *NoveltyEngine) initialize() {
	e.Population = make([]*NoveltyIndividual, e.Config.PopulationSize)

	for i := 0; i < e.Config.PopulationSize; i++ {
		seed := e.Seeds[e.rng.IntN(len(e.Seeds))]
		g := Mutate(seed, e.rng, e.Seeds)
		g.ID = fmt.Sprintf("init_%d", i)
		g.Generation = 0
		e.Population[i] = &NoveltyIndividual{
			Individual: Individual{Genome: g},
		}
	}
}

// evaluatePopulation evaluates EVERY individual every generation -- elites
// included. The old `if ind.Valid { continue }` skip froze a single
// (possibly lucky) estimate forever (the winner's curse; engine.go got the
// fix in f882a67, this engine did not -- carried finding (a) of the
// 2026-06-11 checkpoint). The per-generation seed term gives elites fresh
// games each time, and TotalFitness becomes the running mean over all
// evaluations of the exact genome via EvalCount/FitnessSum. Behavior is
// recomputed from a fresh batch alongside, so the descriptor estimate
// sharpens with the fitness estimate.
func (e *NoveltyEngine) evaluatePopulation() {
	var wg sync.WaitGroup
	sem := make(chan struct{}, e.Config.Workers)

	for i, ind := range e.Population {
		wg.Add(1)
		sem <- struct{}{}

		go func(idx int, ind *NoveltyIndividual) {
			defer wg.Done()
			defer func() { <-sem }()

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

			// Compute behavior from a fresh batch (hooks included --
			// BehaviorBatch is the single descriptor-batch site).
			if batchResult, ok := BehaviorBatch(ind.Genome, seed+5000); ok {
				ind.Behavior = ComputeBehavior(batchResult)
			}
		}(i, ind)
	}

	wg.Wait()

	// MCTS-for-top-decile (audit Task 20b): same pass as the baseline
	// engine, BEFORE best-fitness/novelty bookkeeping so everything
	// downstream flows from the published (possibly MCTS-mean) fitness.
	e.runMCTSTopDecile()

	e.updateBestFitness()
}

// runMCTSTopDecile applies the shared decile pass to this engine's
// population (see evaluateTopDecileMCTS in engine.go).
func (e *NoveltyEngine) runMCTSTopDecile() {
	cands := make([]mctsCandidate, 0, len(e.Population))
	for i, ind := range e.Population {
		if ind != nil && ind.Valid {
			cands = append(cands, mctsCandidate{ind: &ind.Individual, idx: i})
		}
	}
	evaluateTopDecileMCTS(e.Config, e.Generation, cands)
}

// updateBestFitness recomputes BestFitness/BestGenome from the CURRENT
// population's (running-mean) TotalFitness. It deliberately does NOT keep a
// sticky historical max: with elite re-evaluation an elite's mean drifts
// toward its true value, and freezing the highest-ever noisy estimate would
// reintroduce the winner's curse. If no valid individual exists, the
// previous best is retained.
func (e *NoveltyEngine) updateBestFitness() {
	var best *NoveltyIndividual
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

// computeNovelty calculates novelty score for each individual based on
// k-nearest neighbor distance in behavior space, computed WITHIN-SKELETON only.
// Also applies fitness sharing by skeleton niche to prevent monopoly.
func (e *NoveltyEngine) computeNovelty() {
	// Collect behavior points per skeleton (population + archive). Track the
	// owning *NoveltyIndividual so each individual can skip its own entry by
	// identity instead of filtering zero-valued distances -- otherwise
	// converged clusters drop their intra-cluster zeros and pull k-NN from
	// faraway outliers, which would reward (not penalize) duplication.
	type behaviorPoint struct {
		Behavior BehaviorDescriptor
		Owner    *NoveltyIndividual // nil for archive entries
	}
	perSkeleton := make(map[genome.SkeletonType][]behaviorPoint)
	for _, ind := range e.Population {
		if ind.Valid && ind.Fitness.TotalFitness >= FitnessFloor {
			skel := ind.Genome.Skeleton
			perSkeleton[skel] = append(perSkeleton[skel], behaviorPoint{Behavior: ind.Behavior, Owner: ind})
		}
	}
	for _, arch := range e.Archive {
		skel := arch.Genome.Skeleton
		perSkeleton[skel] = append(perSkeleton[skel], behaviorPoint{Behavior: arch.Behavior, Owner: nil})
	}

	if len(perSkeleton) == 0 {
		return
	}

	// Count valid individuals per skeleton for fitness sharing
	nicheCounts := make(map[genome.SkeletonType]int)
	totalValid := 0
	for _, ind := range e.Population {
		if ind.Valid && ind.Fitness.TotalFitness >= FitnessFloor {
			nicheCounts[ind.Genome.Skeleton]++
			totalValid++
		}
	}

	// Compute novelty per skeleton (k-NN within-skeleton only)
	maxNoveltyPerSkel := make(map[genome.SkeletonType]float64)
	for _, ind := range e.Population {
		if !ind.Valid || ind.Fitness.TotalFitness < FitnessFloor {
			ind.Novelty = 0
			continue
		}

		skel := ind.Genome.Skeleton
		points := perSkeleton[skel]

		var distances []float64
		for _, p := range points {
			if p.Owner == ind {
				continue // skip self by identity, not by zero distance
			}
			distances = append(distances, ind.Behavior.Distance(p.Behavior))
		}

		if len(distances) == 0 {
			ind.Novelty = 0
			continue
		}

		sort.Float64s(distances)
		k := NoveltyK
		if k > len(distances) {
			k = len(distances)
		}

		sum := 0.0
		for i := 0; i < k; i++ {
			sum += distances[i]
		}
		ind.Novelty = sum / float64(k)

		if ind.Novelty > maxNoveltyPerSkel[skel] {
			maxNoveltyPerSkel[skel] = ind.Novelty
		}
	}

	// Expected share per skeleton for fitness sharing
	numNiches := len(nicheCounts)
	expectedPerNiche := 0.0
	if numNiches > 0 {
		expectedPerNiche = float64(totalValid) / float64(numNiches)
	}

	// Normalize novelty within each skeleton (so rummy variations compete
	// against rummy, not against all games) and apply fitness sharing.
	for _, ind := range e.Population {
		if !ind.Valid || ind.Fitness.TotalFitness < FitnessFloor {
			ind.Fitness.SharedFitness = 0
			continue
		}

		skel := ind.Genome.Skeleton
		maxNov := maxNoveltyPerSkel[skel]
		normalizedNovelty := 0.0
		if maxNov > 0 {
			normalizedNovelty = ind.Novelty / maxNov
		}

		// Combined: 50% fitness + 50% within-skeleton novelty
		combined := (1-NoveltyWeight)*ind.Fitness.TotalFitness + NoveltyWeight*normalizedNovelty

		// Apply fitness sharing by skeleton niche
		count := float64(nicheCounts[skel])
		ratio := count / expectedPerNiche
		if ratio < 1 {
			// Boost underrepresented niches (same as main engine)
			boost := 1.0 / ratio
			if boost > 3.0 {
				boost = 3.0
			}
			combined *= boost
		} else {
			combined /= ratio
		}

		ind.Fitness.SharedFitness = combined
		// genome.json contract (Task 28 round 2): Fitness stays the RAW
		// TotalFitness report.md shows; the novelty-blended selection score
		// goes to the explicit SharedFitness field. The flagship published
		// genome.json files whose blended 0.41-class "fitness" contradicted
		// their own report.md's 0.94-class raw value.
		ind.Genome.Fitness = ind.Fitness.TotalFitness
		ind.Genome.SharedFitness = ind.Fitness.SharedFitness
	}

	// Archive admission (audit Task 18): absolute distance threshold,
	// within-skeleton. Admit iff the behavior is farther than addThreshold
	// from every same-skeleton archive entry -- including entries admitted
	// earlier in this pass, so duplicates within one generation cannot
	// double-admit and persisting elites (distance 0 to their own archive
	// copy) are never re-admitted. The old rule compared per-generation
	// max-normalized novelty against a constant, which re-admitted elites
	// every generation. The FitnessFloor still gates admission: the
	// archive is exploration memory for QD-qualified games.
	admitted := 0
	for _, ind := range e.Population {
		if !ind.Valid || ind.Fitness.TotalFitness < FitnessFloor {
			continue
		}
		if e.nearestArchiveDistance(ind.Genome.Skeleton, ind.Behavior) > e.addThreshold {
			// SNAPSHOT the genome (round 3 commit 5a): an archive entry is a
			// frozen record of the individual AT ADMISSION. Sharing the live
			// genome pointer let later re-evaluations of the still-evolving
			// elite overwrite the archived genome's Fitness while the
			// archived metrics stayed frozen -- the r2 flagship published a
			// report.md (0.847, from the archived metrics) contradicting its
			// own genome.json (0.808, from the shared pointer's later mean).
			archived := ind.Individual
			archived.Genome = ind.Genome.Clone()
			e.Archive = append(e.Archive, &NoveltyIndividual{
				Individual: archived,
				Behavior:   ind.Behavior,
				Novelty:    ind.Novelty,
			})
			admitted++
		}
	}

	// Adaptive admission control: keep admissions at roughly 2-5% of the
	// population per generation by doubling/halving the threshold.
	if n := len(e.Population); n > 0 {
		rate := float64(admitted) / float64(n)
		if rate > noveltyAdmitHi {
			e.addThreshold = min(e.addThreshold*2, noveltyThresholdMax)
		} else if rate < noveltyAdmitLo {
			e.addThreshold = max(e.addThreshold/2, noveltyThresholdMin)
		}
	}

	// Cap archive size with uniform-random eviction: every entry is equally
	// likely to go, preserving early coverage memory (FIFO truncation made
	// the archive a sliding window of recent generations).
	for len(e.Archive) > NoveltyArchiveCap {
		i := e.rng.IntN(len(e.Archive))
		e.Archive[i] = e.Archive[len(e.Archive)-1]
		e.Archive = e.Archive[:len(e.Archive)-1]
	}
}

// nearestArchiveDistance returns the behavior distance from b to the nearest
// archive entry of the given skeleton, or +Inf if that skeleton has no
// entries yet (so a skeleton's first qualified individual is always
// admitted). Within-skeleton on purpose: descriptors of different skeletons
// share the unit square but describe different game families, consistent
// with the engine's within-skeleton novelty computation.
func (e *NoveltyEngine) nearestArchiveDistance(skel genome.SkeletonType, b BehaviorDescriptor) float64 {
	best := math.Inf(1)
	for _, arch := range e.Archive {
		if arch.Genome.Skeleton != skel {
			continue
		}
		if d := b.Distance(arch.Behavior); d < best {
			best = d
		}
	}
	return best
}

func (e *NoveltyEngine) selectNext() []*NoveltyIndividual {
	// Sort by shared fitness (novelty-adjusted)
	sort.Slice(e.Population, func(i, j int) bool {
		return e.Population[i].Fitness.SharedFitness > e.Population[j].Fitness.SharedFitness
	})

	nextGen := make([]*NoveltyIndividual, e.Config.PopulationSize)

	// Elitism: top N carry forward, including their running-mean state.
	// Elites are re-evaluated with a fresh seed every generation (see
	// evaluatePopulation); carrying EvalCount/FitnessSum is what turns the
	// next evaluation into a running mean instead of a fresh point estimate.
	elite := min(e.Config.EliteSize, len(e.Population))
	for i := 0; i < elite; i++ {
		nextGen[i] = &NoveltyIndividual{
			Individual: Individual{
				Genome:     e.Population[i].Genome,
				Fitness:    e.Population[i].Fitness,
				Valid:      true,
				EvalCount:  e.Population[i].EvalCount,
				FitnessSum: e.Population[i].FitnessSum,
				MctsSum:    e.Population[i].MctsSum,
				MctsCount:  e.Population[i].MctsCount,
			},
			Behavior: e.Population[i].Behavior,
		}
	}

	// Tournament selection + mutation
	for i := elite; i < e.Config.PopulationSize; i++ {
		parent := e.tournament()

		var child *genome.Genome
		if e.rng.Float64() < 0.3 {
			parent2 := e.tournament()
			child = Crossover(parent.Genome, parent2.Genome, e.rng)
		}
		if child == nil {
			child = Mutate(parent.Genome, e.rng, e.Seeds)
		} else {
			child = Mutate(child, e.rng, e.Seeds)
		}

		child.ID = fmt.Sprintf("gen%d_%d", e.Generation+1, e.rng.IntN(100000))
		child.Generation = e.Generation + 1
		nextGen[i] = &NoveltyIndividual{
			Individual: Individual{Genome: child},
		}
	}

	return nextGen
}

func (e *NoveltyEngine) tournament() *NoveltyIndividual {
	best := e.Population[e.rng.IntN(len(e.Population))]
	for i := 1; i < e.Config.TournamentSize; i++ {
		candidate := e.Population[e.rng.IntN(len(e.Population))]
		if candidate.Fitness.SharedFitness > best.Fitness.SharedFitness {
			best = candidate
		}
	}
	return best
}

// AllQualified returns all individuals meeting the fitness floor.
//
// Functionally identical genomes are deduplicated by outputHash, keeping
// each clone group's best-fitness member (Task 28 round 2: ID-only dedup let
// the flagship publish a 6-way clone group under distinct IDs; Wave K fix 2
// widened the key from byte-identical to identical-modulo-dead-genes after
// flagship-r3 ranks 1/2/3). Behaviors stay parallel to individuals
// throughout.
func (e *NoveltyEngine) AllQualified() ([]*Individual, []BehaviorDescriptor) {
	type entry struct {
		ind      *Individual
		behavior BehaviorDescriptor
		order    int // first-seen position, for stable output order
	}
	best := make(map[string]*entry)
	order := 0

	add := func(ind *Individual, b BehaviorDescriptor) {
		hash := outputHash(ind.Genome)
		if cur, ok := best[hash]; ok {
			// Clone-group keep is by OutputRank, the commensurable
			// leaderboard key (Wave K fix 1).
			if ind.OutputRank() > cur.ind.OutputRank() {
				cur.ind, cur.behavior = ind, b // keep first-seen order
			}
			return
		}
		best[hash] = &entry{ind: ind, behavior: b, order: order}
		order++
	}

	for _, ind := range e.Population {
		if ind.Valid && ind.Fitness.TotalFitness >= FitnessFloor {
			add(&ind.Individual, ind.Behavior)
		}
	}
	for _, arch := range e.Archive {
		if arch.Fitness.TotalFitness >= FitnessFloor {
			add(&arch.Individual, arch.Behavior)
		}
	}

	entries := make([]*entry, 0, len(best))
	for _, en := range best {
		entries = append(entries, en)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].order < entries[j].order })

	individuals := make([]*Individual, 0, len(entries))
	behaviors := make([]BehaviorDescriptor, 0, len(entries))
	for _, en := range entries {
		individuals = append(individuals, en.ind)
		behaviors = append(behaviors, en.behavior)
	}
	return individuals, behaviors
}

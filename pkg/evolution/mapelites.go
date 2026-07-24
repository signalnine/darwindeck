package evolution

import (
	"fmt"
	"math/rand/v2"
	"runtime"
	"sort"
	"sync"

	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/genome"
)

const GridSize = 10

// ArchiveCell holds the best individual for a behavior niche.
type ArchiveCell struct {
	Individual *Individual
	Behavior   BehaviorDescriptor
}

// MAPElitesEngine runs MAP-Elites quality-diversity search.
// Maintains one 10x10 archive per skeleton type.
type MAPElitesEngine struct {
	Config   Config
	Seeds    []*genome.Genome
	Archives map[genome.SkeletonType]*Archive
	rng      *rand.Rand

	// evaluate is the fitness seam (default fitness.Evaluate); tests stub
	// it to make challenge re-evaluations deterministic and free.
	evaluate func(*genome.Genome, uint64) fitness.EvaluationResult

	// challenges counts incumbent re-evaluations, deriving a fresh seed for
	// each (see nextChallengeSeed).
	challenges uint64

	// Stats
	BestFitness float64
	BestGenome  *genome.Genome
}

// Archive is a 2D grid of behavior niches.
type Archive struct {
	Cells    [GridSize][GridSize]*ArchiveCell
	Occupied int
	QDScore  float64
}

// NewMAPElitesEngine creates a MAP-Elites engine.
func NewMAPElitesEngine(config Config, seeds []*genome.Genome) *MAPElitesEngine {
	if config.Workers == 0 {
		config.Workers = runtime.NumCPU()
	}
	archives := make(map[genome.SkeletonType]*Archive, len(genome.AllSkeletons()))
	for _, skel := range genome.AllSkeletons() {
		archives[skel] = &Archive{}
	}
	return &MAPElitesEngine{
		Config:   config,
		Seeds:    seeds,
		Archives: archives,
		rng:      rand.New(rand.NewPCG(config.BaseSeed, 0)),
		evaluate: fitness.Evaluate,
	}
}

// Run executes the MAP-Elites algorithm.
func (e *MAPElitesEngine) Run(progress func(gen int, best float64, avg float64)) {
	// Seed the archives with initial evaluations
	e.seedArchives()

	// Main loop: generate offspring, evaluate, attempt insertion
	for gen := 0; gen < e.Config.Generations; gen++ {
		e.generation(gen)

		if progress != nil {
			occupied, qdScore := e.totalStats()
			progress(gen, e.BestFitness, qdScore/float64(max(occupied, 1)))
		}
	}
}

// seedArchives evaluates seed genomes and mutants to initialize archives.
func (e *MAPElitesEngine) seedArchives() {
	var candidates []*genome.Genome

	// Create initial candidates from seeds + mutations
	for i := 0; i < e.Config.PopulationSize; i++ {
		seed := e.Seeds[e.rng.IntN(len(e.Seeds))]
		g := e.mutate(seed)
		g.ID = fmt.Sprintf("init_%d", i)
		g.Generation = 0
		candidates = append(candidates, g)
	}

	// Evaluate and insert
	e.evaluateAndInsert(candidates, 0)
}

// generation produces PopulationSize offspring and attempts archive insertion.
func (e *MAPElitesEngine) generation(gen int) {
	var offspring []*genome.Genome

	for i := 0; i < e.Config.PopulationSize; i++ {
		// Pick a random occupied cell from a random archive
		parent := e.randomArchiveOccupant()
		if parent == nil {
			// No occupants yet, mutate a seed
			seed := e.Seeds[e.rng.IntN(len(e.Seeds))]
			child := e.mutate(seed)
			child.ID = fmt.Sprintf("gen%d_%d", gen+1, e.rng.IntN(100000))
			child.Generation = gen + 1
			offspring = append(offspring, child)
			continue
		}

		var child *genome.Genome
		if e.rng.Float64() < 0.3 {
			// Crossover with another archive occupant of same skeleton. Parents
			// always share a skeleton here, so CrossoverWith takes the ordinary
			// same-skeleton path and the flag is genuinely inert on THIS call --
			// it is threaded for consistency with the mutation path below.
			parent2 := e.randomArchiveOccupantOfSkeleton(parent.Skeleton)
			if parent2 != nil {
				child = CrossoverWith(parent, parent2, e.rng, e.Config.CrossSkeleton)
			}
		}
		if child == nil {
			child = e.mutate(parent)
		} else {
			child = e.mutate(child)
		}

		child.ID = fmt.Sprintf("gen%d_%d", gen+1, e.rng.IntN(100000))
		child.Generation = gen + 1
		offspring = append(offspring, child)
	}

	e.evaluateAndInsert(offspring, gen+1)
}

// evaluateAndInsert evaluates genomes in parallel and inserts qualifying ones into archives.
func (e *MAPElitesEngine) evaluateAndInsert(genomes []*genome.Genome, gen int) {
	type evalResult struct {
		genome   *genome.Genome
		metrics  fitness.Metrics
		behavior BehaviorDescriptor
		valid    bool
	}

	results := make([]evalResult, len(genomes))
	var wg sync.WaitGroup
	sem := make(chan struct{}, e.Config.Workers)

	for i, g := range genomes {
		wg.Add(1)
		sem <- struct{}{}

		go func(idx int, g *genome.Genome) {
			defer wg.Done()
			defer func() { <-sem }()

			seed := e.Config.BaseSeed + uint64(gen)*10000 + uint64(idx)
			result := e.evaluate(g, seed)

			// No fitness gate here: archive ADMISSION is floor-free (audit
			// Task 18). The old hardcoded 0.70 cutoff silently emptied the
			// archive after the Task 14 recalibration (classics 0.43-0.65)
			// and threw away stepping stones. The FitnessFloor applies to
			// OUTPUT (AllQualified) only.
			if !result.Valid {
				return
			}

			// Compute behavior from a fresh batch (hooks included --
			// BehaviorBatch is the single descriptor-batch site).
			batchResult, ok := BehaviorBatch(g, seed+5000)
			if !ok {
				return
			}
			behavior := ComputeBehavior(batchResult)

			results[idx] = evalResult{
				genome:   g,
				metrics:  result.Metrics,
				behavior: behavior,
				valid:    true,
			}
		}(i, g)
	}

	wg.Wait()

	// Insert into archives (sequential to avoid races)
	for _, r := range results {
		if !r.valid {
			continue
		}
		e.insert(r.genome, r.metrics, r.behavior)
	}
}

// mapElitesChallengeSeedBase/Stride derive the fresh seeds for incumbent
// challenge re-evaluations. The base (2^40) sits far above the offspring
// evaluation band (BaseSeed + gen*10000 + idx, internal derived seeds
// spanning < +2020), so re-evaluation games never replay offspring
// evaluation games; the stride covers one evaluation's internal span. The
// counter advances once per challenge, and insertion is sequential
// (evaluateAndInsert inserts after the parallel eval completes), so the
// sequence is deterministic for a fixed run.
const (
	mapElitesChallengeSeedBase   = uint64(1) << 40
	mapElitesChallengeSeedStride = 10000
)

func (e *MAPElitesEngine) nextChallengeSeed() uint64 {
	seed := e.Config.BaseSeed + mapElitesChallengeSeedBase + e.challenges*mapElitesChallengeSeedStride
	e.challenges++
	return seed
}

// reevaluateIncumbent runs one fresh-seed evaluation of an occupied cell's
// incumbent and folds it into the incumbent's running mean (the same
// EvalCount/FitnessSum pattern the engines use, audit Task 13.3). An
// invalid re-evaluation (Tier 0/1 kill at the fresh seed) means the
// incumbent is flaky: its history resets and its mean reads 0, so it must
// re-qualify from scratch -- the engines' policy. QDScore follows the
// published mean.
func (e *MAPElitesEngine) reevaluateIncumbent(archive *Archive, cell *ArchiveCell) {
	ind := cell.Individual
	result := e.evaluate(ind.Genome, e.nextChallengeSeed())
	if result.Valid {
		ind.FitnessSum += result.Metrics.TotalFitness
		ind.EvalCount++
	} else {
		ind.FitnessSum = 0
		ind.EvalCount = 0
	}

	old := ind.Fitness.TotalFitness
	mean := 0.0
	if ind.EvalCount > 0 {
		mean = ind.FitnessSum / float64(ind.EvalCount)
	}
	ind.Fitness.TotalFitness = mean
	ind.Genome.Fitness = mean
	archive.QDScore += mean - old
}

// insert offers an evaluated genome to its skeleton archive. Admission is
// pure cell-local elitism: the cell keeps its best occupant REGARDLESS of
// the global FitnessFloor (audit Task 18) -- sub-floor occupants are
// stepping stones for parent selection (randomArchiveOccupant draws from
// all occupants), while the floor applies to output via AllQualified.
// Ties keep the incumbent (strict > comparison). Returns true if the
// genome took the cell.
//
// Challenge re-evaluation (reviewer finding 3, the MAP-Elites winner's
// curse): cells used to admit on a single-seed evaluation and never
// re-evaluate, so a lucky eval (instant-knock's 0.431 on its one surviving
// seed, clearing the 0.42 output floor) held its cell permanently. Now a
// challenger to an OCCUPIED cell triggers one fresh-seed re-evaluation of
// the incumbent first, and the comparison runs on running means on both
// sides (the challenger's mean is its single eval). Repeated challenges
// drag a lucky mean toward its true value until an honestly better
// challenger evicts it. Cost is bounded: re-evaluations happen on cell
// collisions only.
func (e *MAPElitesEngine) insert(g *genome.Genome, metrics fitness.Metrics, behavior BehaviorDescriptor) bool {
	archive := e.Archives[g.Skeleton]
	if archive == nil { // future skeleton not in AllSkeletons yet: create, don't panic
		archive = &Archive{}
		e.Archives[g.Skeleton] = archive
	}
	// Non-sticky best (both return paths): a challenge re-evaluation below can
	// drag the current best occupant's mean down even when the challenger
	// loses, so the headline is recomputed from the archives either way.
	defer e.updateBestFromArchives()
	row, col := behavior.GridCell(GridSize)

	cell := archive.Cells[row][col]
	if cell != nil {
		e.reevaluateIncumbent(archive, cell)
		if metrics.TotalFitness <= cell.Individual.Fitness.TotalFitness {
			return false
		}
		archive.QDScore -= cell.Individual.Fitness.TotalFitness
	} else {
		archive.Occupied++
	}

	archive.Cells[row][col] = &ArchiveCell{
		Individual: &Individual{
			Genome:  g,
			Fitness: metrics,
			Valid:   true,
			// Start the running mean so this occupant can be challenged.
			EvalCount:  1,
			FitnessSum: metrics.TotalFitness,
		},
		Behavior: behavior,
	}
	archive.QDScore += metrics.TotalFitness
	return true
}

// updateBestFromArchives recomputes BestFitness/BestGenome from the current
// occupants' running-mean TotalFitness -- the engines' non-sticky policy
// (Engine.updateBestFitness): a sticky max of single-seed admissions froze a
// lucky eval no surviving archive mean matched (the winner's curse the
// challenge scheme exists to kill). Expect the headline to move down as well
// as up as re-evaluations drag lucky occupants toward their true value.
func (e *MAPElitesEngine) updateBestFromArchives() {
	var bestFit float64
	var bestGenome *genome.Genome
	for _, skel := range genome.AllSkeletons() {
		archive := e.Archives[skel]
		if archive == nil {
			continue
		}
		for r := 0; r < GridSize; r++ {
			for c := 0; c < GridSize; c++ {
				if cell := archive.Cells[r][c]; cell != nil {
					if f := cell.Individual.Fitness.TotalFitness; bestGenome == nil || f > bestFit {
						bestFit, bestGenome = f, cell.Individual.Genome
					}
				}
			}
		}
	}
	if bestGenome != nil {
		e.BestFitness, e.BestGenome = bestFit, bestGenome
	}
}

// randomArchiveOccupant returns a random genome from any occupied archive cell.
func (e *MAPElitesEngine) randomArchiveOccupant() *genome.Genome {
	// Iterate skeletons in a stable order so seeded runs are reproducible;
	// Go map iteration order is randomized per process and would otherwise
	// shuffle the index space passed to rng.IntN.
	var occupants []*genome.Genome
	for _, skel := range genome.AllSkeletons() {
		archive, ok := e.Archives[skel]
		if !ok {
			continue
		}
		for r := 0; r < GridSize; r++ {
			for c := 0; c < GridSize; c++ {
				if archive.Cells[r][c] != nil {
					occupants = append(occupants, archive.Cells[r][c].Individual.Genome)
				}
			}
		}
	}
	if len(occupants) == 0 {
		return nil
	}
	return occupants[e.rng.IntN(len(occupants))]
}

// randomArchiveOccupantOfSkeleton returns a random genome from the given skeleton's archive.
func (e *MAPElitesEngine) randomArchiveOccupantOfSkeleton(skel genome.SkeletonType) *genome.Genome {
	archive := e.Archives[skel]
	var occupants []*genome.Genome
	for r := 0; r < GridSize; r++ {
		for c := 0; c < GridSize; c++ {
			if archive.Cells[r][c] != nil {
				occupants = append(occupants, archive.Cells[r][c].Individual.Genome)
			}
		}
	}
	if len(occupants) == 0 {
		return nil
	}
	return occupants[e.rng.IntN(len(occupants))]
}

// mutate applies MutateWith threaded with this engine's cross-skeleton flag, so
// cross-family borrow MUTATIONS are reachable under -algorithm mapelites (the
// same seam Engine.mutate provides).
//
// This matters more here than the name suggests. addBorrowedMechanic offers
// casino and vying hosts a borrow candidate ONLY under crossSkeleton, so with
// the flag hard-off those two skeletons reached ZERO borrows -- ever -- and the
// deep borrows (run_play / follow_suit / knock / trick_scoring) were unreachable
// on every host. MAP-Elites is the ILLUMINATION algorithm; hard-wiring the
// cross-family axis off made it blind to exactly the region it exists to map.
// (Crossover is a separate question: archive parents always share a skeleton, so
// hybrid crossover cannot arise here regardless of the flag.)
func (e *MAPElitesEngine) mutate(g *genome.Genome) *genome.Genome {
	return MutateWith(g, e.rng, e.Seeds, e.Config.CrossSkeleton)
}

// totalStats returns total occupied cells and QD-score across all archives.
// Archives are visited in genome.AllSkeletons() order, then any remaining keys
// in ascending skeleton order: ranging over the map accumulated the QD floats in
// a per-process-random order, so the same seeded run reported last-bit-different
// QD scores (the identical defect fixed in cmd/darwindeck/experiment.go, where
// it flaked the determinism test).
func (e *MAPElitesEngine) totalStats() (int, float64) {
	occupied := 0
	qdScore := 0.0
	for _, skel := range e.archiveOrder() {
		archive := e.Archives[skel]
		occupied += archive.Occupied
		qdScore += archive.QDScore
	}
	return occupied, qdScore
}

// archiveOrder returns every archive key in a deterministic order: the fixed
// genome.AllSkeletons() list first, then any extra key insert() created for a
// skeleton not yet in that list, ascending.
func (e *MAPElitesEngine) archiveOrder() []genome.SkeletonType {
	out := make([]genome.SkeletonType, 0, len(e.Archives))
	seen := make(map[genome.SkeletonType]bool, len(e.Archives))
	for _, skel := range genome.AllSkeletons() {
		if _, ok := e.Archives[skel]; ok {
			out = append(out, skel)
			seen[skel] = true
		}
	}
	var extra []genome.SkeletonType
	for skel := range e.Archives {
		if !seen[skel] {
			extra = append(extra, skel)
		}
	}
	sort.Slice(extra, func(i, j int) bool { return extra[i] < extra[j] })
	return append(out, extra...)
}

// AllQualified returns the archive occupants that meet the FitnessFloor.
// Admission is floor-free (see insert), so the archive may hold sub-floor
// stepping stones; output keeps the floor so they are never published.
//
// Functionally identical genomes occupying multiple cells are deduplicated
// by outputHash, keeping the best-fitness occupant (Task 28 round 2: the
// flagship published clone groups under distinct IDs; Wave K fix 2 widened
// the key to ignore dead genes).
func (e *MAPElitesEngine) AllQualified() []*Individual {
	// Iterate skeletons in a stable order so seeded runs are reproducible;
	// Go map iteration order is randomized per process and would otherwise
	// shuffle the output across runs with the same seed.
	type entry struct {
		ind   *Individual
		order int
	}
	best := make(map[string]*entry)
	order := 0
	for _, skel := range genome.AllSkeletons() {
		archive, ok := e.Archives[skel]
		if !ok {
			continue
		}
		for r := 0; r < GridSize; r++ {
			for c := 0; c < GridSize; c++ {
				cell := archive.Cells[r][c]
				if cell == nil || cell.Individual.Fitness.TotalFitness < FitnessFloor {
					continue
				}
				hash := outputHash(cell.Individual.Genome)
				if cur, ok := best[hash]; ok {
					// Clone-group keep is by OutputRank, the commensurable
					// leaderboard key (Wave K fix 1).
					if cell.Individual.OutputRank() > cur.ind.OutputRank() {
						cur.ind = cell.Individual // keep first-seen order
					}
					continue
				}
				best[hash] = &entry{ind: cell.Individual, order: order}
				order++
			}
		}
	}

	entries := make([]*entry, 0, len(best))
	for _, en := range best {
		entries = append(entries, en)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].order < entries[j].order })
	result := make([]*Individual, 0, len(entries))
	for _, en := range entries {
		result = append(result, en.ind)
	}
	return result
}

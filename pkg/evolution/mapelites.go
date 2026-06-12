package evolution

import (
	"fmt"
	"math/rand/v2"
	"runtime"
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
	return &MAPElitesEngine{
		Config: config,
		Seeds:  seeds,
		Archives: map[genome.SkeletonType]*Archive{
			genome.Shedding:    {},
			genome.TrickTaking: {},
			genome.Rummy:       {},
		},
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
		g := Mutate(seed, e.rng, e.Seeds)
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
			child := Mutate(seed, e.rng, e.Seeds)
			child.ID = fmt.Sprintf("gen%d_%d", gen+1, e.rng.IntN(100000))
			child.Generation = gen + 1
			offspring = append(offspring, child)
			continue
		}

		var child *genome.Genome
		if e.rng.Float64() < 0.3 {
			// Crossover with another archive occupant of same skeleton
			parent2 := e.randomArchiveOccupantOfSkeleton(parent.Skeleton)
			if parent2 != nil {
				child = Crossover(parent, parent2, e.rng)
			}
		}
		if child == nil {
			child = Mutate(parent, e.rng, e.Seeds)
		} else {
			child = Mutate(child, e.rng, e.Seeds)
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

	if metrics.TotalFitness > e.BestFitness {
		e.BestFitness = metrics.TotalFitness
		e.BestGenome = g
	}
	return true
}

// randomArchiveOccupant returns a random genome from any occupied archive cell.
func (e *MAPElitesEngine) randomArchiveOccupant() *genome.Genome {
	// Iterate skeletons in a stable order so seeded runs are reproducible;
	// Go map iteration order is randomized per process and would otherwise
	// shuffle the index space passed to rng.IntN.
	var occupants []*genome.Genome
	for _, skel := range []genome.SkeletonType{genome.Shedding, genome.TrickTaking, genome.Rummy} {
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

// totalStats returns total occupied cells and QD-score across all archives.
func (e *MAPElitesEngine) totalStats() (int, float64) {
	occupied := 0
	qdScore := 0.0
	for _, archive := range e.Archives {
		occupied += archive.Occupied
		qdScore += archive.QDScore
	}
	return occupied, qdScore
}

// AllQualified returns the archive occupants that meet the FitnessFloor.
// Admission is floor-free (see insert), so the archive may hold sub-floor
// stepping stones; output keeps the floor so they are never published.
func (e *MAPElitesEngine) AllQualified() []*Individual {
	// Iterate skeletons in a stable order so seeded runs are reproducible;
	// Go map iteration order is randomized per process and would otherwise
	// shuffle the output across runs with the same seed.
	var result []*Individual
	for _, skel := range []genome.SkeletonType{genome.Shedding, genome.TrickTaking, genome.Rummy} {
		archive, ok := e.Archives[skel]
		if !ok {
			continue
		}
		for r := 0; r < GridSize; r++ {
			for c := 0; c < GridSize; c++ {
				if cell := archive.Cells[r][c]; cell != nil && cell.Individual.Fitness.TotalFitness >= FitnessFloor {
					result = append(result, cell.Individual)
				}
			}
		}
	}
	return result
}

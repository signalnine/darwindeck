package evolution

import (
	"encoding/json"
	"fmt"
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
	}
}

// Individual wraps a genome with its fitness evaluation.
type Individual struct {
	Genome  *genome.Genome
	Fitness fitness.Metrics
	Valid   bool
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
		g := Mutate(seed, e.rng, e.Seeds)
		g.ID = fmt.Sprintf("init_%d", i)
		g.Generation = 0
		e.Population[i] = &Individual{Genome: g}
	}
}

// EvaluatePopulation runs fitness evaluation on all individuals in parallel,
// then applies fitness sharing to reward niche diversity.
func (e *Engine) EvaluatePopulation() {
	var wg sync.WaitGroup
	sem := make(chan struct{}, e.Config.Workers)

	for i, ind := range e.Population {
		if ind.Valid {
			continue // Already evaluated (elite carried forward)
		}

		wg.Add(1)
		sem <- struct{}{} // Acquire worker slot

		go func(idx int, ind *Individual) {
			defer wg.Done()
			defer func() { <-sem }() // Release worker slot

			seed := e.Config.BaseSeed + uint64(e.Generation)*10000 + uint64(idx)
			result := fitness.Evaluate(ind.Genome, seed)

			ind.Valid = result.Valid
			ind.Fitness = result.Metrics
			if result.Valid {
				ind.Genome.Fitness = result.Metrics.TotalFitness
			}
		}(i, ind)
	}

	wg.Wait()

	// Apply fitness sharing: divide fitness by niche count.
	// Niches are defined by skeleton type. This prevents a single skeleton
	// from monopolizing the population.
	e.applyFitnessSharing()
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
		ind.Genome.Fitness = ind.Fitness.SharedFitness
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
	for _, ind := range e.Population {
		if ind.Valid && ind.Fitness.TotalFitness > e.BestFitness {
			e.BestFitness = ind.Fitness.TotalFitness
			e.BestGenome = ind.Genome
		}
	}

	nextGen := make([]*Individual, e.Config.PopulationSize)

	// Elitism: top N carry forward
	elite := min(e.Config.EliteSize, len(e.Population))
	for i := 0; i < elite; i++ {
		nextGen[i] = &Individual{
			Genome:  e.Population[i].Genome,
			Fitness: e.Population[i].Fitness,
			Valid:   true, // Skip re-evaluation
		}
	}

	// Fill rest via tournament selection + mutation/crossover
	for i := elite; i < e.Config.PopulationSize; i++ {
		parent := e.tournament()

		if e.rng.Float64() < 0.3 {
			// Crossover
			parent2 := e.tournament()
			child := Crossover(parent.Genome, parent2.Genome, e.rng)
			if child != nil {
				child = Mutate(child, e.rng, e.Seeds)
				nextGen[i] = &Individual{Genome: child}
				continue
			}
		}

		// Mutation only
		child := Mutate(parent.Genome, e.rng, e.Seeds)
		nextGen[i] = &Individual{Genome: child}
	}

	// Diversity: kill duplicate genomes in top 50
	e.dedup(nextGen)

	return nextGen
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
				child := Mutate(parent, e.rng, e.Seeds)
				pop[i].Genome = child
				pop[i].Valid = false
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

	// Final evaluation
	e.EvaluatePopulation()
}

// TopN returns the top N genomes ensuring skeleton diversity.
// Reserves slots per skeleton proportionally, then fills remaining with best overall.
func (e *Engine) TopN(n int) []*Individual {
	sort.Slice(e.Population, func(i, j int) bool {
		return e.Population[i].Fitness.TotalFitness > e.Population[j].Fitness.TotalFitness
	})

	// Reserve at least n/numSkeletons slots per skeleton type
	perSkeleton := n / 3
	if perSkeleton < 2 {
		perSkeleton = 2
	}

	used := make(map[int]bool)
	var result []*Individual

	// First pass: take top perSkeleton from each skeleton
	skeletonCounts := make(map[genome.SkeletonType]int)
	for i, ind := range e.Population {
		if !ind.Valid || ind.Genome == nil {
			continue
		}
		skel := ind.Genome.Skeleton
		if skeletonCounts[skel] < perSkeleton {
			result = append(result, ind)
			used[i] = true
			skeletonCounts[skel]++
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
		if !used[i] && ind.Valid && ind.Genome != nil {
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

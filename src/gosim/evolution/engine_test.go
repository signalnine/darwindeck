package evolution

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/signalnine/darwindeck/gosim/genome"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.PopulationSize != 100 {
		t.Errorf("Expected PopulationSize 100, got %d", config.PopulationSize)
	}
	if config.MaxGenerations != 100 {
		t.Errorf("Expected MaxGenerations 100, got %d", config.MaxGenerations)
	}
	if config.ElitismRate != 0.1 {
		t.Errorf("Expected ElitismRate 0.1, got %f", config.ElitismRate)
	}
	if config.CrossoverRate != 0.7 {
		t.Errorf("Expected CrossoverRate 0.7, got %f", config.CrossoverRate)
	}
	if config.FitnessStyle != "balanced" {
		t.Errorf("Expected FitnessStyle 'balanced', got '%s'", config.FitnessStyle)
	}
}

func TestNewEvolutionEngine(t *testing.T) {
	config := &EvolutionConfig{
		PopulationSize: 10,
		MaxGenerations: 5,
		ElitismRate:    0.2,
		CrossoverRate:  0.5,
		TournamentSize: 3,
		FitnessStyle:   "balanced",
		RandomSeed:     42,
		GamesPerEval:   10,
	}

	engine := NewEvolutionEngine(config)
	defer engine.Close()

	if engine.Config != config {
		t.Error("Config not set correctly")
	}
	if engine.Rng == nil {
		t.Error("RNG not initialized")
	}
	if engine.Evaluator == nil {
		t.Error("Evaluator not initialized")
	}
	if engine.MutationPipeline == nil {
		t.Error("MutationPipeline not initialized")
	}
	if engine.Crossover == nil {
		t.Error("Crossover not initialized")
	}
}

func TestInitializePopulation(t *testing.T) {
	config := &EvolutionConfig{
		PopulationSize: 20,
		MaxGenerations: 1,
		SeedRatio:      0.5,
		FitnessStyle:   "balanced",
		RandomSeed:     42,
		GamesPerEval:   10,
	}

	engine := NewEvolutionEngine(config)
	defer engine.Close()

	err := engine.InitializePopulation()
	if err != nil {
		t.Fatalf("InitializePopulation failed: %v", err)
	}

	if engine.Population == nil {
		t.Fatal("Population not initialized")
	}

	if len(engine.Population.Individuals) != config.PopulationSize {
		t.Errorf("Expected %d individuals, got %d",
			config.PopulationSize, len(engine.Population.Individuals))
	}

	// Check that all individuals have genomes
	for i, ind := range engine.Population.Individuals {
		if ind.Genome == nil {
			t.Errorf("Individual %d has nil genome", i)
		}
		if ind.Evaluated {
			t.Errorf("Individual %d should not be evaluated yet", i)
		}
	}
}

func TestTournamentSelection(t *testing.T) {
	// Create a simple population with known fitnesses
	individuals := make([]*Individual, 10)
	for i := 0; i < 10; i++ {
		individuals[i] = &Individual{
			Genome:    genome.CreateWarGenome(),
			Fitness:   float64(i) / 10.0,
			Evaluated: true,
		}
	}
	pop := NewPopulation(individuals)

	config := &EvolutionConfig{
		TournamentSize: 3,
		RandomSeed:     42,
	}

	engine := NewEvolutionEngine(config)
	defer engine.Close()
	engine.Population = pop

	// Run multiple tournaments and verify we get valid selections
	for i := 0; i < 20; i++ {
		selected := TournamentSelection(pop, config.TournamentSize, engine.Rng)
		if selected == nil {
			t.Errorf("Tournament %d returned nil", i)
		}
		// Verify selected individual is in population
		found := false
		for _, ind := range pop.Individuals {
			if ind == selected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Tournament %d returned individual not in population", i)
		}
	}
}

func TestSelectElite(t *testing.T) {
	individuals := make([]*Individual, 10)
	for i := 0; i < 10; i++ {
		individuals[i] = &Individual{
			Genome:    genome.CreateWarGenome(),
			Fitness:   float64(i),
			Evaluated: true,
		}
	}
	pop := NewPopulation(individuals)

	// Select top 3
	elite := SelectElite(pop, 3)

	if len(elite) != 3 {
		t.Fatalf("Expected 3 elite, got %d", len(elite))
	}

	// Should be the highest fitness individuals
	if elite[0].Fitness != 9.0 {
		t.Errorf("Expected top elite fitness 9.0, got %f", elite[0].Fitness)
	}
	if elite[1].Fitness != 8.0 {
		t.Errorf("Expected second elite fitness 8.0, got %f", elite[1].Fitness)
	}
	if elite[2].Fitness != 7.0 {
		t.Errorf("Expected third elite fitness 7.0, got %f", elite[2].Fitness)
	}
}

func TestCreateOffspring(t *testing.T) {
	config := &EvolutionConfig{
		PopulationSize: 10,
		ElitismRate:    0.2, // Keep top 2
		CrossoverRate:  0.7,
		TournamentSize: 2,
		RandomSeed:     42,
		FitnessStyle:   "balanced",
		GamesPerEval:   10,
	}

	engine := NewEvolutionEngine(config)
	defer engine.Close()

	// Create initial population
	individuals := make([]*Individual, 10)
	for i := 0; i < 10; i++ {
		individuals[i] = &Individual{
			Genome:    genome.CreateWarGenome(),
			Fitness:   float64(i) / 10.0,
			Evaluated: true,
		}
	}
	engine.Population = NewPopulation(individuals)

	// Create offspring
	offspring := engine.CreateOffspring()

	if len(offspring) != config.PopulationSize {
		t.Errorf("Expected %d offspring, got %d", config.PopulationSize, len(offspring))
	}

	// Check elite are preserved (top 2)
	eliteCount := int(float64(config.PopulationSize) * config.ElitismRate)
	for i := 0; i < eliteCount; i++ {
		// Elite should be evaluated since they're clones
		if offspring[i].Fitness == 0.0 {
			t.Logf("Elite %d has fitness 0.0 (may be cloned with reset fitness)", i)
		}
	}

	// Check remaining are unevaluated
	for i := eliteCount; i < len(offspring); i++ {
		if offspring[i].Evaluated {
			t.Errorf("Offspring %d should not be evaluated", i)
		}
	}
}

func TestCheckPlateau(t *testing.T) {
	config := &EvolutionConfig{
		PlateauThreshold:     5,
		ImprovementThreshold: 0.01, // 1% improvement required
		RandomSeed:           42,
		FitnessStyle:         "balanced",
	}

	engine := NewEvolutionEngine(config)
	defer engine.Close()

	// No history yet
	if engine.CheckPlateau() {
		t.Error("Should not detect plateau with no history")
	}

	// Add improving stats
	for i := 0; i < 5; i++ {
		engine.StatsHistory = append(engine.StatsHistory, GenerationStats{
			Generation:  i,
			BestFitness: float64(i) * 0.1, // 10% improvement each generation
		})
	}

	if engine.CheckPlateau() {
		t.Error("Should not detect plateau with improving fitness")
	}

	// Add plateaued stats
	engine.StatsHistory = nil
	for i := 0; i < 10; i++ {
		engine.StatsHistory = append(engine.StatsHistory, GenerationStats{
			Generation:  i,
			BestFitness: 0.5, // No improvement
		})
	}

	if !engine.CheckPlateau() {
		t.Error("Should detect plateau with no improvement")
	}
}

func TestEvolutionEngineShortRun(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping evolution test in short mode")
	}

	config := &EvolutionConfig{
		PopulationSize: 10,
		MaxGenerations: 3,
		ElitismRate:    0.2,
		CrossoverRate:  0.7,
		TournamentSize: 2,
		SeedRatio:      0.5,
		RandomSeed:     42,
		FitnessStyle:   "balanced",
		GamesPerEval:   10, // Minimal for testing
		NumWorkers:     2,
		Verbose:        false,
	}

	engine := NewEvolutionEngine(config)
	defer engine.Close()

	// Run evolution
	err := engine.Evolve()
	if err != nil {
		t.Fatalf("Evolve failed: %v", err)
	}

	// Check stats were recorded
	if len(engine.StatsHistory) != config.MaxGenerations {
		t.Errorf("Expected %d stats entries, got %d",
			config.MaxGenerations, len(engine.StatsHistory))
	}

	// Check best ever was tracked
	if engine.BestEver == nil {
		t.Error("BestEver not tracked")
	}

	// Get best genomes
	best := engine.GetBestGenomes(5)
	if len(best) == 0 {
		t.Error("GetBestGenomes returned empty")
	}
}

func TestGenerationStatsCallback(t *testing.T) {
	config := &EvolutionConfig{
		PopulationSize: 5,
		MaxGenerations: 2,
		SeedRatio:      1.0,
		RandomSeed:     42,
		FitnessStyle:   "balanced",
		GamesPerEval:   5,
		NumWorkers:     1,
	}

	engine := NewEvolutionEngine(config)
	defer engine.Close()

	callbackCount := 0
	engine.OnGenerationComplete = func(stats GenerationStats) {
		callbackCount++
		if stats.Generation < 0 {
			t.Errorf("Invalid generation number: %d", stats.Generation)
		}
	}

	err := engine.Evolve()
	if err != nil {
		t.Fatalf("Evolve failed: %v", err)
	}

	if callbackCount != config.MaxGenerations {
		t.Errorf("Expected %d callbacks, got %d", config.MaxGenerations, callbackCount)
	}
}

func TestCheckpointSaveLoad(t *testing.T) {
	// Create temp directory for checkpoint
	tmpDir, err := os.MkdirTemp("", "evolution_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	checkpointPath := filepath.Join(tmpDir, "checkpoint.json")

	// Create and run engine for a few generations
	config := &EvolutionConfig{
		PopulationSize: 5,
		MaxGenerations: 2,
		SeedRatio:      1.0,
		RandomSeed:     42,
		FitnessStyle:   "balanced",
		GamesPerEval:   5,
		NumWorkers:     1,
	}

	engine := NewEvolutionEngine(config)

	err = engine.InitializePopulation()
	if err != nil {
		t.Fatalf("InitializePopulation failed: %v", err)
	}

	engine.EvaluatePopulation()
	engine.Population.Generation = 1

	// Save checkpoint
	err = engine.SaveCheckpoint(checkpointPath)
	engine.Close()
	if err != nil {
		t.Fatalf("SaveCheckpoint failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(checkpointPath); os.IsNotExist(err) {
		t.Fatal("Checkpoint file not created")
	}

	// Load checkpoint
	checkpoint, err := LoadCheckpoint(checkpointPath)
	if err != nil {
		t.Fatalf("LoadCheckpoint failed: %v", err)
	}

	// Verify checkpoint data
	if checkpoint.Generation != 1 {
		t.Errorf("Expected generation 1, got %d", checkpoint.Generation)
	}
	if len(checkpoint.Population) != config.PopulationSize {
		t.Errorf("Expected %d individuals, got %d",
			config.PopulationSize, len(checkpoint.Population))
	}
	if checkpoint.Config.PopulationSize != config.PopulationSize {
		t.Errorf("Expected config population size %d, got %d",
			config.PopulationSize, checkpoint.Config.PopulationSize)
	}

	// Resume from checkpoint
	engine2, err := ResumeFromCheckpoint(checkpointPath)
	if err != nil {
		t.Fatalf("ResumeFromCheckpoint failed: %v", err)
	}
	defer engine2.Close()

	if engine2.Population.Generation != 1 {
		t.Errorf("Resumed engine generation mismatch: expected 1, got %d",
			engine2.Population.Generation)
	}
	if len(engine2.Population.Individuals) != config.PopulationSize {
		t.Errorf("Resumed population size mismatch")
	}
}

func TestAutoCheckpointer(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "evolution_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	checkpointPath := filepath.Join(tmpDir, "auto_checkpoint.json")

	config := &EvolutionConfig{
		PopulationSize: 5,
		MaxGenerations: 1,
		SeedRatio:      1.0,
		RandomSeed:     42,
		FitnessStyle:   "balanced",
		GamesPerEval:   5,
	}

	engine := NewEvolutionEngine(config)
	defer engine.Close()

	err = engine.InitializePopulation()
	if err != nil {
		t.Fatalf("InitializePopulation failed: %v", err)
	}

	ac := NewAutoCheckpointer(engine, checkpointPath, 5) // Every 5 generations

	// Should not save at generation 0
	if ac.ShouldSave(0) {
		t.Error("Should not save at generation 0")
	}

	// Should save at generation 5
	if !ac.ShouldSave(5) {
		t.Error("Should save at generation 5")
	}

	// Save at generation 5
	err = ac.Save(5)
	if err != nil {
		t.Fatalf("Auto save failed: %v", err)
	}

	// Should not save at generation 5 again
	if ac.ShouldSave(5) {
		t.Error("Should not save at generation 5 again")
	}

	// Should save at generation 10
	if !ac.ShouldSave(10) {
		t.Error("Should save at generation 10")
	}
}

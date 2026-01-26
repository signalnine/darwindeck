package evolution

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/signalnine/darwindeck/gosim/evolution/fitness"
	"github.com/signalnine/darwindeck/gosim/genome"
)

// CheckpointData represents the serializable state of an evolution run.
type CheckpointData struct {
	// Configuration
	Config *EvolutionConfig `json:"config"`

	// Current state
	Generation   int               `json:"generation"`
	Population   []IndividualData  `json:"population"`
	BestEver     *IndividualData   `json:"best_ever,omitempty"`
	StatsHistory []GenerationStats `json:"stats_history"`

	// Metadata
	Timestamp   time.Time `json:"timestamp"`
	RNGSeed     int64     `json:"rng_seed"`
	Version     string    `json:"version"`
}

// IndividualData represents a serializable individual.
type IndividualData struct {
	Genome         *genome.GameGenome     `json:"genome"`
	Fitness        float64                `json:"fitness"`
	Evaluated      bool                   `json:"evaluated"`
	FitnessMetrics *fitness.FitnessMetrics `json:"fitness_metrics,omitempty"`
}

// CheckpointVersion is the current checkpoint format version.
const CheckpointVersion = "1.0"

// SaveCheckpoint saves the current evolution state to a file.
func (e *EvolutionEngine) SaveCheckpoint(path string) error {
	if e.Population == nil {
		return fmt.Errorf("no population to save")
	}

	// Convert population to serializable format
	popData := make([]IndividualData, len(e.Population.Individuals))
	for i, ind := range e.Population.Individuals {
		popData[i] = IndividualData{
			Genome:         ind.Genome,
			Fitness:        ind.Fitness,
			Evaluated:      ind.Evaluated,
			FitnessMetrics: ind.FitnessMetrics,
		}
	}

	// Convert best ever
	var bestData *IndividualData
	if e.BestEver != nil {
		bestData = &IndividualData{
			Genome:         e.BestEver.Genome,
			Fitness:        e.BestEver.Fitness,
			Evaluated:      e.BestEver.Evaluated,
			FitnessMetrics: e.BestEver.FitnessMetrics,
		}
	}

	checkpoint := CheckpointData{
		Config:       e.Config,
		Generation:   e.Population.Generation,
		Population:   popData,
		BestEver:     bestData,
		StatsHistory: e.StatsHistory,
		Timestamp:    time.Now(),
		RNGSeed:      e.Config.RandomSeed,
		Version:      CheckpointVersion,
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create checkpoint directory: %w", err)
	}

	// Write to temp file first, then rename (atomic)
	tempPath := path + ".tmp"
	data, err := json.MarshalIndent(checkpoint, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal checkpoint: %w", err)
	}

	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write checkpoint: %w", err)
	}

	if err := os.Rename(tempPath, path); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to finalize checkpoint: %w", err)
	}

	return nil
}

// LoadCheckpoint loads evolution state from a checkpoint file.
func LoadCheckpoint(path string) (*CheckpointData, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read checkpoint: %w", err)
	}

	var checkpoint CheckpointData
	if err := json.Unmarshal(data, &checkpoint); err != nil {
		return nil, fmt.Errorf("failed to unmarshal checkpoint: %w", err)
	}

	return &checkpoint, nil
}

// RestoreFromCheckpoint restores engine state from checkpoint data.
func (e *EvolutionEngine) RestoreFromCheckpoint(checkpoint *CheckpointData) error {
	if checkpoint == nil {
		return fmt.Errorf("nil checkpoint")
	}

	// Restore configuration (partial - some fields may need to stay as-is)
	if checkpoint.Config != nil {
		e.Config.PopulationSize = checkpoint.Config.PopulationSize
		e.Config.MaxGenerations = checkpoint.Config.MaxGenerations
		e.Config.ElitismRate = checkpoint.Config.ElitismRate
		e.Config.CrossoverRate = checkpoint.Config.CrossoverRate
		e.Config.TournamentSize = checkpoint.Config.TournamentSize
		e.Config.PlateauThreshold = checkpoint.Config.PlateauThreshold
		e.Config.ImprovementThreshold = checkpoint.Config.ImprovementThreshold
		e.Config.DiversityThreshold = checkpoint.Config.DiversityThreshold
		e.Config.FitnessStyle = checkpoint.Config.FitnessStyle
		e.Config.GamesPerEval = checkpoint.Config.GamesPerEval
		e.Config.UseMCTS = checkpoint.Config.UseMCTS
	}

	// Restore population
	individuals := make([]*Individual, len(checkpoint.Population))
	for i, data := range checkpoint.Population {
		individuals[i] = &Individual{
			Genome:         data.Genome,
			Fitness:        data.Fitness,
			Evaluated:      data.Evaluated,
			FitnessMetrics: data.FitnessMetrics,
		}
	}
	e.Population = NewPopulation(individuals)
	e.Population.Generation = checkpoint.Generation

	// Restore best ever
	if checkpoint.BestEver != nil {
		e.BestEver = &Individual{
			Genome:         checkpoint.BestEver.Genome,
			Fitness:        checkpoint.BestEver.Fitness,
			Evaluated:      checkpoint.BestEver.Evaluated,
			FitnessMetrics: checkpoint.BestEver.FitnessMetrics,
		}
	}

	// Restore stats history
	e.StatsHistory = checkpoint.StatsHistory

	return nil
}

// ResumeFromCheckpoint creates a new engine and restores state from a checkpoint.
func ResumeFromCheckpoint(path string) (*EvolutionEngine, error) {
	checkpoint, err := LoadCheckpoint(path)
	if err != nil {
		return nil, err
	}

	// Create engine with checkpoint config
	engine := NewEvolutionEngine(checkpoint.Config)

	// Restore state
	if err := engine.RestoreFromCheckpoint(checkpoint); err != nil {
		engine.Close()
		return nil, err
	}

	return engine, nil
}

// AutoCheckpointer provides automatic checkpoint saving.
type AutoCheckpointer struct {
	Engine     *EvolutionEngine
	Path       string
	Interval   int  // Save every N generations
	LastSaved  int  // Last generation saved
}

// NewAutoCheckpointer creates an auto-checkpointer.
func NewAutoCheckpointer(engine *EvolutionEngine, path string, interval int) *AutoCheckpointer {
	return &AutoCheckpointer{
		Engine:   engine,
		Path:     path,
		Interval: interval,
		LastSaved: -1,
	}
}

// ShouldSave returns true if it's time to save a checkpoint.
func (ac *AutoCheckpointer) ShouldSave(generation int) bool {
	if ac.Interval <= 0 {
		return false
	}
	// Don't save at generation 0, only at interval boundaries (5, 10, 15, etc.)
	if generation == 0 {
		return false
	}
	return generation > ac.LastSaved && generation%ac.Interval == 0
}

// Save saves a checkpoint if needed.
func (ac *AutoCheckpointer) Save(generation int) error {
	if !ac.ShouldSave(generation) {
		return nil
	}

	if err := ac.Engine.SaveCheckpoint(ac.Path); err != nil {
		return err
	}

	ac.LastSaved = generation
	return nil
}

// SaveFinal saves a final checkpoint regardless of interval.
func (ac *AutoCheckpointer) SaveFinal() error {
	return ac.Engine.SaveCheckpoint(ac.Path)
}

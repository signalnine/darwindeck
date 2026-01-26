package fitness

import "github.com/signalnine/darwindeck/gosim/genome"

// StylePresets defines weight configurations for different game styles.
// IMPORTANT: Rules complexity is heavily weighted because complex games
// don't get played - people won't learn rules they can't quickly understand.
var StylePresets = map[string]map[string]float64{
	"balanced": {
		// Balanced preset: games need meaningful decisions AND be learnable
		"decision_density":      0.25, // PRIMARY - no decisions = not a game
		"skill_vs_luck":         0.20, // Skill should matter
		"rules_complexity":      0.18, // Learnable but not dominant
		"comeback_potential":    0.12, // Games should feel winnable
		"interaction_frequency": 0.10, // Social element
		"tension_curve":         0.08, // Nice to have drama
		"bluffing_depth":        0.00,
		"betting_engagement":    0.07,
	},
	"bluffing": {
		// Bluffing games can be slightly more complex, but still need to be learnable
		"rules_complexity":      0.35,
		"decision_density":      0.05,
		"comeback_potential":    0.05,
		"tension_curve":         0.05,
		"interaction_frequency": 0.08,
		"skill_vs_luck":         0.05,
		"bluffing_depth":        0.18, // Quality bluffing mechanics
		"betting_engagement":    0.19, // Betting psychology
	},
	"strategic": {
		// Strategy gamers tolerate MORE complexity, but it still matters a lot
		"rules_complexity":      0.30, // Lower than others, but still significant
		"decision_density":      0.20,
		"comeback_potential":    0.08,
		"tension_curve":         0.05,
		"interaction_frequency": 0.10,
		"skill_vs_luck":         0.27, // High skill emphasis
		"bluffing_depth":        0.00,
		"betting_engagement":    0.00,
	},
	"party": {
		// Party games MUST be dead simple - complexity is the killer
		"rules_complexity":      0.50, // Half of fitness! Must explain in 1-2 minutes
		"decision_density":      0.04,
		"comeback_potential":    0.12, // Everyone can win
		"tension_curve":         0.06,
		"interaction_frequency": 0.14, // High interaction
		"skill_vs_luck":         0.04, // Luck-friendly
		"bluffing_depth":        0.00,
		"betting_engagement":    0.10,
	},
	"trick-taking": {
		// Trick-taking is familiar, so complexity is less of a barrier
		"rules_complexity":      0.30, // Familiar pattern helps, but still important
		"decision_density":      0.15,
		"comeback_potential":    0.10,
		"tension_curve":         0.12,
		"interaction_frequency": 0.18,
		"skill_vs_luck":         0.15,
		"bluffing_depth":        0.00,
		"betting_engagement":    0.00,
	},
}

// Evaluator evaluates game fitness using configurable weights.
type Evaluator struct {
	weights map[string]float64
	style   string
	cache   map[string]*FitnessMetrics
}

// NewEvaluator creates a new fitness evaluator.
// If style is provided and valid, uses that preset.
// If weights is provided, uses those (overrides style).
// Otherwise defaults to "balanced" preset.
func NewEvaluator(style string, weights map[string]float64) *Evaluator {
	var finalWeights map[string]float64
	finalStyle := style

	if weights != nil {
		finalWeights = copyWeights(weights)
		finalStyle = "custom"
	} else if preset, ok := StylePresets[style]; ok {
		finalWeights = copyWeights(preset)
	} else {
		finalWeights = copyWeights(StylePresets["balanced"])
		finalStyle = "balanced"
	}

	// Normalize weights to sum to 1.0
	totalWeight := 0.0
	for _, w := range finalWeights {
		totalWeight += w
	}
	for k := range finalWeights {
		finalWeights[k] /= totalWeight
	}

	return &Evaluator{
		weights: finalWeights,
		style:   finalStyle,
		cache:   make(map[string]*FitnessMetrics),
	}
}

// Style returns the current style preset name.
func (e *Evaluator) Style() string {
	return e.style
}

// Weights returns a copy of the current weights.
func (e *Evaluator) Weights() map[string]float64 {
	return copyWeights(e.weights)
}

// Evaluate computes fitness metrics for a genome given simulation results.
func (e *Evaluator) Evaluate(g *genome.GameGenome, results *SimulationResults) *FitnessMetrics {
	return ComputeMetrics(g, results, e.weights, e.style)
}

// ClearCache clears the fitness cache.
func (e *Evaluator) ClearCache() {
	e.cache = make(map[string]*FitnessMetrics)
}

func copyWeights(w map[string]float64) map[string]float64 {
	result := make(map[string]float64, len(w))
	for k, v := range w {
		result[k] = v
	}
	return result
}

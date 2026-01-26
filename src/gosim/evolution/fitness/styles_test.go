package fitness

import (
	"math"
	"testing"

	"github.com/signalnine/darwindeck/gosim/genome"
)

func TestNewEvaluatorDefault(t *testing.T) {
	evaluator := NewEvaluator("", nil)

	if evaluator.Style() != "balanced" {
		t.Errorf("Expected default style 'balanced', got '%s'", evaluator.Style())
	}

	weights := evaluator.Weights()
	if weights["decision_density"] == 0 {
		t.Error("Expected decision_density weight to be set")
	}
}

func TestNewEvaluatorWithStyle(t *testing.T) {
	evaluator := NewEvaluator("strategic", nil)

	if evaluator.Style() != "strategic" {
		t.Errorf("Expected style 'strategic', got '%s'", evaluator.Style())
	}

	weights := evaluator.Weights()
	// Strategic should have high skill_vs_luck weight
	if weights["skill_vs_luck"] < 0.2 {
		t.Error("Expected high skill_vs_luck weight for strategic style")
	}
}

func TestNewEvaluatorWithCustomWeights(t *testing.T) {
	customWeights := map[string]float64{
		"decision_density":      0.50,
		"skill_vs_luck":         0.50,
		"rules_complexity":      0.00,
		"comeback_potential":    0.00,
		"interaction_frequency": 0.00,
		"tension_curve":         0.00,
		"bluffing_depth":        0.00,
		"betting_engagement":    0.00,
	}

	evaluator := NewEvaluator("", customWeights)

	if evaluator.Style() != "custom" {
		t.Errorf("Expected style 'custom', got '%s'", evaluator.Style())
	}

	weights := evaluator.Weights()
	// Should be normalized
	total := 0.0
	for _, w := range weights {
		total += w
	}
	if math.Abs(total-1.0) > 0.001 {
		t.Errorf("Expected weights to sum to 1.0, got %f", total)
	}
}

func TestNewEvaluatorInvalidStyle(t *testing.T) {
	evaluator := NewEvaluator("nonexistent", nil)

	// Should fall back to balanced
	if evaluator.Style() != "balanced" {
		t.Errorf("Expected fallback to 'balanced', got '%s'", evaluator.Style())
	}
}

func TestStylePresetsExist(t *testing.T) {
	expectedStyles := []string{"balanced", "bluffing", "strategic", "party", "trick-taking"}

	for _, style := range expectedStyles {
		if _, ok := StylePresets[style]; !ok {
			t.Errorf("Missing style preset: %s", style)
		}
	}
}

func TestStylePresetsNormalized(t *testing.T) {
	for style, weights := range StylePresets {
		total := 0.0
		for _, w := range weights {
			total += w
		}
		if math.Abs(total-1.0) > 0.01 {
			t.Errorf("Style '%s' weights sum to %f, expected ~1.0", style, total)
		}
	}
}

func TestStylePresetsHaveAllMetrics(t *testing.T) {
	requiredMetrics := []string{
		"decision_density",
		"skill_vs_luck",
		"rules_complexity",
		"comeback_potential",
		"interaction_frequency",
		"tension_curve",
		"bluffing_depth",
		"betting_engagement",
	}

	for style, weights := range StylePresets {
		for _, metric := range requiredMetrics {
			if _, ok := weights[metric]; !ok {
				t.Errorf("Style '%s' missing metric '%s'", style, metric)
			}
		}
	}
}

func TestEvaluateGenome(t *testing.T) {
	evaluator := NewEvaluator("balanced", nil)
	g := genome.CreateWarGenome()

	results := &SimulationResults{
		TotalGames:  100,
		Wins:        []int{50, 50},
		PlayerCount: 2,
		AvgTurns:    52.0,
	}

	metrics := evaluator.Evaluate(g, results)

	if !metrics.Valid {
		t.Error("Expected valid metrics")
	}

	if metrics.TotalFitness < 0 || metrics.TotalFitness > 1 {
		t.Errorf("Expected fitness in [0,1], got %f", metrics.TotalFitness)
	}
}

func TestEvaluateDifferentStyles(t *testing.T) {
	g := genome.CreateWarGenome()
	results := &SimulationResults{
		TotalGames:  100,
		Wins:        []int{50, 50},
		PlayerCount: 2,
		AvgTurns:    52.0,
	}

	styles := []string{"balanced", "party", "strategic"}
	fitnesses := make(map[string]float64)

	for _, style := range styles {
		evaluator := NewEvaluator(style, nil)
		metrics := evaluator.Evaluate(g, results)
		fitnesses[style] = metrics.TotalFitness
	}

	// Different styles should give different results
	// (at least some should differ)
	allSame := true
	first := fitnesses["balanced"]
	for _, f := range fitnesses {
		if math.Abs(f-first) > 0.001 {
			allSame = false
			break
		}
	}

	if allSame {
		t.Error("Expected different styles to produce different fitness values")
	}
}

func TestClearCache(t *testing.T) {
	evaluator := NewEvaluator("balanced", nil)

	// Shouldn't panic
	evaluator.ClearCache()
}

func TestPartyStylePrefersSimple(t *testing.T) {
	// Party style should weight rules_complexity highly
	partyWeights := StylePresets["party"]

	if partyWeights["rules_complexity"] < 0.4 {
		t.Errorf("Expected party style to heavily weight rules_complexity, got %f",
			partyWeights["rules_complexity"])
	}

	if partyWeights["skill_vs_luck"] > 0.1 {
		t.Errorf("Expected party style to minimize skill_vs_luck, got %f",
			partyWeights["skill_vs_luck"])
	}
}

func TestStrategicStylePrefersSkill(t *testing.T) {
	strategicWeights := StylePresets["strategic"]

	if strategicWeights["skill_vs_luck"] < 0.2 {
		t.Errorf("Expected strategic style to weight skill_vs_luck highly, got %f",
			strategicWeights["skill_vs_luck"])
	}
}

func TestBluffingStylePrefersBluffing(t *testing.T) {
	bluffingWeights := StylePresets["bluffing"]

	if bluffingWeights["bluffing_depth"] < 0.1 {
		t.Errorf("Expected bluffing style to weight bluffing_depth, got %f",
			bluffingWeights["bluffing_depth"])
	}

	if bluffingWeights["betting_engagement"] < 0.1 {
		t.Errorf("Expected bluffing style to weight betting_engagement, got %f",
			bluffingWeights["betting_engagement"])
	}
}

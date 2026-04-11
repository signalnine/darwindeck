package fitness

import (
	"math"

	"github.com/darwindeck/darwindeck/pkg/sim"
)

// Metrics holds the 5 fitness metrics, each 0.0-1.0.
type Metrics struct {
	MeaningfulDecisions float64 // Weight 0.25
	GameArc             float64 // Weight 0.25
	Interaction         float64 // Weight 0.20
	SkillGradient       float64 // Weight 0.20
	SessionLength       float64 // Weight 0.10
	TotalFitness        float64
}

const (
	WeightDecisions = 0.25
	WeightArc       = 0.25
	WeightInteract  = 0.20
	WeightSkill     = 0.20
	WeightLength    = 0.10
)

// ComputeFitness calculates all 5 metrics from simulation results.
func ComputeFitness(
	randomResult sim.BatchResult,
	greedyResult sim.BatchResult,
	numPlayers int,
) Metrics {
	m := Metrics{
		MeaningfulDecisions: computeDecisionDensity(randomResult),
		GameArc:             computeGameArc(randomResult),
		Interaction:         computeInteraction(randomResult),
		SkillGradient:       computeSkillGradient(randomResult, greedyResult, numPlayers),
		SessionLength:       computeSessionLength(randomResult),
	}

	m.TotalFitness = m.MeaningfulDecisions*WeightDecisions +
		m.GameArc*WeightArc +
		m.Interaction*WeightInteract +
		m.SkillGradient*WeightSkill +
		m.SessionLength*WeightLength

	return m
}

// computeDecisionDensity measures the fraction of turns with >1 legal move
// that resulted in different events (proxy for "choice mattered").
// Analyzed from event logs.
func computeDecisionDensity(result sim.BatchResult) float64 {
	if len(result.AllEvents) == 0 {
		return 0
	}

	totalDecisions := 0
	meaningfulDecisions := 0

	for _, events := range result.AllEvents {
		// Count play events (decisions) vs draw/pass events (forced)
		plays := 0
		draws := 0
		for _, e := range events {
			switch e.Type {
			case sim.EventCardPlayed:
				plays++
			case sim.EventCardDrawn:
				draws++
			}
		}

		total := plays + draws
		if total == 0 {
			continue
		}

		// Decision density: fraction of turns that were plays (not forced draws)
		totalDecisions += total
		meaningfulDecisions += plays
	}

	if totalDecisions == 0 {
		return 0
	}

	density := float64(meaningfulDecisions) / float64(totalDecisions)
	return clamp(density, 0, 1)
}

// computeGameArc measures whether the game has a proper narrative arc:
// uncertainty early → convergence → resolution.
// We use the variance of win distribution across the game as a proxy.
func computeGameArc(result sim.BatchResult) float64 {
	if result.Completions == 0 {
		return 0
	}

	numPlayers := len(result.WinCounts)
	if numPlayers == 0 {
		return 0
	}

	// Measure how evenly wins are distributed (high entropy = good arc)
	// A game where anyone can win has better arc than a deterministic one
	totalWins := 0
	for _, w := range result.WinCounts {
		totalWins += w
	}
	if totalWins == 0 {
		return 0
	}

	// Shannon entropy of win distribution, normalized to [0,1]
	maxEntropy := math.Log2(float64(numPlayers))
	if maxEntropy == 0 {
		return 0
	}

	entropy := 0.0
	for _, w := range result.WinCounts {
		if w == 0 {
			continue
		}
		p := float64(w) / float64(totalWins)
		entropy -= p * math.Log2(p)
	}

	// Also consider turn variance: games should have varying length
	// (different games play out differently = good arc)
	turnVariance := computeTurnVariance(result)
	turnScore := clamp(turnVariance/100, 0, 1) // Normalize: 100 variance = max

	// Combine entropy (who wins varies) with turn variance (how it plays varies)
	entropyScore := entropy / maxEntropy
	return clamp(entropyScore*0.6+turnScore*0.4, 0, 1)
}

func computeTurnVariance(result sim.BatchResult) float64 {
	if len(result.TurnsList) < 2 {
		return 0
	}

	mean := result.AvgTurns
	sumSq := 0.0
	for _, t := range result.TurnsList {
		diff := float64(t) - mean
		sumSq += diff * diff
	}
	return sumSq / float64(len(result.TurnsList))
}

// computeInteraction measures how much players' actions affect each other.
// We use the ratio of special-triggered events (skips, draws affecting others)
// to total events as a proxy.
func computeInteraction(result sim.BatchResult) float64 {
	if len(result.AllEvents) == 0 {
		return 0
	}

	totalEvents := 0
	interactionEvents := 0

	for _, events := range result.AllEvents {
		for _, e := range events {
			totalEvents++
			switch e.Type {
			case sim.EventSpecialTriggered:
				interactionEvents++
			case sim.EventTrickWon:
				// Tricks are inherently interactive (every player plays)
				interactionEvents++
			case sim.EventMeldLaid:
				// Melds don't directly interact unless lay-off
			case sim.EventCardPlayed:
				// Cards played to shared areas (discard, trick) are interactive
				if e.Detail == "discard" {
					interactionEvents++ // Changes what opponent can draw
				}
			}
		}
	}

	if totalEvents == 0 {
		return 0
	}

	ratio := float64(interactionEvents) / float64(totalEvents)
	// Scale: 0.3+ interaction ratio = good
	return clamp(ratio/0.3, 0, 1)
}

// computeSkillGradient measures whether better play leads to better results.
// Compares greedy AI win rate vs random AI expected win rate.
func computeSkillGradient(randomResult, greedyResult sim.BatchResult, numPlayers int) float64 {
	if greedyResult.Completions == 0 || numPlayers == 0 {
		return 0
	}

	expectedWR := 1.0 / float64(numPlayers) // Random baseline

	// In greedy games, player 0 uses greedy AI, rest use random.
	// Greedy win rate = player 0's wins / total completions.
	greedyWR := 0.0
	if greedyResult.Completions > 0 && len(greedyResult.WinCounts) > 0 {
		greedyWR = float64(greedyResult.WinCounts[0]) / float64(greedyResult.Completions)
	}

	// Skill = how much better greedy does vs random baseline
	// Cap at 1.0: greedy winning 100% vs 50% expected = skill 1.0
	skillDiff := greedyWR - expectedWR
	if skillDiff < 0 {
		skillDiff = 0 // Greedy worse than random = no skill signal
	}

	// Normalize: a greedy player winning 2x the expected rate = perfect skill
	maxDiff := 1.0 - expectedWR
	if maxDiff == 0 {
		return 0
	}

	return clamp(skillDiff/maxDiff, 0, 1)
}

// computeSessionLength scores the average game length against target range.
// Target: 15-40 turns. Linear falloff outside. Below 5 or above 100 = 0.
func computeSessionLength(result sim.BatchResult) float64 {
	avg := result.AvgTurns
	if avg < 5 || avg > 100 {
		return 0
	}
	if avg >= 15 && avg <= 40 {
		return 1.0
	}
	if avg < 15 {
		return (avg - 5) / 10 // Linear from 5→15
	}
	// avg > 40
	return (100 - avg) / 60 // Linear from 40→100
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

package fitness

import (
	"testing"

	"github.com/signalnine/darwindeck/gosim/genome"
)

func TestComputeMetricsBasic(t *testing.T) {
	g := genome.CreateWarGenome()
	results := &SimulationResults{
		TotalGames:  100,
		Wins:        []int{50, 50},
		PlayerCount: 2,
		Draws:       0,
		AvgTurns:    52.0,
		Errors:      0,
	}

	weights := StylePresets["balanced"]
	metrics := ComputeMetrics(g, results, weights, "balanced")

	if !metrics.Valid {
		t.Error("Expected valid metrics")
	}

	if metrics.GamesSimulated != 100 {
		t.Errorf("Expected 100 games simulated, got %d", metrics.GamesSimulated)
	}

	// Balanced wins should have high comeback potential
	if metrics.ComebackPotential < 0.5 {
		t.Errorf("Expected high comeback potential for balanced wins, got %f", metrics.ComebackPotential)
	}
}

func TestComputeMetricsWithErrors(t *testing.T) {
	g := genome.CreateWarGenome()
	results := &SimulationResults{
		TotalGames:  100,
		Wins:        []int{10, 10},
		PlayerCount: 2,
		Draws:       0,
		AvgTurns:    10.0,
		Errors:      60, // More than half are errors
	}

	weights := StylePresets["balanced"]
	metrics := ComputeMetrics(g, results, weights, "balanced")

	if metrics.Valid {
		t.Error("Expected invalid metrics due to high error rate")
	}

	if metrics.TotalFitness != 0.0 {
		t.Errorf("Expected zero fitness for invalid metrics, got %f", metrics.TotalFitness)
	}
}

func TestComputeMetricsOneSided(t *testing.T) {
	g := genome.CreateWarGenome()
	results := &SimulationResults{
		TotalGames:  100,
		Wins:        []int{90, 10}, // Very one-sided
		PlayerCount: 2,
		Draws:       0,
		AvgTurns:    52.0,
		Errors:      0,
	}

	weights := StylePresets["balanced"]
	metrics := ComputeMetrics(g, results, weights, "balanced")

	// One-sided games should have lower comeback potential
	if metrics.ComebackPotential > 0.5 {
		t.Errorf("Expected low comeback potential for one-sided game, got %f", metrics.ComebackPotential)
	}

	// Should have quality penalty applied
	if metrics.TotalFitness > 0.5 {
		t.Errorf("Expected reduced fitness due to one-sidedness, got %f", metrics.TotalFitness)
	}
}

func TestDecisionDensityHeuristic(t *testing.T) {
	// Game with multiple phases and conditions should have higher decision density
	g := &genome.GameGenome{
		Name: "Test",
		TurnStructure: genome.TurnStructure{
			Phases: []genome.Phase{
				&genome.DrawPhase{Source: genome.LocationDeck, Count: 1, Mandatory: false},
				&genome.PlayPhase{Target: genome.LocationDiscard, Mandatory: false},
				&genome.PlayPhase{Target: genome.LocationTableau, Mandatory: false},
			},
			MaxTurns: 100,
		},
		WinConditions: []genome.WinCondition{
			{Type: genome.WinTypeEmptyHand},
		},
	}

	results := &SimulationResults{
		TotalGames:  100,
		Wins:        []int{50, 50},
		PlayerCount: 2,
		AvgTurns:    50.0,
	}

	density := computeDecisionDensity(g, results)

	// Multiple optional phases should give decent decision density
	if density < 0.3 {
		t.Errorf("Expected higher decision density for multi-phase game, got %f", density)
	}
}

func TestComebackPotentialBalanced(t *testing.T) {
	results := &SimulationResults{
		TotalGames:  100,
		Wins:        []int{50, 50},
		PlayerCount: 2,
		Draws:       0,
	}

	comeback := computeComebackPotential(results)

	// Perfectly balanced should have high comeback potential
	if comeback < 0.8 {
		t.Errorf("Expected high comeback potential for 50/50 wins, got %f", comeback)
	}
}

func TestComebackPotentialUnbalanced(t *testing.T) {
	results := &SimulationResults{
		TotalGames:  100,
		Wins:        []int{95, 5},
		PlayerCount: 2,
		Draws:       0,
	}

	comeback := computeComebackPotential(results)

	// Very unbalanced should have low comeback potential
	if comeback > 0.4 {
		t.Errorf("Expected low comeback potential for 95/5 wins, got %f", comeback)
	}
}

func TestTensionCurveBetting(t *testing.T) {
	results := &SimulationResults{
		TotalGames:   100,
		TotalBets:    300, // Betting game
		AllInCount:   20,
		ShowdownWins: 60,
		Draws:        10,
		Errors:       0,
	}

	tension := computeTensionCurve(results)

	// Betting game with activity should have decent tension
	if tension < 0.3 {
		t.Errorf("Expected reasonable tension for betting game, got %f", tension)
	}
}

func TestBluffingDepthClaims(t *testing.T) {
	results := &SimulationResults{
		TotalClaims:      100,
		TotalBluffs:      60,  // 60% bluff rate (near ideal)
		TotalChallenges:  40,  // 40% challenge rate (near ideal)
		SuccessfulBluffs: 25,
		SuccessfulCatches: 25, // 50/50 balance
	}

	depth := computeBluffingDepth(results)

	// Good bluffing mechanics should score well
	if depth < 0.5 {
		t.Errorf("Expected high bluffing depth for ideal rates, got %f", depth)
	}
}

func TestBettingEngagement(t *testing.T) {
	results := &SimulationResults{
		TotalGames:   100,
		Wins:         []int{50, 50},
		TotalBets:    500, // 5 bets per game
		AllInCount:   15,  // 15% all-in rate
		ShowdownWins: 70,
		FoldWins:     25,
	}

	engagement := computeBettingEngagement(results)

	// Active betting with good showdown rate should engage
	if engagement < 0.4 {
		t.Errorf("Expected good betting engagement, got %f", engagement)
	}
}

func TestCoherencePenalty(t *testing.T) {
	// WAR mode with empty_hand win condition should be penalized
	g := &genome.GameGenome{
		TurnStructure: genome.TurnStructure{
			TableauMode: genome.TableauModeWar,
		},
		WinConditions: []genome.WinCondition{
			{Type: genome.WinTypeEmptyHand},
		},
	}

	penalty := calculateCoherencePenalty(g)

	if penalty < 0.2 {
		t.Errorf("Expected penalty for WAR + empty_hand, got %f", penalty)
	}
}

func TestCoherencePenaltyNone(t *testing.T) {
	// No tableau mode should have no penalty
	g := &genome.GameGenome{
		TurnStructure: genome.TurnStructure{
			TableauMode: genome.TableauModeNone,
		},
		WinConditions: []genome.WinCondition{
			{Type: genome.WinTypeEmptyHand},
		},
	}

	penalty := calculateCoherencePenalty(g)

	if penalty > 0.0 {
		t.Errorf("Expected no penalty for NONE mode, got %f", penalty)
	}
}

func TestSessionLengthValid(t *testing.T) {
	results := &SimulationResults{
		AvgTurns: 450, // 15 minutes (450 turns * 2 sec = 900 sec)
	}

	length, valid := computeSessionLength(results)

	if !valid {
		t.Error("Expected valid session length")
	}

	// 15 minutes is optimal
	if length < 0.9 {
		t.Errorf("Expected near-optimal session length for 15 min game, got %f", length)
	}
}

func TestSessionLengthTooLong(t *testing.T) {
	results := &SimulationResults{
		AvgTurns: 2000, // Over 60 minutes
	}

	_, valid := computeSessionLength(results)

	if valid {
		t.Error("Expected invalid session length for >60 min game")
	}
}

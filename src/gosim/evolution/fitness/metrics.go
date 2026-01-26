// Package fitness provides fitness evaluation for evolved card game genomes.
package fitness

import (
	"math"

	"github.com/signalnine/darwindeck/gosim/genome"
)

// SimulationResults holds the results from batch game simulation.
type SimulationResults struct {
	TotalGames  int
	Wins        []int   // Wins per player (index = player ID)
	PlayerCount int     // Number of players (2-4)
	Draws       int
	AvgTurns    float64
	Errors      int

	// Decision instrumentation
	TotalDecisions  int
	TotalValidMoves int
	ForcedDecisions int
	TotalHandSize   int // For filtering ratio calculation
	TotalInteractions int
	TotalActions    int

	// Bluffing metrics (ClaimPhase games)
	TotalClaims      int
	TotalBluffs      int
	TotalChallenges  int
	SuccessfulBluffs int
	SuccessfulCatches int

	// Betting metrics (BettingPhase games)
	TotalBets    int
	BettingBluffs int
	FoldWins     int
	ShowdownWins int
	AllInCount   int

	// Tension curve metrics
	LeadChanges     int
	DecisiveTurnPct float64
	ClosestMargin   float64
	TrailingWinners int // Games where winner was behind at midpoint

	// Solitaire detection metrics
	MoveDisruptionEvents int
	ContentionEvents     int
	ForcedResponseEvents int
	OpponentTurnCount    int

	// Team play metrics
	TeamWins []int // Win count per team (nil if not a team game)
}

// Player0Wins returns wins for player 0 (backward compatibility).
func (r *SimulationResults) Player0Wins() int {
	if len(r.Wins) > 0 {
		return r.Wins[0]
	}
	return 0
}

// Player1Wins returns wins for player 1 (backward compatibility).
func (r *SimulationResults) Player1Wins() int {
	if len(r.Wins) > 1 {
		return r.Wins[1]
	}
	return 0
}

// FitnessMetrics contains the complete fitness evaluation.
type FitnessMetrics struct {
	DecisionDensity      float64
	ComebackPotential    float64
	TensionCurve         float64
	InteractionFrequency float64
	RulesComplexity      float64
	SessionLength        float64 // Tracked but not averaged (constraint only)
	SkillVsLuck          float64
	BluffingDepth        float64 // Quality of bluffing mechanics
	BettingEngagement    float64 // Psychological appeal of betting
	TotalFitness         float64
	GamesSimulated       int
	Valid                bool
}

// ComputeMetrics calculates fitness metrics from simulation results.
func ComputeMetrics(g *genome.GameGenome, results *SimulationResults, weights map[string]float64, style string) *FitnessMetrics {
	// Check for unplayable games
	if results.Errors > results.TotalGames/2 || results.TotalGames == 0 {
		return &FitnessMetrics{
			GamesSimulated: results.TotalGames,
			Valid:          false,
		}
	}

	// 1. Decision density
	decisionDensity := computeDecisionDensity(g, results)

	// 2. Comeback potential
	comebackPotential := computeComebackPotential(results)

	// 3. Tension curve
	tensionCurve := computeTensionCurve(results)

	// 4. Interaction frequency
	interactionFrequency := computeInteractionFrequency(g, results)

	// 5. Rules complexity (inverted - simpler is better)
	rulesComplexity := ComputeRulesComplexity(g)

	// 6. Session length (constraint)
	sessionLength, valid := computeSessionLength(results)
	if !valid {
		return &FitnessMetrics{
			GamesSimulated: results.TotalGames,
			Valid:          false,
		}
	}

	// 7. Skill vs luck
	skillVsLuck := computeSkillVsLuck(g, results, comebackPotential, style)

	// 8. Bluffing depth
	bluffingDepth := computeBluffingDepth(results)

	// 9. Betting engagement
	bettingEngagement := computeBettingEngagement(results)

	// Check validity
	validResult := results.Errors == 0 && results.TotalGames > 0

	// Compute weighted total
	// KEY INSIGHT: Tension × decision_density as INTERACTION TERM
	effectiveTension := tensionCurve * decisionDensity

	totalFitness := weights["decision_density"]*decisionDensity +
		weights["comeback_potential"]*comebackPotential +
		weights["tension_curve"]*effectiveTension +
		weights["interaction_frequency"]*interactionFrequency +
		weights["rules_complexity"]*rulesComplexity +
		weights["skill_vs_luck"]*skillVsLuck +
		weights["bluffing_depth"]*bluffingDepth +
		weights["betting_engagement"]*bettingEngagement

	// Quality gates
	qualityMultiplier := 1.0

	// Comeback potential gate
	if comebackPotential < 0.15 {
		qualityMultiplier *= 0.5
	}

	// Skill vs luck gate
	if skillVsLuck < 0.15 {
		qualityMultiplier *= 0.7
	}

	// One-sidedness check
	if results.TotalGames > 0 && len(results.Wins) >= 2 {
		maxWins := 0
		for _, w := range results.Wins {
			if w > maxWins {
				maxWins = w
			}
		}
		maxWinRate := float64(maxWins) / float64(results.TotalGames)
		if maxWinRate > 0.80 {
			qualityMultiplier *= 0.6
		}
	}

	// Coherence penalty
	coherencePenalty := calculateCoherencePenalty(g)
	qualityMultiplier *= (1.0 - coherencePenalty)

	totalFitness *= qualityMultiplier

	return &FitnessMetrics{
		DecisionDensity:      decisionDensity,
		ComebackPotential:    comebackPotential,
		TensionCurve:         tensionCurve,
		InteractionFrequency: interactionFrequency,
		RulesComplexity:      rulesComplexity,
		SessionLength:        sessionLength,
		SkillVsLuck:          skillVsLuck,
		BluffingDepth:        bluffingDepth,
		BettingEngagement:    bettingEngagement,
		TotalFitness:         totalFitness,
		GamesSimulated:       results.TotalGames,
		Valid:                validResult,
	}
}

func computeDecisionDensity(g *genome.GameGenome, results *SimulationResults) float64 {
	if results.TotalDecisions > 0 {
		// Real instrumentation available
		avgValidMoves := float64(results.TotalValidMoves) / float64(results.TotalDecisions)
		forcedRatio := float64(results.ForcedDecisions) / float64(results.TotalDecisions)

		var filteringScore, varietyScore float64
		if results.TotalHandSize > 0 {
			movesPerCard := float64(results.TotalValidMoves) / float64(results.TotalHandSize)

			if movesPerCard <= 1.0 {
				filteringScore = 1.0 - movesPerCard
				varietyScore = 0.0
			} else {
				filteringScore = 0.3
				extraOptions := movesPerCard - 1.0
				varietyScore = math.Min(0.5, extraOptions*0.15)
			}
		}

		rawChoiceScore := math.Min(1.0, (avgValidMoves-1)/6.0)
		constraintMultiplier := 0.2 + (filteringScore * 0.8)
		choiceScore := rawChoiceScore * constraintMultiplier

		return math.Min(1.0,
			choiceScore*0.35+
				filteringScore*0.30+
				varietyScore+
				(1.0-forcedRatio)*0.20)
	}

	// Fallback to heuristic
	optionalPhases := 0
	phaseCount := len(g.TurnStructure.Phases)
	hasConditions := 0

	for _, p := range g.TurnStructure.Phases {
		switch phase := p.(type) {
		case *genome.DrawPhase:
			if !phase.Mandatory {
				optionalPhases++
			}
			if phase.Condition != nil {
				hasConditions++
			}
		case *genome.PlayPhase:
			if !phase.Mandatory {
				optionalPhases++
			}
			if phase.ValidPlayCondition != nil {
				hasConditions++
			}
		}
	}

	return math.Min(1.0,
		math.Min(1.0, float64(phaseCount)/6.0)*0.5+
			math.Min(1.0, float64(optionalPhases)/3.0)*0.3+
			math.Min(1.0, float64(hasConditions)/3.0)*0.2)
}

func computeComebackPotential(results *SimulationResults) float64 {
	if results.PlayerCount == 0 {
		return 0.0
	}

	// Win rate balance
	expectedRate := 1.0 / float64(results.PlayerCount)
	maxDeviation := 1.0 - expectedRate

	var avgDeviation float64
	if results.TotalGames > 0 {
		var totalDeviation float64
		for _, wins := range results.Wins {
			actualRate := float64(wins) / float64(results.TotalGames)
			var deviation float64
			if maxDeviation > 0 {
				deviation = math.Abs(actualRate-expectedRate) / maxDeviation
			}
			totalDeviation += deviation
		}
		avgDeviation = totalDeviation / float64(len(results.Wins))
	}

	balanceScore := 1.0 - avgDeviation

	// Trailing winner frequency
	decisiveGames := results.TotalGames - results.Draws - results.Errors
	var trailingScore float64
	if decisiveGames > 0 && results.TrailingWinners > 0 {
		trailingFreq := float64(results.TrailingWinners) / float64(decisiveGames)
		trailingScore = 1.0 - math.Abs(0.5-trailingFreq)*2
	} else {
		trailingScore = balanceScore
	}

	return trailingScore*0.6 + balanceScore*0.4
}

func computeTensionCurve(results *SimulationResults) float64 {
	isBettingGame := results.TotalBets > 0
	hasMeaningfulTracking := results.LeadChanges > 0

	if isBettingGame && !hasMeaningfulTracking {
		// Betting game with no lead tracking: use betting-based tension
		gamesPlayed := float64(max(1, results.TotalGames-results.Draws-results.Errors))
		betsPerGame := float64(results.TotalBets) / gamesPlayed
		allInRate := float64(results.AllInCount) / gamesPlayed
		showdownRate := float64(results.ShowdownWins) / gamesPlayed

		betActivityScore := math.Min(1.0, betsPerGame/3.0)
		allInScore := math.Min(1.0, allInRate*2)
		showdownScore := math.Min(1.0, showdownRate)

		return betActivityScore*0.4 + allInScore*0.3 + showdownScore*0.3
	}

	if hasMeaningfulTracking {
		turnsPerExpectedChange := 20.0
		expectedChanges := math.Max(1, results.AvgTurns/turnsPerExpectedChange)
		leadChangeScore := math.Min(1.0, float64(results.LeadChanges)/expectedChanges)
		decisiveTurnScore := results.DecisiveTurnPct
		marginScore := 1.0 - results.ClosestMargin

		return leadChangeScore*0.4 + decisiveTurnScore*0.4 + marginScore*0.2
	}

	if results.ClosestMargin > 0 && results.ClosestMargin < 1.0 {
		marginScore := 1.0 - results.ClosestMargin
		decisiveScore := results.DecisiveTurnPct
		return marginScore*0.5 + decisiveScore*0.5
	}

	// Fallback
	turnScore := math.Min(1.0, results.AvgTurns/100.0)
	lengthBonus := math.Min(1.0, math.Max(0.0, (results.AvgTurns-20)/50.0))
	return math.Min(0.6, turnScore*0.6+lengthBonus*0.4)
}

func computeInteractionFrequency(g *genome.GameGenome, results *SimulationResults) float64 {
	if results.OpponentTurnCount > 0 {
		moveDisruption := math.Min(1.0, float64(results.MoveDisruptionEvents)/float64(results.OpponentTurnCount))
		forcedResponse := math.Min(1.0, float64(results.ForcedResponseEvents)/float64(results.OpponentTurnCount))

		var contention float64
		if results.TotalActions > 0 {
			contention = math.Min(1.0, float64(results.ContentionEvents)/float64(results.TotalActions))
		}

		return (moveDisruption + contention + forcedResponse) / 3.0
	}

	if results.TotalActions > 0 {
		interactionRatio := float64(results.TotalInteractions) / float64(results.TotalActions)
		return math.Min(1.0, interactionRatio)
	}

	// Fallback to heuristic
	specialEffectsScore := math.Min(1.0, float64(len(g.Effects))/3.0)
	var trickBasedScore float64
	if g.TurnStructure.IsTrickBased {
		trickBasedScore = 0.3
	}
	multiPhaseScore := math.Min(0.4, float64(len(g.TurnStructure.Phases))/10.0)

	return math.Min(1.0, specialEffectsScore*0.4+trickBasedScore+multiPhaseScore)
}

func computeSessionLength(results *SimulationResults) (float64, bool) {
	estimatedDurationSec := results.AvgTurns * 2 // 2 sec per turn
	targetMax := float64(60 * 60)                // 60 minutes

	if estimatedDurationSec > targetMax {
		return 0.0, false // Constraint violated
	}

	optimalSec := float64(15 * 60) // 15 minutes is ideal
	if estimatedDurationSec < optimalSec {
		return estimatedDurationSec / optimalSec, true
	}

	// Gradual decline from 15-60 min
	return 1.0 - (estimatedDurationSec-optimalSec)/(targetMax-optimalSec)*0.5, true
}

func computeSkillVsLuck(g *genome.GameGenome, results *SimulationResults, comebackPotential float64, style string) float64 {
	// Estimate skill potential from game structure
	lengthFactor := math.Min(1.0, results.AvgTurns/80.0)
	balanceFactor := comebackPotential

	phaseComplexity := len(g.TurnStructure.Phases) + len(g.Effects)
	if g.TurnStructure.IsTrickBased {
		phaseComplexity++
	}
	complexityFactor := math.Min(1.0, float64(phaseComplexity)/8.0)

	skillVsLuck := math.Min(1.0,
		lengthFactor*0.4+
			balanceFactor*0.3+
			complexityFactor*0.3)

	// For party style, invert skill metric
	if style == "party" {
		skillVsLuck = 1.0 - skillVsLuck
	}

	return skillVsLuck
}

func computeBluffingDepth(results *SimulationResults) float64 {
	if results.TotalClaims > 0 {
		// ClaimPhase bluffing
		bluffRate := float64(results.TotalBluffs) / float64(results.TotalClaims)
		challengeRate := float64(results.TotalChallenges) / float64(results.TotalClaims)

		bluffScore := 1.0 - math.Abs(bluffRate-0.6)*2
		bluffScore = math.Max(0.0, math.Min(1.0, bluffScore))

		challengeScore := 1.0 - math.Abs(challengeRate-0.4)*2
		challengeScore = math.Max(0.0, math.Min(1.0, challengeScore))

		totalOutcomes := results.SuccessfulBluffs + results.SuccessfulCatches
		var balanceScore float64
		if totalOutcomes > 0 {
			bluffSuccessRate := float64(results.SuccessfulBluffs) / float64(totalOutcomes)
			balanceScore = 1.0 - math.Abs(bluffSuccessRate-0.5)*2
			balanceScore = math.Max(0.0, math.Min(1.0, balanceScore))
		}

		return bluffScore*0.3 + challengeScore*0.3 + balanceScore*0.4
	}

	if results.TotalBets > 0 {
		// BettingPhase bluffing
		bettingBluffRate := float64(results.BettingBluffs) / float64(results.TotalBets)
		bluffScore := 1.0 - math.Abs(bettingBluffRate-0.3)*3
		bluffScore = math.Max(0.0, math.Min(1.0, bluffScore))

		totalWins := results.FoldWins + results.ShowdownWins
		var foldScore float64
		if totalWins > 0 {
			foldWinRate := float64(results.FoldWins) / float64(totalWins)
			foldScore = 1.0 - math.Abs(foldWinRate-0.35)*3
			foldScore = math.Max(0.0, math.Min(1.0, foldScore))
		}

		allInRate := float64(results.AllInCount) / float64(results.TotalBets)
		allInScore := 1.0 - math.Abs(allInRate-0.10)*10
		allInScore = math.Max(0.0, math.Min(1.0, allInScore))

		return bluffScore*0.35 + foldScore*0.40 + allInScore*0.25
	}

	return 0.0
}

func computeBettingEngagement(results *SimulationResults) float64 {
	if results.TotalBets == 0 {
		return 0.0
	}

	totalGames := float64(results.TotalGames)
	totalWins := 0
	for _, w := range results.Wins {
		totalWins += w
	}

	// Resolution rate
	var resolutionScore float64
	if totalGames > 0 {
		resolutionRate := float64(totalWins) / totalGames
		resolutionScore = math.Min(1.0, resolutionRate*1.5)
	}

	// All-in drama
	var dramaScore float64
	if totalGames > 0 {
		allInRate := float64(results.AllInCount) / totalGames
		if allInRate < 0.05 {
			dramaScore = allInRate / 0.05
		} else if allInRate <= 0.25 {
			dramaScore = 1.0
		} else {
			dramaScore = math.Max(0.3, 1.0-(allInRate-0.25)*2)
		}
	}

	// Betting activity
	var activityScore float64
	if totalGames > 0 {
		betsPerGame := float64(results.TotalBets) / totalGames
		if betsPerGame < 2 {
			activityScore = betsPerGame / 2
		} else if betsPerGame <= 20 {
			activityScore = 1.0
		} else {
			activityScore = math.Max(0.5, 1.0-(betsPerGame-20)/50)
		}
	}

	// Win variance
	varianceScore := 0.5
	if totalWins > 0 {
		maxWins := 0
		for _, w := range results.Wins {
			if w > maxWins {
				maxWins = w
			}
		}
		balance := 1.0 - (float64(maxWins) / float64(totalWins))
		varianceScore = balance * 2
	}

	// Showdown excitement
	showdownScore := 0.5
	totalResolved := results.FoldWins + results.ShowdownWins
	if totalResolved > 0 {
		showdownRate := float64(results.ShowdownWins) / float64(totalResolved)
		showdownScore = 1.0 - math.Abs(showdownRate-0.75)*2
		showdownScore = math.Max(0.0, math.Min(1.0, showdownScore))
	}

	return resolutionScore*0.30 +
		dramaScore*0.20 +
		activityScore*0.15 +
		varianceScore*0.15 +
		showdownScore*0.20
}

func calculateCoherencePenalty(g *genome.GameGenome) float64 {
	penalty := 0.0

	winTypes := make(map[genome.WinConditionType]bool)
	for _, wc := range g.WinConditions {
		winTypes[wc.Type] = true
	}

	mode := g.TurnStructure.TableauMode

	// WAR conflicts with empty_hand
	if mode == genome.TableauModeWar && winTypes[genome.WinTypeEmptyHand] {
		penalty += 0.30
	}

	// MATCH_RANK conflicts with capture_all
	if mode == genome.TableauModeMatchRank && winTypes[genome.WinTypeCaptureAll] {
		penalty += 0.20
	}

	// SEQUENCE conflicts with capture_all
	if mode == genome.TableauModeSequence && winTypes[genome.WinTypeCaptureAll] {
		penalty += 0.30
	}

	return math.Min(penalty, 0.50)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

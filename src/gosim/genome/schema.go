// Package genome provides typed game genome structures for the pure Go evolution system.
// This replaces the bytecode-based PhaseDescriptor with typed structs that can be
// directly interpreted by the simulation engine.
package genome

// Phase is the interface for all turn phases.
// Each phase type implements this interface for polymorphic handling.
type Phase interface {
	PhaseType() uint8
	phaseMarker() // unexported to prevent external implementations
}

// PhaseType constants (matching engine.PhaseType* constants)
const (
	PhaseTypeDraw    uint8 = 1
	PhaseTypePlay    uint8 = 2
	PhaseTypeDiscard uint8 = 3
	PhaseTypeTrick   uint8 = 4
	PhaseTypeBetting uint8 = 5
	PhaseTypeClaim   uint8 = 6
	PhaseTypeBidding uint8 = 7
)

// Location constants for card sources/targets
type Location uint8

const (
	LocationDeck         Location = 0
	LocationHand         Location = 1
	LocationDiscard      Location = 2
	LocationTableau      Location = 3
	LocationOpponentHand Location = 4
	LocationCaptured     Location = 5
)

// Condition represents a condition that must be met for a phase to execute.
// nil Condition means the phase always executes.
type Condition struct {
	OpCode   uint8  // Condition type (OpCheckHandSize, etc.)
	Operator uint8  // Comparison operator (OpEQ, OpLT, etc.)
	Value    int32  // Value to compare against
	RefLoc   uint8  // Reference location for some conditions
}

// DrawPhase represents drawing cards from a source.
type DrawPhase struct {
	Source    Location   // Where to draw from (deck, discard, opponent hand)
	Count     int        // Number of cards to draw
	Mandatory bool       // If false, player can choose to pass
	Condition *Condition // Optional condition for this phase
}

func (p *DrawPhase) PhaseType() uint8 { return PhaseTypeDraw }
func (p *DrawPhase) phaseMarker()     {}

// PlayPhase represents playing cards to a target location.
type PlayPhase struct {
	Target            Location   // Where to play cards (tableau, discard)
	MinCards          int        // Minimum cards that must be played
	MaxCards          int        // Maximum cards that can be played
	Mandatory         bool       // If true, must play if able
	PassIfUnable      bool       // If true, can pass when no valid plays
	ValidPlayCondition *Condition // Optional condition cards must satisfy
}

func (p *PlayPhase) PhaseType() uint8 { return PhaseTypePlay }
func (p *PlayPhase) phaseMarker()     {}

// DiscardPhase represents discarding cards.
type DiscardPhase struct {
	Target    Location // Where to discard (usually LocationDiscard)
	Count     int      // Number of cards to discard
	Mandatory bool     // If true, must discard
}

func (p *DiscardPhase) PhaseType() uint8 { return PhaseTypeDiscard }
func (p *DiscardPhase) phaseMarker()     {}

// TrickPhase represents trick-taking mechanics.
type TrickPhase struct {
	LeadSuitRequired bool  // If true, must follow suit if able
	TrumpSuit        uint8 // Trump suit (255 = none)
	HighCardWins     bool  // If true, highest card wins; if false, lowest wins
	BreakingSuit     uint8 // Suit that must be "broken" before leading (255 = none)
}

func (p *TrickPhase) PhaseType() uint8 { return PhaseTypeTrick }
func (p *TrickPhase) phaseMarker()     {}

// BettingPhase represents poker-style betting rounds.
type BettingPhase struct {
	MinBet    int // Minimum bet/raise amount
	MaxRaises int // Maximum raises per round (prevents infinite loops)
}

func (p *BettingPhase) PhaseType() uint8 { return PhaseTypeBetting }
func (p *BettingPhase) phaseMarker()     {}

// ClaimPhase represents bluffing/claiming mechanics (like Cheat/BS).
type ClaimPhase struct {
	// ClaimPhase has minimal configuration - the mechanics are handled
	// by the interpreter based on game state (current claim, etc.)
}

func (p *ClaimPhase) PhaseType() uint8 { return PhaseTypeClaim }
func (p *ClaimPhase) phaseMarker()     {}

// BiddingPhase represents contract bidding (like Spades).
type BiddingPhase struct {
	MinBid   int  // Minimum bid value
	MaxBid   int  // Maximum bid value
	AllowNil bool // If true, players can bid "Nil"

	// Contract scoring parameters
	PointsPerTrickBid     int // Points per trick bid (e.g., 10 for Spades)
	OvertrickPoints       int // Points per overtrick (bag)
	FailedContractPenalty int // Penalty multiplier for failing contract
	NilBonus              int // Bonus for successful Nil bid
	NilPenalty            int // Penalty for failed Nil bid
	BagLimit              int // Number of bags before penalty
	BagPenalty            int // Penalty when bag limit reached
}

func (p *BiddingPhase) PhaseType() uint8 { return PhaseTypeBidding }
func (p *BiddingPhase) phaseMarker()     {}

// WinConditionType constants
type WinConditionType uint8

const (
	WinTypeEmptyHand    WinConditionType = 0
	WinTypeHighScore    WinConditionType = 1
	WinTypeFirstToScore WinConditionType = 2
	WinTypeCaptureAll   WinConditionType = 3
	WinTypeLowScore     WinConditionType = 4
	WinTypeAllHandsEmpty WinConditionType = 5
	WinTypeBestHand     WinConditionType = 6
	WinTypeMostCaptured WinConditionType = 7
)

// WinCondition defines how the game ends and who wins.
type WinCondition struct {
	Type      WinConditionType
	Threshold int32 // Score threshold for score-based wins
}

// TableauMode defines how the tableau is used.
type TableauMode uint8

const (
	TableauModeNone      TableauMode = 0
	TableauModeWar       TableauMode = 1
	TableauModeMatchRank TableauMode = 2
	TableauModeSequence  TableauMode = 3
)

// SequenceDirection for sequence-based tableau play.
type SequenceDirection uint8

const (
	SequenceAscending  SequenceDirection = 0
	SequenceDescending SequenceDirection = 1
	SequenceBoth       SequenceDirection = 2
)

// EffectType constants for special card effects.
type EffectType uint8

const (
	EffectSkipNext    EffectType = 0
	EffectReverse     EffectType = 1
	EffectDrawTwo     EffectType = 2
	EffectDrawFour    EffectType = 3
	EffectWild        EffectType = 4
	EffectSwapHands   EffectType = 5
	EffectBlockNext   EffectType = 6
	EffectStealCard   EffectType = 7
	EffectPeekHand    EffectType = 8
	EffectDiscardPile EffectType = 9
)

// SpecialEffect defines what happens when a specific card is played.
type SpecialEffect struct {
	TriggerRank uint8      // Card rank that triggers this effect (0-12 for 2-A)
	Effect      EffectType // What effect to apply
	Target      uint8      // Target selector (0=next, 1=previous, 2=all, etc.)
	Value       uint8      // Effect-specific value (e.g., number of cards to draw)
}

// ScoringTrigger defines when scoring rules apply.
type ScoringTrigger uint8

const (
	TriggerTrickWin    ScoringTrigger = 0
	TriggerCapture     ScoringTrigger = 1
	TriggerPlay        ScoringTrigger = 2
	TriggerHandEnd     ScoringTrigger = 3
	TriggerSetComplete ScoringTrigger = 4
)

// CardScoringRule defines points for specific cards.
type CardScoringRule struct {
	Suit    uint8          // 0-3 for suits, 255 for "any"
	Rank    uint8          // 0-12 for ranks, 255 for "any"
	Points  int16          // Points to award (can be negative)
	Trigger ScoringTrigger // When this rule applies
}

// HandEvaluationMethod defines how hands are compared.
type HandEvaluationMethod uint8

const (
	EvalMethodNone         HandEvaluationMethod = 0
	EvalMethodHighCard     HandEvaluationMethod = 1
	EvalMethodPointTotal   HandEvaluationMethod = 2
	EvalMethodPatternMatch HandEvaluationMethod = 3
	EvalMethodCardCount    HandEvaluationMethod = 4
)

// CardValue defines point values for card ranks (e.g., Blackjack scoring).
type CardValue struct {
	Rank     uint8 // 0-12 for 2-A
	Value    uint8 // Primary point value
	AltValue uint8 // Alternate value (e.g., Ace = 1 or 11)
}

// HandPattern defines a poker-style hand pattern.
type HandPattern struct {
	Name           string  // e.g., "flush", "straight", "full_house"
	Priority       uint8   // Higher priority wins ties
	RequiredCount  uint8   // Number of cards required
	SameSuitCount  uint8   // Cards that must share suit
	SequenceLength uint8   // Length of required sequence
	SequenceWrap   bool    // If true, sequence can wrap (K-A-2)
	SameRankGroups []uint8 // Required groups of same rank (e.g., [3,2] for full house)
	RequiredRanks  []uint8 // Specific ranks required
}

// HandEvaluation defines how to evaluate and compare hands.
type HandEvaluation struct {
	Method        HandEvaluationMethod
	TargetValue   uint8       // For POINT_TOTAL (e.g., 21 for Blackjack)
	BustThreshold uint8       // For POINT_TOTAL (e.g., 22 for Blackjack bust)
	CardValues    []CardValue // Card point values
	Patterns      []HandPattern // Hand patterns for PATTERN_MATCH
}

// SetupRules defines initial game setup.
type SetupRules struct {
	CardsPerPlayer int  // Cards dealt to each player
	TableauSize    int  // Number of tableau piles (0 = none)
	StartingChips  int  // Chips for betting games (0 = no betting)
	DealToTableau  int  // Cards dealt to tableau at start
}

// TurnStructure defines the phases of each turn.
type TurnStructure struct {
	Phases            []Phase           // Ordered phases in a turn
	MaxTurns          int               // Maximum turns before game ends
	TableauMode       TableauMode       // How tableau is used
	SequenceDirection SequenceDirection // For sequence-based play
	IsTrickBased      bool              // If true, game uses trick-taking mechanics
}

// TeamConfig defines team play settings.
type TeamConfig struct {
	Enabled bool    // If true, team play is active
	Teams   [][]int // Player indices per team, e.g., [[0,2], [1,3]]
}

// GameGenome is the complete game definition.
// This is the top-level struct that fully describes an evolved card game.
type GameGenome struct {
	Name          string          // Human-readable game name
	Generation    int             // Evolution generation number
	Setup         SetupRules      // Initial setup
	TurnStructure TurnStructure   // Turn phases and limits
	WinConditions []WinCondition  // How the game ends
	Effects       []SpecialEffect // Special card effects
	CardScoring   []CardScoringRule // Scoring rules
	HandEval      *HandEvaluation // Hand evaluation (poker, blackjack)
	Teams         *TeamConfig     // Optional team configuration
}

// Clone creates a deep copy of the genome.
func (g *GameGenome) Clone() *GameGenome {
	if g == nil {
		return nil
	}

	clone := &GameGenome{
		Name:       g.Name,
		Generation: g.Generation,
		Setup:      g.Setup, // SetupRules is a value type
	}

	// Clone TurnStructure
	clone.TurnStructure = TurnStructure{
		MaxTurns:          g.TurnStructure.MaxTurns,
		TableauMode:       g.TurnStructure.TableauMode,
		SequenceDirection: g.TurnStructure.SequenceDirection,
		IsTrickBased:      g.TurnStructure.IsTrickBased,
	}

	// Clone phases
	if g.TurnStructure.Phases != nil {
		clone.TurnStructure.Phases = make([]Phase, len(g.TurnStructure.Phases))
		for i, phase := range g.TurnStructure.Phases {
			clone.TurnStructure.Phases[i] = clonePhase(phase)
		}
	}

	// Clone WinConditions
	if g.WinConditions != nil {
		clone.WinConditions = make([]WinCondition, len(g.WinConditions))
		copy(clone.WinConditions, g.WinConditions)
	}

	// Clone Effects
	if g.Effects != nil {
		clone.Effects = make([]SpecialEffect, len(g.Effects))
		copy(clone.Effects, g.Effects)
	}

	// Clone CardScoring
	if g.CardScoring != nil {
		clone.CardScoring = make([]CardScoringRule, len(g.CardScoring))
		copy(clone.CardScoring, g.CardScoring)
	}

	// Clone HandEval
	if g.HandEval != nil {
		clone.HandEval = cloneHandEvaluation(g.HandEval)
	}

	// Clone Teams
	if g.Teams != nil {
		clone.Teams = cloneTeamConfig(g.Teams)
	}

	return clone
}

// clonePhase creates a deep copy of a phase.
func clonePhase(p Phase) Phase {
	switch phase := p.(type) {
	case *DrawPhase:
		cp := *phase
		if phase.Condition != nil {
			cond := *phase.Condition
			cp.Condition = &cond
		}
		return &cp
	case *PlayPhase:
		cp := *phase
		if phase.ValidPlayCondition != nil {
			cond := *phase.ValidPlayCondition
			cp.ValidPlayCondition = &cond
		}
		return &cp
	case *DiscardPhase:
		cp := *phase
		return &cp
	case *TrickPhase:
		cp := *phase
		return &cp
	case *BettingPhase:
		cp := *phase
		return &cp
	case *ClaimPhase:
		cp := *phase
		return &cp
	case *BiddingPhase:
		cp := *phase
		return &cp
	default:
		return nil
	}
}

// cloneHandEvaluation creates a deep copy of hand evaluation config.
func cloneHandEvaluation(h *HandEvaluation) *HandEvaluation {
	if h == nil {
		return nil
	}

	clone := &HandEvaluation{
		Method:        h.Method,
		TargetValue:   h.TargetValue,
		BustThreshold: h.BustThreshold,
	}

	if h.CardValues != nil {
		clone.CardValues = make([]CardValue, len(h.CardValues))
		copy(clone.CardValues, h.CardValues)
	}

	if h.Patterns != nil {
		clone.Patterns = make([]HandPattern, len(h.Patterns))
		for i, p := range h.Patterns {
			clone.Patterns[i] = cloneHandPattern(p)
		}
	}

	return clone
}

// cloneHandPattern creates a deep copy of a hand pattern.
func cloneHandPattern(p HandPattern) HandPattern {
	clone := HandPattern{
		Name:           p.Name,
		Priority:       p.Priority,
		RequiredCount:  p.RequiredCount,
		SameSuitCount:  p.SameSuitCount,
		SequenceLength: p.SequenceLength,
		SequenceWrap:   p.SequenceWrap,
	}

	if p.SameRankGroups != nil {
		clone.SameRankGroups = make([]uint8, len(p.SameRankGroups))
		copy(clone.SameRankGroups, p.SameRankGroups)
	}

	if p.RequiredRanks != nil {
		clone.RequiredRanks = make([]uint8, len(p.RequiredRanks))
		copy(clone.RequiredRanks, p.RequiredRanks)
	}

	return clone
}

// cloneTeamConfig creates a deep copy of team configuration.
func cloneTeamConfig(t *TeamConfig) *TeamConfig {
	if t == nil {
		return nil
	}

	clone := &TeamConfig{
		Enabled: t.Enabled,
	}

	if t.Teams != nil {
		clone.Teams = make([][]int, len(t.Teams))
		for i, team := range t.Teams {
			clone.Teams[i] = make([]int, len(team))
			copy(clone.Teams[i], team)
		}
	}

	return clone
}

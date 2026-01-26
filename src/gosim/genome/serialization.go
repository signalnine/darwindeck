package genome

import (
	"encoding/json"
	"fmt"
	"strings"
)

// PhaseJSON is an intermediate representation for JSON unmarshaling.
// It captures the phase type and raw data for later conversion to typed phases.
// Supports both Go format (nested "data") and Python format (flat structure).
type PhaseJSON struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
	// Python format fields (flat structure)
	Source             string             `json:"source,omitempty"`
	Target             string             `json:"target,omitempty"`
	Count              int                `json:"count,omitempty"`
	Mandatory          bool               `json:"mandatory,omitempty"`
	MinCards           int                `json:"min_cards,omitempty"`
	MaxCards           int                `json:"max_cards,omitempty"`
	ValidPlayCondition *ConditionJSON     `json:"valid_play_condition,omitempty"`
	Condition          *ConditionJSON     `json:"condition,omitempty"`
	LeadSuitRequired   bool               `json:"lead_suit_required,omitempty"`
	TrumpSuit          *string            `json:"trump_suit,omitempty"`
	HighCardWins       bool               `json:"high_card_wins,omitempty"`
	BreakingSuit       *string            `json:"breaking_suit,omitempty"`
	MinBet             int                `json:"min_bet,omitempty"`
	MaxRaises          int                `json:"max_raises,omitempty"`
	MinBid             int                `json:"min_bid,omitempty"`
	MaxBid             int                `json:"max_bid,omitempty"`
	AllowNil           bool               `json:"allow_nil,omitempty"`
	// ClaimPhase fields
	SequentialRank     bool               `json:"sequential_rank,omitempty"`
	AllowChallenge     bool               `json:"allow_challenge,omitempty"`
	PilePenalty        bool               `json:"pile_penalty,omitempty"`
}

// TurnStructureJSON is used for JSON serialization.
type TurnStructureJSON struct {
	Phases            []json.RawMessage `json:"phases"`
	MaxTurns          int               `json:"max_turns,omitempty"`
	TableauMode       string            `json:"tableau_mode,omitempty"`
	SequenceDirection string            `json:"sequence_direction,omitempty"`
	// Python format fields
	IsTrickBased      bool              `json:"is_trick_based,omitempty"`
	TricksPerHand     *int              `json:"tricks_per_hand,omitempty"`
}

// SetupRulesJSON for Python format compatibility.
type SetupRulesJSON struct {
	CardsPerPlayer      int    `json:"cards_per_player"`
	TableauSize         int    `json:"tableau_size,omitempty"`
	StartingChips       int    `json:"starting_chips,omitempty"`
	DealToTableau       int    `json:"deal_to_tableau,omitempty"`
	// Python format fields
	InitialDeck         string `json:"initial_deck,omitempty"`
	InitialDiscardCount int    `json:"initial_discard_count,omitempty"`
	TrumpSuit           string `json:"trump_suit,omitempty"`
	TableauMode         string `json:"tableau_mode,omitempty"`
	SequenceDirection   string `json:"sequence_direction,omitempty"`
}

// GameGenomeJSON is used for JSON serialization.
// Supports both Go format and Python format.
type GameGenomeJSON struct {
	Name          string              `json:"name,omitempty"`
	Setup         json.RawMessage     `json:"setup"`
	TurnStructure TurnStructureJSON   `json:"turn_structure"`
	WinConditions []WinConditionJSON  `json:"win_conditions"`
	Effects       []SpecialEffect     `json:"effects,omitempty"`
	CardScoring   []CardScoringRule   `json:"card_scoring,omitempty"`
	HandEval      *HandEvaluation     `json:"hand_evaluation,omitempty"`
	Teams         *TeamConfig         `json:"teams,omitempty"`
	// Python format fields
	SchemaVersion  string              `json:"schema_version,omitempty"`
	GenomeID       string              `json:"genome_id,omitempty"`
	Generation     int                 `json:"generation,omitempty"`
	SpecialEffects []SpecialEffectJSON `json:"special_effects,omitempty"`
	ScoringRules   []int               `json:"scoring_rules,omitempty"`
	MaxTurns       int                 `json:"max_turns,omitempty"`
	MinTurns       int                 `json:"min_turns,omitempty"`
	PlayerCount    int                 `json:"player_count,omitempty"`
}

// SpecialEffectJSON for Python format compatibility.
type SpecialEffectJSON struct {
	TriggerRank string `json:"trigger_rank"`
	EffectType  string `json:"effect_type"`
	Target      string `json:"target"`
	Value       int    `json:"value"`
}

// WinConditionJSON for JSON serialization.
type WinConditionJSON struct {
	Type      string `json:"type"`
	Threshold int32  `json:"threshold,omitempty"`
}

// DrawPhaseJSON for JSON serialization.
type DrawPhaseJSON struct {
	Source    string         `json:"source"`
	Count     int            `json:"count"`
	Mandatory bool           `json:"mandatory"`
	Condition *ConditionJSON `json:"condition,omitempty"`
}

// PlayPhaseJSON for JSON serialization.
type PlayPhaseJSON struct {
	Target             string         `json:"target"`
	MinCards           int            `json:"min_cards"`
	MaxCards           int            `json:"max_cards"`
	Mandatory          bool           `json:"mandatory"`
	PassIfUnable       bool           `json:"pass_if_unable"`
	ValidPlayCondition *ConditionJSON `json:"valid_play_condition,omitempty"`
}

// DiscardPhaseJSON for JSON serialization.
type DiscardPhaseJSON struct {
	Target    string `json:"target"`
	Count     int    `json:"count"`
	Mandatory bool   `json:"mandatory"`
}

// TrickPhaseJSON for JSON serialization.
type TrickPhaseJSON struct {
	LeadSuitRequired bool   `json:"lead_suit_required"`
	TrumpSuit        string `json:"trump_suit,omitempty"`
	HighCardWins     bool   `json:"high_card_wins"`
	BreakingSuit     string `json:"breaking_suit,omitempty"`
}

// BettingPhaseJSON for JSON serialization.
type BettingPhaseJSON struct {
	MinBet    int `json:"min_bet"`
	MaxRaises int `json:"max_raises"`
}

// ClaimPhaseJSON for JSON serialization.
type ClaimPhaseJSON struct {
	// Currently empty - claim mechanics are state-based
}

// BiddingPhaseJSON for JSON serialization.
type BiddingPhaseJSON struct {
	MinBid                int  `json:"min_bid"`
	MaxBid                int  `json:"max_bid"`
	AllowNil              bool `json:"allow_nil"`
	PointsPerTrickBid     int  `json:"points_per_trick_bid,omitempty"`
	OvertrickPoints       int  `json:"overtrick_points,omitempty"`
	FailedContractPenalty int  `json:"failed_contract_penalty,omitempty"`
	NilBonus              int  `json:"nil_bonus,omitempty"`
	NilPenalty            int  `json:"nil_penalty,omitempty"`
	BagLimit              int  `json:"bag_limit,omitempty"`
	BagPenalty            int  `json:"bag_penalty,omitempty"`
}

// ConditionJSON for JSON serialization.
// Supports both Go format and Python format.
type ConditionJSON struct {
	// Go format fields
	OpCode   string `json:"op_code,omitempty"`
	Operator string `json:"operator,omitempty"`
	Value    int32  `json:"value,omitempty"`
	RefLoc   string `json:"ref_loc,omitempty"`
	// Python format fields
	Type          string           `json:"type,omitempty"`           // "simple" or "compound"
	ConditionType string           `json:"condition_type,omitempty"` // Python enum name
	Reference     interface{}      `json:"reference,omitempty"`      // Can be string or null
	Logic         string           `json:"logic,omitempty"`          // "AND" or "OR" for compound
	Conditions    []ConditionJSON  `json:"conditions,omitempty"`     // For compound conditions
}

// UnmarshalJSON implements custom JSON unmarshaling for GameGenome.
// Supports both Go format and Python format.
func (g *GameGenome) UnmarshalJSON(data []byte) error {
	var jg GameGenomeJSON
	if err := json.Unmarshal(data, &jg); err != nil {
		return fmt.Errorf("failed to unmarshal genome JSON: %w", err)
	}

	// Handle name from either format
	g.Name = jg.Name
	if g.Name == "" && jg.GenomeID != "" {
		g.Name = jg.GenomeID
	}

	// Parse setup from raw JSON to handle both formats
	var setupJSON SetupRulesJSON
	if err := json.Unmarshal(jg.Setup, &setupJSON); err != nil {
		return fmt.Errorf("failed to unmarshal setup: %w", err)
	}
	g.Setup = SetupRules{
		CardsPerPlayer: setupJSON.CardsPerPlayer,
		TableauSize:    setupJSON.TableauSize,
		StartingChips:  setupJSON.StartingChips,
		DealToTableau:  setupJSON.DealToTableau,
	}

	g.Effects = jg.Effects
	g.CardScoring = jg.CardScoring
	g.HandEval = jg.HandEval
	g.Teams = jg.Teams

	// Convert Python SpecialEffects to Go Effects
	if len(jg.SpecialEffects) > 0 {
		g.Effects = make([]SpecialEffect, len(jg.SpecialEffects))
		for i, se := range jg.SpecialEffects {
			g.Effects[i] = SpecialEffect{
				TriggerRank: parseRank(se.TriggerRank),
				Effect:      parseEffectType(se.EffectType),
				Target:      parseTarget(se.Target),
				Value:       uint8(se.Value),
			}
		}
	}

	// Convert turn structure - use Python max_turns if Go max_turns not set
	g.TurnStructure.MaxTurns = jg.TurnStructure.MaxTurns
	if g.TurnStructure.MaxTurns == 0 && jg.MaxTurns > 0 {
		g.TurnStructure.MaxTurns = jg.MaxTurns
	}

	// Handle tableau mode from setup (Python format) or turn_structure (Go format)
	if setupJSON.TableauMode != "" {
		g.TurnStructure.TableauMode = parseTableauMode(setupJSON.TableauMode)
	} else {
		g.TurnStructure.TableauMode = parseTableauMode(jg.TurnStructure.TableauMode)
	}

	// Handle sequence direction from setup (Python format) or turn_structure (Go format)
	if setupJSON.SequenceDirection != "" {
		g.TurnStructure.SequenceDirection = parseSequenceDirection(setupJSON.SequenceDirection)
	} else {
		g.TurnStructure.SequenceDirection = parseSequenceDirection(jg.TurnStructure.SequenceDirection)
	}

	// Convert phases
	phases := make([]Phase, 0, len(jg.TurnStructure.Phases))
	for i, phaseRaw := range jg.TurnStructure.Phases {
		var pj PhaseJSON
		if err := json.Unmarshal(phaseRaw, &pj); err != nil {
			return fmt.Errorf("failed to unmarshal phase %d: %w", i, err)
		}
		phase, err := parsePhase(pj)
		if err != nil {
			return fmt.Errorf("failed to parse phase %d: %w", i, err)
		}
		phases = append(phases, phase)
	}
	g.TurnStructure.Phases = phases

	// Convert win conditions
	g.WinConditions = make([]WinCondition, len(jg.WinConditions))
	for i, wc := range jg.WinConditions {
		g.WinConditions[i] = WinCondition{
			Type:      parseWinConditionType(wc.Type),
			Threshold: wc.Threshold,
		}
	}

	return nil
}

// MarshalJSON implements custom JSON marshaling for GameGenome.
func (g *GameGenome) MarshalJSON() ([]byte, error) {
	// Serialize setup to raw JSON
	setupJSON := SetupRulesJSON{
		CardsPerPlayer: g.Setup.CardsPerPlayer,
		TableauSize:    g.Setup.TableauSize,
		StartingChips:  g.Setup.StartingChips,
		DealToTableau:  g.Setup.DealToTableau,
	}
	setupBytes, err := json.Marshal(setupJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal setup: %w", err)
	}

	jg := GameGenomeJSON{
		Name:        g.Name,
		Setup:       setupBytes,
		Effects:     g.Effects,
		CardScoring: g.CardScoring,
		HandEval:    g.HandEval,
		Teams:       g.Teams,
	}

	// Convert turn structure
	jg.TurnStructure.MaxTurns = g.TurnStructure.MaxTurns
	jg.TurnStructure.TableauMode = tableauModeToString(g.TurnStructure.TableauMode)
	jg.TurnStructure.SequenceDirection = sequenceDirectionToString(g.TurnStructure.SequenceDirection)

	// Convert phases to raw JSON
	jg.TurnStructure.Phases = make([]json.RawMessage, len(g.TurnStructure.Phases))
	for i, phase := range g.TurnStructure.Phases {
		pj, err := marshalPhase(phase)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal phase %d: %w", i, err)
		}
		phaseBytes, err := json.Marshal(pj)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize phase %d: %w", i, err)
		}
		jg.TurnStructure.Phases[i] = phaseBytes
	}

	// Convert win conditions
	jg.WinConditions = make([]WinConditionJSON, len(g.WinConditions))
	for i, wc := range g.WinConditions {
		jg.WinConditions[i] = WinConditionJSON{
			Type:      winConditionTypeToString(wc.Type),
			Threshold: wc.Threshold,
		}
	}

	return json.Marshal(jg)
}

func parsePhase(pj PhaseJSON) (Phase, error) {
	// Normalize phase type to lowercase for matching
	phaseType := strings.ToLower(pj.Type)
	// Handle Python PascalCase format (e.g., "DrawPhase" -> "drawphase" -> "draw")
	phaseType = strings.TrimSuffix(phaseType, "phase")

	switch phaseType {
	case "draw":
		// Check if using Go format (nested data) or Python format (flat)
		if pj.Data != nil && len(pj.Data) > 0 {
			var dp DrawPhaseJSON
			if err := json.Unmarshal(pj.Data, &dp); err != nil {
				return nil, fmt.Errorf("invalid draw phase: %w", err)
			}
			return &DrawPhase{
				Source:    parseLocation(dp.Source),
				Count:     dp.Count,
				Mandatory: dp.Mandatory,
				Condition: parseCondition(dp.Condition),
			}, nil
		}
		// Python format (flat structure)
		return &DrawPhase{
			Source:    parseLocation(pj.Source),
			Count:     pj.Count,
			Mandatory: pj.Mandatory,
			Condition: parseCondition(pj.Condition),
		}, nil

	case "play":
		// Check if using Go format (nested data) or Python format (flat)
		if pj.Data != nil && len(pj.Data) > 0 {
			var pp PlayPhaseJSON
			if err := json.Unmarshal(pj.Data, &pp); err != nil {
				return nil, fmt.Errorf("invalid play phase: %w", err)
			}
			return &PlayPhase{
				Target:             parseLocation(pp.Target),
				MinCards:           pp.MinCards,
				MaxCards:           pp.MaxCards,
				Mandatory:          pp.Mandatory,
				PassIfUnable:       pp.PassIfUnable,
				ValidPlayCondition: parseCondition(pp.ValidPlayCondition),
			}, nil
		}
		// Python format (flat structure)
		return &PlayPhase{
			Target:             parseLocation(pj.Target),
			MinCards:           pj.MinCards,
			MaxCards:           pj.MaxCards,
			Mandatory:          pj.Mandatory,
			PassIfUnable:       !pj.Mandatory, // Python uses mandatory=false, Go uses pass_if_unable=true
			ValidPlayCondition: parseCondition(pj.ValidPlayCondition),
		}, nil

	case "discard":
		if pj.Data != nil && len(pj.Data) > 0 {
			var dp DiscardPhaseJSON
			if err := json.Unmarshal(pj.Data, &dp); err != nil {
				return nil, fmt.Errorf("invalid discard phase: %w", err)
			}
			return &DiscardPhase{
				Target:    parseLocation(dp.Target),
				Count:     dp.Count,
				Mandatory: dp.Mandatory,
			}, nil
		}
		// Python format
		return &DiscardPhase{
			Target:    parseLocation(pj.Target),
			Count:     pj.Count,
			Mandatory: pj.Mandatory,
		}, nil

	case "trick":
		if pj.Data != nil && len(pj.Data) > 0 {
			var tp TrickPhaseJSON
			if err := json.Unmarshal(pj.Data, &tp); err != nil {
				return nil, fmt.Errorf("invalid trick phase: %w", err)
			}
			return &TrickPhase{
				LeadSuitRequired: tp.LeadSuitRequired,
				TrumpSuit:        parseSuit(tp.TrumpSuit),
				HighCardWins:     tp.HighCardWins,
				BreakingSuit:     parseSuit(tp.BreakingSuit),
			}, nil
		}
		// Python format
		trumpSuit := ""
		if pj.TrumpSuit != nil {
			trumpSuit = *pj.TrumpSuit
		}
		breakingSuit := ""
		if pj.BreakingSuit != nil {
			breakingSuit = *pj.BreakingSuit
		}
		return &TrickPhase{
			LeadSuitRequired: pj.LeadSuitRequired,
			TrumpSuit:        parseSuit(trumpSuit),
			HighCardWins:     pj.HighCardWins,
			BreakingSuit:     parseSuit(breakingSuit),
		}, nil

	case "betting":
		if pj.Data != nil && len(pj.Data) > 0 {
			var bp BettingPhaseJSON
			if err := json.Unmarshal(pj.Data, &bp); err != nil {
				return nil, fmt.Errorf("invalid betting phase: %w", err)
			}
			return &BettingPhase{
				MinBet:    bp.MinBet,
				MaxRaises: bp.MaxRaises,
			}, nil
		}
		// Python format
		return &BettingPhase{
			MinBet:    pj.MinBet,
			MaxRaises: pj.MaxRaises,
		}, nil

	case "claim":
		return &ClaimPhase{}, nil

	case "bidding":
		if pj.Data != nil && len(pj.Data) > 0 {
			var bp BiddingPhaseJSON
			if err := json.Unmarshal(pj.Data, &bp); err != nil {
				return nil, fmt.Errorf("invalid bidding phase: %w", err)
			}
			return &BiddingPhase{
				MinBid:                bp.MinBid,
				MaxBid:                bp.MaxBid,
				AllowNil:              bp.AllowNil,
				PointsPerTrickBid:     bp.PointsPerTrickBid,
				OvertrickPoints:       bp.OvertrickPoints,
				FailedContractPenalty: bp.FailedContractPenalty,
				NilBonus:              bp.NilBonus,
				NilPenalty:            bp.NilPenalty,
				BagLimit:              bp.BagLimit,
				BagPenalty:            bp.BagPenalty,
			}, nil
		}
		// Python format
		return &BiddingPhase{
			MinBid:   pj.MinBid,
			MaxBid:   pj.MaxBid,
			AllowNil: pj.AllowNil,
		}, nil

	default:
		return nil, fmt.Errorf("unknown phase type: %s", pj.Type)
	}
}

func marshalPhase(phase Phase) (PhaseJSON, error) {
	var pj PhaseJSON
	var data interface{}

	switch p := phase.(type) {
	case *DrawPhase:
		pj.Type = "draw"
		data = DrawPhaseJSON{
			Source:    locationToString(p.Source),
			Count:     p.Count,
			Mandatory: p.Mandatory,
			Condition: marshalCondition(p.Condition),
		}

	case *PlayPhase:
		pj.Type = "play"
		data = PlayPhaseJSON{
			Target:             locationToString(p.Target),
			MinCards:           p.MinCards,
			MaxCards:           p.MaxCards,
			Mandatory:          p.Mandatory,
			PassIfUnable:       p.PassIfUnable,
			ValidPlayCondition: marshalCondition(p.ValidPlayCondition),
		}

	case *DiscardPhase:
		pj.Type = "discard"
		data = DiscardPhaseJSON{
			Target:    locationToString(p.Target),
			Count:     p.Count,
			Mandatory: p.Mandatory,
		}

	case *TrickPhase:
		pj.Type = "trick"
		data = TrickPhaseJSON{
			LeadSuitRequired: p.LeadSuitRequired,
			TrumpSuit:        suitToString(p.TrumpSuit),
			HighCardWins:     p.HighCardWins,
			BreakingSuit:     suitToString(p.BreakingSuit),
		}

	case *BettingPhase:
		pj.Type = "betting"
		data = BettingPhaseJSON{
			MinBet:    p.MinBet,
			MaxRaises: p.MaxRaises,
		}

	case *ClaimPhase:
		pj.Type = "claim"
		data = ClaimPhaseJSON{}

	case *BiddingPhase:
		pj.Type = "bidding"
		data = BiddingPhaseJSON{
			MinBid:                p.MinBid,
			MaxBid:                p.MaxBid,
			AllowNil:              p.AllowNil,
			PointsPerTrickBid:     p.PointsPerTrickBid,
			OvertrickPoints:       p.OvertrickPoints,
			FailedContractPenalty: p.FailedContractPenalty,
			NilBonus:              p.NilBonus,
			NilPenalty:            p.NilPenalty,
			BagLimit:              p.BagLimit,
			BagPenalty:            p.BagPenalty,
		}

	default:
		return pj, fmt.Errorf("unknown phase type: %T", phase)
	}

	rawData, err := json.Marshal(data)
	if err != nil {
		return pj, err
	}
	pj.Data = rawData

	return pj, nil
}

func parseLocation(s string) Location {
	// Normalize to lowercase for matching
	lower := strings.ToLower(s)
	switch lower {
	case "deck":
		return LocationDeck
	case "hand":
		return LocationHand
	case "discard":
		return LocationDiscard
	case "tableau":
		return LocationTableau
	case "opponent_hand":
		return LocationOpponentHand
	case "captured":
		return LocationCaptured
	default:
		return LocationDeck
	}
}

func locationToString(loc Location) string {
	switch loc {
	case LocationDeck:
		return "deck"
	case LocationHand:
		return "hand"
	case LocationDiscard:
		return "discard"
	case LocationTableau:
		return "tableau"
	case LocationOpponentHand:
		return "opponent_hand"
	case LocationCaptured:
		return "captured"
	default:
		return "deck"
	}
}

func parseSuit(s string) uint8 {
	// Normalize to lowercase for matching
	lower := strings.ToLower(s)
	switch lower {
	case "hearts":
		return 0
	case "diamonds":
		return 1
	case "clubs":
		return 2
	case "spades":
		return 3
	case "none", "":
		return 255
	default:
		return 255
	}
}

func suitToString(suit uint8) string {
	switch suit {
	case 0:
		return "hearts"
	case 1:
		return "diamonds"
	case 2:
		return "clubs"
	case 3:
		return "spades"
	default:
		return "none"
	}
}

func parseTableauMode(s string) TableauMode {
	// Normalize to lowercase for matching
	lower := strings.ToLower(s)
	switch lower {
	case "war":
		return TableauModeWar
	case "match_rank":
		return TableauModeMatchRank
	case "sequence":
		return TableauModeSequence
	default:
		return TableauModeNone
	}
}

func tableauModeToString(mode TableauMode) string {
	switch mode {
	case TableauModeWar:
		return "war"
	case TableauModeMatchRank:
		return "match_rank"
	case TableauModeSequence:
		return "sequence"
	default:
		return "none"
	}
}

func parseSequenceDirection(s string) SequenceDirection {
	// Normalize to lowercase for matching
	lower := strings.ToLower(s)
	switch lower {
	case "ascending":
		return SequenceAscending
	case "descending":
		return SequenceDescending
	case "both":
		return SequenceBoth
	default:
		return SequenceAscending
	}
}

func sequenceDirectionToString(dir SequenceDirection) string {
	switch dir {
	case SequenceAscending:
		return "ascending"
	case SequenceDescending:
		return "descending"
	case SequenceBoth:
		return "both"
	default:
		return "ascending"
	}
}

func parseWinConditionType(s string) WinConditionType {
	// Normalize to lowercase for matching
	lower := strings.ToLower(s)
	switch lower {
	case "empty_hand":
		return WinTypeEmptyHand
	case "high_score":
		return WinTypeHighScore
	case "first_to_score":
		return WinTypeFirstToScore
	case "capture_all":
		return WinTypeCaptureAll
	case "low_score":
		return WinTypeLowScore
	case "all_hands_empty":
		return WinTypeAllHandsEmpty
	case "best_hand":
		return WinTypeBestHand
	case "most_captured":
		return WinTypeMostCaptured
	default:
		return WinTypeEmptyHand
	}
}

func winConditionTypeToString(wct WinConditionType) string {
	switch wct {
	case WinTypeEmptyHand:
		return "empty_hand"
	case WinTypeHighScore:
		return "high_score"
	case WinTypeFirstToScore:
		return "first_to_score"
	case WinTypeCaptureAll:
		return "capture_all"
	case WinTypeLowScore:
		return "low_score"
	case WinTypeAllHandsEmpty:
		return "all_hands_empty"
	case WinTypeBestHand:
		return "best_hand"
	case WinTypeMostCaptured:
		return "most_captured"
	default:
		return "empty_hand"
	}
}

func parseCondition(cj *ConditionJSON) *Condition {
	if cj == nil {
		return nil
	}

	// Handle Python format (has "type" field)
	if cj.Type == "simple" || cj.ConditionType != "" {
		return parsePythonCondition(cj)
	}

	// Handle compound conditions (just return the first condition for now)
	if cj.Type == "compound" && len(cj.Conditions) > 0 {
		return parseCondition(&cj.Conditions[0])
	}

	// Go format
	return &Condition{
		OpCode:   parseOpCode(cj.OpCode),
		Operator: parseOperator(cj.Operator),
		Value:    cj.Value,
		RefLoc:   uint8(parseLocation(cj.RefLoc)),
	}
}

// parsePythonCondition converts Python condition format to Go Condition.
func parsePythonCondition(cj *ConditionJSON) *Condition {
	if cj == nil {
		return nil
	}

	// Map Python condition types to Go opcodes
	opCode := parsePythonConditionType(cj.ConditionType)
	operator := parsePythonOperator(cj.Operator)

	// Handle value - can be int, string (enum name), or interface{}
	var value int32
	switch v := cj.Reference.(type) {
	case string:
		// Reference might be a suit or rank name
		if suit := parseSuit(v); suit != 255 {
			value = int32(suit)
		} else {
			value = int32(parseRank(v))
		}
	case float64:
		value = int32(v)
	case int:
		value = int32(v)
	default:
		value = cj.Value
	}

	return &Condition{
		OpCode:   opCode,
		Operator: operator,
		Value:    value,
		RefLoc:   0, // Python conditions don't typically have ref_loc
	}
}

// parsePythonConditionType maps Python ConditionType enum to Go opcode.
func parsePythonConditionType(s string) uint8 {
	upper := strings.ToUpper(s)
	switch upper {
	case "HAND_SIZE":
		return 0 // check_hand_size
	case "CARD_RANK":
		return 1 // check_card_rank
	case "CARD_SUIT":
		return 2 // check_card_suit
	case "LOCATION_SIZE":
		return 3 // check_location_size
	case "SEQUENCE":
		return 4 // check_sequence
	case "MATCH_RANK":
		return 12 // check_card_matches_rank
	case "MATCH_SUIT":
		return 13 // check_card_matches_suit
	case "BEATS_TOP":
		return 14 // check_card_beats_top
	default:
		return 0
	}
}

// parsePythonOperator maps Python Operator enum to Go operator code.
func parsePythonOperator(s string) uint8 {
	upper := strings.ToUpper(s)
	switch upper {
	case "EQ", "EQUALS", "==":
		return 50
	case "NE", "NOT_EQUALS", "!=":
		return 51
	case "LT", "LESS_THAN", "<":
		return 52
	case "GT", "GREATER_THAN", ">":
		return 53
	case "LE", "LESS_EQUAL", "<=":
		return 54
	case "GE", "GREATER_EQUAL", ">=":
		return 55
	default:
		return 50 // default to equality
	}
}

func marshalCondition(c *Condition) *ConditionJSON {
	if c == nil {
		return nil
	}
	return &ConditionJSON{
		OpCode:   opCodeToString(c.OpCode),
		Operator: operatorToString(c.Operator),
		Value:    c.Value,
		RefLoc:   locationToString(Location(c.RefLoc)),
	}
}

func parseOpCode(s string) uint8 {
	switch s {
	case "check_hand_size":
		return 0
	case "check_card_rank":
		return 1
	case "check_card_suit":
		return 2
	case "check_location_size":
		return 3
	case "check_sequence":
		return 4
	case "check_card_matches_rank":
		return 12
	case "check_card_matches_suit":
		return 13
	case "check_card_beats_top":
		return 14
	default:
		return 0
	}
}

func opCodeToString(op uint8) string {
	switch op {
	case 0:
		return "check_hand_size"
	case 1:
		return "check_card_rank"
	case 2:
		return "check_card_suit"
	case 3:
		return "check_location_size"
	case 4:
		return "check_sequence"
	case 12:
		return "check_card_matches_rank"
	case 13:
		return "check_card_matches_suit"
	case 14:
		return "check_card_beats_top"
	default:
		return "check_hand_size"
	}
}

func parseOperator(s string) uint8 {
	switch s {
	case "eq":
		return 50
	case "ne":
		return 51
	case "lt":
		return 52
	case "gt":
		return 53
	case "le":
		return 54
	case "ge":
		return 55
	default:
		return 50
	}
}

func operatorToString(op uint8) string {
	switch op {
	case 50:
		return "eq"
	case 51:
		return "ne"
	case 52:
		return "lt"
	case 53:
		return "gt"
	case 54:
		return "le"
	case 55:
		return "ge"
	default:
		return "eq"
	}
}

// parseRank converts a rank string to uint8 (0-12 for 2-A).
func parseRank(s string) uint8 {
	upper := strings.ToUpper(s)
	switch upper {
	case "TWO", "2":
		return 0
	case "THREE", "3":
		return 1
	case "FOUR", "4":
		return 2
	case "FIVE", "5":
		return 3
	case "SIX", "6":
		return 4
	case "SEVEN", "7":
		return 5
	case "EIGHT", "8":
		return 6
	case "NINE", "9":
		return 7
	case "TEN", "10":
		return 8
	case "JACK", "J":
		return 9
	case "QUEEN", "Q":
		return 10
	case "KING", "K":
		return 11
	case "ACE", "A":
		return 12
	default:
		return 0
	}
}

// parseEffectType converts an effect type string to EffectType.
func parseEffectType(s string) EffectType {
	upper := strings.ToUpper(s)
	switch upper {
	case "SKIP_NEXT", "SKIP":
		return EffectSkipNext
	case "REVERSE":
		return EffectReverse
	case "DRAW_TWO":
		return EffectDrawTwo
	case "DRAW_FOUR":
		return EffectDrawFour
	case "WILD":
		return EffectWild
	case "SWAP_HANDS":
		return EffectSwapHands
	case "BLOCK_NEXT", "BLOCK":
		return EffectBlockNext
	case "STEAL_CARD":
		return EffectStealCard
	case "PEEK_HAND":
		return EffectPeekHand
	case "DISCARD_PILE":
		return EffectDiscardPile
	default:
		return EffectSkipNext
	}
}

// parseTarget converts a target string to uint8.
func parseTarget(s string) uint8 {
	upper := strings.ToUpper(s)
	switch upper {
	case "NEXT", "NEXT_PLAYER":
		return 0
	case "PREVIOUS", "PREVIOUS_PLAYER":
		return 1
	case "ALL", "ALL_PLAYERS":
		return 2
	case "SELF":
		return 3
	case "CHOSEN", "CHOSEN_PLAYER":
		return 4
	default:
		return 0
	}
}

// LoadGenomeFromJSON parses a GameGenome from JSON bytes.
func LoadGenomeFromJSON(data []byte) (*GameGenome, error) {
	var genome GameGenome
	if err := json.Unmarshal(data, &genome); err != nil {
		return nil, err
	}
	return &genome, nil
}

// SaveGenomeToJSON serializes a GameGenome to JSON bytes.
func SaveGenomeToJSON(genome *GameGenome) ([]byte, error) {
	return json.MarshalIndent(genome, "", "  ")
}

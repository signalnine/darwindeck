package genome

import "fmt"

// SkeletonType identifies the primary game skeleton.
type SkeletonType uint8

const (
	Shedding    SkeletonType = iota
	TrickTaking
	Rummy
)

var skeletonNames = [3]string{"shedding", "trick_taking", "rummy"}

func (s SkeletonType) String() string { return skeletonNames[s] }

// --- Shedding Parameters ---

// MatchRule defines how cards must match to be played.
type MatchRule uint8

const (
	MatchSuit   MatchRule = iota // Must match suit
	MatchRank                    // Must match rank
	MatchEither                  // Match suit OR rank
	MatchBoth                    // Match suit AND rank
)

// SheddingParams controls a shedding game.
type SheddingParams struct {
	MatchRule    MatchRule `json:"match_rule"`
	DrawPenalty  int       `json:"draw_penalty"`   // Cards drawn on no match (1-3)
	CanStack     bool      `json:"can_stack"`       // Chain effects (draw-2 on draw-2)
	PlayMultiple bool      `json:"play_multiple"`   // Play runs/sets at once
}

// --- Trick-Taking Parameters ---

// TrickScoring defines how tricks are scored.
type TrickScoring uint8

const (
	ScorePerTrick  TrickScoring = iota // Each trick worth 1 point
	ScoreCardPoints                     // Specific cards have point values
	ScoreAvoidance                      // Points are bad (Hearts-style)
)

// LeadRule defines restrictions on leading.
type LeadRule uint8

const (
	LeadNone          LeadRule = iota // No restriction
	LeadNoTrumpUntilBroken            // Can't lead trump until broken
	LeadWinnerLeads                   // Previous trick winner leads
)

// TrickTakingParams controls a trick-taking game.
type TrickTakingParams struct {
	MustFollowSuit  bool         `json:"must_follow_suit"`
	TrickScoring    TrickScoring `json:"trick_scoring"`
	LeadRestriction LeadRule     `json:"lead_restriction"`
	RoundsPerGame   int          `json:"rounds_per_game"` // 1-13
}

// --- Rummy Parameters ---

// MeldType defines what kinds of melds are allowed.
type MeldType uint8

const (
	MeldSets  MeldType = iota // Groups of same rank
	MeldRuns                   // Sequential same-suit cards
	MeldBoth                   // Sets and runs
)

// DrawSource defines where players can draw from.
type DrawSource uint8

const (
	DrawDeck    DrawSource = iota
	DrawDiscard
	DrawEither
)

// RummyParams controls a rummy game.
type RummyParams struct {
	MeldTypes      MeldType   `json:"meld_types"`
	MinMeldSize    int        `json:"min_meld_size"`    // 2-4
	DrawFrom       DrawSource `json:"draw_from"`
	CanLayOff      bool       `json:"can_lay_off"`      // Extend existing melds
	KnockThreshold int        `json:"knock_threshold"`  // Deadwood to knock (0 = gin only)
}

// --- Shared Mechanics ---

// TrumpRule defines how trumps work.
type TrumpRule uint8

const (
	TrumpNone    TrumpRule = iota // No trump suit
	TrumpFixed                    // Fixed suit (specified in config)
	TrumpCut                      // Cut from deck during deal
	TrumpLed                      // First suit led becomes trump
)

// SpecialCardType defines special card effects.
type SpecialCardType uint8

const (
	SpecialSkip     SpecialCardType = iota // Skip next player
	SpecialReverse                          // Reverse play direction
	SpecialDrawTwo                          // Next player draws 2
	SpecialDrawFour                         // Next player draws 4
	SpecialWild                             // Can be played on anything
)

// SpecialCard assigns a special effect to cards matching a condition.
type SpecialCard struct {
	Type     SpecialCardType `json:"type"`
	ByRank   uint8           `json:"by_rank,omitempty"`   // 0 = any rank
	BySuit   uint8           `json:"by_suit,omitempty"`   // 0 = any suit (1-4 = specific)
}

// ScoringEvent defines when points are awarded.
type ScoringEvent uint8

const (
	ScoreOnTrickWin ScoringEvent = iota
	ScoreOnCapture
	ScoreOnPlay
	ScoreOnHandEnd
)

// CardScoring assigns point values to cards.
type CardScoring struct {
	Rank   uint8        `json:"rank"`            // 0 = all ranks
	Suit   uint8        `json:"suit"`            // 0 = all suits
	Points int          `json:"points"`
	Event  ScoringEvent `json:"event"`
}

// ScoringConfig holds all scoring rules.
type ScoringConfig struct {
	CardPoints []CardScoring `json:"card_points,omitempty"`
	TrumpSuit  uint8         `json:"trump_suit,omitempty"` // 1-4 for fixed trump
}

// --- Borrowed Mechanics ---

// MechanicType identifies a borrowable mechanic.
type MechanicType uint8

const (
	MechTrickScoring MechanicType = iota // Score based on tricks won
	MechMeldBonus                         // Bonus for forming melds
	MechDrawPenalty                       // Draw cards as penalty
	MechKnock                             // Knock to end round
	MechTrump                             // Trump suit mechanic
	MechAvoidance                         // Points-are-bad scoring
	MechPlayMultiple                      // Play multiple cards
	MechFollowSuit                        // Must follow suit restriction
)

// BorrowedMechanic represents a mechanic borrowed from another skeleton.
type BorrowedMechanic struct {
	Source   SkeletonType `json:"source"`
	Mechanic MechanicType `json:"mechanic"`
}

// --- Genome ---

// Genome encodes a complete card game.
type Genome struct {
	ID         string       `json:"id"`
	Generation int          `json:"generation"`
	Skeleton   SkeletonType `json:"skeleton"`
	Fitness    float64      `json:"fitness,omitempty"`

	// Shared parameters
	Players  int `json:"players"`   // 2-6
	HandSize int `json:"hand_size"` // 3-13

	// Skeleton-specific params (only one is active based on Skeleton)
	Shedding    *SheddingParams    `json:"shedding,omitempty"`
	TrickTaking *TrickTakingParams `json:"trick_taking,omitempty"`
	Rummy       *RummyParams       `json:"rummy,omitempty"`

	// Cross-skeleton mechanics
	Borrowed []BorrowedMechanic `json:"borrowed,omitempty"`

	// Shared optional mechanics
	SpecialCards []SpecialCard `json:"special_cards,omitempty"`
	Scoring      ScoringConfig `json:"scoring"`
	TrumpRule    TrumpRule     `json:"trump_rule"`
}

// Clone returns a deep copy of the genome. Returns nil if g is nil.
// Unlike a JSON round-trip, Clone is total: it cannot fail and never silently
// produces a zero-value genome on error.
func (g *Genome) Clone() *Genome {
	if g == nil {
		return nil
	}
	cp := *g
	if g.Shedding != nil {
		s := *g.Shedding
		cp.Shedding = &s
	}
	if g.TrickTaking != nil {
		t := *g.TrickTaking
		cp.TrickTaking = &t
	}
	if g.Rummy != nil {
		r := *g.Rummy
		cp.Rummy = &r
	}
	if g.Borrowed != nil {
		cp.Borrowed = append([]BorrowedMechanic(nil), g.Borrowed...)
	}
	if g.SpecialCards != nil {
		cp.SpecialCards = append([]SpecialCard(nil), g.SpecialCards...)
	}
	if g.Scoring.CardPoints != nil {
		cp.Scoring.CardPoints = append([]CardScoring(nil), g.Scoring.CardPoints...)
	}
	return &cp
}

// MaxTurns returns the computed maximum turns based on skeleton and params.
func (g *Genome) MaxTurns() int {
	switch g.Skeleton {
	case Shedding:
		// Worst case: cycling through deck multiple times
		return g.HandSize * g.Players * 10
	case TrickTaking:
		if g.TrickTaking != nil {
			return g.TrickTaking.RoundsPerGame * g.Players * g.HandSize
		}
		return g.HandSize * g.Players
	case Rummy:
		// Rummy can go long with draw/discard cycles
		return 52 * 4
	default:
		return 200
	}
}

// ActiveParams returns the skeleton-specific params as a string for debugging.
func (g *Genome) ActiveParams() string {
	switch g.Skeleton {
	case Shedding:
		if g.Shedding != nil {
			return fmt.Sprintf("shedding{match=%d, draw=%d, stack=%v, multi=%v}",
				g.Shedding.MatchRule, g.Shedding.DrawPenalty,
				g.Shedding.CanStack, g.Shedding.PlayMultiple)
		}
	case TrickTaking:
		if g.TrickTaking != nil {
			return fmt.Sprintf("trick{follow=%v, scoring=%d, lead=%d, rounds=%d}",
				g.TrickTaking.MustFollowSuit, g.TrickTaking.TrickScoring,
				g.TrickTaking.LeadRestriction, g.TrickTaking.RoundsPerGame)
		}
	case Rummy:
		if g.Rummy != nil {
			return fmt.Sprintf("rummy{melds=%d, min=%d, draw=%d, layoff=%v, knock=%d}",
				g.Rummy.MeldTypes, g.Rummy.MinMeldSize, g.Rummy.DrawFrom,
				g.Rummy.CanLayOff, g.Rummy.KnockThreshold)
		}
	}
	return "none"
}

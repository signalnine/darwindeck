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

func (s SkeletonType) String() string {
	if int(s) >= len(skeletonNames) {
		return fmt.Sprintf("SkeletonType(%d)", int(s))
	}
	return skeletonNames[s]
}

// --- Shedding Parameters ---

// MatchRule defines how cards must match to be played.
type MatchRule uint8

const (
	MatchSuit   MatchRule = iota // Must match suit
	MatchRank                    // Must match rank
	MatchEither                  // Match suit OR rank
	MatchBoth                    // Match suit AND rank
)

var matchRuleNames = [4]string{"suit", "rank", "either", "both"}

func (m MatchRule) String() string {
	if int(m) >= len(matchRuleNames) {
		return fmt.Sprintf("MatchRule(%d)", m)
	}
	return matchRuleNames[m]
}

// SheddingParams controls a shedding game.
type SheddingParams struct {
	MatchRule   MatchRule `json:"match_rule"`
	DrawPenalty int       `json:"draw_penalty"` // Cards drawn on no match (1-3)
	// RoundsPerGame plays the game as a series of banked-score rounds
	// (Mau-Mau scoring, audit remediation Task 22): emptying a hand ends the
	// ROUND (the scoring borrows bank state.Scores via EventRoundEnd) and the
	// game redeals; after all rounds the highest banked total wins.
	// Range 1-5; 0 is the legacy "unset" encoding (pre-Task-22 genomes) and
	// means 1. Values > 1 only take effect with a scoring borrow present
	// (MechMeldBonus or MechAvoidance -- see Genome.SheddingMultiRound):
	// without one, nothing writes Scores and the game stays single-round
	// (first empty hand wins), exactly the pre-Task-22 behavior.
	RoundsPerGame int `json:"rounds_per_game,omitempty"`
}

// --- Trick-Taking Parameters ---

// TrickScoring defines how tricks are scored.
type TrickScoring uint8

const (
	ScorePerTrick  TrickScoring = iota // Each trick worth 1 point
	ScoreCardPoints                     // Specific cards have point values
	ScoreAvoidance                      // Points are bad (Hearts-style)
)

var trickScoringNames = [3]string{"per_trick", "card_points", "avoidance"}

func (t TrickScoring) String() string {
	if int(t) >= len(trickScoringNames) {
		return fmt.Sprintf("TrickScoring(%d)", t)
	}
	return trickScoringNames[t]
}

// LeadRule defines restrictions on WHICH CARDS may lead a trick (consumed by
// the trick-taking runner's canLead).
//
// LeadWinnerLeads is RESERVED (Task 28 round 2, dd-027 inert-param class):
// winner-leads is the trick-taking skeleton's fixed turn order, hardcoded in
// ApplyMove's trick resolution (state.Active = winner), so as a LeadRule
// value it was byte-identical to LeadNone -- a phantom search dimension whose
// only effect was hash-distinct clone genomes (the rejected no-follow
// flagship champion carried it). Validation rejects it; mutation never
// produces it; the constant stays so the value remains nameable in error
// messages (the MechTrump precedent -- and pre-existing serialized genomes
// decode without renumbering). TestReservedWinnerLeadsValueIsInert pins the
// inertness: anyone giving the value real semantics must lift the
// reservation in the same change.
type LeadRule uint8

const (
	LeadNone          LeadRule = iota // No restriction
	LeadNoTrumpUntilBroken            // Can't lead trump until broken
	LeadWinnerLeads                   // RESERVED: inert (see type doc); rejected by Validate
)

var leadRuleNames = [3]string{"none", "no_trump_until_broken", "winner_leads"}

func (l LeadRule) String() string {
	if int(l) >= len(leadRuleNames) {
		return fmt.Sprintf("LeadRule(%d)", l)
	}
	return leadRuleNames[l]
}

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

var meldTypeNames = [3]string{"sets", "runs", "both"}

func (m MeldType) String() string {
	if int(m) >= len(meldTypeNames) {
		return fmt.Sprintf("MeldType(%d)", m)
	}
	return meldTypeNames[m]
}

// DrawSource defines where players can draw from.
type DrawSource uint8

const (
	DrawDeck    DrawSource = iota
	DrawDiscard
	DrawEither
)

var drawSourceNames = [3]string{"deck", "discard", "either"}

func (d DrawSource) String() string {
	if int(d) >= len(drawSourceNames) {
		return fmt.Sprintf("DrawSource(%d)", d)
	}
	return drawSourceNames[d]
}

// RummyParams controls a rummy game.
type RummyParams struct {
	MeldTypes      MeldType   `json:"meld_types"`
	MinMeldSize    int        `json:"min_meld_size"`   // 3-4 (2 is Tier-0 rejected: a 2-card meld is trivially formable)
	DrawFrom       DrawSource `json:"draw_from"`
	KnockThreshold int        `json:"knock_threshold"` // Deadwood to knock (0 = gin only)
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

var trumpRuleNames = [4]string{"none", "fixed", "cut", "led"}

func (t TrumpRule) String() string {
	if int(t) >= len(trumpRuleNames) {
		return fmt.Sprintf("TrumpRule(%d)", t)
	}
	return trumpRuleNames[t]
}

// SpecialCardType defines special card effects.
type SpecialCardType uint8

const (
	SpecialSkip     SpecialCardType = iota // Skip next player
	SpecialReverse                          // Reverse play direction
	SpecialDrawTwo                          // Next player draws 2
	SpecialDrawFour                         // Next player draws 4
	SpecialWild                             // Can be played on anything
)

var specialCardTypeNames = [5]string{"skip", "reverse", "draw_two", "draw_four", "wild"}

func (s SpecialCardType) String() string {
	if int(s) >= len(specialCardTypeNames) {
		return fmt.Sprintf("SpecialCardType(%d)", s)
	}
	return specialCardTypeNames[s]
}

// SpecialCard assigns a special effect to cards matching a condition.
type SpecialCard struct {
	Type     SpecialCardType `json:"type"`
	ByRank   uint8           `json:"by_rank,omitempty"`   // 0 = any rank
	BySuit   uint8           `json:"by_suit,omitempty"`   // 0 = any suit (1-4 = specific)
}

// MatchesCard reports whether this special-card rule applies to a card of
// the given rank and suit. suit is the sim package's 0-indexed value; BySuit
// uses 1-4 with 0 meaning "any suit", ByRank 0 means "any rank" (so a rule
// with both zero is a catch-all that matches every card). SINGLE SOURCE OF
// TRUTH for special-card matching: the shedding runner's effect application
// and the batch runner's choice-impact profiling both delegate here.
func (sc SpecialCard) MatchesCard(rank, suit uint8) bool {
	if sc.ByRank != 0 && sc.ByRank != rank {
		return false
	}
	if sc.BySuit != 0 && sc.BySuit != suit+1 {
		return false
	}
	return true
}

// ScoringEvent defines when points are awarded.
type ScoringEvent uint8

const (
	ScoreOnTrickWin ScoringEvent = iota
	ScoreOnCapture
	ScoreOnPlay
	ScoreOnHandEnd
)

var scoringEventNames = [4]string{"trick_win", "capture", "play", "hand_end"}

func (s ScoringEvent) String() string {
	if int(s) >= len(scoringEventNames) {
		return fmt.Sprintf("ScoringEvent(%d)", s)
	}
	return scoringEventNames[s]
}

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

// MatchCardPoints returns the points awarded for a card under the given
// CardScoring rules. rank and suit are taken straight from sim.Card (suit
// is 0-indexed; CardScoring.Suit uses 1-4 with 0 meaning "any suit").
//
// When multiple rules match the same card, the most specific wins:
//
//	suit+rank > suit-only > rank-only > catch-all
//
// This makes scoring independent of slice order, so mutators or crossover
// that permute the rules cannot accidentally change a card's score.
func MatchCardPoints(rules []CardScoring, rank, suit uint8) int {
	bestPoints := 0
	bestSpecificity := -1
	for _, cp := range rules {
		rankMatch := cp.Rank == 0 || cp.Rank == rank
		suitMatch := cp.Suit == 0 || cp.Suit == suit+1
		if !rankMatch || !suitMatch {
			continue
		}
		spec := 0
		if cp.Suit != 0 {
			spec += 2
		}
		if cp.Rank != 0 {
			spec++
		}
		if spec > bestSpecificity {
			bestSpecificity = spec
			bestPoints = cp.Points
		}
	}
	return bestPoints
}

// --- Borrowed Mechanics ---

// MechanicType identifies a borrowable mechanic.
type MechanicType uint8

// MechTrump and MechPlayMultiple are reserved: they have no hook or
// runner-side implementation and are not in the validBorrows whitelist
// (validation rejects any genome carrying them; see dd-lnh). The constants
// are kept rather than deleted because MechanicType serializes as a bare
// number -- removing them would renumber later values and silently corrupt
// every existing serialized genome.
const (
	MechTrickScoring MechanicType = iota // Score based on tricks won
	MechMeldBonus                         // Bonus for forming melds
	MechDrawPenalty                       // Draw cards as penalty
	MechKnock                             // Knock to end round
	MechTrump                             // reserved: no implementation, not whitelisted
	MechAvoidance                         // Points-are-bad scoring
	MechPlayMultiple                      // reserved: no implementation, not whitelisted
	MechFollowSuit                        // Must follow suit restriction
)

var mechanicNames = [8]string{
	"trick_scoring", "meld_bonus", "draw_penalty", "knock",
	"trump", "avoidance", "play_multiple", "follow_suit",
}

func (m MechanicType) String() string {
	if int(m) >= len(mechanicNames) {
		return fmt.Sprintf("MechanicType(%d)", m)
	}
	return mechanicNames[m]
}

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
	// Fitness is the RAW (unshared) fitness. It must never hold a
	// sharing/novelty-blended score: the published genome.json used to store
	// SharedFitness here while report.md showed raw fitness, and the two
	// contradicted each other (0.41 vs 0.94 -- Task 28 round 2). In
	// PUBLISHED genome.json files this is the greedy-only running mean
	// (Individual.OutputRank, Wave K fix 1), identical to report.md's
	// headline; during a run the in-memory field tracks the published
	// (possibly MCTS-mode) mean for checkpointing.
	Fitness float64 `json:"fitness,omitempty"`
	// SharedFitness is the niche-sharing/novelty-blended selection score, an
	// explicit separate field so the blend is visible without ever
	// masquerading as fitness.
	SharedFitness float64 `json:"shared_fitness,omitempty"`

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

// HasScoringBorrow reports whether g carries a borrowed scoring mechanic
// (MechMeldBonus or MechAvoidance) -- the two hooks that bank points into
// state.Scores on EventRoundEnd.
func (g *Genome) HasScoringBorrow() bool {
	for _, b := range g.Borrowed {
		if b.Mechanic == MechMeldBonus || b.Mechanic == MechAvoidance {
			return true
		}
	}
	return false
}

// SheddingMultiRound reports whether g plays shedding as a series of
// banked-score rounds (audit remediation Task 22): the shedding skeleton with
// RoundsPerGame > 1 AND a scoring borrow present. Without a scoring borrow
// nothing writes state.Scores, so multiple rounds would have no winner
// signal; such genomes stay single-round (first empty hand wins).
func (g *Genome) SheddingMultiRound() bool {
	return g.Skeleton == Shedding &&
		g.Shedding != nil &&
		g.Shedding.RoundsPerGame > 1 &&
		g.HasScoringBorrow()
}

// MaxTurns returns the computed maximum turns based on skeleton and params.
func (g *Genome) MaxTurns() int {
	switch g.Skeleton {
	case Shedding:
		// Worst case: cycling through deck multiple times
		turns := g.HandSize * g.Players * 10
		// Multi-round games play RoundsPerGame hands back to back; without
		// the scale every multi-round genome dies as a Tier-1 timeout. The
		// cap stays unscaled when rounds are inert (no scoring borrow) so
		// pre-Task-22 timeout detection is unchanged.
		if g.SheddingMultiRound() {
			turns *= g.Shedding.RoundsPerGame
		}
		return turns
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
			return fmt.Sprintf("shedding{match=%s, draw=%d}",
				g.Shedding.MatchRule, g.Shedding.DrawPenalty)
		}
	case TrickTaking:
		if g.TrickTaking != nil {
			return fmt.Sprintf("trick{follow=%v, scoring=%s, lead=%s, rounds=%d}",
				g.TrickTaking.MustFollowSuit, g.TrickTaking.TrickScoring,
				g.TrickTaking.LeadRestriction, g.TrickTaking.RoundsPerGame)
		}
	case Rummy:
		if g.Rummy != nil {
			return fmt.Sprintf("rummy{melds=%s, min=%d, draw=%s, knock=%d}",
				g.Rummy.MeldTypes, g.Rummy.MinMeldSize, g.Rummy.DrawFrom,
				g.Rummy.KnockThreshold)
		}
	}
	return "none"
}

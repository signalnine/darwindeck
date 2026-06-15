package genome

import "fmt"

// SkeletonType identifies the primary game skeleton.
type SkeletonType uint8

const (
	Shedding    SkeletonType = iota
	TrickTaking
	Rummy
	Climbing
	Casino
	Vying
)

var skeletonNames = [6]string{"shedding", "trick_taking", "rummy", "climbing", "casino", "vying"}

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

// --- Climbing Parameters ---

// ClimbingParams controls a climbing/ladder game (Big Two / Tichu / President
// family). On your turn you either play a combination of the SAME type as the
// current one but strictly higher rank, or PASS. When every other player passes
// in succession the last player who played leads a fresh combination (the table
// clears). First player to empty their hand wins.
//
// Params are deliberately minimal but expressive: singles are always allowed
// (the playability floor -- a lead can always play a single, so a legal move
// always exists), and pairs/triples/runs are switched on individually.
type ClimbingParams struct {
	// AllowPairs enables two-of-a-kind combinations (same rank).
	AllowPairs bool `json:"allow_pairs"`
	// AllowTriples enables three-of-a-kind combinations (same rank).
	AllowTriples bool `json:"allow_triples"`
	// AllowRuns enables consecutive-rank runs (mixed suits, like Tichu straights)
	// of length >= MinRunLen.
	AllowRuns bool `json:"allow_runs"`
	// MinRunLen is the minimum length of a run combination (3-5). Only consulted
	// when AllowRuns is true.
	MinRunLen int `json:"min_run_len"`
}

// --- Casino Parameters ---

// CasinoParams controls a fishing/capture game (Casino / Scopa family). On your
// turn you play one card from hand and either CAPTURE table cards (any table
// card of the same rank, or -- when AllowSumCapture -- a subset of number cards
// whose pip values sum to your card's value) into your pile, or TRAIL the card
// face-up onto the table. Trailing is always legal, so a legal move always
// exists. Hands are dealt from the stock, refilled when empty, until the stock
// runs out; the last player to capture sweeps any cards left on the table. Most
// captured cards wins. Player count and hand size use the top-level Genome
// fields; TableSize is the face-up cards dealt to the table at the start.
type CasinoParams struct {
	// TableSize is the number of cards dealt face-up to the table at setup
	// (0-6; standard Casino deals 4).
	TableSize int `json:"table_size"`
	// AllowSumCapture enables capturing a subset of number cards (aces..tens)
	// whose pip values sum to the played card's value, not only same-rank
	// captures. This is the decision-rich core of Casino/Scopa; with it off the
	// game degrades to plain rank-matching fishing (Go Fish-like).
	AllowSumCapture bool `json:"allow_sum_capture"`
}

// --- Vying Parameters ---

// VyingParams controls a vying / betting game (poker / brag family). Each deal,
// every player gets HandSize hidden cards; one rotating player posts a big blind
// (MinBet) so the first actor always faces a bet; then a single betting round
// runs (fold / call / raise, raises capped at MaxRaises) and the best poker hand
// among non-folded players takes the pot. Chips carry across RoundsPerGame deals
// and the largest stack wins. StartingChips must cover the worst-case
// commitment across all deals so no all-in / side pot ever arises (enforced in
// validation). Player count and hand size use the top-level Genome fields.
type VyingParams struct {
	// StartingChips is each player's chip stack at the start (e.g. 1000).
	StartingChips int `json:"starting_chips"`
	// MinBet is the big blind and the raise increment (e.g. 10).
	MinBet int `json:"min_bet"`
	// MaxRaises caps raises per betting round so the round always closes
	// (e.g. 3). Bounds the game's length and guarantees termination.
	MaxRaises int `json:"max_raises"`
	// RoundsPerGame is the number of deals played; chips carry over and the
	// biggest stack wins. Tuned so total decisions land in the session-length
	// band.
	RoundsPerGame int `json:"rounds_per_game"`
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
	MechFollowSuit                        // DEEP borrow: must follow the discard suit (shedding runner)
	MechRunPlay                           // DEEP borrow: multi-card combo discards (shedding runner)
)

var mechanicNames = [9]string{
	"trick_scoring", "meld_bonus", "draw_penalty", "knock",
	"trump", "avoidance", "play_multiple", "follow_suit", "run_play",
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

	// VetoStable / StableEvals record the Wave M publication-integrity check
	// (audit Task 28/29 follow-up). Production publishes a genome from a
	// SINGLE evaluation, so a genome that fails its own degeneracy veto (or
	// Tier-1 kill) on a minority of seeds can still land in the top-N -- the
	// r4 rank02 shedding genome failed greedy_longest_run on 1/10 seeds yet
	// published as rank 2. Before SaveResults writes the top-N, each genome is
	// RE-EVALUATED at K distinct fresh seeds; VetoStable is true iff a majority
	// of those re-evals stayed valid, and StableEvals is the literal "N/K"
	// count. Unstable genomes are demoted below the stable ones in the
	// published order. These fields appear only in PUBLISHED genome.json files;
	// they are output-path metadata, not evolutionary state, and never feed
	// selection or the metric stack (which is frozen). Empty/false when the
	// stability check did not run (e.g. legacy bundles).
	VetoStable  bool   `json:"veto_stable,omitempty"`
	StableEvals string `json:"stable_evals,omitempty"`

	// Shared parameters
	Players  int `json:"players"`   // 2-6
	HandSize int `json:"hand_size"` // 3-13

	// Skeleton-specific params (only one is active based on Skeleton)
	Shedding    *SheddingParams    `json:"shedding,omitempty"`
	TrickTaking *TrickTakingParams `json:"trick_taking,omitempty"`
	Rummy       *RummyParams       `json:"rummy,omitempty"`
	Climbing    *ClimbingParams    `json:"climbing,omitempty"`
	Casino      *CasinoParams      `json:"casino,omitempty"`
	Vying       *VyingParams       `json:"vying,omitempty"`

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
	if g.Climbing != nil {
		c := *g.Climbing
		cp.Climbing = &c
	}
	if g.Casino != nil {
		c := *g.Casino
		cp.Casino = &c
	}
	if g.Vying != nil {
		v := *g.Vying
		cp.Vying = &v
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

// CasinoScored reports whether g is a casino host carrying a scoring borrow
// (MechMeldBonus or MechAvoidance) -- a fishing/capture game scored Scopa-style.
// The casino runner gates its single end-of-game EventRoundEnd and early table
// sweep on this predicate: that one event lets the borrow's hook bank the full
// captured pile (state.Tableau) into state.Scores ONCE, and CheckEnd then picks
// the winner by captured-card COUNT plus that banked meld bonus (or minus the
// avoidance penalty) instead of by raw count. An unscored casino is
// byte-identical (no EventRoundEnd, sweep stays in Upkeep). Like the
// trick-taking and rummy hosts, casino reads state.Scores in CheckEnd, so the
// scoring borrow is live at casino's single end-of-game tally (LiveBorrows only
// prunes such borrows on a single-round Shedding host).
func (g *Genome) CasinoScored() bool {
	return g.Skeleton == Casino && g.HasScoringBorrow()
}

// HasBankingBorrow reports whether g carries ANY borrow whose hook banks into
// state.Scores at round end -- the scoring borrows (MechMeldBonus,
// MechAvoidance) PLUS MechTrickScoring (the cross-skeleton hybrid borrow:
// applyTrickScoring banks a per-round capture bonus). This is the predicate
// the shedding runner uses to decide multi-round banked-score play, broader
// than HasScoringBorrow because the trick-scoring hybrid also needs the rounds
// machinery to give its banked points a winner signal. HasScoringBorrow is
// kept narrow because LiveBorrows uses it specifically for the
// MeldBonus/Avoidance "needs multi-round to be live" rule (those two no-op on
// a single-round host), whereas a trick-scoring borrow is always wired through
// the multi-round path here.
func (g *Genome) HasBankingBorrow() bool {
	for _, b := range g.Borrowed {
		switch b.Mechanic {
		case MechMeldBonus, MechAvoidance, MechTrickScoring:
			return true
		}
	}
	return false
}

// SheddingTrickScored reports whether g is a shedding host carrying a borrowed
// MechTrickScoring mechanic -- the headline cross-skeleton hybrid, a
// shed-to-win game scored by tricks. The shedding runner records each shed
// card into the player's tableau ONLY under this predicate so applyTrickScoring
// (which counts tableau captures) has a per-player signal, without disturbing
// the MeldBonus/Avoidance shedding borrows that read tableau differently.
func (g *Genome) SheddingTrickScored() bool {
	if g.Skeleton != Shedding {
		return false
	}
	for _, b := range g.Borrowed {
		if b.Mechanic == MechTrickScoring {
			return true
		}
	}
	return false
}

// ComboPlay reports whether g is a shedding host carrying a MechRunPlay borrow
// -- a DEEP cross-skeleton borrow (climbing's multi-card combinations ->
// shedding) that changes the LEGAL-MOVE set inside the shedding runner: in
// addition to single-card discards, a player may dump a same-rank set (2+) or
// a same-suit consecutive run (2+) in one turn (when the group matches the
// discard top), so hand reduction is lumpy and combinatorial -- you hold cards
// to unload a run. It is a pure SUPERSET of the normal move set (singles and
// draw/pass remain), so playability and termination are preserved by
// construction and games complete at least as often as plain shedding. Because
// it acts directly inside GenerateMoves/ApplyMove (like MechDrawPenalty, never
// banking) it is always live (LiveBorrows) and works on a single-round host.
// Lives in the runner, never a hook (the hook system has no move extension
// point).
func (g *Genome) ComboPlay() bool {
	if g.Skeleton != Shedding {
		return false
	}
	for _, b := range g.Borrowed {
		if b.Mechanic == MechRunPlay {
			return true
		}
	}
	return false
}

// FollowConstrained reports whether g is a shedding host carrying a
// MechFollowSuit borrow -- a DEEP cross-skeleton borrow (trick-taking's
// follow-suit obligation -> shedding). Under it, if you hold a card of the
// discard top's suit you MUST play one (or a wild); only when void in that suit
// do the normal match plays and the draw reopen. This changes the legal-move set
// inside the shedding runner (a constraint, the mirror of MechRunPlay's
// expansion): hand management becomes about voiding suits, and opponents can
// pin you to a suit. Playability holds -- a forced play still sheds a card, and
// a void hand falls through to the normal moves + draw, so GenerateMoves is
// never empty. Acts directly in GenerateMoves (like MechRunPlay), so it is
// always live and lives in the runner, never a hook.
func (g *Genome) FollowConstrained() bool {
	if g.Skeleton != Shedding {
		return false
	}
	for _, b := range g.Borrowed {
		if b.Mechanic == MechFollowSuit {
			return true
		}
	}
	return false
}

// Knockable reports whether g is a shedding OR climbing host carrying a
// MechKnock borrow -- a DEEP cross-skeleton borrow (rummy's knock) that changes
// the WIN condition. When your hand is small you may KNOCK to end the game
// immediately; the fewest-cards player then wins (CheckEnd), so a wrong knock
// (you are not actually fewest) hands the win to someone else -- a real risk
// decision the plain first-to-empty race lacks. Whitelisted on the two
// empty-hand-race skeletons (shedding and climbing), where "fewest cards" is a
// meaningful lead. The MoveKnock is additive (every other move remains) and can
// only END the game sooner, so playability and termination are preserved. Acts
// directly in the runner (GenerateMoves/ApplyMove/CheckEnd), not a hook.
func (g *Genome) Knockable() bool {
	if g.Skeleton != Shedding && g.Skeleton != Climbing {
		return false
	}
	for _, b := range g.Borrowed {
		if b.Mechanic == MechKnock {
			return true
		}
	}
	return false
}

// SheddingMultiRound reports whether g plays shedding as a series of
// banked-score rounds (audit remediation Task 22): the shedding skeleton with
// RoundsPerGame > 1 AND a banking borrow present. Without a banking borrow
// nothing writes state.Scores, so multiple rounds would have no winner
// signal; such genomes stay single-round (first empty hand wins). The banking
// set was widened from HasScoringBorrow to HasBankingBorrow when the
// cross-skeleton MechTrickScoring borrow was enabled on shedding (the
// shed-to-win-by-tricks hybrid): its applyTrickScoring hook banks per round
// and needs the same rounds machinery.
func (g *Genome) SheddingMultiRound() bool {
	return g.Skeleton == Shedding &&
		g.Shedding != nil &&
		g.Shedding.RoundsPerGame > 1 &&
		g.HasBankingBorrow()
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
	case Climbing:
		// Every play strictly shrinks a hand, and passes resolve to a fresh
		// lead within at most Players consecutive moves, so the game cannot
		// outlast (plays + passes). Worst case: every card dribbles out one at
		// a time (HandSize*Players plays) interleaved with a full pass-around
		// per trick. Scale generously so the turn-cap backstop never
		// misclassifies a healthy climbing game as a timeout.
		return g.HandSize * g.Players * 8
	case Casino:
		// Every turn plays exactly one card from a hand; hands refill only from
		// the finite stock, so total plays <= 52 (every card played once).
		// Scale generously so the cap is never the binding constraint.
		return 52 * 4
	case Vying:
		// Each deal runs one betting round whose actions are bounded by the
		// raise cap (every non-folded player acts O(MaxRaises+1) times), and
		// RoundsPerGame deals run back to back. Scale generously so the backstop
		// never binds on a healthy game (real termination is structural).
		if g.Vying != nil {
			perDeal := g.Players * (g.Vying.MaxRaises + 3)
			return g.Vying.RoundsPerGame * perDeal * 2
		}
		return g.Players * 60
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
	case Climbing:
		if g.Climbing != nil {
			return fmt.Sprintf("climbing{pairs=%v, triples=%v, runs=%v, min_run=%d}",
				g.Climbing.AllowPairs, g.Climbing.AllowTriples,
				g.Climbing.AllowRuns, g.Climbing.MinRunLen)
		}
	case Casino:
		if g.Casino != nil {
			return fmt.Sprintf("casino{table=%d, sum_capture=%v}",
				g.Casino.TableSize, g.Casino.AllowSumCapture)
		}
	case Vying:
		if g.Vying != nil {
			return fmt.Sprintf("vying{chips=%d, min_bet=%d, max_raises=%d, rounds=%d}",
				g.Vying.StartingChips, g.Vying.MinBet, g.Vying.MaxRaises, g.Vying.RoundsPerGame)
		}
	}
	return "none"
}

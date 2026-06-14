package genome

import (
	"fmt"
	"sort"
)

// Validate performs Tier 0 static analysis on a genome.
// Returns a list of violations, or nil if valid.
func Validate(g *Genome) []string {
	var errs []string

	// Player count
	if g.Players < 2 || g.Players > 6 {
		errs = append(errs, fmt.Sprintf("players must be 2-6, got %d", g.Players))
	}

	// Hand size
	if g.HandSize < 3 || g.HandSize > 13 {
		errs = append(errs, fmt.Sprintf("hand_size must be 3-13, got %d", g.HandSize))
	}

	// Total cards dealt must fit in 52-card deck
	if g.HandSize*g.Players > 52 {
		errs = append(errs, fmt.Sprintf("hand_size(%d) * players(%d) = %d exceeds 52-card deck",
			g.HandSize, g.Players, g.HandSize*g.Players))
	}

	// Must have active skeleton params matching skeleton type
	switch g.Skeleton {
	case Shedding:
		if g.Shedding == nil {
			errs = append(errs, "shedding skeleton requires shedding params")
		} else {
			errs = append(errs, validateShedding(g.Shedding)...)
		}
	case TrickTaking:
		if g.TrickTaking == nil {
			errs = append(errs, "trick_taking skeleton requires trick_taking params")
		} else {
			errs = append(errs, validateTrickTaking(g.TrickTaking)...)
		}
	case Rummy:
		if g.Rummy == nil {
			errs = append(errs, "rummy skeleton requires rummy params")
		} else {
			errs = append(errs, validateRummy(g.Rummy)...)
		}
	case Climbing:
		if g.Climbing == nil {
			errs = append(errs, "climbing skeleton requires climbing params")
		} else {
			errs = append(errs, validateClimbing(g.Climbing)...)
		}
	default:
		errs = append(errs, fmt.Sprintf("unknown skeleton type: %d", g.Skeleton))
	}

	// Validate trump rule coherence
	if g.TrumpRule != TrumpNone && g.Skeleton == Rummy {
		errs = append(errs, "trump rule not applicable to rummy skeleton")
	}
	// Climbing has no trump concept (combinations beat by rank within the same
	// type, never by suit); a trump rule on a climbing host is an inert bit that
	// the rulebook would still render, so reject it at Tier 0 (mirrors the rummy
	// rule above).
	if g.TrumpRule != TrumpNone && g.Skeleton == Climbing {
		errs = append(errs, "trump rule not applicable to climbing skeleton")
	}

	// Special cards are only consumed by the shedding runner. On any other
	// skeleton they are inert bits that still get rendered in the rulebook, so
	// reject them at Tier 0 rather than ship a game that lies about its rules
	// (dd-24e).
	if len(g.SpecialCards) > 0 && g.Skeleton != Shedding {
		errs = append(errs, fmt.Sprintf("special cards only supported by shedding skeleton, got %d on %s",
			len(g.SpecialCards), g.Skeleton))
	}

	// Catch-all specials are a LIVENESS violation (Task 28 round 3): a rule
	// with no qualifier (ByRank == 0 AND BySuit == 0) matches EVERY card
	// (SpecialCard.MatchesCard), which statically deletes the skeleton's core
	// rules -- a catch-all wild makes match_rule and draw_penalty dead genes
	// (every card is always playable) and a catch-all effect fires on every
	// single play. "Parameters control what happens, not whether the game
	// works"; a parameter that erases other parameters breaks that contract,
	// so it is rejected at Tier 0 rather than left for the dynamic vetoes to
	// catch one champion at a time (the round-2 flagship's entire shedding
	// top 10 rode a catch-all wild). Mutation never generates the encoding
	// (addSpecialCard forces a suit qualifier when ByRank is 0) and crossover
	// copies special-card slices wholesale, so valid parents cannot produce
	// it.
	for i, sc := range g.SpecialCards {
		if sc.ByRank == 0 && sc.BySuit == 0 {
			errs = append(errs, fmt.Sprintf(
				"special card %d (%s) is a catch-all (by_rank=0, by_suit=0 matches every card): it deletes the skeleton's match/draw rules; qualify it by rank and/or suit", i, sc.Type))
		}
	}

	// Validate borrowed mechanics
	errs = append(errs, validateBorrowed(g)...)

	// Validate scoring coherence
	errs = append(errs, validateScoring(g)...)

	return errs
}

func validateShedding(p *SheddingParams) []string {
	var errs []string
	if p.MatchRule > MatchBoth {
		errs = append(errs, fmt.Sprintf("invalid match_rule: %d", p.MatchRule))
	}
	if p.DrawPenalty < 1 || p.DrawPenalty > 3 {
		errs = append(errs, fmt.Sprintf("draw_penalty must be 1-3, got %d", p.DrawPenalty))
	}
	// 0 is the legacy "unset" encoding (pre-Task-22 genomes carry it) and is
	// treated as 1 round; mutation only produces 1-5.
	if p.RoundsPerGame < 0 || p.RoundsPerGame > 5 {
		errs = append(errs, fmt.Sprintf("rounds_per_game must be 0-5 (0 = unset/1), got %d", p.RoundsPerGame))
	}
	return errs
}

func validateTrickTaking(p *TrickTakingParams) []string {
	var errs []string
	if p.TrickScoring > ScoreAvoidance {
		errs = append(errs, fmt.Sprintf("invalid trick_scoring: %d", p.TrickScoring))
	}
	if p.LeadRestriction > LeadWinnerLeads {
		errs = append(errs, fmt.Sprintf("invalid lead_restriction: %d", p.LeadRestriction))
	}
	// LeadWinnerLeads is reserved: winner-leads is the skeleton's fixed turn
	// order (hardcoded in the runner), so the value is inert -- a genome
	// carrying it advertises a parameter that cannot do anything (dd-027
	// class; see the LeadRule type doc).
	if p.LeadRestriction == LeadWinnerLeads {
		errs = append(errs, "lead_restriction 2 (winner_leads) is reserved: winner-leads is the trick-taking skeleton's fixed turn order, the value is inert")
	}
	if p.RoundsPerGame < 1 || p.RoundsPerGame > 13 {
		errs = append(errs, fmt.Sprintf("rounds_per_game must be 1-13, got %d", p.RoundsPerGame))
	}
	return errs
}

func validateRummy(p *RummyParams) []string {
	var errs []string
	if p.MeldTypes > MeldBoth {
		errs = append(errs, fmt.Sprintf("invalid meld_types: %d", p.MeldTypes))
	}
	// min_meld_size floor raised 2 -> 3 (Task 28 round 4, trivial-meld
	// liveness). A 2-card "meld" is trivially formable for either meld type
	// -- any two same-rank cards are a 2-set, any two sequential same-suit
	// cards are a 2-run -- so melding is consequence-free and the rummy
	// skeleton's deadwood economy never bites (the runs-only-pair-meld
	// flagship champions r3 rank23/rank27 reached deadwood ~0 by turn 7).
	// This is the Tier-0 twin of the catch-all-special liveness rule: a
	// parameter that erases the skeleton's core hold-vs-meld decision breaks
	// the "parameters control what happens, not whether the game works"
	// contract. Real rummy melds are always 3+, so no classic seed (gin/knock
	// use 3) or genuinely-borderline champion (all r3 keepers use 3) is
	// affected; mutation clamps to 3-4 so the engine never generates it.
	if p.MinMeldSize < 3 || p.MinMeldSize > 4 {
		errs = append(errs, fmt.Sprintf("min_meld_size must be 3-4 (a 2-card meld is trivially formable; melding must carry consequence), got %d", p.MinMeldSize))
	}
	if p.DrawFrom > DrawEither {
		errs = append(errs, fmt.Sprintf("invalid draw_from: %d", p.DrawFrom))
	}
	if p.KnockThreshold < 0 || p.KnockThreshold > 100 {
		errs = append(errs, fmt.Sprintf("knock_threshold must be 0-100, got %d", p.KnockThreshold))
	}
	return errs
}

func validateClimbing(p *ClimbingParams) []string {
	var errs []string
	// MinRunLen is only consulted when AllowRuns is true. Constrain it to 3-5
	// (a 2-card "run" is a trivially-formable pair-shaped combination that
	// would collide with the pair type and erode the run decision). When runs
	// are off, MinRunLen is inert; mutation still keeps it in range so a later
	// AllowRuns flip lands on a valid length, so validate it unconditionally.
	if p.AllowRuns {
		if p.MinRunLen < 3 || p.MinRunLen > 5 {
			errs = append(errs, fmt.Sprintf("min_run_len must be 3-5 when runs are allowed, got %d", p.MinRunLen))
		}
	} else if p.MinRunLen != 0 && (p.MinRunLen < 3 || p.MinRunLen > 5) {
		// Runs off: 0 is the natural "unset" encoding; otherwise keep it in the
		// 3-5 band so it is ready if runs are switched on.
		errs = append(errs, fmt.Sprintf("min_run_len must be 0 (unset) or 3-5, got %d", p.MinRunLen))
	}
	return errs
}

// validBorrows defines which mechanics can be borrowed by which skeletons.
// Only mechanics with runner-side implementations belong here -- whitelisting
// a no-op borrow wastes evolutionary search dimensions and produces rulebooks
// that lie about behaviour (see dd-lnh). MechTrump and MechPlayMultiple are
// reserved enum values with no implementation and must never be added here.
var validBorrows = map[SkeletonType]map[MechanicType]bool{
	Shedding: {
		MechMeldBonus: true, // Bonus for playing sets/runs (HookEndOfRound)
		MechAvoidance: true, // Penalty cards (HookScoring)
		// MechTrickScoring: the headline cross-skeleton hybrid (novelty
		// evolution). A shed-to-win game scored by tricks -- not a faithful
		// rediscovery of any single classic. applyTrickScoring banks a
		// per-round capture bonus into state.Scores; the shedding runner
		// records each shed card into the player's tableau under
		// SheddingTrickScored() so the hook has a per-player signal, and
		// SheddingMultiRound() (via HasBankingBorrow) routes the host through
		// the banked-score rounds machinery that reads those scores in
		// CheckEnd. The hook fires and affects the winner -- it is NOT a
		// reserved/no-hook mechanic.
		MechTrickScoring: true,
		// MechRunPlay: a DEEP cross-skeleton borrow (climbing's multi-card
		// combinations -> shedding). Unlike the three banking borrows above it
		// does NOT touch state.Scores: it changes the LEGAL-MOVE set INSIDE the
		// shedding runner (GenerateMoves adds same-rank set and same-suit run
		// discards of 2+ cards that match the top, so hand reduction is lumpy).
		// It is a pure SUPERSET of the normal moves, so it affects WHO empties
		// first (the winner) while preserving the playability/termination
		// floor -- whitelisted per dd-lnh. Implemented in the runner, NOT a
		// hook: the hook system (AfterPlay/EndOfRound/Scoring, fired post-move)
		// cannot change the move set.
		MechRunPlay: true,
	},
	TrickTaking: {
		MechMeldBonus: true, // Bonus for collecting melds from tricks (HookEndOfRound)
		// MechAvoidance: a cross-family ACTIVE scoring borrow (novelty
		// evolution). applyAvoidance subtracts penalty points for cards the
		// player captured into their tableau at round end; findWinner reads
		// state.Scores, so the borrow affects the winner. Distinct from the
		// trick_scoring=avoidance PARAMETER (which scores the tricks
		// themselves): this borrows rummy/shedding-style penalty-card scoring
		// ON TOP of whatever trick scoring the host already runs, a genuine
		// cross-family combination. Requires non-empty CardPoints (the hook
		// no-ops without them); the hybrid builder and mutation seed a default.
		MechAvoidance: true,
		// MechDrawPenalty intentionally NOT borrowable here: applyDrawPenalty
		// appends a card to the active player's hand on face-card plays, which
		// breaks the trick-taking runner's "all hands empty at round end"
		// invariant and causes ~99% timeout rates (dd-wfi).
	},
	Rummy: {
		MechTrickScoring: true, // Score based on some trick-like mechanic
		MechDrawPenalty:  true, // Extra draw penalty
		MechAvoidance:    true, // Certain cards are penalties
	},
	Climbing: {
		// MechDrawPenalty is the ONLY borrow whitelisted on climbing, and it is
		// whitelisted because its hook (applyDrawPenalty) both FIRES and AFFECTS
		// THE WINNER on a climbing host (novelty evolution): applyDrawPenalty
		// fires on EventCardPlayed (which climbing emits on every play) and
		// appends a card to the player's hand on face-card plays. Climbing's
		// winner is first-to-empty-hand (hand-based, NOT state.Scores), so a
		// hook that GROWS a hand directly slows that player's race to empty --
		// outcome-affecting by construction. The banking-scoring borrows
		// (MechMeldBonus / MechAvoidance / MechTrickScoring) are deliberately
		// NOT whitelisted here: they bank into state.Scores, which a climbing
		// CheckEnd never reads, so their hooks would be inert no-ops on this host
		// -- exactly the dd-lnh "don't whitelist a no-op borrow" rule. A
		// climbing-with-meld-bonuses hybrid would need a banked-score climbing
		// variant first (NOTE for a later wave).
		MechDrawPenalty: true,
	},
}

// ValidBorrows returns the borrow whitelist as a fresh copy: for each host
// skeleton, the mechanics it may borrow, sorted by enum value so iteration is
// deterministic. It is the single source of truth for downstream consumers --
// the borrow integration tests in pkg/fitness derive their case lists from it
// so coverage cannot drift from validation (audit remediation Task 26), and
// any CLI tooling that needs the table should call this rather than re-encode
// the map. Mutating the returned value does not affect validation.
func ValidBorrows() map[SkeletonType][]MechanicType {
	out := make(map[SkeletonType][]MechanicType, len(validBorrows))
	for skel, mechs := range validBorrows {
		list := make([]MechanicType, 0, len(mechs))
		for m, ok := range mechs {
			if ok {
				list = append(list, m)
			}
		}
		sort.Slice(list, func(i, j int) bool { return list[i] < list[j] })
		out[skel] = list
	}
	return out
}

func validateBorrowed(g *Genome) []string {
	var errs []string
	allowed := validBorrows[g.Skeleton]

	for _, b := range g.Borrowed {
		if b.Source == g.Skeleton {
			errs = append(errs, fmt.Sprintf("cannot borrow from own skeleton: %s", g.Skeleton))
			continue
		}
		if !allowed[b.Mechanic] {
			errs = append(errs, fmt.Sprintf("mechanic %d not borrowable by %s skeleton", b.Mechanic, g.Skeleton))
		}
	}

	// Check for duplicate borrows (keyed on full (Source, Mechanic) pair).
	type borrowKey struct {
		source   SkeletonType
		mechanic MechanicType
	}
	seen := make(map[borrowKey]bool)
	for _, b := range g.Borrowed {
		k := borrowKey{b.Source, b.Mechanic}
		if seen[k] {
			errs = append(errs, fmt.Sprintf("duplicate borrowed mechanic: %d from %s", b.Mechanic, b.Source))
		}
		seen[k] = true
	}

	return errs
}

func validateScoring(g *Genome) []string {
	var errs []string

	// If trick-taking uses card points or avoidance, must have scoring config
	if g.Skeleton == TrickTaking && g.TrickTaking != nil {
		if g.TrickTaking.TrickScoring == ScoreCardPoints || g.TrickTaking.TrickScoring == ScoreAvoidance {
			if len(g.Scoring.CardPoints) == 0 {
				errs = append(errs, "card_points/avoidance scoring requires card_points in scoring config")
			}
		}
	}

	// Fixed trump must specify suit
	if g.TrumpRule == TrumpFixed && g.Scoring.TrumpSuit == 0 {
		errs = append(errs, "fixed trump rule requires trump_suit in scoring config (1-4)")
	}
	if g.TrumpRule == TrumpFixed && g.Scoring.TrumpSuit > 4 {
		errs = append(errs, fmt.Sprintf("trump_suit must be 1-4, got %d", g.Scoring.TrumpSuit))
	}

	return errs
}

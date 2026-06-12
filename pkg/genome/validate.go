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
	default:
		errs = append(errs, fmt.Sprintf("unknown skeleton type: %d", g.Skeleton))
	}

	// Validate trump rule coherence
	if g.TrumpRule != TrumpNone && g.Skeleton == Rummy {
		errs = append(errs, "trump rule not applicable to rummy skeleton")
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
	if p.MinMeldSize < 2 || p.MinMeldSize > 4 {
		errs = append(errs, fmt.Sprintf("min_meld_size must be 2-4, got %d", p.MinMeldSize))
	}
	if p.DrawFrom > DrawEither {
		errs = append(errs, fmt.Sprintf("invalid draw_from: %d", p.DrawFrom))
	}
	if p.KnockThreshold < 0 || p.KnockThreshold > 100 {
		errs = append(errs, fmt.Sprintf("knock_threshold must be 0-100, got %d", p.KnockThreshold))
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
	},
	TrickTaking: {
		MechMeldBonus: true, // Bonus for collecting melds from tricks
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

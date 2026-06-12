package evolution

import (
	"math/rand/v2"

	"github.com/darwindeck/darwindeck/pkg/genome"
)

// Crossover performs uniform crossover between two same-skeleton genomes.
// Each parameter comes from parent A or B with 50/50 probability.
// Returns nil if skeletons don't match.
func Crossover(a, b *genome.Genome, rng *rand.Rand) *genome.Genome {
	if a.Skeleton != b.Skeleton {
		return nil // No cross-skeleton crossover
	}

	child := cloneGenome(a)
	child.ID = ""
	// Mutate (called by the caller after crossover) is responsible for the
	// final Generation++. Set the base to the higher parent generation so
	// the post-Mutate value is max(a, b) + 1.
	child.Generation = max(a.Generation, b.Generation)

	// Shared params: coin flip each
	if rng.Float64() < 0.5 {
		child.Players = b.Players
	}
	if rng.Float64() < 0.5 {
		child.HandSize = b.HandSize
	}
	if rng.Float64() < 0.5 {
		child.TrumpRule = b.TrumpRule
	}

	// Skeleton-specific crossover
	switch child.Skeleton {
	case genome.Shedding:
		crossoverShedding(child, a, b, rng)
	case genome.TrickTaking:
		crossoverTrickTaking(child, a, b, rng)
	case genome.Rummy:
		crossoverRummy(child, a, b, rng)
	}

	// Special cards: take from one parent. Only shedding consumes them, and
	// crossover is same-skeleton, so a non-shedding child must never inherit
	// them (dd-24e).
	if child.Skeleton == genome.Shedding && rng.Float64() < 0.5 {
		child.SpecialCards = cloneSpecialCards(b.SpecialCards)
	}

	// Borrowed: take from one parent
	if rng.Float64() < 0.5 {
		child.Borrowed = cloneBorrowed(b.Borrowed)
	}

	// Scoring: take from one parent. Deep-copy the CardPoints slice so
	// subsequent in-place mutation on the child does not alias parent B's
	// backing array (parent A is already deep-copied via cloneGenome).
	if rng.Float64() < 0.5 {
		child.Scoring = genome.ScoringConfig{
			CardPoints: append([]genome.CardScoring(nil), b.Scoring.CardPoints...),
			TrumpSuit:  b.Scoring.TrumpSuit,
		}
	}

	repairCrossoverInvariants(child, a, b, rng)

	return child
}

// repairCrossoverInvariants restores genome invariants that independent
// coin flips on related fields can violate. Without this, callers that
// run Crossover without a following Mutate (test code, future seeding
// paths) get children that genome.Validate rejects (see dd-kcp).
func repairCrossoverInvariants(child, a, b *genome.Genome, rng *rand.Rand) {
	// HandSize * Players must fit in a 52-card deck. The standard
	// engine path relies on Mutate to clamp this; mirror that loop
	// here so Crossover's contract is "valid in, valid out."
	for child.HandSize*child.Players > 52 {
		if rng.Float64() < 0.5 {
			child.HandSize--
		} else {
			child.Players--
		}
		if child.HandSize < 3 {
			child.HandSize = 3
		}
		if child.Players < 2 {
			child.Players = 2
		}
	}

	// TrumpFixed must be paired with a valid TrumpSuit (1-4). The
	// TrumpRule and Scoring coin flips are independent, so a child
	// can pick TrumpFixed from one parent and TrumpSuit=0 from the
	// other. Pull the suit from whichever parent contributed the
	// TrumpFixed rule; if neither did, downgrade the rule.
	if child.TrumpRule == genome.TrumpFixed &&
		(child.Scoring.TrumpSuit < 1 || child.Scoring.TrumpSuit > 4) {
		switch {
		case a.TrumpRule == genome.TrumpFixed && a.Scoring.TrumpSuit >= 1 && a.Scoring.TrumpSuit <= 4:
			child.Scoring.TrumpSuit = a.Scoring.TrumpSuit
		case b.TrumpRule == genome.TrumpFixed && b.Scoring.TrumpSuit >= 1 && b.Scoring.TrumpSuit <= 4:
			child.Scoring.TrumpSuit = b.Scoring.TrumpSuit
		default:
			child.TrumpRule = genome.TrumpNone
		}
	}

	// ScoreCardPoints / ScoreAvoidance require non-empty CardPoints.
	// The TrickScoring coin flip lives in crossoverTrickTaking; the
	// CardPoints coin flip lives in the Scoring block above. They can
	// disagree (e.g. take Hearts' ScoreCardPoints with Whist's empty
	// CardPoints). Recover by adopting a non-empty CardPoints slice
	// from whichever parent had this scoring rule; otherwise fall
	// back to ScorePerTrick which has no card-points dependency.
	if child.Skeleton == genome.TrickTaking && child.TrickTaking != nil {
		ts := child.TrickTaking.TrickScoring
		needsPoints := ts == genome.ScoreCardPoints || ts == genome.ScoreAvoidance
		if needsPoints && len(child.Scoring.CardPoints) == 0 {
			switch {
			case a.TrickTaking != nil && a.TrickTaking.TrickScoring == ts && len(a.Scoring.CardPoints) > 0:
				child.Scoring.CardPoints = append([]genome.CardScoring(nil), a.Scoring.CardPoints...)
			case b.TrickTaking != nil && b.TrickTaking.TrickScoring == ts && len(b.Scoring.CardPoints) > 0:
				child.Scoring.CardPoints = append([]genome.CardScoring(nil), b.Scoring.CardPoints...)
			default:
				child.TrickTaking.TrickScoring = genome.ScorePerTrick
			}
		}
	}
}

func crossoverShedding(child *genome.Genome, a, b *genome.Genome, rng *rand.Rand) {
	if a.Shedding == nil || b.Shedding == nil {
		return
	}
	if rng.Float64() < 0.5 {
		child.Shedding.MatchRule = b.Shedding.MatchRule
	}
	if rng.Float64() < 0.5 {
		child.Shedding.DrawPenalty = b.Shedding.DrawPenalty
	}
	if rng.Float64() < 0.5 {
		child.Shedding.RoundsPerGame = b.Shedding.RoundsPerGame
	}
}

func crossoverTrickTaking(child *genome.Genome, a, b *genome.Genome, rng *rand.Rand) {
	if a.TrickTaking == nil || b.TrickTaking == nil {
		return
	}
	if rng.Float64() < 0.5 {
		child.TrickTaking.MustFollowSuit = b.TrickTaking.MustFollowSuit
	}
	if rng.Float64() < 0.5 {
		child.TrickTaking.TrickScoring = b.TrickTaking.TrickScoring
	}
	if rng.Float64() < 0.5 {
		child.TrickTaking.LeadRestriction = b.TrickTaking.LeadRestriction
	}
	if rng.Float64() < 0.5 {
		child.TrickTaking.RoundsPerGame = b.TrickTaking.RoundsPerGame
	}
}

func crossoverRummy(child *genome.Genome, a, b *genome.Genome, rng *rand.Rand) {
	if a.Rummy == nil || b.Rummy == nil {
		return
	}
	if rng.Float64() < 0.5 {
		child.Rummy.MeldTypes = b.Rummy.MeldTypes
	}
	if rng.Float64() < 0.5 {
		child.Rummy.MinMeldSize = b.Rummy.MinMeldSize
	}
	if rng.Float64() < 0.5 {
		child.Rummy.DrawFrom = b.Rummy.DrawFrom
	}
	if rng.Float64() < 0.5 {
		child.Rummy.KnockThreshold = b.Rummy.KnockThreshold
	}
}

func cloneSpecialCards(cards []genome.SpecialCard) []genome.SpecialCard {
	if cards == nil {
		return nil
	}
	out := make([]genome.SpecialCard, len(cards))
	copy(out, cards)
	return out
}

func cloneBorrowed(borrows []genome.BorrowedMechanic) []genome.BorrowedMechanic {
	if borrows == nil {
		return nil
	}
	out := make([]genome.BorrowedMechanic, len(borrows))
	copy(out, borrows)
	return out
}

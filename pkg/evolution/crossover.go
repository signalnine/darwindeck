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
	child.Generation = max(a.Generation, b.Generation) + 1

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

	// Special cards: take from one parent
	if rng.Float64() < 0.5 {
		child.SpecialCards = cloneSpecialCards(b.SpecialCards)
	}

	// Borrowed: take from one parent
	if rng.Float64() < 0.5 {
		child.Borrowed = cloneBorrowed(b.Borrowed)
	}

	// Scoring: take from one parent
	if rng.Float64() < 0.5 {
		child.Scoring = b.Scoring
	}

	return child
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
		child.Shedding.CanStack = b.Shedding.CanStack
	}
	if rng.Float64() < 0.5 {
		child.Shedding.PlayMultiple = b.Shedding.PlayMultiple
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
		child.Rummy.CanLayOff = b.Rummy.CanLayOff
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

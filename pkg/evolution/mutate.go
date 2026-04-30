package evolution

import (
	"fmt"
	"math/rand/v2"

	"github.com/darwindeck/darwindeck/pkg/genome"
)

// Mutate applies random mutations to a genome copy.
// Multiple mutations can fire independently.
func Mutate(g *genome.Genome, rng *rand.Rand, allSeeds []*genome.Genome) *genome.Genome {
	child := cloneGenome(g)
	child.ID = fmt.Sprintf("gen%d_%d", child.Generation+1, rng.IntN(100000))
	child.Generation++

	// Each mutation fires independently
	if rng.Float64() < 0.40 {
		tweakParameter(child, rng)
	}
	// Enforce hand_size * players <= 52 after player/hand mutations
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
	if rng.Float64() < 0.15 {
		flipBool(child, rng)
	}
	if rng.Float64() < 0.15 {
		changeEnum(child, rng)
	}
	if rng.Float64() < 0.08 {
		addSpecialCard(child, rng)
	}
	if rng.Float64() < 0.07 {
		removeSpecialCard(child, rng)
	}
	if rng.Float64() < 0.05 {
		addBorrowedMechanic(child, rng)
	}
	if rng.Float64() < 0.05 {
		removeBorrowedMechanic(child, rng)
	}
	if rng.Float64() < 0.02 {
		changeSkeleton(child, rng, allSeeds)
	}
	if rng.Float64() < 0.03 {
		mutateScoring(child, rng)
	}

	return child
}

func tweakParameter(g *genome.Genome, rng *rand.Rand) {
	// Shared params
	switch rng.IntN(2) {
	case 0:
		g.Players = clampInt(g.Players+rng.IntN(3)-1, 2, 6)
	case 1:
		g.HandSize = clampInt(g.HandSize+rng.IntN(3)-1, 3, 13)
	}

	// Skeleton-specific params
	switch g.Skeleton {
	case genome.Shedding:
		if g.Shedding != nil {
			g.Shedding.DrawPenalty = clampInt(g.Shedding.DrawPenalty+rng.IntN(3)-1, 1, 3)
		}
	case genome.TrickTaking:
		if g.TrickTaking != nil {
			g.TrickTaking.RoundsPerGame = clampInt(g.TrickTaking.RoundsPerGame+rng.IntN(3)-1, 1, 13)
		}
	case genome.Rummy:
		if g.Rummy != nil {
			switch rng.IntN(2) {
			case 0:
				g.Rummy.MinMeldSize = clampInt(g.Rummy.MinMeldSize+rng.IntN(3)-1, 2, 4)
			case 1:
				g.Rummy.KnockThreshold = clampInt(g.Rummy.KnockThreshold+rng.IntN(5)-2, 0, 30)
			}
		}
	}
}

func flipBool(g *genome.Genome, rng *rand.Rand) {
	switch g.Skeleton {
	case genome.Shedding:
		if g.Shedding != nil {
			switch rng.IntN(2) {
			case 0:
				g.Shedding.CanStack = !g.Shedding.CanStack
			case 1:
				g.Shedding.PlayMultiple = !g.Shedding.PlayMultiple
			}
		}
	case genome.TrickTaking:
		if g.TrickTaking != nil {
			g.TrickTaking.MustFollowSuit = !g.TrickTaking.MustFollowSuit
		}
	case genome.Rummy:
		if g.Rummy != nil {
			g.Rummy.CanLayOff = !g.Rummy.CanLayOff
		}
	}
}

func changeEnum(g *genome.Genome, rng *rand.Rand) {
	switch g.Skeleton {
	case genome.Shedding:
		if g.Shedding != nil {
			g.Shedding.MatchRule = genome.MatchRule(rng.IntN(4))
		}
	case genome.TrickTaking:
		if g.TrickTaking != nil {
			switch rng.IntN(2) {
			case 0:
				g.TrickTaking.TrickScoring = genome.TrickScoring(rng.IntN(3))
				// If switching to card-points or avoidance, ensure scoring config exists
				if (g.TrickTaking.TrickScoring == genome.ScoreCardPoints ||
					g.TrickTaking.TrickScoring == genome.ScoreAvoidance) &&
					len(g.Scoring.CardPoints) == 0 {
					g.Scoring.CardPoints = []genome.CardScoring{
						{Suit: uint8(3), Points: 1}, // Hearts worth 1 point
					}
				}
			case 1:
				g.TrickTaking.LeadRestriction = genome.LeadRule(rng.IntN(3))
			}
		}
		// Also mutate trump rule
		if rng.Float64() < 0.3 {
			g.TrumpRule = genome.TrumpRule(rng.IntN(4))
			// If fixed trump, ensure trump suit is set
			if g.TrumpRule == genome.TrumpFixed && g.Scoring.TrumpSuit == 0 {
				g.Scoring.TrumpSuit = uint8(rng.IntN(4) + 1)
			}
		}
	case genome.Rummy:
		if g.Rummy != nil {
			switch rng.IntN(2) {
			case 0:
				g.Rummy.MeldTypes = genome.MeldType(rng.IntN(3))
			case 1:
				g.Rummy.DrawFrom = genome.DrawSource(rng.IntN(3))
			}
		}
	}
}

func addSpecialCard(g *genome.Genome, rng *rand.Rand) {
	if len(g.SpecialCards) >= 6 {
		return // Cap at 6 special cards
	}

	ranks := []uint8{uint8(2), uint8(7), uint8(8), uint8(10), uint8(11), uint8(12)}
	types := []genome.SpecialCardType{
		genome.SpecialSkip,
		genome.SpecialReverse,
		genome.SpecialDrawTwo,
		genome.SpecialWild,
	}

	sc := genome.SpecialCard{
		Type:   types[rng.IntN(len(types))],
		ByRank: ranks[rng.IntN(len(ranks))],
	}

	g.SpecialCards = append(g.SpecialCards, sc)
}

func removeSpecialCard(g *genome.Genome, rng *rand.Rand) {
	if len(g.SpecialCards) == 0 {
		return
	}
	idx := rng.IntN(len(g.SpecialCards))
	g.SpecialCards = append(g.SpecialCards[:idx], g.SpecialCards[idx+1:]...)
}

func addBorrowedMechanic(g *genome.Genome, rng *rand.Rand) {
	if len(g.Borrowed) >= 3 {
		return // Cap at 3 borrowed mechanics
	}

	// Pick a valid borrow for this skeleton
	var candidates []genome.BorrowedMechanic

	switch g.Skeleton {
	case genome.Shedding:
		candidates = []genome.BorrowedMechanic{
			{Source: genome.Rummy, Mechanic: genome.MechMeldBonus},
			{Source: genome.TrickTaking, Mechanic: genome.MechTrump},
			{Source: genome.TrickTaking, Mechanic: genome.MechAvoidance},
		}
	case genome.TrickTaking:
		candidates = []genome.BorrowedMechanic{
			{Source: genome.Rummy, Mechanic: genome.MechMeldBonus},
			{Source: genome.Shedding, Mechanic: genome.MechDrawPenalty},
			{Source: genome.Shedding, Mechanic: genome.MechPlayMultiple},
		}
	case genome.Rummy:
		candidates = []genome.BorrowedMechanic{
			{Source: genome.TrickTaking, Mechanic: genome.MechTrickScoring},
			{Source: genome.Shedding, Mechanic: genome.MechDrawPenalty},
			{Source: genome.TrickTaking, Mechanic: genome.MechAvoidance},
		}
	}

	if len(candidates) == 0 {
		return
	}

	// Don't add duplicates
	pick := candidates[rng.IntN(len(candidates))]
	for _, b := range g.Borrowed {
		if b.Mechanic == pick.Mechanic {
			return
		}
	}

	g.Borrowed = append(g.Borrowed, pick)
}

func removeBorrowedMechanic(g *genome.Genome, rng *rand.Rand) {
	if len(g.Borrowed) == 0 {
		return
	}
	idx := rng.IntN(len(g.Borrowed))
	g.Borrowed = append(g.Borrowed[:idx], g.Borrowed[idx+1:]...)
}

func changeSkeleton(g *genome.Genome, rng *rand.Rand, allSeeds []*genome.Genome) {
	if len(allSeeds) == 0 {
		return
	}
	// Pick a random seed (potentially different skeleton)
	seed := allSeeds[rng.IntN(len(allSeeds))]
	seedCopy := cloneGenome(seed)

	// Keep the child's generation and ID
	seedCopy.ID = g.ID
	seedCopy.Generation = g.Generation

	*g = *seedCopy
}

func mutateScoring(g *genome.Genome, rng *rand.Rand) {
	if len(g.Scoring.CardPoints) == 0 {
		// Add a scoring rule
		g.Scoring.CardPoints = append(g.Scoring.CardPoints, genome.CardScoring{
			Suit:   uint8(rng.IntN(5)),      // 0=all, 1-4=specific
			Rank:   uint8(rng.IntN(13) + 2), // 2-14 (Rank=1 is invalid; 0=all is reserved)
			Points: rng.IntN(13) + 1,        // 1-13
		})
	} else {
		// Modify existing
		idx := rng.IntN(len(g.Scoring.CardPoints))
		g.Scoring.CardPoints[idx].Points = clampInt(
			g.Scoring.CardPoints[idx].Points+rng.IntN(5)-2, 1, 20)
	}
}

func cloneGenome(g *genome.Genome) *genome.Genome {
	return g.Clone()
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

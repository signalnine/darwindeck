package seeds

import (
	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

// Whist returns the Whist seed genome.
// Simplest trick-taker: follow suit, trump cut from deck, score per trick.
// LeadRestriction is LeadNone: the trick winner leading next is the
// skeleton's hardcoded turn order, not a card restriction (the former
// LeadWinnerLeads encoding was inert and is now reserved -- byte-identical
// traces, pinned by TestReservedWinnerLeadsValueIsInert).
func Whist() *genome.Genome {
	return &genome.Genome{
		ID:       "whist",
		Skeleton: genome.TrickTaking,
		Players:  4,
		HandSize: 13,
		TrickTaking: &genome.TrickTakingParams{
			MustFollowSuit:  true,
			TrickScoring:    genome.ScorePerTrick,
			LeadRestriction: genome.LeadNone,
			RoundsPerGame:   1,
		},
		TrumpRule: genome.TrumpCut,
	}
}

// Hearts returns the Hearts seed genome.
// Trick avoidance: hearts are 1 point each, Queen of Spades is 13.
// Lowest score wins. Can't lead hearts until broken.
func Hearts() *genome.Genome {
	return &genome.Genome{
		ID:       "hearts",
		Skeleton: genome.TrickTaking,
		Players:  4,
		HandSize: 13,
		TrickTaking: &genome.TrickTakingParams{
			MustFollowSuit:  true,
			TrickScoring:    genome.ScoreAvoidance,
			LeadRestriction: genome.LeadNoTrumpUntilBroken,
			RoundsPerGame:   1,
		},
		TrumpRule: genome.TrumpNone, // No trump in Hearts
		Scoring: genome.ScoringConfig{
			CardPoints: []genome.CardScoring{
				// All hearts worth 1 point
				{Suit: uint8(sim.Hearts) + 1, Points: 1},
				// Queen of Spades worth 13 points
				{Rank: uint8(sim.Queen), Suit: uint8(sim.Spades) + 1, Points: 13},
			},
		},
	}
}

// Spades returns the Spades seed genome.
// Always-trump (spades), must follow suit, score per trick.
func Spades() *genome.Genome {
	return &genome.Genome{
		ID:       "spades",
		Skeleton: genome.TrickTaking,
		Players:  4,
		HandSize: 13,
		TrickTaking: &genome.TrickTakingParams{
			MustFollowSuit:  true,
			TrickScoring:    genome.ScorePerTrick,
			LeadRestriction: genome.LeadNoTrumpUntilBroken,
			RoundsPerGame:   1,
		},
		TrumpRule: genome.TrumpFixed,
		Scoring: genome.ScoringConfig{
			TrumpSuit: uint8(sim.Spades) + 1, // Spades are always trump
		},
	}
}

// OhHell returns the Oh Hell seed genome.
// Exact-bid scoring: players bid tricks, score only if exact.
// Simplified here as per-trick scoring (bidding is a future extension).
func OhHell() *genome.Genome {
	return &genome.Genome{
		ID:       "oh-hell",
		Skeleton: genome.TrickTaking,
		Players:  4,
		HandSize: 7,
		TrickTaking: &genome.TrickTakingParams{
			MustFollowSuit:  true,
			TrickScoring:    genome.ScorePerTrick,
			LeadRestriction: genome.LeadNone, // winner-leads is hardcoded; see Whist
			RoundsPerGame:   1,
		},
		TrumpRule: genome.TrumpCut,
	}
}

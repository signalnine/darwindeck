package evolution

import (
	"math/rand/v2"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
)

// TestAddBorrowedMechanicReachesFullWhitelist verifies the cross-skeleton
// MUTATION path (addBorrowedMechanic with crossSkeleton=true) can reach EVERY
// whitelisted, hooked borrow for each host. Together with the crossFamilyBorrow
// matrix (recombination), this ensures no host family is structurally barred
// from a novel mechanic via either operator -- the Wave-1 trap was that only
// trick hosts crossed into novelty.
func TestAddBorrowedMechanicReachesFullWhitelist(t *testing.T) {
	rng := rand.New(rand.NewPCG(13, 17))
	whitelist := genome.ValidBorrows()

	bases := map[genome.SkeletonType]func() *genome.Genome{
		genome.Shedding: func() *genome.Genome {
			return &genome.Genome{
				ID: "h", Skeleton: genome.Shedding, Players: 2, HandSize: 5,
				Shedding: &genome.SheddingParams{MatchRule: genome.MatchEither, DrawPenalty: 1},
			}
		},
		genome.TrickTaking: func() *genome.Genome {
			return &genome.Genome{
				ID: "h", Skeleton: genome.TrickTaking, Players: 4, HandSize: 5,
				TrickTaking: &genome.TrickTakingParams{MustFollowSuit: true, TrickScoring: genome.ScorePerTrick},
			}
		},
		genome.Rummy: func() *genome.Genome {
			return &genome.Genome{
				ID: "h", Skeleton: genome.Rummy, Players: 2, HandSize: 7,
				Rummy: &genome.RummyParams{MeldTypes: genome.MeldBoth, MinMeldSize: 3, KnockThreshold: 10},
			}
		},
	}

	for host, mk := range bases {
		reached := make(map[genome.MechanicType]bool)
		for i := 0; i < 4000; i++ {
			g := mk()
			addBorrowedMechanic(g, rng, true /* crossSkeleton */)
			for _, bm := range g.Borrowed {
				reached[bm.Mechanic] = true
			}
		}
		for _, want := range whitelist[host] {
			if ungeneratedBorrows[host][want] {
				// Documented vestigial-by-host combo (Wave-3): whitelisted but
				// deliberately never generated, so it must NOT be reached.
				if reached[want] {
					t.Errorf("host=%v: addBorrowedMechanic generated %v, which is documented ungeneratable (vestigial-by-host)", host, want)
				}
				continue
			}
			if !reached[want] {
				t.Errorf("host=%v: addBorrowedMechanic(crossSkeleton) never reached whitelisted mechanic %v", host, want)
			}
		}
	}
}

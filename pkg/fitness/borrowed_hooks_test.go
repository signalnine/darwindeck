package fitness

import (
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/mechanic"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

// TestBorrowedHooksFireDuringBatchSim asserts that every whitelisted
// (skeleton, mechanic) borrow combination produces hook invocations during
// a real batch sim. Without this, the hook can be silently registered but
// never fire (dd-4ql).
//
// MechTrump / MechPlayMultiple are not in BuildHooks (handled structurally),
// so they are intentionally excluded from this check.
func TestBorrowedHooksFireDuringBatchSim(t *testing.T) {
	cases := []struct {
		host     genome.SkeletonType
		source   genome.SkeletonType
		mechanic genome.MechanicType
	}{
		{genome.Shedding, genome.Rummy, genome.MechMeldBonus},
		{genome.Shedding, genome.TrickTaking, genome.MechAvoidance},
		{genome.TrickTaking, genome.Rummy, genome.MechMeldBonus},
		{genome.TrickTaking, genome.Shedding, genome.MechDrawPenalty},
		{genome.Rummy, genome.TrickTaking, genome.MechTrickScoring},
		{genome.Rummy, genome.Shedding, genome.MechDrawPenalty},
		{genome.Rummy, genome.TrickTaking, genome.MechAvoidance},
	}

	for _, tc := range cases {
		tc := tc
		name := tc.host.String() + "_borrows_" + tc.mechanic.String()
		t.Run(name, func(t *testing.T) {
			g := buildBorrowingGenome(tc.host, tc.source, tc.mechanic)
			if errs := genome.Validate(g); len(errs) > 0 {
				t.Fatalf("genome should be valid: %v", errs)
			}

			hooks := mechanic.BuildHooks(g)
			if len(hooks) == 0 {
				t.Fatalf("expected at least one hook for %s borrowing %s, got 0",
					tc.host, tc.mechanic)
			}

			counters := make([]int, len(hooks))
			funcs := make([]sim.HookFunc, len(hooks))
			for i := range hooks {
				i := i
				h := hooks[i]
				funcs[i] = func(state *sim.GameState, gg *genome.Genome, event sim.Event) {
					switch h.Point {
					case mechanic.HookAfterPlay:
						if event.Type == sim.EventCardPlayed {
							counters[i]++
						}
					case mechanic.HookEndOfRound, mechanic.HookScoring:
						if event.Type == sim.EventRoundEnd {
							counters[i]++
						}
					}
				}
			}

			runner := GetRunner(g)
			if runner == nil {
				t.Fatalf("no runner for %s", tc.host)
			}

			ai := &sim.RandomAI{}
			sim.RunBatch(g, runner, ai, 50, 42, funcs...)

			for i, count := range counters {
				if count == 0 {
					t.Errorf("hook %d (point=%d) for %s borrowing %s never fired across 50 games",
						i, hooks[i].Point, tc.host, tc.mechanic)
				}
			}
		})
	}
}

// buildBorrowingGenome constructs a minimal but valid genome that hosts the
// given borrow. Each host gets the parameters its runner needs to actually
// complete games (otherwise Tier1 timeouts would mask hook coverage).
func buildBorrowingGenome(host, source genome.SkeletonType, mech genome.MechanicType) *genome.Genome {
	g := &genome.Genome{
		ID:       host.String() + "-borrows-" + mech.String(),
		Skeleton: host,
		Players:  2,
		HandSize: 5,
		Borrowed: []genome.BorrowedMechanic{
			{Source: source, Mechanic: mech},
		},
		Scoring: genome.ScoringConfig{
			CardPoints: []genome.CardScoring{
				{Suit: uint8(sim.Hearts) + 1, Points: 1},
			},
		},
	}

	switch host {
	case genome.Shedding:
		g.Shedding = &genome.SheddingParams{
			MatchRule:   genome.MatchEither,
			DrawPenalty: 1,
		}
	case genome.TrickTaking:
		g.HandSize = 6
		g.TrickTaking = &genome.TrickTakingParams{
			MustFollowSuit:  true,
			TrickScoring:    genome.ScorePerTrick,
			LeadRestriction: genome.LeadWinnerLeads,
			RoundsPerGame:   1,
		}
		g.TrumpRule = genome.TrumpCut
	case genome.Rummy:
		g.Rummy = &genome.RummyParams{
			MeldTypes:      genome.MeldBoth,
			MinMeldSize:    3,
			DrawFrom:       genome.DrawEither,
			KnockThreshold: 10,
		}
	}

	return g
}

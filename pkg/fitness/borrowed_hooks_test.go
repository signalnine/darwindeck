package fitness

import (
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/mechanic"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

// TestBorrowedHooksFireDuringBatchSim asserts that every whitelisted
// (skeleton, mechanic) borrow combination produces hook invocations during
// a real batch sim AND completes a healthy fraction of the games it runs.
//
// Hook-fired-at-least-once is necessary but not sufficient: a borrow that
// fires the hook AND breaks the host runner so ~all games timeout is exactly
// the dd-wfi failure mode (MechDrawPenalty x TrickTaking pre-fix). The 70%
// completion floor catches that class of bug.
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
		{genome.Rummy, genome.TrickTaking, genome.MechTrickScoring},
		{genome.Rummy, genome.Shedding, genome.MechDrawPenalty},
		{genome.Rummy, genome.TrickTaking, genome.MechAvoidance},
	}

	const games = 50
	const minCompletionRate = 0.70

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
			result := sim.RunBatch(g, runner, ai, games, 42, funcs...)

			for i, count := range counters {
				if count == 0 {
					t.Errorf("hook %d (point=%d) for %s borrowing %s never fired across %d games",
						i, hooks[i].Point, tc.host, tc.mechanic, games)
				}
			}

			rate := float64(result.Completions) / float64(result.GamesPlayed)
			if rate < minCompletionRate {
				t.Errorf("%s borrowing %s: completion rate %.2f (%d/%d) below floor %.2f -- borrow likely breaks host runner invariants",
					tc.host, tc.mechanic, rate, result.Completions, result.GamesPlayed, minCompletionRate)
			}
		})
	}
}

// TestBorrowedHooksMutateStateDuringBatchSim guards against the silent-no-op
// failure mode that hook-fire counters miss: a hook can fire on the right
// event and still leave state unchanged because it reads from a location the
// host runner never populates (e.g. MechMeldBonus reading state.Hands on a
// trick-taking host whose hands are empty by the time EventRoundEnd fires --
// dd-no2). For each whitelisted borrow combination, observe state.Scores and
// state.Hands deltas across hook invocations and require at least one mutation
// during the batch -- a hook that never moves state across 50 games is
// effectively unwired.
func TestBorrowedHooksMutateStateDuringBatchSim(t *testing.T) {
	cases := []struct {
		host     genome.SkeletonType
		source   genome.SkeletonType
		mechanic genome.MechanicType
	}{
		{genome.Shedding, genome.Rummy, genome.MechMeldBonus},
		{genome.Shedding, genome.TrickTaking, genome.MechAvoidance},
		{genome.TrickTaking, genome.Rummy, genome.MechMeldBonus},
		{genome.Rummy, genome.TrickTaking, genome.MechTrickScoring},
		{genome.Rummy, genome.Shedding, genome.MechDrawPenalty},
		{genome.Rummy, genome.TrickTaking, genome.MechAvoidance},
	}

	const games = 50

	for _, tc := range cases {
		tc := tc
		name := tc.host.String() + "_borrows_" + tc.mechanic.String()
		t.Run(name, func(t *testing.T) {
			g := buildBorrowingGenome(tc.host, tc.source, tc.mechanic)
			// MechMeldBonus on a Shedding host fires correctly but the loser's
			// hand is usually 1-2 cards at EventRoundEnd, so 3-of-a-kind melds
			// are essentially never present at hand sizes below ~8. Bump this
			// case so the integration check exercises a configuration where the
			// hook can plausibly mutate state.
			if tc.host == genome.Shedding && tc.mechanic == genome.MechMeldBonus {
				g.HandSize = 10
			}
			if errs := genome.Validate(g); len(errs) > 0 {
				t.Fatalf("genome should be valid: %v", errs)
			}

			hooks := mechanic.BuildHooks(g)
			if len(hooks) == 0 {
				t.Fatalf("expected at least one hook")
			}

			mutations := 0
			funcs := make([]sim.HookFunc, len(hooks))
			for i := range hooks {
				h := hooks[i]
				funcs[i] = func(state *sim.GameState, gg *genome.Genome, event sim.Event) {
					shouldApply := false
					switch h.Point {
					case mechanic.HookAfterPlay:
						shouldApply = event.Type == sim.EventCardPlayed
					case mechanic.HookEndOfRound, mechanic.HookScoring:
						shouldApply = event.Type == sim.EventRoundEnd
					}
					if !shouldApply {
						return
					}

					scoresBefore := append([]int(nil), state.Scores...)
					handSizesBefore := make([]int, len(state.Hands))
					for k, hand := range state.Hands {
						handSizesBefore[k] = len(hand)
					}

					h.Apply(state, gg, event)

					for k, v := range state.Scores {
						if k >= len(scoresBefore) || v != scoresBefore[k] {
							mutations++
							return
						}
					}
					for k, hand := range state.Hands {
						if k >= len(handSizesBefore) || len(hand) != handSizesBefore[k] {
							mutations++
							return
						}
					}
				}
			}

			runner := GetRunner(g)
			if runner == nil {
				t.Fatalf("no runner for %s", tc.host)
			}

			sim.RunBatch(g, runner, &sim.RandomAI{}, games, 42, funcs...)

			if mutations == 0 {
				t.Errorf("%s borrowing %s: hook never mutated state across %d games -- borrow is silently a no-op",
					tc.host, tc.mechanic, games)
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

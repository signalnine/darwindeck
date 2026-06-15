package fitness

import (
	"sort"
	"sync/atomic"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/mechanic"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

// borrowCase is one whitelisted (host, mechanic) combination plus a legal
// source skeleton to record on the genome's BorrowedMechanic.
type borrowCase struct {
	host     genome.SkeletonType
	source   genome.SkeletonType
	mechanic genome.MechanicType
}

// borrowCases derives the integration-test case list from the validBorrows
// whitelist via genome.ValidBorrows() instead of a hand-copied table, so a
// newly whitelisted borrow is automatically covered here -- and fails until
// its hook actually behaves (audit remediation Task 26).
//
// MechTrump / MechPlayMultiple need no explicit exclusion: they are reserved
// enum values that validation keeps out of the whitelist (dd-lnh), so they
// can never appear in the derived list.
func borrowCases() []borrowCase {
	whitelist := genome.ValidBorrows()

	hosts := make([]genome.SkeletonType, 0, len(whitelist))
	for host := range whitelist {
		hosts = append(hosts, host)
	}
	sort.Slice(hosts, func(i, j int) bool { return hosts[i] < hosts[j] })

	var cases []borrowCase
	for _, host := range hosts {
		for _, mech := range whitelist[host] {
			// Runner-implemented (deep) borrows have NO hook by design: their
			// effect lives in the skeleton runner's GenerateMoves/ApplyMove, not
			// pkg/mechanic.BuildHooks. The hook-firing/state-mutation assertions
			// here only apply to hook-based borrows; runner borrows are covered
			// by their own skeleton package tests (e.g. shedding RunPlay tests).
			if runnerImplementedBorrow[mech] {
				continue
			}
			cases = append(cases, borrowCase{
				host:     host,
				source:   borrowSource(host, mech),
				mechanic: mech,
			})
		}
	}
	return cases
}

// runnerImplementedBorrow lists whitelisted borrows whose behavior lives in a
// skeleton RUNNER (changing the move set / win condition) rather than in a
// pkg/mechanic hook. They are excluded from the hook-based assertions; their
// effect is validated in the owning skeleton package.
var runnerImplementedBorrow = map[genome.MechanicType]bool{
	genome.MechRunPlay:    true,
	genome.MechFollowSuit: true,
}

// borrowSource picks the source skeleton recorded on the BorrowedMechanic.
// BuildHooks keys on Mechanic alone; Source only has to satisfy validation's
// "cannot borrow from own skeleton" rule. Prefer the mechanic's semantic home
// skeleton, fall back to any other skeleton when that home IS the host (or
// when a newly whitelisted mechanic has no entry yet -- the case must still
// run rather than be silently skipped).
func borrowSource(host genome.SkeletonType, mech genome.MechanicType) genome.SkeletonType {
	semanticHome := map[genome.MechanicType]genome.SkeletonType{
		genome.MechTrickScoring: genome.TrickTaking,
		genome.MechMeldBonus:    genome.Rummy,
		genome.MechDrawPenalty:  genome.Shedding,
		genome.MechAvoidance:    genome.TrickTaking,
	}
	if home, ok := semanticHome[mech]; ok && home != host {
		return home
	}
	for _, s := range []genome.SkeletonType{genome.Shedding, genome.TrickTaking, genome.Rummy} {
		if s != host {
			return s
		}
	}
	return host // unreachable: three skeletons exist, host is only one of them
}

// TestBorrowedHooksFireDuringBatchSim asserts that every whitelisted
// (skeleton, mechanic) borrow combination produces hook invocations during
// a real batch sim AND completes a healthy fraction of the games it runs.
//
// Hook-fired-at-least-once is necessary but not sufficient: a borrow that
// fires the hook AND breaks the host runner so ~all games timeout is exactly
// the dd-wfi failure mode (MechDrawPenalty x TrickTaking pre-fix). The 70%
// completion floor catches that class of bug.
func TestBorrowedHooksFireDuringBatchSim(t *testing.T) {
	cases := borrowCases()
	if len(cases) == 0 {
		t.Fatal("derived zero borrow cases from genome.ValidBorrows() -- whitelist empty or accessor broken")
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

			// Atomic: RunBatch plays the batch's games concurrently (Wave I)
			// and these instrumentation hooks aggregate ACROSS games, unlike
			// production hooks (stateless, per-game state only).
			counters := make([]atomic.Int64, len(hooks))
			funcs := make([]sim.HookFunc, len(hooks))
			for i := range hooks {
				i := i
				h := hooks[i]
				funcs[i] = func(state *sim.GameState, gg *genome.Genome, event sim.Event) {
					switch h.Point {
					case mechanic.HookAfterPlay:
						if event.Type == sim.EventCardPlayed {
							counters[i].Add(1)
						}
					case mechanic.HookEndOfRound, mechanic.HookScoring:
						if event.Type == sim.EventRoundEnd {
							counters[i].Add(1)
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

			for i := range counters {
				if counters[i].Load() == 0 {
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
	cases := borrowCases()
	if len(cases) == 0 {
		t.Fatal("derived zero borrow cases from genome.ValidBorrows() -- whitelist empty or accessor broken")
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

			// Atomic for the same Wave I reason as the fire counters above;
			// the state reads/writes inside the hook stay race-free because
			// each game owns its state.
			var mutations atomic.Int64
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
							mutations.Add(1)
							return
						}
					}
					for k, hand := range state.Hands {
						if k >= len(handSizesBefore) || len(hand) != handSizesBefore[k] {
							mutations.Add(1)
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

			if mutations.Load() == 0 {
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
			LeadRestriction: genome.LeadNone, // LeadWinnerLeads is reserved/inert
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
	case genome.Climbing:
		// Climbing's only whitelisted borrow is MechDrawPenalty, whose hook
		// (applyDrawPenalty) fires on face-card plays. A larger hand keeps a
		// deck to draw from and makes face-card plays frequent so the hook both
		// fires and mutates a hand. Climbing reads no CardPoints; the shared
		// Scoring above is inert for it.
		g.HandSize = 9
		g.Climbing = &genome.ClimbingParams{
			AllowPairs:   true,
			AllowTriples: true,
			AllowRuns:    true,
			MinRunLen:    3,
		}
	}

	return g
}

// TestMultiRoundScoringBorrowIsFitnessVisible is Task 22's evolvability
// sub-check (d): run the default Evaluate pipeline on a MeldBonus + 3-rounds
// shedding genome and on the same genome WITHOUT the borrow, and require a
// measurable difference in the raw metrics. If the borrow cannot move any
// metric, it is invisible to selection and still inert from evolution's
// perspective -- the multi-round design would not be done.
//
// The genome mirrors the shedding runner's Task 22 reference fixture:
// HandSize 13 / DrawPenalty 3 / 3 players keep residual hands large enough
// that MeldBonus actually banks points at round end.
func TestMultiRoundScoringBorrowIsFitnessVisible(t *testing.T) {
	withBorrow := &genome.Genome{
		ID:       "meldbonus-3rounds",
		Skeleton: genome.Shedding,
		Players:  3,
		HandSize: 13,
		Shedding: &genome.SheddingParams{
			MatchRule:     genome.MatchEither,
			DrawPenalty:   3,
			RoundsPerGame: 3,
		},
		Borrowed: []genome.BorrowedMechanic{
			{Source: genome.Rummy, Mechanic: genome.MechMeldBonus},
		},
	}
	withoutBorrow := withBorrow.Clone()
	withoutBorrow.ID = "no-borrow-3rounds"
	withoutBorrow.Borrowed = nil // RoundsPerGame stays 3 but is inert: single round

	if errs := genome.Validate(withBorrow); len(errs) > 0 {
		t.Fatalf("with-borrow genome invalid: %v", errs)
	}
	if errs := genome.Validate(withoutBorrow); len(errs) > 0 {
		t.Fatalf("without-borrow genome invalid: %v", errs)
	}

	const seed = 4242
	evalWith := Evaluate(withBorrow, seed)
	evalWithout := Evaluate(withoutBorrow, seed)

	if !evalWith.Valid {
		t.Fatalf("with-borrow genome failed the pipeline (tier0=%v tier1=%q) -- multi-round games must survive evaluation",
			evalWith.Tier0Errors, evalWith.Tier1.Reason)
	}
	if !evalWithout.Valid {
		t.Fatalf("without-borrow control failed the pipeline (tier0=%v tier1=%q)",
			evalWithout.Tier0Errors, evalWithout.Tier1.Reason)
	}

	a, b := evalWith.Metrics, evalWithout.Metrics
	diffs := map[string]float64{
		"decisions": a.MeaningfulDecisions - b.MeaningfulDecisions,
		"arc":       a.GameArc - b.GameArc,
		"interact":  a.Interaction - b.Interaction,
		"skill":     a.SkillGradient - b.SkillGradient,
		"length":    a.SessionLength - b.SessionLength,
	}
	const measurable = 0.02
	maxName, maxAbs := "", 0.0
	for name, d := range diffs {
		if d < 0 {
			d = -d
		}
		if d > maxAbs {
			maxName, maxAbs = name, d
		}
	}
	t.Logf("metric deltas (with-borrow minus without): %+v; max |delta| = %s %.4f", diffs, maxName, maxAbs)
	if maxAbs <= measurable {
		t.Errorf("no metric moved by more than %.2f when the scoring borrow was added (max %s %.4f): the borrow is fitness-invisible and therefore still inert for evolution",
			measurable, maxName, maxAbs)
	}
}

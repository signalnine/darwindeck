package evolution

import (
	"testing"

	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/mechanic"
	"github.com/darwindeck/darwindeck/pkg/seeds"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

// behaviorOf returns the hooked behavior descriptor for g.
func behaviorOf(t *testing.T, g *genome.Genome, seed uint64) BehaviorDescriptor {
	t.Helper()
	res, ok := BehaviorBatch(g, seed)
	if !ok {
		t.Fatalf("no runner for %s", g.ID)
	}
	return ComputeBehavior(res)
}

// borrowChangesWinner reports how many of n games changed winner when the
// genome's borrowed-mechanic hooks are active vs absent, on identical seeds.
// A scoring/penalty borrow with TEETH must flip the winner in a non-trivial
// fraction of games -- otherwise it is a vestigial tally CheckEnd ignores.
func borrowChangesWinner(g *genome.Genome, seed uint64, n int) (changed, total int) {
	runner := fitness.GetRunner(g)
	with := sim.RunBatch(g, runner, &sim.RandomAI{}, n, seed, mechanic.HooksFor(g)...)
	without := sim.RunBatch(g, runner, &sim.RandomAI{}, n, seed)
	for i := range with.AllWinners {
		if i >= len(without.AllWinners) {
			break
		}
		// Only count decided games (winner >= 0 in at least one variant).
		if with.AllWinners[i] != without.AllWinners[i] {
			changed++
		}
		total++
	}
	return changed, total
}

// minTeethDistance is the descriptor-distance floor a wired hybrid must clear
// vs its base classic. The Wave-2 vestigial hybrids sat at EXACTLY 0.0 (a
// pixel-identical Whist clone); any real structural change measured here moves
// at least ~0.008. We require a conservative 0.005 so the gate is about
// "moved at all, by construction" not a tuned magnitude.
const minTeethDistance = 0.005

// minWinnerFlipFrac is the fraction of played games whose winner a banking /
// avoidance borrow must flip vs the hook-free baseline -- proof the borrowed
// scoring DECIDES THE WINNER, not a tally CheckEnd ignores. Conservative: a
// vestigial single-token borrow flips ~0 reliably once the score gap dwarfs it;
// a real penalty/scoring set flips well above this.
const minWinnerFlipFrac = 0.10

// teethCase describes one cross-family borrow grafted onto a base classic by
// the production wiring (wireHybridBorrow / addBorrowedMechanic).
type teethCase struct {
	name string
	base func() *genome.Genome // the base classic seed
	// build returns the wired hybrid AS THE PRODUCTION PATH WOULD, by grafting
	// the borrow and calling the wiring under test.
	build func() *genome.Genome
	mech  genome.MechanicType
	// scoring reports whether the borrow is a banking/avoidance scoring borrow
	// (winner-flip is the proof); draw-penalty borrows prove teeth via the
	// descriptor move alone (they grow a hand mid-play, which the winner-flip
	// counter cannot attribute cleanly because the deck draw shifts the RNG).
	scoring bool
}

// TestCrossBorrowsHaveTeeth is the Wave-3 root-cause gate. For every
// cross-family borrow the engine can graft, the WIRED hybrid (produced by the
// production wiring, not a hand-tuned genome) must:
//
//  1. move its BehaviorDescriptor measurably away from the base classic
//     (decision-density x interaction) -- it does not just PLAY like the classic
//     with a cosmetic tag, and
//  2. for a banking/avoidance scoring borrow, the borrowed scoring must DECIDE
//     THE WINNER in a non-trivial fraction of games (winner flips vs the
//     hook-free baseline) -- not a vestigial tally CheckEnd ignores.
//
// This is the falsifiable replacement for the Wave-2 failure (0 novel because
// borrows were vestigial: descriptor distance was 0.0 and the game still played
// like the base classic).
func TestCrossBorrowsHaveTeeth(t *testing.T) {
	cases := []teethCase{
		{
			name:    "avoidance on trick (Whist host)",
			base:    seeds.Whist,
			build:   func() *genome.Genome { return wiredHybrid(seeds.Whist(), genome.Shedding, genome.MechAvoidance) },
			mech:    genome.MechAvoidance,
			scoring: true,
		},
		{
			name:    "meld-bonus on trick (Whist host)",
			base:    seeds.Whist,
			build:   func() *genome.Genome { return wiredHybrid(seeds.Whist(), genome.Rummy, genome.MechMeldBonus) },
			mech:    genome.MechMeldBonus,
			scoring: true,
		},
		{
			name:    "trick-scoring on shedding (Crazy Eights host)",
			base:    seeds.CrazyEights,
			build:   func() *genome.Genome { return wiredHybrid(seeds.CrazyEights(), genome.TrickTaking, genome.MechTrickScoring) },
			mech:    genome.MechTrickScoring,
			scoring: true,
		},
		{
			name:    "meld-bonus on shedding (Crazy Eights host)",
			base:    seeds.CrazyEights,
			build:   func() *genome.Genome { return wiredHybrid(seeds.CrazyEights(), genome.Rummy, genome.MechMeldBonus) },
			mech:    genome.MechMeldBonus,
			scoring: true,
		},
		{
			name:    "avoidance on shedding (Crazy Eights host)",
			base:    seeds.CrazyEights,
			build:   func() *genome.Genome { return wiredHybrid(seeds.CrazyEights(), genome.TrickTaking, genome.MechAvoidance) },
			mech:    genome.MechAvoidance,
			scoring: true,
		},
		{
			name:    "avoidance on rummy (Gin Rummy host)",
			base:    seeds.GinRummy,
			build:   func() *genome.Genome { return wiredHybrid(seeds.GinRummy(), genome.TrickTaking, genome.MechAvoidance) },
			mech:    genome.MechAvoidance,
			scoring: true,
		},
		{
			name:    "draw-penalty on rummy (Gin Rummy host)",
			base:    seeds.GinRummy,
			build:   func() *genome.Genome { return wiredHybrid(seeds.GinRummy(), genome.Shedding, genome.MechDrawPenalty) },
			mech:    genome.MechDrawPenalty,
			scoring: false,
		},
		{
			name:    "draw-penalty on climbing (Big Two host)",
			base:    seeds.BigTwo,
			build:   func() *genome.Genome { return wiredHybrid(seeds.BigTwo(), genome.Shedding, genome.MechDrawPenalty) },
			mech:    genome.MechDrawPenalty,
			scoring: false,
		},
	}

	const seed = uint64(7)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			base := tc.base()
			hybrid := tc.build()

			// The wired hybrid must be valid and carry a LIVE borrow.
			if errs := genome.Validate(hybrid); len(errs) > 0 {
				t.Fatalf("wired hybrid invalid: %v", errs)
			}
			liveHasMech := false
			for _, bm := range hybrid.LiveBorrows() {
				if bm.Mechanic == tc.mech {
					liveHasMech = true
				}
			}
			if !liveHasMech {
				t.Fatalf("borrow %v is not LIVE on the wired hybrid: live=%+v", tc.mech, hybrid.LiveBorrows())
			}

			// 1. Descriptor must move measurably away from the base classic.
			bd := behaviorOf(t, base, seed)
			hd := behaviorOf(t, hybrid, seed)
			dist := bd.Distance(hd)
			if dist < minTeethDistance {
				t.Errorf("%s: behavior descriptor did not move (dist %.4f < %.4f) -- borrow is vestigial, hybrid plays like the base classic\n  base=%v hybrid=%v",
					tc.name, dist, minTeethDistance, bd, hd)
			}

			// 2. A banking/avoidance scoring borrow must DECIDE the winner.
			if tc.scoring {
				changed, total := borrowChangesWinner(hybrid, seed, 60)
				if total == 0 {
					t.Fatalf("%s: no games played", tc.name)
				}
				frac := float64(changed) / float64(total)
				if frac < minWinnerFlipFrac {
					t.Errorf("%s: borrowed scoring flips the winner in only %d/%d games (%.2f < %.2f) -- it is a vestigial tally CheckEnd ignores",
						tc.name, changed, total, frac, minWinnerFlipFrac)
				}
				t.Logf("%s: dist=%.4f winnerFlip=%d/%d", tc.name, dist, changed, total)
			} else {
				t.Logf("%s: dist=%.4f (draw-penalty: descriptor-move is the teeth proof)", tc.name, dist)
			}

			// 3. Teeth must not be achieved by making the game DEGENERATE: the
			// wired hybrid must stay playable through the real pipeline (Tier 0
			// -> Tier 1 -> degeneracy vetoes) on at least one calibration seed.
			// This is the guard that, e.g., the rummy-avoidance knock-threshold
			// teeth did not collapse the game to a 1-turn instant-knock race.
			survived := 0
			for _, s := range fitness.CalibrationSeeds {
				if fitness.Evaluate(hybrid, s).Valid {
					survived++
				}
			}
			if survived == 0 {
				t.Errorf("%s: wired hybrid was killed on every calibration seed -- teeth must not come from making the game degenerate", tc.name)
			}
		})
	}
}

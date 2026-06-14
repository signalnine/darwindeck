package evolution

import (
	"math/rand/v2"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
)

// countBankingBorrows returns how many of a genome's borrows write to
// state.Scores (the "muddled scoring pile-up" the Wave-2 nudge discourages).
func countBankingBorrows(g *genome.Genome) int {
	n := 0
	for _, b := range g.Borrowed {
		switch b.Mechanic {
		case genome.MechMeldBonus, genome.MechAvoidance, genome.MechTrickScoring:
			n++
		}
	}
	return n
}

// TestCoherenceNudgeDiscouragesSecondBankingBorrow: starting from a host that
// already carries one banking borrow, repeatedly adding a borrow should LAND a
// second banking borrow LESS than always (the nudge bites) but MORE than never
// (it is a soft discouragement, not a hard veto -- legitimately complex games
// stay reachable).
func TestCoherenceNudgeDiscouragesSecondBankingBorrow(t *testing.T) {
	rng := rand.New(rand.NewPCG(21, 22))
	trials := 3000
	gotSecond := 0
	for i := 0; i < trials; i++ {
		g := &genome.Genome{
			ID: "h", Skeleton: genome.Rummy, Players: 2, HandSize: 7,
			Rummy:    &genome.RummyParams{MeldTypes: genome.MeldBoth, MinMeldSize: 3, KnockThreshold: 10},
			Borrowed: []genome.BorrowedMechanic{{Source: genome.TrickTaking, Mechanic: genome.MechTrickScoring}},
		}
		addBorrowedMechanic(g, rng, true)
		if countBankingBorrows(g) >= 2 {
			gotSecond++
		}
	}
	frac := float64(gotSecond) / float64(trials)
	if frac == 0 {
		t.Fatalf("second banking borrow NEVER added (%d/%d): nudge became a hard veto", gotSecond, trials)
	}
	if frac > 0.55 {
		t.Fatalf("second banking borrow added %.2f of the time: nudge is not biting (expected a soft reduction)", frac)
	}
}

// TestCoherenceNudgeLeavesFirstBorrowAlone: a host with NO banking borrow must
// be free to acquire its first one at the normal rate -- the nudge only
// targets the SECOND-and-beyond pile-up, never the first scoring mechanic.
func TestCoherenceNudgeLeavesFirstBorrowAlone(t *testing.T) {
	rng := rand.New(rand.NewPCG(31, 32))
	trials := 2000
	gotFirst := 0
	for i := 0; i < trials; i++ {
		g := &genome.Genome{
			ID: "h", Skeleton: genome.Rummy, Players: 2, HandSize: 7,
			Rummy: &genome.RummyParams{MeldTypes: genome.MeldBoth, MinMeldSize: 3, KnockThreshold: 10},
		}
		addBorrowedMechanic(g, rng, true)
		if countBankingBorrows(g) >= 1 {
			gotFirst++
		}
	}
	// Rummy's candidate set is {TrickScoring, DrawPenalty, Avoidance}; two of
	// three are banking, so first-banking acquisition should be common.
	if frac := float64(gotFirst) / float64(trials); frac < 0.4 {
		t.Fatalf("first banking borrow added only %.2f of the time: nudge wrongly suppressed the FIRST scoring borrow", frac)
	}
}

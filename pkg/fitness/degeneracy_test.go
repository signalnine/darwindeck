package fitness

import (
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

// --- Unit tests on hand-built batches ---

// degBatch builds a single-game batch from explicit records.
func degBatch(records []sim.TurnRecord) sim.BatchResult {
	return sim.BatchResult{GamesPlayed: 1, AllTurns: [][]sim.TurnRecord{records}}
}

// TestDegeneracyAgencyFloor: a batch where (almost) no turn is a meaningful
// decision is non-agentic -- the v1 key constraint ("games must contain
// non-random decision points"), now measurable.
func TestDegeneracyAgencyFloor(t *testing.T) {
	records := make([]sim.TurnRecord, 40)
	for i := range records {
		records[i] = sim.TurnRecord{Player: i % 2, LegalMoves: 5} // counts, no impact
	}
	g := &genome.Genome{Skeleton: genome.TrickTaking, Players: 2}
	if reason := CheckDegeneracy(degBatch(records), g); reason != "non_agentic" {
		t.Fatalf("zero meaningful decisions must flag non_agentic, got %q", reason)
	}

	// 10% meaningful clears the 5% floor.
	for i := 0; i < 4; i++ {
		records[i*10].Meaningful = true
	}
	if reason := CheckDegeneracy(degBatch(records), g); reason != "" {
		t.Fatalf("10%% meaningful turns must pass the agency floor, got %q", reason)
	}
}

// TestDegeneracyTempoMonopoly: a batch dominated by long consecutive
// same-player runs (one player acts while the rest spectate) is degenerate.
// Mean run length 10 (alternating 10-move solo stretches) trips the detector;
// strict alternation and rummy's structural 3-move turns do not.
func TestDegeneracyTempoMonopoly(t *testing.T) {
	g := &genome.Genome{Skeleton: genome.Shedding, Players: 2}

	long := make([]sim.TurnRecord, 0, 40)
	for seg := 0; seg < 4; seg++ {
		for i := 0; i < 10; i++ {
			long = append(long, sim.TurnRecord{Player: seg % 2, LegalMoves: 3, Meaningful: true})
		}
	}
	if reason := CheckDegeneracy(degBatch(long), g); reason != "tempo_monopoly" {
		t.Fatalf("mean run length 10 must flag tempo_monopoly, got %q", reason)
	}

	alternating := make([]sim.TurnRecord, 40)
	for i := range alternating {
		alternating[i] = sim.TurnRecord{Player: i % 2, LegalMoves: 3, Meaningful: true}
	}
	if reason := CheckDegeneracy(degBatch(alternating), g); reason != "" {
		t.Fatalf("strict alternation must pass, got %q", reason)
	}

	// Rummy's draw-meld-discard structure: 3-move runs are structural, not
	// degenerate (measured classics: gin/knock mean run 3.0).
	rummyish := make([]sim.TurnRecord, 0, 60)
	for cycle := 0; cycle < 10; cycle++ {
		for i := 0; i < 3; i++ {
			rummyish = append(rummyish, sim.TurnRecord{Player: cycle % 2, LegalMoves: 3, Meaningful: true})
		}
	}
	gr := &genome.Genome{Skeleton: genome.Rummy, Players: 2}
	if reason := CheckDegeneracy(degBatch(rummyish), gr); reason != "" {
		t.Fatalf("3-move rummy turn cycles must pass, got %q", reason)
	}
}

// TestDegeneracyDrawSupplyChurn: rummy-only detector. When a large share of
// all moves toggles the next player's draw options (nonzero OptionDelta --
// which in rummy attaches only to discards probing the draw phase), the game
// runs on a starved supply treadmill. Measured: gin/knock 0.010, the
// rejected pair-meld champion 0.292.
func TestDegeneracyDrawSupplyChurn(t *testing.T) {
	mk := func(deltaEvery int) []sim.TurnRecord {
		records := make([]sim.TurnRecord, 60)
		for i := range records {
			records[i] = sim.TurnRecord{Player: (i / 3) % 2, LegalMoves: 3, Meaningful: true}
			if deltaEvery > 0 && i%deltaEvery == 0 {
				records[i].OptionDelta = 1
			}
		}
		return records
	}
	gr := &genome.Genome{Skeleton: genome.Rummy, Players: 2}
	if reason := CheckDegeneracy(degBatch(mk(3)), gr); reason != "draw_supply_churn" {
		t.Fatalf("1/3 of moves toggling draw supply must flag draw_supply_churn, got %q", reason)
	}
	if reason := CheckDegeneracy(degBatch(mk(30)), gr); reason != "" {
		t.Fatalf("1/30 of moves toggling draw supply must pass, got %q", reason)
	}

	// The same delta share on a NON-rummy skeleton is regular option
	// coupling (shedding classics measure 0.16-0.18), never churn.
	gs := &genome.Genome{Skeleton: genome.Shedding, Players: 2}
	if reason := CheckDegeneracy(degBatch(mk(3)), gs); reason != "" {
		t.Fatalf("shedding option deltas are coupling, not churn; got %q", reason)
	}
}

// --- Integration: the rejected flagship champions are killed by the
// pipeline, every classic survives it ---
//
// ROUND 3 NOTE: the catch-all-skip champion (round-2 tempo_monopoly
// specimen) is no longer here -- its catch-all special is now rejected
// STATICALLY at Tier 0 (TestTier0RejectsCatchAllChampions in
// calibration_test.go), so it never reaches the veto. tempo_monopoly keeps
// its synthetic unit coverage above and gains a greedy-batch integration
// specimen in the round-3 fixtures.

func TestDegeneracyKillsRejectedChampions(t *testing.T) {
	cases := []struct {
		g      *genome.Genome
		reason string
	}{
		{seeds.NoFollowAvoidanceTrick(), "non_agentic"},
		{seeds.PairMeldKnockRummy(), "draw_supply_churn"},
	}
	for _, tc := range cases {
		res := Evaluate(tc.g, 11)
		if !res.Tier1.Passed {
			t.Errorf("%s: expected a Tier 2 degeneracy kill, but Tier 1 already killed it (%s)", tc.g.ID, tc.g.ID)
			continue
		}
		if res.Valid {
			t.Errorf("%s: rejected champion must be invalid (degeneracy veto), but Valid = true", tc.g.ID)
			continue
		}
		if res.DegenerateReason != tc.reason {
			t.Errorf("%s: degenerate reason = %q, want %q", tc.g.ID, res.DegenerateReason, tc.reason)
		}
	}
}

func TestDegeneracyPassesAllClassics(t *testing.T) {
	for _, g := range seeds.All() {
		res := Evaluate(g, 11)
		if !res.Tier1.Passed {
			continue // Tier 1 noise is Tier 1's business, not the veto's
		}
		if !res.Valid {
			t.Errorf("classic %s killed by degeneracy veto (%s) -- detector thresholds overreach",
				g.ID, res.DegenerateReason)
		}
	}
}

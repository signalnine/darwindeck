package fitness

import "testing"

// TestTier2BatchSizes pins the Tier 2 sample sizes (Task 13.2,
// docs/plans/2026-06-11-audit-remediation.md). The audit found 50 greedy
// games give SE ~= sqrt(0.5*0.5/50) ~= 0.07 on the seat-0 win rate -- too
// noisy for the skill-gradient metric to separate real skill from luck.
// 200 games bring SE down to ~0.035. Random stays at 200.
//
// If you change these constants, re-derive the SE numbers above and
// re-measure evaluation noise in the calibration suite before committing.
func TestTier2BatchSizes(t *testing.T) {
	if tier2RandomGames != 200 {
		t.Errorf("tier2RandomGames = %d, want 200", tier2RandomGames)
	}
	if tier2GreedyGames != 200 {
		t.Errorf("tier2GreedyGames = %d, want 200 (50 gives SE ~0.07 on win rate; 200 gives ~0.035)", tier2GreedyGames)
	}
}

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
		// PairMeldKnockRummy (round-2 draw_supply_churn specimen) moved to a
		// Tier-0 rejection in round 4 (min_meld_size 2) -- it no longer reaches
		// the veto; TestTier0RejectsTrivialMeldChampions covers it now.
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

// --- Round 3: seat participation + greedy-batch detectors ---

// TestDegeneracySeatParticipation: a game where one seat barely ever acts is
// a lockout (the r2 rank03 reverse ping-pong locked 2 of 4 seats out
// entirely). min-seat share of turns, averaged over games, must clear
// 0.5/numPlayers on the random batch.
func TestDegeneracySeatParticipation(t *testing.T) {
	g := &genome.Genome{Skeleton: genome.Shedding, Players: 4}

	// Seats 0 and 1 ping-pong; seats 2 and 3 get one token move each in 42
	// records: min share 1/42 << 0.5/4.
	lockout := make([]sim.TurnRecord, 0, 42)
	for i := 0; i < 40; i++ {
		lockout = append(lockout, sim.TurnRecord{Player: i % 2, LegalMoves: 3, Meaningful: true})
	}
	lockout = append(lockout,
		sim.TurnRecord{Player: 2, LegalMoves: 3, Meaningful: true},
		sim.TurnRecord{Player: 3, LegalMoves: 3, Meaningful: true})
	if reason := CheckDegeneracy(degBatch(lockout), g); reason != "seat_participation" {
		t.Fatalf("2-of-4-seat lockout must flag seat_participation, got %q", reason)
	}

	// Fair rotation: every seat at exactly 1/4 share passes.
	fair := make([]sim.TurnRecord, 40)
	for i := range fair {
		fair[i] = sim.TurnRecord{Player: i % 4, LegalMoves: 3, Meaningful: true}
	}
	if reason := CheckDegeneracy(degBatch(fair), g); reason != "" {
		t.Fatalf("fair 4-seat rotation must pass, got %q", reason)
	}

	// Mild imbalance (a seat at 1/8 share in a 4p game = exactly the 0.5/N
	// boundary) must NOT veto: the threshold is strict-below.
	mild := make([]sim.TurnRecord, 0, 40)
	for i := 0; i < 35; i++ {
		mild = append(mild, sim.TurnRecord{Player: i % 3, LegalMoves: 3, Meaningful: true})
	}
	for i := 0; i < 5; i++ {
		mild = append(mild, sim.TurnRecord{Player: 3, LegalMoves: 3, Meaningful: true})
	}
	if reason := CheckDegeneracy(degBatch(mild), g); reason != "" {
		t.Fatalf("seat at 1/8 share (= 0.5/N boundary) must pass, got %q", reason)
	}
}

// greedyBatch builds a single-game batch shaped like a Tier 2 greedy batch.
func greedyBatch(records []sim.TurnRecord, games, timeouts int) sim.BatchResult {
	res := sim.BatchResult{GamesPlayed: games, Timeouts: timeouts}
	for i := 0; i < games; i++ {
		res.AllTurns = append(res.AllTurns, records)
	}
	return res
}

// TestGreedyDegeneracyTimeout: greedy-batch timeout share > 0.10 is a veto.
// The r2 rank01 class cycled to the 390-turn cap under greedy play while
// completing fine under random -- non-termination that random Tier 1 cannot
// see. Classic greedy batches measure 0 timeouts (margins documented on the
// threshold constant).
func TestGreedyDegeneracyTimeout(t *testing.T) {
	g := &genome.Genome{Skeleton: genome.Shedding, Players: 2}
	healthy := []sim.TurnRecord{{Player: 0, LegalMoves: 3, Meaningful: true}, {Player: 1, LegalMoves: 3, Meaningful: true}}

	if reason := CheckGreedyDegeneracy(greedyBatch(healthy, 20, 3), g); reason != "greedy_timeout" {
		t.Fatalf("15%% greedy timeouts must flag greedy_timeout, got %q", reason)
	}
	if reason := CheckGreedyDegeneracy(greedyBatch(healthy, 20, 2), g); reason != "" {
		t.Fatalf("10%% greedy timeouts (= boundary) must pass, got %q", reason)
	}
	if reason := CheckGreedyDegeneracy(greedyBatch(healthy, 20, 0), g); reason != "" {
		t.Fatalf("clean greedy batch must pass, got %q", reason)
	}
}

// TestGreedyDegeneracyTempoAndSeats: tempo_monopoly and seat_participation
// run on the greedy batch too -- the round-2 blind spot (all detectors saw
// only random play) is closed: a genome whose monopoly only skilled play
// discovers is vetoed by the same thresholds, under greedy-prefixed reasons.
func TestGreedyDegeneracyTempoAndSeats(t *testing.T) {
	g := &genome.Genome{Skeleton: genome.Shedding, Players: 2}

	long := make([]sim.TurnRecord, 0, 40)
	for seg := 0; seg < 4; seg++ {
		for i := 0; i < 10; i++ {
			long = append(long, sim.TurnRecord{Player: seg % 2, LegalMoves: 3, Meaningful: true})
		}
	}
	if reason := CheckGreedyDegeneracy(greedyBatch(long, 5, 0), g); reason != "greedy_tempo_monopoly" {
		t.Fatalf("greedy-batch mean run 10 must flag greedy_tempo_monopoly, got %q", reason)
	}

	g4 := &genome.Genome{Skeleton: genome.Shedding, Players: 4}
	lockout := make([]sim.TurnRecord, 0, 42)
	for i := 0; i < 40; i++ {
		lockout = append(lockout, sim.TurnRecord{Player: i % 2, LegalMoves: 3, Meaningful: true})
	}
	lockout = append(lockout,
		sim.TurnRecord{Player: 2, LegalMoves: 3, Meaningful: true},
		sim.TurnRecord{Player: 3, LegalMoves: 3, Meaningful: true})
	// Interleave so the run length stays short: rebuild as alternating pairs
	// to isolate the seat detector from the tempo detector.
	alt := make([]sim.TurnRecord, 0, 42)
	for i := 0; i < 20; i++ {
		alt = append(alt, sim.TurnRecord{Player: 0, LegalMoves: 3}, sim.TurnRecord{Player: 1, LegalMoves: 3})
	}
	alt = append(alt, sim.TurnRecord{Player: 2, LegalMoves: 3}, sim.TurnRecord{Player: 3, LegalMoves: 3})
	if reason := CheckGreedyDegeneracy(greedyBatch(alt, 5, 0), g4); reason != "greedy_seat_participation" {
		t.Fatalf("greedy-batch seat lockout must flag greedy_seat_participation, got %q", reason)
	}

	fair := make([]sim.TurnRecord, 40)
	for i := range fair {
		fair[i] = sim.TurnRecord{Player: i % 2, LegalMoves: 3, Meaningful: true}
	}
	if reason := CheckGreedyDegeneracy(greedyBatch(fair, 5, 0), g); reason != "" {
		t.Fatalf("healthy greedy batch must pass, got %q", reason)
	}

	// Greedy batch never applies the agency floor or churn: meaningfulness
	// under a deterministic AI is not the metric's semantics, and churn is a
	// random-supply economy statistic.
	forced := make([]sim.TurnRecord, 40)
	for i := range forced {
		forced[i] = sim.TurnRecord{Player: i % 2, LegalMoves: 5} // no Meaningful
	}
	if reason := CheckGreedyDegeneracy(greedyBatch(forced, 5, 0), g); reason != "" {
		t.Fatalf("greedy batch must not apply the non_agentic floor, got %q", reason)
	}
}

// TestGreedyDegeneracyLongestRun (round 4 FIX 2): the LONGEST-run monopoly
// detector catches the single decisive burst that meanConsecutiveRun averages
// away. The r3 uninterruptible-chain champions held attack-card runs that
// played out in ONE mega-turn (6-13 consecutive plays, opponent never acted)
// while most other turns alternated, keeping the MEAN run ~1.4. The per-game
// MAXIMUM run, averaged over the batch, sees the burst.
func TestGreedyDegeneracyLongestRun(t *testing.T) {
	g := &genome.Genome{Skeleton: genome.Shedding, Players: 2}

	// Episodic monopoly: 30 alternating turns (run length 1) followed by one
	// 8-move burst by player 0. Mean run ~1.4 (well under the 6.0 mean-run
	// veto), but the per-game longest run is 8 -> longest_run fires.
	episodic := make([]sim.TurnRecord, 0, 38)
	for i := 0; i < 30; i++ {
		episodic = append(episodic, sim.TurnRecord{Player: i % 2, LegalMoves: 3, Meaningful: true})
	}
	for i := 0; i < 8; i++ {
		episodic = append(episodic, sim.TurnRecord{Player: 0, LegalMoves: 3, Meaningful: true})
	}
	if mr := meanConsecutiveRun(greedyBatch(episodic, 5, 0)); mr > degTempoMonopolyMeanRun {
		t.Fatalf("setup invalid: mean run %.2f should be under the mean-run veto so only longest_run can fire", mr)
	}
	if reason := CheckGreedyDegeneracy(greedyBatch(episodic, 5, 0), g); reason != "greedy_longest_run" {
		t.Fatalf("an 8-move burst per game must flag greedy_longest_run, got %q", reason)
	}

	// Rummy's structural draw-meld-discard cycle produces same-player runs of
	// ~3-4. A batch of pure 4-run games must NOT trip the detector (gin/knock
	// classics measure longest-run ~4.0; the threshold clears them).
	rummyCycle := make([]sim.TurnRecord, 0, 40)
	for cyc := 0; cyc < 10; cyc++ {
		for i := 0; i < 4; i++ {
			rummyCycle = append(rummyCycle, sim.TurnRecord{Player: cyc % 2, LegalMoves: 3, Meaningful: true})
		}
	}
	gr := &genome.Genome{Skeleton: genome.Rummy, Players: 2}
	if reason := CheckGreedyDegeneracy(greedyBatch(rummyCycle, 5, 0), gr); reason != "" {
		t.Fatalf("rummy's structural 4-run cycle must pass longest_run, got %q", reason)
	}

	// A 5-run game is the boundary (threshold is strict-above 5.0): must pass.
	five := make([]sim.TurnRecord, 0, 40)
	for cyc := 0; cyc < 8; cyc++ {
		for i := 0; i < 5; i++ {
			five = append(five, sim.TurnRecord{Player: cyc % 2, LegalMoves: 3, Meaningful: true})
		}
	}
	if reason := CheckGreedyDegeneracy(greedyBatch(five, 5, 0), gr); reason != "" {
		t.Fatalf("a longest-run of exactly 5 must pass (strict-above threshold), got %q", reason)
	}

	// A 6-run game trips it.
	six := make([]sim.TurnRecord, 0, 48)
	for cyc := 0; cyc < 8; cyc++ {
		for i := 0; i < 6; i++ {
			six = append(six, sim.TurnRecord{Player: cyc % 2, LegalMoves: 3, Meaningful: true})
		}
	}
	if reason := CheckGreedyDegeneracy(greedyBatch(six, 5, 0), gr); reason != "greedy_longest_run" {
		t.Fatalf("a longest-run of 6 must flag greedy_longest_run, got %q", reason)
	}
}

// TestMeanLongestRunStatistic pins the per-game-maximum aggregation: it is the
// MEAN over games of each game's LONGEST same-player run, distinct from
// meanConsecutiveRun (which averages ALL runs). The episodic batch is the
// discriminating case.
func TestMeanLongestRunStatistic(t *testing.T) {
	// One game: 1,1,1 (three alternating) then a run of 4 by player 0.
	turns := []sim.TurnRecord{
		{Player: 0}, {Player: 1}, {Player: 0},
		{Player: 1}, {Player: 1}, {Player: 1}, {Player: 1},
	}
	res := sim.BatchResult{GamesPlayed: 1, AllTurns: [][]sim.TurnRecord{turns}}
	if got := meanLongestRun(res); got != 4 {
		t.Fatalf("meanLongestRun = %.2f, want 4 (the single longest run)", got)
	}
	// meanConsecutiveRun on the same game: runs are 1,1,1,4 -> mean 1.75.
	if mr := meanConsecutiveRun(res); mr >= 4 {
		t.Fatalf("meanConsecutiveRun should average down to ~1.75, got %.2f (test setup wrong)", mr)
	}
	// Empty batch -> 0.
	if got := meanLongestRun(sim.BatchResult{}); got != 0 {
		t.Fatalf("empty batch meanLongestRun = %.2f, want 0", got)
	}
}

// TestDegeneracyDeadMatchRule (round 3): the DYNAMIC twin of the Tier-0
// catch-all rule. The r2 flagship's shedding champions carried a catch-all
// wild; with that encoding statically rejected, the same semantics remain
// reachable as a UNION (four suit wilds cover the deck). When (nearly) every
// turn offers the whole hand as legal plays, the skeleton's match/draw rules
// are dead genes dynamically -- whatever the encoding.
func TestDegeneracyDeadMatchRule(t *testing.T) {
	g := &genome.Genome{Skeleton: genome.Shedding, Players: 2}

	mk := func(allPlayableEvery int) []sim.TurnRecord {
		records := make([]sim.TurnRecord, 60)
		for i := range records {
			records[i] = sim.TurnRecord{Player: i % 2, HandSize: 7, LegalMoves: 3, Meaningful: true}
			if allPlayableEvery > 0 && i%allPlayableEvery == 0 {
				records[i].LegalMoves = 7 // whole hand playable
			}
		}
		return records
	}

	if reason := CheckDegeneracy(degBatch(mk(1)), g); reason != "dead_match_rule" {
		t.Fatalf("every-turn-all-playable must flag dead_match_rule, got %q", reason)
	}
	if reason := CheckDegeneracy(degBatch(mk(4)), g); reason != "" {
		t.Fatalf("1/4 all-playable turns (real wilds in hand) must pass, got %q", reason)
	}

	// Trivial hands cannot witness a dead match rule: a 1-card playable hand
	// is "all playable" vacuously, so HandSize < 2 records are excluded.
	tiny := make([]sim.TurnRecord, 40)
	for i := range tiny {
		tiny[i] = sim.TurnRecord{Player: i % 2, HandSize: 1, LegalMoves: 1, Meaningful: true}
	}
	if reason := CheckDegeneracy(degBatch(tiny), g); reason != "" {
		t.Fatalf("1-card hands must not witness dead_match_rule, got %q", reason)
	}

	// Non-shedding skeletons have no match rule to kill: trick-taking leads
	// legitimately offer the whole hand.
	gt := &genome.Genome{Skeleton: genome.TrickTaking, Players: 2}
	if reason := CheckDegeneracy(degBatch(mk(1)), gt); reason != "" {
		t.Fatalf("dead_match_rule must be shedding-only, got %q", reason)
	}
}

// TestDegeneracyPlayableShare (round 4 FIX 1): the PER-CARD playable share --
// mean over shedding choice-turns of PlayableCount/HandSize -- catches a wild
// UNION covering most of the deck that dead_match_rule misses. dead_match_rule
// fires only when the WHOLE hand is playable at once (LegalMoves >= HandSize),
// which at hand 13 almost never holds even when 75% of the deck is wild; the
// per-card share sees that 75% directly. Threshold 0.45: classics measure
// ~0.30, the wild-union champion ~0.62.
func TestDegeneracyPlayableShare(t *testing.T) {
	g := &genome.Genome{Skeleton: genome.Shedding, Players: 2}

	// mk builds records where each HandSize-13 turn has `playable` of 13 cards
	// playable. The whole hand is NEVER playable at once (playable <= 9), so
	// dead_match_rule (LegalMoves >= HandSize) stays silent -- only the
	// per-card share can fire.
	mk := func(playable int) []sim.TurnRecord {
		records := make([]sim.TurnRecord, 60)
		for i := range records {
			records[i] = sim.TurnRecord{
				Player: i % 2, HandSize: 13, LegalMoves: 3, Meaningful: true,
				PlayableCount: uint8(playable),
			}
		}
		return records
	}

	// 9/13 = 0.69 playable per card -> flags playable_share (and NOT
	// dead_match_rule, since LegalMoves 3 << HandSize 13).
	if reason := CheckDegeneracy(degBatch(mk(9)), g); reason != "playable_share" {
		t.Fatalf("9/13 per-card playable must flag playable_share, got %q", reason)
	}
	// 4/13 = 0.31 (classic-shedding level) -> passes.
	if reason := CheckDegeneracy(degBatch(mk(4)), g); reason != "" {
		t.Fatalf("4/13 per-card playable (classic level) must pass, got %q", reason)
	}
	// 5/13 = 0.385 -> passes (under 0.45).
	if reason := CheckDegeneracy(degBatch(mk(5)), g); reason != "" {
		t.Fatalf("5/13 per-card playable must pass (under threshold), got %q", reason)
	}

	// Trivial hands (HandSize < 2) are excluded from the share, like
	// dead_match_rule -- a 1-card playable hand is vacuously all-playable.
	tiny := make([]sim.TurnRecord, 40)
	for i := range tiny {
		tiny[i] = sim.TurnRecord{Player: i % 2, HandSize: 1, LegalMoves: 1, PlayableCount: 1, Meaningful: true}
	}
	if reason := CheckDegeneracy(degBatch(tiny), g); reason != "" {
		t.Fatalf("1-card hands must not witness playable_share, got %q", reason)
	}

	// Non-shedding skeletons never carry a playable count (the field is 0) and
	// the detector is shedding-only regardless.
	gt := &genome.Genome{Skeleton: genome.TrickTaking, Players: 2}
	if reason := CheckDegeneracy(degBatch(mk(9)), gt); reason == "playable_share" {
		t.Fatalf("playable_share must be shedding-only, got %q", reason)
	}
}

// TestDegeneracyKillsRound3Champions: the round-3 failed-review fixtures are
// killed by the pipeline -- the wild-union shedding pair by dead_match_rule
// (the union bypass of the static catch-all rule), the pair-meld-stock
// cousin by the tightened draw-supply churn.
func TestDegeneracyKillsRound3Champions(t *testing.T) {
	cases := []struct {
		g      *genome.Genome
		reason string
	}{
		{seeds.ReverseLockoutShedding(), "dead_match_rule"},
		{seeds.HeartEngineShedding(), "dead_match_rule"},
		// PairMeldStockRummy (round-3 draw_supply_churn specimen) moved to a
		// Tier-0 rejection in round 4 (min_meld_size 2, sets); it no longer
		// reaches the veto -- TestTier0RejectsTrivialMeldChampions covers it.
	}
	for _, tc := range cases {
		res := Evaluate(tc.g, 11)
		if !res.Tier1.Passed {
			t.Errorf("%s: expected a Tier 2 degeneracy kill, but Tier 1 already killed it", tc.g.ID)
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

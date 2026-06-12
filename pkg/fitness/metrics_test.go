package fitness

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

func TestSessionLengthScoring(t *testing.T) {
	tests := []struct {
		avg      float64
		expected float64
	}{
		{3, 0},    // Too short
		{5, 0},    // Minimum
		{10, 0.5}, // Ramping up
		{15, 1.0}, // Target start
		{25, 1.0}, // In range
		{40, 1.0}, // Target end
		{70, 0.5}, // Ramping down
		{100, 0},  // Maximum
		{150, 0},  // Way too long
	}

	for _, tt := range tests {
		result := sim.BatchResult{AvgTurns: tt.avg}
		score := computeSessionLength(result)
		if diff := score - tt.expected; diff > 0.01 || diff < -0.01 {
			t.Errorf("avg=%.0f: expected %.2f, got %.2f", tt.avg, tt.expected, score)
		}
	}
}

// turnsFixture builds a single-game AllTurns batch with `forced` records of
// 1 legal move and `choice` records of >=2 legal moves.
func turnsFixture(forced, choice int) sim.BatchResult {
	records := make([]sim.TurnRecord, 0, forced+choice)
	for i := 0; i < forced; i++ {
		records = append(records, sim.TurnRecord{Player: i % 2, LegalMoves: 1})
	}
	for i := 0; i < choice; i++ {
		records = append(records, sim.TurnRecord{Player: i % 2, LegalMoves: 3})
	}
	return sim.BatchResult{AllTurns: [][]sim.TurnRecord{records}}
}

func TestDecisionDensityAllForcedIsZero(t *testing.T) {
	got := computeDecisionDensity(turnsFixture(10, 0))
	if got != 0.0 {
		t.Fatalf("all-forced fixture (every turn 1 legal move) must score 0.0, got %.3f", got)
	}
}

func TestDecisionDensityAllChoiceIsOne(t *testing.T) {
	got := computeDecisionDensity(turnsFixture(0, 10))
	if got != 1.0 {
		t.Fatalf("all-choice fixture (every turn >=2 legal moves) must score 1.0, got %.3f", got)
	}
}

func TestDecisionDensityMixed30_70(t *testing.T) {
	// 30% of turns offer a real choice, 70% are forced => density 0.3.
	got := computeDecisionDensity(turnsFixture(7, 3))
	if diff := got - 0.3; diff > 1e-9 || diff < -1e-9 {
		t.Fatalf("mixed 30/70 fixture must score 0.3, got %.3f", got)
	}
}

func TestDecisionDensityEmptyBatchIsZero(t *testing.T) {
	if got := computeDecisionDensity(sim.BatchResult{}); got != 0.0 {
		t.Fatalf("empty batch must score 0.0, got %.3f", got)
	}
}

// TestDecisionDensityWhistNotPinned: the old event-taxonomy metric scored every
// trick-taking game exactly 1.0 (every event was a "play"). Real whist forces
// follow-suit on many turns, so legal-move-count density must be strictly
// between 0 and 1.
func TestDecisionDensityWhistNotPinned(t *testing.T) {
	g := seeds.Whist()
	runner := GetRunner(g)
	result := sim.RunBatch(g, runner, &sim.RandomAI{}, 50, 0)

	density := computeDecisionDensity(result)
	t.Logf("Whist decision density: %.3f", density)

	if density <= 0.0 || density >= 1.0 {
		t.Fatalf("whist density must be strictly between 0 and 1 (old structural pin was 1.0), got %.3f", density)
	}
}

func TestFullEvaluation(t *testing.T) {
	allSeeds := []*genome.Genome{
		seeds.CrazyEights(),
		seeds.Whist(),
		seeds.Hearts(),
		seeds.GinRummy(),
	}

	for _, g := range allSeeds {
		result := Evaluate(g, 0)

		if !result.Valid {
			t.Errorf("%s: evaluation invalid. Tier0: %v, Tier1: %s",
				g.ID, result.Tier0Errors, result.Tier1.Reason)
			continue
		}

		m := result.Metrics
		t.Logf("%s: fitness=%.3f decisions=%.3f arc=%.3f interact=%.3f skill=%.3f length=%.3f",
			g.ID, m.TotalFitness, m.MeaningfulDecisions, m.GameArc,
			m.Interaction, m.SkillGradient, m.SessionLength)

		if m.TotalFitness <= 0 {
			t.Errorf("%s: total fitness should be > 0, got %.3f", g.ID, m.TotalFitness)
		}
		if m.TotalFitness > 1.0 {
			t.Errorf("%s: total fitness should be <= 1.0, got %.3f", g.ID, m.TotalFitness)
		}
	}
}

func TestInvalidGenomeGetsZeroFitness(t *testing.T) {
	g := &genome.Genome{
		ID:       "broken",
		Skeleton: genome.Shedding,
		Players:  10, // Invalid
		HandSize: 7,
	}

	result := Evaluate(g, 0)
	if result.Valid {
		t.Fatal("broken genome should not be valid")
	}
	if result.Metrics.TotalFitness != 0 {
		t.Fatalf("broken genome should have 0 fitness, got %.3f", result.Metrics.TotalFitness)
	}
}

// The turn-CV scale-invariance regression test (cards-6n3) was deleted along
// with the turn-CV term itself: audit Task 10 replaced the seat-entropy +
// turn-CV GameArc with a lead-trajectory arc (comeback + resolution + lead
// changes), and duration spread is SessionLength's job, not the arc's. See
// docs/plans/2026-06-11-audit-remediation.md, Task 10.

// leaderBatch builds a BatchResult whose only populated per-game data is the
// leader tracks -- the sole input the lead-trajectory GameArc consumes.
func leaderBatch(tracks ...[]int8) sim.BatchResult {
	return sim.BatchResult{
		GamesPlayed: len(tracks),
		AllLeaders:  tracks,
	}
}

// repeatLeader returns a track of n samples all led by player p.
func repeatLeader(p int8, n int) []int8 {
	track := make([]int8, n)
	for i := range track {
		track[i] = p
	}
	return track
}

// comebackTrack: 20 samples where player 1 leads through the midgame sample
// (index 10), there is a brief scuffle, and player 0 takes the lead for good
// at 75% (index 15) and wins. 3 lead changes.
func comebackTrack() []int8 {
	return []int8{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 1, 1, 0, 0, 0, 0, 0}
}

// holdTrack: 20 samples with an early scuffle (3 lead changes) after which
// player 0 leads from before midgame to the win.
func holdTrack() []int8 {
	return []int8{1, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
}

// TestGameArcWireToWireForegoneConclusion: required case (a). A leader who is
// ahead at every sample and wins gives resolution=1, comeback=0, zero lead
// changes => arc = 0.4*tent(0, 0.5) + 0.4*1 + 0.2*0 = 0.4 exactly.
func TestGameArcWireToWireForegoneConclusion(t *testing.T) {
	tracks := make([][]int8, 6)
	for i := range tracks {
		tracks[i] = repeatLeader(0, 20)
	}
	batch := leaderBatch(tracks...)

	comeback, resolution, leadChanges, counted := arcStats(batch)
	if counted != 6 {
		t.Fatalf("all 6 tracks qualify, counted %d", counted)
	}
	if resolution != 1.0 {
		t.Errorf("wire-to-wire winner must give resolution 1.0, got %.3f", resolution)
	}
	if comeback != 0.0 {
		t.Errorf("wire-to-wire winner must give comeback 0.0, got %.3f", comeback)
	}
	if leadChanges != 0.0 {
		t.Errorf("wire-to-wire track has 0 lead changes, got %.3f", leadChanges)
	}

	got := computeGameArc(batch)
	if math.Abs(got-0.4) > 1e-9 {
		t.Fatalf("wire-to-wire batch must score exactly 0.4 (resolution-only), got %.4f", got)
	}
}

// TestGameArcRandomLeaderRandomWinnerIsLow: required case (b). With 4 players
// and an i.i.d.-random leader at every sample, the final sample (the inferred
// winner) is independent of the 90% sample, so resolution ~ 1/N and the arc
// must land well below a good arc (~0.5 = 0.4*tent(0.75,0.5) + 0.4*0.25 +
// 0.2*saturated lead changes), while staying above zero.
func TestGameArcRandomLeaderRandomWinnerIsLow(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 0))
	tracks := make([][]int8, 60)
	for i := range tracks {
		track := make([]int8, 40)
		for j := range track {
			track[j] = int8(rng.IntN(4))
		}
		tracks[i] = track
	}
	batch := leaderBatch(tracks...)

	_, resolution, _, counted := arcStats(batch)
	if counted != 60 {
		t.Fatalf("all 60 tracks qualify, counted %d", counted)
	}
	if resolution < 0.10 || resolution > 0.45 {
		t.Errorf("random-track resolution should be near 1/N = 0.25, got %.3f", resolution)
	}

	got := computeGameArc(batch)
	t.Logf("random-leader arc: %.3f (resolution %.3f)", got, resolution)
	if got <= 0.05 || got >= 0.6 {
		t.Fatalf("random leader + random winner must score a low (but nonzero) arc, got %.3f", got)
	}
}

// TestGameArcComebackBatchIsHigh: required case (c). Games where the winner
// trails at the 50% sample and leads from 75% on, mixed with held-lead games
// so the batch comeback rate sits at the 0.5 target (a batch where the winner
// ALWAYS comes from behind is itself a foregone conclusion -- the midgame
// leader always loses -- and the tent term zeroes it by design; see
// TestGameArcAllComebackIsAntiPredictive). Resolution 1, comeback 0.5, 3 lead
// changes per game => arc must exceed 0.7.
func TestGameArcComebackBatchIsHigh(t *testing.T) {
	tracks := make([][]int8, 0, 10)
	for i := 0; i < 5; i++ {
		tracks = append(tracks, comebackTrack(), holdTrack())
	}
	batch := leaderBatch(tracks...)

	got := computeGameArc(batch)
	t.Logf("comeback-mix arc: %.3f", got)
	if got <= 0.7 {
		t.Fatalf("winner-trails-at-50%%-leads-from-75%% mix must score arc > 0.7, got %.3f", got)
	}
}

// TestGameArcAllComebackIsAntiPredictive documents the tent term: if EVERY
// game is a comeback, the midgame leader always loses, which is just an
// inverted foregone conclusion. comeback=1 => tent=0, so only resolution
// (0.4) and saturated lead changes (0.2) score: arc = 0.6 exactly, strictly
// below the mixed batch of TestGameArcComebackBatchIsHigh. This is why the
// plan's test (c) shape needs a ~0.5 batch comeback rate to clear 0.7.
func TestGameArcAllComebackIsAntiPredictive(t *testing.T) {
	tracks := make([][]int8, 10)
	for i := range tracks {
		tracks[i] = comebackTrack()
	}
	batch := leaderBatch(tracks...)

	got := computeGameArc(batch)
	if math.Abs(got-0.6) > 1e-9 {
		t.Fatalf("all-comeback batch must score exactly 0.6 (tent(1,0.5)=0), got %.4f", got)
	}
}

// TestGameArcSkipsShortAndWinnerlessTracks: tracks with fewer than 4 leader
// samples, or without a strict final leader (no winner), contribute nothing;
// a batch with no qualifying game scores 0.
func TestGameArcSkipsShortAndWinnerlessTracks(t *testing.T) {
	short := []int8{0, 0, 0}             // 3 samples < 4 minimum
	winnerless := []int8{0, 0, 0, 0, -1} // tie at the end => no strict winner

	if got := computeGameArc(leaderBatch(short, winnerless)); got != 0 {
		t.Fatalf("batch with only short/winnerless tracks must score 0, got %.4f", got)
	}
	if got := computeGameArc(sim.BatchResult{}); got != 0 {
		t.Fatalf("empty batch must score 0, got %.4f", got)
	}

	// One qualifying wire-to-wire game alongside the skipped tracks: only it
	// counts, so the batch scores the wire-to-wire 0.4.
	got := computeGameArc(leaderBatch(short, winnerless, repeatLeader(0, 8)))
	if math.Abs(got-0.4) > 1e-9 {
		t.Fatalf("skipped tracks must not dilute qualifying games: want 0.4, got %.4f", got)
	}
}

// TestGameArcLeadChangesIgnoreTies: -1 (tie) samples are skipped when
// counting lead changes -- a tie interlude within one player's lead is not a
// lead change, while a tie bridging two different leaders is exactly one.
func TestGameArcLeadChangesIgnoreTies(t *testing.T) {
	sameLeader := []int8{0, -1, 0, -1, 0, 0, 0, 0}
	_, _, leadChanges, counted := arcStats(leaderBatch(sameLeader))
	if counted != 1 {
		t.Fatalf("track qualifies (8 samples, strict final leader), counted %d", counted)
	}
	if leadChanges != 0 {
		t.Errorf("0,-1,0 interludes are not lead changes: want 0, got %.3f", leadChanges)
	}

	bridged := []int8{0, 0, -1, 1, 1, 1, 1, 1}
	_, _, leadChanges, _ = arcStats(leaderBatch(bridged))
	if leadChanges != 1 {
		t.Errorf("0,-1,1 bridge is exactly one lead change: want 1, got %.3f", leadChanges)
	}

	// A tie at the 50% sample means the winner was not strictly leading
	// there: it counts as a comeback, and a tie at 90% counts against
	// resolution.
	midTie := []int8{0, 0, 0, 0, 0, 0, -1, 0, 0, 0, -1, 0} // n=12: samples at idx 6 (50%) and 10 (90%) are ties
	comeback, resolution, _, _ := arcStats(leaderBatch(midTie))
	if comeback != 1 {
		t.Errorf("tie at the 50%% sample counts toward comeback: want 1, got %.3f", comeback)
	}
	if resolution != 0 {
		t.Errorf("tie at the 90%% sample counts against resolution: want 0, got %.3f", resolution)
	}
}

func TestSkillGradientUsesEmpiricalBaseline(t *testing.T) {
	// Greedy always plays seat 0. A game with first-player advantage gives seat 0
	// a high random win rate; comparing greedy's seat-0 rate against the theoretical
	// 1/N rather than the *measured* seat-0 random rate miscredits that structural
	// edge as skill. The baseline must come from randomResult, not 1/numPlayers.
	mk := func(seat0Wins int) sim.BatchResult {
		return sim.BatchResult{Completions: 100, WinCounts: []int{seat0Wins, 100 - seat0Wins}}
	}

	// Zero true skill (greedy == random seat-0 rate) must score 0, regardless of
	// how strong the first-player advantage is.
	if got := computeSkillGradient(mk(60), mk(60), 2); got != 0 {
		t.Errorf("FPA game with greedy==random seat-0 rate (60%%) must score 0, got %.3f", got)
	}
	if got := computeSkillGradient(mk(50), mk(50), 2); got != 0 {
		t.Errorf("fair game with greedy==random seat-0 rate (50%%) must score 0, got %.3f", got)
	}

	// A genuine skill edge over the empirical baseline still scores positive.
	if got := computeSkillGradient(mk(50), mk(60), 2); got <= 0 {
		t.Errorf("greedy seat-0 rate (60%%) above random baseline (50%%) must score >0, got %.3f", got)
	}

	// Greedy worse than the empirical baseline clamps to 0.
	if got := computeSkillGradient(mk(60), mk(40), 2); got != 0 {
		t.Errorf("greedy seat-0 rate (40%%) below random baseline (60%%) must score 0, got %.3f", got)
	}
}

func TestSkillGradient(t *testing.T) {
	// Greedy should beat random in trick-taking (play to win tricks).
	// Player 0 uses greedy; the other seats stay random.
	g := seeds.Whist()
	runner := GetRunner(g)
	randomAI := &sim.RandomAI{}

	randomResult := sim.RunBatch(g, runner, randomAI, 100, 0)
	greedyResult := runGreedyBatch(g, runner, 100, 500)

	skill := computeSkillGradient(randomResult, greedyResult, g.Players)
	t.Logf("Whist skill gradient: %.3f (random wins=%v, greedy wins=%v)",
		skill, randomResult.WinCounts, greedyResult.WinCounts)

	// With only P0 greedy vs random opponents, P0 should win noticeably
	// more than the 1/N baseline. If this drops to zero the mix is broken.
	if skill <= 0 {
		t.Errorf("expected positive skill gradient for Whist greedy vs random, got %.3f", skill)
	}
}

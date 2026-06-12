package fitness

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

// sessionBatch builds a batch where game i holds perPlayer[i] TurnRecords for
// EACH of numPlayers players (seats alternate), so game i's decisions-per-
// player is exactly perPlayer[i]. AvgTurns is deliberately left zero: session
// length must read TurnRecords, not the skeleton-dependent state.Turn unit
// (audit Task 12).
func sessionBatch(numPlayers int, perPlayer ...int) sim.BatchResult {
	games := make([][]sim.TurnRecord, len(perPlayer))
	for i, n := range perPlayer {
		records := make([]sim.TurnRecord, 0, n*numPlayers)
		for j := 0; j < n*numPlayers; j++ {
			records = append(records, sim.TurnRecord{Player: j % numPlayers, LegalMoves: 2})
		}
		games[i] = records
	}
	return sim.BatchResult{GamesPlayed: len(perPlayer), AllTurns: games}
}

// TestSessionLengthScoring pins the falloff curve in the new unit (decisions
// per player): hard zero below 5 and above 100, ramp 5->15, flat band 15-40,
// ramp 40->100. Shape is unchanged from the old curve; only the unit moved
// (Task 14 recalibrates the band itself).
func TestSessionLengthScoring(t *testing.T) {
	tests := []struct {
		dpp      int
		expected float64
	}{
		{3, 0},    // Too short (below hard cutoff)
		{5, 0},    // Minimum (ramp starts at 0 here)
		{10, 0.5}, // Ramping up
		{15, 1.0}, // Target start
		{25, 1.0}, // In range
		{40, 1.0}, // Target end
		{70, 0.5}, // Ramping down
		{100, 0},  // Maximum (ramp reaches 0 here)
		{150, 0},  // Way too long (above hard cutoff)
	}

	for _, tt := range tests {
		score := computeSessionLength(sessionBatch(2, tt.dpp), 2)
		if diff := score - tt.expected; diff > 0.01 || diff < -0.01 {
			t.Errorf("decisions/player=%d: expected %.2f, got %.2f", tt.dpp, tt.expected, score)
		}
	}
}

// TestSessionLengthDecisionsPerPlayerExact: required fixture case. Known
// TurnRecords => exact decisions/player. Game 1: player 0 acts 12 times,
// player 1 acts 18 times => (12+18)/2 = 15. Game 2: 20 and 30 => 25. Per-game
// values averaged over the batch => 20.
func TestSessionLengthDecisionsPerPlayerExact(t *testing.T) {
	mkGame := func(p0, p1 int) []sim.TurnRecord {
		records := make([]sim.TurnRecord, 0, p0+p1)
		for i := 0; i < p0; i++ {
			records = append(records, sim.TurnRecord{Player: 0, LegalMoves: 2})
		}
		for i := 0; i < p1; i++ {
			records = append(records, sim.TurnRecord{Player: 1, LegalMoves: 2})
		}
		return records
	}
	batch := sim.BatchResult{
		GamesPlayed: 2,
		AllTurns:    [][]sim.TurnRecord{mkGame(12, 18), mkGame(20, 30)},
	}

	got := avgDecisionsPerPlayer(batch, 2)
	if got != 20.0 {
		t.Fatalf("decisions/player: want exactly 20.0 (mean of per-game 15 and 25), got %.3f", got)
	}

	// 20 decisions/player sits inside the 15-40 band => score 1.0.
	if score := computeSessionLength(batch, 2); score != 1.0 {
		t.Fatalf("20 decisions/player is in band, want 1.0, got %.3f", score)
	}
}

// TestSessionLengthEmptyBatchIsZero: no games (or no players) => 0, not NaN.
func TestSessionLengthEmptyBatchIsZero(t *testing.T) {
	if got := avgDecisionsPerPlayer(sim.BatchResult{}, 2); got != 0 {
		t.Fatalf("empty batch decisions/player must be 0, got %.3f", got)
	}
	if got := computeSessionLength(sim.BatchResult{}, 2); got != 0 {
		t.Fatalf("empty batch session length must be 0, got %.3f", got)
	}
	if got := computeSessionLength(sessionBatch(2, 25), 0); got != 0 {
		t.Fatalf("zero players must score 0, got %.3f", got)
	}
}

// TestSessionLengthUnitIsPerPlayerNotPerTurn: two batches with the SAME total
// turn count (52 records) but different player counts must score differently:
// 52 records across 2 players = 26 decisions/player (in band, 1.0); across 4
// players = 13 decisions/player (low ramp, 0.8). The old unit collapsed both
// to "52 turns" and scored both 0.8.
func TestSessionLengthUnitIsPerPlayerNotPerTurn(t *testing.T) {
	twoPlayer := sessionBatch(2, 26)  // 52 records
	fourPlayer := sessionBatch(4, 13) // 52 records

	if got := computeSessionLength(twoPlayer, 2); got != 1.0 {
		t.Errorf("52 records / 2 players = 26 decisions/player, want 1.0, got %.3f", got)
	}
	got := computeSessionLength(fourPlayer, 4)
	if math.Abs(got-0.8) > 1e-9 {
		t.Errorf("52 records / 4 players = 13 decisions/player, want 0.8, got %.3f", got)
	}
}

// TestSessionLengthWhistNotInflatedByPerCardTurns: required real-evaluation
// case. The old unit was state.Turn, which trick-taking increments PER CARD:
// a 2-player whist with full-deck hands plays ~52 cards, landing deep in the
// old 40-100 falloff even though each player makes only ~26 decisions --
// squarely inside the 15-40 band. The new unit must (a) measure ~26
// decisions/player, (b) score in-band 1.0, and (c) beat the score the old
// inflated unit would have given.
func TestSessionLengthWhistNotInflatedByPerCardTurns(t *testing.T) {
	g := seeds.Whist()
	g.ID = "whist-2p"
	g.Players = 2
	g.HandSize = 26 // full deck: 52 card-play turns, 26 decisions per player
	runner := GetRunner(g)
	result := sim.RunBatch(g, runner, &sim.RandomAI{}, 50, 0)
	if result.Completions != 50 {
		t.Fatalf("whist-2p must complete all 50 games, got %d (errors=%d timeouts=%d)",
			result.Completions, result.Errors, result.Timeouts)
	}

	// Premise check: the old per-card unit really does inflate into the
	// falloff region (40, 100) -- otherwise this test proves nothing.
	if result.AvgTurns <= 40 || result.AvgTurns >= 100 {
		t.Fatalf("premise broken: old-unit avg turns should sit in the (40,100) falloff, got %.1f", result.AvgTurns)
	}
	oldScore := (100 - result.AvgTurns) / 60 // the old unit's down-ramp at this avg

	dpp := avgDecisionsPerPlayer(result, g.Players)
	t.Logf("whist-2p: old-unit avg turns %.1f (old score %.3f), decisions/player %.1f", result.AvgTurns, oldScore, dpp)
	if dpp < 15 || dpp > 40 {
		t.Fatalf("whist-2p decisions/player must land in the 15-40 band, got %.1f", dpp)
	}

	got := computeSessionLength(result, g.Players)
	if got != 1.0 {
		t.Fatalf("whist-2p is in band under the decisions/player unit, want 1.0, got %.3f", got)
	}
	if got <= oldScore {
		t.Fatalf("new unit must beat the old inflated-unit score: new %.3f <= old %.3f", got, oldScore)
	}

	// The 4-player seed whist plays the same 52 cards but only 13 decisions
	// per player -- the unit must attribute decisions to players, not count
	// cards. 13 lands on the low ramp (0.8): near the band and far from the
	// zero region the inflated unit pushed it toward; Task 14 recalibrates
	// the band from the classics.
	g4 := seeds.Whist()
	result4 := sim.RunBatch(g4, GetRunner(g4), &sim.RandomAI{}, 50, 0)
	dpp4 := avgDecisionsPerPlayer(result4, g4.Players)
	if dpp4 != 13.0 {
		t.Fatalf("seed whist (4p) must measure exactly 13 decisions/player (52 cards / 4 seats), got %.2f", dpp4)
	}
	got4 := computeSessionLength(result4, g4.Players)
	if math.Abs(got4-0.8) > 1e-9 {
		t.Fatalf("seed whist (4p) at 13 decisions/player must score 0.8 on the low ramp, got %.3f", got4)
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

// leaderBatch builds a BatchResult from leader tracks plus the real batch
// winners the arc consumes (audit Wave D fix 1): every fixture game here is a
// COMPLETED game whose winner is its final strict leader (-1, i.e. skipped,
// when the track is empty or ends tied). Non-completed games and games whose
// winner differs from the final leader are built explicitly in
// TestGameArcSkipsNonCompletedGames / TestGameArcWinnerFromBatchNotFinalLeader.
func leaderBatch(tracks ...[]int8) sim.BatchResult {
	winners := make([]int, len(tracks))
	for i, track := range tracks {
		winners[i] = -1
		if n := len(track); n > 0 {
			winners[i] = int(track[n-1])
		}
	}
	return sim.BatchResult{
		GamesPlayed: len(tracks),
		AllLeaders:  tracks,
		AllWinners:  winners,
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

// TestGameArcSkipsNonCompletedGames (audit Wave D fix 1): the batch runner
// appends leader tracks for ALL games, including max_turns/stuck/no_moves
// exits, and those games record Winner -1 even when their leader track ends
// with a strict leader. The old code inferred winner = final leader sample, so
// a timed-out game contributed arc stats attributed to a player who won
// nothing. Non-completed games must contribute NOTHING; completed games still
// do.
func TestGameArcSkipsNonCompletedGames(t *testing.T) {
	timedOut := repeatLeader(0, 20) // strict final leader, but Winner -1

	solo := sim.BatchResult{
		GamesPlayed: 1,
		AllLeaders:  [][]int8{timedOut},
		AllWinners:  []int{-1},
	}
	if _, _, _, counted := arcStats(solo); counted != 0 {
		t.Fatalf("timed-out game must not be counted, counted %d", counted)
	}
	if got := computeGameArc(solo); got != 0 {
		t.Fatalf("batch of only non-completed games must score 0, got %.4f", got)
	}

	// Alongside one completed wire-to-wire game, only the completed game
	// counts: arc is the wire-to-wire 0.4 exactly, undiluted and uninflated.
	mixed := sim.BatchResult{
		GamesPlayed: 2,
		AllLeaders:  [][]int8{timedOut, repeatLeader(0, 20)},
		AllWinners:  []int{-1, 0},
	}
	if _, _, _, counted := arcStats(mixed); counted != 1 {
		t.Fatalf("only the completed game qualifies, counted %d", counted)
	}
	if got := computeGameArc(mixed); math.Abs(got-0.4) > 1e-9 {
		t.Fatalf("completed wire-to-wire game must still score 0.4, got %.4f", got)
	}
}

// TestGameArcWinnerFromBatchNotFinalLeader (audit Wave D fix 1): for rummy
// genomes with scoring borrows, the final Progress leader (live deadwood) can
// differ from the actual CheckEnd winner (state.Scores incl. hook
// contributions). The arc must attribute the win to the REAL winner: player 0
// led wire-to-wire on the track, but player 1 won -- that is a comeback with
// zero resolution, not a resolved hold.
func TestGameArcWinnerFromBatchNotFinalLeader(t *testing.T) {
	batch := sim.BatchResult{
		GamesPlayed: 1,
		AllLeaders:  [][]int8{repeatLeader(0, 20)},
		AllWinners:  []int{1},
	}
	comeback, resolution, leadChanges, counted := arcStats(batch)
	if counted != 1 {
		t.Fatalf("completed game must be counted, counted %d", counted)
	}
	if comeback != 1 {
		t.Errorf("real winner never led at 50%%: comeback must be 1, got %.3f", comeback)
	}
	if resolution != 0 {
		t.Errorf("real winner did not lead at the resolution sample: want 0, got %.3f", resolution)
	}
	if leadChanges != 0 {
		t.Errorf("track has 0 lead changes, got %.3f", leadChanges)
	}
}

// TestGameArcShortCoinFlipScoresLowResolution (audit Wave D fix 2): the
// metric's headline promise is "a pure coin flip decided on the last move
// scores low on resolution". But for any track of n <= 10 samples,
// n*9/10 == n-1: the resolution sample WAS the final sample, which correlates
// with (here, equals) the winner, so ultra-short games -- the instant-knock
// degenerate class the calibration suite targets -- got resolution ~1 for
// free. With the sample clamped to min(n*9/10, n-2), a 6-sample batch whose
// lead see-saws and is decided only by the final flip scores ZERO resolution
// (the old index scored it 1.0, arc 0.6).
func TestGameArcShortCoinFlipScoresLowResolution(t *testing.T) {
	flipA := []int8{0, 1, 0, 1, 0, 1} // winner 1, trailing at every even sample
	flipB := []int8{1, 0, 1, 0, 1, 0} // winner 0, mirrored
	tracks := make([][]int8, 0, 10)
	for i := 0; i < 5; i++ {
		tracks = append(tracks, flipA, flipB)
	}
	batch := leaderBatch(tracks...)

	_, resolution, _, counted := arcStats(batch)
	if counted != 10 {
		t.Fatalf("all 10 tracks qualify (6 samples, completed), counted %d", counted)
	}
	if resolution != 0 {
		t.Fatalf("coin flip decided at the final sample must score 0 resolution, got %.3f", resolution)
	}

	// The whole arc collapses to the saturated lead-changes term (0.2):
	// comeback 0 (the winner leads at the 50%% sample in both shapes) zeroes
	// the tent, resolution is 0, lead changes saturate.
	if got := computeGameArc(batch); math.Abs(got-0.2) > 1e-9 {
		t.Fatalf("coin-flip batch must score exactly 0.2 (lead-changes term only), got %.4f", got)
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

// interactionFixture builds a single-game batch from explicit turn records
// and events -- the two inputs computeInteraction consumes (audit Task 11).
func interactionFixture(records []sim.TurnRecord, events []sim.Event) sim.BatchResult {
	return sim.BatchResult{
		GamesPlayed: 1,
		AllTurns:    [][]sim.TurnRecord{records},
		AllEvents:   [][]sim.Event{events},
	}
}

// TestInteractionSolitaireLikeIsZero: required case 1. A game where no move
// ever changes the next player's options (all OptionDelta 0) and no
// direct-attack event fires is NOT interactive, even though every play lands
// on the shared discard pile. The old event-taxonomy metric scored exactly
// this fixture 1.0 (every "discard"-detail EventCardPlayed counted) -- that
// was its central flaw.
func TestInteractionSolitaireLikeIsZero(t *testing.T) {
	records := make([]sim.TurnRecord, 10)
	events := make([]sim.Event, 0, 13)
	for i := 0; i < 10; i++ {
		records[i] = sim.TurnRecord{Player: i % 2, LegalMoves: 3, OptionDelta: 0}
		events = append(events, sim.Event{Type: sim.EventCardPlayed, PlayerID: i % 2, Detail: "discard"})
	}
	for i := 0; i < 3; i++ {
		events = append(events, sim.Event{Type: sim.EventCardDrawn, PlayerID: i % 2})
	}

	if got := computeInteraction(interactionFixture(records, events)); got != 0 {
		t.Fatalf("solitaire-like fixture (all deltas 0, no attack events) must score 0, got %.3f", got)
	}
	if got := computeInteraction(sim.BatchResult{}); got != 0 {
		t.Fatalf("empty batch must score 0, got %.3f", got)
	}
}

// TestInteractionDrawTwoHeavySheddingIsHigh: required case 2. A draw-2-heavy
// shedding game: most moves perturb the next player's options (OptionDelta
// != 0) and draw_two specials fire. The event stream is dominated by the
// victims' penalty EventCardDrawn entries, which the old metric scored
// against (3 specials / 20 events => 0.5); the perturbation metric reads the
// 6/10 interactive turns instead.
func TestInteractionDrawTwoHeavySheddingIsHigh(t *testing.T) {
	records := []sim.TurnRecord{
		{Player: 0, LegalMoves: 4, OptionDelta: -2},
		{Player: 1, LegalMoves: 2, OptionDelta: 0},
		{Player: 0, LegalMoves: 3, OptionDelta: 3},
		{Player: 1, LegalMoves: 5, OptionDelta: -1},
		{Player: 0, LegalMoves: 2, OptionDelta: 0},
		{Player: 1, LegalMoves: 3, OptionDelta: 2},
		{Player: 0, LegalMoves: 4, OptionDelta: 0},
		{Player: 1, LegalMoves: 2, OptionDelta: -3},
		{Player: 0, LegalMoves: 3, OptionDelta: 1},
		{Player: 1, LegalMoves: 2, OptionDelta: 0},
	}
	events := make([]sim.Event, 0, 20)
	for i := 0; i < 3; i++ {
		events = append(events, sim.Event{Type: sim.EventSpecialTriggered, PlayerID: 1, Detail: "draw_two"})
	}
	for i := 0; i < 17; i++ {
		events = append(events, sim.Event{Type: sim.EventCardDrawn, PlayerID: i % 2})
	}

	got := computeInteraction(interactionFixture(records, events))
	if got < 0.9 {
		t.Fatalf("draw-2-heavy fixture (6/10 turns perturb options) must score high (>= 0.9), got %.3f", got)
	}
}

// TestInteractionTrickEventsCountWithoutDeltas: trick-taking records
// OptionDelta 0 BY DESIGN (follow-suit legality is set by the acting player,
// so the counterfactual is ill-defined); its interaction signal is the
// EventTrickWon stream. 2 tricks over 8 card-play turns => ratio 0.25, and
// with the provisional clamp(ratio/0.5) scale that is exactly 0.5 (Task 14
// recalibrates the denominator; update this expectation there).
func TestInteractionTrickEventsCountWithoutDeltas(t *testing.T) {
	records := make([]sim.TurnRecord, 8)
	events := make([]sim.Event, 0, 10)
	for i := 0; i < 8; i++ {
		records[i] = sim.TurnRecord{Player: i % 4, LegalMoves: 2, OptionDelta: 0}
		events = append(events, sim.Event{Type: sim.EventCardPlayed, PlayerID: i % 4})
	}
	events = append(events,
		sim.Event{Type: sim.EventTrickWon, PlayerID: 2},
		sim.Event{Type: sim.EventTrickWon, PlayerID: 0},
	)

	got := computeInteraction(interactionFixture(records, events))
	if math.Abs(got-0.5) > 1e-9 {
		t.Fatalf("2 tricks / 8 turns must score exactly 0.5 under the provisional scale, got %.3f", got)
	}
}

// TestInteractionNonAttackEventsDoNotCount: only the opponent-affecting
// special details emitted by the shedding runner (skip, draw_two, draw_four,
// reverse) and EventTrickWon are attacks. A hypothetical self-targeted
// special, a meld, a discard-detail play, and a round end are not -- the old
// metric counted three of these four.
func TestInteractionNonAttackEventsDoNotCount(t *testing.T) {
	records := make([]sim.TurnRecord, 4)
	for i := range records {
		records[i] = sim.TurnRecord{Player: i % 2, LegalMoves: 3, OptionDelta: 0}
	}
	events := []sim.Event{
		{Type: sim.EventSpecialTriggered, PlayerID: 0, Detail: "wild_suit_chosen"},
		{Type: sim.EventMeldLaid, PlayerID: 1},
		{Type: sim.EventCardPlayed, PlayerID: 0, Detail: "discard"},
		{Type: sim.EventRoundEnd, PlayerID: 1, Detail: "gin"},
	}

	if got := computeInteraction(interactionFixture(records, events)); got != 0 {
		t.Fatalf("non-attack events must not count as interaction, got %.3f", got)
	}
}

// TestInteractionHearts4pNotPinnedToOldConstant: required case 3. The old
// event-taxonomy metric scored hearts-4p a deterministic 0.657 (13 TrickWon /
// 66 events / 0.3 scale) regardless of what happened in the games. The
// perturbation metric must produce something else on a real evaluation.
func TestInteractionHearts4pNotPinnedToOldConstant(t *testing.T) {
	g := seeds.Hearts()
	runner := GetRunner(g)
	result := sim.RunBatch(g, runner, &sim.RandomAI{}, 50, 0)

	got := computeInteraction(result)
	t.Logf("hearts-4p interaction: %.4f", got)
	if math.Abs(got-0.657) < 0.005 {
		t.Fatalf("hearts-4p interaction still pins to the old deterministic 0.657, got %.4f", got)
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

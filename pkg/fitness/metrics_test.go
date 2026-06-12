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

// TestSessionLengthScoring pins the falloff curve in decisions per player.
func TestSessionLengthScoring(t *testing.T) {
	// Band calibrated by Task 14 from the measured classic spread and
	// re-derived in round 2 (see computeSessionLength): flat [6, 60], ramps
	// 3..6 and 60..170. The low edge moved 10 -> 6 because oh-hell (7.0
	// decisions/player, a classic) sat in the previous ramp at 0.5.
	tests := []struct {
		dpp      int
		expected float64
	}{
		{2, 0},          // Too short (below hard cutoff)
		{3, 0},          // Minimum (ramp starts at 0 here)
		{4, 1.0 / 3.0},  // Ramping up
		{5, 2.0 / 3.0},  // Ramping up
		{6, 1.0},        // Target start
		{7, 1.0},        // oh-hell's natural length: in band since round 2
		{25, 1.0},       // In range
		{60, 1.0},       // Target end
		{115, 0.5},      // Ramping down
		{170, 0},        // Maximum (ramp reaches 0 here)
		{180, 0},        // Way too long (above hard cutoff)
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

	// 20 decisions/player sits inside the [10, 60] band => score 1.0.
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
// turn count (280 records) but different player counts must score differently:
// 280 records across 2 players = 140 decisions/player (high ramp, (170-140)/110
// = 0.2727..); across 4 players = 70 decisions/player ((170-70)/110 = 0.9090..).
// A per-turn unit would collapse both to "280 turns" and score them equally.
func TestSessionLengthUnitIsPerPlayerNotPerTurn(t *testing.T) {
	twoPlayer := sessionBatch(2, 140) // 280 records
	fourPlayer := sessionBatch(4, 70) // 280 records

	got2 := computeSessionLength(twoPlayer, 2)
	if math.Abs(got2-30.0/110.0) > 1e-9 {
		t.Errorf("280 records / 2 players = 140 decisions/player, want %.4f, got %.4f", 30.0/110.0, got2)
	}
	got4 := computeSessionLength(fourPlayer, 4)
	if math.Abs(got4-100.0/110.0) > 1e-9 {
		t.Errorf("280 records / 4 players = 70 decisions/player, want %.4f, got %.4f", 100.0/110.0, got4)
	}
	if got4 <= got2 {
		t.Errorf("same total records must score differently by player count: 4p %.4f <= 2p %.4f", got4, got2)
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
	if dpp < 10 || dpp > 60 {
		t.Fatalf("whist-2p decisions/player must land in the [10, 60] band, got %.1f", dpp)
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
	// cards. 13 sits inside the Task 14 band [10, 60] (calibrated to admit
	// the trick-taking classics), scoring 1.0.
	g4 := seeds.Whist()
	result4 := sim.RunBatch(g4, GetRunner(g4), &sim.RandomAI{}, 50, 0)
	dpp4 := avgDecisionsPerPlayer(result4, g4.Players)
	if dpp4 != 13.0 {
		t.Fatalf("seed whist (4p) must measure exactly 13 decisions/player (52 cards / 4 seats), got %.2f", dpp4)
	}
	got4 := computeSessionLength(result4, g4.Players)
	if got4 != 1.0 {
		t.Fatalf("seed whist (4p) at 13 decisions/player is in band, want 1.0, got %.3f", got4)
	}
}

// turnsFixture builds a single-game AllTurns batch with `forced` records of
// 1 legal move and `choice` records of >=2 legal moves whose choice was
// MEANINGFUL (the batch runner's choice-impact sampling found differing
// signatures -- Task 28 round 2; before that fix the flag did not exist and
// density counted raw LegalMoves >= 2).
func turnsFixture(forced, choice int) sim.BatchResult {
	records := make([]sim.TurnRecord, 0, forced+choice)
	for i := 0; i < forced; i++ {
		records = append(records, sim.TurnRecord{Player: i % 2, LegalMoves: 1})
	}
	for i := 0; i < choice; i++ {
		records = append(records, sim.TurnRecord{Player: i % 2, LegalMoves: 3, Meaningful: true})
	}
	return sim.BatchResult{AllTurns: [][]sim.TurnRecord{records}}
}

// TestDecisionDensityInflatedCountsDoNotCount (Task 28 round 2): turns with
// many legal moves but Meaningful = false (all-wild same-effect hands,
// no-follow trick hands) are NOT decisions. The legal-move COUNT alone was
// the archetype A1/A2 inflation vector (0.86-0.92 densities for games with
// near-zero choice impact).
func TestDecisionDensityInflatedCountsDoNotCount(t *testing.T) {
	records := make([]sim.TurnRecord, 10)
	for i := range records {
		records[i] = sim.TurnRecord{Player: i % 2, LegalMoves: 13} // huge counts...
	}
	records[2].Meaningful = true // ...but only one turn's choice mattered
	batch := sim.BatchResult{AllTurns: [][]sim.TurnRecord{records}}
	if got := computeDecisionDensity(batch); math.Abs(got-0.1) > 1e-9 {
		t.Fatalf("10 high-count turns with 1 meaningful must score 0.1, got %.3f", got)
	}
}

// TestDecisionDensityNoFollowTrickCollapses (Task 28 round 2, archetype A2
// integration): the rejected no-follow flagship champion's decision density
// must collapse to exactly 0 -- every lead leaves the follower's options
// untouched and every follow completes the trick.
func TestDecisionDensityNoFollowTrickCollapses(t *testing.T) {
	g := seeds.NoFollowAvoidanceTrick()
	result := sim.RunBatch(g, GetRunner(g), &sim.RandomAI{}, 20, 11)
	if got := computeDecisionDensity(result); got != 0 {
		t.Fatalf("no-follow avoidance trick density must be exactly 0, got %.3f", got)
	}
}

// TestDecisionDensityCatchAllSkipCollapses (Task 28 round 2, archetype A1
// integration): the catch-all-skip champion measured 0.874 under raw
// legal-move counting. Under choice-impact sampling the pure wild-count
// inflation is gone: same-profile self-returning plays (the catch-all skip
// makes EVERY 2p play self-returning, so no coupling probe applies) collapse
// to not-meaningful. What honestly REMAINS (measured 0.633) is the fixture's
// profile mixing: a third of the deck inflicts draw-two/draw-four, and
// dump-an-attack vs dump-a-plain-card is a real choice even in 2p. The bound
// pins the inflation removal (0.874 -> below 0.7); the full separation of
// this archetype from the classics is the calibration gate's job, where its
// remaining density is weighed against its collapsed interaction.
func TestDecisionDensityCatchAllSkipCollapses(t *testing.T) {
	g := seeds.CatchAllSkipShedding()
	result := sim.RunBatch(g, GetRunner(g), &sim.RandomAI{}, 20, 11)
	got := computeDecisionDensity(result)
	t.Logf("catch-all-skip density: %.3f", got)
	if got >= 0.7 {
		t.Fatalf("catch-all-skip density must collapse below 0.7 (was 0.874 with raw counts), got %.3f", got)
	}
	// The same genome with the wild/attack suits stripped to a SINGLE shared
	// profile is the pure all-wild inflation case and must fully collapse.
	pure := &genome.Genome{
		Skeleton: genome.Shedding,
		Players:  2,
		HandSize: 13,
		Shedding: &genome.SheddingParams{MatchRule: genome.MatchEither, DrawPenalty: 2},
		SpecialCards: []genome.SpecialCard{
			{Type: genome.SpecialSkip},
			{Type: genome.SpecialWild},
		},
	}
	pureResult := sim.RunBatch(pure, GetRunner(pure), &sim.RandomAI{}, 20, 11)
	if got := computeDecisionDensity(pureResult); got != 0 {
		t.Fatalf("pure all-wild catch-all-skip density must be exactly 0, got %.3f", got)
	}
}

// TestDecisionDensityCrazyEightsWellAboveZero (Task 28 round 2): the
// choice-impact filter must not destroy the signal for real games -- crazy
// eights' suit/rank choices change the opponent's options and stay
// meaningful.
func TestDecisionDensityCrazyEightsWellAboveZero(t *testing.T) {
	g := seeds.CrazyEights()
	result := sim.RunBatch(g, GetRunner(g), &sim.RandomAI{}, 50, 11)
	got := computeDecisionDensity(result)
	t.Logf("crazy-eights density: %.3f", got)
	if got < 0.1 {
		t.Fatalf("crazy-eights density must stay well above 0 under choice-impact sampling, got %.3f", got)
	}
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
// != 0) and three of the remaining turns are draw_two attacks (Attack set at
// record time, audit Wave D fix 3). Union: 9 of 10 turns interactive. The
// event stream is dominated by the victims' penalty EventCardDrawn entries,
// which the old event-taxonomy metric scored against (3 specials / 20 events
// => 0.5); the per-turn metric reads the records instead.
func TestInteractionDrawTwoHeavySheddingIsHigh(t *testing.T) {
	records := []sim.TurnRecord{
		{Player: 0, LegalMoves: 4, OptionDelta: -2},
		{Player: 1, LegalMoves: 2, OptionDelta: 0, Attack: true},
		{Player: 0, LegalMoves: 3, OptionDelta: 3},
		{Player: 1, LegalMoves: 5, OptionDelta: -1},
		{Player: 0, LegalMoves: 2, OptionDelta: 0, Attack: true},
		{Player: 1, LegalMoves: 3, OptionDelta: 2},
		{Player: 0, LegalMoves: 4, OptionDelta: 0, Attack: true},
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
		t.Fatalf("draw-2-heavy fixture (9/10 turns interactive) must score high (>= 0.9), got %.3f", got)
	}
}

// TestInteractionTrickAttackTurnsCount: in trick-taking the interaction
// signal reaches the records as Attack flags on the trick-completing moves
// (one per EventTrickWon, set at record time -- audit Wave D fix 3). 2
// attack-flagged turns over 8 card-play turns => ratio 0.25, and with the
// provisional clamp(ratio/0.5) scale that is exactly 0.5 (Task 14
// recalibrates the denominator; update this expectation there). Hand-derived
// on the new turn-count basis: same value the old event-count basis gave for
// this shape, because each trick win maps to exactly one move.
func TestInteractionTrickAttackTurnsCount(t *testing.T) {
	records := make([]sim.TurnRecord, 8)
	events := make([]sim.Event, 0, 10)
	for i := 0; i < 8; i++ {
		records[i] = sim.TurnRecord{Player: i % 4, LegalMoves: 2, OptionDelta: 0}
		events = append(events, sim.Event{Type: sim.EventCardPlayed, PlayerID: i % 4})
	}
	records[3].Attack = true // 4th card completes trick 1
	records[7].Attack = true // 8th card completes trick 2
	events = append(events,
		sim.Event{Type: sim.EventTrickWon, PlayerID: 2},
		sim.Event{Type: sim.EventTrickWon, PlayerID: 0},
	)

	got := computeInteraction(interactionFixture(records, events))
	if math.Abs(got-0.5) > 1e-9 {
		t.Fatalf("2 attack turns / 8 turns must score exactly 0.5 under the provisional scale, got %.3f", got)
	}
}

// TestInteractionCollapsesOnTwoPlayerSelfSkip (Task 28 round 2, interaction
// fix): the archetype-A1 engine distilled -- a 2-player shedding genome where
// every card is wild AND every play skips. The mover plays their entire hand
// while the opponent spectates; nothing any player does ever touches the
// other. Hand-computed expectation: every turn is a self-skip play (no
// attack at 2p, next actor == mover so no coupling delta) => interaction is
// EXACTLY 0. Before the fix this shape scored 1.00 (every play emitted a
// "skip" attack event and the self-probe registered the mover's own hand
// shrinking).
func TestInteractionCollapsesOnTwoPlayerSelfSkip(t *testing.T) {
	g := &genome.Genome{
		Skeleton: genome.Shedding,
		Players:  2,
		HandSize: 5,
		Shedding: &genome.SheddingParams{MatchRule: genome.MatchEither, DrawPenalty: 1},
		SpecialCards: []genome.SpecialCard{
			{Type: genome.SpecialSkip}, // catch-all skip
			{Type: genome.SpecialWild}, // catch-all wild
		},
	}
	result := sim.RunBatch(g, GetRunner(g), &sim.RandomAI{}, 20, 11)
	if result.Completions == 0 {
		t.Fatal("premise broken: all-wild shedding games must complete")
	}
	if got := computeInteraction(result); got != 0 {
		t.Fatalf("2p self-skip game must score interaction 0 (self-tempo, no coupling), got %.3f", got)
	}

	// Same genome at 4 players: skips now cost real turns, every play is an
	// attack, so interaction must saturate, not collapse.
	g4 := *g
	g4.Players = 4
	result4 := sim.RunBatch(&g4, GetRunner(&g4), &sim.RandomAI{}, 20, 11)
	if got := computeInteraction(result4); got < 0.9 {
		t.Fatalf("4p catch-all skip must stay highly interactive, got %.3f", got)
	}
}

// TestInteractionNonAttackEventsDoNotCount: only the opponent-affecting
// special details emitted by the shedding runner (skip, draw_two, draw_four,
// reverse) and EventTrickWon flag a turn as an attack (sim.IsAttackEvent, the
// single whitelist). A hypothetical self-targeted special, a meld, a
// discard-detail play, and a round end do not -- the old metric counted three
// of these four -- so records stay Attack-free and the score is 0.
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
	for _, e := range events {
		if sim.IsAttackEvent(e, 4) {
			t.Fatalf("whitelist regression: %v must not be an attack event", e)
		}
	}

	if got := computeInteraction(interactionFixture(records, events)); got != 0 {
		t.Fatalf("non-attack events must not count as interaction, got %.3f", got)
	}
}

// TestInteractionUnionCountsDisjointMixedGames (audit Wave D fix 3): a mixed
// game where 2 turns perturb options and 2 OTHER turns attack has 4
// interactive turns. The old max(deltaTurns, attackEvents) basis collapsed
// that to max(2,2) = 2/10 => 0.4, undercounting disjoint mixed games by half;
// the exact union scores 4/10 => 0.8.
func TestInteractionUnionCountsDisjointMixedGames(t *testing.T) {
	records := make([]sim.TurnRecord, 10)
	for i := range records {
		records[i] = sim.TurnRecord{Player: i % 2, LegalMoves: 3, OptionDelta: 0}
	}
	records[1].OptionDelta = -2
	records[4].OptionDelta = 1
	records[6].Attack = true
	records[8].Attack = true

	got := computeInteraction(interactionFixture(records, nil))
	if math.Abs(got-0.8) > 1e-9 {
		t.Fatalf("disjoint union (2 delta + 2 attack turns of 10) must score 0.8, got %.3f", got)
	}

	// A turn that both perturbs AND attacks is still one interactive turn.
	records[1].Attack = true
	got = computeInteraction(interactionFixture(records, nil))
	if math.Abs(got-0.8) > 1e-9 {
		t.Fatalf("overlapping delta+attack turn must not double count: want 0.8, got %.3f", got)
	}
}

// TestInteractionStackedSpecialTurnCountsOnce (audit Wave D fix 3): one move
// playing a skip+reverse+draw-two card emits THREE attack events but is ONE
// interactive turn. The old basis read the event stream and scored 3/10 =>
// 0.6; the record basis scores 1/10 => 0.2. The three stacked events in the
// fixture must stay invisible to the metric.
func TestInteractionStackedSpecialTurnCountsOnce(t *testing.T) {
	records := make([]sim.TurnRecord, 10)
	for i := range records {
		records[i] = sim.TurnRecord{Player: i % 2, LegalMoves: 3, OptionDelta: 0}
	}
	records[0].Attack = true // the stacked-special move, recorded once
	events := []sim.Event{
		{Type: sim.EventSpecialTriggered, PlayerID: 1, Detail: "draw_two"},
		{Type: sim.EventSpecialTriggered, PlayerID: 1, Detail: "skip"},
		{Type: sim.EventSpecialTriggered, Detail: "reverse"},
	}

	got := computeInteraction(interactionFixture(records, events))
	if math.Abs(got-0.2) > 1e-9 {
		t.Fatalf("stacked special is ONE interactive turn (1/10 => 0.2), got %.3f", got)
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

// TestInteractionTrickTakingNoLongerSkeletonConstant (audit Wave D fix 4):
// with OptionDelta defined as always-0 for trick-taking, Interaction was the
// closed-form constant 2/N for EVERY trick-taking genome (= 0.5 at 4 players:
// one trick-completing attack turn per N card-play turns, doubled by the
// /0.5 scale), recreating the audit's skeleton-constant pathology. With
// lead-constraint deltas the measured values (50 random games, seed 0) are
// hearts 0.8485, whist 0.8423, oh-hell 0.7571 -- all far from the old
// constant. The ROBUST genome gradient is whist (13-card hands) vs oh-hell
// (7-card hands): a >= 0.02 gap stable across seeds. Hearts vs whist differ
// only at noise scale (~0.006; their order flips across seeds), so this test
// pins their exact inequality at the fixed seed but does NOT claim a robust
// hearts/whist gap -- measured honestly, their lead-constraint profiles
// coincide at 13-card hands.
func TestInteractionTrickTakingNoLongerSkeletonConstant(t *testing.T) {
	measure := func(g *genome.Genome) float64 {
		return computeInteraction(sim.RunBatch(g, GetRunner(g), &sim.RandomAI{}, 50, 0))
	}
	whist := measure(seeds.Whist())
	hearts := measure(seeds.Hearts())
	ohHell := measure(seeds.OhHell())
	t.Logf("trick-taking interaction: whist=%.4f hearts=%.4f oh-hell=%.4f (old constant: 0.5)", whist, hearts, ohHell)

	for name, v := range map[string]float64{"whist": whist, "hearts": hearts, "oh-hell": ohHell} {
		if math.Abs(v-0.5) < 0.2 {
			t.Errorf("%s interaction %.4f still sits near the closed-form 2/N constant 0.5", name, v)
		}
	}
	if whist == hearts {
		t.Errorf("whist and hearts interaction identical (%.4f): per-genome variation lost", whist)
	}
	if math.Abs(whist-ohHell) < 0.02 {
		t.Errorf("whist (%.4f) vs oh-hell (%.4f) must separate by >= 0.02 (hand-size gradient)", whist, ohHell)
	}
}

// mkWins builds an n-game batch where seat 0 won seat0Wins games -- the
// shared fixture for the skill-gradient unit tests below.
func mkWins(seat0Wins, n int) sim.BatchResult {
	return sim.BatchResult{Completions: n, WinCounts: []int{seat0Wins, n - seat0Wins}}
}

// twoTierExpected mirrors the metric's final scaling so expected values in
// tests track the calibrated skillScale constant instead of hardcoding it.
func twoTierExpected(raw float64) float64 {
	return clamp(raw/skillScale, 0, 1)
}

func TestSkillGradientUsesEmpiricalBaseline(t *testing.T) {
	// Greedy always plays seat 0. A game with first-player advantage gives seat 0
	// a high random win rate; comparing greedy's seat-0 rate against the theoretical
	// 1/N rather than the *measured* seat-0 random rate miscredits that structural
	// edge as skill. The baseline must come from randomResult, not 1/numPlayers.
	mk := func(seat0Wins int) sim.BatchResult { return mkWins(seat0Wins, 100) }

	// Zero true skill (greedy == random seat-0 rate) must score 0, regardless of
	// how strong the first-player advantage is.
	if got := computeSkillGradient(mk(60), mk(60), sim.BatchResult{}, 2); got != 0 {
		t.Errorf("FPA game with greedy==random seat-0 rate (60%%) must score 0, got %.3f", got)
	}
	if got := computeSkillGradient(mk(50), mk(50), sim.BatchResult{}, 2); got != 0 {
		t.Errorf("fair game with greedy==random seat-0 rate (50%%) must score 0, got %.3f", got)
	}

	// A genuine skill edge over the empirical baseline still scores positive.
	if got := computeSkillGradient(mk(50), mk(60), sim.BatchResult{}, 2); got <= 0 {
		t.Errorf("greedy seat-0 rate (60%%) above random baseline (50%%) must score >0, got %.3f", got)
	}

	// Greedy worse than the empirical baseline clamps to 0.
	if got := computeSkillGradient(mk(60), mk(40), sim.BatchResult{}, 2); got != 0 {
		t.Errorf("greedy seat-0 rate (40%%) below random baseline (60%%) must score 0, got %.3f", got)
	}
}

// TestTwoTierSkillZeroSkillGame pins the Task 20 requirement: a zero-skill
// game (greedy == random == mcts seat-0 rates) scores EXACTLY 0 regardless of
// first-player advantage. Both tiers use empirical seat-0 baselines (dd-qt7):
// the greedy term baselines on the random batch, the MCTS term on the greedy
// batch, so a structural seat edge cancels out of both differences.
func TestTwoTierSkillZeroSkillGame(t *testing.T) {
	for _, rate := range []int{50, 60, 75, 90} {
		got := computeSkillGradient(mkWins(rate, 100), mkWins(rate, 100), mkWins(rate/5, 20), 2)
		if got != 0 {
			t.Errorf("zero-skill game at seat-0 rate %d%% must score 0, got %.3f", rate, got)
		}
	}
}

// TestTwoTierSkillGreedyOnlyCapped pins the cap: a game whose skill is fully
// greedy-detectable but where MCTS does NO better than greedy is capped at
// the 0.4 greedy term -- the top 0.6 of the raw scale is reachable only by
// outplaying greedy. The cap must hold identically whether the MCTS batch
// ties greedy, loses to greedy, or is absent (greedy-only mode).
func TestTwoTierSkillGreedyOnlyCapped(t *testing.T) {
	random, greedy := mkWins(50, 100), mkWins(100, 100) // t1 = (1.0-0.5)/0.5 = 1.0
	want := twoTierExpected(0.4)

	cases := map[string]sim.BatchResult{
		"absent (greedy-only mode)": {},
		"ties greedy":               mkWins(20, 20),
		"below greedy":              mkWins(10, 20),
	}
	for name, mcts := range cases {
		if got := computeSkillGradient(random, greedy, mcts, 2); math.Abs(got-want) > 1e-9 {
			t.Errorf("MCTS %s: greedy-only-detectable skill must equal the scaled 0.4 term %.3f, got %.3f",
				name, want, got)
		}
	}

	// Same cap below saturation: greedy 80% over a 50% baseline (t1 = 0.6),
	// MCTS exactly matching greedy adds nothing.
	want = twoTierExpected(0.4 * 0.6)
	if got := computeSkillGradient(mkWins(50, 100), mkWins(80, 100), mkWins(16, 20), 2); math.Abs(got-want) > 1e-9 {
		t.Errorf("partial greedy-only skill must equal scaled 0.4*0.6 = %.3f, got %.3f", want, got)
	}
}

// TestTwoTierSkillMCTSTermAddsAboveGreedy verifies the verbatim plan formula
// on hand-computed values, and that supplying MCTS data can only raise skill
// relative to greedy-only mode (the second max(0, ...) term is nonnegative).
func TestTwoTierSkillMCTSTermAddsAboveGreedy(t *testing.T) {
	// random 50%, greedy 75%, mcts 90%:
	//   t1 = 0.4*(0.75-0.50)/(1-0.50) = 0.4*0.5 = 0.20
	//   t2 = 0.6*(0.90-0.75)/(1-0.75) = 0.6*0.6 = 0.36
	//   raw = 0.56
	want := twoTierExpected(0.56)
	got := computeSkillGradient(mkWins(50, 100), mkWins(75, 100), mkWins(18, 20), 2)
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("two-tier formula: want scaled 0.56 = %.3f, got %.3f", want, got)
	}

	greedyOnly := computeSkillGradient(mkWins(50, 100), mkWins(75, 100), sim.BatchResult{}, 2)
	if got < greedyOnly {
		t.Errorf("two-tier skill %.3f below greedy-only skill %.3f: MCTS term went negative", got, greedyOnly)
	}
}

// TestTwoTierSkillWarLikeFixture: a war-like game has no decisions, so no AI
// can beat any other -- all three seat-0 rates sit at the same coin-flip
// value (modulo sampling noise) and skill must be ~0.
func TestTwoTierSkillWarLikeFixture(t *testing.T) {
	if got := computeSkillGradient(mkWins(50, 100), mkWins(50, 100), mkWins(10, 20), 2); got != 0 {
		t.Errorf("war-like fixture with identical rates must score exactly 0, got %.3f", got)
	}
	// With realistic sampling noise (greedy 52%, mcts 50%) it stays near 0.
	if got := computeSkillGradient(mkWins(50, 100), mkWins(52, 100), mkWins(10, 20), 2); got > 0.05 {
		t.Errorf("war-like fixture with noise-level greedy edge must score ~0 (<= 0.05), got %.3f", got)
	}
}

// TestTwoTierSkillPerfectGreedyNoNaN: greedyWR == 1.0 zeroes the MCTS term's
// denominator; the term must drop to 0 (nothing left to detect), never NaN.
func TestTwoTierSkillPerfectGreedyNoNaN(t *testing.T) {
	got := computeSkillGradient(mkWins(50, 100), mkWins(100, 100), mkWins(20, 20), 2)
	if math.IsNaN(got) {
		t.Fatal("perfect greedy + perfect mcts produced NaN")
	}
	if want := twoTierExpected(0.4); math.Abs(got-want) > 1e-9 {
		t.Errorf("perfect greedy caps at the scaled 0.4 term %.3f, got %.3f", want, got)
	}
}

// TestComputeFitnessWrapperEquivalence: the 3-arg ComputeFitness (kept for
// callers that have no MCTS batch, e.g. pkg/evolution/behavior.go) must be
// exactly the 4-arg version with an empty MCTS result.
func TestComputeFitnessWrapperEquivalence(t *testing.T) {
	g := seeds.CrazyEights()
	runner := GetRunner(g)
	random := sim.RunBatch(g, runner, &sim.RandomAI{}, 20, 7)
	greedy := runGreedyBatch(g, runner, 20, 507)

	a := ComputeFitness(random, greedy, g.Players)
	b := ComputeFitnessWithMCTS(random, greedy, sim.BatchResult{}, g.Players)
	if a != b {
		t.Errorf("ComputeFitness diverged from ComputeFitnessWithMCTS with empty MCTS batch:\n  %+v\n  %+v", a, b)
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

	skill := computeSkillGradient(randomResult, greedyResult, sim.BatchResult{}, g.Players)
	t.Logf("Whist skill gradient: %.3f (random wins=%v, greedy wins=%v)",
		skill, randomResult.WinCounts, greedyResult.WinCounts)

	// With only P0 greedy vs random opponents, P0 should win noticeably
	// more than the 1/N baseline. If this drops to zero the mix is broken.
	if skill <= 0 {
		t.Errorf("expected positive skill gradient for Whist greedy vs random, got %.3f", skill)
	}
}

// TestDecisionDensityRummyDeadwoodConsequence (Task 28 round 3, the
// predicted count-gamed density archetype, arrived as r2 ranks 21-30): under
// the count-based rummy exception ("meaningful iff >= 2 legal moves") the
// pair-meld archetype (min_meld_size 2, DrawEither, big hands over a starved
// stock) pinned density 0.80 ABOVE gin's 0.69 -- pair hands manufacture
// option counts in every phase while the choices carry no deadwood
// consequence. With the deadwood-consequence probe (rummy.Runner
// ChoiceMatters), gin must sit clearly above the pair-meld archetype on
// density, and gin itself must not crater (its discard decisions are real).
func TestDecisionDensityRummyDeadwoodConsequence(t *testing.T) {
	// rank22-style pair-meld: 4p, hand 12, sets of 2, DrawEither, knock 21
	// (3-card stock; the archetype that parked churn just under the 0.10
	// veto cliff).
	pairMeld := &genome.Genome{
		ID:       "pair-meld-inline",
		Skeleton: genome.Rummy,
		Players:  4,
		HandSize: 12,
		Rummy: &genome.RummyParams{
			MeldTypes:      genome.MeldSets,
			MinMeldSize:    2,
			DrawFrom:       genome.DrawEither,
			KnockThreshold: 21,
		},
	}
	gin := seeds.GinRummy()

	ginResult := sim.RunBatch(gin, GetRunner(gin), &sim.RandomAI{}, 30, 11)
	pairResult := sim.RunBatch(pairMeld, GetRunner(pairMeld), &sim.RandomAI{}, 30, 11)

	ginDensity := computeDecisionDensity(ginResult)
	pairDensity := computeDecisionDensity(pairResult)
	t.Logf("gin density %.3f, pair-meld density %.3f", ginDensity, pairDensity)

	// Measured at the round-3 commit: gin 0.365 vs pair-meld 0.276 (gap
	// +0.089; per-seed density sd is ~0.003 in the calibration table, so the
	// +0.05 bar leaves ~30 sd of regression headroom). Under the count
	// exception the same batches measured gin 0.691 vs pair-meld 0.869 --
	// the INVERTED ordering this test exists to keep dead.
	if ginDensity <= pairDensity+0.05 {
		t.Errorf("gin density %.3f must clearly exceed pair-meld %.3f (+0.05): the count-gamed archetype is back",
			ginDensity, pairDensity)
	}
	if ginDensity < 0.30 {
		t.Errorf("gin density cratered to %.3f (< 0.30); the probe is collapsing real discard decisions", ginDensity)
	}
}

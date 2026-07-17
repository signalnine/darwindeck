package playtest

import (
	"bufio"
	"encoding/json"
	"io"
	"io/fs"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
	"github.com/darwindeck/darwindeck/pkg/sim"
	"github.com/darwindeck/darwindeck/pkg/skeleton/rummy"
	"github.com/darwindeck/darwindeck/pkg/skeleton/shedding"
)

// stubRunner advances the active player on every move, mirroring how real
// shedding/trick-taking runners mutate state.Active inside ApplyMove.
type stubRunner struct{}

func (stubRunner) Setup(g *genome.Genome, rng *rand.Rand) *sim.GameState { return nil }
func (stubRunner) Upkeep(state *sim.GameState, g *genome.Genome)         {}
func (stubRunner) GenerateMoves(state *sim.GameState, g *genome.Genome) []sim.Move {
	return nil
}
func (stubRunner) ApplyMove(state *sim.GameState, move sim.Move, g *genome.Genome) []sim.Event {
	state.NextPlayer()
	return nil
}
func (stubRunner) CheckEnd(state *sim.GameState, g *genome.Genome) int { return -1 }
func (stubRunner) Progress(state *sim.GameState, g *genome.Genome) []float64 {
	return nil
}

// stubAI returns a fixed move regardless of state.
type stubAI struct{ move sim.Move }

func (s stubAI) SelectMove(moves []sim.Move, state *sim.GameState, rng *rand.Rand) sim.Move {
	return s.move
}

// TestAITurnReportsActorBeforeMove verifies that aiTurn attributes the action
// to the player who actually moved, not the next player. The bug was that
// aiTurn read s.State.Active AFTER ApplyMove, which had already advanced it.
func TestAITurnReportsActorBeforeMove(t *testing.T) {
	state := sim.NewGameState(3)
	state.Active = 1
	state.Hands[0] = []sim.Card{}
	state.Hands[1] = []sim.Card{}
	state.Hands[2] = []sim.Card{}

	move := sim.Move{Type: sim.MovePass, PlayerID: 1}

	s := &Session{
		Genome:  &genome.Genome{},
		Runner:  stubRunner{},
		AI:      stubAI{move: move},
		State:   state,
		RNG:     rand.New(rand.NewPCG(1, 2)),
		HumanID: 0,
	}

	preActive := s.State.Active
	actor := s.aiTurn([]sim.Move{move})

	if actor != preActive {
		t.Fatalf("aiTurn reported actor=%d, want %d (the player who moved)", actor, preActive)
	}
	if s.State.Active == preActive {
		t.Fatalf("expected ApplyMove to advance state.Active away from %d, but it did not — test setup broken", preActive)
	}
}

// TestNewMCTSAIIsFullyWired pins the dangerous failure mode of the mcts
// difficulty: sim.MCTSAI with a nil Runner or Genome silently degrades to
// uniform random (a deliberate batch-safety fallback), so a half-wired
// constructor would ship a "mcts" opponent that actually plays randomly.
func TestNewMCTSAIIsFullyWired(t *testing.T) {
	g := seeds.GinRummy()
	runner := &rummy.Runner{}

	ai := NewMCTSAI(g, runner)

	if ai.Runner == nil {
		t.Fatal("NewMCTSAI returned nil Runner: MCTSAI would degrade to random play")
	}
	if ai.Genome == nil {
		t.Fatal("NewMCTSAI returned nil Genome: MCTSAI would degrade to random play")
	}
}

// timedAI wraps an AIPlayer to count decisions and total decision latency.
type timedAI struct {
	inner sim.AIPlayer
	calls int
	total time.Duration
}

func (a *timedAI) SelectMove(moves []sim.Move, state *sim.GameState, rng *rand.Rand) sim.Move {
	start := time.Now()
	mv := a.inner.SelectMove(moves, state, rng)
	a.total += time.Since(start)
	a.calls++
	return mv
}

// TestMCTSSessionCompletesGame runs a fully scripted playtest session against
// the mcts difficulty: the "human" (seat 0) always picks move 1 from stdin
// while the MCTS opponent plays the other seat. Gin rummy is used because the
// rummy runner is the most expensive movegen MCTS can face — this doubles as
// the interactive-latency check (sub-second per AI move, Task 21).
func TestMCTSSessionCompletesGame(t *testing.T) {
	g := seeds.GinRummy()
	runner := &rummy.Runner{}
	ai := &timedAI{inner: NewMCTSAI(g, runner)}

	s := NewSession(g, runner, ai, 7)
	// One stdin line per human decision; a rummy game is bounded by
	// MaxTurns (208) at a few moves per turn, so 4096 lines can never run
	// out (EOF would os.Exit the test binary).
	s.Scanner = bufio.NewScanner(strings.NewReader(strings.Repeat("1\n", 4096)))

	// Silence the interactive transcript for the duration of Run.
	oldStdout := os.Stdout
	devnull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open %s: %v", os.DevNull, err)
	}
	defer func() {
		os.Stdout = oldStdout
		devnull.Close()
	}()
	os.Stdout = devnull

	s.Run()

	os.Stdout = oldStdout

	if s.State == nil {
		t.Fatal("Run never set up a game state")
	}
	if s.State.Turn == 0 {
		t.Fatal("game made no progress")
	}
	// Run returns on exactly three paths: winner, max-turns cap, or the
	// no-legal-moves stuck path. CheckEnd is pure (audit Wave B), so
	// re-querying it distinguishes a real ending from a stuck session.
	winner := runner.CheckEnd(s.State, g)
	if winner < 0 && s.State.Turn < g.MaxTurns() {
		t.Fatalf("session got stuck at turn %d: no winner and below max turns %d",
			s.State.Turn, g.MaxTurns())
	}
	if ai.calls == 0 {
		t.Fatal("MCTS opponent was never asked for a move")
	}

	avg := ai.total / time.Duration(ai.calls)
	t.Logf("mcts difficulty: %d AI moves, avg %v per move (winner=%d, turns=%d)",
		ai.calls, avg, winner, s.State.Turn)
	// Interactive budget: sub-second per move. Measured ~10-30ms on the
	// worst-case skeleton, so 1s gives wide headroom against slow CI.
	if avg >= time.Second {
		t.Fatalf("mcts difficulty too slow for interactive play: avg %v per move (budget < 1s)", avg)
	}
}

// --- Task 24: playtest hook parity + human ratings capture ---

// borrowGenome builds a valid shedding genome carrying the MechAvoidance
// borrow with an all-cards penalty rule (Rank 0 / Suit 0 wildcards): any
// round end MUST bank a negative score for every player still holding cards.
// In this genome state.Scores is written ONLY by the borrowed-mechanic hooks,
// so a Scores mutation is deterministic proof the hooks ran.
func borrowGenome() *genome.Genome {
	return &genome.Genome{
		ID:       "session-hook-parity",
		Skeleton: genome.Shedding,
		Players:  2,
		HandSize: 5,
		Borrowed: []genome.BorrowedMechanic{
			{Source: genome.TrickTaking, Mechanic: genome.MechAvoidance},
		},
		Scoring: genome.ScoringConfig{
			CardPoints: []genome.CardScoring{{Points: 1}}, // every card is a penalty
		},
		Shedding: &genome.SheddingParams{
			MatchRule:   genome.MatchEither,
			DrawPenalty: 1,
		},
	}
}

// TestNewSessionBuildsBorrowHooks pins the parity contract: the session must
// construct the SAME borrowed-mechanic hooks the fitness pipeline evaluates
// with. A hook-less session means humans playtest a different game than the
// one fitness scored (the audit's broken-validation-loop finding).
func TestNewSessionBuildsBorrowHooks(t *testing.T) {
	g := borrowGenome()
	if errs := genome.Validate(g); len(errs) > 0 {
		t.Fatalf("fixture genome invalid: %v", errs)
	}

	s := NewSession(g, &shedding.Runner{}, &sim.RandomAI{}, 7)
	if len(s.Hooks) == 0 {
		t.Fatal("NewSession built no hooks for a borrow-carrying genome")
	}

	plain := borrowGenome()
	plain.Borrowed = nil
	s2 := NewSession(plain, &shedding.Runner{}, &sim.RandomAI{}, 7)
	if len(s2.Hooks) != 0 {
		t.Fatalf("NewSession built %d hooks for a borrow-free genome, want 0", len(s2.Hooks))
	}
}

// TestScriptedSessionFiresBorrowHooks runs a fully scripted interactive
// session against the borrow-carrying genome and asserts the hooks actually
// fired during play, observed via the Scores mutation only the avoidance
// hook can produce. The session loop -- not RunBatch -- is under test here.
func TestScriptedSessionFiresBorrowHooks(t *testing.T) {
	g := borrowGenome()
	if errs := genome.Validate(g); len(errs) > 0 {
		t.Fatalf("fixture genome invalid: %v", errs)
	}

	// Silence the interactive transcript (same pattern as the MCTS test).
	oldStdout := os.Stdout
	devnull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open %s: %v", os.DevNull, err)
	}
	defer func() {
		os.Stdout = oldStdout
		devnull.Close()
	}()
	os.Stdout = devnull

	for seed := uint64(1); seed <= 5; seed++ {
		s := NewSession(g, &shedding.Runner{}, &sim.RandomAI{}, seed)
		s.Scanner = bufio.NewScanner(strings.NewReader(strings.Repeat("1\n", 4096)))
		outcome := s.Run()
		if outcome.Winner < 0 {
			continue // rare non-completion under this seed; try the next
		}
		os.Stdout = oldStdout

		// A finished shedding game leaves the loser holding >= 1 card, and
		// every card is a penalty here, so the round-end avoidance hook must
		// have banked a negative score.
		mutated := false
		for _, sc := range s.State.Scores {
			if sc != 0 {
				mutated = true
			}
		}
		if !mutated {
			t.Fatalf("seed %d: game completed (winner %d, %d turns) but Scores never mutated: borrow hooks did not fire in the session loop",
				seed, outcome.Winner, outcome.Turns)
		}
		if outcome.Turns <= 0 {
			t.Fatalf("seed %d: outcome.Turns = %d, want > 0", seed, outcome.Turns)
		}
		if outcome.Stuck {
			t.Fatalf("seed %d: completed game flagged stuck", seed)
		}
		if got := outcome.WinnerLabel(s.HumanID); got != "human" && got != "ai" {
			t.Fatalf("seed %d: WinnerLabel = %q for a completed game, want human or ai", seed, got)
		}
		return
	}
	os.Stdout = oldStdout
	t.Fatal("no scripted session completed across seeds 1-5; fixture or runner broken")
}

func TestDescribeMoveShortCapture(t *testing.T) {
	played := sim.Card{Rank: 10, Suit: sim.Hearts}
	cap1 := sim.Card{Rank: 7, Suit: sim.Clubs}
	cap2 := sim.Card{Rank: 3, Suit: sim.Spades}

	// Casino capture must name the played card AND the cards it takes (not "Unknown").
	got := describeMoveShort(sim.Move{Type: sim.MoveCapture, Cards: []sim.Card{played, cap1, cap2}})
	if got == "Unknown" || !strings.Contains(got, "capture") {
		t.Errorf("MoveCapture rendered %q, want a capture description", got)
	}
	for _, want := range []string{played.String(), cap1.String(), cap2.String()} {
		if !strings.Contains(got, want) {
			t.Errorf("capture label %q missing card %q", got, want)
		}
	}

	// A capture carrying only the played card still must not render "Unknown".
	if got := describeMoveShort(sim.Move{Type: sim.MoveCapture, Cards: []sim.Card{played}}); got == "Unknown" {
		t.Errorf("single-card MoveCapture rendered %q, want a capture description", got)
	}
}

func TestOutcomeWinnerLabel(t *testing.T) {
	cases := []struct {
		o       Outcome
		humanID int
		want    string
	}{
		{Outcome{Winner: 0, Turns: 10}, 0, "human"},
		{Outcome{Winner: 2, Turns: 10}, 0, "ai"},
		{Outcome{Winner: -1, Turns: 19, Stuck: true}, 0, "stuck"},
		{Outcome{Winner: -1, Turns: 200}, 0, "none"},
	}
	for _, tc := range cases {
		if got := tc.o.WinnerLabel(tc.humanID); got != tc.want {
			t.Errorf("WinnerLabel(%+v, humanID=%d) = %q, want %q", tc.o, tc.humanID, got, tc.want)
		}
	}
}

func scannerFrom(s string) *bufio.Scanner {
	return bufio.NewScanner(strings.NewReader(s))
}

func TestPromptRatingParsesRatingAndComment(t *testing.T) {
	rating, comment := PromptRating(scannerFrom("4\nfun game\n"), io.Discard)
	if rating == nil || *rating != 4 {
		t.Fatalf("rating = %v, want 4", rating)
	}
	if comment != "fun game" {
		t.Fatalf("comment = %q, want %q", comment, "fun game")
	}
}

// TestPromptRatingEmptyInputSkips: empty = null rating, per the plan.
func TestPromptRatingEmptyInputSkips(t *testing.T) {
	rating, comment := PromptRating(scannerFrom("\n\n"), io.Discard)
	if rating != nil {
		t.Fatalf("rating = %d, want nil (skipped)", *rating)
	}
	if comment != "" {
		t.Fatalf("comment = %q, want empty", comment)
	}
}

// TestPromptRatingEOFIsNonInteractiveSafe: a piped session whose input is
// exhausted must skip the prompt silently, never block or error.
func TestPromptRatingEOFIsNonInteractiveSafe(t *testing.T) {
	rating, comment := PromptRating(scannerFrom(""), io.Discard)
	if rating != nil || comment != "" {
		t.Fatalf("EOF should skip: rating=%v comment=%q", rating, comment)
	}
}

func TestPromptRatingRejectsInvalidThenAccepts(t *testing.T) {
	rating, comment := PromptRating(scannerFrom("9\nabc\n3\nok\n"), io.Discard)
	if rating == nil || *rating != 3 {
		t.Fatalf("rating = %v, want 3 after rejecting 9 and abc", rating)
	}
	if comment != "ok" {
		t.Fatalf("comment = %q, want %q", comment, "ok")
	}
}

// TestAppendRecordWritesV2SchemaJSONL checks the JSONL record against the
// plan's v2 schema, with field names matching the v1 writer where the two
// schemas overlap (timestamp, genome_id, genome_path, difficulty, seed,
// winner, turns, rating, comment) plus the v2 "stuck" boolean flag. A skipped
// rating must serialize as JSON null (v1 precedent), and appends must never
// truncate existing records.
func TestAppendRecordWritesV2SchemaJSONL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "playtest_results.jsonl")

	four := 4
	rec1 := Record{
		Timestamp:  "2026-06-11T12:00:00Z",
		GenomeID:   "g1",
		GenomePath: "output/run/g1.json",
		Difficulty: "greedy",
		Seed:       42,
		Winner:     "human",
		Turns:      31,
		Rating:     &four,
		Comment:    "tight endgame",
	}
	rec2 := Record{
		Timestamp:  "2026-06-11T12:05:00Z",
		GenomeID:   "g2",
		GenomePath: "output/run/g2.json",
		Difficulty: "mcts",
		Seed:       43,
		Winner:     "stuck",
		Turns:      19,
		Rating:     nil,
		Stuck:      true,
	}
	if err := AppendRecord(path, rec1); err != nil {
		t.Fatalf("AppendRecord(rec1): %v", err)
	}
	if err := AppendRecord(path, rec2); err != nil {
		t.Fatalf("AppendRecord(rec2): %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2 (append must not truncate)", len(lines))
	}

	var m1 map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &m1); err != nil {
		t.Fatalf("line 1 is not valid JSON: %v", err)
	}
	for _, key := range []string{"timestamp", "genome_id", "genome_path", "difficulty",
		"seed", "winner", "turns", "rating", "comment", "stuck"} {
		if _, ok := m1[key]; !ok {
			t.Errorf("record missing field %q (v1-name-parity + v2 schema)", key)
		}
	}
	if got, ok := m1["rating"].(float64); !ok || got != 4 {
		t.Errorf("rating = %v, want 4", m1["rating"])
	}

	var m2 map[string]any
	if err := json.Unmarshal([]byte(lines[1]), &m2); err != nil {
		t.Fatalf("line 2 is not valid JSON: %v", err)
	}
	if v, ok := m2["rating"]; !ok || v != nil {
		t.Errorf("skipped rating must serialize as JSON null, got %v (present=%v)", v, ok)
	}
	if m2["stuck"] != true {
		t.Errorf("stuck flag = %v, want true", m2["stuck"])
	}
	if m2["winner"] != "stuck" {
		t.Errorf("winner = %v, want %q (v1 vocabulary)", m2["winner"], "stuck")
	}
}

// TestHooksForIsSingleConstructionSite is the plan's grep-test: hook
// construction from a genome must have exactly one site (mechanic.HooksFor)
// so the fitness pipeline and the playtest session cannot drift. Non-test
// production code outside pkg/mechanic must never call BuildHooks directly,
// and both known consumers must go through mechanic.HooksFor.
func TestHooksForIsSingleConstructionSite(t *testing.T) {
	repoRoot := filepath.Join("..", "..")

	var offenders []string
	for _, dir := range []string{"pkg", "cmd"} {
		err := filepath.WalkDir(filepath.Join(repoRoot, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if strings.Contains(string(data), "BuildHooks(") &&
				!strings.Contains(filepath.ToSlash(path), "pkg/mechanic/") {
				offenders = append(offenders, path)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", dir, err)
		}
	}
	if len(offenders) > 0 {
		t.Errorf("BuildHooks called outside pkg/mechanic in non-test code (use mechanic.HooksFor): %v", offenders)
	}

	for _, f := range []string{
		filepath.Join("pkg", "fitness", "evaluate.go"),
		filepath.Join("pkg", "playtest", "session.go"),
	} {
		data, err := os.ReadFile(filepath.Join(repoRoot, f))
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		if !strings.Contains(string(data), "mechanic.HooksFor(") {
			t.Errorf("%s does not use mechanic.HooksFor: hook construction has drifted from the single shared site", f)
		}
	}
}

// TestDescribeMoveShortCoversVying: SimplePoker is a seeds.All() member, so
// the CLI playtest renders vying games; the betting move types once fell to
// the "Unknown" default, making every poker menu unreadable.
func TestDescribeMoveShortCoversVying(t *testing.T) {
	cases := map[sim.MoveType]string{
		sim.MoveCheck: "Check",
		sim.MoveCall:  "Call",
		sim.MoveRaise: "Raise",
		sim.MoveFold:  "Fold",
	}
	for mt, want := range cases {
		if got := describeMoveShort(sim.Move{Type: mt}); got != want {
			t.Errorf("describeMoveShort(%v) = %q, want %q", mt, got, want)
		}
	}
	if got := describeMoveShort(sim.Move{Type: sim.MoveBid, Amount: 3}); got != "Bid 3" {
		t.Errorf("describeMoveShort(bid 3) = %q, want \"Bid 3\"", got)
	}
}

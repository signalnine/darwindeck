package judge

import (
	"math/rand/v2"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/mechanic"
	"github.com/darwindeck/darwindeck/pkg/seeds"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

// TestPlayTracedMatchesRunBatch pins the judge's instrumented game loop to the
// production engine: at the same seeds, with the same hooks, it must reach the
// same outcome game for game as sim.RunBatch. The dossier traces come from
// this loop (it records the MOVES, which sim.GameResult does not expose), so a
// drift here would put a different game in front of the judge than the one
// fitness evaluated -- the very defect the borrow-hook fix closed.
func TestPlayTracedMatchesRunBatch(t *testing.T) {
	// The knock fixtures cover every way a knock ends play: the game on a
	// single-round shedding host, the ROUND on a multi-round one (banked
	// scores, redeal), the climbing race, and rummy's discard-then-knock.
	knockers := []*genome.Genome{knockBorrowGenome(t), multiRoundKnockShedding(t), knockClimbing(t), seeds.KnockRummy()}
	genomes := append(seeds.All(), runPlayMeldShedding(t), drawPenaltyRummy(t), scoredVyingGenome(t))
	genomes = append(genomes, knockers...)
	const n = 40
	const base = uint64(977)
	knocks := map[string]int{}
	for _, g := range genomes {
		runner, ai := mustRunner(t, g), mustAI(g)
		hooks := mechanic.HooksFor(g)
		want := sim.RunBatch(g, runner, ai, n, base, hooks...)
		for i := 0; i < n; i++ {
			rng := rand.New(rand.NewPCG(base+uint64(i), 0))
			got := playTraced(g, runner, ai, rng, g.MaxTurns(), hooks...)
			if got.winner != want.AllWinners[i] {
				t.Errorf("%s game %d: traced loop winner %d, RunBatch winner %d", g.ID, i, got.winner, want.AllWinners[i])
			}
			// A knock shows up in the trace as a declare-out line (the move
			// line DECLARE_OUT, or the runner's neutralized knock event).
			for _, line := range got.lines {
				if strings.Contains(strings.ToLower(line), "declare") {
					knocks[g.ID]++
					break
				}
			}
		}
	}
	// Parity on a knock host only means something if knocks were played.
	for _, g := range knockers {
		if knocks[g.ID] == 0 {
			t.Errorf("%s: no knock was played in %d traced games, so the knock path is not covered by the parity check", g.ID, n)
		}
	}
}

// multiRoundKnockShedding: a knock on a multi-round shedding host ends the
// ROUND (scores bank, hands redeal), not the game.
func multiRoundKnockShedding(t *testing.T) *genome.Genome {
	t.Helper()
	g := seeds.CrazyEights()
	g.ID = "knock-multi-round"
	g.Shedding.RoundsPerGame = 3
	g.Borrowed = []genome.BorrowedMechanic{
		{Source: genome.Rummy, Mechanic: genome.MechKnock},
		{Source: genome.Rummy, Mechanic: genome.MechMeldBonus},
	}
	if errs := genome.Validate(g); len(errs) > 0 {
		t.Fatalf("multi-round knock genome fails Tier-0 validation: %v", errs)
	}
	if !g.SheddingMultiRound() {
		t.Fatal("fixture drift: the multi-round knock genome is not multi-round")
	}
	return g
}

// knockClimbing: Big Two with the knock borrow (the climbing knock path).
func knockClimbing(t *testing.T) *genome.Genome {
	t.Helper()
	g := seeds.BigTwo()
	g.ID = "knock-climbing"
	g.Borrowed = []genome.BorrowedMechanic{{Source: genome.Rummy, Mechanic: genome.MechKnock}}
	if errs := genome.Validate(g); len(errs) > 0 {
		t.Fatalf("climbing knock genome fails Tier-0 validation: %v", errs)
	}
	return g
}

// TestTraceOmitsForcedTurnKeepingPass pins which event-less moves get a
// synthesized trace line. Every DECISION and every action that hands the turn
// on must be visible; the only thing left out is a forced phase skip that
// keeps the turn (rummy's meld phase with nothing to meld), which is neither a
// decision nor turn-taking information and would add a no-op line to every
// rummy turn.
func TestTraceOmitsForcedTurnKeepingPass(t *testing.T) {
	played := []sim.Event{{Type: sim.EventCardPlayed, PlayerID: 0}}
	roundEnd := []sim.Event{{Type: sim.EventRoundEnd, PlayerID: 0, Detail: "showdown"}}
	cases := []struct {
		name     string
		numMoves int
		turnKept bool
		events   []sim.Event
		want     bool
	}{
		{"forced pass that keeps the turn (rummy empty meld phase)", 1, true, nil, false},
		{"chosen pass that keeps the turn (declined a meld)", 3, true, nil, true},
		{"forced pass that hands the turn on (climbing cannot beat)", 1, false, nil, true},
		{"chosen bet with no event (vying call)", 3, false, nil, true},
		{"bet whose only event is the round end (scored vying)", 2, false, roundEnd, true},
		{"play already rendered by its own event", 4, false, played, false},
	}
	for _, c := range cases {
		if got := needsMoveLine(c.numMoves, c.turnKept, c.events); got != c.want {
			t.Errorf("%s: needsMoveLine = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestTraceNoteUsesRealCompletions: the "only one game reached a winner" note
// was keyed on how many DISTINCT traces were shown, so a game whose traces
// de-duplicated to one (every vying trace was the same empty block) was
// reported as hitting the turn cap while 100% of its games completed.
func TestTraceNoteUsesRealCompletions(t *testing.T) {
	if note := traceNote(2, 400, 400); note != "" {
		t.Errorf("two games shown should need no note, got %q", note)
	}
	if note := traceNote(0, 0, 400); !strings.Contains(note, "No games completed") {
		t.Errorf("zero completions note = %q", note)
	}
	if note := traceNote(1, 1, 400); !strings.Contains(note, "Only one of the sampled games reached a winner") {
		t.Errorf("single completion note = %q", note)
	}
	note := traceNote(1, 400, 400)
	if strings.Contains(note, "turn cap") || strings.Contains(note, "Only one") {
		t.Errorf("400 completions with one distinct trace must not claim a turn-cap problem, got %q", note)
	}
	if !strings.Contains(note, "400 of the 400") {
		t.Errorf("identical-traces note should state the real completion count, got %q", note)
	}
}

// traceSection returns the Sample Game Traces part of a dossier (between the
// traces header and the Termination header).
func traceSection(doc string) string {
	start := strings.Index(doc, "## Sample Game Traces")
	end := strings.Index(doc, "## Termination")
	if start < 0 || end < 0 || end < start {
		return ""
	}
	return doc[start:end]
}

// TestVyingTraceShowsBettingDecisions: the unscored vying runner emits no
// events at all (betting moves carry no cards and the showdown is resolved in
// Upkeep), so an event-only trace rendered an EMPTY code block for SimplePoker
// -- and, every empty trace being identical, de-duplication then showed one
// game under a note claiming the rest "hit the turn cap" beside a Termination
// section reporting 100% completion. Every betting decision must be visible.
func TestVyingTraceShowsBettingDecisions(t *testing.T) {
	g := seeds.SimplePoker()
	g.ID = "G01"
	doc, err := BuildDossier(g)
	if err != nil {
		t.Fatalf("BuildDossier: %v", err)
	}
	sec := traceSection(doc)

	if n := strings.Count(sec, "### Game "); n != 2 {
		t.Errorf("vying dossier shows %d sample games, want 2:\n%s", n, sec)
	}
	if strings.Contains(sec, "hit the turn cap") {
		t.Errorf("vying dossier claims games hit the turn cap although every game completes:\n%s", sec)
	}
	for _, action := range []string{" CALL", " RAISE", " FOLD"} {
		if !strings.Contains(sec, action) {
			t.Errorf("vying trace never shows a%s decision:\n%s", action, sec)
		}
	}

	// Deal boundaries carry the chip standings, and chips are conserved in an
	// unscored vying game: every standings line sums to players * stack.
	want := g.Players * g.Vying.StartingChips
	deals := regexp.MustCompile(`DEAL_END chips ((?:P\d+=-?\d+ ?)+)`).FindAllStringSubmatch(sec, -1)
	if len(deals) == 0 {
		t.Fatalf("vying trace shows no deal boundary:\n%s", sec)
	}
	for _, d := range deals {
		sum := 0
		for _, m := range regexp.MustCompile(`P\d+=(-?\d+)`).FindAllStringSubmatch(d[1], -1) {
			n, _ := strconv.Atoi(m[1])
			sum += n
		}
		if sum != want {
			t.Errorf("standings %q sum to %d chips, want %d (chips are conserved)", d[1], sum, want)
		}
	}
}

// TestClimbingTraceShowsPasses: a climbing pass emits no event, so the trace
// hid every pass and showed the player who kept the lead acting several times
// in a row -- exactly the "long uninterrupted single-player run" the rubric
// tells the judge to read as degenerate. It also must not carry the "climb"
// detail token: the rulebook neutralizer strips the skeleton keyword, and the
// trace put it back on every play line.
func TestClimbingTraceShowsPasses(t *testing.T) {
	g := seeds.BigTwo()
	g.ID = "G01"
	doc, err := BuildDossier(g)
	if err != nil {
		t.Fatalf("BuildDossier: %v", err)
	}
	sec := traceSection(doc)
	if !strings.Contains(sec, " PASS") {
		t.Errorf("climbing trace shows no passes:\n%s", sec)
	}
	if strings.Contains(strings.ToLower(sec), "climb") {
		t.Errorf("climbing trace leaks the skeleton keyword:\n%s", sec)
	}
}

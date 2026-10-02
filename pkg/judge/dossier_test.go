package judge

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/mechanic"
	"github.com/darwindeck/darwindeck/pkg/seeds"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

// dossierWinners extracts the "**Winner:** Player N" lines of a dossier's
// sample traces, in order.
func dossierWinners(doc string) []int {
	var out []int
	for _, m := range regexp.MustCompile(`\*\*Winner:\*\* Player (\d+)`).FindAllStringSubmatch(doc, -1) {
		n, _ := strconv.Atoi(m[1])
		out = append(out, n)
	}
	return out
}

// TestBuildDossierTracesApplyBorrowHooks: the dossier must show the SAME game
// fitness evaluated. BuildDossier ran its trace batch without
// mechanic.HooksFor(g), so the hook-scored borrows (meld_bonus, avoidance,
// trick_scoring, draw_penalty) were described in the rulebook but absent from
// the simulation -- on the published run_play+meld_bonus hybrids the dossier's
// winner differed from the real game's in ~330 of 400 sampled games, and the
// judges certified novelty off traces of a game without its scoring borrow.
func TestBuildDossierTracesApplyBorrowHooks(t *testing.T) {
	g := runPlayMeldShedding(t)
	runner, ai := mustRunner(t, g), mustAI(g)

	hooked := sim.RunBatch(g, runner, ai, 400, dossierSeed+1, mechanic.HooksFor(g)...)
	unhooked := sim.RunBatch(g, runner, ai, 400, dossierSeed+1)
	var want, wrong []int
	for _, i := range pickDistinctCompleted(hooked, 2) {
		want = append(want, hooked.AllWinners[i])
	}
	for _, i := range pickDistinctCompleted(unhooked, 2) {
		wrong = append(wrong, unhooked.AllWinners[i])
	}
	if len(want) != 2 || reflect.DeepEqual(want, wrong) {
		t.Fatalf("fixture does not discriminate: hooked winners %v, unhooked %v", want, wrong)
	}

	doc, err := BuildDossier(g)
	if err != nil {
		t.Fatalf("BuildDossier: %v", err)
	}
	if got := dossierWinners(doc); !reflect.DeepEqual(got, want) {
		t.Errorf("dossier trace winners = %v, want %v (the hooked game); the unhooked game gives %v", got, want, wrong)
	}
}

// TestBuildDossierTerminationAppliesBorrowHooks: same defect on the
// Termination section -- observeGame never applied the borrow hooks, so a
// rummy+draw_penalty game reported the completion rate of plain rummy.
func TestBuildDossierTerminationAppliesBorrowHooks(t *testing.T) {
	g := drawPenaltyRummy(t)
	runner, ai := mustRunner(t, g), mustAI(g)

	const termN = 150
	pct := func(r sim.BatchResult) string {
		return fmt.Sprintf("**%.0f%%** of %d sampled games", 100*float64(r.Completions)/float64(termN), termN)
	}
	want := pct(sim.RunBatch(g, runner, ai, termN, dossierSeed+1<<20, mechanic.HooksFor(g)...))
	wrong := pct(sim.RunBatch(g, runner, ai, termN, dossierSeed+1<<20))
	if want == wrong {
		t.Fatalf("fixture does not discriminate: hooked and unhooked both give %s", want)
	}

	doc, err := BuildDossier(g)
	if err != nil {
		t.Fatalf("BuildDossier: %v", err)
	}
	sec := termSection(doc)
	if !strings.Contains(sec, want) {
		t.Errorf("termination section does not report the hooked completion %q (unhooked would be %q):\n%s", want, wrong, sec)
	}
}

// TestBuildDossierHasTerminationSection verifies the dossier builder emits a
// Termination section carrying the reachable-win signal -- the new feature.
func TestBuildDossierHasTerminationSection(t *testing.T) {
	g := seeds.CrazyEights()
	g.ID = "G01"

	doc, err := BuildDossier(g)
	if err != nil {
		t.Fatalf("BuildDossier: %v", err)
	}

	if !strings.Contains(doc, "## Termination") {
		t.Fatalf("dossier missing Termination section:\n%s", doc)
	}
	if !strings.Contains(doc, "Completion at the standard turn cap") {
		t.Error("Termination section missing standard-cap completion line")
	}
	if !strings.Contains(doc, "4x extended turn cap") {
		t.Error("Termination section missing extended-cap completion line")
	}
	if !strings.Contains(doc, "Win-condition reachable") {
		t.Error("Termination section missing reachable-win signal header")
	}
	if !strings.Contains(doc, "Has bidding/contract scoring") {
		t.Error("Termination section missing bidding/contract line")
	}
	// Title is the neutral id, not the true name.
	if !strings.Contains(doc, "# G01") {
		t.Error("rulebook title is not the neutral id")
	}
}

// TestSheddingReachableWinSignal verifies the shedding-specific signal (median
// turns to first empty hand) appears for a shedding game that completes.
func TestSheddingReachableWinSignal(t *testing.T) {
	g := seeds.CrazyEights()
	g.ID = "G02"
	doc, err := BuildDossier(g)
	if err != nil {
		t.Fatalf("BuildDossier: %v", err)
	}
	if !strings.Contains(doc, "emptied their hand") {
		t.Errorf("shedding dossier missing empty-hand reachable signal:\n%s", termSection(doc))
	}
}

// TestRummyReachableWinSignalClearsFalsePositive is THE FIX in action: Gin
// Rummy completes rarely under greedy self-play (the prototype's false
// positive), but the Termination section must prove the win condition is
// reachable: a going-out move becomes legal by the rules.
func TestRummyReachableWinSignalClearsFalsePositive(t *testing.T) {
	for _, mk := range []struct {
		name string
		make func() *genome.Genome
	}{
		{"GinRummy", seeds.GinRummy},
		{"KnockRummy", seeds.KnockRummy},
	} {
		t.Run(mk.name, func(t *testing.T) {
			g := mk.make()
			g.ID = "G0X"
			doc, err := BuildDossier(g)
			if err != nil {
				t.Fatalf("BuildDossier: %v", err)
			}
			sec := termSection(doc)
			if !strings.Contains(sec, "going-out move became LEGAL") {
				t.Errorf("%s: reachable-win signal not present -- the fix failed to clear the false positive:\n%s", mk.name, sec)
			}
		})
	}
}

// TestTerminationReachableComputed checks the underlying termination
// computation directly for a rummy seed.
func TestTerminationReachableComputed(t *testing.T) {
	g := seeds.GinRummy()
	runner := mustRunner(t, g)
	ai := mustAI(g)
	info := computeTermination(g, runner, ai, 150, 1234)
	if !info.ReachableKnock {
		t.Error("expected a knock to become legal across the sampled Gin Rummy games (the false-positive fix)")
	}
	if info.MedianTurnsToKnockLegal <= 0 {
		t.Error("expected a positive median turn-to-knock-legal")
	}
	if info.Skeleton != genome.Rummy {
		t.Errorf("skeleton = %v, want rummy", info.Skeleton)
	}
}

// TestContractScoringSignal verifies the bidding/contract derivation
// distinguishes the trick-taking family (the Spades/Oh-Hell vs Whist collapse).
func TestContractScoringSignal(t *testing.T) {
	// Spades: fixed trump -> contract-like.
	if !hasContractScoring(seeds.Spades()) {
		t.Error("Spades should report contract scoring (trump rule)")
	}
	// Hearts: avoidance scoring -> contract-like.
	if !hasContractScoring(seeds.Hearts()) {
		t.Error("Hearts should report contract scoring (avoidance)")
	}
	// A plain per-trick no-trump trick game -> not contract-like.
	g := seeds.Whist()
	g.TrumpRule = genome.TrumpNone
	g.TrickTaking.TrickScoring = genome.ScorePerTrick
	if hasContractScoring(g) {
		t.Error("plain per-trick no-trump game should NOT report contract scoring")
	}
	// Shedding is never contract scoring.
	if hasContractScoring(seeds.CrazyEights()) {
		t.Error("shedding should not report contract scoring")
	}
}

// unreachableText is the phrase the Termination section uses to tell the judge
// a game may never end -- which the rubric turns into a "degenerate" verdict.
const unreachableText = "may be hard or impossible to reach"

// TestKnockSheddingTerminationIsReachable: a shedding game with the knock
// borrow ends when a player declares out, usually before any hand empties.
// The shedding reachable-win signal only looked for an EMPTIED hand, so the
// dossier said "No sampled game saw a player empty their hand ... The terminal
// state may be hard or impossible to reach" directly under "100% of sampled
// games ended with a winner".
func TestKnockSheddingTerminationIsReachable(t *testing.T) {
	g := knockBorrowGenome(t)
	g.ID = "G01"
	doc, err := BuildDossier(g)
	if err != nil {
		t.Fatalf("BuildDossier: %v", err)
	}
	sec := termSection(doc)
	if !strings.Contains(sec, "**100%** of 150 sampled games ended with a winner") {
		t.Fatalf("fixture drift: the knock-borrow game no longer completes 100%%:\n%s", sec)
	}
	if strings.Contains(sec, unreachableText) {
		t.Errorf("termination section calls a 100%%-completing game possibly unreachable:\n%s", sec)
	}
	if !strings.Contains(sec, "IS reachable") {
		t.Errorf("termination section does not state the win condition is reachable:\n%s", sec)
	}
	if !strings.Contains(sec, "declaring out") {
		t.Errorf("termination section does not say HOW the games ended (declaring out):\n%s", sec)
	}
}

// TestRenderTerminationNeverContradictsCompletion: for EVERY skeleton, a game
// that was observed to end with a winner has a reachable terminal state, no
// matter what the skeleton-specific probe saw; and a game that never ended
// still gets the warning.
func TestRenderTerminationNeverContradictsCompletion(t *testing.T) {
	for _, sk := range genome.AllSkeletons() {
		ended := renderTermination(TerminationInfo{
			Skeleton: sk, GamesSampled: 150, CapStd: 100, CapExt: 400,
			CompletionStdPct: 100, CompletionExtPct: 100, AnyCompleted: true,
		})
		if strings.Contains(ended, unreachableText) {
			t.Errorf("%s: completed games rendered as possibly unreachable:\n%s", sk, ended)
		}
		if !strings.Contains(ended, "IS reachable") {
			t.Errorf("%s: completed games not rendered as reachable:\n%s", sk, ended)
		}

		never := renderTermination(TerminationInfo{
			Skeleton: sk, GamesSampled: 150, CapStd: 100, CapExt: 400,
		})
		if !strings.Contains(never, unreachableText) {
			t.Errorf("%s: a game that never ended carries no unreachable warning:\n%s", sk, never)
		}
		if strings.Contains(never, "IS reachable") {
			t.Errorf("%s: a game that never ended rendered as reachable:\n%s", sk, never)
		}
	}
}

func termSection(doc string) string {
	idx := strings.Index(doc, "## Termination")
	if idx < 0 {
		return ""
	}
	return doc[idx:]
}

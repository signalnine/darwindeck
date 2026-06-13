package judge

import (
	"strings"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
)

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

func termSection(doc string) string {
	idx := strings.Index(doc, "## Termination")
	if idx < 0 {
		return ""
	}
	return doc[idx:]
}

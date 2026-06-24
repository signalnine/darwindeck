package grammar

import (
	"testing"

	"github.com/darwindeck/darwindeck/pkg/sim"
)

// TestAdapterRunsInRealEngine: a grammar spec plugs into the REAL simulation
// engine (sim.RunBatch via sim.GenericRunner), not just the prototype harness.
// Every game must complete with a real winner -- no errors, no timeouts, no
// stuck/no_moves exits -- and emit the event taxonomy the fitness metrics read.
func TestAdapterRunsInRealEngine(t *testing.T) {
	specs := append(Canonical(), EnumerateModified()...)
	for i, s := range specs {
		res := sim.RunBatch(SpecGenome(s), Adapter{s}, &sim.RandomAI{}, 30, uint64(i)*1000+1)
		if res.Completions != res.GamesPlayed {
			t.Errorf("%s: only %d/%d games completed (errors=%d timeouts=%d)",
				s.Family(), res.Completions, res.GamesPlayed, res.Errors, res.Timeouts)
		}
		if res.Errors != 0 || res.Timeouts != 0 {
			t.Errorf("%s: errors=%d timeouts=%d (must be 0 -- playable-by-construction)",
				s.Family(), res.Errors, res.Timeouts)
		}
		// the winner must be a real seat in range
		seated := 0
		for _, w := range res.WinCounts {
			seated += w
		}
		if seated != res.GamesPlayed {
			t.Errorf("%s: win counts sum to %d, want %d", s.Family(), seated, res.GamesPlayed)
		}
		// events emitted (Meaningful Decisions / Interaction metrics consume these)
		anyEvents := false
		for _, evs := range res.AllEvents {
			if len(evs) > 0 {
				anyEvents = true
				break
			}
		}
		if !anyEvents {
			t.Errorf("%s: no events emitted", s.Family())
		}
	}
}

// TestSpecGenomeSkeleton pins the best-fit skeleton mapping (drives the fitness
// layer's Interaction delta mode).
func TestSpecGenomeSkeleton(t *testing.T) {
	for _, s := range Canonical() {
		g := SpecGenome(s)
		if g.Players != s.Players {
			t.Errorf("%s: SpecGenome players=%d, want %d", s.Family(), g.Players, s.Players)
		}
		if g.HandSize < 1 {
			t.Errorf("%s: SpecGenome HandSize=%d zeroes the MaxTurns cap", s.Family(), g.HandSize)
		}
	}
}

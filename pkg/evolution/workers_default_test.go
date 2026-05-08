package evolution

import (
	"testing"
)

// TestNewNoveltyEngineDefaultsWorkers ensures NewNoveltyEngine fills in a
// non-zero worker count when the caller passes Workers=0. A zero-cap semaphore
// in evaluatePopulation deadlocks on the first acquire, so the constructor
// must apply the same auto-detect defaulting as NewEngine.
func TestNewNoveltyEngineDefaultsWorkers(t *testing.T) {
	cfg := Config{Workers: 0, BaseSeed: 1}
	e := NewNoveltyEngine(cfg, allSeeds())
	if e.Config.Workers <= 0 {
		t.Fatalf("NewNoveltyEngine left Workers=%d; expected auto-detected positive value", e.Config.Workers)
	}
}

// TestNewMAPElitesEngineDefaultsWorkers mirrors the above for MAP-Elites.
func TestNewMAPElitesEngineDefaultsWorkers(t *testing.T) {
	cfg := Config{Workers: 0, BaseSeed: 1}
	e := NewMAPElitesEngine(cfg, allSeeds())
	if e.Config.Workers <= 0 {
		t.Fatalf("NewMAPElitesEngine left Workers=%d; expected auto-detected positive value", e.Config.Workers)
	}
}

// TestNewNoveltyEngineRespectsExplicitWorkers ensures the defaulting only
// fires for the zero value, leaving explicit caller-provided counts intact.
func TestNewNoveltyEngineRespectsExplicitWorkers(t *testing.T) {
	cfg := Config{Workers: 3, BaseSeed: 1}
	e := NewNoveltyEngine(cfg, allSeeds())
	if e.Config.Workers != 3 {
		t.Fatalf("NewNoveltyEngine overrode explicit Workers=3 to %d", e.Config.Workers)
	}
}

// TestNewMAPElitesEngineRespectsExplicitWorkers mirrors the above for MAP-Elites.
func TestNewMAPElitesEngineRespectsExplicitWorkers(t *testing.T) {
	cfg := Config{Workers: 3, BaseSeed: 1}
	e := NewMAPElitesEngine(cfg, allSeeds())
	if e.Config.Workers != 3 {
		t.Fatalf("NewMAPElitesEngine overrode explicit Workers=3 to %d", e.Config.Workers)
	}
}

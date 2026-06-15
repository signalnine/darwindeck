package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/evolution"
	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
)

// writeGenomeJSON marshals g to <dir>/<name>/genome.json (the layout an evolve
// run's games/ directory uses), creating the subdirectory.
func writeGenomeJSON(t *testing.T, dir, name string, g *genome.Genome) {
	t.Helper()
	gameDir := filepath.Join(dir, name)
	if err := os.MkdirAll(gameDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", gameDir, err)
	}
	data, err := json.Marshal(g)
	if err != nil {
		t.Fatalf("marshal %s: %v", name, err)
	}
	if err := os.WriteFile(filepath.Join(gameDir, "genome.json"), data, 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// TestLoadSeedDirReadsValidGenomes loads two valid genome.json files nested
// under <dir> and confirms both come back, identified by ID.
func TestLoadSeedDirReadsValidGenomes(t *testing.T) {
	dir := t.TempDir()

	g1 := seeds.CrazyEights()
	g1.ID = "custom-shed"
	g2 := seeds.GinRummy()
	g2.ID = "custom-rummy"
	writeGenomeJSON(t, dir, "games/rank01_x", g1)
	writeGenomeJSON(t, dir, "games/rank02_y", g2)

	loaded, warnings := loadSeedDir(dir)
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 loaded genomes, got %d", len(loaded))
	}
	ids := map[string]bool{}
	for _, g := range loaded {
		ids[g.ID] = true
	}
	if !ids["custom-shed"] || !ids["custom-rummy"] {
		t.Fatalf("expected both custom IDs loaded, got %v", ids)
	}
}

// TestLoadSeedDirSkipsInvalid skips genomes that fail Tier-0 validation and
// files that are not valid JSON, warning on each, while still returning the
// valid ones.
func TestLoadSeedDirSkipsInvalid(t *testing.T) {
	dir := t.TempDir()

	good := seeds.MauMau()
	good.ID = "good-one"
	writeGenomeJSON(t, dir, "games/rank01_good", good)

	// Tier-0 invalid: hand_size * players exceeds the 52-card deck.
	bad := seeds.MauMau()
	bad.ID = "bad-one"
	bad.HandSize = 13
	bad.Players = 6
	writeGenomeJSON(t, dir, "games/rank02_bad", bad)

	// Not valid JSON at all.
	junkDir := filepath.Join(dir, "games", "rank03_junk")
	if err := os.MkdirAll(junkDir, 0o755); err != nil {
		t.Fatalf("mkdir junk: %v", err)
	}
	if err := os.WriteFile(filepath.Join(junkDir, "genome.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatalf("write junk: %v", err)
	}

	loaded, warnings := loadSeedDir(dir)
	if len(loaded) != 1 {
		t.Fatalf("expected 1 valid genome, got %d (%v)", len(loaded), loaded)
	}
	if loaded[0].ID != "good-one" {
		t.Fatalf("expected good-one, got %q", loaded[0].ID)
	}
	if len(warnings) != 2 {
		t.Fatalf("expected 2 warnings (invalid + junk), got %d: %v", len(warnings), warnings)
	}
}

// TestLoadSeedDirEmpty returns no genomes and no warnings when the directory
// contains no genome.json files.
func TestLoadSeedDirEmpty(t *testing.T) {
	dir := t.TempDir()
	loaded, warnings := loadSeedDir(dir)
	if len(loaded) != 0 {
		t.Fatalf("expected 0 loaded, got %d", len(loaded))
	}
	if len(warnings) != 0 {
		t.Fatalf("expected 0 warnings, got %v", warnings)
	}
}

// TestSeedPoolAugmentsClassics is the core invariant: -seed-dir AUGMENTS the
// built-in classic pool rather than replacing it. The combined pool must
// contain every classic family seed PLUS the custom genomes, so cross-family
// crossover always has the classic partners available.
func TestSeedPoolAugmentsClassics(t *testing.T) {
	dir := t.TempDir()
	custom := seeds.CrazyEights()
	custom.ID = "custom-survivor"
	writeGenomeJSON(t, dir, "games/rank01_x", custom)

	classics := getAllSeeds()
	combined := seedPool(dir)

	if len(combined) != len(classics)+1 {
		t.Fatalf("expected %d combined seeds (classics+1 custom), got %d",
			len(classics)+1, len(combined))
	}

	ids := map[string]bool{}
	for _, g := range combined {
		ids[g.ID] = true
	}
	// Every classic must survive (augment, not replace).
	for _, c := range classics {
		if !ids[c.ID] {
			t.Fatalf("classic seed %q missing from augmented pool", c.ID)
		}
	}
	// And the custom one is present.
	if !ids["custom-survivor"] {
		t.Fatal("custom survivor missing from augmented pool")
	}
}

// TestSeedPoolNoDirReturnsClassics confirms that with no -seed-dir the pool is
// exactly the built-in classics (additive: zero behavior change off-flag).
func TestSeedPoolNoDirReturnsClassics(t *testing.T) {
	classics := getAllSeeds()
	pool := seedPool("")
	if len(pool) != len(classics) {
		t.Fatalf("expected %d classics with empty dir, got %d", len(classics), len(pool))
	}
}

// TestSeedDirRunIncludesCustomInInitialPopulation is the end-to-end TDD claim:
// a -seed-dir run wires the custom genomes into the engine's seed pool, which is
// exactly what Initialize() samples every initial-population member from. With a
// custom genome carrying a fingerprint NO classic seed has (a distinctive
// hand_size), at least one member of a large initial population descends from
// the custom seed -- proving custom genomes materially enter generation 0, not
// just the pool list. Deterministic at the pinned seed.
func TestSeedDirRunIncludesCustomInInitialPopulation(t *testing.T) {
	dir := t.TempDir()

	// A valid custom genome with a HandSize fingerprint that no classic seed
	// can reach within a single init-time mutation. The classics use HandSize
	// {5,7,10,13}; tweakParameter only nudges HandSize by +/-1 (and only ~20%
	// of the time), so the nearest classic (MauMau at 5) can drop to 4 at best,
	// never to 3. HandSize is clamped to [3,13], so a custom member at HandSize
	// 3 stays at 3 or rises to 4. Therefore any gen-0 individual carrying
	// HandSize exactly 3 can ONLY have descended from this custom seed --
	// a deterministic, non-flaky fingerprint of custom descent.
	const fingerprint = 3
	custom := seeds.CrazyEights()
	custom.ID = "custom-fingerprint"
	custom.HandSize = fingerprint
	custom.Players = 4 // 3*4 = 12 <= 52, stays Tier-0 valid
	writeGenomeJSON(t, dir, "games/rank01_fp", custom)

	pool := seedPool(dir)

	// Sanity: no classic of the SAME skeleton is within one mutation step of the
	// fingerprint, so a gen-0 (shedding, HandSize==3) genome cannot have come
	// from a mutated classic. (HandSize alone is no longer a fingerprint -- the
	// 10 classics' hand sizes {4,5,7,10,13} blanket 3-13 under +-1 mutation,
	// e.g. Casino=4 is adjacent to 3 -- but Casino is not shedding, and reaching
	// shedding+HandSize-3 from any classic needs more than one step.)
	for _, c := range getAllSeeds() {
		if c.Skeleton == custom.Skeleton && c.HandSize >= fingerprint-1 && c.HandSize <= fingerprint+1 {
			t.Fatalf("test invariant broken: %s classic %q HandSize %d is within one step of fingerprint %d",
				c.Skeleton, c.ID, c.HandSize, fingerprint)
		}
	}

	config := evolution.Config{
		PopulationSize: 200,
		Generations:    1,
		Workers:        4,
		BaseSeed:       42,
	}
	engine := evolution.NewEngine(config, pool)

	// The pool the engine samples from must contain the custom genome.
	foundInPool := false
	for _, s := range engine.Seeds {
		if s.ID == "custom-fingerprint" {
			foundInPool = true
		}
	}
	if !foundInPool {
		t.Fatal("custom genome not in engine.Seeds (augmentation did not reach the engine)")
	}

	engine.Initialize()
	if len(engine.Population) != config.PopulationSize {
		t.Fatalf("expected population %d, got %d", config.PopulationSize, len(engine.Population))
	}

	// At least one gen-0 individual descends from the custom seed (carries its
	// HandSize-3 fingerprint). With 200 draws from a 10-seed pool the chance of
	// zero custom draws is ~(9/10)^200 ~ 1e-9, and the run is seeded, so this is
	// deterministic in practice.
	custodyHits := 0
	for _, ind := range engine.Population {
		if ind.Genome.Skeleton == custom.Skeleton && ind.Genome.HandSize == fingerprint {
			custodyHits++
		}
	}
	if custodyHits == 0 {
		t.Fatal("no initial-population member descended from the custom seed; " +
			"custom genomes did not enter generation 0")
	}
}

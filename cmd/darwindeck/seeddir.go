package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/darwindeck/darwindeck/pkg/genome"
)

// loadSeedDir walks dir recursively, loading every genome.json it finds. Each
// loaded genome is run through Tier-0 static validation (genome.Validate);
// genomes that fail to parse or fail validation are SKIPPED and a human-readable
// warning is returned for each, so an operator (or the judge-gated restart loop)
// can spot a malformed survivor without aborting the run. The returned slice is
// ordered deterministically by file path so a -seed-dir run is reproducible.
func loadSeedDir(dir string) (loaded []*genome.Genome, warnings []string) {
	var paths []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Base(path) == "genome.json" {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("walking %s: %v", dir, err))
		return loaded, warnings
	}
	sort.Strings(paths)

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("skip %s: read error: %v", path, err))
			continue
		}
		var g genome.Genome
		if err := json.Unmarshal(data, &g); err != nil {
			warnings = append(warnings, fmt.Sprintf("skip %s: parse error: %v", path, err))
			continue
		}
		if errs := genome.Validate(&g); len(errs) > 0 {
			warnings = append(warnings, fmt.Sprintf("skip %s: invalid: %v", path, errs))
			continue
		}
		// Defensive copy so callers cannot alias the loop's stack value.
		gc := g
		loaded = append(loaded, &gc)
	}
	return loaded, warnings
}

// seedPool returns the seed pool the evolution engine samples from for
// population initialization and changeSkeleton mutation. With an empty dir it is
// exactly the built-in classic pool (getAllSeeds), so the flag is purely
// additive: off-flag behavior is unchanged. With a non-empty dir the custom
// genomes loaded from dir AUGMENT (never replace) the classic pool, guaranteeing
// that cross-family crossover always has the classic-family partners available.
// Invalid custom genomes are skipped with a warning printed to stderr.
func seedPool(seedDir string) []*genome.Genome {
	classics := getAllSeeds()
	if seedDir == "" {
		return classics
	}
	custom, warnings := loadSeedDir(seedDir)
	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "seed-dir: %s\n", w)
	}
	// Augment, not replace: classics first so the 4 classic families are
	// always present for cross-family crossover, then the custom survivors.
	return append(classics, custom...)
}

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/darwindeck/darwindeck/pkg/evolution"
	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/judge"
)

// cmdJudgeBackfill keeps the composition-keyed verdict table COMPLETE. The
// in-loop judge term keys on Composition (skeleton + sorted borrow-mechanic
// set), and the reachable composition space is small and finite. Once every
// reachable composition is labeled, the in-loop judge is a zero-cost lookup with
// no API call per generation -- the whole point of "distilling" the judge here.
//
// backfill scans -dir for every composition present, subtracts the ones already
// in -table, and emits one blind dossier per MISSING composition (a single
// representative genome each) into -out. Judge those dossiers with the
// judge-novelty workflow, then append the new composition->score entries to the
// table. The answer-key's true_name is the composition slug (":"->"-", ","->"_").
//
//	darwindeck judge backfill -table verdicts.json -dir output/myrun -out dossiers/
func cmdJudgeBackfill(args []string) {
	fs2 := flag.NewFlagSet("judge backfill", flag.ExitOnError)
	tablePath := fs2.String("table", "", "existing verdict table (composition -> score JSON) (required)")
	dir := fs2.String("dir", "", "directory to scan recursively for genome.json (required)")
	out := fs2.String("out", "", "dossier output directory for the missing compositions (required)")
	fs2.Parse(args)

	if *tablePath == "" || *dir == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "usage: darwindeck judge backfill -table <verdicts.json> -dir <genome-dir> -out <dossier-dir>")
		os.Exit(1)
	}

	tableData, err := os.ReadFile(*tablePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "judge backfill: %v\n", err)
		os.Exit(1)
	}
	var table map[string]float64
	if err := json.Unmarshal(tableData, &table); err != nil {
		fmt.Fprintf(os.Stderr, "judge backfill: invalid table %s: %v\n", *tablePath, err)
		os.Exit(1)
	}

	// Scan for the first representative genome of each unlabeled composition.
	reps := map[string]string{} // composition -> genome.json path
	filepath.WalkDir(*dir, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() || d.Name() != "genome.json" {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		var g genome.Genome
		if json.Unmarshal(data, &g) != nil {
			return nil
		}
		c := evolution.Composition(&g)
		if _, labeled := table[c]; labeled {
			return nil
		}
		if _, seen := reps[c]; !seen {
			reps[c] = p
		}
		return nil
	})

	if len(reps) == 0 {
		fmt.Printf("Table is complete: every composition under %s is already in %s.\n", *dir, *tablePath)
		return
	}

	comps := make([]string, 0, len(reps))
	for c := range reps {
		comps = append(comps, c)
	}
	sort.Strings(comps)

	// Stage representatives as <comp-slug>/genome.json so judge.Emit records the
	// composition as the dossier's true_name in the answer key.
	staging, err := os.MkdirTemp("", "dd-backfill-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "judge backfill: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(staging)
	slug := strings.NewReplacer(":", "-", ",", "_")
	for _, c := range comps {
		d := filepath.Join(staging, slug.Replace(c))
		if err := os.MkdirAll(d, 0o755); err != nil {
			continue
		}
		src, _ := os.ReadFile(reps[c])
		os.WriteFile(filepath.Join(d, "genome.json"), src, 0o644)
	}

	res, err := judge.Emit(staging, *out, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "judge backfill: emit: %v\n", err)
		os.Exit(1)
	}
	keyPath := filepath.Join(filepath.Dir(filepath.Clean(*out)), "answer-key.json")
	if err := judge.WriteAnswerKey(keyPath, res.AnswerKey); err != nil {
		fmt.Fprintf(os.Stderr, "judge backfill: answer key: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("%d unlabeled composition(s); emitted %d dossiers to %s\n", len(reps), len(res.IDs), *out)
	fmt.Printf("Answer key (id -> composition slug, PRIVATE): %s\n", keyPath)
	for _, c := range comps {
		fmt.Printf("  %-14s  <- %s\n", c, reps[c])
	}
	fmt.Printf("\nNext: judge these dossiers (judge-novelty workflow), then append each\n")
	fmt.Printf("composition->score to %s (novel > 0, variant/known < 0).\n", *tablePath)
}

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
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
// table. The answer key maps each dossier id to its composition (the
// `composition` field; true_name is the composition slug, ":"->"-", ","->"_")
// and to the representative genome's path. Dossier ids are assigned in a
// pseudo-random order, so the answer key is the ONLY id -> composition mapping.
//
//	darwindeck judge backfill -table verdicts.json -dir output/myrun -out dossiers/
func cmdJudgeBackfill(args []string) {
	fs2 := flag.NewFlagSet("judge backfill", flag.ExitOnError)
	tablePath := fs2.String("table", "", "existing verdict table (composition -> score JSON) (required)")
	dir := fs2.String("dir", "", "directory to scan recursively for genome.json (required)")
	out := fs2.String("out", "", "dossier output directory for the missing compositions (required)")
	answerKey := fs2.String("answer-key", "", "private answer-key.json path, outside -out (default: <out>/../answer-key.json)")
	force := fs2.Bool("force", false, "overwrite an existing answer key (it maps the ids of an earlier dossier set)")
	fs2.Parse(args)

	if *tablePath == "" || *dir == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "usage: darwindeck judge backfill -table <verdicts.json> -dir <genome-dir> -out <dossier-dir> [-answer-key <path>] [-force]")
		os.Exit(1)
	}
	if err := runJudgeBackfill(*tablePath, *dir, *out, *answerKey, *force, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "judge backfill: %v\n", err)
		os.Exit(1)
	}
}

// backfillScan is the result of walking a run dir for unlabeled compositions.
type backfillScan struct {
	// reps maps each composition absent from the table to the first genome.json
	// found carrying it.
	reps map[string]string
	// scanned counts the genome.json files that were read and parsed.
	scanned int
	// problems lists every directory that could not be walked and every
	// genome.json that could not be read or parsed.
	problems []string
}

// scanUnlabeled walks dir for genome.json files and collects one
// representative per composition missing from table. A dir that cannot be
// walked at all is an error. A subdirectory or genome that cannot be read is
// recorded in problems and skipped -- it used to be swallowed, together with
// the walk error itself, so a mistyped -dir (or a run of unreadable genomes)
// scanned nothing and backfill announced "Table is complete".
func scanUnlabeled(dir string, table map[string]float64) (backfillScan, error) {
	scan := backfillScan{reps: map[string]string{}}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if p == dir {
				return walkErr
			}
			scan.problems = append(scan.problems, fmt.Sprintf("%s: %v", p, walkErr))
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() || d.Name() != "genome.json" {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			scan.problems = append(scan.problems, fmt.Sprintf("%s: %v", p, err))
			return nil
		}
		var g genome.Genome
		if err := json.Unmarshal(data, &g); err != nil {
			scan.problems = append(scan.problems, fmt.Sprintf("%s: %v", p, err))
			return nil
		}
		scan.scanned++
		c := evolution.Composition(&g)
		if _, labeled := table[c]; labeled {
			return nil
		}
		if _, seen := scan.reps[c]; !seen {
			scan.reps[c] = p
		}
		return nil
	})
	return scan, err
}

// runJudgeBackfill is `judge backfill` minus flag parsing and the process
// exit, so its failure modes are testable.
func runJudgeBackfill(tablePath, dir, out, answerKey string, force bool, stdout, stderr io.Writer) error {
	tableData, err := os.ReadFile(tablePath)
	if err != nil {
		return err
	}
	var table map[string]float64
	if err := json.Unmarshal(tableData, &table); err != nil {
		return fmt.Errorf("invalid table %s: %w", tablePath, err)
	}

	scan, err := scanUnlabeled(dir, table)
	if err != nil {
		return fmt.Errorf("scanning %s: %w", dir, err)
	}
	for _, p := range scan.problems {
		fmt.Fprintf(stderr, "warning: skipped %s\n", p)
	}
	if scan.scanned == 0 {
		// "Nothing unlabeled" and "nothing scanned" are different facts: the
		// second says nothing about the table.
		return fmt.Errorf("no readable genome.json under %s (%d skipped): nothing was scanned, so the table cannot be called complete", dir, len(scan.problems))
	}
	reps := scan.reps

	if len(reps) == 0 {
		fmt.Fprintf(stdout, "Table is complete: every composition in the %d genome(s) scanned under %s is already in %s.\n", scan.scanned, dir, tablePath)
		if len(scan.problems) > 0 {
			fmt.Fprintf(stdout, "(%d file(s) could not be read and were NOT checked -- see the warnings.)\n", len(scan.problems))
		}
		return nil
	}

	// Vet the answer-key path before anything is emitted (see runJudgeEmit).
	keyPath := answerKey
	if keyPath == "" {
		keyPath = judge.DefaultAnswerKeyPath(out)
	}
	if err := judge.CheckAnswerKeyPath(keyPath, out, force); err != nil {
		return err
	}

	comps := make([]string, 0, len(reps))
	for c := range reps {
		comps = append(comps, c)
	}
	sort.Strings(comps)

	// Stage representatives as <comp-slug>/genome.json and hand judge.Emit an
	// explicit source per staged file, so the answer key records the
	// composition slug as the true_name whatever the genome is (a representative
	// that happens to be a classic seed would otherwise be named after the seed).
	staging, err := os.MkdirTemp("", "dd-backfill-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	slug := strings.NewReplacer(":", "-", ",", "_")
	sources := map[string]judge.GameSource{}
	for _, c := range comps {
		d := filepath.Join(staging, slug.Replace(c))
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("staging %s: %w", c, err)
		}
		src, err := os.ReadFile(reps[c])
		if err != nil {
			return fmt.Errorf("staging %s: %w", c, err)
		}
		staged := filepath.Join(d, "genome.json")
		if err := os.WriteFile(staged, src, 0o644); err != nil {
			return fmt.Errorf("staging %s: %w", c, err)
		}
		sources[staged] = judge.GameSource{Path: reps[c], Source: "backfill", TrueName: slug.Replace(c)}
	}

	res, err := judge.Emit(staging, out, sources)
	if err != nil {
		return fmt.Errorf("emit: %w", err)
	}
	// Emit recorded each genome's path inside the (about to be deleted) staging
	// dir; point the key at the real representative instead.
	for id, rec := range res.AnswerKey {
		rec.Path = reps[rec.Composition]
		res.AnswerKey[id] = rec
	}
	if err := judge.WriteAnswerKey(keyPath, res.AnswerKey); err != nil {
		return fmt.Errorf("answer key: %w", err)
	}

	fmt.Fprintf(stdout, "%d unlabeled composition(s) in %d genome(s) scanned; emitted %d dossiers to %s\n", len(reps), scan.scanned, len(res.IDs), out)
	fmt.Fprintf(stdout, "Answer key (id -> composition, PRIVATE): %s\n", keyPath)
	for _, c := range comps {
		fmt.Fprintf(stdout, "  %-14s  <- %s\n", c, reps[c])
	}
	fmt.Fprintf(stdout, "\nNext: judge these dossiers (judge-novelty workflow), then append each\n")
	fmt.Fprintf(stdout, "composition->score to %s (novel > 0, variant/known < 0). Dossier ids are in a\n", tablePath)
	fmt.Fprintf(stdout, "pseudo-random order: read each id's composition from the answer key.\n")
	return nil
}

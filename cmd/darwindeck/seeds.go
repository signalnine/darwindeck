package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/darwindeck/darwindeck/pkg/seeds"
)

// cmdSeeds exports the built-in classic seed genomes as JSON so they can be
// served as anchors alongside evolved games (a blind ground-truth baseline for
// human ratings). The classics are real published games; serving them unlabeled
// next to the evolved set lets a ratings analysis ask "do the evolved games
// rate near the classics" instead of guessing what a raw 1-5 mean.
//
//	darwindeck seeds export -out <dir>            # all 11 classics
//	darwindeck seeds export -out <dir> gin-rummy big-two   # only the named ones
func cmdSeeds(args []string) {
	if len(args) == 0 || args[0] != "export" {
		fmt.Fprintln(os.Stderr, "usage: darwindeck seeds export -out <dir> [seed-id ...]")
		os.Exit(1)
	}
	positional, flagArgs := splitPositional(args[1:])
	fs := flag.NewFlagSet("seeds export", flag.ExitOnError)
	out := fs.String("out", "", "output directory")
	fs.Parse(flagArgs)
	if *out == "" {
		fmt.Fprintln(os.Stderr, "seeds export: -out <dir> is required")
		os.Exit(1)
	}
	if _, err := exportSeeds(*out, positional, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "seeds export: %v\n", err)
		os.Exit(1)
	}
}

// exportSeeds is `seeds export` minus flag parsing and the process exit, so
// its failure modes are testable. names selects seeds by id; empty = all.
//
// Every requested name is checked BEFORE anything is written. A name matching
// no seed used to be ignored: `seeds export -out d "Gin Rummy" big_two` (the
// ids are gin-rummy and big-two) printed "exported 0 seeds" and exited 0, and
// one typo in a longer list quietly exported a partial anchor set -- a blind
// ratings baseline missing a classic nobody noticed was missing. A seed that
// cannot be written is likewise an error, not a line on stderr under exit 0.
func exportSeeds(out string, names []string, stdout, stderr io.Writer) (int, error) {
	all := seeds.All()
	known := map[string]bool{}
	ids := make([]string, len(all))
	for i, g := range all {
		known[g.ID] = true
		ids[i] = g.ID
	}
	want := map[string]bool{}
	var unknown []string
	for _, n := range names {
		if !known[n] {
			unknown = append(unknown, strconv.Quote(n))
			continue
		}
		want[n] = true
	}
	if len(unknown) > 0 {
		return 0, fmt.Errorf("unknown seed id(s) %s; valid ids: %s", strings.Join(unknown, ", "), strings.Join(ids, ", "))
	}

	if err := os.MkdirAll(out, 0o755); err != nil {
		return 0, err
	}

	n := 0
	failed := 0
	for _, g := range all {
		if len(want) > 0 && !want[g.ID] {
			continue
		}
		data, err := json.MarshalIndent(g, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "  %s: %v\n", g.ID, err)
			failed++
			continue
		}
		slug := strings.ReplaceAll(g.ID, " ", "-")
		path := filepath.Join(out, slug+".json")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			fmt.Fprintf(stderr, "  %s: %v\n", g.ID, err)
			failed++
			continue
		}
		fmt.Fprintf(stdout, "  %s -> %s\n", g.ID, path)
		n++
	}
	fmt.Fprintf(stdout, "exported %d seeds to %s\n", n, out)
	if failed > 0 {
		return n, fmt.Errorf("%d seed(s) could not be exported to %s", failed, out)
	}
	return n, nil
}

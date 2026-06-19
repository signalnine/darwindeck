package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
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
	if err := os.MkdirAll(*out, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "seeds export: %v\n", err)
		os.Exit(1)
	}

	want := map[string]bool{}
	for _, n := range positional {
		want[n] = true
	}

	n := 0
	for _, g := range seeds.All() {
		if len(want) > 0 && !want[g.ID] {
			continue
		}
		data, err := json.MarshalIndent(g, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "  %s: %v\n", g.ID, err)
			continue
		}
		slug := strings.ReplaceAll(g.ID, " ", "-")
		path := filepath.Join(*out, slug+".json")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "  %s: %v\n", g.ID, err)
			continue
		}
		fmt.Printf("  %s -> %s\n", g.ID, path)
		n++
	}
	fmt.Printf("exported %d seeds to %s\n", n, *out)
}

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

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/webplay"
)

// cmdServe starts the browser playtest server over one genome file or a
// directory of them. The human plays the evolved game in a browser; ratings
// land in the same playtest_results.jsonl as CLI playtests.
//
//	darwindeck serve path/to/genome.json [-port 8080]
//	darwindeck serve -dir results/.../genomes [-host 0.0.0.0 -port 8080]
func cmdServe(args []string) {
	// splitPositional lets the genome path appear before OR after the flags
	// (Go's flag package otherwise stops parsing at the first positional, so
	// `serve genome.json -port N` would silently ignore -port).
	positional, flagArgs := splitPositional(args)
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("port", 8080, "TCP port to listen on")
	host := fs.String("host", "127.0.0.1", "bind address (127.0.0.1 = local only; 0.0.0.0 exposes to the network)")
	dir := fs.String("dir", "", "serve every genome under this directory (a game picker) instead of a single file")
	fs.Parse(flagArgs)

	var games []webplay.Game
	if *dir != "" {
		games = loadGamesDir(*dir)
	} else {
		if len(positional) == 0 {
			fmt.Fprintln(os.Stderr, "usage: darwindeck serve <genome.json> [-port N]  |  darwindeck serve -dir <dir> [-port N]")
			os.Exit(1)
		}
		g := loadGameFile(positional[0])
		games = []webplay.Game{webplay.RegisterGame(0, g, positional[0])}
	}
	if len(games) == 0 {
		fmt.Fprintln(os.Stderr, "no valid genomes to serve")
		os.Exit(1)
	}

	srv := webplay.NewServer(games)
	addr := fmt.Sprintf("%s:%d", *host, *port)

	fmt.Printf("DarwinDeck web playtest\n")
	fmt.Printf("  Games: %d\n", len(games))
	for _, g := range games {
		fmt.Printf("    - %s (%s)\n", g.Title, g.Skeleton)
	}
	if *host == "0.0.0.0" || *host == "" {
		fmt.Printf("  WARNING: bound to %s -- reachable from the network. Use -host 127.0.0.1 for local only.\n", addr)
	}
	fmt.Printf("  Open http://%s:%d/\n", displayHost(*host), *port)

	if err := srv.ListenAndServe(addr); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}

func displayHost(host string) string {
	if host == "" || host == "0.0.0.0" {
		return "localhost"
	}
	return host
}

// loadGameFile loads and validates one genome file, exiting on failure.
func loadGameFile(path string) *genome.Genome {
	g, errs := readGenome(path)
	if g == nil {
		fmt.Fprintf(os.Stderr, "Error loading %s: %s\n", path, strings.Join(errs, "; "))
		os.Exit(1)
	}
	if len(errs) > 0 {
		fmt.Fprintf(os.Stderr, "Genome validation failed for %s:\n", path)
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "  - %s\n", e)
		}
		os.Exit(1)
	}
	return g
}

// loadGamesDir walks dir for *.json genomes, loading the valid ones and skipping
// the rest with a warning (mirrors loadSeedDir's tolerance).
func loadGamesDir(dir string) []webplay.Game {
	var paths []string
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".json") {
			paths = append(paths, path)
		}
		return nil
	})
	sort.Strings(paths)

	var games []webplay.Game
	for _, p := range paths {
		g, errs := readGenome(p)
		if g == nil || len(errs) > 0 {
			fmt.Fprintf(os.Stderr, "  skipping %s (%s)\n", p, summarize(errs))
			continue
		}
		games = append(games, webplay.RegisterGame(len(games), g, p))
	}
	return games
}

// readGenome reads + parses + validates a genome file. Returns (nil, errs) on
// read/parse failure; (g, validationErrs) otherwise.
func readGenome(path string) (*genome.Genome, []string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, []string{err.Error()}
	}
	var g genome.Genome
	if err := json.Unmarshal(data, &g); err != nil {
		return nil, []string{"parse: " + err.Error()}
	}
	return &g, genome.Validate(&g)
}

func summarize(errs []string) string {
	if len(errs) == 0 {
		return "invalid"
	}
	return errs[0]
}

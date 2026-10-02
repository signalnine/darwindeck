package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/darwindeck/darwindeck/pkg/judge"
)

// valueFlags are the flags (across the subcommands that use splitPositional)
// that consume the following token as their value, so splitPositional does not
// mistake that token for a positional. The `--flag=value` form is self-
// describing and needs no entry here.
var valueFlags = map[string]bool{
	"-out": true, "--out": true,
	"-answer-key": true, "--answer-key": true,
	// serve
	"-port": true, "--port": true,
	"-host": true, "--host": true,
	"-dir": true, "--dir": true,
	// playtest
	"-difficulty": true, "--difficulty": true,
	"-seed": true, "--seed": true,
}

// splitPositional separates bare positional arguments from flag arguments so
// flags may appear before OR after positionals. A token starting with '-' is a
// flag; if it is a known value-flag without an '=' form, the next token is its
// value and is kept with the flags.
func splitPositional(args []string) (positional, flagArgs []string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if len(a) > 0 && a[0] == '-' {
			flagArgs = append(flagArgs, a)
			// `--flag=value` carries its own value; `--flag value` consumes the
			// next token.
			if valueFlags[a] && i+1 < len(args) {
				i++
				flagArgs = append(flagArgs, args[i])
			}
			continue
		}
		positional = append(positional, a)
	}
	return positional, flagArgs
}

// cmdJudge dispatches the LLM-as-judge subcommands: emit (build blind
// dossiers) and rank (ingest verdicts, re-rank, flag rediscoveries).
func cmdJudge(args []string) {
	if len(args) == 0 {
		printJudgeUsage()
		os.Exit(1)
	}
	switch args[0] {
	case "emit":
		cmdJudgeEmit(args[1:])
	case "rank":
		cmdJudgeRank(args[1:])
	case "backfill":
		cmdJudgeBackfill(args[1:])
	case "help", "--help", "-h":
		printJudgeUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown judge subcommand: %s\n", args[0])
		printJudgeUsage()
		os.Exit(1)
	}
}

func printJudgeUsage() {
	fmt.Println(`Usage: darwindeck judge <subcommand> [flags]

Subcommands:
  emit <input> --out <dir> [--answer-key <path>] [--force]
      Build BLIND dossiers (one per genome.json found recursively under
      <input>) into <dir>, plus manifest.json and prompt.md. Writes a PRIVATE
      answer-key.json OUTSIDE <dir> (default: <dir>/../answer-key.json).
      Dossier ids (G01..) are assigned in a pseudo-random order, not in
      input/rank order: the answer key (path, composition per id) is the only
      id -> genome mapping. An existing answer key is never overwritten
      without --force; give each dossier set its own --answer-key.

  rank <dossier-dir> <verdicts.json> [--out <report.md>]
      Ingest verdicts, aggregate majority-of-3 per id, re-rank by judged
      quality, flag rediscoveries, and write judged-report.md + judged.json.

  backfill -table <verdicts.json> -dir <genome-dir> -out <dossier-dir>
           [-answer-key <path>] [-force]
      Emit blind dossiers for every composition present under <genome-dir>
      that is NOT yet in <verdicts.json>, so the composition-keyed table can be
      completed and the in-loop judge becomes a zero-cost lookup. The answer
      key (default: <dossier-dir>/../answer-key.json) maps each id to its
      composition; same overwrite rule as emit.`)
}

func cmdJudgeEmit(args []string) {
	fs := flag.NewFlagSet("judge emit", flag.ExitOnError)
	out := fs.String("out", "", "output dossier directory (required)")
	answerKey := fs.String("answer-key", "", "private answer-key.json path, outside <out> (default: <out>/../answer-key.json)")
	force := fs.Bool("force", false, "overwrite an existing answer key (it maps the ids of an earlier dossier set)")
	// Parse with the leading positional(s) hoisted out, so flags may appear
	// either before OR after the <input> argument (Go's flag package otherwise
	// stops at the first positional).
	positional, flagArgs := splitPositional(args)
	fs.Parse(flagArgs)

	if len(positional) == 0 || *out == "" {
		fmt.Fprintln(os.Stderr, "usage: darwindeck judge emit <input> --out <dir> [--answer-key <path>] [--force]")
		os.Exit(1)
	}
	if err := runJudgeEmit(positional[0], *out, *answerKey, *force, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "judge emit: %v\n", err)
		os.Exit(1)
	}
}

// runJudgeEmit is `judge emit` minus flag parsing and the process exit, so its
// failure modes are testable.
//
// The answer-key path is vetted BEFORE anything is emitted. The key used to be
// written last with no check at all: a second emit sharing the default
// <out>/../answer-key.json silently replaced the first set's key (ids restart
// at G01 every emit, so that key was the only mapping of dossiers possibly
// already out for judging), and `--out .` put the key inside the blind dir.
// Refusing only after the dossiers were swept and renumbered would leave the
// dossier dir describing a different set than the surviving key.
func runJudgeEmit(input, out, answerKey string, force bool, stdout io.Writer) error {
	keyPath := answerKey
	if keyPath == "" {
		// Default: the parent of the dossier dir, so it stays OUT of the blind set.
		keyPath = judge.DefaultAnswerKeyPath(out)
	}
	if err := judge.CheckAnswerKeyPath(keyPath, out, force); err != nil {
		return err
	}

	res, err := judge.Emit(input, out, nil)
	if err != nil {
		return err
	}
	if err := judge.WriteAnswerKey(keyPath, res.AnswerKey); err != nil {
		return fmt.Errorf("failed to write answer key: %w", err)
	}

	fmt.Fprintf(stdout, "Emitted %d dossiers to %s\n", len(res.IDs), out)
	fmt.Fprintf(stdout, "Manifest: %s\n", filepath.Join(out, "manifest.json"))
	fmt.Fprintf(stdout, "Prompt:   %s\n", filepath.Join(out, "prompt.md"))
	fmt.Fprintf(stdout, "Answer key (PRIVATE, kept out of the dossier dir): %s\n", keyPath)
	fmt.Fprintf(stdout, "IDs: %s\n", strings.Join(res.IDs, " "))
	fmt.Fprintf(stdout, "Ids are assigned in a pseudo-random order, NOT in input/rank order: map an id to its genome through the answer key (path, composition).\n")
	return nil
}

func cmdJudgeRank(args []string) {
	fs := flag.NewFlagSet("judge rank", flag.ExitOnError)
	out := fs.String("out", "", "judged report path (default: <dossier-dir>/judged-report.md)")
	positional, flagArgs := splitPositional(args)
	fs.Parse(flagArgs)

	if len(positional) < 2 {
		fmt.Fprintln(os.Stderr, "usage: darwindeck judge rank <dossier-dir> <verdicts.json> [--out report.md]")
		os.Exit(1)
	}
	dossierDir := positional[0]
	verdictsPath := positional[1]

	verdicts, err := judge.LoadVerdicts(verdictsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "judge rank error: %v\n", err)
		os.Exit(1)
	}

	ranked := judge.Rank(judge.Aggregate(verdicts))

	reportPath := *out
	if reportPath == "" {
		reportPath = filepath.Join(dossierDir, "judged-report.md")
	}
	jsonPath := filepath.Join(filepath.Dir(reportPath), "judged.json")

	if err := judge.WriteReport(reportPath, jsonPath, ranked); err != nil {
		fmt.Fprintf(os.Stderr, "judge rank: failed to write report: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Ranked %d games from %d verdicts\n", len(ranked), len(verdicts))
	fmt.Printf("Report: %s\n", reportPath)
	fmt.Printf("JSON:   %s\n", jsonPath)
}

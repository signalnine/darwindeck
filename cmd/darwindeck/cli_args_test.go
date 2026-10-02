package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// helperArgsEnv carries a darwindeck command line into the helper process
// (fields joined by the ASCII unit separator).
const helperArgsEnv = "DARWINDECK_HELPER_ARGS"

// TestHelperProcessMain is not a test of its own: runCLI re-executes the test
// binary with helperArgsEnv set, and this function then runs the real main()
// with those arguments, so the os.Exit paths of the CLI can be exercised
// end to end. Without the variable it does nothing.
func TestHelperProcessMain(t *testing.T) {
	raw, ok := os.LookupEnv(helperArgsEnv)
	if !ok {
		return
	}
	os.Args = append([]string{"darwindeck"}, strings.Split(raw, "\x1f")...)
	main()
	os.Exit(0)
}

type cliResult struct {
	stdout, stderr string
	exitCode       int
	timedOut       bool
}

// runCLI runs `darwindeck args...` in a child process and reports its output
// and exit status. A run that outlives timeout is killed and flagged.
func runCLI(t *testing.T, timeout time.Duration, args ...string) cliResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestHelperProcessMain$")
	cmd.Env = append(os.Environ(), helperArgsEnv+"="+strings.Join(args, "\x1f"))
	cmd.Dir = t.TempDir() // any default-relative output lands in a temp dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()

	res := cliResult{stdout: stdout.String(), stderr: stderr.String()}
	if ctx.Err() != nil {
		res.timedOut = true
		return res
	}
	var exitErr *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exitErr):
		res.exitCode = exitErr.ExitCode()
	default:
		t.Fatalf("running %v: %v", args, err)
	}
	return res
}

// TestStrayArgsError: Go's flag parsing stops at the first non-flag token and
// silently leaves everything after it unparsed. `evolve ... -cross-skeleton
// true -novelty-select -output x` therefore ran with novelty-select OFF and
// the default output directory: "true" is a positional argument (boolean
// flags take no separate value), and the two flags after it were dropped.
func TestStrayArgsError(t *testing.T) {
	newFS := func() (*flag.FlagSet, *bool, *string) {
		fs := flag.NewFlagSet("evolve", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		fs.Bool("cross-skeleton", false, "")
		nov := fs.Bool("novelty-select", false, "")
		out := fs.String("output", "", "")
		return fs, nov, out
	}

	fs, _, _ := newFS()
	if err := fs.Parse([]string{"-cross-skeleton", "-novelty-select", "-output", "x"}); err != nil {
		t.Fatal(err)
	}
	if err := strayArgsError(fs); err != nil {
		t.Errorf("clean command line rejected: %v", err)
	}

	fs, nov, out := newFS()
	if err := fs.Parse([]string{"-cross-skeleton", "true", "-novelty-select", "-output", "x"}); err != nil {
		t.Fatal(err)
	}
	if *nov || *out != "" {
		t.Fatal("fixture: the flags after the stray token should be unparsed")
	}
	err := strayArgsError(fs)
	if err == nil {
		t.Fatal("stray positional argument accepted; the flags after it were silently dropped")
	}
	for _, want := range []string{"evolve", `"true"`, "-novelty-select"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %s", err, want)
		}
	}
}

// TestCLIRejectsStrayArgs: evolve, experiment and calibrate take no positional
// arguments, so a stray token must stop the command (non-zero exit, a message
// naming the token) before any work starts.
func TestCLIRejectsStrayArgs(t *testing.T) {
	tmp := t.TempDir()
	cases := []struct {
		name  string
		args  []string
		stray string
	}{
		{"evolve", []string{"evolve", "-population", "4", "-generations", "0", "-mcts-decile", "0",
			"-output", filepath.Join(tmp, "evolve"), "-cross-skeleton", "true", "-novelty-select"}, "true"},
		{"experiment", []string{"experiment", "-seeds", "1", "-population", "2", "-generations", "0",
			"-configs", "random", "-output", filepath.Join(tmp, "experiment"), "stray", "-parallel", "1"}, "stray"},
		{"calibrate", []string{"calibrate", "extra"}, "extra"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res := runCLI(t, 30*time.Second, c.args...)
			if res.timedOut {
				t.Fatalf("%s ran (and was killed after the timeout) instead of rejecting the stray argument", c.name)
			}
			if res.exitCode == 0 {
				t.Errorf("exit code 0, want non-zero; stdout:\n%s", res.stdout)
			}
			if want := `unexpected argument "` + c.stray + `"`; !strings.Contains(res.stderr, want) {
				t.Errorf("stderr %q does not contain %q", res.stderr, want)
			}
			if strings.Contains(res.stdout, "Evolution complete") || strings.Contains(res.stdout, "All experiments complete") ||
				strings.Contains(res.stdout, "calibration report") {
				t.Errorf("command did its work despite the stray argument; stdout:\n%s", res.stdout)
			}
		})
	}
}

// TestCLIExperimentParallelZeroCompletes: end-to-end guard for the
// `-parallel 0` hang (see TestExperimentParallelism).
func TestCLIExperimentParallelZeroCompletes(t *testing.T) {
	out := filepath.Join(t.TempDir(), "exp")
	res := runCLI(t, 60*time.Second, "experiment", "-seeds", "1", "-population", "2", "-generations", "0",
		"-configs", "random", "-workers", "2", "-parallel", "0", "-output", out)
	if res.timedOut {
		t.Fatal("experiment -parallel 0 hung")
	}
	if res.exitCode != 0 {
		t.Fatalf("exit code %d; stderr:\n%s", res.exitCode, res.stderr)
	}
	if _, err := os.Stat(filepath.Join(out, "results.json")); err != nil {
		t.Errorf("results.json not written: %v", err)
	}
}

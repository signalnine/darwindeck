package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
)

func exportedFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}

// TestExportSeedsUnknownNameIsError: a name that matches no seed was silently
// ignored, so `seeds export -out d "Gin Rummy" big_two` (the ids are
// gin-rummy and big-two) printed "exported 0 seeds" and exited 0, and a list
// with ONE typo quietly exported a partial anchor set. An unknown name is an
// error that lists the valid ids, and nothing is written.
func TestExportSeedsUnknownNameIsError(t *testing.T) {
	out := filepath.Join(t.TempDir(), "anchors")
	var stdout, stderr bytes.Buffer

	n, err := exportSeeds(out, []string{"gin-rummy", "Gin Rummy", "big_two"}, &stdout, &stderr)
	if err == nil {
		t.Fatalf("unknown seed names were not an error (exported %d); stdout: %s", n, stdout.String())
	}
	msg := err.Error()
	for _, bad := range []string{`"Gin Rummy"`, `"big_two"`} {
		if !strings.Contains(msg, bad) {
			t.Errorf("error does not name the unknown seed %s: %v", bad, err)
		}
	}
	for _, g := range seeds.All() {
		if !strings.Contains(msg, g.ID) {
			t.Errorf("error does not list the valid seed id %q: %v", g.ID, err)
		}
	}
	if files := exportedFiles(t, out); len(files) != 0 {
		t.Errorf("a request with unknown names still exported a partial set: %v", files)
	}
}

// TestExportSeedsWritesNamed: the valid paths are unchanged -- named seeds
// export exactly those, no names exports all 11, each as a loadable genome.
func TestExportSeedsWritesNamed(t *testing.T) {
	root := t.TempDir()
	var stdout, stderr bytes.Buffer

	some := filepath.Join(root, "some")
	n, err := exportSeeds(some, []string{"big-two", "gin-rummy", "big-two"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("exporting two known seeds: %v", err)
	}
	if got, want := exportedFiles(t, some), []string{"big-two.json", "gin-rummy.json"}; n != 2 || strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("exported %d seeds %v, want 2: %v", n, got, want)
	}

	all := filepath.Join(root, "all")
	n, err = exportSeeds(all, nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("exporting all seeds: %v", err)
	}
	files := exportedFiles(t, all)
	if n != len(seeds.All()) || len(files) != len(seeds.All()) {
		t.Fatalf("exported %d seeds into %d files, want %d", n, len(files), len(seeds.All()))
	}
	for _, name := range files {
		data, err := os.ReadFile(filepath.Join(all, name))
		if err != nil {
			t.Fatal(err)
		}
		var g genome.Genome
		if err := json.Unmarshal(data, &g); err != nil {
			t.Errorf("%s is not a loadable genome: %v", name, err)
			continue
		}
		if errs := genome.Validate(&g); len(errs) > 0 {
			t.Errorf("%s fails validation: %v", name, errs)
		}
	}
}

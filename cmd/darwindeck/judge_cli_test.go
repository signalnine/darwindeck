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
	"github.com/darwindeck/darwindeck/pkg/judge"
	"github.com/darwindeck/darwindeck/pkg/seeds"
)

// putGenome writes g to <dir>/<name>/genome.json and returns the file path.
func putGenome(t *testing.T, dir, name string, g *genome.Genome) string {
	t.Helper()
	sub := filepath.Join(dir, name)
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(sub, "genome.json")
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func putFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func dossierFiles(t *testing.T, dir string) []string {
	t.Helper()
	m, err := filepath.Glob(filepath.Join(dir, "G*.md"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(m)
	return m
}

func readAnswerKey(t *testing.T, path string) map[string]judge.AnswerRec {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("answer key not written: %v", err)
	}
	var key map[string]judge.AnswerRec
	if err := json.Unmarshal(data, &key); err != nil {
		t.Fatal(err)
	}
	return key
}

// TestBackfillMissingDirIsError: the scan discarded WalkDir's error, so a
// typo'd -dir scanned nothing, found nothing unlabeled, and printed "Table is
// complete" with exit 0 -- the in-loop judge then ran on an incomplete table
// believed complete.
func TestBackfillMissingDirIsError(t *testing.T) {
	root := t.TempDir()
	table := filepath.Join(root, "verdicts.json")
	putFile(t, table, `{}`)
	var stdout, stderr bytes.Buffer

	err := runJudgeBackfill(table, filepath.Join(root, "no-such-run"), filepath.Join(root, "dossiers"), "", false, &stdout, &stderr)
	if err == nil {
		t.Fatalf("a missing -dir was not an error; stdout: %s", stdout.String())
	}
	if strings.Contains(stdout.String(), "Table is complete") {
		t.Errorf("a missing -dir reported the table complete: %s", stdout.String())
	}
}

// TestBackfillNothingScannedIsError: read and parse failures were swallowed
// too, so a dir whose genome.json files are all unreadable also "completed"
// the table.
func TestBackfillNothingScannedIsError(t *testing.T) {
	root := t.TempDir()
	table := filepath.Join(root, "verdicts.json")
	putFile(t, table, `{}`)
	run := filepath.Join(root, "run")
	bad := filepath.Join(run, "rank01", "genome.json")
	putFile(t, bad, "not json")
	putFile(t, filepath.Join(run, "notes.txt"), "no genomes here")
	var stdout, stderr bytes.Buffer

	err := runJudgeBackfill(table, run, filepath.Join(root, "dossiers"), "", false, &stdout, &stderr)
	if err == nil {
		t.Fatalf("a dir with no parsable genome.json was not an error; stdout: %s", stdout.String())
	}
	if strings.Contains(stdout.String(), "Table is complete") {
		t.Errorf("nothing was scanned but the table was reported complete: %s", stdout.String())
	}
	if !strings.Contains(stderr.String(), bad) {
		t.Errorf("the unparsable genome was not reported; stderr: %s", stderr.String())
	}
}

// TestBackfillCompleteTableIsNotAnError: the legitimate "nothing to do" case
// stays a clean exit.
func TestBackfillCompleteTableIsNotAnError(t *testing.T) {
	root := t.TempDir()
	run := filepath.Join(root, "run")
	putGenome(t, run, "rank01", seeds.CrazyEights()) // composition "0:"
	putGenome(t, run, "rank02", seeds.MauMau())      // composition "0:"
	table := filepath.Join(root, "verdicts.json")
	putFile(t, table, `{"0:": -0.6}`)
	out := filepath.Join(root, "dossiers")
	var stdout, stderr bytes.Buffer

	if err := runJudgeBackfill(table, run, out, "", false, &stdout, &stderr); err != nil {
		t.Fatalf("a fully labeled run is not an error: %v", err)
	}
	if !strings.Contains(stdout.String(), "Table is complete") {
		t.Errorf("stdout = %q, want the table-complete message", stdout.String())
	}
	if _, err := os.Stat(out); err == nil {
		t.Errorf("nothing was unlabeled but %s was created", out)
	}
}

// TestBackfillAnswerKeyFlag: backfill had no -answer-key flag and always wrote
// <out>/../answer-key.json, silently replacing the key of a `judge emit` into a
// sibling dir. With the flag the key goes where asked, the default location is
// untouched, and -- dossier ids being a pseudo-random permutation -- the key
// maps every id to its composition and its real source genome.
func TestBackfillAnswerKeyFlag(t *testing.T) {
	root := t.TempDir()
	run := filepath.Join(root, "run")
	shed := putGenome(t, run, "rank01", seeds.CrazyEights()) // "0:"
	putGenome(t, run, "rank02", seeds.MauMau())              // "0:" again (not a second representative)
	rummy := putGenome(t, run, "rank03", seeds.GinRummy())   // "2:"
	bad := filepath.Join(run, "rank04", "genome.json")
	putFile(t, bad, "{ truncated")
	table := filepath.Join(root, "verdicts.json")
	putFile(t, table, `{}`)
	out := filepath.Join(root, "dossiers")
	key := filepath.Join(root, "keys", "backfill-key.json")
	var stdout, stderr bytes.Buffer

	if err := runJudgeBackfill(table, run, out, key, false, &stdout, &stderr); err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "answer-key.json")); err == nil {
		t.Error("-answer-key was given but the default <out>/../answer-key.json was written too")
	}
	if !strings.Contains(stderr.String(), bad) {
		t.Errorf("the unparsable genome was skipped silently; stderr: %s", stderr.String())
	}

	got := readAnswerKey(t, key)
	if len(got) != 2 || len(dossierFiles(t, out)) != 2 {
		t.Fatalf("want 2 dossiers and 2 key entries (one per unlabeled composition), got key %v, files %v", got, dossierFiles(t, out))
	}
	want := map[string]judge.AnswerRec{
		"0:": {TrueName: "0-", Composition: "0:", Path: shed, Skeleton: "shedding"},
		"2:": {TrueName: "2-", Composition: "2:", Path: rummy, Skeleton: "rummy"},
	}
	for id, rec := range got {
		w, ok := want[rec.Composition]
		if !ok {
			t.Errorf("%s: unexpected composition %q", id, rec.Composition)
			continue
		}
		if rec.TrueName != w.TrueName || rec.Path != w.Path || rec.Skeleton != w.Skeleton {
			t.Errorf("%s: answer key = %+v, want true_name %q path %q skeleton %q", id, rec, w.TrueName, w.Path, w.Skeleton)
		}
		// The dossier under this id must be the game the key names.
		doc, err := os.ReadFile(filepath.Join(out, id+".md"))
		if err != nil {
			t.Fatal(err)
		}
		isRummy := strings.Contains(string(doc), "**rummy game**")
		if isRummy != (rec.Composition == "2:") {
			t.Errorf("%s: the dossier does not match the composition %q the key gives it", id, rec.Composition)
		}
	}
}

// TestJudgeEmitRefusesExistingKeyBeforeEmitting: the answer key was written
// AFTER the dossiers with no existence check. A second emit that shares a key
// path must be refused before it sweeps and renumbers anything -- otherwise
// the dossier dir holds the new set while the key still describes the old one
// (or the old set's key is silently gone).
func TestJudgeEmitRefusesExistingKeyBeforeEmitting(t *testing.T) {
	root := t.TempDir()
	in := filepath.Join(root, "queue")
	putGenome(t, in, "rank01", seeds.CrazyEights())
	putGenome(t, in, "rank02", seeds.GinRummy())
	out := filepath.Join(root, "dossiers", "gen020")
	key := filepath.Join(root, "dossiers", "answer-key.json") // the default location
	var stdout bytes.Buffer

	if err := runJudgeEmit(in, out, "", false, &stdout); err != nil {
		t.Fatalf("first emit: %v", err)
	}
	first, err := os.ReadFile(key)
	if err != nil {
		t.Fatalf("default answer key not written: %v", err)
	}

	// A marker that any emit into this dir would sweep.
	marker := filepath.Join(out, "G99.md")
	putFile(t, marker, "an earlier, larger set's dossier")

	if err := runJudgeEmit(in, out, "", false, &stdout); err == nil {
		t.Fatal("re-emit over an existing answer key was not refused")
	}
	if _, err := os.Stat(marker); err != nil {
		t.Error("the refused emit still swept the dossier dir")
	}
	if now, _ := os.ReadFile(key); !bytes.Equal(now, first) {
		t.Error("the refused emit changed the existing answer key")
	}

	// A different dossier set that would share the default key is refused too.
	sibling := filepath.Join(root, "dossiers", "gen040")
	if err := runJudgeEmit(in, sibling, "", false, &stdout); err == nil {
		t.Error("an emit into a sibling dir silently replaced the other set's answer key")
	}
	if _, err := os.Stat(sibling); err == nil {
		t.Error("the refused emit created its dossier dir")
	}

	// -force (or a distinct -answer-key) is the way through.
	if err := runJudgeEmit(in, out, "", true, &stdout); err != nil {
		t.Fatalf("forced re-emit: %v", err)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Error("the forced emit did not sweep the stale dossier")
	}
	own := filepath.Join(root, "dossiers", "gen040-key.json")
	if err := runJudgeEmit(in, sibling, own, false, &stdout); err != nil {
		t.Fatalf("emit with its own -answer-key: %v", err)
	}
	if len(readAnswerKey(t, own)) != 2 {
		t.Errorf("the sibling set's own key is incomplete")
	}
}

// TestJudgeEmitRejectsKeyInsideOut: a key inside the dossier dir is handed to
// the judge with the blind set.
func TestJudgeEmitRejectsKeyInsideOut(t *testing.T) {
	root := t.TempDir()
	in := filepath.Join(root, "queue")
	putGenome(t, in, "rank01", seeds.CrazyEights())
	out := filepath.Join(root, "dossiers")
	var stdout bytes.Buffer

	if err := runJudgeEmit(in, out, filepath.Join(out, "answer-key.json"), true, &stdout); err == nil {
		t.Fatal("an answer key inside the dossier dir was accepted")
	}
	if _, err := os.Stat(out); err == nil {
		t.Error("the rejected emit still wrote the dossier dir")
	}
}

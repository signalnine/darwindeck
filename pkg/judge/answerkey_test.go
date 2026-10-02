package judge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCheckAnswerKeyPathRefusesOverwrite: dossier ids restart at G01 on every
// emit, so the answer key of an earlier set is the only id -> genome mapping
// of dossiers that may already be out for judging. `judge emit --out
// run/dossiers` followed by `judge backfill -out run/backfill` shared the
// default <out>/../answer-key.json and the second silently replaced the first.
func TestCheckAnswerKeyPathRefusesOverwrite(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(root, "dossiers")
	key := filepath.Join(root, "answer-key.json")

	if err := CheckAnswerKeyPath(key, out, false); err != nil {
		t.Fatalf("a fresh key path outside the dossier dir must be accepted: %v", err)
	}
	if err := os.WriteFile(key, []byte(`{"G01":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	err := CheckAnswerKeyPath(key, out, false)
	if err == nil {
		t.Fatal("an existing answer key was accepted for overwrite without force")
	}
	if !strings.Contains(err.Error(), "already exists") || !strings.Contains(err.Error(), "-force") {
		t.Errorf("refusal should say the key exists and name the override: %v", err)
	}
	if err := CheckAnswerKeyPath(key, out, true); err != nil {
		t.Errorf("force must allow the overwrite: %v", err)
	}
}

// TestCheckAnswerKeyPathRejectsInsideOut: a key inside the dossier dir is
// handed to the judge with the blind set. force does not override this.
func TestCheckAnswerKeyPathRejectsInsideOut(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(root, "dossiers")
	for _, key := range []string{
		filepath.Join(out, "answer-key.json"),
		filepath.Join(out, "private", "key.json"),
		filepath.Join(out, "..", "dossiers", "answer-key.json"),
	} {
		for _, force := range []bool{false, true} {
			err := CheckAnswerKeyPath(key, out, force)
			if err == nil || !strings.Contains(err.Error(), "inside the dossier dir") {
				t.Errorf("key %s (force=%v) inside the dossier dir was not rejected: %v", key, force, err)
			}
		}
	}
	// A sibling whose name merely starts with the dossier dir's name is outside.
	if err := CheckAnswerKeyPath(filepath.Join(root, "dossiers-key.json"), out, false); err != nil {
		t.Errorf("a sibling file was rejected as inside the dossier dir: %v", err)
	}
}

// TestDefaultAnswerKeyPathIsOutsideOut: the default was computed lexically as
// Dir(Clean(out))/answer-key.json. For `--out .` that is ./answer-key.json --
// inside the dossier dir.
func TestDefaultAnswerKeyPathIsOutsideOut(t *testing.T) {
	if got, want := DefaultAnswerKeyPath(filepath.Join("run", "dossiers")), filepath.Join("run", "answer-key.json"); got != want {
		t.Errorf("default key for run/dossiers = %s, want %s (the documented <out>/../answer-key.json)", got, want)
	}

	// "." is resolved against the working directory; nothing is written, so
	// the test needs no chdir. The key must land in the PARENT of the cwd.
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(filepath.Dir(cwd), "answer-key.json")
	for _, out := range []string{".", "./", cwd} {
		if got := DefaultAnswerKeyPath(out); got != want {
			t.Errorf("default key for --out %q = %s, want %s (outside the dossier dir)", out, got, want)
		}
	}
}

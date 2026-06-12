package genome

// Structural inert-parameter guard (audit remediation Task 25).
//
// The audit counted 9+ inert-parameter bugs (the dd-027 class: CanStack,
// PlayMultiple, CanLayOff, ...) caused by six hand-synchronized surfaces:
// a field existed in the genome and was mutated by evolution, but no runner,
// simulator, fitness metric, or mechanic hook ever read it -- evolution was
// optimizing noise and rulebooks lied about it.
//
// This test is a tripwire: every evolvable parameter must appear as a
// selector in at least one CONSUMING package -- pkg/skeleton/..., pkg/sim/...,
// pkg/fitness/..., pkg/mechanic/... -- i.e. somewhere OUTSIDE pkg/genome,
// pkg/evolution, and pkg/output (which only define, mutate, and print
// parameters; reads there do not prove the parameter affects gameplay).
//
// "Appears as selector" is deliberately narrow and cheap: any
// ast.SelectorExpr whose Sel.Name equals the field name, in any non-test
// file of the consuming packages. Field names are unique across the three
// param structs (TestParamFieldNamesUniqueAcrossParamStructs verifies this;
// if two ever collide, qualify the colliding fields by struct type via
// go/types -- only for those). This proves "read somewhere", not
// "semantically affects outcomes"; the calibration/evolvability checks are
// the semantic complement.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// consumingDirs are the package trees where a parameter read counts as
// "consumed". Paths are relative to this package directory (pkg/genome).
var consumingDirs = []string{
	filepath.Join("..", "skeleton"),
	filepath.Join("..", "sim"),
	filepath.Join("..", "fitness"),
	filepath.Join("..", "mechanic"),
}

// mutatedGenomeFields lists every Genome field (or Scoring leaf field) that
// a mutation operator in pkg/evolution/mutate.go can change. Cross-reference
// (keep in sync with mutate.go):
//
//	tweakParameter          -> Players, HandSize (skeleton param ints are
//	                           covered via the param-struct reflection below)
//	flipBool                -> TrickTaking.MustFollowSuit (param struct)
//	changeEnum              -> TrumpRule, Scoring.TrumpSuit,
//	                           Scoring.CardPoints (+ param-struct enums)
//	addSpecialCard/remove   -> SpecialCards
//	addBorrowedMechanic/rem -> Borrowed
//	changeSkeleton          -> Skeleton (whole-genome seed replacement)
//	mutateScoring           -> Scoring.CardPoints
//
// Deliberately excluded: ID and Generation. Mutate writes them, but they are
// lineage bookkeeping, not game parameters -- a runner that never reads them
// is correct, not buggy.
var mutatedGenomeFields = []string{
	"Players",
	"HandSize",
	"TrumpRule",
	"TrumpSuit",
	"CardPoints",
	"SpecialCards",
	"Borrowed",
	"Skeleton",
}

// inertAllowlist exempts fields from the consumed-somewhere requirement.
// Every entry MUST carry a non-empty why-comment as its value, explaining
// why the field is legitimately unread by runners/sim/fitness/mechanic.
//
// HARD CAP: 3 entries. If this list wants a 4th entry, the guard has gone
// toothless -- fix the inert parameter instead (delete it or make a runner
// consume it). The cap is enforced by the test.
var inertAllowlist = map[string]string{
	// (empty -- keep it that way)
}

// paramStructTypes returns the three skeleton parameter structs whose every
// exported field must be consumed. Reflection (not a hand-written name list)
// so that a newly added field is guarded automatically.
func paramStructTypes() []reflect.Type {
	return []reflect.Type{
		reflect.TypeOf(SheddingParams{}),
		reflect.TypeOf(TrickTakingParams{}),
		reflect.TypeOf(RummyParams{}),
	}
}

// TestParamFieldNamesUniqueAcrossParamStructs verifies the assumption that
// makes the cheap selector match sound: no field name appears in more than
// one of the three param structs. If this ever fails, do NOT weaken the
// guard -- resolve the colliding fields' receivers by struct type using
// go/types (only for the collisions).
func TestParamFieldNamesUniqueAcrossParamStructs(t *testing.T) {
	seen := map[string]string{} // field name -> struct that owns it
	for _, st := range paramStructTypes() {
		for i := 0; i < st.NumField(); i++ {
			f := st.Field(i)
			if !f.IsExported() {
				continue
			}
			if owner, dup := seen[f.Name]; dup {
				t.Errorf("field %q appears in both %s and %s: bare-name selector matching is no longer sound for it; qualify by struct type via go/types for the colliding fields",
					f.Name, owner, st.Name())
			}
			seen[f.Name] = st.Name()
		}
	}
}

// collectConsumedSelectors parses every non-test Go file under the consuming
// package trees and returns the set of all ast.SelectorExpr Sel names.
// Test files are excluded: a parameter read only by a test is still inert
// in production.
func collectConsumedSelectors(t *testing.T) map[string]bool {
	t.Helper()
	fset := token.NewFileSet()
	selectors := make(map[string]bool)
	for _, dir := range consumingDirs {
		parsed := 0
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, perr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
			if perr != nil {
				return perr
			}
			parsed++
			ast.Inspect(file, func(n ast.Node) bool {
				if sel, ok := n.(*ast.SelectorExpr); ok {
					selectors[sel.Sel.Name] = true
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatalf("walking consuming package dir %s: %v", dir, err)
		}
		if parsed == 0 {
			t.Fatalf("no non-test Go files found under %s -- if the package moved, update consumingDirs; an empty scan would make this guard vacuous", dir)
		}
	}
	return selectors
}

// TestEvolvableParamsAreConsumed is the guard: every exported field of
// SheddingParams/TrickTakingParams/RummyParams, and every Genome field that
// mutation can touch, must be read somewhere in pkg/skeleton, pkg/sim,
// pkg/fitness, or pkg/mechanic.
func TestEvolvableParamsAreConsumed(t *testing.T) {
	// Allowlist discipline first: cap and mandatory why-comments.
	if len(inertAllowlist) > 3 {
		t.Fatalf("inertAllowlist has %d entries; the cap is 3. A growing allowlist means this guard has gone toothless -- delete the inert parameter or make a runner consume it instead", len(inertAllowlist))
	}
	for field, why := range inertAllowlist {
		if strings.TrimSpace(why) == "" {
			t.Fatalf("inertAllowlist entry %q has no why-comment; every exemption must justify itself", field)
		}
	}

	// Build the required-field set.
	required := []string{}
	requiredSet := map[string]bool{}
	addRequired := func(name string) {
		if !requiredSet[name] {
			requiredSet[name] = true
			required = append(required, name)
		}
	}
	for _, st := range paramStructTypes() {
		for i := 0; i < st.NumField(); i++ {
			if f := st.Field(i); f.IsExported() {
				addRequired(f.Name)
			}
		}
	}
	for _, f := range mutatedGenomeFields {
		addRequired(f)
	}

	// Stale-allowlist check: an exemption for a field we no longer guard is
	// dead weight occupying the cap.
	for field := range inertAllowlist {
		if !requiredSet[field] {
			t.Errorf("inertAllowlist entry %q is not an evolvable parameter (not in any param struct or mutatedGenomeFields); remove the stale exemption", field)
		}
	}

	consumed := collectConsumedSelectors(t)
	for _, field := range required {
		if _, exempt := inertAllowlist[field]; exempt {
			continue
		}
		if !consumed[field] {
			t.Errorf("evolvable parameter %q is never read in pkg/skeleton, pkg/sim, pkg/fitness, or pkg/mechanic (non-test files): it is inert -- evolution mutates it but gameplay ignores it (dd-027 class). Wire it into a runner/fitness or delete it", field)
		}
	}
}

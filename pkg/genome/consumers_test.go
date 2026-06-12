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
// file of the consuming packages. For a field name owned by a SINGLE param
// struct that bare match is sound. When two param structs share a field name
// (RoundsPerGame lives in both SheddingParams and TrickTakingParams since
// Task 22), a bare match could credit one struct's read to the other, so
// colliding fields are qualified by their access chain instead: each owner
// must be read through its unique Genome accessor field --
// `.Shedding.RoundsPerGame` for SheddingParams, `.TrickTaking.RoundsPerGame`
// for TrickTakingParams. The chain pins the receiver type without go/types
// machinery because Genome.Shedding/.TrickTaking/.Rummy are the only fields
// with those names anywhere in the consuming packages
// (TestGenomeAccessorsMatchGenome anchors the accessor map to the real
// Genome struct). The one cost: consuming code must read a colliding field
// through the full chain at least once (params := g.Shedding;
// params.RoundsPerGame alone will not register) -- an acceptable discipline
// for a tripwire. This proves "read somewhere", not "semantically affects
// outcomes"; the calibration/evolvability checks are the semantic
// complement.

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

// genomeAccessor maps each param struct to the Genome field through which all
// consuming code reaches it. Used to qualify field names that collide across
// param structs (see the package comment). TestGenomeAccessorsMatchGenome
// keeps this map anchored to the real Genome struct.
var genomeAccessor = map[string]string{
	"SheddingParams":    "Shedding",
	"TrickTakingParams": "TrickTaking",
	"RummyParams":       "Rummy",
}

// TestGenomeAccessorsMatchGenome verifies the qualification anchor: every
// param struct has exactly the Genome accessor field genomeAccessor claims,
// with the matching pointer type. If Genome is ever restructured so param
// structs are reached another way, the chain-qualified matching below is no
// longer sound and must be reworked (go/types is the heavyweight fallback).
func TestGenomeAccessorsMatchGenome(t *testing.T) {
	gt := reflect.TypeOf(Genome{})
	for _, st := range paramStructTypes() {
		accessor, ok := genomeAccessor[st.Name()]
		if !ok {
			t.Errorf("param struct %s has no genomeAccessor entry", st.Name())
			continue
		}
		f, ok := gt.FieldByName(accessor)
		if !ok {
			t.Errorf("Genome has no field %q (claimed accessor for %s)", accessor, st.Name())
			continue
		}
		if f.Type.Kind() != reflect.Ptr || f.Type.Elem() != st {
			t.Errorf("Genome.%s has type %s, want *%s", accessor, f.Type, st.Name())
		}
	}
}

// collectConsumedSelectors parses every non-test Go file under the consuming
// package trees and returns two sets:
//
//   - bare: all ast.SelectorExpr Sel names ("RoundsPerGame")
//   - qualified: two-step chains where X is itself a selector
//     ("Shedding.RoundsPerGame" for g.Shedding.RoundsPerGame)
//
// Test files are excluded: a parameter read only by a test is still inert
// in production.
func collectConsumedSelectors(t *testing.T) (bare, qualified map[string]bool) {
	t.Helper()
	fset := token.NewFileSet()
	bare = make(map[string]bool)
	qualified = make(map[string]bool)
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
				sel, ok := n.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				bare[sel.Sel.Name] = true
				if x, ok := sel.X.(*ast.SelectorExpr); ok {
					qualified[x.Sel.Name+"."+sel.Sel.Name] = true
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
	return bare, qualified
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

	// Build the required-field set. Param-struct fields track their owning
	// structs so name collisions across structs switch to qualified matching.
	fieldOwners := map[string][]string{} // field name -> owning param structs
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
				fieldOwners[f.Name] = append(fieldOwners[f.Name], st.Name())
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

	bare, qualified := collectConsumedSelectors(t)
	for _, field := range required {
		if _, exempt := inertAllowlist[field]; exempt {
			continue
		}
		owners := fieldOwners[field]
		if len(owners) > 1 {
			// Colliding field name: a bare match could credit one struct's
			// read to the other, so EVERY owner must be read through its
			// unique Genome accessor chain (e.g. g.Shedding.RoundsPerGame).
			for _, owner := range owners {
				accessor := genomeAccessor[owner]
				if accessor == "" {
					t.Errorf("param struct %q has no genomeAccessor entry; cannot qualify colliding field %q", owner, field)
					continue
				}
				if !qualified[accessor+"."+field] {
					t.Errorf("evolvable parameter %s.%s is never read via .%s.%s in pkg/skeleton, pkg/sim, pkg/fitness, or pkg/mechanic (non-test files): it is inert for %s -- evolution mutates it but gameplay ignores it (dd-027 class). Wire it into a runner/fitness or delete it (colliding field names require the full accessor chain at least once)",
						owner, field, accessor, field, owner)
				}
			}
			continue
		}
		if !bare[field] {
			t.Errorf("evolvable parameter %q is never read in pkg/skeleton, pkg/sim, pkg/fitness, or pkg/mechanic (non-test files): it is inert -- evolution mutates it but gameplay ignores it (dd-027 class). Wire it into a runner/fitness or delete it", field)
		}
	}
}

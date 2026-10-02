package judge

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/grammar"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

// BuildGrammarDossier produces the BLIND dossier markdown for one grammar
// composition -- the generative-grammar counterpart of BuildDossier. It contains
// ONLY the natural-language rulebook (no grammar internals, no composition key),
// two greedy-vs-greedy sample traces run through the grammar adapter, and a
// termination note. No fitness/metrics/novelty/true-name. id is the neutral code.
func BuildGrammarDossier(spec grammar.GameSpec, id string) (string, error) {
	g := grammar.SpecGenome(spec)
	runner := grammar.Adapter{Spec: spec}
	ai := fitness.GetGreedyAI(g)

	var b strings.Builder
	// Same neutralizer as every v2 dossier (BuildDossier): the grammar rulebook is
	// written name-blind at the source (grammar.TestRulebookIsNameBlind), and this
	// keeps the blind surface on ONE scrubber should either side drift.
	b.WriteString(neutralizeRulebook(spec.Rulebook(id)))
	b.WriteString("\n---\n\n")

	const traceN = 400
	traceRes := sim.RunBatch(g, runner, ai, traceN, dossierSeed+1)
	b.WriteString("## Sample Game Traces\n\n")
	b.WriteString("Two complete games played by identical automated players (greedy strategy on both sides). Each line is one game event in order, so you can see who acts and when.\n\n")
	picked := pickDistinctCompleted(traceRes, 2)
	if len(picked) == 0 {
		b.WriteString("_No games completed in the sampled batch within the turn cap._\n\n")
	}
	for n, idx := range picked {
		fmt.Fprintf(&b, "### Game %d\n\n", n+1)
		b.WriteString(renderTrace(traceRes.AllEvents[idx]))
		fmt.Fprintf(&b, "\n**Winner:** Player %d\n\n", traceRes.AllWinners[idx])
	}

	// Termination: the grammar guarantees termination by construction (never-empty
	// move set + a runner-level deadlock end), so report the observed completion
	// and state the guarantee rather than the skeleton-specific reachable-win probe.
	// Disjoint seed range from the trace batch above (RunBatch derives game
	// seeds as base+i, so +2 would replay trace games 1..termN).
	const termN = 200
	termRes := sim.RunBatch(g, runner, ai, termN, dossierSeed+1<<20)
	pct := 0.0
	if termRes.GamesPlayed > 0 {
		pct = 100 * float64(termRes.Completions) / float64(termRes.GamesPlayed)
	}
	b.WriteString("---\n\n## Termination\n\n")
	fmt.Fprintf(&b, "- %.0f%% of %d sampled automated games ended with a winner.\n", pct, termN)
	b.WriteString("- The rules guarantee a game always ends: every position has a legal move, and the game resolves either by its win condition or by an unbreakable-deadlock rule, so it cannot run forever.\n")
	return b.String(), nil
}

// EmitGrammar writes a blind dossier set for grammar compositions: one dossier per
// spec to outDir/<id>.md, plus manifest.json and prompt.md (the shared rubric).
// The answer key's true_name is the composition key, so an out-of-loop judge's
// verdicts can be folded straight into a Composition-keyed JudgeVerdicts table.
func EmitGrammar(specs []grammar.GameSpec, outDir string) (EmitResult, error) {
	if len(specs) == 0 {
		return EmitResult{}, fmt.Errorf("no specs to emit")
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return EmitResult{}, err
	}
	if err := cleanStaleDossiers(outDir); err != nil {
		return EmitResult{}, err
	}
	// Ids are assigned by ascending SHA-256 of the composition key, not in the
	// caller's (enumeration) order: G01..Gnn used to walk the families
	// move-generator by move-generator, so the id alone leaked structure. The
	// digest order is a deterministic pseudo-random permutation of the set --
	// re-emitting the same specs, in any order, reproduces the same ids -- and
	// the answer key is the only id -> composition mapping.
	type item struct {
		spec   grammar.GameSpec
		digest [sha256.Size]byte
	}
	items := make([]item, len(specs))
	for i, spec := range specs {
		items[i] = item{spec: spec, digest: sha256.Sum256([]byte(spec.Composition()))}
	}
	sort.SliceStable(items, func(i, j int) bool {
		return bytes.Compare(items[i].digest[:], items[j].digest[:]) < 0
	})

	res := EmitResult{AnswerKey: map[string]AnswerRec{}, DossierDir: outDir}
	var manifest []ManifestEntry
	for i, it := range items {
		spec := it.spec
		id := fmt.Sprintf("G%02d", i+1)
		res.IDs = append(res.IDs, id)
		dossier, err := BuildGrammarDossier(spec, id)
		if err != nil {
			return EmitResult{}, fmt.Errorf("%s: %w", id, err)
		}
		name := id + ".md"
		if err := os.WriteFile(filepath.Join(outDir, name), []byte(dossier), 0o644); err != nil {
			return EmitResult{}, err
		}
		g := grammar.SpecGenome(spec)
		// HandSize is the spec's real deal, NOT g.HandSize: SpecGenome floors
		// that at 8 so the engine's MaxTurns cap never collapses, which made a
		// 7-card (or 0-card banking) game read "hand size 8".
		manifest = append(manifest, ManifestEntry{
			ID: id, Dossier: name, Skeleton: g.Skeleton.String(), Players: spec.Players, HandSize: spec.Deal,
		})
		res.AnswerKey[id] = AnswerRec{Source: "grammar", TrueName: spec.Composition(), Skeleton: g.Skeleton.String(), Composition: spec.Composition()}
	}
	if err := writeJSON(filepath.Join(outDir, "manifest.json"), manifest); err != nil {
		return EmitResult{}, err
	}
	if err := os.WriteFile(filepath.Join(outDir, "prompt.md"), []byte(PromptRubric), 0o644); err != nil {
		return EmitResult{}, err
	}
	return res, nil
}

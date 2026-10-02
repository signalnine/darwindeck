package judge

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/darwindeck/darwindeck/pkg/evolution"
	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
)

// GameSource describes where a discovered genome came from, for the PRIVATE
// answer key only. It is never written into the blind dossier dir.
type GameSource struct {
	// Path is the genome.json path it was loaded from.
	Path string
	// Source is a coarse category ("champion", "classic", "fixture").
	Source string
	// TrueName is the real game name / champion id.
	TrueName string
	// PriorVerdict is the prior-validation verdict, if known.
	PriorVerdict string
}

// AnswerRec is one record in the private answer-key.json.
type AnswerRec struct {
	Source   string `json:"source"`
	TrueName string `json:"true_name"`
	Skeleton string `json:"skeleton"`
	// Path is the source genome.json, relative to the emit input dir
	// (slash-separated). Dossier ids are a pseudo-random permutation of the
	// input, so this -- not the id's position -- is what links an id back to
	// its genome; true_name alone is only a directory name and two run dirs
	// can both hold a rank01.
	Path string `json:"path,omitempty"`
	// Composition is the genome's verdict-table key (evolution.Composition:
	// "<skeleton>:<sorted borrow mechanic ids>"), so a verdict on this id can
	// be folded into the -judge-verdicts table without re-deriving it.
	Composition            string `json:"composition,omitempty"`
	PriorValidationVerdict string `json:"prior_validation_verdict,omitempty"`
}

// ManifestEntry is one record in the public manifest.json (neutral IDs only).
type ManifestEntry struct {
	ID      string `json:"id"`
	Dossier string `json:"dossier"`
	// Skeleton is NOT written to manifest.json: the manifest sits inside the
	// blind dossier dir, and a skeleton name there is a family label ("casino"
	// is a classic's name outright; "climbing"/"rummy"/"vying" are exactly what
	// the rulebook neutralizer keeps out of the judge's view). The skeleton
	// lives in the private answer key. The field stays on the struct so
	// in-process callers that fill it keep compiling.
	Skeleton string `json:"-"`
	Players  int    `json:"players"`
	HandSize int    `json:"hand_size"`
}

// EmitResult is what Emit returns to the CLI.
type EmitResult struct {
	IDs        []string
	AnswerKey  map[string]AnswerRec
	DossierDir string
	AnswerPath string
}

// emitItem is one discovered genome on its way to a dossier.
type emitItem struct {
	path   string // as discovered (the `sources` key)
	rel    string // relative to the input dir, slash-separated
	g      *genome.Genome
	digest [sha256.Size]byte
}

// Emit discovers all genome.json files under inputDir (recursively), assigns
// neutral IDs G01..GNN, writes one blind dossier per genome to
// outDir/<id>.md, plus outDir/manifest.json and outDir/prompt.md. It also
// returns the PRIVATE answer key (neutral id -> source/true_name/path/...);
// the caller writes that OUTSIDE outDir so it never leaks into the blind set.
//
// ID ORDER: ids are assigned in a pseudo-random order that is a pure function
// of the input set (see idOrder), NOT in sorted-path order. Sorted-path order
// made the id sequence the answer key's order: classics came out alphabetical
// by true name and an evolution run dir came out in fitness-rank order
// (G01 = rank01), so rank and identity could be read off the id. The answer
// key is the only id -> genome mapping; never infer it from the id's position.
//
// Every genome is loaded and every dossier built BEFORE outDir is touched, so
// an unreadable genome fails the emit without first sweeping the previous
// dossier set or leaving a half-written one.
//
// sources is an optional override map keyed by the discovered genome.json
// path: if a path is present, its GameSource fills the answer key; otherwise
// the source is inferred (inferSource).
func Emit(inputDir, outDir string, sources map[string]GameSource) (EmitResult, error) {
	paths, err := discoverGenomes(inputDir)
	if err != nil {
		return EmitResult{}, err
	}
	if len(paths) == 0 {
		return EmitResult{}, fmt.Errorf("no genome.json files found under %s", inputDir)
	}

	items := make([]emitItem, len(paths))
	for i, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return EmitResult{}, err
		}
		var g genome.Genome
		if err := json.Unmarshal(data, &g); err != nil {
			return EmitResult{}, fmt.Errorf("%s: unmarshal: %w", path, err)
		}
		rel, err := filepath.Rel(inputDir, path)
		if err != nil {
			rel = path
		}
		items[i] = emitItem{path: path, rel: filepath.ToSlash(rel), g: &g, digest: sha256.Sum256(data)}
	}
	idOrder(items)

	res := EmitResult{
		AnswerKey:  map[string]AnswerRec{},
		DossierDir: outDir,
	}
	var manifest []ManifestEntry
	dossiers := make([]string, len(items))

	for i, it := range items {
		id := fmt.Sprintf("G%02d", i+1)
		res.IDs = append(res.IDs, id)

		// Answer-key facts are read off the genome BEFORE its id is replaced.
		src, ok := sources[it.path]
		if !ok {
			src = inferSource(it.path, it.g)
		}
		res.AnswerKey[id] = AnswerRec{
			Source:                 src.Source,
			TrueName:               src.TrueName,
			Skeleton:               it.g.Skeleton.String(),
			Path:                   it.rel,
			Composition:            evolution.Composition(it.g),
			PriorValidationVerdict: src.PriorVerdict,
		}

		// CRITICAL: set the neutral ID FIRST so the rulebook title is neutral
		// and no original ID leaks into the dossier.
		it.g.ID = id

		dossier, err := BuildDossier(it.g)
		if err != nil {
			return EmitResult{}, fmt.Errorf("%s (%s): %w", id, it.path, err)
		}
		dossiers[i] = dossier

		manifest = append(manifest, ManifestEntry{
			ID:       id,
			Dossier:  id + ".md",
			Players:  it.g.Players,
			HandSize: it.g.HandSize,
		})
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return EmitResult{}, err
	}
	if err := cleanStaleDossiers(outDir); err != nil {
		return EmitResult{}, err
	}
	for i, id := range res.IDs {
		if err := os.WriteFile(filepath.Join(outDir, id+".md"), []byte(dossiers[i]), 0o644); err != nil {
			return EmitResult{}, err
		}
	}
	if err := writeJSON(filepath.Join(outDir, "manifest.json"), manifest); err != nil {
		return EmitResult{}, err
	}
	if err := os.WriteFile(filepath.Join(outDir, "prompt.md"), []byte(PromptRubric), 0o644); err != nil {
		return EmitResult{}, err
	}

	return res, nil
}

// idOrder puts items in the order their neutral ids are assigned: ascending
// SHA-256 of each genome.json's bytes. That is a seeded permutation of the
// input in the sense that matters -- pseudo-random with respect to path, name,
// rank and fitness, and a pure function of the input SET, so re-emitting the
// same genomes reproduces the same ids (and adding a genome does not reshuffle
// the relative order of the others). Byte-identical files tie-break on path.
func idOrder(items []emitItem) {
	sort.SliceStable(items, func(i, j int) bool {
		if c := bytes.Compare(items[i].digest[:], items[j].digest[:]); c != 0 {
			return c < 0
		}
		return items[i].rel < items[j].rel
	})
}

// discoverGenomes finds genome.json files under root, recursively, returning a
// stable, deterministic, sorted list of paths. A run-dir's games/ subdir is
// handled for free by the recursive walk.
func discoverGenomes(root string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Base(p) == "genome.json" {
			paths = append(paths, p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}

// inferSource derives the answer-key source for a genome the caller supplied
// no explicit GameSource for. A genome that IS one of the built-in classic
// seeds (compared structurally -- its id, generation and fitness stamps are
// ignored) is "classic" with the seed's own name: every genome used to be
// labeled "champion", including the classics a judged set is anchored on.
// Anything else is best-effort: the parent directory name becomes the
// true_name and the source is "champion".
func inferSource(path string, g *genome.Genome) GameSource {
	if name, ok := classicName(g); ok {
		return GameSource{Path: path, Source: "classic", TrueName: name}
	}
	return GameSource{
		Path:     path,
		Source:   "champion",
		TrueName: filepath.Base(filepath.Dir(path)),
	}
}

// classicName reports whether g is rule-for-rule one of seeds.All() and, if
// so, which.
func classicName(g *genome.Genome) (string, bool) {
	want := ruleFingerprint(g)
	if want == "" {
		return "", false
	}
	for _, s := range seeds.All() {
		if ruleFingerprint(s) == want {
			return s.ID, true
		}
	}
	return "", false
}

// ruleFingerprint serializes a genome's RULES: everything except the identity
// and publication stamps that differ between a seed and a saved copy of it.
func ruleFingerprint(g *genome.Genome) string {
	c := g.Clone()
	if c == nil {
		return ""
	}
	c.ID, c.Description, c.Generation = "", "", 0
	c.Fitness, c.SharedFitness = 0, 0
	c.VetoStable, c.StableEvals = false, ""
	data, err := json.Marshal(c)
	if err != nil {
		return ""
	}
	return string(data)
}

func loadGenome(path string) (*genome.Genome, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var g genome.Genome
	if err := json.Unmarshal(data, &g); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return &g, nil
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// WriteAnswerKey writes the private answer key to path, creating parent dirs.
// The caller MUST keep path OUTSIDE the dossier dir and should have vetted it
// with CheckAnswerKeyPath BEFORE emitting.
func WriteAnswerKey(path string, key map[string]AnswerRec) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return writeJSON(path, key)
}

// DefaultAnswerKeyPath is where the private answer key goes when the caller
// names no path: answer-key.json in the PARENT of the dossier dir. The parent
// is taken lexically when that already lies outside the dossier dir (so
// relative paths stay relative), and from the absolute path otherwise -- the
// lexical parent of "." is ".", which dropped the key INSIDE the blind set for
// `judge emit <in> --out .`.
func DefaultAnswerKeyPath(outDir string) string {
	key := filepath.Join(filepath.Dir(filepath.Clean(outDir)), "answer-key.json")
	if !pathInside(key, outDir) {
		return key
	}
	if abs, err := filepath.Abs(outDir); err == nil {
		return filepath.Join(filepath.Dir(abs), "answer-key.json")
	}
	return key
}

// CheckAnswerKeyPath vets an answer-key path BEFORE anything is emitted. It
// refuses a path inside the dossier dir (the key would be handed to the judge
// with the blind set) and, unless force is set, a path that already exists:
// ids restart at G01 on every emit, so overwriting the key of an earlier set
// silently destroys the only id -> genome mapping of dossiers that may already
// be out for judging (emit + backfill into sibling dirs shared one default
// key and the second silently replaced the first).
func CheckAnswerKeyPath(keyPath, outDir string, force bool) error {
	if pathInside(keyPath, outDir) {
		return fmt.Errorf("answer key %s is inside the dossier dir %s: it would be handed to the judge with the blind set; choose a path outside it", keyPath, outDir)
	}
	if _, err := os.Stat(keyPath); err == nil {
		if !force {
			return fmt.Errorf("answer key %s already exists (it maps the ids of an earlier dossier set); pass -answer-key to write this set's key elsewhere, or -force to overwrite it", keyPath)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("answer key %s: %w", keyPath, err)
	}
	return nil
}

// pathInside reports whether path lies inside dir (or is dir itself).
func pathInside(path, dir string) bool {
	absPath, err1 := filepath.Abs(path)
	absDir, err2 := filepath.Abs(dir)
	if err1 != nil || err2 != nil {
		return false
	}
	rel, err := filepath.Rel(absDir, absPath)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// SortIDsNumeric sorts neutral IDs (G01, G02, ...) numerically -- actually
// numerically, not lexicographically: IDs are G%02d, so widths diverge at 100
// and a string compare put G100 before G11. Non-numeric suffixes fall back to
// the string order.
func SortIDsNumeric(ids []string) {
	num := func(id string) (int, bool) {
		n, err := strconv.Atoi(strings.TrimPrefix(id, "G"))
		return n, err == nil
	}
	sort.Slice(ids, func(i, j int) bool {
		ni, iok := num(ids[i])
		nj, jok := num(ids[j])
		if iok && jok {
			return ni < nj
		}
		return strings.Compare(ids[i], ids[j]) < 0
	})
}

// dossierFile matches exactly the files Emit / EmitGrammar write as dossiers.
var dossierFile = regexp.MustCompile(`^G\d+\.md$`)

// cleanStaleDossiers removes the DOSSIERS already in outDir before a fresh
// emit. Emit numbers dossiers G01..GNN from scratch, so re-emitting a SMALLER
// set into a dir that held a larger one left stale G(N+1)+.md files beside a
// manifest that no longer listed them -- a judge globbing *.md then scored
// dossiers whose IDs resolve to the WRONG answer-key entries (a silently
// corrupted blind experiment).
//
// Only G<digits>.md is removed. The sweep once took every *.md, which deleted
// a run's README.md when the run dir was also the --out dir, and the
// judged-report.md that `judge rank` writes into the dossier dir by default.
func cleanStaleDossiers(outDir string) error {
	entries, err := os.ReadDir(outDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !dossierFile.MatchString(e.Name()) {
			continue
		}
		if err := os.Remove(filepath.Join(outDir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

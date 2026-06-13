package judge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/darwindeck/darwindeck/pkg/genome"
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
	Source                 string `json:"source"`
	TrueName               string `json:"true_name"`
	Skeleton               string `json:"skeleton"`
	PriorValidationVerdict string `json:"prior_validation_verdict,omitempty"`
}

// ManifestEntry is one record in the public manifest.json (neutral IDs only).
type ManifestEntry struct {
	ID       string `json:"id"`
	Dossier  string `json:"dossier"`
	Skeleton string `json:"skeleton"`
	Players  int    `json:"players"`
	HandSize int    `json:"hand_size"`
}

// EmitResult is what Emit returns to the CLI.
type EmitResult struct {
	IDs         []string
	AnswerKey   map[string]AnswerRec
	DossierDir  string
	AnswerPath  string
}

// Emit discovers all genome.json files under inputDir (recursively), assigns
// neutral IDs G01..G0N in a stable order, writes one blind dossier per genome
// to outDir/<id>.md, plus outDir/manifest.json and outDir/prompt.md. It also
// returns the PRIVATE answer key (neutral id -> source/true_name/...); the
// caller writes that OUTSIDE outDir so it never leaks into the blind set.
//
// sources is an optional override map keyed by absolute genome.json path: if a
// path is present, its GameSource fills the answer key; otherwise the source is
// inferred from the path (champion run dirs) or left generic.
func Emit(inputDir, outDir string, sources map[string]GameSource) (EmitResult, error) {
	paths, err := discoverGenomes(inputDir)
	if err != nil {
		return EmitResult{}, err
	}
	if len(paths) == 0 {
		return EmitResult{}, fmt.Errorf("no genome.json files found under %s", inputDir)
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return EmitResult{}, err
	}

	res := EmitResult{
		AnswerKey:  map[string]AnswerRec{},
		DossierDir: outDir,
	}
	var manifest []ManifestEntry

	for i, path := range paths {
		g, err := loadGenome(path)
		if err != nil {
			return EmitResult{}, fmt.Errorf("%s: %w", path, err)
		}
		id := fmt.Sprintf("G%02d", i+1)
		res.IDs = append(res.IDs, id)

		// CRITICAL: set the neutral ID FIRST so the rulebook title is neutral
		// and no original ID leaks into the dossier.
		g.ID = id

		dossier, err := BuildDossier(g)
		if err != nil {
			return EmitResult{}, fmt.Errorf("%s: %w", id, err)
		}
		dossierName := id + ".md"
		if err := os.WriteFile(filepath.Join(outDir, dossierName), []byte(dossier), 0o644); err != nil {
			return EmitResult{}, err
		}

		manifest = append(manifest, ManifestEntry{
			ID:       id,
			Dossier:  dossierName,
			Skeleton: g.Skeleton.String(),
			Players:  g.Players,
			HandSize: g.HandSize,
		})

		src, ok := sources[path]
		if !ok {
			src = inferSource(path)
		}
		res.AnswerKey[id] = AnswerRec{
			Source:                 src.Source,
			TrueName:               src.TrueName,
			Skeleton:               g.Skeleton.String(),
			PriorValidationVerdict: src.PriorVerdict,
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

// inferSource derives a coarse answer-key source from a genome.json path when
// the caller supplied no explicit GameSource. It is best-effort: the parent
// directory name becomes the true_name and the source is "champion".
func inferSource(path string) GameSource {
	parent := filepath.Base(filepath.Dir(path))
	return GameSource{
		Path:     path,
		Source:   "champion",
		TrueName: parent,
	}
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
// The caller MUST keep path OUTSIDE the dossier dir.
func WriteAnswerKey(path string, key map[string]AnswerRec) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return writeJSON(path, key)
}

// SortIDsNumeric sorts neutral IDs (G01, G02, ...) numerically. Used by callers
// that build ordered displays.
func SortIDsNumeric(ids []string) {
	sort.Slice(ids, func(i, j int) bool {
		return strings.Compare(ids[i], ids[j]) < 0
	})
}

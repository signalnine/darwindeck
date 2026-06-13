package judge

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// Verdict is one judge ruling for one dossier at one repetition. It is the
// ingestion shape for `judge rank`.
type Verdict struct {
	ID                  string  `json:"id"`
	Rep                 int     `json:"rep"`
	Playable            bool    `json:"playable"`
	Quality             string  `json:"quality"`               // publishable | borderline | degenerate
	DegenerateMechanism string  `json:"degenerate_mechanism"`  // why, if degenerate
	Novelty             string  `json:"novelty"`               // novel | variant_of_known
	RediscoveryName     string  `json:"rediscovery_name"`      // closest classic family
	Confidence          float64 `json:"confidence"`
	Reason              string  `json:"reason"`
}

// AggregatedVerdict is the majority-of-3 result for one dossier id.
type AggregatedVerdict struct {
	ID                  string  `json:"id"`
	Quality             string  `json:"quality"`              // majority quality
	Novelty             string  `json:"novelty"`              // majority novelty
	RediscoveryName     string  `json:"rediscovery_name"`     // majority rediscovery label (when variant_of_known)
	Playable            bool    `json:"playable"`             // majority playable
	Confidence          float64 `json:"confidence"`           // mean confidence
	Reason              string  `json:"reason"`               // representative one-liner
	DegenerateMechanism string  `json:"degenerate_mechanism"` // representative, when degenerate
	Demoted             bool    `json:"demoted"`              // true iff a rediscovery demotion applied
	Votes               int     `json:"votes"`                // number of verdicts aggregated
}

// qualityRank orders quality bands for ranking: publishable best, degenerate
// worst. Higher is better.
func qualityRank(q string) int {
	switch q {
	case "publishable":
		return 3
	case "borderline":
		return 2
	case "degenerate":
		return 1
	default:
		return 0
	}
}

// Aggregate groups verdicts by id and computes the majority-of-3 quality and
// novelty per id. Majority is the most-voted value; ties break toward the
// WORSE quality (conservative) and toward "variant_of_known" for novelty (a
// rediscovery suspicion is not discarded on a tie). Confidence is the mean.
// Rediscovery flagging: if majority novelty is variant_of_known, the game is
// DEMOTED one band (publishable->borderline, borderline->degenerate) and
// labeled with the majority rediscovery name; degenerate stays degenerate.
func Aggregate(verdicts []Verdict) []AggregatedVerdict {
	byID := map[string][]Verdict{}
	var order []string
	for _, v := range verdicts {
		if _, ok := byID[v.ID]; !ok {
			order = append(order, v.ID)
		}
		byID[v.ID] = append(byID[v.ID], v)
	}

	var out []AggregatedVerdict
	for _, id := range order {
		vs := byID[id]
		agg := AggregatedVerdict{ID: id, Votes: len(vs)}

		// Majority quality (tie -> worse band).
		agg.Quality = majorityQuality(vs)
		// Majority novelty (tie -> variant_of_known).
		agg.Novelty = majorityNovelty(vs)
		// Majority playable (tie -> true: do not condemn on a split).
		agg.Playable = majorityPlayable(vs)
		// Representative rediscovery name: most common non-empty label.
		agg.RediscoveryName = majorityRediscoveryName(vs)
		// Mean confidence.
		agg.Confidence = meanConfidence(vs)
		// Representative reason + mechanism: take from a verdict matching the
		// majority quality (first such), else the first verdict.
		agg.Reason, agg.DegenerateMechanism = representativeText(vs, agg.Quality)

		// Rediscovery demotion: a variant_of_known game drops one band.
		if agg.Novelty == "variant_of_known" {
			if demoted := demoteQuality(agg.Quality); demoted != agg.Quality {
				agg.Quality = demoted
				agg.Demoted = true
			} else {
				// Already at the floor (degenerate): still flagged as a
				// rediscovery, just not demotable further.
				agg.Demoted = true
			}
		}

		out = append(out, agg)
	}
	return out
}

// Rank orders aggregated verdicts by judged quality (publishable > borderline >
// degenerate), tie-break by playable (playable first) then confidence
// (descending), then id for determinism.
func Rank(aggs []AggregatedVerdict) []AggregatedVerdict {
	ranked := append([]AggregatedVerdict(nil), aggs...)
	sort.SliceStable(ranked, func(i, j int) bool {
		a, b := ranked[i], ranked[j]
		if qa, qb := qualityRank(a.Quality), qualityRank(b.Quality); qa != qb {
			return qa > qb
		}
		if a.Playable != b.Playable {
			return a.Playable // playable first
		}
		if a.Confidence != b.Confidence {
			return a.Confidence > b.Confidence
		}
		return a.ID < b.ID
	})
	return ranked
}

func majorityQuality(vs []Verdict) string {
	counts := map[string]int{}
	for _, v := range vs {
		counts[v.Quality]++
	}
	// Pick max count; tie -> worse band (lower qualityRank).
	best := ""
	for q, c := range counts {
		if best == "" {
			best = q
			continue
		}
		if c > counts[best] || (c == counts[best] && qualityRank(q) < qualityRank(best)) {
			best = q
		}
	}
	if best == "" {
		return "borderline"
	}
	return best
}

func majorityNovelty(vs []Verdict) string {
	novel, variant := 0, 0
	for _, v := range vs {
		switch v.Novelty {
		case "variant_of_known":
			variant++
		case "novel":
			novel++
		}
	}
	if variant >= novel && variant > 0 {
		// Tie or majority -> variant_of_known (a rediscovery suspicion is not
		// dropped on a split).
		return "variant_of_known"
	}
	if novel > 0 {
		return "novel"
	}
	return "novel"
}

func majorityPlayable(vs []Verdict) bool {
	yes, no := 0, 0
	for _, v := range vs {
		if v.Playable {
			yes++
		} else {
			no++
		}
	}
	return yes >= no // tie -> playable
}

func majorityRediscoveryName(vs []Verdict) string {
	counts := map[string]int{}
	var order []string
	for _, v := range vs {
		name := strings.TrimSpace(v.RediscoveryName)
		if name == "" {
			continue
		}
		if _, ok := counts[name]; !ok {
			order = append(order, name)
		}
		counts[name]++
	}
	best := ""
	bestCount := 0
	for _, name := range order {
		if counts[name] > bestCount {
			best = name
			bestCount = counts[name]
		}
	}
	return best
}

func meanConfidence(vs []Verdict) float64 {
	if len(vs) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vs {
		sum += v.Confidence
	}
	return sum / float64(len(vs))
}

func representativeText(vs []Verdict, quality string) (reason, mechanism string) {
	for _, v := range vs {
		if v.Quality == quality {
			return v.Reason, v.DegenerateMechanism
		}
	}
	if len(vs) > 0 {
		return vs[0].Reason, vs[0].DegenerateMechanism
	}
	return "", ""
}

func demoteQuality(q string) string {
	switch q {
	case "publishable":
		return "borderline"
	case "borderline":
		return "degenerate"
	default:
		return q // degenerate stays degenerate
	}
}

// LoadVerdicts reads a verdicts.json file (a JSON array of Verdict).
func LoadVerdicts(path string) ([]Verdict, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var vs []Verdict
	if err := json.Unmarshal(data, &vs); err != nil {
		return nil, fmt.Errorf("parse verdicts: %w", err)
	}
	return vs, nil
}

// RenderReport produces the judged-report.md markdown: a ranked table of
// neutral id, majority quality, novelty / rediscovery name, playable, and a
// one-line reason.
func RenderReport(ranked []AggregatedVerdict) string {
	var b strings.Builder
	b.WriteString("# Judged Report\n\n")
	b.WriteString("Games re-ranked by judged quality (majority of 3 verdicts per game). ")
	b.WriteString("Rediscoveries (majority novelty = variant_of_known) are demoted one band and labeled.\n\n")
	b.WriteString("| Rank | ID | Quality | Novelty | Playable | Confidence | Note |\n")
	b.WriteString("|------|----|---------|---------|----------|-----------|------|\n")
	for i, a := range ranked {
		novelty := a.Novelty
		if a.Novelty == "variant_of_known" && a.RediscoveryName != "" {
			novelty = "variant: " + a.RediscoveryName
		}
		note := a.Reason
		if a.Demoted {
			note = "[demoted: rediscovery] " + note
		}
		note = strings.ReplaceAll(note, "|", "/")
		b.WriteString(fmt.Sprintf("| %d | %s | %s | %s | %v | %.2f | %s |\n",
			i+1, a.ID, a.Quality, novelty, a.Playable, a.Confidence, note))
	}
	b.WriteString("\n")
	return b.String()
}

// WriteReport writes judged-report.md and judged.json for a ranked result set.
func WriteReport(reportPath, jsonPath string, ranked []AggregatedVerdict) error {
	if err := os.WriteFile(reportPath, []byte(RenderReport(ranked)), 0o644); err != nil {
		return err
	}
	return writeJSON(jsonPath, ranked)
}

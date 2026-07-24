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
	Quality             string  `json:"quality"`              // publishable | borderline | degenerate
	DegenerateMechanism string  `json:"degenerate_mechanism"` // why, if degenerate
	Novelty             string  `json:"novelty"`              // novel | variant_of_known
	RediscoveryName     string  `json:"rediscovery_name"`     // closest classic family
	Confidence          float64 `json:"confidence"`
	Reason              string  `json:"reason"`
}

// confidenceLabels maps the qualitative confidence labels an LLM judge
// naturally emits onto the numeric scale Confidence stores. The midpoints keep
// the same ordering and rough magnitude as the numeric form (0.0-1.0).
var confidenceLabels = map[string]float64{
	"low":      0.3,
	"medium":   0.6,
	"moderate": 0.6,
	"high":     0.9,
}

// UnmarshalJSON accepts the `confidence` field as either a JSON number
// (0.0-1.0) or a qualitative string label ("low"/"medium"/"high"), which is
// the shape an LLM judge naturally produces. Everything else unmarshals via the
// struct tags. Unknown string labels are tolerated as 0 rather than erroring,
// so one malformed field does not reject an entire verdict batch.
func (v *Verdict) UnmarshalJSON(data []byte) error {
	type verdictAlias Verdict
	aux := struct {
		Confidence json.RawMessage `json:"confidence"`
		*verdictAlias
	}{verdictAlias: (*verdictAlias)(v)}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	v.Confidence = parseConfidence(aux.Confidence)
	return nil
}

// parseConfidence interprets a raw confidence value as a float: a JSON number
// passes through; a JSON string is mapped via confidenceLabels (case- and
// space-insensitive). Empty or unrecognized values yield 0.
func parseConfidence(raw json.RawMessage) float64 {
	raw = json.RawMessage(strings.TrimSpace(string(raw)))
	if len(raw) == 0 || string(raw) == "null" {
		return 0
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return f
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if val, ok := confidenceLabels[strings.ToLower(strings.TrimSpace(s))]; ok {
			return val
		}
	}
	return 0
}

// AggregatedVerdict is the majority-of-3 result for one dossier id.
//
// Quality (is the game broken?) and Novelty (is it a known game?) are
// ORTHOGONAL. Quality always carries the judges' TRUE majority band and is
// never altered by novelty: "degenerate" means broken/unfun and must never be
// produced by a novelty demotion. A rediscovery only sinks a game in the RANK
// ORDER (see Rank), never in its displayed quality.
type AggregatedVerdict struct {
	ID                  string  `json:"id"`
	Quality             string  `json:"quality"`              // majority quality (NEVER altered by novelty)
	Novelty             string  `json:"novelty"`              // majority novelty
	RediscoveryName     string  `json:"rediscovery_name"`     // majority rediscovery label (when variant_of_known)
	Playable            bool    `json:"playable"`             // majority playable
	Confidence          float64 `json:"confidence"`           // mean confidence
	Reason              string  `json:"reason"`               // representative one-liner
	DegenerateMechanism string  `json:"degenerate_mechanism"` // representative, when degenerate
	Rediscovery         bool    `json:"rediscovery"`          // true iff majority novelty = variant_of_known (advisory; sinks rank only)
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
//
// Quality and novelty are ORTHOGONAL: the aggregated Quality is ALWAYS the
// judges' true majority band and is NEVER mutated by novelty. A majority
// variant_of_known only sets the advisory Rediscovery flag (and the
// rediscovery name); it does NOT relabel quality. The demotion is purely an
// ordering concern handled in Rank.
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

		// Rediscovery flag: a majority variant_of_known game is flagged for the
		// report and sinks in Rank. This NEVER changes agg.Quality -- quality
		// and novelty are orthogonal.
		agg.Rediscovery = agg.Novelty == "variant_of_known"

		out = append(out, agg)
	}
	return out
}

// Rank orders aggregated verdicts by judged quality (publishable > borderline >
// degenerate) FIRST. Within an equal quality band a genuinely novel game ranks
// ahead of a rediscovery/variant (the legitimate "advisory rediscovery
// demoter": it REORDERS, it never relabels quality). Remaining ties break by
// playable (playable first), then confidence (descending), then id for
// determinism.
func Rank(aggs []AggregatedVerdict) []AggregatedVerdict {
	ranked := append([]AggregatedVerdict(nil), aggs...)
	sort.SliceStable(ranked, func(i, j int) bool {
		a, b := ranked[i], ranked[j]
		if qa, qb := qualityRank(a.Quality), qualityRank(b.Quality); qa != qb {
			return qa > qb
		}
		// Within equal quality: novel ahead of rediscovery/variant.
		if a.Rediscovery != b.Rediscovery {
			return !a.Rediscovery // non-rediscovery (novel) first
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
	// No recognized novelty label at all: refuse to certify novelty on zero
	// signal. The old default here was "novel" -- a malformed verdict set got
	// the most generous possible claim for free. Conservative, matching the
	// tie-break above.
	return "variant_of_known"
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
	// Normalize the LLM-judge label vocabulary at the input boundary: judges
	// drift in case/whitespace ("Publishable"), and an unrecognized quality
	// silently ranked BELOW degenerate (qualityRank default 0) while an
	// unrecognized novelty counted toward the generous "novel" default. Warn
	// on labels that stay unrecognized after normalization -- a malformed
	// verdict must be visible, not silently mis-ranked.
	for i := range vs {
		vs[i].Quality = strings.ToLower(strings.TrimSpace(vs[i].Quality))
		vs[i].Novelty = strings.ToLower(strings.TrimSpace(vs[i].Novelty))
		switch vs[i].Quality {
		case "publishable", "borderline", "degenerate":
		default:
			fmt.Fprintf(os.Stderr, "warning: verdict %s rep %d: unrecognized quality %q (ranks below degenerate)\n", vs[i].ID, vs[i].Rep, vs[i].Quality)
		}
		switch vs[i].Novelty {
		case "novel", "variant_of_known":
		default:
			fmt.Fprintf(os.Stderr, "warning: verdict %s rep %d: unrecognized novelty %q (ignored by the majority)\n", vs[i].ID, vs[i].Rep, vs[i].Novelty)
		}
	}
	return vs, nil
}

// RenderReport produces the judged-report.md markdown: a ranked table of
// neutral id, the judges' TRUE majority quality, novelty / rediscovery name,
// playable, and a one-line reason.
//
// Quality and Novelty are orthogonal columns: Quality reports the judges'
// majority band (publishable / borderline / degenerate) verbatim, and is NEVER
// changed by novelty. Rediscoveries are labeled in the Novelty column and sink
// in the rank order, but their displayed quality is untouched.
func RenderReport(ranked []AggregatedVerdict) string {
	var b strings.Builder
	b.WriteString("# Judged Report\n\n")
	b.WriteString("Games re-ranked by judged quality (majority of 3 verdicts per game). ")
	b.WriteString("Quality is the judges' true majority band and is NOT altered by novelty. ")
	b.WriteString("Rediscoveries (majority novelty = variant_of_known) are labeled and sink within their quality band, ")
	b.WriteString("but keep their true quality.\n\n")
	b.WriteString("| Rank | ID | Quality | Novelty | Playable | Confidence | Note |\n")
	b.WriteString("|------|----|---------|---------|----------|-----------|------|\n")
	for i, a := range ranked {
		novelty := a.Novelty
		if a.Novelty == "variant_of_known" && a.RediscoveryName != "" {
			novelty = "variant: " + a.RediscoveryName
		}
		note := a.Reason
		if a.Rediscovery {
			note = "[rediscovery: ranked below novel] " + note
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

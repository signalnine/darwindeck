package judge

import (
	"math"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// TestVerdictConfidenceAcceptsStringOrNumber verifies that the `confidence`
// field unmarshals from both a JSON number and a qualitative string label, the
// shape an LLM judge naturally emits.
func TestVerdictConfidenceAcceptsStringOrNumber(t *testing.T) {
	cases := []struct {
		json string
		want float64
	}{
		{`{"id":"G01","confidence":0.85}`, 0.85},
		{`{"id":"G01","confidence":"high"}`, 0.9},
		{`{"id":"G01","confidence":"medium"}`, 0.6},
		{`{"id":"G01","confidence":"low"}`, 0.3},
		{`{"id":"G01","confidence":"HIGH"}`, 0.9},
		// A number in quotes is still a number: it used to fall through the
		// label table to 0, so a unanimous high-confidence game lost every
		// confidence tie-break without a word.
		{`{"id":"G01","confidence":"0.9"}`, 0.9},
		{`{"id":"G01","confidence":" 0.75 "}`, 0.75},
		{`{"id":"G01"}`, 0.0},
		{`{"id":"G01","confidence":"bogus"}`, 0.0},
	}
	for _, c := range cases {
		var v Verdict
		if err := v.UnmarshalJSON([]byte(c.json)); err != nil {
			t.Fatalf("unmarshal %s: %v", c.json, err)
		}
		if math.Abs(v.Confidence-c.want) > 1e-9 {
			t.Errorf("confidence for %s = %v, want %v", c.json, v.Confidence, c.want)
		}
		if v.ID != "G01" {
			t.Errorf("id for %s = %q, want G01", c.json, v.ID)
		}
	}
}

// TestAggregateMajorityWithSplit checks majority-of-3 aggregation including a
// 2-1 split: two "borderline" + one "publishable" -> borderline.
func TestAggregateMajorityWithSplit(t *testing.T) {
	verdicts := []Verdict{
		{ID: "G01", Rep: 1, Quality: "borderline", Novelty: "novel", Playable: true, Confidence: 0.6},
		{ID: "G01", Rep: 2, Quality: "publishable", Novelty: "novel", Playable: true, Confidence: 0.9},
		{ID: "G01", Rep: 3, Quality: "borderline", Novelty: "novel", Playable: true, Confidence: 0.6},
	}
	aggs := Aggregate(verdicts)
	if len(aggs) != 1 {
		t.Fatalf("got %d aggregated verdicts, want 1", len(aggs))
	}
	a := aggs[0]
	if a.Quality != "borderline" {
		t.Errorf("2-1 split quality = %q, want borderline", a.Quality)
	}
	if a.Votes != 3 {
		t.Errorf("votes = %d, want 3", a.Votes)
	}
	if a.Novelty != "novel" {
		t.Errorf("novelty = %q, want novel", a.Novelty)
	}
	// Mean confidence (0.6+0.9+0.6)/3 = 0.7.
	if a.Confidence < 0.69 || a.Confidence > 0.71 {
		t.Errorf("mean confidence = %.3f, want ~0.70", a.Confidence)
	}
}

// TestAggregateTieBreaksToWorseBand verifies a 1-1-1 quality split breaks to
// the worse band (conservative).
func TestAggregateTieBreaksToWorseBand(t *testing.T) {
	verdicts := []Verdict{
		{ID: "G01", Rep: 1, Quality: "publishable", Novelty: "novel"},
		{ID: "G01", Rep: 2, Quality: "borderline", Novelty: "novel"},
		{ID: "G01", Rep: 3, Quality: "degenerate", Novelty: "novel"},
	}
	a := Aggregate(verdicts)[0]
	if a.Quality != "degenerate" {
		t.Errorf("1-1-1 tie quality = %q, want degenerate (worse band)", a.Quality)
	}
}

// TestRankPublishableAboveDegenerate verifies the re-rank ordering.
func TestRankPublishableAboveDegenerate(t *testing.T) {
	verdicts := []Verdict{
		// G01: degenerate majority.
		{ID: "G01", Rep: 1, Quality: "degenerate", Novelty: "novel", Playable: false, Confidence: 0.9},
		{ID: "G01", Rep: 2, Quality: "degenerate", Novelty: "novel", Playable: false, Confidence: 0.9},
		{ID: "G01", Rep: 3, Quality: "borderline", Novelty: "novel", Playable: true, Confidence: 0.5},
		// G02: publishable majority.
		{ID: "G02", Rep: 1, Quality: "publishable", Novelty: "novel", Playable: true, Confidence: 0.8},
		{ID: "G02", Rep: 2, Quality: "publishable", Novelty: "novel", Playable: true, Confidence: 0.8},
		{ID: "G02", Rep: 3, Quality: "borderline", Novelty: "novel", Playable: true, Confidence: 0.5},
	}
	ranked := Rank(Aggregate(verdicts))
	if len(ranked) != 2 {
		t.Fatalf("got %d ranked, want 2", len(ranked))
	}
	if ranked[0].ID != "G02" {
		t.Errorf("rank 1 = %s, want G02 (publishable)", ranked[0].ID)
	}
	if ranked[1].ID != "G01" {
		t.Errorf("rank 2 = %s, want G01 (degenerate)", ranked[1].ID)
	}
}

// TestRediscoveryFlagDoesNotAlterQuality verifies that a majority
// variant_of_known game is FLAGGED as a rediscovery and labeled, but its
// displayed quality is the judges' TRUE majority band -- novelty never demotes
// quality. Quality and novelty are orthogonal.
func TestRediscoveryFlagDoesNotAlterQuality(t *testing.T) {
	verdicts := []Verdict{
		// Publishable quality, majority says variant_of_known. The game must
		// stay publishable (NOT demoted to borderline) and carry the flag+label.
		{ID: "G01", Rep: 1, Quality: "publishable", Novelty: "variant_of_known", RediscoveryName: "Whist-like", Playable: true, Confidence: 0.8},
		{ID: "G01", Rep: 2, Quality: "publishable", Novelty: "variant_of_known", RediscoveryName: "Whist-like", Playable: true, Confidence: 0.8},
		{ID: "G01", Rep: 3, Quality: "publishable", Novelty: "novel", Playable: true, Confidence: 0.7},
	}
	a := Aggregate(verdicts)[0]
	if !a.Rediscovery {
		t.Error("expected rediscovery flag for majority variant_of_known")
	}
	if a.Quality != "publishable" {
		t.Errorf("quality = %q, want publishable (novelty must NOT demote quality)", a.Quality)
	}
	if a.Novelty != "variant_of_known" {
		t.Errorf("novelty = %q, want variant_of_known", a.Novelty)
	}
	if a.RediscoveryName != "Whist-like" {
		t.Errorf("rediscovery name = %q, want Whist-like", a.RediscoveryName)
	}
}

// TestNoveltyNeverProducesDegenerate pins the core invariant: a borderline game
// that is a rediscovery must NEVER be relabeled degenerate by a novelty
// demotion. "degenerate" means broken/unfun and is reserved for the judges'
// quality verdict alone.
func TestNoveltyNeverProducesDegenerate(t *testing.T) {
	verdicts := []Verdict{
		{ID: "G01", Rep: 1, Quality: "borderline", Novelty: "variant_of_known", RediscoveryName: "Crazy Eights-like", Playable: true, Confidence: 0.9},
		{ID: "G01", Rep: 2, Quality: "borderline", Novelty: "variant_of_known", RediscoveryName: "Crazy Eights-like", Playable: true, Confidence: 0.9},
		{ID: "G01", Rep: 3, Quality: "borderline", Novelty: "variant_of_known", RediscoveryName: "Crazy Eights-like", Playable: true, Confidence: 0.9},
	}
	a := Aggregate(verdicts)[0]
	if a.Quality == "degenerate" {
		t.Fatal("novelty demotion must NEVER produce degenerate quality")
	}
	if a.Quality != "borderline" {
		t.Errorf("quality = %q, want borderline (unaltered by novelty)", a.Quality)
	}
}

// TestCrazyEightsClassKeepsQualityButSinksInRank pins the exact Crazy-Eights
// regression: a borderline game that is a faithful rediscovery of a real
// classic (Crazy Eights) must (1) KEEP quality=borderline in the output -- NOT
// be relabeled degenerate -- and (2) still sort BELOW an equal-quality novel
// game. Quality (broken?) and novelty (known?) are orthogonal.
func TestCrazyEightsClassKeepsQualityButSinksInRank(t *testing.T) {
	verdicts := []Verdict{
		// G12-class: a sound-but-thin Crazy Eights rediscovery. Judges' true
		// majority quality is borderline (2 borderline, 1 publishable).
		{ID: "G12", Rep: 1, Quality: "borderline", Novelty: "variant_of_known", RediscoveryName: "Crazy Eights-like", Playable: true, Confidence: 0.9},
		{ID: "G12", Rep: 2, Quality: "publishable", Novelty: "variant_of_known", RediscoveryName: "Crazy Eights-like", Playable: true, Confidence: 0.9},
		{ID: "G12", Rep: 3, Quality: "borderline", Novelty: "variant_of_known", RediscoveryName: "Crazy Eights-like", Playable: true, Confidence: 0.9},
		// GNV: a genuinely novel game of the SAME (borderline) quality band.
		{ID: "GNV", Rep: 1, Quality: "borderline", Novelty: "novel", Playable: true, Confidence: 0.9},
		{ID: "GNV", Rep: 2, Quality: "borderline", Novelty: "novel", Playable: true, Confidence: 0.9},
		{ID: "GNV", Rep: 3, Quality: "borderline", Novelty: "novel", Playable: true, Confidence: 0.9},
	}
	aggs := Aggregate(verdicts)

	var g12 AggregatedVerdict
	for _, a := range aggs {
		if a.ID == "G12" {
			g12 = a
		}
	}
	// (1) Quality column must show the TRUE majority (borderline), never
	//     degenerate from a novelty demotion.
	if g12.Quality != "borderline" {
		t.Errorf("G12 quality = %q, want borderline (Crazy Eights is a real classic, not degenerate)", g12.Quality)
	}
	if !g12.Rediscovery {
		t.Error("G12 should be flagged as a rediscovery")
	}
	if g12.RediscoveryName != "Crazy Eights-like" {
		t.Errorf("G12 rediscovery name = %q, want Crazy Eights-like", g12.RediscoveryName)
	}

	// (2) Within the equal (borderline) band, the novel game sorts above the
	//     rediscovery.
	ranked := Rank(aggs)
	posG12, posGNV := -1, -1
	for i, a := range ranked {
		switch a.ID {
		case "G12":
			posG12 = i
		case "GNV":
			posGNV = i
		}
	}
	if posGNV >= posG12 {
		t.Errorf("expected novel GNV (pos %d) to rank above rediscovery G12 (pos %d)", posGNV, posG12)
	}

	// And the rendered report must NOT show degenerate for G12.
	report := RenderReport(ranked)
	for _, line := range splitLines(report) {
		if contains(line, "| G12 |") && contains(line, "degenerate") {
			t.Errorf("G12 report row labeled degenerate: %s", line)
		}
	}
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// TestRenderReportContainsRanks checks the report renders a ranked table.
func TestRenderReportContainsRanks(t *testing.T) {
	verdicts := []Verdict{
		{ID: "G01", Rep: 1, Quality: "publishable", Novelty: "novel", Playable: true, Confidence: 0.8, Reason: "good"},
		{ID: "G01", Rep: 2, Quality: "publishable", Novelty: "novel", Playable: true, Confidence: 0.8, Reason: "good"},
		{ID: "G01", Rep: 3, Quality: "publishable", Novelty: "novel", Playable: true, Confidence: 0.8, Reason: "good"},
	}
	report := RenderReport(Rank(Aggregate(verdicts)))
	if !contains(report, "| Rank | ID |") {
		t.Error("report missing table header")
	}
	if !contains(report, "G01") {
		t.Error("report missing G01 row")
	}
}

// TestAggregateDedupesDuplicateReps: a judge batch that is retried or merged
// twice repeats a (id, rep) verdict. Counting it twice turned a 1-1 split
// (which breaks to the WORSE band) into a 2-1 majority for the duplicated
// side: publishable x2 (the same rep) + degenerate ranked the game
// publishable at #1. A repetition is one vote; the first occurrence wins.
func TestAggregateDedupesDuplicateReps(t *testing.T) {
	verdicts := []Verdict{
		{ID: "G02", Rep: 1, Quality: "publishable", Novelty: "novel", Playable: true, Confidence: 0.5, Reason: "first"},
		{ID: "G02", Rep: 1, Quality: "publishable", Novelty: "novel", Playable: true, Confidence: 0.5, Reason: "dup"},
		{ID: "G02", Rep: 2, Quality: "degenerate", Novelty: "variant_of_known", Playable: false, Confidence: 0.5, Reason: "x"},
	}
	a := Aggregate(verdicts)[0]
	if a.Votes != 2 {
		t.Errorf("votes = %d, want 2 (rep 1 appears twice but is one vote)", a.Votes)
	}
	if a.Quality != "degenerate" {
		t.Errorf("quality = %q, want degenerate (a real 1-1 split breaks to the worse band)", a.Quality)
	}
	if a.Novelty != "variant_of_known" {
		t.Errorf("novelty = %q, want variant_of_known (1-1 split)", a.Novelty)
	}

	// Verdicts with no rep (0) carry no repetition identity and must never be
	// collapsed into one another.
	repless := []Verdict{
		{ID: "G05", Quality: "publishable", Novelty: "novel"},
		{ID: "G05", Quality: "publishable", Novelty: "novel"},
		{ID: "G05", Quality: "borderline", Novelty: "novel"},
	}
	if a := Aggregate(repless)[0]; a.Votes != 3 || a.Quality != "publishable" {
		t.Errorf("rep-less verdicts: votes=%d quality=%q, want 3 / publishable", a.Votes, a.Quality)
	}
}

// TestVerdictWarningsFlagDuplicatesAndVoteCounts: a duplicated repetition and
// a dossier judged fewer (or more) than 3 times were both aggregated without a
// word, under a report headed "majority of 3 verdicts per game".
func TestVerdictWarningsFlagDuplicatesAndVoteCounts(t *testing.T) {
	verdicts := []Verdict{
		{ID: "G01", Rep: 1}, {ID: "G01", Rep: 2}, {ID: "G01", Rep: 3},
		{ID: "G02", Rep: 1}, {ID: "G02", Rep: 1}, {ID: "G02", Rep: 2},
		{ID: "G03", Rep: 1},
	}
	warnings := VerdictWarnings(verdicts)
	joined := strings.Join(warnings, "\n")

	if !regexp.MustCompile(`G02.*rep 1.*2 times`).MatchString(joined) {
		t.Errorf("no duplicate warning for G02 rep 1:\n%s", joined)
	}
	if !regexp.MustCompile(`G02.*2 verdicts.*expected 3`).MatchString(joined) {
		t.Errorf("no vote-count warning for G02 (2 distinct reps):\n%s", joined)
	}
	if !regexp.MustCompile(`G03.*1 verdict.*expected 3`).MatchString(joined) {
		t.Errorf("no vote-count warning for G03 (1 verdict):\n%s", joined)
	}
	for _, w := range warnings {
		if strings.Contains(w, "G01") {
			t.Errorf("a clean 3-verdict dossier produced a warning: %s", w)
		}
	}
}

// reportRows returns the data rows of a rendered report table.
func reportRows(t *testing.T, report string) (header string, rows []string) {
	t.Helper()
	lines := splitLines(report)
	for i, line := range lines {
		if strings.HasPrefix(line, "| Rank |") {
			header = line
			for _, row := range lines[i+2:] {
				if row == "" {
					break
				}
				rows = append(rows, row)
			}
			return header, rows
		}
	}
	t.Fatalf("report has no table:\n%s", report)
	return "", nil
}

// TestRenderReportShowsVotes: the vote count was aggregated but never shown,
// so a row backed by one verdict looked exactly like a 3-judge majority.
func TestRenderReportShowsVotes(t *testing.T) {
	verdicts := []Verdict{
		{ID: "G01", Rep: 1, Quality: "publishable", Novelty: "novel", Playable: true},
		{ID: "G01", Rep: 2, Quality: "publishable", Novelty: "novel", Playable: true},
		{ID: "G01", Rep: 3, Quality: "publishable", Novelty: "novel", Playable: true},
		{ID: "G03", Rep: 1, Quality: "borderline", Novelty: "novel", Playable: true},
	}
	header, rows := reportRows(t, RenderReport(Rank(Aggregate(verdicts))))
	if !strings.Contains(header, "| Votes |") {
		t.Fatalf("report header has no Votes column: %s", header)
	}
	col := -1
	for i, cell := range strings.Split(header, "|") {
		if strings.TrimSpace(cell) == "Votes" {
			col = i
		}
	}
	want := map[string]string{"G01": "3", "G03": "1"}
	for _, row := range rows {
		cells := strings.Split(row, "|")
		id := strings.TrimSpace(cells[2])
		if got := strings.TrimSpace(cells[col]); got != want[id] {
			t.Errorf("%s votes cell = %q, want %q (row: %s)", id, got, want[id], row)
		}
	}
}

// TestRenderReportSanitizesCells: only "|" was escaped, so a newline in a
// judge's reason (or rediscovery name) split the row and broke the table.
func TestRenderReportSanitizesCells(t *testing.T) {
	verdicts := []Verdict{
		{ID: "G01", Rep: 1, Quality: "borderline", Novelty: "variant_of_known", Playable: true,
			RediscoveryName: "Crazy\nEights | like", Reason: "line1\nline2 | piped\r\nline3"},
		{ID: "G02", Rep: 1, Quality: "borderline", Novelty: "novel", Playable: true, Reason: "plain"},
	}
	header, rows := reportRows(t, RenderReport(Rank(Aggregate(verdicts))))
	if len(rows) != 2 {
		t.Fatalf("table has %d rows for 2 games (a cell broke a row):\n%s", len(rows), strings.Join(rows, "\n"))
	}
	for _, row := range rows {
		if strings.Count(row, "|") != strings.Count(header, "|") {
			t.Errorf("row has %d cell separators, header has %d: %s", strings.Count(row, "|"), strings.Count(header, "|"), row)
		}
	}
	if !strings.Contains(rows[0]+rows[1], "line1 line2 / piped line3") {
		t.Errorf("reason text was not preserved on one line: %v", rows)
	}
}

// TestRankIsIndependentOfIDOrder: dossier ids are a pseudo-random permutation
// of the input, and verdicts arrive in whatever order the judges finish.
// Aggregation and ranking are keyed on the id alone, so neither the id's
// numeric position nor the verdict order may change the result.
func TestRankIsIndependentOfIDOrder(t *testing.T) {
	mk := func(id, quality, novelty string) []Verdict {
		var out []Verdict
		for rep := 1; rep <= 3; rep++ {
			out = append(out, Verdict{ID: id, Rep: rep, Quality: quality, Novelty: novelty, Playable: true, Confidence: 0.6})
		}
		return out
	}
	a, b, c := mk("G07", "publishable", "novel"), mk("G02", "borderline", "variant_of_known"), mk("G11", "publishable", "variant_of_known")

	var forward, shuffled []Verdict
	forward = append(append(append(forward, a...), b...), c...)
	for i := 0; i < 3; i++ { // interleaved, reversed
		shuffled = append(shuffled, c[2-i], b[2-i], a[2-i])
	}
	want := []string{"G07", "G11", "G02"}
	for name, vs := range map[string][]Verdict{"forward": forward, "shuffled": shuffled} {
		ranked := Rank(Aggregate(vs))
		var got []string
		for _, r := range ranked {
			got = append(got, r.ID)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s verdict order ranks %v, want %v", name, got, want)
		}
	}
}

// TestLoadVerdictsWarnsOnUnrecognizedConfidence: an unrecognized confidence
// was silently counted as 0. It still counts as 0 (one malformed field must
// not reject a batch) but the load must say so.
func TestLoadVerdictsWarnsOnUnrecognizedConfidence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "verdicts.json")
	data := `[
{"id":"G01","rep":1,"quality":"publishable","novelty":"novel","confidence":"0.9"},
{"id":"G01","rep":2,"quality":"publishable","novelty":"novel","confidence":"very sure"},
{"id":"G01","rep":3,"quality":"publishable","novelty":"novel","confidence":"high"}
]`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	vs, warnings, err := LoadVerdictsWithWarnings(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 3 || vs[0].Confidence != 0.9 || vs[1].Confidence != 0 || vs[2].Confidence != 0.9 {
		t.Fatalf("confidences = %v, want 0.9 / 0 / 0.9", vs)
	}
	var conf []string
	for _, w := range warnings {
		if strings.Contains(w, "confidence") {
			conf = append(conf, w)
		}
	}
	if len(conf) != 1 || !strings.Contains(conf[0], "G01 rep 2") || !strings.Contains(conf[0], "very sure") {
		t.Errorf("want exactly one confidence warning naming G01 rep 2 and the bad value, got %v", conf)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

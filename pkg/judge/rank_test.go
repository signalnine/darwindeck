package judge

import (
	"testing"
)

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

// TestRediscoveryFlagDemotes verifies that a majority variant_of_known game is
// demoted one band and labeled with the rediscovery name.
func TestRediscoveryFlagDemotes(t *testing.T) {
	verdicts := []Verdict{
		// Publishable quality but majority says variant_of_known -> demote to
		// borderline + label.
		{ID: "G01", Rep: 1, Quality: "publishable", Novelty: "variant_of_known", RediscoveryName: "Whist-like", Playable: true, Confidence: 0.8},
		{ID: "G01", Rep: 2, Quality: "publishable", Novelty: "variant_of_known", RediscoveryName: "Whist-like", Playable: true, Confidence: 0.8},
		{ID: "G01", Rep: 3, Quality: "publishable", Novelty: "novel", Playable: true, Confidence: 0.7},
	}
	a := Aggregate(verdicts)[0]
	if !a.Demoted {
		t.Error("expected rediscovery demotion flag")
	}
	if a.Quality != "borderline" {
		t.Errorf("demoted quality = %q, want borderline", a.Quality)
	}
	if a.Novelty != "variant_of_known" {
		t.Errorf("novelty = %q, want variant_of_known", a.Novelty)
	}
	if a.RediscoveryName != "Whist-like" {
		t.Errorf("rediscovery name = %q, want Whist-like", a.RediscoveryName)
	}
}

// TestRediscoveryDemotedBelowNovelPublishable verifies a demoted rediscovery
// ranks below a genuinely novel publishable game.
func TestRediscoveryDemotedBelowNovelPublishable(t *testing.T) {
	verdicts := []Verdict{
		// G01: publishable but rediscovery -> demoted to borderline.
		{ID: "G01", Rep: 1, Quality: "publishable", Novelty: "variant_of_known", RediscoveryName: "Gin-like", Playable: true, Confidence: 0.9},
		{ID: "G01", Rep: 2, Quality: "publishable", Novelty: "variant_of_known", RediscoveryName: "Gin-like", Playable: true, Confidence: 0.9},
		{ID: "G01", Rep: 3, Quality: "publishable", Novelty: "variant_of_known", RediscoveryName: "Gin-like", Playable: true, Confidence: 0.9},
		// G02: genuinely novel publishable.
		{ID: "G02", Rep: 1, Quality: "publishable", Novelty: "novel", Playable: true, Confidence: 0.7},
		{ID: "G02", Rep: 2, Quality: "publishable", Novelty: "novel", Playable: true, Confidence: 0.7},
		{ID: "G02", Rep: 3, Quality: "publishable", Novelty: "novel", Playable: true, Confidence: 0.7},
	}
	ranked := Rank(Aggregate(verdicts))
	if ranked[0].ID != "G02" {
		t.Errorf("rank 1 = %s, want G02 (novel publishable above demoted rediscovery)", ranked[0].ID)
	}
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

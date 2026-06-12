package main

import (
	"bytes"
	"math"
	"strings"
	"testing"
)

func TestMeanSD(t *testing.T) {
	// Classic hand-computed example: mean 5, population sd 2.
	mean, sd := meanSD([]float64{2, 4, 4, 4, 5, 5, 7, 9})
	if math.Abs(mean-5) > 1e-12 || math.Abs(sd-2) > 1e-12 {
		t.Errorf("meanSD = %v, %v; want 5, 2", mean, sd)
	}

	mean, sd = meanSD(nil)
	if mean != 0 || sd != 0 {
		t.Errorf("meanSD(nil) = %v, %v; want 0, 0", mean, sd)
	}
}

func TestAggregateRow(t *testing.T) {
	samples := [][5]float64{
		{0.2, 0.0, 1.0, 0.5, 1.0},
		{0.4, 1.0, 1.0, 0.5, 0.0},
	}
	row := aggregateRow("g", "shedding", 10, samples)
	if row.tier1Passes != 2 || row.evals != 10 {
		t.Fatalf("counts = %d/%d; want 2/10", row.tier1Passes, row.evals)
	}
	wantMeans := [5]float64{0.3, 0.5, 1.0, 0.5, 0.5}
	wantSDs := [5]float64{0.1, 0.5, 0.0, 0.0, 0.5}
	for k := range metricNames {
		if math.Abs(row.means[k]-wantMeans[k]) > 1e-12 {
			t.Errorf("%s mean = %v; want %v", metricNames[k], row.means[k], wantMeans[k])
		}
		if math.Abs(row.sds[k]-wantSDs[k]) > 1e-12 {
			t.Errorf("%s sd = %v; want %v", metricNames[k], row.sds[k], wantSDs[k])
		}
	}
}

func TestFormatCell(t *testing.T) {
	if got := formatCell(0.712, 0.013, 3); got != "0.712 sd 0.013" {
		t.Errorf("formatCell = %q", got)
	}
	if got := formatCell(0.5, 0.1, 0); got != "n/a" {
		t.Errorf("formatCell with n=0 = %q; want n/a", got)
	}
}

func TestPrintCalibrationTable(t *testing.T) {
	var buf bytes.Buffer
	printCalibrationTable(&buf, []calRow{
		{id: "gin-rummy", skeleton: "rummy", tier1Passes: 9, evals: 10,
			means: [5]float64{0.5, 0.6, 0.7, 0.8, 0.9},
			sds:   [5]float64{0.01, 0.02, 0.03, 0.04, 0.05}},
		{id: "dead-genome", skeleton: "shedding", tier1Passes: 0, evals: 10},
	})
	out := buf.String()
	for _, want := range []string{"decisions", "arc", "interact", "skill", "length",
		"gin-rummy", "9/10", "0.600 sd 0.020", "dead-genome", "0/10", "n/a"} {
		if !strings.Contains(out, want) {
			t.Errorf("table output missing %q:\n%s", want, out)
		}
	}
}

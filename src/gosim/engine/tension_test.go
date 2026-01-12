package engine

import "testing"

func TestNewTensionMetrics(t *testing.T) {
	tm := NewTensionMetrics(4)

	if tm.currentLeader != -1 {
		t.Errorf("expected currentLeader=-1, got %d", tm.currentLeader)
	}
	if tm.ClosestMargin != 1.0 {
		t.Errorf("expected ClosestMargin=1.0, got %f", tm.ClosestMargin)
	}
	if len(tm.leaderHistory) != 0 {
		t.Errorf("expected empty leaderHistory, got len=%d", len(tm.leaderHistory))
	}
	if cap(tm.leaderHistory) < 100 {
		t.Errorf("expected leaderHistory capacity >= 100, got %d", cap(tm.leaderHistory))
	}
}

package casino

import (
	"testing"

	"github.com/darwindeck/darwindeck/pkg/sim"
)

// finished builds a terminal casino state (hands empty, stock exhausted) with
// the given captured-pile sizes and last capturer.
func finished(captured []int, lastCapturer int) *sim.GameState {
	st := sim.NewGameState(len(captured))
	for i, n := range captured {
		st.Hands[i] = nil
		st.Tableau[i] = make([]sim.Card, n)
	}
	st.Deck = nil
	st.TrickLeader = lastCapturer
	return st
}

// TestCaptureTieBreaksFromLastCapturer: a tie on captured cards used to go to
// the lowest seat. It now goes AGAINST the player who made the last capture
// (they already took the final sweep of the table): the tied player next in
// turn order after the last capturer wins. A sole leader always wins.
func TestCaptureTieBreaksFromLastCapturer(t *testing.T) {
	r := &Runner{}
	cases := []struct {
		name     string
		captured []int
		last     int
		want     int
	}{
		{"untied: most cards wins", []int{30, 22}, 1, 0},
		{"untied, the leader made the last capture", []int{30, 22}, 0, 0},
		{"2p tie, seat 0 captured last: seat 1 wins", []int{26, 26}, 0, 1},
		{"2p tie, seat 1 captured last: seat 0 wins", []int{26, 26}, 1, 0},
		{"3p tie at the top, last capturer among the tied", []int{20, 20, 12}, 1, 0},
		{"3p tie at the top, last capturer not tied: next tied after them", []int{20, 12, 20}, 1, 2},
		{"3p three-way tie: next after the last capturer", []int{17, 17, 17}, 2, 0},
	}
	for _, tc := range cases {
		st := finished(tc.captured, tc.last)
		if got := r.CheckEnd(st, casinoGenome()); got != tc.want {
			t.Errorf("%s: captured %v, last capture P%d: winner = P%d, want P%d", tc.name, tc.captured, tc.last, got, tc.want)
		}
	}

	// The scored variant applies the same rule to count + banked score.
	st := finished([]int{20, 24}, 1)
	st.Scores[0], st.Scores[1] = 6, 2 // 26 vs 26
	if got := r.CheckEnd(st, casinoScoredGenome()); got != 0 {
		t.Errorf("scored tie 26-26, P1 captured last: winner = P%d, want P0", got)
	}
	st = finished([]int{20, 24}, 0)
	st.Scores[0], st.Scores[1] = 6, 2
	if got := r.CheckEnd(st, casinoScoredGenome()); got != 1 {
		t.Errorf("scored tie 26-26, P0 captured last: winner = P%d, want P1", got)
	}
}

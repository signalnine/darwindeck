package genome

import "testing"

// TestKnockWinnerUndercut pins the knock resolution both empty-hand-race hosts
// (shedding, climbing), the greedy scorers and the rulebook share: fewest cards
// wins, the knocker only when STRICTLY fewest, and every tie resolves to the
// tied player nearest after the knocker in turn order -- never to a fixed seat.
func TestKnockWinnerUndercut(t *testing.T) {
	cases := []struct {
		name      string
		sizes     []int
		knocker   int
		direction int
		want      int
	}{
		{"knocker strictly fewest", []int{1, 2, 3}, 0, 1, 0},
		{"knocker strictly fewest, last seat", []int{3, 2, 1}, 2, 1, 2},
		{"tied knocker undercut by next seat", []int{2, 2, 3}, 0, 1, 1},
		{"tied knocker undercut wraps around", []int{2, 3, 2}, 2, 1, 0},
		{"three-way tie goes to next after knocker", []int{2, 2, 2}, 1, 1, 2},
		{"three-way tie, reversed direction", []int{2, 2, 2}, 1, -1, 0},
		{"direction 0 behaves as forward", []int{2, 2, 2}, 1, 0, 2},
		{"knocker behind", []int{3, 2, 1}, 0, 1, 2},
		{"knocker behind, others tied", []int{3, 1, 1}, 0, 1, 1},
		{"knocker behind, others tied, reversed", []int{3, 1, 1}, 0, -1, 2},
		{"two players tied: the non-knocker wins", []int{2, 2}, 1, 1, 0},
		{"unrecorded knocker falls back to lowest tied seat", []int{2, 1, 1}, -1, 1, 1},
	}
	for _, tc := range cases {
		if got := KnockWinner(tc.sizes, tc.knocker, tc.direction); got != tc.want {
			t.Errorf("%s: KnockWinner(%v, knocker %d, dir %d) = %d, want %d",
				tc.name, tc.sizes, tc.knocker, tc.direction, got, tc.want)
		}
	}
	if got := KnockWinner(nil, 0, 1); got != -1 {
		t.Errorf("KnockWinner on no players = %d, want -1", got)
	}
}

// TestKnockWinnerIsSeatNeutral: across every knocker seat, a full tie hands the
// win to a different seat each time -- the property the old lowest-seat rule
// violated (seat 0 won every tie).
func TestKnockWinnerIsSeatNeutral(t *testing.T) {
	wins := make([]int, 4)
	for knocker := 0; knocker < 4; knocker++ {
		wins[KnockWinner([]int{2, 2, 2, 2}, knocker, 1)]++
	}
	for seat, w := range wins {
		if w != 1 {
			t.Errorf("seat %d wins %d of 4 full-tie knocks (one per knocker seat), want exactly 1: %v", seat, w, wins)
		}
	}
}

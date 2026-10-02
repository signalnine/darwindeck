package tricktaking

import (
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

func tieGenome(scoring genome.TrickScoring) *genome.Genome {
	g := &genome.Genome{
		Skeleton: genome.TrickTaking, Players: 4, HandSize: 13,
		TrickTaking: &genome.TrickTakingParams{MustFollowSuit: true, TrickScoring: scoring, RoundsPerGame: 1},
	}
	if scoring != genome.ScorePerTrick {
		g.Scoring.CardPoints = []genome.CardScoring{{Suit: 3, Points: 1}}
	}
	return g
}

// finishedState is a played-out single-round game (all hands empty, Round
// advanced) with the given final scores, whose last trick was won by
// lastTrickWinner.
func finishedState(scores []int, lastTrickWinner int) *sim.GameState {
	st := sim.NewGameState(len(scores))
	copy(st.Scores, scores)
	st.Round, st.MaxRound = 1, 1
	st.TrickLeader = lastTrickWinner
	st.Active = lastTrickWinner
	return st
}

// TestScoreTieBreaksFromLastTrickWinner: a tie on the final score used to go to
// the lowest seat -- and 19-27% of random 4-player games on the classic seeds
// end tied at the top, so seat 0 won ~39% of Whist and ~44% of Oh Hell. The
// tie now goes to the tied player who won the final trick, otherwise to the
// tied player next in turn order after the final trick's winner: a rule a
// table can play, with no fixed seat favored.
func TestScoreTieBreaksFromLastTrickWinner(t *testing.T) {
	r := &Runner{}
	high := tieGenome(genome.ScorePerTrick)
	cases := []struct {
		name   string
		scores []int
		last   int
		want   int
	}{
		{"untied: best score wins regardless of the last trick", []int{5, 3, 3, 2}, 2, 0},
		{"tied, last-trick winner among the tied", []int{4, 4, 3, 2}, 1, 1},
		{"tied, last-trick winner is seat 0", []int{4, 4, 3, 2}, 0, 0},
		{"tied, last-trick winner not tied: next tied seat after them", []int{4, 4, 3, 2}, 2, 0},
		{"tied, last-trick winner not tied (wraps past seat 3)", []int{2, 4, 4, 3}, 3, 1},
		{"three-way tie resolves from the last-trick winner", []int{3, 4, 4, 4}, 2, 2},
		{"three-way tie, last-trick winner not tied", []int{3, 4, 4, 4}, 0, 1},
	}
	for _, tc := range cases {
		st := finishedState(tc.scores, tc.last)
		if got := r.CheckEnd(st, high); got != tc.want {
			t.Errorf("highest-wins %s: scores %v, last trick P%d: winner = P%d, want P%d", tc.name, tc.scores, tc.last, got, tc.want)
		}
	}

	// Lowest-wins (Hearts-style) uses the same anchor on the LOWEST score.
	low := tieGenome(genome.ScoreAvoidance)
	lowCases := []struct {
		scores []int
		last   int
		want   int
	}{
		{[]int{0, 0, 13, 13}, 1, 1},
		{[]int{0, 0, 13, 13}, 3, 0},
		{[]int{13, 0, 0, 13}, 0, 1},
		{[]int{13, 5, 0, 8}, 0, 2},
	}
	for _, tc := range lowCases {
		st := finishedState(tc.scores, tc.last)
		if got := r.CheckEnd(st, low); got != tc.want {
			t.Errorf("lowest-wins: scores %v, last trick P%d: winner = P%d, want P%d", tc.scores, tc.last, got, tc.want)
		}
	}
}

// TestTieBreakIsSeatNeutralOverRandomPlay: with the last-trick anchor no seat
// is structurally favored by ties. Seat 0 still leads the first trick, so it
// keeps a small real edge; the bound here only excludes the old ~39% share.
func TestTieBreakIsSeatNeutralOverRandomPlay(t *testing.T) {
	g := tieGenome(genome.ScorePerTrick)
	g.TrumpRule = genome.TrumpCut
	const n = 2000
	res := sim.RunBatch(g, &Runner{}, &sim.RandomAI{}, n, 20261001)
	if res.Completions != n {
		t.Fatalf("only %d/%d games completed", res.Completions, n)
	}
	if share := float64(res.WinCounts[0]) / n; share > 0.32 {
		t.Errorf("seat 0 wins %.1f%% of random 4-player games (fair share 25%%): ties are still breaking toward a fixed seat; wins %v",
			100*share, res.WinCounts)
	}
}

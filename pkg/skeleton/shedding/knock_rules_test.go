package shedding

import (
	"math/rand/v2"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/mechanic"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

func cards(n int, suit sim.Suit) []sim.Card {
	out := make([]sim.Card, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, sim.Card{Suit: suit, Rank: sim.Rank(2 + i)})
	}
	return out
}

// knockAt builds an n-player state with the given hand sizes, has `knocker`
// knock, and returns the CheckEnd winner.
func knockAt(t *testing.T, g *genome.Genome, sizes []int, knocker, direction int) int {
	t.Helper()
	runner := &Runner{}
	state := sim.NewGameState(len(sizes))
	for i, n := range sizes {
		state.Hands[i] = cards(n, sim.Suit(i%4))
	}
	state.Active = knocker
	state.Direction = direction
	runner.ApplyMove(state, sim.Move{Type: sim.MoveKnock, PlayerID: knocker}, g)
	runner.Upkeep(state, g)
	return runner.CheckEnd(state, g)
}

// TestKnockTieUndercut: a knock is resolved by the UNDERCUT rule, not by seat
// order. The knocker wins only when strictly fewest; a knocker who is merely
// tied loses to the tied player nearest after them in turn order, and ties
// among non-knockers resolve the same way. The old rule (ties to the lowest
// seat) handed seat 0 a structural edge: under random play a 3-player
// Mau-Mau + knock gave seat 0 58% of the wins.
func TestKnockTieUndercut(t *testing.T) {
	g := knockGenome()
	g.Players = 3
	cases := []struct {
		name      string
		sizes     []int
		knocker   int
		direction int
		want      int
	}{
		{"strictly fewest knocker wins", []int{2, 3, 3}, 0, 1, 0},
		{"strictly fewest knocker wins from a late seat", []int{3, 3, 2}, 2, 1, 2},
		{"tied knocker at seat 0 is undercut by the next tied player", []int{2, 2, 3}, 0, 1, 1},
		{"tied knocker at seat 1 is undercut (not rescued by seat order)", []int{2, 2, 3}, 1, 1, 0},
		{"tied knocker: nearest AFTER the knocker wins, not the lowest seat", []int{2, 2, 2}, 1, 1, 2},
		{"reversed direction: nearest after the knocker goes the other way", []int{2, 2, 2}, 1, -1, 0},
		{"knocker behind: strictly fewest other player wins", []int{3, 1, 2}, 0, 1, 1},
		{"knocker behind, others tied: nearest after the knocker wins", []int{3, 1, 1}, 2, 1, 1},
		{"knocker behind, others tied (knocker seat 0)", []int{3, 1, 1}, 0, 1, 1},
	}
	for _, tc := range cases {
		if got := knockAt(t, g, tc.sizes, tc.knocker, tc.direction); got != tc.want {
			t.Errorf("%s: hands %v, P%d knocks (dir %d): winner = P%d, want P%d",
				tc.name, tc.sizes, tc.knocker, tc.direction, got, tc.want)
		}
	}
}

// TestKnockScorerKnocksOnlyWhenAhead: the greedy scorer must agree with the
// undercut resolution -- knock iff STRICTLY fewest. Under the old lowest-seat
// tie rule it knocked when tied at the lowest seat, which now hands the win
// away.
func TestKnockScorerKnocksOnlyWhenAhead(t *testing.T) {
	sc := sim.NewSheddingScorer(knockGenome())
	tied := sim.NewGameState(3)
	tied.Hands[0] = make([]sim.Card, 1)
	tied.Hands[1] = make([]sim.Card, 1)
	tied.Hands[2] = make([]sim.Card, 3)
	for seat := 0; seat < 2; seat++ {
		if got := sc.ScoreMove(sim.Move{Type: sim.MoveKnock, PlayerID: seat}, tied); got >= 0 {
			t.Errorf("tied-for-fewest knock at seat %d is an undercut loss; scorer gave %v, want < 0", seat, got)
		}
	}
	ahead := sim.NewGameState(3)
	ahead.Hands[0] = make([]sim.Card, 2)
	ahead.Hands[1] = make([]sim.Card, 1)
	ahead.Hands[2] = make([]sim.Card, 3)
	if got := sc.ScoreMove(sim.Move{Type: sim.MoveKnock, PlayerID: 1}, ahead); got <= 25 {
		t.Errorf("strictly-fewest knock must outscore any play, got %v", got)
	}
}

func multiRoundKnockGenome(rounds int) *genome.Genome {
	return &genome.Genome{
		Skeleton: genome.Shedding, Players: 2, HandSize: 6,
		Shedding: &genome.SheddingParams{MatchRule: genome.MatchEither, DrawPenalty: 1, RoundsPerGame: rounds},
		Borrowed: []genome.BorrowedMechanic{
			{Source: genome.Rummy, Mechanic: genome.MechMeldBonus},
			{Source: genome.Rummy, Mechanic: genome.MechKnock},
		},
	}
}

func applyWithHooks(runner *Runner, state *sim.GameState, g *genome.Genome, mv sim.Move) {
	hooks := mechanic.HooksFor(g)
	for _, e := range runner.ApplyMove(state, mv, g) {
		for _, h := range hooks {
			h(state, g, e)
		}
	}
}

// TestKnockEndsRoundInMultiRound: in a banked-score multi-round game a knock
// ends the ROUND -- every hand is scored by the banking borrow exactly as when
// a player goes out, and the next round is dealt. It used to end the whole
// game on fewest cards, overriding the rulebook's "after N rounds, the highest
// total score wins" (P0 knocks in round 1 of 3 while trailing 0-500 and wins).
func TestKnockEndsRoundInMultiRound(t *testing.T) {
	g := multiRoundKnockGenome(3)
	if errs := genome.Validate(g); len(errs) != 0 {
		t.Fatalf("fixture invalid: %v", errs)
	}
	runner := &Runner{}
	state := runner.Setup(g, rand.New(rand.NewPCG(7, 7)))
	// P0: two cards, no meld. P1: a pair of fives (meld bonus 2/card = 4).
	state.Hands[0] = []sim.Card{{Suit: sim.Spades, Rank: 2}, {Suit: sim.Hearts, Rank: 9}}
	state.Hands[1] = []sim.Card{{Suit: sim.Spades, Rank: 5}, {Suit: sim.Diamonds, Rank: 5}, {Suit: sim.Clubs, Rank: 13}}
	state.Scores[1] = 500
	state.Active = 0

	applyWithHooks(runner, state, g, sim.Move{Type: sim.MoveKnock, PlayerID: 0})
	if w := runner.CheckEnd(state, g); w != -1 {
		t.Fatalf("a knock in round 1 of 3 must not end the game, CheckEnd = %d", w)
	}
	if state.Scores[0] != 0 || state.Scores[1] != 504 {
		t.Errorf("knock must bank the round like a go-out: scores = %v, want [0 504]", state.Scores)
	}

	runner.Upkeep(state, g)
	if state.Round != 1 {
		t.Errorf("Upkeep after a knock must advance the round: Round = %d, want 1", state.Round)
	}
	if len(state.Hands[0]) != g.HandSize || len(state.Hands[1]) != g.HandSize {
		t.Errorf("next round must be redealt: hand sizes %d/%d, want %d each", len(state.Hands[0]), len(state.Hands[1]), g.HandSize)
	}
	if w := runner.CheckEnd(state, g); w != -1 {
		t.Errorf("game must continue into round 2, CheckEnd = %d", w)
	}
	if len(runner.GenerateMoves(state, g)) == 0 {
		t.Error("no legal moves after the post-knock redeal")
	}
}

// TestFinalRoundKnockEndsGameOnBankedTotals: a knock in the LAST round ends the
// game, and the winner is the banked-score leader -- not the fewest-cards
// player.
func TestFinalRoundKnockEndsGameOnBankedTotals(t *testing.T) {
	g := multiRoundKnockGenome(2)
	runner := &Runner{}
	state := runner.Setup(g, rand.New(rand.NewPCG(7, 7)))
	state.Round = 1 // final round of 2
	state.Hands[0] = []sim.Card{{Suit: sim.Spades, Rank: 2}}
	state.Hands[1] = []sim.Card{{Suit: sim.Spades, Rank: 5}, {Suit: sim.Diamonds, Rank: 8}, {Suit: sim.Clubs, Rank: 13}}
	state.Scores[0], state.Scores[1] = 3, 40
	state.Active = 0

	applyWithHooks(runner, state, g, sim.Move{Type: sim.MoveKnock, PlayerID: 0})
	runner.Upkeep(state, g)
	if w := runner.CheckEnd(state, g); w != 1 {
		t.Fatalf("final-round knock: winner = %d, want 1 (the banked-score leader; P0 merely holds fewer cards)", w)
	}
	// Idempotent once over: a second Upkeep must not redeal or re-advance.
	runner.Upkeep(state, g)
	if state.Round != 2 || runner.CheckEnd(state, g) != 1 {
		t.Errorf("finished game changed on a second Upkeep: Round %d, winner %d", state.Round, runner.CheckEnd(state, g))
	}
}

// TestMultiRoundKnockGamesComplete: random multi-round games with a knock still
// always have a move and terminate, and they play every round (a knock no
// longer truncates the game).
func TestMultiRoundKnockGamesComplete(t *testing.T) {
	g := multiRoundKnockGenome(3)
	runner := &Runner{}
	hooks := mechanic.HooksFor(g)
	completed := 0
	for seed := uint64(0); seed < 200; seed++ {
		rng := rand.New(rand.NewPCG(seed, 3))
		state := runner.Setup(g, rng)
		for state.Turn < g.MaxTurns() {
			runner.Upkeep(state, g)
			if runner.CheckEnd(state, g) >= 0 {
				if state.Round != state.MaxRound {
					t.Fatalf("seed %d: game ended in round %d of %d", seed, state.Round, state.MaxRound)
				}
				completed++
				break
			}
			moves := runner.GenerateMoves(state, g)
			if len(moves) == 0 {
				t.Fatalf("seed %d turn %d: zero moves", seed, state.Turn)
			}
			for _, e := range runner.ApplyMove(state, moves[rng.IntN(len(moves))], g) {
				for _, h := range hooks {
					h(state, g, e)
				}
			}
		}
	}
	if completed < 190 {
		t.Fatalf("multi-round knock host completes too rarely: %d/200", completed)
	}
}

package rummy

import (
	"fmt"
	"math/rand/v2"
	"reflect"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

func runGame(g *genome.Genome, seed uint64) sim.GameResult {
	runner := &Runner{}
	ai := &sim.RandomAI{}
	rng := rand.New(rand.NewPCG(seed, 0))

	state := runner.Setup(g, rng)
	maxTurns := g.MaxTurns()

	for {
		runner.Upkeep(state, g)

		winner := runner.CheckEnd(state, g)
		if winner >= 0 {
			return sim.GameResult{
				Winner: winner,
				Turns:  state.Turn,
				Events: state.Events,
			}
		}

		if state.Turn >= maxTurns {
			// Force end at max turns
			return sim.GameResult{
				Winner: scoreRound(state, g),
				Turns:  state.Turn,
				Events: state.Events,
			}
		}

		moves := runner.GenerateMoves(state, g)
		if len(moves) == 0 {
			return sim.GameResult{
				Winner: -1,
				Turns:  state.Turn,
				Error:  "no_moves",
			}
		}

		move := ai.SelectMove(moves, state, rng)
		events := runner.ApplyMove(state, move, g)
		state.Events = append(state.Events, events...)
	}
}

func TestGinRummyCompletes(t *testing.T) {
	g := seeds.GinRummy()
	completions := 0
	for seed := uint64(0); seed < 100; seed++ {
		result := runGame(g, seed)
		if result.Winner >= 0 {
			completions++
		}
		if result.Error == "no_moves" {
			t.Fatalf("seed %d: no moves", seed)
		}
	}
	t.Logf("Gin Rummy: %d/100 completed", completions)
	if completions < 80 {
		t.Fatalf("too few completions: %d/100", completions)
	}
}

func TestKnockRummyCompletes(t *testing.T) {
	g := seeds.KnockRummy()
	completions := 0
	for seed := uint64(0); seed < 100; seed++ {
		result := runGame(g, seed)
		if result.Winner >= 0 {
			completions++
		}
	}
	t.Logf("Knock Rummy: %d/100 completed", completions)
	if completions < 80 {
		t.Fatalf("too few completions: %d/100", completions)
	}
}

func TestRummyDeterminism(t *testing.T) {
	g := seeds.GinRummy()
	r1 := runGame(g, 42)
	r2 := runGame(g, 42)

	if r1.Winner != r2.Winner {
		t.Fatalf("non-deterministic: winner %d vs %d", r1.Winner, r2.Winner)
	}
	if r1.Turns != r2.Turns {
		t.Fatalf("non-deterministic: turns %d vs %d", r1.Turns, r2.Turns)
	}
}

// TestMeldMoveOrderDeterministic pins dd-audit-1: meld-move generation must not
// depend on Go map iteration order. Same genome+seed => byte-identical event streams.
func TestMeldMoveOrderDeterministic(t *testing.T) {
	g := seeds.GinRummy() // rummy genome with melds reachable
	for seed := uint64(1); seed <= 20; seed++ {
		r1 := sim.RunBatch(g, &Runner{}, &sim.RandomAI{}, 1, seed)
		r2 := sim.RunBatch(g, &Runner{}, &sim.RandomAI{}, 1, seed)
		if !reflect.DeepEqual(r1.AllEvents, r2.AllEvents) {
			t.Fatalf("seed %d: event streams differ", seed)
		}
	}
}

// TestMeldMoveListOrderDeterministic asserts the generated move list itself is
// order-stable across repeated calls on identical state. This catches map-order
// nondeterminism directly, without relying on RandomAI to expose it.
func TestMeldMoveListOrderDeterministic(t *testing.T) {
	g := seeds.GinRummy()
	runner := &Runner{}
	for seed := uint64(1); seed <= 20; seed++ {
		s1 := runner.Setup(g, rand.New(rand.NewPCG(seed, 0)))
		s2 := runner.Setup(g, rand.New(rand.NewPCG(seed, 0)))
		s1.Phase = sim.PhaseMeld
		s2.Phase = sim.PhaseMeld
		m1 := runner.GenerateMoves(s1, g)
		m2 := runner.GenerateMoves(s2, g)
		if !reflect.DeepEqual(m1, m2) {
			t.Fatalf("seed %d: meld move lists differ in order/content:\n%v\nvs\n%v", seed, m1, m2)
		}
	}
}

// TestCheckEndReturnsMinusOneAtMaxTurns verifies CheckEnd does not score a
// hung round when only the max-turns cap has been hit. The batch runner
// classifies that case as a Timeout, which Tier1 needs to detect stalls.
func TestCheckEndReturnsMinusOneAtMaxTurns(t *testing.T) {
	g := seeds.GinRummy()
	runner := &Runner{}
	rng := rand.New(rand.NewPCG(1, 0))
	state := runner.Setup(g, rng)

	// Force a stalled state: round has not ended, but turn counter is at the
	// game-wide cap.
	if state.Phase == sim.PhaseEnd {
		t.Fatalf("setup unexpectedly produced PhaseEnd")
	}
	state.Turn = g.MaxTurns()

	if winner := runner.CheckEnd(state, g); winner != -1 {
		t.Fatalf("CheckEnd at max turns mid-round returned %d, want -1", winner)
	}
}

func TestFindSets(t *testing.T) {
	hand := []sim.Card{
		{Suit: sim.Hearts, Rank: sim.King},
		{Suit: sim.Spades, Rank: sim.King},
		{Suit: sim.Clubs, Rank: sim.King},
		{Suit: sim.Hearts, Rank: sim.Five},
	}

	sets := findSets(hand, 3)
	if len(sets) != 1 {
		t.Fatalf("expected 1 set, got %d", len(sets))
	}
	if len(sets[0]) != 3 {
		t.Fatalf("expected set of 3, got %d", len(sets[0]))
	}
}

func TestFindRuns(t *testing.T) {
	hand := []sim.Card{
		{Suit: sim.Hearts, Rank: sim.Five},
		{Suit: sim.Hearts, Rank: sim.Six},
		{Suit: sim.Hearts, Rank: sim.Seven},
		{Suit: sim.Hearts, Rank: sim.Eight},
		{Suit: sim.Spades, Rank: sim.Two},
	}

	runs := findRuns(hand, 3)
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}
	if len(runs[0]) < 3 {
		t.Fatalf("expected run of at least 3, got %d", len(runs[0]))
	}
}

func TestDeadwood(t *testing.T) {
	params := &genome.RummyParams{
		MeldTypes:   genome.MeldBoth,
		MinMeldSize: 3,
	}

	// Hand with one set of kings (0 deadwood for those) + a 5
	hand := []sim.Card{
		{Suit: sim.Hearts, Rank: sim.King},
		{Suit: sim.Spades, Rank: sim.King},
		{Suit: sim.Clubs, Rank: sim.King},
		{Suit: sim.Hearts, Rank: sim.Five},
	}

	dw := calcDeadwood(hand, params)
	if dw != 5 {
		t.Fatalf("expected deadwood 5, got %d", dw)
	}
}

func TestDeadwoodOverlappingSetAndRun(t *testing.T) {
	params := &genome.RummyParams{
		MeldTypes:   genome.MeldBoth,
		MinMeldSize: 3,
	}

	// Hand where 5H can belong to either a set of three 5s or a run 5H-6H-7H.
	// A card can only be used in one meld, so at most one of these can form.
	// Optimal partition: use the run (saves 5+6+7=18), leaves 5D+5C as deadwood (5+5=10).
	// Set-only partition leaves 6H+7H as deadwood (6+7=13).
	// Buggy behavior (over-marking): deadwood = 0.
	hand := []sim.Card{
		{Suit: sim.Hearts, Rank: sim.Five},
		{Suit: sim.Diamonds, Rank: sim.Five},
		{Suit: sim.Clubs, Rank: sim.Five},
		{Suit: sim.Hearts, Rank: sim.Six},
		{Suit: sim.Hearts, Rank: sim.Seven},
	}

	dw := calcDeadwood(hand, params)
	if dw != 10 {
		t.Fatalf("expected deadwood 10 (optimal run partition), got %d", dw)
	}
}

func TestDeadwoodSubsetSetWithRun(t *testing.T) {
	// 4 fives + a Spades run that needs 5S.
	// Greedy by maximal-meld value picks the 4-set (value 20) and blocks
	// the run, leaving 6S+7S = 13 deadwood.
	// Optimal partition: 3-set [5H,5D,5C] + run [5S,6S,7S] -> 0 deadwood.
	params := &genome.RummyParams{
		MeldTypes:   genome.MeldBoth,
		MinMeldSize: 3,
	}
	hand := []sim.Card{
		{Suit: sim.Hearts, Rank: sim.Five},
		{Suit: sim.Diamonds, Rank: sim.Five},
		{Suit: sim.Clubs, Rank: sim.Five},
		{Suit: sim.Spades, Rank: sim.Five},
		{Suit: sim.Spades, Rank: sim.Six},
		{Suit: sim.Spades, Rank: sim.Seven},
	}
	if dw := calcDeadwood(hand, params); dw != 0 {
		t.Fatalf("expected deadwood 0 (3-set + run partition), got %d", dw)
	}
}

func TestDeadwoodSubRunReleasesCardForSet(t *testing.T) {
	// Run 5H..9H plus 9D,9C means optimal is run [5H,6H,7H,8H] + set [9H,9D,9C] = 0.
	// Naive maximal-meld algorithm picks run [5H..9H] (value 35) which blocks the set,
	// leaving 9D+9C = 20 deadwood.
	params := &genome.RummyParams{
		MeldTypes:   genome.MeldBoth,
		MinMeldSize: 3,
	}
	hand := []sim.Card{
		{Suit: sim.Hearts, Rank: sim.Five},
		{Suit: sim.Hearts, Rank: sim.Six},
		{Suit: sim.Hearts, Rank: sim.Seven},
		{Suit: sim.Hearts, Rank: sim.Eight},
		{Suit: sim.Hearts, Rank: sim.Nine},
		{Suit: sim.Diamonds, Rank: sim.Nine},
		{Suit: sim.Clubs, Rank: sim.Nine},
	}
	if dw := calcDeadwood(hand, params); dw != 0 {
		t.Fatalf("expected deadwood 0 (sub-run + set partition), got %d", dw)
	}
}

func TestDeadwoodEmpty(t *testing.T) {
	params := &genome.RummyParams{MeldTypes: genome.MeldBoth, MinMeldSize: 3}
	dw := calcDeadwood(nil, params)
	if dw != 0 {
		t.Fatalf("expected 0 deadwood for empty hand, got %d", dw)
	}
}

func TestCardValueAce(t *testing.T) {
	// Rummy convention: Ace is worth 1 point as deadwood (low),
	// even though it's ranked high (14) for run ordering.
	ace := sim.Card{Suit: sim.Hearts, Rank: sim.Ace}
	if got := cardValue(ace); got != 1 {
		t.Fatalf("expected Ace value 1, got %d", got)
	}
}

func TestCardValueFaceCards(t *testing.T) {
	// Face cards (10, J, Q, K) are worth 10 each.
	cases := []sim.Rank{sim.Ten, sim.Jack, sim.Queen, sim.King}
	for _, r := range cases {
		c := sim.Card{Suit: sim.Clubs, Rank: r}
		if got := cardValue(c); got != 10 {
			t.Fatalf("expected %s value 10, got %d", r, got)
		}
	}
}

func TestCardValueNumberCards(t *testing.T) {
	// Number cards (2-9) are worth their face value.
	for r := sim.Two; r <= sim.Nine; r++ {
		c := sim.Card{Suit: sim.Diamonds, Rank: r}
		if got := cardValue(c); got != int(r) {
			t.Fatalf("expected %s value %d, got %d", r, int(r), got)
		}
	}
}

func TestDeadwoodWithAces(t *testing.T) {
	// Hand with unmelded aces should score them at 1 each, not 10.
	params := &genome.RummyParams{MeldTypes: genome.MeldBoth, MinMeldSize: 3}
	hand := []sim.Card{
		{Suit: sim.Hearts, Rank: sim.Ace},
		{Suit: sim.Spades, Rank: sim.Ace},
		{Suit: sim.Hearts, Rank: sim.Five},
	}
	// 1 + 1 + 5 = 7 (not 10 + 10 + 5 = 25)
	if dw := calcDeadwood(hand, params); dw != 7 {
		t.Fatalf("expected deadwood 7 (Ace=1, Ace=1, Five=5), got %d", dw)
	}
}

func TestAllRummySeedsValid(t *testing.T) {
	seedGames := []*genome.Genome{
		seeds.GinRummy(),
		seeds.KnockRummy(),
	}

	for _, g := range seedGames {
		errs := genome.Validate(g)
		if len(errs) != 0 {
			t.Errorf("%s failed validation: %v", g.ID, errs)
		}
	}
}

func TestSetupStoresRNG(t *testing.T) {
	g := seeds.GinRummy()
	runner := &Runner{}
	rng := rand.New(rand.NewPCG(7, 0))
	state := runner.Setup(g, rng)
	if state.RNG != rng {
		t.Fatalf("Setup should store rng on state.RNG (got %p, want %p)", state.RNG, rng)
	}
}

func TestMeldingEmptyHandEndsRound(t *testing.T) {
	// If a player melds away every card in hand, the round must end (gin).
	// Otherwise the runner cycles between PhaseMeld(empty) -> Pass -> PhaseDiscard(empty) -> Pass forever.
	g := &genome.Genome{
		Skeleton: genome.Rummy,
		Players:  2,
		Rummy:    &genome.RummyParams{MeldTypes: genome.MeldBoth, MinMeldSize: 3},
	}
	runner := &Runner{}
	state := sim.NewGameState(2)
	state.Hands[0] = []sim.Card{
		{Suit: sim.Hearts, Rank: sim.King},
		{Suit: sim.Spades, Rank: sim.King},
		{Suit: sim.Clubs, Rank: sim.King},
	}
	state.Hands[1] = []sim.Card{{Suit: sim.Diamonds, Rank: sim.Two}}
	state.Phase = sim.PhaseMeld
	state.Active = 0

	move := sim.Move{Type: sim.MoveMeld, PlayerID: 0, Cards: append([]sim.Card{}, state.Hands[0]...)}
	runner.ApplyMove(state, move, g)

	if state.Phase != sim.PhaseEnd {
		t.Fatalf("expected PhaseEnd after melding away whole hand, got %d", state.Phase)
	}
}

// reshuffleState builds a 2-player rummy state poised to trigger the
// stockpile reshuffle: empty deck, full discard, active hand has 2 cards
// (so a discard does not gin-out and the else branch fires).
func reshuffleState(rng *rand.Rand) *sim.GameState {
	state := sim.NewGameState(2)
	state.Hands[0] = []sim.Card{
		{Suit: sim.Hearts, Rank: sim.Two},
		{Suit: sim.Spades, Rank: sim.Three},
	}
	state.Hands[1] = []sim.Card{{Suit: sim.Clubs, Rank: sim.Four}}
	state.Deck = nil
	state.Discard = []sim.Card{
		{Suit: sim.Hearts, Rank: sim.Five},
		{Suit: sim.Hearts, Rank: sim.Six},
		{Suit: sim.Hearts, Rank: sim.Seven},
		{Suit: sim.Hearts, Rank: sim.Eight},
		{Suit: sim.Hearts, Rank: sim.Nine},
		{Suit: sim.Hearts, Rank: sim.Ten},
		{Suit: sim.Diamonds, Rank: sim.Two},
		{Suit: sim.Diamonds, Rank: sim.Three},
		{Suit: sim.Diamonds, Rank: sim.Four},
		{Suit: sim.Diamonds, Rank: sim.Five},
		{Suit: sim.Diamonds, Rank: sim.Six},
		{Suit: sim.Diamonds, Rank: sim.Seven},
	}
	state.Turn = 0
	state.Active = 0
	state.Phase = sim.PhaseDiscard
	state.RNG = rng
	return state
}

func TestStockpileReshuffleUsesRNG(t *testing.T) {
	g := &genome.Genome{Skeleton: genome.Rummy, Players: 2, Rummy: &genome.RummyParams{}}
	runner := &Runner{}

	// Two states identical apart from RNG seed should produce different
	// post-reshuffle deck orderings.
	a := reshuffleState(rand.New(rand.NewPCG(1, 0)))
	b := reshuffleState(rand.New(rand.NewPCG(2, 0)))
	move := sim.Move{Type: sim.MoveDiscard, Cards: []sim.Card{{Suit: sim.Hearts, Rank: sim.Two}}, PlayerID: 0}
	runner.ApplyMove(a, move, g)
	runner.ApplyMove(b, move, g)

	if len(a.Deck) != len(b.Deck) {
		t.Fatalf("decks should be same length, got %d vs %d", len(a.Deck), len(b.Deck))
	}
	same := true
	for i := range a.Deck {
		if a.Deck[i] != b.Deck[i] {
			same = false
			break
		}
	}
	if same {
		t.Fatalf("reshuffle is deterministic across RNG seeds: deck=%v", a.Deck)
	}
}

func TestStockpileReshufflePreservesCards(t *testing.T) {
	g := &genome.Genome{Skeleton: genome.Rummy, Players: 2, Rummy: &genome.RummyParams{}}
	runner := &Runner{}
	state := reshuffleState(rand.New(rand.NewPCG(99, 0)))

	originalDiscard := make([]sim.Card, len(state.Discard))
	copy(originalDiscard, state.Discard)
	expectedTop := originalDiscard[len(originalDiscard)-1]

	move := sim.Move{Type: sim.MoveDiscard, Cards: []sim.Card{{Suit: sim.Hearts, Rank: sim.Two}}, PlayerID: 0}
	runner.ApplyMove(state, move, g)

	// After: discard = [discarded card] (the top after MoveDiscard appends, then reshuffle pulls all but top).
	// originalDiscard had N cards. The MoveDiscard appended Hearts-Two. So pre-reshuffle discard had N+1 cards.
	// Reshuffle keeps top (which is Hearts-Two -- the just-discarded card) and moves N cards into deck.
	if len(state.Deck) != len(originalDiscard) {
		t.Fatalf("expected deck length %d, got %d", len(originalDiscard), len(state.Deck))
	}
	if len(state.Discard) != 1 {
		t.Fatalf("expected discard length 1, got %d", len(state.Discard))
	}
	// The top of new discard is the just-discarded card (Hearts-Two), not the previous top.
	wantTop := sim.Card{Suit: sim.Hearts, Rank: sim.Two}
	if state.Discard[0] != wantTop {
		t.Fatalf("expected discard top %v, got %v", wantTop, state.Discard[0])
	}
	_ = expectedTop // not used (prior top moved into the deck)

	// Multiset check: deck should contain originalDiscard exactly (in some order).
	count := func(cards []sim.Card) map[sim.Card]int {
		m := make(map[sim.Card]int)
		for _, c := range cards {
			m[c]++
		}
		return m
	}
	got := count(state.Deck)
	want := count(originalDiscard)
	if len(got) != len(want) {
		t.Fatalf("multiset size mismatch: got %d kinds, want %d", len(got), len(want))
	}
	for c, n := range want {
		if got[c] != n {
			t.Fatalf("card %v count mismatch: got %d, want %d", c, got[c], n)
		}
	}
}

// TestScoreRoundPreservesHookContributions verifies that deadwood banking
// (Upkeep -> bankDeadwood) uses additive deadwood (-=) instead of assignment
// (=), so that any prior contributions written by HookScoring/HookEndOfRound
// hooks survive into the final scores used to pick the winner (CheckEnd).
// dd-2lq regression: when the banking used `state.Scores[i] = -deadwood` it
// clobbered hook output, silently neutralizing every borrowed scoring
// mechanic that targets Rummy.
func TestScoreRoundPreservesHookContributions(t *testing.T) {
	g := seeds.GinRummy()

	state := &sim.GameState{
		Hands: [][]sim.Card{
			// Player 0: 5-card deadwood (low value).
			{
				{Suit: sim.Hearts, Rank: sim.Two},
				{Suit: sim.Spades, Rank: sim.Three},
				{Suit: sim.Clubs, Rank: sim.Four},
				{Suit: sim.Diamonds, Rank: sim.Five},
				{Suit: sim.Hearts, Rank: sim.Six},
			},
			// Player 1: identical 5-card deadwood — without hook input the
			// score tiebreak goes to the lower index.
			{
				{Suit: sim.Hearts, Rank: sim.Two},
				{Suit: sim.Spades, Rank: sim.Three},
				{Suit: sim.Clubs, Rank: sim.Four},
				{Suit: sim.Diamonds, Rank: sim.Five},
				{Suit: sim.Hearts, Rank: sim.Six},
			},
		},
		// Simulate a HookScoring/HookEndOfRound hook having already awarded
		// player 1 a large bonus before Upkeep banks deadwood.
		Scores:     []int{0, 1000},
		NumPlayers: 2,
		Phase:      sim.PhaseEnd,
	}

	// Drive the real loop sequence: Upkeep banks deadwood, CheckEnd picks
	// the winner from the banked scores.
	runner := &Runner{}
	runner.Upkeep(state, g)
	winner := runner.CheckEnd(state, g)

	if winner != 1 {
		t.Fatalf("hook gave player 1 a +1000 bonus on identical deadwood; "+
			"deadwood banking clobbered it (winner=%d, scores=%v)", winner, state.Scores)
	}
	if state.Scores[1] <= state.Scores[0] {
		t.Fatalf("player 1's hook bonus should survive deadwood subtraction "+
			"(scores=%v)", state.Scores)
	}
}

// TestGenerateMeldMovesOffersSubsetsOfFourOfAKind verifies that a hand with
// 4 cards of the same rank produces multiple meld move options at
// MinMeldSize=3: each 3-card subset plus the 4-card set, rather than only
// the maximal 4-set. dd-9hy: generateMeldMoves was only returning maximal
// melds, blocking optimal sub-meld choices.
func TestGenerateMeldMovesOffersSubsetsOfFourOfAKind(t *testing.T) {
	g := &genome.Genome{
		Skeleton: genome.Rummy,
		Players:  2,
		Rummy: &genome.RummyParams{
			MeldTypes:      genome.MeldBoth,
			MinMeldSize:    3,
			KnockThreshold: 0,
		},
	}
	runner := &Runner{}
	state := sim.NewGameState(2)
	state.Hands[0] = []sim.Card{
		{Suit: sim.Spades, Rank: sim.King},
		{Suit: sim.Hearts, Rank: sim.King},
		{Suit: sim.Diamonds, Rank: sim.King},
		{Suit: sim.Clubs, Rank: sim.King},
		{Suit: sim.Hearts, Rank: sim.Two},
	}
	state.Phase = sim.PhaseMeld
	state.Active = 0

	moves := runner.GenerateMoves(state, g)

	meldMoves := 0
	threeCardKingMelds := 0
	fourCardKingMelds := 0
	for _, m := range moves {
		if m.Type != sim.MoveMeld {
			continue
		}
		meldMoves++
		allKings := true
		for _, c := range m.Cards {
			if c.Rank != sim.King {
				allKings = false
				break
			}
		}
		if !allKings {
			continue
		}
		switch len(m.Cards) {
		case 3:
			threeCardKingMelds++
		case 4:
			fourCardKingMelds++
		}
	}

	// C(4,3) = 4 distinct 3-card subsets, plus the 4-card set.
	if threeCardKingMelds != 4 {
		t.Fatalf("expected 4 three-card king meld options, got %d (meld moves total: %d)",
			threeCardKingMelds, meldMoves)
	}
	if fourCardKingMelds != 1 {
		t.Fatalf("expected 1 four-card king meld option, got %d", fourCardKingMelds)
	}
}

// TestGenerateMeldMovesOffersSubRuns verifies that a 4-card run produces
// both the maximal run and the 3-card sub-run options at MinMeldSize=3.
// Without sub-run enumeration the player cannot lay a 3-card run keeping
// the 4th card for later use.
func TestGenerateMeldMovesOffersSubRuns(t *testing.T) {
	g := &genome.Genome{
		Skeleton: genome.Rummy,
		Players:  2,
		Rummy: &genome.RummyParams{
			MeldTypes:      genome.MeldBoth,
			MinMeldSize:    3,
			KnockThreshold: 0,
		},
	}
	runner := &Runner{}
	state := sim.NewGameState(2)
	state.Hands[0] = []sim.Card{
		{Suit: sim.Hearts, Rank: sim.Five},
		{Suit: sim.Hearts, Rank: sim.Six},
		{Suit: sim.Hearts, Rank: sim.Seven},
		{Suit: sim.Hearts, Rank: sim.Eight},
		{Suit: sim.Spades, Rank: sim.Two},
	}
	state.Phase = sim.PhaseMeld
	state.Active = 0

	moves := runner.GenerateMoves(state, g)

	threeCardRuns := 0
	fourCardRuns := 0
	for _, m := range moves {
		if m.Type != sim.MoveMeld {
			continue
		}
		switch len(m.Cards) {
		case 3:
			threeCardRuns++
		case 4:
			fourCardRuns++
		}
	}

	// Two 3-card sub-runs: [5,6,7] and [6,7,8]. One 4-card run: [5,6,7,8].
	if threeCardRuns != 2 {
		t.Fatalf("expected 2 three-card sub-run options, got %d", threeCardRuns)
	}
	if fourCardRuns != 1 {
		t.Fatalf("expected 1 four-card run option, got %d", fourCardRuns)
	}
}

func TestPhaseProgression(t *testing.T) {
	// Verify the draw → meld → discard phase cycle
	g := seeds.GinRummy()
	runner := &Runner{}
	rng := rand.New(rand.NewPCG(1, 0))
	state := runner.Setup(g, rng)

	if state.Phase != sim.PhaseDraw {
		t.Fatalf("expected PhaseDraw, got %d", state.Phase)
	}

	// Draw
	moves := runner.GenerateMoves(state, g)
	for _, m := range moves {
		if m.Type == sim.MoveDraw {
			runner.ApplyMove(state, m, g)
			break
		}
	}
	if state.Phase != sim.PhaseMeld {
		t.Fatalf("after draw, expected PhaseMeld, got %d", state.Phase)
	}

	// Pass melding
	runner.ApplyMove(state, sim.Move{Type: sim.MovePass, PlayerID: state.Active}, g)
	if state.Phase != sim.PhaseDiscard {
		t.Fatalf("after pass meld, expected PhaseDiscard, got %d", state.Phase)
	}
}

// stateHash serializes every GameState field except RNG (a pointer; the
// purity tests pass a nil RNG sentinel so any dereference panics loudly).
// Used to assert that query methods do not mutate state (audit Task 3).
func stateHash(s *sim.GameState) string {
	top := "nil"
	if s.TopCard != nil {
		top = s.TopCard.String()
	}
	return fmt.Sprintf("deck=%v|hands=%v|discard=%v|tableau=%v|scores=%v|turn=%d|active=%d|phase=%d|np=%d|dir=%d|round=%d|maxround=%d|top=%s|tc=%v|tp=%v|tl=%d|trump=%d|broken=%t|melds=%v|owners=%v|events=%v",
		s.Deck, s.Hands, s.Discard, s.Tableau, s.Scores, s.Turn, s.Active,
		s.Phase, s.NumPlayers, s.Direction, s.Round, s.MaxRound, top,
		s.TrickCards, s.TrickPlayers, s.TrickLeader, s.TrumpSuit, s.TrickBroken,
		s.Melds, s.MeldOwner, s.Events)
}

// TestGenerateMovesIsPure pins audit Task 3: GenerateMoves must be a pure
// query in every phase -- no state mutation, identical move lists on repeat.
func TestGenerateMovesIsPure(t *testing.T) {
	g := seeds.GinRummy()
	runner := &Runner{}

	for _, phase := range []sim.PhaseType{sim.PhaseDraw, sim.PhaseMeld, sim.PhaseDiscard} {
		state := runner.Setup(g, rand.New(rand.NewPCG(7, 0)))
		state.Phase = phase
		state.RNG = nil // sentinel: pure queries must never touch the RNG

		before := stateHash(state)
		m1 := runner.GenerateMoves(state, g)
		after1 := stateHash(state)
		m2 := runner.GenerateMoves(state, g)
		after2 := stateHash(state)

		if after1 != before {
			t.Errorf("phase %d: first GenerateMoves mutated state:\nbefore: %s\nafter:  %s", phase, before, after1)
		}
		if after2 != before {
			t.Errorf("phase %d: second GenerateMoves mutated state:\nbefore: %s\nafter:  %s", phase, before, after2)
		}
		if !reflect.DeepEqual(m1, m2) {
			t.Errorf("phase %d: repeated GenerateMoves returned different moves:\n%v\nvs\n%v", phase, m1, m2)
		}
	}
}

// TestCheckEndIsPure pins audit Task 3: CheckEnd must be a pure query. The
// historical violation: on PhaseEnd it banked each player's deadwood into
// state.Scores (scoreRound), so a second call double-subtracted deadwood and
// could flip the winner. Deadwood banking now lives in Upkeep.
func TestCheckEndIsPure(t *testing.T) {
	g := seeds.GinRummy()
	runner := &Runner{}

	state := sim.NewGameState(2)
	state.Hands[0] = []sim.Card{
		{Suit: sim.Hearts, Rank: sim.Two},
		{Suit: sim.Spades, Rank: sim.Three},
	}
	state.Hands[1] = []sim.Card{
		{Suit: sim.Clubs, Rank: sim.King},
		{Suit: sim.Diamonds, Rank: sim.Queen},
	}
	state.Phase = sim.PhaseEnd
	state.RNG = nil // sentinel: pure queries must never touch the RNG

	before := stateHash(state)
	w1 := runner.CheckEnd(state, g)
	after1 := stateHash(state)
	w2 := runner.CheckEnd(state, g)
	after2 := stateHash(state)

	if after1 != before {
		t.Errorf("first CheckEnd mutated state:\nbefore: %s\nafter:  %s", before, after1)
	}
	if after2 != before {
		t.Errorf("second CheckEnd mutated state:\nbefore: %s\nafter:  %s", before, after2)
	}
	if w1 != w2 {
		t.Errorf("repeated CheckEnd returned different winners: %d vs %d", w1, w2)
	}
}

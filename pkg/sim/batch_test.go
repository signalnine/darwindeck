// Package sim_test exercises the batch runner from outside the sim package so
// it can drive the REAL skeleton runners (shedding, rummy, tricktaking).
// Importing them from an in-package test would be an import cycle: the
// skeletons import sim.
package sim_test

import (
	"math/rand/v2"
	"reflect"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
	"github.com/darwindeck/darwindeck/pkg/sim"
	"github.com/darwindeck/darwindeck/pkg/skeleton/rummy"
	"github.com/darwindeck/darwindeck/pkg/skeleton/shedding"
	"github.com/darwindeck/darwindeck/pkg/skeleton/tricktaking"
)

// stubRunner satisfies GenericRunner but is never invoked when n==0.
type stubRunner struct{}

func (stubRunner) Setup(g *genome.Genome, rng *rand.Rand) *sim.GameState { return nil }
func (stubRunner) Upkeep(state *sim.GameState, g *genome.Genome)         {}
func (stubRunner) GenerateMoves(state *sim.GameState, g *genome.Genome) []sim.Move {
	return nil
}
func (stubRunner) ApplyMove(state *sim.GameState, move sim.Move, g *genome.Genome) []sim.Event {
	return nil
}
func (stubRunner) CheckEnd(state *sim.GameState, g *genome.Genome) int { return -1 }
func (stubRunner) Progress(state *sim.GameState, g *genome.Genome) []float64 {
	return nil
}

// TestRunBatchEmptyDoesNotLeakSentinelMinTurns guards against dd-d80:
// when n==0 the loop never runs, so MinTurns must not surface as the
// initialization sentinel (~2.1B) to downstream metrics.
func TestRunBatchEmptyDoesNotLeakSentinelMinTurns(t *testing.T) {
	g := &genome.Genome{Players: 2, HandSize: 5, Skeleton: genome.Shedding}
	result := sim.RunBatch(g, stubRunner{}, &sim.RandomAI{}, 0, 1)

	if result.GamesPlayed != 0 {
		t.Fatalf("GamesPlayed = %d, want 0", result.GamesPlayed)
	}
	if result.MinTurns != 0 {
		t.Errorf("MinTurns = %d on empty batch, want 0 (sentinel leaked to caller)", result.MinTurns)
	}
	if result.MaxTurns != 0 {
		t.Errorf("MaxTurns = %d on empty batch, want 0", result.MaxTurns)
	}
}

// fixedSetupRunner wraps a real skeleton runner but substitutes a
// hand-constructed state from Setup, so OptionDelta semantics can be verified
// against values computed by hand (audit Task 7).
type fixedSetupRunner struct {
	sim.GenericRunner
	build func() *sim.GameState
}

func (f fixedSetupRunner) Setup(g *genome.Genome, rng *rand.Rand) *sim.GameState {
	st := f.build()
	st.RNG = rng
	return st
}

func card(s sim.Suit, r sim.Rank) sim.Card { return sim.Card{Suit: s, Rank: r} }

// TestBatchRecordsPerTurnData is the Task 7 entry test: a 5-game shedding
// batch must record one TurnRecord per applied move, with a sane player index
// and at least one legal move, and AllTurns/AllLeaders must stay parallel to
// AllEvents.
func TestBatchRecordsPerTurnData(t *testing.T) {
	g := seeds.CrazyEights()
	result := sim.RunBatch(g, &shedding.Runner{}, &sim.RandomAI{}, 5, 1)

	if len(result.AllTurns) != result.GamesPlayed {
		t.Fatalf("len(AllTurns) = %d, want %d (parallel to games)", len(result.AllTurns), result.GamesPlayed)
	}
	if len(result.AllTurns) != len(result.AllEvents) {
		t.Fatalf("len(AllTurns) = %d, len(AllEvents) = %d: must be parallel", len(result.AllTurns), len(result.AllEvents))
	}
	if len(result.AllLeaders) != result.GamesPlayed {
		t.Fatalf("len(AllLeaders) = %d, want %d (parallel to games)", len(result.AllLeaders), result.GamesPlayed)
	}
	for i, turns := range result.AllTurns {
		if len(turns) == 0 {
			t.Errorf("game %d: no turn records", i)
		}
		for j, tr := range turns {
			if tr.LegalMoves < 1 {
				t.Errorf("game %d turn %d: LegalMoves = %d, want >= 1", i, j, tr.LegalMoves)
			}
			if tr.Player < 0 || tr.Player >= g.Players {
				t.Errorf("game %d turn %d: Player = %d out of range", i, j, tr.Player)
			}
		}
	}
	// Leaders (Task 8): one argmax-of-Progress entry per applied move,
	// parallel to the TurnRecords, each either a player index or -1 (tie).
	for i, leaders := range result.AllLeaders {
		if len(leaders) != len(result.AllTurns[i]) {
			t.Errorf("game %d: len(Leaders) = %d, want %d (parallel to TurnRecords)", i, len(leaders), len(result.AllTurns[i]))
		}
		for j, l := range leaders {
			if l < -1 || int(l) >= g.Players {
				t.Errorf("game %d move %d: leader = %d, want -1..%d", i, j, l, g.Players-1)
			}
		}
	}
}

// TestForcedDrawRecordsOneLegalMove pins the plan's forced-play fixture: a
// hand that cannot play anything has exactly one legal move (draw), and that
// must be recorded as LegalMoves == 1.
func TestForcedDrawRecordsOneLegalMove(t *testing.T) {
	g := &genome.Genome{
		Skeleton: genome.Shedding,
		Players:  2,
		HandSize: 1,
		Shedding: &genome.SheddingParams{MatchRule: genome.MatchEither, DrawPenalty: 1},
	}
	runner := fixedSetupRunner{
		GenericRunner: &shedding.Runner{},
		build: func() *sim.GameState {
			st := sim.NewGameState(2)
			top := card(sim.Hearts, sim.Seven)
			st.Discard = []sim.Card{top}
			st.TopCard = &top
			// P0 cannot play 9D on 7H (no suit or rank match): forced draw.
			st.Hands[0] = []sim.Card{card(sim.Diamonds, sim.Nine)}
			// P1 holds a rank match and wins on their turn.
			st.Hands[1] = []sim.Card{card(sim.Diamonds, sim.Seven)}
			st.Deck = []sim.Card{card(sim.Clubs, sim.Two), card(sim.Clubs, sim.Three)}
			st.Phase = sim.PhasePlay
			return st
		},
	}

	result := sim.RunBatch(g, runner, &sim.RandomAI{}, 1, 1)
	turns := result.AllTurns[0]
	if len(turns) == 0 {
		t.Fatal("no turn records")
	}
	first := turns[0]
	if first.Player != 0 {
		t.Errorf("first record Player = %d, want 0", first.Player)
	}
	if first.LegalMoves != 1 {
		t.Errorf("forced draw: LegalMoves = %d, want 1", first.LegalMoves)
	}
	// The draw leaves the discard top unchanged, so P1's options are
	// untouched: 7D matched rank-7 before and after. Delta is 0.
	if first.OptionDelta != 0 {
		t.Errorf("forced draw OptionDelta = %d, want 0 (top card unchanged)", first.OptionDelta)
	}
}

// TestSheddingOptionDeltaHandComputed verifies the shedding OptionDelta
// definition from the Task 7 table: options(next) = legal plays+draw for the
// next player against the discard top, before vs after the move.
//
// Hand-computed: top is 7H. P1 holds {9S, 5H, 6H}: against 7H both hearts
// match by suit => 2 options. P0's only legal play is 7S (rank match). After
// it, the top is 7S and P1's only match is 9S (suit) => 1 option.
// OptionDelta = 1 - 2 = -1.
func TestSheddingOptionDeltaHandComputed(t *testing.T) {
	g := &genome.Genome{
		Skeleton: genome.Shedding,
		Players:  2,
		HandSize: 3,
		Shedding: &genome.SheddingParams{MatchRule: genome.MatchEither, DrawPenalty: 1},
	}
	runner := fixedSetupRunner{
		GenericRunner: &shedding.Runner{},
		build: func() *sim.GameState {
			st := sim.NewGameState(2)
			top := card(sim.Hearts, sim.Seven)
			st.Discard = []sim.Card{top}
			st.TopCard = &top
			// 7S matches rank; 2D matches nothing => exactly one legal move,
			// and the hand stays non-empty so the game continues.
			st.Hands[0] = []sim.Card{card(sim.Spades, sim.Seven), card(sim.Diamonds, sim.Two)}
			st.Hands[1] = []sim.Card{card(sim.Spades, sim.Nine), card(sim.Hearts, sim.Five), card(sim.Hearts, sim.Six)}
			st.Deck = []sim.Card{card(sim.Clubs, sim.Three), card(sim.Clubs, sim.Four)}
			st.Phase = sim.PhasePlay
			return st
		},
	}

	result := sim.RunBatch(g, runner, &sim.RandomAI{}, 1, 1)
	turns := result.AllTurns[0]
	if len(turns) == 0 {
		t.Fatal("no turn records")
	}
	first := turns[0]
	if first.Player != 0 || first.LegalMoves != 1 {
		t.Fatalf("first record = %+v, want Player 0 with exactly 1 legal move", first)
	}
	if first.OptionDelta != -1 {
		t.Errorf("shedding OptionDelta = %d, want -1 (P1 went from 2 matches to 1)", first.OptionDelta)
	}
}

// TestRummyOptionDeltaHandComputed verifies the rummy OptionDelta definition
// from the Task 7 table: options(next) = legal draws+melds+discards for the
// next player (the union across the three turn phases), before vs after the
// turn-passing move. Deltas attach only to MoveDiscard -- draw/meld/pass keep
// the mover acting, and self-perturbation is not coupling, so those record 0.
//
// Hand-computed for P1 = {2H, 2D, 7S} with MeldSets/min 3, DrawEither,
// knock at 10 (deadwood 2+2+7 = 11, so no knock; no 3-of-a-kind, so no melds):
//
//	before P0's discard (discard pile EMPTY): draw 1 (deck only) + meld 1
//	(pass only) + discard 3 = 5
//	after P0 discards a card:                 draw 2 (deck + discard) + 1 + 3 = 6
//
// OptionDelta = 6 - 5 = +1.
func TestRummyOptionDeltaHandComputed(t *testing.T) {
	g := &genome.Genome{
		Skeleton: genome.Rummy,
		Players:  2,
		HandSize: 3,
		Rummy: &genome.RummyParams{
			MeldTypes:      genome.MeldSets,
			MinMeldSize:    3,
			DrawFrom:       genome.DrawEither,
			KnockThreshold: 10,
		},
	}
	runner := fixedSetupRunner{
		GenericRunner: &rummy.Runner{},
		build: func() *sim.GameState {
			st := sim.NewGameState(2)
			st.Phase = sim.PhaseDiscard // P0 is mid-turn, about to discard
			st.Hands[0] = []sim.Card{card(sim.Hearts, sim.Five), card(sim.Clubs, sim.Nine)}
			st.Hands[1] = []sim.Card{card(sim.Hearts, sim.Two), card(sim.Diamonds, sim.Two), card(sim.Spades, sim.Seven)}
			st.Discard = nil // empty: P0's discard CREATES P1's discard-draw option
			st.Deck = []sim.Card{card(sim.Clubs, sim.King), card(sim.Clubs, sim.Queen), card(sim.Clubs, sim.Jack)}
			st.Melds = [][]sim.Card{}
			st.MeldOwner = []int{}
			return st
		},
	}

	result := sim.RunBatch(g, runner, &sim.RandomAI{}, 1, 1)
	turns := result.AllTurns[0]
	if len(turns) == 0 {
		t.Fatal("no turn records")
	}
	first := turns[0]
	if first.Player != 0 || first.LegalMoves != 2 {
		t.Fatalf("first record = %+v, want Player 0 with 2 legal discards", first)
	}
	if first.OptionDelta != 1 {
		t.Errorf("rummy OptionDelta = %d, want +1 (P1 union 5 -> 6)", first.OptionDelta)
	}

	// P1's next decision is the draw (2 legal moves: deck or discard). It
	// does not pass the turn, so it must record OptionDelta 0.
	if len(turns) < 2 {
		t.Fatal("expected a second turn record (P1's draw)")
	}
	second := turns[1]
	if second.Player != 1 || second.LegalMoves != 2 {
		t.Fatalf("second record = %+v, want Player 1 with 2 legal draws", second)
	}
	if second.OptionDelta != 0 {
		t.Errorf("self-handoff (draw) OptionDelta = %d, want 0 (not a coupling move)", second.OptionDelta)
	}
}

// TestTrickTakingLeadDeltaHandComputed verifies the AMENDED Task 7 table
// (2026-06-11, Wave D review): for trick-LEADING plays only -- the trick was
// empty when the move was chosen -- OptionDelta = legalMoves(next, post-lead)
// - len(next player's hand): the constraint the lead imposes on the follower.
// Follows and trick-completing plays stay 0.
//
// Hand-computed, 2 players, MustFollowSuit: P0 = {7H}, P1 = {9H, 5S}.
//
//	move 1: P0 LEADS 7H. P1 must follow hearts: only 9H is legal => 1 option
//	        against a 2-card hand. OptionDelta = 1 - 2 = -1.
//	move 2: P1 follows 9H, completing the trick (9H beats 7H, P1 wins it).
//	        Trick-completing play => OptionDelta 0 (and Attack = true).
//	move 3: P1 leads 5S. The "follower" P0 has an empty hand: 0 legal moves
//	        minus 0 cards => OptionDelta 0.
func TestTrickTakingLeadDeltaHandComputed(t *testing.T) {
	g := &genome.Genome{
		Skeleton: genome.TrickTaking,
		Players:  2,
		HandSize: 2,
		TrickTaking: &genome.TrickTakingParams{
			MustFollowSuit:  true,
			TrickScoring:    genome.ScorePerTrick,
			LeadRestriction: genome.LeadNone,
			RoundsPerGame:   1,
		},
		TrumpRule: genome.TrumpNone,
	}
	runner := fixedSetupRunner{
		GenericRunner: &tricktaking.Runner{},
		build: func() *sim.GameState {
			st := sim.NewGameState(2)
			st.Hands[0] = []sim.Card{card(sim.Hearts, sim.Seven)}
			st.Hands[1] = []sim.Card{card(sim.Hearts, sim.Nine), card(sim.Spades, sim.Five)}
			st.Phase = sim.PhaseTrick
			st.TrumpSuit = -1 // no trump (zero value would mean suit 0!)
			st.MaxRound = 1
			st.TrickCards = make([]sim.Card, 0, 2)
			st.TrickPlayers = make([]int, 0, 2)
			return st
		},
	}

	result := sim.RunBatch(g, runner, &sim.RandomAI{}, 1, 1)
	if result.Completions != 1 {
		t.Fatalf("fixture broken: want 1 completion, got %d (errors=%d timeouts=%d)",
			result.Completions, result.Errors, result.Timeouts)
	}
	turns := result.AllTurns[0]
	if len(turns) != 3 {
		t.Fatalf("fixture broken: want 3 turn records, got %d: %+v", len(turns), turns)
	}
	if turns[0].Player != 0 || turns[0].LegalMoves != 1 {
		t.Fatalf("move 1 = %+v, want P0 with 1 legal lead", turns[0])
	}
	if turns[0].OptionDelta != -1 {
		t.Errorf("lead OptionDelta = %d, want -1 (P1: 1 legal follow vs 2-card hand)", turns[0].OptionDelta)
	}
	if turns[1].OptionDelta != 0 {
		t.Errorf("trick-completing play OptionDelta = %d, want 0", turns[1].OptionDelta)
	}
	if !turns[1].Attack {
		t.Errorf("trick-completing play must record Attack = true")
	}
	if turns[2].OptionDelta != 0 {
		t.Errorf("lead into an empty hand OptionDelta = %d, want 0", turns[2].OptionDelta)
	}
}

// TestTrickTakingLeadDeltaGenomeLinked is the genome-linked gradient test
// (audit Wave D fix 4) on real whist batches: with MustFollowSuit=true, leads
// constrain followers, so nonzero (strictly negative) deltas must appear, and
// ONLY on lead positions (a record is a lead iff it is the game's first move
// or follows a trick-completing Attack record -- whist has no specials, so
// Attack marks exactly the trick completions). The same genome with
// MustFollowSuit=false and LeadRestriction=LeadNone has no follow rules left
// to bind, so every delta must be 0.
func TestTrickTakingLeadDeltaGenomeLinked(t *testing.T) {
	strict := seeds.Whist() // MustFollowSuit: true
	result := sim.RunBatch(strict, &tricktaking.Runner{}, &sim.RandomAI{}, 10, 11)

	nonzero := 0
	for gi, turns := range result.AllTurns {
		for j, tr := range turns {
			isLead := j == 0 || turns[j-1].Attack
			if !isLead && tr.OptionDelta != 0 {
				t.Fatalf("game %d turn %d: non-lead play has OptionDelta %d, want 0", gi, j, tr.OptionDelta)
			}
			if tr.OptionDelta > 0 {
				t.Fatalf("game %d turn %d: lead delta = +%d; lead constraints are never positive", gi, j, tr.OptionDelta)
			}
			if tr.OptionDelta != 0 {
				nonzero++
			}
		}
	}
	if nonzero == 0 {
		t.Fatal("MustFollowSuit=true whist must produce nonzero lead deltas (the genome-linked gradient)")
	}

	free := seeds.Whist()
	free.TrickTaking.MustFollowSuit = false
	free.TrickTaking.LeadRestriction = genome.LeadNone // no lead restriction either
	freeResult := sim.RunBatch(free, &tricktaking.Runner{}, &sim.RandomAI{}, 10, 11)
	for gi, turns := range freeResult.AllTurns {
		for j, tr := range turns {
			if tr.OptionDelta != 0 {
				t.Fatalf("game %d turn %d: free-play genome (no follow rules) has OptionDelta %d, want all-zero",
					gi, j, tr.OptionDelta)
			}
		}
	}
}

// TestBatchLeadersTrackArgmaxProgress verifies the Task 8 leader track by
// hand on the forced-draw fixture (g.HandSize = 1, so shedding Progress is
// 1 - hand/1 floored at 0):
//
//	move 0: P0 forced draw -> P0 holds 2 cards (progress 0, clamped), P1
//	        holds 1 (progress 0). Tied at the max => leader -1.
//	move 1: P1 plays 7D on 7H -> P1 empty (progress 1), P0 at 0 => leader 1,
//	        and P1 wins the game.
func TestBatchLeadersTrackArgmaxProgress(t *testing.T) {
	g := &genome.Genome{
		Skeleton: genome.Shedding,
		Players:  2,
		HandSize: 1,
		Shedding: &genome.SheddingParams{MatchRule: genome.MatchEither, DrawPenalty: 1},
	}
	runner := fixedSetupRunner{
		GenericRunner: &shedding.Runner{},
		build: func() *sim.GameState {
			st := sim.NewGameState(2)
			top := card(sim.Hearts, sim.Seven)
			st.Discard = []sim.Card{top}
			st.TopCard = &top
			// P0 cannot play 9D on 7H: forced draw.
			st.Hands[0] = []sim.Card{card(sim.Diamonds, sim.Nine)}
			// P1 holds a rank match and wins on their turn.
			st.Hands[1] = []sim.Card{card(sim.Diamonds, sim.Seven)}
			st.Deck = []sim.Card{card(sim.Clubs, sim.Two), card(sim.Clubs, sim.Three)}
			st.Phase = sim.PhasePlay
			return st
		},
	}

	result := sim.RunBatch(g, runner, &sim.RandomAI{}, 1, 1)
	want := []int8{-1, 1}
	if !reflect.DeepEqual(result.AllLeaders[0], want) {
		t.Errorf("Leaders = %v, want %v (tie after forced draw, then winner leads)", result.AllLeaders[0], want)
	}
	if result.WinCounts[1] != 1 {
		t.Fatalf("WinCounts = %v, want P1 to win (fixture broken)", result.WinCounts)
	}
}

// TestBatchStackedSpecialsAreOneAttackTurn (audit Wave D fix 3): a single
// played card can match several special rules at once -- skip + reverse +
// draw-two here -- and emit up to 3 attack events from ONE applied move.
// TurnRecord.Attack is set at record time from that move's events, so the
// stacked special is exactly one attack-flagged turn, never three.
func TestBatchStackedSpecialsAreOneAttackTurn(t *testing.T) {
	g := &genome.Genome{
		Skeleton: genome.Shedding,
		Players:  3,
		HandSize: 2,
		Shedding: &genome.SheddingParams{MatchRule: genome.MatchEither, DrawPenalty: 1},
		SpecialCards: []genome.SpecialCard{
			{Type: genome.SpecialSkip, ByRank: uint8(sim.Seven)},
			{Type: genome.SpecialReverse, ByRank: uint8(sim.Seven)},
			{Type: genome.SpecialDrawTwo, ByRank: uint8(sim.Seven)},
		},
	}
	runner := fixedSetupRunner{
		GenericRunner: &shedding.Runner{},
		build: func() *sim.GameState {
			st := sim.NewGameState(3)
			top := card(sim.Hearts, sim.Seven)
			st.Discard = []sim.Card{top}
			st.TopCard = &top
			// P0's only legal play is 7S (rank match); the 2D filler keeps the
			// hand non-empty so the game continues past the stacked special.
			st.Hands[0] = []sim.Card{card(sim.Spades, sim.Seven), card(sim.Diamonds, sim.Two)}
			st.Hands[1] = []sim.Card{card(sim.Hearts, sim.Nine), card(sim.Hearts, sim.Ten)}
			st.Hands[2] = []sim.Card{card(sim.Clubs, sim.Five), card(sim.Clubs, sim.Six)}
			st.Deck = []sim.Card{
				card(sim.Clubs, sim.Three), card(sim.Clubs, sim.Four),
				card(sim.Diamonds, sim.Five), card(sim.Diamonds, sim.Six),
			}
			st.Phase = sim.PhasePlay
			return st
		},
	}

	result := sim.RunBatch(g, runner, &sim.RandomAI{}, 1, 1)
	events := result.AllEvents[0]
	attackEvents := 0
	for _, e := range events {
		if sim.IsAttackEvent(e) {
			attackEvents++
		}
	}
	if attackEvents < 3 {
		t.Fatalf("premise broken: stacked 7S must emit >= 3 attack events (draw_two+skip+reverse), got %d", attackEvents)
	}

	turns := result.AllTurns[0]
	if len(turns) == 0 {
		t.Fatal("no turn records")
	}
	if !turns[0].Attack {
		t.Errorf("stacked-special move must record Attack = true")
	}
}

// TestBatchTrickTakingAttackTurnsMatchTrickWins (audit Wave D fix 3): in
// trick-taking, exactly one applied move completes each trick and emits
// EventTrickWon, so the number of attack-flagged TurnRecords per game must
// equal the number of EventTrickWon events; leads and mid-trick follows must
// record Attack = false.
func TestBatchTrickTakingAttackTurnsMatchTrickWins(t *testing.T) {
	g := seeds.Whist()
	result := sim.RunBatch(g, &tricktaking.Runner{}, &sim.RandomAI{}, 5, 7)

	for i, turns := range result.AllTurns {
		attackTurns := 0
		for _, tr := range turns {
			if tr.Attack {
				attackTurns++
			}
		}
		trickWins := 0
		for _, e := range result.AllEvents[i] {
			if e.Type == sim.EventTrickWon {
				trickWins++
			}
		}
		if trickWins == 0 {
			t.Fatalf("game %d: premise broken, whist must win tricks", i)
		}
		if attackTurns != trickWins {
			t.Errorf("game %d: attack-flagged turns = %d, want %d (one per EventTrickWon)", i, attackTurns, trickWins)
		}
	}
}

// timeoutRunner never ends: CheckEnd is always -1 and ApplyMove only bumps
// state.Turn, so every game exits via max_turns with Winner -1 -- while
// Progress reports player 0 as a STRICT leader on every sample. This is the
// phantom-winner trap (audit Wave D fix 1): arc attribution must come from
// the recorded batch winner, never from the final leader sample, or this
// winnerless game's arc gets credited to player 0.
type timeoutRunner struct{}

func (timeoutRunner) Setup(g *genome.Genome, rng *rand.Rand) *sim.GameState {
	st := sim.NewGameState(g.Players)
	st.Hands[0] = []sim.Card{{Suit: sim.Hearts, Rank: sim.Two}}
	st.Hands[1] = []sim.Card{{Suit: sim.Spades, Rank: sim.Three}}
	st.RNG = rng
	return st
}
func (timeoutRunner) Upkeep(state *sim.GameState, g *genome.Genome) {}
func (timeoutRunner) GenerateMoves(state *sim.GameState, g *genome.Genome) []sim.Move {
	return []sim.Move{{Type: sim.MovePass, PlayerID: state.Active}}
}
func (timeoutRunner) ApplyMove(state *sim.GameState, move sim.Move, g *genome.Genome) []sim.Event {
	state.Turn++
	state.NextPlayer()
	return nil
}
func (timeoutRunner) CheckEnd(state *sim.GameState, g *genome.Genome) int { return -1 }
func (timeoutRunner) Progress(state *sim.GameState, g *genome.Genome) []float64 {
	return []float64{1, 0} // player 0 strictly "leads" every sample, forever
}

// TestBatchAllWinnersParallel (audit Wave D fix 1): BatchResult.AllWinners
// must hold each game's REAL winner, parallel to AllEvents -- -1 for every
// non-completion (max_turns/stuck/no_moves), the CheckEnd winner otherwise.
func TestBatchAllWinnersParallel(t *testing.T) {
	// Timed-out games: a strict leader on every sample, but no winner.
	g := &genome.Genome{
		Skeleton: genome.Shedding,
		Players:  2,
		HandSize: 1, // MaxTurns = 1*2*10 = 20: fast timeout
		Shedding: &genome.SheddingParams{MatchRule: genome.MatchEither, DrawPenalty: 1},
	}
	result := sim.RunBatch(g, timeoutRunner{}, &sim.RandomAI{}, 3, 1)
	if result.Timeouts != 3 {
		t.Fatalf("premise broken: want 3 timeouts, got %d (errors=%d completions=%d)",
			result.Timeouts, result.Errors, result.Completions)
	}
	if len(result.AllWinners) != len(result.AllEvents) {
		t.Fatalf("len(AllWinners) = %d, len(AllEvents) = %d: must be parallel",
			len(result.AllWinners), len(result.AllEvents))
	}
	for i, w := range result.AllWinners {
		if w != -1 {
			t.Errorf("game %d timed out but AllWinners[%d] = %d, want -1", i, i, w)
		}
	}
	// The trap this guards: the leader track DOES end with a strict leader.
	track := result.AllLeaders[0]
	if len(track) == 0 || track[len(track)-1] != 0 {
		t.Fatalf("premise broken: timed-out game should end with strict leader 0, track tail %v", track)
	}

	// Completed game (forced-draw fixture from TestBatchLeadersTrackArgmaxProgress):
	// P1 wins, so AllWinners must record 1.
	gc := &genome.Genome{
		Skeleton: genome.Shedding,
		Players:  2,
		HandSize: 1,
		Shedding: &genome.SheddingParams{MatchRule: genome.MatchEither, DrawPenalty: 1},
	}
	runner := fixedSetupRunner{
		GenericRunner: &shedding.Runner{},
		build: func() *sim.GameState {
			st := sim.NewGameState(2)
			top := card(sim.Hearts, sim.Seven)
			st.Discard = []sim.Card{top}
			st.TopCard = &top
			st.Hands[0] = []sim.Card{card(sim.Diamonds, sim.Nine)}
			st.Hands[1] = []sim.Card{card(sim.Diamonds, sim.Seven)}
			st.Deck = []sim.Card{card(sim.Clubs, sim.Two), card(sim.Clubs, sim.Three)}
			st.Phase = sim.PhasePlay
			return st
		},
	}
	cres := sim.RunBatch(gc, runner, &sim.RandomAI{}, 1, 1)
	if cres.Completions != 1 {
		t.Fatalf("fixture broken: want 1 completion, got %d", cres.Completions)
	}
	if len(cres.AllWinners) != 1 || cres.AllWinners[0] != 1 {
		t.Fatalf("AllWinners = %v, want [1] (P1 wins the fixture)", cres.AllWinners)
	}
}

// TestSameDerivedSeedSameEventStream is the Execution-notes doc-test: per-game
// seeds derive as baseSeed+index, so game i of one batch must be byte-identical
// (events AND turn records) to game 0 of a batch whose baseSeed is shifted by
// i. Batch position must not leak into game outcomes.
func TestSameDerivedSeedSameEventStream(t *testing.T) {
	cases := []struct {
		name   string
		g      *genome.Genome
		runner sim.GenericRunner
	}{
		{"shedding", seeds.CrazyEights(), &shedding.Runner{}},
		{"rummy", seeds.GinRummy(), &rummy.Runner{}}, // exercises mid-game RNG reshuffles
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			const baseSeed, idx = 40, 3
			r1 := sim.RunBatch(tc.g, tc.runner, &sim.RandomAI{}, 5, baseSeed)
			r2 := sim.RunBatch(tc.g, tc.runner, &sim.RandomAI{}, 1, baseSeed+idx)
			if !reflect.DeepEqual(r1.AllEvents[idx], r2.AllEvents[0]) {
				t.Errorf("event streams differ between batch position %d (base %d) and position 0 (base %d)", idx, baseSeed, baseSeed+idx)
			}
			if !reflect.DeepEqual(r1.AllTurns[idx], r2.AllTurns[0]) {
				t.Errorf("turn records differ between batch position %d (base %d) and position 0 (base %d)", idx, baseSeed, baseSeed+idx)
			}
			if !reflect.DeepEqual(r1.AllLeaders[idx], r2.AllLeaders[0]) {
				t.Errorf("leader tracks differ between batch position %d (base %d) and position 0 (base %d)", idx, baseSeed, baseSeed+idx)
			}
		})
	}
}

// BenchmarkRunBatchShedding200 is the Task 7 instrumentation hot-loop gate:
// a 200-game shedding batch, compared before/after per-turn recording.
func BenchmarkRunBatchShedding200(b *testing.B) {
	g := seeds.CrazyEights()
	runner := &shedding.Runner{}
	ai := &sim.RandomAI{}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sim.RunBatch(g, runner, ai, 200, 12345)
	}
}

// BenchmarkRunBatchRummy20 tracks the rummy probe cost (rummy OptionDelta
// probes are the expensive ones: meld generation runs per probe).
func BenchmarkRunBatchRummy20(b *testing.B) {
	g := seeds.GinRummy()
	runner := &rummy.Runner{}
	ai := &sim.RandomAI{}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sim.RunBatch(g, runner, ai, 20, 12345)
	}
}

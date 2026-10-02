package grammar

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/sim"
)

// The 2026-10 bughunt found every "who wins a tie" decision resolved by absolute
// seat index (lowest seat, or -- banking -- highest seat), and rummy ending
// MID-TURN so one fixed seat was always scored on an 11-card hand. Under random
// play that handed seat 0 69% of 2-player rummy games. These tests pin the
// replacement: the game ends at a turn boundary, a rules-meaningful secondary
// criterion breaks most ties, and what is left is counted from the player who
// ended the game (or took the last trick/capture) -- never from seat 0.

func c(rank int, suit sim.Suit) sim.Card { return sim.Card{Rank: sim.Rank(rank), Suit: suit} }

// endState deals a real game (so every per-move-gen field is initialised) and
// hands it back for the test to overwrite into the terminal position it needs.
func endState(s GameSpec) *sim.GameState {
	return Runner{s}.Setup(rand.New(rand.NewPCG(7, 7)))
}

func mustWinner(t *testing.T, s GameSpec, gs *sim.GameState) int {
	t.Helper()
	w, done := Runner{s}.CheckEnd(gs)
	if !done {
		t.Fatalf("%s: constructed state is not terminal", s.Family())
	}
	return w
}

// TestRummyEndsAtTurnBoundary: deck-out used to fire the instant the last card
// was DRAWN, before that player's discard -- so one fixed seat (the last drawer:
// seat 1 at 2 players, 0 at 3, 3 at 4) was scored on Deal+1 cards in 200/200
// games. The game must end only after the discard, with every hand at Deal.
func TestRummyEndsAtTurnBoundary(t *testing.T) {
	for _, mods := range [][]Modifier{nil, {ModKnock}, {ModWild}, {ModWild, ModKnock}} {
		for _, players := range enumPlayers {
			s := GameSpec{Players: players, Deal: 10, Move: Rummy, End: DeckOut, Score: FewestDeadwood, Mods: mods}
			for seed := uint64(1); seed <= 60; seed++ {
				gs, w := playRandomGame(s, seed, nil)
				if w < 0 {
					t.Fatalf("%s seed %d: did not terminate", s, seed)
				}
				for p := 0; p < players; p++ {
					if len(gs.Hands[p]) != s.Deal {
						t.Fatalf("%s seed %d: game ended with seat %d holding %d cards, want %d (ended mid-turn)",
							s, seed, p, len(gs.Hands[p]), s.Deal)
					}
				}
			}
		}
	}
}

// TestRummyTieBreaksByDeadwoodPoints: equal stray-card COUNTS (about half of all
// random games once hands are equal-sized) used to go to seat 0. They now go to
// the lower stray-card point total, whichever seat holds it; an exact tie on both
// goes to the player who ended the game, then onward in turn order.
func TestRummyTieBreaksByDeadwoodPoints(t *testing.T) {
	s := GameSpec{Players: 2, Deal: 4, Move: Rummy, End: DeckOut, Score: FewestDeadwood}
	low := []sim.Card{c(2, sim.Clubs), c(4, sim.Diamonds), c(6, sim.Hearts), c(9, sim.Spades)}     // 4 stray, 21 points
	high := []sim.Card{c(13, sim.Clubs), c(12, sim.Diamonds), c(10, sim.Hearts), c(7, sim.Spades)} // 4 stray, 37 points
	if deadwood(low, -1) != deadwood(high, -1) {
		t.Fatal("test hands must tie on stray-card count")
	}
	for lowSeat := 0; lowSeat < 2; lowSeat++ {
		gs := endState(s)
		gs.Deck, gs.Phase = nil, sim.PhaseDraw
		gs.Hands[lowSeat], gs.Hands[1-lowSeat] = low, high
		if w := mustWinner(t, s, gs); w != lowSeat {
			t.Errorf("low-points hand in seat %d: winner = %d, want %d (tie must break on stray-card points, not seat)", lowSeat, w, lowSeat)
		}
	}
	// Exact tie on count AND points: the holder of the lowest card (here the 2 of
	// clubs, below the twin's 2 of spades) wins, whichever seat they are in and
	// whoever made the final discard. The last discarder is structurally a FIXED
	// seat at deck-out, so a turn-order rule here would just be seat bias again.
	twin := []sim.Card{c(2, sim.Spades), c(4, sim.Hearts), c(6, sim.Diamonds), c(9, sim.Clubs)}
	for lowSeat := 0; lowSeat < 2; lowSeat++ {
		for active := 0; active < 2; active++ {
			gs := endState(s)
			gs.Deck, gs.Phase = nil, sim.PhaseDraw
			gs.Hands[lowSeat], gs.Hands[1-lowSeat] = low, twin
			gs.Active = active
			if w := mustWinner(t, s, gs); w != lowSeat {
				t.Errorf("exact tie, lowest card in seat %d (active %d): winner = %d, want %d", lowSeat, active, w, lowSeat)
			}
		}
	}
}

// TestCountTieGoesToLastTaker: a tie on cards won (16% of 4-player trick games)
// used to go to the lowest seat. In a trick game it now goes to whoever won the
// last trick if they are among the tied, else onward in turn order from them.
func TestCountTieGoesToLastTaker(t *testing.T) {
	s := GameSpec{Players: 4, Deal: 13, Move: Trick, End: DeckOut, Score: MostCaptured}
	for _, tc := range []struct{ lastTaker, want int }{{2, 2}, {0, 0}, {1, 2}, {3, 0}} {
		gs := endState(s)
		for p := range gs.Hands {
			gs.Hands[p] = nil
		}
		gs.Scores = []int{16, 8, 16, 12} // seats 0 and 2 tie for most
		gs.TrickLeader = tc.lastTaker
		if w := mustWinner(t, s, gs); w != tc.want {
			t.Errorf("seats 0,2 tied, last trick to seat %d: winner = %d, want %d", tc.lastTaker, w, tc.want)
		}
	}
}

// TestCaptureTieGoesToHighestCard: a tied capture count used to go to the lowest
// seat. "Last to capture" is no fix -- the last seat plays the last card, so it
// is the last capturer far more often than anyone else -- so the tie goes to the
// tied player whose pile holds the highest card, wherever they sit.
func TestCaptureTieGoesToHighestCard(t *testing.T) {
	s := GameSpec{Players: 4, Deal: 4, Shared: 4, Move: Capture, End: DeckOut, Score: MostCaptured}
	withAce := []sim.Card{c(14, sim.Hearts), c(3, sim.Clubs)}
	withKing := []sim.Card{c(13, sim.Spades), c(13, sim.Hearts)}
	more := []sim.Card{c(14, sim.Spades)} // the highest card of all, but NOT tied for most
	for aceSeat := 0; aceSeat < 4; aceSeat++ {
		kingSeat, otherSeat := (aceSeat+2)%4, (aceSeat+1)%4
		gs := endState(s)
		gs.Deck = nil
		for p := range gs.Hands {
			gs.Hands[p], gs.Tableau[p], gs.Scores[p] = nil, nil, 0
		}
		gs.Tableau[aceSeat], gs.Scores[aceSeat] = withAce, 2
		gs.Tableau[kingSeat], gs.Scores[kingSeat] = withKing, 2
		gs.Tableau[otherSeat], gs.Scores[otherSeat] = more, 1
		if w := mustWinner(t, s, gs); w != aceSeat {
			t.Errorf("seats %d,%d tied on 2 cards, ace of hearts in seat %d's pile: winner = %d, want %d", aceSeat, kingSeat, aceSeat, w, aceSeat)
		}
	}
}

// TestTeamTieGoesToLastTrickTeam: a tied partnership result used to go to team 0
// (seats 0+2) unconditionally. It goes to the partnership that won the last trick.
func TestTeamTieGoesToLastTrickTeam(t *testing.T) {
	s := GameSpec{Players: 4, Deal: 13, Move: Trick, End: DeckOut, Score: MostCaptured, Mods: []Modifier{ModTeams}}
	for _, tc := range []struct{ lastTaker, want int }{{1, 3}, {3, 3}, {0, 0}, {2, 0}} {
		gs := endState(s)
		for p := range gs.Hands {
			gs.Hands[p] = nil
		}
		gs.Scores = []int{16, 12, 10, 14} // evens 26, odds 26; top seats 0 and 3
		gs.TrickLeader = tc.lastTaker
		if w := mustWinner(t, s, gs); w != tc.want {
			t.Errorf("teams tied 26-26, last trick to seat %d: winner = seat %d, want seat %d", tc.lastTaker, w, tc.want)
		}
	}
}

// TestBankingTieBreak: equal totals used to go to the HIGHEST seat (a '>=' scan):
// 468 of 4000 2-player games tied and seat 1 took all 468. A tie now goes to the
// player who reached the total with fewer cards, then to the holder of the
// highest card taken -- in either seat, whoever acted last (equal card counts
// mean equal action counts, so "last to act" is always the last seat). The
// everyone-busted case follows the same ladder.
func TestBankingTieBreak(t *testing.T) {
	s := GameSpec{Players: 2, Shared: 3, Move: Accumulate, Target: 21, End: Bust, Score: ClosestTarget}
	two := []sim.Card{c(10, sim.Clubs), c(13, sim.Diamonds)}                   // 20 in two cards, king high
	twoQ := []sim.Card{c(10, sim.Hearts), c(12, sim.Spades)}                   // 20 in two cards, queen high
	three := []sim.Card{c(5, sim.Clubs), c(7, sim.Diamonds), c(8, sim.Hearts)} // 20 in three cards
	bustA := []sim.Card{c(9, sim.Clubs), c(7, sim.Hearts), c(8, sim.Diamonds)} // 24, nine high
	bustB := []sim.Card{c(10, sim.Spades), c(6, sim.Hearts), c(8, sim.Clubs)}  // 24, ten high
	for _, tc := range []struct {
		name         string
		total        int
		winner, lose []sim.Card
	}{
		{"fewer cards", 20, two, three},
		{"same count, highest card", 20, two, twoQ},
		{"all bust, highest card", 24, bustB, bustA},
	} {
		for winSeat := 0; winSeat < 2; winSeat++ {
			for active := 0; active < 2; active++ {
				gs := endState(s)
				gs.Folded = []bool{true, true}
				gs.Scores = []int{tc.total, tc.total}
				gs.Tableau[winSeat], gs.Tableau[1-winSeat] = tc.winner, tc.lose
				gs.Active = active // the last player to act
				if w := mustWinner(t, s, gs); w != winSeat {
					t.Errorf("%s: %d-%d, deciding pile in seat %d (last to act %d): winner = %d, want %d",
						tc.name, tc.total, tc.total, winSeat, active, w, winSeat)
				}
			}
		}
	}
}

// TestBankingMustTakeBeforeSticking: sticking on an EMPTY pile was legal, so two
// players who both stuck at once tied 0-0 with nothing to compare -- one game in
// nine at 2 players under random play, all decided by seat order. A player must
// now take at least one card before sticking; stick stays the unconditional
// fallback when there is nothing to take, so the move set is still never empty.
func TestBankingMustTakeBeforeSticking(t *testing.T) {
	s := GameSpec{Players: 3, Shared: 3, Move: Accumulate, Target: 21, End: Bust, Score: ClosestTarget}
	r := Runner{s}
	canStick := func(gs *sim.GameState) bool {
		for _, m := range r.LegalMoves(gs) {
			if m.Type == sim.MovePass {
				return true
			}
		}
		return false
	}
	gs := endState(s)
	if canStick(gs) {
		t.Errorf("stick offered before any card was taken (moves %v)", r.LegalMoves(gs))
	}
	if n := len(r.LegalMoves(gs)); n != 2 {
		t.Errorf("opening move set has %d moves, want 2 (take face-up, take blind)", n)
	}
	opener := gs.Active
	r.Apply(gs, mv2(sim.MoveDraw, opener))
	gs.Active = opener
	if !gs.Folded[opener] && !canStick(gs) {
		t.Errorf("stick not offered after taking a card (moves %v)", r.LegalMoves(gs))
	}
	empty := endState(s)
	empty.Deck, empty.Discard = nil, nil
	if ms := r.LegalMoves(empty); len(ms) != 1 || ms[0].Type != sim.MovePass {
		t.Errorf("nothing left to take: moves = %v, want the stick fallback alone", ms)
	}
	for seed := uint64(0); seed < 500; seed++ { // and so no game ends with an empty-handed winner
		end, w := playRandomGame(s, seed, nil)
		if w < 0 || len(end.Tableau[w]) == 0 {
			t.Fatalf("seed %d: winner %d took no card", seed, w)
		}
	}
}

// TestFewestCardsTieCountsFromEnder: the knock / deadlock "fewest cards" rule
// broke ties toward seat 0. A tied knocker now wins their own knock, and a
// deadlock tie is counted from the last player who passed.
func TestFewestCardsTieCountsFromEnder(t *testing.T) {
	hand := func(n int, suit sim.Suit) []sim.Card {
		var h []sim.Card
		for r := 2; r < 2+n; r++ {
			h = append(h, c(r, suit))
		}
		return h
	}
	for _, mv := range []MoveGen{PlayMatch, BeatOrPass} {
		s := GameSpec{Players: 3, Deal: 7, Move: mv, Match: MatchEither, End: EmptyHand, Score: FirstOut, Mods: []Modifier{ModKnock}}
		for _, tc := range []struct{ knocker, want int }{{1, 1}, {0, 0}, {2, 0}} {
			gs := endState(s)
			gs.Hands[0], gs.Hands[1], gs.Hands[2] = hand(3, sim.Clubs), hand(3, sim.Diamonds), hand(5, sim.Hearts)
			if tc.knocker == 2 {
				gs.Hands[2] = hand(3, sim.Hearts) // a three-way tie: still the knocker's
				tc.want = 2
			}
			gs.Active = tc.knocker
			Runner{s}.Apply(gs, mv2(sim.MoveKnock, tc.knocker))
			if w := mustWinner(t, s, gs); w != tc.want {
				t.Errorf("%s: seats 0,1 tied on 3 cards, seat %d knocks: winner = %d, want %d", mv, tc.knocker, w, tc.want)
			}
		}
	}
	// Deadlock (deck empty, everyone passed): tie counted from the last passer.
	s := GameSpec{Players: 3, Deal: 7, Shared: 1, Move: PlayMatch, Match: MatchEither, End: EmptyHand, Score: FirstOut}
	for _, tc := range []struct{ lastPasser, want int }{{2, 2}, {0, 0}, {1, 2}} {
		gs := endState(s)
		gs.Deck = nil
		gs.Hands[0], gs.Hands[1], gs.Hands[2] = hand(3, sim.Clubs), hand(5, sim.Diamonds), hand(3, sim.Hearts)
		gs.PassCount = 3
		gs.Active = (tc.lastPasser + 1) % 3
		if w := mustWinner(t, s, gs); w != tc.want {
			t.Errorf("deadlock, seats 0,2 tied on 3 cards, last pass by seat %d: winner = %d, want %d", tc.lastPasser, w, tc.want)
		}
	}
}

func mv2(t sim.MoveType, p int) sim.Move { return sim.Move{Type: t, PlayerID: p} }

// primaryTied reports whether the finished game was level on its PRIMARY score
// criterion (most cards / fewest stray cards / closest total), i.e. whether the
// winner was named by a tie-break. Only the families where that happens often.
func primaryTied(s GameSpec, gs *sim.GameState) bool {
	n := gs.NumPlayers
	key := make([]int, n) // higher is better
	switch s.Move {
	case Trick, Capture:
		for p := 0; p < n; p++ {
			key[p] = Runner{s}.effScore(gs, p)
		}
	case Rummy:
		for p := 0; p < n; p++ {
			key[p] = -deadwood(gs.Hands[p], -1)
		}
	case Accumulate:
		anyAlive := false
		for p := 0; p < n; p++ {
			anyAlive = anyAlive || gs.Scores[p] <= s.Target
		}
		for p := 0; p < n; p++ {
			switch {
			case !anyAlive:
				key[p] = -gs.Scores[p] // everyone bust: least over
			case gs.Scores[p] > s.Target:
				key[p] = -1
			default:
				key[p] = gs.Scores[p]
			}
		}
	default:
		return false
	}
	best, count := key[0], 0
	for _, k := range key {
		if k > best {
			best = k
		}
	}
	for _, k := range key {
		if k == best {
			count++
		}
	}
	return count > 1
}

// TestNoFixedSeatAdvantage is the end-to-end check: 4000 uniform-random games of
// every canonical family at every player count. Random players have no skill, so
// seats should win about equally. Two arms:
//
//   - overall: each seat's win share is within tol of fair. What legitimately
//     remains is POSITIONAL -- seat 0 opens (sheds first, leads first, bets
//     first), worth up to ~5 points, and in capture the last seat plays onto the
//     fullest table (the dealer's edge, ~3 points; the old seat-0 tie rule used to
//     mask it). Before the fix rummy read 69/31 (2p), 24/52/23 (3p), 44/23/17/16
//     (4p) from the mid-turn end.
//   - tie-broken games only: among games level on the primary criterion, no seat
//     may take more than a fair share + tieTol. This is the arm that pins the tie
//     rules: before the fix seat 1 took 481 of 495 tied 2-player banking games
//     and seat 0 took 352 of 644 tied 4-player trick games (fair: 161).
func TestNoFixedSeatAdvantage(t *testing.T) {
	if testing.Short() {
		t.Skip("84k random games; run without -short")
	}
	const games = 4000
	const tol, tieTol = 0.065, 0.10
	for _, base := range Canonical() {
		for _, players := range enumPlayers {
			s := base
			s.Players = players
			wins, tieWins, ties := make([]int, players), make([]int, players), 0
			for seed := uint64(0); seed < games; seed++ {
				gs, w := playRandomGame(s, seed, nil)
				if w < 0 {
					t.Fatalf("%s seed %d: did not terminate", s, seed)
				}
				wins[w]++
				if primaryTied(s, gs) {
					tieWins[w]++
					ties++
				}
			}
			fair := 1 / float64(players)
			t.Logf("%-12s %dp wins by seat %v; %d tie-broken games won by seat %v", s.Move, players, wins, ties, tieWins)
			for p, n := range wins {
				if share := float64(n) / games; share > fair+tol || share < fair-tol {
					t.Errorf("%-12s %dp: seat %d wins %.1f%% (fair %.1f%%, +/-%.1f); by seat %v",
						s.Move, players, p, 100*share, 100*fair, 100*tol, wins)
				}
			}
			if ties < 100 {
				continue // too few tie-broken games for a share to mean anything
			}
			for p, n := range tieWins {
				if share := float64(n) / float64(ties); share > fair+tieTol {
					t.Errorf("%-12s %dp: seat %d wins %.1f%% of the %d tie-broken games (fair %.1f%%, +%.0f); by seat %v",
						s.Move, players, p, 100*share, ties, 100*fair, 100*tieTol, tieWins)
				}
			}
		}
	}
}

// TestRulebookStatesTieBreaks: every tie-break the runner applies is a rule of
// the game, so the rulebook (the judge's only view of the game) must state it.
func TestRulebookStatesTieBreaks(t *testing.T) {
	want := map[MoveGen][]string{
		PlayMatch:  {"no player can play", "fewest cards", "last player to pass"},
		Accumulate: {"fewer cards", "highest card", "went over by the least", "at least one card before"},
		Capture:    {"tie", "highest card"},
		Trick:      {"tie", "won the last trick"},
		Rummy:      {"point value", "aces count 1", "lowest card"},
		Vying:      {"exact tie", "highest card"},
	}
	for _, s := range Canonical() {
		rb := s.Rulebook("X")
		for _, phrase := range want[s.Move] {
			if !strings.Contains(rb, phrase) {
				t.Errorf("%s rulebook does not state its tie-break (missing %q)", s.Family(), phrase)
			}
		}
	}
	knock := GameSpec{Players: 3, Deal: 7, Shared: 1, Move: PlayMatch, Match: MatchEither, End: EmptyHand, Score: FirstOut, Mods: []Modifier{ModKnock}}
	if rb := knock.Rulebook("X"); !strings.Contains(rb, "tie goes to the knocker") {
		t.Errorf("knock rulebook does not say who wins a tied knock:\n%s", rb)
	}
	teams := GameSpec{Players: 4, Deal: 13, Move: Trick, End: DeckOut, Score: MostCaptured, Mods: []Modifier{ModTeams}}
	if rb := teams.Rulebook("X"); !strings.Contains(rb, "partnership that won the last trick") {
		t.Errorf("partnership rulebook does not state the tied-partnership rule")
	}
}

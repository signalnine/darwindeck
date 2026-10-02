package shedding

import (
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

func plainShedding(players int) *genome.Genome {
	return &genome.Genome{
		Skeleton: genome.Shedding, Players: players, HandSize: 7,
		Shedding: &genome.SheddingParams{MatchRule: genome.MatchEither, DrawPenalty: 1},
	}
}

// deadlock: no deck, a lone discard top (nothing to recycle), and no hand card
// matches the 9 of hearts by suit or rank.
func deadlock() *sim.GameState {
	st := sim.NewGameState(2)
	st.Hands[0] = []sim.Card{{Suit: sim.Spades, Rank: 2}, {Suit: sim.Clubs, Rank: 4}, {Suit: sim.Spades, Rank: 6}, {Suit: sim.Clubs, Rank: 7}}
	st.Hands[1] = []sim.Card{{Suit: sim.Spades, Rank: 3}, {Suit: sim.Diamonds, Rank: 5}, {Suit: sim.Clubs, Rank: 8}, {Suit: sim.Diamonds, Rank: 10}}
	top := sim.Card{Suit: sim.Hearts, Rank: 9}
	st.Discard = []sim.Card{top}
	st.TopCard = &top
	st.Direction = 1
	return st
}

// TestBlockedDetectsDeadlock: the rulebook says a shedding game with an
// exhausted deck where nobody can play "ends in a draw", but the runner never
// declares it -- both seats pass until the turn cap. Blocked reports that
// permanent all-pass position so a caller (the playtest session) can end the
// game the moment it arises.
func TestBlockedDetectsDeadlock(t *testing.T) {
	r := &Runner{}
	g := plainShedding(2)

	st := deadlock()
	if !r.Blocked(st, g) {
		t.Fatal("empty deck, nothing to recycle, nobody can play: the game is blocked")
	}
	if st.Active != 0 {
		t.Errorf("Blocked must not disturb the active player, Active = %d", st.Active)
	}
	// The position really is permanent: passes change nothing.
	for i := 0; i < 6; i++ {
		r.Upkeep(st, g)
		moves := r.GenerateMoves(st, g)
		if len(moves) != 1 || moves[0].Type != sim.MovePass {
			t.Fatalf("iteration %d: blocked position offered %v", i, moves)
		}
		r.ApplyMove(st, moves[0], g)
		if !r.Blocked(st, g) {
			t.Fatalf("iteration %d: a blocked position unblocked itself", i)
		}
	}

	// Not blocked: a card can still be drawn.
	st = deadlock()
	st.Deck = []sim.Card{{Suit: sim.Diamonds, Rank: 2}}
	if r.Blocked(st, g) {
		t.Error("a non-empty deck is not a blocked game")
	}
	// Not blocked: the discard pile can be recycled into a deck by Upkeep.
	st = deadlock()
	st.Discard = append([]sim.Card{{Suit: sim.Diamonds, Rank: 2}}, st.Discard...)
	if r.Blocked(st, g) {
		t.Error("a recyclable discard pile is not a blocked game")
	}
	// Not blocked: one player (not the active one) holds a playable card.
	st = deadlock()
	st.Hands[1][0] = sim.Card{Suit: sim.Hearts, Rank: 3}
	if r.Blocked(st, g) {
		t.Error("a player who can still play means the game is not blocked")
	}
	// Not blocked: a wild is always playable.
	wild := plainShedding(2)
	wild.SpecialCards = []genome.SpecialCard{{Type: genome.SpecialWild, ByRank: 8}}
	if r.Blocked(deadlock(), wild) {
		t.Error("a held wild means the game is not blocked")
	}
	// Not blocked: a knock is still on offer (hand of 3 or fewer).
	knock := plainShedding(2)
	knock.Borrowed = []genome.BorrowedMechanic{{Source: genome.Rummy, Mechanic: genome.MechKnock}}
	st = deadlock()
	st.Hands[0] = st.Hands[0][:2]
	if r.Blocked(st, knock) {
		t.Error("a player who may knock is not blocked")
	}
	// A finished game is not "blocked".
	st = deadlock()
	st.Hands[1] = nil
	if r.Blocked(st, g) {
		t.Error("a game with an empty hand is over, not blocked")
	}
}

// TestPlayableCountRespectsFollowSuit: under the follow_suit borrow a player
// holding the discard top's suit may ONLY play that suit (or a wild), so an
// off-suit rank match is not playable. PlayableCount (the per-card count behind
// the playable-share veto) used to count it, overstating what the player could
// actually play.
func TestPlayableCountRespectsFollowSuit(t *testing.T) {
	r := &Runner{}
	g := plainShedding(2)
	g.SpecialCards = []genome.SpecialCard{{Type: genome.SpecialWild, ByRank: 8}}
	st := sim.NewGameState(2)
	top := sim.Card{Suit: sim.Hearts, Rank: 9}
	st.Discard = []sim.Card{top}
	st.TopCard = &top
	st.Hands[0] = []sim.Card{
		{Suit: sim.Hearts, Rank: 4},   // follows suit
		{Suit: sim.Spades, Rank: 9},   // rank match, off suit
		{Suit: sim.Clubs, Rank: 9},    // rank match, off suit
		{Suit: sim.Diamonds, Rank: 8}, // wild
		{Suit: sim.Clubs, Rank: 2},    // no match
	}
	st.Hands[1] = []sim.Card{{Suit: sim.Spades, Rank: 3}}

	if got := r.PlayableCount(st, g); got != 4 {
		t.Fatalf("plain game: PlayableCount = %d, want 4 (suit match, two rank matches, wild)", got)
	}

	follow := plainShedding(2)
	follow.SpecialCards = g.SpecialCards
	follow.Borrowed = []genome.BorrowedMechanic{{Source: genome.TrickTaking, Mechanic: genome.MechFollowSuit}}
	if got := r.PlayableCount(st, follow); got != 2 {
		t.Errorf("follow_suit while holding the suit: PlayableCount = %d, want 2 (the heart and the wild)", got)
	}
	// It must agree with the single-card plays GenerateMoves offers.
	singles := 0
	for _, m := range r.GenerateMoves(st, follow) {
		if m.Type == sim.MovePlay && len(m.Cards) == 1 {
			singles++
		}
	}
	if singles != 2 {
		t.Fatalf("premise: GenerateMoves offers %d single plays under follow_suit, want 2", singles)
	}

	// Void in the suit: the obligation lifts and every match counts again.
	st.Hands[0][0] = sim.Card{Suit: sim.Diamonds, Rank: 3}
	if got := r.PlayableCount(st, follow); got != 3 {
		t.Errorf("follow_suit while void: PlayableCount = %d, want 3 (two rank matches and the wild)", got)
	}
}

// Package grammar is a de-risk prototype for a generative card-game grammar: a
// game is a composition of typed primitives (setup x turn x move-generator x
// end-condition x scoring), and ONE generic interpreter (runner.go) runs any
// valid composition. The bet it tests: if every move-generator carries an
// unconditional fallback move, GenerateMoves is never empty, so every valid
// composition is playable-by-construction -- the synthesis of v1 (generative)
// and v2 (playable-by-construction skeletons). The reachable family count is
// then the combinatorial product, not the ~6 hand-coded skeletons.
//
// This is NOT production: it reuses pkg/sim types (Card, GameState, Move) and a
// minimal random-AI harness to answer one question -- does a small grammar reach
// hundreds-thousands of families that actually run and terminate under random
// play? See cmd/grammar-proto.
package grammar

import (
	"fmt"
	"strings"
)

// MoveGen is the engine primitive -- the structurally-distinct way a turn
// produces moves. Each one ALWAYS offers a fallback so the move set is non-empty.
type MoveGen int

const (
	PlayMatch  MoveGen = iota // shed a hand card matching the discard top (suit/rank/either); fallback: draw
	BeatOrPass                // play a card out-ranking the table top, or pass; all-pass clears the table
	Accumulate                // take a card onto a running total toward Target, or stick; bust over Target
	Capture                   // play a card to capture equal-rank table cards, else trail (leave it face-up)
	moveGenCount
)

func (m MoveGen) String() string {
	return [...]string{"play_match", "beat_or_pass", "accumulate", "capture"}[m]
}

// MatchRule constrains PlayMatch (ignored by other generators).
type MatchRule int

const (
	MatchSuit MatchRule = iota
	MatchRank
	MatchEither
	matchRuleCount
)

func (m MatchRule) String() string { return [...]string{"suit", "rank", "either"}[m] }

// EndRule decides when the game (or a round) is over.
type EndRule int

const (
	EmptyHand EndRule = iota // a player emptied their hand
	DeckOut                  // hands empty and the deck is exhausted
	Bust                     // all players have stuck or busted (a banking round)
	endRuleCount
)

func (e EndRule) String() string {
	return [...]string{"empty_hand", "deck_out", "bust"}[e]
}

// ScoreRule decides the winner at end-of-game from a per-player signal.
type ScoreRule int

const (
	FirstOut      ScoreRule = iota // the player who emptied their hand
	FewestCards                    // fewest cards left
	ClosestTarget                  // highest running total not over Target
	MostCaptured                   // most cards captured
	HighScore                      // highest Scores[]
	scoreRuleCount
)

func (s ScoreRule) String() string {
	return [...]string{"first_out", "fewest_cards", "closest_target", "most_captured", "high_score"}[s]
}

// GameSpec is a full game as a composition of primitives. The "family" (what a
// hand-coded skeleton used to be) is the structural tuple (Move, Match, End,
// Score); HandSize/Deal/Shared/Target/RoundsN are the fine params.
type GameSpec struct {
	Players int
	Deal    int // cards dealt per player
	Shared  int // cards seeded face-up in the shared zone (market/discard)
	Move    MoveGen
	Match   MatchRule // only meaningful for PlayMatch
	Target  int       // only meaningful for Accumulate
	End     EndRule
	Score   ScoreRule
}

// WellTyped reports whether a spec's scoring signal is actually producible by its
// move + end -- the grammar's coherence type. Two physical rules, validated by the
// random-AI diagnostic (results/2026-06-23-grammar-prototype):
//
//  1. deck_out is only reachable when the move-gen drains the DECK (capture).
//     play_match / beat_or_pass empty HANDS -- the empty-hand->draw fallback
//     refills a spent hand, so the deck only drains via the stalemate path. They
//     pair only with empty_hand.
//  2. play_match needs rank-OR-suit matching to have agency; rank-only / suit-only
//     collapse to forced draws (agency-dead) absent a wild-card modifier.
//
// Tightening Enumerate to WellTyped specs is the grammar's promise made concrete:
// illegal compositions become unrepresentable, not caught at runtime.
func (s GameSpec) WellTyped() bool {
	switch s.Move {
	case PlayMatch:
		return s.Match == MatchEither && s.End == EmptyHand
	case BeatOrPass:
		return s.End == EmptyHand
	case Accumulate:
		return s.End == Bust
	case Capture:
		return s.End == DeckOut
	}
	return false
}

// Family is the structural identity (ignores fine params) -- the unit we count.
func (s GameSpec) Family() string {
	m := s.Move.String()
	if s.Move == PlayMatch {
		m += "/" + s.Match.String()
	}
	return fmt.Sprintf("%s|%s|%s", m, s.End, s.Score)
}

func (s GameSpec) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%dp deal=%d", s.Players, s.Deal)
	if s.Shared > 0 {
		fmt.Fprintf(&b, " shared=%d", s.Shared)
	}
	fmt.Fprintf(&b, " move=%s", s.Move)
	if s.Move == PlayMatch {
		fmt.Fprintf(&b, "(%s)", s.Match)
	}
	if s.Move == Accumulate {
		fmt.Fprintf(&b, " target=%d", s.Target)
	}
	fmt.Fprintf(&b, " end=%s score=%s", s.End, s.Score)
	return b.String()
}

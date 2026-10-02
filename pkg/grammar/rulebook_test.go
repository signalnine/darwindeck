package grammar

import (
	"regexp"
	"strings"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/sim"
)

// The rulebook is the blind judge's ONLY view of a game, so a sentence the
// runner does not play is a bug: the judge certifies (or rejects) a game that
// does not exist. Each subtest pins one place the 2026-10 bughunt found the text
// and the runner disagreeing -- it asserts what the RUNNER does, then that the
// text says exactly that.
func TestRulebookMatchesRunner(t *testing.T) {
	has := func(t *testing.T, rb string, wants ...string) {
		t.Helper()
		for _, w := range wants {
			if !strings.Contains(rb, w) {
				t.Errorf("rulebook is missing %q", w)
			}
		}
	}
	lacks := func(t *testing.T, rb string, bans ...string) {
		t.Helper()
		for _, b := range bans {
			if strings.Contains(rb, b) {
				t.Errorf("rulebook still claims %q", b)
			}
		}
	}
	shedding := Canonical()[0]

	t.Run("no_voluntary_draw", func(t *testing.T) {
		// Runner: a draw is offered ONLY when nothing can be played.
		for seed := uint64(1); seed <= 40; seed++ {
			playRandomGame(shedding, seed, func(gs *sim.GameState, moves []sim.Move) {
				play, draw := false, false
				for _, m := range moves {
					play = play || m.Type == sim.MovePlay
					draw = draw || m.Type == sim.MoveDraw
				}
				if play && draw {
					t.Fatalf("seed %d: runner offers a draw alongside a legal play", seed)
				}
			})
		}
		rb := shedding.Rulebook("X")
		lacks(t, rb, "choose not to")
		has(t, rb, "If you cannot play", "may not draw while you hold a playable card", "ends your turn")
	})

	t.Run("banking_top_card_only", func(t *testing.T) {
		s := Canonical()[2]
		gs := endState(s)
		if len(gs.Discard) != 3 {
			t.Fatalf("expected 3 face-up cards, have %d", len(gs.Discard))
		}
		takes := 0
		for _, m := range (Runner{s}).LegalMoves(gs) {
			if m.Type == sim.MovePlay {
				takes++
				if m.Cards[0] != gs.Discard[len(gs.Discard)-1] {
					t.Errorf("take-face-up offers %v, not the top face-up card %v", m.Cards[0], gs.Discard[len(gs.Discard)-1])
				}
			}
		}
		if takes != 1 {
			t.Fatalf("runner offers %d face-up takes from 3 face-up cards, want 1 (the top one)", takes)
		}
		rb := s.Rulebook("X")
		lacks(t, rb, "either a face-up card from the table")
		has(t, rb, "the TOP face-up card", "only the top one may be taken", "turned up from the deck in its place")
	})

	t.Run("capture_trail_optional", func(t *testing.T) {
		s := Canonical()[3]
		found := false
		for seed := uint64(0); seed < 50 && !found; seed++ {
			gs := Runner{s}.Setup(testRNG(seed))
			moves := Runner{s}.LegalMoves(gs)
			canCapture := map[sim.Card]bool{}
			for _, m := range moves {
				if m.Type == sim.MoveCapture {
					canCapture[m.Cards[0]] = true
					for _, tc := range m.Cards[1:] { // a capture takes EVERY table card of the rank
						if tc.Rank != m.Cards[0].Rank {
							t.Fatalf("capture %v takes a card of another rank", m.Cards)
						}
					}
					n := 0
					for _, tc := range gs.Discard {
						if tc.Rank == m.Cards[0].Rank {
							n++
						}
					}
					if n != len(m.Cards)-1 {
						t.Fatalf("capture %v leaves a same-rank card on the table %v", m.Cards, gs.Discard)
					}
				}
			}
			for _, m := range moves {
				if m.Type == sim.MovePlay && canCapture[m.Cards[0]] {
					found = true // the runner lets a capturing card be trailed instead
				}
			}
		}
		if !found {
			t.Fatal("runner never offered a trail for a card that could capture; the text below would be wrong")
		}
		rb := s.Rulebook("X")
		lacks(t, rb, "If it matches nothing, it stays face-up")
		has(t, rb, "ALL the table cards of that rank", "even one that could capture", "dealt 4 new cards", "belong to nobody")
	})

	t.Run("trick_win_rule_composes", func(t *testing.T) {
		trick := func(players int, mods ...Modifier) string {
			return GameSpec{Players: players, Deal: 13, Move: Trick, End: DeckOut, Score: MostCaptured, Mods: mods}.Rulebook("X")
		}
		plain := trick(4)
		lacks(t, plain, "penalty points", "bonus")
		has(t, plain, "most cards in tricks")
		has(t, trick(4, ModMeldBonus), "plus combination bonuses")
		lacks(t, trick(4, ModMeldBonus), "less any penalty", "minus penalty")
		has(t, trick(4, ModAvoidance), "minus penalty points")
		has(t, trick(4, ModBid, ModAvoidance), "contract score", "minus penalty points")
		has(t, trick(4, ModMeldBonus, ModBid), "contract score", "plus combination bonuses")
		has(t, trick(4, ModTeams), "added together", "partnership")
		// The bid rule must give the real numbers (contractScore).
		has(t, trick(4, ModBid), "10 points per trick bid", "1 point for each extra trick", "10 points for each trick short", "zero")
		if got := contractScore(5, 3); got != 32 {
			t.Errorf("contractScore(5 tricks, bid 3) = %d; the rulebook says 10 per trick bid + 1 per extra = 32", got)
		}
		if got := contractScore(1, 3); got != -20 {
			t.Errorf("contractScore(1 trick, bid 3) = %d; the rulebook says -10 per trick short = -20", got)
		}
		// The combination values must match meldBonus.
		has(t, trick(4, ModMeldBonus), "a pair scores 4", "5 per card", "scores 2", "3 per card")
		pile := []sim.Card{c(5, sim.Clubs), c(5, sim.Hearts), c(9, sim.Spades), c(10, sim.Spades), c(11, sim.Spades)}
		if got := meldBonus(pile); got != 4+9 {
			t.Errorf("meldBonus(pair + 3-run) = %d; the rulebook says 4 + 3 per card = 13", got)
		}
		// Less than a full deck is dealt at 2-3 players: no draw deck exists.
		has(t, trick(3), "13 cards are set aside")
		lacks(t, trick(3), "form the draw deck")
		lacks(t, trick(4), "form the draw deck", "set aside")
	})

	t.Run("run_play_maximal_only", func(t *testing.T) {
		// Runner: three fives offer the triple only (never a pair), and 4-5-6 of a
		// suit offers the whole run only (never 5-6).
		top := c(3, sim.Clubs)
		fives := []sim.Card{c(5, sim.Clubs), c(5, sim.Diamonds), c(5, sim.Hearts), c(9, sim.Spades)}
		if combos := comboPlays(fives, top, true, MatchEither, 0); len(combos) != 1 || len(combos[0].Cards) != 3 {
			t.Fatalf("three fives: combos = %v, want the single 3-card set", combos)
		}
		run := []sim.Card{c(4, sim.Diamonds), c(5, sim.Diamonds), c(6, sim.Diamonds)}
		combos := comboPlays(run, c(13, sim.Diamonds), true, MatchEither, 0)
		if len(combos) != 1 || len(combos[0].Cards) != 3 {
			t.Fatalf("4-5-6 of diamonds: combos = %v, want the single 3-card run", combos)
		}
		if last := combos[0].Cards[2]; last.Rank != 6 {
			t.Fatalf("a run is laid low to high; last card = %v, want the six", last)
		}
		s := shedding
		s.Mods = []Modifier{ModRunPlay}
		rb := s.Rulebook("X")
		has(t, rb, "ALL the cards you hold of one rank", "your LONGEST run", "At least one card of the group must match", "its highest card")
		lacks(t, rb, "you may play a SET of cards of the same rank, or a RUN of consecutive cards of the same suit, all in one turn")
	})

	t.Run("setup_and_deadlock", func(t *testing.T) {
		for _, i := range []int{1, 6} { // climbing and vying never draw
			rb := Canonical()[i].Rulebook("X")
			lacks(t, rb, "form the draw deck")
			has(t, rb, "set aside")
		}
		for _, i := range []int{0, 2, 3, 5} { // these do draw from a deck
			has(t, Canonical()[i].Rulebook("X"), "form the draw deck")
		}
		has(t, shedding.Rulebook("X"), "no player can play")
	})

	t.Run("climbing_lead_and_pass", func(t *testing.T) {
		s := Canonical()[1]
		gs := endState(s) // empty table: the leader must play, and cannot pass
		for _, m := range (Runner{s}).LegalMoves(gs) {
			if m.Type == sim.MovePass {
				t.Fatal("runner lets the leader pass on an empty table")
			}
		}
		rb := s.Rulebook("X")
		has(t, rb, "you must lead", "ace is highest")
	})

	t.Run("rummy_melds_and_wilds", func(t *testing.T) {
		// Runner: sets are formed FIRST, so 5C 5D 5H 6H 7H leaves two stray cards
		// (the run 5H-6H-7H is not chosen over the set).
		hand := []sim.Card{c(5, sim.Clubs), c(5, sim.Diamonds), c(5, sim.Hearts), c(6, sim.Hearts), c(7, sim.Hearts)}
		if d := deadwood(hand, -1); d != 2 {
			t.Fatalf("deadwood(set-or-run hand) = %d, want 2 (sets first)", d)
		}
		// Runner: a wild completes a PAIR or two ADJACENT suited cards -- not a gap.
		if d := deadwood([]sim.Card{c(5, sim.Clubs), c(7, sim.Clubs), c(8, sim.Hearts)}, wildRank); d != 2 {
			t.Fatalf("deadwood(5C 7C + wild) = %d, want 2 (a wild does not fill a gap)", d)
		}
		if d := deadwood([]sim.Card{c(5, sim.Clubs), c(6, sim.Clubs), c(8, sim.Hearts)}, wildRank); d != 0 {
			t.Fatalf("deadwood(5C 6C + wild) = %d, want 0", d)
		}
		s := Canonical()[5]
		has(t, s.Rulebook("X"), "sets are formed first")
		s.Mods = []Modifier{ModWild, ModKnock}
		rb := s.Rulebook("X")
		lacks(t, rb, "stands in for any card you need", "nearly gone")
		has(t, rb, "a pair, or two consecutive cards of one suit", "2 or fewer")
	})

	t.Run("sum_capture_is_one_group", func(t *testing.T) {
		// Runner: a sum capture is its OWN option (a group of 2+ number cards), not
		// combined with the same-rank capture.
		table := []sim.Card{c(7, sim.Clubs), c(3, sim.Hearts), c(4, sim.Spades), c(13, sim.Hearts)}
		opts := captureOptions(table, c(7, sim.Diamonds), true)
		if len(opts) != 2 || len(opts[0]) != 1 || len(opts[1]) != 2 {
			t.Fatalf("7 onto 7,3,4,K: options = %v, want [7C] and [3H 4S] as separate captures", opts)
		}
		s := Canonical()[3]
		s.Mods = []Modifier{ModSumCapture}
		has(t, s.Rulebook("X"), "INSTEAD", "two or more number cards", "one or the other")
	})

	t.Run("vying_raise_cap", func(t *testing.T) {
		s := Canonical()[6]
		gs := endState(s)
		gs.RaiseCount = maxRaises
		for _, m := range (Runner{s}).LegalMoves(gs) {
			if m.Type == sim.MoveRaise {
				t.Fatal("runner allows a raise past the cap")
			}
		}
		has(t, s.Rulebook("X"), "At most 4 raises")
	})
}

// gameNameWords matches vocabulary that names (or all but names) a published
// game. Suit names and generic mechanic words (trump, meld, trick, wild, raise,
// fold, stick, bust) are fine.
var gameNameWords = regexp.MustCompile(`(?i)\b(knock\w*|gin|poker|rummy|deadwood|crazy eights|uno|scopa|casino|blackjack|big two|president|whist|bridge|euchre|pinochle|tichu)\b`)

// TestRulebookIsNameBlind: the rulebook is written straight into blind novelty
// dossiers, so it must describe mechanics without naming the classic they come
// from. "KNOCK"/"knocking" appeared in every going-out family (33 of 137) and
// "five-card poker hand" in every betting one. Checked for every family at every
// player count, at the source (no downstream scrubber needed).
func TestRulebookIsNameBlind(t *testing.T) {
	leaky := 0
	for _, s := range allWellTypedSpecs() {
		if m := gameNameWords.FindString(s.Rulebook("G00")); m != "" {
			if leaky++; leaky <= 6 {
				t.Errorf("%s: rulebook uses %q", s, m)
			}
		}
	}
	if leaky > 6 {
		t.Errorf("... %d specs in total", leaky)
	}
	// The going-out rule and the hand ranking must still be THERE, in neutral words.
	out := GameSpec{Players: 2, Deal: 10, Move: Rummy, End: DeckOut, Score: FewestDeadwood, Mods: []Modifier{ModKnock}}
	if rb := out.Rulebook("G00"); !strings.Contains(rb, "DECLARE OUT") {
		t.Error("going-out rule is missing its neutral name (DECLARE OUT)")
	}
	if rb := Canonical()[6].Rulebook("G00"); !strings.Contains(rb, "full house") || !strings.Contains(rb, "straight flush") {
		t.Error("betting rulebook must spell out the hand ranking it no longer names")
	}
}

package output

import (
	"strings"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
)

func TestRulebookOmitsUnsupportedMechanics(t *testing.T) {
	// CanStack/PlayMultiple (shedding) and CanLayOff (rummy) were inert genome
	// bits no runner ever read, yet the rulebook advertised "play multiple
	// matching cards" and "extend existing melds". They were removed (dd-027);
	// guard against any rulebook re-introducing those claims.
	banned := []string{
		"play multiple matching cards",
		"extend existing melds",
	}
	allSeeds := []*genome.Genome{
		seeds.CrazyEights(), seeds.MauMau(),
		seeds.Whist(), seeds.Hearts(), seeds.Spades(), seeds.OhHell(),
		seeds.GinRummy(), seeds.KnockRummy(),
	}
	for _, g := range allSeeds {
		rb := GenerateRulebook(g)
		for _, phrase := range banned {
			if strings.Contains(rb, phrase) {
				t.Errorf("%s rulebook advertises unsupported mechanic %q", g.ID, phrase)
			}
		}
	}
}

func TestRulebookOmitsSpecialCardsForNonShedding(t *testing.T) {
	// The trick-taking and rummy runners never apply special-card effects, so
	// the rulebook must not advertise them on those skeletons (dd-24e).
	g := &genome.Genome{
		ID:       "tt-with-specials",
		Skeleton: genome.TrickTaking,
		Players:  4,
		HandSize: 13,
		TrickTaking: &genome.TrickTakingParams{
			MustFollowSuit:  true,
			TrickScoring:    genome.ScorePerTrick,
			LeadRestriction: genome.LeadNone,
			RoundsPerGame:   1,
		},
		SpecialCards: []genome.SpecialCard{{Type: genome.SpecialSkip, ByRank: 7}},
	}
	rb := GenerateRulebook(g)
	if strings.Contains(rb, "Special Cards") || strings.Contains(rb, "Skip the next player") {
		t.Errorf("trick-taking rulebook must not advertise special cards:\n%s", rb)
	}
}

func TestSpecialCardName(t *testing.T) {
	tests := []struct {
		name string
		sc   genome.SpecialCard
		want string
	}{
		{
			name: "rank-only renders as plural any-suit rank",
			sc:   genome.SpecialCard{Type: genome.SpecialSkip, ByRank: 7, BySuit: 0},
			want: "any 7",
		},
		{
			name: "rank and suit renders as singular rank-of-suit",
			sc:   genome.SpecialCard{Type: genome.SpecialSkip, ByRank: 7, BySuit: 4},
			want: "the 7 of Spades",
		},
		{
			name: "suit-only renders as any-of-suit",
			sc:   genome.SpecialCard{Type: genome.SpecialSkip, ByRank: 0, BySuit: 2},
			want: "any Diamond",
		},
		{
			name: "neither renders as any card",
			sc:   genome.SpecialCard{Type: genome.SpecialSkip, ByRank: 0, BySuit: 0},
			want: "any card",
		},
		{
			name: "Jack of Hearts",
			sc:   genome.SpecialCard{Type: genome.SpecialWild, ByRank: 11, BySuit: 3},
			want: "the J of Hearts",
		},
		{
			name: "any Club",
			sc:   genome.SpecialCard{Type: genome.SpecialReverse, ByRank: 0, BySuit: 1},
			want: "any Club",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := specialCardName(tc.sc)
			if got != tc.want {
				t.Errorf("specialCardName(%+v) = %q, want %q", tc.sc, got, tc.want)
			}
		})
	}
}

func TestSpecialCardNameDistinguishesSuitBoundFromAnySuit(t *testing.T) {
	anySeven := genome.SpecialCard{Type: genome.SpecialSkip, ByRank: 7, BySuit: 0}
	sevenOfSpades := genome.SpecialCard{Type: genome.SpecialSkip, ByRank: 7, BySuit: 4}
	if specialCardName(anySeven) == specialCardName(sevenOfSpades) {
		t.Errorf("any-7 and 7-of-Spades must produce distinct names; both returned %q",
			specialCardName(anySeven))
	}
}

func TestSpecialCardNameDistinguishesAllCardsFromSuitBound(t *testing.T) {
	anyCard := genome.SpecialCard{Type: genome.SpecialSkip, ByRank: 0, BySuit: 0}
	anyDiamond := genome.SpecialCard{Type: genome.SpecialSkip, ByRank: 0, BySuit: 2}
	if specialCardName(anyCard) == specialCardName(anyDiamond) {
		t.Errorf("any-card and any-Diamond must produce distinct names; both returned %q",
			specialCardName(anyCard))
	}
}

func TestSheddingRulebookDoesNotClaimFewestCardsTiebreak(t *testing.T) {
	// The shedding runner does NOT award the fewest-cards player on
	// deck-out; CheckEnd returns -1 (timeout) so the batch runner
	// classifies the game as a timeout. The rulebook must not claim
	// otherwise. See dd-73h.
	g := seeds.CrazyEights()
	rb := GenerateRulebook(g)

	if strings.Contains(rb, "fewest cards wins") {
		t.Errorf("shedding rulebook still claims 'fewest cards wins' on deck-out, but the runner returns -1 (timeout)")
	}
}

// --- Task 22: multi-round shedding round structure ---

func multiRoundSheddingGenome(borrow genome.MechanicType, source genome.SkeletonType) *genome.Genome {
	g := &genome.Genome{
		ID:       "multi-round-shedding",
		Skeleton: genome.Shedding,
		Players:  2,
		HandSize: 7,
		Shedding: &genome.SheddingParams{
			MatchRule:     genome.MatchEither,
			DrawPenalty:   1,
			RoundsPerGame: 3,
		},
		Borrowed: []genome.BorrowedMechanic{{Source: source, Mechanic: borrow}},
	}
	if borrow == genome.MechAvoidance {
		g.Scoring.CardPoints = []genome.CardScoring{{Suit: 3, Points: 1}}
	}
	return g
}

// TestSheddingRulebookRendersRoundStructure (Task 22 test c): a multi-round
// genome's rulebook must describe the round structure -- rounds played,
// round end on hand-empty, score banking, the highest-total win rule, and
// the FULL tiebreak chain CheckEnd actually applies: fewest cards in hand,
// then lowest seat (reviewer finding 8 -- the rulebook used to stop at
// fewest cards, leaving a rules hole human players would hit).
func TestSheddingRulebookRendersRoundStructure(t *testing.T) {
	rb := GenerateRulebook(multiRoundSheddingGenome(genome.MechMeldBonus, genome.Rummy))
	for _, phrase := range []string{
		"3 rounds",
		"ends the round",
		"highest total score",
		"fewest cards",
		"earliest in the turn order",
	} {
		if !strings.Contains(rb, phrase) {
			t.Errorf("multi-round rulebook missing %q:\n%s", phrase, rb)
		}
	}
	// The single-round win rule must NOT be claimed.
	if strings.Contains(rb, "The first player to play all their cards wins") {
		t.Errorf("multi-round rulebook still claims the single-round win rule:\n%s", rb)
	}
}

// TestSheddingRulebookAvoidanceExplainsPenalties: under MechAvoidance the
// banked points are penalties (negative), so "highest total" means fewest
// penalty points -- the rulebook must say so in player terms.
func TestSheddingRulebookAvoidanceExplainsPenalties(t *testing.T) {
	rb := GenerateRulebook(multiRoundSheddingGenome(genome.MechAvoidance, genome.TrickTaking))
	if !strings.Contains(rb, "fewest penalty points") {
		t.Errorf("avoidance multi-round rulebook does not explain penalty scoring:\n%s", rb)
	}
}

// TestSheddingRulebookSingleRoundUnchanged: no scoring borrow (or
// RoundsPerGame <= 1) keeps the original single-round text -- the runner's
// behavior is unchanged there and the rulebook must not invent rounds.
func TestSheddingRulebookSingleRoundUnchanged(t *testing.T) {
	cases := []*genome.Genome{
		seeds.CrazyEights(),
		func() *genome.Genome { // rounds param without a borrow: inert
			g := multiRoundSheddingGenome(genome.MechMeldBonus, genome.Rummy)
			g.Borrowed = nil
			return g
		}(),
		func() *genome.Genome { // borrow without rounds: single round
			g := multiRoundSheddingGenome(genome.MechMeldBonus, genome.Rummy)
			g.Shedding.RoundsPerGame = 1
			return g
		}(),
	}
	for i, g := range cases {
		rb := GenerateRulebook(g)
		if !strings.Contains(rb, "The first player to play all their cards wins") {
			t.Errorf("case %d: single-round rulebook lost the first-to-empty win rule:\n%s", i, rb)
		}
		if strings.Contains(rb, "ends the round") || strings.Contains(rb, "highest total score") {
			t.Errorf("case %d: single-round rulebook advertises multi-round structure:\n%s", i, rb)
		}
	}
}

// --- Task 28 round 3, commit 6: dead-rule text ---

// TestRulebookOmitsCardPointTableWhenDead (round 3 commit 6a): the card
// point table renders ONLY when something actually consumes
// Scoring.CardPoints -- trick-taking under card_points/avoidance scoring, or
// a LIVE MechAvoidance borrow (rummy host, or multi-round shedding host).
// The r2 flagship printed point tables under ScorePerTrick and on rummy
// genomes with no consumer at all: rules text describing zero behavior.
func TestRulebookOmitsCardPointTableWhenDead(t *testing.T) {
	deadCases := []*genome.Genome{
		// Trick-taking, per-trick scoring: CardPoints never read.
		{
			ID: "tt-per-trick-with-points", Skeleton: genome.TrickTaking, Players: 4, HandSize: 13,
			TrickTaking: &genome.TrickTakingParams{MustFollowSuit: true, TrickScoring: genome.ScorePerTrick, RoundsPerGame: 1},
			Scoring:     genome.ScoringConfig{CardPoints: []genome.CardScoring{{Rank: 11, Suit: 1, Points: 15}}},
		},
		// Rummy with no borrow: nothing reads CardPoints.
		seeds.PairMeldStockRummy(),
		// Single-round shedding with a scoring borrow: the borrow itself is
		// inert (nothing reads banked Scores), so the table is dead too.
		seeds.CatchAllSkipShedding(),
	}
	for _, g := range deadCases {
		rb := GenerateRulebook(g)
		if strings.Contains(rb, "Card Point Values") {
			t.Errorf("%s: rulebook prints a card-point table no rule consumes:\n%s", g.ID, rb)
		}
	}

	liveCases := []*genome.Genome{
		seeds.Hearts(),                  // TT avoidance scoring
		seeds.NoFollowAvoidanceTrick(),  // TT avoidance scoring (fixture)
	}
	for _, g := range liveCases {
		rb := GenerateRulebook(g)
		if !strings.Contains(rb, "Card Point Values") {
			t.Errorf("%s: live card-point table missing:\n%s", g.ID, rb)
		}
	}
}

// TestRulebookOmitsInertScoringBorrowAd (round 3 commit 6b): a scoring
// borrow on SINGLE-round shedding banks Scores nothing reads (the game ends
// at the first empty hand) -- the r2 rank05 advertised 'meld bonuses' that
// could never matter. The rulebook must not advertise it; the multi-round
// case keeps the advertisement.
func TestRulebookOmitsInertScoringBorrowAd(t *testing.T) {
	inert := multiRoundSheddingGenome(genome.MechMeldBonus, genome.Rummy)
	inert.ID = "single-round-meld-borrow"
	inert.Shedding.RoundsPerGame = 1
	rb := GenerateRulebook(inert)
	if strings.Contains(rb, "bonus points for forming sets or runs") || strings.Contains(rb, "Additional Rules") {
		t.Errorf("single-round shedding rulebook advertises an inert scoring borrow:\n%s", rb)
	}

	live := multiRoundSheddingGenome(genome.MechMeldBonus, genome.Rummy)
	rbLive := GenerateRulebook(live)
	if !strings.Contains(rbLive, "bonus points for forming sets or runs") {
		t.Errorf("multi-round shedding rulebook lost its live borrow text:\n%s", rbLive)
	}
}

// TestRulebookNoDeadRuleTextAcrossAllFixtures (round 3 commit 6c): render
// every seed and every fixture rulebook and assert no dead-rule text, with
// the assertions derived from each genome's own liveness properties --
// extending TestRulebookOmitsUnsupportedMechanics' pattern from two banned
// phrases to the genome-conditional ones.
func TestRulebookNoDeadRuleTextAcrossAllFixtures(t *testing.T) {
	all := seeds.All()
	all = append(all, seeds.InstantKnockRummy(), seeds.ForcedShedding())
	all = append(all, seeds.RejectedChampions()...)
	all = append(all, seeds.CatchAllChampions()...)

	for _, g := range all {
		rb := GenerateRulebook(g)

		// Card-point table only with a live consumer.
		ttPoints := g.Skeleton == genome.TrickTaking && g.TrickTaking != nil &&
			(g.TrickTaking.TrickScoring == genome.ScoreCardPoints || g.TrickTaking.TrickScoring == genome.ScoreAvoidance)
		avoidanceLive := false
		for _, bm := range g.Borrowed {
			if bm.Mechanic == genome.MechAvoidance && len(g.Scoring.CardPoints) > 0 &&
				(g.Skeleton == genome.Rummy || g.SheddingMultiRound()) {
				avoidanceLive = true
			}
		}
		if !ttPoints && !avoidanceLive && strings.Contains(rb, "Card Point Values") {
			t.Errorf("%s: dead card-point table rendered", g.ID)
		}

		// Scoring borrows advertised only when they can affect the outcome.
		if g.Skeleton == genome.Shedding && !g.SheddingMultiRound() {
			for _, phrase := range []string{
				"bonus points for forming sets or runs",
				"penalty points — avoid collecting",
			} {
				if strings.Contains(rb, phrase) {
					t.Errorf("%s: single-round shedding rulebook advertises inert scoring borrow text %q", g.ID, phrase)
				}
			}
		}
	}
}

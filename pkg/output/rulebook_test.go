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

// TestClimbingRulebookRendersRules: a climbing genome's rulebook must contain a
// "How to Play" climbing section describing the beat-or-pass loop and the
// enabled combination types (a published climbing champion must not ship a
// rulebook missing its core rules).
func TestClimbingRulebookRendersRules(t *testing.T) {
	rb := GenerateRulebook(seeds.BigTwo())
	for _, phrase := range []string{
		"climbing game",
		"current combination",
		"Pass",
		"Pair",
		"Triple",
		"Run",
		"first player to empty",
	} {
		if !strings.Contains(rb, phrase) {
			t.Errorf("climbing rulebook missing %q\n---\n%s", phrase, rb)
		}
	}
}

// TestCasinoRulebookRendersRules: a casino genome's rulebook must describe the
// fishing/capture core loop (capture-or-trail, the sum rule, the final sweep)
// and -- for the plain seed -- the most-cards win. Regression for the gap where
// casino games rendered no "How to Play" section at all (the switch had no
// Casino case).
func TestCasinoRulebookRendersRules(t *testing.T) {
	rb := GenerateRulebook(seeds.Casino())
	for _, phrase := range []string{
		"How to Play",
		"capture game",
		"Capture:",
		"Trail:",
		"sum",            // AllowSumCapture seed describes pip-sum capture
		"Final Sweep",    // refill + end-sweep section
		"most cards",     // unscored win condition
	} {
		if !strings.Contains(rb, phrase) {
			t.Errorf("casino rulebook missing %q\n---\n%s", phrase, rb)
		}
	}
}

// TestCasinoScoredRulebookDescribesScoring: a casino host carrying a scoring
// borrow must render the scored win condition (score = captures + bonuses) and
// the borrowed meld bonus, not the plain most-cards rule.
func TestCasinoScoredRulebookDescribesScoring(t *testing.T) {
	g := &genome.Genome{
		ID: "casino-scored-rb", Skeleton: genome.Casino, Players: 2, HandSize: 4,
		Casino:   &genome.CasinoParams{TableSize: 4, AllowSumCapture: true},
		Borrowed: []genome.BorrowedMechanic{{Source: genome.Rummy, Mechanic: genome.MechMeldBonus}},
	}
	rb := GenerateRulebook(g)
	if !strings.Contains(rb, "highest score wins") {
		t.Errorf("scored casino rulebook must describe a score-based win\n---\n%s", rb)
	}
	if strings.Contains(rb, "most cards** wins") {
		t.Errorf("scored casino rulebook must NOT claim the plain most-cards win\n---\n%s", rb)
	}
	if !strings.Contains(rb, "Meld bonus") {
		t.Errorf("scored casino rulebook must render the meld bonus borrow\n---\n%s", rb)
	}
}

// TestClimbingRulebookOmitsDisabledCombinations: a singles-only climbing genome
// must not advertise pairs/triples/runs.
func TestClimbingRulebookOmitsDisabledCombinations(t *testing.T) {
	g := &genome.Genome{
		ID: "singles-climb", Skeleton: genome.Climbing, Players: 2, HandSize: 5,
		Climbing: &genome.ClimbingParams{},
	}
	rb := GenerateRulebook(g)
	for _, phrase := range []string{"Pair:", "Triple:", "Run:"} {
		if strings.Contains(rb, phrase) {
			t.Errorf("singles-only climbing rulebook advertises disabled combination %q", phrase)
		}
	}
	if !strings.Contains(rb, "Single:") {
		t.Error("climbing rulebook must always describe singles")
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
	if strings.Contains(rb, "Meld bonus") || strings.Contains(rb, "Additional Rules") {
		t.Errorf("single-round shedding rulebook advertises an inert scoring borrow:\n%s", rb)
	}

	live := multiRoundSheddingGenome(genome.MechMeldBonus, genome.Rummy)
	rbLive := GenerateRulebook(live)
	if !strings.Contains(rbLive, "Meld bonus") {
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
	all = append(all, seeds.TrivialMeldChampions()...)

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
				"Meld bonus",
				"Penalty cards",
			} {
				if strings.Contains(rb, phrase) {
					t.Errorf("%s: single-round shedding rulebook advertises inert scoring borrow text %q", g.ID, phrase)
				}
			}
		}
	}
}

// TestBorrowedRulesDescribeConcreteMechanics (blind-novelty fix): the borrowed
// mechanic is the entire source of a cross-skeleton hybrid's novelty, so its
// rulebook text must state the CONCRETE rule (scoring magnitudes + trigger) the
// hook in pkg/mechanic/hooks.go implements -- not a generic "earn bonus points"
// blurb that left judges (human and LLM) unable to tell a real mechanical fusion
// from an undefined bolt-on. Each anchor is a behavioral fact that must survive
// in the text; the old generic phrases must never come back. This is the
// drift-guard for the borrowedDescription <-> hooks coupling.
func TestBorrowedRulesDescribeConcreteMechanics(t *testing.T) {
	cases := []struct {
		mech    genome.MechanicType
		anchors []string
	}{
		// Anchors cover EVERY magnitude/trigger the doc comment promises stays in
		// sync with the hooks, so a hook-side constant change cannot drift the
		// rulebook silently (the gap a review caught: partial magnitudes only).
		{genome.MechMeldBonus, []string{"Meld bonus", "5 points per card", "2 per card", "3 points per card", "1 per card", "run of 3 or more"}},
		{genome.MechAvoidance, []string{"Penalty cards", "lose points equal to", "Card Point Values"}},
		{genome.MechTrickScoring, []string{"Capture bonus", "captured the most cards", "equal to the number of cards", "split the bonus evenly"}},
		{genome.MechDrawPenalty, []string{"Draw penalty", "face card (Jack or higher)", "draw 1 extra card"}},
		{genome.MechRunPlay, []string{"Combination plays", "set of 2 or more cards of the same rank", "run of 2 or more consecutive"}},
	}
	bannedGeneric := []string{
		"Earn bonus points for forming sets or runs",
		"Score bonus points based on trick-like card combinations",
		"Draw extra cards as a penalty in certain situations",
		"Certain cards carry penalty points — avoid collecting them",
	}
	for _, c := range cases {
		desc := borrowedDescription(genome.BorrowedMechanic{Mechanic: c.mech})
		for _, a := range c.anchors {
			if !strings.Contains(desc, a) {
				t.Errorf("mechanic %v: description missing concrete anchor %q\n  got: %s", c.mech, a, desc)
			}
		}
		for _, bad := range bannedGeneric {
			if strings.Contains(desc, bad) {
				t.Errorf("mechanic %v: description still uses banned generic blurb %q", c.mech, bad)
			}
		}
	}
}

// TestTrickTakingRulebookDescribesRounds: the runner plays RoundsPerGame deals
// with cumulative scores, and mutation moves RoundsPerGame across 1-13. The
// rulebook once omitted the round structure entirely, so a human (or a blind
// judge -- dossiers embed this rulebook) evaluated a one-deal game while the
// engine's traces reflected seven.
func TestTrickTakingRulebookDescribesRounds(t *testing.T) {
	g := &genome.Genome{
		ID:       "multi-round-whist",
		Skeleton: genome.TrickTaking,
		Players:  4,
		HandSize: 13,
		TrickTaking: &genome.TrickTakingParams{
			MustFollowSuit: true,
			TrickScoring:   genome.ScorePerTrick,
			RoundsPerGame:  7,
		},
	}
	rb := GenerateRulebook(g)
	if !strings.Contains(rb, "7 deals") {
		t.Fatalf("multi-round trick-taking rulebook must state the deal count; got:\n%s", rb)
	}

	g.TrickTaking.RoundsPerGame = 1
	if rb := GenerateRulebook(g); strings.Contains(rb, "deals**") {
		t.Fatalf("single-round rulebook must not describe a rounds structure")
	}
}

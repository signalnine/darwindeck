package genome

import (
	"strings"
	"testing"
)

// Regression tests for the 2026-10 runner-vs-rulebook bughunt: parameters that
// validated but were inert (or lied to the rulebook) on their host.

func hasErrContaining(errs []string, sub string) bool {
	for _, e := range errs {
		if strings.Contains(e, sub) {
			return true
		}
	}
	return false
}

// TestValidateRejectsTrumpOnShedding: the shedding runner never reads
// TrumpRule, yet a trump rule validated there (unlike rummy / climbing /
// casino / vying, which reject it) and describe + the report's Quick Take
// advertised "trump cards" for a game with no trump.
func TestValidateRejectsTrumpOnShedding(t *testing.T) {
	for _, rule := range []TrumpRule{TrumpFixed, TrumpCut, TrumpLed} {
		g := sheddingHost(1)
		g.TrumpRule = rule
		g.Scoring.TrumpSuit = 4
		if errs := Validate(g); !hasErrContaining(errs, "trump rule not applicable to shedding skeleton") {
			t.Errorf("trump_rule=%s on shedding: want a 'not applicable to shedding' rejection, got %v", rule, errs)
		}
	}
	plain := sheddingHost(1)
	if errs := Validate(plain); len(errs) != 0 {
		t.Errorf("trump_rule=none shedding genome must stay valid, got %v", errs)
	}
}

// TestValidateRejectsReservedCardScoringEvent: no runner or hook consults
// CardScoring.Event (points are applied by the host's own scoring rule), so a
// non-zero event is an inert bit -- the LeadWinnerLeads / MechTrump precedent:
// the value stays nameable but validation rejects it.
func TestValidateRejectsReservedCardScoringEvent(t *testing.T) {
	base := func(ev ScoringEvent) *Genome {
		return &Genome{
			Skeleton: TrickTaking, Players: 4, HandSize: 13,
			TrickTaking: &TrickTakingParams{MustFollowSuit: true, TrickScoring: ScoreAvoidance, RoundsPerGame: 1},
			Scoring:     ScoringConfig{CardPoints: []CardScoring{{Suit: 3, Points: 1, Event: ev}}},
		}
	}
	if errs := Validate(base(ScoreOnTrickWin)); len(errs) != 0 {
		t.Fatalf("event=0 must stay valid, got %v", errs)
	}
	for _, ev := range []ScoringEvent{ScoreOnCapture, ScoreOnPlay, ScoreOnHandEnd, ScoringEvent(9)} {
		if errs := Validate(base(ev)); !hasErrContaining(errs, "event") {
			t.Errorf("card_points event=%d: want a reserved-event rejection, got %v", ev, errs)
		}
	}
}

// TestLiveLeadRestrictionNeedsTrump: canLead only restricts when a trump suit
// exists, so no_trump_until_broken under trump_rule=none is inert (the Hearts
// seed carries exactly this pair). The liveness predicate is what the rulebook
// renders from; the genome itself stays valid so the seed is not invalidated.
func TestLiveLeadRestrictionNeedsTrump(t *testing.T) {
	g := &Genome{
		Skeleton: TrickTaking, Players: 4, HandSize: 13,
		TrickTaking: &TrickTakingParams{MustFollowSuit: true, LeadRestriction: LeadNoTrumpUntilBroken, RoundsPerGame: 1},
	}
	if g.LiveLeadRestriction() {
		t.Error("no_trump_until_broken with trump_rule=none must not be live")
	}
	if errs := Validate(g); len(errs) != 0 {
		t.Errorf("the inert pair must stay Tier-0 valid (the Hearts seed carries it), got %v", errs)
	}
	for _, rule := range []TrumpRule{TrumpFixed, TrumpCut, TrumpLed} {
		h := g.Clone()
		h.TrumpRule = rule
		h.Scoring.TrumpSuit = 4
		if !h.LiveLeadRestriction() {
			t.Errorf("no_trump_until_broken with trump_rule=%s must be live", rule)
		}
	}
	none := g.Clone()
	none.TrumpRule = TrumpFixed
	none.Scoring.TrumpSuit = 4
	none.TrickTaking.LeadRestriction = LeadNone
	if none.LiveLeadRestriction() {
		t.Error("lead_restriction=none is never live")
	}
	if sheddingHost(1).LiveLeadRestriction() {
		t.Error("non-trick-taking genome reported a live lead restriction")
	}
}

// TestSheddingMultiRoundNeedsLiveBankingBorrow: an avoidance borrow without
// card_points banks nothing (applyAvoidance returns early), so it must not
// switch the shedding host into banked-score rounds -- that left a multi-round
// game with no score signal whose rulebook pointed at a pruned section.
func TestSheddingMultiRoundNeedsLiveBankingBorrow(t *testing.T) {
	avoid := BorrowedMechanic{Source: TrickTaking, Mechanic: MechAvoidance}
	dead := sheddingHost(3, avoid)
	if dead.SheddingMultiRound() {
		t.Error("avoidance borrow without card_points must not make shedding multi-round")
	}
	if got, want := dead.MaxTurns(), sheddingHost(1).MaxTurns(); got != want {
		t.Errorf("MaxTurns with a dead banking borrow = %d, want the single-round cap %d", got, want)
	}
	live := sheddingHost(3, avoid)
	live.Scoring.CardPoints = []CardScoring{{Suit: 3, Points: 1}}
	if !live.SheddingMultiRound() {
		t.Error("avoidance borrow with card_points must make shedding multi-round")
	}
	// A dead avoidance borrow next to a live banking borrow keeps the rounds.
	mixed := sheddingHost(3, avoid, BorrowedMechanic{Source: Rummy, Mechanic: MechMeldBonus})
	if !mixed.SheddingMultiRound() {
		t.Error("meld bonus alongside a dead avoidance borrow must still be multi-round")
	}
}

func vyingAvoidance(chips int) *Genome {
	return &Genome{
		Skeleton: Vying, Players: 2, HandSize: 5,
		Vying:    &VyingParams{StartingChips: chips, MinBet: 10, MaxRaises: 3, RoundsPerGame: 2},
		Scoring:  ScoringConfig{CardPoints: []CardScoring{{Suit: 3, Points: 20}}},
		Borrowed: []BorrowedMechanic{{Source: TrickTaking, Mechanic: MechAvoidance}},
	}
}

// TestValidateVyingAvoidanceSolvency: the stack-sufficiency rule must count
// the avoidance hook's worst-case showdown penalty, or a Tier-0-valid game can
// drive a stack below the bet (fold-only decisions, negative chips). Worst
// case per deal here: 10*(3+1) committed + five 20-point hearts = 140; two
// deals = 280.
func TestValidateVyingAvoidanceSolvency(t *testing.T) {
	if got := vyingAvoidance(80).VyingWorstCaseCommitment(); got != 280 {
		t.Fatalf("VyingWorstCaseCommitment = %d, want 280", got)
	}
	if errs := Validate(vyingAvoidance(80)); !hasErrContaining(errs, "worst-case total commitment 280") {
		t.Errorf("chips 80 (covers betting only): want a solvency rejection naming 280, got %v", errs)
	}
	if errs := Validate(vyingAvoidance(279)); len(errs) == 0 {
		t.Error("chips 279 must still be rejected")
	}
	if errs := Validate(vyingAvoidance(280)); len(errs) != 0 {
		t.Errorf("chips 280 covers betting plus penalties, got %v", errs)
	}
	// Without the borrow the bound is the betting commitment alone.
	plain := vyingAvoidance(80)
	plain.Borrowed = nil
	if got := plain.VyingWorstCaseCommitment(); got != 80 {
		t.Errorf("plain vying worst case = %d, want 80", got)
	}
	if errs := Validate(plain); len(errs) != 0 {
		t.Errorf("plain vying at the betting bound must stay valid, got %v", errs)
	}
	// Specificity is honored: one named 50-point card plus cheap hearts.
	mixed := vyingAvoidance(1000)
	mixed.Scoring.CardPoints = []CardScoring{{Suit: 3, Points: 2}, {Rank: 12, Suit: 4, Points: 50}}
	// Top five penalties: Qs=50 + four hearts at 2 = 58; (40+58)*2 = 196.
	if got := mixed.VyingWorstCaseCommitment(); got != 196 {
		t.Errorf("mixed-penalty worst case = %d, want 196", got)
	}
}

package evolution

import (
	"math/rand/v2"

	"github.com/darwindeck/darwindeck/pkg/genome"
)

// CrossoverWith dispatches crossover with the cross-skeleton flag threaded in.
// It is the single config-aware entry point the engines call:
//
//   - SAME-skeleton parents: ordinary uniform crossover (delegates to
//     Crossover), unaffected by the flag.
//   - DIFFERENT-skeleton parents, flag OFF: returns nil (the historical
//     hard-disabled behavior; the caller falls back to mutation).
//   - DIFFERENT-skeleton parents, flag ON: produces a HYBRID child
//     (hybridCrossover) -- novelty evolution.
//
// Crossover (the 3-arg form) is preserved as the same-skeleton-only operator
// so existing callers and tests keep their contract.
func CrossoverWith(a, b *genome.Genome, rng *rand.Rand, crossSkeleton bool) *genome.Genome {
	if a.Skeleton == b.Skeleton {
		return Crossover(a, b, rng)
	}
	if !crossSkeleton {
		return nil
	}
	return hybridCrossover(a, b, rng)
}

// hybridCrossover builds a cross-skeleton HYBRID child from two
// different-skeleton parents (novelty evolution). The child is parent A's
// skeleton and core params (a full clone of A), PLUS one ACTIVE borrowed
// mechanic characteristic of parent B's family, drawn from the cross-family
// options whose hooks actually fire and affect the winner. The borrow is made
// outcome-affecting (wired through the same banked-score / scoring path the
// shedding scoring-borrow uses) and the child is repaired to validity.
//
// Recombination, not mere mutation: the child inherits A's whole game and gains
// a B-family mechanic, so a shedding parent crossed with a trick-taking parent
// yields a shed-to-win game scored by tricks; a trick-taking parent crossed
// with a rummy/shedding parent yields a trick-taker with penalty-card scoring
// borrowed from the other family. The host (A's skeleton) is the deterministic
// frame and B contributes the cross-family mechanic; which parent is A vs B is
// the caller's choice (the engine picks parent order at random via tournament).
func hybridCrossover(a, b *genome.Genome, rng *rand.Rand) *genome.Genome {
	child := cloneGenome(a)
	child.ID = ""
	child.Generation = max(a.Generation, b.Generation)

	// Pick the cross-family ACTIVE mechanic the host may borrow to characterize
	// B's family. crossFamilyBorrow returns a (Source, Mechanic) whose hook
	// fires and affects the winner on this host, or ok=false when no such
	// borrow exists (then the child is just a clone of A, still a valid genome).
	if bm, ok := crossFamilyBorrow(child.Skeleton, b.Skeleton, rng); ok {
		// Skip if the host already carries this exact borrow (a parent could).
		dup := false
		for _, existing := range child.Borrowed {
			if existing.Source == bm.Source && existing.Mechanic == bm.Mechanic {
				dup = true
				break
			}
		}
		// Coherence nudge (Wave 2 step 3): softly discourage stacking a SECOND
		// banking-scoring borrow on a host that already has one -- the Wave-1
		// "muddled scoring pile-up". Soft (half the time), never a veto, and
		// only when a banking borrow already exists, so the headline single-
		// borrow hybrids and legitimately complex two-borrow games both survive.
		pileUp := isBankingMechanic(bm.Mechanic) && child.HasBankingBorrow() && rng.Float64() < 0.5
		if !dup && !pileUp && len(child.Borrowed) < 3 {
			child.Borrowed = append(child.Borrowed, bm)
			wireHybridBorrow(child, bm, a, b)
		}
	}

	// Reuse the same invariant repair as same-skeleton crossover so the child
	// is "valid in, valid out" even before the post-crossover Mutate runs.
	repairCrossoverInvariants(child, a, child, rng)

	return child
}

// crossFamilyBorrow returns an ACTIVE cross-family borrowed mechanic for a
// host skeleton that characterizes the other parent's family, drawn from the
// whitelisted options whose hooks actually fire and affect the winner. The
// returned Source is the OTHER family (recording the cross-family provenance);
// it must differ from the host (validation rejects borrowing from one's own
// skeleton). Returns ok=false when no cross-family active mechanic is
// available for this (host, other) pair.
//
// The table is intentionally narrower than validBorrows: it lists only the
// mechanics that make a genuinely NOVEL hybrid (a shed-to-win game scored by
// tricks; a trick-taker with cross-family penalty scoring; a rummy with
// trick-style capture scoring), preferring those sourced from the OTHER parent
// when possible. Every entry's hook is wired outcome-affecting by
// wireHybridBorrow.
func crossFamilyBorrow(host, other genome.SkeletonType, rng *rand.Rand) (genome.BorrowedMechanic, bool) {
	// Candidate cross-family mechanics per host, each tagged with the family it
	// best characterizes (the natural Source). Only whitelisted + hooked +
	// outcome-affecting mechanics appear here.
	type cand struct {
		mech   genome.MechanicType
		source genome.SkeletonType
	}
	byHost := map[genome.SkeletonType][]cand{
		genome.Shedding: {
			// shed-to-win scored by tricks (the headline hybrid)
			{genome.MechTrickScoring, genome.TrickTaking},
			// shedding with rummy meld bonuses
			{genome.MechMeldBonus, genome.Rummy},
			// shedding with penalty-card avoidance scoring
			{genome.MechAvoidance, genome.TrickTaking},
		},
		genome.TrickTaking: {
			// trick-taker with cross-family penalty-card scoring
			{genome.MechAvoidance, genome.Shedding},
			// trick-taker collecting rummy meld bonuses from captures
			{genome.MechMeldBonus, genome.Rummy},
		},
		genome.Rummy: {
			// rummy with trick-style capture scoring
			{genome.MechTrickScoring, genome.TrickTaking},
			// rummy with penalty-card avoidance scoring
			{genome.MechAvoidance, genome.TrickTaking},
			// rummy with shedding-style draw-penalty bursts (Wave 2 diversity
			// fix): whitelisted (validBorrows[Rummy][MechDrawPenalty]) and
			// hooked (applyDrawPenalty fires on face-card discards in the rummy
			// runner -- the borrow the pre-Wave-2 cross table omitted, leaving
			// rummy hosts unable to reach a whole novel direction).
			{genome.MechDrawPenalty, genome.Shedding},
		},
	}

	cands := byHost[host]
	if len(cands) == 0 {
		return genome.BorrowedMechanic{}, false
	}

	// Prefer a candidate naturally sourced from the OTHER parent's family so
	// the hybrid genuinely combines the two families; fall back to any
	// candidate (still cross-family relative to the host).
	var preferred []cand
	for _, c := range cands {
		if c.source == other && c.source != host {
			preferred = append(preferred, c)
		}
	}
	pick := func(list []cand) genome.BorrowedMechanic {
		c := list[rng.IntN(len(list))]
		src := c.source
		if src == host {
			// Defensive: never borrow from own skeleton. Record the other
			// parent's family instead (guaranteed != host since the parents
			// differ).
			src = other
		}
		return genome.BorrowedMechanic{Source: src, Mechanic: c.mech}
	}
	if len(preferred) > 0 {
		return pick(preferred), true
	}
	// No candidate is sourced from the other family for this host; still emit a
	// cross-family borrow but stamp its Source as the other parent's family so
	// the provenance is honest and validation (no self-borrow) is satisfied.
	c := cands[rng.IntN(len(cands))]
	src := other
	if src == host {
		return genome.BorrowedMechanic{}, false
	}
	return genome.BorrowedMechanic{Source: src, Mechanic: c.mech}, true
}

// wireHybridBorrow makes a freshly-added hybrid borrow OUTCOME-AFFECTING,
// mirroring the coherent-mutation coupling addBorrowedMechanic applies:
//
//   - On a shedding host, any banking borrow (MechMeldBonus / MechAvoidance /
//     MechTrickScoring) needs RoundsPerGame >= 2 so the banked scores get a
//     winner signal (SheddingMultiRound reads them in CheckEnd). Without this
//     the borrow banks into state.Scores that nothing ever reads.
//   - A borrow whose hook reads CardPoints (MechAvoidance) no-ops without them;
//     seed a default penalty rule (Hearts worth 1), preferring a non-empty
//     CardPoints slice from either parent.
func wireHybridBorrow(child *genome.Genome, bm genome.BorrowedMechanic, a, b *genome.Genome) {
	if child.Skeleton == genome.Shedding && child.Shedding != nil {
		switch bm.Mechanic {
		case genome.MechMeldBonus, genome.MechAvoidance, genome.MechTrickScoring:
			if child.Shedding.RoundsPerGame < 2 {
				switch {
				case a.Shedding != nil && a.Shedding.RoundsPerGame >= 2:
					child.Shedding.RoundsPerGame = a.Shedding.RoundsPerGame
				case b.Shedding != nil && b.Shedding.RoundsPerGame >= 2:
					child.Shedding.RoundsPerGame = b.Shedding.RoundsPerGame
				default:
					child.Shedding.RoundsPerGame = 2
				}
			}
		}
	}
	if bm.Mechanic == genome.MechAvoidance && len(child.Scoring.CardPoints) == 0 {
		switch {
		case len(a.Scoring.CardPoints) > 0:
			child.Scoring.CardPoints = append([]genome.CardScoring(nil), a.Scoring.CardPoints...)
		case len(b.Scoring.CardPoints) > 0:
			child.Scoring.CardPoints = append([]genome.CardScoring(nil), b.Scoring.CardPoints...)
		default:
			child.Scoring.CardPoints = []genome.CardScoring{{Suit: 3, Points: 1}}
		}
	}
}

// Crossover performs uniform crossover between two same-skeleton genomes.
// Each parameter comes from parent A or B with 50/50 probability.
// Returns nil if skeletons don't match.
func Crossover(a, b *genome.Genome, rng *rand.Rand) *genome.Genome {
	if a.Skeleton != b.Skeleton {
		return nil // No cross-skeleton crossover
	}

	child := cloneGenome(a)
	child.ID = ""
	// Mutate (called by the caller after crossover) is responsible for the
	// final Generation++. Set the base to the higher parent generation so
	// the post-Mutate value is max(a, b) + 1.
	child.Generation = max(a.Generation, b.Generation)

	// Shared params: coin flip each
	if rng.Float64() < 0.5 {
		child.Players = b.Players
	}
	if rng.Float64() < 0.5 {
		child.HandSize = b.HandSize
	}
	if rng.Float64() < 0.5 {
		child.TrumpRule = b.TrumpRule
	}

	// Skeleton-specific crossover
	switch child.Skeleton {
	case genome.Shedding:
		crossoverShedding(child, a, b, rng)
	case genome.TrickTaking:
		crossoverTrickTaking(child, a, b, rng)
	case genome.Rummy:
		crossoverRummy(child, a, b, rng)
	}

	// Special cards: take from one parent. Only shedding consumes them, and
	// crossover is same-skeleton, so a non-shedding child must never inherit
	// them (dd-24e).
	if child.Skeleton == genome.Shedding && rng.Float64() < 0.5 {
		child.SpecialCards = cloneSpecialCards(b.SpecialCards)
	}

	// Borrowed: take from one parent
	if rng.Float64() < 0.5 {
		child.Borrowed = cloneBorrowed(b.Borrowed)
	}

	// Scoring: take from one parent. Deep-copy the CardPoints slice so
	// subsequent in-place mutation on the child does not alias parent B's
	// backing array (parent A is already deep-copied via cloneGenome).
	if rng.Float64() < 0.5 {
		child.Scoring = genome.ScoringConfig{
			CardPoints: append([]genome.CardScoring(nil), b.Scoring.CardPoints...),
			TrumpSuit:  b.Scoring.TrumpSuit,
		}
	}

	repairCrossoverInvariants(child, a, b, rng)

	return child
}

// repairCrossoverInvariants restores genome invariants that independent
// coin flips on related fields can violate. Without this, callers that
// run Crossover without a following Mutate (test code, future seeding
// paths) get children that genome.Validate rejects (see dd-kcp).
func repairCrossoverInvariants(child, a, b *genome.Genome, rng *rand.Rand) {
	// HandSize * Players must fit in a 52-card deck. The standard
	// engine path relies on Mutate to clamp this; mirror that loop
	// here so Crossover's contract is "valid in, valid out."
	for child.HandSize*child.Players > 52 {
		if rng.Float64() < 0.5 {
			child.HandSize--
		} else {
			child.Players--
		}
		if child.HandSize < 3 {
			child.HandSize = 3
		}
		if child.Players < 2 {
			child.Players = 2
		}
	}

	// TrumpFixed must be paired with a valid TrumpSuit (1-4). The
	// TrumpRule and Scoring coin flips are independent, so a child
	// can pick TrumpFixed from one parent and TrumpSuit=0 from the
	// other. Pull the suit from whichever parent contributed the
	// TrumpFixed rule; if neither did, downgrade the rule.
	if child.TrumpRule == genome.TrumpFixed &&
		(child.Scoring.TrumpSuit < 1 || child.Scoring.TrumpSuit > 4) {
		switch {
		case a.TrumpRule == genome.TrumpFixed && a.Scoring.TrumpSuit >= 1 && a.Scoring.TrumpSuit <= 4:
			child.Scoring.TrumpSuit = a.Scoring.TrumpSuit
		case b.TrumpRule == genome.TrumpFixed && b.Scoring.TrumpSuit >= 1 && b.Scoring.TrumpSuit <= 4:
			child.Scoring.TrumpSuit = b.Scoring.TrumpSuit
		default:
			child.TrumpRule = genome.TrumpNone
		}
	}

	// A scoring borrow on shedding requires multi-round play (round 3
	// commit 6b): the Borrowed and RoundsPerGame coin flips are independent,
	// so a child can take a scoring borrow from one parent and single-round
	// play from the other -- the inert rank05 combination (scores banked at
	// round end that nothing ever reads). Same coupling as
	// addBorrowedMechanic; an avoidance borrow additionally needs the
	// CardPoints its hook reads.
	if child.Skeleton == genome.Shedding && child.Shedding != nil && child.HasScoringBorrow() {
		if child.Shedding.RoundsPerGame < 2 {
			switch {
			case a.Shedding != nil && a.Shedding.RoundsPerGame >= 2:
				child.Shedding.RoundsPerGame = a.Shedding.RoundsPerGame
			case b.Shedding != nil && b.Shedding.RoundsPerGame >= 2:
				child.Shedding.RoundsPerGame = b.Shedding.RoundsPerGame
			default:
				child.Shedding.RoundsPerGame = 2
			}
		}
	}
	for _, bm := range child.Borrowed {
		if bm.Mechanic == genome.MechAvoidance && len(child.Scoring.CardPoints) == 0 {
			switch {
			case len(a.Scoring.CardPoints) > 0:
				child.Scoring.CardPoints = append([]genome.CardScoring(nil), a.Scoring.CardPoints...)
			case len(b.Scoring.CardPoints) > 0:
				child.Scoring.CardPoints = append([]genome.CardScoring(nil), b.Scoring.CardPoints...)
			default:
				child.Scoring.CardPoints = []genome.CardScoring{{Suit: 3, Points: 1}}
			}
		}
	}

	// ScoreCardPoints / ScoreAvoidance require non-empty CardPoints.
	// The TrickScoring coin flip lives in crossoverTrickTaking; the
	// CardPoints coin flip lives in the Scoring block above. They can
	// disagree (e.g. take Hearts' ScoreCardPoints with Whist's empty
	// CardPoints). Recover by adopting a non-empty CardPoints slice
	// from whichever parent had this scoring rule; otherwise fall
	// back to ScorePerTrick which has no card-points dependency.
	if child.Skeleton == genome.TrickTaking && child.TrickTaking != nil {
		ts := child.TrickTaking.TrickScoring
		needsPoints := ts == genome.ScoreCardPoints || ts == genome.ScoreAvoidance
		if needsPoints && len(child.Scoring.CardPoints) == 0 {
			switch {
			case a.TrickTaking != nil && a.TrickTaking.TrickScoring == ts && len(a.Scoring.CardPoints) > 0:
				child.Scoring.CardPoints = append([]genome.CardScoring(nil), a.Scoring.CardPoints...)
			case b.TrickTaking != nil && b.TrickTaking.TrickScoring == ts && len(b.Scoring.CardPoints) > 0:
				child.Scoring.CardPoints = append([]genome.CardScoring(nil), b.Scoring.CardPoints...)
			default:
				child.TrickTaking.TrickScoring = genome.ScorePerTrick
			}
		}
	}
}

func crossoverShedding(child *genome.Genome, a, b *genome.Genome, rng *rand.Rand) {
	if a.Shedding == nil || b.Shedding == nil {
		return
	}
	if rng.Float64() < 0.5 {
		child.Shedding.MatchRule = b.Shedding.MatchRule
	}
	if rng.Float64() < 0.5 {
		child.Shedding.DrawPenalty = b.Shedding.DrawPenalty
	}
	if rng.Float64() < 0.5 {
		child.Shedding.RoundsPerGame = b.Shedding.RoundsPerGame
	}
}

func crossoverTrickTaking(child *genome.Genome, a, b *genome.Genome, rng *rand.Rand) {
	if a.TrickTaking == nil || b.TrickTaking == nil {
		return
	}
	if rng.Float64() < 0.5 {
		child.TrickTaking.MustFollowSuit = b.TrickTaking.MustFollowSuit
	}
	if rng.Float64() < 0.5 {
		child.TrickTaking.TrickScoring = b.TrickTaking.TrickScoring
	}
	if rng.Float64() < 0.5 {
		child.TrickTaking.LeadRestriction = b.TrickTaking.LeadRestriction
	}
	if rng.Float64() < 0.5 {
		child.TrickTaking.RoundsPerGame = b.TrickTaking.RoundsPerGame
	}
}

func crossoverRummy(child *genome.Genome, a, b *genome.Genome, rng *rand.Rand) {
	if a.Rummy == nil || b.Rummy == nil {
		return
	}
	if rng.Float64() < 0.5 {
		child.Rummy.MeldTypes = b.Rummy.MeldTypes
	}
	if rng.Float64() < 0.5 {
		child.Rummy.MinMeldSize = b.Rummy.MinMeldSize
	}
	if rng.Float64() < 0.5 {
		child.Rummy.DrawFrom = b.Rummy.DrawFrom
	}
	if rng.Float64() < 0.5 {
		child.Rummy.KnockThreshold = b.Rummy.KnockThreshold
	}
}

func cloneSpecialCards(cards []genome.SpecialCard) []genome.SpecialCard {
	if cards == nil {
		return nil
	}
	out := make([]genome.SpecialCard, len(cards))
	copy(out, cards)
	return out
}

func cloneBorrowed(borrows []genome.BorrowedMechanic) []genome.BorrowedMechanic {
	if borrows == nil {
		return nil
	}
	out := make([]genome.BorrowedMechanic, len(borrows))
	copy(out, borrows)
	return out
}

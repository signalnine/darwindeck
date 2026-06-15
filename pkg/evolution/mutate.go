package evolution

import (
	"fmt"
	"math/rand/v2"

	"github.com/darwindeck/darwindeck/pkg/genome"
)

// Mutate applies random mutations to a genome copy with cross-skeleton borrows
// OFF (the historical default). Preserved as the signature every existing
// caller and test uses; delegates to MutateWith(..., false).
func Mutate(g *genome.Genome, rng *rand.Rand, allSeeds []*genome.Genome) *genome.Genome {
	return MutateWith(g, rng, allSeeds, false)
}

// MutateWith applies random mutations to a genome copy. When crossSkeleton is
// true, addBorrowedMechanic may also add the highest-novelty cross-family
// ACTIVE borrows (shedding -> MechTrickScoring, trick-taking -> MechAvoidance),
// so novelty is reachable via mutation and not only via crossover (novelty
// evolution). All other mutation behavior is identical.
// Multiple mutations can fire independently.
func MutateWith(g *genome.Genome, rng *rand.Rand, allSeeds []*genome.Genome, crossSkeleton bool) *genome.Genome {
	child := cloneGenome(g)
	child.ID = fmt.Sprintf("gen%d_%d", child.Generation+1, rng.IntN(100000))
	child.Generation++

	// Each mutation fires independently
	if rng.Float64() < 0.40 {
		tweakParameter(child, rng)
	}
	// Enforce hand_size * players <= 52 after player/hand mutations
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
	if rng.Float64() < 0.15 {
		flipBool(child, rng)
	}
	if rng.Float64() < 0.15 {
		changeEnum(child, rng)
	}
	if rng.Float64() < 0.08 {
		addSpecialCard(child, rng)
	}
	if rng.Float64() < 0.07 {
		removeSpecialCard(child, rng)
	}
	if rng.Float64() < 0.05 {
		addBorrowedMechanic(child, rng, crossSkeleton)
	}
	if rng.Float64() < 0.05 {
		removeBorrowedMechanic(child, rng)
	}
	if rng.Float64() < 0.02 {
		changeSkeleton(child, rng, allSeeds)
	}
	if rng.Float64() < 0.03 {
		mutateScoring(child, rng)
	}

	return child
}

func tweakParameter(g *genome.Genome, rng *rand.Rand) {
	// Shared params
	switch rng.IntN(2) {
	case 0:
		g.Players = clampInt(g.Players+rng.IntN(3)-1, 2, 6)
	case 1:
		g.HandSize = clampInt(g.HandSize+rng.IntN(3)-1, 3, 13)
	}

	// Skeleton-specific params
	switch g.Skeleton {
	case genome.Shedding:
		if g.Shedding != nil {
			// RoundsPerGame is mutable only when the genome carries a
			// scoring borrow: without one the field is inert
			// (genome.SheddingMultiRound is false regardless of its value),
			// and mutating it would burn mutation pressure on a no-op --
			// the repo's coherent-mutation principle (a mutation that
			// touches a mechanic must touch a LIVE mechanic). Borrow-less
			// genomes spend the whole branch on DrawPenalty.
			options := 1
			if g.HasScoringBorrow() {
				options = 2
			}
			switch rng.IntN(options) {
			case 0:
				g.Shedding.DrawPenalty = clampInt(g.Shedding.DrawPenalty+rng.IntN(3)-1, 1, 3)
			case 1:
				// Banked-score rounds (Task 22). Floor 2, not 1: this branch
				// is only reachable with a scoring borrow present, and a
				// scoring borrow at RoundsPerGame 1 is the inert rank05
				// combination (round 3 commit 6b) -- the same coupling
				// addBorrowedMechanic enforces. The clamp also normalizes
				// the legacy 0 ("unset") encoding.
				g.Shedding.RoundsPerGame = clampInt(g.Shedding.RoundsPerGame+rng.IntN(3)-1, 2, 5)
			}
		}
	case genome.TrickTaking:
		if g.TrickTaking != nil {
			g.TrickTaking.RoundsPerGame = clampInt(g.TrickTaking.RoundsPerGame+rng.IntN(3)-1, 1, 13)
		}
	case genome.Rummy:
		if g.Rummy != nil {
			switch rng.IntN(2) {
			case 0:
				// Floor 3 (not 2): a 2-card meld is trivially formable and
				// Tier-0 rejected as a liveness violation (Task 28 round 4).
				g.Rummy.MinMeldSize = clampInt(g.Rummy.MinMeldSize+rng.IntN(3)-1, 3, 4)
			case 1:
				g.Rummy.KnockThreshold = clampInt(g.Rummy.KnockThreshold+rng.IntN(5)-2, 0, 30)
			}
		}
	case genome.Climbing:
		if g.Climbing != nil {
			// MinRunLen is the only int param. Keep it in the valid 3-5 band so
			// a later AllowRuns flip lands on a length the runner accepts (the
			// run/single boundary). The clamp also normalizes the legacy 0
			// "unset" encoding to 3.
			g.Climbing.MinRunLen = clampInt(g.Climbing.MinRunLen+rng.IntN(3)-1, 3, 5)
		}
	case genome.Vying:
		if g.Vying != nil {
			// Mutate one betting param, then restore the stack-sufficiency
			// invariant (StartingChips >= rounds*min_bet*(max_raises+1)) so the
			// mutant stays valid and never reaches an all-in -- validateVying
			// would otherwise zero its fitness.
			switch rng.IntN(4) {
			case 0:
				g.Vying.MinBet = clampInt(g.Vying.MinBet+(rng.IntN(3)-1)*5, 5, 50)
			case 1:
				g.Vying.MaxRaises = clampInt(g.Vying.MaxRaises+rng.IntN(3)-1, 1, 6)
			case 2:
				g.Vying.RoundsPerGame = clampInt(g.Vying.RoundsPerGame+rng.IntN(5)-2, 2, 30)
			case 3:
				g.Vying.StartingChips = clampInt(g.Vying.StartingChips+(rng.IntN(5)-2)*100, 200, 5000)
			}
			if worst := g.Vying.RoundsPerGame * g.Vying.MinBet * (g.Vying.MaxRaises + 1); g.Vying.StartingChips < worst {
				g.Vying.StartingChips = worst
			}
		}
	}
}

// flipBool toggles a boolean skeleton parameter. Shedding and rummy no longer
// expose any runner-consumed bool (CanStack/PlayMultiple/CanLayOff were inert
// and removed -- dd-027), so trick-taking's MustFollowSuit and climbing's
// combination toggles (AllowPairs/AllowTriples/AllowRuns) are the only bools.
func flipBool(g *genome.Genome, rng *rand.Rand) {
	switch g.Skeleton {
	case genome.TrickTaking:
		if g.TrickTaking != nil {
			g.TrickTaking.MustFollowSuit = !g.TrickTaking.MustFollowSuit
		}
	case genome.Climbing:
		if g.Climbing != nil {
			// Flip one combination-type toggle. Singles are always legal (the
			// playability floor), so toggling pairs/triples/runs off can never
			// make the game unplayable. When AllowRuns is turned ON, ensure
			// MinRunLen is a valid length so the runner produces runs.
			switch rng.IntN(3) {
			case 0:
				g.Climbing.AllowPairs = !g.Climbing.AllowPairs
			case 1:
				g.Climbing.AllowTriples = !g.Climbing.AllowTriples
			case 2:
				g.Climbing.AllowRuns = !g.Climbing.AllowRuns
				if g.Climbing.AllowRuns && (g.Climbing.MinRunLen < 3 || g.Climbing.MinRunLen > 5) {
					g.Climbing.MinRunLen = 3
				}
			}
		}
	}
}

func changeEnum(g *genome.Genome, rng *rand.Rand) {
	switch g.Skeleton {
	case genome.Shedding:
		if g.Shedding != nil {
			g.Shedding.MatchRule = genome.MatchRule(rng.IntN(4))
		}
	case genome.TrickTaking:
		if g.TrickTaking != nil {
			switch rng.IntN(2) {
			case 0:
				g.TrickTaking.TrickScoring = genome.TrickScoring(rng.IntN(3))
				// If switching to card-points or avoidance, ensure scoring config exists
				if (g.TrickTaking.TrickScoring == genome.ScoreCardPoints ||
					g.TrickTaking.TrickScoring == genome.ScoreAvoidance) &&
					len(g.Scoring.CardPoints) == 0 {
					g.Scoring.CardPoints = []genome.CardScoring{
						{Suit: uint8(3), Points: 1}, // Hearts worth 1 point
					}
				}
			case 1:
				// LeadWinnerLeads (2) is reserved/inert (see genome.LeadRule):
				// only the two behavioral values are in the search space.
				g.TrickTaking.LeadRestriction = genome.LeadRule(rng.IntN(2))
			}
		}
		// Also mutate trump rule
		if rng.Float64() < 0.3 {
			g.TrumpRule = genome.TrumpRule(rng.IntN(4))
			// If fixed trump, ensure trump suit is set
			if g.TrumpRule == genome.TrumpFixed && g.Scoring.TrumpSuit == 0 {
				g.Scoring.TrumpSuit = uint8(rng.IntN(4) + 1)
			}
		}
	case genome.Rummy:
		if g.Rummy != nil {
			switch rng.IntN(2) {
			case 0:
				g.Rummy.MeldTypes = genome.MeldType(rng.IntN(3))
			case 1:
				g.Rummy.DrawFrom = genome.DrawSource(rng.IntN(3))
			}
		}
	}
}

func addSpecialCard(g *genome.Genome, rng *rand.Rand) {
	// Only the shedding runner applies special-card effects; adding them to
	// other skeletons produces inert genome bits and lying rulebooks (dd-24e).
	if g.Skeleton != genome.Shedding {
		return
	}
	if len(g.SpecialCards) >= 6 {
		return // Cap at 6 special cards
	}

	ranks := []uint8{uint8(2), uint8(7), uint8(8), uint8(10), uint8(11), uint8(12)}
	types := []genome.SpecialCardType{
		genome.SpecialSkip,
		genome.SpecialReverse,
		genome.SpecialDrawTwo,
		genome.SpecialDrawFour,
		genome.SpecialWild,
	}

	// Sample ByRank=0 ("any rank") with ~15% probability so suit-bound
	// catch-alls like "every Heart is wild" are reachable through cumulative
	// mutation, not only via seed copy. Mirrors the mutateScoring catch-all
	// convention from dd-eir (dd-g2m).
	//
	// LIVENESS GUARD (Task 28 round 3): ByRank=0 AND BySuit=0 matches EVERY
	// card -- the catch-all encoding genome.Validate rejects as a Tier-0
	// liveness violation (it deletes match_rule/draw_penalty as dead genes;
	// the round-2 flagship's entire shedding top 10 carried one). When the
	// rank qualifier is dropped, a specific suit is forced; the full BySuit
	// 0-4 range stays reachable for rank-qualified rules. Pinned by
	// TestAddSpecialCardNeverCatchAll.
	var byRank, bySuit uint8
	if rng.Float64() < 0.15 {
		byRank = 0
		bySuit = uint8(rng.IntN(4) + 1) // 1-4: a suit qualifier is mandatory
	} else {
		byRank = ranks[rng.IntN(len(ranks))]
		bySuit = uint8(rng.IntN(5)) // 0=any suit, 1-4=specific
	}

	sc := genome.SpecialCard{
		Type:   types[rng.IntN(len(types))],
		ByRank: byRank,
		BySuit: bySuit,
	}

	g.SpecialCards = append(g.SpecialCards, sc)
}

func removeSpecialCard(g *genome.Genome, rng *rand.Rand) {
	if len(g.SpecialCards) == 0 {
		return
	}
	idx := rng.IntN(len(g.SpecialCards))
	g.SpecialCards = append(g.SpecialCards[:idx], g.SpecialCards[idx+1:]...)
}

func addBorrowedMechanic(g *genome.Genome, rng *rand.Rand, crossSkeleton bool) {
	if len(g.Borrowed) >= 3 {
		return // Cap at 3 borrowed mechanics
	}

	// Pick a valid borrow for this skeleton
	var candidates []genome.BorrowedMechanic

	switch g.Skeleton {
	case genome.Shedding:
		// MechTrump dropped: shedding runner has no trump implementation
		// (see dd-lnh).
		candidates = []genome.BorrowedMechanic{
			{Source: genome.Rummy, Mechanic: genome.MechMeldBonus},
			{Source: genome.TrickTaking, Mechanic: genome.MechAvoidance},
		}
		if crossSkeleton {
			// Novelty evolution: shed-to-win game scored by tricks. Whitelisted
			// + hooked (applyTrickScoring) + outcome-affecting (wired through
			// multi-round banking below).
			candidates = append(candidates,
				genome.BorrowedMechanic{Source: genome.TrickTaking, Mechanic: genome.MechTrickScoring},
				// DEEP move-level borrow: climbing's multi-card combinations
				// (ComboPlay). Runner-implemented (not a hook); giveBorrowTeeth
				// bumps hand size + relaxes match so combos actually form.
				genome.BorrowedMechanic{Source: genome.Climbing, Mechanic: genome.MechRunPlay},
				// DEEP move-level borrow: trick-taking's follow-suit obligation
				// (FollowConstrained). Runner-implemented; giveBorrowTeeth makes
				// suit cards playable so the constraint binds.
				genome.BorrowedMechanic{Source: genome.TrickTaking, Mechanic: genome.MechFollowSuit},
				// DEEP win-condition borrow: rummy's knock (Knockable). Runner-
				// implemented in CheckEnd; declare early, fewest cards wins.
				// Outcome-significant by construction, so no teeth needed.
				genome.BorrowedMechanic{Source: genome.Rummy, Mechanic: genome.MechKnock})
		}
	case genome.TrickTaking:
		// MechPlayMultiple dropped: tricktaking move-gen only ever
		// produces single-card plays (see dd-lnh).
		// MechDrawPenalty dropped: appending cards mid-round breaks
		// the empty-hand round-end invariant (see dd-wfi).
		candidates = []genome.BorrowedMechanic{
			{Source: genome.Rummy, Mechanic: genome.MechMeldBonus},
		}
		if crossSkeleton {
			// Novelty evolution: trick-taker with cross-family penalty-card
			// scoring (applyAvoidance reads tableau captures; findWinner reads
			// state.Scores). Sourced from Shedding to record the cross-family
			// provenance (Avoidance's semantic home is trick-taking == host).
			candidates = append(candidates,
				genome.BorrowedMechanic{Source: genome.Shedding, Mechanic: genome.MechAvoidance})
		}
	case genome.Rummy:
		// MechTrickScoring is intentionally NOT a rummy candidate (Wave-3):
		// rummy "captures" are laid-down melds, which random play almost never
		// forms before knock/gin, so applyTrickScoring fires too rarely to ever
		// decide the winner -- a vestigial tally. It stays whitelisted in
		// validBorrows (a genome already carrying it is valid), but the engine no
		// longer generates the dead combination. The avoidance and draw-penalty
		// borrows DO get teeth (giveBorrowTeeth: deadwood-scale penalty + raised
		// knock for avoidance; a live draw pile + bigger penalty for draw).
		candidates = []genome.BorrowedMechanic{
			{Source: genome.Shedding, Mechanic: genome.MechDrawPenalty},
			{Source: genome.TrickTaking, Mechanic: genome.MechAvoidance},
		}
	case genome.Climbing:
		// Climbing borrows MechDrawPenalty (the one hook that fires AND affects
		// climbing's hand-based winner -- it grows a hand on face-card plays,
		// slowing that player's race to empty) and, under cross-skeleton, the
		// DEEP MechKnock (declare-to-end, fewest cards wins; runner-implemented in
		// CheckEnd, outcome-significant by construction so no teeth). The banking
		// scoring borrows are NOT whitelisted on climbing (CheckEnd never reads
		// state.Scores), so they are not candidates here.
		candidates = []genome.BorrowedMechanic{
			{Source: genome.Shedding, Mechanic: genome.MechDrawPenalty},
		}
		if crossSkeleton {
			candidates = append(candidates,
				genome.BorrowedMechanic{Source: genome.Rummy, Mechanic: genome.MechKnock})
		}
	case genome.Casino:
		// Casino borrows a scoring mechanic only under cross-skeleton: a
		// fishing/capture game scored Scopa-style. Both bank into state.Scores on
		// the single end-of-game EventRoundEnd casino emits under CasinoScored,
		// and CheckEnd reads captured count + that banked score. giveBorrowTeeth
		// seeds the avoidance penalty set; forceBankingRounds is a no-op on casino
		// (it banks at game end, not over RoundsPerGame).
		if crossSkeleton {
			candidates = []genome.BorrowedMechanic{
				{Source: genome.Rummy, Mechanic: genome.MechMeldBonus},
				{Source: genome.TrickTaking, Mechanic: genome.MechAvoidance},
			}
		}
	case genome.Vying:
		// Vying (poker) borrows a scoring mechanic only under cross-skeleton: a
		// poker game scored at showdown by melds (sets/runs in the shown hand) or
		// by a penalty suit. Banked into the chip stacks on the deal's
		// EventRoundEnd (VyingScored mucks folded hands -> showdown-only).
		if crossSkeleton {
			candidates = []genome.BorrowedMechanic{
				{Source: genome.Rummy, Mechanic: genome.MechMeldBonus},
				{Source: genome.TrickTaking, Mechanic: genome.MechAvoidance},
			}
		}
	}

	if len(candidates) == 0 {
		return
	}

	// Don't add duplicates (matched on full (Source, Mechanic) key).
	pick := candidates[rng.IntN(len(candidates))]
	for _, b := range g.Borrowed {
		if b.Source == pick.Source && b.Mechanic == pick.Mechanic {
			return
		}
	}

	// Coherence nudge (Wave 2 step 3): softly discourage the "muddled scoring
	// pile-up" that dominated Wave-1 hybrids -- two or more borrows that all
	// bank into state.Scores (MeldBonus / Avoidance / TrickScoring), so the
	// game's score is an opaque sum of overlapping bonuses. This is a SOFT
	// discouragement, not a veto: when the host already carries a banking
	// borrow and this pick would add a SECOND one, drop it half the time. The
	// first scoring borrow is never touched, and a complex two-banking game is
	// still reachable ~50% of the time, so legitimately rich games are not
	// killed. NOT a hard veto and NOT in the degeneracy gate.
	if isBankingMechanic(pick.Mechanic) && g.HasBankingBorrow() && rng.Float64() < 0.5 {
		return
	}

	g.Borrowed = append(g.Borrowed, pick)

	// Coherent-mutation coupling (round 3 commit 6b), now unified into the
	// Wave-3 teeth wiring: a mechanic's supporting infrastructure lands in the
	// same mutation AND is made outcome-significant by construction. A banking
	// borrow on shedding is forced multi-round (so the banked scores get a
	// winner signal); an avoidance borrow gets a MEANINGFUL penalty set (a full
	// penalty suit, not a single token card) plus avoidance-mode wiring on a
	// trick host; a draw-penalty borrow on climbing gets a live draw pile so its
	// hook can fire. giveBorrowTeeth is the single source of this wiring, shared
	// verbatim with the cross-skeleton crossover path (wireHybridBorrow).
	giveBorrowTeeth(g, pick)
}

// isBankingMechanic reports whether a borrowed mechanic writes to state.Scores
// (the banking-scoring family: MeldBonus, Avoidance, TrickScoring). Mirrors
// genome.HasBankingBorrow's membership test; used by the coherence nudge to
// detect a would-be second scoring pile-up.
func isBankingMechanic(m genome.MechanicType) bool {
	switch m {
	case genome.MechMeldBonus, genome.MechAvoidance, genome.MechTrickScoring:
		return true
	}
	return false
}

func removeBorrowedMechanic(g *genome.Genome, rng *rand.Rand) {
	if len(g.Borrowed) == 0 {
		return
	}
	idx := rng.IntN(len(g.Borrowed))
	g.Borrowed = append(g.Borrowed[:idx], g.Borrowed[idx+1:]...)
}

func changeSkeleton(g *genome.Genome, rng *rand.Rand, allSeeds []*genome.Genome) {
	if len(allSeeds) == 0 {
		return
	}
	// Pick a random seed (potentially different skeleton)
	seed := allSeeds[rng.IntN(len(allSeeds))]
	seedCopy := cloneGenome(seed)

	// Keep the child's generation and ID
	seedCopy.ID = g.ID
	seedCopy.Generation = g.Generation

	*g = *seedCopy
}

func mutateScoring(g *genome.Genome, rng *rand.Rand) {
	if len(g.Scoring.CardPoints) == 0 {
		// Add a scoring rule. Rank=0 is the "all ranks" wildcard; reach it
		// with a small probability so catch-all rules like "every Heart
		// scores N" are evolvable (see dd-eir).
		rank := uint8(0)
		if rng.Float64() >= 0.15 {
			rank = uint8(rng.IntN(13) + 2) // 2-14 (Rank=1 is invalid)
		}
		g.Scoring.CardPoints = append(g.Scoring.CardPoints, genome.CardScoring{
			Suit:   uint8(rng.IntN(5)), // 0=all, 1-4=specific
			Rank:   rank,
			Points: rng.IntN(13) + 1, // 1-13
		})
	} else {
		// Modify existing
		idx := rng.IntN(len(g.Scoring.CardPoints))
		g.Scoring.CardPoints[idx].Points = clampInt(
			g.Scoring.CardPoints[idx].Points+rng.IntN(5)-2, 1, 20)
	}
}

func cloneGenome(g *genome.Genome) *genome.Genome {
	return g.Clone()
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

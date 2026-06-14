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
			// DEEP move-level borrow: climbing's multi-card combinations (dump
			// a same-rank set / same-suit run in one turn). NOT a banking tally
			// -- it changes the legal-move set in the shedding runner
			// (ComboPlay), a genuine cross-family fusion. Given teeth by
			// giveBorrowTeeth (hand size + permissive match so combos form).
			{genome.MechRunPlay, genome.Climbing},
		},
		genome.TrickTaking: {
			// trick-taker with cross-family penalty-card scoring
			{genome.MechAvoidance, genome.Shedding},
			// trick-taker collecting rummy meld bonuses from captures
			{genome.MechMeldBonus, genome.Rummy},
		},
		genome.Rummy: {
			// rummy with penalty-card avoidance scoring -- given teeth by
			// giveRummyAvoidanceTeeth (deadwood-scale penalty suit + raised knock
			// threshold so the avoidance scoring decides the winner and the game
			// plays well off Gin Rummy).
			{genome.MechAvoidance, genome.TrickTaking},
			// rummy with shedding-style draw-penalty bursts (Wave 2 diversity
			// fix): whitelisted (validBorrows[Rummy][MechDrawPenalty]) and
			// hooked (applyDrawPenalty fires on face-card discards in the rummy
			// runner -- the borrow the pre-Wave-2 cross table omitted, leaving
			// rummy hosts unable to reach a whole novel direction).
			{genome.MechDrawPenalty, genome.Shedding},
			// MechTrickScoring is NOT generated on a rummy host (Wave-3): rummy's
			// "captures" are laid-down melds (state.Melds), and random play almost
			// never lays melds before knock/gin, so applyTrickScoring fires too
			// rarely to ever DECIDE the winner -- it would be a vestigial tally,
			// exactly the failure this wave removes. It stays in validBorrows (a
			// genome that already carries it is valid and its hook still works),
			// but the engine no longer manufactures the dead combination. Mirrors
			// the climbing precedent (only the borrow that can get teeth is
			// generated).
		},
		genome.Climbing: {
			// climbing with shedding-style draw-penalty bursts (novelty
			// evolution): a climbing core whose high-card plays inflict an extra
			// card. The ONLY whitelisted climbing borrow -- it both fires
			// (applyDrawPenalty on EventCardPlayed, which climbing emits) and
			// affects the winner (climbing's winner is first-to-empty-hand, and
			// the hook GROWS a hand, slowing the race to empty). The banking
			// scoring borrows are not whitelisted on climbing (CheckEnd never
			// reads state.Scores), so they cannot appear here.
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

// wireHybridBorrow makes a freshly-added hybrid borrow OUTCOME-AFFECTING. It
// inherits any banking-rounds count or non-empty CardPoints set the PARENTS
// already carried (so a child of two rich genomes does not regress to the
// defaults), then delegates to giveBorrowTeeth -- the single wiring that
// guarantees a grafted cross-family borrow makes the hybrid PLAY measurably
// differently from the base classic (Wave-3 root-cause fix), shared verbatim
// with addBorrowedMechanic.
func wireHybridBorrow(child *genome.Genome, bm genome.BorrowedMechanic, a, b *genome.Genome) {
	// Prefer a multi-round count a parent already evolved (>= 2) over the
	// default-2 floor giveBorrowTeeth would install, so a deep banking lineage
	// keeps its tuned round count.
	if child.Skeleton == genome.Shedding && child.Shedding != nil && isBankingMechanic(bm.Mechanic) {
		switch {
		case a.Shedding != nil && a.Shedding.RoundsPerGame > child.Shedding.RoundsPerGame:
			child.Shedding.RoundsPerGame = a.Shedding.RoundsPerGame
		case b.Shedding != nil && b.Shedding.RoundsPerGame > child.Shedding.RoundsPerGame:
			child.Shedding.RoundsPerGame = b.Shedding.RoundsPerGame
		}
	}
	// Prefer a meaningful penalty set a parent already carried for an avoidance
	// borrow; giveBorrowTeeth upgrades a too-thin set to a full penalty suit.
	if bm.Mechanic == genome.MechAvoidance && len(child.Scoring.CardPoints) == 0 {
		switch {
		case len(a.Scoring.CardPoints) > 0:
			child.Scoring.CardPoints = append([]genome.CardScoring(nil), a.Scoring.CardPoints...)
		case len(b.Scoring.CardPoints) > 0:
			child.Scoring.CardPoints = append([]genome.CardScoring(nil), b.Scoring.CardPoints...)
		}
	}

	giveBorrowTeeth(child, bm)
}

// fullSuitPenalty returns a CardScoring rule that makes an ENTIRE suit a
// penalty -- the meaningful avoidance set (13 cards) the teeth wiring installs,
// never a single token card. Suit 3 == sim.Hearts under the CardScoring +1
// convention (Clubs0/Diamonds1/Hearts2/Spades3), the canonical penalty suit.
func fullSuitPenalty() genome.CardScoring {
	return genome.CardScoring{Suit: 3, Points: 1}
}

// avoidanceSetIsMeaningful reports whether g's CardPoints plausibly drive
// avoidance strategy: it must penalize a substantial share of the deck (a full
// suit ~= 13 cards, or several high-value cards), not a single token card that
// the host's native scoring swamps. We count the worst-case penalized cards
// across the standard deck; a full-suit rule (Rank==0, specific Suit) covers 13.
func avoidanceSetIsMeaningful(g *genome.Genome) bool {
	penalized := 0
	for rank := uint8(2); rank <= 14; rank++ {
		for suit := uint8(0); suit <= 3; suit++ {
			if genome.MatchCardPoints(g.Scoring.CardPoints, rank, suit) > 0 {
				penalized++
			}
		}
	}
	// >= 10 penalized cards (about a fifth of the deck) is the floor at which an
	// avoidance set can plausibly out-weigh per-trick scoring and decide the
	// winner. A single token card (1) or a couple of high cards never clears it.
	return penalized >= 10
}

// climbingDeckHeadroom is how many cards the teeth wiring leaves UNDEALT on a
// climbing host carrying a draw-penalty borrow, so applyDrawPenalty (which draws
// from state.Deck) actually has a pile to draw from. Big Two deals the whole
// 52-card deck (4 x 13), leaving state.Deck EMPTY -- the hook can never fire, so
// the borrow is vestigial by construction. Clamping HandSize so
// Players*HandSize <= 52 - headroom guarantees a live draw pile. 16 (vs a bare
// minimum of ~4) sizes the pile so the penalty draws materially slow the racer
// -- at headroom 12 (deck of 12) the descriptor barely moved; at 16 it moves
// ~3x as far, the difference between a cosmetic and a behavior-shifting borrow.
const climbingDeckHeadroom = 16

// giveBorrowTeeth is the SINGLE wiring that makes any grafted cross-family
// borrow OUTCOME-SIGNIFICANT BY CONSTRUCTION (Wave-3 root-cause fix). A borrow
// in the genome must be a borrow that CHANGES THE GAME: after this runs, the
// hybrid plays measurably differently from the base classic (its behavior
// descriptor moves) and the borrowed mechanic decides the winner -- not a
// vestigial single-token card the host's native scoring ignores.
//
// Both production graft paths call it: wireHybridBorrow (cross-skeleton
// crossover) and addBorrowedMechanic (cross-skeleton mutation). It is purely
// ADDITIVE -- it never touches the 5 metric weights, ComputeFitness, the
// vetoes, or the calibration gate, and it is valid-in/valid-out (every field it
// writes stays inside genome.Validate's ranges).
//
// Teeth by borrow family:
//
//   - AVOIDANCE: install a MEANINGFUL penalty-card set (a full penalty suit,
//     ~13 cards) whenever the current set is too thin to drive strategy, so the
//     avoidance scoring plausibly out-weighs the host's native scoring. On a
//     TRICK host, also restrict leading the penalty suit until broken -- so the
//     penalty cards change which moves are legal (the descriptor moves) while the
//     avoidance HOOK supplies the penalty scoring on top of native per-trick
//     scoring (deliberately NOT switching to ScoreAvoidance, which would
//     double-count with the hook into a seat-0 sweep). On a RUMMY host, scale the
//     penalty above single-card deadwood and loosen the knock so the penalty
//     decides the winner without collapsing to instant-knock. Force the
//     multi-round banked-scoring path on shedding so the banked penalties get a
//     winner signal.
//
//   - TRICK-SCORING / MELD-BONUS (banking) on a non-native host: force the
//     multi-round banked-scoring wiring (RoundsPerGame >= 2 + SheddingMultiRound
//     banking path) so the borrowed scoring DECIDES THE WINNER instead of
//     banking into a tally CheckEnd ignores.
//
//   - DRAW-PENALTY: keep the penalty materially affecting the race AND the game
//     terminable. On climbing, clamp HandSize so the deck is not fully dealt
//     (otherwise applyDrawPenalty has no pile to draw from and never fires). On
//     rummy, loosen the knock threshold so the inflated hands the penalty creates
//     can still reach a knock before the turn cap (otherwise ~half the games time
//     out -- a Tier-1 kill).
func giveBorrowTeeth(g *genome.Genome, bm genome.BorrowedMechanic) {
	switch bm.Mechanic {
	case genome.MechAvoidance:
		// A meaningful penalty set is mandatory: upgrade a missing/thin set to a
		// full penalty suit so the avoidance scoring can plausibly drive
		// strategy (never a single token card).
		if !avoidanceSetIsMeaningful(g) {
			g.Scoring.CardPoints = append(g.Scoring.CardPoints, fullSuitPenalty())
			// If still thin (e.g. duplicate suit), the appended full-suit rule
			// now covers 13 cards, which clears the floor.
		}
		switch g.Skeleton {
		case genome.TrickTaking:
			if g.TrickTaking != nil {
				// Restrict leading the penalty suit until broken (the Hearts-style
				// "can't open with penalties" discipline). This changes which
				// moves are legal -- it is what moves the behavior descriptor off
				// the base classic -- while the avoidance HOOK supplies the
				// penalty scoring ON TOP of the host's native per-trick scoring.
				// Deliberately do NOT switch TrickScoring to ScoreAvoidance: the
				// native ScoreAvoidance path ALSO writes penalties to the same
				// state.Scores the hook subtracts from, and the two opposite-sign
				// contributions collide (findWinner picks lowest, the double count
				// collapsed to a structural seat-0 sweep -- a degeneracy veto).
				// Keeping native per-trick scoring yields a genuine cross-family
				// game: win tricks, but penalty cards you capture count against
				// you.
				g.TrickTaking.LeadRestriction = genome.LeadNoTrumpUntilBroken
			}
		case genome.Rummy:
			giveRummyAvoidanceTeeth(g)
		}
		forceBankingRounds(g)

	case genome.MechMeldBonus, genome.MechTrickScoring:
		forceBankingRounds(g)

	case genome.MechDrawPenalty:
		switch g.Skeleton {
		case genome.Climbing:
			ensureClimbingDrawPile(g)
		case genome.Rummy:
			giveRummyDrawPenaltyTeeth(g)
		}

	case genome.MechRunPlay:
		// Combos (ComboPlay) need a hand large enough that 2+ same-rank groups
		// or 2+ same-suit runs arise, and a match rule permissive enough that a
		// group can match the discard top. Bump a too-small hand to 6 and relax
		// an over-strict MatchBoth to MatchEither so the borrow is outcome-
		// affecting (else dd-lnh: a no-op borrow). Valid-in/valid-out: HandSize
		// stays in the 3-13 band and Players*6 <= 52 for the domain's seat counts.
		if g.Skeleton == genome.Shedding && g.Shedding != nil {
			if g.HandSize < 6 {
				g.HandSize = 6
			}
			if g.Shedding.MatchRule == genome.MatchBoth {
				g.Shedding.MatchRule = genome.MatchEither
			}
		}
	}
}

// forceBankingRounds routes a host carrying a banking borrow through a
// MULTI-ROUND banked-scoring game (RoundsPerGame >= 2) so the borrowed scoring
// accumulates over several deals and DECIDES the winner -- not a one-deal tally
// the host's native resolution swamps.
//
//   - SHEDDING: without RoundsPerGame >= 2 the banked scores are written at
//     round end but nothing ever reads them (the single-round game ends at the
//     first empty hand, SheddingMultiRound is false), so the borrow is inert.
//   - TRICK-TAKING: a single-round trick game plays byte-identically to the
//     base classic (the bonus is a cosmetic tally on top of per-trick scores).
//     Multi-round accumulates captures across deals so meld/penalty bonuses
//     actually form and shift the banked total, AND the extra deals move the
//     behavior descriptor off the base classic.
//
// Rummy has no RoundsPerGame field (it is a single knock/gin round); its banking
// borrows are given teeth via the deadwood-competition path, not here.
func forceBankingRounds(g *genome.Genome) {
	switch g.Skeleton {
	case genome.Shedding:
		if g.Shedding != nil && g.Shedding.RoundsPerGame < 2 {
			g.Shedding.RoundsPerGame = 2
		}
	case genome.TrickTaking:
		if g.TrickTaking != nil && g.TrickTaking.RoundsPerGame < 2 {
			g.TrickTaking.RoundsPerGame = 2
		}
	}
}

// ensureClimbingDrawPile clamps a climbing host's HandSize so the deal leaves a
// non-empty draw pile (Players*HandSize <= 52 - climbingDeckHeadroom), the
// precondition for applyDrawPenalty to fire at all. HandSize stays within the
// genome's valid 3-13 band.
func ensureClimbingDrawPile(g *genome.Genome) {
	if g.Players < 1 {
		return
	}
	maxDealt := 52 - climbingDeckHeadroom
	for g.HandSize*g.Players > maxDealt && g.HandSize > 3 {
		g.HandSize--
	}
}

// rummyAvoidancePenaltyPoints is the per-card penalty the avoidance teeth wiring
// stamps on a rummy host's penalty suit. Rummy resolves on banked deadwood
// (face cards worth 10), so a 1-point penalty suit is swamped by deadwood and
// never decides the winner. 15 puts the avoidance penalty ABOVE single-card
// deadwood, so holding even one penalty card meaningfully shifts the banked
// total and the avoidance scoring flips the winner in a non-trivial fraction of
// games.
const rummyAvoidancePenaltyPoints = 15

// rummyAvoidanceKnockThreshold is the knock threshold the avoidance teeth wiring
// installs on a rummy host. Gin Rummy knocks at ~10 deadwood, ending fast before
// penalty cards accumulate. Raising it loosens knocking so the round runs a bit
// longer and the avoidance penalty has cards to bite -- but NOT so high it
// becomes instant-knock (>= ~half the worst-case deadwood collapses the game to
// a 1-turn degenerate race, which would FAIL the degeneracy vetoes). 25 is the
// calibrated middle: ~40-turn games, 100% completion, the descriptor moves off
// Gin Rummy, and the penalty decides the winner.
const rummyAvoidanceKnockThreshold = 25

// rummyDrawKnockThreshold is the knock threshold the draw-penalty teeth wiring
// installs on a rummy host. applyDrawPenalty GROWS a hand (extra draw on
// face-card discards), inflating deadwood; at Gin Rummy's tight knock (~10) the
// inflated hand can never reach the knock/gin condition, so 40-60% of games hit
// the turn cap (a Tier-1 timeout kill). Loosening the threshold lets the bigger
// game resolve -- timeouts drop to ~0-2% -- while the draw penalty still shifts
// the race (descriptor moves well off Gin Rummy). A draw-penalty borrow that
// inflates hands MUST be paired with a looser knock to stay terminable.
const rummyDrawKnockThreshold = 25

// giveRummyDrawPenaltyTeeth keeps a draw-penalty borrow on a RUMMY host both
// outcome-affecting AND terminable. The hook already changes the race (it grows
// the discarder's hand on face-card plays), but on the tight Gin Rummy knock the
// inflated hands never resolve and the game times out. Loosening the knock
// threshold lets the game end before the turn cap.
func giveRummyDrawPenaltyTeeth(g *genome.Genome) {
	if g.Rummy != nil && g.Rummy.KnockThreshold < rummyDrawKnockThreshold {
		g.Rummy.KnockThreshold = rummyDrawKnockThreshold
	}
}

// giveRummyAvoidanceTeeth makes an avoidance borrow on a RUMMY host decide the
// winner. Rummy has no RoundsPerGame, and its deadwood resolution dwarfs a
// token penalty suit, so the borrow was vestigial. Two coupled changes give it
// teeth: (1) scale the penalty suit ABOVE single-card deadwood so it shifts the
// banked total, and (2) loosen the knock threshold so the round runs long enough
// for penalty cards to matter -- which also moves the descriptor off Gin Rummy
// -- while staying well short of instant-knock so the game does not collapse to
// a degenerate 1-turn race.
func giveRummyAvoidanceTeeth(g *genome.Genome) {
	// Raise every penalty-suit rule to deadwood scale (only bumps low ones; a
	// parent that already evolved a heavier penalty keeps it).
	for i := range g.Scoring.CardPoints {
		if g.Scoring.CardPoints[i].Points < rummyAvoidancePenaltyPoints {
			g.Scoring.CardPoints[i].Points = rummyAvoidancePenaltyPoints
		}
	}
	if g.Rummy != nil && g.Rummy.KnockThreshold < rummyAvoidanceKnockThreshold {
		g.Rummy.KnockThreshold = rummyAvoidanceKnockThreshold
	}
}

// wiredHybrid grafts a cross-family borrow (source, mech) onto a clone of base
// and applies the full teeth wiring (giveBorrowTeeth), returning the
// outcome-significant hybrid the production paths would produce. It is the
// single composable graft-with-teeth entry point used by tests and any future
// caller that needs a wired hybrid without driving the RNG-based mutation path.
// The source records the cross-family provenance and must differ from the
// host's own skeleton.
func wiredHybrid(base *genome.Genome, source genome.SkeletonType, mech genome.MechanicType) *genome.Genome {
	g := base.Clone()
	g.ID = ""
	bm := genome.BorrowedMechanic{Source: source, Mechanic: mech}
	// Skip if already present (idempotent).
	for _, existing := range g.Borrowed {
		if existing.Source == bm.Source && existing.Mechanic == bm.Mechanic {
			giveBorrowTeeth(g, bm)
			return g
		}
	}
	g.Borrowed = append(g.Borrowed, bm)
	giveBorrowTeeth(g, bm)
	return g
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
	case genome.Climbing:
		crossoverClimbing(child, a, b, rng)
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

	// Any borrow the child carries (inherited via the same-skeleton Borrowed
	// coin flip, or grafted on the hybrid path) must stay outcome-significant
	// (Wave-3): the Borrowed / RoundsPerGame / Scoring coin flips are
	// independent, so a child can take a scoring borrow from one parent and
	// single-round play (or an empty/thin penalty set) from the other -- the
	// inert rank05 combination. First prefer a richer parent value (a deep
	// banking round count, a real penalty set) over the giveBorrowTeeth
	// defaults, then run the shared teeth wiring so the borrow is never
	// vestigial regardless of how the coin flips fell.
	if child.Skeleton == genome.Shedding && child.Shedding != nil && child.HasBankingBorrow() {
		switch {
		case a.Shedding != nil && a.Shedding.RoundsPerGame > child.Shedding.RoundsPerGame:
			child.Shedding.RoundsPerGame = a.Shedding.RoundsPerGame
		case b.Shedding != nil && b.Shedding.RoundsPerGame > child.Shedding.RoundsPerGame:
			child.Shedding.RoundsPerGame = b.Shedding.RoundsPerGame
		}
	}
	for _, bm := range child.Borrowed {
		if bm.Mechanic == genome.MechAvoidance && len(child.Scoring.CardPoints) == 0 {
			switch {
			case len(a.Scoring.CardPoints) > 0:
				child.Scoring.CardPoints = append([]genome.CardScoring(nil), a.Scoring.CardPoints...)
			case len(b.Scoring.CardPoints) > 0:
				child.Scoring.CardPoints = append([]genome.CardScoring(nil), b.Scoring.CardPoints...)
			}
		}
		giveBorrowTeeth(child, bm)
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

func crossoverClimbing(child *genome.Genome, a, b *genome.Genome, rng *rand.Rand) {
	if a.Climbing == nil || b.Climbing == nil {
		return
	}
	if rng.Float64() < 0.5 {
		child.Climbing.AllowPairs = b.Climbing.AllowPairs
	}
	if rng.Float64() < 0.5 {
		child.Climbing.AllowTriples = b.Climbing.AllowTriples
	}
	if rng.Float64() < 0.5 {
		child.Climbing.AllowRuns = b.Climbing.AllowRuns
	}
	if rng.Float64() < 0.5 {
		child.Climbing.MinRunLen = b.Climbing.MinRunLen
	}
	// Singles are always legal, so any combination of these toggles is a
	// playable game -- no invariant repair needed. But if runs are on, ensure
	// MinRunLen is a valid run length (the two coin flips above are
	// independent, so a child can take AllowRuns from one parent and a
	// run-disabled MinRunLen of 0 from the other).
	if child.Climbing.AllowRuns && (child.Climbing.MinRunLen < 3 || child.Climbing.MinRunLen > 5) {
		child.Climbing.MinRunLen = 3
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

package genome

import "sort"

// Gene-liveness predicates: which optional genes can actually affect a
// genome's OUTCOME under its own rules. Born in pkg/output/rulebook.go
// (round 3 commit 6: the rulebook must not print dead-rule text) and hoisted
// here in Wave K so the output-ranking dedup in pkg/evolution applies the
// EXACT same rules: the flagship-r3 leaderboard's ranks 1/2/3 were one game
// differing only in dead card_points blocks. Any package that renders,
// ranks, or dedups published genomes must route through these predicates
// rather than growing its own copy.

// LiveBorrows returns the borrowed mechanics that can actually affect g's
// outcome -- the ones the rulebook (and report) may advertise (round 3
// commit 6b; the r2 rank05 advertised a meld-bonus borrow that was inert at
// rounds_per_game 1):
//
//   - BANKING borrows (MechMeldBonus, MechAvoidance, MechTrickScoring) bank
//     state.Scores at round end. On a SINGLE-round shedding host nothing ever
//     reads those scores (the game ends at the first empty hand), so they are
//     live only when SheddingMultiRound() -- the same predicate the runner
//     uses. Trick-taking, rummy, and casino hosts read Scores in CheckEnd
//     (casino under CasinoScored: captured count + banked bonus), so they are
//     live there at any round count -- the pruning below only excludes
//     single-round Shedding. MechTrickScoring joined this set when the
//     shed-to-win-by-tricks hybrid was enabled (novelty evolution): its
//     applyTrickScoring hook also banks per round and is inert on a
//     single-round shedding host.
//   - MechAvoidance additionally requires non-empty CardPoints (the hook
//     no-ops without them).
//   - MechDrawPenalty acts directly (appends cards) and is always live.
func (g *Genome) LiveBorrows() []BorrowedMechanic {
	var live []BorrowedMechanic
	for _, bm := range g.Borrowed {
		switch bm.Mechanic {
		case MechMeldBonus, MechAvoidance, MechTrickScoring:
			if g.Skeleton == Shedding && !g.SheddingMultiRound() {
				continue
			}
			if bm.Mechanic == MechAvoidance && len(g.Scoring.CardPoints) == 0 {
				continue
			}
		}
		live = append(live, bm)
	}
	return live
}

// LiveCardPoints reports whether anything in g's RULES reads
// Scoring.CardPoints: trick-taking under card_points/avoidance scoring
// (cardPointValue in the runner), or a LIVE MechAvoidance borrow (the
// applyAvoidance hook returns early on empty CardPoints; see LiveBorrows for
// when the borrow itself is live).
func (g *Genome) LiveCardPoints() bool {
	if len(g.Scoring.CardPoints) == 0 {
		return false
	}
	if g.Skeleton == TrickTaking && g.TrickTaking != nil &&
		(g.TrickTaking.TrickScoring == ScoreCardPoints ||
			g.TrickTaking.TrickScoring == ScoreAvoidance) {
		return true
	}
	for _, bm := range g.LiveBorrows() {
		if bm.Mechanic == MechAvoidance {
			return true
		}
	}
	return false
}

// LiveLeadRestriction reports whether g's lead restriction can actually
// constrain a lead. The trick-taking runner's canLead only restricts when a
// trump suit exists (LeadNoTrumpUntilBroken forbids leading TRUMP), so under
// trump_rule=none the parameter is inert -- the Hearts seed carries exactly
// that pair, and its rulebook used to print "Cannot lead trump until trump has
// been played off-suit" for a game with no trump. The pair stays Tier-0 valid
// (rejecting it would invalidate the seed and every mutant that flips the
// trump rule); renderers route through this predicate instead.
func (g *Genome) LiveLeadRestriction() bool {
	return g.Skeleton == TrickTaking && g.TrickTaking != nil &&
		g.TrickTaking.LeadRestriction == LeadNoTrumpUntilBroken &&
		g.TrumpRule != TrumpNone
}

// VyingWorstCaseCommitment returns the most chips one player can lose over a
// whole vying game: on every deal, funding the capped betting round
// (MinBet*(MaxRaises+1): the blind plus MaxRaises raises) and losing the pot,
// plus -- under a LIVE avoidance borrow -- the worst showdown penalty a shown
// hand can carry (the HandSize most expensive penalty cards in the deck).
// validateVying requires StartingChips to cover it, which is what keeps the
// betting round free of all-ins, side pots, and hook-induced insolvency.
// Returns 0 for a genome without vying params.
func (g *Genome) VyingWorstCaseCommitment() int {
	if g.Vying == nil {
		return 0
	}
	perDeal := g.Vying.MinBet * (g.Vying.MaxRaises + 1)
	if g.Skeleton == Vying {
		for _, bm := range g.Borrowed {
			if bm.Mechanic == MechAvoidance {
				perDeal += g.maxHandPenalty()
				break
			}
		}
	}
	return g.Vying.RoundsPerGame * perDeal
}

// maxHandPenalty is the largest total MatchCardPoints penalty a single
// HandSize-card hand can carry: the sum of the HandSize highest positive
// per-card values across the 52-card deck (0 without CardPoints, when the
// avoidance hook no-ops).
func (g *Genome) maxHandPenalty() int {
	if len(g.Scoring.CardPoints) == 0 || g.HandSize <= 0 {
		return 0
	}
	vals := make([]int, 0, 52)
	for rank := uint8(2); rank <= 14; rank++ {
		for suit := uint8(0); suit < 4; suit++ {
			if v := MatchCardPoints(g.Scoring.CardPoints, rank, suit); v > 0 {
				vals = append(vals, v)
			}
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(vals)))
	total := 0
	for i := 0; i < len(vals) && i < g.HandSize; i++ {
		total += vals[i]
	}
	return total
}

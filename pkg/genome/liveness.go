package genome

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
//   - SCORING borrows (MechMeldBonus, MechAvoidance) bank state.Scores at
//     round end. On a SINGLE-round shedding host nothing ever reads those
//     scores (the game ends at the first empty hand), so they are live only
//     when SheddingMultiRound() -- the same predicate the runner uses.
//     Trick-taking and rummy hosts read Scores in CheckEnd, so they are live
//     there at any round count.
//   - MechAvoidance additionally requires non-empty CardPoints (the hook
//     no-ops without them).
//   - Everything else whitelisted (MechTrickScoring, MechDrawPenalty) acts
//     directly and is always live.
func (g *Genome) LiveBorrows() []BorrowedMechanic {
	var live []BorrowedMechanic
	for _, bm := range g.Borrowed {
		switch bm.Mechanic {
		case MechMeldBonus, MechAvoidance:
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

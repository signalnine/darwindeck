package grammar

import (
	"sort"

	"github.com/darwindeck/darwindeck/pkg/sim"
)

// Modifier is a typed addon to a base GameSpec -- the grammar's lift of v2's
// borrowed mechanics (pkg/mechanic). Each modifier hooks the ONE generic runner
// at a typed phase; CompatibleWith is the type rule that replaces v2's ad-hoc
// per-host whitelist (genome/validate.go validBorrows). The v2 "don't whitelist a
// no-op borrow" rule (dd-lnh) becomes: a modifier is only well-typed on a spec
// whose move/end/score actually consumes the signal it produces.
type Modifier int

const (
	ModRunPlay     Modifier = iota // DEEP move-expand: dump a same-rank set / same-suit run in one turn (v2 run_play, id 8)
	ModFollowSuit                  // DEEP move-restrict: must follow the discard suit if held (v2 follow_suit, id 7)
	ModWild                        // move-relax: a wild rank is always playable (v2 special-cards/isWild) -- rescues agency-dead match rules
	ModDrawPenalty                 // after-move: drawing one extra card after a face-card play (v2 draw_penalty, id 2)
	ModKnock                       // win-override: knock with a small hand to end the game by fewest cards (v2 knock, id 3)
	ModMeldBonus                   // score-adjust: bank set/run bonuses from the captured pile (v2 meld_bonus, id 1)
	ModAvoidance                   // score-adjust: penalty cards in your won pile count AGAINST you (v2 avoidance, id 5) -- gives Hearts on a trick host
	modifierCount
)

func (m Modifier) String() string {
	return [...]string{"run_play", "follow_suit", "wild", "draw_penalty", "knock", "meld_bonus", "avoidance"}[m]
}

// CompatibleWith is the modifier's type rule against the base spec. It reads only
// the structural fields (Move/Match/End/Score), never WellTyped, so it cannot
// recurse. This is the whole point of the grammar: the v2 whitelist that had to
// be hand-maintained per (skeleton, mechanic) pair is here a small total function.
func (m Modifier) CompatibleWith(s GameSpec) bool {
	switch m {
	case ModRunPlay, ModFollowSuit:
		// Move-set modifiers act on the match-and-shed generator (v2: shedding host).
		return s.Move == PlayMatch
	case ModWild:
		// A single wild rank is too sparse to restore agency to rank/suit-only
		// matching (the random-AI rescue experiment shows ~0.01 -> 0.02 agency,
		// still dead). So wild is NOT a productive modifier and never enumerated;
		// the base agency rule (require MatchEither) stands. Kept only so the
		// honest rescue experiment in cmd/grammar-proto can run the runner path.
		return false
	case ModDrawPenalty:
		// Hand-growth penalty only bites in a hand-emptying race (v2: shedding/rummy).
		return s.Move == PlayMatch && s.End == EmptyHand
	case ModKnock:
		// "Fewest cards wins" is only meaningful in an empty-hand race (v2: shedding+climbing).
		return (s.Move == PlayMatch || s.Move == BeatOrPass) && s.End == EmptyHand
	case ModMeldBonus:
		// Bank a weighted set/run bonus on top of the count rule -- v2's casino
		// CheckEnd ("captured COUNT + bonus"). It rides any pile-collecting count
		// rule: capture (Scopa-with-melds) or trick (win tricks AND form melds).
		return (s.Move == Capture || s.Move == Trick) && s.Score == MostCaptured
	case ModAvoidance:
		// Points-are-bad: penalty cards in your won pile subtract from the count,
		// so the winner is whoever AVOIDS them. On a trick host this is Hearts; on
		// capture it is a Scopa penalty-suit. Needs a pile + a count rule to bite.
		return (s.Move == Trick || s.Move == Capture) && s.Score == MostCaptured
	}
	return false
}

const wildRank = 8 // Crazy Eights' eights; mirrors v2 special-cards.

func isWild(c sim.Card, on bool) bool { return on && int(c.Rank) == wildRank }

// comboPlays builds the multi-card moves ModRunPlay adds: maximal same-rank sets
// (2+) and same-suit consecutive runs (2+) where at least one card matches the
// discard top under the match rule. A pure SUPERSET of single-card plays (those
// are added separately), so termination is preserved -- a combo only sheds faster.
func comboPlays(hand []sim.Card, top sim.Card, hasTop bool, rule MatchRule, player int) []sim.Move {
	var out []sim.Move
	// same-rank sets
	byRank := map[sim.Rank][]sim.Card{}
	for _, c := range hand {
		byRank[c.Rank] = append(byRank[c.Rank], c)
	}
	for r := sim.Rank(2); r <= 14; r++ {
		set := byRank[sim.Rank(r)]
		if len(set) < 2 {
			continue
		}
		if comboMatches(set, top, hasTop, rule) {
			out = append(out, mv(sim.MovePlay, player, set...))
		}
	}
	// same-suit consecutive runs (length 2+)
	for s := 0; s < 4; s++ {
		var suited []sim.Card
		for _, c := range hand {
			if int(c.Suit) == s {
				suited = append(suited, c)
			}
		}
		sortByRank(suited)
		for i := 0; i < len(suited); i++ {
			run := []sim.Card{suited[i]}
			for j := i + 1; j < len(suited); j++ {
				if suited[j].Rank == run[len(run)-1].Rank+1 {
					run = append(run, suited[j])
				} else {
					break
				}
			}
			if len(run) >= 2 && comboMatches(run, top, hasTop, rule) {
				out = append(out, mv(sim.MovePlay, player, run...))
				i += len(run) - 1
			}
		}
	}
	return out
}

func comboMatches(combo []sim.Card, top sim.Card, hasTop bool, rule MatchRule) bool {
	if !hasTop {
		return true
	}
	for _, c := range combo {
		if matches(c, top, hasTop, rule) {
			return true
		}
	}
	return false
}

func sortByRank(cs []sim.Card) {
	for i := 1; i < len(cs); i++ {
		for j := i; j > 0 && cs[j].Rank < cs[j-1].Rank; j-- {
			cs[j], cs[j-1] = cs[j-1], cs[j]
		}
	}
}

// meldBonus scores the captured pile for ModMeldBonus, mirroring v2 applyMeldBonus
// (pkg/mechanic/hooks.go): same-rank sets (3+ = 5/card, pair = 2/card) AND same-suit
// consecutive runs (3+ = 3/card, 2-run = 1/card). Sets and runs are tallied
// independently (a greedy bonus, not a partition), as in v2.
func meldBonus(pile []sim.Card) int {
	bonus := 0
	// same-rank sets
	byRank := map[sim.Rank]int{}
	for _, c := range pile {
		byRank[c.Rank]++
	}
	for _, n := range byRank {
		switch {
		case n >= 3:
			bonus += n * 5
		case n == 2:
			bonus += n * 2
		}
	}
	// same-suit consecutive runs
	for s := 0; s < 4; s++ {
		var ranks []int
		for _, c := range pile {
			if int(c.Suit) == s {
				ranks = append(ranks, int(c.Rank))
			}
		}
		sort.Ints(ranks)
		for i := 0; i < len(ranks); {
			j := i + 1
			for j < len(ranks) && ranks[j] == ranks[j-1]+1 {
				j++
			}
			switch runLen := j - i; {
			case runLen >= 3:
				bonus += runLen * 3
			case runLen == 2:
				bonus += runLen * 1
			}
			i = j
		}
	}
	return bonus
}

// avoidancePenalty scores the won pile for ModAvoidance (Hearts-style): each heart
// is 1 penalty point and the Queen of Spades is 13. The score rule subtracts this,
// so the winner is whoever took the FEWEST penalty cards.
func avoidancePenalty(pile []sim.Card) int {
	pen := 0
	for _, c := range pile {
		if c.Suit == sim.Hearts {
			pen++
		}
		if c.Rank == 12 && c.Suit == sim.Spades { // Queen of Spades
			pen += 13
		}
	}
	return pen
}

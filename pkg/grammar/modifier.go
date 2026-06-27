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
	ModTrump                       // trick-resolve: a trump suit beats the led suit -- gives Spades/Bridge-family trick games
	ModSkip                        // turn-order: playing a designated rank skips the next player -- gives Uno-family shedding
	ModForceDraw                   // attack: playing a designated rank makes the next player draw two and lose their turn (Uno draw-two)
	ModBid                         // pre-round phase: each player declares a target number of tricks; scored by making the contract (Spades/Oh Hell/Euchre)
	ModTeams                       // score-aggregation: seats are 2 partnerships (evens vs odds); the team with the best total wins (Spades/Euchre/Bridge)
	modifierCount
)

func (m Modifier) String() string {
	return [...]string{"run_play", "follow_suit", "wild", "draw_penalty", "knock", "meld_bonus", "avoidance", "trump", "skip", "force_draw", "bid", "teams"}[m]
}

// teamOf maps a seat to its partnership (even seats vs odd seats), the standard
// 2v2 arrangement for ModTeams.
func teamOf(seat int) int { return seat % 2 }

const (
	skipRank      = 7 // the rank that skips the next player under ModSkip (Uno-style)
	forceDrawRank = 2 // the rank that forces the next player to draw under ModForceDraw
	forceDrawN    = 2 // how many cards the victim draws
	trumpSuit     = 3 // Spades is trump under ModTrump (sim.Spades == 3); reuses wildRank(8) for rummy melds
)

// CompatibleWith is the modifier's type rule against the base spec. It reads only
// the structural fields (Move/Match/End/Score), never WellTyped, so it cannot
// recurse. This is the whole point of the grammar: the v2 whitelist that had to
// be hand-maintained per (skeleton, mechanic) pair is here a small total function.
func (m Modifier) CompatibleWith(s GameSpec) bool {
	switch m {
	case ModRunPlay:
		// Multi-card combos. On play_match: dump a set/run that matches the discard
		// (shedding). On beat_or_pass: lead a set and beat it only with a higher
		// same-size set (Big Two / President). Either way a pure superset that sheds
		// faster, so the empty-hand race still terminates.
		return s.Move == PlayMatch || s.Move == BeatOrPass
	case ModFollowSuit:
		// Move-restrict acts on the match-and-shed generator (v2: shedding host).
		return s.Move == PlayMatch
	case ModWild:
		// On play_match a single wild rank is too sparse to restore agency to
		// rank/suit-only matching (the rescue experiment: ~0.01 -> 0.02, still dead),
		// so it is NOT enumerated there. On RUMMY it is genuinely productive: a wild
		// completes melds and cuts deadwood, a real decision (deuces-wild rummy).
		return s.Move == Rummy
	case ModDrawPenalty:
		// Hand-growth penalty only bites in a hand-emptying race (v2: shedding/rummy).
		return s.Move == PlayMatch && s.End == EmptyHand
	case ModSkip:
		// A skip card (Uno-style turn-order attack) only matters in a shedding race.
		return s.Move == PlayMatch && s.End == EmptyHand
	case ModForceDraw:
		// Force-an-opponent-to-draw (Uno draw-two) is a shedding-race attack: it
		// grows the victim's hand, the inverse of self draw_penalty. The deck is
		// finite so it cannot prevent termination.
		return s.Move == PlayMatch && s.End == EmptyHand
	case ModKnock:
		// "Fewest cards wins" in an empty-hand race (shedding/climbing); on RUMMY it
		// is the Gin go-out: knock by low DEADWOOD, fewest-deadwood wins. Additive --
		// deck-out remains the floor, so it stays playable-by-construction.
		if s.Move == Rummy {
			return s.End == DeckOut
		}
		return (s.Move == PlayMatch || s.Move == BeatOrPass) && s.End == EmptyHand
	case ModTrump:
		// A trump suit that beats the led suit -- only meaningful in trick-taking.
		return s.Move == Trick
	case ModBid:
		// A bid/contract phase before the tricks: each player declares a target and
		// is scored on making it (Spades, Oh Hell, Euchre, 500, ...). Trick-only;
		// the bid round is a finite one-shot, so it terminates trivially.
		return s.Move == Trick
	case ModTeams:
		// 2v2 partnerships (evens vs odds): the winner is the best TEAM, not the
		// best individual. Needs exactly 4 seats to be a clean two-team split, and a
		// trick host where seat scores aggregate. Pure score-aggregation, no effect
		// on moves or termination.
		return s.Move == Trick && s.Players == 4
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

// contractScore is the ModBid (trick-contract) scoring for one player: tricksWon
// is gs.Scores[p]/NumPlayers (each trick banks NumPlayers cards). A bid of 0 is a
// nil (big bonus for taking none, penalty per trick taken); otherwise making the
// bid scores the bid plus small overtricks, and missing it is penalized.
func contractScore(tricksWon, bid int) int {
	if bid == 0 {
		if tricksWon == 0 {
			return 10
		}
		return -10 * tricksWon
	}
	if tricksWon >= bid {
		return 10*bid + (tricksWon - bid)
	}
	return -10 * (bid - tricksWon)
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

// beatCombos builds the Big Two / President moves under ModRunPlay on
// beat_or_pass: LEADING, play a same-rank set of any size; FOLLOWING, play a
// same-rank set of the SAME size as the table combo but a higher rank, else pass.
// Singles are size-1 sets, so this is a strict superset of single-card beat-or-pass
// and preserves termination (every play still sheds >= 1 card; the leader always
// has a set; pass is always legal when following).
func beatCombos(hand []sim.Card, table []sim.Card, leading bool, player int) []sim.Move {
	byRank := map[sim.Rank][]sim.Card{}
	for _, c := range hand {
		byRank[c.Rank] = append(byRank[c.Rank], c)
	}
	var out []sim.Move
	if leading {
		for r := sim.Rank(2); r <= 14; r++ {
			set := byRank[r]
			for size := 1; size <= len(set); size++ {
				out = append(out, mv(sim.MovePlay, player, set[:size]...))
			}
		}
		return out
	}
	k := len(table)
	for r := table[0].Rank + 1; r >= 2 && r <= 14; r++ { // r>=2 guards uint8 wrap past Ace
		if set := byRank[r]; len(set) >= k {
			out = append(out, mv(sim.MovePlay, player, set[:k]...))
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

// deadwood returns how many cards in a hand are NOT part of a meld -- the rummy
// score signal (fewest deadwood wins). Greedy melding: same-rank sets of 3+ first,
// then same-suit consecutive runs of 3+ among the leftovers. wildRank >= 0 enables
// a wild (ModWild): wild-rank cards are never deadwood themselves and each one
// completes a leftover near-meld (a pair or a 2-card run), cutting deadwood by 2 --
// a real reason to hold wilds. Greedy approximates the optimal partition but is
// monotone and rewards meld-building, the decision the game is about.
func deadwood(hand []sim.Card, wildRank int) int {
	var wilds int
	cards := make([]sim.Card, 0, len(hand))
	for _, c := range hand {
		if wildRank >= 0 && int(c.Rank) == wildRank {
			wilds++
		} else {
			cards = append(cards, c)
		}
	}
	melded := make([]bool, len(cards))
	byRank := map[sim.Rank][]int{}
	for i, c := range cards {
		byRank[c.Rank] = append(byRank[c.Rank], i)
	}
	for _, idxs := range byRank {
		if len(idxs) >= 3 {
			for _, i := range idxs {
				melded[i] = true
			}
		}
	}
	for s := 0; s < 4; s++ {
		var idxs []int
		for i, c := range cards {
			if int(c.Suit) == s && !melded[i] {
				idxs = append(idxs, i)
			}
		}
		sort.Slice(idxs, func(a, b int) bool { return cards[idxs[a]].Rank < cards[idxs[b]].Rank })
		for i := 0; i < len(idxs); {
			j := i + 1
			for j < len(idxs) && cards[idxs[j]].Rank == cards[idxs[j-1]].Rank+1 {
				j++
			}
			if j-i >= 3 {
				for k := i; k < j; k++ {
					melded[idxs[k]] = true
				}
			}
			i = j
		}
	}
	var leftover []sim.Card
	for i, m := range melded {
		if !m {
			leftover = append(leftover, cards[i])
		}
	}
	for wilds > 0 { // each wild completes one leftover near-meld (pair or 2-run)
		if i, j, ok := findNearMeld(leftover); ok {
			leftover = append(leftover[:j], leftover[j+1:]...)
			leftover = append(leftover[:i], leftover[i+1:]...)
			wilds--
		} else {
			break
		}
	}
	return len(leftover)
}

// findNearMeld returns the indices of two cards a single wild can turn into a meld:
// a pair (same rank) or two consecutive same-suit cards.
func findNearMeld(cards []sim.Card) (int, int, bool) {
	for i := 0; i < len(cards); i++ {
		for j := i + 1; j < len(cards); j++ {
			if cards[i].Rank == cards[j].Rank {
				return i, j, true
			}
			if cards[i].Suit == cards[j].Suit {
				if d := int(cards[i].Rank) - int(cards[j].Rank); d == 1 || d == -1 {
					return i, j, true
				}
			}
		}
	}
	return 0, 0, false
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

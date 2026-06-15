// Package vying implements the vying / betting skeleton (poker / brag family):
// wager chips on hidden hands across betting rounds, showdown by poker hand
// rank. It is the first family whose core decision is a WAGER, not a card play.
package vying

import (
	"sort"

	"github.com/darwindeck/darwindeck/pkg/sim"
)

// HandCategory ranks the standard poker hand types; higher is stronger.
type HandCategory int

const (
	HighCard HandCategory = iota
	OnePair
	TwoPair
	ThreeOfAKind
	Straight
	Flush
	FullHouse
	FourOfAKind
	StraightFlush
)

// HandStrength returns a comparable strength for the best 5-card poker hand in
// cards: the hand category in the high bits, then up to five rank kickers (each
// 0-14, four bits) as tiebreakers. Higher strength beats lower; equal strength
// is a genuine tie (split pot). For more than five cards it picks the best five;
// for exactly five it ranks those. Pure and deterministic -- no map iteration
// order leaks into the result (kickers are sorted), so the seeded batch stays
// reproducible.
func HandStrength(cards []sim.Card) int64 {
	if len(cards) <= 5 {
		return eval5(cards)
	}
	best := int64(-1)
	idx := make([]int, 5)
	var rec func(start, depth int)
	rec = func(start, depth int) {
		if depth == 5 {
			pick := make([]sim.Card, 5)
			for i, j := range idx {
				pick[i] = cards[j]
			}
			if s := eval5(pick); s > best {
				best = s
			}
			return
		}
		for j := start; j < len(cards); j++ {
			idx[depth] = j
			rec(j+1, depth+1)
		}
	}
	rec(0, 0)
	return best
}

// eval5 ranks up to five cards as a single poker hand.
func eval5(cards []sim.Card) int64 {
	cnt := map[sim.Rank]int{}
	suitCount := map[sim.Suit]int{}
	for _, c := range cards {
		cnt[c.Rank]++
		suitCount[c.Suit]++
	}

	isFlush := false
	for _, s := range suitCount {
		if s == 5 {
			isFlush = true
		}
	}

	// Straight: five distinct consecutive ranks, with the wheel (A-2-3-4-5)
	// counting the ace low (high card = 5).
	uniq := make([]int, 0, len(cnt))
	for r := range cnt {
		uniq = append(uniq, int(r))
	}
	sort.Sort(sort.Reverse(sort.IntSlice(uniq)))
	straightHigh := 0
	if len(uniq) == 5 {
		if uniq[0]-uniq[4] == 4 {
			straightHigh = uniq[0]
		}
		if uniq[0] == 14 && uniq[1] == 5 && uniq[2] == 4 && uniq[3] == 3 && uniq[4] == 2 {
			straightHigh = 5 // wheel
		}
	}
	isStraight := straightHigh > 0

	// Group ranks by (count desc, rank desc) so kickers compare correctly:
	// trips before the pair in a full house, high pair before low in two pair.
	type grp struct{ rank, count int }
	groups := make([]grp, 0, len(cnt))
	for r, c := range cnt {
		groups = append(groups, grp{int(r), c})
	}
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].count != groups[j].count {
			return groups[i].count > groups[j].count
		}
		return groups[i].rank > groups[j].rank
	})
	kick := make([]int, 0, 5)
	for _, g := range groups {
		for k := 0; k < g.count; k++ {
			kick = append(kick, g.rank)
		}
	}

	cat := HighCard
	switch {
	case isStraight && isFlush:
		cat = StraightFlush
	case groups[0].count == 4:
		cat = FourOfAKind
	case groups[0].count == 3 && len(groups) > 1 && groups[1].count == 2:
		cat = FullHouse
	case isFlush:
		cat = Flush
	case isStraight:
		cat = Straight
	case groups[0].count == 3:
		cat = ThreeOfAKind
	case groups[0].count == 2 && len(groups) > 1 && groups[1].count == 2:
		cat = TwoPair
	case groups[0].count == 2:
		cat = OnePair
	}

	// Straights compare on the high card only (the wheel's ace counts low), so
	// override the rank-desc kicker list which would otherwise put the ace top.
	var ks [5]int
	if cat == Straight || cat == StraightFlush {
		ks[0] = straightHigh
	} else {
		for i := 0; i < 5 && i < len(kick); i++ {
			ks[i] = kick[i]
		}
	}

	return int64(cat)<<20 |
		int64(ks[0])<<16 |
		int64(ks[1])<<12 |
		int64(ks[2])<<8 |
		int64(ks[3])<<4 |
		int64(ks[4])
}

// CategoryOf returns the hand category encoded in a HandStrength value (for
// rulebook/diagnostic display).
func CategoryOf(strength int64) HandCategory { return HandCategory(strength >> 20) }

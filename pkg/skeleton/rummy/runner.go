package rummy

import (
	"math/rand/v2"
	"sort"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

// Runner implements the rummy game skeleton.
// Draw, form melds (sets/runs), discard. Knock or go gin to end round.
type Runner struct{}

func (r *Runner) Setup(g *genome.Genome, rng *rand.Rand) *sim.GameState {
	deck := sim.StandardDeck()
	sim.ShuffleDeck(deck, rng)

	state := sim.NewGameState(g.Players)

	// Deal hands
	for i := 0; i < g.Players; i++ {
		hand, rest := sim.DrawN(deck, g.HandSize)
		state.Hands[i] = make([]sim.Card, len(hand))
		copy(state.Hands[i], hand)
		deck = rest
	}

	// Flip one card to discard pile
	if len(deck) > 0 {
		top := deck[0]
		deck = deck[1:]
		state.Discard = []sim.Card{top}
	}

	state.Deck = deck
	state.Phase = sim.PhaseDraw
	state.Melds = make([][]sim.Card, 0)
	state.MeldOwner = make([]int, 0)
	state.RNG = rng

	return state
}

func (r *Runner) GenerateMoves(state *sim.GameState, g *genome.Genome) []sim.Move {
	params := g.Rummy
	if params == nil {
		return []sim.Move{{Type: sim.MovePass, PlayerID: state.Active}}
	}

	switch state.Phase {
	case sim.PhaseDraw:
		return r.generateDrawMoves(state, params)
	case sim.PhaseMeld:
		return r.generateMeldMoves(state, params, g)
	case sim.PhaseDiscard:
		return r.generateDiscardMoves(state)
	default:
		return []sim.Move{{Type: sim.MovePass, PlayerID: state.Active}}
	}
}

func (r *Runner) generateDrawMoves(state *sim.GameState, params *genome.RummyParams) []sim.Move {
	var moves []sim.Move

	switch params.DrawFrom {
	case genome.DrawDeck:
		if len(state.Deck) > 0 {
			moves = append(moves, sim.Move{Type: sim.MoveDraw, PlayerID: state.Active})
		}
	case genome.DrawDiscard:
		if len(state.Discard) > 0 {
			moves = append(moves, sim.Move{
				Type:     sim.MoveDraw,
				Cards:    []sim.Card{state.Discard[len(state.Discard)-1]},
				PlayerID: state.Active,
			})
		}
	case genome.DrawEither:
		if len(state.Deck) > 0 {
			moves = append(moves, sim.Move{Type: sim.MoveDraw, PlayerID: state.Active})
		}
		if len(state.Discard) > 0 {
			moves = append(moves, sim.Move{
				Type:     sim.MoveDraw,
				Cards:    []sim.Card{state.Discard[len(state.Discard)-1]},
				PlayerID: state.Active,
			})
		}
	}

	// If no draws possible, pass
	if len(moves) == 0 {
		moves = append(moves, sim.Move{Type: sim.MovePass, PlayerID: state.Active})
	}

	return moves
}

func (r *Runner) generateMeldMoves(state *sim.GameState, params *genome.RummyParams, g *genome.Genome) []sim.Move {
	hand := state.ActiveHand()
	var moves []sim.Move

	// Find all valid melds in hand
	melds := findMelds(hand, params)
	for _, meld := range melds {
		moves = append(moves, sim.Move{
			Type:     sim.MoveMeld,
			Cards:    meld,
			PlayerID: state.Active,
		})
	}

	// Knock option (if deadwood is low enough)
	deadwood := calcDeadwood(hand, params)
	if deadwood <= params.KnockThreshold {
		moves = append(moves, sim.Move{
			Type:     sim.MoveKnock,
			PlayerID: state.Active,
		})
	}

	// Can always pass (skip melding, go to discard)
	moves = append(moves, sim.Move{Type: sim.MovePass, PlayerID: state.Active})

	return moves
}

func (r *Runner) generateDiscardMoves(state *sim.GameState) []sim.Move {
	hand := state.ActiveHand()
	if len(hand) == 0 {
		return []sim.Move{{Type: sim.MovePass, PlayerID: state.Active}}
	}

	moves := make([]sim.Move, len(hand))
	for i, card := range hand {
		moves[i] = sim.Move{
			Type:     sim.MoveDiscard,
			Cards:    []sim.Card{card},
			PlayerID: state.Active,
		}
	}
	return moves
}

func (r *Runner) ApplyMove(state *sim.GameState, move sim.Move, g *genome.Genome) []sim.Event {
	var events []sim.Event

	switch move.Type {
	case sim.MoveDraw:
		if len(move.Cards) > 0 {
			// Draw from discard
			card := state.Discard[len(state.Discard)-1]
			state.Discard = state.Discard[:len(state.Discard)-1]
			state.Hands[state.Active] = append(state.Hands[state.Active], card)
			events = append(events, sim.Event{
				Type:     sim.EventCardDrawn,
				PlayerID: state.Active,
				Cards:    []sim.Card{card},
				Detail:   "discard",
			})
		} else {
			// Draw from deck
			if len(state.Deck) > 0 {
				drawn, rest := sim.DrawN(state.Deck, 1)
				state.Deck = rest
				state.Hands[state.Active] = append(state.Hands[state.Active], drawn...)
				events = append(events, sim.Event{
					Type:     sim.EventCardDrawn,
					PlayerID: state.Active,
					Cards:    drawn,
					Detail:   "deck",
				})
			}
		}
		state.Phase = sim.PhaseMeld

	case sim.MoveMeld:
		// Remove meld cards from hand
		for _, card := range move.Cards {
			state.Hands[state.Active] = removeCard(state.Hands[state.Active], card)
		}
		meldCopy := make([]sim.Card, len(move.Cards))
		copy(meldCopy, move.Cards)
		state.Melds = append(state.Melds, meldCopy)
		state.MeldOwner = append(state.MeldOwner, state.Active)
		events = append(events, sim.Event{
			Type:     sim.EventMeldLaid,
			PlayerID: state.Active,
			Cards:    move.Cards,
		})
		// Gin via meld: if all cards have been laid down, end the round here.
		// Otherwise PhaseDiscard would be entered with an empty hand and
		// MovePass on an empty discard hand never advances Turn or Phase.
		if len(state.Hands[state.Active]) == 0 {
			state.Phase = sim.PhaseEnd
			events = append(events, sim.Event{
				Type:     sim.EventRoundEnd,
				PlayerID: state.Active,
				Detail:   "gin",
			})
		}
		// Otherwise stay in meld phase (can lay multiple melds)

	case sim.MoveKnock:
		// Knock: lay down the optimal disjoint partition of melds (the same
		// partition calcDeadwood scores against), then score the round.
		params := g.Rummy
		if params != nil {
			hand := state.Hands[state.Active]
			groups := bestMeldGroups(hand, params)
			for _, group := range groups {
				for _, card := range group {
					state.Hands[state.Active] = removeCard(state.Hands[state.Active], card)
				}
				meldCopy := make([]sim.Card, len(group))
				copy(meldCopy, group)
				state.Melds = append(state.Melds, meldCopy)
				state.MeldOwner = append(state.MeldOwner, state.Active)
			}
		}
		state.Phase = sim.PhaseEnd
		events = append(events, sim.Event{
			Type:     sim.EventRoundEnd,
			PlayerID: state.Active,
			Detail:   "knock",
		})

	case sim.MoveDiscard:
		card := move.Cards[0]
		state.Hands[state.Active] = removeCard(state.Hands[state.Active], card)
		state.Discard = append(state.Discard, card)
		events = append(events, sim.Event{
			Type:     sim.EventCardPlayed,
			PlayerID: state.Active,
			Cards:    []sim.Card{card},
			Detail:   "discard",
		})

		// Check for gin (empty hand after discard)
		if len(state.Hands[state.Active]) == 0 {
			state.Phase = sim.PhaseEnd
			events = append(events, sim.Event{
				Type:     sim.EventRoundEnd,
				PlayerID: state.Active,
				Detail:   "gin",
			})
		} else {
			// Next player's turn
			state.Turn++
			state.NextPlayer()
			state.Phase = sim.PhaseDraw

			// If deck is empty, reshuffle discard (keep top card)
			if len(state.Deck) == 0 && len(state.Discard) > 1 {
				top := state.Discard[len(state.Discard)-1]
				state.Deck = state.Discard[:len(state.Discard)-1]
				state.Discard = []sim.Card{top}
				// Fisher-Yates with the game's seeded RNG so reshuffles
				// stay reproducible across seeds and uniformly distributed.
				if state.RNG != nil {
					sim.ShuffleDeck(state.Deck, state.RNG)
				}
			}
		}
		return events

	case sim.MovePass:
		if state.Phase == sim.PhaseMeld {
			state.Phase = sim.PhaseDiscard
		} else if state.Phase == sim.PhaseDraw {
			state.Phase = sim.PhaseMeld
		}
	}

	// Turn is bumped only in the MoveDiscard branch (which also advances
	// the active player). Other moves stay within the same player's turn.

	return events
}

func (r *Runner) CheckEnd(state *sim.GameState, g *genome.Genome) int {
	if state.Phase == sim.PhaseEnd {
		return scoreRound(state, g)
	}

	// At max turns, return -1 so the batch runner classifies the game as a
	// genuine timeout rather than a scored completion. Scoring a hung round
	// here would mask stalled rummy genomes from Tier1 timeout detection.
	return -1
}

func scoreRound(state *sim.GameState, g *genome.Genome) int {
	params := g.Rummy
	if params == nil {
		return 0
	}

	// Score each player's deadwood
	for i := 0; i < state.NumPlayers; i++ {
		deadwood := calcDeadwood(state.Hands[i], params)
		state.Scores[i] = -deadwood // Negative deadwood = better
	}

	// Highest score (least deadwood) wins
	best := 0
	for i := 1; i < state.NumPlayers; i++ {
		if state.Scores[i] > state.Scores[best] {
			best = i
		}
	}
	return best
}

// findMelds finds all valid melds in a hand (greedy, largest first).
func findMelds(hand []sim.Card, params *genome.RummyParams) [][]sim.Card {
	var melds [][]sim.Card

	if params.MeldTypes == genome.MeldSets || params.MeldTypes == genome.MeldBoth {
		melds = append(melds, findSets(hand, params.MinMeldSize)...)
	}

	if params.MeldTypes == genome.MeldRuns || params.MeldTypes == genome.MeldBoth {
		melds = append(melds, findRuns(hand, params.MinMeldSize)...)
	}

	return melds
}

// findSets finds groups of cards with the same rank.
func findSets(hand []sim.Card, minSize int) [][]sim.Card {
	byRank := make(map[sim.Rank][]sim.Card)
	for _, c := range hand {
		byRank[c.Rank] = append(byRank[c.Rank], c)
	}

	var sets [][]sim.Card
	for _, cards := range byRank {
		if len(cards) >= minSize {
			set := make([]sim.Card, len(cards))
			copy(set, cards)
			sets = append(sets, set)
		}
	}
	return sets
}

// findRuns finds sequences of consecutive cards in the same suit.
func findRuns(hand []sim.Card, minSize int) [][]sim.Card {
	bySuit := make(map[sim.Suit][]sim.Card)
	for _, c := range hand {
		bySuit[c.Suit] = append(bySuit[c.Suit], c)
	}

	var runs [][]sim.Card
	for _, cards := range bySuit {
		if len(cards) < minSize {
			continue
		}

		// Sort by rank
		sort.Slice(cards, func(i, j int) bool {
			return cards[i].Rank < cards[j].Rank
		})

		// Find consecutive sequences
		run := []sim.Card{cards[0]}
		for i := 1; i < len(cards); i++ {
			if cards[i].Rank == cards[i-1].Rank+1 {
				run = append(run, cards[i])
			} else {
				if len(run) >= minSize {
					runCopy := make([]sim.Card, len(run))
					copy(runCopy, run)
					runs = append(runs, runCopy)
				}
				run = []sim.Card{cards[i]}
			}
		}
		if len(run) >= minSize {
			runCopy := make([]sim.Card, len(run))
			copy(runCopy, run)
			runs = append(runs, runCopy)
		}
	}
	return runs
}

// calcDeadwood calculates the total deadwood points in a hand.
// Cards not part of any meld count as deadwood.
// Face cards = 10, Ace = 1, others = face value.
//
// Maximal-meld greedy is wrong because a 4-of-a-kind set (or a long run)
// can swallow a card that another meld needs. We enumerate all valid
// sub-melds (sub-sets and sub-runs of size >= MinMeldSize) and choose the
// max-value disjoint partition via subset DP.
func calcDeadwood(hand []sim.Card, params *genome.RummyParams) int {
	n := len(hand)
	if n == 0 {
		return 0
	}

	totalValue := 0
	for _, c := range hand {
		totalValue += cardValue(c)
	}

	candidates := enumerateMeldCandidates(hand, params)
	if len(candidates) == 0 {
		return totalValue
	}

	used := bestPartition(candidates, n)
	dead := 0
	for i, card := range hand {
		if !used[i] {
			dead += cardValue(card)
		}
	}
	return dead
}

// meldCandidate is one disjoint-partition option, identified by hand indices.
type meldCandidate struct {
	mask  uint32 // bit i set if hand[i] is in this meld
	value int
}

// enumerateMeldCandidates returns every valid sub-meld at size >= MinMeldSize.
// Sub-sets: every k-combination of cards sharing a rank, for k in [min, count].
// Sub-runs: every contiguous-rank window in a suit, for length in [min, runLen].
func enumerateMeldCandidates(hand []sim.Card, params *genome.RummyParams) []meldCandidate {
	var out []meldCandidate
	min := params.MinMeldSize
	if min < 1 {
		return out
	}

	if params.MeldTypes == genome.MeldSets || params.MeldTypes == genome.MeldBoth {
		byRank := make(map[sim.Rank][]int)
		for i, c := range hand {
			byRank[c.Rank] = append(byRank[c.Rank], i)
		}
		for _, idxs := range byRank {
			if len(idxs) < min {
				continue
			}
			for size := min; size <= len(idxs); size++ {
				addCombinations(hand, idxs, size, &out)
			}
		}
	}

	if params.MeldTypes == genome.MeldRuns || params.MeldTypes == genome.MeldBoth {
		bySuit := make(map[sim.Suit][]int)
		for i, c := range hand {
			bySuit[c.Suit] = append(bySuit[c.Suit], i)
		}
		for _, idxs := range bySuit {
			if len(idxs) < min {
				continue
			}
			sort.Slice(idxs, func(i, j int) bool {
				return hand[idxs[i]].Rank < hand[idxs[j]].Rank
			})
			// Walk maximal consecutive-rank segments; equal or non-+1 ranks break.
			i := 0
			for i < len(idxs) {
				j := i + 1
				for j < len(idxs) && hand[idxs[j]].Rank == hand[idxs[j-1]].Rank+1 {
					j++
				}
				segLen := j - i
				if segLen >= min {
					for size := min; size <= segLen; size++ {
						for start := i; start+size <= j; start++ {
							var mask uint32
							v := 0
							for k := start; k < start+size; k++ {
								mask |= 1 << uint(idxs[k])
								v += cardValue(hand[idxs[k]])
							}
							out = append(out, meldCandidate{mask: mask, value: v})
						}
					}
				}
				i = j
			}
		}
	}
	return out
}

func addCombinations(hand []sim.Card, idxs []int, size int, out *[]meldCandidate) {
	chosen := make([]int, 0, size)
	var pick func(start int)
	pick = func(start int) {
		if len(chosen) == size {
			var mask uint32
			v := 0
			for _, k := range chosen {
				mask |= 1 << uint(k)
				v += cardValue(hand[k])
			}
			*out = append(*out, meldCandidate{mask: mask, value: v})
			return
		}
		need := size - len(chosen)
		for i := start; i <= len(idxs)-need; i++ {
			chosen = append(chosen, idxs[i])
			pick(i + 1)
			chosen = chosen[:len(chosen)-1]
		}
	}
	pick(0)
}

// bestPartition finds the max-value disjoint subset of candidates and returns
// which hand indices it uses. Subset DP for handSize <= 20; backtracking for
// larger (which shouldn't happen given HandSize is validated <= 13).
func bestPartition(candidates []meldCandidate, handSize int) []bool {
	masks := bestPartitionMasks(candidates, handSize)
	used := make([]bool, handSize)
	for _, m := range masks {
		for i := 0; i < handSize; i++ {
			if m&(1<<uint(i)) != 0 {
				used[i] = true
			}
		}
	}
	return used
}

// bestPartitionMasks returns the masks of the chosen meld candidates that
// maximize total meld value. MoveKnock uses this to lay down the same
// disjoint partition that calcDeadwood scores against, so the table state
// matches the reported deadwood.
func bestPartitionMasks(candidates []meldCandidate, handSize int) []uint32 {
	if handSize <= 20 {
		return bestPartitionMasksDP(candidates, handSize)
	}
	return bestPartitionMasksDFS(candidates, handSize)
}

func bestPartitionMasksDP(candidates []meldCandidate, handSize int) []uint32 {
	full := uint32(1)<<uint(handSize) - 1
	size := int(full) + 1
	value := make([]int, size)
	pickedMask := make([]uint32, size)
	pickedFrom := make([]int32, size)
	for i := range pickedFrom {
		pickedFrom[i] = -1
	}

	for mask := uint32(1); mask <= full; mask++ {
		// Best when not using lowest set bit of mask.
		lowBit := mask & (^mask + 1)
		prev := mask ^ lowBit
		value[mask] = value[prev]
		pickedMask[mask] = 0
		pickedFrom[mask] = -1

		for ci, c := range candidates {
			if c.mask&mask != c.mask {
				continue
			}
			rest := mask ^ c.mask
			if v := value[rest] + c.value; v > value[mask] {
				value[mask] = v
				pickedMask[mask] = c.mask
				pickedFrom[mask] = int32(ci)
			}
		}
	}

	var masks []uint32
	mask := full
	for mask != 0 {
		if pickedFrom[mask] < 0 {
			lowBit := mask & (^mask + 1)
			mask ^= lowBit
			continue
		}
		cMask := pickedMask[mask]
		masks = append(masks, cMask)
		mask ^= cMask
	}
	return masks
}

func bestPartitionMasksDFS(candidates []meldCandidate, handSize int) []uint32 {
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].value > candidates[j].value
	})
	bestVal := 0
	var bestMasks []uint32
	usedNow := uint64(0)
	var nowMasks []uint32
	var dfs func(idx, val int)
	dfs = func(idx, val int) {
		if val > bestVal {
			bestVal = val
			bestMasks = append(bestMasks[:0], nowMasks...)
		}
		if idx >= len(candidates) {
			return
		}
		c := candidates[idx]
		cm := uint64(c.mask)
		if usedNow&cm == 0 {
			usedNow |= cm
			nowMasks = append(nowMasks, c.mask)
			dfs(idx+1, val+c.value)
			nowMasks = nowMasks[:len(nowMasks)-1]
			usedNow &^= cm
		}
		dfs(idx+1, val)
	}
	dfs(0, 0)
	out := make([]uint32, len(bestMasks))
	copy(out, bestMasks)
	return out
}

// bestMeldGroups returns the optimal disjoint partition of hand into melds
// (each group of cards corresponds to one chosen meldCandidate). Empty when
// no valid melds exist.
func bestMeldGroups(hand []sim.Card, params *genome.RummyParams) [][]sim.Card {
	candidates := enumerateMeldCandidates(hand, params)
	if len(candidates) == 0 {
		return nil
	}
	masks := bestPartitionMasks(candidates, len(hand))
	groups := make([][]sim.Card, 0, len(masks))
	for _, m := range masks {
		var group []sim.Card
		for i := 0; i < len(hand); i++ {
			if m&(1<<uint(i)) != 0 {
				group = append(group, hand[i])
			}
		}
		if len(group) > 0 {
			groups = append(groups, group)
		}
	}
	return groups
}

func cardValue(c sim.Card) int {
	switch {
	case c.Rank == sim.Ace:
		return 1 // Ace (low in rummy deadwood)
	case c.Rank >= sim.Ten:
		return 10 // 10, J, Q, K
	default:
		return int(c.Rank) // 2-9
	}
}

// Ace is high for runs but low for deadwood — handle in rank ordering
// For simplicity, Ace is only high (14) in our rank system.
// Runs with Ace-low (A-2-3) would need special handling.

func removeCard(hand []sim.Card, card sim.Card) []sim.Card {
	for i, c := range hand {
		if c == card {
			return append(hand[:i], hand[i+1:]...)
		}
	}
	return hand
}

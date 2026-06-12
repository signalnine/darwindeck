package rummy

import (
	"math/rand/v2"
	"slices"
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

// Upkeep performs start-of-turn maintenance: when the round has ended
// (PhaseEnd via knock or gin), bank each player's deadwood into Scores. This
// used to live inside CheckEnd (scoreRound), which made CheckEnd a mutating
// query whose second call double-subtracted deadwood (audit Task 3). The
// game loop calls Upkeep exactly once per iteration and exits at the next
// CheckEnd, so the banking runs once per game.
func (r *Runner) Upkeep(state *sim.GameState, g *genome.Genome) {
	if state.Phase == sim.PhaseEnd {
		bankDeadwood(state, g)
	}
}

// Progress returns each player's progress toward winning in [0,1] (audit
// Task 8): clamp(1 - deadwood/initialDeadwoodEstimate, 0, 1). The estimate
// is HandSize * 10 -- the dealt hand at the maximum per-card deadwood value
// (see cardValue: 10/J/Q/K are all worth 10), i.e. the worst hand a player
// can be dealt. Deadwood is computed directly from live hands via
// calcDeadwood; Scores is deliberately NOT read, because deadwood is only
// banked there at round end by Upkeep, and bankDeadwood is NOT idempotent --
// Progress must stay a pure query. Lower deadwood => higher progress, the
// same ordering bestScore applies to banked scores at round end. Mid-turn
// hands hold HandSize+1 cards and can exceed the estimate; the clamp floors
// those at 0. The batch loop calls this after every applied move.
func (r *Runner) Progress(state *sim.GameState, g *genome.Genome) []float64 {
	out := make([]float64, state.NumPlayers)
	params := g.Rummy
	if params == nil {
		return out
	}
	est := g.HandSize * 10
	if est < 1 {
		est = 10
	}
	for i := 0; i < state.NumPlayers; i++ {
		p := 1 - float64(calcDeadwood(state.Hands[i], params))/float64(est)
		if p < 0 {
			p = 0
		} else if p > 1 {
			p = 1
		}
		out[i] = p
	}
	return out
}

func (r *Runner) CheckEnd(state *sim.GameState, g *genome.Genome) int {
	if state.Phase == sim.PhaseEnd {
		// Upkeep has already banked deadwood into Scores; picking the winner
		// is a pure read. Highest score (least deadwood) wins.
		return bestScore(state)
	}

	// At max turns, return -1 so the batch runner classifies the game as a
	// genuine timeout rather than a scored completion. Scoring a hung round
	// here would mask stalled rummy genomes from Tier1 timeout detection.
	return -1
}

// bankDeadwood subtracts each player's deadwood from their score. Use -= so
// any contributions already written by HookScoring/HookEndOfRound hooks
// (applyAvoidance, applyTrickScoring, applyMeldBonus) survive. Using = here
// would clobber them and silently neutralize every borrowed scoring
// mechanic that targets Rummy (dd-2lq).
func bankDeadwood(state *sim.GameState, g *genome.Genome) {
	params := g.Rummy
	if params == nil {
		return
	}
	for i := 0; i < state.NumPlayers; i++ {
		deadwood := calcDeadwood(state.Hands[i], params)
		state.Scores[i] -= deadwood
	}
}

// bestScore returns the player with the highest score (lowest index on
// ties). Pure: reads state.Scores only.
func bestScore(state *sim.GameState) int {
	best := 0
	for i := 1; i < state.NumPlayers; i++ {
		if state.Scores[i] > state.Scores[best] {
			best = i
		}
	}
	return best
}

// scoreRound banks deadwood and returns the winner. Kept for callers that
// need to force-score a round outside the normal Upkeep/CheckEnd loop
// (e.g. the test helper's max-turns fallback).
// MUST NOT be called on a state Upkeep has already banked (Phase == PhaseEnd
// after the loop's Upkeep): bankDeadwood is not idempotent and a second call
// double-subtracts deadwood, which can flip the winner.
func scoreRound(state *sim.GameState, g *genome.Genome) int {
	if g.Rummy == nil {
		return 0
	}
	bankDeadwood(state, g)
	return bestScore(state)
}

// findMelds returns every valid sub-meld in a hand (sub-sets and sub-runs of
// size >= MinMeldSize). Shares enumeration with calcDeadwood's
// enumerateMeldCandidates so the move generator offers exactly the
// partitions the scorer considers: a player holding 4-of-a-kind can choose
// any 3-card subset, and a 4-card run can be laid as either sub-run.
func findMelds(hand []sim.Card, params *genome.RummyParams) [][]sim.Card {
	candidates := enumerateMeldCandidates(hand, params)
	melds := make([][]sim.Card, 0, len(candidates))
	for _, c := range candidates {
		var group []sim.Card
		for i := 0; i < len(hand); i++ {
			if c.mask&(1<<uint(i)) != 0 {
				group = append(group, hand[i])
			}
		}
		melds = append(melds, group)
	}
	return melds
}

// findSets finds groups of cards with the same rank.
func findSets(hand []sim.Card, minSize int) [][]sim.Card {
	byRank := make(map[sim.Rank][]sim.Card)
	for _, c := range hand {
		byRank[c.Rank] = append(byRank[c.Rank], c)
	}

	// Iterate ranks in sorted order: map iteration order is randomized in Go,
	// which would make move-list order (and thus fixed-seed games)
	// nondeterministic (dd-audit-1).
	ranks := make([]sim.Rank, 0, len(byRank))
	for r := range byRank {
		ranks = append(ranks, r)
	}
	slices.Sort(ranks)

	var sets [][]sim.Card
	for _, r := range ranks {
		cards := byRank[r]
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

	// Sorted-key iteration for determinism (dd-audit-1).
	suits := make([]sim.Suit, 0, len(bySuit))
	for s := range bySuit {
		suits = append(suits, s)
	}
	slices.Sort(suits)

	var runs [][]sim.Card
	for _, s := range suits {
		cards := bySuit[s]
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

	if n <= 20 {
		// Candidates are disjoint unions of card values, so the deadwood is
		// total minus the best meld cover -- no need to reconstruct WHICH
		// cards were covered. Progress (audit Task 8) calls calcDeadwood
		// after every applied move, so this path skips bestPartitionMasksDP's
		// two reconstruction arrays and the backtrack.
		return totalValue - bestPartitionValueDP(candidates, n)
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
//
// Hand indices are bucketed by rank (2-14) and suit (0-3) into fixed arrays.
// This used to use maps with sorted-key iteration; once Progress (audit
// Task 8) started calling calcDeadwood after every applied move, map hashing
// dominated the rummy batch profile. Array indexing keeps the exact same
// deterministic candidate order (ranks/suits ascending, hand order within a
// rank, rank-ascending within a suit) with zero bucket allocations. Buckets
// hold 32 entries because meld masks are uint32 -- hands beyond 32 cards were
// never representable here.
func enumerateMeldCandidates(hand []sim.Card, params *genome.RummyParams) []meldCandidate {
	var out []meldCandidate
	min := params.MinMeldSize
	if min < 1 {
		return out
	}

	n := len(hand)
	if n > 32 {
		n = 32
	}

	if params.MeldTypes == genome.MeldSets || params.MeldTypes == genome.MeldBoth {
		var byRank [15][32]uint8
		var rankLen [15]uint8
		for i := 0; i < n; i++ {
			r := hand[i].Rank
			if r > 14 {
				continue // defensive: Rank is 2-14
			}
			byRank[r][rankLen[r]] = uint8(i)
			rankLen[r]++
		}
		for r := 2; r <= 14; r++ {
			cnt := int(rankLen[r])
			if cnt < min {
				continue
			}
			idxs := byRank[r][:cnt]
			for size := min; size <= cnt; size++ {
				addCombinations(hand, idxs, size, &out)
			}
		}
	}

	if params.MeldTypes == genome.MeldRuns || params.MeldTypes == genome.MeldBoth {
		var bySuit [4][32]uint8
		var suitLen [4]uint8
		for i := 0; i < n; i++ {
			s := hand[i].Suit
			if s > 3 {
				continue // defensive: Suit is 0-3
			}
			bySuit[s][suitLen[s]] = uint8(i)
			suitLen[s]++
		}
		for s := 0; s < 4; s++ {
			cnt := int(suitLen[s])
			if cnt < min {
				continue
			}
			idxs := bySuit[s][:cnt]
			// Insertion sort by rank (stable, allocation-free; suits hold at
			// most 13 cards).
			for a := 1; a < cnt; a++ {
				for b := a; b > 0 && hand[idxs[b]].Rank < hand[idxs[b-1]].Rank; b-- {
					idxs[b], idxs[b-1] = idxs[b-1], idxs[b]
				}
			}
			// Walk maximal consecutive-rank segments; equal or non-+1 ranks break.
			i := 0
			for i < cnt {
				j := i + 1
				for j < cnt && hand[idxs[j]].Rank == hand[idxs[j-1]].Rank+1 {
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

func addCombinations(hand []sim.Card, idxs []uint8, size int, out *[]meldCandidate) {
	chosen := make([]uint8, 0, size)
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

// bestPartitionValueDP returns only the maximum total value of a disjoint
// subset of candidates -- the value the full bestPartitionMasksDP would
// reconstruct, without its pickedMask/pickedFrom arrays or backtrack.
// calcDeadwood needs just this number, and Progress (audit Task 8) calls
// calcDeadwood for every player after every applied move.
func bestPartitionValueDP(candidates []meldCandidate, handSize int) int {
	full := uint32(1)<<uint(handSize) - 1
	value := make([]int, int(full)+1)
	for mask := uint32(1); mask <= full; mask++ {
		// Best when not using the lowest set bit of mask.
		lowBit := mask & (^mask + 1)
		best := value[mask^lowBit]
		for _, c := range candidates {
			if c.mask&mask != c.mask {
				continue
			}
			if v := value[mask^c.mask] + c.value; v > best {
				best = v
			}
		}
		value[mask] = best
	}
	return value[full]
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

// --- Choice-impact probe (Task 28 round 3, rummy density fix) ---

// ChoiceMatters implements sim.ChoiceConsequenceProber: the rummy-real
// meaningfulness test that replaced the count-based exception ("meaningful
// iff >= 2 legal moves"), which the r2 flagship's pair-meld archetype gamed
// (min_meld_size 2 inflates option counts in every phase; pinned density
// 0.80 > gin 0.69). A rummy turn is meaningful iff the acting player's
// options differ in DEADWOOD CONSEQUENCE, per phase:
//
//	draw    -- a real deck-vs-discard choice exists AND the KNOWN top card
//	           is a structural information edge: it enters the best
//	           partition (deadwood(hand+top) < deadwood(hand) + value(top))
//	           AND its meld gain beats the best of the sampled deck cards --
//	           the lottery's plausible outcomes. A structurally inert top is
//	           a deadwood lottery ticket the deck offers equally; a melding
//	           top in a pairing-trivial genome is no edge because most deck
//	           cards meld too.
//	meld    -- NEVER meaningful: the phase is consequence-free on hands (see
//	           the inline comment -- laying is already credited by the best
//	           partition, knocking changes no hand). Knock-timing quality is
//	           the skill metric's domain, not density's.
//	discard -- the sampled discard options span a deadwood-delta range > 0:
//	           not all resulting best-partition deadwoods are equal. Sampled
//	           at the same deterministic index spread as the sim package's
//	           choice probes (first/last/third-points, up to 4) to bound the
//	           DP cost.
//
// END-AT-WILL VOIDING (draw + discard): when the acting player's hand
// already satisfies the knock condition (deadwood <= KnockThreshold), the
// round continues only at their pleasure -- the one live decision left is
// knock/no-knock, and it is counted exactly once, at the meld phase.
// Counting every draw/discard made in that state as meaningful would
// double-count that single decision N times per cycle; this was the
// pair-meld archetype's residual inflation (loose thresholds over trivially
// melding hands keep every seat end-at-will from the second cycle, measured
// discard rate 0.996 while the game is a knock race). Gin-shaped genomes
// (threshold 0) are essentially never end-at-will before the auto-win, so
// their discard decisions keep full weight.
//
// The probe must stay PURE (no state mutation): the batch runner calls it on
// the live game state before the chosen move applies.
func (r *Runner) ChoiceMatters(state *sim.GameState, g *genome.Genome, moves []sim.Move) bool {
	params := g.Rummy
	if params == nil || len(moves) < 2 {
		return false
	}
	hand := state.Hands[state.Active]

	switch state.Phase {
	case sim.PhaseDraw:
		// Identify the discard-top draw among the options (Cards non-empty).
		var top *sim.Card
		for i := range moves {
			if moves[i].Type == sim.MoveDraw && len(moves[i].Cards) > 0 {
				top = &moves[i].Cards[0]
			}
		}
		if top == nil {
			return false // deck-only: no draw choice exists
		}
		base := calcDeadwood(hand, params)
		if base <= params.KnockThreshold {
			// END-AT-WILL VOIDING (see the type doc): the hand already
			// satisfies the knock condition; the live decision is
			// knock/no-knock, counted once at the meld phase.
			return false
		}
		// Structural gain of a candidate card at two tiers: full melds
		// (the real partition) and NEAR-melds (proto-melds: MinMeldSize-1
		// groups, floored at 2 -- the pair/two-card-run building blocks that
		// make gin's draw decision real long before a meld completes). The
		// near tier collapses onto the full tier for pairing-trivial genomes
		// (MinMeldSize 2), so it can never double-credit them.
		probe := newGainProbe(hand, base, params)
		topFull := probe.full(*top)
		topNear := probe.near(*top)
		if topFull <= 0 && topNear <= 0 {
			return false // structurally inert top: the deck lottery dominates
		}
		// The known top is an information edge only if it CLEARLY dominates
		// the lottery: it must strictly beat the best of the sampled deck
		// cards (the lottery's plausible best, ~upper quartile of 4 spread
		// samples) lexicographically -- full-meld gain first, proto-meld
		// gain as the tiebreak. In a pairing-trivial genome most deck cards
		// meld too (the pair-meld archetype's draw inflation: 0.44
		// "meaningful" under a naive top-melds test); in a gin-shaped genome
		// the deck's sampled best is usually 0 and a melding or pairing top
		// is a real edge.
		deck := state.Deck
		if len(deck) == 0 {
			return true // no lottery alternative left to compare against
		}
		idxs := [4]int{0, len(deck) - 1, len(deck) / 3, (2 * len(deck)) / 3}
		bestFull, bestNear := 0, 0
		seenIdx := -1
		for _, di := range idxs {
			if di == seenIdx { // collapse duplicate spread indices on tiny decks
				continue
			}
			seenIdx = di
			full := probe.full(deck[di])
			if full > bestFull {
				bestFull, bestNear = full, probe.near(deck[di])
			} else if full == bestFull {
				// Near tier is only a tiebreak: compute it lazily.
				if near := probe.near(deck[di]); near > bestNear {
					bestNear = near
				}
			}
		}
		return topFull > bestFull || (topFull == bestFull && topNear > bestNear)

	case sim.PhaseMeld:
		// The meld phase is consequence-FREE on hands in this engine, so no
		// meld-phase record is a meaningful density decision:
		//   - meld vs pass: calcDeadwood's best partition already credits
		//     melds held in hand, and bankDeadwood scores hands by the same
		//     partition, so laying changes nothing.
		//   - knock vs pass: knocking ends the round but changes NO hand's
		//     deadwood -- the winner is lowest-deadwood either way. Knock
		//     TIMING quality is the SKILL metric's domain, where the ISMCTS
		//     tier measurably rewards it (TestMCTSTierRewardsDegenKnockTiming);
		//     crediting it to density let knock-race archetypes (loose
		//     thresholds, instant melds) bank a meaningful record on every
		//     knock-legal meld visit -- and a turn laying k melds re-banked
		//     it k times.
		return false

	case sim.PhaseDiscard:
		base := calcDeadwood(hand, params)
		// END-AT-WILL VOIDING (see the type doc): a knockable hand's discard
		// micro-choice is subordinate to the knock decision itself.
		if base <= params.KnockThreshold {
			return false
		}
		// Deterministic sample spread, mirroring the sim package's
		// choiceSampleIndices: first, last, and the two third-points.
		// Throughput: a discard that participates in no meld candidate
		// (canParticipate prefilter) removes exactly its own deadwood, so
		// its resulting deadwood is base - value(d) with no DP.
		probe := newGainProbe(hand, base, params)
		n := len(moves)
		indices := [4]int{0, n - 1, n / 3, (2 * n) / 3}
		scratch := make([]sim.Card, 0, len(hand))
		first := 0
		seen := false
		for _, idx := range indices {
			m := moves[idx]
			if m.Type != sim.MoveDiscard || len(m.Cards) == 0 {
				continue
			}
			var dw int
			if !probe.canParticipate(m.Cards[0], params) {
				dw = base - cardValue(m.Cards[0])
			} else {
				scratch = scratch[:0]
				removed := false
				for _, c := range hand {
					if !removed && c == m.Cards[0] {
						removed = true
						continue
					}
					scratch = append(scratch, c)
				}
				dw = calcDeadwood(scratch, params)
			}
			if !seen {
				first, seen = dw, true
			} else if dw != first {
				return true
			}
		}
		return false
	}

	// Unknown phase: preserve the count semantics (>= 2 moves) rather than
	// silently zeroing a future phase's density.
	return true
}

// meldImprovement returns how many deadwood points card c sheds by entering
// the hand's best partition: base + value(c) - deadwood(hand+c). Zero means
// the card sits outside every meld (pure deadwood); positive means it joins
// or completes one. buf is a caller-owned scratch slice reused across calls
// so probe loops stay allocation-light.
func meldImprovement(hand []sim.Card, c sim.Card, base int, params *genome.RummyParams, buf *[]sim.Card) int {
	*buf = append(append((*buf)[:0], hand...), c)
	return base + cardValue(c) - calcDeadwood(*buf, params)
}

// gainProbe computes two-tier structural gains for candidate cards against a
// fixed hand, with the throughput hot path in mind (the probe runs on every
// multi-option draw record; the deadwood DP is the expensive part):
//   - the base is computed once per probe;
//   - canParticipate is a no-DP prefilter: a card with NO same-rank hand
//     card and NO same-suit card within run reach takes part in no meld
//     candidate, so its gain is exactly 0 -- most deck samples in gin-shaped
//     genomes exit here;
//   - the near tier is an O(hand) pairwise surrogate, never a DP: at
//     MinMeldSize-1 the candidate count explodes (every pair is a
//     candidate) and the DP was measured at 56% of the whole rummy batch.
type gainProbe struct {
	hand   []sim.Card
	base   int
	params *genome.RummyParams
	buf    []sim.Card
}

func newGainProbe(hand []sim.Card, base int, params *genome.RummyParams) *gainProbe {
	return &gainProbe{
		hand:   hand,
		base:   base,
		params: params,
		buf:    make([]sim.Card, 0, len(hand)+1),
	}
}

func (p *gainProbe) full(c sim.Card) int {
	if !p.canParticipate(c, p.params) {
		return 0
	}
	return meldImprovement(p.hand, c, p.base, p.params, &p.buf)
}

// near scores c's PROTO-MELD strength: the value of the strongest pairwise
// building block c forms with a hand card (value(c) + best compatible
// partner) -- the pair/two-card-run combinations that make gin's draw
// decision real long before a meld completes. For pairing-trivial genomes
// (MinMeldSize 2) a "proto-meld" IS a full meld, so the tier collapses onto
// full() and can never double-credit them. An O(hand) surrogate by design:
// magnitudes are only ever compared lexicographically after the full tier,
// between candidates scored by the same rule.
func (p *gainProbe) near(c sim.Card) int {
	if p.params.MinMeldSize <= 2 {
		return p.full(c)
	}
	sets := p.params.MeldTypes == genome.MeldSets || p.params.MeldTypes == genome.MeldBoth
	runs := p.params.MeldTypes == genome.MeldRuns || p.params.MeldTypes == genome.MeldBoth
	reach := p.params.MinMeldSize - 1
	best := 0
	for _, h := range p.hand {
		compatible := false
		if sets && h.Rank == c.Rank {
			compatible = true
		}
		if runs && h.Suit == c.Suit {
			d := int(h.Rank) - int(c.Rank)
			if d < 0 {
				d = -d
			}
			if d != 0 && d <= reach {
				compatible = true
			}
		}
		if compatible {
			if v := cardValue(h); v > best {
				best = v
			}
		}
	}
	if best == 0 {
		return 0
	}
	return cardValue(c) + best
}

// canParticipate reports whether c could join ANY meld candidate with the
// hand under the given params: a same-rank card (sets) or a same-suit card
// within MinMeldSize-1 ranks (runs). If neither exists, adding c cannot
// change the best partition and its gain is exactly 0 -- no DP needed.
func (p *gainProbe) canParticipate(c sim.Card, params *genome.RummyParams) bool {
	sets := params.MeldTypes == genome.MeldSets || params.MeldTypes == genome.MeldBoth
	runs := params.MeldTypes == genome.MeldRuns || params.MeldTypes == genome.MeldBoth
	reach := params.MinMeldSize - 1
	for _, h := range p.hand {
		if sets && h.Rank == c.Rank {
			return true
		}
		if runs && h.Suit == c.Suit {
			d := int(h.Rank) - int(c.Rank)
			if d < 0 {
				d = -d
			}
			if d != 0 && d <= reach {
				return true
			}
		}
	}
	return false
}

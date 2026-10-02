package grammar

import (
	"math/rand/v2"

	"github.com/darwindeck/darwindeck/pkg/sim"
	"github.com/darwindeck/darwindeck/pkg/skeleton/vying"
)

// maxRaises caps the raises per vying betting round -- the bound that makes the
// betting move-gen terminate under random play (mirrors v2's max_raises).
const maxRaises = 4

func nonFolded(gs *sim.GameState) int {
	n := 0
	for _, f := range gs.Folded {
		if !f {
			n++
		}
	}
	return n
}

func nextNonFolded(gs *sim.GameState, from int) int {
	for i := 1; i <= gs.NumPlayers; i++ {
		if q := (from + i) % gs.NumPlayers; !gs.Folded[q] {
			return q
		}
	}
	return from
}

// Runner interprets a GameSpec over sim.GameState. It is the single generic
// engine that plays ANY composition -- the whole point of the grammar.
type Runner struct{ Spec GameSpec }

func cardValue(r sim.Rank) int {
	v := int(r)
	switch {
	case v >= 11 && v <= 13: // J, Q, K
		return 10
	case v == 14: // Ace high
		return 11
	default:
		return v // 2-10
	}
}

func lastCard(cards []sim.Card) (sim.Card, bool) {
	if len(cards) == 0 {
		return sim.Card{}, false
	}
	return cards[len(cards)-1], true
}

// topOf reads the match reference from the shedding-conventional state.TopCard.
func topOf(gs *sim.GameState) (sim.Card, bool) {
	if gs.TopCard == nil {
		return sim.Card{}, false
	}
	return *gs.TopCard, true
}

// setTop points state.TopCard at a copy of c (the new discard top).
func setTop(gs *sim.GameState, c sim.Card) {
	cp := c
	gs.TopCard = &cp
}

func matches(c, top sim.Card, ok bool, rule MatchRule) bool {
	if !ok {
		return true // empty discard: anything plays
	}
	switch rule {
	case MatchSuit:
		return c.Suit == top.Suit
	case MatchRank:
		return c.Rank == top.Rank
	default: // MatchEither
		return c.Suit == top.Suit || c.Rank == top.Rank
	}
}

func removeCard(hand []sim.Card, c sim.Card) []sim.Card {
	for i, h := range hand {
		if h.Suit == c.Suit && h.Rank == c.Rank {
			return append(hand[:i:i], hand[i+1:]...)
		}
	}
	return hand
}

func mv(t sim.MoveType, player int, cards ...sim.Card) sim.Move {
	return sim.Move{Type: t, PlayerID: player, Cards: cards}
}

func hasRank(cards []sim.Card, rank int) bool {
	for _, c := range cards {
		if int(c.Rank) == rank {
			return true
		}
	}
	return false
}

// ginKnockThreshold is the deadwood at or below which a rummy player may knock
// (ModKnock on rummy = the Gin go-out).
const ginKnockThreshold = 2

// followSuitFilter (ModFollowSuit) keeps only plays that follow the discard suit
// (or a wild, or -- under ModNominate -- a rank-8: "you may play an eight on
// anything") when the player holds the suit -- the move-RESTRICT modifier. If the
// player is void in the suit, or the filter would empty the set, it leaves the
// moves untouched so the never-empty invariant holds.
func followSuitFilter(moves []sim.Move, hand []sim.Card, top sim.Card, wild, nominate bool) []sim.Move {
	holds := false
	for _, c := range hand {
		if c.Suit == top.Suit {
			holds = true
			break
		}
	}
	if !holds {
		return moves
	}
	kept := moves[:0]
	for _, m := range moves {
		if m.Type != sim.MovePlay {
			continue
		}
		for _, c := range m.Cards {
			if c.Suit == top.Suit || isWild(c, wild) || (nominate && int(c.Rank) == wildRank) {
				kept = append(kept, m)
				break
			}
		}
	}
	if len(kept) == 0 {
		return moves // obligation unsatisfiable (e.g. only a wild qualifies): fall through
	}
	return kept
}

// appendKnock (ModKnock) adds a knock move for a small hand -- the win-override
// modifier. ADDITIVE: every play/draw/pass remains, so the set never empties.
func (rr Runner) appendKnock(moves []sim.Move, gs *sim.GameState, p int) []sim.Move {
	if !rr.Spec.hasMod(ModKnock) {
		return moves
	}
	if n := len(gs.Hands[p]); n >= 1 && n <= 3 {
		moves = append(moves, mv(sim.MoveKnock, p))
	}
	return moves
}

// Setup deals a fresh game per the spec.
func (rr Runner) Setup(rng *rand.Rand) *sim.GameState {
	s := rr.Spec
	deck := sim.StandardDeck()
	sim.ShuffleDeck(deck, rng)
	gs := sim.NewGameState(s.Players)
	gs.RNG = rng
	gs.NumPlayers = s.Players
	for p := 0; p < s.Players; p++ {
		h, rem := sim.DrawN(deck, s.Deal)
		gs.Hands[p] = h
		deck = rem
	}
	if s.Shared > 0 {
		sh, rem := sim.DrawN(deck, s.Shared)
		gs.Discard = sh
		deck = rem
		if c, ok := lastCard(gs.Discard); ok {
			setTop(gs, c) // shedding match reference (read by the runner + the metric probe)
		}
	}
	gs.Deck = deck
	if s.Move == Trick {
		// Trick-taking plays out the dealt hands only; any undealt remainder is a
		// dead kitty, so drop it -- then deck_out fires the moment hands empty.
		gs.Deck = nil
		gs.TrumpSuit = -1 // no trump (the TrickTakingScorer treats <0 as none)
		if s.hasMod(ModTrump) {
			gs.TrumpSuit = trumpSuit // Spades trump (also makes the TrickTakingScorer trump-aware)
		}
		gs.TrickCards = nil
		gs.TrickPlayers = nil
		gs.TrickLeader = 0
		if s.hasMod(ModBid) {
			gs.Bids = make([]int, s.Players) // -1 = not yet bid; each seat bids before play
			for p := range gs.Bids {
				gs.Bids[p] = -1
			}
		}
	}
	if s.Move == Rummy {
		gs.Phase = sim.PhaseDraw // a rummy turn is two moves: draw, then discard
	}
	if s.Move == Vying {
		gs.Committed = make([]int, s.Players)
		gs.CurrentBet, gs.RaiseCount, gs.Pot = 0, 0, 0
		gs.ToAct = s.Players // every seat owes at least one action this round
	}
	gs.Folded = make([]bool, s.Players)
	gs.Active = 0
	return gs
}

// LegalMoves returns the legal moves for the active player. INVARIANT: never
// empty -- every generator carries an unconditional fallback.
func (rr Runner) LegalMoves(gs *sim.GameState) []sim.Move {
	p := gs.Active
	hand := gs.Hands[p]
	switch rr.Spec.Move {
	case PlayMatch:
		// Match against state.TopCard (the shedding-conventional field) so the
		// fitness layer's choice-impact probe -- which hypothesizes by swapping
		// TopCard and re-running this generator -- actually perturbs the move set.
		top, ok := topOf(gs)
		wild := rr.Spec.hasMod(ModWild)
		nominate := rr.Spec.hasMod(ModNominate)
		var moves []sim.Move
		for _, c := range hand {
			if nominate && int(c.Rank) == wildRank { // wild 8: always legal, name the next suit
				for suit := 0; suit < 4; suit++ {
					moves = append(moves, sim.Move{Type: sim.MovePlay, PlayerID: p, Cards: []sim.Card{c}, Amount: suit})
				}
				continue
			}
			if matches(c, top, ok, rr.Spec.Match) || isWild(c, wild) {
				moves = append(moves, mv(sim.MovePlay, p, c))
			}
		}
		if rr.Spec.hasMod(ModRunPlay) { // EXPAND: same-rank set / same-suit run combos
			combos := comboPlays(hand, top, ok, rr.Spec.Match, p)
			if nominate {
				// A combo ending in the wild rank nominates in Apply, so it needs
				// the same 4-suit branching as a single-card 8 -- otherwise
				// Amount's zero value silently names clubs with no player choice.
				combos = expandNominate(combos)
			}
			moves = append(moves, combos...)
		}
		if rr.Spec.hasMod(ModFollowSuit) && ok { // RESTRICT: must follow the discard suit if held
			moves = followSuitFilter(moves, hand, top, wild, nominate)
		}
		if len(moves) == 0 { // fallback: draw, or pass if the deck is gone
			if len(gs.Deck) > 0 {
				moves = append(moves, mv(sim.MoveDraw, p))
			} else {
				moves = append(moves, mv(sim.MovePass, p))
			}
		}
		return rr.appendKnock(moves, gs, p)

	case BeatOrPass:
		// The combination to beat lives in state.TrickCards (the climbing-
		// conventional field the ClimbingScorer + deltaModeClimbing probe read),
		// not Discard -- so the fitness layer sees the beat/pass coupling.
		leading := len(gs.TrickCards) == 0
		var moves []sim.Move
		if rr.Spec.hasMod(ModRunPlay) { // Big Two: lead a set, beat with a higher same-size set
			moves = beatCombos(hand, gs.TrickCards, leading, p)
		} else {
			for _, c := range hand {
				if leading || c.Rank > gs.TrickCards[0].Rank {
					moves = append(moves, mv(sim.MovePlay, p, c))
				}
			}
		}
		if !leading {
			moves = append(moves, mv(sim.MovePass, p)) // pass: always legal when following
		}
		if len(moves) == 0 { // leading with an empty hand: end will fire; pass is the floor
			moves = append(moves, mv(sim.MovePass, p))
		}
		return rr.appendKnock(moves, gs, p)

	case Accumulate:
		var moves []sim.Move
		if top, ok := lastCard(gs.Discard); ok {
			moves = append(moves, mv(sim.MovePlay, p, top)) // take the face-up card
		}
		if len(gs.Deck) > 0 {
			moves = append(moves, mv(sim.MoveDraw, p)) // take a blind card
		}
		// STICK is the fallback, but only once you have taken a card: sticking on
		// an empty pile let two players tie 0-0 with nothing to compare (one
		// random 2-player game in nine), which only seat order could settle.
		// With nothing left to take, stick is unconditional -- never empty.
		if len(gs.Tableau[p]) > 0 || len(moves) == 0 {
			moves = append(moves, mv(sim.MovePass, p))
		}
		return moves

	case Capture:
		// Enumerate the capture CHOICES (which table cards a card takes), so the
		// decision is explicit -- the next player's options shift with the table,
		// which the interaction metric reads. Trail is always available.
		var moves []sim.Move
		sum := rr.Spec.hasMod(ModSumCapture)
		for _, c := range hand {
			for _, capt := range captureOptions(gs.Discard, c, sum) {
				cards := append([]sim.Card{c}, capt...)
				moves = append(moves, sim.Move{Type: sim.MoveCapture, PlayerID: p, Cards: cards})
			}
			moves = append(moves, mv(sim.MovePlay, p, c)) // trail
		}
		if len(moves) == 0 {
			moves = append(moves, mv(sim.MovePass, p)) // empty hand: pass (refill in Upkeep)
		}
		return moves

	case Rummy:
		// A turn is two moves: DRAW a card from the deck (forced -- the deck drains
		// one per turn, which is what terminates the game), then DISCARD any card.
		// The discard choice (keep meld cards, shed deadwood) is the whole decision.
		if gs.Phase == sim.PhaseDiscard {
			var moves []sim.Move
			for _, c := range hand {
				moves = append(moves, mv(sim.MoveDiscard, p, c))
			}
			return moves // hand is Deal+1 here, never empty
		}
		// Draw phase: draw from the deck, or (ModKnock = Gin go-out) knock when your
		// deadwood is low. Additive -- deck-out is still the floor.
		var moves []sim.Move
		if len(gs.Deck) > 0 {
			moves = append(moves, mv(sim.MoveDraw, p))
		}
		if rr.Spec.hasMod(ModKnock) {
			wr := -1
			if rr.Spec.hasMod(ModWild) {
				wr = wildRank
			}
			if deadwood(hand, wr) <= ginKnockThreshold {
				moves = append(moves, mv(sim.MoveKnock, p))
			}
		}
		if len(moves) == 0 {
			moves = append(moves, mv(sim.MovePass, p)) // deck empty: deck_out fires in CheckEnd
		}
		return moves

	case Trick:
		// ModBid: a one-shot bid round before any tricks -- the active player
		// declares a target 0..Deal. Once every seat has bid, play begins.
		if rr.Spec.hasMod(ModBid) && gs.Bids[p] == -1 {
			moves := make([]sim.Move, 0, rr.Spec.Deal+1)
			for b := 0; b <= rr.Spec.Deal; b++ {
				moves = append(moves, sim.Move{Type: sim.MoveBid, PlayerID: p, Amount: b})
			}
			return moves
		}
		// Lead the trick with any card; otherwise FOLLOW the lead suit if you hold
		// it, else play anything. TrickCards holds the cards played so far this trick
		// (TrickCards[0] = the lead), so the TrickTakingScorer + delta probe read it.
		var moves []sim.Move
		if len(gs.TrickCards) > 0 {
			lead := gs.TrickCards[0].Suit
			for _, c := range hand {
				if c.Suit == lead {
					moves = append(moves, mv(sim.MovePlay, p, c))
				}
			}
		}
		if len(moves) == 0 { // leading, or void in the lead suit: any card
			for _, c := range hand {
				moves = append(moves, mv(sim.MovePlay, p, c))
			}
		}
		if len(moves) == 0 {
			moves = append(moves, mv(sim.MovePass, p)) // empty hand (deck_out is firing)
		}
		return moves

	case Vying:
		// Betting round: if you owe nothing you may check; if you owe, call or fold;
		// you may raise until the per-round cap. The cap bounds the round so it
		// terminates under random play.
		var moves []sim.Move
		if gs.CurrentBet-gs.Committed[p] <= 0 {
			moves = append(moves, mv(sim.MoveCheck, p))
		} else {
			moves = append(moves, mv(sim.MoveCall, p), mv(sim.MoveFold, p))
		}
		if gs.RaiseCount < maxRaises {
			moves = append(moves, mv(sim.MoveRaise, p))
		}
		return moves
	}
	return []sim.Move{mv(sim.MovePass, p)}
}

// Apply mutates the state for the chosen move and advances the turn.
func (rr Runner) Apply(gs *sim.GameState, m sim.Move) {
	p := gs.Active
	s := rr.Spec
	if m.Type == sim.MoveKnock { // ModKnock: end the game now; winner is fewest cards (CheckEnd)
		gs.Phase = sim.PhaseEnd
		gs.Turn++
		return
	}
	if m.Type == sim.MoveBid { // ModBid: record the contract and pass to the next bidder
		gs.Bids[p] = m.Amount
		gs.Active = (p + 1) % gs.NumPlayers // after the last seat bids, wraps to 0 to lead trick 1
		gs.Turn++                           // a bid is a turn, like MoveKnock above (SpecGenome's RoundsPerGame=2 cap leaves ample headroom)
		return
	}
	if s.Move == Rummy { // two-phase turn; counts ONE Turn per draw+discard pair
		if gs.Phase == sim.PhaseDraw && m.Type == sim.MoveDraw {
			drawn, rem := sim.DrawN(gs.Deck, 1)
			gs.Hands[p] = append(gs.Hands[p], drawn...)
			gs.Deck = rem
			gs.Phase = sim.PhaseDiscard // same player discards next
		} else { // discard
			gs.Hands[p] = removeCard(gs.Hands[p], m.Cards[0])
			gs.Discard = append(gs.Discard, m.Cards[0])
			gs.Phase = sim.PhaseDraw
			gs.Active = (p + 1) % gs.NumPlayers
			gs.Turn++
		}
		return
	}
	switch s.Move {
	case PlayMatch:
		switch m.Type {
		case sim.MovePlay: // one or many cards (ModRunPlay combos); last becomes the new top
			for _, c := range m.Cards {
				gs.Hands[p] = removeCard(gs.Hands[p], c)
			}
			gs.Discard = append(gs.Discard, m.Cards...)
			last := m.Cards[len(m.Cards)-1]
			if rr.Spec.hasMod(ModNominate) && int(last.Rank) == wildRank {
				// ModNominate: the played 8 names the next required suit (Move.Amount).
				setTop(gs, sim.Card{Rank: sim.Rank(wildRank), Suit: sim.Suit(m.Amount)})
			} else {
				setTop(gs, last)
			}
			gs.PassCount = 0
			if rr.Spec.hasMod(ModDrawPenalty) && len(gs.Deck) > 0 { // face-card play -> draw one
				if last := m.Cards[len(m.Cards)-1]; int(last.Rank) >= 11 {
					drawn, rem := sim.DrawN(gs.Deck, 1)
					gs.Hands[p] = append(gs.Hands[p], drawn...)
					gs.Deck = rem
				}
			}
		case sim.MoveDraw:
			drawn, rem := sim.DrawN(gs.Deck, 1)
			gs.Hands[p] = append(gs.Hands[p], drawn...)
			gs.Deck = rem
			gs.PassCount = 0
		case sim.MovePass: // only reachable with an empty deck and no legal play
			gs.PassCount++
		}
		if rr.Spec.hasMod(ModReverse) && m.Type == sim.MovePlay && hasRank(m.Cards, reverseRank) {
			if gs.Direction == 0 {
				gs.Direction = 1
			}
			gs.Direction = -gs.Direction // flip, then advance in the new direction
		}
		dir := gs.Direction
		if dir == 0 {
			dir = 1
		}
		step := 1
		if m.Type == sim.MovePlay {
			if rr.Spec.hasMod(ModSkip) && hasRank(m.Cards, skipRank) {
				step = 2 // ModSkip: playing the skip rank skips the next player
			}
			if rr.Spec.hasMod(ModForceDraw) && hasRank(m.Cards, forceDrawRank) {
				victim := wrap(p+dir, gs.NumPlayers)
				drawn, rem := sim.DrawN(gs.Deck, forceDrawN) // bounded by the (finite) deck
				gs.Hands[victim] = append(gs.Hands[victim], drawn...)
				gs.Deck = rem
				step = 2 // the victim drew and loses their turn
			}
		}
		gs.Active = wrap(p+step*dir, gs.NumPlayers)

	case BeatOrPass:
		switch m.Type {
		case sim.MovePlay:
			for _, c := range m.Cards { // one card, or a same-rank set under ModRunPlay
				gs.Hands[p] = removeCard(gs.Hands[p], c)
			}
			// The beaten combination goes to the discard pile -- overwriting it
			// silently dropped cards from the game (52 -> 51 from turn 2 on).
			gs.Discard = append(gs.Discard, gs.TrickCards...)
			gs.TrickCards = append([]sim.Card(nil), m.Cards...) // new combination to beat
			gs.TrickLeader = p
			gs.PassCount = 0
		case sim.MovePass:
			gs.PassCount++
			if gs.PassCount >= gs.NumPlayers-1 { // all others passed: table clears
				gs.Discard = append(gs.Discard, gs.TrickCards...) // cleared cards are discarded, not lost
				gs.TrickCards = nil
				gs.PassCount = 0
			}
		}
		gs.Active = (p + 1) % gs.NumPlayers

	case Accumulate:
		switch m.Type {
		case sim.MovePlay: // take the face-up card
			c := m.Cards[0]
			if top, ok := lastCard(gs.Discard); ok && top == c {
				gs.Discard = gs.Discard[:len(gs.Discard)-1]
			}
			gs.Scores[p] += cardValue(c.Rank)
			gs.Tableau[p] = append(gs.Tableau[p], c) // the taken card is KEPT in the taker's pile, not dropped
			rr.refillMarket(gs)
			if gs.Scores[p] > s.Target {
				gs.Folded[p] = true // bust
			}
		case sim.MoveDraw: // take a blind card
			drawn, rem := sim.DrawN(gs.Deck, 1)
			gs.Deck = rem
			if len(drawn) > 0 {
				gs.Scores[p] += cardValue(drawn[0].Rank)
				gs.Tableau[p] = append(gs.Tableau[p], drawn[0])
				if gs.Scores[p] > s.Target {
					gs.Folded[p] = true
				}
			}
		case sim.MovePass: // stick
			gs.Folded[p] = true
		}
		gs.Active = rr.nextActive(gs)

	case Capture:
		switch m.Type {
		case sim.MoveCapture: // Cards[0] played, Cards[1:] the captured table set
			c := m.Cards[0]
			gs.Hands[p] = removeCard(gs.Hands[p], c)
			for _, t := range m.Cards[1:] {
				gs.Discard = removeCard(gs.Discard, t)
			}
			gs.Scores[p] += len(m.Cards) // captured cards + the played card
			gs.Tableau[p] = append(gs.Tableau[p], m.Cards...)
		case sim.MovePlay: // trail
			c := m.Cards[0]
			gs.Hands[p] = removeCard(gs.Hands[p], c)
			gs.Discard = append(gs.Discard, c)
		}
		gs.Active = (p + 1) % gs.NumPlayers

	case Trick:
		if m.Type == sim.MovePlay {
			c := m.Cards[0]
			gs.Hands[p] = removeCard(gs.Hands[p], c)
			gs.TrickCards = append(gs.TrickCards, c)
			gs.TrickPlayers = append(gs.TrickPlayers, p)
		}
		if len(gs.TrickCards) >= gs.NumPlayers { // trick complete: resolve it
			lead := gs.TrickCards[0].Suit
			trump := gs.TrumpSuit // -1 when no ModTrump
			win, winCard := gs.TrickPlayers[0], gs.TrickCards[0]
			winTrump := trump >= 0 && int(winCard.Suit) == trump
			for i := 1; i < len(gs.TrickCards); i++ {
				tc := gs.TrickCards[i]
				tcTrump := trump >= 0 && int(tc.Suit) == trump
				beats := false
				switch {
				case tcTrump && !winTrump: // any trump beats a non-trump
					beats = true
				case tcTrump == winTrump && tcTrump: // both trump: higher rank
					beats = tc.Rank > winCard.Rank
				case tcTrump == winTrump: // neither trump: must be lead suit and higher
					beats = tc.Suit == lead && tc.Rank > winCard.Rank
				}
				if beats {
					win, winCard, winTrump = gs.TrickPlayers[i], tc, tcTrump
				}
			}
			gs.Scores[win] += len(gs.TrickCards) // cards won (the most-captured signal)
			gs.Tableau[win] = append(gs.Tableau[win], gs.TrickCards...)
			gs.TrickCards, gs.TrickPlayers, gs.TrickLeader = nil, nil, win
			gs.Active = win // the winner leads the next trick
		} else {
			gs.Active = (p + 1) % gs.NumPlayers
		}

	case Vying:
		switch m.Type {
		case sim.MoveCheck:
			gs.ToAct--
		case sim.MoveCall:
			owed := gs.CurrentBet - gs.Committed[p]
			if owed < 0 {
				owed = 0
			}
			gs.Committed[p] += owed
			gs.Pot += owed
			gs.ToAct--
		case sim.MoveRaise:
			owed := gs.CurrentBet - gs.Committed[p]
			if owed < 0 {
				owed = 0
			}
			gs.Committed[p] += owed + 1
			gs.Pot += owed + 1
			gs.CurrentBet++
			gs.RaiseCount++
			gs.ToAct = nonFolded(gs) - 1 // every other live seat owes a response
		case sim.MoveFold:
			gs.Folded[p] = true
			gs.ToAct--
		}
		gs.Active = nextNonFolded(gs, p)
	}
	gs.Turn++
}

// Upkeep runs once per loop iteration before the end check (Capture refill).
func (rr Runner) Upkeep(gs *sim.GameState) {
	if rr.Spec.Move != Capture {
		return
	}
	allEmpty := true
	for p := 0; p < gs.NumPlayers; p++ {
		if len(gs.Hands[p]) > 0 {
			allEmpty = false
			break
		}
	}
	if allEmpty && len(gs.Deck) >= gs.NumPlayers {
		for p := 0; p < gs.NumPlayers; p++ {
			drawn, rem := sim.DrawN(gs.Deck, rr.Spec.Deal)
			gs.Hands[p] = append(gs.Hands[p], drawn...)
			gs.Deck = rem
		}
	}
}

func (rr Runner) refillMarket(gs *sim.GameState) {
	if rr.Spec.Shared > 0 && len(gs.Discard) < rr.Spec.Shared && len(gs.Deck) > 0 {
		drawn, rem := sim.DrawN(gs.Deck, 1)
		gs.Discard = append(gs.Discard, drawn...)
		gs.Deck = rem
	}
}

func (rr Runner) nextActive(gs *sim.GameState) int {
	for i := 1; i <= gs.NumPlayers; i++ {
		q := (gs.Active + i) % gs.NumPlayers
		if !gs.Folded[q] {
			return q
		}
	}
	return gs.Active // all folded; CheckEnd handles it
}

// CheckEnd returns the winner seat (>=0) or -1 if the game continues. A returned
// winner of -1 from a terminal state (e.g. everyone busted) is reported as a
// drawn-but-TERMINATED game by the harness, not a hang.
func (rr Runner) CheckEnd(gs *sim.GameState) (winner int, done bool) {
	if gs.Phase == sim.PhaseEnd { // ModKnock fired
		if rr.Spec.Move == Rummy { // Gin go-out: fewest DEADWOOD wins
			return rr.score(gs), true
		}
		return rr.fewestCards(gs), true // shedding/climbing: fewest cards
	}
	s := rr.Spec
	// Runner-level liveness: a PlayMatch game can stall when the deck is empty and
	// nobody holds a legal play (everyone passes). End it by the standing progress
	// (fewest cards). This is the termination guarantee living in the RUNNER, not
	// in the harness -- so the grammar is playable-by-construction in the real
	// engine (sim.RunBatch), which has no stalemate net of its own.
	if s.Move == PlayMatch && gs.PassCount >= gs.NumPlayers {
		return rr.fewestCards(gs), true
	}
	switch s.End {
	case EmptyHand:
		for p := 0; p < gs.NumPlayers; p++ {
			if len(gs.Hands[p]) == 0 {
				return rr.score(gs), true
			}
		}
	case DeckOut:
		if s.Move == Rummy {
			// Rummy hands stay a constant size, so deck_out means the DECK is
			// exhausted -- which the one-draw-per-turn dynamic guarantees. It
			// fires only at a TURN BOUNDARY: the player who drew the last card
			// still discards first (the discard phase always has a legal move,
			// so this costs exactly one more move and cannot stall). Ending on
			// the draw scored one fixed seat on Deal+1 cards every game.
			if len(gs.Deck) == 0 && gs.Phase != sim.PhaseDiscard {
				return rr.score(gs), true
			}
			return -1, false
		}
		allEmpty := true
		for p := 0; p < gs.NumPlayers; p++ {
			if len(gs.Hands[p]) > 0 {
				allEmpty = false
			}
		}
		// End when hands are empty and the deck can't deal another full round --
		// the leftover-remainder case (deck in 1..NumPlayers-1) would otherwise
		// stall on passes forever in the real engine.
		if allEmpty && len(gs.Deck) < gs.NumPlayers {
			return rr.score(gs), true
		}
	case Bust:
		for p := 0; p < gs.NumPlayers; p++ {
			if !gs.Folded[p] {
				return -1, false
			}
		}
		return rr.score(gs), true // all stuck or busted
	case Showdown:
		// The betting round closes when only one player is live, or every live seat
		// has matched the bet (ToAct exhausted). The max-raises cap guarantees this.
		if nonFolded(gs) <= 1 || gs.ToAct <= 0 {
			return rr.score(gs), true
		}
	}
	return -1, false
}

// Tie-breaking (2026-10 bughunt). Every tie used to resolve by absolute seat
// index (lowest seat; in banking, highest), which under random play was a
// standing advantage for one fixed seat. The ladder is now, per score rule:
//
//  1. a rules-meaningful secondary criterion (rummy: fewer stray-card points;
//     banking: fewer cards taken) -- folded into the rule's better() ordering;
//  2. where the tied players hold cards the rule is about, the CARD rule: the
//     tied player holding the single highest (or, where cards in hand are bad,
//     lowest) card wins. Cards are unique and dealt at random, so this is decided
//     by the deal, never by where anyone sits;
//  3. otherwise turn order counted from tieOrigin -- the player whose action
//     ended the game -- which is a different seat from game to game.
//
// A "last player to act" anchor alone is NOT enough for the games whose final
// mover is structurally a fixed seat (rummy's last discard; banking, where equal
// card counts mean equal action counts so the last seat always acted last; and
// capture, where the last seat plays the last card) -- hence step 2 for those.
// rulebook.go states each rule; keep the two in sync.

// tieOrigin is the seat a turn-order tie is counted FROM. A tie goes to that
// player if they are among the tied, else to the tied player soonest after them.
//
//	trick:    the winner of the last trick        (Apply sets TrickLeader)
//	shedding: the knocker, else the last to pass  (the all-pass deadlock end)
//	rummy:    the knocker, else the last discarder
//	climbing: the knocker                         (Apply leaves Active on them)
//	others:   the seat left active by the final move
func (rr Runner) tieOrigin(gs *sim.GameState) int {
	n := gs.NumPlayers
	switch rr.Spec.Move {
	case Trick:
		return wrap(gs.TrickLeader, n)
	case Rummy, PlayMatch:
		if gs.Phase == sim.PhaseEnd { // knocked: Apply returned before advancing the turn
			return wrap(gs.Active, n)
		}
		dir := gs.Direction
		if dir == 0 {
			dir = 1
		}
		return wrap(gs.Active-dir, n) // the player whose move passed the turn on
	}
	return wrap(gs.Active, n)
}

// bestSeat scans the seats in turn order starting at origin and returns the one
// that beats all others; better(a, b) reports whether seat a STRICTLY beats seat
// b. Only a strictly better seat displaces the incumbent, so a tie stays with the
// tied seat nearest the origin. ok (may be nil) restricts the candidates; with no
// candidate the result is -1.
func bestSeat(n, origin int, ok func(p int) bool, better func(a, b int) bool) int {
	best := -1
	for i := 0; i < n; i++ {
		p := wrap(origin+i, n)
		if ok != nil && !ok(p) {
			continue
		}
		if best < 0 || better(p, best) {
			best = p
		}
	}
	return best
}

// cardOrder ranks a card for the card rule: by rank (ace high), then by suit
// (clubs < diamonds < hearts < spades). No two cards share an order.
func cardOrder(c sim.Card) int { return int(c.Rank)*4 + int(c.Suit) }

// cardHolder returns, among the seats where in(p) holds, the one holding the
// single highest (highest=true) or lowest card under cardOrder; -1 when none of
// those seats holds a card at all.
func cardHolder(n int, in func(p int) bool, cards func(p int) []sim.Card, highest bool) int {
	holder, bestOrd := -1, 0
	for p := 0; p < n; p++ {
		if !in(p) {
			continue
		}
		for _, c := range cards(p) {
			o := cardOrder(c)
			if holder < 0 || (highest && o > bestOrd) || (!highest && o < bestOrd) {
				holder, bestOrd = p, o
			}
		}
	}
	return holder
}

// decide names the winner: the best candidate under better; if several tie, the
// card rule over cards (nil = this rule has no card step), else turn order from
// tieOrigin. See the tie-breaking note above.
func (rr Runner) decide(gs *sim.GameState, ok func(p int) bool, better func(a, b int) bool,
	cards func(p int) []sim.Card, highest bool) int {
	n := gs.NumPlayers
	w := bestSeat(n, rr.tieOrigin(gs), ok, better)
	if w < 0 || cards == nil {
		return w
	}
	tied := func(p int) bool { return (ok == nil || ok(p)) && !better(w, p) && !better(p, w) }
	if h := cardHolder(n, tied, cards, highest); h >= 0 {
		return h
	}
	return w
}

// effScore is seat p's EFFECTIVE count under the pile-collecting score rules
// (most_captured / high_score): the raw tally with every scoring modifier
// applied. It is the single definition of "who is ahead" for those games -- the
// winner (score) and the leader track (Adapter.Progress) both read it, so the
// metrics' leader cannot disagree with the rule that decides the game.
func (rr Runner) effScore(gs *sim.GameState, p int) int {
	v := gs.Scores[p]
	// ModBid (trick contracts): the base signal is how well the bid was
	// MADE, not the raw trick count. Each trick banks NumPlayers cards.
	// The co-typed scoring modifiers below COMPOSE on top of the contract
	// (Pinochle precedent: bidding and melds coexist), never replace it.
	if rr.Spec.hasMod(ModBid) && rr.Spec.Move == Trick {
		v = 0 // no contract yet (still in the bid round): nothing banked
		if p < len(gs.Bids) && gs.Bids[p] >= 0 {
			v = contractScore(gs.Scores[p]/gs.NumPlayers, gs.Bids[p])
		}
	}
	if rr.Spec.hasMod(ModMeldBonus) {
		v += meldBonus(gs.Tableau[p]) // set/run bonuses
	}
	if rr.Spec.hasMod(ModAvoidance) {
		v -= avoidancePenalty(gs.Tableau[p]) // points-are-bad (Hearts)
	}
	return v
}

// score names the winner under the spec's score rule; ties go through decide.
func (rr Runner) score(gs *sim.GameState) int {
	s := rr.Spec
	n := gs.NumPlayers
	hand := func(p int) []sim.Card { return gs.Hands[p] }
	pile := func(p int) []sim.Card { return gs.Tableau[p] }
	switch s.Score {
	case FirstOut:
		for p := 0; p < n; p++ {
			if len(gs.Hands[p]) == 0 {
				return p
			}
		}
	case FewestCards:
		return rr.fewestCards(gs)
	case BestHand:
		// Equal strength means equal ranks, so the card rule decides on suit.
		str := make([]int64, n)
		live := func(p int) bool { return !gs.Folded[p] }
		for p := 0; p < n; p++ {
			if live(p) {
				str[p] = vying.HandStrength(gs.Hands[p])
			}
		}
		w := rr.decide(gs, live, func(a, b int) bool { return str[a] > str[b] }, hand, true)
		if w < 0 {
			w = 0 // all folded (degenerate); deterministic fallback
		}
		return w
	case FewestDeadwood:
		// Fewest stray cards; a tie on the COUNT (about half of all random games)
		// goes to the lower stray-card point total, then to the holder of the
		// lowest card (low cards are the good ones here).
		wr := -1
		if rr.Spec.hasMod(ModWild) {
			wr = wildRank
		}
		cnt, pts := make([]int, n), make([]int, n)
		for p := 0; p < n; p++ {
			cnt[p], pts[p] = deadwoodStats(gs.Hands[p], wr)
		}
		return rr.decide(gs, nil, func(a, b int) bool {
			if cnt[a] != cnt[b] {
				return cnt[a] < cnt[b]
			}
			return pts[a] < pts[b]
		}, hand, false)
	case ClosestTarget:
		// Highest total not over the target; a tied total goes to the player who
		// reached it with FEWER cards, then to the holder of the highest card
		// taken (the piles are in Tableau).
		alive := func(p int) bool { return gs.Scores[p] <= s.Target }
		w := rr.decide(gs, alive, func(a, b int) bool {
			if gs.Scores[a] != gs.Scores[b] {
				return gs.Scores[a] > gs.Scores[b]
			}
			return len(gs.Tableau[a]) < len(gs.Tableau[b])
		}, pile, true)
		if w < 0 { // everyone busted: least-over wins (the GenericRunner contract needs a winner >= 0)
			w = rr.decide(gs, nil, func(a, b int) bool {
				if gs.Scores[a] != gs.Scores[b] {
					return gs.Scores[a] < gs.Scores[b]
				}
				return len(gs.Tableau[a]) < len(gs.Tableau[b])
			}, pile, true)
		}
		return w
	case MostCaptured, HighScore:
		eff := make([]int, n) // scoring modifiers adjust the count from the won pile
		for p := 0; p < n; p++ {
			eff[p] = rr.effScore(gs, p)
		}
		higher := func(a, b int) bool { return eff[a] > eff[b] }
		if rr.Spec.hasMod(ModTeams) { // 2v2: the best TEAM wins, reported as its top seat
			origin := rr.tieOrigin(gs)
			var team [2]int
			for p := 0; p < n; p++ {
				team[teamOf(p)] += eff[p]
			}
			winTeam := teamOf(origin) // a tied result goes to the partnership that took the last trick
			if team[0] != team[1] {
				winTeam = 0
				if team[1] > team[0] {
					winTeam = 1
				}
			}
			return bestSeat(n, origin, func(p int) bool { return teamOf(p) == winTeam }, higher)
		}
		if s.Move == Trick {
			return rr.decide(gs, nil, higher, nil, false) // a tie goes to the winner of the last trick
		}
		return rr.decide(gs, nil, higher, pile, true) // capture: the holder of the highest captured card
	}
	return -1
}

// fewestCards returns the seat holding the fewest cards -- the ModKnock win
// condition and the shedding deadlock end. A tie is counted from tieOrigin (the
// knocker / the last player to pass), not from seat 0.
func (rr Runner) fewestCards(gs *sim.GameState) int {
	return rr.decide(gs, nil, func(a, b int) bool {
		return len(gs.Hands[a]) < len(gs.Hands[b])
	}, nil, false)
}

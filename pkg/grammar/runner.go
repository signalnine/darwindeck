package grammar

import (
	"math/rand/v2"

	"github.com/darwindeck/darwindeck/pkg/sim"
)

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
// (or a wild) when the player holds the suit -- the move-RESTRICT modifier. If the
// player is void in the suit, or the filter would empty the set, it leaves the
// moves untouched so the never-empty invariant holds.
func followSuitFilter(moves []sim.Move, hand []sim.Card, top sim.Card, wild bool) []sim.Move {
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
			if c.Suit == top.Suit || isWild(c, wild) {
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
	}
	if s.Move == Rummy {
		gs.Phase = sim.PhaseDraw // a rummy turn is two moves: draw, then discard
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
		var moves []sim.Move
		for _, c := range hand {
			if matches(c, top, ok, rr.Spec.Match) || isWild(c, wild) {
				moves = append(moves, mv(sim.MovePlay, p, c))
			}
		}
		if rr.Spec.hasMod(ModRunPlay) { // EXPAND: same-rank set / same-suit run combos
			moves = append(moves, comboPlays(hand, top, ok, rr.Spec.Match, p)...)
		}
		if rr.Spec.hasMod(ModFollowSuit) && ok { // RESTRICT: must follow the discard suit if held
			moves = followSuitFilter(moves, hand, top, wild)
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
		for _, c := range hand {
			if leading || c.Rank > gs.TrickCards[0].Rank {
				moves = append(moves, mv(sim.MovePlay, p, c))
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
		moves = append(moves, mv(sim.MovePass, p)) // STICK: always legal, the fallback
		return moves

	case Capture:
		var moves []sim.Move
		for _, c := range hand {
			moves = append(moves, mv(sim.MovePlay, p, c)) // capture-or-trail, decided in Apply
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
			setTop(gs, m.Cards[len(m.Cards)-1])
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
		step := 1
		if m.Type == sim.MovePlay {
			if rr.Spec.hasMod(ModSkip) && hasRank(m.Cards, skipRank) {
				step = 2 // ModSkip: playing the skip rank skips the next player
			}
			if rr.Spec.hasMod(ModForceDraw) && hasRank(m.Cards, forceDrawRank) {
				victim := (p + 1) % gs.NumPlayers
				drawn, rem := sim.DrawN(gs.Deck, forceDrawN) // bounded by the (finite) deck
				gs.Hands[victim] = append(gs.Hands[victim], drawn...)
				gs.Deck = rem
				step = 2 // the victim drew and loses their turn
			}
		}
		gs.Active = (p + step) % gs.NumPlayers

	case BeatOrPass:
		switch m.Type {
		case sim.MovePlay:
			c := m.Cards[0]
			gs.Hands[p] = removeCard(gs.Hands[p], c)
			gs.TrickCards = []sim.Card{c} // new combination to beat
			gs.TrickLeader = p
			gs.PassCount = 0
		case sim.MovePass:
			gs.PassCount++
			if gs.PassCount >= gs.NumPlayers-1 { // all others passed: table clears
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
			rr.refillMarket(gs)
			if gs.Scores[p] > s.Target {
				gs.Folded[p] = true // bust
			}
		case sim.MoveDraw: // take a blind card
			drawn, rem := sim.DrawN(gs.Deck, 1)
			gs.Deck = rem
			if len(drawn) > 0 {
				gs.Scores[p] += cardValue(drawn[0].Rank)
				if gs.Scores[p] > s.Target {
					gs.Folded[p] = true
				}
			}
		case sim.MovePass: // stick
			gs.Folded[p] = true
		}
		gs.Active = rr.nextActive(gs)

	case Capture:
		if m.Type == sim.MovePlay {
			c := m.Cards[0]
			gs.Hands[p] = removeCard(gs.Hands[p], c)
			var capt []sim.Card
			var rest []sim.Card
			for _, t := range gs.Discard {
				if t.Rank == c.Rank {
					capt = append(capt, t)
				} else {
					rest = append(rest, t)
				}
			}
			if len(capt) > 0 { // capture
				gs.Discard = rest
				gs.Scores[p] += len(capt) + 1
				gs.Tableau[p] = append(gs.Tableau[p], append(capt, c)...)
			} else { // trail
				gs.Discard = append(gs.Discard, c)
			}
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
		return fewestCards(gs), true // shedding/climbing: fewest cards
	}
	s := rr.Spec
	// Runner-level liveness: a PlayMatch game can stall when the deck is empty and
	// nobody holds a legal play (everyone passes). End it by the standing progress
	// (fewest cards). This is the termination guarantee living in the RUNNER, not
	// in the harness -- so the grammar is playable-by-construction in the real
	// engine (sim.RunBatch), which has no stalemate net of its own.
	if s.Move == PlayMatch && gs.PassCount >= gs.NumPlayers {
		return fewestCards(gs), true
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
			// exhausted -- which the one-draw-per-turn dynamic guarantees.
			if len(gs.Deck) == 0 {
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
	}
	return -1, false
}

func (rr Runner) score(gs *sim.GameState) int {
	s := rr.Spec
	best, bestVal := -1, 0
	switch s.Score {
	case FirstOut:
		for p := 0; p < gs.NumPlayers; p++ {
			if len(gs.Hands[p]) == 0 {
				return p
			}
		}
	case FewestCards:
		best, bestVal = 0, len(gs.Hands[0])
		for p := 1; p < gs.NumPlayers; p++ {
			if len(gs.Hands[p]) < bestVal {
				best, bestVal = p, len(gs.Hands[p])
			}
		}
		return best
	case FewestDeadwood:
		wr := -1
		if rr.Spec.hasMod(ModWild) {
			wr = wildRank
		}
		best, bestVal = 0, deadwood(gs.Hands[0], wr)
		for p := 1; p < gs.NumPlayers; p++ {
			if d := deadwood(gs.Hands[p], wr); d < bestVal {
				best, bestVal = p, d
			}
		}
		return best
	case ClosestTarget:
		best, bestVal = -1, -1
		for p := 0; p < gs.NumPlayers; p++ {
			if gs.Scores[p] <= s.Target && gs.Scores[p] >= bestVal {
				best, bestVal = p, gs.Scores[p]
			}
		}
		if best < 0 { // everyone busted: least-over wins (the GenericRunner contract needs a winner >= 0)
			best, bestVal = 0, gs.Scores[0]
			for p := 1; p < gs.NumPlayers; p++ {
				if gs.Scores[p] < bestVal {
					best, bestVal = p, gs.Scores[p]
				}
			}
		}
		return best
	case MostCaptured, HighScore:
		eff := func(p int) int { // scoring modifiers adjust the count from the won pile
			v := gs.Scores[p]
			if rr.Spec.hasMod(ModMeldBonus) {
				v += meldBonus(gs.Tableau[p]) // set/run bonuses
			}
			if rr.Spec.hasMod(ModAvoidance) {
				v -= avoidancePenalty(gs.Tableau[p]) // points-are-bad (Hearts)
			}
			return v
		}
		best, bestVal = 0, eff(0)
		for p := 1; p < gs.NumPlayers; p++ {
			if e := eff(p); e > bestVal {
				best, bestVal = p, e
			}
		}
		return best
	}
	return best
}

// fewestCards returns the seat holding the fewest cards (ties to lowest seat) --
// the ModKnock win condition.
func fewestCards(gs *sim.GameState) int {
	best, bestN := 0, len(gs.Hands[0])
	for p := 1; p < gs.NumPlayers; p++ {
		if len(gs.Hands[p]) < bestN {
			best, bestN = p, len(gs.Hands[p])
		}
	}
	return best
}

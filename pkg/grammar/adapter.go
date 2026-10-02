package grammar

import (
	"math/rand/v2"
	"strconv"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/sim"
	"github.com/darwindeck/darwindeck/pkg/skeleton/vying"
)

// Adapter makes a GameSpec satisfy sim.GenericRunner, so a grammar composition
// runs through the SAME simulation engine (sim.RunBatch) the hand-coded skeletons
// use -- step 4 of the rearchitecture (results/2026-06-23-grammar-prototype). The
// *genome.Genome the interface threads is the SIMULATION layer's config object;
// the grammar's config is the Spec, so the adapter ignores g in the hot methods
// and emits the event taxonomy the fitness metrics consume.
//
// NOTE the seam this exposes: the SIMULATION layer is genuinely generic (this
// adapter is all it takes), but the FITNESS layer is still skeleton-coupled --
// optionDeltaModeFor switches on g.Skeleton for the Interaction metric, and the
// greedy scorers are per-skeleton. SpecGenome below carries a best-fit skeleton so
// the existing modes apply; making those metrics fully generic is the next sub-step.
type Adapter struct{ Spec GameSpec }

var _ sim.GenericRunner = Adapter{}

func (a Adapter) Setup(_ *genome.Genome, rng *rand.Rand) *sim.GameState {
	return Runner{a.Spec}.Setup(rng)
}

func (a Adapter) Upkeep(s *sim.GameState, _ *genome.Genome) { Runner{a.Spec}.Upkeep(s) }

func (a Adapter) GenerateMoves(s *sim.GameState, _ *genome.Genome) []sim.Move {
	return Runner{a.Spec}.LegalMoves(s)
}

// ApplyMove applies the move and emits the event taxonomy the metrics read:
// EventCardPlayed / EventCardDrawn / EventSpecialTriggered(knock), plus a single
// EventRoundEnd when the move ends the game (so end-of-game scoring is legible).
//
// The stream is also the judge dossier's game trace, so EVERY move leaves a line
// and the line says what was chosen: a pass/stick is an event (a silent pass made
// the trace read as players acting out of turn), a bid carries its amount, a
// nominated eight carries the suit it named, and each betting action is named.
// These details are metric-inert by construction: the fitness layer reads events
// only through sim.IsAttackEvent, whose whitelist is trick wins plus the
// draw_two/draw_four/skip/reverse specials -- none of the details added here.
func (a Adapter) ApplyMove(s *sim.GameState, m sim.Move, _ *genome.Genome) []sim.Event {
	p := s.Active
	var ev []sim.Event
	switch m.Type {
	case sim.MovePlay:
		e := sim.Event{Type: sim.EventCardPlayed, PlayerID: p, Cards: m.Cards}
		switch {
		case a.Spec.Move == Capture:
			e.Detail = "trail" // left face-up on the table (v2 casino's detail)
		case a.Spec.Move == Accumulate:
			e.Detail = "take_face_up"
		case a.Spec.Move == PlayMatch && a.Spec.hasMod(ModNominate) && int(m.Cards[len(m.Cards)-1].Rank) == wildRank:
			e.Detail = "names_suit=" + sim.Suit(m.Amount).String()
		}
		ev = append(ev, e)
	case sim.MoveDraw:
		ev = append(ev, sim.Event{Type: sim.EventCardDrawn, PlayerID: p})
	case sim.MoveDiscard: // rummy: the chosen discard is the meaningful action
		ev = append(ev, sim.Event{Type: sim.EventCardPlayed, PlayerID: p, Cards: m.Cards, Detail: "discard"})
	case sim.MoveCapture: // casino: a chosen capture from the shared table
		ev = append(ev, sim.Event{Type: sim.EventCardPlayed, PlayerID: p, Cards: m.Cards, Detail: "capture"})
	case sim.MoveCheck: // vying betting: name the action, not a generic "bet"
		ev = append(ev, sim.Event{Type: sim.EventSpecialTriggered, PlayerID: p, Detail: "check"})
	case sim.MoveCall:
		ev = append(ev, sim.Event{Type: sim.EventSpecialTriggered, PlayerID: p, Detail: "call"})
	case sim.MoveRaise:
		ev = append(ev, sim.Event{Type: sim.EventSpecialTriggered, PlayerID: p, Detail: "raise"})
	case sim.MoveFold:
		ev = append(ev, sim.Event{Type: sim.EventSpecialTriggered, PlayerID: p, Detail: "fold"})
	case sim.MoveKnock:
		ev = append(ev, sim.Event{Type: sim.EventSpecialTriggered, PlayerID: p, Detail: "knock"})
	case sim.MoveBid:
		ev = append(ev, sim.Event{Type: sim.EventSpecialTriggered, PlayerID: p, Detail: "bid=" + strconv.Itoa(m.Amount)})
	case sim.MovePass:
		detail := "pass"
		if a.Spec.Move == Accumulate {
			detail = "stick" // banking: lock in the total
		}
		ev = append(ev, sim.Event{Type: sim.EventSpecialTriggered, PlayerID: p, Detail: detail})
	}
	// Turn-order / draw attacks are the interaction signal (IsAttackEvent counts
	// "draw_two" always, and "skip"/"reverse" for >2 players).
	if m.Type == sim.MovePlay {
		if a.Spec.hasMod(ModSkip) && hasRank(m.Cards, skipRank) {
			ev = append(ev, sim.Event{Type: sim.EventSpecialTriggered, PlayerID: p, Detail: "skip"})
		}
		if a.Spec.hasMod(ModForceDraw) && hasRank(m.Cards, forceDrawRank) {
			ev = append(ev, sim.Event{Type: sim.EventSpecialTriggered, PlayerID: p, Detail: "draw_two"})
		}
		if a.Spec.hasMod(ModReverse) && hasRank(m.Cards, reverseRank) {
			ev = append(ev, sim.Event{Type: sim.EventSpecialTriggered, PlayerID: p, Detail: "reverse"})
		}
	}
	// A trick that completes on this play is the interaction signal the metric
	// counts (EventTrickWon); detect it across the Apply, which clears TrickCards
	// and sets Active to the winner.
	trickCompleting := a.Spec.Move == Trick && m.Type == sim.MovePlay && len(s.TrickCards) == s.NumPlayers-1
	r := Runner{a.Spec}
	r.Apply(s, m)
	if trickCompleting && len(s.TrickCards) == 0 {
		ev = append(ev, sim.Event{Type: sim.EventTrickWon, PlayerID: s.Active}) // the winner leads next
	}
	if _, done := r.CheckEnd(s); done {
		ev = append(ev, sim.Event{Type: sim.EventRoundEnd, PlayerID: p})
	}
	return ev
}

// CheckEnd returns the winning seat (>= 0) when the game is over, else -1 to
// continue -- the sim.GenericRunner convention. The grammar's score always names a
// winner (no -1 draws), so a finished game always reports a real seat.
func (a Adapter) CheckEnd(s *sim.GameState, _ *genome.Genome) int {
	if w, done := (Runner{a.Spec}).CheckEnd(s); done {
		return w
	}
	return -1
}

// Progress is the per-player ranking snapshot the batch loop turns into a leader
// track. Monotonicity is not required; this is a cheap closeness-to-winning proxy
// per move-gen (mirrors the GenericRunner doc's per-skeleton definitions).
func (a Adapter) Progress(s *sim.GameState, _ *genome.Genome) []float64 {
	out := make([]float64, s.NumPlayers)
	switch a.Spec.Move {
	case PlayMatch, BeatOrPass: // empty-hand race: fewer cards = closer
		d := a.Spec.Deal
		if d < 1 {
			d = 1
		}
		for p := range out {
			v := 1 - float64(len(s.Hands[p]))/float64(d)
			if v < 0 {
				v = 0
			}
			out[p] = v
		}
	case Accumulate: // banking: running total toward the target
		t := a.Spec.Target
		if t < 1 {
			t = 1
		}
		for p := range out {
			v := float64(s.Scores[p]) / float64(t)
			if v > 1 {
				// Over the target. Under closest_target that is a BUST: the
				// player is out, so 0 -- clamping to 1 made every bust the leader.
				v = 1
				if a.Spec.Score == ClosestTarget {
					v = 0
				}
			}
			out[p] = v
		}
	case Capture, Trick:
		// Share of the EFFECTIVE count -- the same Runner.effScore the winner is
		// decided by, so avoidance penalties, bid contracts and combination
		// bonuses move the leader track exactly as they move the result. (The
		// raw card share named a leader who then lost about half the games
		// under trick+avoidance or trick+bid.)
		r := Runner{a.Spec}
		rank := make([]int, s.NumPlayers)
		for p := range rank {
			rank[p] = r.effScore(s, p)
		}
		if a.Spec.hasMod(ModTeams) {
			// Partnerships: a seat ranks by its TEAM's total, and within the team
			// the top seat (the one a team win is reported as) edges its partner.
			// Top seats of level teams tie exactly, as do level partners.
			var team, top [2]int
			top[0], top[1] = -1<<31, -1<<31
			for p, v := range rank {
				team[teamOf(p)] += v
				if v > top[teamOf(p)] {
					top[teamOf(p)] = v
				}
			}
			for p, v := range rank {
				rank[p] = 2 * team[teamOf(p)]
				if v == top[teamOf(p)] {
					rank[p]++
				}
			}
		}
		// Effective counts can be negative (penalties, a failed contract): shift
		// so the lowest sits at 0, then take shares. With no negative value the
		// shift is 0 and this is the plain share of cards won.
		low, total := 0, 0
		for _, v := range rank {
			if v < low {
				low = v
			}
		}
		for _, v := range rank {
			total += v - low
		}
		for p := range out {
			if total > 0 {
				out[p] = float64(rank[p]-low) / float64(total)
			}
		}
	case Rummy:
		// Closeness to winning = fewer unmelded cards, then fewer points in them
		// -- the two keys Runner.score ranks by, folded into one number (the
		// count dominates; the points only order equal counts). A hand holds at
		// most Deal+1 cards (mid-turn), each worth at most 10, so the result is
		// always inside (0,1].
		d := a.Spec.Deal
		if d < 1 {
			d = 1
		}
		wr := -1
		if a.Spec.hasMod(ModWild) {
			wr = wildRank
		}
		w := 10*(d+1) + 1 // exceeds any hand's stray-point total
		for p := range out {
			cnt, pts := deadwoodStats(s.Hands[p], wr)
			v := 1 - float64(cnt*w+pts)/float64((d+2)*w)
			if v < 0 {
				v = 0
			}
			out[p] = v
		}
	case Vying: // share of poker hand strength among the live (non-folded) seats
		str := make([]int64, s.NumPlayers)
		var total int64
		for p := 0; p < s.NumPlayers; p++ {
			if p < len(s.Folded) && s.Folded[p] {
				continue
			}
			str[p] = vying.HandStrength(s.Hands[p])
			total += str[p]
		}
		for p := range out {
			if total > 0 {
				out[p] = float64(str[p]) / float64(total)
			}
		}
	}
	return out
}

// PlayableCount implements sim.PlayableShareProber for PlayMatch specs, so the
// fitness layer's playable-share vetoes (dead_match_rule successor,
// pkg/fitness/degeneracy.go playable_share) are not blind to grammar shedding
// games. It mirrors LegalMoves' per-card predicate -- match the top under the
// spec's rule, or an always-playable nominate-8, narrowed by ModFollowSuit when
// that obligation is live -- counting every qualifying card once (the prober
// contract: no move-level dedup). It equals the number of distinct single-card
// plays LegalMoves offers (TestPlayableCountMatchesLegalMoves). Pure query. Non-shedding
// move-gens report 0; the veto only reads shedding-skeleton records, and the
// other Shedding-mapped move-gen (Accumulate) deals no hand, so its records
// fall under the HandSize >= 2 floor.
func (a Adapter) PlayableCount(s *sim.GameState, _ *genome.Genome) int {
	if a.Spec.Move != PlayMatch {
		return 0
	}
	hand := s.Hands[s.Active]
	top, ok := topOf(s)
	nominate := a.Spec.hasMod(ModNominate)
	wild := a.Spec.hasMod(ModWild)
	// count: cards playable under the match rule. follow: the subset that also
	// satisfies ModFollowSuit (the discard suit, a wild, or a nominate-8) --
	// followSuitFilter's keep predicate. holds: the obligation is live.
	count, follow, holds := 0, 0, false
	for _, c := range hand {
		eight := nominate && int(c.Rank) == wildRank
		if ok && c.Suit == top.Suit {
			holds = true
		}
		if eight || matches(c, top, ok, a.Spec.Match) || isWild(c, wild) {
			count++
			if c.Suit == top.Suit || eight || isWild(c, wild) {
				follow++
			}
		}
	}
	if !a.Spec.hasMod(ModFollowSuit) || !holds {
		return count
	}
	// ModFollowSuit RESTRICTS the plays to the discard suit when it is held, so
	// only those cards are playable. (Ignoring it reported 3 playable cards for
	// 2C/7D/7H on a 7C when the rule leaves one, inflating the playable share.)
	if follow > 0 {
		return follow
	}
	// The suit is held but no single card satisfies the obligation (only under a
	// rank-only match rule). Whether the filter then falls through depends on the
	// multi-card plays, so count what LegalMoves actually offers.
	seen := make(map[sim.Card]struct{})
	for _, m := range (Runner{a.Spec}).LegalMoves(s) {
		if m.Type == sim.MovePlay && len(m.Cards) == 1 {
			seen[m.Cards[0]] = struct{}{}
		}
	}
	return len(seen)
}

var _ sim.PlayableShareProber = Adapter{}

// SpecGenome builds the minimal *genome.Genome the simulation/fitness layer reads
// alongside the adapter: a best-fit skeleton (so optionDeltaModeFor picks a
// sensible Interaction mode) plus the player/hand counts. Accumulate (banking) has
// no v2 skeleton, so it borrows Shedding as a neutral host for the delta probes.
func SpecGenome(s GameSpec) *genome.Genome {
	hs := s.Deal
	if hs < 8 { // MaxTurns() scales by HandSize; a 0-deal (banking) must not zero the cap
		hs = 8
	}
	g := &genome.Genome{
		Skeleton: specSkeleton(s.Move),
		Players:  s.Players,
		HandSize: hs,
	}
	if s.Move == Trick {
		// With TrickTaking nil, MaxTurns() = HandSize*Players -- EXACTLY the play
		// count of a full trick game, zero headroom: any future Turn-consuming
		// change flips every trick spec to max_turns truncation. RoundsPerGame=2
		// doubles the cap through the genome's own derivation (the spec is still
		// single-round; the cap is a backstop, and the zero-value TrickScoring
		// leaves the greedy scorer's avoidance check unchanged).
		g.TrickTaking = &genome.TrickTakingParams{RoundsPerGame: 2}
	}
	return g
}

func specSkeleton(m MoveGen) genome.SkeletonType {
	switch m {
	case BeatOrPass:
		return genome.Climbing
	case Capture:
		return genome.Casino
	case Trick:
		return genome.TrickTaking
	case Rummy:
		return genome.Rummy
	case Vying:
		return genome.Vying
	default: // PlayMatch, Accumulate
		return genome.Shedding
	}
}

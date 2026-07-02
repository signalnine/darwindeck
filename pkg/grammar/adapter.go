package grammar

import (
	"math/rand/v2"

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
func (a Adapter) ApplyMove(s *sim.GameState, m sim.Move, _ *genome.Genome) []sim.Event {
	p := s.Active
	var ev []sim.Event
	switch m.Type {
	case sim.MovePlay:
		ev = append(ev, sim.Event{Type: sim.EventCardPlayed, PlayerID: p, Cards: m.Cards})
	case sim.MoveDraw:
		ev = append(ev, sim.Event{Type: sim.EventCardDrawn, PlayerID: p})
	case sim.MoveDiscard: // rummy: the chosen discard is the meaningful action
		ev = append(ev, sim.Event{Type: sim.EventCardPlayed, PlayerID: p, Cards: m.Cards})
	case sim.MoveCapture: // casino: a chosen capture from the shared table
		ev = append(ev, sim.Event{Type: sim.EventCardPlayed, PlayerID: p, Cards: m.Cards})
	case sim.MoveCheck, sim.MoveCall, sim.MoveRaise, sim.MoveFold: // vying betting
		ev = append(ev, sim.Event{Type: sim.EventSpecialTriggered, PlayerID: p, Detail: "bet"})
	case sim.MoveKnock:
		ev = append(ev, sim.Event{Type: sim.EventSpecialTriggered, PlayerID: p, Detail: "knock"})
	case sim.MoveBid:
		ev = append(ev, sim.Event{Type: sim.EventSpecialTriggered, PlayerID: p, Detail: "bid"})
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
				v = 1
			}
			out[p] = v
		}
	case Capture, Trick: // share of cards captured / tricks won so far
		total := 0
		for p := 0; p < s.NumPlayers; p++ {
			total += s.Scores[p]
		}
		for p := range out {
			if total > 0 {
				out[p] = float64(s.Scores[p]) / float64(total)
			}
		}
	case Rummy: // closeness to winning = fewer unmelded cards
		d := a.Spec.Deal
		if d < 1 {
			d = 1
		}
		wr := -1
		if a.Spec.hasMod(ModWild) {
			wr = wildRank
		}
		for p := range out {
			v := 1 - float64(deadwood(s.Hands[p], wr))/float64(d)
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
// spec's rule, or an always-playable nominate-8 -- counting every qualifying
// card once (the prober contract: no move-level dedup). Pure query. Non-shedding
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
	count := 0
	for _, c := range hand {
		if (nominate && int(c.Rank) == wildRank) || matches(c, top, ok, a.Spec.Match) {
			count++
		}
	}
	return count
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

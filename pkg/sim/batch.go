package sim

import (
	"math/rand/v2"

	"github.com/darwindeck/darwindeck/pkg/genome"
)

// BatchResult holds aggregated results from running multiple games.
type BatchResult struct {
	GamesPlayed int
	Completions int   // Games that ended with a winner
	Errors      int   // Games that errored
	Timeouts    int   // Games that hit max turns
	WinCounts   []int // Wins per player
	TotalTurns  int   // Sum of all turns
	MinTurns    int
	MaxTurns    int
	AvgTurns    float64
	TurnsList   []int     // All turn counts for distribution analysis
	AllEvents   [][]Event // Events from each game (for fitness analysis)
	// AllTurns holds each game's per-move TurnRecords, parallel to AllEvents
	// (audit Task 7).
	AllTurns [][]TurnRecord
	// AllLeaders holds each game's per-move leader track (argmax of the
	// runner's Progress after each applied move, -1 = tie), parallel to
	// AllEvents (audit Task 8).
	AllLeaders [][]int8
	// AllWinners holds each game's real winner (GameResult.Winner: -1 for
	// every non-completion -- max_turns/stuck/no_moves exits), parallel to
	// AllEvents. Arc attribution must read these, never the final leader
	// sample: a timed-out game has a leader but no winner, and a rummy
	// scoring borrow can hand the CheckEnd win (state.Scores incl. hook
	// contributions) to a player who never led on live deadwood (audit Wave D
	// fix 1).
	AllWinners []int
}

// GenericRunner is the interface all skeleton runners implement.
// The genome is passed to each method so the runner can read parameters.
type GenericRunner interface {
	Setup(g *genome.Genome, rng *rand.Rand) *GameState
	// Upkeep performs start-of-turn state maintenance (deck recycling,
	// round transitions/redeals). It is the ONLY method besides ApplyMove
	// allowed to mutate state. Game loops must call it exactly once at the
	// top of each iteration, before CheckEnd.
	Upkeep(state *GameState, g *genome.Genome)
	GenerateMoves(state *GameState, g *genome.Genome) []Move // must be pure
	ApplyMove(state *GameState, move Move, g *genome.Genome) []Event
	CheckEnd(state *GameState, g *genome.Genome) int // must be pure
	// Progress returns each player's progress toward winning in [0,1].
	// Monotonicity is NOT required; it is a snapshot ranking signal the
	// batch loop turns into a per-move leader track (audit Task 8).
	// Per-skeleton definitions:
	//   shedding:     1 - hand/initialHandSize (floored at 0)
	//   tricktaking:  playerScore / max(1, totalAwardedSoFar); inverted
	//                 (1 - share) under ScoreAvoidance
	//   rummy:        clamp(1 - deadwood/(HandSize*10), 0, 1), computed
	//                 directly from hands -- never from Scores
	// Must be pure and allocation-light: it runs after every applied move.
	Progress(state *GameState, g *genome.Genome) []float64
}

// HookFunc is called after each move with the resulting events.
type HookFunc func(state *GameState, g *genome.Genome, event Event)

// RunBatch plays n games with the given genome, runner, and AI.
// Returns aggregated statistics.
func RunBatch(g *genome.Genome, runner GenericRunner, ai AIPlayer, n int, baseSeed uint64, hooks ...HookFunc) BatchResult {
	result := BatchResult{
		GamesPlayed: n,
		WinCounts:   make([]int, g.Players),
		TurnsList:   make([]int, 0, n),
		AllEvents:   make([][]Event, 0, n),
		AllTurns:    make([][]TurnRecord, 0, n),
		AllLeaders:  make([][]int8, 0, n),
		AllWinners:  make([]int, 0, n),
	}

	maxTurns := g.MaxTurns()

	for i := 0; i < n; i++ {
		rng := rand.New(rand.NewPCG(baseSeed+uint64(i), 0))
		gr := runSingleGame(g, runner, ai, rng, maxTurns, hooks...)

		result.TurnsList = append(result.TurnsList, gr.Turns)
		result.TotalTurns += gr.Turns

		if i == 0 || gr.Turns < result.MinTurns {
			result.MinTurns = gr.Turns
		}
		if gr.Turns > result.MaxTurns {
			result.MaxTurns = gr.Turns
		}

		if gr.Error != "" {
			if gr.Error == "max_turns" {
				result.Timeouts++
			} else {
				result.Errors++
			}
		} else if gr.Winner >= 0 && gr.Winner < g.Players {
			result.Completions++
			result.WinCounts[gr.Winner]++
		}

		result.AllEvents = append(result.AllEvents, gr.Events)
		result.AllTurns = append(result.AllTurns, gr.TurnRecords)
		result.AllLeaders = append(result.AllLeaders, gr.Leaders)
		result.AllWinners = append(result.AllWinners, gr.Winner)
	}

	if n > 0 {
		result.AvgTurns = float64(result.TotalTurns) / float64(n)
	}

	return result
}

func runSingleGame(g *genome.Genome, runner GenericRunner, ai AIPlayer, rng *rand.Rand, maxTurns int, hooks ...HookFunc) GameResult {
	state := runner.Setup(g, rng)

	// Cap inner iterations independently of maxTurns: some skeletons (rummy)
	// run multiple ApplyMove calls per turn (draw, meld, discard) and a buggy
	// runner/genome combo can stall in a phase without ever bumping Turn.
	// Without this guard the game loop runs forever and EvaluatePopulation
	// hangs across all workers (see dd-505).
	iterCap := (maxTurns + 1) * 100
	if iterCap < 10000 {
		iterCap = 10000
	}
	iter := 0

	// Per-turn decision recording (audit Task 7). The baseline slice is
	// reused across the whole game to keep the hot loop allocation-free.
	mode := optionDeltaModeFor(g)
	var baseline []int
	if mode == deltaModeShedding {
		baseline = make([]int, state.NumPlayers)
	}
	records := make([]TurnRecord, 0, 64)
	leaders := make([]int8, 0, 64)
	// Reused by the choice-impact probes so the hypothetical top-card swap
	// never heap-allocates in the hot loop.
	var scratchCard Card

	for {
		runner.Upkeep(state, g)

		winner := runner.CheckEnd(state, g)
		if winner >= 0 {
			return GameResult{
				Winner:      winner,
				Turns:       state.Turn,
				Events:      state.Events,
				TurnRecords: records,
				Leaders:     leaders,
			}
		}

		if state.Turn >= maxTurns {
			return GameResult{
				Winner:      -1,
				Turns:       state.Turn,
				Events:      state.Events,
				Error:       "max_turns",
				TurnRecords: records,
				Leaders:     leaders,
			}
		}

		if iter >= iterCap {
			return GameResult{
				Winner:      -1,
				Turns:       state.Turn,
				Events:      state.Events,
				Error:       "stuck",
				TurnRecords: records,
				Leaders:     leaders,
			}
		}
		iter++

		moves := runner.GenerateMoves(state, g)
		if len(moves) == 0 {
			return GameResult{
				Winner:      -1,
				Turns:       state.Turn,
				Events:      state.Events,
				Error:       "no_moves",
				TurnRecords: records,
				Leaders:     leaders,
			}
		}

		move := ai.SelectMove(moves, state, rng)
		mover := state.Active

		// Choice impact (Task 28 round 2): decide BEFORE the move applies
		// whether this decision point was meaningful -- the probes below
		// hypothesize against the pre-move state.
		meaningful := turnIsMeaningful(runner, state, g, moves, mode, &scratchCard)

		// Pre-move option baselines. GenerateMoves is pure (audit Task 3),
		// so probing other players via an Active swap cannot disturb the game.
		var rummyNext, rummyBaseline int
		wasLead := false
		switch mode {
		case deltaModeTrickTaking:
			// A lead is a play chosen while the trick was empty. The
			// follower's pre-move reference is their unconstrained hand
			// size, so no pre-move probe is needed.
			wasLead = move.Type == MovePlay && len(state.TrickCards) == 0
		case deltaModeShedding:
			// Specials (skip/reverse/draw) make the next actor unpredictable
			// before ApplyMove, so capture every player's baseline. The
			// mover's baseline is the move list already in hand.
			for p := 0; p < state.NumPlayers; p++ {
				if p == mover {
					baseline[p] = len(moves)
					continue
				}
				baseline[p] = probeOptionCount(runner, state, g, p)
			}
		case deltaModeRummy:
			// Only MoveDiscard hands the turn to a next player (draw/meld/
			// pass stay with the mover, and self-perturbation is not
			// coupling), so only discards can carry a nonzero delta. The
			// discard leaves the next player's hand untouched, so the meld
			// and discard components of their option union cancel exactly:
			// the union delta reduces to the draw-phase difference, which is
			// all we probe (a full union probe per move runs meld generation
			// and was a 4x batch slowdown).
			rummyNext = -1
			if move.Type == MoveDiscard {
				rummyNext = peekNextPlayer(state)
				rummyBaseline = probePhaseOptionCount(runner, state, g, rummyNext, PhaseDraw)
			}
		}

		events := runner.ApplyMove(state, move, g)
		state.Events = append(state.Events, events...)

		// Post-move option count for the actual next actor. Skipped when the
		// move ended the game (CheckEnd is pure): there is no next turn to
		// perturb, and probing terminal states is meaningless.
		//
		// ROUND BOUNDARY: when a move ends a ROUND but not the game
		// (multi-round shedding/trick-taking), this probe still runs -- the
		// redeal happens in the NEXT iteration's Upkeep -- so the one record
		// per round-ending move measures the next player's post-move options
		// against a pre-redeal baseline. That delta compares a hand about to
		// be thrown away, but the noise is bounded: at most one such record
		// per round (RoundsPerGame <= 13 records per game), against hundreds
		// of in-round records, and it can only blur OptionDelta toward 0 or a
		// spurious nonzero on a turn that was genuinely a coupling boundary
		// anyway. Not worth special-casing; revisit if rounds ever shorten to
		// a handful of moves.
		delta := 0
		if mode != deltaModeNone && runner.CheckEnd(state, g) < 0 {
			next := state.Active
			switch mode {
			case deltaModeShedding:
				// Self-perturbation is not coupling (the rummy mode's rule,
				// applied here too -- Task 28 round 2, archetype A1): when a
				// skip/reverse hands the turn straight back to the mover, the
				// probe would measure the mover's own hand shrinking, which
				// pinned the catch-all-skip champion's interaction at 1.00.
				// The opponent's coupling delta is still measured on the move
				// that finally passes them the turn (their baseline refreshes
				// every iteration).
				if next != mover {
					after := probeOptionCount(runner, state, g, next)
					if after >= 0 && baseline[next] >= 0 {
						delta = after - baseline[next]
					}
				}
			case deltaModeRummy:
				if next == rummyNext && rummyBaseline >= 0 {
					after := probePhaseOptionCount(runner, state, g, next, PhaseDraw)
					if after >= 0 {
						delta = after - rummyBaseline
					}
				}
			case deltaModeTrickTaking:
				// Lead-constraint delta. The len(TrickCards) == 1 guard
				// confirms the lead is still on the table awaiting the
				// follower (always true for >= 2 players, but a runner bug
				// must degrade to 0, never misattribute).
				if wasLead && len(state.TrickCards) == 1 {
					after := probeOptionCount(runner, state, g, next)
					if after >= 0 {
						delta = after - len(state.Hands[next])
					}
				}
			}
		}

		// Attack flag: true iff THIS move emitted at least one attack event.
		// Computed per move, not from the batch event stream, so a stacked
		// special emitting several attack events is one interactive turn
		// (audit Wave D fix 3).
		attack := false
		for _, event := range events {
			if IsAttackEvent(event, state.NumPlayers) {
				attack = true
				break
			}
		}

		records = append(records, TurnRecord{
			Player:      mover,
			LegalMoves:  capLegalMoves(len(moves)),
			OptionDelta: clampOptionDelta(delta),
			Attack:      attack,
			Meaningful:  meaningful,
		})

		// Leader after this move: argmax of Progress, -1 on a tie at the
		// max (audit Task 8). Progress is pure, so the post-move snapshot
		// cannot disturb the game.
		leaders = append(leaders, leaderOf(runner.Progress(state, g)))

		// Run hooks after each move
		for _, event := range events {
			for _, hook := range hooks {
				hook(state, g, event)
			}
		}
	}
}

// optionDeltaMode selects the per-skeleton OptionDelta semantics. The
// definitions are fixed by the table in
// docs/plans/2026-06-11-audit-remediation.md (Task 7); do not improvise new
// ones in code.
type optionDeltaMode uint8

const (
	// deltaModeNone: OptionDelta is always 0. Unknown skeletons record 0
	// (never crash, never guess).
	deltaModeNone optionDeltaMode = iota
	// deltaModeShedding: options(p) = legal plays+draw for p against the
	// discard top (one pure GenerateMoves probe; the discard top is the
	// entire coupling surface).
	deltaModeShedding
	// deltaModeRummy: options(p) = legal draws + melds + discards for p, the
	// union across the three turn phases (per the Task 7 table; a
	// single-phase reading would compare counts across different phases and
	// every discard would score drawOptions-discardOptions of pure phase
	// noise). Deltas attach only to the turn-passing move (MoveDiscard):
	// draw/meld/pass keep the mover acting, and self-perturbation is not
	// coupling -- the coupling surface is the discard top + table melds.
	// Implementation note: a discard does not touch the next player's hand,
	// and the meld + discard components of their union depend only on that
	// hand, so before/after they cancel exactly and the union delta equals
	// the draw-phase delta. The loop therefore probes only PhaseDraw. If a
	// future mechanic lets a discard change the next player's meld or
	// discard options (e.g. laying off on table melds), this cancellation
	// no longer holds and the probe must widen back to the full union.
	deltaModeRummy
	// deltaModeTrickTaking: deltas attach to trick-LEADING plays only (the
	// trick was empty when the move was chosen):
	//
	//	OptionDelta = legalMoves(next, post-lead) - len(next player's hand)
	//
	// i.e. the constraint the lead imposes on the follower, measured against
	// the follower's unconstrained hand size as the pre-move reference.
	// Always <= 0; nonzero only when follow rules bind (MustFollowSuit with
	// a partially-matching hand). Follows and trick-completing plays record
	// 0: mid-trick counterfactuals are ill-defined -- the leader sets follow
	// legality -- but the lead's constraining power IS well-defined and
	// genome-linked: MustFollowSuit genomes produce negative lead deltas,
	// free-play genomes produce all zeros, a real within-skeleton gradient.
	// AMENDED per the Task 7 table (2026-06-11, Wave D review): the original
	// always-0 rule made Interaction a closed-form constant (2/N) for
	// trick-taking, recreating the audit's skeleton-constant pathology.
	deltaModeTrickTaking
)

func optionDeltaModeFor(g *genome.Genome) optionDeltaMode {
	switch g.Skeleton {
	case genome.Shedding:
		return deltaModeShedding
	case genome.Rummy:
		return deltaModeRummy
	case genome.TrickTaking:
		return deltaModeTrickTaking
	default:
		return deltaModeNone
	}
}

// --- Choice-impact sampling (Task 28 round 2, decisions fix) ---
//
// A turn with >= 2 legal moves is MEANINGFUL only if the choice plausibly
// matters. Cheap sampled test: up to maxChoiceSamples legal moves at a
// deterministic index spread (first/last/two middles -- no RNG, so per-seed
// traces stay reproducible) are reduced to a choice signature of
//
//	(move type, special-effect profile, next-player option-SET probe)
//
// and the turn is meaningful iff any two sampled signatures differ. The
// probe is a hash of the next player's legal-move SET, not its count: two
// hypothetical tops can leave the opponent the same NUMBER of options while
// changing WHICH cards are playable, and count-equality misread those real
// choices as impactless (measured: crazy-eights density fell to 0.125 under
// a count probe vs 0.28 with the exact set discriminant; the probe already
// generates the moves, so hashing them costs nothing extra). The probe
// semantics are per skeleton, mirroring the OptionDelta table:
//
//	shedding:     hypothetical top-card swap (the discard top is the entire
//	              coupling surface), option count of the canonical next
//	              player (peekNextPlayer; specials may redirect the actual
//	              next actor, but the profile component already carries the
//	              skip/draw/reverse differences)
//	tricktaking:  hypothetical card appended to the open trick for
//	              non-completing plays; trick-COMPLETING plays keep probe 0 --
//	              the option-count probe is undefined across trick
//	              resolution, and post-trick hand sizes are equal anyway
//	rummy/none:   no probe; meaningful iff >= 2 legal moves (unchanged
//	              semantics). The cheap probe cannot capture rummy's coupling
//	              surface: a discard's value to the next player is
//	              hidden-information-dependent (the known card vs the unknown
//	              stock), and the next player's own option COUNTS are
//	              insensitive to WHICH card is discarded -- a count probe
//	              would collapse gin rummy's core discard decision exactly as
//	              hard as a degenerate's. Per the Task 7 discipline, do not
//	              improvise a rummy definition in code; the pair-meld fixture
//	              is separated by the calibration gate instead.
//
// Collapse cases (Task 28 round-2 fixtures): all-wild same-effect shedding
// hands (A1) and no-follow trick hands (A2) sample identical signatures =>
// NOT meaningful; real crazy-eights suit choices and must-follow leads
// produce differing probes => meaningful.
const maxChoiceSamples = 4

// choiceSignature is the discriminant the sampled moves are compared on.
// probe is 0 both when no probe applies to the move and when a probe
// panicked (probeOptionSetHash's recover contract); equal values always mean
// "no observed difference", so a degraded probe can only make a turn LESS
// meaningful, never crash a batch worker.
type choiceSignature struct {
	moveType MoveType
	profile  uint8
	probe    uint64
}

// choiceSampleIndices fills buf with up to maxChoiceSamples distinct indices
// spread over [0, n): first, last, and the two third-points.
func choiceSampleIndices(n int, buf *[maxChoiceSamples]int) []int {
	candidates := [maxChoiceSamples]int{0, n - 1, n / 3, (2 * n) / 3}
	out := buf[:0]
	for _, idx := range candidates {
		dup := false
		for _, seen := range out {
			if seen == idx {
				dup = true
				break
			}
		}
		if !dup {
			out = append(out, idx)
		}
	}
	return out
}

// specialEffectProfile returns a bitmask of the EFFECT special types (skip,
// reverse, draw-two, draw-four) the card triggers under g's rules.
// SpecialWild is excluded: wildness is playability, already reflected in the
// legal-move count, not an effect of choosing this card over another.
func specialEffectProfile(g *genome.Genome, c Card) uint8 {
	var profile uint8
	for _, sc := range g.SpecialCards {
		if sc.Type == genome.SpecialWild {
			continue
		}
		if sc.MatchesCard(uint8(c.Rank), uint8(c.Suit)) {
			profile |= 1 << sc.Type
		}
	}
	return profile
}

// turnIsMeaningful implements the choice-impact test described above.
// scratch is a caller-owned Card reused for the hypothetical top-card swap so
// the hot loop stays allocation-free. All state mutations are swap+restore;
// GenerateMoves is pure (audit Task 3), so the game is undisturbed.
func turnIsMeaningful(runner GenericRunner, state *GameState, g *genome.Genome, moves []Move, mode optionDeltaMode, scratch *Card) bool {
	if len(moves) < 2 {
		return false
	}
	if mode != deltaModeShedding && mode != deltaModeTrickTaking {
		return true
	}
	var buf [maxChoiceSamples]int
	indices := choiceSampleIndices(len(moves), &buf)
	var first choiceSignature
	for i, idx := range indices {
		sig := choiceSignatureOf(runner, state, g, moves[idx], mode, scratch)
		if i == 0 {
			first = sig
		} else if sig != first {
			return true
		}
	}
	return false
}

// choiceSignatureOf computes one sampled move's signature. Only card plays
// carry a profile/probe; other move types are discriminated by type alone
// (a knock vs a pass is trivially a different choice).
func choiceSignatureOf(runner GenericRunner, state *GameState, g *genome.Genome, m Move, mode optionDeltaMode, scratch *Card) choiceSignature {
	sig := choiceSignature{moveType: m.Type}
	if m.Type != MovePlay || len(m.Cards) == 0 {
		return sig
	}
	c := m.Cards[0]
	switch mode {
	case deltaModeShedding:
		sig.profile = specialEffectProfile(g, c)
		// Self-returning plays carry no coupling probe: in a 2-player game
		// EVERY effect special (skip, reverse, and the draw penalties'
		// "draw and lose your turn") hands the turn straight back to the
		// mover, so the opponent never acts against this hypothetical top
		// card -- probing it measures a counterfactual the effect makes
		// unreachable (same principle as the OptionDelta self-perturbation
		// guard, Task 28 round 2). The profile component still discriminates
		// inflict-vs-plain choices, which ARE real even in 2p.
		if state.NumPlayers == 2 && sig.profile != 0 {
			return sig
		}
		prevTop := state.TopCard
		*scratch = c
		state.TopCard = scratch
		sig.probe = probeOptionSetHash(runner, state, g, peekNextPlayer(state))
		state.TopCard = prevTop
	case deltaModeTrickTaking:
		if len(state.TrickCards)+1 < state.NumPlayers {
			n := len(state.TrickCards)
			state.TrickCards = append(state.TrickCards, c)
			sig.probe = probeOptionSetHash(runner, state, g, peekNextPlayer(state))
			state.TrickCards = state.TrickCards[:n]
		}
	}
	return sig
}

// probeOptionSetHash returns an FNV-1a hash over player p's legal-move set
// (move types and card sequences, in generation order -- deterministic since
// audit Task 1) in the given state, by temporarily retargeting state.Active.
// GenerateMoves is pure (audit Task 3), so swap+restore leaves the state
// bit-identical. Returns 0 if the runner panics on the out-of-turn probe:
// per the choiceSignature contract, a degraded probe reads as "no observed
// difference" -- a batch worker must never crash on instrumentation.
func probeOptionSetHash(runner GenericRunner, state *GameState, g *genome.Genome, p int) (h uint64) {
	prevActive := state.Active
	defer func() {
		state.Active = prevActive
		if recover() != nil {
			h = 0
		}
	}()
	state.Active = p
	const (
		fnvOffset64 = 14695981039346656037
		fnvPrime64  = 1099511628211
	)
	h = fnvOffset64
	for _, m := range runner.GenerateMoves(state, g) {
		h ^= uint64(m.Type) + 1
		h *= fnvPrime64
		for _, c := range m.Cards {
			h ^= uint64(c.Suit)<<8 | uint64(c.Rank)
			h *= fnvPrime64
		}
	}
	return h
}

// probeOptionCount returns how many legal moves player p would have in the
// given state, by temporarily retargeting state.Active. GenerateMoves is pure
// (audit Task 3), so swap+restore leaves the state bit-identical. Returns -1
// if the runner panics on an out-of-turn probe: per Task 7, a skeleton whose
// move generation cannot be probed degrades to OptionDelta 0 -- a batch
// worker must never crash on instrumentation.
func probeOptionCount(runner GenericRunner, state *GameState, g *genome.Genome, p int) (n int) {
	prevActive := state.Active
	defer func() {
		state.Active = prevActive
		if recover() != nil {
			n = -1
		}
	}()
	state.Active = p
	return len(runner.GenerateMoves(state, g))
}

// probePhaseOptionCount returns how many legal moves player p would have in
// the given state at the given phase (rummy's GenerateMoves switches on
// state.Phase). Swap+restore is safe because GenerateMoves is pure; returns
// -1 on probe panic, like probeOptionCount.
func probePhaseOptionCount(runner GenericRunner, state *GameState, g *genome.Genome, p int, phase PhaseType) (n int) {
	prevActive, prevPhase := state.Active, state.Phase
	defer func() {
		state.Active, state.Phase = prevActive, prevPhase
		if recover() != nil {
			n = -1
		}
	}()
	state.Active = p
	state.Phase = phase
	return len(runner.GenerateMoves(state, g))
}

// peekNextPlayer returns the player after Active in the current play
// direction without mutating state (mirrors GameState.NextPlayer).
func peekNextPlayer(state *GameState) int {
	dir := state.Direction
	if dir == 0 {
		dir = 1
	}
	return ((state.Active+dir)%state.NumPlayers + state.NumPlayers) % state.NumPlayers
}

// leaderOf returns the index holding the strictly greatest progress value,
// or -1 when the maximum is shared (tie) or the slice is empty.
func leaderOf(progress []float64) int8 {
	if len(progress) == 0 {
		return -1
	}
	best := 0
	tied := false
	for i := 1; i < len(progress); i++ {
		if progress[i] > progress[best] {
			best = i
			tied = false
		} else if progress[i] == progress[best] {
			tied = true
		}
	}
	if tied {
		return -1
	}
	return int8(best)
}

func capLegalMoves(n int) uint8 {
	if n > 255 {
		return 255
	}
	return uint8(n)
}

func clampOptionDelta(d int) int8 {
	if d > 127 {
		return 127
	}
	if d < -128 {
		return -128
	}
	return int8(d)
}

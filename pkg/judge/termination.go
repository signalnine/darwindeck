package judge

import (
	"math/rand/v2"
	"sort"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

// TerminationInfo is the structural termination evidence for one game. It is
// THE FIX for the prior validation flaw: an LLM judge that conflated "low
// completion under greedy self-play" (a game that stalls near the turn cap)
// with "degenerate design". The completion percentages alone are exactly the
// signal that misled the prototype into condemning Gin/Knock Rummy; the
// reachable-win signal is the corrective evidence that proves a sound win
// condition exists BY DESIGN, regardless of how fast the AI reaches it.
type TerminationInfo struct {
	// CompletionStdPct is the percentage of games that ended with a winner at
	// the genome's standard turn cap (g.MaxTurns()).
	CompletionStdPct float64
	// CompletionExtPct is the percentage that ended with a winner when the cap
	// is extended 4x. A game whose completion CLIMBS sharply at the extended
	// cap is slow, not broken: its win condition is reachable, the AI just
	// needs more turns.
	CompletionExtPct float64
	// CapStd / CapExt are the turn caps used (for the dossier text).
	CapStd int
	CapExt int
	// GamesSampled is the batch size used for each measurement.
	GamesSampled int

	// Skeleton drives which reachable-win field below is populated.
	Skeleton genome.SkeletonType

	// --- rummy reachable-win signal ---
	// ReachableKnock is true iff, in at least one sampled game, some player's
	// deadwood fell to <= the knock threshold so that a knock/gin became a
	// LEGAL move (a knock move appeared in the legal-move list). This proves
	// the win condition is reachable by the rules, even when the greedy AI
	// rarely actually knocks before the cap.
	ReachableKnock bool
	// MedianTurnsToKnockLegal is the median turn (over games where it
	// happened) at which a knock first became legal. 0 if it never did.
	MedianTurnsToKnockLegal int
	// AnyKnockOrGin is true iff any sampled game actually ended via a
	// knock/gin round-end (Detail "knock"/"gin").
	AnyKnockOrGin bool

	// --- shedding reachable-win signal ---
	// MedianTurnsToEmptyHand is the median turn count, over COMPLETED sampled
	// games, at which a player first emptied their hand (i.e. won the round).
	// 0 if no game completed.
	MedianTurnsToEmptyHand int

	// --- trick-taking reachable-win signal ---
	// RoundsComplete is true iff at least one sampled game reached round
	// completion (all tricks played out to a scored round).
	RoundsComplete bool

	// --- bidding/contract signal (all skeletons; meaningful for trick) ---
	// HasContractScoring is true iff the game uses bid/contract-style scoring
	// rather than plain per-trick scoring. v2 has no explicit bidding phase, so
	// this is derived: trick-taking games with card-points or avoidance
	// scoring, OR a trump rule, are "contract-like"; plain per-trick no-trump
	// is not. This disambiguates the Spades/Oh-Hell/Hearts family from plain
	// Whist (the prototype's naming collapse).
	HasContractScoring bool
}

// computeTermination runs instrumented self-play to produce the termination
// evidence. It uses ONLY the public GenericRunner interface (Setup, Upkeep,
// CheckEnd, GenerateMoves, ApplyMove) and never mutates the genome or the
// frozen metric stack -- the loop is a read-only observer. baseSeed makes the
// measurement reproducible.
func computeTermination(g *genome.Genome, runner sim.GenericRunner, ai sim.AIPlayer, n int, baseSeed uint64) TerminationInfo {
	info := TerminationInfo{
		Skeleton:           g.Skeleton,
		GamesSampled:       n,
		CapStd:             g.MaxTurns(),
		CapExt:             g.MaxTurns() * 4,
		HasContractScoring: hasContractScoring(g),
	}

	stdCompletions := 0
	extCompletions := 0
	var knockLegalTurns []int
	var emptyHandTurns []int

	// record folds one game's reachable-win observations into info. The
	// reachable signals (knock-legal, hand-empty, round-complete) are collected
	// from BOTH the standard-cap and extended-cap runs: a reachable terminal
	// state is a property of the rules, so observing it under either cap is
	// valid evidence, and pooling the two caps makes the boolean signal robust
	// to the greedy AI's slow, seed-sensitive descent toward the win condition
	// (the whole point of the fix -- a sound but slow game like Gin must not be
	// reported unreachable just because one cap's sample missed the knock).
	record := func(obs gameObservation) {
		if obs.knockBecameLegal {
			info.ReachableKnock = true
			knockLegalTurns = append(knockLegalTurns, obs.firstKnockLegalTurn)
		}
		if obs.knockOrGinEnded {
			info.AnyKnockOrGin = true
		}
		if obs.roundCompleted {
			info.RoundsComplete = true
		}
		if obs.completed && obs.emptyHandTurn > 0 {
			emptyHandTurns = append(emptyHandTurns, obs.emptyHandTurn)
		}
	}

	for i := 0; i < n; i++ {
		// Standard-cap probe: completion% measured here.
		rngStd := rand.New(rand.NewPCG(baseSeed+uint64(i), 0))
		obs := observeGame(g, runner, ai, rngStd, info.CapStd)
		if obs.completed {
			stdCompletions++
		}
		record(obs)

		// Extended-cap probe (distinct stream): completion% measured here too,
		// and its reachable-win observations are pooled into the signal.
		rngExt := rand.New(rand.NewPCG(baseSeed+uint64(i), 1))
		obsExt := observeGame(g, runner, ai, rngExt, info.CapExt)
		if obsExt.completed {
			extCompletions++
		}
		record(obsExt)
	}

	if n > 0 {
		info.CompletionStdPct = 100 * float64(stdCompletions) / float64(n)
		info.CompletionExtPct = 100 * float64(extCompletions) / float64(n)
	}
	info.MedianTurnsToKnockLegal = medianInt(knockLegalTurns)
	info.MedianTurnsToEmptyHand = medianInt(emptyHandTurns)

	return info
}

// gameObservation captures the per-game signals the dossier needs.
type gameObservation struct {
	completed bool
	// knockBecameLegal: at some turn, the active rummy player's legal-move list
	// included a MoveKnock (deadwood <= threshold), so a knock/gin was a legal
	// option by the rules.
	knockBecameLegal    bool
	firstKnockLegalTurn int
	// knockOrGinEnded: a round ended with Detail "knock" or "gin".
	knockOrGinEnded bool
	// roundCompleted: a trick-taking round reached completion (EventRoundEnd
	// "tricks_complete") or any round-end fired.
	roundCompleted bool
	// emptyHandTurn: the turn at which a shedding player first emptied their
	// hand (the round/game win moment). 0 if it never happened.
	emptyHandTurn int
}

// observeGame plays one instrumented game to the given cap. It mirrors the
// production single-game loop's control flow (Upkeep -> CheckEnd -> turn-cap ->
// GenerateMoves -> ApplyMove) but records the skeleton-specific reachable-win
// signals. It is a pure read-only observer: it never touches the genome or any
// frozen package state.
func observeGame(g *genome.Genome, runner sim.GenericRunner, ai sim.AIPlayer, rng *rand.Rand, maxTurns int) gameObservation {
	var obs gameObservation
	state := runner.Setup(g, rng)

	// Independent iteration guard, matching production runSingleGame: rummy
	// runs several ApplyMove calls per turn, so a stalled phase must not loop
	// forever.
	iterCap := (maxTurns+1)*100 + 10000
	iter := 0

	for {
		runner.Upkeep(state, g)

		if winner := runner.CheckEnd(state, g); winner >= 0 {
			obs.completed = true
			return obs
		}
		if state.Turn >= maxTurns || iter >= iterCap {
			return obs
		}
		iter++

		moves := runner.GenerateMoves(state, g)
		if len(moves) == 0 {
			return obs
		}

		// Reachable-win observation BEFORE the move applies: is a knock a legal
		// option right now? (rummy)
		if g.Skeleton == genome.Rummy && !obs.knockBecameLegal {
			for _, m := range moves {
				if m.Type == sim.MoveKnock {
					obs.knockBecameLegal = true
					obs.firstKnockLegalTurn = state.Turn
					break
				}
			}
		}

		move := ai.SelectMove(moves, state, rng)
		mover := state.Active
		moverHandBefore := len(state.Hands[mover])

		events := runner.ApplyMove(state, move, g)
		state.Events = append(state.Events, events...)

		// Shedding: detect the win moment (a player emptied their hand). The
		// mover going from non-empty to empty is the round/game win.
		if g.Skeleton == genome.Shedding && obs.emptyHandTurn == 0 &&
			moverHandBefore > 0 && len(state.Hands[mover]) == 0 {
			obs.emptyHandTurn = state.Turn
		}

		// Round-end signals from events.
		for _, e := range events {
			if e.Type == sim.EventRoundEnd {
				switch e.Detail {
				case "knock", "gin":
					obs.knockOrGinEnded = true
				case "tricks_complete":
					obs.roundCompleted = true
				}
				if g.Skeleton == genome.TrickTaking {
					obs.roundCompleted = true
				}
			}
		}
	}
}

// hasContractScoring derives whether the game uses bid/contract-style scoring
// rather than plain per-trick. See TerminationInfo.HasContractScoring.
func hasContractScoring(g *genome.Genome) bool {
	if g.Skeleton != genome.TrickTaking || g.TrickTaking == nil {
		return false
	}
	if g.TrickTaking.TrickScoring != genome.ScorePerTrick {
		return true
	}
	if g.TrumpRule != genome.TrumpNone {
		return true
	}
	return false
}

func medianInt(xs []int) int {
	if len(xs) == 0 {
		return 0
	}
	cp := append([]int(nil), xs...)
	sort.Ints(cp)
	return cp[len(cp)/2]
}

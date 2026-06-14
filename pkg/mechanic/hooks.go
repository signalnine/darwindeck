package mechanic

import (
	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

// HookPoint identifies when a hook fires.
type HookPoint int

const (
	HookAfterPlay    HookPoint = iota // After a card is played
	HookEndOfRound                     // After a round/hand ends
	HookScoring                        // During scoring phase
)

// Hook is a borrowed mechanic's behavior injected into a skeleton runner.
type Hook struct {
	Point    HookPoint
	Mechanic genome.MechanicType
	Apply    func(state *sim.GameState, g *genome.Genome, event sim.Event)
}

// BuildHooks creates the hook functions for a genome's borrowed mechanics.
//
// Historical note: MechTrump and MechPlayMultiple used to have empty cases
// here. They never had a hook (or runner-side) implementation, so the borrows
// were inert; both are now reserved enum values rejected by validation
// (dd-lnh; audit remediation Task 23).
func BuildHooks(g *genome.Genome) []Hook {
	var hooks []Hook

	for _, bm := range g.Borrowed {
		switch bm.Mechanic {
		case genome.MechAvoidance:
			hooks = append(hooks, Hook{
				Point:    HookScoring,
				Mechanic: genome.MechAvoidance,
				Apply:    applyAvoidance,
			})

		case genome.MechMeldBonus:
			hooks = append(hooks, Hook{
				Point:    HookEndOfRound,
				Mechanic: genome.MechMeldBonus,
				Apply:    applyMeldBonus,
			})

		case genome.MechDrawPenalty:
			hooks = append(hooks, Hook{
				Point:    HookAfterPlay,
				Mechanic: genome.MechDrawPenalty,
				Apply:    applyDrawPenalty,
			})

		case genome.MechTrickScoring:
			hooks = append(hooks, Hook{
				Point:    HookEndOfRound,
				Mechanic: genome.MechTrickScoring,
				Apply:    applyTrickScoring,
			})
		}
	}

	return hooks
}

// HooksFor converts a genome's borrowed mechanics into sim.HookFunc closures
// ready to pass to sim.RunBatch or an interactive playtest session, mapping
// each hook's HookPoint onto the event type that triggers it (HookAfterPlay
// <- EventCardPlayed; HookEndOfRound and HookScoring <- EventRoundEnd).
//
// This is the SINGLE hook-construction site (audit Task 24): the fitness
// pipeline (Tier 1 + Tier 2, pkg/fitness/evaluate.go) and the playtest
// session (pkg/playtest/session.go) both build their hooks here, so humans
// always playtest exactly the game fitness evaluated. Do not hand-roll the
// HookPoint->event mapping anywhere else; the grep-test
// TestHooksForIsSingleConstructionSite (pkg/playtest) enforces this.
//
// CONCURRENCY GUARD (Wave I, audited 2026-06-12): sim.RunBatch plays a
// batch's games in parallel and shares ONE HooksFor result across all of
// them. That is safe because every hook here is STATELESS: each closure
// captures only its Hook value (immutable after construction), and all four
// Apply implementations (applyAvoidance, applyMeldBonus, applyDrawPenalty,
// applyTrickScoring) are pure functions of (state, g, event) that mutate
// nothing but the per-game *state* they are handed -- no closure-captured
// counters, maps, or other cross-call mutable state. If you add a hook that
// carries per-game mutable state (e.g. a once-per-round latch or a running
// counter), it CANNOT live in a shared closure: hooks would then have to be
// constructed per game inside RunBatch's worker loop. Keep new hooks
// stateless, or redesign that call site first. Tripwire:
// TestRunBatchHookedBatchRaceClean (pkg/sim/parallel_test.go) runs a hooked
// parallel batch under -race.
func HooksFor(g *genome.Genome) []sim.HookFunc {
	if len(g.Borrowed) == 0 {
		return nil
	}

	hooks := BuildHooks(g)
	if len(hooks) == 0 {
		return nil
	}

	funcs := make([]sim.HookFunc, 0, len(hooks))
	for _, h := range hooks {
		hook := h // capture
		funcs = append(funcs, func(state *sim.GameState, g *genome.Genome, event sim.Event) {
			switch hook.Point {
			case HookAfterPlay:
				if event.Type == sim.EventCardPlayed {
					hook.Apply(state, g, event)
				}
			case HookEndOfRound, HookScoring:
				if event.Type == sim.EventRoundEnd {
					hook.Apply(state, g, event)
				}
			}
		})
	}

	return funcs
}

// RunHooks executes all hooks matching the given point.
func RunHooks(hooks []Hook, point HookPoint, state *sim.GameState, g *genome.Genome, event sim.Event) {
	for _, h := range hooks {
		if h.Point == point {
			h.Apply(state, g, event)
		}
	}
}

// applyAvoidance adds penalty points for certain cards collected.
// In shedding: cards still in hand at end are penalties.
// In rummy: same as deadwood but with card-specific multipliers.
func applyAvoidance(state *sim.GameState, g *genome.Genome, event sim.Event) {
	if len(g.Scoring.CardPoints) == 0 {
		return
	}

	for i := 0; i < state.NumPlayers; i++ {
		penalty := 0
		// Check cards in hand (shedding/rummy) and captured cards (trick-taking tableau)
		for _, card := range state.Hands[i] {
			penalty += cardPenalty(card, g)
		}
		if i < len(state.Tableau) {
			for _, card := range state.Tableau[i] {
				penalty += cardPenalty(card, g)
			}
		}
		state.Scores[i] -= penalty // Negative = bad
	}
}

// applyMeldBonus awards bonus points for sets/runs in a player's hand or tableau.
// Trick-taking hosts only fire EventRoundEnd once hands are empty, so the
// bonus must consider state.Tableau captures too -- otherwise the borrow is
// silently a no-op on every non-Rummy host (dd-no2).
//
// TEETH (Wave-3): the bonus rewards PAIRS (2+ same rank) and 2-card partial
// runs, not only 3+ melds. Under random play only ~7% of residual hands hold a
// triple but ~78% hold a pair, so the 3+-only threshold fired so rarely that
// the borrow was a vestigial tally CheckEnd ignored (it almost never changed
// the banked totals, so the winner was decided by the fallback tiebreak). Pairs
// score LESS per card than full sets, so a genuine 3+ meld still dominates and
// the gradient "more/bigger melds win" is preserved -- but the bonus now fires
// reliably enough to actually DECIDE the winner. Seeds carry no borrows, so the
// calibration ground-truth is unaffected.
func applyMeldBonus(state *sim.GameState, g *genome.Genome, event sim.Event) {
	for i := 0; i < state.NumPlayers; i++ {
		cards := append([]sim.Card(nil), state.Hands[i]...)
		if i < len(state.Tableau) {
			cards = append(cards, state.Tableau[i]...)
		}
		bonus := 0

		// Check for sets: 3+ same rank score 5/card; pairs (exactly 2) score
		// 2/card -- enough to fire reliably under random play, less than a real
		// set so bigger melds still win.
		rankCount := make(map[sim.Rank]int)
		for _, c := range cards {
			rankCount[c.Rank]++
		}
		for _, count := range rankCount {
			switch {
			case count >= 3:
				bonus += count * 5 // 5 points per card in a set
			case count == 2:
				bonus += count * 2 // 2 points per card in a pair
			}
		}

		// Check for runs of same suit: 3+ consecutive score 3/card; a 2-card
		// consecutive partial run scores 1/card (same "fires reliably, full run
		// still dominates" rationale as pairs above).
		suitCards := make(map[sim.Suit][]int)
		for _, c := range cards {
			suitCards[c.Suit] = append(suitCards[c.Suit], int(c.Rank))
		}
		for _, ranks := range suitCards {
			if len(ranks) < 2 {
				continue
			}
			// Sort and find consecutive
			sortInts(ranks)
			run := 1
			for j := 1; j < len(ranks); j++ {
				if ranks[j] == ranks[j-1]+1 {
					run++
				} else {
					bonus += runBonus(run)
					run = 1
				}
			}
			bonus += runBonus(run)
		}

		state.Scores[i] += bonus
	}
}

// runBonus scores a maximal same-suit consecutive run of length n: 3 points per
// card for a full 3+ run, 1 point per card for a 2-card partial run, nothing
// for a lone card. The partial-run tier is the Wave-3 teeth (fires reliably
// under random play) while keeping full runs strictly more valuable.
func runBonus(n int) int {
	switch {
	case n >= 3:
		return n * 3
	case n == 2:
		return n * 1
	default:
		return 0
	}
}

// applyDrawPenalty forces the active player to draw extra cards after certain plays.
func applyDrawPenalty(state *sim.GameState, g *genome.Genome, event sim.Event) {
	if event.Type != sim.EventCardPlayed {
		return
	}
	// Draw 1 extra card on high-value plays (face cards)
	if len(event.Cards) > 0 && event.Cards[0].Rank >= sim.Jack {
		if len(state.Deck) > 0 {
			drawn, rest := sim.DrawN(state.Deck, 1)
			state.Deck = rest
			state.Hands[event.PlayerID] = append(state.Hands[event.PlayerID], drawn...)
		}
	}
}

// applyTrickScoring adds trick-like scoring to non-trick games.
// Awards a bonus to the player who "captured" the most cards this round.
//
// For trick-taking hosts, captures live in state.Tableau. For rummy hosts
// (the only currently-whitelisted Rummy borrow target for MechTrickScoring),
// the runner never populates Tableau; laid-down melds live in state.Melds /
// state.MeldOwner instead. Counting both keeps the borrow functional across
// every legal host without coupling skeletons via shared tableau semantics
// (dd-25u).
func applyTrickScoring(state *sim.GameState, g *genome.Genome, event sim.Event) {
	captures := make([]int, state.NumPlayers)
	for i := 0; i < state.NumPlayers; i++ {
		if i < len(state.Tableau) {
			captures[i] += len(state.Tableau[i])
		}
	}
	for mi, owner := range state.MeldOwner {
		if owner < 0 || owner >= state.NumPlayers {
			continue
		}
		if mi >= len(state.Melds) {
			continue
		}
		captures[owner] += len(state.Melds[mi])
	}

	maxCapture := 0
	for _, c := range captures {
		if c > maxCapture {
			maxCapture = c
		}
	}
	if maxCapture == 0 {
		return
	}
	// Split the bonus across every player tied for max captures. A strict
	// "first wins" tiebreak biased toward the lowest-indexed seat (dd-hid),
	// systematically inflating greedy's apparent score whenever a borrowed
	// trick-scoring host produced a tie.
	tied := 0
	for _, c := range captures {
		if c == maxCapture {
			tied++
		}
	}
	share := maxCapture / tied
	if share == 0 {
		return
	}
	for i, c := range captures {
		if c == maxCapture {
			state.Scores[i] += share
		}
	}
}

// cardPenalty returns the penalty points for card under g's scoring rules.
// Delegates to genome.MatchCardPoints so penalty resolution stays in lockstep
// with cardPointValue in pkg/skeleton/tricktaking/runner.go (dd-cto).
func cardPenalty(card sim.Card, g *genome.Genome) int {
	return genome.MatchCardPoints(g.Scoring.CardPoints, uint8(card.Rank), uint8(card.Suit))
}

func sortInts(a []int) {
	// Simple insertion sort for small slices
	for i := 1; i < len(a); i++ {
		key := a[i]
		j := i - 1
		for j >= 0 && a[j] > key {
			a[j+1] = a[j]
			j--
		}
		a[j+1] = key
	}
}

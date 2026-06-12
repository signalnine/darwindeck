package sim

import (
	"math"
	"math/rand/v2"

	"github.com/darwindeck/darwindeck/pkg/genome"
)

// ISMCTS defaults (audit Task 19).
//
// PERFORMANCE BUDGET RESULT (measured 2026-06-11, i7-14700, single thread):
// BenchmarkMCTSGame (gin rummy, seat 0 at these defaults vs random) runs at
// ~720-775ms per game, so the design's 20 MCTS games per Tier 2 genome cost
// ~14.5s -- the plan's <= 2s budget FAILS by ~7x. CPU/alloc profiles show
// >95% of the cost inside the rummy runner's move generation
// (enumerateMeldCandidates / bestPartitionValueDP / calcDeadwood and the
// per-call move-slice allocation), which rollouts invoke once per simulated
// move; the sim-layer share (Clone ~1%, Move.Key ~2.6%, tree bookkeeping
// ~1%) is negligible, so no optimization within this file can close the gap.
// Per the plan, the defaults stay as designed and Task 20 takes the
// pre-defined fallback: MCTS-for-top-decile-only (greedy two-tier for the
// rest), with the mode recorded in results meta.json.
const (
	DefaultMCTSIterations       = 200
	DefaultMCTSDeterminizations = 10
	DefaultMCTSRolloutCap       = 200

	// mctsExploration is UCB1's c constant (plan-fixed at 1.4 ~ sqrt(2)).
	mctsExploration = 1.4
)

// MCTSAI is a determinized information-set MCTS player (single-observer
// ISMCTS, Cowling et al. style). For each of Determinizations resamplings of
// the hidden information (Determinize: deck + all OTHER hands pooled and
// reshuffled; own hand and public zones fixed), it runs Iterations/
// Determinizations UCT iterations on a per-determinization tree, then
// aggregates root-child visit counts across determinizations by Move.Key and
// returns the most-visited move.
//
// Hidden information is handled at the information-set boundary: the search
// never reads the true opponent hands (v1's omniscient MCTS is explicitly
// not reproduced here).
//
// Known model limitation: the search steps states with the runner's
// GenerateMoves/ApplyMove/Upkeep/CheckEnd only -- borrowed-mechanic hooks
// (which the outer batch loop applies after each move) are not simulated, so
// for genomes with borrows the internal model is hook-blind. This mirrors
// the greedy scorer's situation and is recorded for Task 20/22.
//
// Allocation strategy (benchmark-determined per the plan): the naive
// heap-allocating Clone stays. BenchmarkCloneRollout measured GC at well over
// the 10% threshold (GOGC=off runs ~3x faster), but alloc_space profiling
// shows ~99% of allocation inside the rummy runner's move generation
// (bestPartitionValueDP 63%, move slices ~31%); GameState.Clone itself is
// 0.5%. Pooling GameState would not move the needle -- the hot spot is the
// runner, which is outside Task 19's scope and a Task 20 budget concern.
//
// Zero-valued fields fall back to the Default* constants, so
// &MCTSAI{Runner: r, Genome: g} runs at production settings. A nil Runner or
// Genome degrades to a uniform random choice -- a batch worker must never
// crash on a misconfigured AI.
//
// CONCURRENCY (Wave I): one MCTSAI instance IS shared across all games of a
// parallel RunBatch (fitness.runMCTSBatch builds exactly one for the 20-game
// Tier 2 skill batch). That is safe because every field here is read-only
// configuration during SelectMove: all per-decision mutable structures
// (determinizations, trees, visit maps, the rollout state) are locals of the
// call. Do not add mutable fields that SelectMove writes (caches, counters,
// "last move" memos) without moving construction into RunBatch's per-game
// worker loop first. Tripwire: TestRunBatchSharedMCTSAIRaceClean
// (parallel_test.go) under -race.
type MCTSAI struct {
	Runner GenericRunner
	Genome *genome.Genome

	Iterations       int // total UCT iterations, split across determinizations
	Determinizations int // hidden-info resamplings per decision
	RolloutCap       int // max applied moves per random rollout
}

// mctsNode is one tree node. The root has no edge; every other node is
// reached by exactly one move and stores statistics from the perspective of
// the player who made that move.
type mctsNode struct {
	player   int     // player who took the edge into this node
	visits   int     // simulations through this node
	avail    int     // times this node was available for selection (ISMCTS availability count)
	wins     float64 // reward sum for `player`
	children map[string]*mctsNode
}

// SelectMove implements AIPlayer.
func (ai *MCTSAI) SelectMove(moves []Move, state *GameState, rng *rand.Rand) Move {
	if len(moves) == 0 {
		return Move{Type: MovePass}
	}
	if len(moves) == 1 {
		return moves[0]
	}
	if ai.Runner == nil || ai.Genome == nil || state == nil {
		return moves[rng.IntN(len(moves))]
	}

	dets := ai.Determinizations
	if dets <= 0 {
		dets = DefaultMCTSDeterminizations
	}
	iters := ai.Iterations
	if iters <= 0 {
		iters = DefaultMCTSIterations
	}
	itersPerDet := iters / dets
	if itersPerDet < 1 {
		itersPerDet = 1
	}

	rootPlayer := state.Active
	maxTurns := ai.Genome.MaxTurns()

	// Aggregate root-child visit counts across determinizations, keyed by
	// the canonical Move.Key (Task 19 step 0: keys are stable across
	// determinizations of the same info-state).
	totalVisits := make(map[string]int, len(moves))

	for d := 0; d < dets; d++ {
		det := Determinize(state, rootPlayer, rng)
		root := &mctsNode{player: -1, children: make(map[string]*mctsNode, len(moves))}
		for it := 0; it < itersPerDet; it++ {
			ai.runIteration(root, det, maxTurns, rng)
		}
		for key, child := range root.children {
			totalVisits[key] += child.visits
		}
	}

	// Most-visited root move; ties break to the earliest move in the
	// caller's list, keeping selection deterministic under a fixed rng.
	best := 0
	bestVisits := totalVisits[moves[0].Key()]
	for i := 1; i < len(moves); i++ {
		if v := totalVisits[moves[i].Key()]; v > bestVisits {
			best, bestVisits = i, v
		}
	}
	return moves[best]
}

// runIteration performs one UCT iteration (selection, expansion, rollout,
// backpropagation) on a fresh clone of the determinized root state.
//
// CONTRACT (audit Wave B finding): Upkeep is NOT idempotent -- rummy banks
// deadwood and tricktaking advances Round/redeals at round end. The search
// must therefore run Upkeep exactly once after each ApplyMove and never
// before the first one: det (like the real state SelectMove was handed) is
// already AT a decision point -- the game loop ran Upkeep+CheckEnd before
// calling the AI. stepState is the only place the search mutates a state.
func (ai *MCTSAI) runIteration(root *mctsNode, det *GameState, maxTurns int, rng *rand.Rand) {
	st := det.Clone()
	st.RNG = rng // clones carry no RNG; rollout shuffles draw from the search rng

	node := root
	path := make([]*mctsNode, 0, 16)
	winner := -1
	terminal := false

	// Selection + expansion. Depth is bounded by the per-determinization
	// iteration count (each iteration adds at most one node), so no separate
	// depth guard is needed.
	for {
		moves := ai.Runner.GenerateMoves(st, ai.Genome)
		if len(moves) == 0 {
			terminal = true // dead end: no winner (batch loop classifies as no_moves)
			break
		}

		// Availability bookkeeping plus untried-move scan in one pass.
		untried := -1
		nUntried := 0
		for i := range moves {
			if _, ok := node.children[moves[i].Key()]; !ok {
				nUntried++
				// Reservoir-sample one untried move uniformly.
				if rng.IntN(nUntried) == 0 {
					untried = i
				}
			}
		}

		if untried >= 0 {
			// Expansion: create the child, step into it, then rollout.
			mv := moves[untried]
			child := &mctsNode{player: st.Active}
			node.children[mv.Key()] = child
			for i := range moves {
				if c, ok := node.children[moves[i].Key()]; ok {
					c.avail++
				}
			}
			path = append(path, child)
			if w, done := ai.stepState(st, mv, maxTurns); done {
				winner, terminal = w, true
			}
			break
		}

		// All moves tried: UCB1 selection among the available children.
		bestIdx := -1
		bestScore := math.Inf(-1)
		for i := range moves {
			c := node.children[moves[i].Key()]
			c.avail++
			score := c.wins/float64(c.visits) +
				mctsExploration*math.Sqrt(math.Log(float64(c.avail))/float64(c.visits))
			if score > bestScore {
				bestIdx, bestScore = i, score
			}
		}
		child := node.children[moves[bestIdx].Key()]
		path = append(path, child)
		if w, done := ai.stepState(st, moves[bestIdx], maxTurns); done {
			winner, terminal = w, true
			break
		}
		if child.children == nil {
			child.children = make(map[string]*mctsNode, 4)
		}
		node = child
	}

	// Rollout: uniform random to terminal or RolloutCap applied moves.
	if !terminal {
		winner = ai.rollout(st, maxTurns, rng)
	}

	// Backpropagation: reward from each edge-player's own perspective.
	numPlayers := st.NumPlayers
	root.visits++
	for _, n := range path {
		n.visits++
		n.wins += rewardFor(n.player, winner, numPlayers)
	}
}

// stepState applies one move and then runs the between-moves segment of the
// game loop exactly as sim.runSingleGame does: Upkeep ONCE, then the pure
// CheckEnd and turn-cap queries. Returns (winner, done); winner is -1 for a
// turn-cap exit.
func (ai *MCTSAI) stepState(st *GameState, mv Move, maxTurns int) (int, bool) {
	ai.Runner.ApplyMove(st, mv, ai.Genome)
	ai.Runner.Upkeep(st, ai.Genome)
	if w := ai.Runner.CheckEnd(st, ai.Genome); w >= 0 {
		return w, true
	}
	if st.Turn >= maxTurns {
		return -1, true
	}
	return -1, false
}

// rollout plays uniformly random moves until the game ends or RolloutCap
// applied moves elapse. The cap also bounds phase-stall loops that never
// advance st.Turn (the batch loop's iterCap analog).
func (ai *MCTSAI) rollout(st *GameState, maxTurns int, rng *rand.Rand) int {
	cap := ai.RolloutCap
	if cap <= 0 {
		cap = DefaultMCTSRolloutCap
	}
	for i := 0; i < cap; i++ {
		moves := ai.Runner.GenerateMoves(st, ai.Genome)
		if len(moves) == 0 {
			return -1
		}
		if w, done := ai.stepState(st, moves[rng.IntN(len(moves))], maxTurns); done {
			return w
		}
	}
	return -1 // cap reached: treated as a draw (neutral reward)
}

// rewardFor scores an outcome from player p's perspective: 1 for a win, 0
// for a loss, 1/N for no winner (rollout cap, dead end, turn cap) so
// inconclusive lines are neutral rather than punished.
func rewardFor(p, winner, numPlayers int) float64 {
	if winner < 0 {
		if numPlayers <= 0 {
			return 0
		}
		return 1.0 / float64(numPlayers)
	}
	if p == winner {
		return 1
	}
	return 0
}

package playtest

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"strconv"
	"strings"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/mechanic"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

// Session manages an interactive playtest game.
type Session struct {
	Genome  *genome.Genome
	Runner  sim.GenericRunner
	AI      sim.AIPlayer
	State   *sim.GameState
	RNG     *rand.Rand
	Scanner *bufio.Scanner
	HumanID int // Which player is the human
	// Hooks are the genome's borrowed-mechanic hooks, built by
	// mechanic.HooksFor -- the same single construction site the fitness
	// pipeline uses (audit Task 24). They run after every applied move so a
	// human plays exactly the game fitness evaluated.
	Hooks []sim.HookFunc
}

// Outcome summarizes a finished session for ratings capture (audit Task 24).
// Winner is a player index, or -1 when the game ended without one (max turns
// or stuck). Stuck is true only for the no-legal-moves termination path.
type Outcome struct {
	Winner int
	Turns  int
	Stuck  bool
}

// WinnerLabel maps the outcome to the playtest_results.jsonl winner
// vocabulary established by the v1 file: "human", "ai", "stuck", or "none"
// (max-turns cap, no winner).
func (o Outcome) WinnerLabel(humanID int) string {
	switch {
	case o.Stuck:
		return "stuck"
	case o.Winner < 0:
		return "none"
	case o.Winner == humanID:
		return "human"
	default:
		return "ai"
	}
}

// NewMCTSAI constructs the ISMCTS opponent for the playtest `mcts`
// difficulty. Runner and Genome MUST both be set: sim.MCTSAI deliberately
// degrades to uniform random play when either is nil (batch-safety
// fallback), which would silently hand the user a random opponent labeled
// "mcts" — this constructor exists so the session and the CLI cannot
// half-wire it.
//
// Iterations/Determinizations/RolloutCap are left zero, falling back to the
// production defaults (200/10/200, pkg/sim/mcts.go). Interactive latency
// budget is sub-second per move (Task 21): at those defaults the
// worst-case skeleton (rummy movegen dominates MCTS cost, see the
// BenchmarkMCTSGame notes) measures ~10-30ms per decision, ~30-75x inside
// budget, so no Iterations tuning is needed; TestMCTSSessionCompletesGame
// enforces the budget.
func NewMCTSAI(g *genome.Genome, runner sim.GenericRunner) *sim.MCTSAI {
	return &sim.MCTSAI{Runner: runner, Genome: g}
}

// NewSession creates a playtest session.
func NewSession(g *genome.Genome, runner sim.GenericRunner, ai sim.AIPlayer, seed uint64) *Session {
	rng := rand.New(rand.NewPCG(seed, 0))
	return &Session{
		Genome:  g,
		Runner:  runner,
		AI:      ai,
		RNG:     rng,
		Scanner: bufio.NewScanner(os.Stdin),
		HumanID: 0,
		Hooks:   mechanic.HooksFor(g),
	}
}

// Run plays the game interactively and returns the outcome for ratings
// capture.
func (s *Session) Run() Outcome {
	s.State = s.Runner.Setup(s.Genome, s.RNG)
	maxTurns := s.Genome.MaxTurns()

	fmt.Printf("\n=== %s ===\n", gameName(s.Genome))
	fmt.Printf("Skeleton: %s | Players: %d | Hand: %d cards\n\n",
		s.Genome.Skeleton, s.Genome.Players, s.Genome.HandSize)

	for {
		// Mirror the simulation loop in sim.RunBatch: Upkeep runs once per
		// iteration, before CheckEnd. Skipping it here would make human games
		// diverge from simulated ones (no deck recycling, no round redeals,
		// no rummy deadwood banking).
		s.Runner.Upkeep(s.State, s.Genome)

		winner := s.Runner.CheckEnd(s.State, s.Genome)
		if winner >= 0 {
			s.printFinalState(winner)
			return Outcome{Winner: winner, Turns: s.State.Turn}
		}
		if s.State.Turn >= maxTurns {
			fmt.Printf("\nGame ended at max turns (%d).\n", maxTurns)
			return Outcome{Winner: -1, Turns: s.State.Turn}
		}

		moves := s.Runner.GenerateMoves(s.State, s.Genome)
		if len(moves) == 0 {
			fmt.Println("No legal moves — game stuck!")
			return Outcome{Winner: -1, Turns: s.State.Turn, Stuck: true}
		}

		if s.State.Active == s.HumanID {
			s.humanTurn(moves)
		} else {
			s.aiTurn(moves)
		}
	}
}

// afterMove mirrors the post-ApplyMove sequence of sim.RunBatch's game loop:
// record the move's events on the state and dispatch each one to the
// borrowed-mechanic hooks. Without this the session would play a hook-less
// variant of the game fitness evaluated -- the audit's playtest-parity
// finding (Task 24).
func (s *Session) afterMove(events []sim.Event) {
	s.State.Events = append(s.State.Events, events...)
	for _, e := range events {
		for _, hook := range s.Hooks {
			hook(s.State, s.Genome, e)
		}
	}
}

func (s *Session) humanTurn(moves []sim.Move) {
	fmt.Printf("\n--- Turn %d (You) ---\n", s.State.Turn+1)
	s.printState()

	fmt.Println("\nLegal moves:")
	for i, m := range moves {
		fmt.Printf("  %d) %s\n", i+1, describeMoveShort(m))
	}

	choice := s.getChoice(len(moves))
	move := moves[choice]
	events := s.Runner.ApplyMove(s.State, move, s.Genome)
	s.afterMove(events)
	for _, e := range events {
		if e.Detail != "" {
			fmt.Printf("  > %s\n", describeEvent(e))
		}
	}
}

func (s *Session) aiTurn(moves []sim.Move) int {
	actor := s.State.Active
	move := s.AI.SelectMove(moves, s.State, s.RNG)
	events := s.Runner.ApplyMove(s.State, move, s.Genome)
	s.afterMove(events)

	fmt.Printf("  Player %d: %s", actor, describeMoveShort(move))
	for _, e := range events {
		if e.Type == sim.EventSpecialTriggered {
			fmt.Printf(" [%s]", e.Detail)
		}
	}
	fmt.Println()
	return actor
}

func (s *Session) printState() {
	fmt.Printf("Your hand: %s\n", formatCards(s.State.Hands[s.HumanID]))

	if s.State.TopCard != nil {
		fmt.Printf("Top card: %s\n", s.State.TopCard)
	}

	// Show the shared pile's actual cards, not just a count: for casino this is the
	// face-up TABLE you capture from (you cannot choose a capture without seeing
	// it); elsewhere it is the discard pile. Cap very long piles to the recent tail.
	pileLabel := "Discard"
	if s.Genome.Skeleton == genome.Casino {
		pileLabel = "Table"
	}
	switch disc, n := s.State.Discard, len(s.State.Discard); {
	case n == 0:
		fmt.Printf("Deck: %d cards | %s: (empty)\n", len(s.State.Deck), pileLabel)
	case n <= 16:
		fmt.Printf("Deck: %d cards | %s: %s\n", len(s.State.Deck), pileLabel, formatCards(disc))
	default:
		fmt.Printf("Deck: %d cards | %s: %d cards (recent: %s)\n",
			len(s.State.Deck), pileLabel, n, formatCards(disc[n-16:]))
	}

	for i := 0; i < s.State.NumPlayers; i++ {
		if i == s.HumanID {
			continue
		}
		fmt.Printf("Player %d: %d cards", i, len(s.State.Hands[i]))
		if s.State.Scores[i] != 0 {
			fmt.Printf(" (score: %d)", s.State.Scores[i])
		}
		fmt.Println()
	}

	if s.State.Scores[s.HumanID] != 0 {
		fmt.Printf("Your score: %d\n", s.State.Scores[s.HumanID])
	}
}

func (s *Session) printFinalState(winner int) {
	fmt.Printf("\n=== Game Over (turn %d) ===\n", s.State.Turn)
	if winner == s.HumanID {
		fmt.Println("You win!")
	} else {
		fmt.Printf("Player %d wins!\n", winner)
	}

	fmt.Println("\nFinal scores:")
	for i := 0; i < s.State.NumPlayers; i++ {
		label := fmt.Sprintf("Player %d", i)
		if i == s.HumanID {
			label = "You"
		}
		fmt.Printf("  %s: %d points, %d cards remaining\n",
			label, s.State.Scores[i], len(s.State.Hands[i]))
	}
}

func (s *Session) getChoice(numMoves int) int {
	for {
		fmt.Printf("Choose (1-%d): ", numMoves)
		if !s.Scanner.Scan() {
			fmt.Println("\nGoodbye!")
			os.Exit(0)
		}
		input := strings.TrimSpace(s.Scanner.Text())
		if input == "q" || input == "quit" {
			fmt.Println("Quitting...")
			os.Exit(0)
		}
		n, err := strconv.Atoi(input)
		if err != nil || n < 1 || n > numMoves {
			fmt.Printf("Invalid choice. Enter 1-%d or 'q' to quit.\n", numMoves)
			continue
		}
		return n - 1
	}
}

func gameName(g *genome.Genome) string {
	if g.ID != "" {
		return g.ID
	}
	return fmt.Sprintf("Evolved %s Game", g.Skeleton)
}

func formatCards(cards []sim.Card) string {
	if len(cards) == 0 {
		return "(empty)"
	}
	parts := make([]string, len(cards))
	for i, c := range cards {
		parts[i] = c.String()
	}
	return strings.Join(parts, " ")
}

func describeMoveShort(m sim.Move) string {
	switch m.Type {
	case sim.MovePlay:
		return fmt.Sprintf("Play %s", formatCards(m.Cards))
	case sim.MoveDraw:
		if len(m.Cards) > 0 {
			return fmt.Sprintf("Draw %s from discard", formatCards(m.Cards))
		}
		return "Draw from deck"
	case sim.MovePass:
		return "Pass"
	case sim.MoveKnock:
		return "Knock"
	case sim.MoveMeld:
		return fmt.Sprintf("Meld %s", formatCards(m.Cards))
	case sim.MoveDiscard:
		return fmt.Sprintf("Discard %s", formatCards(m.Cards))
	case sim.MoveCapture:
		// Casino: Cards[0] is the card played from hand, Cards[1:] are the table
		// cards it captures. Render both so a human can see what each option takes.
		if len(m.Cards) > 1 {
			return fmt.Sprintf("Play %s to capture %s", m.Cards[0], formatCards(m.Cards[1:]))
		}
		return fmt.Sprintf("Play %s (capture)", formatCards(m.Cards))
	default:
		return "Unknown"
	}
}

// ResultsFile is the append-only human-ratings log. It is the same file the
// v1 Python playtest wrote, so v1 and v2 session records accumulate together.
const ResultsFile = "playtest_results.jsonl"

// Record is one line of playtest_results.jsonl (the plan's v2 schema, audit
// Task 24). Field names match the v1 writer where the two schemas overlap
// (timestamp, genome_id, genome_path, difficulty, seed, winner, turns,
// rating, comment); the v2 "stuck" boolean replaces v1's stuck_reason string.
// Rating is a pointer so a skipped prompt serializes as JSON null, exactly
// like v1's unrated sessions.
type Record struct {
	Timestamp  string `json:"timestamp"`
	GenomeID   string `json:"genome_id"`
	GenomePath string `json:"genome_path"`
	Difficulty string `json:"difficulty"`
	Seed       uint64 `json:"seed"`
	Winner     string `json:"winner"`
	Turns      int    `json:"turns"`
	Rating     *int   `json:"rating"`
	Comment    string `json:"comment"`
	Stuck      bool   `json:"stuck"`
}

// PromptRating asks for a 1-5 rating and a free-text comment on scanner,
// writing prompts to w. Empty input skips the rating (nil = null in the
// record); EOF skips everything silently so piped/non-interactive sessions
// never block or error. Out-of-range or non-numeric input re-prompts.
func PromptRating(scanner *bufio.Scanner, w io.Writer) (*int, string) {
	var rating *int
	for {
		fmt.Fprint(w, "Rate this game 1-5 (Enter to skip): ")
		if !scanner.Scan() {
			return nil, "" // EOF: non-interactive input exhausted
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			break // skipped: null rating
		}
		n, err := strconv.Atoi(input)
		if err != nil || n < 1 || n > 5 {
			fmt.Fprintln(w, "Enter a number 1-5, or press Enter to skip.")
			continue
		}
		rating = &n
		break
	}

	fmt.Fprint(w, "Comment (Enter to skip): ")
	if !scanner.Scan() {
		return rating, ""
	}
	return rating, strings.TrimSpace(scanner.Text())
}

// AppendRecord appends rec as one JSONL line to path, creating the file if
// needed. Append-only by construction: existing (v1 or v2) records are never
// rewritten.
func AppendRecord(path string, rec Record) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = f.Write(data)
	return err
}

func describeEvent(e sim.Event) string {
	switch e.Type {
	case sim.EventSpecialTriggered:
		return fmt.Sprintf("Special: %s", e.Detail)
	case sim.EventTrickWon:
		return fmt.Sprintf("Player %d wins the trick", e.PlayerID)
	case sim.EventMeldLaid:
		return fmt.Sprintf("Player %d melds %s", e.PlayerID, formatCards(e.Cards))
	case sim.EventRoundEnd:
		return fmt.Sprintf("Round ends: %s", e.Detail)
	default:
		return e.Detail
	}
}

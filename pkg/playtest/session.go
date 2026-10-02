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
	// Out is where the session prints (nil = os.Stdout). Tests point it at a
	// buffer to assert on what the player is shown.
	Out io.Writer
}

// out returns the session's output stream.
func (s *Session) out() io.Writer {
	if s.Out != nil {
		return s.Out
	}
	return os.Stdout
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

	fmt.Fprintf(s.out(), "\n=== %s ===\n", gameName(s.Genome))
	fmt.Fprintf(s.out(), "Skeleton: %s | Players: %d | Hand: %d cards\n\n",
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
			fmt.Fprintf(s.out(), "\nGame ended at max turns (%d).\n", maxTurns)
			return Outcome{Winner: -1, Turns: s.State.Turn}
		}

		// A blocked shedding game (deck exhausted, nobody can play) is a draw
		// by the rulebook. The runner itself only ever passes there until the
		// turn cap (see shedding.Runner.Blocked for why it must not name a
		// winner); a human should not have to sit through that.
		if bd, ok := s.Runner.(blockedDetector); ok && bd.Blocked(s.State, s.Genome) {
			fmt.Fprintln(s.out(), "\nThe deck is exhausted and no player can play: the game is blocked and ends in a draw.")
			return Outcome{Winner: -1, Turns: s.State.Turn}
		}

		moves := s.Runner.GenerateMoves(s.State, s.Genome)
		if len(moves) == 0 {
			fmt.Fprintln(s.out(), "No legal moves — game stuck!")
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
	fmt.Fprintf(s.out(), "\n--- Turn %d (You) ---\n", s.State.Turn+1)
	s.printState()

	fmt.Fprintln(s.out(), "\nLegal moves:")
	for i, m := range moves {
		fmt.Fprintf(s.out(), "  %d) %s\n", i+1, describeMoveShort(m))
	}

	choice := s.getChoice(len(moves))
	move := moves[choice]
	events := s.Runner.ApplyMove(s.State, move, s.Genome)
	s.afterMove(events)
	for _, e := range events {
		if e.Detail != "" {
			fmt.Fprintf(s.out(), "  > %s\n", describeEvent(e))
		}
	}
}

func (s *Session) aiTurn(moves []sim.Move) int {
	actor := s.State.Active
	move := s.AI.SelectMove(moves, s.State, s.RNG)
	events := s.Runner.ApplyMove(s.State, move, s.Genome)
	s.afterMove(events)

	fmt.Fprintf(s.out(), "  Player %d: %s", actor, describeMoveShort(move))
	for _, e := range events {
		if e.Type == sim.EventSpecialTriggered {
			fmt.Fprintf(s.out(), " [%s]", e.Detail)
		}
	}
	fmt.Fprintln(s.out())
	return actor
}

// blockedDetector is implemented by runners that can recognize a permanently
// blocked position (the shedding runner's all-pass deadlock).
type blockedDetector interface {
	Blocked(state *sim.GameState, g *genome.Genome) bool
}

var suitLongNames = [4]string{"Clubs", "Diamonds", "Hearts", "Spades"}

// seatName labels a seat from the human's point of view.
func (s *Session) seatName(p int) string {
	if p == s.HumanID {
		return "You"
	}
	return fmt.Sprintf("Player %d", p)
}

// capturesShown reports whether per-player captured piles (state.Tableau) are
// part of this game's public state: trick-taking tricks and casino captures.
// (A shedding tableau is only the trick-scoring borrow's shed tally.)
func (s *Session) capturesShown() bool {
	return s.Genome.Skeleton == genome.TrickTaking || s.Genome.Skeleton == genome.Casino
}

func (s *Session) capturedCount(p int) int {
	if p < len(s.State.Tableau) {
		return len(s.State.Tableau[p])
	}
	return 0
}

// printState shows the human everything public that bears on their move. Each
// skeleton keeps different things on the table, so the middle block is
// per-skeleton: the trump suit and the trick in progress (trick-taking), the
// combination to beat (climbing), the melds (rummy), the table and captured
// piles (casino), the pot (vying).
func (s *Session) printState() {
	st, w := s.State, s.out()
	fmt.Fprintf(w, "Your hand: %s\n", formatCards(st.Hands[s.HumanID]))
	if st.MaxRound > 1 {
		fmt.Fprintf(w, "Round %d of %d\n", st.Round+1, st.MaxRound)
	}

	switch s.Genome.Skeleton {
	case genome.TrickTaking:
		switch {
		case st.TrumpSuit >= 0 && st.TrumpSuit < len(suitLongNames):
			fmt.Fprintf(w, "Trump: %s\n", suitLongNames[st.TrumpSuit])
		case st.TrumpSuit == -2:
			fmt.Fprintln(w, "Trump: not set yet (the first suit led becomes trump)")
		default:
			fmt.Fprintln(w, "Trump: none")
		}
		if len(st.TrickCards) == 0 {
			fmt.Fprintln(w, "Current trick: (you lead)")
		} else {
			parts := make([]string, len(st.TrickCards))
			for i, c := range st.TrickCards {
				who := "?"
				if i < len(st.TrickPlayers) {
					who = s.seatName(st.TrickPlayers[i])
				}
				parts[i] = fmt.Sprintf("%s: %s", who, c)
			}
			fmt.Fprintf(w, "Current trick: %s\n", strings.Join(parts, ", "))
		}

	case genome.Climbing:
		if len(st.TrickCards) == 0 {
			fmt.Fprintln(w, "Table is clear: you lead any combination")
		} else {
			passes := "no passes since"
			if st.PassCount == 1 {
				passes = "1 pass since"
			} else if st.PassCount > 1 {
				passes = fmt.Sprintf("%d passes since", st.PassCount)
			}
			fmt.Fprintf(w, "To beat: %s (played by %s; %s)\n", formatCards(st.TrickCards), s.seatName(st.TrickLeader), passes)
		}
		if len(st.Deck) > 0 {
			fmt.Fprintf(w, "Deck: %d cards\n", len(st.Deck))
		}

	case genome.Vying:
		// The betting state is the whole decision surface -- without it a
		// human cannot price a call or a fold. Scores hold the chip stacks.
		owed := st.CurrentBet
		if s.HumanID < len(st.Committed) {
			owed = st.CurrentBet - st.Committed[s.HumanID]
			if owed < 0 {
				owed = 0
			}
		}
		fmt.Fprintf(w, "Pot: %d | Current bet: %d | To call: %d | Your chips: %d\n",
			st.Pot, st.CurrentBet, owed, st.Scores[s.HumanID])

	default: // shedding, rummy, casino: a deck and a shared face-up pile
		if st.TopCard != nil {
			fmt.Fprintf(w, "Top card: %s\n", st.TopCard)
		}
		// Show the shared pile's actual cards, not just a count: for casino
		// this is the face-up TABLE you capture from (you cannot choose a
		// capture without seeing it); elsewhere it is the discard pile. Cap
		// very long piles to the recent tail.
		pileLabel := "Discard"
		if s.Genome.Skeleton == genome.Casino {
			pileLabel = "Table"
		}
		switch disc, n := st.Discard, len(st.Discard); {
		case n == 0:
			fmt.Fprintf(w, "Deck: %d cards | %s: (empty)\n", len(st.Deck), pileLabel)
		case n <= 16:
			fmt.Fprintf(w, "Deck: %d cards | %s: %s\n", len(st.Deck), pileLabel, formatCards(disc))
		default:
			fmt.Fprintf(w, "Deck: %d cards | %s: %d cards (recent: %s)\n",
				len(st.Deck), pileLabel, n, formatCards(disc[n-16:]))
		}
		// Rummy: melds laid on the table, by owner.
		if len(st.Melds) > 0 {
			fmt.Fprintln(w, "Melds on the table:")
			for i, meld := range st.Melds {
				owner := "?"
				if i < len(st.MeldOwner) {
					owner = s.seatName(st.MeldOwner[i])
				}
				fmt.Fprintf(w, "  %s: %s\n", owner, formatCards(meld))
			}
		}
	}

	for i := 0; i < st.NumPlayers; i++ {
		if i == s.HumanID {
			continue
		}
		fmt.Fprintf(w, "Player %d: %d cards", i, len(st.Hands[i]))
		if s.capturesShown() {
			fmt.Fprintf(w, ", captured %d", s.capturedCount(i))
		}
		if st.Scores[i] != 0 {
			fmt.Fprintf(w, " (score: %d)", st.Scores[i])
		}
		if i < len(st.Folded) && st.Folded[i] {
			fmt.Fprintf(w, " (folded)")
		}
		fmt.Fprintln(w)
	}

	if s.capturesShown() {
		fmt.Fprintf(w, "You have captured %d cards\n", s.capturedCount(s.HumanID))
	}
	if st.Scores[s.HumanID] != 0 && s.Genome.Skeleton != genome.Vying {
		fmt.Fprintf(w, "Your score: %d\n", st.Scores[s.HumanID])
	}
}

// printFinalState reports the result in the terms the game is decided by:
// captured cards for casino (plus the banked bonus when a scoring borrow is
// live), points and captures for trick-taking, chips for vying, points and
// cards left in hand elsewhere.
func (s *Session) printFinalState(winner int) {
	st, w := s.State, s.out()
	fmt.Fprintf(w, "\n=== Game Over (turn %d) ===\n", st.Turn)
	if winner == s.HumanID {
		fmt.Fprintln(w, "You win!")
	} else {
		fmt.Fprintf(w, "Player %d wins!\n", winner)
	}

	fmt.Fprintln(w, "\nFinal scores:")
	for i := 0; i < st.NumPlayers; i++ {
		label := s.seatName(i)
		switch s.Genome.Skeleton {
		case genome.Casino:
			if s.Genome.CasinoScored() {
				fmt.Fprintf(w, "  %s: %d cards captured %+d bonus = %d\n",
					label, s.capturedCount(i), st.Scores[i], s.capturedCount(i)+st.Scores[i])
			} else {
				fmt.Fprintf(w, "  %s: %d cards captured\n", label, s.capturedCount(i))
			}
		case genome.TrickTaking:
			fmt.Fprintf(w, "  %s: %d points, %d cards captured\n", label, st.Scores[i], s.capturedCount(i))
		case genome.Vying:
			fmt.Fprintf(w, "  %s: %d chips\n", label, st.Scores[i])
		default:
			fmt.Fprintf(w, "  %s: %d points, %d cards remaining\n",
				label, st.Scores[i], len(st.Hands[i]))
		}
	}
}

func (s *Session) getChoice(numMoves int) int {
	for {
		fmt.Fprintf(s.out(), "Choose (1-%d): ", numMoves)
		if !s.Scanner.Scan() {
			fmt.Fprintln(s.out(), "\nGoodbye!")
			os.Exit(0)
		}
		input := strings.TrimSpace(s.Scanner.Text())
		if input == "q" || input == "quit" {
			fmt.Fprintln(s.out(), "Quitting...")
			os.Exit(0)
		}
		n, err := strconv.Atoi(input)
		if err != nil || n < 1 || n > numMoves {
			fmt.Fprintf(s.out(), "Invalid choice. Enter 1-%d or 'q' to quit.\n", numMoves)
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
		// A rummy knock is "discard X and knock": the move carries the discard
		// (the kept hand is what gets scored), and the player must see which
		// card goes. Shedding/climbing knocks carry no card.
		if len(m.Cards) > 0 {
			return fmt.Sprintf("Discard %s and knock", formatCards(m.Cards[:1]))
		}
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
	// Vying betting moves. Labels are static (no amounts): this function is
	// also called AFTER ApplyMove for the AI narration, where the bet state
	// has already advanced -- the pot/current-bet line in printState is where
	// the human reads what a call costs.
	case sim.MoveCheck:
		return "Check"
	case sim.MoveCall:
		return "Call"
	case sim.MoveRaise:
		return "Raise"
	case sim.MoveFold:
		return "Fold"
	case sim.MoveBid:
		return fmt.Sprintf("Bid %d", m.Amount)
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

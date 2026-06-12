package playtest

import (
	"bufio"
	"fmt"
	"math/rand/v2"
	"os"
	"strconv"
	"strings"

	"github.com/darwindeck/darwindeck/pkg/genome"
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
	}
}

// Run plays the game interactively.
func (s *Session) Run() {
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
			return
		}
		if s.State.Turn >= maxTurns {
			fmt.Printf("\nGame ended at max turns (%d).\n", maxTurns)
			return
		}

		moves := s.Runner.GenerateMoves(s.State, s.Genome)
		if len(moves) == 0 {
			fmt.Println("No legal moves — game stuck!")
			return
		}

		if s.State.Active == s.HumanID {
			s.humanTurn(moves)
		} else {
			s.aiTurn(moves)
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

	fmt.Printf("Deck: %d cards | Discard: %d cards\n",
		len(s.State.Deck), len(s.State.Discard))

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
	default:
		return "Unknown"
	}
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

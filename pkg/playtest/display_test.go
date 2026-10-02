package playtest

import (
	"bufio"
	"bytes"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
	"github.com/darwindeck/darwindeck/pkg/sim"
	"github.com/darwindeck/darwindeck/pkg/skeleton/shedding"
)

// The human must be shown everything the rules make public and that decides
// their move: the trump suit, the cards on the table, the melds, who has
// captured what. The session used to print only hands sizes and the discard
// pile -- a Whist player could not see trump, and a Casino game ended "Player 1
// wins!" with every seat at "0 points, 0 cards remaining".

func displaySession(g *genome.Genome, st *sim.GameState) (*Session, *bytes.Buffer) {
	var buf bytes.Buffer
	return &Session{Genome: g, State: st, HumanID: 0, Out: &buf}, &buf
}

func mustContain(t *testing.T, out string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(out, w) {
			t.Errorf("display must show %q; got:\n%s", w, out)
		}
	}
}

func TestPrintStateShowsTrumpAndTrick(t *testing.T) {
	g := seeds.Whist()
	st := sim.NewGameState(4)
	st.Hands[0] = []sim.Card{{Suit: sim.Hearts, Rank: 4}, {Suit: sim.Spades, Rank: sim.King}}
	for p := 1; p < 4; p++ {
		st.Hands[p] = []sim.Card{{Suit: sim.Clubs, Rank: sim.Rank(2 + p)}}
	}
	st.TrumpSuit = int(sim.Spades)
	st.TrickCards = []sim.Card{{Suit: sim.Hearts, Rank: sim.Ace}, {Suit: sim.Hearts, Rank: 10}}
	st.TrickPlayers = []int{2, 3}
	st.Tableau[2] = make([]sim.Card, 8)
	st.Scores[2] = 2

	s, buf := displaySession(g, st)
	s.printState()
	out := buf.String()
	mustContain(t, out, "Trump: Spades", "Current trick: Player 2: AH, Player 3: 10H", "Player 2: 1 cards", "captured 8")
	if strings.Contains(out, "Discard: (empty)") {
		t.Errorf("a trick-taking game has no discard pile to show:\n%s", out)
	}

	// No trump, nothing played yet: say so rather than print nothing.
	h := seeds.Hearts()
	st.TrumpSuit = -1
	st.TrickCards, st.TrickPlayers = nil, nil
	s, buf = displaySession(h, st)
	s.printState()
	mustContain(t, buf.String(), "Trump: none", "Current trick: (you lead)")

	// "First suit led" trump that has not been set yet.
	led := seeds.Whist()
	led.TrumpRule = genome.TrumpLed
	st.TrumpSuit = -2
	s, buf = displaySession(led, st)
	s.printState()
	mustContain(t, buf.String(), "Trump: not set yet")
}

func TestPrintStateShowsClimbingTable(t *testing.T) {
	g := seeds.BigTwo()
	st := sim.NewGameState(4)
	st.Hands[0] = []sim.Card{{Suit: sim.Hearts, Rank: 4}, {Suit: sim.Spades, Rank: sim.King}}
	for p := 1; p < 4; p++ {
		st.Hands[p] = []sim.Card{{Suit: sim.Clubs, Rank: sim.Rank(2 + p)}}
	}
	st.TrickCards = []sim.Card{{Suit: sim.Hearts, Rank: 9}, {Suit: sim.Clubs, Rank: 9}}
	st.TrickLeader = 3
	st.PassCount = 1

	s, buf := displaySession(g, st)
	s.printState()
	mustContain(t, buf.String(), "To beat: 9H 9C (played by Player 3", "1 pass")

	st.TrickCards = nil
	st.PassCount = 0
	s, buf = displaySession(g, st)
	s.printState()
	mustContain(t, buf.String(), "Table is clear: you lead")
}

func TestPrintStateShowsMeldsAndCaptures(t *testing.T) {
	// Rummy: melds on the table, by owner.
	g := seeds.GinRummy()
	st := sim.NewGameState(2)
	st.Hands[0] = []sim.Card{{Suit: sim.Hearts, Rank: 4}}
	st.Hands[1] = []sim.Card{{Suit: sim.Clubs, Rank: 5}}
	st.Discard = []sim.Card{{Suit: sim.Spades, Rank: 2}}
	st.Melds = [][]sim.Card{
		{{Suit: sim.Clubs, Rank: 7}, {Suit: sim.Diamonds, Rank: 7}, {Suit: sim.Spades, Rank: 7}},
		{{Suit: sim.Hearts, Rank: 9}, {Suit: sim.Hearts, Rank: 10}, {Suit: sim.Hearts, Rank: sim.Jack}},
	}
	st.MeldOwner = []int{1, 0}
	st.Phase = sim.PhaseMeld
	s, buf := displaySession(g, st)
	s.printState()
	mustContain(t, buf.String(), "Melds on the table:", "Player 1: 7C 7D 7S", "You: 9H 10H JH")

	// Casino: the table, and every seat's captured pile.
	c := seeds.Casino()
	st = sim.NewGameState(2)
	st.Hands[0] = []sim.Card{{Suit: sim.Hearts, Rank: 4}}
	st.Hands[1] = []sim.Card{{Suit: sim.Clubs, Rank: 5}}
	st.Discard = []sim.Card{{Suit: sim.Spades, Rank: 2}, {Suit: sim.Diamonds, Rank: 9}}
	st.Tableau[0] = make([]sim.Card, 6)
	st.Tableau[1] = make([]sim.Card, 11)
	s, buf = displaySession(c, st)
	s.printState()
	mustContain(t, buf.String(), "Table: 2S 9D", "You have captured 6 cards", "Player 1: 1 cards, captured 11")
}

func TestFinalStateShowsCasinoCaptures(t *testing.T) {
	c := seeds.Casino()
	st := sim.NewGameState(2)
	st.Tableau[0] = make([]sim.Card, 22)
	st.Tableau[1] = make([]sim.Card, 30)
	s, buf := displaySession(c, st)
	s.printFinalState(1)
	mustContain(t, buf.String(), "Player 1 wins!", "You: 22 cards captured", "Player 1: 30 cards captured")

	// Trick-taking reports tricks' worth of captures alongside the score.
	w := seeds.Whist()
	st = sim.NewGameState(4)
	st.Scores = []int{4, 3, 5, 1}
	for p, n := range []int{16, 12, 20, 4} {
		st.Tableau[p] = make([]sim.Card, n)
	}
	s, buf = displaySession(w, st)
	s.printFinalState(2)
	mustContain(t, buf.String(), "You: 4 points, 16 cards captured", "Player 2: 5 points, 20 cards captured")
}

// blockedRunner is the real shedding runner started from a hand-built blocked
// position.
type blockedRunner struct {
	*shedding.Runner
	start *sim.GameState
}

func (b blockedRunner) Setup(g *genome.Genome, rng *rand.Rand) *sim.GameState { return b.start }

// TestSessionDeclaresBlockedGameADraw: with the deck exhausted and nobody able
// to play, the shedding rulebook says the game is a draw. The session used to
// make the human pass back and forth until the turn cap; it now ends at once
// with no winner.
func TestSessionDeclaresBlockedGameADraw(t *testing.T) {
	g := &genome.Genome{
		ID: "blocked", Skeleton: genome.Shedding, Players: 2, HandSize: 7,
		Shedding: &genome.SheddingParams{MatchRule: genome.MatchEither, DrawPenalty: 1},
	}
	st := sim.NewGameState(2)
	st.Hands[0] = []sim.Card{{Suit: sim.Spades, Rank: 2}, {Suit: sim.Clubs, Rank: 4}, {Suit: sim.Spades, Rank: 6}, {Suit: sim.Clubs, Rank: 7}}
	st.Hands[1] = []sim.Card{{Suit: sim.Spades, Rank: 3}, {Suit: sim.Diamonds, Rank: 5}, {Suit: sim.Clubs, Rank: 8}, {Suit: sim.Diamonds, Rank: 10}}
	top := sim.Card{Suit: sim.Hearts, Rank: 9}
	st.Discard = []sim.Card{top}
	st.TopCard = &top
	st.Direction = 1

	var buf bytes.Buffer
	s := &Session{
		Genome:  g,
		Runner:  blockedRunner{Runner: &shedding.Runner{}, start: st},
		AI:      &sim.RandomAI{},
		RNG:     rand.New(rand.NewPCG(1, 1)),
		Scanner: bufio.NewScanner(strings.NewReader("")), // no input: a prompt would exit the process
		HumanID: 0,
		Out:     &buf,
	}
	out := s.Run()
	if out.Winner != -1 || out.Stuck {
		t.Errorf("blocked game outcome = %+v, want a draw (Winner -1, not stuck)", out)
	}
	if out.Turns != 0 {
		t.Errorf("the draw must be declared immediately, not after %d turns of passing", out.Turns)
	}
	mustContain(t, buf.String(), "blocked", "draw")
	if got := out.WinnerLabel(0); got != "none" {
		t.Errorf("WinnerLabel = %q, want \"none\"", got)
	}
}

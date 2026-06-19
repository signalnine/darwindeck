package webplay

import (
	"fmt"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

// logTail is how many narration lines the view ships to the client.
const logTail = 14

// MoveView is one legal move, presented for a button. The client submits Index;
// the server resolves it against its canonical move list (submitMove), so a
// move is never reconstructed from client JSON.
type MoveView struct {
	Index int      `json:"index"`
	Label string   `json:"label"`
	Type  string   `json:"type"`
	Cards []string `json:"cards"`
}

// OppView is one opponent seat as the human sees it (hand count, not contents).
type OppView struct {
	Seat      int  `json:"seat"`
	HandCount int  `json:"handCount"`
	Score     int  `json:"score"`
	Folded    bool `json:"folded"`
	Committed int  `json:"committed,omitempty"`
}

// TableView is the shared/table state, filled per-skeleton (omitempty hides the
// fields a given family doesn't use).
type TableView struct {
	DeckCount    int        `json:"deckCount"`
	DiscardCount int        `json:"discardCount"`
	TopCard      string     `json:"topCard,omitempty"`
	Trick        []string   `json:"trick,omitempty"`
	TrumpSuit    string     `json:"trumpSuit,omitempty"`
	Pot          int        `json:"pot,omitempty"`
	CurrentBet   int        `json:"currentBet,omitempty"`
	Melds        [][]string `json:"melds,omitempty"`
}

// View is the full client-facing snapshot returned by every game endpoint.
type View struct {
	Session     string     `json:"session"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	Skeleton    string     `json:"skeleton"`
	Status      string     `json:"status"`
	Turn        int        `json:"turn"`
	YourSeat    int        `json:"yourSeat"`
	YourHand    []string   `json:"yourHand"`
	YourScore   int        `json:"yourScore"`
	Opponents   []OppView  `json:"opponents"`
	Table       TableView  `json:"table"`
	LegalMoves  []MoveView `json:"legalMoves"`
	Log         []string   `json:"log"`
	Winner      int        `json:"winner"`
	WinnerLabel string     `json:"winnerLabel"`
	Rules       string     `json:"rules,omitempty"`
}

// view builds the client snapshot. Caller holds mu. includeRules sends the
// (large) rules markdown only when it changes (new game / explicit refresh).
func (ws *WebSession) view(includeRules bool) View {
	st := ws.state
	v := View{
		Session:     ws.ID,
		Title:       gameTitle(ws.Genome),
		Description: ws.Genome.Description,
		Skeleton:    ws.Genome.Skeleton.String(),
		Status:      ws.status,
		Turn:        st.Turn,
		YourSeat:    HumanSeat,
		YourHand:    cardStrings(st.Hands[HumanSeat]),
		YourScore:   scoreAt(st.Scores, HumanSeat),
		Winner:      -1,
		Table:       tableView(st, ws.Genome),
		Log:         tail(ws.log, logTail),
		LegalMoves:  []MoveView{},
		Opponents:   []OppView{},
	}

	for seat := 0; seat < st.NumPlayers; seat++ {
		if seat == HumanSeat {
			continue
		}
		opp := OppView{
			Seat:      seat,
			HandCount: len(st.Hands[seat]),
			Score:     scoreAt(st.Scores, seat),
		}
		if seat < len(st.Folded) {
			opp.Folded = st.Folded[seat]
		}
		if seat < len(st.Committed) {
			opp.Committed = st.Committed[seat]
		}
		v.Opponents = append(v.Opponents, opp)
	}

	if ws.status == StatusHumanTurn {
		for i, mv := range ws.legalMoves {
			v.LegalMoves = append(v.LegalMoves, MoveView{
				Index: i,
				Label: moveLabel(mv, st, ws.Genome),
				Type:  moveTypeName(mv.Type),
				Cards: cardStrings(mv.Cards),
			})
		}
	}

	if ws.status == StatusGameOver {
		v.Winner = ws.winner
		switch {
		case ws.winner == HumanSeat:
			v.WinnerLabel = "You win!"
		case ws.winner < 0:
			v.WinnerLabel = "No winner (turn limit)."
		default:
			v.WinnerLabel = fmt.Sprintf("Player %d wins.", ws.winner)
		}
	} else if ws.status == StatusStuck {
		v.WinnerLabel = "Game stuck (no legal moves)."
	}

	if includeRules {
		v.Rules = ws.rules
	}
	return v
}

func tableView(st *sim.GameState, g *genome.Genome) TableView {
	t := TableView{
		DeckCount:    len(st.Deck),
		DiscardCount: len(st.Discard),
	}
	if st.TopCard != nil {
		t.TopCard = st.TopCard.String()
	}
	if len(st.TrickCards) > 0 {
		t.Trick = cardStrings(st.TrickCards)
	}
	// Trump only means something in trick-taking; other skeletons leave
	// TrumpSuit at its zero value (0 = Clubs), which would otherwise render a
	// phantom trump tag for poker/shedding/etc.
	if g.Skeleton == genome.TrickTaking && st.TrumpSuit >= 0 && st.TrumpSuit <= 3 {
		t.TrumpSuit = sim.Suit(st.TrumpSuit).String()
	}
	t.Pot = st.Pot
	t.CurrentBet = st.CurrentBet
	for _, m := range st.Melds {
		t.Melds = append(t.Melds, cardStrings(m))
	}
	return t
}

// moveLabel renders a move as a human-readable button label. Covers every
// MoveType across the 6 skeletons; vying betting amounts are read from state
// (they live there, not on the move).
func moveLabel(mv sim.Move, st *sim.GameState, g *genome.Genome) string {
	switch mv.Type {
	case sim.MovePlay:
		return "Play " + cardList(mv.Cards)
	case sim.MoveDraw:
		if len(mv.Cards) > 0 {
			return "Take " + cardList(mv.Cards) + " from discard"
		}
		return "Draw from deck"
	case sim.MovePass:
		return "Pass"
	case sim.MoveKnock:
		return "Knock"
	case sim.MoveMeld:
		return "Meld " + cardList(mv.Cards)
	case sim.MoveDiscard:
		return "Discard " + cardList(mv.Cards)
	case sim.MoveCapture:
		if len(mv.Cards) > 1 {
			return fmt.Sprintf("Capture %s with %s", cardList(mv.Cards[1:]), mv.Cards[0].String())
		}
		return "Capture " + cardList(mv.Cards)
	case sim.MoveCheck:
		return "Check"
	case sim.MoveCall:
		if owed := amountOwed(st); owed > 0 {
			return fmt.Sprintf("Call %d", owed)
		}
		return "Call"
	case sim.MoveRaise:
		if g.Vying != nil {
			return fmt.Sprintf("Raise %d", g.Vying.MinBet)
		}
		return "Raise"
	case sim.MoveFold:
		return "Fold"
	default:
		return "Move"
	}
}

// amountOwed is the chips the active vying seat must add to call.
func amountOwed(st *sim.GameState) int {
	seat := st.Active
	if seat < 0 || seat >= len(st.Committed) {
		return st.CurrentBet
	}
	owed := st.CurrentBet - st.Committed[seat]
	if owed < 0 {
		return 0
	}
	return owed
}

func moveTypeName(t sim.MoveType) string {
	switch t {
	case sim.MovePlay:
		return "play"
	case sim.MoveDraw:
		return "draw"
	case sim.MovePass:
		return "pass"
	case sim.MoveKnock:
		return "knock"
	case sim.MoveMeld:
		return "meld"
	case sim.MoveDiscard:
		return "discard"
	case sim.MoveCapture:
		return "capture"
	case sim.MoveCheck:
		return "check"
	case sim.MoveCall:
		return "call"
	case sim.MoveRaise:
		return "raise"
	case sim.MoveFold:
		return "fold"
	default:
		return "move"
	}
}

func cardStrings(cards []sim.Card) []string {
	out := make([]string, len(cards))
	for i, c := range cards {
		out[i] = c.String()
	}
	return out
}

func cardList(cards []sim.Card) string {
	if len(cards) == 0 {
		return "(none)"
	}
	s := ""
	for i, c := range cards {
		if i > 0 {
			s += " "
		}
		s += c.String()
	}
	return s
}

func gameTitle(g *genome.Genome) string {
	if g.ID != "" {
		return g.ID
	}
	return "Evolved " + g.Skeleton.String() + " Game"
}

func scoreAt(scores []int, seat int) int {
	if seat >= 0 && seat < len(scores) {
		return scores[seat]
	}
	return 0
}

func tail(s []string, n int) []string {
	if len(s) <= n {
		out := make([]string, len(s))
		copy(out, s)
		return out
	}
	out := make([]string, n)
	copy(out, s[len(s)-n:])
	return out
}

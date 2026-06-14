package judge

import (
	"fmt"
	"strings"

	"github.com/darwindeck/darwindeck/pkg/sim"
)

// neutralizeRulebook rewrites the rummy "Knock"/"Gin" mechanic vocabulary to
// neutral going-out phrasing, and strips the climbing rulebook's game-name
// parenthetical ("Big Two / Tichu / President family"). These tokens are
// emitted identically for every genome of their skeleton by
// output.GenerateRulebook, so this does not erase any distinguishing signal --
// it only removes a game-name collision (Gin Rummy; Big Two/Tichu/President)
// from the blind text. Suit references (Hearts, Spades, etc.) and the
// card-attribute word "rank" are left intact: those are legitimate mechanical
// card text, not identity or metric leaks. The shared output package is never
// touched; this rewrite lives entirely in the judge tool.
func neutralizeRulebook(s string) string {
	repl := strings.NewReplacer(
		"### Knocking & Gin", "### Going Out",
		"You may **knock** when", "You may **declare out** when",
		"You can only go out with **Gin** (no deadwood at all).",
		"You can only go out by reaching **zero deadwood** (no leftover cards).",
		"when someone knocks or goes gin wins", "when someone declares out or reaches zero deadwood wins",
		"Knock to end the round early", "Declare out to end the round early",
		// Climbing intro: drop the named-game family AND the skeleton keyword so
		// neither the game names (Big Two/Tichu/President) nor "climbing" leak,
		// while keeping the mechanic description accurate.
		"This is a **climbing game** (Big Two / Tichu / President family) — the goal is to be the first player to empty your hand.",
		"In this game you beat the current play with a stronger same-shape combination or pass — the goal is to be the first player to empty your hand.",
	)
	return repl.Replace(s)
}

// neutralizeDetail rewrites the rummy round-end reason strings ("knock"/"gin")
// to neutral going-out vocabulary in trace lines, for the same reason as
// neutralizeRulebook: these are mechanic details emitted for every rummy game,
// but the literal tokens collide with game names.
func neutralizeDetail(d string) string {
	switch d {
	case "knock":
		return "declared_out"
	case "gin":
		return "zero_deadwood"
	default:
		return d
	}
}

var eventNames = map[sim.EventType]string{
	sim.EventCardPlayed:       "PLAY",
	sim.EventCardDrawn:        "DRAW",
	sim.EventTrickWon:         "TRICK_WON",
	sim.EventMeldLaid:         "MELD",
	sim.EventSpecialTriggered: "SPECIAL",
	sim.EventRoundEnd:         "ROUND_END",
}

// renderTrace renders a flat event list as a numbered move list. Each event is
// numbered ("turn N: P{player} {EVENT} {cards} {detail}") so a reader can see
// turn-taking and spot long uninterrupted single-player runs (the degenerate
// "one player wins while the opponent barely acts" signal the rubric asks the
// judge to look for). The trace is capped to keep dossiers readable; a
// one-sided game still shows the imbalance well within the cap.
// renderTrace renders a game's event stream HEAD+TAIL bounded. The cap keeps a
// dossier inside a tight judge context budget (an uncapped long game overflowed
// a 4096-token window once the rubric was prepended), and showing the TAIL --
// not just the head -- lets the reader see how the game ENDS (who wins, whether
// one player monopolizes the close), which a head-only truncation hid.
func renderTrace(events []sim.Event) string {
	const headN, tailN = 20, 10
	var b strings.Builder
	b.WriteString("```\n")
	n := len(events)
	render := func(i int) {
		e := events[i]
		name := eventNames[e.Type]
		if name == "" {
			name = fmt.Sprintf("EVENT(%d)", int(e.Type))
		}
		cards := renderCards(e.Cards)
		detail := neutralizeDetail(e.Detail)
		line := fmt.Sprintf("turn %d: P%d %s", i+1, e.PlayerID, name)
		if cards != "" {
			line += " " + cards
		}
		if detail != "" {
			line += " " + detail
		}
		b.WriteString(line + "\n")
	}
	if n <= headN+tailN {
		for i := range events {
			render(i)
		}
	} else {
		for i := 0; i < headN; i++ {
			render(i)
		}
		b.WriteString(fmt.Sprintf("... (%d turns omitted) ...\n", n-headN-tailN))
		for i := n - tailN; i < n; i++ {
			render(i)
		}
	}
	b.WriteString("```\n")
	return b.String()
}

func renderCards(cards []sim.Card) string {
	if len(cards) == 0 {
		return ""
	}
	parts := make([]string, len(cards))
	for i, c := range cards {
		parts[i] = c.String()
	}
	return strings.Join(parts, ",")
}

package judge

import (
	"fmt"
	"strings"

	"github.com/darwindeck/darwindeck/pkg/sim"
)

// neutralizeRulebook rewrites the rummy "Knock"/"Gin" mechanic vocabulary to
// neutral going-out phrasing. These tokens are emitted identically for every
// rummy genome by output.GenerateRulebook, so this does not erase any
// distinguishing signal -- it only removes a game-name collision (Gin Rummy)
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
func renderTrace(events []sim.Event) string {
	const cap = 120
	var b strings.Builder
	b.WriteString("```\n")
	shown := events
	truncated := false
	if len(shown) > cap {
		shown = shown[:cap]
		truncated = true
	}
	for i, e := range shown {
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
	if truncated {
		b.WriteString(fmt.Sprintf("... (%d more events)\n", len(events)-cap))
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

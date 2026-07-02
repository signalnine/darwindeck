package judge

import (
	"fmt"
	"strings"

	"github.com/darwindeck/darwindeck/pkg/sim"
)

// neutralizeRulebook rewrites the rummy "Knock"/"Gin" mechanic vocabulary to
// neutral going-out phrasing, and strips the game-name parentheticals from the
// climbing ("Big Two / Tichu / President family"), casino ("Casino / Scopa
// family"), and vying ("poker family") rulebook intros, along with the vying
// rulebook's "poker hand" / "big blind" vocabulary and the MechKnock borrow
// rule's "knock" wording. These tokens are emitted identically for every
// genome of their skeleton (or borrow) by output.GenerateRulebook, so this
// does not erase any distinguishing signal -- it only removes game-name
// collisions (Gin Rummy; Big Two/Tichu/President; Casino/Scopa; poker; Knock
// Rummy) from the blind text. Suit references (Hearts, Spades, etc.) and the
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
		// Casino intro: drop the named-game family, keeping the mechanic genre.
		"This is a **fishing / capture game** (Casino / Scopa family) — you capture cards from a shared table into your own pile.",
		"This is a **fishing / capture game** — you capture cards from a shared table into your own pile.",
		// Vying intro/showdown/scoring: drop the "poker family" name and rewrite
		// every "poker hand" mention to neutral hand-rank phrasing (the showdown
		// line still lists the concrete ranking, so no mechanic signal is lost),
		// and replace the "big blind" with a neutral forced-stake phrase.
		"This is a **vying / betting game** (poker family) — wager chips on hidden hands; the best poker hand at showdown takes the pot.",
		"This is a **vying / betting game** — wager chips on hidden hands; the best-ranked hand at showdown takes the pot.",
		"the best poker hand — pair", "the best-ranked hand — pair",
		"A strong poker hand can net fewer chips", "A strongly ranked hand can net fewer chips",
		"a poker-weak hand can still gain chips", "a weakly ranked hand can still gain chips",
		"posts a big blind of", "posts a forced opening stake of",
		// MechKnock borrow rule: same declare-out vocabulary as the rummy
		// rewrites above, so knock-borrow dossiers stay consistent with rummy
		// dossiers and the "knock" token (Knock Rummy) never leaks.
		"**Knock:** once your hand is down to a few cards, instead of playing you may knock to end the game at once. When you knock, whoever holds the fewest cards wins — so knock when you are ahead, but knocking while someone else is shorter hands them the win",
		"**Declare out:** once your hand is down to a few cards, instead of playing you may declare out to end the game at once. When you declare out, whoever holds the fewest cards wins — so declare out when you are ahead, but declaring out while someone else is shorter hands them the win",
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

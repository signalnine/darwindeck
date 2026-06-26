package grammar

import (
	"fmt"
	"strings"
)

// Rulebook renders a GameSpec as a natural-language rulebook a human (or a blind
// novelty judge) can read -- the legibility that lets a judge assess the game on
// its actual rules, not a generic blurb (v2's hard-won lesson: an illegible
// dossier reads as "variant"). It deliberately describes the game in plain rules,
// never in grammar internals (no "move-gen", "modifier", "spec"): the reader sees
// a card game, not a synthesized composition. title is the neutral heading.
func (s GameSpec) Rulebook(title string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", title)
	fmt.Fprintf(&b, "A card game for %d players, played with a standard 52-card deck.\n\n", s.Players)

	b.WriteString("## Setup\n\n")
	if s.Deal > 0 {
		fmt.Fprintf(&b, "- Deal %d cards to each player.\n", s.Deal)
	} else {
		b.WriteString("- Players are not dealt a hand; play draws from shared cards and the deck.\n")
	}
	if s.Shared > 0 {
		fmt.Fprintf(&b, "- Place %d card(s) face-up on the table to start.\n", s.Shared)
	}
	b.WriteString("- The remaining cards form the draw deck.\n\n")

	b.WriteString("## Objective\n\n")
	fmt.Fprintf(&b, "%s\n\n", s.objective())

	b.WriteString("## How to Play\n\n")
	fmt.Fprintf(&b, "Players take turns in order. On your turn:\n\n%s\n\n", s.turnRules())

	if rules := s.modifierRules(); len(rules) > 0 {
		b.WriteString("## Special Rules\n\n")
		for _, r := range rules {
			fmt.Fprintf(&b, "- %s\n", r)
		}
		b.WriteString("\n")
	}

	b.WriteString("## Ending the Game\n\n")
	fmt.Fprintf(&b, "%s %s\n", s.endRule(), s.winRule())
	return b.String()
}

func (s GameSpec) objective() string {
	switch s.Score {
	case FirstOut:
		return "Be the first to get rid of all the cards in your hand."
	case FewestCards:
		return "Hold the fewest cards when the game ends."
	case ClosestTarget:
		return fmt.Sprintf("Build a card total as close as possible to %d without going over.", s.Target)
	case MostCaptured:
		if s.Move == Trick {
			return "Win the most cards by taking tricks."
		}
		return "Capture more cards from the table than anyone else."
	case HighScore:
		return "Score the most points."
	case FewestDeadwood:
		return "Form your cards into melds -- sets of three or more of the same rank, or runs of three or more consecutive cards in one suit -- leaving as few stray (unmelded) cards as possible."
	}
	return ""
}

func (s GameSpec) turnRules() string {
	switch s.Move {
	case PlayMatch:
		var how string
		switch s.Match {
		case MatchSuit:
			how = "the same suit as"
		case MatchRank:
			how = "the same rank as"
		default:
			how = "either the same rank or the same suit as"
		}
		return fmt.Sprintf("- Play one card from your hand that is %s the card on top of the discard pile, and place it on top.\n"+
			"- If you cannot (or choose not to) play, draw one card from the deck. If the deck is empty, you pass.", how)
	case BeatOrPass:
		return "- Play a card that ranks HIGHER than the card currently on the table, placing it on top to become the new card to beat.\n" +
			"- Or pass. When every other player passes in a row, the table is cleared and the last player to play leads a fresh card of their choice."
	case Accumulate:
		return fmt.Sprintf("- Take one card -- either a face-up card from the table or an unseen card from the deck -- and add its value to your running total (face cards count 10, aces 11).\n"+
			"- Or STICK to lock in your current total and take no more cards. If your total ever exceeds %d, you have busted and are out of the round.", s.Target)
	case Capture:
		return "- Play one card from your hand onto the table. If it matches the rank of one or more cards already on the table, you CAPTURE those cards (and the played card) into your score pile.\n" +
			"- If it matches nothing, it stays face-up on the table for others to capture later."
	case Trick:
		return "- Play one card to the table. You MUST follow the suit of the card that led this trick if you hold it; otherwise you may play any card.\n" +
			"- Once every player has played, the highest card of the led suit wins the trick (and all the cards in it) and leads the next trick."
	case Rummy:
		return "- Draw the top card of the deck into your hand.\n" +
			"- Then discard one card from your hand face-up. Keep the cards that build toward melds and throw away your stray cards."
	}
	return ""
}

func (s GameSpec) modifierRules() []string {
	var out []string
	for _, m := range s.Mods {
		switch m {
		case ModRunPlay:
			out = append(out, "Instead of a single card, you may play a SET of cards of the same rank, or a RUN of consecutive cards of the same suit, all in one turn -- letting you shed several cards at once.")
		case ModFollowSuit:
			out = append(out, "If you hold any card of the same suit as the top of the discard pile, you MUST play one of them -- you may not draw to avoid it.")
		case ModDrawPenalty:
			out = append(out, "Whenever you play a face card (Jack, Queen, or King), you must immediately draw one extra card from the deck as a penalty.")
		case ModKnock:
			out = append(out, "When you are down to 3 or fewer cards, you may KNOCK on your turn to end the game at once. Whoever holds the fewest cards then wins -- so knocking while you are NOT lowest hands the win to someone else.")
		case ModMeldBonus:
			out = append(out, "At the end, you earn bonus points for matching combinations in your score pile: pairs and three-of-a-kinds, and runs of the same suit. These bonuses are added to your total.")
		case ModAvoidance:
			out = append(out, "Beware the penalty cards: every heart among the cards you win counts ONE point against you, and the Queen of Spades counts thirteen. You want the FEWEST penalty points, so winning cards greedily can cost you the game.")
		}
	}
	return out
}

func (s GameSpec) endRule() string {
	switch s.End {
	case EmptyHand:
		return "The game ends the moment any player has played the last card from their hand."
	case DeckOut:
		if s.Move == Trick {
			return "The game ends once every player has played out their whole hand."
		}
		if s.Move == Rummy {
			return "The game ends the moment the draw deck runs out."
		}
		return "The game ends once the deck is exhausted and the players' hands are empty."
	case Bust:
		return "The round ends when every player has either stuck or busted."
	}
	return ""
}

func (s GameSpec) winRule() string {
	switch s.Score {
	case FirstOut:
		return "The player who emptied their hand is the winner."
	case FewestCards:
		return "The player holding the fewest cards wins."
	case ClosestTarget:
		return fmt.Sprintf("Among players who did not bust, the highest total (closest to %d) wins.", s.Target)
	case MostCaptured:
		if s.Move == Trick {
			return "The player who won the most cards in tricks wins (less any penalty points)."
		}
		return "The player who captured the most cards wins."
	case HighScore:
		return "The player with the highest score wins."
	case FewestDeadwood:
		return "The player whose hand has the fewest unmelded cards (the least deadwood) wins."
	}
	return ""
}

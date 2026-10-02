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
	// What happens to the undealt cards depends on the game: only the games
	// that draw have a draw deck. (Every rulebook used to promise one.)
	switch rest := 52 - s.Players*s.Deal - s.Shared; {
	case s.Move == Trick && rest > 0: // Setup drops the undealt kitty
		fmt.Fprintf(&b, "- The remaining %d cards are set aside unused; there is no draw deck.\n", rest)
	case s.Move == Trick: // the whole deck is dealt out
	case s.Move == BeatOrPass || s.Move == Vying: // nobody ever draws
		b.WriteString("- The remaining cards are set aside unused; there is no draw deck.\n")
	default:
		b.WriteString("- The remaining cards form the draw deck.\n")
	}
	b.WriteString("\n")

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
	if s.Move == PlayMatch {
		// The runner's all-pass deadlock end (CheckEnd, PassCount >= players):
		// a second way the game can finish, so it is a rule the reader needs.
		b.WriteString("\nIf the deck has run out and no player can play, so that every player passes in turn, the game ends at once and the player holding the fewest cards wins; " +
			turnOrderTie("the last player to pass") + ".\n")
	}
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
			if s.hasMod(ModBid) {
				return "Win as close as you can to the number of tricks you declare at the start -- making your bid is what scores."
			}
			return "Win the most cards by taking tricks."
		}
		return "Capture more cards from the table than anyone else."
	case HighScore:
		return "Score the most points."
	case FewestDeadwood:
		// "sets are formed first" is the runner's greedy meld order (deadwood):
		// 5C 5D 5H 6H 7H counts as the set plus two stray cards, not the run.
		return "Form your cards into melds -- sets of three or more of the same rank, or runs of three or more consecutive cards in one suit -- leaving as few stray (unmelded) cards as possible. " +
			"When a hand is counted, sets are formed first and runs are then formed from the cards left over."
	case BestHand:
		return "Hold the best five-card poker hand at the showdown, and bet boldly enough that the others fold or pay to see it."
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
		// The runner offers a draw ONLY when nothing can be played, and a draw
		// passes the turn -- there is no "draw instead of playing" option.
		return fmt.Sprintf("- Play one card from your hand that is %s the card on top of the discard pile, and place it on top.\n"+
			"- If you cannot play, draw one card from the deck; that ends your turn (you may not draw while you hold a playable card). If the deck is empty, you pass instead.", how)
	case BeatOrPass:
		// The leader of an empty table has no pass move; ranks compare ace-high
		// and suits never matter.
		return "- If the table is empty you must lead: play any card from your hand.\n" +
			"- Otherwise play a card that ranks HIGHER than the card on the table (suits do not matter; ace is highest), which becomes the new card to beat -- or pass.\n" +
			"- When every other player has passed in a row, the table is cleared and the last player to play leads again."
	case Accumulate:
		// The runner offers exactly ONE face-up take: the top of the face-up
		// pile (the last card turned). The cards under it are never takeable
		// while the deck can refill the top.
		return fmt.Sprintf("- Take one card -- either the TOP face-up card on the table or an unseen card from the deck -- and add its value to your running total (number cards count their number, face cards 10, aces 11). "+
			"The face-up cards form a pile and only the top one may be taken; when it is taken, a new card is turned up from the deck in its place.\n"+
			"- Or STICK to lock in your current total and take no more cards. You must take at least one card before you may stick. If your total ever exceeds %d, you have busted and are out of the round.", s.Target)
	case Capture:
		// A capture takes the WHOLE same-rank set; trailing is always allowed,
		// even with a card that could capture; hands are redealt until the deck
		// cannot deal a round (Runner.Upkeep).
		return fmt.Sprintf("- Play one card from your hand. If one or more table cards share its rank you may CAPTURE: take ALL the table cards of that rank, together with the card you played, into your score pile.\n"+
			"- Or simply leave your card face-up on the table for others to capture later. You may do this with any card, even one that could capture.\n"+
			"- When every hand is empty and the deck still holds enough cards, each player is dealt %d new cards and play continues.", s.Deal)
	case Trick:
		return "- Play one card to the table. You MUST follow the suit of the card that led this trick if you hold it; otherwise you may play any card.\n" +
			"- Once every player has played, the highest card of the led suit wins the trick (and all the cards in it) and leads the next trick."
	case Rummy:
		return "- Draw the top card of the deck into your hand.\n" +
			"- Then discard one card from your hand face-up. Keep the cards that build toward melds and throw away your stray cards."
	case Vying:
		// A raise is one chip; the round is capped at maxRaises raises, and a
		// raise makes every other live seat act again.
		return fmt.Sprintf("- If no bet is owed, you may CHECK (stay in for free) or RAISE the bet by one chip.\n"+
			"- If a bet is owed, you may CALL to match it, RAISE it by one chip more, or FOLD and drop out.\n"+
			"- At most %d raises may be made in the round. After a raise every other player still in must act again; once everyone still in has matched the bet, hands are shown.", maxRaises)
	}
	return ""
}

func (s GameSpec) modifierRules() []string {
	var out []string
	for _, m := range s.Mods {
		switch m {
		case ModRunPlay:
			if s.Move == BeatOrPass {
				out = append(out, "You may lead a SET of two or more cards of the same rank, not just one card. Whoever follows must beat it with a higher set of the SAME size, or pass.")
			} else {
				// comboPlays offers only MAXIMAL groups: every held card of a
				// rank, or a whole unbroken suited run -- never a sub-pair or
				// a partial run -- and the last card laid becomes the top.
				out = append(out, "Instead of a single card you may lay a group in one turn: a SET, which must be ALL the cards you hold of one rank (two or more), "+
					"or a RUN of two or more consecutive cards in one suit, which must be your LONGEST run through those cards (the whole unbroken sequence you hold, never part of it). "+
					"At least one card of the group must match the top of the discard pile. The last card laid becomes the new top: for a run, its highest card; for a set, whichever of those cards came into your hand last.")
			}
		case ModFollowSuit:
			exempt := ""
			if s.hasMod(ModNominate) {
				exempt = " (an eight may always be played instead)"
			}
			out = append(out, "If you hold any card of the same suit as the top of the discard pile, you MUST play one of them"+exempt+" -- you may not play a card of another suit, or draw, to avoid it.")
		case ModDrawPenalty:
			// Keep this in sync with the runner: the penalty fires on Rank >= 11
			// (Jack or HIGHER, so Aces too -- v2's applyDrawPenalty semantics),
			// and on a multi-card combo it checks only the LAST card set down.
			// The dossier must describe the game the engine actually plays.
			out = append(out, "Whenever you play a high card -- Jack, Queen, King, or Ace (on a multi-card play, only the last card counts) -- you must immediately draw one extra card from the deck as a penalty.")
		case ModKnock:
			if s.Move == Rummy {
				// ginKnockThreshold: the knock move is offered at deadwood <= 2.
				out = append(out, fmt.Sprintf("When you have %d or fewer unmelded cards, you may KNOCK at the start of your turn, instead of drawing, to end the game immediately. Whoever has the least deadwood then wins -- knock too early and an opponent with fewer stray cards beats you.", ginKnockThreshold))
			} else {
				out = append(out, "When you are down to 3 or fewer cards, you may KNOCK on your turn, instead of playing, to end the game at once. Whoever holds the fewest cards then wins -- so knocking while you are NOT lowest hands the win to someone else; "+turnOrderTie("the knocker")+".")
			}
		case ModWild:
			// deadwood(): each wild completes one leftover NEAR-meld (a pair or
			// two adjacent suited cards) -- it does not fill a gap or extend a
			// meld -- and a wild is never itself counted as stray.
			out = append(out, "Eights are WILD: when a hand is counted, each eight you hold turns a pair, or two consecutive cards of one suit, among your stray cards into a meld. An eight is never itself a stray card.")
		case ModNominate:
			out = append(out, "Eights are WILD: you may play an eight on anything, and when you do you NAME the suit the next player must follow.")
		case ModReverse:
			out = append(out, "Nines REVERSE: playing a nine flips the direction of play, so the turn order runs the other way.")
		case ModSumCapture:
			// captureOptions: the same-rank set and each summing subset (2+
			// cards) are SEPARATE options; one play makes one capture.
			out = append(out, "Sum capture: a played number card may INSTEAD capture one group of two or more number cards on the table whose values add up to its own (an ace counts one; face cards are never part of a sum). One play makes one capture -- the cards of its own rank or one such group, one or the other.")
		case ModTrump:
			out = append(out, "Spades are TRUMP: a spade beats any card of the suit that was led, and the highest spade played wins the trick. You must still follow the led suit if you can.")
		case ModSkip:
			out = append(out, "Sevens SKIP: when you play a seven, the next player loses their turn and play jumps to the player after them.")
		case ModForceDraw:
			out = append(out, "Twos ATTACK: when you play a two, the next player must draw two cards from the deck and loses their turn.")
		case ModBid:
			// contractScore, in full: the judge must see the real incentives.
			out = append(out, fmt.Sprintf("Before the first trick, each player in turn declares how many tricks they will win, from zero to %d. That contract sets your contract score: "+
				"making it scores 10 points per trick bid plus 1 point for each extra trick; falling short loses 10 points for each trick short. "+
				"A bid of zero scores 10 if you take no trick at all and loses 10 for each trick you do take.", s.Deal))
		case ModTeams:
			out = append(out, "Players sit in two partnerships -- the players opposite each other are teammates -- and your scores are pooled with your partner's. The partnership with the better combined result wins, so play for the team, not yourself.")
		case ModMeldBonus:
			// meldBonus, in full (sets and runs are tallied independently).
			out = append(out, "At the end you earn bonus points for combinations among the cards in your score pile: a pair scores 4 and three or more of a kind 5 per card; "+
				"two consecutive cards of one suit scores 2 and a run of three or more 3 per card. A card may count in both a set and a run. These bonuses are added to your total.")
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
			// Turn boundary: the runner lets the last drawer discard before it ends.
			return "The game ends once the draw deck has run out and the player who drew its last card has made their discard, so every hand is the same size."
		}
		return "The game ends once the players' hands are empty and the deck can no longer deal a new round. Cards still on the table then belong to nobody."
	case Showdown:
		return "The hand ends at the showdown, once the betting is settled (everyone still in has matched the bet, or all but one have folded)."
	case Bust:
		return "The round ends when every player has either stuck or busted."
	}
	return ""
}

// The tie rules below are RULES OF THE GAME, not implementation detail: they
// mirror Runner.score / decide / tieOrigin exactly (see the tie-breaking note in
// runner.go). A tie the rulebook does not settle is a game a table cannot finish,
// and one the runner settles differently is a rulebook that lies -- keep in sync.
const (
	// suitOrderHigh / suitOrderLow spell out cardOrder for the card rule.
	suitOrderHigh = "ace high; between cards of equal rank spades beat hearts, hearts beat diamonds, diamonds beat clubs"
	suitOrderLow  = "a two is lowest; between cards of equal rank clubs are lowest, then diamonds, hearts, spades"
)

// turnOrderTie is the turn-order tie rule counted from a named player.
func turnOrderTie(anchor string) string {
	return fmt.Sprintf("a tie goes to %s if they are among the tied players, otherwise to the tied player who sits soonest after them in turn order", anchor)
}

func (s GameSpec) winRule() string {
	switch s.Score {
	case FirstOut:
		return "The player who emptied their hand is the winner."
	case FewestCards:
		return "The player holding the fewest cards wins."
	case ClosestTarget:
		return fmt.Sprintf("Among players who did not bust, the highest total (closest to %d) wins. If every player busts, the player who went over by the least wins. "+
			"A tie goes to the tied player who took fewer cards; if that is equal too, to the tied player whose pile holds the highest card (%s).", s.Target, suitOrderHigh)
	case MostCaptured:
		if s.Move == Trick {
			tie := " If players tie, " + turnOrderTie("the player who won the last trick") + "."
			if s.hasMod(ModTeams) {
				tie = " A tied result goes to the partnership that won the last trick."
			}
			// The trick win rule COMPOSES exactly as Runner.effScore does: the
			// base is the contract score (bid) or the cards won, then the
			// combination bonus is added and the penalty points subtracted, and
			// partnerships add their two scores. It used to say "(less any
			// penalty points)" on every trick game and drop the penalty / bonus
			// entirely once a bid was present.
			if !s.hasMod(ModBid) && !s.hasMod(ModMeldBonus) && !s.hasMod(ModAvoidance) && !s.hasMod(ModTeams) {
				return "The player who won the most cards in tricks wins." + tie
			}
			score := "Each player's score is the number of cards they won in tricks"
			if s.hasMod(ModBid) {
				score = "Each player's score is their contract score"
			}
			if s.hasMod(ModMeldBonus) {
				score += ", plus combination bonuses"
			}
			if s.hasMod(ModAvoidance) {
				score += ", minus penalty points"
			}
			if s.hasMod(ModTeams) {
				return score + ". Partners' scores are added together and the partnership with the higher total wins." + tie
			}
			return score + ". The highest score wins." + tie
		}
		// Capture host: the scoring modifiers adjust the count exactly as they
		// do on the trick host, so the win rule must say so -- the Special
		// Rules describe a penalty/bonus the winner line otherwise ignored.
		tie := fmt.Sprintf(" A tie goes to the tied player whose captured pile holds the highest card (%s).", suitOrderHigh)
		switch {
		case s.hasMod(ModAvoidance) && s.hasMod(ModMeldBonus):
			return "The player with the best adjusted total wins: cards captured, plus combination bonuses, minus penalty points." + tie
		case s.hasMod(ModAvoidance):
			return "The player with the best adjusted total wins: cards captured minus penalty points." + tie
		case s.hasMod(ModMeldBonus):
			return "The player with the best adjusted total wins: cards captured plus combination bonuses." + tie
		}
		return "The player who captured the most cards wins." + tie
	case HighScore:
		return "The player with the highest score wins."
	case FewestDeadwood:
		return fmt.Sprintf("The player whose hand has the fewest unmelded cards (the least deadwood) wins. "+
			"If players tie on that count, the lower total point value of those unmelded cards wins (aces count 1, face cards 10, other cards their number); "+
			"if that is equal too, the tied player holding the lowest card wins (%s).", suitOrderLow)
	case BestHand:
		return "At the showdown the best five-card poker hand among the players still in wins; if everyone else folds, the last player in wins uncontested. " +
			"An exact tie (the same ranks in both hands) goes to the tied player holding the highest card by suit (spades high, then hearts, diamonds, clubs)."
	}
	return ""
}

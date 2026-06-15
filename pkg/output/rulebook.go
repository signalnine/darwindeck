package output

import (
	"fmt"
	"strings"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

// GenerateRulebook produces a human-readable markdown rulebook from a genome.
func GenerateRulebook(g *genome.Genome) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("# %s\n\n", gameName(g)))
	b.WriteString(fmt.Sprintf("**Players:** %d  \n", g.Players))
	b.WriteString(fmt.Sprintf("**Cards:** Standard 52-card deck  \n"))
	b.WriteString(fmt.Sprintf("**Hand Size:** %d cards per player  \n\n", g.HandSize))

	switch g.Skeleton {
	case genome.Shedding:
		writeSheddingRules(&b, g)
	case genome.TrickTaking:
		writeTrickTakingRules(&b, g)
	case genome.Rummy:
		writeRummyRules(&b, g)
	case genome.Climbing:
		writeClimbingRules(&b, g)
	case genome.Casino:
		writeCasinoRules(&b, g)
	case genome.Vying:
		writeVyingRules(&b, g)
	}

	// Special cards are only simulated by the shedding runner, so only render
	// them for shedding games -- otherwise the rulebook advertises skip/draw
	// effects that never fire (dd-24e).
	if g.Skeleton == genome.Shedding && len(g.SpecialCards) > 0 {
		writeSpecialCards(&b, g)
	}

	if borrows := liveBorrows(g); len(borrows) > 0 {
		writeBorrowedRules(&b, borrows)
	}

	// Only render the point table when a rule actually consumes it: a table
	// nothing reads is dead text that lies about the game (round 3 commit
	// 6a; the r2 flagship printed point tables under ScorePerTrick and on
	// borrow-less rummy genomes).
	if liveCardPoints(g) {
		writeScoringTable(&b, g)
	}

	return b.String()
}

// liveCardPoints delegates to the genome-level liveness predicate (hoisted
// to pkg/genome in Wave K so the output-ranking dedup shares the exact same
// rules; semantics pinned by pkg/genome/liveness_test.go).
func liveCardPoints(g *genome.Genome) bool {
	return g.LiveCardPoints()
}

// liveBorrows delegates to the genome-level liveness predicate (see
// liveCardPoints).
func liveBorrows(g *genome.Genome) []genome.BorrowedMechanic {
	return g.LiveBorrows()
}

func gameName(g *genome.Genome) string {
	if g.ID != "" {
		return g.ID
	}
	return fmt.Sprintf("Evolved %s Game", g.Skeleton)
}

func writeSheddingRules(b *strings.Builder, g *genome.Genome) {
	b.WriteString("## How to Play\n\n")
	b.WriteString("This is a **shedding game** — the goal is to be the first player to empty your hand.\n\n")

	b.WriteString("### Setup\n\n")
	b.WriteString(fmt.Sprintf("Deal %d cards to each player. Flip the top card of the deck to start the discard pile.\n\n", g.HandSize))

	b.WriteString("### On Your Turn\n\n")

	if g.Shedding != nil {
		matchDesc := matchRuleDescription(g.Shedding.MatchRule)
		b.WriteString(fmt.Sprintf("Play a card from your hand that **%s** the top card of the discard pile.\n\n", matchDesc))
		b.WriteString(fmt.Sprintf("If you cannot play, **draw %d card(s)** from the deck.\n\n", g.Shedding.DrawPenalty))
	}

	if g.SheddingMultiRound() {
		writeSheddingRoundStructure(b, g)
		return
	}

	b.WriteString("### Winning\n\n")
	b.WriteString("The first player to play all their cards wins. If no player can play and the deck runs out, the game ends in a draw.\n\n")
}

// writeSheddingRoundStructure renders the multi-round win rules (Task 22):
// rounds end on an emptied hand, the borrowed scoring banks points, and after
// all rounds the highest banked total wins. MechAvoidance banks penalties as
// negative points, so "highest total" is explained as fewest penalty points.
func writeSheddingRoundStructure(b *strings.Builder, g *genome.Genome) {
	rounds := g.Shedding.RoundsPerGame

	b.WriteString("### Rounds\n\n")
	b.WriteString(fmt.Sprintf("The game is played over **%d rounds**. Playing your last card ends the round: everyone's hand is scored (see Additional Rules), the scores are banked, and all cards are gathered, shuffled, and redealt for the next round.\n\n", rounds))

	b.WriteString("### Winning\n\n")
	if hasAvoidanceBorrow(g) && !hasMeldBonusBorrow(g) {
		b.WriteString(fmt.Sprintf("After %d rounds, the player with the **fewest penalty points** across all rounds wins. Going out first protects you: cards still in hand when a round ends count against their holder.\n\n", rounds))
	} else if hasAvoidanceBorrow(g) {
		b.WriteString(fmt.Sprintf("After %d rounds, the **highest total score** wins. Meld bonuses add points; penalty cards still in hand subtract them, so the winner is whoever banked the best balance (with avoidance alone, that is the player with the fewest penalty points).\n\n", rounds))
	} else {
		b.WriteString(fmt.Sprintf("After %d rounds, the **highest total score** wins. Emptying your hand ends a round, but points come from the banked scoring -- a player can win on points without ending a single round.\n\n", rounds))
	}
	// Full tiebreak chain, matching the shedding runner's CheckEnd exactly:
	// banked score, then fewest cards in hand, then seat order (the runner's
	// strict comparison keeps the earliest-seated tied player).
	b.WriteString("If scores are tied, the tied player holding the fewest cards at the end of the final round wins; if that is tied too, the tied player seated earliest in the turn order (closest to the dealer's left) wins.\n\n")
}

func hasAvoidanceBorrow(g *genome.Genome) bool {
	for _, bm := range g.Borrowed {
		if bm.Mechanic == genome.MechAvoidance {
			return true
		}
	}
	return false
}

func hasMeldBonusBorrow(g *genome.Genome) bool {
	for _, bm := range g.Borrowed {
		if bm.Mechanic == genome.MechMeldBonus {
			return true
		}
	}
	return false
}

func writeTrickTakingRules(b *strings.Builder, g *genome.Genome) {
	b.WriteString("## How to Play\n\n")
	b.WriteString("This is a **trick-taking game** — players play one card each per trick.\n\n")

	b.WriteString("### Setup\n\n")
	b.WriteString(fmt.Sprintf("Deal %d cards to each player.\n\n", g.HandSize))

	if g.TrumpRule != genome.TrumpNone {
		b.WriteString(fmt.Sprintf("**Trump:** %s\n\n", trumpDescription(g)))
	}

	b.WriteString("### Playing Tricks\n\n")
	b.WriteString("The lead player plays any card. Other players follow in order.\n\n")

	if g.TrickTaking != nil {
		if g.TrickTaking.MustFollowSuit {
			b.WriteString("You **must follow the led suit** if you can. If you cannot, play any card.\n\n")
		} else {
			b.WriteString("You may play any card from your hand.\n\n")
		}

		leadDesc := leadRestrictionDescription(g.TrickTaking.LeadRestriction)
		if leadDesc != "" {
			b.WriteString(fmt.Sprintf("**Lead restriction:** %s\n\n", leadDesc))
		}
	}

	b.WriteString("The highest card of the led suit wins the trick, unless trumped. The trick winner leads the next trick.\n\n")

	b.WriteString("### Scoring\n\n")
	if g.TrickTaking != nil {
		b.WriteString(scoringDescription(g.TrickTaking.TrickScoring))
	}
}

func writeRummyRules(b *strings.Builder, g *genome.Genome) {
	b.WriteString("## How to Play\n\n")
	b.WriteString("This is a **rummy game** — form melds to reduce your deadwood.\n\n")

	b.WriteString("### Setup\n\n")
	b.WriteString(fmt.Sprintf("Deal %d cards to each player. Place one card face-up to start the discard pile.\n\n", g.HandSize))

	b.WriteString("### On Your Turn\n\n")

	if g.Rummy != nil {
		drawDesc := drawDescription(g.Rummy.DrawFrom)
		b.WriteString(fmt.Sprintf("1. **Draw** %s\n", drawDesc))

		meldDesc := meldDescription(g.Rummy)
		b.WriteString(fmt.Sprintf("2. **Meld** (optional): %s\n", meldDesc))

		b.WriteString("3. **Discard** one card to the discard pile\n\n")

		b.WriteString("### Knocking & Gin\n\n")
		if g.Rummy.KnockThreshold == 0 {
			b.WriteString("You can only go out with **Gin** (no deadwood at all).\n\n")
		} else {
			b.WriteString(fmt.Sprintf("You may **knock** when your deadwood is %d points or less.\n\n", g.Rummy.KnockThreshold))
		}
	}

	b.WriteString("### Deadwood Values\n\n")
	b.WriteString("- Face cards (J, Q, K): 10 points\n")
	b.WriteString("- Ace: 1 point\n")
	b.WriteString("- Number cards: face value\n\n")

	b.WriteString("### Winning\n\n")
	b.WriteString("The player with the lowest deadwood when someone knocks or goes gin wins the round.\n\n")
}

func writeClimbingRules(b *strings.Builder, g *genome.Genome) {
	b.WriteString("## How to Play\n\n")
	b.WriteString("This is a **climbing game** (Big Two / Tichu / President family) — the goal is to be the first player to empty your hand.\n\n")

	b.WriteString("### Setup\n\n")
	b.WriteString(fmt.Sprintf("Deal %d cards to each player. The first player leads.\n\n", g.HandSize))

	b.WriteString("### On Your Turn\n\n")
	b.WriteString("There is a **current combination** on the table that must be beaten. On your turn you either:\n\n")
	b.WriteString("- Play a combination of the **same type** as the current one but of **strictly higher rank**, or\n")
	b.WriteString("- **Pass**.\n\n")
	b.WriteString("When you lead (the table is clear) you may play **any** valid combination.\n\n")

	b.WriteString("### Combinations\n\n")
	b.WriteString("- **Single:** one card (always allowed)\n")
	if g.Climbing != nil {
		if g.Climbing.AllowPairs {
			b.WriteString("- **Pair:** two cards of the same rank\n")
		}
		if g.Climbing.AllowTriples {
			b.WriteString("- **Triple:** three cards of the same rank\n")
		}
		if g.Climbing.AllowRuns {
			minLen := g.Climbing.MinRunLen
			if minLen < 3 {
				minLen = 3
			}
			b.WriteString(fmt.Sprintf("- **Run:** %d or more cards of consecutive rank\n", minLen))
		}
	}
	b.WriteString("\n")

	b.WriteString("### Clearing the Table\n\n")
	b.WriteString("When every other player passes in succession, the player who played the current combination wins the round, the table clears, and that player leads a fresh combination.\n\n")

	b.WriteString("### Winning\n\n")
	b.WriteString("The first player to empty their hand wins.\n\n")
}

func writeCasinoRules(b *strings.Builder, g *genome.Genome) {
	b.WriteString("## How to Play\n\n")
	b.WriteString("This is a **fishing / capture game** (Casino / Scopa family) — you capture cards from a shared table into your own pile.\n\n")

	tableN := 0
	allowSum := false
	if g.Casino != nil {
		tableN = g.Casino.TableSize
		allowSum = g.Casino.AllowSumCapture
	}

	b.WriteString("### Setup\n\n")
	b.WriteString(fmt.Sprintf("Deal %d cards to each player and lay %d card(s) face-up on the table. The rest forms the stock.\n\n", g.HandSize, tableN))

	b.WriteString("### On Your Turn\n\n")
	b.WriteString("Play **one card** from your hand and either:\n\n")
	b.WriteString("- **Capture:** take every table card of the **same rank** as the card you played")
	if allowSum {
		b.WriteString(", or take any set of number cards whose values **sum** to your card's value")
	}
	b.WriteString(" — moving them and the card you played into your captured pile, or\n")
	b.WriteString("- **Trail:** if you do not capture, leave the card you played face-up on the table.\n\n")
	b.WriteString("Trailing is always legal, so you always have a move.\n\n")
	if allowSum {
		b.WriteString("For summing, **Ace = 1** and number cards are worth their pips; face cards (J/Q/K) have no value and capture only by matching rank.\n\n")
	}

	b.WriteString("### Refilling and the Final Sweep\n\n")
	b.WriteString("When every hand is empty, deal fresh hands from the stock. Once the stock can no longer deal a full round, the player who captured most recently takes any cards still on the table, and the game ends.\n\n")

	b.WriteString("### Winning\n\n")
	if g.CasinoScored() {
		b.WriteString("Your score is the number of cards you captured plus the bonuses in the Additional Rules below (penalty cards count against you). The highest score wins.\n\n")
	} else {
		b.WriteString("Whoever has captured the **most cards** wins.\n\n")
	}
}

func writeVyingRules(b *strings.Builder, g *genome.Genome) {
	b.WriteString("## How to Play\n\n")
	b.WriteString("This is a **vying / betting game** (poker family) — wager chips on hidden hands; the best poker hand at showdown takes the pot.\n\n")

	chips, minBet, maxRaises, rounds := 0, 0, 0, 0
	if g.Vying != nil {
		chips, minBet, maxRaises, rounds = g.Vying.StartingChips, g.Vying.MinBet, g.Vying.MaxRaises, g.Vying.RoundsPerGame
	}

	b.WriteString("### Setup\n\n")
	b.WriteString(fmt.Sprintf("Each player starts with %d chips. Every deal, each player is dealt %d cards face-down.\n\n", chips, g.HandSize))

	b.WriteString("### Each Deal\n\n")
	b.WriteString(fmt.Sprintf("One player (rotating each deal) posts a big blind of %d chips, so the first to act always faces a bet. Going around, each player in turn either:\n\n", minBet))
	b.WriteString("- **Fold:** drop out, forfeiting any chips already in the pot, or\n")
	b.WriteString("- **Call:** match the current bet, or\n")
	b.WriteString(fmt.Sprintf("- **Raise:** match the bet and increase it by %d (at most %d raises per deal).\n\n", minBet, maxRaises))
	b.WriteString("When nothing is owed you may **check** (stay in for free) instead of calling.\n\n")

	b.WriteString("### Showdown\n\n")
	b.WriteString("Once the betting is settled, the players still in reveal their hands and the best poker hand — pair, two pair, three of a kind, straight, flush, full house, four of a kind, straight flush — takes the pot. If everyone but one player has folded, that player takes the pot uncontested.\n\n")

	b.WriteString("### Winning\n\n")
	b.WriteString(fmt.Sprintf("Chips carry over across %d deals. The player with the most chips at the end wins.\n\n", rounds))
}

func writeSpecialCards(b *strings.Builder, g *genome.Genome) {
	b.WriteString("## Special Cards\n\n")
	for _, sc := range g.SpecialCards {
		card := specialCardName(sc)
		effect := specialCardEffect(sc)
		b.WriteString(fmt.Sprintf("- **%s:** %s\n", card, effect))
	}
	b.WriteString("\n")
}

func writeBorrowedRules(b *strings.Builder, borrows []genome.BorrowedMechanic) {
	b.WriteString("## Additional Rules\n\n")
	for _, bm := range borrows {
		b.WriteString(fmt.Sprintf("- %s\n", borrowedDescription(bm)))
	}
	b.WriteString("\n")
}

func writeScoringTable(b *strings.Builder, g *genome.Genome) {
	b.WriteString("## Card Point Values\n\n")
	b.WriteString("| Card | Points |\n|------|--------|\n")
	for _, cp := range g.Scoring.CardPoints {
		card := scoringCardName(cp)
		b.WriteString(fmt.Sprintf("| %s | %d |\n", card, cp.Points))
	}
	b.WriteString("\n")
}

// --- Helper description functions ---

func matchRuleDescription(r genome.MatchRule) string {
	switch r {
	case genome.MatchSuit:
		return "matches the suit of"
	case genome.MatchRank:
		return "matches the rank of"
	case genome.MatchEither:
		return "matches the suit or rank of"
	case genome.MatchBoth:
		return "matches both the suit and rank of"
	default:
		return "matches"
	}
}

func trumpDescription(g *genome.Genome) string {
	switch g.TrumpRule {
	case genome.TrumpFixed:
		suits := [5]string{"", "Clubs", "Diamonds", "Hearts", "Spades"}
		s := int(g.Scoring.TrumpSuit)
		if s >= 1 && s <= 4 {
			return suits[s] + " are always trump"
		}
		return "Fixed trump suit"
	case genome.TrumpCut:
		return "Cut a card from the deck to determine trump suit"
	case genome.TrumpLed:
		return "The first suit led becomes trump"
	default:
		return "No trump"
	}
}

func leadRestrictionDescription(r genome.LeadRule) string {
	switch r {
	case genome.LeadNoTrumpUntilBroken:
		return "Cannot lead trump until trump has been played off-suit"
	default:
		// LeadWinnerLeads is reserved/inert (see genome.LeadRule): the
		// universal "The trick winner leads the next trick." sentence already
		// states the hardcoded turn order, so describing the value as an
		// extra restriction would duplicate (not lie about) the rules.
		return ""
	}
}

func scoringDescription(s genome.TrickScoring) string {
	switch s {
	case genome.ScorePerTrick:
		return "Each trick won is worth **1 point**. Highest score wins.\n\n"
	case genome.ScoreCardPoints:
		return "Tricks are scored by the **point values** of the cards they contain (see scoring table). Highest score wins.\n\n"
	case genome.ScoreAvoidance:
		return "Tricks are scored by the **point values** of the cards they contain (see scoring table). **Lowest score wins** — avoid taking point cards!\n\n"
	default:
		return "Score by tricks won.\n\n"
	}
}

func drawDescription(d genome.DrawSource) string {
	switch d {
	case genome.DrawDeck:
		return "one card from the deck"
	case genome.DrawDiscard:
		return "the top card from the discard pile"
	case genome.DrawEither:
		return "one card from the deck OR the top card from the discard pile"
	default:
		return "one card"
	}
}

func meldDescription(p *genome.RummyParams) string {
	var parts []string
	switch p.MeldTypes {
	case genome.MeldSets:
		parts = append(parts, fmt.Sprintf("groups of %d+ cards of the same rank", p.MinMeldSize))
	case genome.MeldRuns:
		parts = append(parts, fmt.Sprintf("sequences of %d+ consecutive cards in the same suit", p.MinMeldSize))
	case genome.MeldBoth:
		parts = append(parts, fmt.Sprintf("groups of %d+ same-rank cards OR sequences of %d+ consecutive same-suit cards", p.MinMeldSize, p.MinMeldSize))
	}
	return strings.Join(parts, ". ")
}

func specialCardName(sc genome.SpecialCard) string {
	suitNames := [5]string{"", "Club", "Diamond", "Heart", "Spade"}
	hasRank := sc.ByRank != 0
	hasSuit := sc.BySuit >= 1 && int(sc.BySuit) < len(suitNames)

	switch {
	case hasRank && hasSuit:
		return fmt.Sprintf("the %s of %ss", sim.Rank(sc.ByRank).String(), suitNames[sc.BySuit])
	case hasRank:
		return fmt.Sprintf("any %s", sim.Rank(sc.ByRank).String())
	case hasSuit:
		return fmt.Sprintf("any %s", suitNames[sc.BySuit])
	default:
		return "any card"
	}
}

func specialCardEffect(sc genome.SpecialCard) string {
	switch sc.Type {
	case genome.SpecialSkip:
		return "Skip the next player's turn"
	case genome.SpecialReverse:
		return "Reverse play direction"
	case genome.SpecialDrawTwo:
		return "Next player draws 2 cards"
	case genome.SpecialDrawFour:
		return "Next player draws 4 cards"
	case genome.SpecialWild:
		return "Can be played on any card"
	default:
		return "Special effect"
	}
}

// borrowedDescription renders a borrowed (cross-skeleton) mechanic as a
// CONCRETE rule, not a generic blurb. The borrow is the entire source of a
// hybrid game's novelty, so a vague one-liner ("Earn bonus points for forming
// sets or runs") leaves a reader -- human or LLM judge -- unable to tell a real
// mechanical fusion from an undefined bolt-on, and unable to assess the game at
// all (dd: blind-novelty bug). Each description below states exactly what the
// corresponding hook in pkg/mechanic/hooks.go does, INCLUDING its scoring
// magnitudes and trigger, so the text the judge reads matches the behavior that
// actually fires in play.
//
// KEEP IN SYNC with pkg/mechanic/hooks.go: the point magnitudes here are the
// constants in applyMeldBonus/runBonus and the trigger in applyDrawPenalty.
// TestBorrowedRulesDescribeConcreteMechanics asserts the anchor phrases so this
// coupling cannot silently drift back into a generic blurb.
//
// Only the four whitelisted borrows (MechMeldBonus, MechAvoidance,
// MechTrickScoring, MechDrawPenalty -- see genome.ValidBorrows) can appear on a
// valid genome; the reserved cases are kept harmless for defensiveness.
func borrowedDescription(bm genome.BorrowedMechanic) string {
	switch bm.Mechanic {
	case genome.MechTrickScoring:
		// applyTrickScoring: at EventRoundEnd, the player with the most
		// captured cards (tableau captures + laid-down melds) gains a bonus
		// equal to that count; ties split it evenly.
		return "**Capture bonus:** at the end of each round, whoever has captured the most cards (from won tricks and laid-down melds) scores bonus points equal to the number of cards they captured; players tied for the most split the bonus evenly"
	case genome.MechMeldBonus:
		// applyMeldBonus + runBonus: scored at EventRoundEnd over hand +
		// captured cards. Sets: 3+ same rank = 5/card, pair = 2/card. Runs:
		// 3+ consecutive same-suit = 3/card, 2-card run = 1/card.
		return "**Meld bonus:** at the end of each round, score points for card combinations across your hand and the cards you have captured — a set of 3 or more of the same rank scores 5 points per card (a plain pair scores 2 per card), and a run of 3 or more consecutive cards in one suit scores 3 points per card (a 2-card run scores 1 per card)"
	case genome.MechDrawPenalty:
		// applyDrawPenalty: after playing a Jack-or-higher, draw 1 extra card.
		return "**Draw penalty:** whenever you play a face card (Jack or higher), you must immediately draw 1 extra card from the deck"
	case genome.MechAvoidance:
		// applyAvoidance: at round end, subtract the Card Point Values of
		// penalty cards left in hand + captures (liveness guarantees CardPoints).
		return "**Penalty cards:** at the end of each round, lose points equal to the value of any scoring cards (see Card Point Values) still in your hand or among the cards you captured — avoid collecting them"
	case genome.MechRunPlay:
		// shedding/runner.go ComboPlay: discard same-rank sets / same-suit runs
		// of 2+ cards in one turn (when the group matches the top).
		return "**Combination plays:** instead of one card, you may discard a set of 2 or more cards of the same rank, or a run of 2 or more consecutive cards in one suit, in a single turn — as long as one of those cards legally matches the discard top. Unloading several cards at once lets you empty your hand in bursts, so it pays to hold cards and dump a whole run"
	case genome.MechKnock:
		// shedding/runner.go Knockable: once your hand is down to a few cards
		// you may knock to end the game immediately; fewest cards then wins.
		return "**Knock:** once your hand is down to a few cards, instead of playing you may knock to end the game at once. When you knock, whoever holds the fewest cards wins — so knock when you are ahead, but knocking while someone else is shorter hands them the win"
	case genome.MechTrump:
		return "One suit is designated as trump and beats other suits"
	case genome.MechPlayMultiple:
		return "Play multiple matching cards at once"
	case genome.MechFollowSuit:
		// shedding/runner.go FollowConstrained: if you hold the discard top's
		// suit you must play it (or a wild); only when void do other plays and
		// drawing reopen.
		return "**Follow the discard suit:** if you hold any card of the current discard's suit, you must play one of them (or a wild) -- you may only play off-suit or draw when you have no card of that suit"
	default:
		return "Additional mechanic"
	}
}

func scoringCardName(cp genome.CardScoring) string {
	rank := "Any"
	if cp.Rank != 0 {
		rank = sim.Rank(cp.Rank).String()
	}
	suit := "any suit"
	if cp.Suit != 0 {
		suits := [5]string{"", "Clubs", "Diamonds", "Hearts", "Spades"}
		if int(cp.Suit) < len(suits) {
			suit = suits[cp.Suit]
		}
	}
	if cp.Rank == 0 && cp.Suit != 0 {
		return fmt.Sprintf("All %s", suit)
	}
	if cp.Rank != 0 && cp.Suit == 0 {
		return fmt.Sprintf("All %ss", rank)
	}
	return fmt.Sprintf("%s of %s", rank, suit)
}

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

// liveCardPoints reports whether anything in g's RULES reads
// Scoring.CardPoints: trick-taking under card_points/avoidance scoring
// (cardPointValue in the runner), or a LIVE MechAvoidance borrow (the
// applyAvoidance hook returns early on empty CardPoints; see liveBorrows for
// when the borrow itself is live).
func liveCardPoints(g *genome.Genome) bool {
	if len(g.Scoring.CardPoints) == 0 {
		return false
	}
	if g.Skeleton == genome.TrickTaking && g.TrickTaking != nil &&
		(g.TrickTaking.TrickScoring == genome.ScoreCardPoints ||
			g.TrickTaking.TrickScoring == genome.ScoreAvoidance) {
		return true
	}
	for _, bm := range liveBorrows(g) {
		if bm.Mechanic == genome.MechAvoidance {
			return true
		}
	}
	return false
}

// liveBorrows returns the borrowed mechanics that can actually affect g's
// outcome -- the ones the rulebook (and report) may advertise (round 3
// commit 6b; the r2 rank05 advertised a meld-bonus borrow that was inert at
// rounds_per_game 1):
//
//   - SCORING borrows (MechMeldBonus, MechAvoidance) bank state.Scores at
//     round end. On a SINGLE-round shedding host nothing ever reads those
//     scores (the game ends at the first empty hand), so they are live only
//     when genome.SheddingMultiRound() -- the same predicate the runner
//     uses. Trick-taking and rummy hosts read Scores in CheckEnd, so they
//     are live there at any round count.
//   - MechAvoidance additionally requires non-empty CardPoints (the hook
//     no-ops without them).
//   - Everything else whitelisted (MechTrickScoring, MechDrawPenalty) acts
//     directly and is always live.
func liveBorrows(g *genome.Genome) []genome.BorrowedMechanic {
	var live []genome.BorrowedMechanic
	for _, bm := range g.Borrowed {
		switch bm.Mechanic {
		case genome.MechMeldBonus, genome.MechAvoidance:
			if g.Skeleton == genome.Shedding && !g.SheddingMultiRound() {
				continue
			}
			if bm.Mechanic == genome.MechAvoidance && len(g.Scoring.CardPoints) == 0 {
				continue
			}
		}
		live = append(live, bm)
	}
	return live
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

func borrowedDescription(bm genome.BorrowedMechanic) string {
	switch bm.Mechanic {
	case genome.MechTrickScoring:
		return "Score bonus points based on trick-like card combinations"
	case genome.MechMeldBonus:
		return "Earn bonus points for forming sets or runs"
	case genome.MechDrawPenalty:
		return "Draw extra cards as a penalty in certain situations"
	case genome.MechKnock:
		return "Knock to end the round early"
	case genome.MechTrump:
		return "One suit is designated as trump and beats other suits"
	case genome.MechAvoidance:
		return "Certain cards carry penalty points — avoid collecting them"
	case genome.MechPlayMultiple:
		return "Play multiple matching cards at once"
	case genome.MechFollowSuit:
		return "Must play a card of the led suit if possible"
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

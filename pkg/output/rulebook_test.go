package output

import (
	"strings"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
)

func TestSpecialCardName(t *testing.T) {
	tests := []struct {
		name string
		sc   genome.SpecialCard
		want string
	}{
		{
			name: "rank-only renders as plural any-suit rank",
			sc:   genome.SpecialCard{Type: genome.SpecialSkip, ByRank: 7, BySuit: 0},
			want: "any 7",
		},
		{
			name: "rank and suit renders as singular rank-of-suit",
			sc:   genome.SpecialCard{Type: genome.SpecialSkip, ByRank: 7, BySuit: 4},
			want: "the 7 of Spades",
		},
		{
			name: "suit-only renders as any-of-suit",
			sc:   genome.SpecialCard{Type: genome.SpecialSkip, ByRank: 0, BySuit: 2},
			want: "any Diamond",
		},
		{
			name: "neither renders as any card",
			sc:   genome.SpecialCard{Type: genome.SpecialSkip, ByRank: 0, BySuit: 0},
			want: "any card",
		},
		{
			name: "Jack of Hearts",
			sc:   genome.SpecialCard{Type: genome.SpecialWild, ByRank: 11, BySuit: 3},
			want: "the J of Hearts",
		},
		{
			name: "any Club",
			sc:   genome.SpecialCard{Type: genome.SpecialReverse, ByRank: 0, BySuit: 1},
			want: "any Club",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := specialCardName(tc.sc)
			if got != tc.want {
				t.Errorf("specialCardName(%+v) = %q, want %q", tc.sc, got, tc.want)
			}
		})
	}
}

func TestSpecialCardNameDistinguishesSuitBoundFromAnySuit(t *testing.T) {
	anySeven := genome.SpecialCard{Type: genome.SpecialSkip, ByRank: 7, BySuit: 0}
	sevenOfSpades := genome.SpecialCard{Type: genome.SpecialSkip, ByRank: 7, BySuit: 4}
	if specialCardName(anySeven) == specialCardName(sevenOfSpades) {
		t.Errorf("any-7 and 7-of-Spades must produce distinct names; both returned %q",
			specialCardName(anySeven))
	}
}

func TestSpecialCardNameDistinguishesAllCardsFromSuitBound(t *testing.T) {
	anyCard := genome.SpecialCard{Type: genome.SpecialSkip, ByRank: 0, BySuit: 0}
	anyDiamond := genome.SpecialCard{Type: genome.SpecialSkip, ByRank: 0, BySuit: 2}
	if specialCardName(anyCard) == specialCardName(anyDiamond) {
		t.Errorf("any-card and any-Diamond must produce distinct names; both returned %q",
			specialCardName(anyCard))
	}
}

func TestSheddingRulebookDoesNotClaimFewestCardsTiebreak(t *testing.T) {
	// The shedding runner does NOT award the fewest-cards player on
	// deck-out; CheckEnd returns -1 (timeout) so the batch runner
	// classifies the game as a timeout. The rulebook must not claim
	// otherwise. See dd-73h.
	g := seeds.CrazyEights()
	rb := GenerateRulebook(g)

	if strings.Contains(rb, "fewest cards wins") {
		t.Errorf("shedding rulebook still claims 'fewest cards wins' on deck-out, but the runner returns -1 (timeout)")
	}
}

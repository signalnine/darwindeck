package genome

import (
	"strings"
	"testing"
)

// TestDuplicateMechanicBorrowRejected pins the Tier-0 duplicate-borrow rule
// keying on Mechanic ALONE. Source is provenance, not behavior: BuildHooks
// builds one hook per Borrowed entry, so a genome carrying the same mechanic
// twice under DIFFERENT Sources would double-apply the effect per event
// (silently doubling the rulebook-stated bonus) and print the rule twice in
// the rulebook. The old check keyed on the full (Source, Mechanic) pair and
// let such genomes through.
func TestDuplicateMechanicBorrowRejected(t *testing.T) {
	base := func() *Genome {
		return &Genome{
			ID:       "dup-borrow",
			Skeleton: Shedding,
			Players:  2,
			HandSize: 7,
			Shedding: &SheddingParams{MatchRule: MatchEither, DrawPenalty: 1, RoundsPerGame: 2},
		}
	}

	cases := []struct {
		name     string
		borrowed []BorrowedMechanic
		wantDup  bool
	}{
		{
			name: "same mechanic, different sources",
			borrowed: []BorrowedMechanic{
				{Source: Rummy, Mechanic: MechMeldBonus},
				{Source: TrickTaking, Mechanic: MechMeldBonus},
			},
			wantDup: true,
		},
		{
			name: "same mechanic, same source",
			borrowed: []BorrowedMechanic{
				{Source: Rummy, Mechanic: MechMeldBonus},
				{Source: Rummy, Mechanic: MechMeldBonus},
			},
			wantDup: true,
		},
		{
			name: "distinct mechanics",
			borrowed: []BorrowedMechanic{
				{Source: Rummy, Mechanic: MechMeldBonus},
				{Source: TrickTaking, Mechanic: MechAvoidance},
			},
			wantDup: false,
		},
	}

	for _, tc := range cases {
		g := base()
		g.Borrowed = tc.borrowed
		if tc.name == "distinct mechanics" {
			// The avoidance hook needs a penalty set to be coherent elsewhere;
			// keep the genome representative of what the engine produces.
			g.Scoring.CardPoints = []CardScoring{{Suit: 3, Points: 1}}
		}
		errs := Validate(g)
		gotDup := false
		for _, e := range errs {
			if strings.Contains(e, "duplicate borrowed mechanic") {
				gotDup = true
			}
		}
		if gotDup != tc.wantDup {
			t.Errorf("%s: duplicate-borrow error = %v, want %v (errs: %v)", tc.name, gotDup, tc.wantDup, errs)
		}
	}
}

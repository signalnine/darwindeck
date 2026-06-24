// Command grammar-proto is the de-risk experiment for a generative card-game
// grammar (pkg/grammar). It (1) checks the grammar expresses the existing
// skeletons, (2) counts the family space, and (3) runs every composition through
// random AI to test the bet: does a playable-by-construction grammar stay in the
// playable manifold, or rediscover the v1 desert?
package main

import (
	"fmt"
	"sort"

	"github.com/darwindeck/darwindeck/pkg/grammar"
)

func main() {
	const trials = 40

	fmt.Println("== Expressiveness: the grammar reproduces these hand-coded skeletons ==")
	names := []string{"shedding", "climbing", "banking", "casino"}
	for i, s := range grammar.Canonical() {
		sm := grammar.Runner{Spec: s}.Playability(trials, 1)
		fmt.Printf("  %-9s %-56s term=%2d/%d agency=%.2f turns=%5.0f %s\n",
			names[i], s.String(), sm.Terminated, sm.Trials, sm.AgencyFrac, sm.MeanTurns, mark(sm.Playable()))
	}

	// The untyped cross-product vs the coherence-typed grammar. The collapse from
	// the former to the latter is the grammar's promise: illegal compositions are
	// unrepresentable, not caught at runtime.
	all := grammar.EnumerateAll()
	typed := grammar.Enumerate()
	canon := grammar.CanonicalFamilies()
	famAll := grammar.Families(all)
	famTyped := grammar.Families(typed)
	fmt.Printf("\n== Family space (4 move-generators x ends x scorings x params) ==\n")
	fmt.Printf("  untyped cross-product : %3d specs / %2d families\n", len(all), len(famAll))
	fmt.Printf("  WELL-TYPED grammar    : %3d specs / %2d families  (%d canonical, %d novel)\n",
		len(typed), len(famTyped), countCanon(famTyped, canon), len(famTyped)-countCanon(famTyped, canon))

	// Per-family rollup over the UNTYPED set, so the typing rules stay grounded in
	// the data and the excluded families are visible with their failure reason.
	type famStat struct {
		specs, natural, stalemate, lowAgency int
		typed                                bool
	}
	famStats := map[string]*famStat{}
	for _, s := range all {
		sm := grammar.Runner{Spec: s}.Playability(trials, 7)
		f := s.Family()
		fs := famStats[f]
		if fs == nil {
			fs = &famStat{typed: s.WellTyped()}
			famStats[f] = fs
		}
		fs.specs++
		switch {
		case sm.AgencyFrac <= 0.05:
			fs.lowAgency++
		case !sm.NaturalEnd():
			fs.stalemate++
		default:
			fs.natural++
		}
	}

	// Headline: playability of the WELL-TYPED grammar (what search would explore).
	var playable, natural, stuckN, nontermN int
	famSeen := map[string]bool{}
	var novelPlayable []string
	for _, s := range typed {
		sm := grammar.Runner{Spec: s}.Playability(trials, 7)
		if sm.Stuck > 0 {
			stuckN++
		}
		if sm.HitCap > 0 {
			nontermN++
		}
		if sm.Playable() {
			playable++
			if sm.NaturalEnd() {
				natural++
				if f := s.Family(); !canon[f] && !famSeen[f] {
					famSeen[f] = true
					novelPlayable = append(novelPlayable, f)
				}
			}
		}
	}
	fmt.Printf("\n== Playability of the WELL-TYPED grammar under random AI (%d trials, %d-turn cap) ==\n", trials, 2000)
	fmt.Printf("  PLAYABLE (terminates + never stuck + has agency)   : %d/%d (%.0f%%)\n",
		playable, len(typed), 100*float64(playable)/float64(len(typed)))
	fmt.Printf("  of which NATURAL-END (own end fires, not stalemate): %d\n", natural)
	fmt.Printf("  stalemate-reliant (should be 0 after typing)       : %d\n", playable-natural)
	fmt.Printf("  ever STUCK (empty move set = safety violation)     : %d\n", stuckN)
	fmt.Printf("  ever hit the turn cap (non-termination)            : %d\n", nontermN)

	sort.Strings(novelPlayable)
	fmt.Printf("\n== NOVEL, well-typed, playable families (not any hand-coded skeleton) ==\n")
	for _, f := range novelPlayable {
		fmt.Printf("  %s\n", f)
	}

	// The diagnostic that drove the typing rules: which families typing excludes
	// (MIS-TYPED / AGENCY-DEAD) vs keeps (well-typed), over the untyped set.
	fams := make([]string, 0, len(famStats))
	for f := range famStats {
		fams = append(fams, f)
	}
	sort.Strings(fams)
	fmt.Printf("\n== Per-family verdict over the untyped set (what the type rules act on) ==\n")
	fmt.Printf("  %-44s %5s %5s %5s %5s  %s\n", "family", "spec", "nat", "stale", "agz", "type verdict")
	for _, f := range fams {
		fs := famStats[f]
		tag := "well-typed  KEEP"
		switch {
		case !fs.typed && fs.lowAgency >= fs.stalemate && fs.lowAgency > 0:
			tag = "AGENCY-DEAD drop (forced moves)"
		case !fs.typed && fs.stalemate > 0:
			tag = "MIS-TYPED   drop (end unreachable)"
		case !fs.typed:
			tag = "off-type    drop"
		}
		mk := ""
		if canon[f] {
			mk = " [canonical]"
		}
		fmt.Printf("  %-44s %5d %5d %5d %5d  %s%s\n", f, fs.specs, fs.natural, fs.stalemate, fs.lowAgency, tag, mk)
	}
}

func countCanon(fam map[string]int, canon map[string]bool) int {
	n := 0
	for f := range fam {
		if canon[f] {
			n++
		}
	}
	return n
}

func mark(b bool) string {
	if b {
		return "OK"
	}
	return "FAIL"
}

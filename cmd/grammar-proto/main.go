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

	specs := grammar.Enumerate()
	fam := grammar.Families(specs)
	canon := grammar.CanonicalFamilies()
	novelFam := 0
	for f := range fam {
		if !canon[f] {
			novelFam++
		}
	}
	fmt.Printf("\n== Family space (4 move-generators x ends x scorings x params) ==\n")
	fmt.Printf("  full specs enumerated : %d\n", len(specs))
	fmt.Printf("  distinct families     : %d  (%d canonical, %d NOVEL)\n", len(fam), len(fam)-novelFam, novelFam)

	// Per-family rollup so the typing work is grounded in data, not guesswork.
	type famStat struct {
		specs, natural, stalemate, lowAgency int
	}
	famStats := map[string]*famStat{}
	var playable, natural, stuckN, nontermN int
	famSeen := map[string]bool{}
	var novelPlayable []string
	for _, s := range specs {
		sm := grammar.Runner{Spec: s}.Playability(trials, 7)
		f := s.Family()
		fs := famStats[f]
		if fs == nil {
			fs = &famStat{}
			famStats[f] = fs
		}
		fs.specs++
		if sm.Stuck > 0 {
			stuckN++
		}
		if sm.HitCap > 0 {
			nontermN++
		}
		switch {
		case sm.AgencyFrac <= 0.05:
			fs.lowAgency++
		case !sm.NaturalEnd():
			fs.stalemate++
		default:
			fs.natural++
		}
		if sm.Playable() {
			playable++
			if sm.NaturalEnd() {
				natural++
				if !canon[f] && !famSeen[f] {
					famSeen[f] = true
					novelPlayable = append(novelPlayable, f)
				}
			}
		}
	}
	fmt.Printf("\n== Playability under random AI (%d trials each, %d-turn cap) ==\n", trials, 2000)
	fmt.Printf("  PLAYABLE (terminates + never stuck + has agency)   : %d/%d (%.0f%%)\n",
		playable, len(specs), 100*float64(playable)/float64(len(specs)))
	fmt.Printf("  of which NATURAL-END (own end fires, not stalemate): %d  <- the well-typed families\n", natural)
	fmt.Printf("  stalemate-reliant (end unreachable = mis-typed)    : %d  <- prune via composition typing\n", playable-natural)
	fmt.Printf("  ever STUCK (empty move set = safety violation)     : %d\n", stuckN)
	fmt.Printf("  ever hit the turn cap (non-termination)            : %d\n", nontermN)

	sort.Strings(novelPlayable)
	fmt.Printf("\n== NOVEL, natural-end, playable families (not any hand-coded skeleton) ==\n")
	for i, f := range novelPlayable {
		if i >= 12 {
			fmt.Printf("  ... and %d more\n", len(novelPlayable)-12)
			break
		}
		fmt.Printf("  %s\n", f)
	}

	// The diagnostic that drives the typing rules: which families are well-typed,
	// which only end via stalemate (end unreachable), which are agency-dead.
	fams := make([]string, 0, len(famStats))
	for f := range famStats {
		fams = append(fams, f)
	}
	sort.Strings(fams)
	fmt.Printf("\n== Per-family verdict (specs = param variants of the family) ==\n")
	fmt.Printf("  %-44s %5s %5s %5s %5s  %s\n", "family", "spec", "nat", "stale", "agz", "tag")
	for _, f := range fams {
		fs := famStats[f]
		tag := "well-typed"
		switch {
		case fs.natural == 0 && fs.stalemate > 0:
			tag = "MIS-TYPED (end unreachable)"
		case fs.natural == 0 && fs.lowAgency > 0:
			tag = "AGENCY-DEAD"
		case fs.stalemate > 0 || fs.lowAgency > 0:
			tag = "mixed (tighten params)"
		}
		mk := ""
		if canon[f] {
			mk = " [canonical]"
		}
		fmt.Printf("  %-44s %5d %5d %5d %5d  %s%s\n", f, fs.specs, fs.natural, fs.stalemate, fs.lowAgency, tag, mk)
	}
}

func mark(b bool) string {
	if b {
		return "OK"
	}
	return "FAIL"
}

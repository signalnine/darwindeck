package grammar

// This file enumerates the structurally-coherent composition space. Coherence
// here means only "the score's signal is produced by the move/end" -- it does
// NOT prejudge playability; that is what the random-AI harness measures. The
// point is to show the family count is the combinatorial product of the
// primitives, and that the product stays inside the playable manifold.

func matchesFor(m MoveGen) []MatchRule {
	if m == PlayMatch {
		return []MatchRule{MatchSuit, MatchRank, MatchEither}
	}
	return []MatchRule{MatchEither} // not meaningful for the others
}

func endsFor(m MoveGen) []EndRule {
	switch m {
	case PlayMatch, BeatOrPass:
		return []EndRule{EmptyHand, DeckOut}
	case Accumulate:
		return []EndRule{Bust}
	case Capture:
		return []EndRule{DeckOut}
	}
	return nil
}

func scoresFor(m MoveGen) []ScoreRule {
	switch m {
	case PlayMatch, BeatOrPass:
		return []ScoreRule{FirstOut, FewestCards}
	case Accumulate:
		return []ScoreRule{ClosestTarget, HighScore}
	case Capture:
		return []ScoreRule{MostCaptured, HighScore}
	}
	return nil
}

func dealFor(m MoveGen) []int {
	switch m {
	case PlayMatch, BeatOrPass:
		return []int{5, 7}
	case Accumulate:
		return []int{0} // banking builds from the market/deck, not a hand
	case Capture:
		return []int{4}
	}
	return []int{5}
}

func sharedFor(m MoveGen) int {
	switch m {
	case PlayMatch:
		return 1 // a starting discard to match
	case Accumulate:
		return 3 // the face-up market to take from
	case Capture:
		return 4 // the casino table
	}
	return 0 // BeatOrPass leads onto an empty table
}

func targetsFor(m MoveGen) []int {
	if m == Accumulate {
		return []int{21, 31}
	}
	return []int{0}
}

var enumPlayers = []int{2, 3, 4}

// Enumerate returns every WELL-TYPED full GameSpec -- the actual grammar. The
// coherence type (GameSpec.WellTyped) is what makes illegal compositions
// unrepresentable; this is the set evolution/search would operate over.
func Enumerate() []GameSpec {
	var out []GameSpec
	for _, s := range EnumerateAll() {
		if s.WellTyped() {
			out = append(out, s)
		}
	}
	return out
}

// EnumerateAll returns the loose cross-product BEFORE the coherence type is
// applied -- kept so the typing collapse (untyped families vs well-typed) stays
// reproducible, not just a committed text file.
func EnumerateAll() []GameSpec {
	var out []GameSpec
	for m := MoveGen(0); m < moveGenCount; m++ {
		for _, match := range matchesFor(m) {
			for _, end := range endsFor(m) {
				for _, sc := range scoresFor(m) {
					for _, players := range enumPlayers {
						for _, deal := range dealFor(m) {
							for _, target := range targetsFor(m) {
								out = append(out, GameSpec{
									Players: players, Deal: deal, Shared: sharedFor(m),
									Move: m, Match: match, Target: target, End: end, Score: sc,
								})
							}
						}
					}
				}
			}
		}
	}
	return out
}

// Families counts distinct structural families (Move(+Match)|End|Score).
func Families(specs []GameSpec) map[string]int {
	fam := map[string]int{}
	for _, s := range specs {
		fam[s.Family()]++
	}
	return fam
}

// Canonical expresses the hand-coded skeletons this 4-primitive grammar can
// represent (4 of the 6: shedding, climbing, banking, casino). Trick-taking and
// rummy need two more move-generators (follow-suit-trick, draw-meld-discard) --
// each of which further multiplies the family space.
func Canonical() []GameSpec {
	return []GameSpec{
		{Players: 4, Deal: 7, Shared: 1, Move: PlayMatch, Match: MatchEither, End: EmptyHand, Score: FirstOut}, // shedding
		{Players: 4, Deal: 7, Shared: 0, Move: BeatOrPass, End: EmptyHand, Score: FirstOut},                    // climbing
		{Players: 4, Deal: 0, Shared: 3, Move: Accumulate, Target: 21, End: Bust, Score: ClosestTarget},        // banking
		{Players: 4, Deal: 4, Shared: 4, Move: Capture, End: DeckOut, Score: MostCaptured},                     // casino
	}
}

// CanonicalFamilies marks the structural families of the canonical skeletons, so
// the driver can report how many enumerated families are NOVEL.
func CanonicalFamilies() map[string]bool {
	out := map[string]bool{}
	for _, s := range Canonical() {
		out[s.Family()] = true
	}
	return out
}

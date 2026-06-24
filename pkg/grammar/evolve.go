package grammar

import "math/rand/v2"

// This file is the search machinery over the typed composition space: genetic
// operators that stay inside the well-typed (playable-by-construction) manifold by
// construction, plus the composition key the judge-in-loop verdict table is keyed
// on. Because every operator output is re-checked against GameSpec.WellTyped,
// evolution can NEVER produce a broken or off-type game -- the v1 desert is
// unreachable, so the whole population is always playable.

// Composition is the verdict-table key for the judge-in-loop: the structural
// identity of a game (move/match/end/score + sorted modifier set), independent of
// fine params. It is the grammar's analogue of v2's evolution.Composition
// (skeleton id + sorted mechanic ids); params don't change novelty, so one verdict
// covers a whole lineage of that composition.
func (s GameSpec) Composition() string { return s.Family() }

// randomizeParams resamples the fine params within the move-gen's valid ranges.
func randomizeParams(s GameSpec, rng *rand.Rand) GameSpec {
	deals := dealFor(s.Move)
	s.Deal = deals[rng.IntN(len(deals))]
	s.Shared = sharedFor(s.Move)
	targets := targetsFor(s.Move)
	s.Target = targets[rng.IntN(len(targets))]
	return s
}

// canonicalBase returns the i-th canonical base with no mods and randomized
// players -- the structural seed a mutation/crossover builds from.
func canonicalBase(i int, rng *rand.Rand) GameSpec {
	b := Canonical()[i]
	b.Mods = nil
	b.Players = 2 + rng.IntN(3) // 2..4
	return randomizeParams(b, rng)
}

// RandomSpec draws a uniformly-random WELL-TYPED spec: a random canonical base,
// random params, and a random compatible modifier subset (<= modCap).
func RandomSpec(rng *rand.Rand) GameSpec {
	s := canonicalBase(rng.IntN(len(Canonical())), rng)
	for _, m := range compatibleMods(s) {
		if len(s.Mods) < modCap && rng.IntN(2) == 0 {
			s.Mods = append(s.Mods, m)
		}
	}
	if !s.WellTyped() { // params/mods can't break typing, but be defensive
		s.Mods = nil
	}
	return s
}

// Mutate returns a WELL-TYPED neighbour of s: it swaps the base family, adds or
// removes a modifier, or resamples params. It retries until the result is
// well-typed AND structurally different from s, falling back to s unchanged.
func Mutate(s GameSpec, rng *rand.Rand) GameSpec {
	for attempt := 0; attempt < 24; attempt++ {
		m := s
		switch rng.IntN(4) {
		case 0: // swap base family, carrying compatible mods over
			nb := canonicalBase(rng.IntN(len(Canonical())), rng)
			for _, mod := range s.Mods {
				if mod.CompatibleWith(nb) && len(nb.Mods) < modCap {
					nb.Mods = append(nb.Mods, mod)
				}
			}
			m = nb
		case 1: // add a compatible modifier not already present
			var avail []Modifier
			for _, c := range compatibleMods(m) {
				if !m.hasMod(c) {
					avail = append(avail, c)
				}
			}
			if len(avail) > 0 && len(m.Mods) < modCap {
				m.Mods = append(cloneMods(m.Mods), avail[rng.IntN(len(avail))])
			}
		case 2: // drop a modifier
			if len(m.Mods) > 0 {
				i := rng.IntN(len(m.Mods))
				m.Mods = append(cloneMods(m.Mods[:i]), m.Mods[i+1:]...)
			}
		case 3: // resample params
			m = randomizeParams(m, rng)
			m.Players = 2 + rng.IntN(3)
		}
		if m.WellTyped() && !sameSpec(m, s) {
			return m
		}
	}
	return s
}

// Crossover breeds a child WELL-TYPED spec: the base comes from a, the modifier
// set is a random a-compatible subset of both parents' mods, and players may come
// from either parent.
func Crossover(a, b GameSpec, rng *rand.Rand) GameSpec {
	child := a
	child.Mods = nil
	seen := map[Modifier]bool{}
	for _, m := range append(cloneMods(a.Mods), b.Mods...) {
		if seen[m] || len(child.Mods) >= modCap || !m.CompatibleWith(child) {
			continue
		}
		if rng.IntN(2) == 0 {
			seen[m] = true
			child.Mods = append(child.Mods, m)
		}
	}
	if rng.IntN(2) == 0 {
		child.Players = b.Players
	}
	if !child.WellTyped() {
		child.Mods = nil
	}
	return child
}

func cloneMods(m []Modifier) []Modifier { return append([]Modifier{}, m...) }

// sameSpec compares the structural identity plus the fine params that move the
// metrics (players/deal/target) -- the granularity a mutation should change.
func sameSpec(a, b GameSpec) bool {
	return a.Family() == b.Family() && a.Players == b.Players &&
		a.Deal == b.Deal && a.Target == b.Target
}

package grammar

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/sim"
)

// TestAdapterRunsInRealEngine: a grammar spec plugs into the REAL simulation
// engine (sim.RunBatch via sim.GenericRunner), not just the prototype harness.
// Every game must complete with a real winner -- no errors, no timeouts, no
// stuck/no_moves exits -- and emit the event taxonomy the fitness metrics read.
func TestAdapterRunsInRealEngine(t *testing.T) {
	specs := append(Canonical(), EnumerateModified()...)
	for i, s := range specs {
		res := sim.RunBatch(SpecGenome(s), Adapter{s}, &sim.RandomAI{}, 30, uint64(i)*1000+1)
		if res.Completions != res.GamesPlayed {
			t.Errorf("%s: only %d/%d games completed (errors=%d timeouts=%d)",
				s.Family(), res.Completions, res.GamesPlayed, res.Errors, res.Timeouts)
		}
		if res.Errors != 0 || res.Timeouts != 0 {
			t.Errorf("%s: errors=%d timeouts=%d (must be 0 -- playable-by-construction)",
				s.Family(), res.Errors, res.Timeouts)
		}
		// the winner must be a real seat in range
		seated := 0
		for _, w := range res.WinCounts {
			seated += w
		}
		if seated != res.GamesPlayed {
			t.Errorf("%s: win counts sum to %d, want %d", s.Family(), seated, res.GamesPlayed)
		}
		// events emitted (Meaningful Decisions / Interaction metrics consume these)
		anyEvents := false
		for _, evs := range res.AllEvents {
			if len(evs) > 0 {
				anyEvents = true
				break
			}
		}
		if !anyEvents {
			t.Errorf("%s: no events emitted", s.Family())
		}
	}
}

// TestSpecGenomeSkeleton pins the best-fit skeleton mapping (drives the fitness
// layer's Interaction delta mode).
func TestSpecGenomeSkeleton(t *testing.T) {
	for _, s := range Canonical() {
		g := SpecGenome(s)
		if g.Players != s.Players {
			t.Errorf("%s: SpecGenome players=%d, want %d", s.Family(), g.Players, s.Players)
		}
		if g.HandSize < 1 {
			t.Errorf("%s: SpecGenome HandSize=%d zeroes the MaxTurns cap", s.Family(), g.HandSize)
		}
	}
}

// TestAdapterEventsAreLegible: the event stream is the judge dossier's game
// trace, so every move must leave a line a reader can follow. A pass/stick used
// to emit NOTHING (a 31-move climbing game with 17 passes showed 14 plays that
// read as players acting out of turn), every betting action read "bet", a bid
// hid its amount and a nominated eight hid the suit it named. The new details
// must stay metric-inert: none of them may count as an attack (sim.IsAttackEvent
// is the only event consumer in the fitness layer).
func TestAdapterEventsAreLegible(t *testing.T) {
	want := map[sim.MoveType]string{
		sim.MoveCheck: "check", sim.MoveCall: "call", sim.MoveRaise: "raise", sim.MoveFold: "fold",
	}
	specs := append(Canonical(),
		GameSpec{Players: 4, Deal: 13, Move: Trick, End: DeckOut, Score: MostCaptured, Mods: []Modifier{ModBid}},
		GameSpec{Players: 3, Deal: 7, Shared: 1, Move: PlayMatch, Match: MatchEither, End: EmptyHand, Score: FirstOut, Mods: []Modifier{ModRunPlay, ModNominate}},
	)
	seen := map[string]bool{}
	for _, s := range specs {
		a := Adapter{s}
		g := SpecGenome(s)
		for seed := uint64(1); seed <= 30; seed++ {
			rng := rand.New(rand.NewPCG(seed, 11))
			gs := a.Setup(g, rng)
			for a.CheckEnd(gs, g) < 0 {
				a.Upkeep(gs, g)
				if a.CheckEnd(gs, g) >= 0 {
					break
				}
				moves := a.GenerateMoves(gs, g)
				m := moves[rng.IntN(len(moves))]
				evs := a.ApplyMove(gs, m, g)
				detail := ""
				for _, e := range evs {
					if e.Type != sim.EventRoundEnd && e.Type != sim.EventTrickWon {
						detail = e.Detail
						break
					}
				}
				switch {
				case m.Type == sim.MovePass:
					wantD := "pass"
					if s.Move == Accumulate {
						wantD = "stick"
					}
					if detail != wantD {
						t.Fatalf("%s: a pass emitted detail %q (events %v), want %q", s.Family(), detail, evs, wantD)
					}
					seen[wantD] = true
				case m.Type == sim.MoveBid:
					if wantD := fmt.Sprintf("bid=%d", m.Amount); detail != wantD {
						t.Fatalf("%s: bid emitted detail %q, want %q", s.Family(), detail, wantD)
					}
					seen["bid"] = true
				case want[m.Type] != "":
					if detail != want[m.Type] {
						t.Fatalf("%s: betting move %d emitted detail %q, want %q", s.Family(), m.Type, detail, want[m.Type])
					}
					seen[detail] = true
				case m.Type == sim.MovePlay && s.hasMod(ModNominate) && int(m.Cards[len(m.Cards)-1].Rank) == wildRank:
					if wantD := "names_suit=" + sim.Suit(m.Amount).String(); detail != wantD {
						t.Fatalf("%s: nominated eight emitted detail %q, want %q", s.Family(), detail, wantD)
					}
					seen["names_suit"] = true
				}
				for _, e := range evs {
					switch e.Detail {
					case "skip", "draw_two", "reverse": // the pre-existing attack specials
						continue
					}
					if e.Type != sim.EventTrickWon && sim.IsAttackEvent(e, 4) {
						t.Fatalf("%s: legibility event %+v counts as an attack (would shift Interaction)", s.Family(), e)
					}
				}
			}
		}
	}
	for _, d := range []string{"pass", "stick", "bid", "check", "call", "raise", "fold", "names_suit"} {
		if !seen[d] {
			t.Errorf("sweep never exercised a %q event (test is not covering it)", d)
		}
	}
}

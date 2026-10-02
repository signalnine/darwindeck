package evolution

import (
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
)

// Clone-group keep rule. When several individuals are one published game
// (equal outputHash), AllQualified kept the member with the highest
// OutputRank. In the hybrid engine the group is typically a frozen archive
// SNAPSHOT (the genome's estimate at admission, often a single evaluation)
// plus the LIVE population member whose running mean kept accumulating -- and
// "highest rank" then prefers whichever estimate is luckiest, i.e. the
// snapshot whenever the mean regressed. That is the winner's curse the
// running-mean scheme exists to kill, reintroduced at publication (2/39
// published games in an 8-generation run). The better-ESTIMATED member (more
// evaluations) wins; rank only breaks ties between equally-sampled members.

func TestAllQualifiedPrefersBetterEstimatedCloneMember(t *testing.T) {
	base := distinctShedding(0)
	live := &NoveltyIndividual{
		Individual: *mkEvaluated(cloneWithID(base, "live"), 1.50, 3, 0, 0), // 3 evals, mean 0.50
		Behavior:   BehaviorDescriptor{0.2, 0.2},
	}
	snapshot := &NoveltyIndividual{
		Individual: *mkEvaluated(cloneWithID(base, "snapshot"), 0.60, 1, 0, 0), // 1 lucky eval, 0.60
		Behavior:   BehaviorDescriptor{0.4, 0.4},
	}
	e := &NoveltyEngine{
		Population: []*NoveltyIndividual{live},
		Archive:    []*NoveltyIndividual{snapshot},
	}

	inds, behaviors := e.AllQualified()

	if len(inds) != 1 || len(behaviors) != 1 {
		t.Fatalf("clone group must collapse to 1, got %d individuals / %d behaviors", len(inds), len(behaviors))
	}
	if inds[0].Genome.ID != "live" {
		t.Errorf("kept %q (evals=%d, mean %.2f), want the live member (3 evals, mean 0.50) over the 1-eval snapshot",
			inds[0].Genome.ID, inds[0].EvalCount, inds[0].OutputRank())
	}
	if behaviors[0] != (BehaviorDescriptor{0.2, 0.2}) {
		t.Errorf("behavior %v does not belong to the kept individual", behaviors[0])
	}

	// Order independence: the snapshot seen FIRST must still lose.
	e = &NoveltyEngine{Population: []*NoveltyIndividual{snapshot, live}}
	if inds, _ = e.AllQualified(); len(inds) != 1 || inds[0].Genome.ID != "live" {
		t.Errorf("with the snapshot first, kept %q, want live", inds[0].Genome.ID)
	}
}

func TestMAPElitesAllQualifiedPrefersBetterEstimatedCloneMember(t *testing.T) {
	base := distinctShedding(0)
	archive := &Archive{}
	// The same game in two cells: one admitted on a single lucky evaluation,
	// one challenged three more times and dragged toward its true mean.
	archive.Cells[0][0] = &ArchiveCell{Individual: mkEvaluated(cloneWithID(base, "lucky"), 0.60, 1, 0, 0)}
	archive.Cells[1][1] = &ArchiveCell{Individual: mkEvaluated(cloneWithID(base, "settled"), 2.20, 4, 0, 0)} // 0.55
	e := &MAPElitesEngine{Archives: map[genome.SkeletonType]*Archive{genome.Shedding: archive}}

	out := e.AllQualified()

	if len(out) != 1 {
		t.Fatalf("clone pair must collapse to 1, got %d", len(out))
	}
	if out[0].Genome.ID != "settled" {
		t.Errorf("kept %q (evals=%d), want the 4-evaluation occupant over the 1-evaluation one", out[0].Genome.ID, out[0].EvalCount)
	}
}

// TestGenomeHashIgnoresCardScoringEvent: no runner, hook or rulebook reads
// CardScoring.Event (MatchCardPoints matches on rank and suit alone), so two
// genomes differing only in it are the same game. genomeHash serialized the
// field, so event-only variants defeated population dedup and took separate
// output slots.
func TestGenomeHashIgnoresCardScoringEvent(t *testing.T) {
	mk := func(ev genome.ScoringEvent) *genome.Genome {
		return &genome.Genome{
			Skeleton: genome.TrickTaking, Players: 4, HandSize: 13,
			TrickTaking: &genome.TrickTakingParams{
				MustFollowSuit: true, TrickScoring: genome.ScoreAvoidance, RoundsPerGame: 1,
			},
			Scoring: genome.ScoringConfig{CardPoints: []genome.CardScoring{
				{Rank: 0, Suit: 3, Points: 1, Event: ev},
				{Rank: 12, Suit: 4, Points: 13, Event: ev},
			}},
		}
	}
	a, b := mk(genome.ScoreOnTrickWin), mk(genome.ScoreOnHandEnd)
	if !a.LiveCardPoints() {
		t.Fatal("fixture: card_points must be live so outputHash keeps them")
	}
	if genomeHash(a) != genomeHash(b) {
		t.Errorf("genomeHash distinguishes genomes that differ only in the never-read card_points event:\n%s\n%s", genomeHash(a), genomeHash(b))
	}
	if outputHash(a) != outputHash(b) {
		t.Error("outputHash distinguishes genomes that differ only in the never-read card_points event")
	}

	// The fields that ARE read still separate genomes.
	c := mk(genome.ScoreOnTrickWin)
	c.Scoring.CardPoints[1].Points = 5
	if genomeHash(a) == genomeHash(c) {
		t.Error("genomeHash must still distinguish different card_points values")
	}
	// Canonicalization must not touch the genome itself (the JSON format is
	// unchanged).
	if b.Scoring.CardPoints[0].Event != genome.ScoreOnHandEnd {
		t.Error("hashing mutated the genome's card_points event")
	}
}

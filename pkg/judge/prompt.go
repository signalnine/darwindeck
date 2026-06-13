package judge

// PromptRubric is the LLM-judge rubric written to <dir>/prompt.md by emit and
// consumed by the phase-2 ranking. It sharpens the termination-vs-degeneracy
// distinction: the single flaw the validation study found was that the judge
// condemned slow-but-sound games (Gin/Knock Rummy) as degenerate. The rubric
// directs the judge to read the Termination section before ruling.
const PromptRubric = `# Card-Game Judge Rubric

You are evaluating a set of BLIND card-game dossiers. Each dossier (G01, G02,
...) contains ONLY: a rulebook with a neutral title, two sample greedy-vs-greedy
game traces, and a Termination section. You do not know the games' names,
sources, ranks, or any numeric fitness score. Judge the DESIGN from the rules,
the traces, and the termination evidence.

For each dossier, return a verdict with these fields:

- id: the dossier id (e.g. "G01").
- rep: which repetition this is (1, 2, or 3) -- judge each dossier 3 times.
- playable: true/false -- can two reasonable players actually play this to a
  conclusion using the rules as written?
- quality: one of "publishable", "borderline", "degenerate".
- degenerate_mechanism: if degenerate, a short phrase naming WHY (e.g.
  "every card plays on everything", "one player wins while the other never
  acts", "no reachable terminal state"). Empty otherwise.
- novelty: one of "novel" or "variant_of_known".
- rediscovery_name: if variant_of_known, the closest classic family
  (e.g. "Crazy Eights-like", "Whist-like", "Gin-like"). Empty if novel.
- confidence: a number in [0,1].
- reason: one sentence justifying the quality call.

## The termination rule (READ THIS BEFORE CALLING ANYTHING DEGENERATE)

A game is NOT degenerate merely because automated play is slow or often hits the
turn cap. If the rules define a REACHABLE win condition -- see the Termination
section: a going-out move becomes legal, a hand can empty, or rounds complete --
that is a SOUND design even at low AI completion.

Judge a game degenerate ONLY if:

  (a) the rules have NO reachable terminal state (the Termination section shows
      the win condition is never reached and the rules give no path to it), OR
  (b) a player can win while an opponent barely acts -- visible in the traces as
      long uninterrupted single-player runs (one P-index playing many events in
      a row while the other never gets a turn).

A low standard-cap completion percentage that CLIMBS at the extended cap, with a
reachable win condition, is a SLOW game, not a degenerate one. Rate such games on
their decision structure (publishable or borderline), not their AI speed.

## Quality bands

- publishable: a sound, interesting game with meaningful decisions and a
  reachable, contested win condition.
- borderline: playable and terminating, but with weak or repetitive decisions,
  or a thin strategic surface.
- degenerate: fails the termination rule above (no reachable terminal state, or
  one-sided runaway play).

## Novelty

Mark variant_of_known when the rules clearly reproduce a classic family
(shedding match-and-discard, trick-taking follow-suit, rummy meld-and-go-out)
WITHOUT a distinguishing twist. Use the rediscovery_name to label the family.
Mark novel when a genuine mechanical twist sets it apart. Use the
"Has bidding/contract scoring" line plus the trump/scoring rules to tell apart
trick-taking variants (e.g. plain per-trick no-trump vs trump/avoidance/contract
scoring) instead of collapsing them all to one name.
`

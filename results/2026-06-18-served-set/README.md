# Served set (cards.signalnine.net), 2026-06-18

The 10 games served by the public web playtest, for fun-validation. Each genome's
`id` is an invented title and `description` is a one-line pitch (see
`pkg/genome` Description field). Titles are uniform-style on purpose: the three
classics are blind-mixed with the seven evolved games so ratings aren't biased by
name recognition. `catalog.json` maps title -> {kind, skeleton, players, orig_id}.

- **evolved (7):** Cascade, Standoff, Black Deuce, Knockabout, Tightrope, Tidewater, Showhand
- **classic anchors (3):** Wildfire (Crazy Eights), Tenline (Gin Rummy), Highrise (Big Two)

Pull + analyze ratings: `scripts/ratings-report.sh`.

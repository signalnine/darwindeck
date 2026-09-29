# Genome Schema Reference (v2)

**Date:** 2026-09-28
**Source of truth:** `pkg/genome/genome.go` (types), `pkg/genome/validate.go` (Tier-0 rules), `pkg/genome/liveness.go` (which genes actually affect outcome)

This is the `genome.json` format read by `darwindeck playtest|describe|serve`, `evolve -seed-dir`, and `judge emit`, and written by `evolve` and `seeds export`. It replaces the v1 Python schema in `docs/genome-schema-examples.md`.

A v2 genome never encodes game logic. It picks one of six skeleton runners (each playable by construction) and parameterizes it, plus optional cross-skeleton borrows, special cards, and card scoring. For the design rationale see `docs/deep-dive-genome-and-evolution.md`.

## Encoding conventions

- **Enums are bare integers** in JSON (no string names). Enum types have no custom JSON marshalling, and the integer values are part of the file format: reserved values are kept, not deleted, so that old files keep decoding correctly.
- **Ranks:** 2-10 as-is, J=11, Q=12, K=13, A=14. `0` means "any rank" where a filter is allowed.
- **Suits in genome fields are 1-indexed:** 1=Clubs, 2=Diamonds, 3=Hearts, 4=Spades. `0` means "any suit". Internally `sim.Suit` is 0-indexed; `MatchCardPoints` and `SpecialCard.MatchesCard` do the conversion.
- **Unknown JSON fields are ignored** (`json.Unmarshal` without `DisallowUnknownFields`), and fields that are missing default to zero values. The CLI loaders (`playtest`, `describe`, `serve`, `evolve -seed-dir`) run `genome.Validate` after decoding and reject a genome with any error.

## Top-level fields

| Field | Type | Range / rule | Notes |
|---|---|---|---|
| `id` | string | | Free-form identifier. |
| `description` | string | optional | One-line pitch shown in the `serve` lobby. Cosmetic; never read by evolution or metrics. |
| `generation` | int | | Generation the genome was born in (0 for seeds). |
| `skeleton` | enum | 0-5 | 0 shedding, 1 trick_taking, 2 rummy, 3 climbing, 4 casino, 5 vying. |
| `players` | int | 2-6 | |
| `hand_size` | int | 3-13 | `hand_size * players <= 52`. Casino also adds `table_size` (see below). |
| `shedding` / `trick_taking` / `rummy` / `climbing` / `casino` / `vying` | object | | Exactly the one matching `skeleton` must be present; the others are ignored. |
| `borrowed` | array | optional | Cross-skeleton mechanics. See [Borrowed mechanics](#borrowed-mechanics). |
| `special_cards` | array | optional, shedding only | See [Special cards](#special-cards). |
| `scoring` | object | | `card_points` and `trump_suit`. See [Scoring](#scoring). |
| `trump_rule` | enum | 0-3 | 0 none, 1 fixed, 2 cut, 3 led. Rejected on rummy, climbing, casino, vying. |

**Output-only metadata** (written into published `genome.json`, never read by selection or metrics):

| Field | Meaning |
|---|---|
| `fitness` | Raw (unshared) fitness. In published files, the greedy-only running mean, matching `report.md`. |
| `shared_fitness` | The niche-sharing/novelty-blended selection score, kept separate so it never masquerades as fitness. |
| `veto_stable` | True iff a majority of K=5 fresh-seed re-evaluations stayed valid (`pkg/output/stability.go`). |
| `stable_evals` | The literal count, e.g. `"5/5"`. A minority failure still publishes as stable but shows here. |

## Skeleton parameters

### `shedding` (skeleton 0) -- Crazy Eights, Mau-Mau

Empty your hand first by discarding onto a pile that each card must match.

| Field | Type | Range | Notes |
|---|---|---|---|
| `match_rule` | enum | 0-2 | 0 suit, 1 rank, 2 either. 3 (both) is rejected as statically unplayable (only the top card itself matches). |
| `draw_penalty` | int | 1-3 | Cards drawn when you can't match. |
| `rounds_per_game` | int | 0-5 | 0 = unset, which means 1. Values > 1 only take effect with a banking borrow (`meld_bonus`, `avoidance`, `trick_scoring`): the game becomes banked-score rounds, highest total wins (`Genome.SheddingMultiRound`). Without one, the game stays single-round. |

### `trick_taking` (skeleton 1) -- Whist, Hearts, Spades, Oh Hell

| Field | Type | Range | Notes |
|---|---|---|---|
| `must_follow_suit` | bool | | |
| `trick_scoring` | enum | 0-2 | 0 per_trick, 1 card_points, 2 avoidance (points are bad, Hearts-style). 1 and 2 require non-empty `scoring.card_points`. |
| `lead_restriction` | enum | 0-1 | 0 none, 1 no_trump_until_broken. 2 (winner_leads) is reserved and rejected: winner-leads is already the fixed turn order. |
| `rounds_per_game` | int | 1-13 | |

Trump (`trump_rule` + `scoring.trump_suit`) is only consulted by this skeleton: fixed uses `trump_suit`, cut takes the suit of the top card of the pre-deal deck, led makes the first suit led trump.

### `rummy` (skeleton 2) -- Gin Rummy, Knock Rummy

| Field | Type | Range | Notes |
|---|---|---|---|
| `meld_types` | enum | 0-2 | 0 sets, 1 runs, 2 both. |
| `min_meld_size` | int | 3-4 | 2 is rejected (a 2-card meld is trivially formable). |
| `draw_from` | enum | 0-2 | 0 deck, 1 discard, 2 either. |
| `knock_threshold` | int | 0-100 | Max deadwood to knock; 0 = gin only. |

### `climbing` (skeleton 3) -- Big Two

Beat the current combination with a higher one of the same type, or pass. When everyone else passes, the last player to play leads fresh. First to empty their hand wins. Singles are always allowed, so a lead always has a legal play.

| Field | Type | Range | Notes |
|---|---|---|---|
| `allow_pairs` | bool | | |
| `allow_triples` | bool | | |
| `allow_runs` | bool | | Mixed-suit consecutive-rank runs. |
| `min_run_len` | int | 3-5 | Required when `allow_runs`; otherwise 0 (unset) or 3-5. |

### `casino` (skeleton 4) -- simplified Casino / Scopa

Play one card and either capture table cards or trail it onto the table (trailing is always legal). Hands refill from the stock until it runs out; the last capturer sweeps the table. Most captured cards wins.

| Field | Type | Range | Notes |
|---|---|---|---|
| `table_size` | int | 0-6 | Face-up cards dealt at setup. `hand_size * players + table_size <= 52`. |
| `allow_sum_capture` | bool | | Also capture number cards whose pips sum to the played card. Off = rank-match only. |

### `vying` (skeleton 5) -- poker

Each deal: hidden hands, a rotating big blind of `min_bet`, one betting round (fold / call / raise), best poker hand among non-folders takes the pot. Chips carry across deals; largest stack wins.

| Field | Type | Range | Notes |
|---|---|---|---|
| `starting_chips` | int | > 0 | Must be >= `rounds_per_game * min_bet * (max_raises + 1)` so no all-in or side pot can arise. |
| `min_bet` | int | > 0 | Big blind and raise increment. |
| `max_raises` | int | >= 1 | Per betting round; guarantees the round closes. |
| `rounds_per_game` | int | >= 1 | Deals played. |

`hand_size` must also be >= 2 (enforced by the vying validator; the global floor of 3 already covers it).

## Special cards

Shedding only (rejected on every other skeleton).

```json
{ "type": 4, "by_rank": 8 }
```

| Field | Type | Notes |
|---|---|---|
| `type` | enum | 0 skip, 1 reverse, 2 draw_two, 3 draw_four, 4 wild. |
| `by_rank` | uint8 | 0 = any, else 2-14. |
| `by_suit` | uint8 | 0 = any, else 1-4. |

At least one of `by_rank` / `by_suit` must be set: a catch-all (both 0) would match every card and delete the skeleton's match/draw rules, so it is rejected.

## Scoring

```json
"scoring": {
  "card_points": [ { "rank": 0, "suit": 3, "points": 1, "event": 0 } ],
  "trump_suit": 4
}
```

| Field | Type | Notes |
|---|---|---|
| `card_points[].rank` | uint8 | 0 = any, else 2-14. |
| `card_points[].suit` | uint8 | 0 = any, else 1-4. |
| `card_points[].points` | int | |
| `card_points[].event` | enum | 0 trick_win, 1 capture, 2 play, 3 hand_end. **Not consulted by any runner today**: card points apply the same way regardless of this value. |
| `trump_suit` | uint8 | 1-4; required when `trump_rule` is 1 (fixed). |

When several `card_points` rules match one card, the most specific wins (suit+rank > suit > rank > catch-all), so rule order never matters.

`card_points` are only live under trick-taking `card_points`/`avoidance` scoring or a live `avoidance` borrow (`Genome.LiveCardPoints`). Otherwise they are dead genes: evolution's output dedup strips them, and the rulebook doesn't print them.

## Borrowed mechanics

```json
"borrowed": [ { "source": 3, "mechanic": 8 } ]
```

`source` is the donor skeleton (same 0-5 enum; must differ from the genome's own `skeleton`). `mechanic`:

| Value | Name | Depth | What it does |
|---|---|---|---|
| 0 | `trick_scoring` | shallow (banking) | Banks a per-round capture bonus into scores. |
| 1 | `meld_bonus` | shallow (banking) | Banks a bonus for sets/runs at round end. |
| 2 | `draw_penalty` | shallow (direct) | Extra draw penalty; always live. |
| 3 | `knock` | **deep** | Knock when your hand is small to end the game; fewest cards wins, so a wrong knock hands someone else the win. |
| 4 | `trump` | reserved | No implementation; rejected. |
| 5 | `avoidance` | shallow (banking) | Penalty cards (uses `scoring.card_points`; inert without them). |
| 6 | `play_multiple` | reserved | No implementation; rejected. |
| 7 | `follow_suit` | **deep** | Must play the discard's suit if you hold it (restricts moves). |
| 8 | `run_play` | **deep** | Dump a same-rank set or same-suit run (2+) in one turn (expands moves). |

Shallow borrows are hooks that fire after moves and adjust scores. Deep borrows live inside the skeleton runner and change the legal-move set or the win condition.

**Whitelist** (`ValidBorrows()` in `pkg/genome/validate.go`):

| Host skeleton | Allowed mechanics |
|---|---|
| shedding | trick_scoring, meld_bonus, knock, avoidance, follow_suit, run_play |
| trick_taking | meld_bonus, avoidance |
| rummy | trick_scoring, draw_penalty, avoidance |
| climbing | draw_penalty, knock |
| casino | meld_bonus, avoidance |
| vying | meld_bonus, avoidance |

Extra rules:
- No duplicate mechanics, even from different sources: their hooks would apply twice.
- `avoidance` on trick_taking requires `trick_scoring` = 0 (per_trick). With native card-point scoring, the borrow cancels the points exactly and seat 0 always wins.
- `follow_suit` on shedding requires `match_rule` suit or either. Under rank matching, the obligation collapses into all-draw and the game can't finish.
- Banking borrows (`meld_bonus`, `avoidance`, `trick_scoring`) on a single-round shedding host are dead: nothing reads the banked scores. `LiveBorrows()` prunes them from the rulebook and from output dedup.

## Examples

Classic seeds, exported with `darwindeck seeds export -out <dir>` (source: `pkg/seeds/`):

**Crazy Eights** (shedding; match suit or rank; 8s are wild):

```json
{
  "id": "crazy-eights",
  "generation": 0,
  "skeleton": 0,
  "players": 2,
  "hand_size": 7,
  "shedding": { "match_rule": 2, "draw_penalty": 1 },
  "special_cards": [ { "type": 4, "by_rank": 8 } ],
  "scoring": {},
  "trump_rule": 0
}
```

**Hearts** (trick-taking avoidance; each heart 1 point, Q of spades 13; can't lead hearts until broken):

```json
{
  "id": "hearts",
  "generation": 0,
  "skeleton": 1,
  "players": 4,
  "hand_size": 13,
  "trick_taking": {
    "must_follow_suit": true,
    "trick_scoring": 2,
    "lead_restriction": 1,
    "rounds_per_game": 1
  },
  "scoring": {
    "card_points": [
      { "rank": 0, "suit": 3, "points": 1, "event": 0 },
      { "rank": 12, "suit": 4, "points": 13, "event": 0 }
    ]
  },
  "trump_rule": 0
}
```

The other skeletons, parameter block only:

```json
"rummy":    { "meld_types": 2, "min_meld_size": 3, "draw_from": 2, "knock_threshold": 10 }
"climbing": { "allow_pairs": true, "allow_triples": true, "allow_runs": true, "min_run_len": 3 }
"casino":   { "table_size": 4, "allow_sum_capture": true }
"vying":    { "starting_chips": 1000, "min_bet": 10, "max_raises": 3, "rounds_per_game": 12 }
```

(Gin Rummy 2p/10 cards, Big Two 4p/13, Casino 2p/4, SimplePoker 4p/5.)

**An evolved cross-skeleton hybrid**, `results/2026-06-14-evolved-novel-hybrids/rank07_gen60_76619/genome.json`. This is a 3-round shedding game with three borrows: `run_play` from climbing (deep), `meld_bonus` from rummy, and `avoidance` from trick-taking with hearts as penalty cards. The borrows are what make `rounds_per_game: 3` live.

```json
{
  "id": "gen60_76619",
  "generation": 60,
  "skeleton": 0,
  "fitness": 0.6698696054259943,
  "shared_fitness": 0.4709487016349936,
  "veto_stable": true,
  "stable_evals": "5/5",
  "players": 4,
  "hand_size": 10,
  "shedding": { "match_rule": 2, "draw_penalty": 1, "rounds_per_game": 3 },
  "borrowed": [
    { "source": 3, "mechanic": 8 },
    { "source": 2, "mechanic": 1 },
    { "source": 1, "mechanic": 5 }
  ],
  "special_cards": [
    { "type": 4, "by_rank": 8 },
    { "type": 2, "by_rank": 2 },
    { "type": 1, "by_rank": 10 },
    { "type": 2, "by_suit": 3 },
    { "type": 4, "by_rank": 11, "by_suit": 3 },
    { "type": 3, "by_rank": 11, "by_suit": 4 }
  ],
  "scoring": {
    "card_points": [ { "rank": 0, "suit": 3, "points": 1, "event": 0 } ]
  },
  "trump_rule": 0
}
```

`./bin/darwindeck describe <genome.json>` prints `Validation: OK` or the list of errors; `rulebook.md` next to a published genome is the natural-language rendering.

## Not in the genome

The generative grammar (`pkg/grammar`) uses its own `GameSpec` composition type (move-generator x end-condition x scoring + modifiers), not this schema. Grammar specs reach the fitness layer through a carrier genome built by `SpecGenome` (`pkg/grammar/adapter.go`). See the Generative Grammar section of `CLAUDE.md`.

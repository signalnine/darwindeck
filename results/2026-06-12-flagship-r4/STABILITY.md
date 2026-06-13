# Veto-stability restamp (Wave M)

Each game below was re-evaluated K=5 times at fresh seeds through the
default greedy-only pipeline (Tier 0/1 + the degeneracy vetoes). A game is
VETO-STABLE iff a majority (>=3/5) of those re-evals stayed valid; unstable
games are demoted below every stable game in the published rank order.
`fresh_eval` is the single-seed evaluation at the restamp seed -- the
production-equivalent published draw -- shown next to the K=5 verdict so a
single-eval/multi-eval disagreement (the bug this closes) is visible.

Re-evaluation cost: 30 games x 5 evals + fresh eval in 9.433s.

| Rank | Genome | Skeleton | Fresh greedy fitness | Fresh eval | Veto-stable | Stable evals | Failing reasons |
|------|--------|----------|----------------------|------------|-------------|--------------|------------------|
| 1 | gen200_15931 | shedding | 0.721 | valid | yes | 5/5 | - |
| 2 | gen200_93822 | shedding | 0.715 | valid | yes | 5/5 | - |
| 3 | gen200_71693 | shedding | 0.709 | valid | yes | 5/5 | - |
| 4 | gen200_39793 | trick_taking | 0.707 | valid | yes | 5/5 | - |
| 5 | gen185_36417 | shedding | 0.702 | valid | yes | 4/5 | veto:greedy_longest_run |
| 6 | gen199_93368 | shedding | 0.698 | valid | yes | 4/5 | veto:greedy_timeout |
| 7 | gen200_47499 | trick_taking | 0.688 | valid | yes | 5/5 | - |
| 8 | gen200_12886 | trick_taking | 0.687 | valid | yes | 5/5 | - |
| 9 | gen200_59791 | trick_taking | 0.687 | valid | yes | 5/5 | - |
| 10 | gen195_65338 | trick_taking | 0.686 | valid | yes | 5/5 | - |
| 11 | gen200_37243 | trick_taking | 0.681 | valid | yes | 5/5 | - |
| 12 | gen200_15024 | shedding | 0.678 | valid | yes | 5/5 | - |
| 13 | gen187_63092 | shedding | 0.678 | valid | yes | 4/5 | veto:greedy_longest_run |
| 14 | gen195_98220 | shedding | 0.676 | valid | yes | 5/5 | - |
| 15 | gen195_28231 | trick_taking | 0.676 | valid | yes | 5/5 | - |
| 16 | gen200_51709 | shedding | 0.676 | valid | yes | 5/5 | - |
| 17 | gen200_87238 | trick_taking | 0.675 | valid | yes | 5/5 | - |
| 18 | gen200_49900 | trick_taking | 0.668 | valid | yes | 5/5 | - |
| 19 | gen199_58798 | trick_taking | 0.668 | valid | yes | 5/5 | - |
| 20 | gen200_31192 | rummy | 0.528 | valid | yes | 5/5 | - |
| 21 | gen200_9899 | rummy | 0.517 | valid | yes | 5/5 | - |
| 22 | gen200_14339 | rummy | 0.513 | valid | yes | 5/5 | - |
| 23 | gen200_68956 | rummy | 0.507 | valid | yes | 5/5 | - |
| 24 | gen200_17538 | rummy | 0.506 | valid | yes | 5/5 | - |
| 25 | gen200_80537 | rummy | 0.506 | valid | yes | 5/5 | - |
| 26 | gen199_32147 | rummy | 0.504 | valid | yes | 5/5 | - |
| 27 | gen200_87170 | rummy | 0.499 | valid | yes | 5/5 | - |
| 28 | gen200_26733 | rummy | 0.493 | valid | yes | 5/5 | - |
| 29 | gen200_55718 | shedding | 0.000 | veto:greedy_longest_run | yes | 4/5 | veto:greedy_longest_run |
| 30 | gen200_12106 | rummy | 0.000 | veto:draw_supply_churn | yes | 4/5 | veto:draw_supply_churn |


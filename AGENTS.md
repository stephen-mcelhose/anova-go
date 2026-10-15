# ANOVA for Go — Agent and Contributor Guide

## Purpose

Build a production-quality Go library in this order:

1. stable batch, unweighted, fixed-group one-way ANOVA;
2. exact append-only streaming one-way ANOVA using mergeable sufficient statistics;
3. only then consider separately specified extensions.

The normative project wiki is `wiki/`. Read these documents before changing code:

1. `wiki/00-scope-and-contract.md`
2. `wiki/02-one-way-anova.md`
3. `wiki/04-numerical-stability.md`
4. `wiki/03-streaming-one-way-anova.md`
5. `wiki/08-test-oracles.md`
6. `wiki/09-adrs.md`
7. `wiki/10-implementation-roadmap.md`

## Non-negotiable constraints

- v1 is only unweighted, fixed-group, append-only one-way ANOVA.
- Reject NaN and infinities; failed `Add` or `Merge` must not mutate state.
- Use centered moments `(count, mean, M2)` with Welford updates and Chan-style merges. Do not use raw `sum/sumSq` variance as the production path.
- Enforce `MaxExactCount = 2^53` so counts used as `float64` weights remain exactly representable.
- `Merge` assumes disjoint data partitions. Reject self-merge, but do not claim overlap detection without a provenance design.
- `Snapshot` is non-mutating. Repeated snapshot p-values are not sequential-valid decisions.
- Represent invalid, indeterminate, infinite-F, numerical-warning, and numerical-failure cases explicitly. Do not fabricate p-values.
- P-values are release-blocked until a pinned distribution adapter has passed the specified R/SciPy differential tail tests.

## Required checks for code changes

- Add or update tests mapped to `TST-*` IDs in `wiki/08-test-oracles.md`.
- Preserve batch/stream/partition-merge equivalence within defined tolerances.
- Include adversarial large-offset, imbalance, degenerate, and overflow tests whenever math changes.
- Keep public documentation explicit about ANOVA assumptions and v1 exclusions.

## Deliberately deferred

Do not introduce Welch, post-hoc tests, factor models, dynamic groups, rolling windows, repeated measures, covariance, or sequential testing without a new ADR, statistical specification, oracle strategy, and tests.

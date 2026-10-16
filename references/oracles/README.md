# F-Tail Oracle Fixtures

This directory contains independently generated reference values for the
F-distribution upper-tail probability (survival function), used to validate the
anova-go library's p-value computation against a trusted external tool.

## Purpose

Per **ADR-005** (Separate core aggregation from F-tail adapter), p-values in
`Snapshot()` are release-blocked until differential tail tests pass against an
independent oracle. The test **TST-034** (`TestOracle_FTail` in
`oracle_test.go`) reads `f_tail_fixtures.csv` and verifies that the Gonum
adapter produces p-values within **1 × 10⁻⁹ absolute tolerance** of R's `pf()`
for every fixture row.

## Files

| File                      | Description                                                        |
| ------------------------- | ------------------------------------------------------------------ |
| `generate_fixtures.R`     | R script that produces `f_tail_fixtures.csv` from `pf()`          |
| `f_tail_fixtures.csv`     | Committed oracle output; checked in and treated as source of truth |
| `README.md`               | This file — provenance, tolerance rationale, and tool versions     |

## Tool versions

| Tool   | Version                                | Platform                 |
| ------ | -------------------------------------- | ------------------------ |
| R      | 4.5.0 (2025-04-11 "How About a Twenty-Six") | aarch64-apple-darwin20 |
| Gonum  | v0.17.0 (`gonum.org/v1/gonum/stat/distuv`) | same build environment |

## Generation

To reproduce `f_tail_fixtures.csv`:

```sh
cd references/oracles
Rscript generate_fixtures.R
```

The script uses only base R's `pf()` (stats package) — no additional packages
are required.

## Fixture coverage

The 22 rows cover the fixture IDs and test-plan ranges defined in
`wiki/08-test-oracles.md`:

| Rows                                 | Coverage                                           |
| ------------------------------------ | -------------------------------------------------- |
| FX-001                               | Balanced 3×3, F=27, df=(2,6) — hand-auditable     |
| FX-002                               | Unbalanced 3-group, F=21, df=(2,6)                 |
| FX-003                               | Null-like, small F=0.375, df=(1,4)                 |
| FX-004                               | Separated means, F=121.5, df=(1,4)                 |
| FX-008                               | Large-offset (1×10¹²), F=50, df=(2,12)             |
| Boundary F=0                         | P must equal 1 — two df pairs                      |
| TST-026: small df, moderate tail     | df=(1,1), (1,5), (2,6) at critical-value F         |
| TST-027: moderate df                 | df=(5,5), (3,100), (10,100)                        |
| TST-028: far left tail               | Tiny F, P near 1 — two df pairs                    |
| TST-029: far right tail              | Large F, P near 0 — three df pairs incl. F=10⁶     |
| TST-030: alpha critical values       | 0.05 and 0.01 critical F across several df pairs   |

## Tolerance rationale

`1 × 10⁻⁹` absolute tolerance is chosen to:

1. Exceed the rounding error introduced by writing p-values with 15 significant
   digits in the CSV (error ≲ 10⁻¹⁵).
2. Remain tighter than any meaningful statistical precision (conventional
   software typically agrees to ~10⁻⁶ on F-distribution tails).
3. Match the level at which Gonum's regularized incomplete beta function and R's
   `pbeta()` are expected to agree across a broad parameter range.

If a future R or Gonum update widens the gap beyond 1 × 10⁻⁹ for any fixture,
the fixture should be investigated before relaxing the tolerance.

## Regeneration policy

Regenerate `f_tail_fixtures.csv` and update this README only when:

- The pinned R version changes (update the version table above).
- New fixture rows are added to extend coverage (update the table above).
- A statistical error is found in an existing fixture row.

After regeneration, run `go test ./... -run TestOracle_FTail` to verify
agreement before committing.

# Scope and Semantic Contract

**Status:** binding planning baseline  
**Links:** [[Concept graph]] · [[One-way ANOVA]] · [[Streaming one-way ANOVA]] · [[Architecture decisions]]

## Product statement

The initial Go package will answer one question:

> For a scalar response grouped into #$#k#$# independently sampled groups, is the observed between-group mean variation large relative to within-group variation under the classical pooled-variance model?

It returns an ANOVA table and an omnibus F-test, not a causal claim and not a post-hoc comparison result.

## Included in v1

- **Model:** unweighted fixed-effects one-way ANOVA with an intercept.
- **Input:** finite #$#float64#$# responses and group identifiers.
- **Batch API:** group slices and/or observations `(group, value)`.
- **Stream API:** append-only observations to a fixed set of registered groups; snapshots at arbitrary points; merge compatible summaries.
- **Output:** group and residual degrees of freedom, sums of squares, mean squares, F statistic, upper-tail p-value, group summary statistics, and explicit validity/status fields.
- **Inference domain:** valid only when the documented assumptions are substantively reasonable. See [[Assumptions and alternatives]].

## Explicit exclusions

| Exclusion | Why it is separate work |
|---|---|
| Welch ANOVA | Changes the test statistic, degrees-of-freedom approximation, and streaming result contract. |
| Two-way/n-way ANOVA | Requires a design matrix, contrasts, estimability/rank rules, interactions, and Type I/II/III sum-of-squares policy. |
| Repeated-measures ANOVA | Observations are correlated; subject structure and sphericity handling are required. |
| ANCOVA/regression | Covariates and model-matrix fitting require QR/SVD and rank-deficiency policy. |
| Post-hoc tests | Multiplicity policy and studentized-range distribution must be specified. |
| Rolling/sliding windows | Exact deletion needs inverse sufficient-statistic operations and retention/expiry semantics; see [[Streaming one-way ANOVA]]. |
| Missing-data imputation | Missingness is an analysis policy, not a silent numerical behavior. |

## Invariants

- **REQ-001:** reject NaN and ±Inf at ingestion; do not silently drop them.
- **REQ-002:** require #$#N>k_+#$# and at least two non-empty groups before reporting a residual mean square.
- **REQ-003:** retain empty registered groups in state but exclude them from the model rank and report them separately.
- **REQ-004:** p-values are reported only when the F distribution call is well-defined and residual degrees of freedom are positive.
- **REQ-005:** constructors copy group configuration; immutable result snapshots and returned group summaries must not expose mutable accumulator maps/slices. Results use the registered canonical group order.
- **REQ-006:** merge only states with the same semantic configuration: group universe/identity policy, non-finite-value policy, and accumulator version.
- **REQ-007:** every result carries a diagnostic status for degenerate data, such as all groups constant.
- **REQ-008:** all arithmetic errors (a count exceeding `MaxExactCount = 2^53`, non-finite intermediate, non-representable sum of squares, or F-tail failure) return a numerical-failure status and never a fabricated p-value.
- **REQ-009:** failed `Add` and `Merge` operations are atomic: the receiver is unchanged.
- **REQ-010:** `Merge` requires disjoint input partitions; this is a caller precondition except self-merge, which must be rejected.

## Degeneracy policy

The implementation must distinguish these conditions rather than manufacturing a p-value:

| Condition | Interpretation / v1 result policy |
|---|---|
| Fewer than two non-empty groups | invalid test; no F/p-value. |
| #$#df_W \le 0#$# | invalid residual estimate; no F/p-value. |
| #$#SS_W=0, SS_B>0#$# | limit F is +∞; report `+Inf` F and p-value 0 if the F CDF supports it. |
| #$#SS_W=0, SS_B=0#$# | 0/0 is indeterminate; no F/p-value. |
| finite arithmetic yields tiny negative SS from roundoff | clamp only within a documented scale-aware tolerance and surface a numerical-warning status; otherwise error. |

## Non-goals disguised as requirements

“Streaming” does **not** mean that every statistical extension can be computed from the current one-way sufficient statistics. Append-only one-way ANOVA is exact because each group can retain #$#(n,\dbar{x},M_2)#$#. Changing membership, applying weights, adding covariates, or expiring observations changes the required state and/or inferential model.

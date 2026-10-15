# Numerical Stability and Merging

**Status:** binding algorithmic guidance  
**Links:** [[One-way ANOVA]] · [[Streaming one-way ANOVA]] · [[Test oracles]]

## Failure mode to avoid

The textbook computational identity #$#\sum x^2-(\sum x)^2/n#$# subtracts nearby large values when data have a large location and small spread. In binary floating point that can lose most meaningful digits and may yield a negative variance estimate. Do not use it as the production implementation of within-group SS.

## Required algorithms

1. **Within each group:** Welford update, retaining #$#(n,\bar{x},M_2)#$#.
2. **Across partitions or groups:** Chan–Golub–LeVeque/parallel-variance merge:

#$#M_{2,A\cup B}=M_{2,A}+M_{2,B}+(\mu_B-\mu_A)^2\frac{n_An_B}{n_A+n_B}#$#

3. **Between-group SS:** merge group mean summaries #$#(n_i,\bar{x}_i,0)#$#; the final merged #$#M_2#$# equals #$#SS_B#$#.
4. **Tail probability:** use a survival-function path or a numerically stable complement, not naive #$#1-CDF(F)#$# when p is near zero.

## Numerical policies

| Policy | Requirement |
|---|---|
| Count type | Use `uint64` with checked addition, but define `MaxExactCount = 2^53`. Reject an update/merge before mutation if the resulting count exceeds `MaxExactCount`; this ensures every count converted to `float64` for mean/merge arithmetic is exactly representable. |
| Intermediate arithmetic | After every delta, mean, cross term, sum-of-squares, mean square, F, and distribution argument calculation, require a finite representable value unless the explicitly allowed final #$#F=+\infty#$# limit applies. Otherwise return numerical failure atomically. |
| Summation order | Snapshot traverses groups in a deterministic canonical order, so output is reproducible for a fixed input/update order. |
| Negative SS | For an SS #$#s<0#$#, let #$#S=\sum |t_i|#$# be the sum of absolute finite terms #$#t_i#$# used to form that SS. Clamp only when #$#S>0#$# and #$#|s|\le32\epsilon S#$#, where #$#\epsilon#$# is `float64` machine epsilon. If #$#S=0#$# or the bound is exceeded, return numerical failure. Emit a numerical-warning status after a clamp. This clamp bound is not the test-comparison tolerance. |
| Infinity | Reject non-finite input; reserve output +Inf for the mathematically meaningful zero-within-SS, positive-between-SS limit. |
| Merge tree | Document last-bit non-determinism across distributed partition trees. Validate approximate equivalence, not bit identity. |
| Serialization | If later added, version state and preserve count/mean/M2 exactly as encoded float64 fields; no JSON-number round-trip assumption. |

## Recommended tolerance contract

For validation comparisons of nonnegative sum-of-squares quantities #$#a,b#$#, use:

#$#|a-b|\le a_{abs}+a_{rel}\max(|a|,|b|)#$#

where test-specific #$#a_{abs},a_{rel}#$# are selected from observed oracle precision. Do not hard-code a tolerance without adversarial large-offset tests.

## Adversarial data families

- large location with tiny spread: #$#10^{12}+\{-2,-1,0,1,2\}#$#;
- highly unbalanced groups;
- singleton groups mixed with large groups;
- identical observations within and across groups;
- very small and very large magnitudes, avoiding overflow by construction;
- random partition/merge-tree permutations;
- group labels in non-insertion order.

## Consequence for API design

A result can be statistically undefined or numerically suspect. These are different states. The result contract must represent validity/status rather than returning arbitrary finite zeros.

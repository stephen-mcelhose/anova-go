# Assumptions, Diagnostics, and Alternatives

**Status:** interpretation guardrails  
**Links:** [[One-way ANOVA]] · [[Scope and semantic contract]] · [[Test oracles]]

## Classical test assumptions

| Assumption | Meaning | Engineering implication |
|---|---|---|
| Independent observational units | Dependence is determined by data collection, not by a residual plot | The API cannot verify this. Documentation must require users to supply the correct unit/grouping. |
| Normal residuals within groups | Most important for small samples and extreme tail calibration | Do not claim a normality check proves validity. |
| Equal population variances | Standard one-way F uses a pooled residual variance | Unequal variances with imbalanced group sizes can distort type-I error. |
| Correct response/group definition | Groups and response must encode the intended comparison | Validate finite scalar inputs; leave substantive coding to the caller. |

## Diagnostics are not automatic approval

Future diagnostic helpers may provide residual plots, group variances, and warnings, but no automatic rule converts observational/design problems into a valid causal or inferential analysis. In particular, testing variances and conditionally selecting a test alters inference if not planned.

## Alternatives to specify separately

| Situation | Candidate method | Why not silently substitute |
|---|---|---|
| Unequal variances | Welch ANOVA | Different statistic and approximate denominator df. |
| Strong outliers/non-normal ordinal response | Kruskal–Wallis or robust/trimmed-mean methods | Tests a different estimand/hypothesis; ties and post-hoc policies matter. |
| Repeated observations per subject | Repeated-measures model or mixed model | Independence fails. |
| Covariates | ANCOVA/linear model | Requires model matrix, rank/contrast, and diagnostic design. |
| Multiple pairwise questions | Prespecified contrasts / post-hoc procedure with multiplicity control | Omnibus F does not identify pairs. |
| Sequential monitoring | Sequential-valid method | Repeated ordinary p-values are not a sequential test. |

## User-facing wording requirement

Every public result description must include: “The p-value is calibrated under the one-way ANOVA model; assess independence, variance homogeneity, residual distribution, and study design before interpretation.”

## Documentation test cases

- A repeated-measures example must direct users away from v1 one-way ANOVA.
- An unequal-variance, unequal-size example must identify Welch ANOVA as deferred rather than label the ordinary F result robust.
- A streaming dashboard example must warn against alerting on first p < alpha without sequential controls.

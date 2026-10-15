# Reference Implementation Register

**Status:** research baseline; re-verify exact versions and licenses when vendoring/copying behavior  
**Links:** [[Go/Gonum research]] · [[Test oracles]] · [[One-way ANOVA]]

This register separates **behavioral oracles** from **code sources**. Implement the documented mathematics; do not copy source code without a license review.

| ID | Reference | Coverage / value | Use | URL |
|---|---|---|---|---|
| REF-001 | R `stats::aov` / `stats::anova` | Mature linear-model ANOVA semantics; important for model/SS conventions | Batch result oracle for simple balanced/unbalanced one-way fixtures; do not inherit broader formula behavior in v1 | https://stat.ethz.ch/R-manual/R-devel/library/stats/html/aov.html |
| REF-002 | SciPy `scipy.stats.f_oneway` | Widely used one-way ANOVA API; documents degeneracy warnings | Numeric oracle and edge-case comparison; record SciPy version in fixture provenance | https://docs.scipy.org/doc/scipy/reference/generated/scipy.stats.f_oneway.html |
| REF-003 | SciPy `scipy.stats.f` | F distribution CDF/SF reference | Tail-probability oracle | https://docs.scipy.org/doc/scipy/reference/generated/scipy.stats.f.html |
| REF-004 | statsmodels `anova_lm` | Regression-model ANOVA table and typ/robustness context | Future factorial/linear-model design research, not v1 direct dependency | https://www.statsmodels.org/stable/generated/statsmodels.stats.anova.anova_lm.html |
| REF-005 | Apache Commons Math `OneWayAnova` | Independent Java implementation/API and input validation ideas | Behavioral cross-check only; license review before any code reuse | https://commons.apache.org/proper/commons-math/javadocs/api-3.6.1/org/apache/commons/math3/stat/inference/OneWayAnova.html |
| REF-006 | GNU Scientific Library `gsl_stats` | C numerical/statistical routines | Supplementary numerical-reference inspection | https://www.gnu.org/software/gsl/doc/html/statistics.html |
| REF-007 | Chan, Golub, and LeVeque (1983) | Pairwise/parallel computation of sample variance | Normative merge derivation for streaming accumulator | https://doi.org/10.1080/00031305.1983.10483115 |
| REF-008 | Pébay (2008), Sandia report | Formulas for parallel/mergeable moments | Supporting derivation and extension path | https://www.osti.gov/biblio/1028931 |
| REF-009 | Welford (1962) | Stable one-pass variance update | Normative `Add` recurrence | https://doi.org/10.1080/00401706.1962.10490022 |

## Reference selection policy

- **Arithmetic source of truth:** equations in [[One-way ANOVA]] plus update/merge equations in [[Streaming one-way ANOVA]].
- **Batch p-value oracle:** a pinned SciPy release and independently generated R fixture values.
- **Streaming oracle:** batch computation over the same raw observations and random partition/merge equivalence.
- **Distribution primitive:** Gonum research decides the supported implementation path; see [[Go/Gonum research]].

## Ingestion checklist for each reference

1. Capture release/version, documentation URL, license, retrieval date, and a local fixture script or source hash.
2. State exact behavior for NaN, infinity, empty/singleton groups, constant groups, and unequal sizes.
3. Never equate APIs merely because each is named “ANOVA”; document model, degrees-of-freedom, and tail convention.
4. Store expected values with source/version provenance and tolerances.

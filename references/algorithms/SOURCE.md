# Algorithm Sources

## Welford online variance

- **Citation:** B. P. Welford, “Note on a Method for Calculating Corrected Sums of Squares and Products,” *Technometrics*, 1962.
- **DOI:** https://doi.org/10.1080/00401706.1962.10490022
- **Role:** derivation of per-group append-only centered moments.
- **Ingested behavior:** `mean` and `M2` update used in `anova.go`.
- **Raw status:** no source-code snapshot; paper/license review is required before retaining a downloaded copy.

## Parallel/merge variance

- **Citation:** T. F. Chan, G. H. Golub, and R. J. LeVeque, “Algorithms for Computing the Sample Variance: Analysis and Recommendations,” *The American Statistician*, 1983.
- **DOI:** https://doi.org/10.1080/00031305.1983.10483115
- **Role:** merge formula for disjoint `(count, mean, M2)` summaries.
- **Ingested behavior:** merge recurrence used in `anova.go`.
- **Raw status:** no source-code snapshot; paper/license review is required before retaining a downloaded copy.

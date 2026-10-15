# Go and Gonum Research

**Status:** integration research and acceptance gates  
**Links:** [[Reference implementation register]] · [[Architecture decisions]] · [[Implementation roadmap]]

## Intended dependency role

The core one-way accumulator requires only `float64` arithmetic and maps/slices. A math dependency is required for the F-distribution upper-tail probability. Gonum is the preferred ecosystem candidate because it provides numerical primitives and distributions, but **the exact package/API/version must be verified during implementation and pinned in `go.mod`**.

## Research targets

| Need | Preferred investigation | Acceptance gate |
|---|---|---|
| F CDF/SF | Gonum distribution APIs and their documentation/source | Must calculate upper tails accurately against SciPy/R fixtures for moderate and extreme p-values. |
| Regularized incomplete beta | Gonum special-function API, if distribution wrapper lacks survival function | Validate symmetry/complement behavior and parameter-domain errors. |
| Linear algebra | `gonum/mat` only for future regression/factorial ANOVA | v1 direct one-way implementation must not depend on matrix fitting. |
| Random test data | standard library or Gonum random utilities | Reproducible seeds; no production inference dependence. |

## Evaluation procedure

1. Create a compile-time spike against a pinned current Gonum version.
2. Locate an F distribution / regularized incomplete-beta path and verify its semantics (CDF versus survival; parameters; NaN/Inf behavior).
3. Compare p-values to fixture values generated from pinned R and SciPy sources over a grid of #$#df_1,df_2,F#$#, including far-tail cases.
4. Benchmark only after correctness; the expected core operation cost is #$#O(1)#$# update and #$#O(k)#$# snapshot.
5. Record exact import path, version, behavior, and license in an ADR before production dependency adoption.

## Fallback hierarchy

1. Gonum provides a validated survival-function implementation: use it.
2. Gonum provides a validated CDF only: use a documented numerically stable complement only if tail acceptance tests pass.
3. Gonum exposes an incomplete-beta primitive: implement a thin, fully tested F-survival adapter from the identity below, calculating its argument with overflow-safe scaling (never naively form #$#d_2+d_1f#$# if it can overflow).
4. No acceptable primitive: defer p-values rather than ship an unvalidated special-function implementation.

For #$#F\ge0#$# with #$#d_1,d_2>0#$#, a common identity is:

#$#P(F_{d_1,d_2}\ge f)=I_{\frac{d_2}{d_2+d_1f}}\left(\frac{d_2}{2},\frac{d_1}{2}\right)#$#

The API must test this identity against oracles rather than rely on transcribing it alone.

## Go API design constraints

- No Gonum matrix object is needed for `oneway` v1.
- Keep the result table in native structs so callers do not inherit a matrix dependency.
- Separate statistical core from distribution adapter behind an internal interface if this enables deterministic arithmetic tests independent of special functions.
- Pin Go and Gonum versions in CI once a repository/module exists.

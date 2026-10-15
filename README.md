# ANOVA for Go — Reference Wiki and Delivery Plan

This docs-first repository is the reference authority and initial implementation for a Go library: numerically stable batch one-way ANOVA and exact append-only streaming one-way ANOVA.

## Start here

- [Scope and semantic contract](wiki/00-scope-and-contract.md)
- [Concept graph](wiki/01-concept-graph.md)
- [One-way ANOVA theory](wiki/02-one-way-anova.md)
- [Streaming sufficient statistics](wiki/03-streaming-one-way-anova.md)
- [Numerical stability and merging](wiki/04-numerical-stability.md)
- [Assumptions, diagnostics, and alternatives](wiki/05-assumptions-and-alternatives.md)
- [Reference implementation register](wiki/06-reference-implementation-register.md)
- [Go/Gonum integration research](wiki/07-gonum-research.md)
- [Test oracle and verification strategy](wiki/08-test-oracles.md)
- [Architecture decisions](wiki/09-adrs.md)
- [Implementation roadmap](wiki/10-implementation-roadmap.md)
- [Open decisions and handoff](wiki/11-open-decisions.md)

## Status

An initial arithmetic implementation is present in `anova.go`, with batch ingestion, fixed-group append-only streaming, non-mutating snapshots, atomic merge semantics, explicit degenerate statuses, and core verification tests. It deliberately leaves `Result.P` as `NaN`: p-values are release-blocked until a pinned Gonum F-tail adapter passes the specified R/SciPy differential fixtures. It does not claim support for factorial, repeated-measures, Welch, or rolling-window ANOVA.

## Conventions

- `[[Page]]` is an internal wiki link.
- `REQ-*`, `ADR-*`, `TST-*`, `RSK-*`, and `OQ-*` are traceability IDs.
- Formulas use `#$# … #$#`.

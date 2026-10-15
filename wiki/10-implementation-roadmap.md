# Implementation Roadmap

**Status:** delivery plan  
**Links:** [[Scope and semantic contract]] · [[Test oracles]] · [[Architecture decisions]]

## Phase 0 — Freeze semantics and source fixtures

**Exit criteria:** ADR-005 resolved; Go module/version decided; exact Gonum distribution import/version validated; R/SciPy fixture generator and fixture metadata checked in.

- EP-001: establish module/repository conventions and license policy.
- EP-002: run Gonum F-tail spike and record findings.
- EP-003: create canonical differential fixture corpus.
- EP-004: approve public result-status behavior.

## Phase 1 — Batch one-way ANOVA

**Goal:** a small, stable batch API built from the same accumulator primitives planned for streaming.

- ST-101: implement unexported centered-moment state with input validation.
- ST-102: implement group registration and batch ingestion.
- ST-103: implement table computation, degeneracy statuses, and F-tail adapter.
- ST-104: implement unit/property/differential tests TST-001–040.
- ST-105: write package examples with assumption warnings.

**Definition of done:** all v1 REQ IDs and relevant TST IDs pass; behavior matches pinned fixture provenance; no unreviewed formula or distribution dependency remains.

## Phase 2 — Append-only streaming

**Goal:** exact snapshot and merge semantics with no retained raw data.

- ST-201: expose accumulator construction with fixed group universe.
- ST-202: implement `Add`, pure `Snapshot`, and atomic reject behavior.
- ST-203: implement compatibility-checked `Merge`.
- ST-204: add random partition/merge property testing and fuzzing.
- ST-205: choose/document concurrency policy; add synchronized wrapper only if required.

**Definition of done:** batch/stream/partition results are equivalent within tolerance and all stream restrictions are user-visible in docs.

## Phase 3 — Hardening and release

- ST-301: fuzz add/merge sequences and edge labels.
- ST-302: benchmark #$#O(1)#$# add and #$#O(k)#$# snapshot behavior.
- ST-303: review API naming/error semantics and GoDoc examples.
- ST-304: reproducible release/CI with pinned dependency and fixture provenance.

## Future decision gates

| Candidate | Gate before implementation |
|---|---|
| Welch ANOVA | Adopt estimator, df approximation, p-value oracle, and weighted/group-size semantics. |
| Dynamic groups | Define schema evolution and whether empty/retired groups affect df. |
| Sliding windows | Define event time, late data, expiration, memory bound, exactness/rebuild policy, and merge constraints. |
| Factorial ANOVA | Adopt design-matrix and contrast system; choose Type I/II/III SS and rank-deficiency behavior. |
| Post-hoc tests | Choose comparisons, correction, CI/test families, and studentized-range dependency. |
| Sequential inference | Specify alpha control and snapshot decision semantics with statistical review. |

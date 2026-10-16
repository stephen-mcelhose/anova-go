# Architecture Decision Records

**Links:** [[Scope and semantic contract]] · [[Streaming one-way ANOVA]] · [[Go/Gonum research]]

## ADR-001 — Scope v1 to fixed-group, unweighted one-way ANOVA

**Status:** Accepted  
**Decision:** Implement only the intercept-plus-group one-way model, with finite scalar responses and fixed registered groups.  
**Rationale:** It has clear sufficient statistics and a bounded API. Welch, factorial, repeated-measures, covariate, and post-hoc methods have different model or inference contracts.  
**Consequences:** Documentation must actively route unsupported designs to future extensions/other tools.

## ADR-002 — Use centered online moments, not raw moments

**Status:** Accepted  
**Decision:** Store #$#(n,mean,M_2)#$# and use Welford updates plus Chan-style merges.  
**Rationale:** Stable one-pass state and mergeability; raw #$#sum/sumSq#$# suffers cancellation.  
**Consequences:** Results are stable but not universally bit-identical across update/merge order.

## ADR-003 — Append-only streaming is exact; windowing is a separate feature

**Status:** Accepted  
**Decision:** `Add`, `Snapshot`, and `Merge` define streaming. No delete/window API in v1.  
**Rationale:** Window semantics require event lifecycle, removal/rebuild policy, and additional test surface.  
**Consequences:** Marketing/docs must never imply rolling-window support.

## ADR-004 — Snapshot p-values are not sequential decisions

**Status:** Accepted  
**Decision:** Expose ordinary F-test p-values at snapshots with a repeated-monitoring warning.  
**Rationale:** The accumulator computes statistics, not alpha spending or always-valid inference.  
**Consequences:** Alerting examples require an explicit external sequential-method policy.

## ADR-005 — Separate core aggregation from F-tail adapter

**Status:** Accepted for architecture; dependency binding deferred to Phase 0  
**Decision:** Keep distribution calculation behind an internal adapter. Bind a Gonum package/API/version only after a version-pinned spike passes differential tail tests.  
**Rationale:** The accumulation core can be validated independently; special-function accuracy is a separate dependency risk.  
**Consequences:** Batch/stream arithmetic implementation may proceed, but a released p-value feature is blocked until the exact distribution adapter is accepted and this ADR records its version/API/tail-test evidence.

## ADR-006 — Invalid and degenerate results are explicit statuses

**Status:** Accepted  
**Decision:** Result types distinguish invalid prerequisites, indeterminate #$#0/0#$#, meaningful #$#+\\infty#$# F, and numerical failures.  
**Rationale:** Returning fabricated 0/NaN p-values silently corrupts downstream interpretation.  
**Consequences:** API consumers must check status/validity before using p-values.

## ADR-008 — v1 concurrency policy: caller-owned synchronization, no library wrapper

**Status:** Accepted  
**Decision:** `Accumulator` is not concurrency-safe. v1 documents caller-owned synchronization as the required policy and ships no synchronized wrapper type.  
**Rationale:** The intended streaming pipeline routes each observation to a single owning goroutine (`source | filter | route-to-owner | Add | sink`). Routing is the synchronization: no two goroutines ever write to the same `Accumulator`, so a library mutex adds overhead without solving a real problem. For callers with a different topology who need shared access, wrapping with `sync.Mutex` is ~5 lines and is not a library concern. Adding a `SyncAccumulator` type would introduce non-trivial lock-ordering logic in `Merge` (ABBA deadlock risk) and a permanent maintenance surface with no benefit for the primary use case.  
**Consequences:** The GoDoc and package examples must state the caller-sync contract explicitly. An `ExampleAccumulator_callerSync` example demonstrates the `sync.Mutex` pattern for callers who need it. A synchronized wrapper may be revisited in a future ADR if evidence of widespread need emerges.

## ADR-007 — R is the trusted oracle for pinned ANOVA table fixtures; revisit for fuzz

**Status:** Accepted  
**Decision:** Use R 4.5.0 `aov()` as the external oracle for pinned full-table differential tests (TST-021–025). Accept its output as ground truth for non-adversarial fixtures. Do not use it as a dynamic fuzz oracle.  
**Rationale:** During Phase 1 gap-filling we compared R's `aov()` output against our Welford/Chan implementation across FX-001–004 and three non-trivial decimal fixtures. Agreement was within 1e-14 to 3e-13 on all tested fields — well inside the 1e-9 pinned tolerance. R uses a QR-based algorithm which is independently trustworthy for normal-scale data. One exception was found: FX-008 (large-offset adversarial, values ~1e12 with small spread). R's `aov()` produced SS_B ≈ 250.004 (~1.8e-5 relative error) due to cancellation in its intermediate sums, whereas the Go Welford/Chan implementation yields SS_B = 250.000 (provably correct by translation invariance). FX-008 is therefore excluded from the R oracle comparison and covered instead by `TestMath_TranslationInvariance`. For fuzz and exhaustive math tests, a static CSV is not a viable oracle. The right strategy for those is mathematical invariants (decomposition, non-negativity, translation invariance, partition equivalence) and, if absolute-value cross-checks are needed, a separate controlled naive reference implementation for well-conditioned inputs.  
**Consequences:** The pinned oracle CSV (`anova_table_fixtures.csv`) is a regression guard, not a fuzz harness. If fuzz testing or exhaustive math tests surface failures that require a dynamic trusted reference, revisit this ADR to choose between: R via subprocess, a naive two-pass Go reference restricted to well-conditioned inputs, or an alternative trusted library. Do not extend the R oracle CSV to adversarial inputs without first verifying R's answer is correct for that fixture.

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
**Decision:** Result types distinguish invalid prerequisites, indeterminate #$#0/0#$#, meaningful #$#+\infty#$# F, and numerical failures.  
**Rationale:** Returning fabricated 0/NaN p-values silently corrupts downstream interpretation.  
**Consequences:** API consumers must check status/validity before using p-values.

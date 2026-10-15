# Open Decisions and Handoff

**Links:** [[Implementation roadmap]] · [[Go/Gonum research]] · [[Architecture decisions]]

## Decisions that need owner input before code is committed

| ID | Decision | Default recommended in this plan | Why it matters |
|---|---|---|---|
| OQ-001 | Module path and repository/license | Create a new module; select license with legal/organizational policy | Public API and reuse terms depend on it. |
| OQ-002 | Group key type | Start with an ordered `string` key and document canonical ordering | Influences memory, serialization, and determinism. |
| OQ-003 | Fixed groups vs dynamic registration | Fixed groups in v1 | Keeps degrees-of-freedom and merge schema explicit. |
| OQ-004 | Concurrency | Caller-owned synchronization in v1 | Avoids hidden locking/latency; a wrapper can be added with evidence of need. |
| OQ-005 | Dependency approval | Pin Gonum after acceptance spike | F-tail accuracy and supply-chain policy need evidence. |
| OQ-006 | API ergonomics | Offer accumulator plus batch convenience function | Maintains one arithmetic core while serving both use cases. |
| OQ-007 | Result contract | Struct with validity/status plus table fields | Required for explicit degenerate-data behavior. |
| OQ-008 | Streaming product meaning | Append-only analytics snapshot, not rolling or sequential inference | Prevents incorrect dashboard/alert interpretation. |

## Implementation handoff package

A coding agent or developer should load, in order:

1. `README.md`;
2. [[Scope and semantic contract]];
3. [[One-way ANOVA]] and [[Numerical stability]];
4. [[Streaming one-way ANOVA]];
5. [[Test oracles]];
6. [[Architecture decisions]] and this page.

Then complete Phase 0 of [[Implementation roadmap]] before implementing a public API. The blocking engineering task is a small, pinned Gonum F-tail spike and source-fixture ingestion. No broader product clarification is required to write the arithmetic core, but the OQ decisions are required before stabilizing exported names and release behavior.

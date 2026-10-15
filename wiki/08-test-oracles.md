# Test Oracles and Verification Strategy

**Status:** binding test plan  
**Links:** [[Reference implementation register]] · [[Numerical stability]] · [[Streaming one-way ANOVA]] · [[Implementation roadmap]]

## Verification layers

| Layer | Objective | Test IDs |
|---|---|---|
| Unit | Validate state transitions and formula components | TST-001–010 |
| Properties | Validate invariants across generated data/partitions | TST-011–020 |
| Differential | Match independent trusted batch implementations | TST-021–030 |
| Numerical stress | Expose cancellation, extreme scales, merge-order effects | TST-031–040 |
| API/contract | Reject invalid input and preserve snapshot/merge semantics | TST-041–050 |
| Benchmark/race | Establish operational behavior, not statistical correctness | TST-051–055 |

## Canonical fixtures

| ID | Raw groups | Expected arithmetic | Purpose |
|---|---|---|---|
| FX-001 | A=[1,2,3], B=[4,5,6], C=[7,8,9] | #$#SS_B=54,SS_W=6,df_B=2,df_W=6,MS_B=27,MS_W=1,F=27#$# | Hand-auditable balanced fixture; p is generated from pinned external oracle. |
| FX-002 | Three groups with unequal #$#n_i#$# | Pinned R/SciPy expected table | Unbalanced arithmetic/oracle comparison. |
| FX-003 | Equal means with positive within-group variation | Pinned expected table | Null-like behavior. |
| FX-004 | Clearly separated means with positive residual variation | Pinned expected table | Small upper-tail p-value. |
| FX-005 | Every value identical | `IndeterminateZeroOverZero` | Degenerate #$#0/0#$# F. |
| FX-006 | Each group constant but group means differ | #$#F=+\infty,p=0#$# | Degenerate infinite-F limit. |
| FX-007 | Two non-empty singleton groups | `InsufficientResidualDF` | #$#df_W=0#$#. |
| FX-008 | #$#10^{12}+\{-2,-1,0,1,2\}#$# pattern per group | Stable reference result | Detects raw-sums cancellation. |

Fixture metadata must include raw data, expected values, oracle name/version, generation script, and tolerance.

## Enumerated contract tests

- **TST-001:** first observation initializes #$#n=1,mean=x,M_2=0#$#.
- **TST-002:** Welford update matches hand-calculated two/three-observation summaries.
- **TST-003:** Chan merge matches direct summary for two disjoint partitions.
- **TST-004:** table assembly from group summaries matches FX-001.
- **TST-005:** empty and singleton group rank/df behavior follows REQ-002/003.
- **TST-006:** constant-all and constant-different group statuses follow FX-005/006.
- **TST-007:** NaN/+Inf/-Inf add rejection is atomic.
- **TST-008:** unknown/unregistered group rejection is atomic.
- **TST-009:** result/configuration defensive-copy behavior and canonical ordering.
- **TST-010:** F-tail adapter validates boundary/domain behavior including #$#F=+\infty#$#.

- **TST-021–025:** pinned R/SciPy table comparisons for FX-001–004 plus randomized valid fixtures.
- **TST-026–030:** F-survival grid spanning small/large #$#df_1,df_2#$#, moderate tails, far tails, and infinite-F boundary.
- **TST-031:** large-offset cancellation fixture.
- **TST-032:** finite extreme magnitudes that overflow a delta/cross term return numerical failure atomically.
- **TST-033:** `MaxExactCount=2^53` boundary: a state at the bound rejects a further add/merge atomically; a merge totaling exactly the bound succeeds.
- **TST-034:** randomized merge-tree order remains within tolerance.
- **TST-035:** scale-aware negative-SS clamp boundary and beyond-bound failure.
- **TST-036–040:** highly unbalanced, singleton-mixed, tiny-scale, huge-scale, and deterministic-order stress cases.
- **TST-041:** incompatible-configuration merge rejection is atomic.
- **TST-042:** self-merge is rejected atomically.
- **TST-043:** documented overlap precondition is present in API examples; no false detection claim.
- **TST-044:** numeric-failure merge rejection is atomic.
- **TST-045–050:** snapshot purity, constructor-input mutation, returned-result mutation, caller synchronization contract, error-status, and no-panic checks.
- **TST-051–055:** `Add`/snapshot benchmarks, merge benchmark, fuzz add sequences, fuzz merge sequences, and race test for any synchronized wrapper.

## Required properties

- **TST-011 Partition equivalence:** feeding all data to one stream versus feeding random disjoint partitions and merging gives equivalent group summaries and ANOVA results within tolerance.
- **TST-012 Batch/stream equivalence:** batch construction and sequential `Add` give equivalent results.
- **TST-013 Translation invariance:** adding a finite constant to every response leaves SS and F unchanged within tolerance.
- **TST-014 Within-group permutation invariance:** response order within a group changes results only within floating tolerance.
- **TST-015 Label bijection invariance:** renaming groups does not change table values.
- **TST-016 Decomposition:** #$#SS_T\approx SS_B+SS_W#$# for valid fixtures.
- **TST-017 Non-negativity:** valid SS values are nonnegative within documented tolerance.
- **TST-018 Snapshot purity:** two snapshots without additions are identical; snapshot does not mutate accumulator state.
- **TST-019 Invalid input atomicity:** rejected `Add` does not change state.
- **TST-020 Merge compatibility:** incompatible configurations are rejected with no partial mutation.

## Differential testing

Generate fixtures externally with pinned versions of R and SciPy and compare:

- #$#SS_B,SS_W,df_B,df_W,F,p#$#;
- policies for constant input and empty groups, noting that warnings/exceptions may differ;
- upper-tail probabilities on an F/df grid.

R and SciPy agreement is evidence, not a substitute for independently checking definitions. The Go implementation may legitimately differ in error shape while matching the documented semantic contract.

## CI gates

- `go test` unit/property/differential suites;
- race tests if a synchronized wrapper is offered;
- fuzz target for finite values, labels, add/merge sequences, and no-panic/no-negative-count invariants;
- benchmark regression threshold only after a stable baseline exists;
- no network-dependent oracle calls in normal CI: fixtures are checked in with provenance.

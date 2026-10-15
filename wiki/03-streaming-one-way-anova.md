# Streaming One-Way ANOVA

**Status:** binding append-only streaming design  
**Links:** [[One-way ANOVA]] · [[Numerical stability]] · [[Architecture decisions]] · [[Test oracles]]

## Definition of streaming in v1

A stream is an append-only sequence of finite observations #$#(g,x)#$# for a predeclared group universe. `Snapshot()` represents every observation accepted so far. It is exact with respect to the same floating-point update/merge tolerance, not a sequential hypothesis test.

A snapshot p-value is an ordinary fixed-horizon ANOVA p-value applied repeatedly. It must not be treated as valid evidence under optional stopping without a separately designed sequential-inference method.

## Per-group state

For every group, store:

- #$#n#$#: count;
- #$#\bar{x}#$#: running mean;
- #$#M_2=\sum(x-\bar{x})^2#$#: centered sum of squares.

On observation #$#x#$#, update using Welford’s recurrence:

#$#n'=n+1,\quad \delta=x-\bar{x},\quad \bar{x}'=\bar{x}+\delta/n',\quad M_2'=M_2+\delta(x-\bar{x}')#$#

At snapshot time:

#$#SS_W=\sum_i M_{2i}#$#

Build #$#SS_B#$# by merging group summaries. If summaries #$#A=(n_A,\mu_A,M_{2A})#$# and #$#B=(n_B,\mu_B,M_{2B})#$# are disjoint, then:

#$#n=n_A+n_B,\quad \delta=\mu_B-\mu_A,\quad \mu=\mu_A+\delta n_B/n#$#

#$#M_2=M_{2A}+M_{2B}+\delta^2n_An_B/n#$#

Thus merge all group **means with counts** starting with zero within-group M2; the resulting M2 is #$#SS_B#$#. This also establishes a direct partition-merge equivalence test.

## Merge semantics

`Merge(other)` means union of two **disjoint** observation partitions, not replay or deduplication. The base summary does not retain observation identities, so it cannot prove disjointness. Disjointness is therefore a documented caller precondition; merging overlapping summaries, including a summary with itself, double-counts observations. `Merge(self)` must be rejected, but other overlap cannot be detected without a future provenance layer.

For each group, merge the two per-group summaries using the formula above. The operation is associative in exact arithmetic, but floating point makes the last bits merge-tree dependent. Results are compared with documented tolerances rather than bit identity. A merge is all-or-nothing: validate compatibility and every count/arithmetic operation before committing, or construct a temporary merged state and swap only on success.

**REQ-020:** reject merge when configuration versions or group universes are incompatible.  
**REQ-021:** do not make `Add` and `Snapshot` concurrently safe implicitly. v1 exposes one of two explicit choices: caller synchronization, or a separately documented synchronized wrapper.  
**REQ-022:** snapshot must not alter state.  
**REQ-023:** reject self-merge and document disjointness as the caller’s obligation.  
**REQ-024:** any rejected merge, including numeric/count overflow, leaves receiver state unchanged.

## What this state cannot do

### Sliding windows

Removal is algebraically possible only if the removed contribution is known, but stable inverse updates are delicate and require event identity/order, expiration semantics, and a policy for reconstruction when numerical error accumulates. Exact event-time windows also need late-event and watermark behavior. This is a separate feature with retained observations or time-bucket summaries plus periodic rebuilds.

### Dynamic group policy

Adding a new group is mechanically feasible if group identity is defined, but it changes model degrees of freedom over time. v1 uses registered groups. A later `AllowNewGroups` policy must declare whether empty/retired groups count and how merge schemas evolve.

### Sequential inference

Repeated snapshots with stop-on-significance inflate false positives. The accumulator is compatible with a later alpha-spending, always-valid, or confidence-sequence layer, but it does not provide one.

## Complexity

`Add`: #$#O(1)#$# time and #$#O(k)#$# state across groups.  
`Snapshot`: #$#O(k)#$# time, #$#O(1)#$# extra state if traversing summaries.  
`Merge`: #$#O(k)#$# time.  
No raw observations are retained in the base accumulator.

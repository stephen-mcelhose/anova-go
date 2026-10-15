# One-Way ANOVA

**Status:** binding statistical specification for v1  
**Links:** [[Scope and semantic contract]] · [[Streaming one-way ANOVA]] · [[Numerical stability]] · [[Assumptions and alternatives]] · [[Test oracles]]

## Model and estimand

For group #$#i=1,\dots,k#$# and observation #$#j=1,\dots,n_i#$#:

#$#Y_{ij}=\mu+\tau_i+\epsilon_{ij},\quad \sum_i n_i \tau_i=0#$#

The null hypothesis is #$#H_0:\mu_1=\cdots=\mu_k#$#. The classical F-test is an omnibus test: rejection says at least one population mean differs under its assumptions; it does not identify which means differ.

Let #$#N=\sum_i n_i#$#, group means #$#\bar{Y}_i#$#, and grand mean #$#\bar{Y}#$#. With #$#k_+#$# non-empty groups, calculate:

#$#SS_W=\sum_{i=1}^{k_+}\sum_{j=1}^{n_i}(Y_{ij}-\bar{Y}_i)^2#$#

#$#SS_B=\sum_{i=1}^{k_+}n_i(\bar{Y}_i-\bar{Y})^2#$#

#$#SS_T=\sum_{i=1}^{k_+}\sum_{j=1}^{n_i}(Y_{ij}-\bar{Y})^2=SS_B+SS_W#$#

#$#df_B=k_+-1,\quad df_W=N-k_+,\quad df_T=N-1#$#

#$#MS_B=SS_B/df_B,\quad MS_W=SS_W/df_W,\quad F=MS_B/MS_W#$#

The upper-tail p-value is:

#$#p=\Pr(F_{df_B,df_W}\ge F_{obs})#$#

## Required arithmetic formulation

Compute #$#SS_W#$# as the sum of each group’s centered #$#M_2#$#. Compute #$#SS_B#$# with a stable weighted-mean merge or a scale-aware centered sum. Do **not** use #$#\sum y^2-(\sum y)^2/n#$# as the primary algorithm because catastrophic cancellation can produce inaccurate or negative variance for large-offset, low-variance data.

The total SS identity is a test invariant, not the preferred production calculation:

#$#SS_T \approx SS_B+SS_W#$#

## Preconditions

- Each response is finite.
- Each observation belongs to exactly one nominal group.
- At least two groups have observations.
- #$#N>k_+#$# to estimate #$#MS_W#$#.

## Result table contract

| Source | SS | df | MS | F | p |
|---|---:|---:|---:|---:|---:|
| Between groups | #$#SS_B#$# | #$#df_B#$# | #$#MS_B#$# | #$#F#$# | upper tail |
| Within groups / residual | #$#SS_W#$# | #$#df_W#$# | #$#MS_W#$# | — | — |
| Total | #$#SS_T#$# | #$#df_T#$# | — | — | — |

## Deferred effect sizes

#$#\eta^2=SS_B/SS_T#$# and #$#\omega^2=(SS_B-df_BMS_W)/(SS_T+MS_W)#$# are useful descriptive measures but require explicit zero-total-variance and bias-correction policies. They are v1.1 candidates, not silently bundled into v1.

## Interpretation guardrails

- The F-test assumes independent errors, approximately normal residuals for small samples, and equal within-group variances for its nominal finite-sample calibration.
- Balanced designs are not required, but unequal group sizes worsen sensitivity to heteroscedasticity.
- Statistical significance is not effect magnitude or practical significance.
- The package must label output as an **omnibus one-way ANOVA test** and must not state that group membership caused the difference.

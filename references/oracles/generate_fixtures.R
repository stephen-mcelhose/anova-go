# generate_fixtures.R
#
# Produces F-distribution upper-tail oracle fixtures for anova-go.
# Output: f_tail_fixtures.csv in the same directory.
#
# Usage:
#   cd references/oracles
#   Rscript generate_fixtures.R
#
# R version used to produce the committed CSV:
#   R version 4.5.0 (2025-04-11) -- "How About a Twenty-Six"
#   Platform: aarch64-apple-darwin20
#
# pf() computes the F-distribution CDF. The upper-tail probability
# P(F_{d1,d2} >= f) = pf(f, d1, d2, lower.tail = FALSE).
#
# Tolerance accepted by TestOracle_FTail (TST-034): 1e-9 absolute.

f_pvalue <- function(f, df1, df2) pf(f, df1, df2, lower.tail = FALSE)

# ── ANOVA fixture helpers ──────────────────────────────────────────────────────

anova_stats <- function(...) {
  groups <- list(...)
  labels <- names(groups)
  y <- unlist(groups)
  g <- factor(rep(labels, sapply(groups, length)), levels = labels)
  fit <- summary(aov(y ~ g))[[1]]
  list(
    df1   = fit$Df[1],
    df2   = fit$Df[2],
    f     = fit[["F value"]][1],
    p     = fit[["Pr(>F)"]][1]
  )
}

# ── Fixture accumulator ────────────────────────────────────────────────────────

rows <- list()

add <- function(description, df1, df2, f) {
  p <- f_pvalue(f, df1, df2)
  rows[[length(rows) + 1]] <<- list(
    description = description,
    df_between  = as.integer(df1),
    df_within   = as.integer(df2),
    f_stat      = f,
    p_value     = p
  )
}

add_anova <- function(description, ...) {
  s <- anova_stats(...)
  add(description, s$df1, s$df2, s$f)
}

# ── FX-001: balanced 3×3 ──────────────────────────────────────────────────────
# A=[1,2,3], B=[4,5,6], C=[7,8,9]
# Hand-verified: SS_B=54, SS_W=6, df_B=2, df_W=6, F=27
add("FX-001 balanced 3x3 A=[1,2,3] B=[4,5,6] C=[7,8,9]", 2, 6, 27.0)

# ── FX-002: unbalanced 3-group (n=4,2,3) ──────────────────────────────────────
# A=[1,2,3,4], B=[5,6], C=[7,8,9]
# Hand-verified: SS_B=52.5, SS_W=7.5, df_B=2, df_W=6, F=21
add("FX-002 unbalanced 3-group n=4,2,3 A=[1..4] B=[5,6] C=[7,8,9]", 2, 6, 21.0)

# ── FX-003: null-like (small F) ───────────────────────────────────────────────
# A=[1,3,5], B=[2,4,6]
# Group means: A=3, B=4; grand mean=3.5
# SS_B=1.5, SS_W=16, df_B=1, df_W=4, F=0.375
add("FX-003 null-like A=[1,3,5] B=[2,4,6] F=0.375", 1, 4, 0.375)

# ── FX-004: clearly separated means (small p-value) ──────────────────────────
# A=[1,2,3], B=[10,11,12]
# SS_B=121.5, SS_W=4, df_B=1, df_W=4, F=121.5
add("FX-004 separated A=[1,2,3] B=[10,11,12] F=121.5", 1, 4, 121.5)

# ── FX-008: large-offset adversarial ─────────────────────────────────────────
# Groups shifted by 1e12; F must equal the unshifted value exactly.
# A=1e12+{-2,-1,0,1,2}, B=1e12+{3,4,5,6,7}, C=1e12+{8,9,10,11,12}
# Unshifted: group means 0, 5, 10; grand mean 5
# SS_B=5*(0-5)^2+5*(5-5)^2+5*(10-5)^2=250, SS_W=10+10+10=30
# df_B=2, df_W=12, MS_B=125, MS_W=2.5, F=50
add("FX-008 large-offset adversarial offset=1e12 F=50", 2, 12, 50.0)

# ── Boundary: F=0 ─────────────────────────────────────────────────────────────
# P(F>=0) = 1 for any positive-df F distribution.
add("boundary F=0 df=2,6", 2, 6, 0.0)
add("boundary F=0 df=1,30", 1, 30, 0.0)

# ── TST-026: small df, moderate tail ──────────────────────────────────────────
add("TST-026 df=1,1 F=161.4", 1, 1, 161.4)
add("TST-026 df=1,5 F=6.61", 1, 5, 6.61)
add("TST-026 df=2,6 F=5.14", 2, 6, 5.14)

# ── TST-027: moderate df ──────────────────────────────────────────────────────
add("TST-027 df=5,5 F=5.0", 5, 5, 5.0)
add("TST-027 df=3,100 F=2.70", 3, 100, 2.70)
add("TST-027 df=10,100 F=2.5", 10, 100, 2.5)

# ── TST-028: far left tail (tiny F, p near 1) ─────────────────────────────────
add("TST-028 df=2,6 F=0.001", 2, 6, 0.001)
add("TST-028 df=1,30 F=0.01", 1, 30, 0.01)

# ── TST-029: far right tail (large F, p near 0) ───────────────────────────────
add("TST-029 df=2,6 F=1000", 2, 6, 1000.0)
add("TST-029 df=1,30 F=4.17", 1, 30, 4.17)
add("TST-029 df=1,1000000 F=1000000", 1, 1000000, 1e6)

# ── TST-030: F at common alpha critical values ─────────────────────────────────
# alpha=0.05 critical values (approximate)
add("TST-030 alpha=0.05 critical df=1,1 F=161.45", 1, 1, 161.45)
add("TST-030 alpha=0.05 critical df=2,6 F=5.143", 2, 6, 5.143)
add("TST-030 alpha=0.01 critical df=2,6 F=10.925", 2, 6, 10.925)
add("TST-030 alpha=0.05 critical df=5,20 F=2.711", 5, 20, 2.711)

# ── Write CSV ─────────────────────────────────────────────────────────────────

df_out <- do.call(rbind, lapply(rows, as.data.frame, stringsAsFactors = FALSE))

# Emit p_value at full double precision (17 significant digits) so that
# the Go oracle test can verify within 1e-9 absolute tolerance without
# loss from decimal truncation.
options(digits = 17)

# Print R session info for provenance
cat("# R session info:\n")
cat(paste0("# ", capture.output(sessionInfo()), collapse = "\n"), "\n")
cat(sprintf("# Rows: %d\n", nrow(df_out)))
cat("# Columns: description,df_between,df_within,f_stat,p_value\n")

write.csv(df_out, file = "f_tail_fixtures.csv", row.names = FALSE)
cat("Written: f_tail_fixtures.csv\n")

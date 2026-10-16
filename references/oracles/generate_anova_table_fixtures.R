# generate_anova_table_fixtures.R
#
# Produces full one-way ANOVA table oracle fixtures for anova-go.
# Output: anova_table_fixtures.csv in the same directory.
#
# Usage:
#   cd references/oracles
#   Rscript generate_anova_table_fixtures.R
#
# R version used to produce the committed CSV:
#   R version 4.5.0 (2025-04-11) -- "How About a Twenty-Six"
#   Platform: aarch64-apple-darwin20
#
# aov() / summary.aov() is used to compute the ANOVA table.  R uses a
# numerically stable QR-based implementation.
#
# NOTE: FX-008 (large-offset adversarial) is intentionally excluded.  R's
# aov() suffers ~1.8e-5 relative cancellation error on that fixture (SS_B
# 250.004 vs. exact 250.000), whereas the Go Welford/Chan implementation
# yields the correct answer.  FX-008 is covered by TestMath_TranslationInvariance
# and TestSnapshot_Arithmetic in the Go test suite instead.
#
# TST-025 fixtures use fixed decimal values so the Go test can embed the same
# inputs without an R->Go random-seed translation step.
#
# Tolerance accepted by TestOracle_AnovaTable (TST-021-025): 1e-9 absolute.

anova_stats_full <- function(...) {
  groups <- list(...)
  labels <- names(groups)
  y <- unlist(groups)
  g <- factor(rep(labels, sapply(groups, length)), levels = labels)
  fit <- summary(aov(y ~ g))[[1]]
  list(
    ss_between = fit[["Sum Sq"]][1],
    ss_within  = fit[["Sum Sq"]][2],
    df1        = fit$Df[1],
    df2        = fit$Df[2],
    ms_between = fit[["Mean Sq"]][1],
    ms_within  = fit[["Mean Sq"]][2],
    f          = fit[["F value"]][1],
    p          = fit[["Pr(>F)"]][1]
  )
}

# ── Fixture accumulator ────────────────────────────────────────────────────────

rows <- list()

add_table <- function(description, ...) {
  s <- anova_stats_full(...)
  rows[[length(rows) + 1]] <<- list(
    description = description,
    ss_between  = s$ss_between,
    ss_within   = s$ss_within,
    df_between  = as.integer(s$df1),
    df_within   = as.integer(s$df2),
    ms_between  = s$ms_between,
    ms_within   = s$ms_within,
    f_stat      = s$f,
    p_value     = s$p
  )
}

# ── FX-001: balanced 3×3 ──────────────────────────────────────────────────────
# Hand-verified: SS_B=54, SS_W=6, df_B=2, df_W=6, MS_B=27, MS_W=1, F=27
add_table("FX-001 balanced 3x3 A=[1,2,3] B=[4,5,6] C=[7,8,9]",
  A=c(1,2,3), B=c(4,5,6), C=c(7,8,9))

# ── FX-002: unbalanced 3-group (n=4,2,3) ──────────────────────────────────────
# Hand-verified: SS_B=52.5, SS_W=7.5, df_B=2, df_W=6, MS_B=26.25, MS_W=1.25, F=21
add_table("FX-002 unbalanced 3-group n=4,2,3 A=[1,2,3,4] B=[5,6] C=[7,8,9]",
  A=c(1,2,3,4), B=c(5,6), C=c(7,8,9))

# ── FX-003: null-like (small F) ───────────────────────────────────────────────
# Hand-verified: SS_B=1.5, SS_W=16, df_B=1, df_W=4, MS_B=1.5, MS_W=4, F=0.375
add_table("FX-003 null-like A=[1,3,5] B=[2,4,6]",
  A=c(1,3,5), B=c(2,4,6))

# ── FX-004: clearly separated means (small p-value) ──────────────────────────
# Hand-verified: SS_B=121.5, SS_W=4, df_B=1, df_W=4, MS_B=121.5, MS_W=1, F=121.5
add_table("FX-004 separated means A=[1,2,3] B=[10,11,12]",
  A=c(1,2,3), B=c(10,11,12))

# ── TST-025: fixed-value non-trivial fixtures ──────────────────────────────────
# Values chosen to produce non-integer SS; embedded verbatim in the Go test.

add_table("TST-025a 3-group n=4,4,4 non-integer values",
  A=c(1.1, 2.3, 1.7, 2.9),
  B=c(4.2, 5.0, 4.8, 5.6),
  C=c(8.1, 7.5, 9.2, 8.6))

add_table("TST-025b 2-group n=5,3 with negative values",
  A=c(-1.5, -0.3, 0.7, -0.9, 1.1),
  B=c( 3.2,  4.8, 4.0))

add_table("TST-025c 4-group n=3,3,3,3 small spread",
  A=c(0.1, 0.2, 0.3),
  B=c(1.1, 1.2, 1.3),
  C=c(2.1, 2.2, 2.3),
  D=c(3.1, 3.2, 3.3))

# ── Write CSV ─────────────────────────────────────────────────────────────────
#
# write.csv() uses R's default format() which does NOT honour options(digits)
# reliably and may emit shortened representations that lose precision when
# parsed back as float64.  Write manually with sprintf("%.17g", ...) for
# every floating-point field.

df_out <- do.call(rbind, lapply(rows, as.data.frame, stringsAsFactors = FALSE))

# Print R session info for provenance
cat("# R session info:\n")
cat(paste0("# ", capture.output(sessionInfo()), collapse = "\n"), "\n")
cat(sprintf("# Rows: %d\n", nrow(df_out)))

con <- file("anova_table_fixtures.csv", open = "w")

# Header
writeLines("description,ss_between,ss_within,df_between,df_within,ms_between,ms_within,f_stat,p_value", con)

for (i in seq_len(nrow(df_out))) {
  line <- sprintf('"%s",%.17g,%.17g,%d,%d,%.17g,%.17g,%.17g,%.17g',
    df_out$description[i],
    df_out$ss_between[i], df_out$ss_within[i],
    df_out$df_between[i], df_out$df_within[i],
    df_out$ms_between[i], df_out$ms_within[i],
    df_out$f_stat[i],     df_out$p_value[i])
  writeLines(line, con)
}

close(con)
cat("Written: anova_table_fixtures.csv\n")

package anova

import (
	"encoding/csv"
	"math"
	"os"
	"strconv"
	"testing"

	"github.com/stephen-mcelhose/anova-go/internal/ftail"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOracle_AnovaTable [TST-021–025] compares the full one-way ANOVA table
// (SS_B, SS_W, df_B, df_W, MS_B, MS_W, F, p) against R 4.5.0 aov() output
// pinned in references/oracles/anova_table_fixtures.csv.
//
// FX-001–004 are hand-auditable with integer arithmetic.  TST-025a/b/c use
// fixed decimal inputs (embedded below) to avoid an R→Go RNG translation step.
//
// FX-008 (large-offset adversarial) is excluded from this oracle comparison:
// R's aov() has ~1.8e-5 relative cancellation error on that fixture, whereas
// the Go Welford/Chan implementation yields the provably correct answer.
// FX-008 is covered by TestMath_TranslationInvariance and TestSnapshot_Arithmetic.
func TestOracle_AnovaTable(t *testing.T) {
	const fixtureFile = "references/oracles/anova_table_fixtures.csv"
	const absTol = 1e-9

	// inputs maps each CSV description to the raw group data passed to OneWay.
	// Values must match the R script verbatim.
	inputs := map[string]map[string][]float64{
		"FX-001 balanced 3x3 A=[1,2,3] B=[4,5,6] C=[7,8,9]": {
			"A": {1, 2, 3}, "B": {4, 5, 6}, "C": {7, 8, 9},
		},
		"FX-002 unbalanced 3-group n=4,2,3 A=[1,2,3,4] B=[5,6] C=[7,8,9]": {
			"A": {1, 2, 3, 4}, "B": {5, 6}, "C": {7, 8, 9},
		},
		"FX-003 null-like A=[1,3,5] B=[2,4,6]": {
			"A": {1, 3, 5}, "B": {2, 4, 6},
		},
		"FX-004 separated means A=[1,2,3] B=[10,11,12]": {
			"A": {1, 2, 3}, "B": {10, 11, 12},
		},
		"TST-025a 3-group n=4,4,4 non-integer values": {
			"A": {1.1, 2.3, 1.7, 2.9},
			"B": {4.2, 5.0, 4.8, 5.6},
			"C": {8.1, 7.5, 9.2, 8.6},
		},
		"TST-025b 2-group n=5,3 with negative values": {
			"A": {-1.5, -0.3, 0.7, -0.9, 1.1},
			"B": {3.2, 4.8, 4.0},
		},
		"TST-025c 4-group n=3,3,3,3 small spread": {
			"A": {0.1, 0.2, 0.3},
			"B": {1.1, 1.2, 1.3},
			"C": {2.1, 2.2, 2.3},
			"D": {3.1, 3.2, 3.3},
		},
	}

	f, err := os.Open(fixtureFile)
	require.NoError(t, err, "MUST be able to open oracle fixture file %s", fixtureFile)
	defer f.Close()

	r := csv.NewReader(f)
	header, err := r.Read()
	require.NoError(t, err, "MUST read CSV header")
	require.Equal(t,
		[]string{"description", "ss_between", "ss_within", "df_between", "df_within", "ms_between", "ms_within", "f_stat", "p_value"},
		header, "MUST have expected CSV column order")

	records, err := r.ReadAll()
	require.NoError(t, err, "MUST parse all CSV rows without error")
	require.NotEmpty(t, records, "MUST have at least one fixture row")

	for _, rec := range records {
		rec := rec
		desc := rec[0]
		t.Run(desc, func(t *testing.T) {
			groups, ok := inputs[desc]
			require.True(t, ok, "MUST have input data for description %q — update inputs map if R script changed", desc)

			wantSSB, _ := strconv.ParseFloat(rec[1], 64)
			wantSSW, _ := strconv.ParseFloat(rec[2], 64)
			wantDFB, _ := strconv.ParseUint(rec[3], 10, 64)
			wantDFW, _ := strconv.ParseUint(rec[4], 10, 64)
			wantMSB, _ := strconv.ParseFloat(rec[5], 64)
			wantMSW, _ := strconv.ParseFloat(rec[6], 64)
			wantF, _ := strconv.ParseFloat(rec[7], 64)
			wantP, _ := strconv.ParseFloat(rec[8], 64)

			got, err := OneWay(groups)
			require.NoError(t, err)
			require.Equal(t, StatusOK, got.Status)

			assert.InDelta(t, wantSSB, got.BetweenSS, absTol, "BetweenSS")
			assert.InDelta(t, wantSSW, got.WithinSS, absTol, "WithinSS")
			assert.Equal(t, wantDFB, got.BetweenDF, "BetweenDF")
			assert.Equal(t, wantDFW, got.WithinDF, "WithinDF")
			assert.InDelta(t, wantMSB, got.BetweenMS, absTol, "BetweenMS")
			assert.InDelta(t, wantMSW, got.WithinMS, absTol, "WithinMS")
			assert.InDelta(t, wantF, got.F, absTol, "F")
			assert.InDelta(t, wantP, got.P, absTol, "P")
		})
	}
}

// TestOracle_FTail [TST-034] is the differential tail-test gate required by
// ADR-005. It reads references/oracles/f_tail_fixtures.csv, produced by
// references/oracles/generate_fixtures.R using R 4.5.0 pf(), and asserts that
// the anova package's F-tail adapter (gonum distuv.F.Survival) matches every
// reference value within 1e-9 absolute tolerance.
//
// P-values are release-blocked until this test is green.
//
// The fixtures cover:
//   - FX-001 through FX-004 canonical ANOVA cases
//   - FX-008 large-offset adversarial case
//   - F=0 boundary (p must equal 1)
//   - TST-026: small df, moderate-tail critical values
//   - TST-027: moderate df grid
//   - TST-028: far left tail (F near 0, p near 1)
//   - TST-029: far right tail (large F, p near 0)
//   - TST-030: common alpha critical-value boundaries
func TestOracle_FTail(t *testing.T) {
	const fixtureFile = "references/oracles/f_tail_fixtures.csv"
	const absTol = 1e-9

	f, err := os.Open(fixtureFile)
	require.NoError(t, err, "MUST be able to open oracle fixture file %s", fixtureFile)
	defer f.Close()

	r := csv.NewReader(f)
	// Skip header row.
	header, err := r.Read()
	require.NoError(t, err, "MUST read CSV header")
	require.Equal(t, []string{"description", "df_between", "df_within", "f_stat", "p_value"},
		header, "MUST have expected CSV column order")

	records, err := r.ReadAll()
	require.NoError(t, err, "MUST parse all CSV rows without error")
	require.NotEmpty(t, records, "MUST have at least one fixture row")

	for _, rec := range records {
		rec := rec // capture
		description := rec[0]

		t.Run(description, func(t *testing.T) {
			dfBetween, err := strconv.ParseUint(rec[1], 10, 64)
			require.NoError(t, err, "MUST parse df_between as uint64")

			dfWithin, err := strconv.ParseUint(rec[2], 10, 64)
			require.NoError(t, err, "MUST parse df_within as uint64")

			fStat, err := strconv.ParseFloat(rec[3], 64)
			require.NoError(t, err, "MUST parse f_stat as float64")

			wantP, err := strconv.ParseFloat(rec[4], 64)
			require.NoError(t, err, "MUST parse p_value as float64")

			gotP := ftail.PValue(fStat, dfBetween, dfWithin)

			require.False(t, math.IsNaN(gotP),
				"MUST NOT return NaN for valid parameters (df_between=%d df_within=%d f=%g)",
				dfBetween, dfWithin, fStat)

			assert.InDelta(t, wantP, gotP, absTol,
				"MUST match R pf() within %g absolute tolerance for "+
					"(df_between=%d df_within=%d f=%g)",
				absTol, dfBetween, dfWithin, fStat)
		})
	}
}

// TestFTailAdapter_Boundaries [TST-010] verifies that the F-tail adapter
// handles boundary and degenerate parameter combinations correctly without
// panicking and with explicit NaN or sentinel values as documented.
func TestFTailAdapter_Boundaries(t *testing.T) {
	tests := []struct {
		name       string
		f          float64
		dfBetween  uint64
		dfWithin   uint64
		wantNaN    bool
		wantApprox float64
	}{
		{
			name: "MUST return NaN when dfBetween is zero",
			f:    1.0, dfBetween: 0, dfWithin: 6,
			wantNaN: true,
		},
		{
			name: "MUST return NaN when dfWithin is zero",
			f:    1.0, dfBetween: 2, dfWithin: 0,
			wantNaN: true,
		},
		{
			name: "MUST return NaN for negative F",
			f:    -1.0, dfBetween: 2, dfWithin: 6,
			wantNaN: true,
		},
		{
			name: "MUST return NaN for NaN F",
			f:    math.NaN(), dfBetween: 2, dfWithin: 6,
			wantNaN: true,
		},
		{
			name: "MUST return 0 for F=+Inf",
			f:    math.Inf(1), dfBetween: 2, dfWithin: 6,
			wantNaN: false, wantApprox: 0,
		},
		{
			name: "MUST return 1 for F=0",
			f:    0, dfBetween: 2, dfWithin: 6,
			wantNaN: false, wantApprox: 1,
		},
		{
			name: "MUST return p in (0,1) for finite positive F",
			f:    27, dfBetween: 2, dfWithin: 6,
			wantNaN: false, wantApprox: 0.001,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := ftail.PValue(tt.f, tt.dfBetween, tt.dfWithin)
			if tt.wantNaN {
				assert.True(t, math.IsNaN(got),
					"MUST return NaN (got %v)", got)
			} else {
				require.False(t, math.IsNaN(got),
					"MUST NOT return NaN for valid parameters")
				assert.InDelta(t, tt.wantApprox, got, 1e-9,
					"MUST match expected value within 1e-9")
			}
		})
	}
}

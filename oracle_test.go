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

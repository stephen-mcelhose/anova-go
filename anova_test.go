package anova

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tol is a generous absolute tolerance for hand-worked float comparisons.
// Welford/Chan arithmetic on small integer datasets is exact to many ULPs.
const tol = 1e-10

// TestOneWay covers batch ANOVA arithmetic against hand-worked reference values.
// TST-004 / FX-001.
func TestOneWay(t *testing.T) {
	tests := []struct {
		name          string
		groups        map[string][]float64
		wantStatus    Status
		wantBetweenSS float64
		wantWithinSS  float64
		wantTotalSS   float64
		wantBetweenMS float64
		wantWithinMS  float64
		wantF         float64
		wantBetweenDF uint64
		wantWithinDF  uint64
		wantTotalDF   uint64
		wantOrder     []string
	}{
		{
			name:          "FX-001: balanced 3×3",
			groups:        map[string][]float64{"C": {7, 8, 9}, "A": {1, 2, 3}, "B": {4, 5, 6}},
			wantStatus:    StatusOK,
			wantBetweenSS: 54, wantWithinSS: 6, wantTotalSS: 60,
			wantBetweenMS: 27, wantWithinMS: 1, wantF: 27,
			wantBetweenDF: 2, wantWithinDF: 6, wantTotalDF: 8,
			wantOrder: []string{"A", "B", "C"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := OneWay(tt.groups)
			require.NoError(t, err)

			assert.Equal(t, tt.wantStatus, r.Status)
			assert.InDelta(t, tt.wantBetweenSS, r.BetweenSS, tol, "BetweenSS")
			assert.InDelta(t, tt.wantWithinSS, r.WithinSS, tol, "WithinSS")
			assert.InDelta(t, tt.wantTotalSS, r.TotalSS, tol, "TotalSS")
			assert.InDelta(t, tt.wantBetweenMS, r.BetweenMS, tol, "BetweenMS")
			assert.InDelta(t, tt.wantWithinMS, r.WithinMS, tol, "WithinMS")
			assert.InDelta(t, tt.wantF, r.F, tol, "F")
			assert.Equal(t, tt.wantBetweenDF, r.BetweenDF, "BetweenDF")
			assert.Equal(t, tt.wantWithinDF, r.WithinDF, "WithinDF")
			assert.Equal(t, tt.wantTotalDF, r.TotalDF, "TotalDF")
			assert.True(t, math.IsNaN(r.P), "P must be NaN until F-tail adapter is validated")

			if tt.wantOrder != nil {
				got := make([]string, len(r.Groups))
				for i, g := range r.Groups {
					got[i] = g.Group
				}
				assert.Equal(t, tt.wantOrder, got, "canonical group order")
			}
		})
	}
}

// TestDegenerateStatuses covers inputs that cannot produce a valid F statistic.
// TST-005 / TST-006.
func TestDegenerateStatuses(t *testing.T) {
	tests := []struct {
		name       string
		groups     map[string][]float64
		wantStatus Status
		wantInfF   bool
	}{
		{
			name:       "one empty group",
			groups:     map[string][]float64{"A": {1, 2}, "B": {}},
			wantStatus: StatusInsufficientGroups,
		},
		{
			name:       "one observation per group — no residual df",
			groups:     map[string][]float64{"A": {1}, "B": {2}},
			wantStatus: StatusInsufficientResidualDF,
		},
		{
			name:       "all values identical — 0/0",
			groups:     map[string][]float64{"A": {1, 1}, "B": {1, 1}},
			wantStatus: StatusIndeterminateZeroOverZero,
		},
		{
			name:       "zero within-group variance — infinite F",
			groups:     map[string][]float64{"A": {1, 1}, "B": {2, 2}},
			wantStatus: StatusInfiniteF,
			wantInfF:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := OneWay(tt.groups)
			require.NoError(t, err)
			assert.Equal(t, tt.wantStatus, r.Status)
			if tt.wantInfF {
				assert.True(t, math.IsInf(r.F, 1), "expected F = +Inf, got %v", r.F)
			}
		})
	}
}

// TestBatchStreamMergeEquivalence verifies that batch OneWay, streaming Add,
// and partition Merge produce identical ANOVA tables. TST-003 / TST-011 / TST-012.
func TestBatchStreamMergeEquivalence(t *testing.T) {
	all := map[string][]float64{
		"A": {1, 2, 3, 4},
		"B": {10, 12, 11},
		"C": {-3, -1, 0},
	}

	batch, err := OneWay(all)
	require.NoError(t, err)

	left, err := New("A", "B", "C")
	require.NoError(t, err)
	right, err := New("A", "B", "C")
	require.NoError(t, err)

	for _, v := range []float64{1, 3} {
		require.NoError(t, left.Add("A", v))
	}
	for _, v := range []float64{10, 11} {
		require.NoError(t, left.Add("B", v))
	}
	for _, v := range []float64{-3, 0} {
		require.NoError(t, left.Add("C", v))
	}
	for _, v := range []float64{2, 4} {
		require.NoError(t, right.Add("A", v))
	}
	require.NoError(t, right.Add("B", 12))
	require.NoError(t, right.Add("C", -1))

	require.NoError(t, left.Merge(right))
	stream := left.Snapshot()

	assert.InDelta(t, batch.BetweenSS, stream.BetweenSS, tol, "BetweenSS")
	assert.InDelta(t, batch.WithinSS, stream.WithinSS, tol, "WithinSS")
	assert.InDelta(t, batch.F, stream.F, tol, "F")
	assert.Equal(t, batch.Observations, stream.Observations, "Observations")
	assert.Equal(t, batch.WithinDF, stream.WithinDF, "WithinDF")
}

// TestAtomicity verifies that every failing Add or Merge leaves the
// accumulator state unchanged. TST-007 / TST-008 / TST-019 / TST-041 / TST-042.
func TestAtomicity(t *testing.T) {
	setup := func(t *testing.T) *Accumulator {
		t.Helper()
		a, err := New("A", "B")
		require.NoError(t, err)
		require.NoError(t, a.Add("A", 1))
		return a
	}

	tests := []struct {
		name    string
		op      func(*Accumulator) error
		wantErr error
	}{
		{
			name:    "NaN rejected",
			op:      func(a *Accumulator) error { return a.Add("A", math.NaN()) },
			wantErr: ErrNonFiniteValue,
		},
		{
			name:    "+Inf rejected",
			op:      func(a *Accumulator) error { return a.Add("A", math.Inf(1)) },
			wantErr: ErrNonFiniteValue,
		},
		{
			name:    "unknown group rejected",
			op:      func(a *Accumulator) error { return a.Add("missing", 1) },
			wantErr: ErrUnknownGroup,
		},
		{
			name:    "self-merge rejected",
			op:      func(a *Accumulator) error { return a.Merge(a) },
			wantErr: ErrSelfMerge,
		},
		{
			name: "incompatible merge rejected",
			op: func(a *Accumulator) error {
				other, _ := New("A", "C")
				return a.Merge(other)
			},
			wantErr: ErrIncompatible,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := setup(t)
			before := a.Snapshot()

			require.ErrorIs(t, tt.op(a), tt.wantErr)

			after := a.Snapshot()
			// Groups carries all mutable state; Status and Observations are derived.
			assert.Equal(t, before.Groups, after.Groups, "Groups mutated")
			assert.Equal(t, before.Status, after.Status, "Status changed")
			assert.Equal(t, before.Observations, after.Observations, "Observations changed")
		})
	}
}

// TestSnapshotPurity verifies Snapshot is non-mutating and returned results
// do not alias internal accumulator state. TST-009 / TST-018.
func TestSnapshotPurity(t *testing.T) {
	a, err := New("B", "A")
	require.NoError(t, err)
	for _, v := range []float64{1, 2} {
		require.NoError(t, a.Add("A", v))
	}
	for _, v := range []float64{3, 4} {
		require.NoError(t, a.Add("B", v))
	}

	one := a.Snapshot()
	two := a.Snapshot()

	assert.Equal(t, one.Status, two.Status)
	assert.Equal(t, one.Observations, two.Observations)
	assert.InDelta(t, one.BetweenSS, two.BetweenSS, tol)
	assert.InDelta(t, one.WithinSS, two.WithinSS, tol)
	assert.Equal(t, one.Groups, two.Groups)

	// Mutating the returned result must not reach the accumulator.
	one.Groups[0].Mean = 999
	one.EmptyGroups = append(one.EmptyGroups, "injected")

	three := a.Snapshot()
	assert.NotEqual(t, 999.0, three.Groups[0].Mean, "result aliased accumulator Groups")
	assert.Equal(t, len(two.EmptyGroups), len(three.EmptyGroups), "result aliased accumulator EmptyGroups")
}

// TestTranslationInvariance verifies that adding a large constant offset to all
// observations does not change SS or F values. TST-013 / TST-031.
func TestTranslationInvariance(t *testing.T) {
	base := map[string][]float64{
		"A": {1, 2, 3, 4, 5},
		"B": {2, 3, 4, 5, 6},
		"C": {-1, 0, 1, 2, 3},
	}

	tests := []struct {
		name   string
		offset float64
	}{
		{"offset +1e12", 1e12},
		{"offset -1e12", -1e12},
	}

	ref, err := OneWay(base)
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shifted := make(map[string][]float64, len(base))
			for g, xs := range base {
				s := make([]float64, len(xs))
				for i, x := range xs {
					s[i] = tt.offset + x
				}
				shifted[g] = s
			}
			r, err := OneWay(shifted)
			require.NoError(t, err)
			assert.InDelta(t, ref.BetweenSS, r.BetweenSS, tol, "BetweenSS")
			assert.InDelta(t, ref.WithinSS, r.WithinSS, tol, "WithinSS")
			assert.InDelta(t, ref.F, r.F, tol, "F")
		})
	}
}

// TestOverflowRejectedAtomically verifies that an Add whose Welford update
// would overflow to ±Inf is rejected and leaves state unchanged. TST-032.
func TestOverflowRejectedAtomically(t *testing.T) {
	a, err := New("A", "B")
	require.NoError(t, err)
	require.NoError(t, a.Add("A", math.MaxFloat64))

	before := a.Snapshot()

	// Second MaxFloat64 with opposite sign: delta = -2·MaxFloat64 → -Inf.
	require.ErrorIs(t, a.Add("A", -math.MaxFloat64), ErrNumericalFailure)

	after := a.Snapshot()
	assert.Equal(t, before.Groups, after.Groups, "accumulator mutated by overflowing Add")
}

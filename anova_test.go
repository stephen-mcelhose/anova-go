package anova

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tol is an absolute tolerance for float64 assertions.
// Welford/Chan arithmetic on small-integer datasets is exact to many ULPs.
// Only use tol for normal-scale fixtures (SS values O(1)–O(1000)).
// For extreme-scale data use withinTol instead — see wiki/04 §Tolerance.
const tol = 1e-10

// withinTol implements the wiki/04-numerical-stability.md §Recommended tolerance
// contract: |a−b| ≤ absTol + relTol·max(|a|,|b|).
//
// Use relTol ≈ 1e-12 for Chan-merge comparisons (a few ULPs at any scale).
// absTol catches the near-zero case where both values are tiny.
func withinTol(a, b, absTol, relTol float64) bool {
	diff := math.Abs(a - b)
	scale := math.Max(math.Abs(a), math.Abs(b))
	return diff <= absTol+relTol*scale
}

// assertWithinTol calls t.Errorf if withinTol(a, b, absTol, relTol) is false.
func assertWithinTol(t *testing.T, a, b, absTol, relTol float64, label string) {
	t.Helper()
	if !withinTol(a, b, absTol, relTol) {
		diff := math.Abs(a - b)
		scale := math.Max(math.Abs(a), math.Abs(b))
		t.Errorf("%s: |%g − %g| = %g > absTol(%g) + relTol(%g)·scale(%g) = %g",
			label, a, b, diff, absTol, relTol, scale, absTol+relTol*scale)
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

// obs adds values to a group, failing the test immediately on any error.
func obs(t *testing.T, a *Accumulator, group string, vals ...float64) {
	t.Helper()
	for _, v := range vals {
		require.NoError(t, a.Add(group, v))
	}
}

// summary returns the GroupSummary for group from the current Snapshot.
func summary(t *testing.T, a *Accumulator, group string) GroupSummary {
	t.Helper()
	for _, gs := range a.Snapshot().Groups {
		if gs.Group == group {
			return gs
		}
	}
	t.Fatalf("group %q not found in snapshot", group)
	return GroupSummary{}
}

// newAcc constructs an Accumulator or fails the test.
func newAcc(t *testing.T, groups ...string) *Accumulator {
	t.Helper()
	a, err := New(groups...)
	require.NoError(t, err)
	return a
}

// assertMathInvariants checks the pure mathematical identities on any valid
// Result. Extracted so it can be reused by a future FuzzMath target.
func assertMathInvariants(t *testing.T, r Result) {
	t.Helper()
	if r.Status != StatusOK && r.Status != StatusNumericalWarning {
		return
	}
	assert.InDelta(t, r.BetweenSS+r.WithinSS, r.TotalSS, tol,
		"SS_T MUST equal SS_B plus SS_W")
	assert.GreaterOrEqual(t, r.BetweenSS, 0.0,
		"SS_B MUST be non-negative")
	assert.GreaterOrEqual(t, r.WithinSS, 0.0,
		"SS_W MUST be non-negative")
	assert.InDelta(t, r.BetweenMS/r.WithinMS, r.F, tol,
		"F MUST equal MS_B divided by MS_W")
	// StatusOK results carry a real p-value from the F-tail adapter.
	// StatusNumericalWarning results retain NaN because the adapter is not
	// invoked when SS clamping has occurred.
	if r.Status == StatusOK {
		assert.True(t, r.P >= 0 && r.P <= 1,
			"P MUST be in [0,1] for StatusOK")
	} else {
		assert.True(t, math.IsNaN(r.P),
			"P MUST be NaN for non-OK status")
	}
}

// ── New ──────────────────────────────────────────────────────────────────────

func TestNew_Validation(t *testing.T) {
	tests := []struct {
		name    string
		groups  []string
		wantErr error
	}{
		{
			name:    "MUST return ErrInvalidGroup when no groups are provided",
			groups:  []string{},
			wantErr: ErrInvalidGroup,
		},
		{
			name:    "MUST return ErrInvalidGroup when any group name is the empty string",
			groups:  []string{"A", "", "B"},
			wantErr: ErrInvalidGroup,
		},
		{
			name:    "MUST return ErrDuplicateGroup when group names are not unique",
			groups:  []string{"A", "B", "A"},
			wantErr: ErrDuplicateGroup,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.groups...)
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestNew_Initialization(t *testing.T) {
	tests := []struct {
		name      string
		groups    []string
		wantOrder []string
	}{
		{
			name:      "MUST store groups in canonical lexicographic order regardless of input order",
			groups:    []string{"C", "A", "B"},
			wantOrder: []string{"A", "B", "C"},
		},
		{
			name:      "MUST initialize every group with zero count, zero mean, and zero M2",
			groups:    []string{"X", "Y"},
			wantOrder: []string{"X", "Y"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, err := New(tt.groups...)
			require.NoError(t, err)
			r := a.Snapshot()

			got := make([]string, len(r.Groups))
			for i, g := range r.Groups {
				got[i] = g.Group
				assert.Equal(t, uint64(0), g.Count, "group %q Count", g.Group)
				assert.Equal(t, 0.0, g.Mean, "group %q Mean", g.Group)
				assert.Equal(t, 0.0, g.M2, "group %q M2", g.Group)
			}
			assert.Equal(t, tt.wantOrder, got)
		})
	}
}

// ── Add: input validation ─────────────────────────────────────────────────────

func TestAdd_NonFiniteRejection(t *testing.T) {
	tests := []struct {
		name  string
		value float64
	}{
		{
			name:  "MUST return ErrNonFiniteValue for NaN",
			value: math.NaN(),
		},
		{
			name:  "MUST return ErrNonFiniteValue for positive infinity",
			value: math.Inf(1),
		},
		{
			name:  "MUST return ErrNonFiniteValue for negative infinity",
			value: math.Inf(-1),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newAcc(t, "A", "B")
			require.ErrorIs(t, a.Add("A", tt.value), ErrNonFiniteValue)
		})
	}
}

// TestAdd_UnknownGroupRejection verifies that Add rejects observations for
// groups not in the registered universe. TST-008.
func TestAdd_UnknownGroupRejection(t *testing.T) {
	tests := []struct {
		name  string
		group string
	}{
		{
			name:  "MUST return ErrUnknownGroup for a name absent from the registered universe",
			group: "NOT_REGISTERED",
		},
		{
			name:  "MUST return ErrUnknownGroup for a name that is a prefix of a registered group",
			group: "A", // registry has "AB", not "A"
		},
	}

	a := newAcc(t, "AB", "CD")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.ErrorIs(t, a.Add(tt.group, 1.0), ErrUnknownGroup)
		})
	}
}

// TestAdd_OverflowRejection covers the case where the Welford intermediate
// overflows to a non-finite value (TST-032). The delta between two ±MaxFloat64
// observations exceeds the representable range.
func TestAdd_OverflowRejection(t *testing.T) {
	tests := []struct {
		name  string
		seed  float64
		value float64
	}{
		{
			name:  "MUST return ErrNumericalFailure when the Welford delta overflows to negative infinity",
			seed:  math.MaxFloat64,
			value: -math.MaxFloat64,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newAcc(t, "A", "B")
			require.NoError(t, a.Add("A", tt.seed))
			require.ErrorIs(t, a.Add("A", tt.value), ErrNumericalFailure)
		})
	}
}

// ── Add: Welford recurrence ───────────────────────────────────────────────────

// TestAdd_WelfordRecurrence verifies that Add maintains the centered sufficient
// statistics (Count, Mean, M2) per the Welford online algorithm. TST-001/TST-002.
func TestAdd_WelfordRecurrence(t *testing.T) {
	tests := []struct {
		name     string
		values   []float64
		wantN    uint64
		wantMean float64
		wantM2   float64
	}{
		{
			// First observation is the base case: n=1, mean=x, M2=0.
			name:     "MUST set n=1, mean=x, M2=0 for the first observation",
			values:   []float64{7},
			wantN:    1,
			wantMean: 7,
			wantM2:   0,
		},
		{
			// Two observations: mean=(1+3)/2=2, M2=(1-2)²+(3-2)²=2.
			name:     "MUST update n, mean, and M2 correctly for two observations",
			values:   []float64{1, 3},
			wantN:    2,
			wantMean: 2,
			wantM2:   2,
		},
		{
			// Three observations {1,2,3}: mean=2, sample var=1, M2=(n-1)*var=2.
			name:     "MUST maintain the running M2 correctly for three observations",
			values:   []float64{1, 2, 3},
			wantN:    3,
			wantMean: 2,
			wantM2:   2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newAcc(t, "A", "B")
			obs(t, a, "A", tt.values...)
			gs := summary(t, a, "A")
			assert.Equal(t, tt.wantN, gs.Count, "Count")
			assert.InDelta(t, tt.wantMean, gs.Mean, tol, "Mean")
			assert.InDelta(t, tt.wantM2, gs.M2, tol, "M2")
		})
	}
}

// ── Add: atomicity ────────────────────────────────────────────────────────────

// TestAdd_Atomicity verifies that every rejected Add leaves the accumulator
// state byte-for-byte identical to its pre-call state. TST-007/TST-019.
func TestAdd_Atomicity(t *testing.T) {
	tests := []struct {
		name    string
		op      func(*Accumulator) error
		wantErr error
	}{
		{
			name:    "MUST NOT mutate state when the value is NaN",
			op:      func(a *Accumulator) error { return a.Add("A", math.NaN()) },
			wantErr: ErrNonFiniteValue,
		},
		{
			name:    "MUST NOT mutate state when the value is positive infinity",
			op:      func(a *Accumulator) error { return a.Add("A", math.Inf(1)) },
			wantErr: ErrNonFiniteValue,
		},
		{
			name:    "MUST NOT mutate state when the value is negative infinity",
			op:      func(a *Accumulator) error { return a.Add("A", math.Inf(-1)) },
			wantErr: ErrNonFiniteValue,
		},
		{
			name:    "MUST NOT mutate state when the group name is not registered",
			op:      func(a *Accumulator) error { return a.Add("MISSING", 1.0) },
			wantErr: ErrUnknownGroup,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newAcc(t, "A", "B")
			obs(t, a, "A", 1.0)
			before := a.Snapshot().Groups

			require.ErrorIs(t, tt.op(a), tt.wantErr)

			assert.Equal(t, before, a.Snapshot().Groups,
				"Groups MUST be unchanged after a failed Add")
		})
	}

	// Overflow atomicity uses a different seed so it gets its own sub-test.
	t.Run("MUST NOT mutate state when the Welford delta overflows", func(t *testing.T) {
		a := newAcc(t, "A", "B")
		require.NoError(t, a.Add("A", math.MaxFloat64))
		before := a.Snapshot().Groups

		require.ErrorIs(t, a.Add("A", -math.MaxFloat64), ErrNumericalFailure)

		assert.Equal(t, before, a.Snapshot().Groups,
			"Groups MUST be unchanged after overflow")
	})
}

// ── Merge: validation ─────────────────────────────────────────────────────────

func TestMerge_Validation(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T) (a, b *Accumulator)
		wantErr error
	}{
		{
			name: "MUST return ErrSelfMerge when the receiver and argument are the same pointer",
			setup: func(t *testing.T) (*Accumulator, *Accumulator) {
				a := newAcc(t, "A", "B")
				return a, a
			},
			wantErr: ErrSelfMerge,
		},
		{
			name: "MUST return ErrIncompatible when the group counts differ",
			setup: func(t *testing.T) (*Accumulator, *Accumulator) {
				return newAcc(t, "A", "B"), newAcc(t, "A", "B", "C")
			},
			wantErr: ErrIncompatible,
		},
		{
			name: "MUST return ErrIncompatible when group names differ even at equal count",
			setup: func(t *testing.T) (*Accumulator, *Accumulator) {
				return newAcc(t, "A", "B"), newAcc(t, "A", "C")
			},
			wantErr: ErrIncompatible,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, b := tt.setup(t)
			require.ErrorIs(t, a.Merge(b), tt.wantErr)
		})
	}
}

// ── Merge: Chan mechanics ─────────────────────────────────────────────────────

// TestMerge_ChanMechanics verifies that Merge applies the Chan parallel
// update formula correctly. TST-003.
//
// Left partition A=[1,3], B=[10,11]; right partition A=[2,4], B=[12].
// Expected merged state per hand calculation:
//
//	A: n=4, mean=2.5, M2=5  ({1,2,3,4}: mean=2.5, M2=Σ(xᵢ-2.5)²=5)
//	B: n=3, mean=11,  M2=2  ({10,11,12}: mean=11, M2=2)
func TestMerge_ChanMechanics(t *testing.T) {
	left := newAcc(t, "A", "B")
	obs(t, left, "A", 1, 3)
	obs(t, left, "B", 10, 11)

	right := newAcc(t, "A", "B")
	obs(t, right, "A", 2, 4)
	obs(t, right, "B", 12)

	require.NoError(t, left.Merge(right))

	tests := []struct {
		name     string
		group    string
		wantN    uint64
		wantMean float64
		wantM2   float64
	}{
		{
			name:     "MUST merge group A to n=4, mean=2.5, M2=5",
			group:    "A",
			wantN:    4,
			wantMean: 2.5,
			wantM2:   5,
		},
		{
			name:     "MUST merge group B to n=3, mean=11, M2=2",
			group:    "B",
			wantN:    3,
			wantMean: 11,
			wantM2:   2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gs := summary(t, left, tt.group)
			assert.Equal(t, tt.wantN, gs.Count, "Count")
			assert.InDelta(t, tt.wantMean, gs.Mean, tol, "Mean")
			assert.InDelta(t, tt.wantM2, gs.M2, tol, "M2")
		})
	}
}

// ── Merge: atomicity ──────────────────────────────────────────────────────────

// TestMerge_Atomicity verifies that every rejected Merge leaves the receiver
// state byte-for-byte identical to its pre-call state. TST-041/TST-042.
func TestMerge_Atomicity(t *testing.T) {
	tests := []struct {
		name    string
		op      func(t *testing.T, a *Accumulator) error
		wantErr error
	}{
		{
			name:    "MUST NOT mutate state when the merge target is self",
			op:      func(_ *testing.T, a *Accumulator) error { return a.Merge(a) },
			wantErr: ErrSelfMerge,
		},
		{
			name: "MUST NOT mutate state when group schemas are incompatible",
			op: func(t *testing.T, a *Accumulator) error {
				other := newAcc(t, "A", "C")
				return a.Merge(other)
			},
			wantErr: ErrIncompatible,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newAcc(t, "A", "B")
			obs(t, a, "A", 1.0, 2.0)
			before := a.Snapshot().Groups

			require.ErrorIs(t, tt.op(t, a), tt.wantErr)

			assert.Equal(t, before, a.Snapshot().Groups,
				"Groups MUST be unchanged after a failed Merge")
		})
	}
}

// ── Load ──────────────────────────────────────────────────────────────────────

func TestLoad_Validation(t *testing.T) {
	tests := []struct {
		name    string
		groups  []GroupSummary
		wantErr error
	}{
		{
			name:    "MUST return ErrInvalidGroup when the group list is empty",
			groups:  []GroupSummary{},
			wantErr: ErrInvalidGroup,
		},
		{
			name:    "MUST return ErrInvalidGroup when any group name is the empty string",
			groups:  []GroupSummary{{Group: "", Count: 0}},
			wantErr: ErrInvalidGroup,
		},
		{
			name:    "MUST return ErrDuplicateGroup when group names are not unique",
			groups:  []GroupSummary{{Group: "A", Count: 0}, {Group: "A", Count: 1, Mean: 1}},
			wantErr: ErrDuplicateGroup,
		},
		{
			name:    "MUST return ErrCountLimit when any group count exceeds MaxExactCount",
			groups:  []GroupSummary{{Group: "A", Count: MaxExactCount + 1}},
			wantErr: ErrCountLimit,
		},
		{
			name:    "MUST return ErrNumericalFailure when mean is NaN for a non-empty group",
			groups:  []GroupSummary{{Group: "A", Count: 1, Mean: math.NaN(), M2: 0}},
			wantErr: ErrNumericalFailure,
		},
		{
			name:    "MUST return ErrNumericalFailure when M2 is negative",
			groups:  []GroupSummary{{Group: "A", Count: 2, Mean: 1, M2: -1}},
			wantErr: ErrNumericalFailure,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load(tt.groups)
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestLoad_StatePreservation(t *testing.T) {
	tests := []struct {
		name   string
		groups []GroupSummary
	}{
		{
			name: "MUST preserve Count, Mean, and M2 for each loaded group",
			groups: []GroupSummary{
				{Group: "A", Count: 3, Mean: 2, M2: 2},
				{Group: "B", Count: 3, Mean: 5, M2: 2},
			},
		},
		{
			name: "MUST zero Mean and M2 for groups with Count == 0",
			groups: []GroupSummary{
				{Group: "A", Count: 0, Mean: 999, M2: 999},
			},
		},
		{
			name: "MUST sort groups into canonical order regardless of input order",
			groups: []GroupSummary{
				{Group: "C", Count: 1, Mean: 7, M2: 0},
				{Group: "A", Count: 1, Mean: 1, M2: 0},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, err := Load(tt.groups)
			require.NoError(t, err)
			snap := a.Snapshot()

			byGroup := make(map[string]GroupSummary, len(snap.Groups))
			for _, gs := range snap.Groups {
				byGroup[gs.Group] = gs
			}

			for _, want := range tt.groups {
				got, ok := byGroup[want.Group]
				require.True(t, ok, "group %q missing from snapshot", want.Group)
				if want.Count == 0 {
					assert.Equal(t, uint64(0), got.Count)
					assert.Equal(t, 0.0, got.Mean, "Mean MUST be zeroed for empty groups")
					assert.Equal(t, 0.0, got.M2, "M2 MUST be zeroed for empty groups")
				} else {
					assert.Equal(t, want.Count, got.Count)
					assert.InDelta(t, want.Mean, got.Mean, tol)
					assert.InDelta(t, want.M2, got.M2, tol)
				}
			}
		})
	}
}

// TestLoad_MatchesAccumulatedState verifies that a loaded accumulator produces
// the same Snapshot as one that reached the same state through sequential Add.
func TestLoad_MatchesAccumulatedState(t *testing.T) {
	built := newAcc(t, "A", "B", "C")
	obs(t, built, "A", 1, 2, 3)
	obs(t, built, "B", 4, 5, 6)
	obs(t, built, "C", 7, 8, 9)
	snapBuilt := built.Snapshot()

	loaded, err := Load(snapBuilt.Groups)
	require.NoError(t, err)
	snapLoaded := loaded.Snapshot()

	assert.Equal(t, snapBuilt.Status, snapLoaded.Status)
	assert.Equal(t, snapBuilt.Groups, snapLoaded.Groups)
	assert.InDelta(t, snapBuilt.BetweenSS, snapLoaded.BetweenSS, tol)
	assert.InDelta(t, snapBuilt.WithinSS, snapLoaded.WithinSS, tol)
	assert.InDelta(t, snapBuilt.F, snapLoaded.F, tol)
}

// ── MaxExactCount boundary (TST-033) ──────────────────────────────────────────

// TestCountLimit uses Load to place an accumulator at the MaxExactCount
// boundary without looping 2^53 times, then verifies the rejection and
// success semantics of Add and Merge at that edge. TST-033.
func TestCountLimit(t *testing.T) {
	const half = MaxExactCount / 2

	t.Run("MUST return ErrCountLimit when Add would push a group past MaxExactCount", func(t *testing.T) {
		a, err := Load([]GroupSummary{
			{Group: "A", Count: MaxExactCount, Mean: 0, M2: 0},
			{Group: "B", Count: 0},
		})
		require.NoError(t, err)
		before := a.Snapshot().Groups

		require.ErrorIs(t, a.Add("A", 1.0), ErrCountLimit)

		assert.Equal(t, before, a.Snapshot().Groups,
			"MUST NOT mutate state when Add is rejected at MaxExactCount boundary")
	})

	t.Run("MUST return ErrCountLimit when Merge would push total count past MaxExactCount", func(t *testing.T) {
		a, err := Load([]GroupSummary{
			{Group: "A", Count: half + 1, Mean: 0, M2: 0},
			{Group: "B", Count: 0},
		})
		require.NoError(t, err)

		other, err := Load([]GroupSummary{
			{Group: "A", Count: half, Mean: 0, M2: 0},
			{Group: "B", Count: 0},
		})
		require.NoError(t, err)

		before := a.Snapshot().Groups
		require.ErrorIs(t, a.Merge(other), ErrCountLimit)
		assert.Equal(t, before, a.Snapshot().Groups,
			"MUST NOT mutate state when Merge is rejected at MaxExactCount boundary")
	})

	t.Run("MUST succeed when Merge brings total count to exactly MaxExactCount", func(t *testing.T) {
		a, err := Load([]GroupSummary{
			{Group: "A", Count: half, Mean: 0, M2: 0},
			{Group: "B", Count: 0},
		})
		require.NoError(t, err)

		other, err := Load([]GroupSummary{
			{Group: "A", Count: half, Mean: 0, M2: 0},
			{Group: "B", Count: 0},
		})
		require.NoError(t, err)

		require.NoError(t, a.Merge(other))
		gs := summary(t, a, "A")
		assert.Equal(t, MaxExactCount, gs.Count,
			"MUST accept a merge that lands exactly on MaxExactCount")
	})
}

// ── Snapshot: status determination ───────────────────────────────────────────

// TestSnapshot_Status exhausts the Status enumeration. Every code path that
// returns a non-OK status must appear here. TST-005/TST-006.
func TestSnapshot_Status(t *testing.T) {
	tests := []struct {
		name       string
		groups     map[string][]float64
		wantStatus Status
		extraCheck func(t *testing.T, r Result)
	}{
		{
			name:       "MUST return StatusInsufficientGroups when fewer than 2 groups have any observations",
			groups:     map[string][]float64{"A": {1, 2}, "B": {}},
			wantStatus: StatusInsufficientGroups,
		},
		{
			name:       "MUST return StatusInsufficientGroups when only one group is registered",
			groups:     map[string][]float64{"A": {1, 2, 3}},
			wantStatus: StatusInsufficientGroups,
		},
		{
			name:       "MUST return StatusInsufficientResidualDF when total observations do not exceed non-empty group count",
			groups:     map[string][]float64{"A": {1}, "B": {2}},
			wantStatus: StatusInsufficientResidualDF,
		},
		{
			name:       "MUST return StatusIndeterminateZeroOverZero when within-SS and between-SS are both zero",
			groups:     map[string][]float64{"A": {1, 1}, "B": {1, 1}},
			wantStatus: StatusIndeterminateZeroOverZero,
		},
		{
			name:       "MUST return StatusInfiniteF when within-SS is zero and between-SS is strictly positive",
			groups:     map[string][]float64{"A": {1, 1}, "B": {2, 2}},
			wantStatus: StatusInfiniteF,
			extraCheck: func(t *testing.T, r Result) {
				assert.True(t, math.IsInf(r.F, 1), "F MUST be +Inf when within-SS is zero")
			},
		},
		{
			name:       "MUST return StatusOK for a fully valid ANOVA computation",
			groups:     map[string][]float64{"A": {1, 2, 3}, "B": {4, 5, 6}, "C": {7, 8, 9}},
			wantStatus: StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := OneWay(tt.groups)
			require.NoError(t, err)
			assert.Equal(t, tt.wantStatus, r.Status)
			if tt.extraCheck != nil {
				tt.extraCheck(t, r)
			}
		})
	}
}

// TestSnapshot_EmptyGroupTracking verifies that groups with no observations
// are listed in EmptyGroups and excluded from the ANOVA computation.
func TestSnapshot_EmptyGroupTracking(t *testing.T) {
	tests := []struct {
		name            string
		groups          map[string][]float64
		wantEmptyGroups []string
		wantNonEmpty    uint64
	}{
		{
			name:            "MUST list groups with zero observations in EmptyGroups",
			groups:          map[string][]float64{"A": {1, 2}, "B": {}, "C": {3, 4}},
			wantEmptyGroups: []string{"B"},
			wantNonEmpty:    2,
		},
		{
			name:            "MUST list all groups in EmptyGroups when none have observations",
			groups:          map[string][]float64{"A": {}, "B": {}},
			wantEmptyGroups: []string{"A", "B"},
			wantNonEmpty:    0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := OneWay(tt.groups)
			require.NoError(t, err)
			assert.Equal(t, tt.wantEmptyGroups, r.EmptyGroups)
			assert.Equal(t, tt.wantNonEmpty, r.NonEmptyGroup)
		})
	}
}

// ── Snapshot: arithmetic ──────────────────────────────────────────────────────

// TestSnapshot_Arithmetic verifies every computed field in the ANOVA table
// against hand-audited fixtures. TST-004.
func TestSnapshot_Arithmetic(t *testing.T) {
	tests := []struct {
		name          string
		groups        map[string][]float64
		wantBetweenSS float64
		wantWithinSS  float64
		wantTotalSS   float64
		wantBetweenDF uint64
		wantWithinDF  uint64
		wantTotalDF   uint64
		wantBetweenMS float64
		wantWithinMS  float64
		wantF         float64
		wantOrder     []string
	}{
		{
			// FX-001: balanced 3×3. Hand-verifiable with integer arithmetic.
			// SS_B = 3·[(1-5)²+(4-5)²+(7-5)²]·... actually:
			// Grand mean = 5. Group means: A=2, B=5, C=8.
			// SS_B = 3·(9+0+9) = 54. SS_W = 2·(1+1+1) = 6.
			name:          "FX-001: MUST compute correct table for balanced 3×3 (A=[1,2,3] B=[4,5,6] C=[7,8,9])",
			groups:        map[string][]float64{"C": {7, 8, 9}, "A": {1, 2, 3}, "B": {4, 5, 6}},
			wantBetweenSS: 54, wantWithinSS: 6, wantTotalSS: 60,
			wantBetweenDF: 2, wantWithinDF: 6, wantTotalDF: 8,
			wantBetweenMS: 27, wantWithinMS: 1, wantF: 27,
			wantOrder: []string{"A", "B", "C"},
		},
		{
			// FX-002: unbalanced 3-group (n=[4,2,3]). Hand-verified.
			// A=[1,2,3,4] mean=2.5; B=[5,6] mean=5.5; C=[7,8,9] mean=8.
			// Grand mean=(10+11+24)/9=5.
			// SS_B = 4(2.5-5)²+2(5.5-5)²+3(8-5)² = 25+0.5+27 = 52.5
			// SS_W = 5 + 0.5 + 2 = 7.5
			name:          "FX-002: MUST compute correct table for unbalanced 3-group (n=4,2,3)",
			groups:        map[string][]float64{"A": {1, 2, 3, 4}, "B": {5, 6}, "C": {7, 8, 9}},
			wantBetweenSS: 52.5, wantWithinSS: 7.5, wantTotalSS: 60,
			wantBetweenDF: 2, wantWithinDF: 6, wantTotalDF: 8,
			wantBetweenMS: 26.25, wantWithinMS: 1.25, wantF: 21,
			wantOrder: []string{"A", "B", "C"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := OneWay(tt.groups)
			require.NoError(t, err)
			require.Equal(t, StatusOK, r.Status)

			assert.InDelta(t, tt.wantBetweenSS, r.BetweenSS, tol, "BetweenSS")
			assert.InDelta(t, tt.wantWithinSS, r.WithinSS, tol, "WithinSS")
			assert.InDelta(t, tt.wantTotalSS, r.TotalSS, tol, "TotalSS")
			assert.Equal(t, tt.wantBetweenDF, r.BetweenDF, "BetweenDF")
			assert.Equal(t, tt.wantWithinDF, r.WithinDF, "WithinDF")
			assert.Equal(t, tt.wantTotalDF, r.TotalDF, "TotalDF")
			assert.InDelta(t, tt.wantBetweenMS, r.BetweenMS, tol, "BetweenMS")
			assert.InDelta(t, tt.wantWithinMS, r.WithinMS, tol, "WithinMS")
			assert.InDelta(t, tt.wantF, r.F, tol, "F")
			assert.True(t, r.P >= 0 && r.P <= 1,
				"P MUST be in [0,1] for StatusOK")

			got := make([]string, len(r.Groups))
			for i, g := range r.Groups {
				got[i] = g.Group
			}
			assert.Equal(t, tt.wantOrder, got, "MUST present groups in canonical lexicographic order")
		})
	}
}

// ── Snapshot: purity ──────────────────────────────────────────────────────────

// TestSnapshot_Purity verifies that Snapshot is non-mutating and that
// callers cannot reach internal state through returned values. TST-009/TST-018.
func TestSnapshot_Purity(t *testing.T) {
	t.Run("MUST return identical results for two consecutive Snapshots without intervening Add", func(t *testing.T) {
		a := newAcc(t, "A", "B")
		obs(t, a, "A", 1, 2)
		obs(t, a, "B", 3, 4)

		one := a.Snapshot()
		two := a.Snapshot()

		assert.Equal(t, one.Status, two.Status)
		assert.Equal(t, one.Observations, two.Observations)
		assert.InDelta(t, one.BetweenSS, two.BetweenSS, tol)
		assert.InDelta(t, one.WithinSS, two.WithinSS, tol)
		assert.Equal(t, one.Groups, two.Groups)
	})

	t.Run("MUST NOT allow mutation of returned Groups to alter subsequent Snapshots", func(t *testing.T) {
		a := newAcc(t, "A", "B")
		obs(t, a, "A", 1, 2)
		obs(t, a, "B", 3, 4)

		r := a.Snapshot()
		r.Groups[0].Mean = 999

		fresh := a.Snapshot()
		assert.NotEqual(t, 999.0, fresh.Groups[0].Mean,
			"modifying a returned GroupSummary MUST NOT affect the accumulator")
	})

	t.Run("MUST NOT allow mutation of returned EmptyGroups to alter subsequent Snapshots", func(t *testing.T) {
		a := newAcc(t, "A", "B", "C")
		obs(t, a, "A", 1)
		obs(t, a, "B", 2)
		// C is empty

		r := a.Snapshot()
		before := len(r.EmptyGroups)
		r.EmptyGroups = append(r.EmptyGroups, "injected")

		fresh := a.Snapshot()
		assert.Equal(t, before, len(fresh.EmptyGroups),
			"appending to a returned EmptyGroups MUST NOT affect the accumulator")
	})
}

// ── Math: pure invariants (fuzz-ready) ───────────────────────────────────────
//
// Each test calls assertMathInvariants, which can be lifted directly into a
// FuzzMath target once fixtures are generated externally.

var mathFixtures = []struct {
	name   string
	groups map[string][]float64
}{
	{
		name:   "FX-001: balanced 3×3",
		groups: map[string][]float64{"A": {1, 2, 3}, "B": {4, 5, 6}, "C": {7, 8, 9}},
	},
	{
		name:   "FX-002: unbalanced 3-group",
		groups: map[string][]float64{"A": {1, 2, 3, 4}, "B": {5, 6}, "C": {7, 8, 9}},
	},
	{
		name:   "two groups, many observations",
		groups: map[string][]float64{"A": {1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, "B": {11, 12, 13, 14, 15}},
	},
	{
		name:   "groups with negative values",
		groups: map[string][]float64{"A": {-3, -1, 1}, "B": {-2, 0, 2}, "C": {0, 2, 4}},
	},
}

// TestMath_Decomposition verifies SS_T = SS_B + SS_W for every fixture.
func TestMath_Decomposition(t *testing.T) {
	for _, fx := range mathFixtures {
		t.Run("SS_T MUST equal SS_B plus SS_W: "+fx.name, func(t *testing.T) {
			r, err := OneWay(fx.groups)
			require.NoError(t, err)
			assertMathInvariants(t, r)
		})
	}
}

// TestMath_NonNegativity verifies SS_B ≥ 0 and SS_W ≥ 0 for every fixture.
func TestMath_NonNegativity(t *testing.T) {
	for _, fx := range mathFixtures {
		t.Run("SS_B and SS_W MUST be non-negative: "+fx.name, func(t *testing.T) {
			r, err := OneWay(fx.groups)
			require.NoError(t, err)
			assertMathInvariants(t, r)
		})
	}
}

// TestMath_FIdentity verifies F = MS_B / MS_W for every fixture.
func TestMath_FIdentity(t *testing.T) {
	for _, fx := range mathFixtures {
		t.Run("F MUST equal MS_B divided by MS_W: "+fx.name, func(t *testing.T) {
			r, err := OneWay(fx.groups)
			require.NoError(t, err)
			assertMathInvariants(t, r)
		})
	}
}

// TestMath_TranslationInvariance verifies that adding a finite constant to
// every observation leaves SS values and F unchanged. TST-013/TST-031.
func TestMath_TranslationInvariance(t *testing.T) {
	offsets := []struct {
		name   string
		offset float64
	}{
		{"offset +1e12", +1e12},
		{"offset -1e12", -1e12},
		{"offset +1e6", +1e6},
	}

	for _, fx := range mathFixtures {
		for _, off := range offsets {
			name := fx.name + " — " + off.name
			t.Run("SS and F MUST be invariant under translation: "+name, func(t *testing.T) {
				ref, err := OneWay(fx.groups)
				require.NoError(t, err)

				shifted := make(map[string][]float64, len(fx.groups))
				for g, xs := range fx.groups {
					s := make([]float64, len(xs))
					for i, x := range xs {
						s[i] = off.offset + x
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
}

// TestMath_BatchStreamEquivalence verifies that sequential Add and batch
// OneWay produce identical ANOVA tables. TST-012.
func TestMath_BatchStreamEquivalence(t *testing.T) {
	for _, fx := range mathFixtures {
		t.Run("batch and sequential Add MUST produce equivalent results: "+fx.name, func(t *testing.T) {
			batch, err := OneWay(fx.groups)
			require.NoError(t, err)

			labels := make([]string, 0, len(fx.groups))
			for g := range fx.groups {
				labels = append(labels, g)
			}
			stream := newAcc(t, labels...)
			for g, xs := range fx.groups {
				obs(t, stream, g, xs...)
			}
			snap := stream.Snapshot()

			assert.Equal(t, batch.Status, snap.Status)
			assert.InDelta(t, batch.BetweenSS, snap.BetweenSS, tol, "BetweenSS")
			assert.InDelta(t, batch.WithinSS, snap.WithinSS, tol, "WithinSS")
			assert.InDelta(t, batch.F, snap.F, tol, "F")
			assert.Equal(t, batch.Observations, snap.Observations)
		})
	}
}

// TestMath_PartitionEquivalence verifies that merging two disjoint partitions
// produces the same result as adding all observations in sequence. TST-003/TST-011.
func TestMath_PartitionEquivalence(t *testing.T) {
	for _, fx := range mathFixtures {
		t.Run("partition Merge MUST produce results equivalent to sequential Add: "+fx.name, func(t *testing.T) {
			ref, err := OneWay(fx.groups)
			require.NoError(t, err)

			labels := make([]string, 0, len(fx.groups))
			for g := range fx.groups {
				labels = append(labels, g)
			}

			left := newAcc(t, labels...)
			right := newAcc(t, labels...)

			// Split each group: even indices → left, odd → right.
			for g, xs := range fx.groups {
				for i, x := range xs {
					if i%2 == 0 {
						require.NoError(t, left.Add(g, x))
					} else {
						require.NoError(t, right.Add(g, x))
					}
				}
			}
			require.NoError(t, left.Merge(right))
			merged := left.Snapshot()

			assert.Equal(t, ref.Status, merged.Status)
			assert.InDelta(t, ref.BetweenSS, merged.BetweenSS, tol, "BetweenSS")
			assert.InDelta(t, ref.WithinSS, merged.WithinSS, tol, "WithinSS")
			assert.InDelta(t, ref.F, merged.F, tol, "F")
			assert.Equal(t, ref.Observations, merged.Observations)
		})
	}
}

// TestMath_PermutationInvariance verifies that reordering observations within
// each group does not change SS values or F beyond floating-point tolerance.
// TST-014.
func TestMath_PermutationInvariance(t *testing.T) {
	for _, fx := range mathFixtures {
		t.Run("within-group permutation MUST leave SS and F unchanged: "+fx.name, func(t *testing.T) {
			ref, err := OneWay(fx.groups)
			require.NoError(t, err)
			if ref.Status != StatusOK {
				t.Skip("fixture does not produce StatusOK — skipping permutation check")
			}

			// Reverse every group's slice as a deterministic permutation.
			reversed := make(map[string][]float64, len(fx.groups))
			for g, xs := range fx.groups {
				cp := make([]float64, len(xs))
				for i, x := range xs {
					cp[len(xs)-1-i] = x
				}
				reversed[g] = cp
			}

			got, err := OneWay(reversed)
			require.NoError(t, err)
			assert.Equal(t, ref.Status, got.Status)
			assert.InDelta(t, ref.BetweenSS, got.BetweenSS, tol, "BetweenSS")
			assert.InDelta(t, ref.WithinSS, got.WithinSS, tol, "WithinSS")
			assert.InDelta(t, ref.F, got.F, tol, "F")
		})
	}
}

// TestMath_LabelBijectionInvariance verifies that renaming groups does not
// change SS values, df, or F. TST-015.
func TestMath_LabelBijectionInvariance(t *testing.T) {
	for _, fx := range mathFixtures {
		t.Run("label rename MUST leave table values unchanged: "+fx.name, func(t *testing.T) {
			ref, err := OneWay(fx.groups)
			require.NoError(t, err)
			if ref.Status != StatusOK {
				t.Skip("fixture does not produce StatusOK — skipping bijection check")
			}

			// Map each original label to a synthetic name "grp_N".
			renamed := make(map[string][]float64, len(fx.groups))
			i := 0
			for g, xs := range fx.groups {
				_ = g
				renamed[strconv.Itoa(i)] = xs
				i++
			}

			got, err := OneWay(renamed)
			require.NoError(t, err)
			assert.Equal(t, ref.Status, got.Status)
			assert.InDelta(t, ref.BetweenSS, got.BetweenSS, tol, "BetweenSS")
			assert.InDelta(t, ref.WithinSS, got.WithinSS, tol, "WithinSS")
			assert.Equal(t, ref.BetweenDF, got.BetweenDF, "BetweenDF")
			assert.Equal(t, ref.WithinDF, got.WithinDF, "WithinDF")
			assert.InDelta(t, ref.F, got.F, tol, "F")
		})
	}
}

// ── Numerical stress tests TST-036–040 ───────────────────────────────────────
//
// These fixtures exercise adversarial data families from wiki/04 §Adversarial.
// They use withinTol (wiki/04 §Tolerance) rather than the fixed tol constant
// because SS values are extreme-scale or the offset-to-spread ratio is large.
//
// relTol=1e-12 corresponds to ~4 ULPs at any scale for Chan-merge arithmetic.
// absTol=1e-20 catches the near-zero case without masking real errors.

const (
	stressAbsTol = 1e-20
	stressRelTol = 1e-12
)

// TestStress_HighlyUnbalanced [TST-036] checks that a 1:20 group size ratio
// produces a valid ANOVA table with correct decomposition and non-negative SS.
func TestStress_HighlyUnbalanced(t *testing.T) {
	groups := map[string][]float64{
		"A": {100},
		"B": {1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20},
	}
	r, err := OneWay(groups)
	require.NoError(t, err)
	require.Equal(t, StatusOK, r.Status)
	assert.True(t, r.BetweenSS >= 0, "BetweenSS must be non-negative")
	assert.True(t, r.WithinSS >= 0, "WithinSS must be non-negative")
	assert.True(t, withinTol(r.TotalSS, r.BetweenSS+r.WithinSS, stressAbsTol, stressRelTol),
		"SS decomposition: TotalSS=%g BetweenSS+WithinSS=%g", r.TotalSS, r.BetweenSS+r.WithinSS)
}

// TestStress_SingletonMixed [TST-037] checks two singleton groups mixed with
// a larger group. Verifies decomposition; also verifies translation invariance
// at a +1e6 offset (a moderate offset well within Chan merge accuracy).
func TestStress_SingletonMixed(t *testing.T) {
	base := map[string][]float64{
		"A": {42},
		"B": {1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
		"C": {20},
	}
	ref, err := OneWay(base)
	require.NoError(t, err)
	require.Equal(t, StatusOK, ref.Status)

	assert.True(t, ref.BetweenSS >= 0, "BetweenSS must be non-negative")
	assert.True(t, ref.WithinSS >= 0, "WithinSS must be non-negative")
	assert.True(t, withinTol(ref.TotalSS, ref.BetweenSS+ref.WithinSS, stressAbsTol, stressRelTol),
		"SS decomposition")

	// Translation invariance at +1e6 (moderate offset; well within Chan accuracy).
	const offset = 1e6
	shifted := make(map[string][]float64, len(base))
	for g, xs := range base {
		cp := make([]float64, len(xs))
		for i, x := range xs {
			cp[i] = x + offset
		}
		shifted[g] = cp
	}
	got, err := OneWay(shifted)
	require.NoError(t, err)
	require.Equal(t, StatusOK, got.Status)
	assertWithinTol(t, ref.BetweenSS, got.BetweenSS, stressAbsTol, stressRelTol, "BetweenSS under +1e6 translation")
	assertWithinTol(t, ref.WithinSS, got.WithinSS, stressAbsTol, stressRelTol, "WithinSS under +1e6 translation")
	assertWithinTol(t, ref.F, got.F, stressAbsTol, stressRelTol, "F under +1e6 translation")
}

// TestStress_TinyScale [TST-038] checks that values near the floor of float64
// (~1e-300) either produce a valid table or return a documented degenerate
// status — and never panic or return StatusNumericalFailure silently.
func TestStress_TinyScale(t *testing.T) {
	groups := map[string][]float64{
		"A": {1e-300, 2e-300, 3e-300},
		"B": {4e-300, 5e-300, 6e-300},
	}
	r, err := OneWay(groups)
	require.NoError(t, err)
	// Values may underflow to produce zero SS — InfiniteF or IndeterminateZeroOverZero
	// are both acceptable degenerate statuses. NumericalFailure is not acceptable
	// unless the result is genuinely un-representable.
	acceptable := r.Status == StatusOK ||
		r.Status == StatusInfiniteF ||
		r.Status == StatusIndeterminateZeroOverZero ||
		r.Status == StatusNumericalWarning
	assert.True(t, acceptable, "unexpected status for tiny-scale data: %v", r.Status)
	if r.Status == StatusOK || r.Status == StatusNumericalWarning {
		assert.True(t, r.BetweenSS >= 0, "BetweenSS must be non-negative")
		assert.True(t, r.WithinSS >= 0, "WithinSS must be non-negative")
	}
}

// TestStress_HugeScale [TST-039] checks that values near 1e+150 produce
// a valid table with correct decomposition within relative tolerance.
// SS values are O(1e301) so the fixed tol=1e-10 is meaningless; we use
// stressRelTol=1e-12 (~4 ULPs) per wiki/04.
func TestStress_HugeScale(t *testing.T) {
	groups := map[string][]float64{
		"A": {1e150, 2e150, 3e150},
		"B": {4e150, 5e150, 6e150},
	}
	r, err := OneWay(groups)
	require.NoError(t, err)
	require.Equal(t, StatusOK, r.Status)
	assert.True(t, r.BetweenSS >= 0, "BetweenSS must be non-negative")
	assert.True(t, r.WithinSS >= 0, "WithinSS must be non-negative")
	assert.True(t, withinTol(r.TotalSS, r.BetweenSS+r.WithinSS, stressAbsTol, stressRelTol),
		"SS decomposition: TotalSS=%g BetweenSS+WithinSS=%g", r.TotalSS, r.BetweenSS+r.WithinSS)

	// Partition equivalence: split each group in half and merge.
	labels := []string{"A", "B"}
	left := newAcc(t, labels...)
	right := newAcc(t, labels...)
	for g, xs := range groups {
		for i, x := range xs {
			if i%2 == 0 {
				require.NoError(t, left.Add(g, x))
			} else {
				require.NoError(t, right.Add(g, x))
			}
		}
	}
	require.NoError(t, left.Merge(right))
	merged := left.Snapshot()
	require.Equal(t, StatusOK, merged.Status)
	assertWithinTol(t, r.BetweenSS, merged.BetweenSS, stressAbsTol, stressRelTol, "BetweenSS partition equiv")
	assertWithinTol(t, r.WithinSS, merged.WithinSS, stressAbsTol, stressRelTol, "WithinSS partition equiv")
	assertWithinTol(t, r.F, merged.F, stressAbsTol, stressRelTol, "F partition equiv")
}

// TestStress_DeterministicOrder [TST-040] verifies that group label insertion
// order does not affect SS or F — canonical ordering in Snapshot must be stable.
func TestStress_DeterministicOrder(t *testing.T) {
	// Same data fed in two different map literal orders.
	order1 := map[string][]float64{"D": {7, 8}, "A": {1, 2}, "C": {5, 6}, "B": {3, 4}}
	order2 := map[string][]float64{"A": {1, 2}, "B": {3, 4}, "C": {5, 6}, "D": {7, 8}}

	r1, err := OneWay(order1)
	require.NoError(t, err)
	r2, err := OneWay(order2)
	require.NoError(t, err)

	require.Equal(t, r1.Status, r2.Status)
	assert.InDelta(t, r1.BetweenSS, r2.BetweenSS, tol, "BetweenSS must be order-independent")
	assert.InDelta(t, r1.WithinSS, r2.WithinSS, tol, "WithinSS must be order-independent")
	assert.InDelta(t, r1.F, r2.F, tol, "F must be order-independent")
	assert.Equal(t, r1.BetweenDF, r2.BetweenDF, "BetweenDF must be order-independent")
	assert.Equal(t, r1.WithinDF, r2.WithinDF, "WithinDF must be order-independent")
}

// ── Randomized merge-tree order (TST-034) ────────────────────────────────────

// TestMath_RandomMergeTreeOrder [TST-034] verifies that merging N disjoint
// shards in a randomly-permuted order gives results equivalent to a single
// sequential accumulator within the documented floating-point tolerance.
//
// The existing TST-011 (TestMath_PartitionEquivalence) and TST-039
// (TestStress_HugeScale) exercise fixed 2-way even/odd splits. This test
// adds N-ary trees (N ∈ {2, 3, 5, 8}) with randomly-permuted merge order,
// exposing floating-point non-commutativity across deeper merge trees.
//
// A fixed seed (42) keeps the test deterministic and CI-stable.
func TestMath_RandomMergeTreeOrder(t *testing.T) {
	rng := rand.New(rand.NewSource(42))

	shardCounts := []int{2, 3, 5, 8}

	for _, fx := range mathFixtures {
		for _, n := range shardCounts {
			n := n
			name := fmt.Sprintf("%s — %d shards", fx.name, n)
			t.Run(name, func(t *testing.T) {
				// Reference: single accumulator fed all observations.
				labels := make([]string, 0, len(fx.groups))
				for g := range fx.groups {
					labels = append(labels, g)
				}
				sort.Strings(labels)

				ref := newAcc(t, labels...)
				for _, g := range labels {
					obs(t, ref, g, fx.groups[g]...)
				}
				want := ref.Snapshot()
				if want.Status != StatusOK && want.Status != StatusNumericalWarning {
					t.Skipf("fixture %q produces non-OK status %v; skipping merge-tree check", fx.name, want.Status)
				}

				// Build n shards: observation i goes to shard i%n.
				shards := make([]*Accumulator, n)
				for i := range shards {
					shards[i] = newAcc(t, labels...)
				}
				for _, g := range labels {
					for i, x := range fx.groups[g] {
						require.NoError(t, shards[i%n].Add(g, x))
					}
				}

				// Merge shards in a random permutation into shards[0].
				perm := rng.Perm(n)
				base := shards[perm[0]]
				for _, idx := range perm[1:] {
					require.NoError(t, base.Merge(shards[idx]))
				}
				got := base.Snapshot()

				assertWithinTol(t, want.BetweenSS, got.BetweenSS, stressAbsTol, stressRelTol, "BetweenSS")
				assertWithinTol(t, want.WithinSS, got.WithinSS, stressAbsTol, stressRelTol, "WithinSS")
				assertWithinTol(t, want.F, got.F, stressAbsTol, stressRelTol, "F")
				assert.Equal(t, want.Observations, got.Observations, "observation count must be exact")
			})
		}
	}
}

// ── JSON serialization ────────────────────────────────────────────────────────

// TestJSON_GroupSummary verifies the wire format of GroupSummary.
// GroupSummary is the only state that must survive serialization; everything
// else in Result is derived and can be recomputed.
func TestJSON_GroupSummary(t *testing.T) {
	tests := []struct {
		name     string
		gs       GroupSummary
		wantJSON string
	}{
		{
			name:     "MUST serialize with keys group, count, mean, m2",
			gs:       GroupSummary{Group: "control", Count: 3, Mean: 2.0, M2: 2.0},
			wantJSON: `{"group":"control","count":3,"mean":2,"m2":2}`,
		},
		{
			name:     "MUST round-trip a group with zero count and zero moments",
			gs:       GroupSummary{Group: "empty", Count: 0, Mean: 0, M2: 0},
			wantJSON: `{"group":"empty","count":0,"mean":0,"m2":0}`,
		},
		{
			name:     "MUST round-trip a group with a negative mean",
			gs:       GroupSummary{Group: "neg", Count: 2, Mean: -1.5, M2: 0.5},
			wantJSON: `{"group":"neg","count":2,"mean":-1.5,"m2":0.5}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(tt.gs)
			require.NoError(t, err)
			assert.JSONEq(t, tt.wantJSON, string(b))

			var got GroupSummary
			require.NoError(t, json.Unmarshal(b, &got))
			assert.Equal(t, tt.gs, got)
		})
	}
}

// TestJSON_Status verifies that Status marshals as its string name, not its
// numeric value, and that every defined value round-trips cleanly.
func TestJSON_Status(t *testing.T) {
	tests := []struct {
		name     string
		status   Status
		wantJSON string
	}{
		{
			name:     `MUST serialize StatusOK as "ok"`,
			status:   StatusOK,
			wantJSON: `"ok"`,
		},
		{
			name:     `MUST serialize StatusInsufficientGroups as "insufficient_groups"`,
			status:   StatusInsufficientGroups,
			wantJSON: `"insufficient_groups"`,
		},
		{
			name:     `MUST serialize StatusInsufficientResidualDF as "insufficient_residual_df"`,
			status:   StatusInsufficientResidualDF,
			wantJSON: `"insufficient_residual_df"`,
		},
		{
			name:     `MUST serialize StatusIndeterminateZeroOverZero as "indeterminate_zero_over_zero"`,
			status:   StatusIndeterminateZeroOverZero,
			wantJSON: `"indeterminate_zero_over_zero"`,
		},
		{
			name:     `MUST serialize StatusInfiniteF as "infinite_f"`,
			status:   StatusInfiniteF,
			wantJSON: `"infinite_f"`,
		},
		{
			name:     `MUST serialize StatusNumericalWarning as "numerical_warning"`,
			status:   StatusNumericalWarning,
			wantJSON: `"numerical_warning"`,
		},
		{
			name:     `MUST serialize StatusNumericalFailure as "numerical_failure"`,
			status:   StatusNumericalFailure,
			wantJSON: `"numerical_failure"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(tt.status)
			require.NoError(t, err)
			assert.Equal(t, tt.wantJSON, string(b))

			var got Status
			require.NoError(t, json.Unmarshal(b, &got))
			assert.Equal(t, tt.status, got)
		})
	}

	t.Run("MUST return an error when unmarshaling an unrecognized status string", func(t *testing.T) {
		var s Status
		require.Error(t, json.Unmarshal([]byte(`"bogus"`), &s))
	})
}

// TestJSON_Groups_RoundTrip is the canonical demonstration of how to persist
// and restore accumulator state. Only Groups needs to be serialized; all ANOVA
// table fields are derived and need not be stored.
//
// Serialized shape for FX-001 (A=[1,2,3], B=[4,5,6], C=[7,8,9]):
//
//	[
//	  {"group":"A","count":3,"mean":2,"m2":2},
//	  {"group":"B","count":3,"mean":5,"m2":2},
//	  {"group":"C","count":3,"mean":8,"m2":2}
//	]
func TestJSON_Groups_RoundTrip(t *testing.T) {
	a := newAcc(t, "A", "B", "C")
	obs(t, a, "A", 1, 2, 3)
	obs(t, a, "B", 4, 5, 6)
	obs(t, a, "C", 7, 8, 9)
	snap := a.Snapshot()

	b, err := json.Marshal(snap.Groups)
	require.NoError(t, err)

	wantJSON := `[
		{"group":"A","count":3,"mean":2,"m2":2},
		{"group":"B","count":3,"mean":5,"m2":2},
		{"group":"C","count":3,"mean":8,"m2":2}
	]`
	assert.JSONEq(t, wantJSON, string(b))

	var loaded []GroupSummary
	require.NoError(t, json.Unmarshal(b, &loaded))
	assert.Equal(t, snap.Groups, loaded,
		"deserialized Groups MUST be identical to the originals")
}

// TestJSON_Result_NaNInfLimitation documents json.Marshal behavior for Result.
// StatusOK results now have a finite P value and marshal successfully.
// StatusInfiniteF results still fail to marshal because F is +Inf.
//
// To persist accumulator state, serialize Groups only (see TestJSON_Groups_RoundTrip).
func TestJSON_Result_NaNInfLimitation(t *testing.T) {
	t.Run("MUST marshal a StatusOK Result now that P is a finite value", func(t *testing.T) {
		r, err := OneWay(map[string][]float64{"A": {1, 2, 3}, "B": {4, 5, 6}})
		require.NoError(t, err)
		require.Equal(t, StatusOK, r.Status)
		assert.True(t, r.P >= 0 && r.P <= 1, "P MUST be in [0,1] for StatusOK")

		_, marshalErr := json.Marshal(r)
		require.NoError(t, marshalErr,
			"json.Marshal MUST succeed for a StatusOK Result with finite P")
	})

	t.Run("MUST fail to marshal a StatusInfiniteF Result because F is +Inf", func(t *testing.T) {
		r, err := OneWay(map[string][]float64{"A": {1, 1}, "B": {2, 2}})
		require.NoError(t, err)
		require.Equal(t, StatusInfiniteF, r.Status)
		assert.True(t, math.IsInf(r.F, 1), "F is +Inf when within-SS is zero")

		_, marshalErr := json.Marshal(r)
		require.Error(t, marshalErr,
			"json.Marshal MUST return an error when F is +Inf; marshal Groups instead")
	})
}

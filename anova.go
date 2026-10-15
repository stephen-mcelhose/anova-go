// Package anova implements numerically stable, unweighted one-way ANOVA.
//
// It supports a fixed universe of string-labelled groups, batch ingestion, and
// append-only streaming snapshots. The stream stores centered sufficient
// statistics (count, mean, M2) using Welford updates and Chan-style merges.
//
// The package deliberately does not implement Welch, factorial, repeated-
// measures, post-hoc, sliding-window, or sequential-valid ANOVA. Snapshot
// p-values are intentionally unavailable until a version-pinned F-distribution
// adapter is validated against independent R and SciPy reference fixtures.
package anova

import (
	"errors"
	"fmt"
	"math"
	"sort"
)

// MaxExactCount is the largest integer exactly representable by float64.
// Counts above this limit are rejected so merge weights remain exact.
const MaxExactCount uint64 = 1 << 53

var (
	ErrInvalidGroup       = errors.New("anova: group must be non-empty")
	ErrDuplicateGroup     = errors.New("anova: duplicate group")
	ErrUnknownGroup       = errors.New("anova: unknown group")
	ErrNonFiniteValue     = errors.New("anova: value must be finite")
	ErrCountLimit         = errors.New("anova: count exceeds MaxExactCount")
	ErrIncompatible       = errors.New("anova: incompatible accumulators")
	ErrSelfMerge          = errors.New("anova: cannot merge an accumulator with itself")
	ErrNumericalFailure   = errors.New("anova: numerical failure")
)

// Status describes whether a one-way ANOVA table is interpretable.
type Status uint8

const (
	StatusOK Status = iota
	StatusInsufficientGroups
	StatusInsufficientResidualDF
	StatusIndeterminateZeroOverZero
	StatusInfiniteF
	StatusNumericalWarning
	StatusNumericalFailure
)

func (s Status) String() string {
	switch s {
	case StatusOK:
		return "ok"
	case StatusInsufficientGroups:
		return "insufficient_groups"
	case StatusInsufficientResidualDF:
		return "insufficient_residual_df"
	case StatusIndeterminateZeroOverZero:
		return "indeterminate_zero_over_zero"
	case StatusInfiniteF:
		return "infinite_f"
	case StatusNumericalWarning:
		return "numerical_warning"
	case StatusNumericalFailure:
		return "numerical_failure"
	default:
		return "unknown"
	}
}

// GroupSummary is an immutable copy of a group's centered sufficient statistics.
type GroupSummary struct {
	Group string
	Count uint64
	Mean  float64
	M2    float64
}

// Result is an immutable ANOVA snapshot. P is unavailable (NaN) in this
// implementation pending validated F-tail support. Check Status before using
// F or other inferential fields.
type Result struct {
	Status Status

	Groups        []GroupSummary
	EmptyGroups   []string
	NonEmptyGroup uint64
	Observations  uint64

	BetweenSS float64
	WithinSS  float64
	TotalSS   float64
	BetweenDF uint64
	WithinDF  uint64
	TotalDF   uint64
	BetweenMS float64
	WithinMS  float64
	F         float64
	P         float64
}

// Valid reports whether the result has finite, non-degenerate F arithmetic.
func (r Result) Valid() bool { return r.Status == StatusOK || r.Status == StatusNumericalWarning }

type moments struct {
	n    uint64
	mean float64
	m2   float64
}

func (m moments) summary(group string) GroupSummary {
	return GroupSummary{Group: group, Count: m.n, Mean: m.mean, M2: m.m2}
}

func checkedCount(a, b uint64) (uint64, error) {
	if b > MaxExactCount-a {
		return 0, ErrCountLimit
	}
	return a + b, nil
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

func (m moments) add(x float64) (moments, error) {
	if !finite(x) {
		return moments{}, ErrNonFiniteValue
	}
	n, err := checkedCount(m.n, 1)
	if err != nil {
		return moments{}, err
	}
	if m.n == 0 {
		return moments{n: 1, mean: x}, nil
	}
	delta := x - m.mean
	mean := m.mean + delta/float64(n)
	m2 := m.m2 + delta*(x-mean)
	if !finite(delta) || !finite(mean) || !finite(m2) {
		return moments{}, ErrNumericalFailure
	}
	return moments{n: n, mean: mean, m2: m2}, nil
}

func mergeMoments(a, b moments) (moments, error) {
	if a.n == 0 {
		return b, nil
	}
	if b.n == 0 {
		return a, nil
	}
	n, err := checkedCount(a.n, b.n)
	if err != nil {
		return moments{}, err
	}
	delta := b.mean - a.mean
	weight := float64(b.n) / float64(n)
	mean := a.mean + delta*weight
	cross := delta * delta * (float64(a.n) * float64(b.n) / float64(n))
	m2 := a.m2 + b.m2 + cross
	if !finite(delta) || !finite(weight) || !finite(mean) || !finite(cross) || !finite(m2) {
		return moments{}, ErrNumericalFailure
	}
	return moments{n: n, mean: mean, m2: m2}, nil
}

// Accumulator stores fixed-group, append-only one-way ANOVA sufficient state.
// It is not safe for concurrent mutation or snapshotting without caller-owned
// synchronization. Merge inputs must summarize disjoint observation partitions.
type Accumulator struct {
	groups []string
	data   map[string]moments
}

// New constructs an accumulator for a fixed group universe. Input order is not
// semantically meaningful; groups are stored in sorted canonical order.
func New(groups ...string) (*Accumulator, error) {
	if len(groups) == 0 {
		return nil, ErrInvalidGroup
	}
	copied := append([]string(nil), groups...)
	sort.Strings(copied)
	data := make(map[string]moments, len(copied))
	for i, group := range copied {
		if group == "" {
			return nil, ErrInvalidGroup
		}
		if i > 0 && copied[i-1] == group {
			return nil, fmt.Errorf("%w: %q", ErrDuplicateGroup, group)
		}
		data[group] = moments{}
	}
	return &Accumulator{groups: copied, data: data}, nil
}

// Add atomically accepts one finite response into a registered group.
func (a *Accumulator) Add(group string, value float64) error {
	if a == nil {
		return errors.New("anova: nil accumulator")
	}
	current, ok := a.data[group]
	if !ok {
		return fmt.Errorf("%w: %q", ErrUnknownGroup, group)
	}
	next, err := current.add(value)
	if err != nil {
		return err
	}
	a.data[group] = next
	return nil
}

// Merge atomically unions another accumulator's disjoint observation partition.
// It rejects self-merge and incompatible group schemas. It cannot detect other
// overlapping partitions because observation identities are not retained.
func (a *Accumulator) Merge(other *Accumulator) error {
	if a == nil || other == nil {
		return errors.New("anova: nil accumulator")
	}
	if a == other {
		return ErrSelfMerge
	}
	if len(a.groups) != len(other.groups) {
		return ErrIncompatible
	}
	for i := range a.groups {
		if a.groups[i] != other.groups[i] {
			return ErrIncompatible
		}
	}
	next := make(map[string]moments, len(a.data))
	for _, group := range a.groups {
		merged, err := mergeMoments(a.data[group], other.data[group])
		if err != nil {
			return err
		}
		next[group] = merged
	}
	a.data = next
	return nil
}

func clampNonnegative(v, scale float64) (float64, bool, error) {
	if !finite(v) || !finite(scale) || scale < 0 {
		return 0, false, ErrNumericalFailure
	}
	if v >= 0 {
		return v, false, nil
	}
	epsilon := math.Nextafter(1, 2) - 1
	if scale > 0 && math.Abs(v) <= 32*epsilon*scale {
		return 0, true, nil
	}
	return 0, false, ErrNumericalFailure
}

// Snapshot computes the ANOVA table without mutating the accumulator.
func (a *Accumulator) Snapshot() Result {
	result := Result{Status: StatusNumericalFailure, P: math.NaN(), F: math.NaN(), BetweenMS: math.NaN(), WithinMS: math.NaN()}
	if a == nil {
		return result
	}
	result.Groups = make([]GroupSummary, 0, len(a.groups))
	result.EmptyGroups = make([]string, 0)
	var grand moments
	withinScale := 0.0
	for _, group := range a.groups {
		m := a.data[group]
		result.Groups = append(result.Groups, m.summary(group))
		if m.n == 0 {
			result.EmptyGroups = append(result.EmptyGroups, group)
			continue
		}
		result.NonEmptyGroup++
		var err error
		grand, err = mergeMoments(grand, moments{n: m.n, mean: m.mean})
		if err != nil {
			return result
		}
		result.Observations, err = checkedCount(result.Observations, m.n)
		if err != nil {
			return result
		}
		result.WithinSS += m.m2
		withinScale += math.Abs(m.m2)
		if !finite(result.WithinSS) || !finite(withinScale) {
			return result
		}
	}
	if result.NonEmptyGroup < 2 {
		result.Status = StatusInsufficientGroups
		return result
	}
	if result.Observations <= result.NonEmptyGroup {
		result.Status = StatusInsufficientResidualDF
		return result
	}
	result.BetweenDF = result.NonEmptyGroup - 1
	result.WithinDF = result.Observations - result.NonEmptyGroup
	result.TotalDF = result.Observations - 1
	between, warned, err := clampNonnegative(grand.m2, math.Abs(grand.m2))
	if err != nil {
		return result
	}
	within, warnedWithin, err := clampNonnegative(result.WithinSS, withinScale)
	if err != nil {
		return result
	}
	result.BetweenSS, result.WithinSS = between, within
	result.TotalSS = between + within
	if !finite(result.TotalSS) {
		return result
	}
	result.BetweenMS = between / float64(result.BetweenDF)
	result.WithinMS = within / float64(result.WithinDF)
	if !finite(result.BetweenMS) || !finite(result.WithinMS) {
		return result
	}
	if within == 0 {
		if between == 0 {
			result.Status = StatusIndeterminateZeroOverZero
			return result
		}
		result.F = math.Inf(1)
		result.Status = StatusInfiniteF
		return result
	}
	result.F = result.BetweenMS / result.WithinMS
	if !finite(result.F) || result.F < 0 {
		return result
	}
	if warned || warnedWithin {
		result.Status = StatusNumericalWarning
	} else {
		result.Status = StatusOK
	}
	return result
}

// OneWay computes a batch one-way ANOVA from fixed labelled groups. All map
// entries form the group universe, including empty slices. P is unavailable;
// see package documentation.
func OneWay(groups map[string][]float64) (Result, error) {
	if len(groups) == 0 {
		return Result{}, ErrInvalidGroup
	}
	labels := make([]string, 0, len(groups))
	for group := range groups {
		labels = append(labels, group)
	}
	acc, err := New(labels...)
	if err != nil {
		return Result{}, err
	}
	for _, group := range acc.groups {
		for _, value := range groups[group] {
			if err := acc.Add(group, value); err != nil {
				return Result{}, err
			}
		}
	}
	return acc.Snapshot(), nil
}

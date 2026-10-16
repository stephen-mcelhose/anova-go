// Package anova implements numerically stable, unweighted one-way ANOVA.
//
// It supports a fixed universe of string-labelled groups, batch ingestion, and
// append-only streaming snapshots. The stream stores centered sufficient
// statistics (count, mean, M2) using Welford updates and Chan-style merges.
//
// The package deliberately does not implement Welch, factorial, repeated-
// measures, post-hoc, sliding-window, or sequential-valid ANOVA.
//
// P-values for StatusOK snapshots are computed via the F-distribution survival
// function provided by gonum.org/v1/gonum/stat/distuv (v0.17.0), validated
// against independent R 4.5.0 reference fixtures (see references/oracles/).
// P is NaN for all non-OK statuses.
package anova

import (
	"errors"
	"fmt"
	"math"
	"sort"

	"github.com/stephen-mcelhose/anova-go/internal/ftail"
)

// MaxExactCount is the largest integer exactly representable by float64.
// Counts above this limit are rejected so merge weights remain exact.
const MaxExactCount uint64 = 1 << 53

var (
	ErrInvalidGroup     = errors.New("anova: group must be non-empty")
	ErrDuplicateGroup   = errors.New("anova: duplicate group")
	ErrUnknownGroup     = errors.New("anova: unknown group")
	ErrNonFiniteValue   = errors.New("anova: value must be finite")
	ErrCountLimit       = errors.New("anova: count exceeds MaxExactCount")
	ErrIncompatible     = errors.New("anova: incompatible accumulators")
	ErrSelfMerge        = errors.New("anova: cannot merge an accumulator with itself")
	ErrNumericalFailure = errors.New("anova: numerical failure")
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

// MarshalText implements encoding.TextMarshaler so Status serializes as its
// string name in JSON and other text formats.
func (s Status) MarshalText() ([]byte, error) {
	return []byte(s.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (s *Status) UnmarshalText(b []byte) error {
	switch string(b) {
	case "ok":
		*s = StatusOK
	case "insufficient_groups":
		*s = StatusInsufficientGroups
	case "insufficient_residual_df":
		*s = StatusInsufficientResidualDF
	case "indeterminate_zero_over_zero":
		*s = StatusIndeterminateZeroOverZero
	case "infinite_f":
		*s = StatusInfiniteF
	case "numerical_warning":
		*s = StatusNumericalWarning
	case "numerical_failure":
		*s = StatusNumericalFailure
	default:
		return fmt.Errorf("anova: unknown status %q", string(b))
	}
	return nil
}

// GroupSummary is an immutable copy of a group's centered sufficient statistics.
// It is the minimal state required to restore or continue an Accumulator.
type GroupSummary struct {
	Group string  `json:"group"`
	Count uint64  `json:"count"`
	Mean  float64 `json:"mean"`
	M2    float64 `json:"m2"`
}

// Result is an immutable ANOVA snapshot. P is computed for StatusOK via the
// F-distribution survival function and is NaN for all other statuses. Check
// Status before using F or other inferential fields.
//
// All table fields (BetweenSS through P) are derived from Groups; only Groups
// needs to be persisted to restore an Accumulator via Load.
type Result struct {
	Status Status `json:"status"`

	Groups        []GroupSummary `json:"groups"`
	EmptyGroups   []string       `json:"empty_groups"`
	NonEmptyGroup uint64         `json:"non_empty_groups"`
	Observations  uint64         `json:"observations"`

	BetweenSS float64 `json:"between_ss"`
	WithinSS  float64 `json:"within_ss"`
	TotalSS   float64 `json:"total_ss"`
	BetweenDF uint64  `json:"between_df"`
	WithinDF  uint64  `json:"within_df"`
	TotalDF   uint64  `json:"total_df"`
	BetweenMS float64 `json:"between_ms"`
	WithinMS  float64 `json:"within_ms"`
	F         float64 `json:"f"`
	P         float64 `json:"p"`
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
		result.P = ftail.PValue(result.F, result.BetweenDF, result.WithinDF)
	}
	return result
}

// Load creates an Accumulator pre-loaded with the given group state. It is
// the inverse of Snapshot().Groups and is the intended way to restore a
// persisted or synthetically constructed accumulator — including in tests that
// need to reach boundary conditions (e.g. MaxExactCount) without looping.
//
// Validation mirrors New: group names must be non-empty and unique. Each
// group's Count must not exceed MaxExactCount. Mean and M2 must be finite for
// any group with Count > 0; M2 must be non-negative. Groups with Count == 0
// are accepted and their Mean/M2 are zeroed.
//
// The returned Accumulator behaves identically to one that reached the same
// state through Add and Merge calls.
func Load(groups []GroupSummary) (*Accumulator, error) {
	if len(groups) == 0 {
		return nil, ErrInvalidGroup
	}
	labels := make([]string, len(groups))
	for i, g := range groups {
		labels[i] = g.Group
	}
	acc, err := New(labels...)
	if err != nil {
		return nil, err
	}
	for _, g := range groups {
		if g.Count > MaxExactCount {
			return nil, ErrCountLimit
		}
		if g.Count == 0 {
			// zero count: normalise to avoid inconsistent state
			acc.data[g.Group] = moments{}
			continue
		}
		if !finite(g.Mean) || !finite(g.M2) || g.M2 < 0 {
			return nil, ErrNumericalFailure
		}
		acc.data[g.Group] = moments{n: g.Count, mean: g.Mean, m2: g.M2}
	}
	return acc, nil
}

// OneWay computes a batch one-way ANOVA from fixed labelled groups. All map
// entries form the group universe, including empty slices. P is available for
// StatusOK results; see Result for details.
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

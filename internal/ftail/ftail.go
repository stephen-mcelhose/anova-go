// Package ftail provides the F-distribution upper-tail adapter used by the
// anova package to compute p-values. It wraps gonum.org/v1/gonum/stat/distuv
// and validates all parameters so callers never receive a panic.
//
// The adapter is intentionally internal: the exact Gonum version and API
// surface are an implementation detail pinned by ADR-005. External code must
// not import this package directly.
package ftail

import (
	"math"

	"gonum.org/v1/gonum/stat/distuv"
)

// PValue returns the upper-tail probability P(F_{dfBetween,dfWithin} ≥ f),
// i.e. the survival function 1 − CDF of the F distribution.
//
// Parameter validity rules:
//   - dfBetween == 0 or dfWithin == 0 → NaN (undefined distribution)
//   - f < 0 or f is NaN → NaN (out of support)
//   - f == +Inf → 0 (no probability mass above +∞)
//   - f == 0 → 1 (all mass is ≥ 0 for a non-degenerate F)
//   - finite f > 0 → Survival(f) via gonum distuv.F
//
// The function never panics. If the underlying Gonum call produces a
// non-finite result (implementation guard), it returns NaN.
func PValue(f float64, dfBetween, dfWithin uint64) float64 {
	if dfBetween == 0 || dfWithin == 0 {
		return math.NaN()
	}
	if math.IsNaN(f) || f < 0 {
		return math.NaN()
	}
	if math.IsInf(f, 1) {
		return 0
	}
	dist := distuv.F{D1: float64(dfBetween), D2: float64(dfWithin)}
	p := dist.Survival(f)
	// Guard against any unexpected non-finite result from the library.
	if math.IsNaN(p) || math.IsInf(p, 0) || p < 0 || p > 1 {
		return math.NaN()
	}
	return p
}

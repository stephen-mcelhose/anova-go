package anova_test

// One-way ANOVA assumptions (v1, fixed-effects, unweighted):
//
//   1. Independence — observations are independent within and across groups.
//   2. Normality — residuals are approximately normally distributed.
//   3. Homoscedasticity — population variances are equal across groups
//      (Bartlett's or Levene's test can check this; v1 does not).
//   4. Fixed effects — group labels are fixed, not a random sample of a
//      larger population.
//
// Violations of these assumptions affect the validity of the F statistic and
// the p-value. The library enforces none of them; that is the caller's
// responsibility. Always check Result.Status before using numeric fields.

import (
	"fmt"
	"log"

	anova "github.com/stephen-mcelhose/anova-go"
)

// ExampleOneWay demonstrates the simplest usage: supply a map of group labels
// to observations and read the ANOVA table from the returned Result.
//
// Always check Result.Status (or Result.Valid) before interpreting F or P.
// A status other than StatusOK means the arithmetic is degenerate or invalid.
func ExampleOneWay() {
	groups := map[string][]float64{
		"control":   {4.1, 3.8, 4.3, 4.0},
		"low_dose":  {5.2, 5.8, 5.5, 5.1},
		"high_dose": {7.9, 8.3, 7.6, 8.1},
	}

	r, err := anova.OneWay(groups)
	if err != nil {
		log.Fatal(err)
	}
	if r.Status != anova.StatusOK {
		log.Fatalf("degenerate result: %v", r.Status)
	}

	fmt.Printf("F(%.0f,%.0f) = %.4f  p = %.4f\n", float64(r.BetweenDF), float64(r.WithinDF), r.F, r.P)
	// Output:
	// F(2,9) = 205.2366  p = 0.0000
}

// ExampleOneWay_degenerate shows how to handle degenerate inputs where all
// observations are identical. The library returns an explicit Status rather
// than a fabricated p-value.
func ExampleOneWay_degenerate() {
	groups := map[string][]float64{
		"A": {5, 5, 5},
		"B": {5, 5, 5},
	}

	r, err := anova.OneWay(groups)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(r.Status)
	// Output:
	// indeterminate_zero_over_zero
}

// ExampleAccumulator demonstrates streaming ingestion: register groups up
// front, Add observations one at a time, then call Snapshot to read the
// current ANOVA table without resetting state.
//
// Snapshot is non-mutating; subsequent Add calls continue from where they
// left off.
func ExampleAccumulator() {
	a, err := anova.New("A", "B", "C")
	if err != nil {
		log.Fatal(err)
	}

	for _, v := range []float64{1, 2, 3} {
		if err := a.Add("A", v); err != nil {
			log.Fatal(err)
		}
	}
	for _, v := range []float64{4, 5, 6} {
		if err := a.Add("B", v); err != nil {
			log.Fatal(err)
		}
	}
	for _, v := range []float64{7, 8, 9} {
		if err := a.Add("C", v); err != nil {
			log.Fatal(err)
		}
	}

	r := a.Snapshot()
	if !r.Valid() {
		log.Fatalf("invalid result: %v", r.Status)
	}

	fmt.Printf("SS_B=%.1f  SS_W=%.1f  F=%.1f\n", r.BetweenSS, r.WithinSS, r.F)
	// Output:
	// SS_B=54.0  SS_W=6.0  F=27.0
}

// ExampleAccumulator_merge shows how two independently-built Accumulators over
// disjoint data partitions can be merged. The merged result is equivalent to
// having fed all observations to a single Accumulator.
//
// Merge assumes the two Accumulators were built from disjoint data. Merging
// overlapping partitions is undefined behavior; the library does not detect it.
func ExampleAccumulator_merge() {
	left, err := anova.New("A", "B")
	if err != nil {
		log.Fatal(err)
	}
	right, err := anova.New("A", "B")
	if err != nil {
		log.Fatal(err)
	}

	// First half of each group goes to left, second half to right.
	for _, v := range []float64{1, 2} {
		left.Add("A", v)   //nolint:errcheck
		left.Add("B", v*4) //nolint:errcheck
	}
	for _, v := range []float64{3, 4, 5} {
		right.Add("A", v)   //nolint:errcheck
		right.Add("B", v*4) //nolint:errcheck
	}

	if err := left.Merge(right); err != nil {
		log.Fatal(err)
	}

	r := left.Snapshot()
	if !r.Valid() {
		log.Fatalf("invalid result: %v", r.Status)
	}

	fmt.Printf("observations=%d  groups=%d\n", r.Observations, r.NonEmptyGroup)
	// Output:
	// observations=10  groups=2
}

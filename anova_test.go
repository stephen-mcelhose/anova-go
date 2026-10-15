package anova

import (
	"errors"
	"math"
	"reflect"
	"testing"
)

func closeEnough(a, b float64) bool {
	const rel = 1e-12
	return math.Abs(a-b) <= rel*math.Max(1, math.Max(math.Abs(a), math.Abs(b)))
}

func mustAccumulator(t *testing.T, groups ...string) *Accumulator {
	t.Helper()
	a, err := New(groups...)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func addAll(t *testing.T, a *Accumulator, values map[string][]float64) {
	t.Helper()
	for group, xs := range values {
		for _, x := range xs {
			if err := a.Add(group, x); err != nil {
				t.Fatalf("Add(%q, %v): %v", group, x, err)
			}
		}
	}
}

func TestFX001HandWorkedTable(t *testing.T) {
	// TST-004 / FX-001.
	r, err := OneWay(map[string][]float64{
		"C": {7, 8, 9}, "A": {1, 2, 3}, "B": {4, 5, 6},
	})
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != StatusOK {
		t.Fatalf("status = %v", r.Status)
	}
	for name, got, want := range map[string]struct{ got, want float64 }{
		"between SS": {r.BetweenSS, 54}, "within SS": {r.WithinSS, 6},
		"total SS": {r.TotalSS, 60}, "between MS": {r.BetweenMS, 27},
		"within MS": {r.WithinMS, 1}, "F": {r.F, 27},
	} {
		if !closeEnough(got, want) {
			t.Fatalf("%s = %.17g, want %.17g", name, got, want)
		}
	}
	if r.BetweenDF != 2 || r.WithinDF != 6 || r.TotalDF != 8 {
		t.Fatalf("unexpected dfs: %#v", r)
	}
	if !math.IsNaN(r.P) {
		t.Fatal("P must remain unavailable before validated F-tail adapter")
	}
	gotLabels := []string{r.Groups[0].Group, r.Groups[1].Group, r.Groups[2].Group}
	if !reflect.DeepEqual(gotLabels, []string{"A", "B", "C"}) {
		t.Fatalf("canonical group order = %v", gotLabels)
	}
}

func TestBatchStreamAndMergeEquivalence(t *testing.T) {
	// TST-011 / TST-012 / TST-003.
	all := map[string][]float64{"A": {1, 2, 3, 4}, "B": {10, 12, 11}, "C": {-3, -1, 0}}
	batch, err := OneWay(all)
	if err != nil { t.Fatal(err) }
	left, right := mustAccumulator(t, "A", "B", "C"), mustAccumulator(t, "A", "B", "C")
	addAll(t, left, map[string][]float64{"A": {1, 3}, "B": {10, 11}, "C": {-3, 0}})
	addAll(t, right, map[string][]float64{"A": {2, 4}, "B": {12}, "C": {-1}})
	if err := left.Merge(right); err != nil { t.Fatal(err) }
	stream := left.Snapshot()
	for name, got, want := range map[string]struct{got,want float64}{
		"between": {stream.BetweenSS, batch.BetweenSS}, "within": {stream.WithinSS, batch.WithinSS}, "F": {stream.F, batch.F},
	} {
		if !closeEnough(got, want) { t.Fatalf("%s: got %.17g want %.17g", name, got, want) }
	}
	if stream.Observations != batch.Observations || stream.WithinDF != batch.WithinDF { t.Fatal("count/df mismatch") }
}

func TestDegenerateStatuses(t *testing.T) {
	// TST-005 / TST-006.
	for name, groups := range map[string]map[string][]float64{
		"insufficient groups": {"A": {1, 2}, "B": {}},
		"insufficient residual df": {"A": {1}, "B": {2}},
		"zero over zero": {"A": {1, 1}, "B": {1, 1}},
		"infinite F": {"A": {1, 1}, "B": {2, 2}},
	} {
		r, err := OneWay(groups)
		if err != nil { t.Fatalf("%s: %v", name, err) }
		switch name {
		case "insufficient groups": if r.Status != StatusInsufficientGroups { t.Fatalf("%s: %v", name, r.Status) }
		case "insufficient residual df": if r.Status != StatusInsufficientResidualDF { t.Fatalf("%s: %v", name, r.Status) }
		case "zero over zero": if r.Status != StatusIndeterminateZeroOverZero { t.Fatalf("%s: %v", name, r.Status) }
		case "infinite F": if r.Status != StatusInfiniteF || !math.IsInf(r.F, 1) { t.Fatalf("%s: %#v", name, r) }
		}
	}
}

func TestAddAndMergeFailuresAreAtomic(t *testing.T) {
	// TST-007 / TST-008 / TST-019 / TST-041 / TST-042.
	a := mustAccumulator(t, "A", "B")
	if err := a.Add("A", 1); err != nil { t.Fatal(err) }
	before := a.Snapshot()
	if err := a.Add("A", math.NaN()); !errors.Is(err, ErrNonFiniteValue) { t.Fatalf("got %v", err) }
	if got := a.Snapshot(); !reflect.DeepEqual(got, before) { t.Fatal("failed Add mutated accumulator") }
	if err := a.Add("missing", 1); !errors.Is(err, ErrUnknownGroup) { t.Fatalf("got %v", err) }
	if got := a.Snapshot(); !reflect.DeepEqual(got, before) { t.Fatal("unknown group mutated accumulator") }
	if err := a.Merge(a); !errors.Is(err, ErrSelfMerge) { t.Fatalf("got %v", err) }
	if got := a.Snapshot(); !reflect.DeepEqual(got, before) { t.Fatal("self Merge mutated accumulator") }
	incompatible := mustAccumulator(t, "A", "C")
	if err := a.Merge(incompatible); !errors.Is(err, ErrIncompatible) { t.Fatalf("got %v", err) }
	if got := a.Snapshot(); !reflect.DeepEqual(got, before) { t.Fatal("incompatible Merge mutated accumulator") }
}

func TestSnapshotIsPureAndResultsDoNotAlias(t *testing.T) {
	// TST-009 / TST-018.
	a := mustAccumulator(t, "B", "A")
	addAll(t, a, map[string][]float64{"A": {1, 2}, "B": {3, 4}})
	one, two := a.Snapshot(), a.Snapshot()
	if one.Status != two.Status || one.Observations != two.Observations || !closeEnough(one.BetweenSS, two.BetweenSS) || !closeEnough(one.WithinSS, two.WithinSS) || !reflect.DeepEqual(one.Groups, two.Groups) {
		t.Fatal("snapshots differ without add")
	}
	one.Groups[0].Mean = 999
	one.EmptyGroups = append(one.EmptyGroups, "mutated")
	three := a.Snapshot()
	if three.Groups[0].Mean == 999 || len(three.EmptyGroups) != len(two.EmptyGroups) { t.Fatal("result aliases accumulator") }
}

func TestLargeOffsetAndTranslationInvariance(t *testing.T) {
	// TST-013 / TST-031.
	base := map[string][]float64{"A": {1, 2, 3, 4, 5}, "B": {2, 3, 4, 5, 6}, "C": {-1, 0, 1, 2, 3}}
	shifted := map[string][]float64{}
	for g, xs := range base {
		for _, x := range xs { shifted[g] = append(shifted[g], 1e12+x) }
	}
	a, err := OneWay(base); if err != nil { t.Fatal(err) }
	b, err := OneWay(shifted); if err != nil { t.Fatal(err) }
	if !closeEnough(a.BetweenSS, b.BetweenSS) || !closeEnough(a.WithinSS, b.WithinSS) || !closeEnough(a.F, b.F) {
		t.Fatalf("translation changed result: %#v %#v", a, b)
	}
}

func TestNonFiniteIntermediateRejectedAtomically(t *testing.T) {
	// TST-032.
	a := mustAccumulator(t, "A", "B")
	if err := a.Add("A", math.MaxFloat64); err != nil { t.Fatal(err) }
	before := a.Snapshot()
	if err := a.Add("A", -math.MaxFloat64); !errors.Is(err, ErrNumericalFailure) { t.Fatalf("got %v", err) }
	if got := a.Snapshot(); !reflect.DeepEqual(got, before) { t.Fatal("failed extreme Add mutated accumulator") }
}

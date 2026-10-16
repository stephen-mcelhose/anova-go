package anova

// Benchmarks [TST-051/052] establish the O(1) Add and O(k) Snapshot/Merge
// operational profile. No regression threshold is set in Phase 2; a stable
// baseline will be captured and gated in Phase 3 (ST-302).

import "testing"

// BenchmarkAdd [TST-051] measures the per-observation cost of Add.
// Expected: O(1), dominated by Welford arithmetic (~10–20 ns/op on modern hw).
func BenchmarkAdd(b *testing.B) {
	a, err := New("A", "B", "C", "D", "E", "F", "G", "H", "I", "J")
	if err != nil {
		b.Fatal(err)
	}
	groups := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = a.Add(groups[i%len(groups)], float64(i))
	}
}

// BenchmarkSnapshot [TST-051] measures the O(k) snapshot cost with k=10 groups.
// The accumulator is pre-loaded to avoid measuring the Add path.
func BenchmarkSnapshot(b *testing.B) {
	a, err := New("A", "B", "C", "D", "E", "F", "G", "H", "I", "J")
	if err != nil {
		b.Fatal(err)
	}
	groups := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J"}
	for i := 0; i < 1000; i++ {
		if err := a.Add(groups[i%len(groups)], float64(i)); err != nil {
			b.Fatal(err)
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = a.Snapshot()
	}
}

// BenchmarkMerge [TST-052] measures the O(k) merge cost with k=10 groups.
// Both sides are pre-built and snapshotted; each iteration restores a fresh
// base via Load so only the Merge call is timed.
func BenchmarkMerge(b *testing.B) {
	groups := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J"}
	build := func(start, count int) *Accumulator {
		a, err := New(groups...)
		if err != nil {
			b.Fatal(err)
		}
		for i := start; i < start+count; i++ {
			if err := a.Add(groups[i%len(groups)], float64(i)); err != nil {
				b.Fatal(err)
			}
		}
		return a
	}

	// Snapshot both sides once; restore base via Load each iteration so only
	// the Merge call is included in the measured time.
	baseSnap := build(0, 500).Snapshot()
	other := build(1000, 500)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		base, err := Load(baseSnap.Groups)
		if err != nil {
			b.Fatal(err)
		}
		if err := base.Merge(other); err != nil {
			b.Fatal(err)
		}
	}
}

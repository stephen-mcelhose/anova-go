package anova

import (
	"math"
	"testing"
)

// FuzzAdd [TST-053] exercises Add with arbitrary byte-encoded (group, value)
// sequences and verifies that the accumulator never panics, always rejects
// non-finite values atomically, and satisfies math invariants for valid results.
//
// Input encoding: each byte in groupBytes selects a group by modulo; every 8
// consecutive bytes in valueBytes encode one float64 via math.Float64frombits.
// The number of observations is min(len(groupBytes), len(valueBytes)/8).
//
// Note: most random 64-bit patterns produce NaN or Inf (the exponent field
// 0x7FF covers ~0.4% of the float64 space as finite values). The fuzz target
// still earns its keep — it proves no panic on any input and confirms that
// rejection is always atomic — but math-invariant assertions will be reached
// rarely. Use the seed corpus entries for targeted arithmetic coverage.
func FuzzAdd(f *testing.F) {
	// Seed: FX-001 balanced A=[1,2,3] B=[4,5,6] C=[7,8,9].
	f.Add(
		[]byte{0, 0, 0, 1, 1, 1, 2, 2, 2},
		[]byte{
			0, 0, 0, 0, 0, 0, 0xf0, 0x3f, // 1.0
			0, 0, 0, 0, 0, 0, 0x00, 0x40, // 2.0
			0, 0, 0, 0, 0, 0, 0x08, 0x40, // 3.0
			0, 0, 0, 0, 0, 0, 0x10, 0x40, // 4.0
			0, 0, 0, 0, 0, 0, 0x14, 0x40, // 5.0
			0, 0, 0, 0, 0, 0, 0x18, 0x40, // 6.0
			0, 0, 0, 0, 0, 0, 0x1c, 0x40, // 7.0
			0, 0, 0, 0, 0, 0, 0x20, 0x40, // 8.0
			0, 0, 0, 0, 0, 0, 0x22, 0x40, // 9.0
		},
	)
	// Seed: single observation.
	f.Add([]byte{0}, []byte{0, 0, 0, 0, 0, 0, 0xf0, 0x3f}) // 1.0
	// Seed: empty — nothing to add.
	f.Add([]byte{}, []byte{})
	// Seed: large-offset adversarial (1e12 + small spread).
	f.Add(
		[]byte{0, 0, 1, 1},
		[]byte{
			0x00, 0x40, 0x88, 0xb4, 0xe8, 0x74, 0x42, 0x43, // 1e12+1
			0x00, 0x80, 0x88, 0xb4, 0xe8, 0x74, 0x42, 0x43, // 1e12+2
			0x00, 0x40, 0x88, 0xb4, 0xe8, 0x74, 0x42, 0x43, // 1e12+1
			0x00, 0x80, 0x88, 0xb4, 0xe8, 0x74, 0x42, 0x43, // 1e12+2
		},
	)

	f.Fuzz(func(t *testing.T, groupBytes []byte, valueBytes []byte) {
		const k = 3
		groupNames := [k]string{"A", "B", "C"}

		nObs := len(groupBytes)
		if len(valueBytes)/8 < nObs {
			nObs = len(valueBytes) / 8
		}
		if nObs == 0 {
			return
		}

		a, err := New(groupNames[:]...)
		if err != nil {
			t.Fatal(err)
		}

		for i := 0; i < nObs; i++ {
			g := groupNames[int(groupBytes[i])%k]
			bits := uint64(valueBytes[i*8]) |
				uint64(valueBytes[i*8+1])<<8 |
				uint64(valueBytes[i*8+2])<<16 |
				uint64(valueBytes[i*8+3])<<24 |
				uint64(valueBytes[i*8+4])<<32 |
				uint64(valueBytes[i*8+5])<<40 |
				uint64(valueBytes[i*8+6])<<48 |
				uint64(valueBytes[i*8+7])<<56
			v := math.Float64frombits(bits)

			stateBefore := a.Snapshot()
			err := a.Add(g, v)
			if !finite(v) {
				if err == nil {
					t.Errorf("non-finite value %v must be rejected", v)
				}
				// State must be unchanged after rejection.
				stateAfter := a.Snapshot()
				if stateBefore.Observations != stateAfter.Observations {
					t.Errorf("rejected Add must not mutate observation count: before=%d after=%d",
						stateBefore.Observations, stateAfter.Observations)
				}
			}
		}

		r := a.Snapshot()
		switch r.Status {
		case StatusOK, StatusNumericalWarning, StatusNumericalFailure,
			StatusInsufficientGroups, StatusInsufficientResidualDF,
			StatusIndeterminateZeroOverZero, StatusInfiniteF:
			// all documented statuses — ok
		default:
			t.Errorf("unknown status: %v", r.Status)
		}
		if r.Status == StatusOK {
			if r.BetweenSS < 0 {
				t.Errorf("BetweenSS must be non-negative for StatusOK: %v", r.BetweenSS)
			}
			if r.WithinSS < 0 {
				t.Errorf("WithinSS must be non-negative for StatusOK: %v", r.WithinSS)
			}
		}
	})
}

// FuzzMerge [TST-054] verifies structural partition-merge invariants under
// arbitrary two-way splits of arbitrary finite inputs.
//
// Input encoding: groupBytes and valueBytes are the same as FuzzAdd. splitMask
// determines which accumulator each observation goes to: bit i of splitMask
// routes observation i to accumulator 0 (bit=0) or accumulator 1 (bit=1).
//
// # What this target checks
//
// This target asserts structural correctness only:
//   - Merge MUST NOT panic or return a spurious error for valid inputs.
//   - Observation counts after merge MUST equal the sum of both partitions.
//   - BetweenSS and WithinSS MUST be non-negative after merge.
//   - Status MUST be a known, valid constant.
//
// # What this target does NOT check
//
// Numerical agreement between the merged result and a sequential-Welford
// reference MUST NOT be asserted in a fuzz target over the full float64 domain.
//
// Sequential Welford and the Chan parallel merge formula are two distinct
// float64 approximations to the true sum-of-squares. When observations share a
// large offset relative to their spread — which can occur at any magnitude, not
// only at large scale — catastrophic cancellation in the running-mean update
// causes the two paths to diverge significantly from each other and from the
// ground truth. Neither path returns an error in this regime; both are working
// correctly within the limits of float64 arithmetic.
//
// Precision equivalence (partition merge ≈ sequential reference within
// relTol=1e-12) is verified by hand-crafted fixtures in anova_test.go
// (TST-011, TST-036–040) that use controlled, well-conditioned data with
// independently verified oracle values. Those fixtures are the authoritative
// precision tests; this target is the adversarial structural test.
func FuzzMerge(f *testing.F) {
	// Seed: FX-001 split even/odd.
	f.Add(
		[]byte{0, 0, 0, 1, 1, 1, 2, 2, 2},
		[]byte{
			0, 0, 0, 0, 0, 0, 0xf0, 0x3f,
			0, 0, 0, 0, 0, 0, 0x00, 0x40,
			0, 0, 0, 0, 0, 0, 0x08, 0x40,
			0, 0, 0, 0, 0, 0, 0x10, 0x40,
			0, 0, 0, 0, 0, 0, 0x14, 0x40,
			0, 0, 0, 0, 0, 0, 0x18, 0x40,
			0, 0, 0, 0, 0, 0, 0x1c, 0x40,
			0, 0, 0, 0, 0, 0, 0x20, 0x40,
			0, 0, 0, 0, 0, 0, 0x22, 0x40,
		},
		[]byte{0b01010101, 0b00000001}, // alternating split
	)
	// Seed: single observation goes to acc0.
	f.Add([]byte{0}, []byte{0, 0, 0, 0, 0, 0, 0xf0, 0x3f}, []byte{0x00})
	// Seed: empty.
	f.Add([]byte{}, []byte{}, []byte{})

	f.Fuzz(func(t *testing.T, groupBytes []byte, valueBytes []byte, splitMask []byte) {
		const k = 3
		groupNames := [k]string{"A", "B", "C"}

		nObs := len(groupBytes)
		if len(valueBytes)/8 < nObs {
			nObs = len(valueBytes) / 8
		}
		if nObs == 0 {
			return
		}

		// Decode observations; skip non-finite values (both accumulators agree
		// to reject them, so counts stay in sync across the split).
		type obs struct {
			group string
			value float64
		}
		observations := make([]obs, 0, nObs)
		for i := 0; i < nObs; i++ {
			bits := uint64(valueBytes[i*8]) |
				uint64(valueBytes[i*8+1])<<8 |
				uint64(valueBytes[i*8+2])<<16 |
				uint64(valueBytes[i*8+3])<<24 |
				uint64(valueBytes[i*8+4])<<32 |
				uint64(valueBytes[i*8+5])<<40 |
				uint64(valueBytes[i*8+6])<<48 |
				uint64(valueBytes[i*8+7])<<56
			v := math.Float64frombits(bits)
			if !finite(v) {
				continue
			}
			observations = append(observations, obs{
				group: groupNames[int(groupBytes[i])%k],
				value: v,
			})
		}
		if len(observations) == 0 {
			return
		}

		// Split observations across two accumulators according to splitMask,
		// also tracking per-partition counts for the count-invariant check.
		acc0, err := New(groupNames[:]...)
		if err != nil {
			t.Fatal(err)
		}
		acc1, err := New(groupNames[:]...)
		if err != nil {
			t.Fatal(err)
		}
		var n0, n1 uint64
		for i, o := range observations {
			byteIdx, bitIdx := i/8, uint(i%8)
			var maskByte byte
			if byteIdx < len(splitMask) {
				maskByte = splitMask[byteIdx]
			}
			if (maskByte>>bitIdx)&1 == 0 {
				if err := acc0.Add(o.group, o.value); err != nil {
					return // count overflow: skip
				}
				n0++
			} else {
				if err := acc1.Add(o.group, o.value); err != nil {
					return // count overflow: skip
				}
				n1++
			}
		}

		// Merge MAY return ErrNumericalFailure when the Chan cross-term
		// overflows (e.g. means at extreme magnitude). This is documented
		// behaviour, not a bug. Any other error (incompatible universes, nil,
		// self-merge) would be a bug — but cannot occur here since both
		// accumulators were constructed with the same group universe and are
		// distinct objects.
		if err := acc0.Merge(acc1); err != nil {
			return // numerical overflow in merge: expected, not a bug
		}
		got := acc0.Snapshot()

		// Invariant 1: observation count MUST equal the sum of both partitions,
		// provided the grand-mean computation did not fail mid-loop.
		// When Status=numerical_failure, Snapshot MAY return a partial count
		// (the loop exits on the first non-finite grand-mean update), so the
		// count invariant MUST NOT be asserted in that case.
		if got.Status != StatusNumericalFailure && got.Observations != n0+n1 {
			t.Errorf("count invariant: got=%d want=%d (n0=%d n1=%d)",
				got.Observations, n0+n1, n0, n1)
		}

		// Invariant 2: SS values MUST be non-negative (or zero) after merge.
		if got.BetweenSS < 0 {
			t.Errorf("BetweenSS negative after merge: %v", got.BetweenSS)
		}
		if got.WithinSS < 0 {
			t.Errorf("WithinSS negative after merge: %v", got.WithinSS)
		}

		// Invariant 3: Status MUST be a known constant.
		switch got.Status {
		case StatusOK, StatusNumericalWarning, StatusNumericalFailure,
			StatusInsufficientGroups, StatusInsufficientResidualDF,
			StatusInfiniteF, StatusIndeterminateZeroOverZero:
			// valid
		default:
			t.Errorf("unknown status after merge: %v", got.Status)
		}
	})
}

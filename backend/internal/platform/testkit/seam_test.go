package testkit

import (
	"sync"
	"testing"
	"time"
)

var (
	addFn       = func(a, b int) int { return a + b }
	swapTargetI = 10
)

func TestSwap_FunctionAndRestore(t *testing.T) {
	// run swap in a subtest so Cleanup runs before we validate restoration
	t.Run("swap-in-subtest", func(t *testing.T) {
		orig := addFn(1, 2)
		if orig != 3 {
			t.Fatalf("precondition failed, addFn(1,2)=%d want 3", orig)
		}
		Swap(t, &addFn, func(a, b int) int { return 99 })
		if got := addFn(1, 2); got != 99 {
			t.Fatalf("swap did not take effect, got %d want 99", got)
		}
	})

	// after subtest completes, Cleanup restored the original
	if got := addFn(1, 2); got != 3 {
		t.Fatalf("swap did not restore original, got %d want 3", got)
	}
}

func TestSwap_NonFunctionType(t *testing.T) {
	t.Parallel()

	// swap an int and ensure it restores
	t.Run("int", func(t *testing.T) {
		if swapTargetI != 10 {
			t.Fatalf("precondition failed, got %d", swapTargetI)
		}
		Swap(t, &swapTargetI, 42)
		if swapTargetI != 42 {
			t.Fatalf("swap failed, got %d want 42", swapTargetI)
		}
	})
	if swapTargetI != 10 {
		t.Fatalf("swap did not restore original, got %d want 10", swapTargetI)
	}
}

func TestSerial_GuardsConcurrentSubtests(t *testing.T) {
	t.Parallel()

	var seqMu sync.Mutex
	seq := make([]string, 0, 4)

	record := func(s string) {
		seqMu.Lock()
		seq = append(seq, s)
		seqMu.Unlock()
	}

	t.Run("A", func(t *testing.T) {
		t.Parallel()
		Serial(t)
		record("A-start")
		time.Sleep(50 * time.Millisecond)
		record("A-end")
	})

	t.Run("B", func(t *testing.T) {
		t.Parallel()
		Serial(t)
		record("B-start")
		time.Sleep(50 * time.Millisecond)
		record("B-end")
	})

	t.Cleanup(func() {
		// ensure no interleaving across A and B
		// valid orders:
		// A-start, A-end, B-start, B-end
		// or
		// B-start, B-end, A-start, A-end
		seqMu.Lock()
		defer seqMu.Unlock()
		if len(seq) != 4 {
			t.Fatalf("unexpected sequence length %d, seq=%v", len(seq), seq)
		}
		// detect grouping
		aStart, aEnd, bStart, bEnd := -1, -1, -1, -1
		for i, s := range seq {
			switch s {
			case "A-start":
				aStart = i
			case "A-end":
				aEnd = i
			case "B-start":
				bStart = i
			case "B-end":
				bEnd = i
			}
		}
		// grouped if A-end before B-start or B-end before A-start
		groupedAFirst := aStart != -1 && aEnd != -1 && aStart < aEnd && aEnd < bStart
		groupedBFirst := bStart != -1 && bEnd != -1 && bStart < bEnd && bEnd < aStart
		if !(groupedAFirst || groupedBFirst) {
			t.Fatalf("expected grouped execution, got seq=%v", seq)
		}
	})
}

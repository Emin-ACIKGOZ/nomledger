// SPDX-License-Identifier: MIT

package core_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/Emin-ACIKGOZ/nomledger/pkg/core"
)

var memorySink core.Transaction

const (
	p95Rounds     = 3
	p95Iterations = 100
	p95Threshold  = 50 * time.Millisecond
	buildRuns     = 3
	buildTimeout  = 120 * time.Second
	buildLimit    = 60 * time.Second
)

func percentile95(samples []time.Duration) time.Duration {
	sorted := append([]time.Duration(nil), samples...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	rank := (95*len(sorted) + 99) / 100
	if rank < 1 {
		rank = 1
	}
	if rank > len(sorted) {
		rank = len(sorted)
	}
	return sorted[rank-1]
}

func medianDuration(values []time.Duration) time.Duration {
	sorted := append([]time.Duration(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	return sorted[len(sorted)/2]
}

// TestValidationP95: median of three round p95 values must be <= 50ms.
func TestValidationP95(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping performance test in short mode")
	}
	ledger := perfLedger(t)
	input := perfInput(t, 1000)

	p95Values := make([]time.Duration, 0, p95Rounds)
	for round := 1; round <= p95Rounds; round++ {
		samples := make([]time.Duration, 0, p95Iterations)
		for i := 0; i < p95Iterations; i++ {
			start := time.Now()
			if _, err := ledger.ValidateTransaction(context.Background(), input); err != nil {
				t.Fatalf("round %d iteration %d: %v", round, i, err)
			}
			samples = append(samples, time.Since(start))
		}
		p95 := percentile95(samples)
		p95Values = append(p95Values, p95)
		t.Logf("round %d p95 = %s", round, p95)
	}

	median := medianDuration(p95Values)
	t.Logf("p95 values: %v; median = %s; threshold = %s", p95Values, median, p95Threshold)
	if median > p95Threshold {
		t.Errorf("median p95 = %s; want <= %s", median, p95Threshold)
	}
}

// TestMemoryStability: heap allocation growth over 1,000 validations
// must remain within 10% of the warmed baseline.
func TestMemoryStability(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping memory test in short mode")
	}
	ledger := perfLedger(t)
	input := perfInput(t, 2)

	for i := 0; i < 100; i++ {
		transaction, err := ledger.ValidateTransaction(context.Background(), input)
		if err != nil {
			t.Fatalf("warmup %d: %v", i, err)
		}
		memorySink = transaction
	}

	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	for i := 0; i < 1000; i++ {
		transaction, err := ledger.ValidateTransaction(context.Background(), input)
		if err != nil {
			t.Fatalf("validation %d: %v", i, err)
		}
		memorySink = transaction
	}

	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	runtime.KeepAlive(memorySink)

	if after.HeapAlloc > before.HeapAlloc {
		growth := after.HeapAlloc - before.HeapAlloc
		limit := before.HeapAlloc / 10
		t.Logf("HeapAlloc before=%d after=%d growth=%d limit=%d",
			before.HeapAlloc, after.HeapAlloc, growth, limit)
		if growth > limit {
			t.Errorf("heap growth = %d bytes; want <= %d (10%% of baseline %d)",
				growth, limit, before.HeapAlloc)
		}
	} else {
		t.Logf("HeapAlloc before=%d after=%d; no heap growth", before.HeapAlloc, after.HeapAlloc)
	}
}

// TestBuildTime: median `go build ./...` time must be <= 60s.
func TestBuildTime(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping build-time test in short mode")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skipf("go toolchain unavailable: %v", err)
	}
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}

	durations := make([]time.Duration, 0, buildRuns)
	for run := 1; run <= buildRuns; run++ {
		ctx, cancel := context.WithTimeout(context.Background(), buildTimeout)
		cmd := exec.CommandContext(ctx, "go", "build", "./...")
		cmd.Dir = repoRoot
		cmd.Env = append(os.Environ(), "GOPROXY=off", "GOFLAGS=-mod=readonly")

		start := time.Now()
		output, err := cmd.CombinedOutput()
		elapsed := time.Since(start)
		cancel()
		if err != nil {
			t.Fatalf("build %d failed: %v\n%s", run, err, output)
		}
		durations = append(durations, elapsed)
		t.Logf("build %d = %s", run, elapsed)
	}

	median := medianDuration(durations)
	t.Logf("build durations: %v; median = %s; limit = %s", durations, median, buildLimit)
	if median > buildLimit {
		t.Errorf("median build time = %s; want <= %s", median, buildLimit)
	}
}

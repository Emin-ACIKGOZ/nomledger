// SPDX-License-Identifier: MIT

package core_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Emin-ACIKGOZ/nomledger/pkg/core"

	"github.com/shopspring/decimal"
)

func perfLedger(tb testing.TB) *core.Ledger {
	tb.Helper()
	config := core.LedgerConfig{
		FunctionalCurrency: "USD",
		RateTolerance:      decimal.Zero,
	}
	clock := core.ClockFunc(func() time.Time {
		return time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	})
	ledger, err := core.NewLedger(config, mockRateProvider{}, core.WithClock(clock))
	if err != nil {
		tb.Fatalf("NewLedger: %v", err)
	}
	return ledger
}

// perfInput builds an even-sized, same-currency USD transaction whose entries
// sum exactly to zero. A single entry cannot be valid under the zero-sum rule.
func perfInput(tb testing.TB, entryCount int) core.TransactionInput {
	tb.Helper()
	if entryCount%2 != 0 {
		tb.Fatalf("entry count %d must be even for a zero-sum transaction", entryCount)
	}
	entries := make([]core.Entry, 0, entryCount)
	for i := 0; i < entryCount/2; i++ {
		entry, err := core.NewEntry(
			fmt.Sprintf("debit-%d", i),
			decimal.NewFromInt(1),
			"USD",
			decimal.NewFromInt(1),
			"USD",
		)
		if err != nil {
			tb.Fatalf("NewEntry debit %d: %v", i, err)
		}
		entries = append(entries, entry)
	}
	for i := 0; i < entryCount/2; i++ {
		entry, err := core.NewEntry(
			fmt.Sprintf("credit-%d", i),
			decimal.NewFromInt(-1),
			"USD",
			decimal.NewFromInt(1),
			"USD",
		)
		if err != nil {
			tb.Fatalf("NewEntry credit %d: %v", i, err)
		}
		entries = append(entries, entry)
	}
	return core.TransactionInput{
		ID:            "benchmark-transaction",
		EffectiveDate: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC),
		Entries:       entries,
	}
}

// BenchmarkValidateTransaction measures the canonical validation path.
//
// The minimum valid transaction is two entries because one entry cannot satisfy
// zero-sum.
func BenchmarkValidateTransaction(b *testing.B) {
	ledger := perfLedger(b)
	cases := []struct {
		name  string
		count int
	}{
		{name: "small_2", count: 2},
		{name: "entries_100", count: 100},
		{name: "entries_1000", count: 1000},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			input := perfInput(b, tc.count)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := ledger.ValidateTransaction(context.Background(), input); err != nil {
					b.Fatalf("ValidateTransaction: %v", err)
				}
			}
		})
	}
}

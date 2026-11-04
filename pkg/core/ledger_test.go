// SPDX-License-Identifier: MIT

package core_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Emin-ACIKGOZ/nomledger/pkg/core"

	"github.com/shopspring/decimal"
)

func TestLedger_ConcurrencySafety(t *testing.T) {
	// Setup
	config := core.LedgerConfig{
		ClosedDate:         time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC),
		FunctionalCurrency: codeUSD,
	}
	rp := mockRateProvider{rates: map[string]decimal.Decimal{pairEURUSD: decimal.NewFromFloat(1.1)}}

	// Create the facade
	ledger, err := core.NewLedger(config, rp)
	if err != nil {
		t.Fatalf("NewLedger: %v", err)
	}

	// Simulate high-concurrency usage
	// 100 goroutines attempting to validate transactions simultaneously
	var wg sync.WaitGroup
	count := 100

	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Each goroutine calls ValidateTransaction
			// The Ledger struct ensures a new Builder is created internally for each call
			e1, err1 := core.NewEntry("acc1", decimal.NewFromInt(100), codeUSD, decimal.NewFromInt(1), codeUSD)
			e2, err2 := core.NewEntry("acc2", decimal.NewFromInt(-100), codeUSD, decimal.NewFromInt(1), codeUSD)
			if err1 != nil || err2 != nil {
				t.Errorf("Entry construction failed")
				return
			}

			_, err := ledger.ValidateTransaction(context.Background(), core.TransactionInput{
				ID:            "concurrent-tx",
				EffectiveDate: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
				Entries:       []core.Entry{e1, e2},
			})

			if err != nil {
				t.Errorf("Concurrent validation failed: %v", err)
			}
		}()
	}
	wg.Wait()
}

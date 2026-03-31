// SPDX-License-Identifier: MIT

package core_test

import (
	"sync"
	"testing"
	"time"

	"nomledger/pkg/core"

	"github.com/shopspring/decimal"
)

func TestLedger_ConcurrencySafety(t *testing.T) {
	// Setup
	config := core.LedgerConfig{
		ClosedDate:         time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC),
		FunctionalCurrency: "USD",
	}
	rp := mockRateProvider{rates: map[string]decimal.Decimal{"EUR/USD": decimal.NewFromFloat(1.1)}}

	// Create the facade
	ledger := core.NewLedger(config, rp)

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
			e1, err1 := core.NewEntry("acc1", decimal.NewFromInt(100), "USD", decimal.NewFromInt(1), "USD")
			e2, err2 := core.NewEntry("acc2", decimal.NewFromInt(-100), "USD", decimal.NewFromInt(1), "USD")
			if err1 != nil || err2 != nil {
				t.Errorf("Entry construction failed")
				return
			}

			_, err := ledger.ValidateTransaction(
				"concurrent-tx",
				time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
				false,
				e1,
				e2,
			)

			if err != nil {
				t.Errorf("Concurrent validation failed: %v", err)
			}
		}()
	}
	wg.Wait()
}

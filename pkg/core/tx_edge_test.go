// SPDX-License-Identifier: MIT

package core_test

import (
	"errors"
	"testing"
	"time"

	"nomledger/pkg/core"
	pkgerr "nomledger/pkg/errors"

	"github.com/shopspring/decimal"
)

// failingRateProvider simulates a broken upstream dependency (e.g., DB down).
type failingRateProvider struct{}

func (f failingRateProvider) GetRate(_, _ string) (decimal.Decimal, error) {
	return decimal.Zero, errors.New("upstream rate service unavailable")
}

func TestTxBuilder_PennySplit_RoundingViolation(t *testing.T) {
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	config := core.LedgerConfig{
		ClosedDate:         time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC),
		FunctionalCurrency: "USD",
		RateTolerance:      decimal.NewFromFloat(0.00001),
	}
	rp := mockRateProvider{
		rates: map[string]decimal.Decimal{
			"EUR/USD": decimal.NewFromFloat(1.1),
		},
	}

	// Scenario: Splitting $100.00 among 3 accounts.
	b := core.NewTxBuilder("tx-split", baseTime, config, rp)

	// Debit Source: 100.00
	eSource, err := core.NewEntry("source", decimal.NewFromFloat(100.00), "USD", decimal.NewFromInt(1), "USD")
	if err != nil {
		t.Fatal(err)
	}
	b.AddEntry(eSource)

	// Credit 1 & 2: -33.33
	eDest1, err := core.NewEntry("dest1", decimal.NewFromFloat(-33.33), "USD", decimal.NewFromInt(1), "USD")
	if err != nil {
		t.Fatal(err)
	}
	b.AddEntry(eDest1)
	eDest2, err := core.NewEntry("dest2", decimal.NewFromFloat(-33.33), "USD", decimal.NewFromInt(1), "USD")
	if err != nil {
		t.Fatal(err)
	}
	b.AddEntry(eDest2)

	// Case A: Naive Entry (-33.33) -> Sum is 0.01 (Violation)
	badEntry, err := core.NewEntry("dest3", decimal.NewFromFloat(-33.33), "USD", decimal.NewFromInt(1), "USD")
	if err != nil {
		t.Fatal(err)
	}
	b.AddEntry(badEntry)

	_, err = b.Build()
	if !errors.Is(err, pkgerr.ErrZeroSumViolation) {
		t.Errorf("Penny Split: Expected ErrZeroSumViolation for naive split, got %v", err)
	}
}

func TestTxBuilder_PennySplit_ManualAllocation(t *testing.T) {
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	config := core.LedgerConfig{
		ClosedDate:         time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC),
		FunctionalCurrency: "USD",
		RateTolerance:      decimal.NewFromFloat(0.00001),
	}
	rp := mockRateProvider{
		rates: map[string]decimal.Decimal{
			"EUR/USD": decimal.NewFromFloat(1.1),
		},
	}

	// Case B: Corrected Entry (-33.34) -> Sum is 0.00
	bCorrect := core.NewTxBuilder("tx-split-ok", baseTime, config, rp)
	eSourceC, err := core.NewEntry("source", decimal.NewFromFloat(100.00), "USD", decimal.NewFromInt(1), "USD")
	if err != nil {
		t.Fatal(err)
	}
	bCorrect.AddEntry(eSourceC)
	eDest1C, err := core.NewEntry("dest1", decimal.NewFromFloat(-33.33), "USD", decimal.NewFromInt(1), "USD")
	if err != nil {
		t.Fatal(err)
	}
	bCorrect.AddEntry(eDest1C)
	eDest2C, err := core.NewEntry("dest2", decimal.NewFromFloat(-33.33), "USD", decimal.NewFromInt(1), "USD")
	if err != nil {
		t.Fatal(err)
	}
	bCorrect.AddEntry(eDest2C)

	goodEntry, err := core.NewEntry("dest3", decimal.NewFromFloat(-33.34), "USD", decimal.NewFromInt(1), "USD")
	if err != nil {
		t.Fatal(err)
	}
	bCorrect.AddEntry(goodEntry)

	_, err = bCorrect.Build()
	if err != nil {
		t.Errorf("Penny Split: Expected success for manual penny allocation, got %v", err)
	}
}

func TestTxBuilder_EdgeCases_DependencyFailure(t *testing.T) {
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	config := core.LedgerConfig{
		ClosedDate:         time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC),
		FunctionalCurrency: "USD",
		RateTolerance:      decimal.NewFromFloat(0.00001),
	}

	t.Run("Propagation", func(t *testing.T) {
		// If the RateProvider fails, the Builder must fail nicely, not panic.
		failingRP := failingRateProvider{}
		b := core.NewTxBuilder("tx-fail", baseTime, config, failingRP)

		// Entry 1: Requires conversion (Will fail at Rate check)
		// 100 EUR * 1.1 = 110 USD
		e1, err := core.NewEntry("acc", decimal.NewFromInt(100), "EUR", decimal.NewFromFloat(1.1), "USD")
		if err != nil {
			t.Fatal(err)
		}
		b.AddEntry(e1)

		// Entry 2: Balancing Entry (USD)
		// We add -110 USD so the Zero-Sum check passes.
		// Since TxCurrency (USD) == FuncCurrency (USD), this skips the rate check,
		// isolating the failure to Entry 1.
		e2, err := core.NewEntry("acc-balance", decimal.NewFromInt(-110), "USD", decimal.NewFromInt(1), "USD")
		if err != nil {
			t.Fatal(err)
		}
		b.AddEntry(e2)

		_, err = b.Build()
		if err == nil {
			t.Fatal("Expected error from failing rate provider, got nil")
		}

		// We verify it returned the error from the RateProvider, not a validation error
		if err.Error() != "upstream rate service unavailable" {
			t.Errorf("Unexpected error message: %v", err)
		}
	})
}

func TestTxBuilder_EdgeCases_Tolerance(t *testing.T) {
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	config := core.LedgerConfig{
		ClosedDate:         time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC),
		FunctionalCurrency: "USD",
		RateTolerance:      decimal.NewFromFloat(0.00001),
	}
	rp := mockRateProvider{
		rates: map[string]decimal.Decimal{
			"EUR/USD": decimal.NewFromFloat(1.1),
		},
	}

	t.Run("Exact Boundary", func(t *testing.T) {
		b := core.NewTxBuilder("tx-boundary", baseTime, config, rp)

		// 1. Exact Boundary Pass (Diff 0.00001)
		ratePass := decimal.NewFromFloat(1.10001)
		ePass, err := core.NewEntry("acc1", decimal.NewFromInt(100), "EUR", ratePass, "USD")
		if err != nil {
			t.Fatal(err)
		}

		// 2. Just Over Boundary Fail (Diff 0.00002)
		rateFail := decimal.NewFromFloat(1.10002)
		eFail, err := core.NewEntry("acc2", decimal.NewFromInt(100), "EUR", rateFail, "USD")
		if err != nil {
			t.Fatal(err)
		}

		// Test Pass Case
		b.AddEntry(ePass)
		// Balancing entry: -100 EUR converted at SAME rate to avoid noise
		ePassOffset, err := core.NewEntry("acc1-offset", decimal.NewFromInt(-100), "EUR", ratePass, "USD")
		if err != nil {
			t.Fatal(err)
		}
		b.AddEntry(ePassOffset)

		_, err = b.Build()
		if err != nil {
			t.Errorf("Expected exact tolerance boundary (0.00001) to PASS, got %v", err)
		}

		// Test Fail Case
		bFail := core.NewTxBuilder("tx-boundary-fail", baseTime, config, rp)
		bFail.AddEntry(eFail)
		eFailOffset, err := core.NewEntry("acc2-offset", decimal.NewFromInt(-100), "EUR", rateFail, "USD")
		if err != nil {
			t.Fatal(err)
		}
		bFail.AddEntry(eFailOffset)

		_, err = bFail.Build()
		if !errors.Is(err, pkgerr.ErrRateDeviation) {
			t.Errorf("Expected > tolerance (0.00002) to FAIL, got %v", err)
		}
	})
}

func TestTxBuilder_EdgeCases_Timezone(t *testing.T) {
	config := core.LedgerConfig{
		ClosedDate:         time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC),
		FunctionalCurrency: "USD",
		RateTolerance:      decimal.NewFromFloat(0.00001),
	}
	rp := mockRateProvider{
		rates: map[string]decimal.Decimal{
			"EUR/USD": decimal.NewFromFloat(1.1),
		},
	}

	t.Run("Crossing Logic", func(t *testing.T) {
		locEST, _ := time.LoadLocation("America/New_York")

		// Case A: Just barely closed (Equal to ClosedDate)
		// 18:59:59 EST = 23:59:59 UTC
		dateClosed := time.Date(2023, 12, 31, 18, 59, 59, 0, locEST)

		bClosed := core.NewTxBuilder("tx-tz-closed", dateClosed, config, rp)
		eaClosed, err := core.NewEntry("a", decimal.NewFromInt(1), "USD", decimal.NewFromInt(1), "USD")
		if err != nil {
			t.Fatal(err)
		}
		bClosed.AddEntry(eaClosed)
		ebClosed, err := core.NewEntry("b", decimal.NewFromInt(-1), "USD", decimal.NewFromInt(1), "USD")
		if err != nil {
			t.Fatal(err)
		}
		bClosed.AddEntry(ebClosed)

		_, err = bClosed.Build()
		if !errors.Is(err, pkgerr.ErrPeriodClosed) {
			t.Errorf("Timezone logic: Expected 18:59:59 EST (23:59:59 UTC) to be CLOSED, got %v", err)
		}

		// Case B: Just barely open
		// 19:00:00 EST = 00:00:00 UTC (Jan 1)
		dateOpen := time.Date(2023, 12, 31, 19, 0, 0, 0, locEST)

		bOpen := core.NewTxBuilder("tx-tz-open", dateOpen, config, rp)
		eaOpen, err := core.NewEntry("a", decimal.NewFromInt(1), "USD", decimal.NewFromInt(1), "USD")
		if err != nil {
			t.Fatal(err)
		}
		bOpen.AddEntry(eaOpen)
		ebOpen, err := core.NewEntry("b", decimal.NewFromInt(-1), "USD", decimal.NewFromInt(1), "USD")
		if err != nil {
			t.Fatal(err)
		}
		bOpen.AddEntry(ebOpen)

		_, err = bOpen.Build()
		if err != nil {
			t.Errorf("Timezone logic: Expected 19:00:00 EST (00:00:00 UTC) to be OPEN, got %v", err)
		}
	})
}

func TestTxBuilder_EdgeCases_Empty(t *testing.T) {
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	config := core.LedgerConfig{
		ClosedDate:         time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC),
		FunctionalCurrency: "USD",
		RateTolerance:      decimal.NewFromFloat(0.00001),
	}
	rp := mockRateProvider{
		rates: map[string]decimal.Decimal{
			"EUR/USD": decimal.NewFromFloat(1.1),
		},
	}

	t.Run("Empty Transaction", func(t *testing.T) {
		b := core.NewTxBuilder("tx-empty", baseTime, config, rp)
		_, err := b.Build()
		if !errors.Is(err, pkgerr.ErrEmptyTransaction) {
			t.Errorf("Expected ErrEmptyTransaction, got %v", err)
		}
	})
}

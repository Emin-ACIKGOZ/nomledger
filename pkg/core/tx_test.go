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

// mockRateProvider helps us test Rate Integrity invariants deterministically.
type mockRateProvider struct {
	rates map[string]decimal.Decimal
}

func (m mockRateProvider) GetRate(base, quote string) (decimal.Decimal, error) {
	key := base + "/" + quote
	if val, ok := m.rates[key]; ok {
		return val, nil
	}
	return decimal.Zero, errors.New("rate not found")
}

func TestTxBuilder_ValidTransaction(t *testing.T) {
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	config := core.LedgerConfig{
		ClosedDate:         time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC),
		FunctionalCurrency: "USD",
		RateTolerance:      decimal.NewFromFloat(0.00001),
	}
	rp := mockRateProvider{rates: map[string]decimal.Decimal{"EUR/USD": decimal.NewFromFloat(1.1)}}

	t.Run("Valid Simple Transaction", func(t *testing.T) {
		b := core.NewTxBuilder("tx1", baseTime, config, rp)

		// +100 USD
		b.AddEntry(core.NewEntry("acc1", decimal.NewFromInt(100), "USD", decimal.NewFromInt(1), "USD"))
		// -100 USD
		b.AddEntry(core.NewEntry("acc2", decimal.NewFromInt(-100), "USD", decimal.NewFromInt(1), "USD"))

		tx, err := b.Build()
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if len(tx.Entries()) != 2 {
			t.Errorf("Expected 2 entries, got %d", len(tx.Entries()))
		}
	})
}

func TestTxBuilder_ZeroSum(t *testing.T) {
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	config := core.LedgerConfig{
		ClosedDate:         time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC),
		FunctionalCurrency: "USD",
		RateTolerance:      decimal.NewFromFloat(0.00001),
	}
	rp := mockRateProvider{rates: map[string]decimal.Decimal{"EUR/USD": decimal.NewFromFloat(1.1)}}

	t.Run("Zero-Sum Violation", func(t *testing.T) {
		b := core.NewTxBuilder("tx2", baseTime, config, rp)

		// +100 USD
		b.AddEntry(core.NewEntry("acc1", decimal.NewFromInt(100), "USD", decimal.NewFromInt(1), "USD"))
		// -99 USD (Sum = 1)
		b.AddEntry(core.NewEntry("acc2", decimal.NewFromInt(-99), "USD", decimal.NewFromInt(1), "USD"))

		_, err := b.Build()
		if !errors.Is(err, pkgerr.ErrZeroSumViolation) {
			t.Errorf("Expected ErrZeroSumViolation, got %v", err)
		}
	})
}

func TestTxBuilder_RateIntegrity(t *testing.T) {
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	config := core.LedgerConfig{
		ClosedDate:         time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC),
		FunctionalCurrency: "USD",
		RateTolerance:      decimal.NewFromFloat(0.00001),
	}
	rp := mockRateProvider{rates: map[string]decimal.Decimal{"EUR/USD": decimal.NewFromFloat(1.1)}}

	t.Run("Rate Deviation", func(t *testing.T) {
		b := core.NewTxBuilder("tx3", baseTime, config, rp)

		wrongRate := decimal.NewFromFloat(1.5)

		e1 := core.NewEntry("acc1", decimal.NewFromInt(100), "EUR", wrongRate, "USD")
		e2 := core.NewEntry("acc2", decimal.NewFromInt(-150), "USD", decimal.NewFromInt(1), "USD")

		b.AddEntry(e1).AddEntry(e2)

		_, err := b.Build()
		if !errors.Is(err, pkgerr.ErrRateDeviation) {
			t.Errorf("Expected ErrRateDeviation, got %v", err)
		}
	})
}

func TestTxBuilder_PeriodClosure(t *testing.T) {
	closedDate := time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC)
	config := core.LedgerConfig{
		ClosedDate:         closedDate,
		FunctionalCurrency: "USD",
		RateTolerance:      decimal.NewFromFloat(0.00001),
	}
	rp := mockRateProvider{rates: map[string]decimal.Decimal{"EUR/USD": decimal.NewFromFloat(1.1)}}

	t.Run("Failure", func(t *testing.T) {
		pastDate := closedDate.Add(-24 * time.Hour)
		b := core.NewTxBuilder("tx4", pastDate, config, rp)

		b.AddEntry(core.NewEntry("acc1", decimal.NewFromInt(100), "USD", decimal.NewFromInt(1), "USD"))
		b.AddEntry(core.NewEntry("acc2", decimal.NewFromInt(-100), "USD", decimal.NewFromInt(1), "USD"))

		_, err := b.Build()
		if !errors.Is(err, pkgerr.ErrPeriodClosed) {
			t.Errorf("Expected ErrPeriodClosed, got %v", err)
		}
	})

	t.Run("Adjusting Event Bypass", func(t *testing.T) {
		pastDate := closedDate.Add(-24 * time.Hour)
		b := core.NewTxBuilder("tx5", pastDate, config, rp)

		b.SetAdjustingEvent(true)

		b.AddEntry(core.NewEntry("acc1", decimal.NewFromInt(100), "USD", decimal.NewFromInt(1), "USD"))
		b.AddEntry(core.NewEntry("acc2", decimal.NewFromInt(-100), "USD", decimal.NewFromInt(1), "USD"))

		_, err := b.Build()
		if err != nil {
			t.Errorf("Expected success for Adjusting Event in closed period, got %v", err)
		}
	})
}

func TestTxBuilder_CurrencyConsistency(t *testing.T) {
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	config := core.LedgerConfig{
		ClosedDate:         time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC),
		FunctionalCurrency: "USD",
		RateTolerance:      decimal.NewFromFloat(0.00001),
	}
	rp := mockRateProvider{rates: map[string]decimal.Decimal{"EUR/USD": decimal.NewFromFloat(1.1)}}

	t.Run("Mismatch Check", func(t *testing.T) {
		b := core.NewTxBuilder("tx6", baseTime, config, rp)
		// Entry in EUR vs Config in USD
		e1 := core.NewEntry("acc1", decimal.NewFromInt(100), "EUR", decimal.NewFromInt(1), "EUR")
		b.AddEntry(e1)
		_, err := b.Build()
		if !errors.Is(err, pkgerr.ErrCurrencyMismatch) {
			t.Errorf("Expected ErrCurrencyMismatch, got %v", err)
		}
	})
}

func TestTxBuilder_NegativeRates(t *testing.T) {
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	config := core.LedgerConfig{
		ClosedDate:         time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC),
		FunctionalCurrency: "USD",
		RateTolerance:      decimal.NewFromFloat(0.00001),
	}
	rp := mockRateProvider{rates: map[string]decimal.Decimal{"EUR/USD": decimal.NewFromFloat(1.1)}}

	t.Run("Entry Violation", func(t *testing.T) {
		b := core.NewTxBuilder("tx-neg-entry", baseTime, config, rp)

		// Entry uses negative exchange rate: -1.1
		// 100 * -1.1 = -110 USD (Functional Amount)
		e1 := core.NewEntry("acc1", decimal.NewFromInt(100), "EUR", decimal.NewFromFloat(-1.1), "USD")

		// Balancing entry: Must be +110 to result in Sum = 0.
		// If we set this to 110, the Zero-Sum check passes, and we proceed to ValidateRates,
		// which should correctly flag the negative rate in e1.
		e2 := core.NewEntry("acc2", decimal.NewFromInt(110), "USD", decimal.NewFromInt(1), "USD")

		b.AddEntry(e1).AddEntry(e2)

		_, err := b.Build()
		if !errors.Is(err, pkgerr.ErrInvalidExchangeRate) {
			t.Errorf("Expected ErrInvalidExchangeRate for negative entry rate, got %v", err)
		}
	})

	t.Run("Provider Zero Rate Violation", func(t *testing.T) {
		zeroRP := mockRateProvider{
			rates: map[string]decimal.Decimal{
				"EUR/USD": decimal.Zero,
			},
		}
		b := core.NewTxBuilder("tx-zero-rate", baseTime, config, zeroRP)
		// Entry rate 1.0 vs Provider rate 0.0 -> Strictly Invalid
		e1 := core.NewEntry("acc1", decimal.NewFromInt(100), "EUR", decimal.NewFromInt(1), "USD")
		e2 := core.NewEntry("acc2", decimal.NewFromInt(-100), "USD", decimal.NewFromInt(1), "USD")
		b.AddEntry(e1).AddEntry(e2)

		_, err := b.Build()
		if !errors.Is(err, pkgerr.ErrInvalidExchangeRate) {
			t.Errorf("Expected ErrInvalidExchangeRate for zero provider rate, got %v", err)
		}
	})
}

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
	config := core.LedgerConfig{
		ClosedDate:         time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC),
		FunctionalCurrency: "USD",
	}
	rp := mockRateProvider{rates: map[string]decimal.Decimal{"EUR/USD": decimal.NewFromFloat(1.1)}}

	b := core.NewTxBuilder("tx1", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), config, rp)
	e1, _ := core.NewEntry("acc1", decimal.NewFromInt(100), "USD", decimal.NewFromInt(1), "USD")
	e2, _ := core.NewEntry("acc2", decimal.NewFromInt(-100), "USD", decimal.NewFromInt(1), "USD")

	b.AddEntry(e1).AddEntry(e2)
	tx, err := b.Build()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(tx.Entries()) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(tx.Entries()))
	}
}

func TestTxBuilder_ZeroSum(t *testing.T) {
	config := core.LedgerConfig{FunctionalCurrency: "USD"}
	b := core.NewTxBuilder("tx2", time.Now(), config, mockRateProvider{})

	e1, _ := core.NewEntry("acc1", decimal.NewFromInt(100), "USD", decimal.NewFromInt(1), "USD")
	e2, _ := core.NewEntry("acc2", decimal.NewFromInt(-99), "USD", decimal.NewFromInt(1), "USD")

	b.AddEntry(e1).AddEntry(e2)
	if _, err := b.Build(); !errors.Is(err, pkgerr.ErrZeroSumViolation) {
		t.Errorf("Expected ErrZeroSumViolation, got %v", err)
	}
}

func TestTxBuilder_RateIntegrity(t *testing.T) {
	config := core.LedgerConfig{
		FunctionalCurrency: "USD",
		RateTolerance:      decimal.NewFromFloat(0.00001),
	}
	rp := mockRateProvider{rates: map[string]decimal.Decimal{"EUR/USD": decimal.NewFromFloat(1.1)}}
	b := core.NewTxBuilder("tx3", time.Now(), config, rp)

	e1, _ := core.NewEntry("acc1", decimal.NewFromInt(100), "EUR", decimal.NewFromFloat(1.5), "USD")
	e2, _ := core.NewEntry("acc2", decimal.NewFromInt(-150), "USD", decimal.NewFromInt(1), "USD")

	b.AddEntry(e1).AddEntry(e2)
	if _, err := b.Build(); !errors.Is(err, pkgerr.ErrRateDeviation) {
		t.Errorf("Expected ErrRateDeviation, got %v", err)
	}
}

func TestTxBuilder_CurrencyConsistency(t *testing.T) {
	config := core.LedgerConfig{FunctionalCurrency: "USD"}
	b := core.NewTxBuilder("tx6", time.Now(), config, mockRateProvider{})

	// This now fails at NewEntry construction because of identity rate validation EUR vs USD
	// If we want to test the Builder's internal check, we must ensure NewEntry succeeds first.
	e1, _ := core.NewEntry("acc1", decimal.NewFromInt(100), "EUR", decimal.NewFromFloat(1.1), "EUR")
	b.AddEntry(e1)

	if _, err := b.Build(); !errors.Is(err, pkgerr.ErrCurrencyMismatch) {
		t.Errorf("Expected ErrCurrencyMismatch, got %v", err)
	}
}

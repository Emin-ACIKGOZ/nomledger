// SPDX-License-Identifier: MIT

package core_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Emin-ACIKGOZ/nomledger/pkg/core"
	pkgerr "github.com/Emin-ACIKGOZ/nomledger/pkg/errors"
	"github.com/Emin-ACIKGOZ/nomledger/pkg/interfaces"

	"github.com/shopspring/decimal"
)

type mockRateProvider struct {
	rates map[string]decimal.Decimal
}

func (m mockRateProvider) GetRate(_ context.Context, req interfaces.RateRequest) (decimal.Decimal, error) {
	key := string(req.Base) + "/" + string(req.Quote)
	if val, ok := m.rates[key]; ok {
		return val, nil
	}
	return decimal.Zero, errors.New("rate not found")
}

func TestTxBuilder_ValidTransaction(t *testing.T) {
	config := core.LedgerConfig{
		ClosedDate:         time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC),
		FunctionalCurrency: codeUSD,
	}
	rp := mockRateProvider{rates: map[string]decimal.Decimal{pairEURUSD: decimal.NewFromFloat(1.1)}}

	b := core.NewTxBuilder("tx1", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), config, rp)
	e1, _ := core.NewEntry("acc1", decimal.NewFromInt(100), codeUSD, decimal.NewFromInt(1), codeUSD)
	e2, _ := core.NewEntry("acc2", decimal.NewFromInt(-100), codeUSD, decimal.NewFromInt(1), codeUSD)

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
	config := core.LedgerConfig{FunctionalCurrency: codeUSD}
	b := core.NewTxBuilder("tx2", time.Now(), config, mockRateProvider{})

	e1, _ := core.NewEntry("acc1", decimal.NewFromInt(100), codeUSD, decimal.NewFromInt(1), codeUSD)
	e2, _ := core.NewEntry("acc2", decimal.NewFromInt(-99), codeUSD, decimal.NewFromInt(1), codeUSD)

	b.AddEntry(e1).AddEntry(e2)
	if _, err := b.Build(); !errors.Is(err, pkgerr.ErrZeroSumViolation) {
		t.Errorf("Expected ErrZeroSumViolation, got %v", err)
	}
}

func TestTxBuilder_RateIntegrity(t *testing.T) {
	config := core.LedgerConfig{
		FunctionalCurrency: codeUSD,
		RateTolerance:      decimal.NewFromFloat(0.00001),
	}
	rp := mockRateProvider{rates: map[string]decimal.Decimal{pairEURUSD: decimal.NewFromFloat(1.1)}}
	b := core.NewTxBuilder("tx3", time.Now(), config, rp)

	e1, _ := core.NewEntry("acc1", decimal.NewFromInt(100), codeEUR, decimal.NewFromFloat(1.5), codeUSD)
	e2, _ := core.NewEntry("acc2", decimal.NewFromInt(-150), codeUSD, decimal.NewFromInt(1), codeUSD)

	b.AddEntry(e1).AddEntry(e2)
	if _, err := b.Build(); !errors.Is(err, pkgerr.ErrRateDeviation) {
		t.Errorf("Expected ErrRateDeviation, got %v", err)
	}
}

func TestTxBuilder_CurrencyConsistency(t *testing.T) {
	config := core.LedgerConfig{FunctionalCurrency: codeUSD}
	b := core.NewTxBuilder("tx6", time.Now().UTC(), config, mockRateProvider{})

	e1, err := core.NewEntry("acc1", decimal.NewFromInt(100), codeEUR, decimal.NewFromInt(1), codeEUR)
	if err != nil {
		t.Fatalf("NewEntry: %v", err)
	}
	b.AddEntry(e1)

	_, err = b.Build()
	assertBuildValidationError(t, err, pkgerr.ErrCurrencyMismatch, "functionalCurrency", 0)
}

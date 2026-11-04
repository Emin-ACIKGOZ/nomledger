// SPDX-License-Identifier: MIT

package core

import (
	"context"
	"errors"
	"testing"
	"time"

	pkgerr "github.com/Emin-ACIKGOZ/nomledger/pkg/errors"

	"github.com/shopspring/decimal"
)

// TestLedgerNilProviderInternal bypasses NewLedger to prove Build never panics
// when a nil provider is reached through the unexported field.
func TestLedgerNilProviderInternal(t *testing.T) {
	ledger := &Ledger{
		config:       LedgerConfig{FunctionalCurrency: testCurrencyUSD},
		rateProvider: nil,
	}
	effective := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	t.Run("same-currency succeeds", func(t *testing.T) {
		testNilProviderSameCurrency(t, ledger, effective)
	})
	t.Run("foreign returns ErrNilRateProvider", func(t *testing.T) {
		testNilProviderForeign(t, ledger, effective)
	})
}

func testNilProviderSameCurrency(t *testing.T, ledger *Ledger, effective time.Time) {
	t.Helper()
	debit, err := NewEntry("internal-debit", decimal.NewFromInt(100), testCurrencyUSD, decimal.NewFromInt(1), testCurrencyUSD)
	if err != nil {
		t.Fatalf("NewEntry debit: %v", err)
	}
	credit, err := NewEntry("internal-credit", decimal.NewFromInt(-100), testCurrencyUSD, decimal.NewFromInt(1), testCurrencyUSD)
	if err != nil {
		t.Fatalf("NewEntry credit: %v", err)
	}
	if _, err := ledger.ValidateTransaction(context.Background(), TransactionInput{
		ID:            "tx-same-currency",
		EffectiveDate: effective,
		Entries:       []Entry{debit, credit},
	}); err != nil {
		t.Errorf("same-currency transaction error = %v; want nil", err)
	}
}

func testNilProviderForeign(t *testing.T, ledger *Ledger, effective time.Time) {
	t.Helper()
	fxDebit, err := NewEntry(
		"internal-fx-debit",
		decimal.NewFromInt(100),
		testEntryCurrencyEUR,
		mustEntryDecimalInternal(t, "1.1"),
		testCurrencyUSD,
	)
	if err != nil {
		t.Fatalf("NewEntry fx debit: %v", err)
	}
	fxCredit, err := NewEntry(
		"internal-fx-credit",
		decimal.NewFromInt(-110),
		testCurrencyUSD,
		decimal.NewFromInt(1),
		testCurrencyUSD,
	)
	if err != nil {
		t.Fatalf("NewEntry fx credit: %v", err)
	}

	_, err = ledger.ValidateTransaction(context.Background(), TransactionInput{
		ID:            "tx-foreign",
		EffectiveDate: effective,
		Entries:       []Entry{fxDebit, fxCredit},
	})
	if !errors.Is(err, pkgerr.ErrNilRateProvider) {
		t.Fatalf("foreign transaction error = %v; want %v", err, pkgerr.ErrNilRateProvider)
	}
	var validationErr *pkgerr.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("errors.As(err, *ValidationError) = false; err = %v", err)
	}
	if validationErr.Operation != validateTransactionOperation {
		t.Errorf("Operation = %q; want %q", validationErr.Operation, validateTransactionOperation)
	}
	if validationErr.Field != "rate" {
		t.Errorf("Field = %q; want %q", validationErr.Field, "rate")
	}
	if validationErr.EntryIndex != 0 {
		t.Errorf("EntryIndex = %d; want 0", validationErr.EntryIndex)
	}
}

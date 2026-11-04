// SPDX-License-Identifier: MIT

package core

import (
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

	debit, err := NewEntry("internal-debit", decimal.NewFromInt(100), testCurrencyUSD, decimal.NewFromInt(1), testCurrencyUSD)
	if err != nil {
		t.Fatalf("NewEntry debit: %v", err)
	}
	credit, err := NewEntry("internal-credit", decimal.NewFromInt(-100), testCurrencyUSD, decimal.NewFromInt(1), testCurrencyUSD)
	if err != nil {
		t.Fatalf("NewEntry credit: %v", err)
	}
	if _, err := ledger.ValidateTransaction("tx-same-currency", effective, false, debit, credit); err != nil {
		t.Errorf("same-currency transaction error = %v; want nil", err)
	}

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

	if _, err := ledger.ValidateTransaction("tx-foreign", effective, false, fxDebit, fxCredit); !errors.Is(err, pkgerr.ErrNilRateProvider) {
		t.Errorf("foreign transaction error = %v; want %v", err, pkgerr.ErrNilRateProvider)
	}
}

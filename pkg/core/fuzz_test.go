// SPDX-License-Identifier: MIT

package core_test

import (
	"context"
	"testing"
	"time"

	"github.com/Emin-ACIKGOZ/nomledger/pkg/core"
	"github.com/Emin-ACIKGOZ/nomledger/pkg/interfaces"

	"github.com/shopspring/decimal"
)

const (
	fuzzEffectiveDate = "2024-01-01T00:00:00Z"
	fuzzAccountID     = "acc"
)

// FuzzNewEntry exercises arbitrary entry inputs. Parse failures are skipped;
// anything accepted must re-validate and have a non-zero functional amount.
func FuzzNewEntry(f *testing.F) {
	seeds := [][5]string{
		{fuzzAccountID, txAmountHundred, "1", codeUSD, codeUSD},
		{"", txAmountHundred, "1", codeUSD, codeUSD},
		{"acc\x00id", txAmountHundred, "1", codeUSD, codeUSD},
		{fuzzAccountID, txAmountOverPrecision, "1", codeUSD, codeUSD},
		{fuzzAccountID, "0", "1", codeUSD, codeUSD},
		{fuzzAccountID, txAmountHundred, "0", codeUSD, codeUSD},
		{fuzzAccountID, txAmountHundred, "-1", codeUSD, codeUSD},
		{fuzzAccountID, txAmountHundred, "1", "XAU", codeUSD},
		{fuzzAccountID, txAmountHundred, "1", codeUSD, "BGN"},
		{fuzzAccountID, txAmountHundred, "1", "EUR", codeUSD},
	}
	for _, seed := range seeds {
		f.Add(seed[0], seed[1], seed[2], seed[3], seed[4])
	}

	f.Fuzz(func(t *testing.T, accountID, amountText, rateText, txCurrency, functionalCurrency string) {
		amount, err := decimal.NewFromString(amountText)
		if err != nil {
			return
		}
		rate, err := decimal.NewFromString(rateText)
		if err != nil {
			return
		}

		entry, err := core.NewEntry(accountID, amount, txCurrency, rate, functionalCurrency)
		if err != nil {
			return
		}
		if err := entry.Validate(); err != nil {
			t.Fatalf("accepted entry failed Validate: %v", err)
		}
		if entry.FunctionalAmount().IsZero() {
			t.Fatalf("accepted entry has zero functional amount")
		}
	})
}

// FuzzValidateTransaction drives the canonical ledger path with arbitrary field
// values. Parse failures are skipped; validation errors are allowed.
func FuzzValidateTransaction(f *testing.F) {
	seeds := [][6]string{
		{"tx", fuzzEffectiveDate, txAmountHundred, txAmountNegHundred, codeUSD, metaKeyNote},
		{"", fuzzEffectiveDate, txAmountHundred, txAmountNegHundred, codeUSD, ""},
		{"tx", "not-a-date", txAmountHundred, txAmountNegHundred, codeUSD, metaKeyNote},
		{"tx", fuzzEffectiveDate, txAmountOverPrecision, txAmountNegOverPrecision, codeUSD, ""},
		{"tx", fuzzEffectiveDate, txAmountHundred, "-99.00", codeUSD, ""},
		{"tx", fuzzEffectiveDate, txAmountHundred, txAmountNegHundred, "EUR", metaKeyNote},
		{"tx", fuzzEffectiveDate, txAmountHundred, txAmountNegHundred, "BGN", "bad\x00key"},
	}
	for _, seed := range seeds {
		f.Add(seed[0], seed[1], seed[2], seed[3], seed[4], seed[5])
	}

	config := core.LedgerConfig{
		ClosedDate:         time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC),
		FunctionalCurrency: codeUSD,
		RateTolerance:      decimal.RequireFromString("0.01"),
	}
	provider := interfaces.RateProviderFunc(func(context.Context, interfaces.RateRequest) (decimal.Decimal, error) {
		return decimal.NewFromInt(1), nil
	})
	clock := core.ClockFunc(func() time.Time {
		return time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	})
	ledger, err := core.NewLedger(config, provider, core.WithClock(clock))
	if err != nil {
		f.Fatalf("NewLedger: %v", err)
	}

	f.Fuzz(func(_ *testing.T, id, dateText, debitText, creditText, currencyCode, metadataKey string) {
		effective, err := time.Parse(time.RFC3339Nano, dateText)
		if err != nil {
			return
		}
		debitAmount, err := decimal.NewFromString(debitText)
		if err != nil {
			return
		}
		creditAmount, err := decimal.NewFromString(creditText)
		if err != nil {
			return
		}

		debit, err := core.NewEntry("fuzz-debit", debitAmount, currencyCode, decimal.NewFromInt(1), codeUSD)
		if err != nil {
			return
		}
		credit, err := core.NewEntry("fuzz-credit", creditAmount, currencyCode, decimal.NewFromInt(1), codeUSD)
		if err != nil {
			return
		}

		var metadata map[string]any
		if metadataKey != "" {
			metadata = map[string]any{metadataKey: true}
		}
		_, _ = ledger.ValidateTransaction(context.Background(), core.TransactionInput{
			ID:            id,
			EffectiveDate: effective,
			Entries:       []core.Entry{debit, credit},
			Metadata:      metadata,
		})
	})
}

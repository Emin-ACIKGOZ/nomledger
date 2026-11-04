// SPDX-License-Identifier: MIT

package core

import (
	"testing"

	"github.com/Emin-ACIKGOZ/nomledger/pkg/currency"
	pkgerr "github.com/Emin-ACIKGOZ/nomledger/pkg/errors"

	"github.com/shopspring/decimal"
)

const (
	testEntryOpValidate      = "Entry.Validate"
	testEntryFieldEntry      = "entry"
	testEntryFieldFuncAmount = "functionalAmount"
	testEntryAmountTen       = "10.00"
	testEntryAmountBad       = "9.99"
	testEntryCurrencyEUR     = "EUR"
)

func mustEntryDecimalInternal(t *testing.T, value string) decimal.Decimal {
	t.Helper()
	d, err := decimal.NewFromString(value)
	if err != nil {
		t.Fatalf("decimal.NewFromString(%q): %v", value, err)
	}
	return d
}

// TestEntryValidateInternalTampering proves Validate catches corrupted
// unexported state that callers cannot produce through NewEntry.
func TestEntryValidateInternalTampering(t *testing.T) {
	base, err := NewEntry(
		testAccountID,
		mustEntryDecimalInternal(t, testEntryAmountTen),
		testCurrencyUSD,
		mustEntryDecimalInternal(t, "1"),
		testCurrencyUSD,
	)
	if err != nil {
		t.Fatalf("NewEntry: %v", err)
	}
	if err := base.Validate(); err != nil {
		t.Fatalf("base Validate(): %v", err)
	}

	badTxAmount, err := currency.NewMoney(mustEntryDecimalInternal(t, testEntryAmountBad), currency.Code(testCurrencyUSD))
	if err != nil {
		t.Fatalf("NewMoney bad tx amount: %v", err)
	}
	badTxCurrency, err := currency.NewMoney(
		mustEntryDecimalInternal(t, testEntryAmountTen),
		currency.Code(testEntryCurrencyEUR),
	)
	if err != nil {
		t.Fatalf("NewMoney bad tx currency: %v", err)
	}
	badFunctionalAmount, err := currency.NewMoney(
		mustEntryDecimalInternal(t, testEntryAmountBad),
		currency.Code(testCurrencyUSD),
	)
	if err != nil {
		t.Fatalf("NewMoney bad functional amount: %v", err)
	}
	badFunctionalCurrency, err := currency.NewMoney(
		mustEntryDecimalInternal(t, testEntryAmountTen),
		currency.Code(testEntryCurrencyEUR),
	)
	if err != nil {
		t.Fatalf("NewMoney bad functional currency: %v", err)
	}

	cases := []struct {
		name   string
		mutate func(*Entry)
		field  string
	}{
		{
			name:   "inconsistent functional amount",
			mutate: func(e *Entry) { e.functionalAmount = mustEntryDecimalInternal(t, testEntryAmountBad) },
			field:  testEntryFieldFuncAmount,
		},
		{
			name:   "functional amount zero",
			mutate: func(e *Entry) { e.functionalAmount = decimal.Zero },
			field:  testEntryFieldFuncAmount,
		},
		{
			name:   "mismatched txMoney amount",
			mutate: func(e *Entry) { e.txMoney = badTxAmount },
			field:  testEntryFieldEntry,
		},
		{
			name:   "mismatched txMoney currency",
			mutate: func(e *Entry) { e.txMoney = badTxCurrency },
			field:  testEntryFieldEntry,
		},
		{
			name:   "mismatched functionalMoney amount",
			mutate: func(e *Entry) { e.functionalMoney = badFunctionalAmount },
			field:  testEntryFieldFuncAmount,
		},
		{
			name:   "mismatched functionalMoney currency",
			mutate: func(e *Entry) { e.functionalMoney = badFunctionalCurrency },
			field:  testEntryFieldFuncAmount,
		},
		{
			name:   "non-canonical stored tx currency",
			mutate: func(e *Entry) { e.txCurrency = testCurrencyLower },
			field:  testEntryFieldEntry,
		},
		{
			name:   "non-canonical stored functional currency",
			mutate: func(e *Entry) { e.functionalCurrency = testCurrencyLower },
			field:  testEntryFieldEntry,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry := base
			tc.mutate(&entry)

			first := entry.Validate()
			assertAccountStructuredError(t, first, pkgerr.ErrInvalidEntry, tc.field, testEntryOpValidate)

			second := entry.Validate()
			assertAccountStructuredError(t, second, pkgerr.ErrInvalidEntry, tc.field, testEntryOpValidate)
		})
	}
}

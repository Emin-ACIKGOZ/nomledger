// SPDX-License-Identifier: MIT

package core_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/Emin-ACIKGOZ/nomledger/pkg/core"
	"github.com/Emin-ACIKGOZ/nomledger/pkg/currency"
	pkgerr "github.com/Emin-ACIKGOZ/nomledger/pkg/errors"
	"github.com/Emin-ACIKGOZ/nomledger/pkg/interfaces"

	"github.com/shopspring/decimal"
)

func coreNewBuilder(t *testing.T) *core.TxBuilder {
	t.Helper()
	return coreNewBuilderWithProvider(t, mockRateProvider{})
}

func coreNewBuilderWithProvider(t *testing.T, provider interfaces.RateProvider) *core.TxBuilder {
	t.Helper()
	return core.NewTxBuilder(txID, ledgerEffectiveDate(), txTestConfig(), provider)
}

func coreNewEntry(t *testing.T, amount string, code currency.Code) (core.Entry, error) {
	t.Helper()
	d, err := decimal.NewFromString(amount)
	if err != nil {
		t.Fatalf("decimal.NewFromString(%q): %v", amount, err)
	}
	return core.NewEntry("acc-property", d, string(code), decimal.NewFromInt(1), string(code))
}

// TestZeroSumProperty verifies exact zero-sum behavior for balanced pairs and
// multi-entry splits.
func TestZeroSumProperty(t *testing.T) {
	cases := [][]string{
		{txAmountOne, txAmountNegOne},
		{"0.01", "-0.01"},
		{txAmountHundred, "-33.33", "-33.33", "-33.34"},
		{"999999.99", "-499999.99", "-500000.00"},
		{"0.10", "0.20", "-0.30"},
	}

	for index, amounts := range cases {
		t.Run(fmt.Sprintf("case_%d", index+1), func(t *testing.T) {
			builder := coreNewBuilder(t)
			for entryIndex, amount := range amounts {
				builder.AddEntry(mustTxEntry(t, fmt.Sprintf("acc-%d-%d", index, entryIndex), amount))
			}
			transaction, err := builder.Build()
			if err != nil {
				t.Fatalf("Build: %v", err)
			}

			sum := decimal.Zero
			for _, entry := range transaction.Entries() {
				sum = sum.Add(entry.FunctionalAmount())
			}
			if !sum.IsZero() {
				t.Errorf("functional sum = %s; want exactly zero", sum)
			}
		})
	}
}

// TestEntryPrecisionProperty verifies accepted entries never exceed the
// transaction currency scale and rejected over-precision values are not rounded.
func TestEntryPrecisionProperty(t *testing.T) {
	t.Run("accepted entries", testAcceptedEntryPrecision)
	t.Run("rejected entries", testRejectedEntryPrecision)
}

func testAcceptedEntryPrecision(t *testing.T) {
	t.Helper()
	accepted := []struct {
		amount string
		code   currency.Code
	}{
		{txAmountOne, codeUSD},
		{"1.2", codeUSD},
		{txAmountNegOne, codeUSD},
		{"100", codeJPY},
		{"1.234", codeBHD},
		{"0.0167", codeCLF},
	}
	for _, tc := range accepted {
		t.Run("accepts "+tc.amount+" "+string(tc.code), func(t *testing.T) {
			entry, err := coreNewEntry(t, tc.amount, tc.code)
			if err != nil {
				t.Fatalf("NewEntry: %v", err)
			}
			currencyInfo, err := currency.Lookup(string(tc.code))
			if err != nil {
				t.Fatalf("Lookup: %v", err)
			}
			minorUnits, _ := currencyInfo.MinorUnits()
			if entry.TxAmount().Exponent() < -minorUnits {
				t.Errorf("TxAmount() = %s; scale exceeds %d minor units", entry.TxAmount(), minorUnits)
			}
			if entry.FunctionalAmount().IsZero() {
				t.Error("accepted entry has zero functional amount")
			}
		})
	}
}

func testRejectedEntryPrecision(t *testing.T) {
	t.Helper()
	rejected := []struct {
		amount string
		code   currency.Code
	}{
		{txAmountOverPrecision, codeUSD},
		{txAmountNegOverPrecision, codeUSD},
		{"1.5", codeJPY},
		{"1.2345", codeBHD},
		{"0.01675", codeCLF},
	}
	for _, tc := range rejected {
		t.Run("rejects "+tc.amount+" "+string(tc.code), func(t *testing.T) {
			entry, err := coreNewEntry(t, tc.amount, tc.code)
			if !errors.Is(err, pkgerr.ErrInvalidPrecision) {
				t.Fatalf("NewEntry error = %v; want %v", err, pkgerr.ErrInvalidPrecision)
			}
			if entry.AccountID() != "" || entry.TxCurrency() != "" || entry.FunctionalCurrency() != "" {
				t.Errorf("rejected NewEntry returned non-zero Entry: %+v", entry)
			}
			if !entry.TxAmount().IsZero() || !entry.FunctionalAmount().IsZero() {
				t.Errorf("rejected NewEntry silently rounded: %+v", entry)
			}
		})
	}
}

// TestRateDedupeProperty verifies repeated pairs resolve once per unique request
// regardless of how many entries share the pair.
func TestRateDedupeProperty(t *testing.T) {
	for _, repeats := range []int{1, 3, 10} {
		t.Run(fmt.Sprintf("repeats_%d", repeats), func(t *testing.T) {
			provider := &recordingRateProvider{rates: map[string]decimal.Decimal{
				pairEURUSD: decimal.RequireFromString("1.1"),
				pairGBPUSD: decimal.RequireFromString("1.25"),
			}}
			builder := coreNewBuilderWithProvider(t, provider)

			sum := decimal.Zero
			for i := 0; i < repeats; i++ {
				eurEntry := mustRateEntry(t, fmt.Sprintf("eur-%d", i), txAmountHundred, codeEUR, ledgerRate, codeUSD)
				builder.AddEntry(eurEntry)
				sum = sum.Add(eurEntry.FunctionalAmount())

				gbpEntry := mustRateEntry(t, fmt.Sprintf("gbp-%d", i), rateAmountFifty, codeGBP, rateGBPUSD, codeUSD)
				builder.AddEntry(gbpEntry)
				sum = sum.Add(gbpEntry.FunctionalAmount())
			}
			builder.AddEntry(mustRateEntry(t, "usd-balance", sum.Neg().StringFixed(2), codeUSD, txRateOne, codeUSD))

			if _, err := builder.Build(); err != nil {
				t.Fatalf("Build: %v", err)
			}
			if provider.calls() != 2 {
				t.Errorf("provider calls = %d; want 2 unique pairs", provider.calls())
			}
		})
	}
}

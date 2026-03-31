// SPDX-License-Identifier: MIT

package core_test

import (
	"errors"
	"nomledger/pkg/core"
	pkgerr "nomledger/pkg/errors"
	"testing"

	"github.com/shopspring/decimal"
)

func TestNewEntry_CanonicalMath(t *testing.T) {
	cases := []struct {
		name           string
		txAmount       string
		rate           string
		txCurr         string
		funcCurr       string
		wantFuncAmount string // The expected result after Rate * Amount -> Rounding
	}{
		{
			name:           "Simple Conversion USD to EUR",
			txAmount:       "100.00",
			rate:           "0.85",
			txCurr:         "USD",
			funcCurr:       "EUR",
			wantFuncAmount: "85.00",
		},
		{
			name:           "Rounding Required JPY",
			txAmount:       "10.00",
			rate:           "10.55",
			txCurr:         "USD",
			funcCurr:       "JPY", // Precision 0
			wantFuncAmount: "106",
		},
		{
			name:           "Negative Credit Logic",
			txAmount:       "-100.00",
			rate:           "1.0",
			txCurr:         "USD",
			funcCurr:       "USD",
			wantFuncAmount: "-100.00",
		},
		{
			name:           "High Precision Input Truncation",
			txAmount:       "10.123456",
			rate:           "1.0",
			txCurr:         "USD",
			funcCurr:       "USD", // Precision 2
			wantFuncAmount: "10.12",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			amt, _ := decimal.NewFromString(tc.txAmount)
			rate, _ := decimal.NewFromString(tc.rate)
			want, _ := decimal.NewFromString(tc.wantFuncAmount)

			e, err := core.NewEntry("test-id", amt, tc.txCurr, rate, tc.funcCurr)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if !e.FunctionalAmount().Equal(want) {
				t.Errorf("NewEntry FunctionalAmount = %s; want %s (Input: %s @ %s)",
					e.FunctionalAmount(), want, amt, rate)
			}

			// Verify Immutability / Accessors
			if e.TxAmount().String() != amt.String() {
				t.Errorf("TxAmount accessor mismatch")
			}
			if e.ExchangeRate().String() != rate.String() {
				t.Errorf("ExchangeRate accessor mismatch")
			}
		})
	}
}

func TestNewEntry_WhenCurrenciesMatch_EnforcesIdentityRate(t *testing.T) {
	t.Run("Reject Arbitrary Value Creation", func(t *testing.T) {
		amt := decimal.NewFromInt(100)
		rate := decimal.NewFromFloat(1.1)

		_, err := core.NewEntry("acc-1", amt, "USD", rate, "USD")

		if !errors.Is(err, pkgerr.ErrInvalidExchangeRate) {
			t.Errorf("Expected ErrInvalidExchangeRate, got %v", err)
		}
	})
}

func TestNewEntry_Rounding_PreventsFunctionalZeroEntries(t *testing.T) {
	t.Run("Disallow Rounding to Zero", func(t *testing.T) {
		amt := decimal.NewFromFloat(0.004)
		rate := decimal.NewFromInt(1)

		_, err := core.NewEntry("acc-dust", amt, "USD", rate, "USD")

		if err == nil {
			t.Error("Expected error for functional amount rounding to zero, got nil")
		}
	})
}

func TestNewEntry_HighPrecision_PreservesDecimalIntegrity(t *testing.T) {
	t.Run("CLF Precision Preservation", func(t *testing.T) {
		// $1000 USD converted to CLF (Chile)
		// CLF has precision 4 in the table.
		amt := decimal.NewFromInt(1000)
		rate := decimal.NewFromFloat(0.00001667)

		// 1000 * 0.00001667 = 0.01667
		// Rounding 0.01667 to 4 decimals (Half-Even) -> 0.0167
		e, err := core.NewEntry("test-acc", amt, "USD", rate, "CLF")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		expected := decimal.NewFromFloat(0.0167)
		if !e.FunctionalAmount().Equal(expected) {
			t.Errorf("Measurement Failure: Expected %s CLF, got %s", expected, e.FunctionalAmount())
		}
	})
}

func TestNewEntry_RejectUnknownCurrency(t *testing.T) {
	t.Run("Unknown Functional Currency", func(t *testing.T) {
		amt := decimal.NewFromInt(100)
		rate := decimal.NewFromInt(1)

		// XYZ is NOT in the precision table, should be rejected.
		_, err := core.NewEntry("acc-1", amt, "USD", rate, "XYZ")

		if err == nil {
			t.Error("Expected error for unknown functional currency (XYZ), got nil")
		}
	})
}

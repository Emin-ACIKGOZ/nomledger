// SPDX-License-Identifier: MIT

package core_test

import (
	"nomledger/pkg/core"
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
			wantFuncAmount: "85.00", // 100 * 0.85 = 85.00
		},
		{
			name:           "Rounding Required JPY",
			txAmount:       "10.00",
			rate:           "10.55",
			txCurr:         "USD",
			funcCurr:       "JPY", // Precision 0
			wantFuncAmount: "106", // 10 * 10.55 = 105.5 -> RoundHalfEven -> 106
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

			e := core.NewEntry("test-id", amt, tc.txCurr, rate, tc.funcCurr)

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

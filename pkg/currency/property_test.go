// SPDX-License-Identifier: MIT

package currency_test

import (
	"errors"
	"testing"

	"github.com/Emin-ACIKGOZ/nomledger/pkg/currency"
	pkgerr "github.com/Emin-ACIKGOZ/nomledger/pkg/errors"

	"github.com/shopspring/decimal"
)

// TestPrecisionProperty verifies accepted Money amounts stay within the
// currency scale and over-precision inputs are rejected without rounding.
const (
	jpyInexactAmount = "1.5"
	bhdInexactAmount = "1.2345"
)

type precisionCase struct {
	name    string
	amount  string
	code    currency.Code
	wantErr error
}

func TestPrecisionProperty(t *testing.T) {
	cases := []precisionCase{
		{"USD 1.00", "1.00", codeUSD, nil},
		{"USD 1.2", "1.2", codeUSD, nil},
		{"USD -1.00", "-1.00", codeUSD, nil},
		{"USD 1.001", "1.001", codeUSD, pkgerr.ErrInvalidPrecision},
		{"USD -1.001", "-1.001", codeUSD, pkgerr.ErrInvalidPrecision},
		{"USD 1.230", "1.230", codeUSD, pkgerr.ErrInvalidPrecision},
		{"JPY 100", "100", codeJPY, nil},
		{"JPY 1.5", jpyInexactAmount, codeJPY, pkgerr.ErrInvalidPrecision},
		{"BHD 1.234", "1.234", codeBHD, nil},
		{"BHD 1.2345", bhdInexactAmount, codeBHD, pkgerr.ErrInvalidPrecision},
		{"CLF 0.0167", "0.0167", "CLF", nil},
		{"CLF 0.01675", "0.01675", "CLF", pkgerr.ErrInvalidPrecision},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertPrecisionCase(t, tc)
		})
	}
}

func assertPrecisionCase(t *testing.T, tc precisionCase) {
	t.Helper()
	amount, err := decimal.NewFromString(tc.amount)
	if err != nil {
		t.Fatalf("decimal.NewFromString: %v", err)
	}
	money, err := currency.NewMoney(amount, tc.code)

	if tc.wantErr != nil {
		if !errors.Is(err, tc.wantErr) {
			t.Fatalf("NewMoney error = %v; want %v", err, tc.wantErr)
		}
		if !money.Amount().IsZero() || money.Currency() != "" {
			t.Errorf("rejected NewMoney returned non-zero Money: %+v", money)
		}
		return
	}
	if err != nil {
		t.Fatalf("NewMoney: %v", err)
	}

	currencyInfo, err := currency.Lookup(string(tc.code))
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	minorUnits, _ := currencyInfo.MinorUnits()
	if money.Amount().Exponent() < -minorUnits {
		t.Errorf("Amount() = %s; scale exceeds %d minor units", money.Amount(), minorUnits)
	}
	if !money.Amount().Equal(amount) {
		t.Errorf("Amount() = %s; want %s", money.Amount(), amount)
	}
}

// SPDX-License-Identifier: MIT

package currency_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/Emin-ACIKGOZ/nomledger/pkg/currency"
	pkgerr "github.com/Emin-ACIKGOZ/nomledger/pkg/errors"

	"github.com/shopspring/decimal"
)

// FuzzLookup: malformed input is rejected before registry lookup, and
// well-formed unknown input returns ErrUnknownCurrency without panicking.
func FuzzLookup(f *testing.F) {
	seeds := []string{
		codeUSD, codeUSDLower, codeUSDMixedCase, codeUnknown, "", " USD", "US", "US1", "БГЛ",
		codeXAU, codeXDR, codeBGN, codeJPY, "ZZZ", "123",
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, code string) {
		c, err := currency.Lookup(code)

		if !isThreeASCIILetters(code) {
			if !errors.Is(err, pkgerr.ErrInvalidCurrencyCode) {
				t.Fatalf("Lookup(%q) error = %v; want %v", code, err, pkgerr.ErrInvalidCurrencyCode)
			}
			return
		}

		if err != nil {
			if !errors.Is(err, pkgerr.ErrUnknownCurrency) {
				t.Fatalf("Lookup(%q) error = %v; want %v", code, err, pkgerr.ErrUnknownCurrency)
			}
			return
		}

		wantCode := strings.ToUpper(code)
		if string(c.Code()) != wantCode {
			t.Fatalf("Lookup(%q).Code() = %q; want %q", code, c.Code(), wantCode)
		}
	})
}

// FuzzRound is the rounding fuzz target: parseable amounts never panic, and a
// successful result is expressed at or below the currency's minor-unit scale.
func FuzzRound(f *testing.F) {
	for _, amount := range []string{"1.005", "-1.005", "1.230", "0", "-0", "1000", "abc", "1e3"} {
		for _, code := range []string{codeUSD, codeJPY, codeBHD, codeXAU, codeBGN, "US", ""} {
			for _, mode := range []byte{0, 1, 2, 3, 4, 5, 6} {
				f.Add(amount, code, mode)
			}
		}
	}

	f.Fuzz(func(t *testing.T, amountText, currencyCode string, modeByte byte) {
		amount, err := decimal.NewFromString(amountText)
		if err != nil {
			return
		}

		result, err := currency.Round(amount, currency.Code(currencyCode), currency.RoundingMode(modeByte))
		if err != nil {
			if !allowedRoundError(err) {
				t.Fatalf("Round(%q, %q, %d) unexpected error: %v", amountText, currencyCode, modeByte, err)
			}
			return
		}

		currencyInfo, lookupErr := currency.Lookup(currencyCode)
		if lookupErr != nil {
			t.Fatalf("Round succeeded for invalid code %q", currencyCode)
		}
		minorUnits, ratable := currencyInfo.MinorUnits()
		if !ratable {
			t.Fatalf("Round succeeded for non-ratable code %q", currencyCode)
		}
		if result.Exponent() < -minorUnits {
			t.Fatalf("Round(%q, %q, %d) = %s; scale exceeds minor units %d",
				amountText, currencyCode, modeByte, result, minorUnits)
		}
	})
}

func allowedRoundError(err error) bool {
	return errors.Is(err, pkgerr.ErrInvalidCurrencyCode) ||
		errors.Is(err, pkgerr.ErrUnknownCurrency) ||
		errors.Is(err, pkgerr.ErrCurrencyNotRatable) ||
		errors.Is(err, pkgerr.ErrInvalidRoundingMode) ||
		errors.Is(err, pkgerr.ErrInexactRounding)
}

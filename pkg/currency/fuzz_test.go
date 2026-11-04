// SPDX-License-Identifier: MIT

package currency_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/Emin-ACIKGOZ/nomledger/pkg/currency"
	pkgerr "github.com/Emin-ACIKGOZ/nomledger/pkg/errors"
)

// FuzzLookup: malformed input is rejected before registry lookup, and
// well-formed unknown input returns ErrUnknownCurrency without panicking.
func FuzzLookup(f *testing.F) {
	seeds := []string{
		codeUSD, "usd", codeUSDMixedCase, codeUnknown, "", " USD", "US", "US1", "БГЛ",
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

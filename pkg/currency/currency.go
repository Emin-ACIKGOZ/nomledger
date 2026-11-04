// SPDX-License-Identifier: MIT

package currency

import (
	"sort"
	"strings"

	"github.com/Emin-ACIKGOZ/nomledger/internal/iso4217"
	pkgerr "github.com/Emin-ACIKGOZ/nomledger/pkg/errors"

	"github.com/shopspring/decimal"
)

const codeLength = 3

// Code is an ISO 4217 alphabetic currency code.
type Code string

// Currency is an immutable registry entry for one active ISO 4217 currency.
type Currency struct {
	entry iso4217.Entry
}

var (
	registry map[Code]Currency
	ordered  []Currency
)

func init() {
	entries := iso4217.Entries()
	registry = make(map[Code]Currency, len(entries))
	ordered = make([]Currency, 0, len(entries))

	for _, entry := range entries {
		currency := Currency{entry: entry}
		registry[Code(entry.Code)] = currency
		ordered = append(ordered, currency)
	}

	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].Code() < ordered[j].Code()
	})
}

// Lookup returns the registry entry for a three-letter ISO 4217 code.
//
// Lookup accepts any ASCII letter case and normalises the result to the
// canonical uppercase code. It validates the code before consulting the
// registry and returns ErrInvalidCurrencyCode for malformed input or
// ErrUnknownCurrency for well-formed codes that are not active. Ratability is
// not enforced; use Currency.MinorUnits to check whether an amount can be
// represented in the currency.
func Lookup(code string) (Currency, error) {
	if !validCode(code) {
		return Currency{}, pkgerr.ErrInvalidCurrencyCode
	}

	currency, ok := registry[Code(strings.ToUpper(code))]
	if !ok {
		return Currency{}, pkgerr.ErrUnknownCurrency
	}
	return currency, nil
}

// All returns a defensive copy of every active currency, sorted by code.
func All() []Currency {
	all := make([]Currency, len(ordered))
	copy(all, ordered)
	return all
}

// Code returns the canonical uppercase ISO 4217 alphabetic code.
func (c Currency) Code() Code {
	return Code(c.entry.Code)
}

// Numeric returns the ISO 4217 numeric code as a zero-padded three-digit string.
func (c Currency) Numeric() string {
	return c.entry.Numeric
}

// MinorUnits returns the ISO 4217 minor-unit exponent and whether the currency
// is ratable. Currencies whose minor units are published as N.A. return
// (0, false).
func (c Currency) MinorUnits() (int32, bool) {
	return c.entry.MinorUnits, c.entry.Ratable
}

// Name returns the published ISO 4217 currency name.
func (c Currency) Name() string {
	return c.entry.Name
}

// IsFund reports whether the currency is registered in SIX List Two as a fund.
func (c Currency) IsFund() bool {
	return c.entry.Fund
}

// GetPrecision returns the ISO 4217 exponent for a given currency code.
//
// Unknown, malformed, and non-ratable codes return (0, false).
//
// Deprecated: use Lookup and Currency.MinorUnits instead.
func GetPrecision(code string) (int32, bool) {
	currency, err := Lookup(code)
	if err != nil {
		return 0, false
	}
	return currency.MinorUnits()
}

// RoundToPrecision aligns the amount to the specific precision of the provided currency.
//
// It utilizes Banker's Rounding (Half-to-Even) to minimize bias in accumulated
// operations.
//
// Deprecated: use Round with RoundHalfEven, or NewMoney, instead.
func RoundToPrecision(amount decimal.Decimal, currencyCode string) (decimal.Decimal, error) {
	return Round(amount, Code(currencyCode), RoundHalfEven)
}

func validCode(code string) bool {
	if len(code) != codeLength {
		return false
	}
	for i := 0; i < len(code); i++ {
		if !isASCIILetter(code[i]) {
			return false
		}
	}
	return true
}

func isASCIILetter(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}

// SPDX-License-Identifier: MIT

package currency

import (
	"github.com/Emin-ACIKGOZ/nomledger/pkg/errors"
	"github.com/shopspring/decimal"
)

// Money is an immutable amount in a specific currency.
type Money struct {
	amount decimal.Decimal
	code   Code
}

// NewMoney constructs a Money amount in the currency identified by code.
//
// It rejects malformed or unknown codes through Lookup, rejects non-ratable
// currencies with ErrCurrencyNotRatable, and rejects amounts whose decimal
// scale exceeds the currency's minor units with ErrInvalidPrecision. Amounts
// are never rounded silently.
func NewMoney(amount decimal.Decimal, code Code) (Money, error) {
	currency, minorUnits, err := ratableCurrency(code)
	if err != nil {
		return Money{}, err
	}
	if amount.Exponent() < -minorUnits {
		return Money{}, errors.ErrInvalidPrecision
	}
	return newMoney(amount, currency.Code()), nil
}

// newMoney constructs Money without revalidating the amount's precision.
func newMoney(amount decimal.Decimal, code Code) Money {
	return Money{amount: amount, code: code}
}

// Amount returns the stored decimal amount.
func (m Money) Amount() decimal.Decimal {
	return m.amount
}

// Currency returns the canonical uppercase currency code.
func (m Money) Currency() Code {
	return m.code
}

// Add returns the sum of m and other.
//
// It returns ErrCurrencyMismatch when the currencies differ.
func (m Money) Add(other Money) (Money, error) {
	if m.code != other.code {
		return Money{}, errors.ErrCurrencyMismatch
	}
	return newMoney(m.amount.Add(other.amount), m.code), nil
}

// Sub returns the difference between m and other.
//
// It returns ErrCurrencyMismatch when the currencies differ.
func (m Money) Sub(other Money) (Money, error) {
	if m.code != other.code {
		return Money{}, errors.ErrCurrencyMismatch
	}
	return newMoney(m.amount.Sub(other.amount), m.code), nil
}

// Neg returns the additive inverse of m.
func (m Money) Neg() Money {
	return newMoney(m.amount.Neg(), m.code)
}

// IsZero reports whether the amount is numerically zero.
func (m Money) IsZero() bool {
	return m.amount.IsZero()
}

// String returns the amount formatted to the currency's minor units followed
// by the canonical currency code, such as "10.00 USD". The zero-value Money
// renders only its amount, without a currency suffix.
func (m Money) String() string {
	if m.code == "" {
		return m.amount.String()
	}
	currency, err := Lookup(string(m.code))
	if err != nil {
		return m.amount.String() + " " + string(m.code)
	}
	minorUnits, ratable := currency.MinorUnits()
	if !ratable {
		return m.amount.String() + " " + string(m.code)
	}
	return m.amount.StringFixed(minorUnits) + " " + string(m.code)
}

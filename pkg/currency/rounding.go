// SPDX-License-Identifier: MIT

package currency

import (
	"github.com/Emin-ACIKGOZ/nomledger/pkg/errors"
	"github.com/shopspring/decimal"
)

const (
	halfUnitCoefficient = 5
	halfUnitExponent    = -1
)

var halfUnit = decimal.New(halfUnitCoefficient, halfUnitExponent)

// RoundingMode selects how a decimal amount is rounded to a currency's minor
// units.
type RoundingMode uint8

const (
	// RoundHalfEven rounds ties to the nearest even digit.
	RoundHalfEven RoundingMode = iota
	// RoundHalfUp rounds ties away from zero.
	RoundHalfUp
	// RoundHalfDown rounds ties toward zero.
	RoundHalfDown
	// RoundFloor rounds toward negative infinity.
	RoundFloor
	// RoundCeiling rounds toward positive infinity.
	RoundCeiling
	// RoundUnnecessary rejects amounts that would change when rounded.
	RoundUnnecessary
)

// Round rounds amount to the minor units of code using mode.
//
// It returns the lookup error for malformed or unknown codes,
// ErrCurrencyNotRatable for currencies without defined minor units,
// ErrInvalidRoundingMode for unknown modes, and ErrInexactRounding when
// RoundUnnecessary would change the numeric value.
func Round(amount decimal.Decimal, code Code, mode RoundingMode) (decimal.Decimal, error) {
	_, minorUnits, err := ratableCurrency(code)
	if err != nil {
		return decimal.Zero, err
	}

	switch mode {
	case RoundHalfEven:
		return amount.RoundBank(minorUnits), nil
	case RoundHalfUp:
		return amount.Round(minorUnits), nil
	case RoundHalfDown:
		return roundHalfDown(amount, minorUnits), nil
	case RoundFloor:
		return amount.RoundFloor(minorUnits), nil
	case RoundCeiling:
		return amount.RoundCeil(minorUnits), nil
	case RoundUnnecessary:
		return roundUnnecessary(amount, minorUnits)
	default:
		return decimal.Zero, errors.ErrInvalidRoundingMode
	}
}

func roundHalfDown(amount decimal.Decimal, places int32) decimal.Decimal {
	scaled := amount.Shift(places)
	truncated := scaled.Truncate(0)
	if scaled.Sub(truncated).Abs().Equal(halfUnit) {
		return truncated.Shift(-places)
	}
	return amount.Round(places)
}

func roundUnnecessary(amount decimal.Decimal, places int32) (decimal.Decimal, error) {
	rounded := amount.Round(places)
	if amount.Equal(rounded) {
		return rounded, nil
	}
	return decimal.Zero, errors.ErrInexactRounding
}

func ratableCurrency(code Code) (Currency, int32, error) {
	currency, err := Lookup(string(code))
	if err != nil {
		return Currency{}, 0, err
	}
	minorUnits, ratable := currency.MinorUnits()
	if !ratable {
		return Currency{}, 0, errors.ErrCurrencyNotRatable
	}
	return currency, minorUnits, nil
}

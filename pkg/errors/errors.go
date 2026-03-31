// SPDX-License-Identifier: MIT

package errors

import "errors"

// Typed Error Taxonomy as defined in the Technical Design.
var (
	// ErrZeroSumViolation indicates the transaction entries do not sum to zero.
	ErrZeroSumViolation = errors.New("transaction does not sum to zero")

	// ErrPeriodClosed indicates the effective date falls in a closed period,
	// and the event is not marked as an adjusting event.
	ErrPeriodClosed = errors.New("effective date falls in closed period")

	// ErrRateDeviation indicates the exchange rate provided exceeds the
	// allowable tolerance compared to the RateProvider.
	ErrRateDeviation = errors.New("exchange rate exceeds tolerance")

	// ErrInvalidPrecision indicates an amount exceeds the ISO 4217 precision
	// defined for that currency.
	ErrInvalidPrecision = errors.New("amount exceeds currency precision")

	// ErrCurrencyMismatch indicates an entry's functional currency does not
	// match the ledger's configured functional currency.
	ErrCurrencyMismatch = errors.New("entry functional currency does not match ledger")

	// ErrEmptyTransaction indicates the transaction contains no entries.
	// Zero-entry transactions are strictly invalid.
	ErrEmptyTransaction = errors.New("transaction must contain at least one entry")

	// ErrInvalidExchangeRate indicates a logic error in the exchange rate.
	// Both entry-provided rates and provider-returned rates must be strictly positive.
	// Zero or negative rates are invalid.
	ErrInvalidExchangeRate = errors.New("exchange rate must be strictly positive")

	// ErrInvalidConfiguration indicates the LedgerConfig is malformed (e.g. negative tolerance).
	ErrInvalidConfiguration = errors.New("invalid ledger configuration")

	// ErrUnknownCurrency indicates the currency code is not supported by the precision table.
	ErrUnknownCurrency = errors.New("unknown or unsupported currency code")
)

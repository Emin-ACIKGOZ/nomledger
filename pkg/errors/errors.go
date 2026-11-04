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

	// ErrRateProvider indicates a rate provider failure or context cancellation
	// while obtaining exchange rates.
	ErrRateProvider = errors.New("rate provider error")

	// ErrInvalidPrecision indicates an amount exceeds the ISO 4217 precision
	// defined for that currency.
	ErrInvalidPrecision = errors.New("amount exceeds currency precision")

	// ErrCurrencyMismatch indicates an entry's functional currency does not
	// match the ledger's configured functional currency.
	ErrCurrencyMismatch = errors.New("entry functional currency does not match ledger")

	// ErrEmptyTransaction indicates the transaction contains no entries.
	// Zero-entry transactions are strictly invalid.
	ErrEmptyTransaction = errors.New("transaction must contain at least one entry")

	// ErrZeroAmount indicates a transaction amount is zero.
	ErrZeroAmount = errors.New("transaction amount must be non-zero")

	// ErrFunctionalAmountZero indicates a non-zero transaction amount rounds to
	// zero in the functional currency.
	ErrFunctionalAmountZero = errors.New("functional amount rounds to zero")

	// ErrInvalidEntry indicates a zero-value or internally inconsistent entry.
	ErrInvalidEntry = errors.New("invalid entry")

	// ErrNilRateProvider indicates a nil rate provider was supplied or a rate
	// lookup was attempted without one.
	ErrNilRateProvider = errors.New("rate provider must not be nil")

	// ErrInvalidTransactionID indicates the transaction ID is empty, too long,
	// or contains a control character.
	ErrInvalidTransactionID = errors.New("invalid transaction ID")

	// ErrInvalidEffectiveDate indicates the effective date is the zero time.
	ErrInvalidEffectiveDate = errors.New("invalid effective date")

	// ErrInvalidMetadata indicates a metadata key is empty, too long, or
	// contains a control character.
	ErrInvalidMetadata = errors.New("invalid metadata")

	// ErrInvalidAccountID indicates the account ID is empty, too long, or
	// contains a control character.
	ErrInvalidAccountID = errors.New("invalid account ID")

	// ErrInvalidAccountType indicates the account type is not one of the
	// supported reporting classifications.
	ErrInvalidAccountType = errors.New("invalid account type")

	// ErrInvalidMeasurementCategory indicates the measurement category is not
	// one of the supported IFRS 9 tags.
	ErrInvalidMeasurementCategory = errors.New("invalid measurement category")

	// ErrInvalidExchangeRate indicates a logic error in the exchange rate.
	// Both entry-provided rates and provider-returned rates must be strictly positive.
	// Zero or negative rates are invalid.
	ErrInvalidExchangeRate = errors.New("exchange rate must be strictly positive")

	// ErrInvalidConfiguration indicates the LedgerConfig is malformed (e.g. negative tolerance).
	ErrInvalidConfiguration = errors.New("invalid ledger configuration")

	// ErrUnknownCurrency indicates the currency code is well-formed but not
	// present in the active ISO 4217 registry.
	ErrUnknownCurrency = errors.New("unknown or unsupported currency code")

	// ErrInvalidCurrencyCode indicates the currency code is not exactly three
	// ASCII letters.
	ErrInvalidCurrencyCode = errors.New("currency code must be exactly three ASCII letters")

	// ErrCurrencyNotRatable indicates the currency has no defined minor units
	// and therefore cannot be used for monetary amounts.
	ErrCurrencyNotRatable = errors.New("currency has no defined minor units")

	// ErrInvalidRoundingMode indicates the requested rounding mode is unknown.
	ErrInvalidRoundingMode = errors.New("invalid rounding mode")

	// ErrInexactRounding indicates an unnecessary-rounding request would change
	// the numeric value.
	ErrInexactRounding = errors.New("rounding would change the value")
)

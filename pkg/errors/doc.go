// SPDX-License-Identifier: MIT

// Package errors defines the typed failure taxonomy returned by NomLedger.
//
// # Matching
//
// Exported sentinels identify each failure class and are matched with errors.Is:
//
//	if errors.Is(err, ErrInvalidPrecision) {
//		// handle the failure
//	}
//
// Public validation failures are *ValidationError values carrying the sentinel
// code, operation, entry index, field, and an optional cause. Structured context
// is extracted with errors.As:
//
//	var validationErr *ValidationError
//	if errors.As(err, &validationErr) {
//		// inspect validationErr.Operation, Field, EntryIndex, and Cause
//	}
//
// ValidationError.Unwrap exposes both Code and Cause, so errors.Is can match
// either one when both are present. In particular, ErrRateProvider covers
// cancellation or deadline expiry before any validation/provider work
// (including same-currency transactions), cancellation during a provider call,
// and provider errors. When cancellation and a provider error occur together,
// errors.Is matches both ErrRateProvider and context.Canceled; errors.As can
// still recover the original provider error type.
//
// # Retryability
//
// Retryable means the caller can retry without changing the input, for example
// after a transient provider failure or with a fresh context. Non-retryable
// errors require changing the input, configuration, or caller usage.
//
// # Sentinels
//
//	| Sentinel | Message | Returned when | Retryable |
//	|---|---|---|---|
//	| ErrZeroSumViolation | transaction does not sum to zero | Build sees functional amounts whose exact sum is non-zero. | No |
//	| ErrPeriodClosed | effective date falls in closed period | A non-adjusting transaction has an effective date at or before ClosedDate. A zero ClosedDate disables the check. | No |
//	| ErrRateDeviation | exchange rate exceeds tolerance | Provider and entry rates differ by more than RateTolerance. | No |
//	| ErrRateProvider | rate provider error | Context cancellation or deadline expiry before any validation/provider work (including same-currency transactions), cancellation during a provider call, or a provider call returns an error. | Yes |
//	| ErrInvalidPrecision | amount exceeds currency precision | An Entry or Money amount has more decimal places than the currency's minor units. | No |
//	| ErrCurrencyMismatch | entry functional currency does not match ledger | An entry's functional currency differs from the ledger's, or Money arithmetic mixes currencies. | No |
//	| ErrEmptyTransaction | transaction must contain at least one entry | Build receives zero entries. | No |
//	| ErrZeroAmount | transaction amount must be non-zero | NewEntry receives a zero transaction amount. | No |
//	| ErrFunctionalAmountZero | functional amount rounds to zero | A non-zero transaction amount rounds to zero in the functional currency. | No |
//	| ErrInvalidEntry | invalid entry | Entry.Validate sees a zero-value or internally inconsistent entry, or Build's entry check fails. | No |
//	| ErrNilRateProvider | rate provider must not be nil | NewLedger receives a nil provider, or a rate lookup is attempted without one. | No |
//	| ErrInvalidTransactionID | invalid transaction ID | Build receives an empty, over-255-byte, or control-character transaction ID. | No |
//	| ErrInvalidEffectiveDate | invalid effective date | Build receives the zero time as the effective date. | No |
//	| ErrInvalidMetadata | invalid metadata | Build receives a metadata key that is empty, over 64 bytes, or contains a control character. | No |
//	| ErrInvalidAccountID | invalid account ID | NewAccount, NewEntry, or their Validate methods receive an invalid account ID. | No |
//	| ErrInvalidAccountType | invalid account type | NewAccount or Account.Validate receives an unsupported account type. | No |
//	| ErrInvalidMeasurementCategory | invalid measurement category | NewAccount or Account.Validate receives an unsupported measurement category. | No |
//	| ErrInvalidExchangeRate | exchange rate must be strictly positive | An entry or provider rate is zero or negative, or a same-currency entry rate is not exactly one. | No |
//	| ErrInvalidConfiguration | invalid ledger configuration | LedgerConfig has an invalid functional currency or a negative tolerance. | No |
//	| ErrUnknownCurrency | unknown or unsupported currency code | A well-formed code is absent from the active ISO 4217 registry. | No |
//	| ErrInvalidCurrencyCode | currency code must be exactly three ASCII letters | A currency code is not exactly three ASCII letters. | No |
//	| ErrCurrencyNotRatable | currency has no defined minor units | A currency with N.A. minor units is used where ratability is required. | No |
//	| ErrInvalidRoundingMode | invalid rounding mode | Round receives an unknown RoundingMode. | No |
//	| ErrInexactRounding | rounding would change the value | RoundUnnecessary would change the numeric amount. | No |
//	| ErrBuilderFinalized | builder is already finalized | Build is called again after a successful build finalized the builder. | No |
package errors

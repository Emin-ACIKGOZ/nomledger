// SPDX-License-Identifier: MIT

// Package errors defines the taxonomy of typed faults returned by the
// validation engine.
//
// Exported sentinels identify each failure class and are matched with
// errors.Is:
//
//	if errors.Is(err, ErrInvalidPrecision) {
//		// handle the failure
//	}
//
// Packages migrated to structured errors return *ValidationError values that
// carry the sentinel code, operation, entry index, field, and an optional
// cause. The remaining validation paths still return sentinel errors directly
// and will be migrated in later work units. Structured context is extracted
// with errors.As:
//
//	var validationErr *ValidationError
//	if errors.As(err, &validationErr) {
//		// inspect validationErr.Operation, Field, EntryIndex, and Cause
//	}
//
// ValidationError.Unwrap exposes both Code and Cause, so errors.Is can match
// either one when both are present.
package errors

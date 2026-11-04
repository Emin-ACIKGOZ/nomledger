// SPDX-License-Identifier: MIT

package errors

// NewValidationError returns a transaction-level ValidationError. Its
// EntryIndex is -1.
func NewValidationError(code error, operation, field string, cause error) *ValidationError {
	return &ValidationError{
		Code:       code,
		Operation:  operation,
		EntryIndex: -1,
		Field:      field,
		Cause:      cause,
	}
}

// NewEntryValidationError returns an entry-level ValidationError with the
// supplied zero-based entry index.
func NewEntryValidationError(code error, operation string, entryIndex int, field string, cause error) *ValidationError {
	return &ValidationError{
		Code:       code,
		Operation:  operation,
		EntryIndex: entryIndex,
		Field:      field,
		Cause:      cause,
	}
}

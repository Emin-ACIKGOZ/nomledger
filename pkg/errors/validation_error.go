// SPDX-License-Identifier: MIT

package errors

import (
	"errors"
	"strconv"
	"strings"
)

// ValidationError describes a validation failure and the context in which it
// occurred.
type ValidationError struct {
	// Code is the sentinel error that identifies the failure class.
	Code error
	// Operation is the public operation that rejected the input.
	Operation string
	// EntryIndex is -1 for transaction-level failures or the zero-based entry
	// index for entry-level failures.
	EntryIndex int
	// Field is the input field associated with the failure, when known.
	Field string
	// Cause is the underlying error, when one exists.
	Cause error
}

// Error returns a deterministic, readable description of the validation
// failure. It includes the operation, entry index, field, code, and cause when
// those details are present.
func (e *ValidationError) Error() string {
	if e == nil {
		return "<nil>"
	}

	var b strings.Builder
	b.WriteString("validation error")
	if e.Operation != "" {
		b.WriteString(" in ")
		b.WriteString(e.Operation)
	}
	if e.EntryIndex >= 0 {
		b.WriteString(" at entry ")
		b.WriteString(strconv.Itoa(e.EntryIndex))
	}
	if e.Field != "" {
		b.WriteString(" (field ")
		b.WriteString(e.Field)
		b.WriteString(")")
	}
	if detail := errorDetail(e.Code, e.Cause); detail != "" {
		b.WriteString(": ")
		b.WriteString(detail)
	}
	return b.String()
}

// Unwrap exposes both the sentinel code and the cause so errors.Is and
// errors.As can inspect either one.
func (e *ValidationError) Unwrap() error {
	if e == nil {
		return nil
	}

	switch {
	case e.Code == nil:
		return e.Cause
	case e.Cause == nil:
		return e.Code
	case sameError(e.Code, e.Cause):
		return e.Code
	default:
		return errors.Join(e.Code, e.Cause)
	}
}

func sameError(a, b error) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return errors.Is(a, b) && errors.Is(b, a)
}

func errorDetail(code, cause error) string {
	codeText, causeText := "", ""
	if code != nil {
		codeText = code.Error()
	}
	if cause != nil {
		causeText = cause.Error()
	}

	switch {
	case codeText == "":
		return causeText
	case causeText == "":
		return codeText
	case strings.Contains(codeText, causeText):
		return codeText
	case strings.Contains(causeText, codeText):
		return causeText
	default:
		return codeText + ": " + causeText
	}
}

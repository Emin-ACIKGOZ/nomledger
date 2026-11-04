// SPDX-License-Identifier: MIT

package errors_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Emin-ACIKGOZ/nomledger/pkg/currency"
	pkgerr "github.com/Emin-ACIKGOZ/nomledger/pkg/errors"

	"github.com/shopspring/decimal"
)

const (
	codeUSD            = "USD"
	codeEUR            = "EUR"
	codeBGN            = "BGN"
	codeXAU            = "XAU"
	codeMalformed      = "US"
	opValidate         = "ValidateTransaction"
	fieldContext       = "context"
	opNewMoney         = "NewMoney"
	fieldAmount        = "amount"
	testField          = "field"
	opRoundToPrecision = "RoundToPrecision"
)

// testCause is a custom cause type used to verify errors.As traversal.
type testCause struct {
	message string
}

func (c *testCause) Error() string { return c.message }

func TestValidationErrorIsAndAs(t *testing.T) {
	sentinel := pkgerr.ErrInvalidPrecision
	err := pkgerr.NewValidationError(sentinel, opNewMoney, fieldAmount, nil)

	if !errors.Is(err, sentinel) {
		t.Errorf("errors.Is(err, sentinel) = false; want true")
	}

	var validationErr *pkgerr.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("errors.As(err, *ValidationError) = false; want true")
	}
	if validationErr.Code != sentinel {
		t.Errorf("Code = %v; want %v", validationErr.Code, sentinel)
	}
	if validationErr.Operation != opNewMoney {
		t.Errorf("Operation = %q; want %q", validationErr.Operation, opNewMoney)
	}
	if validationErr.Field != fieldAmount {
		t.Errorf("Field = %q; want %q", validationErr.Field, fieldAmount)
	}

	wrapped := fmt.Errorf("outer: %w", err)
	if !errors.Is(wrapped, sentinel) {
		t.Errorf("wrapped errors.Is(err, sentinel) = false; want true")
	}
	var wrappedValidationErr *pkgerr.ValidationError
	if !errors.As(wrapped, &wrappedValidationErr) {
		t.Errorf("wrapped errors.As(err, *ValidationError) = false; want true")
	}
}

func TestValidationErrorCause(t *testing.T) {
	t.Run("wrapped cause sentinel matches", func(t *testing.T) {
		causeSentinel := errors.New("cause sentinel")
		cause := fmt.Errorf("provider: %w", causeSentinel)
		err := pkgerr.NewValidationError(pkgerr.ErrInvalidPrecision, opValidate, fieldContext, cause)

		if !errors.Is(err, pkgerr.ErrInvalidPrecision) {
			t.Errorf("errors.Is(err, ErrInvalidPrecision) = false; want true")
		}
		if !errors.Is(err, causeSentinel) {
			t.Errorf("errors.Is(err, cause sentinel) = false; want true")
		}
	})

	t.Run("custom cause type is extractable", func(t *testing.T) {
		cause := &testCause{message: "custom provider failure"}
		err := pkgerr.NewValidationError(pkgerr.ErrInvalidPrecision, opValidate, fieldContext, cause)

		var got *testCause
		if !errors.As(err, &got) {
			t.Fatalf("errors.As(err, *testCause) = false; want true")
		}
		if got != cause {
			t.Errorf("extracted cause = %p; want %p", got, cause)
		}
	})
}

func TestValidationErrorJoined(t *testing.T) {
	err := pkgerr.NewValidationError(pkgerr.ErrInvalidPrecision, opValidate, fieldContext, context.Canceled)

	if !errors.Is(err, pkgerr.ErrInvalidPrecision) {
		t.Errorf("errors.Is(err, ErrInvalidPrecision) = false; want true")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("errors.Is(err, context.Canceled) = false; want true")
	}

	message := err.Error()
	for _, want := range []string{pkgerr.ErrInvalidPrecision.Error(), context.Canceled.Error()} {
		if !strings.Contains(message, want) {
			t.Errorf("Error() = %q; want it to contain %q", message, want)
		}
	}
}

func TestValidationErrorEntryIndex(t *testing.T) {
	transactionErr := pkgerr.NewValidationError(pkgerr.ErrEmptyTransaction, opValidate, "entries", nil)
	if transactionErr.EntryIndex != -1 {
		t.Errorf("NewValidationError EntryIndex = %d; want -1", transactionErr.EntryIndex)
	}
	if strings.Contains(transactionErr.Error(), "entry ") {
		t.Errorf("transaction-level Error() = %q; want no entry index", transactionErr.Error())
	}

	for _, index := range []int{0, 2} {
		entryErr := pkgerr.NewEntryValidationError(pkgerr.ErrInvalidPrecision, opValidate, index, fieldAmount, nil)
		if entryErr.EntryIndex != index {
			t.Errorf("NewEntryValidationError EntryIndex = %d; want %d", entryErr.EntryIndex, index)
		}
		wantEntry := fmt.Sprintf("entry %d", index)
		if !strings.Contains(entryErr.Error(), wantEntry) {
			t.Errorf("Error() = %q; want it to contain %q", entryErr.Error(), wantEntry)
		}
	}
}

func TestValidationErrorFields(t *testing.T) {
	t.Run("all context fields are rendered", func(t *testing.T) {
		err := pkgerr.NewValidationError(pkgerr.ErrInvalidPrecision, opNewMoney, fieldAmount, context.Canceled)
		message := err.Error()
		for _, want := range []string{
			opNewMoney,
			fieldAmount,
			pkgerr.ErrInvalidPrecision.Error(),
			context.Canceled.Error(),
		} {
			if !strings.Contains(message, want) {
				t.Errorf("Error() = %q; want it to contain %q", message, want)
			}
		}
	})

	t.Run("same code and cause is not duplicated", func(t *testing.T) {
		sentinel := errors.New("repeated text")
		err := pkgerr.NewValidationError(sentinel, opNewMoney, fieldAmount, sentinel)
		message := err.Error()
		if got := strings.Count(message, sentinel.Error()); got != 1 {
			t.Errorf("Error() = %q; occurrence count = %d; want 1", message, got)
		}
	})

	t.Run("same code and cause message is not duplicated", func(t *testing.T) {
		code := errors.New("same message")
		cause := errors.New("same message")
		err := pkgerr.NewValidationError(code, opNewMoney, fieldAmount, cause)
		message := err.Error()
		if got := strings.Count(message, code.Error()); got != 1 {
			t.Errorf("Error() = %q; occurrence count = %d; want 1", message, got)
		}
	})

	t.Run("empty context is stable and non-empty", func(t *testing.T) {
		sentinel := pkgerr.ErrInvalidPrecision
		err := pkgerr.NewValidationError(sentinel, "", "", nil)
		first := err.Error()
		second := err.Error()
		if first == "" {
			t.Fatal("Error() = empty; want non-empty")
		}
		if first != second {
			t.Errorf("Error() is unstable: %q != %q", first, second)
		}
		if !strings.Contains(first, sentinel.Error()) {
			t.Errorf("Error() = %q; want it to contain %q", first, sentinel.Error())
		}
	})
}

// TestValidationErrorErrorContainmentDedup is the O-06 regression guard: when
// one of code/cause text contains the other, that text is rendered once.
func TestValidationErrorErrorContainmentDedup(t *testing.T) {
	codeText := pkgerr.ErrInvalidPrecision.Error()

	t.Run("code contains cause text", func(t *testing.T) {
		cause := fmt.Errorf("precision: %w", pkgerr.ErrInvalidPrecision)
		err := pkgerr.NewValidationError(pkgerr.ErrInvalidPrecision, opNewMoney, fieldAmount, cause)
		message := err.Error()
		if got := strings.Count(message, codeText); got != 1 {
			t.Errorf("Error() = %q; code occurrence count = %d; want 1", message, got)
		}
	})

	t.Run("cause contains code text", func(t *testing.T) {
		code := fmt.Errorf("precision: %w", pkgerr.ErrInvalidPrecision)
		err := pkgerr.NewValidationError(code, opNewMoney, fieldAmount, pkgerr.ErrInvalidPrecision)
		message := err.Error()
		if got := strings.Count(message, codeText); got != 1 {
			t.Errorf("Error() = %q; code occurrence count = %d; want 1", message, got)
		}
	})

	t.Run("distinct code and cause render both", func(t *testing.T) {
		err := pkgerr.NewValidationError(pkgerr.ErrInvalidPrecision, opNewMoney, fieldAmount, context.Canceled)
		message := err.Error()
		for _, want := range []string{codeText, context.Canceled.Error()} {
			if got := strings.Count(message, want); got != 1 {
				t.Errorf("Error() = %q; occurrence count for %q = %d; want 1", message, want, got)
			}
		}
	})

	t.Run("cause only renders cause", func(t *testing.T) {
		err := &pkgerr.ValidationError{Cause: context.Canceled, EntryIndex: -1}
		message := err.Error()
		if got := strings.Count(message, context.Canceled.Error()); got != 1 {
			t.Errorf("Error() = %q; cause occurrence count = %d; want 1", message, got)
		}
	})

	t.Run("code only renders code", func(t *testing.T) {
		err := &pkgerr.ValidationError{Code: pkgerr.ErrInvalidPrecision, EntryIndex: -1}
		message := err.Error()
		if got := strings.Count(message, codeText); got != 1 {
			t.Errorf("Error() = %q; code occurrence count = %d; want 1", message, got)
		}
	})
}

func TestValidationErrorUnwrapNil(t *testing.T) {
	t.Run("code only", testUnwrapCodeOnly)
	t.Run("cause only", testUnwrapCauseOnly)
	t.Run("both nil", testUnwrapBothNil)
	t.Run("both non-nil", testUnwrapBothSet)
	t.Run("nil receiver is safe", testUnwrapNilReceiver)
}

func testUnwrapCodeOnly(t *testing.T) {
	t.Helper()
	codeSentinel := errors.New("code sentinel")
	err := pkgerr.NewValidationError(codeSentinel, opValidate, testField, nil)
	if got := err.Unwrap(); got != codeSentinel {
		t.Errorf("Unwrap() = %v; want code sentinel", got)
	}
	if !errors.Is(err, codeSentinel) {
		t.Errorf("errors.Is(err, code) = false; want true")
	}
}

func testUnwrapCauseOnly(t *testing.T) {
	t.Helper()
	causeSentinel := errors.New("cause sentinel")
	err := &pkgerr.ValidationError{Cause: causeSentinel, EntryIndex: -1}
	if got := err.Unwrap(); got != causeSentinel {
		t.Errorf("Unwrap() = %v; want cause sentinel", got)
	}
	if !errors.Is(err, causeSentinel) {
		t.Errorf("errors.Is(err, cause) = false; want true")
	}
}

func testUnwrapBothNil(t *testing.T) {
	t.Helper()
	codeSentinel := errors.New("code sentinel")
	err := &pkgerr.ValidationError{EntryIndex: -1}
	if got := err.Unwrap(); got != nil {
		t.Errorf("Unwrap() = %v; want nil", got)
	}
	if errors.Is(err, codeSentinel) {
		t.Errorf("errors.Is(err, code) = true; want false")
	}
}

func testUnwrapBothSet(t *testing.T) {
	t.Helper()
	codeSentinel := errors.New("code sentinel")
	causeSentinel := errors.New("cause sentinel")
	err := pkgerr.NewValidationError(codeSentinel, opValidate, testField, causeSentinel)
	if got := err.Unwrap(); got == nil {
		t.Fatal("Unwrap() = nil; want joined code and cause")
	}
	if !errors.Is(err, codeSentinel) {
		t.Errorf("errors.Is(err, code) = false; want true")
	}
	if !errors.Is(err, causeSentinel) {
		t.Errorf("errors.Is(err, cause) = false; want true")
	}
}

func testUnwrapNilReceiver(t *testing.T) {
	t.Helper()
	var err *pkgerr.ValidationError
	if got := err.Error(); got != "<nil>" {
		t.Errorf("nil Error() = %q; want %q", got, "<nil>")
	}
	if got := err.Unwrap(); got != nil {
		t.Errorf("nil Unwrap() = %v; want nil", got)
	}
}

// TestCurrencyErrorsAreStructured: public currency failures are
// *ValidationError values with operation, field, and transaction-level context.
func TestCurrencyErrorsAreStructured(t *testing.T) {
	for _, tc := range currencyErrorCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			assertStructuredCurrencyError(t, tc)
		})
	}
}

type currencyErrorCase struct {
	name          string
	call          func() error
	want          error
	wantOperation string
}

func currencyErrorCases(t *testing.T) []currencyErrorCase {
	t.Helper()
	usd, err := currency.NewMoney(mustDecimal(t, "1.00"), codeUSD)
	if err != nil {
		t.Fatalf("NewMoney(1.00, USD): %v", err)
	}
	eur, err := currency.NewMoney(mustDecimal(t, "2.00"), codeEUR)
	if err != nil {
		t.Fatalf("NewMoney(2.00, EUR): %v", err)
	}

	return []currencyErrorCase{
		{
			name: "malformed lookup code",
			call: func() error {
				_, lookupErr := currency.Lookup(codeMalformed)
				return lookupErr
			},
			want: pkgerr.ErrInvalidCurrencyCode,
		},
		{
			name: "unknown lookup code",
			call: func() error {
				_, lookupErr := currency.Lookup(codeBGN)
				return lookupErr
			},
			want: pkgerr.ErrUnknownCurrency,
		},
		{
			name: "malformed round code",
			call: func() error {
				_, roundErr := currency.Round(decimal.NewFromInt(1), codeMalformed, currency.RoundHalfEven)
				return roundErr
			},
			want: pkgerr.ErrInvalidCurrencyCode,
		},
		{
			name: "unknown round code",
			call: func() error {
				_, roundErr := currency.Round(decimal.NewFromInt(1), codeBGN, currency.RoundHalfEven)
				return roundErr
			},
			want: pkgerr.ErrUnknownCurrency,
		},
		{
			name: "non-ratable round",
			call: func() error {
				_, roundErr := currency.Round(decimal.NewFromInt(1), codeXAU, currency.RoundHalfEven)
				return roundErr
			},
			want: pkgerr.ErrCurrencyNotRatable,
		},
		{
			name: "invalid rounding mode",
			call: func() error {
				_, roundErr := currency.Round(decimal.NewFromInt(1), codeUSD, currency.RoundingMode(99))
				return roundErr
			},
			want: pkgerr.ErrInvalidRoundingMode,
		},
		{
			name: "inexact unnecessary round",
			call: func() error {
				_, roundErr := currency.Round(mustDecimal(t, "1.231"), codeUSD, currency.RoundUnnecessary)
				return roundErr
			},
			want: pkgerr.ErrInexactRounding,
		},
		{
			name: "malformed NewMoney code",
			call: func() error {
				_, moneyErr := currency.NewMoney(decimal.NewFromInt(1), codeMalformed)
				return moneyErr
			},
			want: pkgerr.ErrInvalidCurrencyCode,
		},
		{
			name: "unknown NewMoney code",
			call: func() error {
				_, moneyErr := currency.NewMoney(decimal.NewFromInt(1), codeBGN)
				return moneyErr
			},
			want: pkgerr.ErrUnknownCurrency,
		},
		{
			name: "non-ratable NewMoney",
			call: func() error {
				_, moneyErr := currency.NewMoney(decimal.NewFromInt(1), codeXAU)
				return moneyErr
			},
			want: pkgerr.ErrCurrencyNotRatable,
		},
		{
			name: "excess precision",
			call: func() error {
				_, moneyErr := currency.NewMoney(mustDecimal(t, "1.001"), codeUSD)
				return moneyErr
			},
			want: pkgerr.ErrInvalidPrecision,
		},
		{
			name: "cross-currency add",
			call: func() error {
				_, addErr := usd.Add(eur)
				return addErr
			},
			want: pkgerr.ErrCurrencyMismatch,
		},
		{
			name: "cross-currency sub",
			call: func() error {
				_, subErr := usd.Sub(eur)
				return subErr
			},
			want: pkgerr.ErrCurrencyMismatch,
		},
		{
			name: "malformed RoundToPrecision code",
			call: func() error {
				_, precisionErr := currency.RoundToPrecision(decimal.NewFromInt(1), codeMalformed) //nolint:staticcheck // exercising the deprecated wrapper
				return precisionErr
			},
			want:          pkgerr.ErrInvalidCurrencyCode,
			wantOperation: opRoundToPrecision,
		},
		{
			name: "unknown RoundToPrecision code",
			call: func() error {
				_, precisionErr := currency.RoundToPrecision(decimal.NewFromInt(1), codeBGN) //nolint:staticcheck // exercising the deprecated wrapper
				return precisionErr
			},
			want:          pkgerr.ErrUnknownCurrency,
			wantOperation: opRoundToPrecision,
		},
		{
			name: "non-ratable RoundToPrecision",
			call: func() error {
				_, precisionErr := currency.RoundToPrecision(decimal.NewFromInt(1), codeXAU) //nolint:staticcheck // exercising the deprecated wrapper
				return precisionErr
			},
			want:          pkgerr.ErrCurrencyNotRatable,
			wantOperation: opRoundToPrecision,
		},
	}
}

func assertStructuredCurrencyError(t *testing.T, tc currencyErrorCase) {
	t.Helper()
	err := tc.call()
	if !errors.Is(err, tc.want) {
		t.Fatalf("errors.Is(err, %v) = false; err = %v", tc.want, err)
	}

	var validationErr *pkgerr.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("errors.As(err, *ValidationError) = false; err = %v", err)
	}
	if validationErr.Operation == "" {
		t.Error("Operation is empty; want non-empty")
	}
	if validationErr.Field == "" {
		t.Error("Field is empty; want non-empty")
	}
	if validationErr.EntryIndex != -1 {
		t.Errorf("EntryIndex = %d; want -1 for transaction-level errors", validationErr.EntryIndex)
	}
	if tc.wantOperation != "" && validationErr.Operation != tc.wantOperation {
		t.Errorf("Operation = %q; want %q", validationErr.Operation, tc.wantOperation)
	}
}

func mustDecimal(t *testing.T, value string) decimal.Decimal {
	t.Helper()
	d, err := decimal.NewFromString(value)
	if err != nil {
		t.Fatalf("decimal.NewFromString(%q): %v", value, err)
	}
	return d
}

// SPDX-License-Identifier: MIT

package core

import (
	"errors"

	"github.com/Emin-ACIKGOZ/nomledger/pkg/currency"
	pkgerr "github.com/Emin-ACIKGOZ/nomledger/pkg/errors"
)

const (
	maxAccountIDBytes = 255
	asciiDEL          = 0x7F
)

// AccountType represents the high-level reporting classification of an account.
//
// These types are used for report generation (e.g., distinguishing Assets from
// Liabilities). Their membership is validated by NewAccount and Account.Validate,
// but the engine does not enforce natural-balance or reporting policy, allowing
// for flexible transaction structures.
type AccountType int8

const (
	// Asset identifies resources controlled by the entity.
	Asset AccountType = iota + 1
	// Liability identifies present obligations of the entity.
	Liability
	// Equity identifies the residual interest in the assets of the entity.
	Equity
	// Income identifies increases in economic benefits during the accounting period.
	Income
	// Expense identifies decreases in economic benefits during the accounting period.
	Expense
)

// MeasurementCategory represents IFRS 9 classification tags.
//
// These are declarative markers used for downstream reporting and valuation
// logic. The library stores these tags but does not enforce business-model
// tests, amortization logic, or any measurement treatment.
type MeasurementCategory int8

const (
	// NotApplicable indicates that no IFRS 9 measurement category applies.
	NotApplicable MeasurementCategory = iota
	// AmortizedCost indicates the asset is held to collect contractual cash flows.
	AmortizedCost
	// FVTPL indicates the asset is measured at Fair Value Through Profit or Loss.
	FVTPL
	// FVOCI indicates the asset is measured at Fair Value Through Other
	// Comprehensive Income.
	FVOCI
)

// Account is the fundamental unit of storage for financial classification.
//
// Accounts are structurally immutable after construction through NewAccount.
// Normative data (ID, type, category, and currency) is validated at construction
// and by Account.Validate. Measurement categories are declarative tags only.
//
// Metadata is non-normative and is never consulted by validation. The map is
// shallowly copied on construction and on every Metadata call; values that are
// reference types remain shared with the caller.
type Account struct {
	id           string
	typ          AccountType
	cat          MeasurementCategory
	currencyCode string
	metadata     map[string]any
}

// NewAccount constructs and validates a structurally immutable Account.
//
// It copies the supplied metadata map, treating nil as an empty map. Validation
// checks the ID, type, measurement category, and currency in that order. The
// returned error is a *ValidationError wrapping the relevant sentinel.
func NewAccount(
	id string,
	typ AccountType,
	cat MeasurementCategory,
	currencyCode string,
	metadata map[string]any,
) (Account, error) {
	canonicalCurrency, err := validateAccount(id, typ, cat, currencyCode, "NewAccount")
	if err != nil {
		return Account{}, err
	}

	return Account{
		id:           id,
		typ:          typ,
		cat:          cat,
		currencyCode: canonicalCurrency,
		metadata:     copyMetadata(metadata),
	}, nil
}

// ID returns the account identifier.
func (a Account) ID() string {
	return a.id
}

// Type returns the account's reporting classification.
func (a Account) Type() AccountType {
	return a.typ
}

// Category returns the account's declarative IFRS 9 measurement tag.
func (a Account) Category() MeasurementCategory {
	return a.cat
}

// CurrencyCode returns the canonical uppercase ISO 4217 currency code.
func (a Account) CurrencyCode() string {
	return a.currencyCode
}

// Metadata returns a defensive shallow copy of the account's metadata.
func (a Account) Metadata() map[string]any {
	return copyMetadata(a.metadata)
}

// Validate re-validates the account without modifying it.
//
// Validation checks the ID, type, measurement category, and currency in that
// order and returns the first failure as a *ValidationError.
func (a Account) Validate() error {
	_, err := validateAccount(a.id, a.typ, a.cat, a.currencyCode, "Account.Validate")
	return err
}

func validateAccount(
	id string,
	typ AccountType,
	cat MeasurementCategory,
	currencyCode string,
	operation string,
) (string, error) {
	if !validAccountID(id) {
		return "", pkgerr.NewValidationError(pkgerr.ErrInvalidAccountID, operation, "id", nil)
	}
	if !validAccountType(typ) {
		return "", pkgerr.NewValidationError(pkgerr.ErrInvalidAccountType, operation, "type", nil)
	}
	if !validMeasurementCategory(cat) {
		return "", pkgerr.NewValidationError(pkgerr.ErrInvalidMeasurementCategory, operation, "category", nil)
	}
	return validateAccountCurrency(currencyCode, operation)
}

func validateAccountCurrency(code string, operation string) (string, error) {
	currencyInfo, err := currency.Lookup(code)
	if err != nil {
		return "", rewrapValidationError(err, operation, "currency")
	}

	_, ratable := currencyInfo.MinorUnits()
	if !ratable {
		return "", pkgerr.NewValidationError(pkgerr.ErrCurrencyNotRatable, operation, "currency", nil)
	}
	return string(currencyInfo.Code()), nil
}

func rewrapValidationError(err error, operation, field string) error {
	var validationErr *pkgerr.ValidationError
	if errors.As(err, &validationErr) {
		return pkgerr.NewValidationError(validationErr.Code, operation, field, validationErr.Cause)
	}
	return pkgerr.NewValidationError(err, operation, field, nil)
}

func validAccountID(id string) bool {
	if id == "" || len(id) > maxAccountIDBytes {
		return false
	}
	for i := 0; i < len(id); i++ {
		if id[i] < ' ' || id[i] == asciiDEL {
			return false
		}
	}
	return true
}

func validAccountType(typ AccountType) bool {
	switch typ {
	case Asset, Liability, Equity, Income, Expense:
		return true
	default:
		return false
	}
}

func validMeasurementCategory(cat MeasurementCategory) bool {
	switch cat {
	case NotApplicable, AmortizedCost, FVTPL, FVOCI:
		return true
	default:
		return false
	}
}

func copyMetadata(metadata map[string]any) map[string]any {
	copied := make(map[string]any, len(metadata))
	for key, value := range metadata {
		copied[key] = value
	}
	return copied
}

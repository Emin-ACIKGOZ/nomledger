// SPDX-License-Identifier: MIT

package core

// AccountType represents the high-level reporting classification of an account.
//
// These types are used for report generation (e.g., distinguishing Assets from
// Liabilities) but are not enforced by the validation engine itself. The engine
// remains agnostic to "natural balance" rules, allowing for flexible transaction
// structures.
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
// These are declarative markers used for downstream reporting and valuation logic.
// The library stores these tags but does not enforce business model tests (SPPI)
// or amortization logic.
type MeasurementCategory int8

const (
	// AmortizedCost indicates the asset is held to collect contractual cash flows.
	AmortizedCost MeasurementCategory = iota + 1
	// FVTPL indicates the asset is measured at Fair Value Through Profit or Loss.
	FVTPL
)

// Account is the fundamental unit of storage for financial classification.
//
// It separates normative data (ID, Type), which is required for basic reporting,
// from descriptive metadata. The library does not enforce currency consistency
// at the Account level; this is enforced at the Transaction level against the Ledger.
type Account struct {
	ID string

	// Type determines the reporting category (Asset, Liability, etc.).
	Type AccountType

	// Category stores the IFRS 9 measurement tag for reporting utility.
	Category MeasurementCategory

	// CurrencyCode is the ISO 4217 string for the account's denomination.
	CurrencyCode string

	// Metadata contains non-normative data (e.g., descriptions, internal codes).
	// It is passed through opaquely and is not used in validation logic.
	Metadata map[string]interface{}
}

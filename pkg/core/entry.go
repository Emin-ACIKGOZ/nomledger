// SPDX-License-Identifier: MIT

package core

import (
	"github.com/Emin-ACIKGOZ/nomledger/pkg/currency"
	pkgerr "github.com/Emin-ACIKGOZ/nomledger/pkg/errors"

	"github.com/shopspring/decimal"
)

const (
	entryOperationNew      = "NewEntry"
	entryOperationValidate = "Entry.Validate"
)

// Entry represents a single line item in a financial transaction.
//
// It enforces a strict signed-amount convention where positive values represent
// debits and negative values represent credits. Fields are unexported to keep
// the entry structurally immutable after construction.
//
// NewEntry validates the account ID, both currency codes, the transaction
// amount's precision and non-zero value, and the exchange rate. It rejects
// same-currency entries unless the rate is exactly one and uses HalfEven
// rounding for the functional amount. A non-zero transaction amount that rounds
// to zero in the functional currency is rejected.
type Entry struct {
	accountID string

	txAmount     decimal.Decimal
	txCurrency   string
	exchangeRate decimal.Decimal

	functionalAmount   decimal.Decimal
	functionalCurrency string

	txMoney         currency.Money
	functionalMoney currency.Money
}

// NewEntry constructs and validates a structurally immutable Entry.
//
// Functional amounts use canonical HalfEven rounding at the functional
// currency's minor units. The returned error is a *ValidationError wrapping the
// relevant sentinel.
func NewEntry(
	accountID string,
	txAmount decimal.Decimal,
	txCurrency string,
	exchangeRate decimal.Decimal,
	functionalCurrency string,
) (Entry, error) {
	fields, err := validateEntryFields(
		accountID,
		txAmount,
		txCurrency,
		exchangeRate,
		functionalCurrency,
		entryOperationNew,
	)
	if err != nil {
		return Entry{}, err
	}

	functionalAmount, err := currency.Round(
		txAmount.Mul(exchangeRate),
		currency.Code(fields.functionalCurrency),
		currency.RoundHalfEven,
	)
	if err != nil {
		return Entry{}, rewrapValidationError(err, entryOperationNew, "functionalCurrency")
	}
	if !txAmount.IsZero() && functionalAmount.IsZero() {
		return Entry{}, pkgerr.NewValidationError(
			pkgerr.ErrFunctionalAmountZero,
			entryOperationNew,
			"functionalAmount",
			nil,
		)
	}

	txMoney, err := currency.NewMoney(txAmount, currency.Code(fields.txCurrency))
	if err != nil {
		return Entry{}, rewrapValidationError(err, entryOperationNew, "txAmount")
	}
	functionalMoney, err := currency.NewMoney(functionalAmount, currency.Code(fields.functionalCurrency))
	if err != nil {
		return Entry{}, rewrapValidationError(err, entryOperationNew, "functionalAmount")
	}

	return Entry{
		accountID:          accountID,
		txAmount:           txAmount,
		txCurrency:         fields.txCurrency,
		exchangeRate:       exchangeRate,
		functionalAmount:   functionalAmount,
		functionalCurrency: fields.functionalCurrency,
		txMoney:            txMoney,
		functionalMoney:    functionalMoney,
	}, nil
}

// AccountID returns the unique identifier of the associated account.
func (e Entry) AccountID() string {
	return e.accountID
}

// TxMoney returns the transaction amount as a currency-aware Money value.
func (e Entry) TxMoney() currency.Money {
	return e.txMoney
}

// TxAmount returns the raw amount in the transaction currency.
// Positive values indicate a debit; negative values indicate a credit.
func (e Entry) TxAmount() decimal.Decimal {
	return e.txAmount
}

// TxCurrency returns the canonical uppercase ISO 4217 transaction currency code.
func (e Entry) TxCurrency() string {
	return e.txCurrency
}

// ExchangeRate returns the rate used to convert TxAmount to FunctionalAmount.
func (e Entry) ExchangeRate() decimal.Decimal {
	return e.exchangeRate
}

// FunctionalMoney returns the functional amount as a currency-aware Money value.
func (e Entry) FunctionalMoney() currency.Money {
	return e.functionalMoney
}

// FunctionalAmount returns the converted amount in the functional currency.
// This value is rounded to the precision defined by the functional currency.
func (e Entry) FunctionalAmount() decimal.Decimal {
	return e.functionalAmount
}

// FunctionalCurrency returns the canonical uppercase ISO 4217 reporting
// currency code.
func (e Entry) FunctionalCurrency() string {
	return e.functionalCurrency
}

// Validate re-validates the entry without modifying it.
//
// A zero-value Entry returns ErrInvalidEntry. Otherwise the stored fields and
// currency-aware Money values are checked against a recomputed functional
// amount. Validation is idempotent and read-only.
func (e Entry) Validate() error {
	if e == (Entry{}) {
		return pkgerr.NewValidationError(pkgerr.ErrInvalidEntry, entryOperationValidate, "entry", nil)
	}

	fields, err := validateEntryFields(
		e.accountID,
		e.txAmount,
		e.txCurrency,
		e.exchangeRate,
		e.functionalCurrency,
		entryOperationValidate,
	)
	if err != nil {
		return err
	}

	expectedAmount, err := currency.Round(
		e.txAmount.Mul(e.exchangeRate),
		currency.Code(fields.functionalCurrency),
		currency.RoundHalfEven,
	)
	if err != nil {
		return rewrapValidationError(err, entryOperationValidate, "functionalAmount")
	}
	if expectedAmount.IsZero() {
		return pkgerr.NewValidationError(
			pkgerr.ErrInvalidEntry,
			entryOperationValidate,
			"functionalAmount",
			nil,
		)
	}
	if !e.functionalAmount.Equal(expectedAmount) {
		return pkgerr.NewValidationError(
			pkgerr.ErrInvalidEntry,
			entryOperationValidate,
			"functionalAmount",
			nil,
		)
	}

	return e.validateStoredValues(fields)
}

func (e Entry) validateStoredValues(fields entryFields) error {
	if e.txCurrency != fields.txCurrency || e.functionalCurrency != fields.functionalCurrency {
		return pkgerr.NewValidationError(pkgerr.ErrInvalidEntry, entryOperationValidate, "entry", nil)
	}
	if !e.txMoney.Amount().Equal(e.txAmount) || e.txMoney.Currency() != currency.Code(fields.txCurrency) {
		return pkgerr.NewValidationError(pkgerr.ErrInvalidEntry, entryOperationValidate, "entry", nil)
	}
	if !e.functionalMoney.Amount().Equal(e.functionalAmount) ||
		e.functionalMoney.Currency() != currency.Code(fields.functionalCurrency) {
		return pkgerr.NewValidationError(
			pkgerr.ErrInvalidEntry,
			entryOperationValidate,
			"functionalAmount",
			nil,
		)
	}
	return nil
}

type entryFields struct {
	txCurrency         string
	functionalCurrency string
}

func validateEntryFields(
	accountID string,
	txAmount decimal.Decimal,
	txCurrency string,
	exchangeRate decimal.Decimal,
	functionalCurrency string,
	operation string,
) (entryFields, error) {
	if !validAccountID(accountID) {
		return entryFields{}, pkgerr.NewValidationError(
			pkgerr.ErrInvalidAccountID,
			operation,
			"accountID",
			nil,
		)
	}

	txCurrencyInfo, err := lookupRatableCurrency(txCurrency, operation, "txCurrency")
	if err != nil {
		return entryFields{}, err
	}
	functionalCurrencyInfo, err := lookupRatableCurrency(functionalCurrency, operation, "functionalCurrency")
	if err != nil {
		return entryFields{}, err
	}

	txMinorUnits, _ := txCurrencyInfo.MinorUnits()
	if txAmount.Exponent() < -txMinorUnits {
		return entryFields{}, pkgerr.NewValidationError(
			pkgerr.ErrInvalidPrecision,
			operation,
			"txAmount",
			nil,
		)
	}
	if txAmount.IsZero() {
		return entryFields{}, pkgerr.NewValidationError(pkgerr.ErrZeroAmount, operation, "txAmount", nil)
	}
	if exchangeRate.Sign() <= 0 {
		return entryFields{}, pkgerr.NewValidationError(
			pkgerr.ErrInvalidExchangeRate,
			operation,
			"exchangeRate",
			nil,
		)
	}

	txCode := string(txCurrencyInfo.Code())
	functionalCode := string(functionalCurrencyInfo.Code())
	if txCode == functionalCode && !exchangeRate.Equal(decimal.NewFromInt(1)) {
		return entryFields{}, pkgerr.NewValidationError(
			pkgerr.ErrInvalidExchangeRate,
			operation,
			"exchangeRate",
			nil,
		)
	}

	return entryFields{
		txCurrency:         txCode,
		functionalCurrency: functionalCode,
	}, nil
}

func lookupRatableCurrency(code string, operation string, field string) (currency.Currency, error) {
	currencyInfo, err := currency.Lookup(code)
	if err != nil {
		return currency.Currency{}, rewrapValidationError(err, operation, field)
	}

	if _, ratable := currencyInfo.MinorUnits(); !ratable {
		return currency.Currency{}, pkgerr.NewValidationError(
			pkgerr.ErrCurrencyNotRatable,
			operation,
			field,
			nil,
		)
	}
	return currencyInfo, nil
}

// SPDX-License-Identifier: MIT

package core

import (
	"nomledger/pkg/currency"

	"github.com/shopspring/decimal"
)

// Entry represents a single line item in a financial transaction.
//
// It enforces a strict signed-amount convention where positive values represent
// debits and negative values represent credits. This convention is uniform across
// all account types. Fields are unexported to ensure the struct remains immutable
// and internally consistent after construction.
type Entry struct {
	accountID string

	txAmount     decimal.Decimal
	txCurrency   string
	exchangeRate decimal.Decimal

	// functionalAmount is the canonical result of RoundBank(txAmount * exchangeRate).
	// It represents the value at Initial Recognition.
	functionalAmount   decimal.Decimal
	functionalCurrency string
}

// NewEntry constructs an immutable Entry and calculates the functional amount immediately.
//
// It applies the exchange rate to the transaction amount and performs canonical
// Banker's Rounding (Half-to-Even) based on the precision of the functional currency.
// This ensures that the functional amount is fixed at the moment of creation.
func NewEntry(
	accountID string,
	txAmount decimal.Decimal,
	txCurrency string,
	exchangeRate decimal.Decimal,
	functionalCurrency string,
) Entry {
	rawAmount := txAmount.Mul(exchangeRate)
	canonicalAmount := currency.RoundToPrecision(rawAmount, functionalCurrency)

	return Entry{
		accountID:          accountID,
		txAmount:           txAmount,
		txCurrency:         txCurrency,
		exchangeRate:       exchangeRate,
		functionalAmount:   canonicalAmount,
		functionalCurrency: functionalCurrency,
	}
}

// AccountID returns the unique identifier of the associated account.
func (e Entry) AccountID() string {
	return e.accountID
}

// TxAmount returns the raw amount in the transaction currency.
// Positive values indicate a debit; negative values indicate a credit.
func (e Entry) TxAmount() decimal.Decimal {
	return e.txAmount
}

// TxCurrency returns the ISO 4217 code of the original transaction amount.
func (e Entry) TxCurrency() string {
	return e.txCurrency
}

// ExchangeRate returns the rate used to convert TxAmount to FunctionalAmount.
func (e Entry) ExchangeRate() decimal.Decimal {
	return e.exchangeRate
}

// FunctionalAmount returns the converted amount in the functional currency.
// This value is rounded to the precision defined by the functional currency.
func (e Entry) FunctionalAmount() decimal.Decimal {
	return e.functionalAmount
}

// FunctionalCurrency returns the ISO 4217 code of the reporting currency.
func (e Entry) FunctionalCurrency() string {
	return e.functionalCurrency
}

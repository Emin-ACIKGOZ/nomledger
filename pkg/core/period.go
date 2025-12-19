// SPDX-License-Identifier: MIT

package core

import (
	"time"

	pkgerr "nomledger/pkg/errors"

	"github.com/shopspring/decimal"
)

// LedgerConfig holds the immutable configuration for a validation batch.
// It acts as the "Closure state definition" for the ledger.
//
// Invariant 3 (Period Closure): The ClosedDate acts as a watermark.
// Transactions with an EffectiveDate <= ClosedDate are rejected unless
// they are explicitly marked as Adjusting Events.
type LedgerConfig struct {
	// ClosedDate is the timestamp up to which the books are finalized.
	ClosedDate time.Time

	// FunctionalCurrency is the base currency for this ledger instance.
	// Used to ensure all entries in a transaction conform to the entity's reporting currency.
	FunctionalCurrency string

	// RateTolerance defines the maximum allowable absolute difference between
	// the provided entry exchange rate and the RateProvider's rate.
	// This value must be explicitly configured (e.g., 0.00001).
	RateTolerance decimal.Decimal
}

// Validate ensures the configuration is well-formed.
// It returns ErrInvalidConfiguration if the RateTolerance is negative or
// if FunctionalCurrency is missing.
func (c LedgerConfig) Validate() error {
	if c.RateTolerance.IsNegative() {
		return pkgerr.ErrInvalidConfiguration
	}
	if c.FunctionalCurrency == "" {
		return pkgerr.ErrInvalidConfiguration
	}
	return nil
}

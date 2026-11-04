// SPDX-License-Identifier: MIT

package core

import (
	"time"

	"github.com/Emin-ACIKGOZ/nomledger/pkg/currency"
	pkgerr "github.com/Emin-ACIKGOZ/nomledger/pkg/errors"

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
//
// FunctionalCurrency must be a valid, ratable, active ISO 4217 currency and
// RateTolerance must be zero or positive. ClosedDate may be zero. The returned
// error is a *ValidationError wrapping ErrInvalidConfiguration.
func (c LedgerConfig) Validate() error {
	currencyInfo, err := currency.Lookup(c.FunctionalCurrency)
	if err != nil {
		return pkgerr.NewValidationError(
			pkgerr.ErrInvalidConfiguration,
			"LedgerConfig.Validate",
			"functionalCurrency",
			err,
		)
	}
	if _, ratable := currencyInfo.MinorUnits(); !ratable {
		return pkgerr.NewValidationError(
			pkgerr.ErrInvalidConfiguration,
			"LedgerConfig.Validate",
			"functionalCurrency",
			pkgerr.ErrCurrencyNotRatable,
		)
	}
	if c.RateTolerance.IsNegative() {
		return pkgerr.NewValidationError(
			pkgerr.ErrInvalidConfiguration,
			"LedgerConfig.Validate",
			"rateTolerance",
			nil,
		)
	}
	return nil
}

// normalizeConfig canonicalises fields that have a unique representation while
// leaving invalid values untouched for Validate to reject.
func normalizeConfig(config LedgerConfig) LedgerConfig {
	if currencyInfo, err := currency.Lookup(config.FunctionalCurrency); err == nil {
		config.FunctionalCurrency = string(currencyInfo.Code())
	}
	if !config.ClosedDate.IsZero() {
		config.ClosedDate = config.ClosedDate.UTC()
	}
	return config
}

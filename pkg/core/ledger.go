// SPDX-License-Identifier: MIT

package core

import (
	"time"

	"nomledger/pkg/interfaces"
)

// Ledger is the concurrency-safe entry point for the validation engine.
//
// It encapsulates the configuration and dependencies required to ensure consistent
// validation rules across all transactions. It serves as a factory for internal
// builders, preventing state leakage between requests.
type Ledger struct {
	config       LedgerConfig
	rateProvider interfaces.RateProvider
}

// NewLedger constructs a new Ledger instance.
//
// The provided configuration and rate provider are captured and used for all
// subsequent validations performed by this instance.
func NewLedger(config LedgerConfig, rp interfaces.RateProvider) *Ledger {
	return &Ledger{
		config:       config,
		rateProvider: rp,
	}
}

// ValidateTransaction constructs and validates a transaction in a single atomic operation.
//
// It is the recommended API for standard usage as it automatically manages the
// lifecycle of the underlying builder and dependencies. It returns a fully validated,
// immutable Transaction or a typed error if validation fails.
func (l *Ledger) ValidateTransaction(
	id string,
	effectiveDate time.Time,
	isAdjustingEvent bool,
	entries ...Entry,
) (Transaction, error) {
	b := NewTxBuilder(id, effectiveDate, l.config, l.rateProvider)

	b.SetAdjustingEvent(isAdjustingEvent)

	for _, e := range entries {
		b.AddEntry(e)
	}

	return b.Build()
}

// UpdateConfig returns a new Ledger instance with updated configuration.
//
// It does not mutate the existing Ledger instance, ensuring that ongoing validations
// are not affected by configuration changes.
func (l *Ledger) UpdateConfig(newConfig LedgerConfig) *Ledger {
	return &Ledger{
		config:       newConfig,
		rateProvider: l.rateProvider,
	}
}

// SPDX-License-Identifier: MIT

package core

import (
	"time"

	pkgerr "github.com/Emin-ACIKGOZ/nomledger/pkg/errors"
	"github.com/Emin-ACIKGOZ/nomledger/pkg/interfaces"
)

// Ledger is the concurrency-safe entry point for the validation engine.
//
// It encapsulates the validated configuration and rate provider required to
// enforce consistent validation rules across all transactions. Fields are
// unexported and immutable after construction; each validation uses a fresh
// internal builder.
type Ledger struct {
	config       LedgerConfig
	rateProvider interfaces.RateProvider
}

// NewLedger constructs a new Ledger instance.
//
// It validates and normalises the configuration before accepting it, and rejects
// a nil rate provider with ErrNilRateProvider. Errors are *ValidationError
// values with Operation "NewLedger" and Field "config" or "provider".
func NewLedger(config LedgerConfig, provider interfaces.RateProvider) (*Ledger, error) {
	if err := config.Validate(); err != nil {
		return nil, rewrapValidationError(err, "NewLedger", "config")
	}
	if provider == nil {
		return nil, pkgerr.NewValidationError(pkgerr.ErrNilRateProvider, "NewLedger", "provider", nil)
	}

	return &Ledger{
		config:       normalizeConfig(config),
		rateProvider: provider,
	}, nil
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

// Config returns a copy of the ledger's normalised configuration.
func (l *Ledger) Config() LedgerConfig {
	return l.config
}

// UpdateConfig returns a new validated Ledger with updated configuration.
//
// It preserves the existing rate provider and never mutates the receiver.
// Errors are *ValidationError values with Operation "UpdateConfig" and Field
// "config".
func (l *Ledger) UpdateConfig(config LedgerConfig) (*Ledger, error) {
	if err := config.Validate(); err != nil {
		return nil, rewrapValidationError(err, "UpdateConfig", "config")
	}

	return &Ledger{
		config:       normalizeConfig(config),
		rateProvider: l.rateProvider,
	}, nil
}

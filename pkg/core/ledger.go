// SPDX-License-Identifier: MIT

package core

import (
	"context"
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
	clock        Clock
}

// NewLedger constructs a new Ledger instance.
//
// It validates and normalizes the configuration before accepting it, applies
// options, defaults a nil clock to time.Now, and rejects a nil rate provider
// with ErrNilRateProvider. Option failures and other errors are *ValidationError
// values with Operation "NewLedger" and Field "clock", "config", or "provider".
func NewLedger(config LedgerConfig, provider interfaces.RateProvider, opts ...LedgerOption) (*Ledger, error) {
	if err := config.Validate(); err != nil {
		return nil, rewrapValidationError(err, "NewLedger", "config")
	}

	options := ledgerOptions{}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(&options); err != nil {
			return nil, rewrapValidationError(err, "NewLedger", "clock")
		}
	}
	clock := options.clock
	if clock == nil {
		clock = ClockFunc(time.Now)
	}

	if provider == nil {
		return nil, pkgerr.NewValidationError(pkgerr.ErrNilRateProvider, "NewLedger", "provider", nil)
	}

	return &Ledger{
		config:       normalizeConfig(config),
		rateProvider: provider,
		clock:        clock,
	}, nil
}

// ValidateTransaction validates a TransactionInput and returns an immutable
// Transaction. It is the canonical validation path.
//
// A nil context is treated as context.Background(). A canceled context returns
// a *ValidationError matching both ErrRateProvider and the context error before
// any validation work, including for same-currency-only transactions. The input
// entries and metadata map are copied, so later mutation of the input cannot
// change the returned transaction. Validation uses the same pipeline as
// TxBuilder.Build and does not mutate the ledger. The ledger's injected clock
// determines PostedAt, which is always stored in UTC.
func (l *Ledger) ValidateTransaction(ctx context.Context, in TransactionInput) (Transaction, error) {
	b := NewTxBuilder(
		in.ID,
		in.EffectiveDate,
		l.config,
		l.rateProvider,
		WithBuilderClock(l.clock),
	)
	b.isAdjustingEvent = in.IsAdjustingEvent
	b.entries = append([]Entry(nil), in.Entries...)
	b.WithMetadata(in.Metadata)
	b.operation = validateTransactionOperation

	return b.build(ctx)
}

// ValidateTransactionLegacy validates a transaction using the pre-1.0 call
// shape. It delegates to ValidateTransaction with context.Background().
//
// Deprecated: use ValidateTransaction.
func (l *Ledger) ValidateTransactionLegacy(
	id string,
	effectiveDate time.Time,
	isAdjustingEvent bool,
	entries ...Entry,
) (Transaction, error) {
	return l.ValidateTransaction(context.Background(), TransactionInput{
		ID:               id,
		EffectiveDate:    effectiveDate,
		IsAdjustingEvent: isAdjustingEvent,
		Entries:          entries,
	})
}

// Config returns a copy of the ledger's normalized configuration.
func (l *Ledger) Config() LedgerConfig {
	return l.config
}

// UpdateConfig returns a new validated Ledger with updated configuration.
//
// It preserves the existing rate provider and clock, and never mutates the
// receiver. Errors are *ValidationError values with Operation "UpdateConfig"
// and Field "config".
func (l *Ledger) UpdateConfig(config LedgerConfig) (*Ledger, error) {
	if err := config.Validate(); err != nil {
		return nil, rewrapValidationError(err, "UpdateConfig", "config")
	}

	return &Ledger{
		config:       normalizeConfig(config),
		rateProvider: l.rateProvider,
		clock:        l.clock,
	}, nil
}

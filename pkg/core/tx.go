// SPDX-License-Identifier: MIT

package core

import (
	"time"

	pkgerr "github.com/Emin-ACIKGOZ/nomledger/pkg/errors" // Alias package errors to avoid collision
	"github.com/Emin-ACIKGOZ/nomledger/pkg/interfaces"

	"github.com/shopspring/decimal"
)

// Transaction represents a persisted, valid financial state.
//
// It is the structurally immutable result of a successful validation process.
// Callers can safely persist this struct or share it across goroutines. Fields
// are unexported to enforce structural immutability.
//
// Entries and the top-level metadata map are copied on build and on every
// accessor call. Metadata values remain caller-owned: if they are reference
// types, the underlying data remains shared.
type Transaction struct {
	id               string
	effectiveDate    time.Time
	isAdjustingEvent bool
	postedAt         time.Time
	entries          []Entry
	metadata         map[string]any
}

// ID returns the unique identifier of the transaction.
func (t Transaction) ID() string { return t.id }

// EffectiveDate returns the date on which the transaction impacts the ledger.
func (t Transaction) EffectiveDate() time.Time { return t.effectiveDate }

// IsAdjustingEvent reports whether the transaction is an IAS 10 adjusting event.
func (t Transaction) IsAdjustingEvent() bool { return t.isAdjustingEvent }

// PostedAt returns the UTC instant at which the transaction was built.
func (t Transaction) PostedAt() time.Time { return t.postedAt }

// Entries returns a defensive copy of the immutable entries associated with this transaction.
// This prevents external mutation of the underlying slice.
func (t Transaction) Entries() []Entry {
	entriesCopy := make([]Entry, len(t.entries))
	copy(entriesCopy, t.entries)
	return entriesCopy
}

// Metadata returns a defensive copy of the transaction's metadata map.
//
// A nil internal map returns a non-nil empty map. The copy is shallow: metadata
// values that are reference types remain caller-owned.
func (t Transaction) Metadata() map[string]any {
	return copyMetadata(t.metadata)
}

// TxBuilder accumulates state for constructing a Transaction.
//
// It is stateful and not safe for concurrent use. A new builder must be
// instantiated for each transaction attempt.
type TxBuilder struct {
	id               string
	effectiveDate    time.Time
	isAdjustingEvent bool
	entries          []Entry
	metadata         map[string]any

	rateProvider interfaces.RateProvider
	config       LedgerConfig
}

// NewTxBuilder initializes a builder with the required validation context.
func NewTxBuilder(
	id string,
	date time.Time,
	config LedgerConfig,
	rp interfaces.RateProvider,
) *TxBuilder {
	return &TxBuilder{
		id:            id,
		effectiveDate: date,
		config:        config,
		rateProvider:  rp,
		entries:       make([]Entry, 0),
		metadata:      make(map[string]any),
	}
}

// AddEntry appends a line item to the transaction.
func (b *TxBuilder) AddEntry(e Entry) *TxBuilder {
	b.entries = append(b.entries, e)
	return b
}

// SetAdjustingEvent marks the transaction as an IAS 10 Adjusting Event.
// If set to true, the transaction may be booked into a closed period.
func (b *TxBuilder) SetAdjustingEvent(isAdjusting bool) *TxBuilder {
	b.isAdjustingEvent = isAdjusting
	return b
}

// WithMetadata adds non-normative data to the transaction.
//
// It performs a shallow copy of the provided map into the builder, so mutating
// the caller's top-level map after this call does not affect the builder.
// Metadata values that are reference types remain caller-owned.
func (b *TxBuilder) WithMetadata(meta map[string]any) *TxBuilder {
	if meta == nil {
		return b
	}
	if b.metadata == nil {
		b.metadata = make(map[string]any, len(meta))
	}
	for key, value := range meta {
		b.metadata[key] = value
	}
	return b
}

// Build validates the accumulated state and returns an immutable Transaction.
//
// It returns a typed error if the state violates zero-sum, period closure,
// exchange rate tolerance, or configuration invariants.
func (b *TxBuilder) Build() (Transaction, error) {
	// 0. Validate Configuration Integrity
	if err := b.config.Validate(); err != nil {
		return Transaction{}, err
	}

	// 1. Validate Consistency (Ledger Currency vs Entry Currency)
	if err := b.validateCurrencyConsistency(); err != nil {
		return Transaction{}, err
	}

	// 2. Validate Zero-Sum (and Check for Empty Entries)
	if err := b.validateZeroSum(); err != nil {
		return Transaction{}, err
	}

	// 3. Validate Period Closure
	if err := b.validatePeriod(); err != nil {
		return Transaction{}, err
	}

	// 4. Validate Rates
	if err := b.validateRates(); err != nil {
		return Transaction{}, err
	}

	return Transaction{
		id:               b.id,
		effectiveDate:    b.effectiveDate,
		isAdjustingEvent: b.isAdjustingEvent,
		postedAt:         time.Now().UTC(),
		entries:          append([]Entry(nil), b.entries...),
		metadata:         copyMetadata(b.metadata),
	}, nil
}

func (b *TxBuilder) validateCurrencyConsistency() error {
	for _, e := range b.entries {
		if e.FunctionalCurrency() != b.config.FunctionalCurrency {
			return pkgerr.ErrCurrencyMismatch
		}
	}
	return nil
}

func (b *TxBuilder) validateZeroSum() error {
	if len(b.entries) == 0 {
		return pkgerr.ErrEmptyTransaction
	}

	sum := decimal.Zero
	for _, e := range b.entries {
		sum = sum.Add(e.FunctionalAmount())
	}

	// Invariant 1: Sum == 0
	if !sum.IsZero() {
		return pkgerr.ErrZeroSumViolation
	}
	return nil
}

func (b *TxBuilder) validatePeriod() error {
	// Invariant 3: EffectiveDate > ClosedDate OR AdjustingEvent
	if b.effectiveDate.After(b.config.ClosedDate) {
		return nil
	}
	if b.isAdjustingEvent {
		return nil
	}
	return pkgerr.ErrPeriodClosed
}

func (b *TxBuilder) validateRates() error {
	tolerance := b.config.RateTolerance

	for _, e := range b.entries {
		// Skip if tx currency is same as functional (rate is 1.0)
		if e.TxCurrency() == e.FunctionalCurrency() {
			continue
		}

		// Invariant 2a: Entry Rate Sanity Check
		// Exchange rates must be strictly positive. Zero is mathematically invalid for conversion.
		if e.ExchangeRate().LessThanOrEqual(decimal.Zero) {
			return pkgerr.ErrInvalidExchangeRate
		}

		// Invariant 2b: Rate Check against Provider
		providerRate, err := b.rateProvider.GetRate(e.TxCurrency(), e.FunctionalCurrency())
		if err != nil {
			return err // Return dependency error verbatim
		}

		// Sanity Check: Provider rates must also be strictly positive.
		if providerRate.LessThanOrEqual(decimal.Zero) {
			return pkgerr.ErrInvalidExchangeRate
		}

		diff := providerRate.Sub(e.ExchangeRate()).Abs()

		// Comparison Semantics: Exclusive limit.
		// A difference exactly equal to tolerance is VALID.
		// A difference greater than tolerance is INVALID.
		if diff.GreaterThan(tolerance) {
			return pkgerr.ErrRateDeviation
		}
	}
	return nil
}

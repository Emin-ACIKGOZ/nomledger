// SPDX-License-Identifier: MIT

package core

import (
	"context"
	"errors"
	"time"

	pkgerr "github.com/Emin-ACIKGOZ/nomledger/pkg/errors" // Alias package errors to avoid collision
	"github.com/Emin-ACIKGOZ/nomledger/pkg/interfaces"

	"github.com/shopspring/decimal"
)

const (
	buildOperation               = "Build"
	validateTransactionOperation = "ValidateTransaction"
	maxMetadataKeyBytes          = 64
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
//
// Transaction IDs and metadata keys are bounded and control-character-free;
// Build enforces those rules. Effective dates are stored in UTC.
type Transaction struct {
	id               string
	effectiveDate    time.Time
	isAdjustingEvent bool
	postedAt         time.Time
	entries          []Entry
	metadata         map[string]any
}

// ID returns the validated transaction identifier.
func (t Transaction) ID() string { return t.id }

// EffectiveDate returns the date on which the transaction impacts the ledger,
// normalised to UTC.
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

// TransactionInput is the canonical value-type input for validation.
//
// Entries and the top-level Metadata map are copied during validation, so the
// caller may safely reuse or mutate the input after the call returns. The copy
// is shallow: metadata values that are reference types remain caller-owned and
// shared.
type TransactionInput struct {
	ID               string
	EffectiveDate    time.Time
	IsAdjustingEvent bool
	Entries          []Entry
	Metadata         map[string]any
}

// TxBuilder accumulates state for constructing a Transaction.
//
// It is stateful and not safe for concurrent use. A new builder must be
// instantiated for each transaction attempt. TxBuilder.Build runs the same
// validation pipeline as Ledger.ValidateTransaction.
type TxBuilder struct {
	id               string
	effectiveDate    time.Time
	isAdjustingEvent bool
	entries          []Entry
	metadata         map[string]any
	operation        string

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
		config:        normalizeConfig(config),
		rateProvider:  rp,
		entries:       make([]Entry, 0),
		metadata:      make(map[string]any),
		operation:     buildOperation,
	}
}

// AddEntry appends a line item to the transaction.
func (b *TxBuilder) AddEntry(e Entry) *TxBuilder {
	b.entries = append(b.entries, e)
	return b
}

// SetAdjustingEvent marks the transaction as an IAS 10 Adjusting Event.
//
// The flag is a caller-supplied classification and only bypasses the period
// closure check; it does not affect any other validation rule.
func (b *TxBuilder) SetAdjustingEvent(isAdjusting bool) *TxBuilder {
	b.isAdjustingEvent = isAdjusting
	return b
}

// WithMetadata adds non-normative data to the transaction.
//
// It performs a shallow copy of the provided map into the builder, so mutating
// the caller's top-level map after this call does not affect the builder.
// Metadata values that are reference types remain caller-owned. Keys
// are validated when Build runs: they must be non-empty, at most 64 bytes, and
// free of NUL and ASCII control characters.
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
// It delegates to the same validation pipeline used by
// Ledger.ValidateTransaction. Validation follows a fixed order: configuration,
// transaction ID, effective date, non-empty entries, metadata keys, entry
// validity, currency consistency, zero-sum balance, period closure, and rate
// integrity. It fails fast, calls no later checks or providers after a failure,
// and returns the zero Transaction on every error. Field-level failures are
// *ValidationError values; provider errors are wrapped with ErrRateProvider
// while preserving their cause.
func (b *TxBuilder) Build() (Transaction, error) {
	return b.build(context.Background())
}

func (b *TxBuilder) build(ctx context.Context) (Transaction, error) {
	if b.operation == "" {
		b.operation = buildOperation
	}
	ctx, err := validationContext(ctx, b.operation)
	if err != nil {
		return Transaction{}, err
	}

	// 0. Validate Configuration Integrity
	if err := b.config.Validate(); err != nil {
		return Transaction{}, rewrapValidationError(err, b.operation, "config")
	}

	// 1. Validate Transaction Fields
	effectiveDate, err := b.validateTransactionFields()
	if err != nil {
		return Transaction{}, err
	}

	// 2. Validate Entries
	if err := b.validateEntries(); err != nil {
		return Transaction{}, err
	}

	// 3. Validate Consistency (Ledger Currency vs Entry Currency)
	if err := b.validateCurrencyConsistency(); err != nil {
		return Transaction{}, err
	}

	// 4. Validate Zero-Sum
	if err := b.validateZeroSum(); err != nil {
		return Transaction{}, err
	}

	// 5. Validate Period Closure
	if err := b.validatePeriod(effectiveDate); err != nil {
		return Transaction{}, err
	}

	// 6. Validate Rates
	if err := validateRates(
		ctx,
		b.entries,
		b.config.RateTolerance,
		b.rateProvider,
		effectiveDate,
		b.operation,
	); err != nil {
		return Transaction{}, err
	}

	return Transaction{
		id:               b.id,
		effectiveDate:    effectiveDate,
		isAdjustingEvent: b.isAdjustingEvent,
		postedAt:         time.Now().UTC(),
		entries:          append([]Entry(nil), b.entries...),
		metadata:         copyMetadata(b.metadata),
	}, nil
}

func validationContext(ctx context.Context, operation string) (context.Context, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, pkgerr.NewValidationError(
			pkgerr.ErrRateProvider,
			operation,
			"context",
			err,
		)
	}
	return ctx, nil
}

func (b *TxBuilder) validateTransactionFields() (time.Time, error) {
	// Transaction IDs share the same bounds and control-character rules as
	// account IDs.
	if !validAccountID(b.id) {
		return time.Time{}, pkgerr.NewValidationError(
			pkgerr.ErrInvalidTransactionID,
			b.operation,
			"id",
			nil,
		)
	}
	if b.effectiveDate.IsZero() {
		return time.Time{}, pkgerr.NewValidationError(
			pkgerr.ErrInvalidEffectiveDate,
			b.operation,
			"effectiveDate",
			nil,
		)
	}

	effectiveDate := b.effectiveDate.UTC()

	if len(b.entries) == 0 {
		return time.Time{}, pkgerr.NewValidationError(
			pkgerr.ErrEmptyTransaction,
			b.operation,
			"entries",
			nil,
		)
	}
	if err := validateMetadataKeys(b.metadata, b.operation); err != nil {
		return time.Time{}, err
	}

	return effectiveDate, nil
}

func validateMetadataKeys(metadata map[string]any, operation string) error {
	for key := range metadata {
		if !validMetadataKey(key) {
			return pkgerr.NewValidationError(
				pkgerr.ErrInvalidMetadata,
				operation,
				"metadata",
				nil,
			)
		}
	}
	return nil
}

func validMetadataKey(key string) bool {
	if key == "" || len(key) > maxMetadataKeyBytes {
		return false
	}
	for i := 0; i < len(key); i++ {
		if key[i] < ' ' || key[i] == asciiDEL {
			return false
		}
	}
	return true
}

func (b *TxBuilder) validateEntries() error {
	for i, entry := range b.entries {
		if err := entry.Validate(); err != nil {
			return rewrapEntryValidationError(err, i, b.operation)
		}
	}
	return nil
}

func rewrapEntryValidationError(err error, entryIndex int, operation string) error {
	var validationErr *pkgerr.ValidationError
	if errors.As(err, &validationErr) {
		return pkgerr.NewEntryValidationError(
			validationErr.Code,
			operation,
			entryIndex,
			validationErr.Field,
			validationErr.Cause,
		)
	}
	return pkgerr.NewEntryValidationError(err, operation, entryIndex, "entry", nil)
}

func (b *TxBuilder) validateCurrencyConsistency() error {
	for i, entry := range b.entries {
		if entry.FunctionalCurrency() != b.config.FunctionalCurrency {
			return pkgerr.NewEntryValidationError(
				pkgerr.ErrCurrencyMismatch,
				b.operation,
				i,
				"functionalCurrency",
				nil,
			)
		}
	}
	return nil
}

func (b *TxBuilder) validateZeroSum() error {
	sum := decimal.Zero
	for _, entry := range b.entries {
		sum = sum.Add(entry.FunctionalAmount())
	}

	// Invariant 1: Sum == 0
	if !sum.IsZero() {
		return pkgerr.NewValidationError(
			pkgerr.ErrZeroSumViolation,
			b.operation,
			"entries",
			nil,
		)
	}
	return nil
}

func (b *TxBuilder) validatePeriod(effectiveDate time.Time) error {
	// Invariant 3: EffectiveDate > ClosedDate OR AdjustingEvent
	if effectiveDate.After(b.config.ClosedDate) {
		return nil
	}
	if b.isAdjustingEvent {
		return nil
	}
	return pkgerr.NewValidationError(
		pkgerr.ErrPeriodClosed,
		b.operation,
		"effectiveDate",
		nil,
	)
}

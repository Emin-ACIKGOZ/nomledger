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
// normalized to UTC.
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
	// ID is the transaction identifier.
	ID string
	// EffectiveDate is the transaction effective date; it is normalized to UTC.
	EffectiveDate time.Time
	// IsAdjustingEvent marks the transaction as an IAS 10 adjusting event.
	IsAdjustingEvent bool
	// Entries are the transaction line items; the slice is copied on validation.
	Entries []Entry
	// Metadata is non-normative data; the map is shallowly copied on validation.
	Metadata map[string]any
}

// TxBuilder accumulates state for constructing a Transaction.
//
// A builder starts accumulating, a failed Build leaves it retryable, and a
// successful Build finalizes it. After finalization, AddEntry, SetAdjustingEvent,
// and WithMetadata are no-ops, and Build returns ErrBuilderFinalized. It is
// stateful and not safe for concurrent use. TxBuilder.Build runs the same
// validation pipeline as Ledger.ValidateTransaction.
type TxBuilder struct {
	id               string
	effectiveDate    time.Time
	isAdjustingEvent bool
	entries          []Entry
	metadata         map[string]any
	operation        string
	finalized        bool
	initErr          error
	clock            Clock

	rateProvider interfaces.RateProvider
	config       LedgerConfig
}

// NewTxBuilder initializes a builder with the required validation context.
//
// Options configure the builder; a nil clock option means time.Now. Option
// errors are stored and returned by Build as a structured error with Field
// "clock".
func NewTxBuilder(
	id string,
	date time.Time,
	config LedgerConfig,
	rp interfaces.RateProvider,
	opts ...BuilderOption,
) *TxBuilder {
	options := builderOptions{}
	var initErr error
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(&options); err != nil {
			initErr = err
			break
		}
	}
	clock := options.clock
	if clock == nil {
		clock = ClockFunc(time.Now)
	}

	return &TxBuilder{
		id:            id,
		effectiveDate: date,
		config:        normalizeConfig(config),
		rateProvider:  rp,
		entries:       make([]Entry, 0),
		metadata:      make(map[string]any),
		operation:     buildOperation,
		initErr:       initErr,
		clock:         clock,
	}
}

// AddEntry appends a line item to the transaction.
//
// It is a no-op after a successful Build.
func (b *TxBuilder) AddEntry(e Entry) *TxBuilder {
	if b.finalized {
		return b
	}
	b.entries = append(b.entries, e)
	return b
}

// SetAdjustingEvent marks the transaction as an IAS 10 Adjusting Event.
//
// The flag is a caller-supplied classification and only bypasses the period
// closure check; it does not affect any other validation rule. It is a no-op
// after a successful Build.
func (b *TxBuilder) SetAdjustingEvent(isAdjusting bool) *TxBuilder {
	if b.finalized {
		return b
	}
	b.isAdjustingEvent = isAdjusting
	return b
}

// WithMetadata adds non-normative data to the transaction.
//
// It performs a shallow copy of the provided map into the builder, so mutating
// the caller's top-level map after this call does not affect the builder.
// Metadata values that are reference types remain caller-owned. Keys
// are validated when Build runs: they must be non-empty, at most 64 bytes, and
// free of NUL and ASCII control characters. It is a no-op after a successful
// Build.
func (b *TxBuilder) WithMetadata(meta map[string]any) *TxBuilder {
	if b.finalized || meta == nil {
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
// Ledger.ValidateTransaction. After builder-option and context checks, validation
// follows a fixed order: configuration, transaction ID, effective date, non-empty
// entries, metadata keys, entry validity, currency consistency, zero-sum balance,
// period closure, and rate integrity. It fails fast, calls no later checks or providers after a failure,
// and returns the zero Transaction on every error. Field-level failures are
// *ValidationError values; provider errors are wrapped with ErrRateProvider
// while preserving their cause.
//
// A failed Build leaves the builder retryable. A successful Build finalizes the
// builder; a subsequent Build returns ErrBuilderFinalized. A builder-option
// error is returned before other validation. PostedAt is read once from the
// builder clock (default time.Now) and stored in UTC.
func (b *TxBuilder) Build() (Transaction, error) {
	operation := b.operation
	if operation == "" {
		operation = buildOperation
	}
	if b.finalized {
		return Transaction{}, pkgerr.NewValidationError(
			pkgerr.ErrBuilderFinalized,
			operation,
			"builder",
			nil,
		)
	}
	return b.build(context.Background())
}

func (b *TxBuilder) build(ctx context.Context) (Transaction, error) {
	if err := b.prepare(); err != nil {
		return Transaction{}, err
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

	transaction := Transaction{
		id:               b.id,
		effectiveDate:    effectiveDate,
		isAdjustingEvent: b.isAdjustingEvent,
		postedAt:         b.now(),
		entries:          append([]Entry(nil), b.entries...),
		metadata:         copyMetadata(b.metadata),
	}
	b.finalized = true
	return transaction, nil
}

func (b *TxBuilder) prepare() error {
	if b.operation == "" {
		b.operation = buildOperation
	}
	if b.initErr != nil {
		return rewrapValidationError(b.initErr, b.operation, "clock")
	}
	return nil
}

func (b *TxBuilder) now() time.Time {
	clock := b.clock
	if clock == nil {
		clock = ClockFunc(time.Now)
	}
	return clock.Now().UTC()
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

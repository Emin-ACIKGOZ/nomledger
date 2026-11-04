// SPDX-License-Identifier: MIT

package core_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/Emin-ACIKGOZ/nomledger/pkg/core"
	pkgerr "github.com/Emin-ACIKGOZ/nomledger/pkg/errors"
	"github.com/Emin-ACIKGOZ/nomledger/pkg/interfaces"

	"github.com/shopspring/decimal"
)

const (
	validateTransactionOperation = "ValidateTransaction"
	fieldContext                 = "context"
)

func validationErrorOf(t *testing.T, err error) *pkgerr.ValidationError {
	t.Helper()
	var validationErr *pkgerr.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("errors.As(err, *ValidationError) = false; err = %v", err)
	}
	return validationErr
}

func assertCanonicalValidationError(
	t *testing.T,
	err error,
	want error,
	field string,
	operation string,
	entryIndex int,
) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("error = %v; want %v", err, want)
	}
	validationErr := validationErrorOf(t, err)
	if validationErr.Operation != operation {
		t.Errorf("Operation = %q; want %q", validationErr.Operation, operation)
	}
	if validationErr.Field != field {
		t.Errorf("Field = %q; want %q", validationErr.Field, field)
	}
	if validationErr.EntryIndex != entryIndex {
		t.Errorf("EntryIndex = %d; want %d", validationErr.EntryIndex, entryIndex)
	}
}

func eurUSDEntries(t *testing.T) []core.Entry {
	t.Helper()
	debit, credit := mustEURUSDEntries(t)
	return []core.Entry{debit, credit}
}

func mustValidLedger(t *testing.T) *core.Ledger {
	t.Helper()
	ledger, err := core.NewLedger(txTestConfig(), mockRateProvider{})
	if err != nil {
		t.Fatalf("NewLedger: %v", err)
	}
	return ledger
}

func validTransactionInput(t *testing.T) core.TransactionInput {
	t.Helper()
	return core.TransactionInput{
		ID:            txID,
		EffectiveDate: ledgerEffectiveDate(),
		Entries:       transactionFieldEntries(t),
	}
}

// TestValidateTransactionCopiesInput.
func TestValidateTransactionCopiesInput(t *testing.T) {
	ledger := mustValidLedger(t)
	t.Run("metadata present", func(t *testing.T) {
		testValidateCopiesMetadata(t, ledger)
	})
	t.Run("nil metadata", func(t *testing.T) {
		testValidateCopiesNilMetadata(t, ledger)
	})
}

func testValidateCopiesMetadata(t *testing.T, ledger *core.Ledger) {
	t.Helper()
	entries := transactionFieldEntries(t)
	expectedEntries := append([]core.Entry(nil), entries...)
	metadata := map[string]any{
		txMetaNote:  txMetaValue,
		txMetaCount: 1,
	}
	input := core.TransactionInput{
		ID:            txID,
		EffectiveDate: ledgerEffectiveDate(),
		Entries:       entries,
		Metadata:      metadata,
	}

	transaction, err := ledger.ValidateTransaction(context.Background(), input)
	if err != nil {
		t.Fatalf("ValidateTransaction: %v", err)
	}

	if len(input.Entries) != len(expectedEntries) {
		t.Fatalf("input Entries length = %d; want %d", len(input.Entries), len(expectedEntries))
	}
	if input.Entries[0].AccountID() != expectedEntries[0].AccountID() {
		t.Errorf("input Entries[0] = %q; want %q", input.Entries[0].AccountID(), expectedEntries[0].AccountID())
	}
	if len(input.Metadata) != 2 {
		t.Errorf("input Metadata length = %d; want 2", len(input.Metadata))
	}

	input.Entries[0] = core.Entry{}
	input.Entries = append(input.Entries, mustTxEntry(t, txExtraAccount, txAmountOne))
	input.Metadata[txMetaNote] = txMetaChanged
	delete(input.Metadata, txMetaCount)
	input.Metadata["added"] = true

	assertEntriesEqual(t, transaction.Entries(), expectedEntries)
	stored := transaction.Metadata()
	if stored[txMetaNote] != txMetaValue {
		t.Errorf("Metadata()[note] = %v; want %v", stored[txMetaNote], txMetaValue)
	}
	if _, ok := stored[txMetaCount]; !ok {
		t.Errorf("Metadata() lost key %q after input mutation", txMetaCount)
	}
	if _, ok := stored["added"]; ok {
		t.Error("Metadata() gained key \"added\" after input mutation")
	}
}

func testValidateCopiesNilMetadata(t *testing.T, ledger *core.Ledger) {
	t.Helper()
	input := validTransactionInput(t)
	transaction, err := ledger.ValidateTransaction(context.Background(), input)
	if err != nil {
		t.Fatalf("ValidateTransaction: %v", err)
	}
	metadata := transaction.Metadata()
	if metadata == nil {
		t.Fatal("Metadata() = nil; want non-nil empty map")
	}
	if len(metadata) != 0 {
		t.Errorf("Metadata() length = %d; want 0", len(metadata))
	}

	input.Metadata = map[string]any{"later": true}
	if _, ok := transaction.Metadata()["later"]; ok {
		t.Error("assigning input metadata after validation changed the transaction")
	}
}

// TestValidateTransactionContext.
func TestValidateTransactionContext(t *testing.T) {
	ledger := mustValidLedger(t)
	input := validTransactionInput(t)

	t.Run("nil context succeeds", func(t *testing.T) {
		transaction, err := ledger.ValidateTransaction(nil, input) //nolint:staticcheck // T-063 requires nil-context handling
		if err != nil {
			t.Fatalf("ValidateTransaction(nil): %v", err)
		}
		if len(transaction.Entries()) != 2 {
			t.Errorf("Entries() length = %d; want 2", len(transaction.Entries()))
		}
	})

	t.Run("cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		transaction, err := ledger.ValidateTransaction(ctx, input)
		if !errors.Is(err, pkgerr.ErrRateProvider) {
			t.Fatalf("error = %v; want ErrRateProvider", err)
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("error = %v; want context.Canceled", err)
		}
		assertCanonicalValidationError(t, err, pkgerr.ErrRateProvider,
			fieldContext, validateTransactionOperation, -1)
		if !reflect.DeepEqual(transaction, core.Transaction{}) {
			t.Errorf("ValidateTransaction returned non-zero Transaction: %+v", transaction)
		}
	})

	t.Run("deadline exceeded context", func(t *testing.T) {
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
		defer cancel()
		transaction, err := ledger.ValidateTransaction(ctx, input)
		if !errors.Is(err, pkgerr.ErrRateProvider) {
			t.Fatalf("error = %v; want ErrRateProvider", err)
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("error = %v; want context.DeadlineExceeded", err)
		}
		assertCanonicalValidationError(t, err, pkgerr.ErrRateProvider,
			fieldContext, validateTransactionOperation, -1)
		if !reflect.DeepEqual(transaction, core.Transaction{}) {
			t.Errorf("ValidateTransaction returned non-zero Transaction: %+v", transaction)
		}
	})
}

type canonicalParityCase struct {
	name     string
	config   core.LedgerConfig
	provider interfaces.RateProvider
	input    core.TransactionInput
	wantErr  error
}

// TestCanonicalPathParity.
func TestCanonicalPathParity(t *testing.T) {
	sentinel := errors.New("provider failure")
	validDate := ledgerEffectiveDate()

	cases := []canonicalParityCase{
		{
			name:     "transaction ID",
			config:   txTestConfig(),
			provider: mockRateProvider{},
			input: core.TransactionInput{
				ID:            "",
				EffectiveDate: validDate,
				Entries:       transactionFieldEntries(t),
			},
			wantErr: pkgerr.ErrInvalidTransactionID,
		},
		{
			name:     "effective date",
			config:   txTestConfig(),
			provider: mockRateProvider{},
			input: core.TransactionInput{
				ID:      txID,
				Entries: transactionFieldEntries(t),
			},
			wantErr: pkgerr.ErrInvalidEffectiveDate,
		},
		{
			name:     "empty entries",
			config:   txTestConfig(),
			provider: mockRateProvider{},
			input: core.TransactionInput{
				ID:            txID,
				EffectiveDate: validDate,
			},
			wantErr: pkgerr.ErrEmptyTransaction,
		},
		{
			name:     "invalid metadata",
			config:   txTestConfig(),
			provider: mockRateProvider{},
			input: core.TransactionInput{
				ID:            txID,
				EffectiveDate: validDate,
				Entries:       transactionFieldEntries(t),
				Metadata:      map[string]any{"": true},
			},
			wantErr: pkgerr.ErrInvalidMetadata,
		},
		{
			name:     "zero-value entry",
			config:   txTestConfig(),
			provider: mockRateProvider{},
			input: core.TransactionInput{
				ID:            txID,
				EffectiveDate: validDate,
				Entries:       []core.Entry{{}},
			},
			wantErr: pkgerr.ErrInvalidEntry,
		},
		{
			name:     "currency mismatch",
			config:   txTestConfig(),
			provider: mockRateProvider{},
			input: core.TransactionInput{
				ID:            txID,
				EffectiveDate: validDate,
				Entries:       []core.Entry{mustRateEntry(t, "eur-acc", txAmountHundred, codeEUR, txRateOne, codeEUR)},
			},
			wantErr: pkgerr.ErrCurrencyMismatch,
		},
		{
			name:     "zero-sum imbalance",
			config:   txTestConfig(),
			provider: mockRateProvider{},
			input: core.TransactionInput{
				ID:            txID,
				EffectiveDate: validDate,
				Entries:       unbalancedEntries(t),
			},
			wantErr: pkgerr.ErrZeroSumViolation,
		},
		{
			name:     "period closed",
			config:   periodClosedConfig(),
			provider: mockRateProvider{},
			input: core.TransactionInput{
				ID:            txID,
				EffectiveDate: validDate,
				Entries:       transactionFieldEntries(t),
			},
			wantErr: pkgerr.ErrPeriodClosed,
		},
		{
			name:   "rate deviation",
			config: txTestConfig(),
			provider: &recordingRateProvider{rates: map[string]decimal.Decimal{
				pairEURUSD: mustTxDecimal(t, "1.5"),
			}},
			input: core.TransactionInput{
				ID:            txID,
				EffectiveDate: validDate,
				Entries:       eurUSDEntries(t),
			},
			wantErr: pkgerr.ErrRateDeviation,
		},
		{
			name:     "provider error",
			config:   txTestConfig(),
			provider: &recordingRateProvider{err: sentinel},
			input: core.TransactionInput{
				ID:            txID,
				EffectiveDate: validDate,
				Entries:       eurUSDEntries(t),
			},
			wantErr: pkgerr.ErrRateProvider,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertCanonicalParity(t, tc)
		})
	}
}

func assertCanonicalParity(t *testing.T, tc canonicalParityCase) {
	t.Helper()
	canonicalValidationErr := assertCanonicalFailure(t, tc)
	buildValidationErr := assertBuildFailure(t, tc)
	if canonicalValidationErr.EntryIndex != buildValidationErr.EntryIndex {
		t.Errorf("EntryIndex mismatch: canonical=%d build=%d",
			canonicalValidationErr.EntryIndex, buildValidationErr.EntryIndex)
	}
}

func assertCanonicalFailure(t *testing.T, tc canonicalParityCase) *pkgerr.ValidationError {
	t.Helper()
	ledger, err := core.NewLedger(tc.config, tc.provider)
	if err != nil {
		t.Fatalf("NewLedger: %v", err)
	}
	transaction, err := ledger.ValidateTransaction(context.Background(), tc.input)
	if err == nil {
		t.Fatal("ValidateTransaction returned nil error")
	}
	if !errors.Is(err, tc.wantErr) {
		t.Errorf("canonical error = %v; want %v", err, tc.wantErr)
	}
	if !reflect.DeepEqual(transaction, core.Transaction{}) {
		t.Errorf("canonical path returned non-zero Transaction: %+v", transaction)
	}
	return validationErrorOf(t, err)
}

func assertBuildFailure(t *testing.T, tc canonicalParityCase) *pkgerr.ValidationError {
	t.Helper()
	builder := core.NewTxBuilder(tc.input.ID, tc.input.EffectiveDate, tc.config, tc.provider)
	builder.SetAdjustingEvent(tc.input.IsAdjustingEvent)
	for _, entry := range tc.input.Entries {
		builder.AddEntry(entry)
	}
	if tc.input.Metadata != nil {
		builder.WithMetadata(tc.input.Metadata)
	}
	transaction, err := builder.Build()
	if err == nil {
		t.Fatal("Build returned nil error")
	}
	if !errors.Is(err, tc.wantErr) {
		t.Errorf("Build error = %v; want %v", err, tc.wantErr)
	}
	if !reflect.DeepEqual(transaction, core.Transaction{}) {
		t.Errorf("Build returned non-zero Transaction: %+v", transaction)
	}
	return validationErrorOf(t, err)
}

// TestValidateTransactionLegacy verifies the deprecated call shape still works.
func TestValidateTransactionLegacy(t *testing.T) {
	ledger := mustValidLedger(t)
	entries := transactionFieldEntries(t)
	effective := ledgerEffectiveDate()

	canonical, err := ledger.ValidateTransaction(context.Background(), core.TransactionInput{
		ID:            txID,
		EffectiveDate: effective,
		Entries:       entries,
	})
	if err != nil {
		t.Fatalf("ValidateTransaction: %v", err)
	}

	legacy, err := ledger.ValidateTransactionLegacy(txID, effective, false, entries...)
	if err != nil {
		t.Fatalf("ValidateTransactionLegacy: %v", err)
	}

	if legacy.ID() != canonical.ID() {
		t.Errorf("legacy ID = %q; want %q", legacy.ID(), canonical.ID())
	}
	if !legacy.EffectiveDate().Equal(canonical.EffectiveDate()) {
		t.Errorf("legacy EffectiveDate = %s; want %s", legacy.EffectiveDate(), canonical.EffectiveDate())
	}
	if legacy.IsAdjustingEvent() != canonical.IsAdjustingEvent() {
		t.Errorf("legacy IsAdjustingEvent = %t; want %t",
			legacy.IsAdjustingEvent(), canonical.IsAdjustingEvent())
	}
	assertEntriesEqual(t, legacy.Entries(), canonical.Entries())
}

// TestValidateTransactionOperation checks operation attribution between the
// canonical API and the builder.
func TestValidateTransactionOperation(t *testing.T) {
	ledger := mustValidLedger(t)
	input := core.TransactionInput{
		ID:            "",
		EffectiveDate: ledgerEffectiveDate(),
		Entries:       transactionFieldEntries(t),
	}

	_, err := ledger.ValidateTransaction(context.Background(), input)
	assertCanonicalValidationError(t, err, pkgerr.ErrInvalidTransactionID,
		"id", validateTransactionOperation, -1)

	builder := core.NewTxBuilder(input.ID, input.EffectiveDate, txTestConfig(), mockRateProvider{})
	for _, entry := range input.Entries {
		builder.AddEntry(entry)
	}
	_, err = builder.Build()
	assertBuildValidationError(t, err, pkgerr.ErrInvalidTransactionID, "id", -1)
}

// TestRateErrorOperation checks rate-error operation attribution on both paths.
func TestRateErrorOperation(t *testing.T) {
	sentinel := errors.New("provider failure")
	cases := []struct {
		name     string
		provider interfaces.RateProvider
		wantErr  error
	}{
		{
			name:     "provider error",
			provider: &recordingRateProvider{err: sentinel},
			wantErr:  pkgerr.ErrRateProvider,
		},
		{
			name: "rate deviation",
			provider: &recordingRateProvider{rates: map[string]decimal.Decimal{
				pairEURUSD: mustTxDecimal(t, "1.5"),
			}},
			wantErr: pkgerr.ErrRateDeviation,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := core.TransactionInput{
				ID:            txID,
				EffectiveDate: ledgerEffectiveDate(),
				Entries:       eurUSDEntries(t),
			}
			ledger, err := core.NewLedger(txTestConfig(), tc.provider)
			if err != nil {
				t.Fatalf("NewLedger: %v", err)
			}

			_, err = ledger.ValidateTransaction(context.Background(), input)
			assertCanonicalValidationError(t, err, tc.wantErr,
				rateFieldRate, validateTransactionOperation, 0)

			builder := core.NewTxBuilder(input.ID, input.EffectiveDate, txTestConfig(), tc.provider)
			for _, entry := range input.Entries {
				builder.AddEntry(entry)
			}
			_, err = builder.Build()
			assertBuildValidationError(t, err, tc.wantErr, rateFieldRate, 0)
		})
	}
}

// TestValidateTransactionLegacyOperation verifies legacy failures use the
// canonical operation name.
func TestValidateTransactionLegacyOperation(t *testing.T) {
	ledger := mustValidLedger(t)
	_, err := ledger.ValidateTransactionLegacy("", ledgerEffectiveDate(), false, transactionFieldEntries(t)...)
	assertCanonicalValidationError(t, err, pkgerr.ErrInvalidTransactionID,
		fieldID, validateTransactionOperation, -1)
}

// TestCancellationPrecedence proves context cancellation is checked before any
// input validation.
func TestCancellationPrecedence(t *testing.T) {
	ledger := mustValidLedger(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	transaction, err := ledger.ValidateTransaction(ctx, core.TransactionInput{})
	if !errors.Is(err, pkgerr.ErrRateProvider) {
		t.Fatalf("error = %v; want ErrRateProvider", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v; want context.Canceled", err)
	}
	assertCanonicalValidationError(t, err, pkgerr.ErrRateProvider,
		fieldContext, validateTransactionOperation, -1)
	if !reflect.DeepEqual(transaction, core.Transaction{}) {
		t.Errorf("ValidateTransaction returned non-zero Transaction: %+v", transaction)
	}
}

// TestZeroValueTxBuilderOperation verifies a zero-value builder defaults its
// operation name to Build.
func TestZeroValueTxBuilderOperation(t *testing.T) {
	var builder core.TxBuilder
	transaction, err := builder.Build()
	if err == nil {
		t.Fatal("Build returned nil error")
	}
	assertZeroTransaction(t, transaction)
	assertBuildValidationError(t, err, pkgerr.ErrInvalidConfiguration, "config", -1)
}

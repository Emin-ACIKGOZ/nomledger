// SPDX-License-Identifier: MIT

package core_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Emin-ACIKGOZ/nomledger/pkg/core"
	pkgerr "github.com/Emin-ACIKGOZ/nomledger/pkg/errors"
)

const builderFinalizedField = "builder"

func TestBuilderFinalization(t *testing.T) {
	t.Run("success finalizes the builder", testBuilderSuccessFinalized)
	t.Run("failed build is retryable", testBuilderFailedBuildRetryable)
	t.Run("repeated failures keep the original error", testBuilderRepeatedFailures)
	t.Run("ledger path is never finalized", testLedgerPathNeverFinalized)
}

func testBuilderSuccessFinalized(t *testing.T) {
	t.Helper()
	builder := core.NewTxBuilder(txID, ledgerEffectiveDate(), txTestConfig(), mockRateProvider{})
	for _, entry := range transactionFieldEntries(t) {
		builder.AddEntry(entry)
	}

	transaction, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	builder.AddEntry(mustTxEntry(t, txExtraAccount, txAmountOne))
	builder.WithMetadata(map[string]any{txMetaBuilder: txMetaChanged})
	builder.SetAdjustingEvent(true)

	if len(transaction.Entries()) != 2 {
		t.Errorf("original Entries() length = %d; want 2", len(transaction.Entries()))
	}
	if transaction.IsAdjustingEvent() {
		t.Error("post-Build SetAdjustingEvent changed the transaction")
	}
	if _, ok := transaction.Metadata()[txMetaBuilder]; ok {
		t.Error("post-Build WithMetadata changed the transaction")
	}

	finalizedTx, err := builder.Build()
	assertZeroTransaction(t, finalizedTx)
	assertBuildValidationError(t, err, pkgerr.ErrBuilderFinalized, builderFinalizedField, -1)

	if errors.Is(err, pkgerr.ErrZeroSumViolation) {
		t.Error("ErrBuilderFinalized matched ErrZeroSumViolation")
	}
	if errors.Is(err, pkgerr.ErrInvalidTransactionID) {
		t.Error("ErrBuilderFinalized matched ErrInvalidTransactionID")
	}
}

func testBuilderFailedBuildRetryable(t *testing.T) {
	t.Helper()
	builder := core.NewTxBuilder(txID, ledgerEffectiveDate(), txTestConfig(), mockRateProvider{})
	for _, entry := range unbalancedEntries(t) {
		builder.AddEntry(entry)
	}

	if _, err := builder.Build(); !errors.Is(err, pkgerr.ErrZeroSumViolation) {
		t.Fatalf("first Build error = %v; want ErrZeroSumViolation", err)
	}

	builder.AddEntry(mustTxEntry(t, txExtraAccount, "-1.00"))
	transaction, err := builder.Build()
	if err != nil {
		t.Fatalf("retry Build: %v", err)
	}
	if len(transaction.Entries()) != 3 {
		t.Errorf("retry Entries() length = %d; want 3", len(transaction.Entries()))
	}

	finalizedTx, err := builder.Build()
	assertZeroTransaction(t, finalizedTx)
	assertBuildValidationError(t, err, pkgerr.ErrBuilderFinalized, builderFinalizedField, -1)
}

func testBuilderRepeatedFailures(t *testing.T) {
	t.Helper()
	builder := core.NewTxBuilder(txID, ledgerEffectiveDate(), txTestConfig(), mockRateProvider{})
	for _, entry := range unbalancedEntries(t) {
		builder.AddEntry(entry)
	}

	first, err := builder.Build()
	if !errors.Is(err, pkgerr.ErrZeroSumViolation) {
		t.Fatalf("first Build error = %v; want ErrZeroSumViolation", err)
	}
	assertZeroTransaction(t, first)

	second, err := builder.Build()
	if !errors.Is(err, pkgerr.ErrZeroSumViolation) {
		t.Errorf("second Build error = %v; want ErrZeroSumViolation", err)
	}
	if errors.Is(err, pkgerr.ErrBuilderFinalized) {
		t.Error("failed Build finalized the builder")
	}
	assertZeroTransaction(t, second)
}

func testLedgerPathNeverFinalized(t *testing.T) {
	t.Helper()
	ledger := mustValidLedger(t)
	input := validTransactionInput(t)

	for i := 0; i < 3; i++ {
		transaction, err := ledger.ValidateTransaction(context.Background(), input)
		if err != nil {
			t.Fatalf("ValidateTransaction call %d: %v", i, err)
		}
		if len(transaction.Entries()) != 2 {
			t.Errorf("call %d Entries() length = %d; want 2", i, len(transaction.Entries()))
		}
	}
}

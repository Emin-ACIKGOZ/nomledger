// SPDX-License-Identifier: MIT

package core_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/Emin-ACIKGOZ/nomledger/pkg/core"
)

func fixedUTC() time.Time {
	return time.Date(2026, 11, 4, 12, 34, 56, 123456789, time.UTC)
}

func mustLedgerWithClock(t *testing.T, clock core.Clock) *core.Ledger {
	t.Helper()
	ledger, err := core.NewLedger(txTestConfig(), mockRateProvider{}, core.WithClock(clock))
	if err != nil {
		t.Fatalf("NewLedger: %v", err)
	}
	return ledger
}

// TestFixedClockPostedAt.
func TestFixedClockPostedAt(t *testing.T) {
	t.Run("ledger fixed clock", testLedgerFixedClock)
	t.Run("builder fixed clock", testBuilderFixedClock)
	t.Run("repeatable transactions", testFixedClockRepeatable)
	t.Run("non-UTC clock normalized", testNonUTCFixedClock)
	t.Run("non-UTC builder clock normalized", testNonUTCBuilderFixedClock)
	t.Run("default ledger clock", testDefaultLedgerClock)
	t.Run("default builder clock", testDefaultBuilderClock)
	t.Run("nil clock options use default", testNilClockOptions)
	t.Run("typed nil clocks use default", testTypedNilClocks)
	t.Run("legacy uses ledger clock", testLegacyUsesLedgerClock)
}

func testLedgerFixedClock(t *testing.T) {
	t.Helper()
	expected := fixedUTC()
	ledger := mustLedgerWithClock(t, core.ClockFunc(func() time.Time { return expected }))

	transaction, err := ledger.ValidateTransaction(context.Background(), validTransactionInput(t))
	if err != nil {
		t.Fatalf("ValidateTransaction: %v", err)
	}
	if !transaction.PostedAt().Equal(expected) {
		t.Errorf("PostedAt() = %s; want %s", transaction.PostedAt(), expected)
	}
	if transaction.PostedAt().Location() != time.UTC {
		t.Errorf("PostedAt().Location() = %v; want UTC", transaction.PostedAt().Location())
	}
}

func testBuilderFixedClock(t *testing.T) {
	t.Helper()
	expected := fixedUTC()
	builder := core.NewTxBuilder(txID, ledgerEffectiveDate(), txTestConfig(), mockRateProvider{},
		core.WithBuilderClock(core.ClockFunc(func() time.Time { return expected })))
	for _, entry := range transactionFieldEntries(t) {
		builder.AddEntry(entry)
	}
	transaction, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if !transaction.PostedAt().Equal(expected) {
		t.Errorf("PostedAt() = %s; want %s", transaction.PostedAt(), expected)
	}
	if transaction.PostedAt().Location() != time.UTC {
		t.Errorf("PostedAt().Location() = %v; want UTC", transaction.PostedAt().Location())
	}
}

func testFixedClockRepeatable(t *testing.T) {
	t.Helper()
	expected := fixedUTC()
	ledger := mustLedgerWithClock(t, core.ClockFunc(func() time.Time { return expected }))
	input := validTransactionInput(t)

	first, err := ledger.ValidateTransaction(context.Background(), input)
	if err != nil {
		t.Fatalf("first ValidateTransaction: %v", err)
	}
	second, err := ledger.ValidateTransaction(context.Background(), input)
	if err != nil {
		t.Fatalf("second ValidateTransaction: %v", err)
	}
	if !first.PostedAt().Equal(expected) || !second.PostedAt().Equal(expected) {
		t.Errorf("PostedAt() = %s / %s; want %s", first.PostedAt(), second.PostedAt(), expected)
	}
	if !reflect.DeepEqual(first, second) {
		t.Errorf("transactions differ with a fixed clock:\nfirst=%+v\nsecond=%+v", first, second)
	}
}

func testNonUTCFixedClock(t *testing.T) {
	t.Helper()
	zone := time.FixedZone("EST", -5*60*60)
	zoned := time.Date(2026, 11, 4, 7, 34, 56, 123456789, zone)
	ledger := mustLedgerWithClock(t, core.ClockFunc(func() time.Time { return zoned }))

	transaction, err := ledger.ValidateTransaction(context.Background(), validTransactionInput(t))
	if err != nil {
		t.Fatalf("ValidateTransaction: %v", err)
	}
	got := transaction.PostedAt()
	if !got.Equal(zoned) {
		t.Errorf("PostedAt() instant = %s; want %s", got, zoned)
	}
	if got.Location() != time.UTC {
		t.Errorf("PostedAt().Location() = %v; want UTC", got.Location())
	}
	if got.Nanosecond() != zoned.Nanosecond() {
		t.Errorf("PostedAt().Nanosecond() = %d; want %d", got.Nanosecond(), zoned.Nanosecond())
	}
}

func testNonUTCBuilderFixedClock(t *testing.T) {
	t.Helper()
	zone := time.FixedZone("EST", -5*60*60)
	zoned := time.Date(2026, 11, 4, 7, 34, 56, 123456789, zone)
	builder := core.NewTxBuilder(txID, ledgerEffectiveDate(), txTestConfig(), mockRateProvider{},
		core.WithBuilderClock(core.ClockFunc(func() time.Time { return zoned })))
	for _, entry := range transactionFieldEntries(t) {
		builder.AddEntry(entry)
	}
	transaction, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	got := transaction.PostedAt()
	if !got.Equal(zoned) {
		t.Errorf("PostedAt() instant = %s; want %s", got, zoned)
	}
	if got.Location() != time.UTC {
		t.Errorf("PostedAt().Location() = %v; want UTC", got.Location())
	}
	if got.Nanosecond() != zoned.Nanosecond() {
		t.Errorf("PostedAt().Nanosecond() = %d; want %d", got.Nanosecond(), zoned.Nanosecond())
	}
}

func testDefaultLedgerClock(t *testing.T) {
	t.Helper()
	ledger, err := core.NewLedger(txTestConfig(), mockRateProvider{})
	if err != nil {
		t.Fatalf("NewLedger: %v", err)
	}
	transaction, err := ledger.ValidateTransaction(context.Background(), validTransactionInput(t))
	if err != nil {
		t.Fatalf("ValidateTransaction: %v", err)
	}
	if transaction.PostedAt().IsZero() {
		t.Error("PostedAt() = zero; want non-zero")
	}
	if transaction.PostedAt().Location() != time.UTC {
		t.Errorf("PostedAt().Location() = %v; want UTC", transaction.PostedAt().Location())
	}
}

func testDefaultBuilderClock(t *testing.T) {
	t.Helper()
	builder := core.NewTxBuilder(txID, ledgerEffectiveDate(), txTestConfig(), mockRateProvider{})
	for _, entry := range transactionFieldEntries(t) {
		builder.AddEntry(entry)
	}
	transaction, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if transaction.PostedAt().IsZero() {
		t.Error("PostedAt() = zero; want non-zero")
	}
	if transaction.PostedAt().Location() != time.UTC {
		t.Errorf("PostedAt().Location() = %v; want UTC", transaction.PostedAt().Location())
	}
}

func testNilClockOptions(t *testing.T) {
	t.Helper()
	ledger, err := core.NewLedger(txTestConfig(), mockRateProvider{}, core.WithClock(nil))
	if err != nil {
		t.Fatalf("NewLedger(WithClock(nil)): %v", err)
	}
	transaction, err := ledger.ValidateTransaction(context.Background(), validTransactionInput(t))
	if err != nil {
		t.Fatalf("ValidateTransaction: %v", err)
	}
	if transaction.PostedAt().IsZero() {
		t.Error("ledger nil-clock PostedAt() = zero; want non-zero")
	}

	builder := core.NewTxBuilder(txID, ledgerEffectiveDate(), txTestConfig(), mockRateProvider{},
		core.WithBuilderClock(nil))
	for _, entry := range transactionFieldEntries(t) {
		builder.AddEntry(entry)
	}
	transaction, err = builder.Build()
	if err != nil {
		t.Fatalf("Build(WithBuilderClock(nil)): %v", err)
	}
	if transaction.PostedAt().IsZero() {
		t.Error("builder nil-clock PostedAt() = zero; want non-zero")
	}
	if transaction.PostedAt().Location() != time.UTC {
		t.Errorf("builder nil-clock PostedAt().Location() = %v; want UTC", transaction.PostedAt().Location())
	}
}

func testTypedNilClocks(t *testing.T) {
	t.Helper()
	ledger, err := core.NewLedger(txTestConfig(), mockRateProvider{}, core.WithClock(core.ClockFunc(nil)))
	if err != nil {
		t.Fatalf("NewLedger(ClockFunc(nil)): %v", err)
	}
	transaction, err := ledger.ValidateTransaction(context.Background(), validTransactionInput(t))
	if err != nil {
		t.Fatalf("ValidateTransaction: %v", err)
	}
	if transaction.PostedAt().IsZero() {
		t.Error("typed-nil ledger PostedAt() = zero; want default non-zero")
	}
	if transaction.PostedAt().Location() != time.UTC {
		t.Errorf("typed-nil ledger PostedAt().Location() = %v; want UTC", transaction.PostedAt().Location())
	}

	builder := core.NewTxBuilder(txID, ledgerEffectiveDate(), txTestConfig(), mockRateProvider{},
		core.WithBuilderClock(core.ClockFunc(nil)))
	for _, entry := range transactionFieldEntries(t) {
		builder.AddEntry(entry)
	}
	transaction, err = builder.Build()
	if err != nil {
		t.Fatalf("Build(ClockFunc(nil)): %v", err)
	}
	if transaction.PostedAt().IsZero() {
		t.Error("typed-nil builder PostedAt() = zero; want default non-zero")
	}
	if transaction.PostedAt().Location() != time.UTC {
		t.Errorf("typed-nil builder PostedAt().Location() = %v; want UTC", transaction.PostedAt().Location())
	}
}

func testLegacyUsesLedgerClock(t *testing.T) {
	t.Helper()
	expected := fixedUTC()
	ledger := mustLedgerWithClock(t, core.ClockFunc(func() time.Time { return expected }))

	canonical, err := ledger.ValidateTransaction(context.Background(), validTransactionInput(t))
	if err != nil {
		t.Fatalf("ValidateTransaction: %v", err)
	}
	legacy, err := ledger.ValidateTransactionLegacy(txID, ledgerEffectiveDate(), false, transactionFieldEntries(t)...)
	if err != nil {
		t.Fatalf("ValidateTransactionLegacy: %v", err)
	}
	if !legacy.PostedAt().Equal(expected) {
		t.Errorf("legacy PostedAt() = %s; want %s", legacy.PostedAt(), expected)
	}
	if !legacy.PostedAt().Equal(canonical.PostedAt()) {
		t.Errorf("legacy PostedAt() = %s; want canonical %s", legacy.PostedAt(), canonical.PostedAt())
	}
}

// TestClockFunc verifies the adapter implements Clock and works through
// WithClock.
func TestClockFunc(t *testing.T) {
	var _ core.Clock = core.ClockFunc(nil)
	expected := fixedUTC()
	clock := core.ClockFunc(func() time.Time { return expected })
	if !clock.Now().Equal(expected) {
		t.Errorf("ClockFunc.Now() = %s; want %s", clock.Now(), expected)
	}

	ledger := mustLedgerWithClock(t, clock)
	transaction, err := ledger.ValidateTransaction(context.Background(), validTransactionInput(t))
	if err != nil {
		t.Fatalf("ValidateTransaction: %v", err)
	}
	if !transaction.PostedAt().Equal(expected) {
		t.Errorf("PostedAt() = %s; want %s", transaction.PostedAt(), expected)
	}
}

// TestValidationDeterminism.
func TestValidationDeterminism(t *testing.T) {
	expected := fixedUTC()
	ledger := mustLedgerWithClock(t, core.ClockFunc(func() time.Time { return expected }))
	input := validTransactionInput(t)

	start := time.Now()
	first, err := ledger.ValidateTransaction(context.Background(), input)
	if err != nil {
		t.Fatalf("first ValidateTransaction: %v", err)
	}
	for i := 1; i < 10000; i++ {
		transaction, err := ledger.ValidateTransaction(context.Background(), input)
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		if !reflect.DeepEqual(transaction, first) {
			t.Fatalf("iteration %d produced a different transaction", i)
		}
	}
	t.Logf("10000 deterministic validations in %s", time.Since(start))
}

// TestUpdateConfigPreservesClock verifies both ledgers keep the fixed clock.
func TestUpdateConfigPreservesClock(t *testing.T) {
	expected := fixedUTC()
	ledger := mustLedgerWithClock(t, core.ClockFunc(func() time.Time { return expected }))

	updated, err := ledger.UpdateConfig(core.LedgerConfig{FunctionalCurrency: codeEUR})
	if err != nil {
		t.Fatalf("UpdateConfig: %v", err)
	}
	if updated.Config().FunctionalCurrency != codeEUR {
		t.Errorf("updated currency = %q; want %q", updated.Config().FunctionalCurrency, codeEUR)
	}

	eurEntries := []core.Entry{
		mustRateEntry(t, "eur-debit", txAmountHundred, codeEUR, txRateOne, codeEUR),
		mustRateEntry(t, "eur-credit", txAmountNegHundred, codeEUR, txRateOne, codeEUR),
	}
	updatedTx, err := updated.ValidateTransaction(context.Background(), core.TransactionInput{
		ID:            txID,
		EffectiveDate: ledgerEffectiveDate(),
		Entries:       eurEntries,
	})
	if err != nil {
		t.Fatalf("updated ValidateTransaction: %v", err)
	}
	if !updatedTx.PostedAt().Equal(expected) {
		t.Errorf("updated PostedAt() = %s; want %s", updatedTx.PostedAt(), expected)
	}

	originalTx, err := ledger.ValidateTransaction(context.Background(), validTransactionInput(t))
	if err != nil {
		t.Fatalf("original ValidateTransaction: %v", err)
	}
	if !originalTx.PostedAt().Equal(expected) {
		t.Errorf("original PostedAt() = %s; want %s", originalTx.PostedAt(), expected)
	}
}

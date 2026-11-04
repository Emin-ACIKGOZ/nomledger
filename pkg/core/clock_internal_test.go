// SPDX-License-Identifier: MIT

package core

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	pkgerr "github.com/Emin-ACIKGOZ/nomledger/pkg/errors"
	"github.com/Emin-ACIKGOZ/nomledger/pkg/interfaces"

	"github.com/shopspring/decimal"
)

func TestClockOptionErrorsInternal(t *testing.T) {
	config := LedgerConfig{FunctionalCurrency: testCurrencyUSD}
	effective := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	t.Run("ledger option error", func(t *testing.T) {
		cause := errors.New("ledger clock option failed")
		option := LedgerOption(func(*ledgerOptions) error {
			return cause
		})

		ledger, err := NewLedger(config, nil, option)
		if ledger != nil {
			t.Errorf("NewLedger returned non-nil ledger; want nil")
		}
		assertAccountStructuredError(t, err, cause, "clock", "NewLedger")
	})

	t.Run("builder option error", func(t *testing.T) {
		cause := errors.New("builder clock option failed")
		option := BuilderOption(func(*builderOptions) error {
			return cause
		})

		builder := NewTxBuilder("tx-option-error", effective, config, nil, option)
		transaction, err := builder.Build()
		if !reflect.DeepEqual(transaction, Transaction{}) {
			t.Errorf("Build returned non-zero Transaction: %+v", transaction)
		}
		assertAccountStructuredError(t, err, cause, "clock", buildOperation)
	})
}

type countingClock struct {
	reads int
	now   time.Time
}

func (c *countingClock) Now() time.Time {
	c.reads++
	return c.now
}

func clockTestTime() time.Time {
	return time.Date(2026, 11, 4, 12, 34, 56, 123456789, time.UTC)
}

func clockTestProvider() interfaces.RateProvider {
	return interfaces.RateProviderFunc(func(context.Context, interfaces.RateRequest) (decimal.Decimal, error) {
		return decimal.Zero, errors.New("provider must not be called")
	})
}

func mustCountingClockEntries(t *testing.T) []Entry {
	t.Helper()
	debit, err := NewEntry("clock-debit", decimal.NewFromInt(100), testCurrencyUSD, decimal.NewFromInt(1), testCurrencyUSD)
	if err != nil {
		t.Fatalf("NewEntry debit: %v", err)
	}
	credit, err := NewEntry("clock-credit", decimal.NewFromInt(-100), testCurrencyUSD, decimal.NewFromInt(1), testCurrencyUSD)
	if err != nil {
		t.Fatalf("NewEntry credit: %v", err)
	}
	return []Entry{debit, credit}
}

// TestCountingClockInternal asserts PostedAt reads the clock exactly once on
// success and never on paths that cannot build a transaction.
func TestCountingClockInternal(t *testing.T) {
	t.Run("ledger success reads once", testCountingLedgerSuccess)
	t.Run("builder success reads once", testCountingBuilderSuccess)
	t.Run("legacy success reads once", testCountingLegacySuccess)
	t.Run("zero-sum failure reads zero", testCountingZeroSumFailure)
	t.Run("canceled context reads zero", testCountingCanceledContext)
	t.Run("option error reads zero", testCountingOptionError)
	t.Run("finalized rebuild reads zero", testCountingFinalizedRebuild)
}

func testCountingLedgerSuccess(t *testing.T) {
	t.Helper()
	clock := &countingClock{now: clockTestTime()}
	ledger, err := NewLedger(
		LedgerConfig{FunctionalCurrency: testCurrencyUSD},
		clockTestProvider(),
		WithClock(clock),
	)
	if err != nil {
		t.Fatalf("NewLedger: %v", err)
	}
	_, err = ledger.ValidateTransaction(context.Background(), TransactionInput{
		ID:            "clock-ledger",
		EffectiveDate: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		Entries:       mustCountingClockEntries(t),
	})
	if err != nil {
		t.Fatalf("ValidateTransaction: %v", err)
	}
	if clock.reads != 1 {
		t.Errorf("clock reads = %d; want 1", clock.reads)
	}
}

func testCountingBuilderSuccess(t *testing.T) {
	t.Helper()
	clock := &countingClock{now: clockTestTime()}
	builder := NewTxBuilder(
		"clock-builder",
		time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		LedgerConfig{FunctionalCurrency: testCurrencyUSD},
		clockTestProvider(),
		WithBuilderClock(clock),
	)
	for _, entry := range mustCountingClockEntries(t) {
		builder.AddEntry(entry)
	}
	if _, err := builder.Build(); err != nil {
		t.Fatalf("Build: %v", err)
	}
	if clock.reads != 1 {
		t.Errorf("clock reads = %d; want 1", clock.reads)
	}
}

func testCountingLegacySuccess(t *testing.T) {
	t.Helper()
	clock := &countingClock{now: clockTestTime()}
	ledger, err := NewLedger(
		LedgerConfig{FunctionalCurrency: testCurrencyUSD},
		clockTestProvider(),
		WithClock(clock),
	)
	if err != nil {
		t.Fatalf("NewLedger: %v", err)
	}
	if _, err := ledger.ValidateTransactionLegacy(
		"clock-legacy",
		time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		false,
		mustCountingClockEntries(t)...,
	); err != nil {
		t.Fatalf("ValidateTransactionLegacy: %v", err)
	}
	if clock.reads != 1 {
		t.Errorf("clock reads = %d; want 1", clock.reads)
	}
}

func testCountingZeroSumFailure(t *testing.T) {
	t.Helper()
	clock := &countingClock{now: clockTestTime()}
	builder := NewTxBuilder(
		"clock-zero-sum",
		time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		LedgerConfig{FunctionalCurrency: testCurrencyUSD},
		clockTestProvider(),
		WithBuilderClock(clock),
	)
	unbalanced, err := NewEntry(
		"clock-credit-fail",
		mustEntryDecimalInternal(t, "-99.00"),
		testCurrencyUSD,
		decimal.NewFromInt(1),
		testCurrencyUSD,
	)
	if err != nil {
		t.Fatalf("NewEntry unbalanced credit: %v", err)
	}
	builder.AddEntry(mustCountingClockEntries(t)[0])
	builder.AddEntry(unbalanced)

	if _, err := builder.Build(); !errors.Is(err, pkgerr.ErrZeroSumViolation) {
		t.Fatalf("Build error = %v; want zero-sum failure", err)
	}
	if clock.reads != 0 {
		t.Errorf("clock reads = %d; want 0", clock.reads)
	}
}

func testCountingCanceledContext(t *testing.T) {
	t.Helper()
	clock := &countingClock{now: clockTestTime()}
	ledger, err := NewLedger(
		LedgerConfig{FunctionalCurrency: testCurrencyUSD},
		clockTestProvider(),
		WithClock(clock),
	)
	if err != nil {
		t.Fatalf("NewLedger: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = ledger.ValidateTransaction(ctx, TransactionInput{
		ID:            "clock-canceled",
		EffectiveDate: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		Entries:       mustCountingClockEntries(t),
	})
	if err == nil {
		t.Fatal("ValidateTransaction returned nil error")
	}
	if clock.reads != 0 {
		t.Errorf("clock reads = %d; want 0", clock.reads)
	}
}

func testCountingOptionError(t *testing.T) {
	t.Helper()
	clock := &countingClock{now: clockTestTime()}
	option := BuilderOption(func(*builderOptions) error {
		return errors.New("builder option failed")
	})
	builder := NewTxBuilder(
		"clock-option-error",
		time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		LedgerConfig{FunctionalCurrency: testCurrencyUSD},
		clockTestProvider(),
		WithBuilderClock(clock),
		option,
	)
	if _, err := builder.Build(); err == nil {
		t.Fatal("Build returned nil error")
	}
	if clock.reads != 0 {
		t.Errorf("clock reads = %d; want 0", clock.reads)
	}
}

func testCountingFinalizedRebuild(t *testing.T) {
	t.Helper()
	clock := &countingClock{now: clockTestTime()}
	builder := NewTxBuilder(
		"clock-finalized",
		time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		LedgerConfig{FunctionalCurrency: testCurrencyUSD},
		clockTestProvider(),
		WithBuilderClock(clock),
	)
	for _, entry := range mustCountingClockEntries(t) {
		builder.AddEntry(entry)
	}
	if _, err := builder.Build(); err != nil {
		t.Fatalf("first Build: %v", err)
	}
	if clock.reads != 1 {
		t.Fatalf("clock reads after first Build = %d; want 1", clock.reads)
	}
	if _, err := builder.Build(); err == nil {
		t.Fatal("second Build returned nil error")
	}
	if clock.reads != 1 {
		t.Errorf("clock reads after finalized Build = %d; want 1", clock.reads)
	}
}

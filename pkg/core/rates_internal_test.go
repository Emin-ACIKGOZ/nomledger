// SPDX-License-Identifier: MIT

package core

import (
	"context"
	"errors"
	"testing"
	"time"

	pkgerr "github.com/Emin-ACIKGOZ/nomledger/pkg/errors"
	"github.com/Emin-ACIKGOZ/nomledger/pkg/interfaces"

	"github.com/shopspring/decimal"
)

func assertRateErrorInternal(t *testing.T, err error, want error, field string, entryIndex int) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("error = %v; want %v", err, want)
	}
	var validationErr *pkgerr.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("errors.As(err, *ValidationError) = false; err = %v", err)
	}
	if validationErr.Operation != buildOperation {
		t.Errorf("Operation = %q; want %q", validationErr.Operation, buildOperation)
	}
	if validationErr.EntryIndex != entryIndex {
		t.Errorf("EntryIndex = %d; want %d", validationErr.EntryIndex, entryIndex)
	}
	if validationErr.Field != field {
		t.Errorf("Field = %q; want %q", validationErr.Field, field)
	}
}

type cancelingTestProvider struct {
	cancel context.CancelFunc
	calls  int
}

func (p *cancelingTestProvider) GetRate(_ context.Context, _ interfaces.RateRequest) (decimal.Decimal, error) {
	p.calls++
	if p.cancel != nil {
		p.cancel()
	}
	return decimal.NewFromInt(1), nil
}

type testProviderFailure struct {
	message string
}

func (e *testProviderFailure) Error() string { return e.message }

type cancelingErrorProvider struct {
	cancel context.CancelFunc
	err    error
	calls  int
}

func (p *cancelingErrorProvider) GetRate(_ context.Context, _ interfaces.RateRequest) (decimal.Decimal, error) {
	p.calls++
	if p.cancel != nil {
		p.cancel()
	}
	return decimal.Zero, p.err
}

func mustForeignRateEntries(t *testing.T) []Entry {
	t.Helper()
	first, err := NewEntry(
		"rate-first",
		decimal.NewFromInt(100),
		testEntryCurrencyEUR,
		mustEntryDecimalInternal(t, "1.1"),
		testCurrencyUSD,
	)
	if err != nil {
		t.Fatalf("NewEntry first: %v", err)
	}
	second, err := NewEntry(
		"rate-second",
		decimal.NewFromInt(100),
		testEntryCurrencyEUR,
		mustEntryDecimalInternal(t, "1.2"),
		testCurrencyUSD,
	)
	if err != nil {
		t.Fatalf("NewEntry second: %v", err)
	}
	return []Entry{first, second}
}

// TestContextCancellationDuringProviderCall.
func TestContextCancellationDuringProviderCall(t *testing.T) {
	t.Run("already cancelled before first call", testCancelledBeforeCall)
	t.Run("non-positive entry rate", testNonPositiveEntryRate)
	t.Run("provider cancels during call", testProviderCancelsDuringCall)
	t.Run("provider error and cancellation are both preserved", testProviderErrorWithCancellation)
}

func testCancelledBeforeCall(t *testing.T) {
	t.Helper()
	asOf := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	tolerance := decimal.Zero

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	provider := &cancelingTestProvider{}

	err := validateRates(ctx, mustForeignRateEntries(t), tolerance, provider, asOf)
	if !errors.Is(err, pkgerr.ErrRateProvider) {
		t.Fatalf("error = %v; want ErrRateProvider", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v; want context.Canceled", err)
	}
	assertRateErrorInternal(t, err, pkgerr.ErrRateProvider, "context", 0)
	if provider.calls != 0 {
		t.Errorf("provider calls = %d; want 0 after cancellation", provider.calls)
	}
}

func testNonPositiveEntryRate(t *testing.T) {
	t.Helper()
	asOf := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	tolerance := decimal.Zero

	entry := mustForeignRateEntries(t)[0]
	entry.exchangeRate = decimal.Zero
	provider := &cancelingTestProvider{}

	err := validateRates(context.Background(), []Entry{entry}, tolerance, provider, asOf)
	assertRateErrorInternal(t, err, pkgerr.ErrInvalidExchangeRate, "exchangeRate", 0)
	if provider.calls != 0 {
		t.Errorf("provider calls = %d; want 0 for a non-positive entry rate", provider.calls)
	}
}

func testProviderCancelsDuringCall(t *testing.T) {
	t.Helper()
	asOf := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	tolerance := decimal.Zero

	ctx, cancel := context.WithCancel(context.Background())
	provider := &cancelingTestProvider{cancel: cancel}

	err := validateRates(ctx, mustForeignRateEntries(t), tolerance, provider, asOf)
	if !errors.Is(err, pkgerr.ErrRateProvider) {
		t.Fatalf("error = %v; want ErrRateProvider", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v; want context.Canceled", err)
	}
	assertRateErrorInternal(t, err, pkgerr.ErrRateProvider, "context", 0)
	if provider.calls != 1 {
		t.Errorf("provider calls = %d; want 1 (no calls after cancellation)", provider.calls)
	}
}

func testProviderErrorWithCancellation(t *testing.T) {
	t.Helper()
	asOf := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	tolerance := decimal.Zero

	ctx, cancel := context.WithCancel(context.Background())
	sentinel := &testProviderFailure{message: "provider failure"}
	provider := &cancelingErrorProvider{cancel: cancel, err: sentinel}

	err := validateRates(ctx, mustForeignRateEntries(t), tolerance, provider, asOf)
	if !errors.Is(err, pkgerr.ErrRateProvider) {
		t.Fatalf("error = %v; want ErrRateProvider", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v; want context.Canceled", err)
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error = %v; want provider sentinel", err)
	}

	var extracted *testProviderFailure
	if !errors.As(err, &extracted) {
		t.Fatalf("errors.As(err, *testProviderFailure) = false; err = %v", err)
	}
	if extracted != sentinel {
		t.Errorf("extracted provider error = %p; want %p", extracted, sentinel)
	}

	assertRateErrorInternal(t, err, pkgerr.ErrRateProvider, "context", 0)
	if provider.calls != 1 {
		t.Errorf("provider calls = %d; want exactly 1", provider.calls)
	}
}

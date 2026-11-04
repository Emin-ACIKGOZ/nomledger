// SPDX-License-Identifier: MIT

package core_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Emin-ACIKGOZ/nomledger/pkg/core"
	pkgerr "github.com/Emin-ACIKGOZ/nomledger/pkg/errors"
	"github.com/Emin-ACIKGOZ/nomledger/pkg/interfaces"

	"github.com/shopspring/decimal"
)

const (
	rateFieldRate     = "rate"
	rateFieldContext  = "context"
	codeGBP           = "GBP"
	pairGBPUSD        = "GBP/USD"
	pairUSDEUR        = "USD/EUR"
	rateGBPUSD        = "1.25"
	rateSmallPositive = "0.0001"
	rateTolerance     = "0.01"
	rateWithinTol     = "1.11"
	rateOutsideTol    = "1.1101"
	rateAmountFifty   = "50.00"
	rateAmountNeg165  = "-165.00"
	rateAmountNeg235  = "-235.00"
	rateAmountNeg100  = "-100.00"
	rateAmountNeg001  = "-0.01"
)

type recordingRateProvider struct {
	rates    map[string]decimal.Decimal
	err      error
	requests []interfaces.RateRequest
}

func (p *recordingRateProvider) GetRate(_ context.Context, req interfaces.RateRequest) (decimal.Decimal, error) {
	p.requests = append(p.requests, req)
	if p.err != nil {
		return decimal.Zero, p.err
	}
	if rate, ok := p.rates[string(req.Base)+"/"+string(req.Quote)]; ok {
		return rate, nil
	}
	return decimal.Zero, errors.New("missing test rate")
}

func (p *recordingRateProvider) calls() int {
	return len(p.requests)
}

func mustRateEntry(t *testing.T, accountID, amount, txCurrency, rate, functionalCurrency string) core.Entry {
	t.Helper()
	entry, err := core.NewEntry(
		accountID,
		mustTxDecimal(t, amount),
		txCurrency,
		mustTxDecimal(t, rate),
		functionalCurrency,
	)
	if err != nil {
		t.Fatalf("NewEntry(%s, %s, %s, %s, %s): %v",
			accountID, amount, txCurrency, rate, functionalCurrency, err)
	}
	return entry
}

func buildWithProvider(
	t *testing.T,
	config core.LedgerConfig,
	id string,
	effective time.Time,
	provider interfaces.RateProvider,
	entries []core.Entry,
) (core.Transaction, error) {
	t.Helper()
	builder := core.NewTxBuilder(id, effective, config, provider)
	for _, entry := range entries {
		builder.AddEntry(entry)
	}
	return builder.Build()
}

func assertRateValidationError(t *testing.T, err error, want error, field string, entryIndex int) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("error = %v; want %v", err, want)
	}
	var validationErr *pkgerr.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("errors.As(err, *ValidationError) = false; err = %v", err)
	}
	if validationErr.Operation != transactionBuildOperation {
		t.Errorf("Operation = %q; want %q", validationErr.Operation, transactionBuildOperation)
	}
	if validationErr.EntryIndex != entryIndex {
		t.Errorf("EntryIndex = %d; want %d", validationErr.EntryIndex, entryIndex)
	}
	if validationErr.Field != field {
		t.Errorf("Field = %q; want %q", validationErr.Field, field)
	}
}

// TestProviderRequest.
func TestProviderRequest(t *testing.T) {
	t.Run("EUR transaction to USD functional", testProviderRequestEURUSD)
	t.Run("USD transaction to EUR functional", testProviderRequestUSDEUR)
}

func testProviderRequestEURUSD(t *testing.T) {
	t.Helper()
	zone := time.FixedZone("EST", -5*60*60)
	effective := time.Date(2024, 1, 15, 12, 0, 0, 123456789, zone)
	provider := &recordingRateProvider{rates: map[string]decimal.Decimal{
		pairEURUSD: mustTxDecimal(t, ledgerRate),
	}}
	debit, credit := mustEURUSDEntries(t)
	if _, err := buildWithProvider(t, txTestConfig(), txID, effective, provider, []core.Entry{debit, credit}); err != nil {
		t.Fatalf("Build: %v", err)
	}

	if provider.calls() != 1 {
		t.Fatalf("provider calls = %d; want 1", provider.calls())
	}
	request := provider.requests[0]
	if request.Base != codeEUR {
		t.Errorf("Base = %q; want %q", request.Base, codeEUR)
	}
	if request.Quote != codeUSD {
		t.Errorf("Quote = %q; want %q", request.Quote, codeUSD)
	}
	if !request.AsOf.Equal(effective.UTC()) {
		t.Errorf("AsOf = %s; want %s", request.AsOf, effective.UTC())
	}
	if request.AsOf.Location() != time.UTC {
		t.Errorf("AsOf location = %v; want UTC", request.AsOf.Location())
	}
}

func testProviderRequestUSDEUR(t *testing.T) {
	t.Helper()
	zone := time.FixedZone("EST", -5*60*60)
	effective := time.Date(2024, 1, 15, 12, 0, 0, 123456789, zone)
	provider := &recordingRateProvider{rates: map[string]decimal.Decimal{
		pairUSDEUR: mustTxDecimal(t, ledgerRateUSDToEUR),
	}}
	debit := mustRateEntry(t, "usd-acc", txAmountHundred, codeUSD, ledgerRateUSDToEUR, codeEUR)
	credit := mustRateEntry(t, "eur-acc", ledgerAmountNeg90, codeEUR, txRateOne, codeEUR)
	config := core.LedgerConfig{FunctionalCurrency: codeEUR}
	if _, err := buildWithProvider(t, config, txID, effective, provider, []core.Entry{debit, credit}); err != nil {
		t.Fatalf("Build: %v", err)
	}

	request := provider.requests[0]
	if request.Base != codeUSD || request.Quote != codeEUR {
		t.Errorf("request = %s/%s; want USD/EUR", request.Base, request.Quote)
	}
	if !request.AsOf.Equal(effective.UTC()) {
		t.Errorf("AsOf = %s; want %s", request.AsOf, effective.UTC())
	}
}

// TestProviderDeduplication.
func TestProviderDeduplication(t *testing.T) {
	effective := ledgerEffectiveDate()

	t.Run("same pair called once", func(t *testing.T) {
		provider := &recordingRateProvider{rates: map[string]decimal.Decimal{
			pairEURUSD: mustTxDecimal(t, ledgerRate),
		}}
		entries := []core.Entry{
			mustRateEntry(t, "eur-1", txAmountHundred, codeEUR, ledgerRate, codeUSD),
			mustRateEntry(t, "eur-2", rateAmountFifty, codeEUR, ledgerRate, codeUSD),
			mustRateEntry(t, "usd-1", rateAmountNeg165, codeUSD, txRateOne, codeUSD),
		}
		if _, err := buildWithProvider(t, txTestConfig(), txID, effective, provider, entries); err != nil {
			t.Fatalf("Build: %v", err)
		}
		if provider.calls() != 1 {
			t.Errorf("provider calls = %d; want 1", provider.calls())
		}
	})

	t.Run("distinct pairs called twice", func(t *testing.T) {
		provider := &recordingRateProvider{rates: map[string]decimal.Decimal{
			pairEURUSD: mustTxDecimal(t, ledgerRate),
			pairGBPUSD: mustTxDecimal(t, rateGBPUSD),
		}}
		entries := []core.Entry{
			mustRateEntry(t, "eur-1", txAmountHundred, codeEUR, ledgerRate, codeUSD),
			mustRateEntry(t, "gbp-1", txAmountHundred, codeGBP, rateGBPUSD, codeUSD),
			mustRateEntry(t, "usd-1", rateAmountNeg235, codeUSD, txRateOne, codeUSD),
		}
		if _, err := buildWithProvider(t, txTestConfig(), txID, effective, provider, entries); err != nil {
			t.Fatalf("Build: %v", err)
		}
		if provider.calls() != 2 {
			t.Errorf("provider calls = %d; want 2", provider.calls())
		}
	})

	t.Run("cache is per validation", testProviderCachePerValidation)
}

func testProviderCachePerValidation(t *testing.T) {
	t.Helper()
	effective := ledgerEffectiveDate()

	provider := &recordingRateProvider{rates: map[string]decimal.Decimal{
		pairEURUSD: mustTxDecimal(t, ledgerRate),
	}}
	entries := []core.Entry{
		mustRateEntry(t, "eur-1", txAmountHundred, codeEUR, ledgerRate, codeUSD),
		mustRateEntry(t, "eur-2", rateAmountFifty, codeEUR, ledgerRate, codeUSD),
		mustRateEntry(t, "usd-1", rateAmountNeg165, codeUSD, txRateOne, codeUSD),
	}

	firstBuilder := core.NewTxBuilder(txID, effective, txTestConfig(), provider)
	for _, entry := range entries {
		firstBuilder.AddEntry(entry)
	}
	if _, err := firstBuilder.Build(); err != nil {
		t.Fatalf("first Build: %v", err)
	}
	if provider.calls() != 1 {
		t.Fatalf("calls after first Build = %d; want 1", provider.calls())
	}

	secondBuilder := core.NewTxBuilder(txID, effective, txTestConfig(), provider)
	for _, entry := range entries {
		secondBuilder.AddEntry(entry)
	}
	if _, err := secondBuilder.Build(); err != nil {
		t.Fatalf("second Build: %v", err)
	}
	if provider.calls() != 2 {
		t.Errorf("calls after second Build = %d; want 2", provider.calls())
	}
}

// TestProviderErrorWrap.
func TestProviderErrorWrap(t *testing.T) {
	sentinel := errors.New("provider unavailable")
	provider := &recordingRateProvider{err: sentinel}
	debit, credit := mustEURUSDEntries(t)
	zone := time.FixedZone("EST", -5*60*60)
	effective := time.Date(2024, 1, 15, 12, 0, 0, 123456789, zone)

	_, err := buildWithProvider(t, txTestConfig(), txID, effective, provider, []core.Entry{debit, credit})
	if !errors.Is(err, pkgerr.ErrRateProvider) {
		t.Fatalf("error = %v; want ErrRateProvider", err)
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error = %v; want provider sentinel", err)
	}
	assertRateValidationError(t, err, pkgerr.ErrRateProvider, rateFieldRate, 0)

	message := err.Error()
	for _, want := range []string{
		codeEUR,
		codeUSD,
		effective.UTC().Format(time.RFC3339Nano),
		sentinel.Error(),
	} {
		if !strings.Contains(message, want) {
			t.Errorf("error message = %q; want it to contain %q", message, want)
		}
	}
}

// TestSameCurrencySkipsProvider.
func TestSameCurrencySkipsProvider(t *testing.T) {
	provider := &recordingRateProvider{}
	entries := transactionFieldEntries(t)
	if _, err := buildWithProvider(t, txTestConfig(), txID, ledgerEffectiveDate(), provider, entries); err != nil {
		t.Fatalf("Build: %v", err)
	}
	if provider.calls() != 0 {
		t.Errorf("provider calls = %d; want 0", provider.calls())
	}
}

// TestProviderNotCalledAfterEarlierFailure.
func TestProviderNotCalledAfterEarlierFailure(t *testing.T) {
	effective := ledgerEffectiveDate()

	t.Run("empty entries", func(t *testing.T) {
		provider := &recordingRateProvider{}
		_, err := buildWithProvider(t, txTestConfig(), txID, effective, provider, nil)
		if !errors.Is(err, pkgerr.ErrEmptyTransaction) {
			t.Fatalf("error = %v; want ErrEmptyTransaction", err)
		}
		if provider.calls() != 0 {
			t.Errorf("provider calls = %d; want 0", provider.calls())
		}
	})

	t.Run("invalid ID", func(t *testing.T) {
		provider := &recordingRateProvider{}
		_, err := buildWithProvider(t, txTestConfig(), "", effective, provider, transactionFieldEntries(t))
		if !errors.Is(err, pkgerr.ErrInvalidTransactionID) {
			t.Fatalf("error = %v; want ErrInvalidTransactionID", err)
		}
		if provider.calls() != 0 {
			t.Errorf("provider calls = %d; want 0", provider.calls())
		}
	})

	t.Run("zero-sum failure", func(t *testing.T) {
		provider := &recordingRateProvider{rates: map[string]decimal.Decimal{
			pairEURUSD: mustTxDecimal(t, ledgerRate),
		}}
		entries := []core.Entry{
			mustRateEntry(t, "eur-1", txAmountHundred, codeEUR, ledgerRate, codeUSD),
			mustRateEntry(t, "usd-1", rateAmountNeg100, codeUSD, txRateOne, codeUSD),
		}
		_, err := buildWithProvider(t, txTestConfig(), txID, effective, provider, entries)
		if !errors.Is(err, pkgerr.ErrZeroSumViolation) {
			t.Fatalf("error = %v; want ErrZeroSumViolation", err)
		}
		if provider.calls() != 0 {
			t.Errorf("provider calls = %d; want 0", provider.calls())
		}
	})
}

// TestProviderRatePositivity.
func TestProviderRatePositivity(t *testing.T) {
	entries := []core.Entry{
		mustRateEntry(t, "eur-1", txAmountHundred, codeEUR, rateSmallPositive, codeUSD),
		mustRateEntry(t, "usd-1", rateAmountNeg001, codeUSD, txRateOne, codeUSD),
	}

	cases := []struct {
		name    string
		rate    string
		wantErr error
	}{
		{"zero", "0", pkgerr.ErrInvalidExchangeRate},
		{"negative", "-1", pkgerr.ErrInvalidExchangeRate},
		{"small positive", rateSmallPositive, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			provider := &recordingRateProvider{rates: map[string]decimal.Decimal{
				pairEURUSD: mustTxDecimal(t, tc.rate),
			}}
			_, err := buildWithProvider(t, txTestConfig(), txID, ledgerEffectiveDate(), provider, entries)
			if tc.wantErr != nil {
				assertRateValidationError(t, err, tc.wantErr, rateFieldRate, 0)
				return
			}
			if err != nil {
				t.Errorf("Build: %v", err)
			}
		})
	}
}

// TestToleranceBoundary.
func TestToleranceBoundary(t *testing.T) {
	config := core.LedgerConfig{FunctionalCurrency: codeUSD, RateTolerance: mustTxDecimal(t, rateTolerance)}
	entries := func(t *testing.T) []core.Entry {
		t.Helper()
		debit, credit := mustEURUSDEntries(t)
		return []core.Entry{debit, credit}
	}

	t.Run("exact tolerance passes", func(t *testing.T) {
		provider := &recordingRateProvider{rates: map[string]decimal.Decimal{
			pairEURUSD: mustTxDecimal(t, rateWithinTol),
		}}
		if _, err := buildWithProvider(t, config, txID, ledgerEffectiveDate(), provider, entries(t)); err != nil {
			t.Errorf("Build: %v; want nil at exact tolerance", err)
		}
	})

	t.Run("just over tolerance fails", func(t *testing.T) {
		provider := &recordingRateProvider{rates: map[string]decimal.Decimal{
			pairEURUSD: mustTxDecimal(t, rateOutsideTol),
		}}
		_, err := buildWithProvider(t, config, txID, ledgerEffectiveDate(), provider, entries(t))
		assertRateValidationError(t, err, pkgerr.ErrRateDeviation, rateFieldRate, 0)
	})
}

type rateContextKey struct{}

// TestRateProviderFunc verifies the adapter implements RateProvider and
// preserves context and request.
func TestRateProviderFunc(t *testing.T) {
	var _ interfaces.RateProvider = interfaces.RateProviderFunc(nil)

	key := rateContextKey{}
	ctx := context.WithValue(context.Background(), key, "value")
	request := interfaces.RateRequest{
		Base:  codeEUR,
		Quote: codeUSD,
		AsOf:  ledgerEffectiveDate(),
	}
	var gotContext context.Context
	var gotRequest interfaces.RateRequest
	provider := interfaces.RateProviderFunc(
		func(callCtx context.Context, callRequest interfaces.RateRequest) (decimal.Decimal, error) {
			gotContext = callCtx
			gotRequest = callRequest
			return mustTxDecimal(t, ledgerRate), nil
		},
	)

	rate, err := provider.GetRate(ctx, request)
	if err != nil {
		t.Fatalf("GetRate: %v", err)
	}
	if !rate.Equal(mustTxDecimal(t, ledgerRate)) {
		t.Errorf("rate = %s; want %s", rate, ledgerRate)
	}
	if gotContext != ctx {
		t.Error("GetRate did not preserve the context")
	}
	if gotContext.Value(key) != "value" {
		t.Errorf("context value = %v; want value", gotContext.Value(key))
	}
	if gotRequest != request {
		t.Errorf("request = %+v; want %+v", gotRequest, request)
	}
}

type providerCustomError struct {
	message string
}

func (e *providerCustomError) Error() string { return e.message }

// TestProviderErrorCustomType proves the provider cause chain survives without
// context cancellation.
func TestProviderErrorCustomType(t *testing.T) {
	sentinel := &providerCustomError{message: "provider typed failure"}
	provider := &recordingRateProvider{err: sentinel}
	debit, credit := mustEURUSDEntries(t)

	_, err := buildWithProvider(t, txTestConfig(), txID, ledgerEffectiveDate(), provider, []core.Entry{debit, credit})
	if !errors.Is(err, pkgerr.ErrRateProvider) {
		t.Fatalf("error = %v; want ErrRateProvider", err)
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error = %v; want provider sentinel", err)
	}

	var extracted *providerCustomError
	if !errors.As(err, &extracted) {
		t.Fatalf("errors.As(err, *providerCustomError) = false; err = %v", err)
	}
	if extracted != sentinel {
		t.Errorf("extracted provider error = %p; want %p", extracted, sentinel)
	}

	assertRateValidationError(t, err, pkgerr.ErrRateProvider, rateFieldRate, 0)
}

// TestNilProviderBuildStructured verifies the nil-provider rate error shape
// through the public Build path.
func TestNilProviderBuildStructured(t *testing.T) {
	builder := core.NewTxBuilder(txID, ledgerEffectiveDate(), txTestConfig(), nil)
	debit, credit := mustEURUSDEntries(t)
	builder.AddEntry(debit).AddEntry(credit)

	_, err := builder.Build()
	assertRateValidationError(t, err, pkgerr.ErrNilRateProvider, rateFieldRate, 0)
}

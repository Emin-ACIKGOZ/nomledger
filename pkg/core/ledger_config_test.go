// SPDX-License-Identifier: MIT

package core_test

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/Emin-ACIKGOZ/nomledger/pkg/core"
	pkgerr "github.com/Emin-ACIKGOZ/nomledger/pkg/errors"

	"github.com/shopspring/decimal"
)

const (
	ledgerOpValidate     = "LedgerConfig.Validate"
	ledgerOpNew          = "NewLedger"
	ledgerOpUpdate       = "UpdateConfig"
	ledgerFieldCurrency  = "functionalCurrency"
	ledgerFieldTolerance = "rateTolerance"
	ledgerFieldConfig    = "config"
	ledgerFieldProvider  = "provider"

	ledgerRate           = "1.1"
	ledgerRateNear       = "1.1000001"
	ledgerRateWithin     = "1.100001"
	ledgerRateUSDToEUR   = "0.9"
	ledgerToleranceZero  = "0"
	ledgerToleranceSmall = "0.00001"
	ledgerToleranceNeg   = "-0.00001"
	ledgerAmountNeg110   = "-110.00"
	ledgerAmountNeg90    = "-90.00"

	pairEURUSD   = "EUR/USD"
	codeUSDMixed = "UsD"
)

func validateLedger(
	t *testing.T,
	ledger *core.Ledger,
	id string,
	effectiveDate time.Time,
	entries ...core.Entry,
) (core.Transaction, error) {
	t.Helper()
	return ledger.ValidateTransaction(context.Background(), core.TransactionInput{
		ID:            id,
		EffectiveDate: effectiveDate,
		Entries:       entries,
	})
}

func mustLedgerDecimal(t *testing.T, value string) decimal.Decimal {
	t.Helper()
	d, err := decimal.NewFromString(value)
	if err != nil {
		t.Fatalf("decimal.NewFromString(%q): %v", value, err)
	}
	return d
}

func ledgerEffectiveDate() time.Time {
	return time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
}

func mustEURUSDEntries(t *testing.T) (core.Entry, core.Entry) {
	t.Helper()
	debit, err := core.NewEntry(
		"acc-eur",
		mustLedgerDecimal(t, txAmountHundred),
		codeEUR,
		mustLedgerDecimal(t, ledgerRate),
		codeUSD,
	)
	if err != nil {
		t.Fatalf("NewEntry EUR debit: %v", err)
	}
	credit, err := core.NewEntry(
		"acc-usd",
		mustLedgerDecimal(t, ledgerAmountNeg110),
		codeUSD,
		mustLedgerDecimal(t, txRateOne),
		codeUSD,
	)
	if err != nil {
		t.Fatalf("NewEntry USD credit: %v", err)
	}
	return debit, credit
}

func mustUSDPairEntries(t *testing.T) (core.Entry, core.Entry) {
	t.Helper()
	return mustTxEntry(t, txDebitAccount, txAmountHundred), mustTxEntry(t, txCreditAccount, txAmountNegHundred)
}

// TestLedgerConfigFunctionalCurrency.
func TestLedgerConfigFunctionalCurrency(t *testing.T) {
	cases := []struct {
		name         string
		currencyCode string
		wantErr      bool
	}{
		{codeUSD, codeUSD, false},
		{"lowercase", codeUSDLower, false},
		{"mixed case", codeUSDMixed, false},
		{"non-ratable XAU", codeXAU, true},
		{"historical BGN", codeBGN, true},
		{"short code", codeShort, true},
		{"spaced code", codeSpaced, true},
		{"non-ASCII code", codeCyrillic, true},
		{"empty code", "", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			config := core.LedgerConfig{FunctionalCurrency: tc.currencyCode}
			err := config.Validate()
			if tc.wantErr {
				assertStructuredValidationError(t, err, pkgerr.ErrInvalidConfiguration,
					ledgerFieldCurrency, ledgerOpValidate)
				return
			}
			if err != nil {
				t.Fatalf("Validate() = %v; want nil", err)
			}
		})
	}

	for _, input := range []string{codeUSDLower, codeUSDMixed, codeUSD} {
		t.Run("normalised "+input, func(t *testing.T) {
			ledger, err := core.NewLedger(core.LedgerConfig{FunctionalCurrency: input}, mockRateProvider{})
			if err != nil {
				t.Fatalf("NewLedger: %v", err)
			}
			if got := ledger.Config().FunctionalCurrency; got != codeUSD {
				t.Errorf("Config().FunctionalCurrency = %q; want %q", got, codeUSD)
			}
		})
	}

	for _, input := range []string{codeXAU, codeBGN, codeShort, codeSpaced, codeCyrillic, ""} {
		t.Run("reject "+input, func(t *testing.T) {
			ledger, err := core.NewLedger(core.LedgerConfig{FunctionalCurrency: input}, mockRateProvider{})
			if ledger != nil {
				t.Errorf("NewLedger returned non-nil ledger for invalid currency %q", input)
			}
			assertStructuredValidationError(t, err, pkgerr.ErrInvalidConfiguration,
				ledgerFieldConfig, ledgerOpNew)
		})
	}
}

// TestLedgerConfigTolerance.
func TestLedgerConfigTolerance(t *testing.T) {
	base := core.LedgerConfig{FunctionalCurrency: codeUSD}

	negative := base
	negative.RateTolerance = mustLedgerDecimal(t, ledgerToleranceNeg)
	assertStructuredValidationError(t, negative.Validate(), pkgerr.ErrInvalidConfiguration,
		ledgerFieldTolerance, ledgerOpValidate)

	zero := base
	zero.RateTolerance = mustLedgerDecimal(t, ledgerToleranceZero)
	if err := zero.Validate(); err != nil {
		t.Errorf("zero tolerance Validate() = %v; want nil", err)
	}

	positive := base
	positive.RateTolerance = mustLedgerDecimal(t, ledgerToleranceSmall)
	if err := positive.Validate(); err != nil {
		t.Errorf("positive tolerance Validate() = %v; want nil", err)
	}

	t.Run("zero tolerance requires exact equality", func(t *testing.T) {
		ledger, err := core.NewLedger(
			core.LedgerConfig{FunctionalCurrency: codeUSD, RateTolerance: decimal.Zero},
			mockRateProvider{rates: map[string]decimal.Decimal{
				pairEURUSD: mustLedgerDecimal(t, ledgerRateNear),
			}},
		)
		if err != nil {
			t.Fatalf("NewLedger: %v", err)
		}
		debit, credit := mustEURUSDEntries(t)
		if _, err := validateLedger(t, ledger, "tx-tol", ledgerEffectiveDate(), debit, credit); !errors.Is(err, pkgerr.ErrRateDeviation) {
			t.Errorf("zero tolerance error = %v; want %v", err, pkgerr.ErrRateDeviation)
		}

		exact, err := core.NewLedger(
			core.LedgerConfig{FunctionalCurrency: codeUSD, RateTolerance: decimal.Zero},
			mockRateProvider{rates: map[string]decimal.Decimal{
				pairEURUSD: mustLedgerDecimal(t, ledgerRate),
			}},
		)
		if err != nil {
			t.Fatalf("NewLedger exact: %v", err)
		}
		if _, err := validateLedger(t, exact, "tx-tol-exact", ledgerEffectiveDate(), debit, credit); err != nil {
			t.Errorf("exact provider rate error = %v; want nil", err)
		}
	})

	t.Run("positive tolerance accepts small deviation", func(t *testing.T) {
		ledger, err := core.NewLedger(
			core.LedgerConfig{
				FunctionalCurrency: codeUSD,
				RateTolerance:      mustLedgerDecimal(t, ledgerToleranceSmall),
			},
			mockRateProvider{rates: map[string]decimal.Decimal{
				pairEURUSD: mustLedgerDecimal(t, ledgerRateWithin),
			}},
		)
		if err != nil {
			t.Fatalf("NewLedger: %v", err)
		}
		debit, credit := mustEURUSDEntries(t)
		if _, err := validateLedger(t, ledger, "tx-tol-ok", ledgerEffectiveDate(), debit, credit); err != nil {
			t.Errorf("within-tolerance error = %v; want nil", err)
		}
	})
}

// TestLedgerConfigClosedDate.
func TestLedgerConfigClosedDate(t *testing.T) {
	t.Run("zero closed date accepted", testZeroClosedDate)
	t.Run("non-UTC closed date normalised", testNonUTCClosedDate)
	t.Run("period check compares by instant", testPeriodComparesByInstant)
}

func testZeroClosedDate(t *testing.T) {
	t.Helper()
	config := core.LedgerConfig{FunctionalCurrency: codeUSD}
	if err := config.Validate(); err != nil {
		t.Errorf("Validate() = %v; want nil", err)
	}
	ledger, err := core.NewLedger(config, mockRateProvider{})
	if err != nil {
		t.Fatalf("NewLedger: %v", err)
	}
	if !ledger.Config().ClosedDate.IsZero() {
		t.Errorf("Config().ClosedDate = %s; want zero", ledger.Config().ClosedDate)
	}

	debit, credit := mustUSDPairEntries(t)
	effective := time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
	if _, err := validateLedger(t, ledger, "tx-1970", effective, debit, credit); err != nil {
		t.Errorf("1970 transaction with zero closed date error = %v; want nil", err)
	}
}

func testNonUTCClosedDate(t *testing.T) {
	t.Helper()
	zone := time.FixedZone("CEST", 2*60*60)
	input := time.Date(2024, 6, 1, 15, 30, 0, 0, zone)
	ledger, err := core.NewLedger(
		core.LedgerConfig{FunctionalCurrency: codeUSD, ClosedDate: input},
		mockRateProvider{},
	)
	if err != nil {
		t.Fatalf("NewLedger: %v", err)
	}
	got := ledger.Config().ClosedDate
	if got.Location() != time.UTC {
		t.Errorf("ClosedDate location = %v; want UTC", got.Location())
	}
	if !got.Equal(input) {
		t.Errorf("ClosedDate instant = %s; want %s", got, input)
	}
	if got.Hour() != 13 || got.Minute() != 30 {
		t.Errorf("ClosedDate wall clock = %02d:%02d UTC; want 13:30", got.Hour(), got.Minute())
	}
}

func testPeriodComparesByInstant(t *testing.T) {
	t.Helper()
	closed := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	ledger, err := core.NewLedger(
		core.LedgerConfig{FunctionalCurrency: codeUSD, ClosedDate: closed},
		mockRateProvider{},
	)
	if err != nil {
		t.Fatalf("NewLedger: %v", err)
	}
	debit, credit := mustUSDPairEntries(t)

	zone := time.FixedZone("CEST", 2*60*60)
	equalInstant := time.Date(2024, 6, 1, 14, 0, 0, 0, zone)
	if _, err := validateLedger(t, ledger, "tx-closed", equalInstant, debit, credit); !errors.Is(err, pkgerr.ErrPeriodClosed) {
		t.Errorf("equal-instant transaction error = %v; want %v", err, pkgerr.ErrPeriodClosed)
	}

	after := closed.Add(time.Nanosecond)
	if _, err := validateLedger(t, ledger, "tx-open", after, debit, credit); err != nil {
		t.Errorf("post-closure transaction error = %v; want nil", err)
	}
}

// TestNewLedger.
func TestNewLedger(t *testing.T) {
	invalidConfig := core.LedgerConfig{FunctionalCurrency: codeXAU}
	validConfig := core.LedgerConfig{FunctionalCurrency: codeUSD}

	t.Run("invalid config", func(t *testing.T) {
		ledger, err := core.NewLedger(invalidConfig, mockRateProvider{})
		if ledger != nil {
			t.Errorf("NewLedger returned non-nil ledger; want nil")
		}
		assertStructuredValidationError(t, err, pkgerr.ErrInvalidConfiguration,
			ledgerFieldConfig, ledgerOpNew)
	})

	t.Run("nil provider", func(t *testing.T) {
		ledger, err := core.NewLedger(validConfig, nil)
		if ledger != nil {
			t.Errorf("NewLedger returned non-nil ledger; want nil")
		}
		assertStructuredValidationError(t, err, pkgerr.ErrNilRateProvider,
			ledgerFieldProvider, ledgerOpNew)
	})

	t.Run("valid config and provider", func(t *testing.T) {
		ledger, err := core.NewLedger(validConfig, mockRateProvider{})
		if err != nil {
			t.Fatalf("NewLedger: %v", err)
		}
		if ledger == nil {
			t.Fatal("NewLedger returned nil ledger")
		}
	})

	t.Run("config error wins over nil provider", func(t *testing.T) {
		ledger, err := core.NewLedger(invalidConfig, nil)
		if ledger != nil {
			t.Errorf("NewLedger returned non-nil ledger; want nil")
		}
		if !errors.Is(err, pkgerr.ErrInvalidConfiguration) {
			t.Errorf("error = %v; want ErrInvalidConfiguration", err)
		}
		if errors.Is(err, pkgerr.ErrNilRateProvider) {
			t.Error("error matched ErrNilRateProvider; want config error first")
		}
	})
}

// TestUpdateConfigAndConfigCopy.
func TestUpdateConfigAndConfigCopy(t *testing.T) {
	originalConfig := core.LedgerConfig{FunctionalCurrency: codeUSD}
	provider := mockRateProvider{rates: map[string]decimal.Decimal{
		pairEURUSD: mustLedgerDecimal(t, ledgerRate),
		"USD/EUR":  mustLedgerDecimal(t, ledgerRateUSDToEUR),
	}}
	ledger, err := core.NewLedger(originalConfig, provider)
	if err != nil {
		t.Fatalf("NewLedger: %v", err)
	}

	assertConfigCopyIndependent(t, ledger)
	updated := mustUpdateConfigNormalised(t, ledger)

	t.Run("same provider retained", func(t *testing.T) {
		assertUpdatedUsesProvider(t, updated)
	})
	t.Run("invalid update leaves original usable", func(t *testing.T) {
		assertInvalidUpdateLeavesOriginal(t, ledger)
	})
}

func assertConfigCopyIndependent(t *testing.T, ledger *core.Ledger) {
	t.Helper()
	cfg := ledger.Config()
	cfg.FunctionalCurrency = codeEUR
	cfg.RateTolerance = mustLedgerDecimal(t, ledgerToleranceNeg)
	cfg.ClosedDate = time.Now()

	if got := ledger.Config().FunctionalCurrency; got != codeUSD {
		t.Errorf("mutating Config() copy changed ledger currency to %q", got)
	}
	if ledger.Config().RateTolerance.IsNegative() || !ledger.Config().RateTolerance.IsZero() {
		t.Errorf("mutating Config() copy changed ledger tolerance to %s", ledger.Config().RateTolerance)
	}
	if !ledger.Config().ClosedDate.IsZero() {
		t.Errorf("mutating Config() copy changed ClosedDate to %s", ledger.Config().ClosedDate)
	}
}

func mustUpdateConfigNormalised(t *testing.T, ledger *core.Ledger) *core.Ledger {
	t.Helper()
	newConfig := core.LedgerConfig{
		FunctionalCurrency: "eur",
		RateTolerance:      mustLedgerDecimal(t, ledgerToleranceSmall),
	}
	updated, err := ledger.UpdateConfig(newConfig)
	if err != nil {
		t.Fatalf("UpdateConfig: %v", err)
	}
	if updated == nil {
		t.Fatal("UpdateConfig returned nil ledger")
	}
	if got := updated.Config().FunctionalCurrency; got != codeEUR {
		t.Errorf("updated currency = %q; want %q", got, codeEUR)
	}
	if !updated.Config().RateTolerance.Equal(mustLedgerDecimal(t, ledgerToleranceSmall)) {
		t.Errorf("updated tolerance = %s; want %s",
			updated.Config().RateTolerance, ledgerToleranceSmall)
	}
	if got := ledger.Config().FunctionalCurrency; got != codeUSD {
		t.Errorf("original currency = %q; want %q after UpdateConfig", got, codeUSD)
	}
	if ledger.Config().RateTolerance.IsNegative() || !ledger.Config().RateTolerance.IsZero() {
		t.Errorf("original tolerance = %s; want zero", ledger.Config().RateTolerance)
	}
	return updated
}

func assertUpdatedUsesProvider(t *testing.T, updated *core.Ledger) {
	t.Helper()
	debit, err := core.NewEntry(
		"acc-usd",
		mustLedgerDecimal(t, txAmountHundred),
		codeUSD,
		mustLedgerDecimal(t, ledgerRateUSDToEUR),
		codeEUR,
	)
	if err != nil {
		t.Fatalf("NewEntry USD debit: %v", err)
	}
	credit, err := core.NewEntry(
		"acc-eur",
		mustLedgerDecimal(t, ledgerAmountNeg90),
		codeEUR,
		mustLedgerDecimal(t, txRateOne),
		codeEUR,
	)
	if err != nil {
		t.Fatalf("NewEntry EUR credit: %v", err)
	}
	if _, err := validateLedger(t, updated, "tx-updated", ledgerEffectiveDate(), debit, credit); err != nil {
		t.Errorf("updated ledger transaction error = %v; want nil", err)
	}
}

func assertInvalidUpdateLeavesOriginal(t *testing.T, ledger *core.Ledger) {
	t.Helper()
	rejected, err := ledger.UpdateConfig(core.LedgerConfig{FunctionalCurrency: codeXAU})
	if rejected != nil {
		t.Errorf("UpdateConfig returned non-nil ledger for invalid config")
	}
	assertStructuredValidationError(t, err, pkgerr.ErrInvalidConfiguration,
		ledgerFieldConfig, ledgerOpUpdate)

	if got := ledger.Config().FunctionalCurrency; got != codeUSD {
		t.Errorf("original currency = %q; want %q", got, codeUSD)
	}
	debit, credit := mustUSDPairEntries(t)
	if _, err := validateLedger(t, ledger, "tx-original", ledgerEffectiveDate(), debit, credit); err != nil {
		t.Errorf("original ledger after failed update error = %v; want nil", err)
	}
}

// TestLedgerImmutability.
func TestLedgerImmutability(t *testing.T) {
	typ := reflect.TypeOf(core.Ledger{})
	if typ.Kind() != reflect.Struct {
		t.Fatalf("Ledger kind = %v; want struct", typ.Kind())
	}
	if typ.NumField() == 0 {
		t.Fatal("Ledger has no fields")
	}
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath == "" {
			t.Errorf("Ledger field %s is exported; want unexported", field.Name)
		}
	}

	config := core.LedgerConfig{
		ClosedDate:         time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC),
		FunctionalCurrency: codeUSD,
		RateTolerance:      mustLedgerDecimal(t, ledgerToleranceSmall),
	}
	ledger, err := core.NewLedger(config, mockRateProvider{})
	if err != nil {
		t.Fatalf("NewLedger: %v", err)
	}

	const readers = 16
	var wg sync.WaitGroup
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got := ledger.Config()
			if got.FunctionalCurrency != codeUSD {
				t.Errorf("concurrent Config().FunctionalCurrency = %q; want %q", got.FunctionalCurrency, codeUSD)
			}
			if !got.ClosedDate.Equal(config.ClosedDate) {
				t.Errorf("concurrent Config().ClosedDate = %s; want %s", got.ClosedDate, config.ClosedDate)
			}
			if !got.RateTolerance.Equal(config.RateTolerance) {
				t.Errorf("concurrent Config().RateTolerance = %s; want %s", got.RateTolerance, config.RateTolerance)
			}
		}()
	}
	wg.Wait()
}

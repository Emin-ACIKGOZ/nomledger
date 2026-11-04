// SPDX-License-Identifier: MIT

package errors_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Emin-ACIKGOZ/nomledger/pkg/core"
	"github.com/Emin-ACIKGOZ/nomledger/pkg/currency"
	pkgerr "github.com/Emin-ACIKGOZ/nomledger/pkg/errors"
	"github.com/Emin-ACIKGOZ/nomledger/pkg/interfaces"

	"github.com/shopspring/decimal"
)

const taxonomyAccountID = "taxonomy-account"

func taxonomyDecimal(t *testing.T, value string) decimal.Decimal {
	t.Helper()
	d, err := decimal.NewFromString(value)
	if err != nil {
		t.Fatalf("decimal.NewFromString(%q): %v", value, err)
	}
	return d
}

func taxonomyDate() time.Time {
	return time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
}

func taxonomyConfig() core.LedgerConfig {
	return core.LedgerConfig{
		FunctionalCurrency: "USD",
		RateTolerance:      decimal.Zero,
	}
}

func taxonomyProvider(rate string, providerErr error) interfaces.RateProvider {
	return interfaces.RateProviderFunc(func(context.Context, interfaces.RateRequest) (decimal.Decimal, error) {
		if providerErr != nil {
			return decimal.Zero, providerErr
		}
		return taxonomyDecimalForProvider(rate), nil
	})
}

func taxonomyDecimalForProvider(value string) decimal.Decimal {
	d, err := decimal.NewFromString(value)
	if err != nil {
		panic(err)
	}
	return d
}

func taxonomyEntry(t *testing.T, accountID, amount, txCurrency, rate, functionalCurrency string) core.Entry {
	t.Helper()
	entry, err := core.NewEntry(
		accountID,
		taxonomyDecimal(t, amount),
		txCurrency,
		taxonomyDecimal(t, rate),
		functionalCurrency,
	)
	if err != nil {
		t.Fatalf("NewEntry(%s, %s, %s, %s, %s): %v",
			accountID, amount, txCurrency, rate, functionalCurrency, err)
	}
	return entry
}

func taxonomyUSDEntries(t *testing.T) []core.Entry {
	t.Helper()
	return []core.Entry{
		taxonomyEntry(t, "debit", "100.00", "USD", "1", "USD"),
		taxonomyEntry(t, "credit", "-100.00", "USD", "1", "USD"),
	}
}

func taxonomyBuilder(t *testing.T, config core.LedgerConfig, provider interfaces.RateProvider, entries ...core.Entry) *core.TxBuilder {
	t.Helper()
	builder := core.NewTxBuilder("taxonomy-tx", taxonomyDate(), config, provider)
	for _, entry := range entries {
		builder.AddEntry(entry)
	}
	return builder
}

func taxonomyLedger(t *testing.T, provider interfaces.RateProvider) *core.Ledger {
	t.Helper()
	clock := core.ClockFunc(func() time.Time { return taxonomyDate().Add(time.Hour) })
	ledger, err := core.NewLedger(taxonomyConfig(), provider, core.WithClock(clock))
	if err != nil {
		t.Fatalf("NewLedger: %v", err)
	}
	return ledger
}

type taxonomyCase struct {
	id             string
	name           string
	trigger        func(t *testing.T) error
	want           error
	wantAlso       error
	wantEntryIndex int
}

// TestErrorTaxonomy covers every failure code ERR-001 through ERR-026.
func TestErrorTaxonomy(t *testing.T) {
	provider := taxonomyProvider("1", nil)
	cases := []taxonomyCase{
		{
			id: "ERR-001", name: "invalid currency code",
			trigger: func(_ *testing.T) error {
				_, err := currency.Lookup("US")
				return err
			},
			want: pkgerr.ErrInvalidCurrencyCode, wantEntryIndex: -1,
		},
		{
			id: "ERR-002", name: "unknown currency",
			trigger: func(_ *testing.T) error {
				_, err := currency.Lookup("BGN")
				return err
			},
			want: pkgerr.ErrUnknownCurrency, wantEntryIndex: -1,
		},
		{
			id: "ERR-003", name: "non-ratable currency",
			trigger: func(_ *testing.T) error {
				_, err := currency.Round(decimal.NewFromInt(1), "XAU", currency.RoundHalfEven)
				return err
			},
			want: pkgerr.ErrCurrencyNotRatable, wantEntryIndex: -1,
		},
		{
			id: "ERR-004", name: "invalid precision",
			trigger: func(t *testing.T) error {
				_, err := currency.NewMoney(taxonomyDecimal(t, "1.001"), "USD")
				return err
			},
			want: pkgerr.ErrInvalidPrecision, wantEntryIndex: -1,
		},
		{
			id: "ERR-005", name: "zero amount",
			trigger: func(_ *testing.T) error {
				_, err := core.NewEntry("acc", decimal.Zero, "USD", decimal.NewFromInt(1), "USD")
				return err
			},
			want: pkgerr.ErrZeroAmount, wantEntryIndex: -1,
		},
		{
			id: "ERR-006", name: "functional amount zero",
			trigger: func(t *testing.T) error {
				_, err := core.NewEntry("acc", taxonomyDecimal(t, "0.001"), "BHD", decimal.NewFromInt(1), "USD")
				return err
			},
			want: pkgerr.ErrFunctionalAmountZero, wantEntryIndex: -1,
		},
		{
			id: "ERR-007", name: "zero rate",
			trigger: func(_ *testing.T) error {
				_, err := core.NewEntry("acc", decimal.NewFromInt(1), "USD", decimal.Zero, "USD")
				return err
			},
			want: pkgerr.ErrInvalidExchangeRate, wantEntryIndex: -1,
		},
		{
			id: "ERR-008", name: "identity rate not one",
			trigger: func(t *testing.T) error {
				_, err := core.NewEntry("acc", decimal.NewFromInt(1), "USD", taxonomyDecimal(t, "1.1"), "USD")
				return err
			},
			want: pkgerr.ErrInvalidExchangeRate, wantEntryIndex: -1,
		},
		{
			id: "ERR-009", name: "invalid account ID",
			trigger: func(_ *testing.T) error {
				_, err := core.NewAccount("", core.Asset, core.NotApplicable, "USD", nil)
				return err
			},
			want: pkgerr.ErrInvalidAccountID, wantEntryIndex: -1,
		},
		{
			id: "ERR-010", name: "invalid transaction ID",
			trigger: func(t *testing.T) error {
				builder := core.NewTxBuilder("", taxonomyDate(), taxonomyConfig(), provider)
				for _, entry := range taxonomyUSDEntries(t) {
					builder.AddEntry(entry)
				}
				_, err := builder.Build()
				return err
			},
			want: pkgerr.ErrInvalidTransactionID, wantEntryIndex: -1,
		},
		{
			id: "ERR-011", name: "invalid effective date",
			trigger: func(t *testing.T) error {
				builder := core.NewTxBuilder("tx", time.Time{}, taxonomyConfig(), provider)
				for _, entry := range taxonomyUSDEntries(t) {
					builder.AddEntry(entry)
				}
				_, err := builder.Build()
				return err
			},
			want: pkgerr.ErrInvalidEffectiveDate, wantEntryIndex: -1,
		},
		{
			id: "ERR-012", name: "empty transaction",
			trigger: func(t *testing.T) error {
				_, err := taxonomyBuilder(t, taxonomyConfig(), provider).Build()
				return err
			},
			want: pkgerr.ErrEmptyTransaction, wantEntryIndex: -1,
		},
		{
			id: "ERR-013", name: "currency mismatch",
			trigger: func(t *testing.T) error {
				entry := taxonomyEntry(t, "eur", "100.00", "EUR", "1", "EUR")
				_, err := taxonomyBuilder(t, taxonomyConfig(), provider, entry).Build()
				return err
			},
			want: pkgerr.ErrCurrencyMismatch, wantEntryIndex: 0,
		},
		{
			id: "ERR-014", name: "zero sum violation",
			trigger: func(t *testing.T) error {
				entries := []core.Entry{
					taxonomyEntry(t, "debit", "100.00", "USD", "1", "USD"),
					taxonomyEntry(t, "credit", "-99.00", "USD", "1", "USD"),
				}
				_, err := taxonomyBuilder(t, taxonomyConfig(), provider, entries...).Build()
				return err
			},
			want: pkgerr.ErrZeroSumViolation, wantEntryIndex: -1,
		},
		{
			id: "ERR-015", name: "period closed",
			trigger: func(t *testing.T) error {
				config := taxonomyConfig()
				config.ClosedDate = taxonomyDate()
				_, err := taxonomyBuilder(t, config, provider, taxonomyUSDEntries(t)...).Build()
				return err
			},
			want: pkgerr.ErrPeriodClosed, wantEntryIndex: -1,
		},
		{
			id: "ERR-016", name: "rate deviation",
			trigger: func(t *testing.T) error {
				entries := []core.Entry{
					taxonomyEntry(t, "eur", "100.00", "EUR", "1.1", "USD"),
					taxonomyEntry(t, "usd", "-110.00", "USD", "1", "USD"),
				}
				_, err := taxonomyBuilder(t, taxonomyConfig(), taxonomyProvider("1.5", nil), entries...).Build()
				return err
			},
			want: pkgerr.ErrRateDeviation, wantEntryIndex: 0,
		},
		{
			id: "ERR-017", name: "provider error",
			trigger: func(t *testing.T) error {
				entries := []core.Entry{
					taxonomyEntry(t, "eur", "100.00", "EUR", "1.1", "USD"),
					taxonomyEntry(t, "usd", "-110.00", "USD", "1", "USD"),
				}
				_, err := taxonomyBuilder(t, taxonomyConfig(), taxonomyProvider("1", errors.New("provider down")), entries...).Build()
				return err
			},
			want: pkgerr.ErrRateProvider, wantEntryIndex: 0,
		},
		{
			id: "ERR-018", name: "context canceled",
			trigger: func(t *testing.T) error {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				ledger := taxonomyLedger(t, provider)
				_, err := ledger.ValidateTransaction(ctx, core.TransactionInput{
					ID:            "tx",
					EffectiveDate: taxonomyDate(),
					Entries:       taxonomyUSDEntries(t),
				})
				return err
			},
			want: pkgerr.ErrRateProvider, wantAlso: context.Canceled, wantEntryIndex: -1,
		},
		{
			id: "ERR-019", name: "invalid configuration",
			trigger: func(_ *testing.T) error {
				_, err := core.NewLedger(core.LedgerConfig{}, provider)
				return err
			},
			want: pkgerr.ErrInvalidConfiguration, wantEntryIndex: -1,
		},
		{
			id: "ERR-020", name: "nil provider",
			trigger: func(_ *testing.T) error {
				_, err := core.NewLedger(taxonomyConfig(), nil)
				return err
			},
			want: pkgerr.ErrNilRateProvider, wantEntryIndex: -1,
		},
		{
			id: "ERR-021", name: "builder finalized",
			trigger: func(t *testing.T) error {
				builder := taxonomyBuilder(t, taxonomyConfig(), provider, taxonomyUSDEntries(t)...)
				if _, err := builder.Build(); err != nil {
					t.Fatalf("first Build: %v", err)
				}
				_, err := builder.Build()
				return err
			},
			want: pkgerr.ErrBuilderFinalized, wantEntryIndex: -1,
		},
		{
			id: "ERR-022", name: "invalid metadata",
			trigger: func(t *testing.T) error {
				builder := taxonomyBuilder(t, taxonomyConfig(), provider, taxonomyUSDEntries(t)...)
				builder.WithMetadata(map[string]any{"": true})
				_, err := builder.Build()
				return err
			},
			want: pkgerr.ErrInvalidMetadata, wantEntryIndex: -1,
		},
		{
			id: "ERR-023", name: "invalid account type",
			trigger: func(_ *testing.T) error {
				_, err := core.NewAccount(taxonomyAccountID, core.AccountType(99), core.NotApplicable, "USD", nil)
				return err
			},
			want: pkgerr.ErrInvalidAccountType, wantEntryIndex: -1,
		},
		{
			id: "ERR-023", name: "invalid measurement category",
			trigger: func(_ *testing.T) error {
				_, err := core.NewAccount(taxonomyAccountID, core.Asset, core.MeasurementCategory(99), "USD", nil)
				return err
			},
			want: pkgerr.ErrInvalidMeasurementCategory, wantEntryIndex: -1,
		},
		{
			id: "ERR-024", name: invalidRoundingModeText,
			trigger: func(_ *testing.T) error {
				_, err := currency.Round(decimal.NewFromInt(1), "USD", currency.RoundingMode(99))
				return err
			},
			want: pkgerr.ErrInvalidRoundingMode, wantEntryIndex: -1,
		},
		{
			id: "ERR-025", name: "inexact unnecessary rounding",
			trigger: func(_ *testing.T) error {
				_, err := currency.Round(taxonomyDecimal(t, "1.231"), "USD", currency.RoundUnnecessary)
				return err
			},
			want: pkgerr.ErrInexactRounding, wantEntryIndex: -1,
		},
		{
			id: "ERR-026", name: "invalid entry",
			trigger: func(t *testing.T) error {
				_, err := taxonomyBuilder(t, taxonomyConfig(), provider, core.Entry{}).Build()
				return err
			},
			want: pkgerr.ErrInvalidEntry, wantEntryIndex: 0,
		},
	}

	seen := make(map[string]bool)
	for _, tc := range cases {
		seen[tc.id] = true
		t.Run(tc.id+" "+tc.name, func(t *testing.T) {
			assertTaxonomyCase(t, tc)
		})
	}

	for i := 1; i <= 26; i++ {
		id := fmt.Sprintf("ERR-%03d", i)
		if !seen[id] {
			t.Errorf("taxonomy table missing %s", id)
		}
	}
}

func assertTaxonomyCase(t *testing.T, tc taxonomyCase) {
	t.Helper()
	err := tc.trigger(t)
	if err == nil {
		t.Fatal("trigger returned nil error")
	}
	if !errors.Is(err, tc.want) {
		t.Fatalf("error = %v; want %v", err, tc.want)
	}
	if tc.wantAlso != nil && !errors.Is(err, tc.wantAlso) {
		t.Errorf("error = %v; want also %v", err, tc.wantAlso)
	}

	var validationErr *pkgerr.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("errors.As(err, *ValidationError) = false; err = %v", err)
	}
	if validationErr.Operation == "" {
		t.Error("Operation is empty")
	}
	if validationErr.Field == "" {
		t.Error("Field is empty")
	}
	if validationErr.EntryIndex != tc.wantEntryIndex {
		t.Errorf("EntryIndex = %d; want %d", validationErr.EntryIndex, tc.wantEntryIndex)
	}
}

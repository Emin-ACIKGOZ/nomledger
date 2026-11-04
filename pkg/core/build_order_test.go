// SPDX-License-Identifier: MIT

package core_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/Emin-ACIKGOZ/nomledger/pkg/core"
	pkgerr "github.com/Emin-ACIKGOZ/nomledger/pkg/errors"

	"github.com/shopspring/decimal"
)

func assertBuildValidationError(t *testing.T, err error, want error, field string, entryIndex int) {
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

func assertZeroTransaction(t *testing.T, transaction core.Transaction) {
	t.Helper()
	if !reflect.DeepEqual(transaction, core.Transaction{}) {
		t.Errorf("Build returned non-zero Transaction: %+v", transaction)
	}
}

func invalidConfig() core.LedgerConfig {
	return core.LedgerConfig{FunctionalCurrency: ""}
}

func periodClosedConfig() core.LedgerConfig {
	return core.LedgerConfig{
		ClosedDate:         ledgerEffectiveDate(),
		FunctionalCurrency: codeUSD,
	}
}

func unbalancedEntries(t *testing.T) []core.Entry {
	t.Helper()
	return []core.Entry{
		mustTxEntry(t, txDebitAccount, txAmountHundred),
		mustTxEntry(t, txCreditAccount, "-99.00"),
	}
}

// TestBuildAtomicFailure: every failure category returns the zero
// Transaction alongside a non-nil error.
func TestBuildAtomicFailure(t *testing.T) {
	closed := periodClosedConfig()
	validDate := ledgerEffectiveDate()

	cases := []struct {
		name  string
		build func() (core.Transaction, error)
	}{
		{
			name: "config",
			build: func() (core.Transaction, error) {
				return core.NewTxBuilder(txID, validDate, invalidConfig(), mockRateProvider{}).Build()
			},
		},
		{
			name: "transaction ID",
			build: func() (core.Transaction, error) {
				builder := core.NewTxBuilder("", validDate, txTestConfig(), mockRateProvider{})
				for _, entry := range transactionFieldEntries(t) {
					builder.AddEntry(entry)
				}
				return builder.Build()
			},
		},
		{
			name: "effective date",
			build: func() (core.Transaction, error) {
				builder := core.NewTxBuilder(txID, time.Time{}, txTestConfig(), mockRateProvider{})
				for _, entry := range transactionFieldEntries(t) {
					builder.AddEntry(entry)
				}
				return builder.Build()
			},
		},
		{
			name: "empty entries",
			build: func() (core.Transaction, error) {
				return core.NewTxBuilder(txID, validDate, txTestConfig(), mockRateProvider{}).Build()
			},
		},
		{
			name: "metadata keys",
			build: func() (core.Transaction, error) {
				builder := core.NewTxBuilder(txID, validDate, txTestConfig(), mockRateProvider{})
				builder.WithMetadata(map[string]any{"": true})
				for _, entry := range transactionFieldEntries(t) {
					builder.AddEntry(entry)
				}
				return builder.Build()
			},
		},
		{
			name: "entry validity",
			build: func() (core.Transaction, error) {
				builder := core.NewTxBuilder(txID, validDate, txTestConfig(), mockRateProvider{})
				builder.AddEntry(core.Entry{})
				return builder.Build()
			},
		},
		{
			name: "currency consistency",
			build: func() (core.Transaction, error) {
				builder := core.NewTxBuilder(txID, validDate, txTestConfig(), mockRateProvider{})
				builder.AddEntry(mustRateEntry(t, "eur-acc", txAmountHundred, codeEUR, txRateOne, codeEUR))
				return builder.Build()
			},
		},
		{
			name: "zero sum",
			build: func() (core.Transaction, error) {
				builder := core.NewTxBuilder(txID, validDate, txTestConfig(), mockRateProvider{})
				for _, entry := range unbalancedEntries(t) {
					builder.AddEntry(entry)
				}
				return builder.Build()
			},
		},
		{
			name: "period",
			build: func() (core.Transaction, error) {
				builder := core.NewTxBuilder(txID, validDate, closed, mockRateProvider{})
				for _, entry := range transactionFieldEntries(t) {
					builder.AddEntry(entry)
				}
				return builder.Build()
			},
		},
		{
			name: "rates",
			build: func() (core.Transaction, error) {
				provider := &recordingRateProvider{err: errors.New("rate failure")}
				builder := core.NewTxBuilder(txID, validDate, txTestConfig(), provider)
				debit, credit := mustEURUSDEntries(t)
				builder.AddEntry(debit).AddEntry(credit)
				return builder.Build()
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			transaction, err := tc.build()
			if err == nil {
				t.Fatal("Build returned nil error")
			}
			assertZeroTransaction(t, transaction)
		})
	}
}

// TestValidationOrderPrecedence: the earlier check wins for
// double-fault inputs.
func TestValidationOrderPrecedence(t *testing.T) {
	validDate := ledgerEffectiveDate()
	closed := periodClosedConfig()

	cases := []struct {
		name      string
		build     func() (core.Transaction, error)
		wantErr   error
		wantField string
		wantIndex int
	}{
		{
			name: "config before ID",
			build: func() (core.Transaction, error) {
				return core.NewTxBuilder("", validDate, invalidConfig(), mockRateProvider{}).Build()
			},
			wantErr:   pkgerr.ErrInvalidConfiguration,
			wantField: "config",
			wantIndex: -1,
		},
		{
			name: "ID before date",
			build: func() (core.Transaction, error) {
				builder := core.NewTxBuilder("", time.Time{}, txTestConfig(), mockRateProvider{})
				for _, entry := range transactionFieldEntries(t) {
					builder.AddEntry(entry)
				}
				return builder.Build()
			},
			wantErr:   pkgerr.ErrInvalidTransactionID,
			wantField: "id",
			wantIndex: -1,
		},
		{
			name: "date before entries",
			build: func() (core.Transaction, error) {
				return core.NewTxBuilder(txID, time.Time{}, txTestConfig(), mockRateProvider{}).Build()
			},
			wantErr:   pkgerr.ErrInvalidEffectiveDate,
			wantField: "effectiveDate",
			wantIndex: -1,
		},
		{
			name: "entries before metadata",
			build: func() (core.Transaction, error) {
				builder := core.NewTxBuilder(txID, validDate, txTestConfig(), mockRateProvider{})
				builder.WithMetadata(map[string]any{"": true})
				return builder.Build()
			},
			wantErr:   pkgerr.ErrEmptyTransaction,
			wantField: "entries",
			wantIndex: -1,
		},
		{
			name: "metadata before entry validity",
			build: func() (core.Transaction, error) {
				builder := core.NewTxBuilder(txID, validDate, txTestConfig(), mockRateProvider{})
				builder.WithMetadata(map[string]any{"": true})
				builder.AddEntry(core.Entry{})
				return builder.Build()
			},
			wantErr:   pkgerr.ErrInvalidMetadata,
			wantField: "metadata",
			wantIndex: -1,
		},
		{
			name: "entry validity before currency",
			build: func() (core.Transaction, error) {
				builder := core.NewTxBuilder(txID, validDate, txTestConfig(), mockRateProvider{})
				builder.AddEntry(core.Entry{})
				return builder.Build()
			},
			wantErr:   pkgerr.ErrInvalidEntry,
			wantField: "entry",
			wantIndex: 0,
		},
		{
			name: "currency before zero sum",
			build: func() (core.Transaction, error) {
				builder := core.NewTxBuilder(txID, validDate, txTestConfig(), mockRateProvider{})
				builder.AddEntry(mustRateEntry(t, "eur-acc", txAmountHundred, codeEUR, txRateOne, codeEUR))
				builder.AddEntry(mustTxEntry(t, txDebitAccount, "-1.00"))
				return builder.Build()
			},
			wantErr:   pkgerr.ErrCurrencyMismatch,
			wantField: "functionalCurrency",
			wantIndex: 0,
		},
		{
			name: "zero sum before period",
			build: func() (core.Transaction, error) {
				builder := core.NewTxBuilder(txID, validDate, closed, mockRateProvider{})
				for _, entry := range unbalancedEntries(t) {
					builder.AddEntry(entry)
				}
				return builder.Build()
			},
			wantErr:   pkgerr.ErrZeroSumViolation,
			wantField: "entries",
			wantIndex: -1,
		},
		{
			name: "period before rates",
			build: func() (core.Transaction, error) {
				provider := &recordingRateProvider{rates: map[string]decimal.Decimal{
					pairEURUSD: mustTxDecimal(t, "1.5"),
				}}
				builder := core.NewTxBuilder(txID, validDate, closed, provider)
				debit, credit := mustEURUSDEntries(t)
				builder.AddEntry(debit).AddEntry(credit)
				return builder.Build()
			},
			wantErr:   pkgerr.ErrPeriodClosed,
			wantField: "effectiveDate",
			wantIndex: -1,
		},
		{
			name: "first rate fault wins",
			build: func() (core.Transaction, error) {
				provider := &recordingRateProvider{rates: map[string]decimal.Decimal{
					pairEURUSD: mustTxDecimal(t, "1.5"),
				}}
				builder := core.NewTxBuilder(txID, validDate, txTestConfig(), provider)
				builder.AddEntry(mustRateEntry(t, "eur-acc", txAmountHundred, codeEUR, ledgerRate, codeUSD))
				builder.AddEntry(mustRateEntry(t, "gbp-acc", txAmountHundred, codeGBP, ledgerRate, codeUSD))
				builder.AddEntry(mustRateEntry(t, "usd-acc", "-220.00", codeUSD, txRateOne, codeUSD))
				return builder.Build()
			},
			wantErr:   pkgerr.ErrRateDeviation,
			wantField: "rate",
			wantIndex: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			transaction, err := tc.build()
			if err == nil {
				t.Fatal("Build returned nil error")
			}
			assertZeroTransaction(t, transaction)
			assertBuildValidationError(t, err, tc.wantErr, tc.wantField, tc.wantIndex)
		})
	}
}

// TestBuildEntryValidity covers zero-value entries at each position.
func TestBuildEntryValidity(t *testing.T) {
	t.Run("zero entry at index 0", func(t *testing.T) {
		builder := core.NewTxBuilder(txID, ledgerEffectiveDate(), txTestConfig(), mockRateProvider{})
		builder.AddEntry(core.Entry{})
		_, err := builder.Build()
		assertBuildValidationError(t, err, pkgerr.ErrInvalidEntry, "entry", 0)
	})

	t.Run("zero entry after a valid entry", func(t *testing.T) {
		builder := core.NewTxBuilder(txID, ledgerEffectiveDate(), txTestConfig(), mockRateProvider{})
		builder.AddEntry(mustTxEntry(t, txDebitAccount, txAmountHundred))
		builder.AddEntry(core.Entry{})
		_, err := builder.Build()
		assertBuildValidationError(t, err, pkgerr.ErrInvalidEntry, "entry", 1)
	})
}

// TestNoProviderCallsAfterEarlierFailure verifies the provider is not reached
// for any pre-rate failure.
func TestNoProviderCallsAfterEarlierFailure(t *testing.T) {
	validDate := ledgerEffectiveDate()
	closed := periodClosedConfig()

	cases := []struct {
		name    string
		build   func(provider *recordingRateProvider) (core.Transaction, error)
		wantErr error
	}{
		{
			name: "metadata keys",
			build: func(provider *recordingRateProvider) (core.Transaction, error) {
				builder := core.NewTxBuilder(txID, validDate, txTestConfig(), provider)
				builder.WithMetadata(map[string]any{"": true})
				for _, entry := range transactionFieldEntries(t) {
					builder.AddEntry(entry)
				}
				return builder.Build()
			},
			wantErr: pkgerr.ErrInvalidMetadata,
		},
		{
			name: "entry validity",
			build: func(provider *recordingRateProvider) (core.Transaction, error) {
				builder := core.NewTxBuilder(txID, validDate, txTestConfig(), provider)
				builder.AddEntry(core.Entry{})
				return builder.Build()
			},
			wantErr: pkgerr.ErrInvalidEntry,
		},
		{
			name: "currency consistency",
			build: func(provider *recordingRateProvider) (core.Transaction, error) {
				builder := core.NewTxBuilder(txID, validDate, txTestConfig(), provider)
				builder.AddEntry(mustRateEntry(t, "eur-acc", txAmountHundred, codeEUR, txRateOne, codeEUR))
				return builder.Build()
			},
			wantErr: pkgerr.ErrCurrencyMismatch,
		},
		{
			name: "zero sum",
			build: func(provider *recordingRateProvider) (core.Transaction, error) {
				builder := core.NewTxBuilder(txID, validDate, txTestConfig(), provider)
				for _, entry := range unbalancedEntries(t) {
					builder.AddEntry(entry)
				}
				return builder.Build()
			},
			wantErr: pkgerr.ErrZeroSumViolation,
		},
		{
			name: "period",
			build: func(provider *recordingRateProvider) (core.Transaction, error) {
				builder := core.NewTxBuilder(txID, validDate, closed, provider)
				debit, credit := mustEURUSDEntries(t)
				builder.AddEntry(debit).AddEntry(credit)
				return builder.Build()
			},
			wantErr: pkgerr.ErrPeriodClosed,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			provider := &recordingRateProvider{}
			_, err := tc.build(provider)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v; want %v", err, tc.wantErr)
			}
			if provider.calls() != 0 {
				t.Errorf("provider calls = %d; want 0", provider.calls())
			}
		})
	}
}

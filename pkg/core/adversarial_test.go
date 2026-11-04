// SPDX-License-Identifier: MIT

package core_test

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Emin-ACIKGOZ/nomledger/pkg/core"
	pkgerr "github.com/Emin-ACIKGOZ/nomledger/pkg/errors"

	"github.com/shopspring/decimal"
)

func callWithoutPanic(t *testing.T, fn func() error) (err error) {
	t.Helper()
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("unexpected panic: %v", recovered)
		}
	}()
	return fn()
}

type adversarialCase struct {
	name    string
	call    func() error
	wantErr error
}

// TestAdversarialInputs drives zero values, malformed fields, nil maps/context,
// and representative validation faults without permitting panics.
//
//nolint:cyclop // table-driven adversarial matrix of independent cases
func TestAdversarialInputs(t *testing.T) {
	ledger := mustValidLedger(t)
	input := validTransactionInput(t)

	cases := []adversarialCase{
		{
			name: "zero-value builder",
			call: func() error {
				var builder core.TxBuilder
				_, err := builder.Build()
				return err
			},
			wantErr: pkgerr.ErrInvalidConfiguration,
		},
		{
			name: "adversarial empty entries",
			call: func() error {
				_, err := coreNewBuilder(t).Build()
				return err
			},
			wantErr: pkgerr.ErrEmptyTransaction,
		},
		{
			name: "empty transaction ID",
			call: func() error {
				builder := core.NewTxBuilder("", ledgerEffectiveDate(), txTestConfig(), mockRateProvider{})
				for _, entry := range transactionFieldEntries(t) {
					builder.AddEntry(entry)
				}
				_, err := builder.Build()
				return err
			},
			wantErr: pkgerr.ErrInvalidTransactionID,
		},
		{
			name: "oversized transaction ID",
			call: func() error {
				builder := core.NewTxBuilder(strings.Repeat("a", overMaxIDBytes), ledgerEffectiveDate(), txTestConfig(), mockRateProvider{})
				for _, entry := range transactionFieldEntries(t) {
					builder.AddEntry(entry)
				}
				_, err := builder.Build()
				return err
			},
			wantErr: pkgerr.ErrInvalidTransactionID,
		},
		{
			name: "control transaction ID",
			call: func() error {
				builder := core.NewTxBuilder("tx\x01id", ledgerEffectiveDate(), txTestConfig(), mockRateProvider{})
				for _, entry := range transactionFieldEntries(t) {
					builder.AddEntry(entry)
				}
				_, err := builder.Build()
				return err
			},
			wantErr: pkgerr.ErrInvalidTransactionID,
		},
		{
			name: "nil context succeeds",
			call: func() error {
				_, err := ledger.ValidateTransaction(nil, input) //nolint:staticcheck // adversarial nil-context coverage
				return err
			},
		},
		{
			name: "cancelled context",
			call: func() error {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				_, err := ledger.ValidateTransaction(ctx, input)
				return err
			},
			wantErr: pkgerr.ErrRateProvider,
		},
		{
			name: "nil metadata succeeds",
			call: func() error {
				nilMetadata := input
				nilMetadata.Metadata = nil
				_, err := ledger.ValidateTransaction(context.Background(), nilMetadata)
				return err
			},
		},
		{
			name: "empty metadata key",
			call: func() error {
				builder := coreNewBuilder(t)
				builder.WithMetadata(map[string]any{"": true})
				for _, entry := range transactionFieldEntries(t) {
					builder.AddEntry(entry)
				}
				_, err := builder.Build()
				return err
			},
			wantErr: pkgerr.ErrInvalidMetadata,
		},
		{
			name: "oversized metadata key",
			call: func() error {
				builder := coreNewBuilder(t)
				builder.WithMetadata(map[string]any{strings.Repeat("a", metadataKeyOverMax): true})
				for _, entry := range transactionFieldEntries(t) {
					builder.AddEntry(entry)
				}
				_, err := builder.Build()
				return err
			},
			wantErr: pkgerr.ErrInvalidMetadata,
		},
		{
			name: "control metadata key",
			call: func() error {
				builder := coreNewBuilder(t)
				builder.WithMetadata(map[string]any{"meta\x01key": true})
				for _, entry := range transactionFieldEntries(t) {
					builder.AddEntry(entry)
				}
				_, err := builder.Build()
				return err
			},
			wantErr: pkgerr.ErrInvalidMetadata,
		},
		{
			name: "malformed transaction currency",
			call: func() error {
				_, err := core.NewEntry("acc", decimal.NewFromInt(1), codeShort, decimal.NewFromInt(1), codeUSD)
				return err
			},
			wantErr: pkgerr.ErrInvalidCurrencyCode,
		},
		{
			name: "unknown transaction currency",
			call: func() error {
				_, err := core.NewEntry("acc", decimal.NewFromInt(1), codeBGN, decimal.NewFromInt(1), codeUSD)
				return err
			},
			wantErr: pkgerr.ErrUnknownCurrency,
		},
		{
			name: "non-ratable transaction currency",
			call: func() error {
				_, err := core.NewEntry("acc", decimal.NewFromInt(1), codeXAU, decimal.NewFromInt(1), codeUSD)
				return err
			},
			wantErr: pkgerr.ErrCurrencyNotRatable,
		},
		{
			name: "malformed functional currency",
			call: func() error {
				_, err := core.NewEntry("acc", decimal.NewFromInt(1), codeUSD, decimal.NewFromInt(1), codeShort)
				return err
			},
			wantErr: pkgerr.ErrInvalidCurrencyCode,
		},
		{
			name: "zero amount",
			call: func() error {
				_, err := core.NewEntry("acc", decimal.Zero, codeUSD, decimal.NewFromInt(1), codeUSD)
				return err
			},
			wantErr: pkgerr.ErrZeroAmount,
		},
		{
			name: "negative rate",
			call: func() error {
				_, err := core.NewEntry("acc", decimal.NewFromInt(1), codeUSD, decimal.NewFromInt(-1), codeUSD)
				return err
			},
			wantErr: pkgerr.ErrInvalidExchangeRate,
		},
		{
			name: "over-precision amount",
			call: func() error {
				_, err := core.NewEntry("acc", decimal.RequireFromString(txAmountOverPrecision), codeUSD, decimal.NewFromInt(1), codeUSD)
				return err
			},
			wantErr: pkgerr.ErrInvalidPrecision,
		},
		{
			name: "zero-value entry",
			call: func() error {
				builder := coreNewBuilder(t)
				builder.AddEntry(core.Entry{})
				_, err := builder.Build()
				return err
			},
			wantErr: pkgerr.ErrInvalidEntry,
		},
		{
			name: "currency mismatch",
			call: func() error {
				builder := coreNewBuilder(t)
				builder.AddEntry(mustRateEntry(t, "eur", txAmountHundred, codeEUR, txRateOne, codeEUR))
				_, err := builder.Build()
				return err
			},
			wantErr: pkgerr.ErrCurrencyMismatch,
		},
		{
			name: "adversarial zero-sum",
			call: func() error {
				builder := coreNewBuilder(t)
				for _, entry := range unbalancedEntries(t) {
					builder.AddEntry(entry)
				}
				_, err := builder.Build()
				return err
			},
			wantErr: pkgerr.ErrZeroSumViolation,
		},
		{
			name: "period closed",
			call: func() error {
				builder := core.NewTxBuilder(txID, ledgerEffectiveDate(), periodClosedConfig(), mockRateProvider{})
				for _, entry := range transactionFieldEntries(t) {
					builder.AddEntry(entry)
				}
				_, err := builder.Build()
				return err
			},
			wantErr: pkgerr.ErrPeriodClosed,
		},
		{
			name: "adversarial rate deviation",
			call: func() error {
				provider := &recordingRateProvider{rates: map[string]decimal.Decimal{
					pairEURUSD: decimal.RequireFromString("1.5"),
				}}
				builder := core.NewTxBuilder(txID, ledgerEffectiveDate(), txTestConfig(), provider)
				for _, entry := range eurUSDEntries(t) {
					builder.AddEntry(entry)
				}
				_, err := builder.Build()
				return err
			},
			wantErr: pkgerr.ErrRateDeviation,
		},
		{
			name: "adversarial provider error",
			call: func() error {
				provider := &recordingRateProvider{err: errors.New("provider down")}
				builder := core.NewTxBuilder(txID, ledgerEffectiveDate(), txTestConfig(), provider)
				for _, entry := range eurUSDEntries(t) {
					builder.AddEntry(entry)
				}
				_, err := builder.Build()
				return err
			},
			wantErr: pkgerr.ErrRateProvider,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertAdversarialResult(t, tc)
		})
	}
}

func assertAdversarialResult(t *testing.T, tc adversarialCase) {
	t.Helper()
	err := callWithoutPanic(t, tc.call)
	if tc.wantErr == nil {
		if err != nil {
			t.Errorf("expected success, got %v", err)
		}
		return
	}
	if !errors.Is(err, tc.wantErr) {
		t.Fatalf("error = %v; want %v", err, tc.wantErr)
	}
	var validationErr *pkgerr.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("errors.As(err, *ValidationError) = false; err = %v", err)
	}
	if validationErr.Operation == "" || validationErr.Field == "" {
		t.Errorf("structured error missing context: %+v", validationErr)
	}
}

// TestNoFloatInMoneyPath: no float types in the money path.
func TestNoFloatInMoneyPath(t *testing.T) {
	roots := []string{"../currency", "../core", "../errors", "../interfaces"}
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			return scanGoFileForFloats(t, path)
		})
		if err != nil {
			t.Fatalf("scan %s: %v", root, err)
		}
	}
}

func scanGoFileForFloats(t *testing.T, path string) error {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	ast.Inspect(file, func(node ast.Node) bool {
		ident, ok := node.(*ast.Ident)
		if ok && (ident.Name == "float32" || ident.Name == "float64") {
			t.Errorf("%s uses %s in the money path", path, ident.Name)
		}
		return true
	})
	return nil
}

// TestConcurrentValidation1000 exercises the ledger under the race detector with
// 1,000 goroutines and checks each result matches its own input ID.
func TestConcurrentValidation1000(t *testing.T) {
	ledger := mustValidLedger(t)
	entries := transactionFieldEntries(t)
	const goroutines = 1000

	var wg sync.WaitGroup
	errCh := make(chan error, goroutines)
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			input := core.TransactionInput{
				ID:            fmt.Sprintf("tx-concurrent-%d", index),
				EffectiveDate: ledgerEffectiveDate(),
				Entries:       entries,
			}
			transaction, err := ledger.ValidateTransaction(context.Background(), input)
			if err != nil {
				errCh <- fmt.Errorf("goroutine %d: %w", index, err)
				return
			}
			if transaction.ID() != input.ID {
				errCh <- fmt.Errorf("goroutine %d: ID = %q; want %q", index, transaction.ID(), input.ID)
			}
			if len(transaction.Entries()) != len(entries) {
				errCh <- fmt.Errorf("goroutine %d: entries = %d; want %d",
					index, len(transaction.Entries()), len(entries))
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}
}

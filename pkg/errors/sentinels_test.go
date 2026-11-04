// SPDX-License-Identifier: MIT

package errors_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"testing"

	pkgerr "github.com/Emin-ACIKGOZ/nomledger/pkg/errors"
)

const invalidRoundingModeText = "invalid rounding mode"

var goldenSentinelMessages = map[string]string{
	"ErrZeroSumViolation":           "transaction does not sum to zero",
	"ErrPeriodClosed":               "effective date falls in closed period",
	"ErrRateDeviation":              "exchange rate exceeds tolerance",
	"ErrRateProvider":               "rate provider error",
	"ErrInvalidPrecision":           "amount exceeds currency precision",
	"ErrCurrencyMismatch":           "entry functional currency does not match ledger",
	"ErrEmptyTransaction":           "transaction must contain at least one entry",
	"ErrZeroAmount":                 "transaction amount must be non-zero",
	"ErrFunctionalAmountZero":       "functional amount rounds to zero",
	"ErrInvalidEntry":               "invalid entry",
	"ErrNilRateProvider":            "rate provider must not be nil",
	"ErrInvalidTransactionID":       "invalid transaction ID",
	"ErrInvalidEffectiveDate":       "invalid effective date",
	"ErrInvalidMetadata":            "invalid metadata",
	"ErrInvalidAccountID":           "invalid account ID",
	"ErrInvalidAccountType":         "invalid account type",
	"ErrInvalidMeasurementCategory": "invalid measurement category",
	"ErrInvalidExchangeRate":        "exchange rate must be strictly positive",
	"ErrInvalidConfiguration":       "invalid ledger configuration",
	"ErrUnknownCurrency":            "unknown or unsupported currency code",
	"ErrInvalidCurrencyCode":        "currency code must be exactly three ASCII letters",
	"ErrCurrencyNotRatable":         "currency has no defined minor units",
	"ErrInvalidRoundingMode":        invalidRoundingModeText,
	"ErrInexactRounding":            "rounding would change the value",
	"ErrBuilderFinalized":           "builder is already finalized",
}

func declaredSentinelNames(t *testing.T, path string) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	var names []string
	for _, declaration := range file.Decls {
		gen, ok := declaration.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		names = append(names, sentinelNamesFromSpecs(gen.Specs)...)
	}
	sort.Strings(names)
	return names
}

func sentinelNamesFromSpecs(specs []ast.Spec) []string {
	var names []string
	for _, spec := range specs {
		valueSpec, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		for _, name := range valueSpec.Names {
			if len(name.Name) > 3 && name.Name[:3] == "Err" && ast.IsExported(name.Name) {
				names = append(names, name.Name)
			}
		}
	}
	return names
}

// TestSentinelMessages: every exported sentinel is pinned to an
// exact user-visible message.
func TestSentinelMessages(t *testing.T) {
	sentinels := sentinelByName()
	declared := declaredSentinelNames(t, "errors.go")

	var missing []string
	for _, name := range declared {
		if _, ok := goldenSentinelMessages[name]; !ok {
			missing = append(missing, name)
		}
	}
	var extra []string
	for name := range goldenSentinelMessages {
		if _, ok := sentinels[name]; !ok {
			extra = append(extra, name)
		}
	}
	if len(missing) > 0 {
		t.Errorf("sentinel declarations missing from golden table: %v", missing)
	}
	if len(extra) > 0 {
		t.Errorf("golden table entries not declared: %v", extra)
	}

	for name, sentinel := range sentinels {
		t.Run(name, func(t *testing.T) {
			want, ok := goldenSentinelMessages[name]
			if !ok {
				t.Fatalf("no golden message for %s", name)
			}
			if sentinel.Error() != want {
				t.Errorf("%s.Error() = %q; want %q", name, sentinel.Error(), want)
			}
		})
	}
}

func sentinelByName() map[string]error {
	return map[string]error{
		"ErrZeroSumViolation":           pkgerr.ErrZeroSumViolation,
		"ErrPeriodClosed":               pkgerr.ErrPeriodClosed,
		"ErrRateDeviation":              pkgerr.ErrRateDeviation,
		"ErrRateProvider":               pkgerr.ErrRateProvider,
		"ErrInvalidPrecision":           pkgerr.ErrInvalidPrecision,
		"ErrCurrencyMismatch":           pkgerr.ErrCurrencyMismatch,
		"ErrEmptyTransaction":           pkgerr.ErrEmptyTransaction,
		"ErrZeroAmount":                 pkgerr.ErrZeroAmount,
		"ErrFunctionalAmountZero":       pkgerr.ErrFunctionalAmountZero,
		"ErrInvalidEntry":               pkgerr.ErrInvalidEntry,
		"ErrNilRateProvider":            pkgerr.ErrNilRateProvider,
		"ErrInvalidTransactionID":       pkgerr.ErrInvalidTransactionID,
		"ErrInvalidEffectiveDate":       pkgerr.ErrInvalidEffectiveDate,
		"ErrInvalidMetadata":            pkgerr.ErrInvalidMetadata,
		"ErrInvalidAccountID":           pkgerr.ErrInvalidAccountID,
		"ErrInvalidAccountType":         pkgerr.ErrInvalidAccountType,
		"ErrInvalidMeasurementCategory": pkgerr.ErrInvalidMeasurementCategory,
		"ErrInvalidExchangeRate":        pkgerr.ErrInvalidExchangeRate,
		"ErrInvalidConfiguration":       pkgerr.ErrInvalidConfiguration,
		"ErrUnknownCurrency":            pkgerr.ErrUnknownCurrency,
		"ErrInvalidCurrencyCode":        pkgerr.ErrInvalidCurrencyCode,
		"ErrCurrencyNotRatable":         pkgerr.ErrCurrencyNotRatable,
		"ErrInvalidRoundingMode":        pkgerr.ErrInvalidRoundingMode,
		"ErrInexactRounding":            pkgerr.ErrInexactRounding,
		"ErrBuilderFinalized":           pkgerr.ErrBuilderFinalized,
	}
}

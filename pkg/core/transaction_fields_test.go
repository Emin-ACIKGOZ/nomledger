// SPDX-License-Identifier: MIT

package core_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Emin-ACIKGOZ/nomledger/pkg/core"
	pkgerr "github.com/Emin-ACIKGOZ/nomledger/pkg/errors"
)

const (
	transactionBuildOperation = "Build"
	fieldEffectiveDate        = "effectiveDate"
	fieldEntries              = "entries"
	fieldMetadata             = "metadata"

	metadataKeyMaxBytes     = 64
	metadataKeyOverMax      = 65
	metadataKeyUnicode      = "Clé-日本-Key"
	metadataKey64           = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	metadataValue           = "value"
	metadataNestedKey       = "nested"
	metadataInnerKey        = "inner"
	metadataSliceKey        = "items"
	metadataNilKey          = "nil"
	metadataInvalidValueKey = "invalid-value"
)

func buildTransactionWith(
	t *testing.T,
	config core.LedgerConfig,
	id string,
	effectiveDate time.Time,
	adjusting bool,
	metadata map[string]any,
	entries []core.Entry,
) (core.Transaction, error) {
	t.Helper()
	builder := core.NewTxBuilder(id, effectiveDate, config, mockRateProvider{})
	builder.SetAdjustingEvent(adjusting)
	if metadata != nil {
		builder.WithMetadata(metadata)
	}
	for _, entry := range entries {
		builder.AddEntry(entry)
	}
	return builder.Build()
}

func transactionFieldEntries(t *testing.T) []core.Entry {
	t.Helper()
	return []core.Entry{
		mustTxEntry(t, txDebitAccount, txAmountHundred),
		mustTxEntry(t, txCreditAccount, txAmountNegHundred),
	}
}

// TestTransactionIDValidation.
func TestTransactionIDValidation(t *testing.T) {
	entries := transactionFieldEntries(t)
	validDate := ledgerEffectiveDate()
	config := txTestConfig()

	cases := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{"transaction ID valid", txID, false},
		{"transaction ID 255 bytes", strings.Repeat("a", maxIDBytes), false},
		{"transaction ID empty", "", true},
		{"transaction ID 256 bytes", strings.Repeat("a", overMaxIDBytes), true},
		{"transaction ID NUL", "tx\x00id", true},
		{"transaction ID SOH control", "tx\x01id", true},
		{"transaction ID unit separator control", "tx\x1fid", true},
		{"transaction ID DEL control", "tx\x7fid", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			transaction, err := buildTransactionWith(t, config, tc.id, validDate, false, nil, entries)
			if tc.wantErr {
				assertStructuredValidationError(t, err, pkgerr.ErrInvalidTransactionID,
					fieldID, transactionBuildOperation)
				return
			}
			if err != nil {
				t.Fatalf("Build: %v", err)
			}
			if transaction.ID() != tc.id {
				t.Errorf("ID() = %q; want %q", transaction.ID(), tc.id)
			}
		})
	}
}

// TestTransactionEffectiveDateValidation.
func TestTransactionEffectiveDateValidation(t *testing.T) {
	entries := transactionFieldEntries(t)
	config := txTestConfig()

	t.Run("zero date rejected", func(t *testing.T) {
		_, err := buildTransactionWith(t, config, txID, time.Time{}, false, nil, entries)
		assertStructuredValidationError(t, err, pkgerr.ErrInvalidEffectiveDate,
			fieldEffectiveDate, transactionBuildOperation)
	})

	t.Run("non-UTC date normalized", func(t *testing.T) {
		zone := time.FixedZone("EST", -5*60*60)
		effective := time.Date(2024, 1, 15, 12, 34, 56, 123456789, zone)
		transaction, err := buildTransactionWith(t, config, txID, effective, false, nil, entries)
		if err != nil {
			t.Fatalf("Build: %v", err)
		}
		got := transaction.EffectiveDate()
		if got.Location() != time.UTC {
			t.Errorf("EffectiveDate().Location() = %v; want UTC", got.Location())
		}
		if !got.Equal(effective) {
			t.Errorf("EffectiveDate() instant = %s; want %s", got, effective)
		}
		if got.Nanosecond() != effective.Nanosecond() {
			t.Errorf("EffectiveDate().Nanosecond() = %d; want %d", got.Nanosecond(), effective.Nanosecond())
		}
		if got.Hour() != 17 || got.Minute() != 34 || got.Second() != 56 {
			t.Errorf("EffectiveDate() UTC wall clock = %02d:%02d:%02d; want 17:34:56",
				got.Hour(), got.Minute(), got.Second())
		}
	})
}

// TestTransactionEmptyEntries.
func TestTransactionEmptyEntries(t *testing.T) {
	t.Run("zero entries rejected", func(t *testing.T) {
		_, err := buildTransactionWith(t, txTestConfig(), txID, ledgerEffectiveDate(), false, nil, nil)
		assertStructuredValidationError(t, err, pkgerr.ErrEmptyTransaction,
			fieldEntries, transactionBuildOperation)
	})

	t.Run("balanced pair accepted", func(t *testing.T) {
		transaction, err := buildTransactionWith(t, txTestConfig(), txID, ledgerEffectiveDate(), false, nil,
			transactionFieldEntries(t))
		if err != nil {
			t.Fatalf("Build: %v", err)
		}
		if len(transaction.Entries()) != 2 {
			t.Errorf("Entries() length = %d; want 2", len(transaction.Entries()))
		}
	})
}

// TestTransactionAdjustingEvent.
func TestTransactionAdjustingEvent(t *testing.T) {
	closed := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	config := core.LedgerConfig{ClosedDate: closed, FunctionalCurrency: codeUSD}
	entries := transactionFieldEntries(t)

	cases := []struct {
		name      string
		effective time.Time
		adjusting bool
		wantErr   error
	}{
		{"adjusting equal to closed", closed, true, nil},
		{"adjusting before closed", closed.Add(-time.Hour), true, nil},
		{"non-adjusting equal to closed", closed, false, pkgerr.ErrPeriodClosed},
		{"non-adjusting before closed", closed.Add(-time.Nanosecond), false, pkgerr.ErrPeriodClosed},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildTransactionWith(t, config, txID, tc.effective, tc.adjusting, nil, entries)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("Build error = %v; want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Build: %v", err)
			}
		})
	}
}

// TestTransactionMetadataKeys.
func TestTransactionMetadataKeys(t *testing.T) {
	t.Run("rejects invalid keys", testRejectedMetadataKeys)
	t.Run("accepts 64-byte key", testMetadata64ByteKey)
	t.Run("stores Unicode key exactly", testMetadataUnicodeKey)
	t.Run("does not validate values", testMetadataValuesNotValidated)
}

func testRejectedMetadataKeys(t *testing.T) {
	t.Helper()
	entries := transactionFieldEntries(t)
	config := txTestConfig()
	rejected := []struct {
		name string
		key  string
	}{
		{"metadata key empty", ""},
		{"metadata key 65 bytes", strings.Repeat("a", metadataKeyOverMax)},
		{"metadata key NUL", "key\x00suffix"},
		{"metadata key SOH control", "key\x01suffix"},
		{"metadata key unit separator control", "key\x1fsuffix"},
		{"metadata key DEL control", "key\x7fsuffix"},
	}

	for _, tc := range rejected {
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildTransactionWith(t, config, txID, ledgerEffectiveDate(), false,
				map[string]any{tc.key: metadataValue}, entries)
			assertStructuredValidationError(t, err, pkgerr.ErrInvalidMetadata,
				fieldMetadata, transactionBuildOperation)
		})
	}
}

func testMetadata64ByteKey(t *testing.T) {
	t.Helper()
	transaction, err := buildTransactionWith(t, txTestConfig(), txID, ledgerEffectiveDate(), false,
		map[string]any{metadataKey64: metadataValue}, transactionFieldEntries(t))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got := transaction.Metadata()[metadataKey64]; got != metadataValue {
		t.Errorf("Metadata()[64-byte key] = %v; want %q", got, metadataValue)
	}
}

func testMetadataUnicodeKey(t *testing.T) {
	t.Helper()
	transaction, err := buildTransactionWith(t, txTestConfig(), txID, ledgerEffectiveDate(), false,
		map[string]any{metadataKeyUnicode: metadataValue}, transactionFieldEntries(t))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	metadata := transaction.Metadata()
	if got := metadata[metadataKeyUnicode]; got != metadataValue {
		t.Errorf("Metadata()[%q] = %v; want %q", metadataKeyUnicode, got, metadataValue)
	}
}

func testMetadataValuesNotValidated(t *testing.T) {
	t.Helper()
	metadata := map[string]any{
		metadataNilKey:          nil,
		metadataNestedKey:       map[string]any{metadataInnerKey: metadataValue},
		metadataSliceKey:        []string{"a", "b"},
		metadataInvalidValueKey: "value\x00with-control",
	}
	transaction, err := buildTransactionWith(t, txTestConfig(), txID, ledgerEffectiveDate(), false,
		metadata, transactionFieldEntries(t))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	stored := transaction.Metadata()
	if value, ok := stored[metadataNilKey]; !ok || value != nil {
		t.Errorf("Metadata()[nil] = %v, present=%t; want nil, present", value, ok)
	}
	if _, ok := stored[metadataNestedKey].(map[string]any); !ok {
		t.Errorf("Metadata()[nested] type = %T; want map[string]any", stored[metadataNestedKey])
	}
	if _, ok := stored[metadataSliceKey].([]string); !ok {
		t.Errorf("Metadata()[items] type = %T; want []string", stored[metadataSliceKey])
	}
	if stored[metadataInvalidValueKey] != "value\x00with-control" {
		t.Errorf("Metadata()[invalid-value] = %v; want control-containing value unchanged",
			stored[metadataInvalidValueKey])
	}
}

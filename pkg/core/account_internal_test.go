// SPDX-License-Identifier: MIT

package core

import (
	"errors"
	"strings"
	"testing"

	pkgerr "github.com/Emin-ACIKGOZ/nomledger/pkg/errors"
)

const (
	testAccountID     = "acct-internal"
	testCurrencyUSD   = "USD"
	testCurrencyLower = "usd"
	testCurrencyXAU   = "XAU"
	testCurrencyBGN   = "BGN"
	testCurrencyShort = "US"

	testOpNewAccount = "NewAccount"
	testOpValidate   = "Account.Validate"

	testFieldID       = "id"
	testFieldType     = "type"
	testFieldCategory = "category"
	testFieldCurrency = "currency"

	metaKeyNested  = "nested"
	metaKeyItems   = "items"
	metaKeyInner   = "key"
	metaValueInit  = "original"
	metaValueFirst = "first"
)

const (
	testInvalidType     = AccountType(99)
	testInvalidCategory = MeasurementCategory(99)
)

// assertAccountStructuredError verifies the ValidationError contract used by
// both NewAccount and Account.Validate.
func assertAccountStructuredError(
	t *testing.T,
	err error,
	want error,
	wantField string,
	wantOperation string,
) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("error = %v; want errors.Is(%v)", err, want)
	}
	var validationErr *pkgerr.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("errors.As(err, *ValidationError) = false; err = %v", err)
	}
	if validationErr.EntryIndex != -1 {
		t.Errorf("EntryIndex = %d; want -1", validationErr.EntryIndex)
	}
	if validationErr.Operation != wantOperation {
		t.Errorf("Operation = %q; want %q", validationErr.Operation, wantOperation)
	}
	if validationErr.Field != wantField {
		t.Errorf("Field = %q; want %q", validationErr.Field, wantField)
	}
}

// TestAccountValidateInternalInvalidFields constructs invalid Account values
// directly, which the external test package cannot do.
func TestAccountValidateInternalInvalidFields(t *testing.T) {
	base := Account{
		id:           testAccountID,
		typ:          Asset,
		cat:          NotApplicable,
		currencyCode: testCurrencyUSD,
	}

	cases := []struct {
		name    string
		mutate  func(*Account)
		wantErr error
		field   string
	}{
		{
			name:    "non-zero invalid type",
			mutate:  func(a *Account) { a.typ = testInvalidType },
			wantErr: pkgerr.ErrInvalidAccountType,
			field:   testFieldType,
		},
		{
			name:    "invalid category",
			mutate:  func(a *Account) { a.cat = testInvalidCategory },
			wantErr: pkgerr.ErrInvalidMeasurementCategory,
			field:   testFieldCategory,
		},
		{
			name:    "non-ratable currency",
			mutate:  func(a *Account) { a.currencyCode = testCurrencyXAU },
			wantErr: pkgerr.ErrCurrencyNotRatable,
			field:   testFieldCurrency,
		},
		{
			name:    "unknown currency",
			mutate:  func(a *Account) { a.currencyCode = testCurrencyBGN },
			wantErr: pkgerr.ErrUnknownCurrency,
			field:   testFieldCurrency,
		},
		{
			name:    "malformed currency",
			mutate:  func(a *Account) { a.currencyCode = testCurrencyShort },
			wantErr: pkgerr.ErrInvalidCurrencyCode,
			field:   testFieldCurrency,
		},
		{
			name: "type wins over category and currency",
			mutate: func(a *Account) {
				a.typ = testInvalidType
				a.cat = testInvalidCategory
				a.currencyCode = testCurrencyShort
			},
			wantErr: pkgerr.ErrInvalidAccountType,
			field:   testFieldType,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			account := base
			tc.mutate(&account)
			assertAccountStructuredError(t, account.Validate(), tc.wantErr, tc.field, testOpValidate)
		})
	}
}

// TestNewAccountZeroOnError verifies that every constructor failure returns a
// fully zero Account value.
func TestNewAccountZeroOnError(t *testing.T) {
	cases := []struct {
		name    string
		id      string
		typ     AccountType
		cat     MeasurementCategory
		code    string
		wantErr error
		field   string
	}{
		{"empty ID", "", Asset, NotApplicable, testCurrencyUSD, pkgerr.ErrInvalidAccountID, testFieldID},
		{"NUL ID", "acct\x00id", Asset, NotApplicable, testCurrencyUSD, pkgerr.ErrInvalidAccountID, testFieldID},
		{"oversized ID", strings.Repeat("a", maxAccountIDBytes+1), Asset, NotApplicable, testCurrencyUSD, pkgerr.ErrInvalidAccountID, testFieldID},
		{"invalid type", testAccountID, testInvalidType, NotApplicable, testCurrencyUSD, pkgerr.ErrInvalidAccountType, testFieldType},
		{"invalid category", testAccountID, Asset, testInvalidCategory, testCurrencyUSD, pkgerr.ErrInvalidMeasurementCategory, testFieldCategory},
		{"non-ratable currency", testAccountID, Asset, NotApplicable, testCurrencyXAU, pkgerr.ErrCurrencyNotRatable, testFieldCurrency},
		{"unknown currency", testAccountID, Asset, NotApplicable, testCurrencyBGN, pkgerr.ErrUnknownCurrency, testFieldCurrency},
		{"malformed currency", testAccountID, Asset, NotApplicable, testCurrencyShort, pkgerr.ErrInvalidCurrencyCode, testFieldCurrency},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			account, err := NewAccount(tc.id, tc.typ, tc.cat, tc.code, map[string]any{"ignored": true})
			assertAccountStructuredError(t, err, tc.wantErr, tc.field, testOpNewAccount)

			if account.ID() != "" {
				t.Errorf("ID() = %q; want empty", account.ID())
			}
			if account.Type() != 0 {
				t.Errorf("Type() = %v; want zero", account.Type())
			}
			if account.Category() != 0 {
				t.Errorf("Category() = %v; want zero", account.Category())
			}
			if account.CurrencyCode() != "" {
				t.Errorf("CurrencyCode() = %q; want empty", account.CurrencyCode())
			}
			metadata := account.Metadata()
			if metadata == nil {
				t.Fatal("Metadata() = nil; want non-nil empty map")
			}
			if len(metadata) != 0 {
				t.Errorf("Metadata() length = %d; want 0", len(metadata))
			}
		})
	}
}

// TestAccountValidateCopyInternal checks that a copied valid Account validates
// and keeps stable accessors.
func TestAccountValidateCopyInternal(t *testing.T) {
	account, err := NewAccount(testAccountID, Asset, FVOCI, testCurrencyLower, map[string]any{metaKeyInner: "value"})
	if err != nil {
		t.Fatalf("NewAccount: %v", err)
	}
	if account.CurrencyCode() != testCurrencyUSD {
		t.Fatalf("CurrencyCode() = %q; want %q", account.CurrencyCode(), testCurrencyUSD)
	}

	copied := account
	if err := copied.Validate(); err != nil {
		t.Errorf("copied Validate() = %v; want nil", err)
	}
	if copied.ID() != account.ID() {
		t.Errorf("copied ID() = %q; want %q", copied.ID(), account.ID())
	}
	if copied.Type() != account.Type() {
		t.Errorf("copied Type() = %v; want %v", copied.Type(), account.Type())
	}
	if copied.Category() != account.Category() {
		t.Errorf("copied Category() = %v; want %v", copied.Category(), account.Category())
	}
	if copied.CurrencyCode() != account.CurrencyCode() {
		t.Errorf("copied CurrencyCode() = %q; want %q", copied.CurrencyCode(), account.CurrencyCode())
	}
}

// TestAccountMetadataShallowLimit documents the shallow-copy behavior: the top-level metadata map
// is copied, but nested reference values remain shared with the caller.
func TestAccountMetadataShallowLimit(t *testing.T) {
	nested := map[string]any{metaKeyInner: metaValueInit}
	items := []string{metaValueFirst, "second"}
	input := map[string]any{
		metaKeyNested: nested,
		metaKeyItems:  items,
	}

	account, err := NewAccount(testAccountID, Asset, NotApplicable, testCurrencyUSD, input)
	if err != nil {
		t.Fatalf("NewAccount: %v", err)
	}

	// The top-level map is copied: structural changes stay outside.
	input["added"] = true
	metadata := account.Metadata()
	if _, ok := metadata["added"]; ok {
		t.Error("top-level input mutation leaked into Metadata()")
	}
	metadata["added2"] = true
	if _, ok := account.Metadata()["added2"]; ok {
		t.Error("top-level output mutation leaked into the stored metadata")
	}

	// Nested values are shared by design.
	nested[metaKeyInner] = "mutated"
	nestedAfter := account.Metadata()[metaKeyNested].(map[string]any)
	if nestedAfter[metaKeyInner] != "mutated" {
		t.Errorf("nested map value = %v; want shared mutation visible", nestedAfter[metaKeyInner])
	}

	items[0] = "changed"
	itemsAfter := account.Metadata()[metaKeyItems].([]string)
	if itemsAfter[0] != "changed" {
		t.Errorf("nested slice value = %q; want shared mutation visible", itemsAfter[0])
	}

	// Mutation through the returned nested value is also shared.
	nestedAfter[metaKeyInner] = "via-metadata"
	if nested[metaKeyInner] != "via-metadata" {
		t.Errorf("original nested map = %v; want mutation through Metadata() visible", nested[metaKeyInner])
	}
}

// TestAccountValidateLowercaseCurrencyInternal documents that Validate accepts a
// lowercase currency but does not canonicalise it; NewAccount guarantees
// canonical storage.
func TestAccountValidateLowercaseCurrencyInternal(t *testing.T) {
	account := Account{
		id:           testAccountID,
		typ:          Asset,
		cat:          NotApplicable,
		currencyCode: testCurrencyLower,
		metadata:     map[string]any{},
	}

	if err := account.Validate(); err != nil {
		t.Fatalf("Validate() with lowercase currency = %v; want nil", err)
	}
	if account.CurrencyCode() != testCurrencyLower {
		t.Errorf("CurrencyCode() = %q; want unchanged %q", account.CurrencyCode(), testCurrencyLower)
	}
}

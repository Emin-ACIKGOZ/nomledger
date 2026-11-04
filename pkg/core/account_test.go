// SPDX-License-Identifier: MIT

package core_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/Emin-ACIKGOZ/nomledger/pkg/core"
	pkgerr "github.com/Emin-ACIKGOZ/nomledger/pkg/errors"
)

const (
	codeUSD      = "USD"
	codeEUR      = "EUR"
	codeJPY      = "JPY"
	codeBHD      = "BHD"
	codeXAU      = "XAU"
	codeBGN      = "BGN"
	codeShort    = "US"
	codeSpaced   = " USD"
	codeCyrillic = "БГЛ"
	codeUnknown  = "BAD"

	validID      = "acct-1"
	opNewAccount = "NewAccount"
	opValidate   = "Account.Validate"

	fieldID       = "id"
	fieldType     = "type"
	fieldCategory = "category"
	fieldCurrency = "currency"

	maxIDBytes     = 255
	overMaxIDBytes = 256

	metaKeyNote       = "note"
	metaKeyCount      = "n"
	metaValueOriginal = "original"
)

const (
	invalidAccountTypeZero = core.AccountType(0)
	invalidAccountTypeHigh = core.AccountType(99)
	invalidAccountTypeNeg  = core.AccountType(-1)

	invalidCategoryHigh = core.MeasurementCategory(99)
	invalidCategoryNeg  = core.MeasurementCategory(-1)
)

// assertStructuredValidationError checks the structured contract shared by all
// account validation failures.
func assertStructuredValidationError(
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

func mustAccount(
	t *testing.T,
	id string,
	typ core.AccountType,
	cat core.MeasurementCategory,
	currencyCode string,
	metadata map[string]any,
) core.Account {
	t.Helper()
	account, err := core.NewAccount(id, typ, cat, currencyCode, metadata)
	if err != nil {
		t.Fatalf("NewAccount(%q, %v, %v, %q): %v", id, typ, cat, currencyCode, err)
	}
	return account
}

// TestNewAccountValidation: field validation in the required order
// with structured errors, plus idempotent Validate.
func TestNewAccountValidation(t *testing.T) {
	validMetadata := map[string]any{metaKeyNote: "valid"}

	cases := []struct {
		name         string
		id           string
		typ          core.AccountType
		cat          core.MeasurementCategory
		currencyCode string
		metadata     map[string]any
		wantErr      error
		wantField    string
		wantCurrency string
	}{
		{"valid asset", validID, core.Asset, core.NotApplicable, codeUSD, nil, nil, "", codeUSD},
		{"valid liability", "acct-2", core.Liability, core.AmortizedCost, codeEUR, nil, nil, "", codeEUR},
		{"valid equity", "acct-3", core.Equity, core.FVTPL, codeJPY, nil, nil, "", codeJPY},
		{"valid income", "acct-4", core.Income, core.FVOCI, codeBHD, nil, nil, "", codeBHD},
		{"valid expense with metadata", "acct-5", core.Expense, core.NotApplicable, codeUSD, validMetadata, nil, "", codeUSD},
		{"valid lowercase currency", "acct-6", core.Asset, core.NotApplicable, "usd", nil, nil, "", codeUSD},
		{"valid unicode ID", "acct-ü-1", core.Asset, core.NotApplicable, codeUSD, nil, nil, "", codeUSD},

		{"empty ID", "", core.Asset, core.NotApplicable, codeUSD, nil, pkgerr.ErrInvalidAccountID, fieldID, ""},
		{"ID at 256 bytes", strings.Repeat("a", overMaxIDBytes), core.Asset, core.NotApplicable, codeUSD, nil, pkgerr.ErrInvalidAccountID, fieldID, ""},
		{"ID with NUL", "acct\x00id", core.Asset, core.NotApplicable, codeUSD, nil, pkgerr.ErrInvalidAccountID, fieldID, ""},
		{"ID with SOH", "acct\x01id", core.Asset, core.NotApplicable, codeUSD, nil, pkgerr.ErrInvalidAccountID, fieldID, ""},
		{"ID with unit separator", "acct\x1fid", core.Asset, core.NotApplicable, codeUSD, nil, pkgerr.ErrInvalidAccountID, fieldID, ""},
		{"ID with DEL", "acct\x7fid", core.Asset, core.NotApplicable, codeUSD, nil, pkgerr.ErrInvalidAccountID, fieldID, ""},
		{"ID at 256 multibyte bytes", strings.Repeat("é", overMaxIDBytes/2), core.Asset, core.NotApplicable, codeUSD, nil, pkgerr.ErrInvalidAccountID, fieldID, ""},

		{"zero type", validID, invalidAccountTypeZero, core.NotApplicable, codeUSD, nil, pkgerr.ErrInvalidAccountType, fieldType, ""},
		{"high type", validID, invalidAccountTypeHigh, core.NotApplicable, codeUSD, nil, pkgerr.ErrInvalidAccountType, fieldType, ""},
		{"negative type", validID, invalidAccountTypeNeg, core.NotApplicable, codeUSD, nil, pkgerr.ErrInvalidAccountType, fieldType, ""},

		{"high category", validID, core.Asset, invalidCategoryHigh, codeUSD, nil, pkgerr.ErrInvalidMeasurementCategory, fieldCategory, ""},
		{"negative category", validID, core.Asset, invalidCategoryNeg, codeUSD, nil, pkgerr.ErrInvalidMeasurementCategory, fieldCategory, ""},

		{"non-ratable currency", validID, core.Asset, core.NotApplicable, codeXAU, nil, pkgerr.ErrCurrencyNotRatable, fieldCurrency, ""},
		{"unknown currency", validID, core.Asset, core.NotApplicable, codeBGN, nil, pkgerr.ErrUnknownCurrency, fieldCurrency, ""},
		{"short currency", validID, core.Asset, core.NotApplicable, codeShort, nil, pkgerr.ErrInvalidCurrencyCode, fieldCurrency, ""},
		{"spaced currency", validID, core.Asset, core.NotApplicable, codeSpaced, nil, pkgerr.ErrInvalidCurrencyCode, fieldCurrency, ""},
		{"non-ASCII currency", validID, core.Asset, core.NotApplicable, codeCyrillic, nil, pkgerr.ErrInvalidCurrencyCode, fieldCurrency, ""},

		// Multiple invalid fields must report the first failure in order.
		{"ID wins over type, category, currency", "", invalidAccountTypeZero, invalidCategoryHigh, codeUnknown, nil, pkgerr.ErrInvalidAccountID, fieldID, ""},
		{"type wins over category and currency", validID, invalidAccountTypeZero, invalidCategoryHigh, codeUnknown, nil, pkgerr.ErrInvalidAccountType, fieldType, ""},
		{"category wins over currency", validID, core.Asset, invalidCategoryHigh, codeUnknown, nil, pkgerr.ErrInvalidMeasurementCategory, fieldCategory, ""},
		{"currency is checked last", validID, core.Asset, core.NotApplicable, codeUnknown, nil, pkgerr.ErrUnknownCurrency, fieldCurrency, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			account, err := core.NewAccount(tc.id, tc.typ, tc.cat, tc.currencyCode, tc.metadata)

			if tc.wantErr != nil {
				assertStructuredValidationError(t, err, tc.wantErr, tc.wantField, opNewAccount)
				return
			}
			if err != nil {
				t.Fatalf("NewAccount: unexpected error %v", err)
			}

			if account.ID() != tc.id {
				t.Errorf("ID() = %q; want %q", account.ID(), tc.id)
			}
			if account.Type() != tc.typ {
				t.Errorf("Type() = %v; want %v", account.Type(), tc.typ)
			}
			if account.Category() != tc.cat {
				t.Errorf("Category() = %v; want %v", account.Category(), tc.cat)
			}
			if account.CurrencyCode() != tc.wantCurrency {
				t.Errorf("CurrencyCode() = %q; want %q", account.CurrencyCode(), tc.wantCurrency)
			}

			if err := account.Validate(); err != nil {
				t.Errorf("Validate() = %v; want nil", err)
			}
			if err := account.Validate(); err != nil {
				t.Errorf("second Validate() = %v; want nil (idempotent)", err)
			}
		})
	}
}

// TestAccountTypes: exactly five distinct account types and rejection
// of zero/unknown values.
func TestAccountTypes(t *testing.T) {
	types := []core.AccountType{
		core.Asset,
		core.Liability,
		core.Equity,
		core.Income,
		core.Expense,
	}

	seen := make(map[core.AccountType]bool, len(types))
	for _, typ := range types {
		if seen[typ] {
			t.Errorf("duplicate AccountType %v", typ)
		}
		seen[typ] = true
	}
	if len(seen) != 5 {
		t.Errorf("distinct AccountType count = %d; want 5", len(seen))
	}

	for _, typ := range types {
		t.Run("accepts type", func(t *testing.T) {
			if _, err := core.NewAccount(validID, typ, core.NotApplicable, codeUSD, nil); err != nil {
				t.Errorf("NewAccount with type %v: %v", typ, err)
			}
		})
	}

	for _, typ := range []core.AccountType{invalidAccountTypeZero, invalidAccountTypeHigh} {
		t.Run("rejects type", func(t *testing.T) {
			_, err := core.NewAccount(validID, typ, core.NotApplicable, codeUSD, nil)
			assertStructuredValidationError(t, err, pkgerr.ErrInvalidAccountType, fieldType, opNewAccount)
		})
	}
}

// TestMeasurementCategories: exactly four distinct categories,
// NotApplicable as zero, FVOCI present, and unknown values rejected.
func TestMeasurementCategories(t *testing.T) {
	categories := []core.MeasurementCategory{
		core.NotApplicable,
		core.AmortizedCost,
		core.FVTPL,
		core.FVOCI,
	}

	if core.NotApplicable != 0 {
		t.Errorf("NotApplicable = %v; want zero value", core.NotApplicable)
	}

	seen := make(map[core.MeasurementCategory]bool, len(categories))
	for _, category := range categories {
		if seen[category] {
			t.Errorf("duplicate MeasurementCategory %v", category)
		}
		seen[category] = true
	}
	if len(seen) != 4 {
		t.Errorf("distinct MeasurementCategory count = %d; want 4", len(seen))
	}

	for _, category := range categories {
		t.Run("accepts category", func(t *testing.T) {
			if _, err := core.NewAccount(validID, core.Asset, category, codeUSD, nil); err != nil {
				t.Errorf("NewAccount with category %v: %v", category, err)
			}
		})
	}

	for _, category := range []core.MeasurementCategory{invalidCategoryHigh, invalidCategoryNeg} {
		t.Run("rejects category", func(t *testing.T) {
			_, err := core.NewAccount(validID, core.Asset, category, codeUSD, nil)
			assertStructuredValidationError(t, err, pkgerr.ErrInvalidMeasurementCategory, fieldCategory, opNewAccount)
		})
	}
}

// TestAccountCurrency: currency must be a valid, ratable, active code
// and is canonicalised on construction.
func TestAccountCurrency(t *testing.T) {
	cases := []struct {
		name         string
		code         string
		wantErr      error
		wantCurrency string
	}{
		{codeUSD, codeUSD, nil, codeUSD},
		{"lowercase USD", "usd", nil, codeUSD},
		{"mixed case USD", "UsD", nil, codeUSD},
		{"JPY", codeJPY, nil, codeJPY},
		{"XAU non-ratable", codeXAU, pkgerr.ErrCurrencyNotRatable, ""},
		{"BGN historical", codeBGN, pkgerr.ErrUnknownCurrency, ""},
		{"short code", codeShort, pkgerr.ErrInvalidCurrencyCode, ""},
		{"spaced code", codeSpaced, pkgerr.ErrInvalidCurrencyCode, ""},
		{"non-ASCII code", codeCyrillic, pkgerr.ErrInvalidCurrencyCode, ""},
		{"empty code", "", pkgerr.ErrInvalidCurrencyCode, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			account, err := core.NewAccount(validID, core.Asset, core.NotApplicable, tc.code, nil)

			if tc.wantErr != nil {
				assertStructuredValidationError(t, err, tc.wantErr, fieldCurrency, opNewAccount)
				return
			}
			if err != nil {
				t.Fatalf("NewAccount: unexpected error %v", err)
			}
			if account.CurrencyCode() != tc.wantCurrency {
				t.Errorf("CurrencyCode() = %q; want %q", account.CurrencyCode(), tc.wantCurrency)
			}
			if err := account.Validate(); err != nil {
				t.Errorf("Validate() = %v; want nil", err)
			}
		})
	}
}

// TestAccountIDBounds: 255 UTF-8 bytes are allowed, 256 are not, and
// NUL/ASCII control characters are rejected.
func TestAccountIDBounds(t *testing.T) {
	cases := []struct {
		name    string
		id      string
		wantErr error
	}{
		{"valid", validID, nil},
		{"empty", "", pkgerr.ErrInvalidAccountID},
		{"256 ASCII bytes", strings.Repeat("a", overMaxIDBytes), pkgerr.ErrInvalidAccountID},
		{"255 ASCII bytes", strings.Repeat("a", maxIDBytes), nil},
		{"NUL", "acct\x00id", pkgerr.ErrInvalidAccountID},
		{"SOH control", "acct\x01id", pkgerr.ErrInvalidAccountID},
		{"unit separator control", "acct\x1fid", pkgerr.ErrInvalidAccountID},
		{"DEL control", "acct\x7fid", pkgerr.ErrInvalidAccountID},
		{"255 multibyte bytes", strings.Repeat("é", (maxIDBytes-1)/2) + "a", nil},
		{"256 multibyte bytes", strings.Repeat("é", overMaxIDBytes/2), pkgerr.ErrInvalidAccountID},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			account, err := core.NewAccount(tc.id, core.Asset, core.NotApplicable, codeUSD, nil)

			if tc.wantErr != nil {
				assertStructuredValidationError(t, err, tc.wantErr, fieldID, opNewAccount)
				return
			}
			if err != nil {
				t.Fatalf("NewAccount: unexpected error %v", err)
			}
			if account.ID() != tc.id {
				t.Errorf("ID() = %q; want %q", account.ID(), tc.id)
			}
			if err := account.Validate(); err != nil {
				t.Errorf("Validate() = %v; want nil", err)
			}
		})
	}
}

// TestAccountMetadataCopy: metadata is copied on input and output and
// never affects validation.
func TestAccountMetadataCopy(t *testing.T) {
	input := map[string]any{
		metaKeyNote:  metaValueOriginal,
		metaKeyCount: 1,
	}
	account := mustAccount(t, validID, core.Asset, core.NotApplicable, codeUSD, input)
	if err := account.Validate(); err != nil {
		t.Fatalf("Validate() before metadata mutation = %v; want nil", err)
	}

	input[metaKeyNote] = "mutated input"
	delete(input, metaKeyCount)
	input["extra"] = true

	metadata := account.Metadata()
	if metadata[metaKeyNote] != metaValueOriginal {
		t.Errorf("Metadata()[note] = %v; want original", metadata[metaKeyNote])
	}
	if _, ok := metadata[metaKeyCount]; !ok {
		t.Errorf("Metadata() lost key %q after input mutation", metaKeyCount)
	}
	if _, ok := metadata["extra"]; ok {
		t.Errorf("Metadata() gained key %q after input mutation", "extra")
	}

	metadata[metaKeyNote] = "mutated output"
	metadata["extra2"] = true

	again := account.Metadata()
	if again[metaKeyNote] != metaValueOriginal {
		t.Errorf("second Metadata()[note] = %v; want original", again[metaKeyNote])
	}
	if _, ok := again["extra2"]; ok {
		t.Errorf("second Metadata() gained key %q after output mutation", "extra2")
	}

	if err := account.Validate(); err != nil {
		t.Errorf("Validate() after metadata mutation = %v; want nil", err)
	}

	t.Run("nil metadata becomes empty map", func(t *testing.T) {
		empty := mustAccount(t, "acct-nil-meta", core.Asset, core.NotApplicable, codeUSD, nil)
		metadata := empty.Metadata()
		if metadata == nil {
			t.Fatal("Metadata() = nil; want non-nil empty map")
		}
		if len(metadata) != 0 {
			t.Errorf("Metadata() length = %d; want 0", len(metadata))
		}
	})
}

// TestAccountImmutability verifies that Account has no exported fields and that
// copying a value leaves its accessors stable.
func TestAccountImmutability(t *testing.T) {
	typ := reflect.TypeOf(core.Account{})
	if typ.Kind() != reflect.Struct {
		t.Fatalf("Account kind = %v; want struct", typ.Kind())
	}
	if typ.NumField() == 0 {
		t.Fatal("Account has no fields")
	}
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath == "" {
			t.Errorf("Account field %s is exported; want unexported", field.Name)
		}
	}

	account := mustAccount(t, validID, core.Asset, core.FVOCI, codeUSD, map[string]any{metaKeyNote: "x"})
	copied := account

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

// TestZeroValueAccountValidate checks that validation starts with the ID field
// and remains idempotent on the zero value.
func TestZeroValueAccountValidate(t *testing.T) {
	var account core.Account

	first := account.Validate()
	assertStructuredValidationError(t, first, pkgerr.ErrInvalidAccountID, fieldID, opValidate)

	second := account.Validate()
	assertStructuredValidationError(t, second, pkgerr.ErrInvalidAccountID, fieldID, opValidate)
}

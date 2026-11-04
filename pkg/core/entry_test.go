// SPDX-License-Identifier: MIT

package core_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/Emin-ACIKGOZ/nomledger/pkg/core"
	pkgerr "github.com/Emin-ACIKGOZ/nomledger/pkg/errors"

	"github.com/shopspring/decimal"
)

const (
	entryOpNew           = "NewEntry"
	entryOpValidate      = "Entry.Validate"
	entryFieldID         = "accountID"
	entryFieldTxCurr     = "txCurrency"
	entryFieldFuncCurr   = "functionalCurrency"
	entryFieldTxAmount   = "txAmount"
	entryFieldRate       = "exchangeRate"
	entryFieldFuncAmount = "functionalAmount"
	entryFieldEntry      = "entry"

	entryAmountTen      = "10.00"
	entryRateEightyFive = "0.85"
	entryRateHalf       = "0.5"
	codeUSDLower        = "usd"
	entryIDNUL          = "acct\x00id"
	entryIDSOH          = "acct\x01id"
	entryIDUnitSep      = "acct\x1fid"
	entryIDDEL          = "acct\x7fid"
)

func mustEntryDecimal(t *testing.T, value string) decimal.Decimal {
	t.Helper()
	d, err := decimal.NewFromString(value)
	if err != nil {
		t.Fatalf("decimal.NewFromString(%q): %v", value, err)
	}
	return d
}

func mustValidEntry(t *testing.T, accountID, amount, txCurrency, rate, functionalCurrency string) core.Entry {
	t.Helper()
	entry, err := core.NewEntry(
		accountID,
		mustEntryDecimal(t, amount),
		txCurrency,
		mustEntryDecimal(t, rate),
		functionalCurrency,
	)
	if err != nil {
		t.Fatalf("NewEntry(%s, %s, %s, %s, %s): %v",
			accountID, amount, txCurrency, rate, functionalCurrency, err)
	}
	return entry
}

func assertNewEntryError(t *testing.T, entry core.Entry, err error, want error, field string) {
	t.Helper()
	assertStructuredValidationError(t, err, want, field, entryOpNew)
	assertZeroEntry(t, entry)
}

// assertZeroEntry verifies that NewEntry returned a fully zero Entry value.
func assertZeroEntry(t *testing.T, entry core.Entry) {
	t.Helper()
	assertZeroEntryScalars(t, entry)
	assertZeroEntryMoney(t, entry)
}

func assertZeroEntryScalars(t *testing.T, entry core.Entry) {
	t.Helper()
	if entry.AccountID() != "" {
		t.Errorf("AccountID() = %q; want empty", entry.AccountID())
	}
	if !entry.TxAmount().IsZero() {
		t.Errorf("TxAmount() = %s; want zero", entry.TxAmount())
	}
	if entry.TxCurrency() != "" {
		t.Errorf("TxCurrency() = %q; want empty", entry.TxCurrency())
	}
	if !entry.ExchangeRate().IsZero() {
		t.Errorf("ExchangeRate() = %s; want zero", entry.ExchangeRate())
	}
	if !entry.FunctionalAmount().IsZero() {
		t.Errorf("FunctionalAmount() = %s; want zero", entry.FunctionalAmount())
	}
	if entry.FunctionalCurrency() != "" {
		t.Errorf("FunctionalCurrency() = %q; want empty", entry.FunctionalCurrency())
	}
}

func assertZeroEntryMoney(t *testing.T, entry core.Entry) {
	t.Helper()
	if !entry.TxMoney().Amount().IsZero() || entry.TxMoney().Currency() != "" {
		t.Errorf("TxMoney() = %s %s; want zero value", entry.TxMoney().Amount(), entry.TxMoney().Currency())
	}
	if !entry.FunctionalMoney().Amount().IsZero() || entry.FunctionalMoney().Currency() != "" {
		t.Errorf("FunctionalMoney() = %s %s; want zero value",
			entry.FunctionalMoney().Amount(), entry.FunctionalMoney().Currency())
	}
}

// TestNewEntry_CanonicalMath: HalfEven functional rounding at the
// functional currency's minor units, including negative amounts and ties.
func TestNewEntry_CanonicalMath(t *testing.T) {
	cases := []struct {
		name         string
		amount       string
		rate         string
		txCurrency   string
		funcCurrency string
		want         string
	}{
		{"USD to EUR", "100.00", entryRateEightyFive, codeUSD, codeEUR, "85.00"},
		{"USD to JPY", entryAmountTen, "10.55", codeUSD, codeJPY, "106"},
		{"USD to CLF high precision", "1000", "0.00001667", codeUSD, "CLF", "0.0167"},
		{"negative USD to EUR", "-100.00", entryRateEightyFive, codeUSD, codeEUR, "-85.00"},
		{"negative USD to JPY", "-10.00", "10.55", codeUSD, codeJPY, "-106"},
		{"HalfEven tie down", "1.01", entryRateHalf, codeEUR, codeUSD, "0.50"},
		{"HalfEven tie up", "1.03", entryRateHalf, codeEUR, codeUSD, "0.52"},
		{"negative HalfEven tie down", "-1.01", entryRateHalf, codeEUR, codeUSD, "-0.50"},
		{"negative HalfEven tie up", "-1.03", entryRateHalf, codeEUR, codeUSD, "-0.52"},
		{"same currency identity rate", entryAmountTen, "1", codeUSD, codeUSD, entryAmountTen},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry := mustValidEntry(t, validID, tc.amount, tc.txCurrency, tc.rate, tc.funcCurrency)

			want := mustEntryDecimal(t, tc.want)
			if !entry.FunctionalAmount().Equal(want) {
				t.Errorf("FunctionalAmount() = %s; want %s", entry.FunctionalAmount(), want)
			}
			if entry.FunctionalAmount().IsZero() {
				t.Errorf("FunctionalAmount() = 0; want non-zero")
			}
			if !entry.TxAmount().Equal(mustEntryDecimal(t, tc.amount)) {
				t.Errorf("TxAmount() = %s; want %s", entry.TxAmount(), tc.amount)
			}
			if !entry.ExchangeRate().Equal(mustEntryDecimal(t, tc.rate)) {
				t.Errorf("ExchangeRate() = %s; want %s", entry.ExchangeRate(), tc.rate)
			}
			if err := entry.Validate(); err != nil {
				t.Errorf("Validate() = %v; want nil", err)
			}
		})
	}
}

// TestNewEntry_AccountIDValidation.
func TestNewEntry_AccountIDValidation(t *testing.T) {
	validLongID := strings.Repeat("a", maxIDBytes)

	cases := []struct {
		name    string
		id      string
		wantErr error
	}{
		{"account ID valid", validID, nil},
		{"empty", "", pkgerr.ErrInvalidAccountID},
		{"256 bytes", strings.Repeat("a", overMaxIDBytes), pkgerr.ErrInvalidAccountID},
		{"NUL", entryIDNUL, pkgerr.ErrInvalidAccountID},
		{"SOH control", entryIDSOH, pkgerr.ErrInvalidAccountID},
		{"unit separator control", entryIDUnitSep, pkgerr.ErrInvalidAccountID},
		{"DEL control", entryIDDEL, pkgerr.ErrInvalidAccountID},
		{"255 bytes", validLongID, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry, err := core.NewEntry(
				tc.id,
				mustEntryDecimal(t, "1.00"),
				codeUSD,
				mustEntryDecimal(t, "1"),
				codeUSD,
			)
			if tc.wantErr != nil {
				assertNewEntryError(t, entry, err, tc.wantErr, entryFieldID)
				return
			}
			if err != nil {
				t.Fatalf("NewEntry: unexpected error %v", err)
			}
			if entry.AccountID() != tc.id {
				t.Errorf("AccountID() = %q; want %q", entry.AccountID(), tc.id)
			}
			if err := entry.Validate(); err != nil {
				t.Errorf("Validate() = %v; want nil", err)
			}
		})
	}
}

// TestNewEntry_CurrencyValidation.
func TestNewEntry_CurrencyValidation(t *testing.T) {
	cases := []struct {
		name         string
		txCurrency   string
		funcCurrency string
		rate         string
		wantErr      error
		field        string
		wantTx       string
		wantFunc     string
	}{
		{"malformed tx", codeShort, codeEUR, "1", pkgerr.ErrInvalidCurrencyCode, entryFieldTxCurr, "", ""},
		{"spaced tx", codeSpaced, codeEUR, "1", pkgerr.ErrInvalidCurrencyCode, entryFieldTxCurr, "", ""},
		{"non-ASCII tx", codeCyrillic, codeEUR, "1", pkgerr.ErrInvalidCurrencyCode, entryFieldTxCurr, "", ""},
		{"empty tx", "", codeEUR, "1", pkgerr.ErrInvalidCurrencyCode, entryFieldTxCurr, "", ""},
		{"unknown tx", codeBGN, codeEUR, "1", pkgerr.ErrUnknownCurrency, entryFieldTxCurr, "", ""},
		{"non-ratable tx", codeXAU, codeEUR, "1", pkgerr.ErrCurrencyNotRatable, entryFieldTxCurr, "", ""},
		{"malformed functional", codeUSD, codeShort, "1", pkgerr.ErrInvalidCurrencyCode, entryFieldFuncCurr, "", ""},
		{"unknown functional", codeUSD, codeBGN, "1", pkgerr.ErrUnknownCurrency, entryFieldFuncCurr, "", ""},
		{"non-ratable functional", codeUSD, codeXAU, "1", pkgerr.ErrCurrencyNotRatable, entryFieldFuncCurr, "", ""},
		{"lowercase canonicalised", codeUSDLower, "eur", "1", nil, "", codeUSD, codeEUR},
		{"lowercase same currency", codeUSDLower, codeUSDLower, "1", nil, "", codeUSD, codeUSD},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry, err := core.NewEntry(
				validID,
				mustEntryDecimal(t, "1.00"),
				tc.txCurrency,
				mustEntryDecimal(t, tc.rate),
				tc.funcCurrency,
			)
			if tc.wantErr != nil {
				assertNewEntryError(t, entry, err, tc.wantErr, tc.field)
				return
			}
			if err != nil {
				t.Fatalf("NewEntry: unexpected error %v", err)
			}
			if entry.TxCurrency() != tc.wantTx {
				t.Errorf("TxCurrency() = %q; want %q", entry.TxCurrency(), tc.wantTx)
			}
			if entry.FunctionalCurrency() != tc.wantFunc {
				t.Errorf("FunctionalCurrency() = %q; want %q", entry.FunctionalCurrency(), tc.wantFunc)
			}
			if err := entry.Validate(); err != nil {
				t.Errorf("Validate() = %v; want nil", err)
			}
		})
	}
}

// TestNewEntry_Precision.
func TestNewEntry_Precision(t *testing.T) {
	cases := []struct {
		name         string
		amount       string
		txCurrency   string
		funcCurrency string
		wantErr      error
	}{
		{"USD 1.001 rejected", "1.001", codeUSD, codeEUR, pkgerr.ErrInvalidPrecision},
		{"USD 1.00 accepted", "1.00", codeUSD, codeEUR, nil},
		{"JPY 1.5 rejected", "1.5", codeJPY, codeUSD, pkgerr.ErrInvalidPrecision},
		{"BHD 1.234 accepted", "1.234", codeBHD, codeUSD, nil},
		{"BHD 1.2345 rejected", "1.2345", codeBHD, codeUSD, pkgerr.ErrInvalidPrecision},
		{"USD 10.123456 rejected", "10.123456", codeUSD, codeUSD, pkgerr.ErrInvalidPrecision},
		{"negative USD 1.001 rejected", "-1.001", codeUSD, codeEUR, pkgerr.ErrInvalidPrecision},
		{"JPY 100 accepted", "100", codeJPY, codeUSD, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry, err := core.NewEntry(
				validID,
				mustEntryDecimal(t, tc.amount),
				tc.txCurrency,
				mustEntryDecimal(t, "1"),
				tc.funcCurrency,
			)
			if tc.wantErr != nil {
				assertNewEntryError(t, entry, err, tc.wantErr, entryFieldTxAmount)
				return
			}
			if err != nil {
				t.Fatalf("NewEntry: unexpected error %v", err)
			}
			if err := entry.Validate(); err != nil {
				t.Errorf("Validate() = %v; want nil", err)
			}
		})
	}
}

// TestNewEntry_ZeroAmount.
func TestNewEntry_ZeroAmount(t *testing.T) {
	cases := []struct {
		name   string
		amount decimal.Decimal
	}{
		{"decimal.Zero", decimal.Zero},
		{"NewFromInt(0)", decimal.NewFromInt(0)},
		{"negative zero string", mustEntryDecimal(t, "-0.00")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry, err := core.NewEntry(validID, tc.amount, codeUSD, mustEntryDecimal(t, "1"), codeEUR)
			assertNewEntryError(t, entry, err, pkgerr.ErrZeroAmount, entryFieldTxAmount)
		})
	}
}

// TestNewEntry_ExchangeRatePositivity.
func TestNewEntry_ExchangeRatePositivity(t *testing.T) {
	cases := []struct {
		name         string
		txCurrency   string
		funcCurrency string
		rate         string
		wantErr      error
	}{
		{"negative rate", codeUSD, codeEUR, "-1", pkgerr.ErrInvalidExchangeRate},
		{"zero rate", codeUSD, codeEUR, "0", pkgerr.ErrInvalidExchangeRate},
		{"small positive foreign rate", codeUSD, codeEUR, "0.0001", nil},
		{"identity rate same currency", codeUSD, codeUSD, "1", nil},
		{"foreign rate", codeUSD, codeEUR, "1.1", nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry, err := core.NewEntry(
				validID,
				mustEntryDecimal(t, "100.00"),
				tc.txCurrency,
				mustEntryDecimal(t, tc.rate),
				tc.funcCurrency,
			)
			if tc.wantErr != nil {
				assertNewEntryError(t, entry, err, tc.wantErr, entryFieldRate)
				return
			}
			if err != nil {
				t.Fatalf("NewEntry: unexpected error %v", err)
			}
			if err := entry.Validate(); err != nil {
				t.Errorf("Validate() = %v; want nil", err)
			}
		})
	}
}

// TestNewEntry_WhenCurrenciesMatch_EnforcesIdentityRate.
func TestNewEntry_WhenCurrenciesMatch_EnforcesIdentityRate(t *testing.T) {
	cases := []struct {
		name         string
		txCurrency   string
		funcCurrency string
		rate         string
		wantErr      error
		wantTx       string
		wantFunc     string
	}{
		{"USD identity accepted", codeUSD, codeUSD, "1", nil, codeUSD, codeUSD},
		{"USD non-identity rejected", codeUSD, codeUSD, "1.1", pkgerr.ErrInvalidExchangeRate, "", ""},
		{"USD just below identity rejected", codeUSD, codeUSD, "0.9999", pkgerr.ErrInvalidExchangeRate, "", ""},
		{"EUR identity accepted", codeEUR, codeEUR, "1", nil, codeEUR, codeEUR},
		{"lowercase identity accepted", codeUSDLower, codeUSD, "1", nil, codeUSD, codeUSD},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry, err := core.NewEntry(
				validID,
				mustEntryDecimal(t, "100.00"),
				tc.txCurrency,
				mustEntryDecimal(t, tc.rate),
				tc.funcCurrency,
			)
			if tc.wantErr != nil {
				assertNewEntryError(t, entry, err, tc.wantErr, entryFieldRate)
				return
			}
			if err != nil {
				t.Fatalf("NewEntry: unexpected error %v", err)
			}
			if entry.TxCurrency() != tc.wantTx {
				t.Errorf("TxCurrency() = %q; want %q", entry.TxCurrency(), tc.wantTx)
			}
			if entry.FunctionalCurrency() != tc.wantFunc {
				t.Errorf("FunctionalCurrency() = %q; want %q", entry.FunctionalCurrency(), tc.wantFunc)
			}
			if err := entry.Validate(); err != nil {
				t.Errorf("Validate() = %v; want nil", err)
			}
		})
	}
}

// TestNewEntry_Rounding_PreventsFunctionalZeroEntries.
func TestNewEntry_Rounding_PreventsFunctionalZeroEntries(t *testing.T) {
	t.Run("0.001 BHD rounds to zero USD", func(t *testing.T) {
		entry, err := core.NewEntry(
			validID,
			mustEntryDecimal(t, "0.001"),
			codeBHD,
			mustEntryDecimal(t, "1"),
			codeUSD,
		)
		assertNewEntryError(t, entry, err, pkgerr.ErrFunctionalAmountZero, entryFieldFuncAmount)
	})

	t.Run("0.004 USD is rejected for precision", func(t *testing.T) {
		entry, err := core.NewEntry(validID, mustEntryDecimal(t, "0.004"), codeUSD, mustEntryDecimal(t, "1"), codeUSD)
		assertNewEntryError(t, entry, err, pkgerr.ErrInvalidPrecision, entryFieldTxAmount)
	})
}

// TestNewEntry_HighPrecision_PreservesDecimalIntegrity keeps the CLF case with
// exact decimal fixtures.
func TestNewEntry_HighPrecision_PreservesDecimalIntegrity(t *testing.T) {
	entry := mustValidEntry(t, validID, "1000", codeUSD, "0.00001667", "CLF")
	want := mustEntryDecimal(t, "0.0167")
	if !entry.FunctionalAmount().Equal(want) {
		t.Errorf("FunctionalAmount() = %s; want %s", entry.FunctionalAmount(), want)
	}
	if entry.FunctionalAmount().IsZero() {
		t.Error("FunctionalAmount() = 0; want non-zero")
	}
}

// TestNewEntry_Immutability.
func TestNewEntry_Immutability(t *testing.T) {
	typ := reflect.TypeOf(core.Entry{})
	if typ.Kind() != reflect.Struct {
		t.Fatalf("Entry kind = %v; want struct", typ.Kind())
	}
	if typ.NumField() == 0 {
		t.Fatal("Entry has no fields")
	}
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath == "" {
			t.Errorf("Entry field %s is exported; want unexported", field.Name)
		}
	}

	entry := mustValidEntry(t, validID, entryAmountTen, codeUSD, "1", codeUSD)
	assertEntryCopyStable(t, entry)
}

func assertEntryCopyStable(t *testing.T, entry core.Entry) {
	t.Helper()
	copied := entry
	if copied.AccountID() != entry.AccountID() {
		t.Errorf("copied AccountID() = %q; want %q", copied.AccountID(), entry.AccountID())
	}
	if !copied.TxAmount().Equal(entry.TxAmount()) {
		t.Errorf("copied TxAmount() = %s; want %s", copied.TxAmount(), entry.TxAmount())
	}
	if copied.TxCurrency() != entry.TxCurrency() {
		t.Errorf("copied TxCurrency() = %q; want %q", copied.TxCurrency(), entry.TxCurrency())
	}
	if !copied.ExchangeRate().Equal(entry.ExchangeRate()) {
		t.Errorf("copied ExchangeRate() = %s; want %s", copied.ExchangeRate(), entry.ExchangeRate())
	}
	if !copied.FunctionalAmount().Equal(entry.FunctionalAmount()) {
		t.Errorf("copied FunctionalAmount() = %s; want %s", copied.FunctionalAmount(), entry.FunctionalAmount())
	}
	if copied.FunctionalCurrency() != entry.FunctionalCurrency() {
		t.Errorf("copied FunctionalCurrency() = %q; want %q",
			copied.FunctionalCurrency(), entry.FunctionalCurrency())
	}
}

// TestNewEntry_Accessors.
func TestNewEntry_Accessors(t *testing.T) {
	entry := mustValidEntry(t, validID, "12.34", "eur", "1.25", codeUSDLower)

	assertEntryScalarAccessors(t, entry)
	assertEntryMoneyAccessors(t, entry)

	if err := entry.Validate(); err != nil {
		t.Errorf("Validate() = %v; want nil", err)
	}
}

func assertEntryScalarAccessors(t *testing.T, entry core.Entry) {
	t.Helper()
	if entry.AccountID() != validID {
		t.Errorf("AccountID() = %q; want %q", entry.AccountID(), validID)
	}
	if !entry.TxAmount().Equal(mustEntryDecimal(t, "12.34")) {
		t.Errorf("TxAmount() = %s; want 12.34", entry.TxAmount())
	}
	if entry.TxCurrency() != codeEUR {
		t.Errorf("TxCurrency() = %q; want %q", entry.TxCurrency(), codeEUR)
	}
	if !entry.ExchangeRate().Equal(mustEntryDecimal(t, "1.25")) {
		t.Errorf("ExchangeRate() = %s; want 1.25", entry.ExchangeRate())
	}
	if !entry.FunctionalAmount().Equal(mustEntryDecimal(t, "15.42")) {
		t.Errorf("FunctionalAmount() = %s; want 15.42", entry.FunctionalAmount())
	}
	if entry.FunctionalCurrency() != codeUSD {
		t.Errorf("FunctionalCurrency() = %q; want %q", entry.FunctionalCurrency(), codeUSD)
	}
}

func assertEntryMoneyAccessors(t *testing.T, entry core.Entry) {
	t.Helper()
	if !entry.TxMoney().Amount().Equal(mustEntryDecimal(t, "12.34")) {
		t.Errorf("TxMoney().Amount() = %s; want 12.34", entry.TxMoney().Amount())
	}
	if entry.TxMoney().Currency() != codeEUR {
		t.Errorf("TxMoney().Currency() = %q; want %q", entry.TxMoney().Currency(), codeEUR)
	}
	if !entry.FunctionalMoney().Amount().Equal(mustEntryDecimal(t, "15.42")) {
		t.Errorf("FunctionalMoney().Amount() = %s; want 15.42", entry.FunctionalMoney().Amount())
	}
	if entry.FunctionalMoney().Currency() != codeUSD {
		t.Errorf("FunctionalMoney().Currency() = %q; want %q", entry.FunctionalMoney().Currency(), codeUSD)
	}
}

// TestNewEntry_Validate.
func TestNewEntry_Validate(t *testing.T) {
	t.Run("zero-value entry", func(t *testing.T) {
		var entry core.Entry
		first := entry.Validate()
		assertStructuredValidationError(t, first, pkgerr.ErrInvalidEntry, entryFieldEntry, entryOpValidate)
		if errors.Is(first, pkgerr.ErrInvalidAccountID) {
			t.Error("zero-value Validate() matched ErrInvalidAccountID; want ErrInvalidEntry")
		}
		second := entry.Validate()
		assertStructuredValidationError(t, second, pkgerr.ErrInvalidEntry, entryFieldEntry, entryOpValidate)
	})

	t.Run("valid entry is idempotent", func(t *testing.T) {
		entry := mustValidEntry(t, validID, entryAmountTen, codeUSD, "1", codeUSD)
		if err := entry.Validate(); err != nil {
			t.Errorf("Validate() = %v; want nil", err)
		}
		if err := entry.Validate(); err != nil {
			t.Errorf("second Validate() = %v; want nil", err)
		}
	})
}

// TestNewEntry_FunctionalAmountNonZero: every valid entry has a
// non-zero functional amount; negative functional amounts are allowed.
func TestNewEntry_FunctionalAmountNonZero(t *testing.T) {
	cases := []struct {
		name         string
		amount       string
		rate         string
		txCurrency   string
		funcCurrency string
		wantNegative bool
	}{
		{"positive USD", "1.00", "1", codeUSD, codeUSD, false},
		{"negative USD", "-1.00", "1", codeUSD, codeUSD, true},
		{"positive foreign", "100.00", entryRateEightyFive, codeUSD, codeEUR, false},
		{"negative foreign", "-100.00", entryRateEightyFive, codeUSD, codeEUR, true},
		{"small positive", "0.01", "1", codeUSD, codeUSD, false},
		{"small negative", "-0.01", "1", codeUSD, codeUSD, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry := mustValidEntry(t, validID, tc.amount, tc.txCurrency, tc.rate, tc.funcCurrency)
			if entry.FunctionalAmount().IsZero() {
				t.Fatalf("FunctionalAmount() = 0; want non-zero")
			}
			if gotNegative := entry.FunctionalAmount().Sign() < 0; gotNegative != tc.wantNegative {
				t.Errorf("FunctionalAmount() = %s; negative = %t; want %t",
					entry.FunctionalAmount(), gotNegative, tc.wantNegative)
			}
			if err := entry.Validate(); err != nil {
				t.Errorf("Validate() = %v; want nil", err)
			}
		})
	}
}

// SPDX-License-Identifier: MIT

package currency_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/Emin-ACIKGOZ/nomledger/pkg/currency"
	pkgerr "github.com/Emin-ACIKGOZ/nomledger/pkg/errors"

	"github.com/shopspring/decimal"
)

// TestMoneyImmutability: Money is a value type with unexported fields,
// and operations on copies do not change the original.
func TestMoneyImmutability(t *testing.T) {
	t.Run("no exported fields", testMoneyHasNoExportedFields)
	t.Run("copy operations are independent", testMoneyCopyIndependent)
	t.Run("repeated accessors are stable", testMoneyAccessorsStable)
}

func testMoneyHasNoExportedFields(t *testing.T) {
	t.Helper()
	typ := reflect.TypeOf(currency.Money{})
	if typ.Kind() != reflect.Struct {
		t.Fatalf("Money kind = %v; want struct", typ.Kind())
	}
	if typ.NumField() == 0 {
		t.Fatal("Money has no fields")
	}
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath == "" {
			t.Errorf("Money field %s is exported; want unexported", field.Name)
		}
	}
}

func testMoneyCopyIndependent(t *testing.T) {
	t.Helper()
	original := mustMoney(t, "1.23", codeUSD)
	copied := original

	negated := copied.Neg()
	if !negated.Amount().Equal(mustDecimal(t, "-1.23")) {
		t.Errorf("Neg() amount = %s; want -1.23", negated.Amount())
	}

	other := mustMoney(t, "2.00", codeUSD)
	sum, err := copied.Add(other)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if !sum.Amount().Equal(mustDecimal(t, "3.23")) {
		t.Errorf("Add amount = %s; want 3.23", sum.Amount())
	}
	difference, err := copied.Sub(other)
	if err != nil {
		t.Fatalf("Sub: %v", err)
	}
	if !difference.Amount().Equal(mustDecimal(t, "-0.77")) {
		t.Errorf("Sub amount = %s; want -0.77", difference.Amount())
	}

	assertMoneyValue(t, "original", original, "1.23", codeUSD)
	assertMoneyValue(t, "copy", copied, "1.23", codeUSD)
}

func testMoneyAccessorsStable(t *testing.T) {
	t.Helper()
	m := mustMoney(t, "10.50", codeBHD)
	for i := 0; i < 3; i++ {
		if !m.Amount().Equal(mustDecimal(t, "10.50")) {
			t.Errorf("access %d Amount = %s; want 10.50", i, m.Amount())
		}
		if m.Currency() != codeBHD {
			t.Errorf("access %d Currency = %s; want %s", i, m.Currency(), codeBHD)
		}
	}
}

func assertMoneyValue(t *testing.T, name string, m currency.Money, wantAmount string, wantCode currency.Code) {
	t.Helper()
	if !m.Amount().Equal(mustDecimal(t, wantAmount)) {
		t.Errorf("%s Amount = %s; want %s", name, m.Amount(), wantAmount)
	}
	if m.Currency() != wantCode {
		t.Errorf("%s Currency = %s; want %s", name, m.Currency(), wantCode)
	}
}

// TestMoneyArithmetic: currency-safe addition/subtraction with results
// at or below the currency's minor-unit scale.
func TestMoneyArithmetic(t *testing.T) {
	t.Run("add", testMoneyAdd)
	t.Run("sub", testMoneySub)
	t.Run("result scale does not exceed minor units", testMoneyResultScale)
	t.Run("cross-currency mismatch", testMoneyCrossCurrencyMismatch)
	t.Run("mixed case codes normalise", testMoneyMixedCaseNormalise)
	t.Run("neg and zero", testMoneyNegAndZero)
	t.Run("positive exponent operands", testMoneyPositiveExponentOperands)
}

func testMoneyAdd(t *testing.T) {
	t.Helper()
	sum, err := mustMoney(t, usdOne, codeUSD).Add(mustMoney(t, "2.00", codeUSD))
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	assertMoneyValue(t, "sum", sum, "3.00", codeUSD)
}

func testMoneySub(t *testing.T) {
	t.Helper()
	difference, err := mustMoney(t, "2.00", codeUSD).Sub(mustMoney(t, usdOne, codeUSD))
	if err != nil {
		t.Fatalf("Sub: %v", err)
	}
	assertMoneyValue(t, "difference", difference, usdOne, codeUSD)
}

func testMoneyResultScale(t *testing.T) {
	t.Helper()
	cases := []struct {
		name string
		a    string
		b    string
		code currency.Code
		want string
	}{
		{codeUSD, usdOne, "2.00", codeUSD, "3.00"},
		{"BHD", bhdOneTwoThreeFour, bhdOneTwoThreeFour, codeBHD, "2.468"},
		{codeJPY, "100", "200", codeJPY, "300"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sum, err := mustMoney(t, tc.a, tc.code).Add(mustMoney(t, tc.b, tc.code))
			if err != nil {
				t.Fatalf("Add: %v", err)
			}
			if !sum.Amount().Equal(mustDecimal(t, tc.want)) {
				t.Errorf("sum = %s; want %s", sum.Amount(), tc.want)
			}
			minorUnits, _ := mustLookup(t, tc.code).MinorUnits()
			if sum.Amount().Exponent() < -minorUnits {
				t.Errorf("%s sum exponent = %d; scale exceeds %d minor units",
					tc.code, sum.Amount().Exponent(), minorUnits)
			}
		})
	}
}

func testMoneyCrossCurrencyMismatch(t *testing.T) {
	t.Helper()
	usd := mustMoney(t, usdOne, codeUSD)
	eur := mustMoney(t, "2.00", "EUR")
	if _, err := usd.Add(eur); !errors.Is(err, pkgerr.ErrCurrencyMismatch) {
		t.Errorf("Add mismatch error = %v; want %v", err, pkgerr.ErrCurrencyMismatch)
	}
	if _, err := usd.Sub(eur); !errors.Is(err, pkgerr.ErrCurrencyMismatch) {
		t.Errorf("Sub mismatch error = %v; want %v", err, pkgerr.ErrCurrencyMismatch)
	}
}

func testMoneyMixedCaseNormalise(t *testing.T) {
	t.Helper()
	lower := mustMoney(t, usdOne, codeUSDLower)
	mixed := mustMoney(t, "2.00", codeUSDMixedCase)
	if lower.Currency() != codeUSD {
		t.Errorf("lowercase Currency = %s; want %s", lower.Currency(), codeUSD)
	}
	if mixed.Currency() != codeUSD {
		t.Errorf("mixed-case Currency = %s; want %s", mixed.Currency(), codeUSD)
	}
	sum, err := lower.Add(mixed)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	assertMoneyValue(t, "sum", sum, "3.00", codeUSD)
}

func testMoneyNegAndZero(t *testing.T) {
	t.Helper()
	m := mustMoney(t, usdOne, codeUSD)
	negated := m.Neg()
	assertMoneyValue(t, "negated", negated, usdNegOne, codeUSD)
	assertMoneyValue(t, "original", m, usdOne, codeUSD)

	if !mustMoney(t, "0", codeUSD).IsZero() {
		t.Error("Money(0 USD).IsZero() = false; want true")
	}
	if mustMoney(t, usdOne, codeUSD).IsZero() {
		t.Error("Money(1.00 USD).IsZero() = true; want false")
	}
}

func testMoneyPositiveExponentOperands(t *testing.T) {
	t.Helper()
	// decimal.New(12, 1) is 120 (exponent +1), not a scaled integer.
	oneTwenty := decimal.New(12, 1)
	thirty := decimal.New(3, 1)
	cases := []struct {
		name           string
		code           currency.Code
		wantSum        decimal.Decimal
		wantDiff       decimal.Decimal
		wantSumString  string
		wantDiffString string
	}{
		{"USD", codeUSD, decimal.New(15, 1), decimal.New(9, 1), "150.00 USD", "90.00 USD"},
		{"JPY", codeJPY, decimal.New(15, 1), decimal.New(9, 1), "150 JPY", "90 JPY"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := mustMoneyDecimal(t, oneTwenty, tc.code)
			b := mustMoneyDecimal(t, thirty, tc.code)

			sum, err := a.Add(b)
			if err != nil {
				t.Fatalf("Add: %v", err)
			}
			if !sum.Amount().Equal(tc.wantSum) {
				t.Errorf("sum = %s; want %s", sum.Amount(), tc.wantSum)
			}
			if sum.String() != tc.wantSumString {
				t.Errorf("sum String() = %q; want %q", sum.String(), tc.wantSumString)
			}
			minorUnits, _ := mustLookup(t, tc.code).MinorUnits()
			if sum.Amount().Exponent() < -minorUnits {
				t.Errorf("%s sum exponent = %d; scale exceeds %d minor units",
					tc.code, sum.Amount().Exponent(), minorUnits)
			}

			difference, err := a.Sub(b)
			if err != nil {
				t.Fatalf("Sub: %v", err)
			}
			if !difference.Amount().Equal(tc.wantDiff) {
				t.Errorf("difference = %s; want %s", difference.Amount(), tc.wantDiff)
			}
			if difference.String() != tc.wantDiffString {
				t.Errorf("difference String() = %q; want %q", difference.String(), tc.wantDiffString)
			}
			if difference.Amount().Exponent() < -minorUnits {
				t.Errorf("%s difference exponent = %d; scale exceeds %d minor units",
					tc.code, difference.Amount().Exponent(), minorUnits)
			}
		})
	}
}

// TestNewMoneyPrecision: NewMoney rejects excess scale without
// silently rounding.
func TestNewMoneyPrecision(t *testing.T) {
	cases := []struct {
		name    string
		amount  string
		code    currency.Code
		wantErr error
	}{
		{"USD 1.001 rejected", amtOnePointZeroZeroOne, codeUSD, pkgerr.ErrInvalidPrecision},
		{"USD -1.001 rejected", amtNegOnePointZeroZeroOne, codeUSD, pkgerr.ErrInvalidPrecision},
		{"USD 1.230 rejected", amtScaleThree, codeUSD, pkgerr.ErrInvalidPrecision},
		{"JPY 1.5 rejected", "1.5", codeJPY, pkgerr.ErrInvalidPrecision},
		{"BHD 1.2345 rejected", "1.2345", codeBHD, pkgerr.ErrInvalidPrecision},
		{"USD 1.00 accepted", usdOne, codeUSD, nil},
		{"USD 1.2 accepted", amtOnePointTwo, codeUSD, nil},
		{"BHD 1.234 accepted", bhdOneTwoThreeFour, codeBHD, nil},
		{"BHD 1.230 accepted", amtScaleThree, codeBHD, nil},
		{"JPY 100 accepted", "100", codeJPY, nil},
		{"zero accepted", "0", codeUSD, nil},
		{"zero with cents accepted", "0.00", codeUSD, nil},
		{"lowercase accepted", usdOne, codeUSDLower, nil},
		{"non-ratable rejected", "1", codeXAU, pkgerr.ErrCurrencyNotRatable},
		{"malformed code rejected", "1", "US", pkgerr.ErrInvalidCurrencyCode},
		{"unknown code rejected", "1", codeUnknown, pkgerr.ErrUnknownCurrency},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			amount := mustDecimal(t, tc.amount)
			m, err := currency.NewMoney(amount, tc.code)

			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("NewMoney(%s, %s) error = %v; want %v", tc.amount, tc.code, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewMoney(%s, %s): unexpected error %v", tc.amount, tc.code, err)
			}
			if !m.Amount().Equal(amount) {
				t.Errorf("NewMoney(%s, %s).Amount() = %s; want %s", tc.amount, tc.code, m.Amount(), amount)
			}
			if wantCode := currency.Code(strings.ToUpper(string(tc.code))); m.Currency() != wantCode {
				t.Errorf("NewMoney(%s, %s).Currency() = %s; want %s", tc.amount, tc.code, m.Currency(), wantCode)
			}
		})
	}

	t.Run("Unnecessary accepts what NewMoney rejects", testNewMoneyUnnecessary)
	t.Run("positive exponent amounts", testNewMoneyPositiveExponent)
	t.Run("negative zero representations", testNewMoneyNegativeZero)
}

func testNewMoneyUnnecessary(t *testing.T) {
	t.Helper()
	amount := mustDecimal(t, amtScaleThree)
	if _, err := currency.NewMoney(amount, codeUSD); !errors.Is(err, pkgerr.ErrInvalidPrecision) {
		t.Fatalf("NewMoney(1.230, USD) error = %v; want %v", err, pkgerr.ErrInvalidPrecision)
	}
	rounded, err := currency.Round(amount, codeUSD, currency.RoundUnnecessary)
	if err != nil {
		t.Fatalf("Round(1.230, USD, Unnecessary): %v", err)
	}
	if !rounded.Equal(mustDecimal(t, "1.23")) {
		t.Errorf("Round(1.230, USD, Unnecessary) = %s; want 1.23", rounded)
	}
}

func testNewMoneyPositiveExponent(t *testing.T) {
	t.Helper()
	// decimal.New(12, 1) is 120, with exponent +1 rather than -2.
	amount := decimal.New(12, 1)
	cases := []struct {
		code       currency.Code
		wantString string
	}{
		{codeUSD, "120.00 USD"},
		{codeJPY, "120 JPY"},
	}
	for _, tc := range cases {
		t.Run(string(tc.code), func(t *testing.T) {
			m, err := currency.NewMoney(amount, tc.code)
			if err != nil {
				t.Fatalf("NewMoney(120, %s): %v", tc.code, err)
			}
			if !m.Amount().Equal(amount) {
				t.Errorf("Amount() = %s; want %s", m.Amount(), amount)
			}
			if m.String() != tc.wantString {
				t.Errorf("String() = %q; want %q", m.String(), tc.wantString)
			}
		})
	}
}

func testNewMoneyNegativeZero(t *testing.T) {
	t.Helper()
	m, err := currency.NewMoney(mustDecimal(t, "-0.00"), codeUSD)
	if err != nil {
		t.Fatalf("NewMoney(-0.00, USD): %v", err)
	}
	if !m.IsZero() {
		t.Error("NewMoney(-0.00, USD).IsZero() = false; want true")
	}
	if !m.Amount().IsZero() {
		t.Errorf("NewMoney(-0.00, USD).Amount() = %s; want zero", m.Amount())
	}
	if _, err := currency.NewMoney(mustDecimal(t, "-0.000"), codeUSD); !errors.Is(err, pkgerr.ErrInvalidPrecision) {
		t.Errorf("NewMoney(-0.000, USD) error = %v; want %v", err, pkgerr.ErrInvalidPrecision)
	}
}

// TestMoneyZeroValue verifies that the zero-value Money is safe to inspect,
// negate, and format.
func TestMoneyZeroValue(t *testing.T) {
	var zero currency.Money

	if !zero.Amount().IsZero() {
		t.Errorf("zero Amount() = %s; want zero", zero.Amount())
	}
	if zero.Currency() != "" {
		t.Errorf("zero Currency() = %q; want empty", zero.Currency())
	}
	if !zero.IsZero() {
		t.Error("zero IsZero() = false; want true")
	}
	if got := zero.String(); got != "0" {
		t.Errorf("zero String() = %q; want %q", got, "0")
	}

	negated := zero.Neg()
	if !negated.Amount().IsZero() {
		t.Errorf("zero Neg().Amount() = %s; want zero", negated.Amount())
	}
	if negated.Currency() != "" {
		t.Errorf("zero Neg().Currency() = %q; want empty", negated.Currency())
	}
	if !negated.IsZero() {
		t.Error("zero Neg().IsZero() = false; want true")
	}
	if got := negated.String(); got != "0" {
		t.Errorf("zero Neg().String() = %q; want %q", got, "0")
	}
}

// TestMoneyString asserts the documented "<amount> <CODE>" formatting at the
// currency's minor-unit scale.
func TestMoneyString(t *testing.T) {
	cases := []struct {
		amount string
		code   currency.Code
		want   string
	}{
		{"10", codeUSD, "10.00 USD"},
		{amtOnePointTwo, codeUSD, "1.20 USD"},
		{"0", codeUSD, "0.00 USD"},
		{usdNegOne, codeUSD, "-1.00 USD"},
		{bhdOneTwoThreeFour, codeBHD, "1.234 BHD"},
		{amtOnePointTwo, codeBHD, "1.200 BHD"},
		{"1234.5", codeBHD, "1234.500 BHD"},
		{"100", codeJPY, "100 JPY"},
		{"101", codeJPY, "101 JPY"},
	}

	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			got := mustMoney(t, tc.amount, tc.code).String()
			if got != tc.want {
				t.Errorf("Money.String() = %q; want %q", got, tc.want)
			}
		})
	}
}

func mustMoney(t *testing.T, amount string, code currency.Code) currency.Money {
	t.Helper()
	return mustMoneyDecimal(t, mustDecimal(t, amount), code)
}

func mustMoneyDecimal(t *testing.T, amount decimal.Decimal, code currency.Code) currency.Money {
	t.Helper()
	m, err := currency.NewMoney(amount, code)
	if err != nil {
		t.Fatalf("NewMoney(%s, %s): %v", amount, code, err)
	}
	return m
}

func mustLookup(t *testing.T, code currency.Code) currency.Currency {
	t.Helper()
	c, err := currency.Lookup(string(code))
	if err != nil {
		t.Fatalf("Lookup(%s): %v", code, err)
	}
	return c
}

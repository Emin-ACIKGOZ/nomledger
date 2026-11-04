// SPDX-License-Identifier: MIT

package currency_test

import (
	"errors"
	"testing"

	"github.com/Emin-ACIKGOZ/nomledger/pkg/currency"
	pkgerr "github.com/Emin-ACIKGOZ/nomledger/pkg/errors"

	"github.com/shopspring/decimal"
)

// Shared decimal fixtures used across the rounding and Money tables.
const (
	usdOne                    = "1.00"
	usdNegOne                 = "-1.00"
	amtOnePointZeroZeroOne    = "1.001"
	amtNegOnePointZeroZeroOne = "-1.001"
	amtBelowTie               = "1.004"
	amtAboveTie               = "1.006"
	jpyHalf                   = "100.5"
	jpyNegHalf                = "-100.5"
	jpyAboveHalf              = "100.6"
	amtNegOnePointZeroOne     = "-1.01"
	amtNegTieOdd              = "-1.015"
	amtTieOdd                 = "1.015"
	amtOnePointTwo            = "1.2"
	amtScaleThree             = "1.230"
	bhdOneTwoThreeFour        = "1.234"
	bhdOne                    = "1.000"
	bhdNegOne                 = "-1.000"
	bhdAboveTie               = "1.0006"
	amtTieEven                = "1.005"
	amtNegTieEven             = "-1.005"
	amtOnePointZeroOne        = "1.01"
	bhdTie                    = "1.0005"
	bhdNegTie                 = "-1.0005"
	codeUSDLower              = "usd"
)

type roundCase struct {
	name    string
	amount  string
	code    currency.Code
	mode    currency.RoundingMode
	want    string
	wantErr error
}

var roundModeNames = []struct {
	name string
	mode currency.RoundingMode
}{
	{"HalfEven", currency.RoundHalfEven},
	{"HalfUp", currency.RoundHalfUp},
	{"HalfDown", currency.RoundHalfDown},
	{"Floor", currency.RoundFloor},
	{"Ceiling", currency.RoundCeiling},
}

// TestRoundModes: every mode across positive, negative, and exact-half
// values at USD (2), JPY (0), and BHD (3) scales.
func TestRoundModes(t *testing.T) {
	cases := []roundCase{
		// HalfEven: ties to the nearest even digit.
		{"USD HalfEven 1.005", amtTieEven, codeUSD, currency.RoundHalfEven, usdOne, nil},
		{"USD HalfEven 1.015", amtTieOdd, codeUSD, currency.RoundHalfEven, "1.02", nil},
		{"USD HalfEven -1.005", amtNegTieEven, codeUSD, currency.RoundHalfEven, usdNegOne, nil},
		{"USD HalfEven -1.015", amtNegTieOdd, codeUSD, currency.RoundHalfEven, "-1.02", nil},
		{"USD HalfEven 1.245", "1.245", codeUSD, currency.RoundHalfEven, "1.24", nil},
		{"USD HalfEven 2.675", "2.675", codeUSD, currency.RoundHalfEven, "2.68", nil},
		{"USD HalfEven below tie", amtBelowTie, codeUSD, currency.RoundHalfEven, usdOne, nil},
		{"USD HalfEven above tie", amtAboveTie, codeUSD, currency.RoundHalfEven, amtOnePointZeroOne, nil},

		// HalfUp: ties away from zero, non-ties to nearest.
		{"USD HalfUp 1.005", amtTieEven, codeUSD, currency.RoundHalfUp, amtOnePointZeroOne, nil},
		{"USD HalfUp -1.005", amtNegTieEven, codeUSD, currency.RoundHalfUp, amtNegOnePointZeroOne, nil},
		{"USD HalfUp 1.015", amtTieOdd, codeUSD, currency.RoundHalfUp, "1.02", nil},
		{"USD HalfUp -1.015", amtNegTieOdd, codeUSD, currency.RoundHalfUp, "-1.02", nil},
		{"USD HalfUp below tie", amtBelowTie, codeUSD, currency.RoundHalfUp, usdOne, nil},
		{"USD HalfUp negative below tie", "-1.004", codeUSD, currency.RoundHalfUp, usdNegOne, nil},
		{"USD HalfUp above tie", amtAboveTie, codeUSD, currency.RoundHalfUp, amtOnePointZeroOne, nil},

		// HalfDown: ties toward zero, non-ties to nearest.
		{"USD HalfDown 1.005", amtTieEven, codeUSD, currency.RoundHalfDown, usdOne, nil},
		{"USD HalfDown -1.005", amtNegTieEven, codeUSD, currency.RoundHalfDown, usdNegOne, nil},
		{"USD HalfDown 1.015", amtTieOdd, codeUSD, currency.RoundHalfDown, amtOnePointZeroOne, nil},
		{"USD HalfDown -1.015", amtNegTieOdd, codeUSD, currency.RoundHalfDown, amtNegOnePointZeroOne, nil},
		{"USD HalfDown above tie", amtAboveTie, codeUSD, currency.RoundHalfDown, amtOnePointZeroOne, nil},
		{"USD HalfDown negative above tie", "-1.006", codeUSD, currency.RoundHalfDown, amtNegOnePointZeroOne, nil},
		{"USD HalfDown below tie", amtBelowTie, codeUSD, currency.RoundHalfDown, usdOne, nil},
		{"USD HalfDown just below tie", "1.0049", codeUSD, currency.RoundHalfDown, usdOne, nil},
		{"USD HalfDown negative just below tie", "-1.0049", codeUSD, currency.RoundHalfDown, usdNegOne, nil},
		{"BHD HalfDown high-scale tie", "1.0005000", codeBHD, currency.RoundHalfDown, bhdOne, nil},
		{"BHD HalfDown negative high-scale tie", "-1.0005000", codeBHD, currency.RoundHalfDown, bhdNegOne, nil},
		{"CLF HalfDown high-scale tie", "0.01675", codeCLF, currency.RoundHalfDown, "0.0167", nil},
		{"CLF HalfDown negative high-scale tie", "-0.01675", codeCLF, currency.RoundHalfDown, "-0.0167", nil},

		// Floor and Ceiling on positive and negative values.
		{"USD Floor 1.009", "1.009", codeUSD, currency.RoundFloor, usdOne, nil},
		{"USD Floor 1.001", amtOnePointZeroZeroOne, codeUSD, currency.RoundFloor, usdOne, nil},
		{"USD Floor -1.001", amtNegOnePointZeroZeroOne, codeUSD, currency.RoundFloor, amtNegOnePointZeroOne, nil},
		{"USD Floor -1.009", "-1.009", codeUSD, currency.RoundFloor, amtNegOnePointZeroOne, nil},
		{"USD Floor exact", usdOne, codeUSD, currency.RoundFloor, usdOne, nil},
		{"USD Ceiling 1.001", amtOnePointZeroZeroOne, codeUSD, currency.RoundCeiling, amtOnePointZeroOne, nil},
		{"USD Ceiling 1.009", "1.009", codeUSD, currency.RoundCeiling, amtOnePointZeroOne, nil},
		{"USD Ceiling -1.001", amtNegOnePointZeroZeroOne, codeUSD, currency.RoundCeiling, usdNegOne, nil},
		{"USD Ceiling -1.009", "-1.009", codeUSD, currency.RoundCeiling, usdNegOne, nil},
		{"USD Ceiling exact", usdOne, codeUSD, currency.RoundCeiling, usdOne, nil},

		// Unnecessary: exact values pass, inexact values are rejected.
		{"USD Unnecessary 1.230", amtScaleThree, codeUSD, currency.RoundUnnecessary, "1.23", nil},
		{"USD Unnecessary 1.2", amtOnePointTwo, codeUSD, currency.RoundUnnecessary, "1.20", nil},
		{"USD Unnecessary exact", usdOne, codeUSD, currency.RoundUnnecessary, usdOne, nil},
		{"USD Unnecessary inexact", "1.231", codeUSD, currency.RoundUnnecessary, "", pkgerr.ErrInexactRounding},
		{"USD Unnecessary negative inexact", "-1.231", codeUSD, currency.RoundUnnecessary, "", pkgerr.ErrInexactRounding},
		{"USD Unnecessary tie", amtTieEven, codeUSD, currency.RoundUnnecessary, "", pkgerr.ErrInexactRounding},

		// JPY (zero minor units).
		{"JPY HalfEven 100.5", jpyHalf, codeJPY, currency.RoundHalfEven, "100", nil},
		{"JPY HalfEven 101.5", "101.5", codeJPY, currency.RoundHalfEven, "102", nil},
		{"JPY HalfEven -100.5", jpyNegHalf, codeJPY, currency.RoundHalfEven, "-100", nil},
		{"JPY HalfEven -101.5", "-101.5", codeJPY, currency.RoundHalfEven, "-102", nil},
		{"JPY HalfEven 100.4", "100.4", codeJPY, currency.RoundHalfEven, "100", nil},
		{"JPY HalfEven 100.6", jpyAboveHalf, codeJPY, currency.RoundHalfEven, "101", nil},
		{"JPY HalfUp 100.5", jpyHalf, codeJPY, currency.RoundHalfUp, "101", nil},
		{"JPY HalfUp -100.5", jpyNegHalf, codeJPY, currency.RoundHalfUp, "-101", nil},
		{"JPY HalfUp 100.4", "100.4", codeJPY, currency.RoundHalfUp, "100", nil},
		{"JPY HalfUp 100.6", jpyAboveHalf, codeJPY, currency.RoundHalfUp, "101", nil},
		{"JPY HalfDown 100.5", jpyHalf, codeJPY, currency.RoundHalfDown, "100", nil},
		{"JPY HalfDown -100.5", jpyNegHalf, codeJPY, currency.RoundHalfDown, "-100", nil},
		{"JPY HalfDown 100.6", jpyAboveHalf, codeJPY, currency.RoundHalfDown, "101", nil},
		{"JPY HalfDown -100.6", "-100.6", codeJPY, currency.RoundHalfDown, "-101", nil},
		{"JPY Floor 100.9", "100.9", codeJPY, currency.RoundFloor, "100", nil},
		{"JPY Floor -100.1", "-100.1", codeJPY, currency.RoundFloor, "-101", nil},
		{"JPY Floor exact", "100", codeJPY, currency.RoundFloor, "100", nil},
		{"JPY Ceiling 100.1", "100.1", codeJPY, currency.RoundCeiling, "101", nil},
		{"JPY Ceiling -100.9", "-100.9", codeJPY, currency.RoundCeiling, "-100", nil},
		{"JPY Unnecessary exact", "100", codeJPY, currency.RoundUnnecessary, "100", nil},
		{"JPY Unnecessary decimal zero", "100.0", codeJPY, currency.RoundUnnecessary, "100", nil},
		{"JPY Unnecessary inexact", "1.5", codeJPY, currency.RoundUnnecessary, "", pkgerr.ErrInexactRounding},

		// BHD (three minor units).
		{"BHD HalfEven 1.0005", bhdTie, codeBHD, currency.RoundHalfEven, bhdOne, nil},
		{"BHD HalfEven 1.0015", "1.0015", codeBHD, currency.RoundHalfEven, "1.002", nil},
		{"BHD HalfEven -1.0005", bhdNegTie, codeBHD, currency.RoundHalfEven, bhdNegOne, nil},
		{"BHD HalfEven -1.0015", "-1.0015", codeBHD, currency.RoundHalfEven, "-1.002", nil},
		{"BHD HalfEven below tie", "1.0004", codeBHD, currency.RoundHalfEven, bhdOne, nil},
		{"BHD HalfEven above tie", bhdAboveTie, codeBHD, currency.RoundHalfEven, amtOnePointZeroZeroOne, nil},
		{"BHD HalfUp 1.0005", bhdTie, codeBHD, currency.RoundHalfUp, amtOnePointZeroZeroOne, nil},
		{"BHD HalfUp -1.0005", bhdNegTie, codeBHD, currency.RoundHalfUp, amtNegOnePointZeroZeroOne, nil},
		{"BHD HalfUp 1.0004", "1.0004", codeBHD, currency.RoundHalfUp, bhdOne, nil},
		{"BHD HalfUp 1.0006", bhdAboveTie, codeBHD, currency.RoundHalfUp, amtOnePointZeroZeroOne, nil},
		{"BHD HalfDown 1.0005", bhdTie, codeBHD, currency.RoundHalfDown, bhdOne, nil},
		{"BHD HalfDown -1.0005", bhdNegTie, codeBHD, currency.RoundHalfDown, bhdNegOne, nil},
		{"BHD HalfDown 1.0006", bhdAboveTie, codeBHD, currency.RoundHalfDown, amtOnePointZeroZeroOne, nil},
		{"BHD HalfDown -1.0006", "-1.0006", codeBHD, currency.RoundHalfDown, amtNegOnePointZeroZeroOne, nil},
		{"BHD Floor 1.0009", "1.0009", codeBHD, currency.RoundFloor, bhdOne, nil},
		{"BHD Floor -1.0001", "-1.0001", codeBHD, currency.RoundFloor, amtNegOnePointZeroZeroOne, nil},
		{"BHD Floor exact", bhdOneTwoThreeFour, codeBHD, currency.RoundFloor, bhdOneTwoThreeFour, nil},
		{"BHD Ceiling 1.0001", "1.0001", codeBHD, currency.RoundCeiling, amtOnePointZeroZeroOne, nil},
		{"BHD Ceiling -1.0009", "-1.0009", codeBHD, currency.RoundCeiling, bhdNegOne, nil},
		{"BHD Unnecessary exact", bhdOneTwoThreeFour, codeBHD, currency.RoundUnnecessary, bhdOneTwoThreeFour, nil},
		{"BHD Unnecessary trailing zero", amtScaleThree, codeBHD, currency.RoundUnnecessary, amtScaleThree, nil},
		{"BHD Unnecessary excess zero", "1.2340", codeBHD, currency.RoundUnnecessary, bhdOneTwoThreeFour, nil},
		{"BHD Unnecessary inexact", "1.2345", codeBHD, currency.RoundUnnecessary, "", pkgerr.ErrInexactRounding},

		// Lookup normalisation applies to Round codes too.
		{"USD HalfEven lowercase code", amtTieEven, codeUSDLower, currency.RoundHalfEven, usdOne, nil},

		// Invalid modes and currency lookup failures.
		{"unknown mode", usdOne, codeUSD, currency.RoundingMode(99), "", pkgerr.ErrInvalidRoundingMode},
		{"non-ratable currency", usdOne, codeXAU, currency.RoundHalfEven, "", pkgerr.ErrCurrencyNotRatable},
		{"malformed code", usdOne, "US", currency.RoundHalfEven, "", pkgerr.ErrInvalidCurrencyCode},
		{"unknown code", usdOne, codeUnknown, currency.RoundHalfEven, "", pkgerr.ErrUnknownCurrency},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			amount := mustDecimal(t, tc.amount)
			got, err := currency.Round(amount, tc.code, tc.mode)

			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("Round(%s, %s, mode) error = %v; want %v", tc.amount, tc.code, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Round(%s, %s, mode): unexpected error %v", tc.amount, tc.code, err)
			}
			want := mustDecimal(t, tc.want)
			if !got.Equal(want) {
				t.Errorf("Round(%s, %s, mode) = %s; want %s", tc.amount, tc.code, got, want)
			}
		})
	}
}

// TestRoundDoesNotMutateInput verifies decimal immutability and that the result
// is expressed at the currency scale.
func TestRoundDoesNotMutateInput(t *testing.T) {
	cases := []struct {
		name         string
		amount       string
		code         currency.Code
		mode         currency.RoundingMode
		want         string
		wantExponent int32
	}{
		{codeUSD, amtTieEven, codeUSD, currency.RoundHalfEven, usdOne, -2},
		{codeJPY, jpyHalf, codeJPY, currency.RoundHalfEven, "100", 0},
		{"BHD", bhdTie, codeBHD, currency.RoundHalfEven, bhdOne, -3},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := mustDecimal(t, tc.amount)
			before := input.String()

			got, err := currency.Round(input, tc.code, tc.mode)
			if err != nil {
				t.Fatalf("Round(%s): %v", tc.amount, err)
			}
			if input.String() != before {
				t.Errorf("Round mutated input: got %s; want %s", input, before)
			}
			if !got.Equal(mustDecimal(t, tc.want)) {
				t.Errorf("Round(%s) = %s; want %s", tc.amount, got, tc.want)
			}
			if got.Exponent() != tc.wantExponent {
				t.Errorf("Round(%s) exponent = %d; want %d (currency scale)", tc.amount, got.Exponent(), tc.wantExponent)
			}
		})
	}
}

// TestRoundIdempotent verifies that applying any mode twice is stable.
func TestRoundIdempotent(t *testing.T) {
	values := []struct {
		amount string
		code   currency.Code
	}{
		{amtTieEven, codeUSD},
		{amtNegTieEven, codeUSD},
		{amtTieOdd, codeUSD},
		{jpyHalf, codeJPY},
		{jpyNegHalf, codeJPY},
		{bhdTie, codeBHD},
		{bhdNegTie, codeBHD},
	}

	for _, tc := range roundModeNames {
		for _, value := range values {
			t.Run(tc.name+"/"+value.amount, func(t *testing.T) {
				first, err := currency.Round(mustDecimal(t, value.amount), value.code, tc.mode)
				if err != nil {
					t.Fatalf("first Round(%s, %s): %v", value.amount, value.code, err)
				}
				second, err := currency.Round(first, value.code, tc.mode)
				if err != nil {
					t.Fatalf("second Round(%s, %s): %v", first, value.code, err)
				}
				if !second.Equal(first) {
					t.Errorf("rounding twice changed %s: %s != %s", value.amount, first, second)
				}
			})
		}
	}

	exact := []struct {
		amount string
		code   currency.Code
	}{
		{amtScaleThree, codeUSD},
		{"100", codeJPY},
		{amtScaleThree, codeBHD},
	}
	for _, tc := range exact {
		t.Run("Unnecessary/"+tc.amount, func(t *testing.T) {
			first, err := currency.Round(mustDecimal(t, tc.amount), tc.code, currency.RoundUnnecessary)
			if err != nil {
				t.Fatalf("first Round(%s, %s): %v", tc.amount, tc.code, err)
			}
			second, err := currency.Round(first, tc.code, currency.RoundUnnecessary)
			if err != nil {
				t.Fatalf("second Round(%s, %s): %v", first, tc.code, err)
			}
			if !second.Equal(first) {
				t.Errorf("rounding twice changed %s: %s != %s", tc.amount, first, second)
			}
		})
	}
}

func mustDecimal(t *testing.T, value string) decimal.Decimal {
	t.Helper()
	d, err := decimal.NewFromString(value)
	if err != nil {
		t.Fatalf("decimal.NewFromString(%q): %v", value, err)
	}
	return d
}

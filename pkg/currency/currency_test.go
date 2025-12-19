// SPDX-License-Identifier: MIT

package currency_test

import (
	"nomledger/pkg/currency"
	"testing"

	"github.com/shopspring/decimal"
)

func TestGetPrecision(t *testing.T) {
	cases := []struct {
		code     string
		expected int32
	}{
		{"USD", 2},
		{"usd", 2}, // Case insensitivity check
		{"JPY", 0},
		{"BHD", 3},
		{"CLF", 4},
		{"XYZ", 2}, // Unknown defaults to 2
	}

	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			got := currency.GetPrecision(tc.code)
			if got != tc.expected {
				t.Errorf("GetPrecision(%s) = %d; want %d", tc.code, got, tc.expected)
			}
		})
	}
}

func TestRoundToPrecision(t *testing.T) {
	// Banker's Rounding (Half-to-Even) Logic Verification
	cases := []struct {
		name     string
		amount   string // String to ensure exact decimal representation
		currency string
		expected string
	}{
		// Standard Rounding (USD - 2 decimals)
		{"USD Normal Down", "10.123", "USD", "10.12"},
		{"USD Normal Up", "10.126", "USD", "10.13"},

		// Banker's Rounding Edge Cases (Half to Even)
		{"USD Half Down to Even", "10.125", "USD", "10.12"}, // 2 is even
		{"USD Half Up to Even", "10.135", "USD", "10.14"},   // 4 is even

		// Zero Precision (JPY)
		{"JPY Normal", "100.1", "JPY", "100"},
		{"JPY Half Down", "100.5", "JPY", "100"}, // 0 is even
		{"JPY Half Up", "101.5", "JPY", "102"},   // 2 is even

		// High Precision (BHD - 3 decimals)
		{"BHD Precision", "10.1234", "BHD", "10.123"},
		{"BHD Rounding", "10.1235", "BHD", "10.124"}, // 4 is even
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input, _ := decimal.NewFromString(tc.amount)
			want, _ := decimal.NewFromString(tc.expected)

			got := currency.RoundToPrecision(input, tc.currency)

			if !got.Equal(want) {
				t.Errorf("RoundToPrecision(%s, %s) = %s; want %s",
					tc.amount, tc.currency, got.String(), want.String())
			}
		})
	}
}

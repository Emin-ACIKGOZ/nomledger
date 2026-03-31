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
		found    bool
	}{
		{"USD", 2, true},
		{"JPY", 0, true},
		{"ABC", 0, false},
	}

	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			got, ok := currency.GetPrecision(tc.code)
			if ok != tc.found || (ok && got != tc.expected) {
				t.Errorf("GetPrecision(%s) = %d, %v; want %d, %v", tc.code, got, ok, tc.expected, tc.found)
			}
		})
	}
}

func TestRoundToPrecision_ErrorOnUnknown(t *testing.T) {
	t.Run("Reject Unknown Currency", func(t *testing.T) {
		_, err := currency.RoundToPrecision(decimal.NewFromInt(100), "UNKNOWN")
		if err == nil {
			t.Error("Expected error for unknown currency, got nil")
		}
	})
}

func TestRoundToPrecision(t *testing.T) {
	cases := []struct {
		name     string
		amount   string
		currency string
		expected string
	}{
		// Standard Rounding (USD - 2 decimals)
		{"USD Normal Down", "10.123", "USD", "10.12"},
		{"USD Normal Up", "10.126", "USD", "10.13"},

		// Banker's Rounding (Half to Even)
		{"USD Half Down to Even", "10.125", "USD", "10.12"},
		{"USD Half Up to Even", "10.135", "USD", "10.14"},

		// Zero Precision (JPY)
		{"JPY Normal", "100.1", "JPY", "100"},
		{"JPY Half Down", "100.5", "JPY", "100"},
		{"JPY Half Up", "101.5", "JPY", "102"},

		// High Precision (BHD - 3 decimals)
		{"BHD Precision", "10.1234", "BHD", "10.123"},
		{"BHD Rounding", "10.1235", "BHD", "10.124"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input, err := decimal.NewFromString(tc.amount)
			if err != nil {
				t.Fatalf("Invalid input amount in test case: %s", tc.amount)
			}
			want, err := decimal.NewFromString(tc.expected)
			if err != nil {
				t.Fatalf("Invalid expected amount in test case: %s", tc.expected)
			}

			got, err := currency.RoundToPrecision(input, tc.currency)
			if err != nil {
				t.Errorf("RoundToPrecision(%s, %s) returned unexpected error: %v", tc.amount, tc.currency, err)
				return
			}

			if !got.Equal(want) {
				t.Errorf("RoundToPrecision(%s, %s) = %s; want %s",
					tc.amount, tc.currency, got.String(), want.String())
			}
		})
	}
}

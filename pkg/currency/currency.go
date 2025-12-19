// SPDX-License-Identifier: MIT

package currency

import (
	"strings"

	"github.com/shopspring/decimal"
)

const (
	defaultPrecision = 2
	precisionBHD     = 3
	precisionCLF     = 4
)

// precisionTable holds the ISO 4217 exponents for supported currencies.
// In a production environment, this might be loaded from a config or external standard.
var precisionTable = map[string]int32{
	"USD": defaultPrecision, // United States Dollar
	"EUR": defaultPrecision, // Euro
	"GBP": defaultPrecision, // British Pound Sterling
	"JPY": 0,                // Japanese Yen
	"TRY": defaultPrecision, // Turkish Lira
	"AUD": defaultPrecision, // Australian Dollar
	"BHD": precisionBHD,     // Bahraini Dinar
	"CLF": precisionCLF,     // Unidad de Fomento
}

// GetPrecision returns the ISO 4217 exponent for a given currency code.
//
// If the code is unknown, it defaults to 2. This behavior allows the system
// to handle standard currencies gracefully without strict configuration requirements,
// though production systems should ideally ensure all codes are known.
func GetPrecision(code string) int32 {
	p, ok := precisionTable[strings.ToUpper(code)]
	if !ok {
		return defaultPrecision
	}
	return p
}

// RoundToPrecision aligns the amount to the specific precision of the provided currency.
//
// It utilizes Banker's Rounding (Half-to-Even) to minimize bias in accumulated
// operations.
func RoundToPrecision(amount decimal.Decimal, currencyCode string) decimal.Decimal {
	precision := GetPrecision(currencyCode)
	return amount.RoundBank(precision)
}

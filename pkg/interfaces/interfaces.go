// SPDX-License-Identifier: MIT

package interfaces

import "github.com/shopspring/decimal"

// RateProvider abstracts the source of foreign exchange rates.
//
// Implementations are responsible for fetching rates from databases, APIs, or caches.
// The core engine expects this interface to be thread-safe if the Ledger is used concurrently.
type RateProvider interface {
	// GetRate returns the exchange rate for a currency pair at a specific time.
	// It should return an error if the rate cannot be determined.
	GetRate(base, quote string) (decimal.Decimal, error)
}
